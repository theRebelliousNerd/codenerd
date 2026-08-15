package logging

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// resetAllLoggingState clears every package global these tests touch,
// including the ones resetLoggingState (logging_comprehensive_test.go) predates:
// the workspace binding, the injected-config pin, and the LLM I/O sync.Once.
func resetAllLoggingState(t *testing.T) {
	t.Helper()
	closeAllSinks()
	resetLLMIOLogger()

	loggersMu.Lock()
	loggers = make(map[Category]*Logger)
	loggersMu.Unlock()

	configMu.Lock()
	config = loggingConfig{}
	configLoaded = false
	configInjected = false
	logLevel = LevelInfo
	configMu.Unlock()

	initMu.Lock()
	logsDir = ""
	workspace = ""
	boundWorkspace = ""
	initOnce = sync.Once{}
	initErr = nil
	initialized = false
	initMu.Unlock()

	auditLogger = nil
	runPrefixMu.Lock()
	runPrefix = ""
	runPrefixMu.Unlock()
}

// newWorkspace creates a temp workspace whose .nerd/config.json holds the given
// logging block (the JSON object body, without the outer braces).
func newWorkspace(t *testing.T, loggingBlock string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".nerd"), 0o755); err != nil {
		t.Fatalf("mkdir .nerd: %v", err)
	}
	content := "{\n\"logging\": {" + loggingBlock + "}\n}"
	if err := os.WriteFile(filepath.Join(dir, ".nerd", "config.json"), []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return dir
}

// readLog returns the contents of the newest <run>_<suffix>.log in the
// workspace's logs directory.
func readLog(t *testing.T, ws, suffix string) string {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(ws, ".nerd", "logs", "*_"+suffix+".log"))
	if err != nil || len(matches) == 0 {
		t.Fatalf("no %s log in %s (err=%v)", suffix, ws, err)
	}
	data, err := os.ReadFile(matches[len(matches)-1])
	if err != nil {
		t.Fatalf("read %s: %v", matches[len(matches)-1], err)
	}
	return string(data)
}

// globLogs lists log files matching a suffix in the workspace logs directory.
func globLogs(t *testing.T, ws, suffix string) []string {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(ws, ".nerd", "logs", "*_"+suffix+".log"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	return matches
}
