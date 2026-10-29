package main

import (
	"bytes"
	"strings"
	"testing"

	"codenerd/internal/world"
)

// TestWorldRunbookCmd_WhenRun_ShouldPrintTheOwnedRunbook — the runbook must
// come from the world package, not a copy in the CLI, or the help text drifts
// from the scanner it describes.
func TestWorldRunbookCmd_WhenRun_ShouldPrintTheOwnedRunbook(t *testing.T) {
	var out bytes.Buffer
	worldRunbookCmd.SetOut(&out)
	worldRunbookCmd.SetArgs(nil)
	if err := worldRunbookCmd.RunE(worldRunbookCmd, nil); err != nil {
		t.Fatalf("runbook: %v", err)
	}
	if !strings.Contains(out.String(), world.ScanRunbook) {
		t.Error("runbook output is not the world package's ScanRunbook")
	}
	for _, want := range []string{"WHICH COMMAND TO USE", "OWNERSHIP", "WHEN THE WORLD LOOKS STALE"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("runbook is missing the %q section", want)
		}
	}
}

// TestWorldPredicatesCmd_WhenRun_ShouldListEveryOwnerGroup — an operator has to
// be able to see which writer owns a predicate to reason about what a scan
// deletes.
func TestWorldPredicatesCmd_WhenRun_ShouldListEveryOwnerGroup(t *testing.T) {
	var out bytes.Buffer
	worldPredicatesCmd.SetOut(&out)
	if err := worldPredicatesCmd.RunE(worldPredicatesCmd, nil); err != nil {
		t.Fatalf("predicates: %v", err)
	}
	got := out.String()
	for _, p := range world.WorldPredicates {
		if !strings.Contains(got, p) {
			t.Errorf("predicate %q missing from the listing", p)
		}
	}
}
