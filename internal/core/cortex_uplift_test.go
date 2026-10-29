package core

import (
	"context"
	"errors"
	"strings"
	"testing"

	"codenerd/internal/types"
)

// TestCortexTransaction_NoCrossShardRollback pins the honest Commit contract:
// each shard commits atomically, but a failed cross-shard commit leaves the
// shards that already committed untouched (no rollback), and the error names
// the failed shard. Per-shard commit order is map order, so the test retries
// until the healthy shard demonstrably lands first (2^-10 flake odds).
func TestCortexTransaction_NoCrossShardRollback(t *testing.T) {
	cortex := NewCortexKernel("main")
	good := setupTestShard(t, "good", []string{"good_pred"})
	broken, err := NewKernelShard(KernelShardConfig{
		Domain:          "broken",
		OwnedPredicates: []string{"bad_pred"},
	})
	if err != nil {
		t.Fatalf("NewKernelShard: %v", err)
	}
	boom := errors.New("boom")
	broken.kernel.simulateCommitErr = boom
	if err := cortex.RegisterShard(good); err != nil {
		t.Fatalf("RegisterShard good: %v", err)
	}
	if err := cortex.RegisterShard(broken); err != nil {
		t.Fatalf("RegisterShard broken: %v", err)
	}

	landed := false
	for i := 0; i < 10 && !landed; i++ {
		tx := cortex.Transaction()
		tx.Assert(types.Fact{Predicate: "good_pred", Args: []any{"v"}})
		tx.Assert(types.Fact{Predicate: "bad_pred", Args: []any{"v"}})
		err := tx.Commit()
		if err == nil {
			t.Fatal("Commit with a broken shard = nil, want failure")
		}
		if !strings.Contains(err.Error(), "broken") {
			t.Fatalf("Commit error = %v, want it to name the failed shard", err)
		}
		facts, qerr := good.Query("good_pred")
		if qerr != nil {
			t.Fatalf("Query: %v", qerr)
		}
		landed = len(facts) == 1
	}
	if !landed {
		t.Fatal("healthy shard never landed across 10 failed commits (expected partial writes)")
	}
}

// TestDreamer_CampaignProjections pins campaign effect projection: campaign
// writers must project /modified like the file handlers they delegate to,
// and a campaign write under .git must simulate Unsafe end to end via the
// panic_state rules (proving the projection reaches evaluation, not just
// the fact list).
func TestDreamer_CampaignProjections(t *testing.T) {
	d, _ := setupTestDreamer(t)
	ctx := context.Background()

	hasAtom := func(facts []Fact, want MangleAtom) bool {
		for _, f := range facts {
			if f.Predicate != "projected_fact" || len(f.Args) < 2 {
				continue
			}
			if atom, ok := f.Args[1].(MangleAtom); ok && atom == want {
				return true
			}
		}
		return false
	}

	res := d.SimulateAction(ctx, ActionRequest{Type: ActionCampaignCreateFile, Target: "notes.txt"})
	if res.Unsafe {
		t.Fatalf("benign campaign write simulated unsafe: %s", res.Reason)
	}
	if !hasAtom(res.ProjectedFacts, "/modified") {
		t.Fatalf("campaign write lacks /modified projection: %+v", res.ProjectedFacts)
	}

	res = d.SimulateAction(ctx, ActionRequest{Type: ActionCampaignRunTest, Target: "go test ./..."})
	if !hasAtom(res.ProjectedFacts, "/exec_cmd") {
		t.Fatalf("campaign test run lacks /exec_cmd projection: %+v", res.ProjectedFacts)
	}

	res = d.SimulateAction(ctx, ActionRequest{Type: ActionCampaignCreateFile, Target: ".git/config"})
	if !res.Unsafe {
		t.Fatal("campaign write under .git simulated safe, want Unsafe")
	}
}

// TestDreamPlan_QuestionAndCounterPins covers two dream-plan fixes: question
// extraction must capture the sentence ending with "?" (not the text after
// it), and double-marking a subtask must not inflate the step counters.
func TestDreamPlan_QuestionAndCounterPins(t *testing.T) {
	plan, err := ExtractDreamPlan("migrate the store", []DreamConsultation{
		{ShardName: "s", ShardType: "coder", Perspective: "Should we migrate the store first? Yes, absolutely.\n1. Create the new schema file."},
	})
	if err != nil {
		t.Fatalf("ExtractDreamPlan: %v", err)
	}
	if len(plan.PendingQuestions) != 1 || !strings.HasSuffix(plan.PendingQuestions[0], "?") {
		t.Fatalf("PendingQuestions = %q, want the single trailing-? question", plan.PendingQuestions)
	}
	if !strings.Contains(plan.PendingQuestions[0], "migrate") {
		t.Fatalf("question lost its subject: %q", plan.PendingQuestions[0])
	}

	p := NewDreamPlan("p1", "h")
	p.AddSubtask(DreamSubtask{ID: "s1", Status: SubtaskStatusPending})
	p.MarkSubtaskCompleted("s1", "ok")
	p.MarkSubtaskCompleted("s1", "ok-again")
	if p.CompletedSteps != 1 {
		t.Fatalf("CompletedSteps = %d after double-mark, want 1", p.CompletedSteps)
	}
	p.AddSubtask(DreamSubtask{ID: "s2", Status: SubtaskStatusPending})
	p.MarkSubtaskFailed("s2", "x")
	p.MarkSubtaskFailed("s2", "x-again")
	if p.FailedSteps != 1 {
		t.Fatalf("FailedSteps = %d after double-mark, want 1", p.FailedSteps)
	}
}

// TestToolRegistry_CatalogDeterministic pins stable catalog bytes: groups
// sort by affinity and tools by name, so repeated builds are identical and
// prompt caches hold.
func TestToolRegistry_CatalogDeterministic(t *testing.T) {
	reg := NewToolRegistry(t.TempDir())
	for _, tool := range []*Tool{
		{Name: "zebra", ShardAffinity: "/all"},
		{Name: "mid", ShardAffinity: "/coder"},
		{Name: "alpha", ShardAffinity: "/coder"},
	} {
		if err := reg.RegisterToolWithInfo(tool); err != nil {
			t.Fatalf("register %s: %v", tool.Name, err)
		}
	}
	// A /coder query returns /coder tools plus /all tools: two groups.
	first := reg.BuildToolCatalog("/coder")
	for i := 0; i < 10; i++ {
		if got := reg.BuildToolCatalog("/coder"); got != first {
			t.Fatalf("catalog build %d differs:\n%s\n---\n%s", i, got, first)
		}
	}
	ia, im, iz := strings.Index(first, "alpha"), strings.Index(first, "mid"), strings.Index(first, "zebra")
	if ia < 0 || im < 0 || iz < 0 {
		t.Fatalf("catalog missing tools:\n%s", first)
	}
	if !(iz < ia && ia < im) {
		t.Fatalf("catalog not sorted by affinity then name:\n%s", first)
	}
}
