package logging

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// problemsLogPath returns today's aggregated problems log inside a workspace.
func problemsLogPath(ws string) string {
	return filepath.Join(ws, ".nerd", "logs", time.Now().Format("2006-01-02")+"_problems.log")
}

// Every WARN and ERROR must land in one aggregated file, tagged with the
// category it came from. Without it, diagnosing a run means grepping ~25
// category files and interleaving them by hand -- which is how a cold start
// reported success while 195 of 196 LLM calls were failing.
func TestProblemsLog_AggregatesWarnAndErrorAcrossCategories(t *testing.T) {
	resetLoggingState(t)
	defer resetLoggingState(t)
	ws := setupDebugWorkspace(t)

	if err := Initialize(ws); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	logLevel = LevelDebug

	Get(CategoryKernel).Warn("kernel is unhappy: %d", 7)
	Get(CategorySession).Error("session blew up: %v", "boom")
	Get(CategoryPerception).Info("this must NOT be aggregated")
	Get(CategoryPerception).Debug("nor this")

	CloseAll()

	data, err := os.ReadFile(problemsLogPath(ws))
	if err != nil {
		t.Fatalf("problems log not written: %v", err)
	}
	got := string(data)

	for _, want := range []string{
		"[WARN] [kernel] kernel is unhappy: 7",
		"[ERROR] [session] session blew up: boom",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("problems log missing %q\n--- got ---\n%s", want, got)
		}
	}
	// Info/Debug are the bulk of the volume; aggregating them would recreate
	// the haystack this file exists to remove.
	for _, unwanted := range []string{"must NOT be aggregated", "nor this"} {
		if strings.Contains(got, unwanted) {
			t.Errorf("problems log should only carry WARN/ERROR, found %q", unwanted)
		}
	}
}

// The aggregate is a MIRROR, not a move: anything reading a category file today
// must still find its own WARN/ERROR lines there.
func TestProblemsLog_DoesNotStealFromCategoryFiles(t *testing.T) {
	resetLoggingState(t)
	defer resetLoggingState(t)
	ws := setupDebugWorkspace(t)

	if err := Initialize(ws); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	logLevel = LevelDebug

	Get(CategoryKernel).Error("still in the category file")
	CloseAll()

	catPath := filepath.Join(ws, ".nerd", "logs", time.Now().Format("2006-01-02")+"_kernel.log")
	data, err := os.ReadFile(catPath)
	if err != nil {
		t.Fatalf("category log not written: %v", err)
	}
	if !strings.Contains(string(data), "still in the category file") {
		t.Error("category log lost its ERROR line; the problems log must mirror, not move")
	}
}

// Logging must never take down the process it observes, so a WARN before
// Initialize() (no logs dir yet) has to be a silent no-op rather than a panic.
func TestProblemsLog_NoPanicBeforeInitialize(t *testing.T) {
	resetLoggingState(t)
	defer resetLoggingState(t)

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("logging before Initialize panicked: %v", r)
		}
	}()
	Get(CategoryKernel).Warn("no logs dir yet")
	Get(CategoryKernel).Error("still no logs dir")
}
