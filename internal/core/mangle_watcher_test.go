package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
)

func TestMangleWatcher_New(t *testing.T) {
	k := setupMockKernel(t)
	defer k.Clear()

	tempDir := t.TempDir()
	mw, err := NewMangleWatcher(tempDir, k)
	if err != nil {
		t.Fatalf("Failed to create MangleWatcher: %v", err)
	}
	defer mw.watcher.Close()

	if mw.workspaceDir != tempDir {
		t.Errorf("Expected workspaceDir %s, got %s", tempDir, mw.workspaceDir)
	}

	expectedMangleDir := filepath.Join(tempDir, ".nerd", "mangle")
	if mw.mangleDir != expectedMangleDir {
		t.Errorf("Expected mangleDir %s, got %s", expectedMangleDir, mw.mangleDir)
	}

	if mw.debounceDur != 500*time.Millisecond {
		t.Errorf("Expected default debounce duration 500ms, got %v", mw.debounceDur)
	}
}

func TestMangleWatcher_StartStop(t *testing.T) {
	k := setupMockKernel(t)
	defer k.Clear()

	tempDir := t.TempDir()
	mw, err := NewMangleWatcher(tempDir, k)
	if err != nil {
		t.Fatalf("Failed to create MangleWatcher: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := mw.Start(ctx); err != nil {
		t.Fatalf("Failed to start MangleWatcher: %v", err)
	}

	if !mw.IsWatching() {
		t.Error("Expected IsWatching() to be true after start")
	}

	// Starting again should be a no-op
	if err := mw.Start(ctx); err != nil {
		t.Fatalf("Starting already running watcher failed: %v", err)
	}

	mw.Stop()

	if mw.IsWatching() {
		t.Error("Expected IsWatching() to be false after stop")
	}
}

func TestMangleWatcher_GetStatsAndReset(t *testing.T) {
	k := setupMockKernel(t)
	defer k.Clear()

	tempDir := t.TempDir()
	mw, err := NewMangleWatcher(tempDir, k)
	if err != nil {
		t.Fatalf("Failed to create MangleWatcher: %v", err)
	}
	defer mw.watcher.Close()

	mw.mu.Lock()
	mw.stats.FilesCreated = 5
	mw.stats.FilesModified = 3
	mw.stats.FilesDeleted = 2
	mw.stats.ValidationTriggered = 4
	mw.stats.RepairsTriggered = 1
	mw.stats.Errors = 0
	mw.stats.LastEventPath = "test.mg"
	mw.stats.LastEventType = "modify"
	mw.mu.Unlock()

	stats := mw.GetStats()
	if stats.FilesCreated != 5 || stats.FilesModified != 3 || stats.LastEventPath != "test.mg" {
		t.Errorf("GetStats returned incorrect stats: %+v", stats)
	}

	mw.ResetStats()
	stats = mw.GetStats()
	if stats.FilesCreated != 0 || stats.LastEventPath != "" {
		t.Errorf("ResetStats did not clear stats: %+v", stats)
	}
}

func TestMangleWatcher_GetWatchedDirs(t *testing.T) {
	k := setupMockKernel(t)
	defer k.Clear()

	tempDir := t.TempDir()
	mw, err := NewMangleWatcher(tempDir, k)
	if err != nil {
		t.Fatalf("Failed to create MangleWatcher: %v", err)
	}
	defer mw.watcher.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := mw.Start(ctx); err != nil {
		t.Fatalf("Failed to start MangleWatcher: %v", err)
	}
	defer mw.Stop()

	dirs := mw.GetWatchedDirs()
	if len(dirs) == 0 {
		t.Log("Watched directories list is empty (expected if directories do not exist yet on some OS)")
	}
}

func TestMangleWatcher_ExtractRules(t *testing.T) {
	mw := &MangleWatcher{}

	content := `
# A comment here
parent(/foo, /bar).

# Another comment
sibling(X, Y) :-
	parent(P, X),
	parent(P, Y),
	X != Y.

# Trailing comment
`
	rules := mw.extractRules(content)
	if len(rules) != 2 {
		t.Fatalf("Expected 2 rules, got %d: %q", len(rules), rules)
	}

	if rules[0] != "parent(/foo, /bar)." {
		t.Errorf("Expected first rule to be 'parent(/foo, /bar).', got %q", rules[0])
	}

	expectedSecond := "sibling(X, Y) :-\n\tparent(P, X),\n\tparent(P, Y),\n\tX != Y."
	if rules[1] != expectedSecond {
		t.Errorf("Expected second rule to match structure, got %q", rules[1])
	}
}

func TestMangleWatcher_BasicValidation(t *testing.T) {
	mw := &MangleWatcher{}

	// Basic validation logs warnings. We just ensure it does not panic.
	rules := []string{
		"missing_period(X)",
		"atom_confusion(\"active\").",
		"souffle_syntax :- .decl predicate.",
	}

	mw.basicValidation("test.mg", rules)
}

func TestMangleWatcher_HandleEventAndDebounce(t *testing.T) {
	k := setupMockKernel(t)
	defer k.Clear()

	tempDir := t.TempDir()
	mw, err := NewMangleWatcher(tempDir, k)
	if err != nil {
		t.Fatalf("Failed to create MangleWatcher: %v", err)
	}
	defer mw.watcher.Close()

	mw.debounceDur = 1 * time.Millisecond

	ctx := context.Background()

	// 1. Non-mg files should be ignored
	mw.handleEvent(ctx, fsnotify.Event{
		Name: "test.txt",
		Op:   fsnotify.Write,
	})
	if len(mw.debounceMap) != 0 {
		t.Error("Expected text file event to be ignored")
	}

	// 2. mg files events
	mw.handleEvent(ctx, fsnotify.Event{
		Name: filepath.Join(mw.mangleDir, "test1.mg"),
		Op:   fsnotify.Create,
	})
	mw.handleEvent(ctx, fsnotify.Event{
		Name: filepath.Join(mw.mangleDir, "test2.mg"),
		Op:   fsnotify.Write,
	})
	mw.handleEvent(ctx, fsnotify.Event{
		Name: filepath.Join(mw.mangleDir, "test3.mg"),
		Op:   fsnotify.Remove,
	})

	stats := mw.GetStats()
	if stats.FilesCreated != 1 || stats.FilesModified != 1 || stats.FilesDeleted != 1 {
		t.Errorf("Incorrect event counts in stats: %+v", stats)
	}

	if len(mw.debounceMap) != 3 {
		t.Errorf("Expected 3 items in debounce map, got %d", len(mw.debounceMap))
	}

	// 3. Process debounced events before they are settled (should do nothing)
	mw.processDebouncedEvents(ctx)
	if len(mw.debounceMap) != 3 {
		t.Error("Expected events to remain in debounce map before settle time")
	}

	// 4. Wait for settle and process
	time.Sleep(2 * time.Millisecond)

	// Create dummy files for test1 and test2 so they can be read
	err = os.MkdirAll(mw.mangleDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create mangle dir: %v", err)
	}

	ruleContent := "parent(/a, /b).\n"
	err = os.WriteFile(filepath.Join(mw.mangleDir, "test1.mg"), []byte(ruleContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write test1.mg: %v", err)
	}

	mw.processDebouncedEvents(ctx)
	if len(mw.debounceMap) != 0 {
		t.Errorf("Expected all settled events to be processed and removed, got %d remaining", len(mw.debounceMap))
	}
}

func TestMangleWatcher_TriggerValidation(t *testing.T) {
	k := setupMockKernel(t)
	defer k.Clear()

	tempDir := t.TempDir()
	mw, err := NewMangleWatcher(tempDir, k)
	if err != nil {
		t.Fatalf("Failed to create MangleWatcher: %v", err)
	}
	defer mw.watcher.Close()

	ctx := context.Background()

	// Direct manual validation on empty dir
	if err := mw.TriggerValidation(ctx); err != nil {
		t.Fatalf("TriggerValidation failed on empty dir: %v", err)
	}

	// Create mangleDir and write a .mg file
	err = os.MkdirAll(mw.mangleDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create mangleDir: %v", err)
	}

	err = os.WriteFile(filepath.Join(mw.mangleDir, "manual.mg"), []byte("fact(/x, /y).\n"), 0644)
	if err != nil {
		t.Fatalf("Failed to write manual.mg: %v", err)
	}

	if err := mw.TriggerValidation(ctx); err != nil {
		t.Fatalf("TriggerValidation failed: %v", err)
	}

	stats := mw.GetStats()
	if stats.ValidationTriggered != 1 {
		t.Errorf("Expected 1 validation triggered, got %d", stats.ValidationTriggered)
	}
}

func TestMangleWatcher_ValidateAndRepair_FailedRead(t *testing.T) {
	k := setupMockKernel(t)
	defer k.Clear()

	tempDir := t.TempDir()
	mw, err := NewMangleWatcher(tempDir, k)
	if err != nil {
		t.Fatalf("Failed to create MangleWatcher: %v", err)
	}
	defer mw.watcher.Close()

	ctx := context.Background()

	// Calling validateAndRepair on non-existent path should return without error/panic
	mw.validateAndRepair(ctx, filepath.Join(mw.mangleDir, "does-not-exist.mg"))

	stats := mw.GetStats()
	if stats.Errors != 0 {
		t.Errorf("Expected 0 errors for non-existent file (treated as deleted), got %d", stats.Errors)
	}
}
