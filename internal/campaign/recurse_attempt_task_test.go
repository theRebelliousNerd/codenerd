package campaign

import (
	"context"
	"strings"
	"testing"

	"codenerd/internal/gates"
	"codenerd/internal/session"
	"codenerd/internal/tools"
	"codenerd/internal/types"
)

// The task string keeps every fact the attempt measured. How to approach the
// work moved to the recurse attempt atoms; those sentences must not come back
// here, or the model is instructed twice and the atoms are a second copy.
func TestRecurseAttemptTask_CarriesTheFindingAndNotTheInstructions(t *testing.T) {
	// The head token sits before a full limit of filler, so the tail-keep
	// drops it and the task still carries the failure the runner printed last.
	const headToken = "DROPPEDHEADTOKEN"
	evidence := headToken + strings.Repeat("q", recurseEvidenceLimit) + "TAIL the assertion failed at store.Get"
	a := RecurseAttempt{
		Pass:  2,
		Cycle: 4,
		Node:  SubsystemNode{ID: "store", Title: "the store", Paths: []string{"store", "store/get.go"}},
		Finding: gates.Finding{
			Kind:    gates.Test,
			Gate:    "go:test",
			Message: "Get() != 2",
			Target:  "store::TestGet",
		},
		Evidence: evidence,
		Check:    []string{"go", "test", "-count=1", "./store"},
		Prior: []PriorAttempt{
			{Cycle: 1, Why: "finding still open", Tried: "return 1"},
			{Cycle: 3, Why: "coverage dropped", Tried: "return 3"},
		},
	}
	task := recurseAttemptTask(a, "store, store/get.go")
	for _, want := range []string{
		"FIX (recurse pass 2, cycle 4) in the store (store, store/get.go).",
		"The workspace's test gate go:test reports:",
		"Get() != 2",
		"target: store::TestGet",
		"Acceptance: `go test -count=1 ./store`",
		"...\n",
		"TAIL the assertion failed at store.Get",
		"Cycle 3, reverted: coverage dropped",
		"return 3",
		"Cycle 1, reverted: finding still open",
		"return 1",
	} {
		if !strings.Contains(task, want) {
			t.Errorf("task missing %q\n%s", want, task)
		}
	}
	if strings.Index(task, "Cycle 3, reverted") > strings.Index(task, "Cycle 1, reverted") {
		t.Errorf("priors are not newest first:\n%s", task)
	}
	if strings.Contains(task, headToken) {
		t.Errorf("evidence kept the head past the limit")
	}
	for _, gone := range []string{
		"Fix the cause with the smallest change",
		"Do not weaken, skip or delete",
		"no gate is worse",
		"passes and no longer reports this",
		"Earlier attempts at this were reverted",
		"Find behaviour in this node",
		"Every gate must stay at least as green",
	} {
		if strings.Contains(task, gone) {
			t.Errorf("task still carries instruction prose %q", gone)
		}
	}
}

func TestRecurseImproveTask_CarriesTheNumbersAndNotTheInstructions(t *testing.T) {
	base := RecurseAttempt{
		Pass:  1,
		Cycle: 7,
		Node:  SubsystemNode{ID: "web", Title: "the web", Paths: []string{"web"}},
		Angle: "stabilize",
		Metrics: map[string]int{
			gates.MetricTests:    4,
			gates.MetricLines:    10,
			gates.MetricCoverage: 1234,
		},
		NorthStar: "serve every route",
		Prior: []PriorAttempt{
			{Cycle: 6, Why: "no metric moved", Tried: strings.Repeat("a", recurseEvidenceLimit+8)},
		},
	}
	task := recurseImproveTask(base, "web")
	for _, want := range []string{
		"STABILIZE (recurse pass 1, cycle 7) the web (web).",
		"tests: 4",
		"lines: 10",
		"coverage: 12.34%",
		"Cycle 6, reverted: no metric moved",
		"\n... (truncated)",
	} {
		if !strings.Contains(task, want) {
			t.Errorf("stabilize task missing %q\n%s", want, task)
		}
	}
	if strings.Contains(task, "serve every route") {
		t.Errorf("a stabilize task carried the north star:\n%s", task)
	}
	for _, gone := range []string{
		"Find behaviour in this node",
		"Every gate must stay at least as green",
		"Improve this node from the",
		"Earlier attempts at this were reverted",
		"Fix the cause with the smallest change",
	} {
		if strings.Contains(task, gone) {
			t.Errorf("task still carries instruction prose %q", gone)
		}
	}

	extend := base
	extend.Angle = "extend"
	extend.Prior = nil
	extend.Metrics = nil
	got := recurseImproveTask(extend, "the whole tree")
	if !strings.Contains(got, "EXTEND (recurse pass 1, cycle 7) the web (the whole tree).") {
		t.Fatalf("extend header:\n%s", got)
	}
	if !strings.Contains(got, "North star:\nserve every route") {
		t.Fatalf("extend dropped the north star:\n%s", got)
	}

	empty := RecurseAttempt{Pass: 0, Cycle: 0, Node: SubsystemNode{Title: "root"}, Angle: "polish"}
	whole := recurseImproveTask(empty, "the whole tree")
	if !strings.Contains(whole, "POLISH (recurse pass 0, cycle 0) root (the whole tree).") {
		t.Fatalf("empty-scope header:\n%s", whole)
	}
}

func TestRecurseAttemptCampaign_SetsThePromptPhase(t *testing.T) {
	fix := RecurseAttemptCampaign(t.TempDir(), fixAttempt([]string{"go", "test", "./store"}))
	if fix.PromptPhase != RecurseFixPromptPhase {
		t.Fatalf("fix phase = %q, want %q", fix.PromptPhase, RecurseFixPromptPhase)
	}
	if !strings.Contains(fix.Phases[0].Tasks[0].Description, "Acceptance: `go test ./store`") {
		t.Fatalf("fix task dropped the check:\n%s", fix.Phases[0].Tasks[0].Description)
	}

	improve := RecurseAttemptCampaign(t.TempDir(), RecurseAttempt{
		Cycle: 1, Node: attemptNode(), Angle: "harden",
	})
	if improve.PromptPhase != RecurseImprovePromptPhase {
		t.Fatalf("improve phase = %q, want %q", improve.PromptPhase, RecurseImprovePromptPhase)
	}
	noPaths := RecurseAttemptCampaign(t.TempDir(), RecurseAttempt{
		Cycle: 1, Node: SubsystemNode{ID: "root", Title: "root"}, Angle: "simplify",
	})
	if !strings.Contains(noPaths.Phases[0].Tasks[0].Description, "(the whole tree)") {
		t.Fatalf("empty paths did not become the whole tree:\n%s", noPaths.Phases[0].Tasks[0].Description)
	}
}

func TestWithPromptPhase(t *testing.T) {
	if got := withPromptPhase(nil, ""); got != nil {
		t.Fatalf("empty phase wrapped a nil context: %#v", got)
	}
	parent := context.Background()
	if withPromptPhase(parent, "   ") != parent {
		t.Fatal("a blank phase must return the context unchanged")
	}

	wrapped := withPromptPhase(nil, "  "+RecurseFixPromptPhase+" ")
	if wrapped == nil {
		t.Fatal("a phase on a nil context produced nil")
	}
	sc := types.GetSessionContext(wrapped)
	if sc == nil || !sc.CampaignActive || sc.CampaignPhase != RecurseFixPromptPhase {
		t.Fatalf("nil-ctx phase = %+v", sc)
	}

	base := &types.SessionContext{DreamMode: true, GitBranch: "main", CampaignPhase: "Discovery"}
	src := types.WithSessionContext(context.Background(), base)
	out := withPromptPhase(src, RecurseImprovePromptPhase)
	got := types.GetSessionContext(out)
	if got == nil || !got.CampaignActive || got.CampaignPhase != RecurseImprovePromptPhase || !got.DreamMode || got.GitBranch != "main" {
		t.Fatalf("copied context = %+v", got)
	}
	if base.CampaignActive || base.CampaignPhase != "Discovery" {
		t.Fatalf("withPromptPhase mutated the caller's session context: %+v", base)
	}
}

func TestSpawnTask_SetsRecursePromptPhase(t *testing.T) {
	var got *types.SessionContext
	o := &Orchestrator{
		workspace: t.TempDir(),
		campaign: &Campaign{
			ID:          "/campaign_phase_test",
			PromptPhase: RecurseFixPromptPhase,
			Acceptance:  &Acceptance{Command: []string{"go", "test", "./store"}},
		},
		taskExecutor: &MockTaskExecutor{
			ExecuteFunc: func(ctx context.Context, req session.TaskRequest) (string, error) {
				got = types.GetSessionContext(ctx)
				if _, ok := tools.CampaignCheckFrom(ctx); !ok {
					t.Errorf("prompt phase wrapping dropped the campaign check")
				}
				return "done", nil
			},
		},
	}
	task := &Task{ID: "/task_phase_0", Description: "fix", Type: TaskTypeFileModify}
	if _, err := o.spawnTask(context.Background(), task, "/fix", task.Description); err != nil {
		t.Fatalf("spawnTask: %v", err)
	}
	if got == nil || !got.CampaignActive || got.CampaignPhase != RecurseFixPromptPhase {
		t.Fatalf("session context = %+v, want active phase %s", got, RecurseFixPromptPhase)
	}

	var nilTask *types.SessionContext
	o.taskExecutor = &MockTaskExecutor{
		ExecuteFunc: func(ctx context.Context, req session.TaskRequest) (string, error) {
			nilTask = types.GetSessionContext(ctx)
			return "done", nil
		},
	}
	if _, err := o.spawnTask(context.Background(), nil, "/fix", "data"); err != nil {
		t.Fatalf("spawnTask nil task: %v", err)
	}
	if nilTask == nil || nilTask.CampaignPhase != RecurseFixPromptPhase {
		t.Fatalf("nil task context = %+v", nilTask)
	}

	var absent bool
	o.campaign.PromptPhase = ""
	o.taskExecutor = &MockTaskExecutor{
		ExecuteFunc: func(ctx context.Context, req session.TaskRequest) (string, error) {
			absent = types.GetSessionContext(ctx) == nil
			return "done", nil
		},
	}
	if _, err := o.spawnTask(context.Background(), task, "/fix", task.Description); err != nil {
		t.Fatalf("spawnTask empty phase: %v", err)
	}
	if !absent {
		t.Fatal("an empty prompt phase attached a session context")
	}
}
