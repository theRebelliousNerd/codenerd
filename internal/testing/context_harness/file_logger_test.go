package context_harness

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileLoggerCreatesLogsAndManifest(t *testing.T) {
	baseDir := t.TempDir()
	var console bytes.Buffer

	logger, err := NewFileLogger(baseDir, &console)
	if err != nil {
		t.Fatalf("NewFileLogger failed: %v", err)
	}

	sessionDir := logger.GetSessionDir()
	if sessionDir == "" {
		t.Fatalf("expected session dir to be set")
	}
	if _, err := os.Stat(sessionDir); err != nil {
		t.Fatalf("session dir missing: %v", err)
	}

	if _, err := logger.GetPromptWriter().Write([]byte("hello\n")); err != nil {
		t.Fatalf("write prompt log failed: %v", err)
	}

	if err := logger.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	logFiles := []string{
		"prompts.log",
		"jit-compilation.log",
		"spreading-activation.log",
		"compression.log",
		"piggyback-protocol.log",
		"summary.log",
	}
	for _, name := range logFiles {
		path := filepath.Join(sessionDir, name)
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected log file %s: %v", name, err)
		}
	}

	manifestPath := filepath.Join(sessionDir, "MANIFEST.txt")
	if _, err := os.Stat(manifestPath); err != nil {
		t.Fatalf("manifest missing: %v", err)
	}

	promptLog := filepath.Join(sessionDir, "prompts.log")
	data, err := os.ReadFile(promptLog)
	if err != nil {
		t.Fatalf("read prompts.log: %v", err)
	}
	if !strings.Contains(string(data), "Context Harness - Test Session") {
		t.Fatalf("expected header in prompts.log")
	}
}
