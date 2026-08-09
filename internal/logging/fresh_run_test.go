package logging

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestGenerateRunPrefix_DistinctAndSortable(t *testing.T) {
	p1 := generateRunPrefix()
	p2 := generateRunPrefix()
	if p1 == "" || p2 == "" {
		t.Fatalf("generateRunPrefix returned empty: p1=%q p2=%q", p1, p2)
	}
	if p1 == p2 {
		t.Fatalf("generateRunPrefix returned identical values: %q", p1)
	}
	if p1 >= p2 {
		t.Fatalf("expected lexically sortable distinct values p1 < p2, got p1=%q p2=%q", p1, p2)
	}
}

func TestClearOrdinaryLogs_RemovesAndPreserves(t *testing.T) {
	tmp := t.TempDir()
	logsDir := filepath.Join(tmp, ".nerd", "logs")
	if err := os.MkdirAll(logsDir, 0o700); err != nil {
		t.Fatalf("MkdirAll logsDir: %v", err)
	}

	topFiles := map[string]string{
		"a.log": "stale A",
		"B.LOG": "stale B",
		"c.LoG": "stale C",
	}
	for name, content := range topFiles {
		if err := os.WriteFile(filepath.Join(logsDir, name), []byte(content), 0o600); err != nil {
			t.Fatalf("write top file %s: %v", name, err)
		}
	}

	nonLogPath := filepath.Join(logsDir, "notes.txt")
	nonLogContent := "keep me"
	if err := os.WriteFile(nonLogPath, []byte(nonLogContent), 0o600); err != nil {
		t.Fatalf("write non-log file: %v", err)
	}

	nestedDir := filepath.Join(logsDir, "archive")
	if err := os.MkdirAll(nestedDir, 0o700); err != nil {
		t.Fatalf("MkdirAll nested: %v", err)
	}
	nestedPath := filepath.Join(nestedDir, "nested.log")
	nestedContent := "nested keep"
	if err := os.WriteFile(nestedPath, []byte(nestedContent), 0o600); err != nil {
		t.Fatalf("write nested log: %v", err)
	}

	// Leaf symlink: symlink inside logsDir pointing outside.
	targetPath := filepath.Join(tmp, "target.log")
	targetContent := "target content"
	if err := os.WriteFile(targetPath, []byte(targetContent), 0o600); err != nil {
		t.Fatalf("write symlink target: %v", err)
	}
	linkPath := filepath.Join(logsDir, "link.log")
	symlinkCreated := true
	if err := os.Symlink(targetPath, linkPath); err != nil {
		if runtime.GOOS == "windows" {
			symlinkCreated = false
		} else {
			t.Fatalf("Symlink creation failed: %v", err)
		}
	}

	clearOrdinaryLogs(logsDir)

	// Assert top-level .log files no longer exist (removed).
	for name := range topFiles {
		p := filepath.Join(logsDir, name)
		_, err := os.Stat(p)
		if err == nil {
			t.Fatalf("expected %s to be removed, but it still exists", name)
		}
		if !os.IsNotExist(err) {
			t.Fatalf("Stat %s: %v", name, err)
		}
	}

	// Non-log file preserved exactly.
	data, err := os.ReadFile(nonLogPath)
	if err != nil {
		t.Fatalf("ReadFile notes.txt: %v", err)
	}
	if string(data) != nonLogContent {
		t.Fatalf("notes.txt content = %q, want %q", string(data), nonLogContent)
	}

	// Nested .log preserved exactly.
	data, err = os.ReadFile(nestedPath)
	if err != nil {
		t.Fatalf("ReadFile nested.log: %v", err)
	}
	if string(data) != nestedContent {
		t.Fatalf("nested.log content = %q, want %q", string(data), nestedContent)
	}

	// Leaf symlink preserved without touching target (skip only this assertion on Windows if denied).
	if symlinkCreated {
		fi, err := os.Lstat(linkPath)
		if err != nil {
			t.Fatalf("Lstat link.log: %v", err)
		}
		if fi.Mode()&os.ModeSymlink == 0 {
			t.Fatalf("expected link.log to remain a symlink")
		}
		data, err = os.ReadFile(targetPath)
		if err != nil {
			t.Fatalf("ReadFile target: %v", err)
		}
		if string(data) != targetContent {
			t.Fatalf("symlink target content = %q, want %q", string(data), targetContent)
		}
		// Also ensure link target data via symlink path is unchanged (read through link).
		data, err = os.ReadFile(linkPath)
		if err != nil {
			t.Fatalf("ReadFile through symlink: %v", err)
		}
		if string(data) != targetContent {
			t.Fatalf("content via symlink = %q, want %q", string(data), targetContent)
		}
	}
}

func TestClearOrdinaryLogs_RefusesSymlinkedLogsDir(t *testing.T) {
	tmp := t.TempDir()
	realLogs := filepath.Join(tmp, "real_logs")
	if err := os.MkdirAll(realLogs, 0o700); err != nil {
		t.Fatalf("MkdirAll realLogs: %v", err)
	}
	staleContent := "should remain"
	stalePath := filepath.Join(realLogs, "stale.log")
	if err := os.WriteFile(stalePath, []byte(staleContent), 0o600); err != nil {
		t.Fatalf("write stale.log: %v", err)
	}
	nerdDir := filepath.Join(tmp, ".nerd")
	if err := os.MkdirAll(nerdDir, 0o700); err != nil {
		t.Fatalf("MkdirAll .nerd: %v", err)
	}
	logsDir := filepath.Join(nerdDir, "logs")
	if err := os.Symlink(realLogs, logsDir); err != nil {
		t.Skipf("symlink creation denied (skipping): %v", err)
	}

	clearOrdinaryLogs(logsDir)

	data, err := os.ReadFile(stalePath)
	if err != nil {
		t.Fatalf("ReadFile stale.log: %v", err)
	}
	if string(data) != staleContent {
		t.Fatalf("symlinked logs dir: stale.log content = %q, want %q (should be preserved)", string(data), staleContent)
	}
	if len(data) == 0 {
		t.Fatalf("symlinked logs dir: stale.log was truncated but should be preserved")
	}
}

func TestClearOrdinaryLogs_RefusesSymlinkedNerdParent(t *testing.T) {
	tmp := t.TempDir()
	realNerd := filepath.Join(tmp, "real_nerd")
	realLogs := filepath.Join(realNerd, "logs")
	if err := os.MkdirAll(realLogs, 0o700); err != nil {
		t.Fatalf("MkdirAll realLogs: %v", err)
	}
	staleContent := "should remain parent"
	stalePath := filepath.Join(realLogs, "stale.log")
	if err := os.WriteFile(stalePath, []byte(staleContent), 0o600); err != nil {
		t.Fatalf("write stale.log: %v", err)
	}
	nerdLink := filepath.Join(tmp, ".nerd")
	if err := os.Symlink(realNerd, nerdLink); err != nil {
		t.Skipf("symlink creation denied (skipping): %v", err)
	}
	logsDir := filepath.Join(nerdLink, "logs")

	clearOrdinaryLogs(logsDir)

	data, err := os.ReadFile(stalePath)
	if err != nil {
		t.Fatalf("ReadFile stale.log: %v", err)
	}
	if string(data) != staleContent {
		t.Fatalf("symlinked .nerd parent: stale.log content = %q, want %q (should be preserved)", string(data), staleContent)
	}
	if len(data) == 0 {
		t.Fatalf("symlinked .nerd parent: stale.log was truncated but should be preserved")
	}
}

func TestInitialize_DebugFalse_ClearsStaleLogAndSetsPrefix(t *testing.T) {
	tmp := t.TempDir()
	nerdDir := filepath.Join(tmp, ".nerd")
	logsDir := filepath.Join(nerdDir, "logs")
	if err := os.MkdirAll(logsDir, 0o700); err != nil {
		t.Fatalf("MkdirAll logsDir: %v", err)
	}
	stalePath := filepath.Join(logsDir, "stale.log")
	staleContent := "old content"
	if err := os.WriteFile(stalePath, []byte(staleContent), 0o600); err != nil {
		t.Fatalf("write stale.log: %v", err)
	}
	configPath := filepath.Join(nerdDir, "config.json")
	configContent := `{"logging":{"debug_mode":false}}`
	if err := os.WriteFile(configPath, []byte(configContent), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	resetLoggingState(t)
	defer resetLoggingState(t)

	if err := Initialize(tmp); err != nil {
		t.Fatalf("Initialize: %v", err)
	}

	if got := currentRunPrefix(); got == "" {
		t.Fatalf("expected nonempty run prefix after Initialize, got %q", got)
	}

	if _, err := os.Stat(stalePath); !os.IsNotExist(err) {
		if err == nil {
			t.Fatalf("expected stale.log to be removed after Initialize with debug_mode false, but it still exists")
		}
		t.Fatalf("Stat stale.log after Initialize: %v", err)
	}
}
