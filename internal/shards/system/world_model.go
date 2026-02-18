// world_model.go implements the World Model Ingestor system shard.
//
// The World Model Ingestor maintains the fact-based filesystem:
// - file_topology facts (path, hash, language, modified time)
// - diagnostic facts (errors, warnings from linters/build)
// - symbol_graph facts (functions, classes, interfaces)
// - dependency_link facts (import relationships)
//
// This shard is ON-DEMAND (starts when workspace is loaded) and HYBRID:
// - Deterministic for fact generation (file watching, AST parsing)
// - LLM for semantic interpretation of complex changes
package system

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"codenerd/internal/core"
	"codenerd/internal/logging"
	"codenerd/internal/types"
	"codenerd/internal/world"
)

// FileInfo represents a tracked file.
type FileInfo struct {
	Path         string
	Hash         string
	Language     string
	LastModified time.Time
	IsTestFile   bool
	Size         int64
}

// Diagnostic represents a code diagnostic (error, warning).
type Diagnostic struct {
	Severity string // "error", "warning", "info"
	FilePath string
	Line     int
	Code     string
	Message  string
}

// Symbol represents a code symbol.
type Symbol struct {
	ID         string
	Type       string // "function", "class", "interface", "method", "variable"
	Name       string
	Visibility string // "public", "private", "internal"
	FilePath   string
	Line       int
	Signature  string
}

// Dependency represents an import/dependency relationship.
type Dependency struct {
	CallerID   string
	CalleeID   string
	ImportPath string
}

// WorldModelConfig holds configuration for the world model ingestor.
type WorldModelConfig struct {
	// Workspace
	RootPath        string   // Workspace root
	IncludePatterns []string // File patterns to include
	ExcludePatterns []string // File patterns to exclude

	// Performance
	TickInterval       time.Duration // How often to scan for changes
	IdleTimeout        time.Duration // Auto-stop after no changes
	MaxFilesPerScan    int           // Limit files per tick
	HashOnlyLargeFiles bool          // Skip content analysis for large files
	LargeFileThreshold int64         // Bytes threshold for "large"

	// Features
	EnableSymbolGraph  bool // Parse AST for symbols
	EnableDiagnostics  bool // Run linters
	EnableDependencies bool // Track imports
}

// DefaultWorldModelConfig returns sensible defaults.
func DefaultWorldModelConfig() WorldModelConfig {
	return WorldModelConfig{
		RootPath: ".",
		IncludePatterns: []string{
			"*.go", "*.py", "*.js", "*.ts", "*.tsx",
			"*.java", "*.rs", "*.c", "*.cpp", "*.h",
			"*.md", "*.json", "*.yaml", "*.yml",
			"*.pdf", "*.txt",
		},
		ExcludePatterns: []string{
			"vendor/*", "node_modules/*", ".git/*",
			"*.exe", "*.dll", "*.so", "*.dylib",
			"*.bin", "*.dat",
		},
		TickInterval:       5 * time.Second,
		IdleTimeout:        5 * time.Minute,
		MaxFilesPerScan:    100,
		HashOnlyLargeFiles: true,
		LargeFileThreshold: 1024 * 1024, // 1MB
		EnableSymbolGraph:  true,
		EnableDiagnostics:  true,
		EnableDependencies: true,
	}
}

// WorldModelIngestorShard maintains the fact-based filesystem.
type WorldModelIngestorShard struct {
	*BaseSystemShard
	mu sync.RWMutex

	// Configuration
	config WorldModelConfig

	// State
	files        map[string]FileInfo
	symbols      map[string]Symbol
	dependencies []Dependency
	diagnostics  []Diagnostic

	parser *world.ASTParser

	// Change tracking
	lastScan     time.Time
	changeCount  int
	lastActivity time.Time

	// Running state
	running bool
}

// NewWorldModelIngestorShard creates a new World Model Ingestor shard.
func NewWorldModelIngestorShard() *WorldModelIngestorShard {
	return NewWorldModelIngestorShardWithConfig(DefaultWorldModelConfig())
}

// NewWorldModelIngestorShardWithConfig creates a world model ingestor with custom config.
func NewWorldModelIngestorShardWithConfig(cfg WorldModelConfig) *WorldModelIngestorShard {
	base := NewBaseSystemShard("world_model_ingestor", StartupOnDemand)

	// Configure permissions
	base.Config.Permissions = []types.ShardPermission{
		types.PermissionReadFile,
		types.PermissionExecCmd,
		types.PermissionCodeGraph,
	}
	base.Config.Model = types.ModelConfig{
		Capability: types.CapabilityHighSpeed, // Use fast model for interpretations
	}

	// Configure idle timeout
	base.CostGuard.IdleTimeout = cfg.IdleTimeout

	return &WorldModelIngestorShard{
		BaseSystemShard: base,
		config:          cfg,
		files:           make(map[string]FileInfo),
		symbols:         make(map[string]Symbol),
		dependencies:    make([]Dependency, 0),
		diagnostics:     make([]Diagnostic, 0),
		parser:          world.NewASTParser(),
		lastActivity:    time.Now(),
	}
}

// Execute runs the World Model Ingestor's continuous scanning loop.
func (w *WorldModelIngestorShard) Execute(ctx context.Context, task string) (string, error) {
	w.SetState(types.ShardStateRunning)
	w.mu.Lock()
	w.running = true
	w.StartTime = time.Now()
	w.lastActivity = time.Now()
	w.mu.Unlock()

	defer func() {
		w.SetState(types.ShardStateCompleted)
		w.mu.Lock()
		w.running = false
		if w.parser != nil {
			w.parser.Close()
		}
		w.mu.Unlock()
	}()

	// Initialize kernel if not set
	if w.Kernel == nil {
		kernel, err := core.NewRealKernel()
		if err != nil {
			return "", fmt.Errorf("failed to create kernel: %w", err)
		}
		w.Kernel = kernel
	}

	// Parse task for root path
	if task != "" {
		w.config.RootPath = task
	}

	// Initial full scan
	if err := w.performFullScan(ctx); err != nil {
		return "", fmt.Errorf("initial scan failed: %w", err)
	}

	ticker := time.NewTicker(w.config.TickInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return w.generateShutdownSummary("context cancelled"), ctx.Err()
		case <-w.StopCh:
			return w.generateShutdownSummary("stopped"), nil
		case <-ticker.C:
			// Check for trigger fact (Fix 15.1: World Model Event-Loop Breakage)
			updatingFacts, _ := w.Kernel.Query("world_model_updating")
			if len(updatingFacts) > 0 {
				logging.SystemShardsDebug("[WorldModel] Triggered by world_model_updating fact")
				if err := w.performIncrementalScan(ctx); err != nil {
					_ = w.Kernel.Assert(types.Fact{
						Predicate: "world_model_error",
						Args:      []interface{}{err.Error(), time.Now().Unix()},
					})
				}
				// Retract trigger
				_ = w.Kernel.Retract("world_model_updating")
				// Reset idle timer
				w.mu.Lock()
				w.lastActivity = time.Now()
				w.mu.Unlock()
				continue
			}

			// Check idle timeout
			if w.CostGuard.IsIdle() {
				return w.generateShutdownSummary("idle timeout"), nil
			}

			// Incremental scan
			if err := w.performIncrementalScan(ctx); err != nil {
				_ = w.Kernel.Assert(types.Fact{
					Predicate: "world_model_error",
					Args:      []interface{}{err.Error(), time.Now().Unix()},
				})
			}

			// Emit heartbeat
			_ = w.Kernel.Assert(types.Fact{
				Predicate: "world_model_heartbeat",
				Args:      []interface{}{w.ID, len(w.files), time.Now().Unix()},
			})

			// Check for autopoiesis
			if w.Autopoiesis.ShouldPropose() {
				w.handleAutopoiesis(ctx)
			}
		}
	}
}

// performFullScan does a complete workspace scan.
func (w *WorldModelIngestorShard) performFullScan(ctx context.Context) error {
	w.mu.Lock()
	w.files = make(map[string]FileInfo)
	w.symbols = make(map[string]Symbol)
	w.dependencies = make([]Dependency, 0)
	w.mu.Unlock()

	batchFacts := make([]types.Fact, 0)

	err := filepath.Walk(w.config.RootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		// Check context
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Skip directories
		if info.IsDir() {
			// Check exclude patterns
			for _, pattern := range w.config.ExcludePatterns {
				if matched, _ := filepath.Match(pattern, info.Name()); matched {
					return filepath.SkipDir
				}
				if strings.Contains(path, strings.TrimSuffix(pattern, "/*")) {
					return filepath.SkipDir
				}
			}
			return nil
		}

		// Check include patterns
		included := false
		for _, pattern := range w.config.IncludePatterns {
			if matched, _ := filepath.Match(pattern, info.Name()); matched {
				included = true
				break
			}
		}
		if !included {
			return nil
		}

		// Check exclude patterns
		for _, pattern := range w.config.ExcludePatterns {
			if matched, _ := filepath.Match(pattern, path); matched {
				return nil
			}
		}

		// Process file
		fileInfo, facts, err := w.processFile(ctx, path, info)
		if err != nil {
			return nil // Skip errors
		}

		w.mu.Lock()
		w.files[path] = fileInfo
		w.mu.Unlock()

		// Emit file_topology fact
		ft := types.Fact{
			Predicate: "file_topology",
			Args: []interface{}{
				fileInfo.Path,
				fileInfo.Hash,
				fileInfo.Language,
				fileInfo.LastModified.Unix(),
				fileInfo.IsTestFile,
			},
		}
		facts = append(facts, ft)

		// Batch all facts for a single evaluation pass.
		batchFacts = append(batchFacts, facts...)

		// Persist to knowledge.db if available
		w.persistToKnowledge(facts)

		return nil
	})
	if err != nil {
		return err
	}

	if len(batchFacts) > 0 {
		return w.Kernel.AssertBatch(batchFacts)
	}
	return nil
}

// performIncrementalScan checks for changes since last scan.
func (w *WorldModelIngestorShard) performIncrementalScan(ctx context.Context) error {
	changedFiles := 0
	batchFacts := make([]types.Fact, 0)

	err := filepath.Walk(w.config.RootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		// Limit files per scan
		if changedFiles >= w.config.MaxFilesPerScan {
			return nil
		}

		// Check context
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Skip excluded patterns
		for _, pattern := range w.config.ExcludePatterns {
			if strings.Contains(path, strings.TrimSuffix(pattern, "/*")) {
				return nil
			}
		}

		w.mu.RLock()
		existing, exists := w.files[path]
		w.mu.RUnlock()

		// Check if file changed
		if exists && existing.LastModified.Equal(info.ModTime()) {
			return nil
		}

		// Process changed file
		fileInfo, facts, err := w.processFile(ctx, path, info)
		if err != nil {
			return nil
		}

		w.mu.Lock()
		w.files[path] = fileInfo
		w.changeCount++
		w.lastActivity = time.Now()
		w.mu.Unlock()

		changedFiles++

		// Emit updated file_topology fact
		ft := types.Fact{
			Predicate: "file_topology",
			Args: []interface{}{
				fileInfo.Path,
				fileInfo.Hash,
				fileInfo.Language,
				fileInfo.LastModified.Unix(),
				fileInfo.IsTestFile,
			},
		}
		// Mark file as modified for impact analysis
		mod := types.Fact{
			Predicate: "modified",
			Args:      []interface{}{fileInfo.Path},
		}

		facts = append(facts, ft, mod)
		batchFacts = append(batchFacts, facts...)

		// Persist updated facts to knowledge.db
		w.persistToKnowledge(facts)

		return nil
	})

	w.mu.Lock()
	w.lastScan = time.Now()
	w.mu.Unlock()

	if err != nil {
		return err
	}

	if len(batchFacts) > 0 {
		return w.Kernel.AssertBatch(batchFacts)
	}
	return nil
}

// processFile extracts information from a file.
func (w *WorldModelIngestorShard) processFile(ctx context.Context, path string, info os.FileInfo) (FileInfo, []types.Fact, error) {
	fi := FileInfo{
		Path:         path,
		LastModified: info.ModTime(),
		Size:         info.Size(),
		Language:     detectLanguage(path),
		IsTestFile:   isTestFile(path),
	}
	var facts []types.Fact

	// Compute hash
	if w.config.HashOnlyLargeFiles && info.Size() > w.config.LargeFileThreshold {
		// For large files, use size+mtime as pseudo-hash
		fi.Hash = fmt.Sprintf("size:%d:mtime:%d", info.Size(), info.ModTime().Unix())
	} else {
		content, err := os.ReadFile(path)
		if err != nil {
			return fi, facts, err
		}
		hash := sha256.Sum256(content)
		fi.Hash = hex.EncodeToString(hash[:])
	}

	// Parse AST for symbols/dependencies if enabled
	if w.config.EnableSymbolGraph || w.config.EnableDependencies {
		if w.parser != nil {
			parsedFacts, err := w.parser.Parse(path)
			if err == nil && len(parsedFacts) > 0 {
				facts = append(facts, parsedFacts...)
			}
		}
	}

	return fi, facts, nil
}

// detectLanguage determines the programming language from file extension.
func detectLanguage(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".go":
		return "go"
	case ".py":
		return "python"
	case ".js":
		return "javascript"
	case ".ts", ".tsx":
		return "typescript"
	case ".java":
		return "java"
	case ".rs":
		return "rust"
	case ".c", ".h":
		return "c"
	case ".cpp", ".hpp", ".cc":
		return "cpp"
	case ".rb":
		return "ruby"
	case ".php":
		return "php"
	case ".swift":
		return "swift"
	case ".kt":
		return "kotlin"
	case ".cs":
		return "csharp"
	case ".md":
		return "markdown"
	case ".json":
		return "json"
	case ".yaml", ".yml":
		return "yaml"
	case ".pdf":
		return "pdf"
	case ".txt":
		return "text"
	default:
		return "unknown"
	}
}

// isTestFile checks if a file is a test file.
func isTestFile(path string) bool {
	base := filepath.Base(path)
	patterns := []string{
		"_test.go",
		"_test.py",
		".test.js", ".test.ts", ".test.tsx",
		".spec.js", ".spec.ts", ".spec.tsx",
		"Test.java",
		"_test.rs",
	}
	for _, pattern := range patterns {
		if strings.HasSuffix(base, pattern) || strings.Contains(base, pattern) {
			return true
		}
	}
	return false
}

// handleAutopoiesis uses LLM for semantic interpretation.
func (w *WorldModelIngestorShard) handleAutopoiesis(ctx context.Context) {
	cases := w.Autopoiesis.GetUnhandledCases()
	if len(cases) == 0 {
		return
	}

	if w.LLMClient == nil {
		return
	}

	can, _ := w.CostGuard.CanCall()
	if !can {
		for _, cas := range cases {
			w.Autopoiesis.RecordUnhandled(cas.Query, cas.Context, cas.FactsAtTime)
		}
		return
	}

	// Use LLM to interpret complex changes
	userPrompt := w.buildInterpretationPrompt(cases)

	// Use JIT prompt compilation (no fallback - atoms in internal/prompt/atoms/system/autopoiesis.yaml)
	systemPrompt, jitUsed := w.TryJITPrompt(ctx, "world_model_autopoiesis")
	if !jitUsed || systemPrompt == "" {
		logging.SystemShards("[WorldModel] [ERROR] JIT compilation failed - skipping autopoiesis (ensure system/autopoiesis atoms exist)")
		for _, cas := range cases {
			w.Autopoiesis.RecordUnhandled(cas.Query, cas.Context, cas.FactsAtTime)
		}
		return
	}
	logging.SystemShards("[WorldModel] [JIT] Using JIT-compiled autopoiesis prompt")

	result, err := w.GuardedLLMCall(ctx, systemPrompt, userPrompt)
	if err != nil {
		return
	}

	// Parse interpretation and emit facts
	w.applyInterpretation(result)
}

// buildInterpretationPrompt creates a prompt for semantic interpretation.
func (w *WorldModelIngestorShard) buildInterpretationPrompt(cases []UnhandledCase) string {
	var sb strings.Builder
	sb.WriteString("The following file changes need semantic interpretation:\n\n")

	for i, cas := range cases {
		sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, cas.Query))
		if cas.Context != nil {
			for k, v := range cas.Context {
				sb.WriteString(fmt.Sprintf("   %s: %s\n", k, v))
			}
		}
	}

	sb.WriteString("\nWhat facts should be derived from these changes?\n")
	sb.WriteString("Format: FACT: predicate(arg1, arg2, ...)\n")

	return sb.String()
}

// applyInterpretation parses LLM output and emits derived facts.
func (w *WorldModelIngestorShard) applyInterpretation(output string) {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "FACT:") {
			factStr := strings.TrimSpace(strings.TrimPrefix(line, "FACT:"))
			// Parse and emit the fact (simplified parsing)
			if idx := strings.Index(factStr, "("); idx > 0 {
				predicate := factStr[:idx]
				f := types.Fact{
					Predicate: predicate,
					Args:      []interface{}{factStr}, // Store full fact string
				}
				_ = w.Kernel.Assert(f)
				w.persistToKnowledge([]types.Fact{f})
			}
		}
	}
}

// generateShutdownSummary creates a summary of the shard's activity.
func (w *WorldModelIngestorShard) generateShutdownSummary(reason string) string {
	w.mu.RLock()
	defer w.mu.RUnlock()

	return fmt.Sprintf(
		"World Model shutdown (%s). Files: %d, Symbols: %d, Changes: %d, Runtime: %s",
		reason,
		len(w.files),
		len(w.symbols),
		w.changeCount,
		time.Since(w.StartTime).String(),
	)
}

// GetFiles returns tracked files.
func (w *WorldModelIngestorShard) GetFiles() map[string]FileInfo {
	w.mu.RLock()
	defer w.mu.RUnlock()
	result := make(map[string]FileInfo)
	for k, v := range w.files {
		result[k] = v
	}
	return result
}

// GetSymbols returns tracked symbols.
func (w *WorldModelIngestorShard) GetSymbols() map[string]Symbol {
	w.mu.RLock()
	defer w.mu.RUnlock()
	result := make(map[string]Symbol)
	for k, v := range w.symbols {
		result[k] = v
	}
	return result
}

// persistToKnowledge stores derived facts in knowledge.db via VirtualStore when available.
func (w *WorldModelIngestorShard) persistToKnowledge(facts []types.Fact) {
	if len(facts) == 0 || w.VirtualStore == nil {
		return
	}
	if err := w.VirtualStore.PersistFactsToKnowledge(facts, "fact", 6); err != nil {
		fmt.Printf("[WorldModel] Knowledge persistence warning: %v\n", err)
	}
	// Also project dependency links into knowledge_graph for fast lookup
	for _, f := range facts {
		if f.Predicate == "dependency_link" && len(f.Args) >= 2 {
			a := types.ExtractString(f.Args[0])
			b := types.ExtractString(f.Args[1])
			rel := "depends_on"
			if len(f.Args) >= 3 {
				rel = "depends_on:" + types.ExtractString(f.Args[2])
			}
			_ = w.VirtualStore.PersistLink(a, rel, b, 1.0, map[string]interface{}{"source": "world_model"})
		}
		if f.Predicate == "symbol_graph" && len(f.Args) >= 4 {
			symbolID := types.ExtractString(f.Args[0])
			filePath := types.ExtractString(f.Args[3])
			_ = w.VirtualStore.PersistLink(symbolID, "defined_in", filePath, 1.0, map[string]interface{}{"source": "world_model"})
		}
	}
}

// NOTE: Legacy worldModelAutopoiesisPrompt constant has been DELETED.
// World model autopoiesis prompts are now JIT-compiled from:
//   internal/prompt/atoms/system/autopoiesis.yaml (id: system/autopoiesis/world_model)
