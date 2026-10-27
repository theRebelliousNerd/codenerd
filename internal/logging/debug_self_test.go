package logging

import (
	"strings"
	"testing"
)

// debug_self implies debug mode from disk too: without it no log files
// exist for the debug-self watcher to read.
func TestDebugSelf_FromDiskImpliesDebugModeAndWritesLogs(t *testing.T) {
	ws := newWorkspace(t, `"debug_mode": false, "debug_self": true, "level": "debug"`)
	resetAllLoggingState(t)
	defer resetAllLoggingState(t)

	if err := Initialize(ws); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	if !IsDebugMode() || !IsDebugSelf() {
		t.Fatalf("IsDebugMode=%v IsDebugSelf=%v, want both true", IsDebugMode(), IsDebugSelf())
	}
	Get(CategoryKernel).Error("debug-self disk line")
	CloseAll()
	if !strings.Contains(readLog(t, ws, "problems"), "[ERROR] [kernel] debug-self disk line") {
		t.Error("problems log must exist under debug_self even with debug_mode false")
	}
}

func TestDebugSelf_ApplyConfigImpliesDebugMode(t *testing.T) {
	resetAllLoggingState(t)
	defer resetAllLoggingState(t)

	ApplyConfig(Config{DebugMode: false, DebugSelf: true, Level: "debug"})
	if !IsDebugMode() || !IsDebugSelf() || !IsCategoryEnabled(CategoryKernel) {
		t.Fatal("ApplyConfig with DebugSelf must make debug mode effective")
	}
}
