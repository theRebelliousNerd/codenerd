package logging

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
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


func TestClearOrdinaryLogs_RetentionWindow_KeepsNewest10(t *testing.T) {
	if DefaultLogRetentionRuns != 10 {
		t.Fatalf("DefaultLogRetentionRuns = %d, want 10", DefaultLogRetentionRuns)
	}
	tmp := t.TempDir()
	logsDir := filepath.Join(tmp, ".nerd", "logs")
	if err := os.MkdirAll(logsDir, 0o700); err != nil {
		t.Fatalf("MkdirAll logsDir: %v", err)
	}

	// Create 13 distinct synthetic run prefixes. Format matches
	// generateRunPrefix: 20060102_150405.000000000_<pid>_<seq>_<rand> (46 chars)
	// Lexical order equals chronological order, so increasing nanos = newer.
	const total = 13
	prefixes := make([]string, total)
	for i := 0; i < total; i++ {
		// Use deterministic sortable prefixes: timestamp nanos increments with i,
		// pid/seq fixed, rand derived from i to keep distinct but still ordered by nanos.
		prefixes[i] = fmt.Sprintf("20250101_000000.%09d_000001_000001_%06x", i, i)
		if got := runPrefixFromLogName(prefixes[i] + "_boot.log"); got != prefixes[i] {
			t.Fatalf("synthetic prefix %q not recognised by runPrefixFromLogName: got %q", prefixes[i], got)
		}
	}
	// Create two log files per prefix to verify grouping keeps all files for a prefix.
	for _, p := range prefixes {
		for _, suffix := range []string{"_boot.log", "_audit.log"} {
			path := filepath.Join(logsDir, p+suffix)
			if err := os.WriteFile(path, []byte("data "+p), 0o600); err != nil {
				t.Fatalf("write %s: %v", path, err)
			}
		}
	}
	// Also create a nested dir and a non-log file to ensure they are preserved
	// even when retention trimming occurs.
	nestedDir := filepath.Join(logsDir, "nested_keep")
	if err := os.MkdirAll(nestedDir, 0o700); err != nil {
		t.Fatalf("MkdirAll nested: %v", err)
	}
	nestedLog := filepath.Join(nestedDir, "old.log")
	if err := os.WriteFile(nestedLog, []byte("keep"), 0o600); err != nil {
		t.Fatalf("write nested log: %v", err)
	}
	notesPath := filepath.Join(logsDir, "notes.txt")
	if err := os.WriteFile(notesPath, []byte("keep notes"), 0o600); err != nil {
		t.Fatalf("write notes.txt: %v", err)
	}

	clearOrdinaryLogs(logsDir)

	// Determine expected survivors: newest 10 distinct prefixes (lexically largest).
	// Since prefixes were created in ascending order (0 oldest, 12 newest),
	// oldest 3 are prefixes[0], [1], [2]; newest 10 are [3]..[12].
	sorted := append([]string(nil), prefixes...)
	sort.Strings(sorted) // ascending = oldest first
	oldest := sorted[:3]
	newest := sorted[3:]

	// Oldest 3 prefixes' files must be gone.
	for _, p := range oldest {
		for _, suffix := range []string{"_boot.log", "_audit.log"} {
			path := filepath.Join(logsDir, p+suffix)
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				if err == nil {
					t.Fatalf("expected oldest prefix %q file %s to be deleted, but it still exists", p, suffix)
				}
				t.Fatalf("Stat %s: %v", path, err)
			}
		}
	}
	// Newest 10 prefixes' files must survive intact.
	for _, p := range newest {
		for _, suffix := range []string{"_boot.log", "_audit.log"} {
			path := filepath.Join(logsDir, p+suffix)
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("expected newest prefix %q file %s to survive, but ReadFile failed: %v", p, suffix, err)
			}
			want := "data " + p
			if string(data) != want {
				t.Fatalf("newest prefix %q file %s content = %q, want %q", p, suffix, string(data), want)
			}
		}
	}
	// Verify exactly 10 distinct prefixes remain (20 files).
	entries, err := os.ReadDir(logsDir)
	if err != nil {
		t.Fatalf("ReadDir logsDir: %v", err)
	}
	remainingPrefixes := make(map[string]struct{})
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !isLogFile(name) {
			continue
		}
		p := runPrefixFromLogName(name)
		if p == "" {
			t.Fatalf("unexpected unprefixed .log file remaining: %q", name)
		}
		remainingPrefixes[p] = struct{}{}
	}
	if len(remainingPrefixes) != 10 {
		t.Fatalf("remaining distinct prefixes = %d, want 10; got %v", len(remainingPrefixes), remainingPrefixes)
	}
	// Check nested and non-log preserved.
	if data, err := os.ReadFile(nestedLog); err != nil || string(data) != "keep" {
		t.Fatalf("nested log should be preserved: err=%v data=%q", err, string(data))
	}
	if data, err := os.ReadFile(notesPath); err != nil || string(data) != "keep notes" {
		t.Fatalf("notes.txt should be preserved: err=%v data=%q", err, string(data))
	}
}

// isLogFile reports whether name ends with .log case-insensitive (helper for test).
func isLogFile(name string) bool {
	return strings.HasSuffix(strings.ToLower(name), ".log")
}