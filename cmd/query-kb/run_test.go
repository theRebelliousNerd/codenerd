package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The full-content view (deep_query.go, once a separate `go run` script) is
// reached through the command's own arguments.
func TestRunDispatchesEveryMode(t *testing.T) {
	dir := t.TempDir()
	db := filepath.Join(dir, "kb.db")
	deterministicMakeKB(t, db, true, true)

	cases := []struct {
		args       []string
		code       int
		want, deny []string
	}{
		{[]string{db}, 0, []string{"Tables:", "Sample data:"}, []string{"(Full Content"}},
		{[]string{db, "--deep"}, 0, []string{"KNOWLEDGE ATOMS (Full Content)", "VECTORS (Full Content with Metadata)", "META: meta-alpha"}, nil},
		{[]string{db, "--atoms"}, 0, []string{"KNOWLEDGE ATOMS (Full Content)", "concept-alpha (90%)"}, []string{"VECTORS (Full"}},
		{[]string{db, "--vectors"}, 0, []string{"VECTORS (Full Content with Metadata)"}, []string{"KNOWLEDGE ATOMS (Full"}},
		{[]string{db, "--bogus"}, 2, []string{`unknown option "--bogus"`, "Usage: query-kb"}, nil},
		{[]string{db, "--deep", "extra"}, 2, []string{"Usage: query-kb"}, nil},
	}
	for _, tc := range cases {
		var out bytes.Buffer
		if code := run(tc.args, &out); code != tc.code {
			t.Fatalf("run %v = %d, want %d\n%s", tc.args, code, tc.code, out.String())
		}
		for _, s := range tc.want {
			if !strings.Contains(out.String(), s) {
				t.Fatalf("run %v output lacks %q:\n%s", tc.args, s, out.String())
			}
		}
		for _, s := range tc.deny {
			if strings.Contains(out.String(), s) {
				t.Fatalf("run %v output has %q:\n%s", tc.args, s, out.String())
			}
		}
	}
}

// With no argument the command samples every shard database, one whole
// report per database, and skips other files.
func TestSampleShardDBsReportsEachDatabaseWhole(t *testing.T) {
	dir := t.TempDir()
	deterministicMakeKB(t, filepath.Join(dir, "a.db"), true, false)
	deterministicMakeKB(t, filepath.Join(dir, "b.db"), true, false)
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if code := sampleShardDBs(dir, &out); code != 0 {
		t.Fatalf("code %d:\n%s", code, out.String())
	}
	got := out.String()
	if strings.Count(got, "=== a.db ===") != 1 || strings.Count(got, "=== b.db ===") != 1 || strings.Contains(got, "notes.txt") {
		t.Fatalf("reports:\n%s", got)
	}
	for _, header := range []string{"=== a.db ===", "=== b.db ==="} {
		report := got[strings.Index(got, header):]
		if next := strings.Index(report[len(header):], "\n=== "); next >= 0 {
			report = report[:len(header)+next]
		}
		if !strings.Contains(report, "Tables:") || !strings.Contains(report, "Sample data:") {
			t.Fatalf("report %s is not whole:\n%s", header, report)
		}
	}
	out.Reset()
	if code := sampleShardDBs(filepath.Join(dir, "missing"), &out); code != 1 || !strings.Contains(out.String(), "Error reading shards dir") {
		t.Fatalf("missing dir: code %d, %q", code, out.String())
	}
}
