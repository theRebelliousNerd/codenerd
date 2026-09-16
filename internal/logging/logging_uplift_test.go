package logging

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Dedup must be timestamp-insensitive: every generated fact leads with a
// Unix-millisecond timestamp, so raw-string comparison made collapse depend on
// whether two identical writes straddled a millisecond boundary (flaky
// TestExportAuditFacts under load). These pins craft the log directly, so no
// wall-clock timing is involved.

func writeUpliftAuditLog(t *testing.T, facts ...string) string {
	t.Helper()
	var b strings.Builder
	for _, f := range facts {
		line, err := json.Marshal(map[string]string{"event": "shard_spawn", "mangle": f})
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		b.Write(line)
		b.WriteByte('\n')
	}
	path := filepath.Join(t.TempDir(), "run_audit.log")
	if err := os.WriteFile(path, []byte(b.String()), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	return path
}

func TestExportAuditFacts_TimestampOnlyDifference_Collapses(t *testing.T) {
	path := writeUpliftAuditLog(t,
		`shard_lifecycle(1789500000000, /shard_spawn, "shard-1", "coder", true).`,
		`shard_lifecycle(1789500000001, /shard_spawn, "shard-1", "coder", true).`,
	)
	var out bytes.Buffer
	stats, err := ExportAuditFacts(path, &out, nil)
	if err != nil {
		t.Fatalf("ExportAuditFacts: %v", err)
	}
	if stats.Events != 2 || stats.Facts != 1 || stats.Duplicates != 1 {
		t.Errorf("events=%d facts=%d dups=%d, want 2/1/1",
			stats.Events, stats.Facts, stats.Duplicates)
	}
	if n := strings.Count(out.String(), "shard_lifecycle("); n != 2 { // Decl + one fact
		t.Errorf("saw %d shard_lifecycle occurrences, want 2", n)
	}
	if !strings.Contains(out.String(), "1789500000000") {
		t.Error("emitted fact should keep the first-seen timestamp")
	}
}

func TestExportAuditFacts_NonTimestampDifference_DoesNotCollapse(t *testing.T) {
	path := writeUpliftAuditLog(t,
		`shard_lifecycle(1789500000000, /shard_spawn, "shard-1", "coder", true).`,
		`shard_lifecycle(1789500000000, /shard_spawn, "shard-2", "coder", true).`,
	)
	var out bytes.Buffer
	stats, err := ExportAuditFacts(path, &out, nil)
	if err != nil {
		t.Fatalf("ExportAuditFacts: %v", err)
	}
	if stats.Events != 2 || stats.Facts != 2 || stats.Duplicates != 0 {
		t.Errorf("events=%d facts=%d dups=%d, want 2/2/0",
			stats.Events, stats.Facts, stats.Duplicates)
	}
}

func TestDedupKey_Shapes(t *testing.T) {
	cases := []struct {
		name  string
		a, b  string
		equal bool
	}{
		{
			name:  "same fact different timestamp",
			a:     `safety_check(100, /safety_allow, "/edit", true).`,
			b:     `safety_check(200, /safety_allow, "/edit", true).`,
			equal: true,
		},
		{
			name:  "comma inside later string arg does not break split",
			a:     `error_event(100, /error_generic, "kernel", "failed: a, b").`,
			b:     `error_event(999, /error_generic, "kernel", "failed: a, b").`,
			equal: true,
		},
		{
			name:  "different payload never collapses",
			a:     `safety_check(100, /safety_allow, "/edit", true).`,
			b:     `safety_check(100, /safety_block, "/edit", false).`,
			equal: false,
		},
		{
			name:  "non-numeric first arg compares exactly",
			a:     `custom(/tag, "x").`,
			b:     `custom(/tag, "x").`,
			equal: true,
		},
		{
			name:  "non-numeric first arg differs",
			a:     `custom(/a, "x").`,
			b:     `custom(/b, "x").`,
			equal: false,
		},
		{
			name:  "malformed fact is its own identity",
			a:     `no_parens.`,
			b:     `no_parens.`,
			equal: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := dedupKey(tc.a) == dedupKey(tc.b)
			if got != tc.equal {
				t.Errorf("dedupKey(%q) vs dedupKey(%q): equal=%v, want %v",
					tc.a, tc.b, got, tc.equal)
			}
		})
	}
}
