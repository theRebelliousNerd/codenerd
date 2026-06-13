package context_harness

import (
	"testing"

	"codenerd/internal/core"
)

func TestFormatNumber(t *testing.T) {
	cases := map[int]string{
		0:       "0",
		42:      "42",
		999:     "999",
		1000:    "1,000",
		12345:   "12,345",
		1000000: "1,000,000",
		2500300: "2,500,300",
	}
	for in, want := range cases {
		if got := formatNumber(in); got != want {
			t.Errorf("formatNumber(%d)=%q, want %q", in, got, want)
		}
	}
}

func TestTruncate(t *testing.T) {
	if got := truncate("short", 10); got != "short" {
		t.Errorf("truncate kept-as-is got %q", got)
	}
	if got := truncate("abcdefgh", 3); got != "abc..." {
		t.Errorf("truncate(abcdefgh,3)=%q, want abc...", got)
	}
	if got := truncate("exact", 5); got != "exact" {
		t.Errorf("truncate at exact length should not append ellipsis, got %q", got)
	}
}

func TestSumTokensAndGroupByCategory(t *testing.T) {
	atoms := []CompiledAtom{
		{ID: "a", Category: "core", Tokens: 10},
		{ID: "b", Category: "core", Tokens: 5},
		{ID: "c", Category: "history", Tokens: 7},
	}
	if got := sumTokens(atoms); got != 22 {
		t.Errorf("sumTokens=%d, want 22", got)
	}
	groups := groupByCategory(atoms)
	if len(groups["core"]) != 2 || len(groups["history"]) != 1 {
		t.Errorf("groupByCategory mismatch: core=%d history=%d", len(groups["core"]), len(groups["history"]))
	}
}

func TestPercent(t *testing.T) {
	if got := percent(0, 0); got != 0 {
		t.Errorf("percent(0,0)=%v, want 0 (no division by zero)", got)
	}
	if got := percent(1, 4); got != 25 {
		t.Errorf("percent(1,4)=%v, want 25", got)
	}
}

// TestFactSeeder_CampaignContext drives the seeder against a real Mangle kernel
// and verifies the campaign facts are queryable afterwards — a cross-boundary
// check of the harness's seeding path.
func TestFactSeeder_CampaignContext(t *testing.T) {
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	kernel.SetSchemas("Decl current_campaign(ID).\nDecl campaign_phase(ID, Phase, Num).\nDecl phase_objective(Phase, Goal).")
	kernel.SetPolicy("")

	fs := NewFactSeeder(kernel)
	if err := fs.SeedCampaignContext("camp-1", "/design", 1, []string{"draft schema", "review"}); err != nil {
		t.Fatalf("SeedCampaignContext: %v", err)
	}

	camps, err := kernel.Query("current_campaign")
	if err != nil {
		t.Fatalf("Query current_campaign: %v", err)
	}
	if len(camps) != 1 {
		t.Errorf("expected 1 current_campaign fact, got %d", len(camps))
	}
	objs, err := kernel.Query("phase_objective")
	if err != nil {
		t.Fatalf("Query phase_objective: %v", err)
	}
	if len(objs) != 2 {
		t.Errorf("expected 2 phase_objective facts (one per goal), got %d", len(objs))
	}

	// Clear is a documented no-op for fresh-kernel isolation; it must not error.
	if err := fs.Clear(); err != nil {
		t.Errorf("Clear: %v", err)
	}
}
