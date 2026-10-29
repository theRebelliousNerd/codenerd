package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestRetrieveCmd_ShouldReportTiersAndFacts exercises the command end to end.
// The command is the operator-facing proof that retrieval reaches the kernel, so
// it has to be run, not just compiled.
func TestRetrieveCmd_ShouldReportTiersAndFacts(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "go.mod"), "module retrievetest\n\ngo 1.26\n")
	mustWrite(t, filepath.Join(dir, "internal", "alpha", "alpha.go"),
		"package alpha\n\n// WidgetError is returned by build_widget.\ntype WidgetError struct{}\n\nfunc build_widget() error { return nil }\n")

	t.Cleanup(resetRetrieveFlags)
	retrieveWorkspace = dir
	retrieveTimeout = 30 * time.Second
	retrieveMaxFiles = 20
	retrieveShowFacts = true
	retrieveStats = true

	var out bytes.Buffer
	retrieveCmd.SetOut(&out)
	retrieveCmd.SetErr(&out)
	retrieveCmd.SetContext(context.Background())

	if err := runRetrieve(retrieveCmd, []string{
		"WidgetError raised from internal/alpha/alpha.go in build_widget()",
	}); err != nil {
		t.Fatalf("runRetrieve: %v", err)
	}

	got := out.String()
	for _, want := range []string{
		"tiers:", "candidates:", "facts:",
		"tier1", "internal/alpha/alpha.go",
		"issue_text(", "issue_keyword(", "tiered_context_file(",
		"metrics:", "cache_hit_rate",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q\n---\n%s", want, got)
		}
	}
}

func resetRetrieveFlags() {
	retrieveWorkspace = "."
	retrieveTimeout = 0
	retrieveMaxFiles = 50
	retrieveShowFacts = false
	retrieveStats = false
	retrieveRipgrep = false
	retrieveCmd.SetOut(nil)
	retrieveCmd.SetErr(nil)
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
