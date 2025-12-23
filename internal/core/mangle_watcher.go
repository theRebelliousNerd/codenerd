package core

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"codenerd/internal/logging"

	"github.com/fsnotify/fsnotify"
)

// MangleWatcher watches .nerd/mangle/*.mg files for changes and triggers validation/repair.
// It watches workspace-relative paths so it works wherever codeNERD is running.
type MangleWatcher struct {
	mu           sync.RWMutex
	watcher      *fsnotify.Watcher
	kernel       *RealKernel
	workspaceDir string // Base workspace directory (e.g., /path/to/project)
	mangleDir    string // Full path to watch (e.g., /path/to/project/.nerd/mangle)
	debounceMap  map[string]time.Time
	debounceDur  time.Duration
	stopCh       chan struct{}
	doneCh       chan struct{}
	running      bool

	// Stats for stress testing
	stats MangleWatcherStats
}

// MangleWatcherStats tracks watcher activity for stress testing and debugging.
type MangleWatcherStats struct {
	FilesCreated       int
	FilesModified      int
	FilesDeleted       int
	ValidationTriggered int
	RepairsTriggered   int
	Errors             int
	LastEventTime      time.Time
	LastEventPath      string
	LastEventType      string
}

// NewMangleWatcher creates a new MangleWatcher for the given workspace.
// workspaceDir should be the project root (e.g., /path/to/project).
// The watcher will monitor workspaceDir/.nerd/mangle/*.mg files.
func NewMangleWatcher(workspaceDir string, kernel *RealKernel) (*MangleWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	mangleDir := filepath.Join(workspaceDir, ".nerd", "mangle")

	mw := &MangleWatcher{
		watcher:      watcher,
		kernel:       kernel,
		workspaceDir: workspaceDir,
		mangleDir:    mangleDir,
		debounceMap:  make(map[string]time.Time),
		debounceDur:  500 * time.Millisecond, // Debounce rapid saves
		stopCh:       make(chan struct{}),
		doneCh:       make(chan struct{}),
	}

	return mw, nil
}

// Start begins watching the .nerd/mangle directory for changes.
// This method is non-blocking; it starts the watcher in a goroutine.
func (mw *MangleWatcher) Start(ctx context.Context) error {
	mw.mu.Lock()
	if mw.running {
		mw.mu.Unlock()
		return nil // Already running
	}
	mw.running = true
	mw.mu.Unlock()

	// Ensure the mangle directory exists
	if err := os.MkdirAll(mw.mangleDir, 0755); err != nil {
		logging.Get(logging.CategoryKernel).Warn("MangleWatcher: failed to create mangle dir %s: %v (continuing anyway)", mw.mangleDir, err)
		// Continue anyway - directory might be created later
	}

	// Add the mangle directory to the watcher
	if err := mw.watcher.Add(mw.mangleDir); err != nil {
		// Directory may not exist yet - that's OK, we'll try again
		logging.Get(logging.CategoryKernel).Warn("MangleWatcher: initial watch failed (dir may not exist): %v", err)
	} else {
		logging.Kernel("MangleWatcher: watching directory: %s", mw.mangleDir)
	}

	// Also watch the internal mangle directory if it exists
	internalDir := filepath.Join(mw.workspaceDir, "internal", "mangle")
	if _, err := os.Stat(internalDir); err == nil {
		if err := mw.watcher.Add(internalDir); err == nil {
			logging.Kernel("MangleWatcher: also watching internal: %s", internalDir)
		}
	}

	go mw.run(ctx)

	return nil
}

// Stop stops the watcher and waits for cleanup.
func (mw *MangleWatcher) Stop() {
	mw.mu.Lock()
	if !mw.running {
		mw.mu.Unlock()
		return
	}
	mw.running = false
	mw.mu.Unlock()

	close(mw.stopCh)
	<-mw.doneCh

	if err := mw.watcher.Close(); err != nil {
		logging.Get(logging.CategoryKernel).Error("MangleWatcher: error closing watcher: %v", err)
	}
	logging.Kernel("MangleWatcher: stopped")
}

// run is the main event loop for the watcher.
func (mw *MangleWatcher) run(ctx context.Context) {
	defer close(mw.doneCh)

	// Debounce timer for batching rapid changes
	debounceTicker := time.NewTicker(100 * time.Millisecond)
	defer debounceTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			logging.Kernel("MangleWatcher: context cancelled")
			return

		case <-mw.stopCh:
			logging.Kernel("MangleWatcher: stop signal received")
			return

		case event, ok := <-mw.watcher.Events:
			if !ok {
				logging.Kernel("MangleWatcher: event channel closed")
				return
			}
			mw.handleEvent(ctx, event)

		case err, ok := <-mw.watcher.Errors:
			if !ok {
				logging.Kernel("MangleWatcher: error channel closed")
				return
			}
			logging.Get(logging.CategoryKernel).Error("MangleWatcher error: %v", err)
			mw.mu.Lock()
			mw.stats.Errors++
			mw.mu.Unlock()

		case <-debounceTicker.C:
			mw.processDebouncedEvents(ctx)
		}
	}
}

// handleEvent processes a single filesystem event.
func (mw *MangleWatcher) handleEvent(ctx context.Context, event fsnotify.Event) {
	// Only care about .mg files
	if !strings.HasSuffix(event.Name, ".mg") {
		return
	}

	// Determine event type
	var eventType string
	switch {
	case event.Op&fsnotify.Create != 0:
		eventType = "create"
	case event.Op&fsnotify.Write != 0:
		eventType = "modify"
	case event.Op&fsnotify.Remove != 0:
		eventType = "delete"
	case event.Op&fsnotify.Rename != 0:
		eventType = "rename"
	default:
		return // Ignore chmod, etc.
	}

	logging.KernelDebug("MangleWatcher: %s event for %s", eventType, event.Name)

	// Update stats
	mw.mu.Lock()
	mw.stats.LastEventTime = time.Now()
	mw.stats.LastEventPath = event.Name
	mw.stats.LastEventType = eventType

	switch eventType {
	case "create":
		mw.stats.FilesCreated++
	case "modify":
		mw.stats.FilesModified++
	case "delete", "rename":
		mw.stats.FilesDeleted++
	}

	// Debounce: record the event for later processing
	mw.debounceMap[event.Name] = time.Now()
	mw.mu.Unlock()
}

// processDebouncedEvents processes events that have settled past the debounce window.
func (mw *MangleWatcher) processDebouncedEvents(ctx context.Context) {
	mw.mu.Lock()
	now := time.Now()
	toProcess := make([]string, 0)

	for path, eventTime := range mw.debounceMap {
		if now.Sub(eventTime) >= mw.debounceDur {
			toProcess = append(toProcess, path)
			delete(mw.debounceMap, path)
		}
	}
	mw.mu.Unlock()

	// Process settled events
	for _, path := range toProcess {
		mw.validateAndRepair(ctx, path)
	}
}

// validateAndRepair validates a .mg file and triggers repair if needed.
func (mw *MangleWatcher) validateAndRepair(ctx context.Context, path string) {
	logging.Kernel("MangleWatcher: validating file: %s", path)

	// Check if file still exists
	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			logging.KernelDebug("MangleWatcher: file deleted, skipping validation: %s", path)
			return
		}
		logging.Get(logging.CategoryKernel).Error("MangleWatcher: failed to read %s: %v", path, err)
		mw.mu.Lock()
		mw.stats.Errors++
		mw.mu.Unlock()
		return
	}

	mw.mu.Lock()
	mw.stats.ValidationTriggered++
	mw.mu.Unlock()

	// Validate each rule in the file
	rules := mw.extractRules(string(content))
	if len(rules) == 0 {
		logging.KernelDebug("MangleWatcher: no rules found in %s", path)
		return
	}

	// Get the repair interceptor
	interceptor := mw.kernel.GetRepairInterceptor()
	if interceptor == nil {
		logging.KernelDebug("MangleWatcher: no repair interceptor set, using basic validation")
		mw.basicValidation(path, rules)
		return
	}

	// Use the repair interceptor to validate each rule
	needsRewrite := false
	repairedRules := make([]string, 0, len(rules))

	for _, rule := range rules {
		repairedRule, err := interceptor.InterceptLearnedRule(ctx, rule)
		if err != nil {
			logging.Get(logging.CategoryKernel).Warn("MangleWatcher: rule validation failed: %v", err)
			// Comment out the invalid rule
			repairedRules = append(repairedRules, "# INVALID (MangleWatcher): "+rule)
			needsRewrite = true
			mw.mu.Lock()
			mw.stats.RepairsTriggered++
			mw.mu.Unlock()
		} else if repairedRule != rule {
			logging.Kernel("MangleWatcher: rule was repaired: %s", filepath.Base(path))
			repairedRules = append(repairedRules, repairedRule)
			needsRewrite = true
			mw.mu.Lock()
			mw.stats.RepairsTriggered++
			mw.mu.Unlock()
		} else {
			repairedRules = append(repairedRules, rule)
		}
	}

	// Rewrite file if any rules were repaired
	if needsRewrite {
		newContent := strings.Join(repairedRules, "\n\n")
		if err := os.WriteFile(path, []byte(newContent), 0644); err != nil {
			logging.Get(logging.CategoryKernel).Error("MangleWatcher: failed to write repaired file: %v", err)
			mw.mu.Lock()
			mw.stats.Errors++
			mw.mu.Unlock()
		} else {
			logging.Kernel("MangleWatcher: wrote repaired file: %s", path)
		}
	}
}

// extractRules extracts individual rules from file content.
// Rules are separated by blank lines or comments.
func (mw *MangleWatcher) extractRules(content string) []string {
	lines := strings.Split(content, "\n")
	var rules []string
	var currentRule strings.Builder

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Skip comments and empty lines between rules
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			if currentRule.Len() > 0 {
				rule := strings.TrimSpace(currentRule.String())
				if rule != "" {
					rules = append(rules, rule)
				}
				currentRule.Reset()
			}
			continue
		}

		if currentRule.Len() > 0 {
			currentRule.WriteString("\n")
		}
		currentRule.WriteString(line)
	}

	// Don't forget the last rule
	if currentRule.Len() > 0 {
		rule := strings.TrimSpace(currentRule.String())
		if rule != "" {
			rules = append(rules, rule)
		}
	}

	return rules
}

// basicValidation performs basic syntax validation when no repair interceptor is available.
func (mw *MangleWatcher) basicValidation(path string, rules []string) {
	for i, rule := range rules {
		// Basic checks
		if !strings.HasSuffix(strings.TrimSpace(rule), ".") {
			logging.Get(logging.CategoryKernel).Warn("MangleWatcher: rule %d in %s missing period", i+1, filepath.Base(path))
		}

		// Check for common AI mistakes
		if strings.Contains(rule, "\"active\"") || strings.Contains(rule, "\"pending\"") {
			logging.Get(logging.CategoryKernel).Warn("MangleWatcher: rule %d may have atom/string confusion", i+1)
		}

		if strings.Contains(rule, ".decl") || strings.Contains(rule, ".Decl") {
			logging.Get(logging.CategoryKernel).Warn("MangleWatcher: rule %d may have Souffle syntax (use Decl not .decl)", i+1)
		}
	}
}

// GetStats returns the current watcher statistics.
func (mw *MangleWatcher) GetStats() MangleWatcherStats {
	mw.mu.RLock()
	defer mw.mu.RUnlock()
	return mw.stats
}

// ResetStats resets the watcher statistics.
func (mw *MangleWatcher) ResetStats() {
	mw.mu.Lock()
	defer mw.mu.Unlock()
	mw.stats = MangleWatcherStats{}
}

// IsWatching returns true if the watcher is currently running.
func (mw *MangleWatcher) IsWatching() bool {
	mw.mu.RLock()
	defer mw.mu.RUnlock()
	return mw.running
}

// GetWatchedDirs returns the directories being watched.
func (mw *MangleWatcher) GetWatchedDirs() []string {
	return mw.watcher.WatchList()
}

// TriggerValidation manually triggers validation of all .mg files.
// Useful for startup or stress testing.
func (mw *MangleWatcher) TriggerValidation(ctx context.Context) error {
	logging.Kernel("MangleWatcher: manual validation triggered")

	entries, err := os.ReadDir(mw.mangleDir)
	if err != nil {
		if os.IsNotExist(err) {
			logging.KernelDebug("MangleWatcher: mangle dir does not exist: %s", mw.mangleDir)
			return nil
		}
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".mg") {
			continue
		}

		path := filepath.Join(mw.mangleDir, entry.Name())
		mw.validateAndRepair(ctx, path)
	}

	return nil
}
