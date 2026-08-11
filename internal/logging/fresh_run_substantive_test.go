package logging

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// TestClearOrdinaryLogs_SubstantiveRetention verifies the regression described
// in the task: a substantive run (audit log containing safety_block / shard_ /
// action_ / tool_) must survive a burst of diagnostic runs (perf_metric only)
// that would previously have evicted it via the ordinary 10-run window.
// It also verifies growth remains bounded: the substantive budget itself is
// finite and the oldest substantive run is evicted once it is exceeded.
func TestClearOrdinaryLogs_SubstantiveRetention(t *testing.T) {
	if DefaultLogRetentionRuns != 10 {
		t.Fatalf("DefaultLogRetentionRuns = %d, want 10", DefaultLogRetentionRuns)
	}
	if DefaultSubstantiveRetentionRuns != 10 {
		t.Fatalf("DefaultSubstantiveRetentionRuns = %d, want 10", DefaultSubstantiveRetentionRuns)
	}

	syntheticPrefix := func(date string, i int) string {
		// Must match generateRunPrefix format (46 chars) and be lexically sortable.
		// date is YYYYMMDD, e.g. "20250101". The remainder is fixed except for nanos/i.
		return fmt.Sprintf("%s_000000.%09d_000001_000001_%06x", date, i, i)
	}

	t.Run("DiagnosticBurstDoesNotEvictSubstantive", func(t *testing.T) {
		tmp := t.TempDir()
		logsDir := filepath.Join(tmp, ".nerd", "logs")
		if err := os.MkdirAll(logsDir, 0o700); err != nil {
			t.Fatalf("MkdirAll logsDir: %v", err)
		}

		// One substantive run at i=0 (oldest). Its audit log contains a safety_block
		// marker, which is one of the four substantive substrings.
		substantivePrefix := syntheticPrefix("20250101", 0)
		if got := runPrefixFromLogName(substantivePrefix + "_audit.log"); got != substantivePrefix {
			t.Fatalf("synthetic substantive prefix %q not recognised: got %q", substantivePrefix, got)
		}
		// Create two files for the substantive prefix to verify grouping.
		substantiveAudit := filepath.Join(logsDir, substantivePrefix+"_audit.log")
		substantiveBoot := filepath.Join(logsDir, substantivePrefix+"_boot.log")
		// Include safety_block and also a kernel_query to ensure mixed content still qualifies.
		auditContent := "{\"event\":\"safety_block\",\"msg\":\"blocked 9 calls\"}\n{\"event\":\"kernel_query\",\"q\":\"test\"}\n"
		if err := os.WriteFile(substantiveAudit, []byte(auditContent), 0o600); err != nil {
			t.Fatalf("write substantive audit: %v", err)
		}
		if err := os.WriteFile(substantiveBoot, []byte("substantive boot data"), 0o600); err != nil {
			t.Fatalf("write substantive boot: %v", err)
		}

		// Create 12 trivial diagnostic runs (i=1..12), each audit log holds only perf_metric/kernel_query.
		// Prune after each addition, mimicking Initialize's fresh-run cleanup.
		for i := 1; i <= 12; i++ {
			p := syntheticPrefix("20250101", i)
			auditPath := filepath.Join(logsDir, p+"_audit.log")
			bootPath := filepath.Join(logsDir, p+"_boot.log")
			diagContent := "{\"event\":\"perf_metric\",\"duration\":123}\n{\"event\":\"kernel_query\",\"q\":\"diag\"}\n"
			if err := os.WriteFile(auditPath, []byte(diagContent), 0o600); err != nil {
				t.Fatalf("write diag audit i=%d: %v", i, err)
			}
			if err := os.WriteFile(bootPath, []byte("diag boot"), 0o600); err != nil {
				t.Fatalf("write diag boot i=%d: %v", i, err)
			}
			clearOrdinaryLogs(logsDir)
		}

		// Regression assertion: substantive run must still exist.
		for _, path := range []string{substantiveAudit, substantiveBoot} {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("expected substantive file %s to survive diagnostic burst (regression): %v", filepath.Base(path), err)
			}
			if len(data) == 0 {
				t.Fatalf("substantive file %s was truncated but should be preserved", filepath.Base(path))
			}
		}
		// Oldest diagnostic prefixes (i=1,2) are outside the ordinary 10-window and not substantive, so they must be gone.
		for _, i := range []int{1, 2} {
			p := syntheticPrefix("20250101", i)
			auditPath := filepath.Join(logsDir, p+"_audit.log")
			if _, err := os.Stat(auditPath); !os.IsNotExist(err) {
				if err == nil {
					t.Fatalf("expected diagnostic prefix %q (i=%d) to be evicted, but it still exists", p, i)
				}
				t.Fatalf("Stat %s: %v", auditPath, err)
			}
			bootPath := filepath.Join(logsDir, p+"_boot.log")
			if _, err := os.Stat(bootPath); !os.IsNotExist(err) {
				if err == nil {
					t.Fatalf("expected diagnostic prefix %q boot log to be evicted, but it still exists", p)
				}
				t.Fatalf("Stat %s: %v", bootPath, err)
			}
		}
		// Newest 10 diagnostic runs (i=3..12) must survive as they are within the ordinary window.
		for i := 3; i <= 12; i++ {
			p := syntheticPrefix("20250101", i)
			auditPath := filepath.Join(logsDir, p+"_audit.log")
			if _, err := os.Stat(auditPath); err != nil {
				t.Fatalf("expected newest diagnostic prefix %q (i=%d) to survive: %v", p, i, err)
			}
		}
		// Verify distinct prefix count: 10 ordinary recent + 1 substantive extra = 11.
		entries, err := os.ReadDir(logsDir)
		if err != nil {
			t.Fatalf("ReadDir logsDir: %v", err)
		}
		remaining := make(map[string]struct{})
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			if !strings.HasSuffix(strings.ToLower(name), ".log") {
				continue
			}
			p := runPrefixFromLogName(name)
			if p == "" {
				t.Fatalf("unexpected unprefixed .log remaining: %q", name)
			}
			remaining[p] = struct{}{}
		}
		if len(remaining) != 11 {
			// Provide sorted list for debugging.
			keys := make([]string, 0, len(remaining))
			for k := range remaining {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			t.Fatalf("remaining distinct prefixes = %d, want 11 (10 recent + 1 substantive); got %v", len(remaining), keys)
		}
		if _, ok := remaining[substantivePrefix]; !ok {
			t.Fatalf("substantive prefix %q not in remaining set %v", substantivePrefix, remaining)
		}
	})

	t.Run("GrowthStillBoundedForSubstantiveRuns", func(t *testing.T) {
		tmp := t.TempDir()
		logsDir := filepath.Join(tmp, ".nerd", "logs")
		if err := os.MkdirAll(logsDir, 0o700); err != nil {
			t.Fatalf("MkdirAll logsDir: %v", err)
		}
		total := DefaultSubstantiveRetentionRuns + 1 // 11, one more than budget
		prefixes := make([]string, total)
		for i := 0; i < total; i++ {
			p := syntheticPrefix("20250201", i)
			prefixes[i] = p
			auditPath := filepath.Join(logsDir, p+"_audit.log")
			bootPath := filepath.Join(logsDir, p+"_boot.log")
			// Rotate through the four substantive markers to ensure each is recognised.
			var marker string
			switch i % 4 {
			case 0:
				marker = `"event":"shard_test"`
			case 1:
				marker = `"event":"action_test"`
			case 2:
				marker = `"event":"tool_call"`
			case 3:
				marker = `"event":"safety_block"`
			}
			// Build a minimal JSON line containing the marker substring.
			content := fmt.Sprintf("{%s,\"i\":%d}\n", marker, i)
			if err := os.WriteFile(auditPath, []byte(content), 0o600); err != nil {
				t.Fatalf("write substantive audit i=%d: %v", i, err)
			}
			if err := os.WriteFile(bootPath, []byte("boot"), 0o600); err != nil {
				t.Fatalf("write boot i=%d: %v", i, err)
			}
			clearOrdinaryLogs(logsDir)
		}
		oldest := prefixes[0]
		if _, err := os.Stat(filepath.Join(logsDir, oldest+"_audit.log")); !os.IsNotExist(err) {
			if err == nil {
				t.Fatalf("expected oldest substantive prefix %q to be evicted when exceeding substantive budget (%d), but it still exists", oldest, DefaultSubstantiveRetentionRuns)
			}
			t.Fatalf("Stat oldest audit: %v", err)
		}
		if _, err := os.Stat(filepath.Join(logsDir, oldest+"_boot.log")); !os.IsNotExist(err) {
			if err == nil {
				t.Fatalf("expected oldest substantive prefix %q boot log to be evicted", oldest)
			}
			t.Fatalf("Stat oldest boot: %v", err)
		}
		// The newest 10 substantive prefixes must survive.
		for _, p := range prefixes[1:] {
			if _, err := os.Stat(filepath.Join(logsDir, p+"_audit.log")); err != nil {
				t.Fatalf("expected substantive prefix %q to survive (within budget): %v", p, err)
			}
		}
		// Verify exactly 10 distinct prefixes remain.
		entries, err := os.ReadDir(logsDir)
		if err != nil {
			t.Fatalf("ReadDir logsDir: %v", err)
		}
		remaining := make(map[string]struct{})
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			if !strings.HasSuffix(strings.ToLower(name), ".log") {
				continue
			}
			p := runPrefixFromLogName(name)
			if p == "" {
				t.Fatalf("unexpected unprefixed .log remaining: %q", name)
			}
			remaining[p] = struct{}{}
		}
		if len(remaining) != DefaultSubstantiveRetentionRuns {
			keys := make([]string, 0, len(remaining))
			for k := range remaining {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			t.Fatalf("remaining distinct prefixes = %d, want %d (substantive budget); got %v", len(remaining), DefaultSubstantiveRetentionRuns, keys)
		}
	})
}
