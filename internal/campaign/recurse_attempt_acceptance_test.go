package campaign

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/gates"
	"codenerd/internal/tactile"
)

// A fix attempt carries its finding's own check as the campaign's acceptance
// witness, copied: the attempt outlives the loop's gate state.
func TestRecurseAttemptCampaign_CarriesItsCheckAsAcceptance(t *testing.T) {
	check := []string{"go", "test", "-count=1", "./store"}
	c := RecurseAttemptCampaign(t.TempDir(), fixAttempt(check))
	if c.Acceptance == nil {
		t.Fatal("a fix attempt declares no acceptance witness")
	}
	if got := strings.Join(c.Acceptance.Command, " "); got != strings.Join(check, " ") {
		t.Fatalf("acceptance = %q, want %q", got, strings.Join(check, " "))
	}
	check[0] = "mutated"
	if c.Acceptance.Command[0] != "go" {
		t.Fatalf("the witness aliases the attempt's check: %v", c.Acceptance.Command)
	}
}

// An improvement has no finding and no single gate to re-run -- only metrics
// the ratchet reads afterwards -- so it declares no witness and completion
// stays the phases', as before.
func TestRecurseAttemptCampaign_ImprovementDeclaresNoWitness(t *testing.T) {
	c := RecurseAttemptCampaign(t.TempDir(), RecurseAttempt{
		Cycle: 1, Node: attemptNode(), Angle: "stabilize",
	})
	if c.Acceptance != nil {
		t.Fatalf("an improvement declares a witness: %+v", c.Acceptance)
	}
}

func fixAttempt(check []string) RecurseAttempt {
	return RecurseAttempt{
		Cycle: 1, Node: attemptNode(), Check: check,
		Finding: gates.Finding{ID: "f1", Gate: "go:test", Target: "store::TestGet", Message: "Get() != 2"},
	}
}

func attemptNode() SubsystemNode {
	return SubsystemNode{ID: "store", Title: "store", Paths: []string{"store"}}
}

// attemptModule is a Go module whose store test wants 2: got is what Get
// returns, so "1" fails the check and "2" passes it.
func attemptModule() map[string]string {
	return map[string]string{
		"go.mod":              "module example.com/attempt\n\ngo 1.21\n",
		"store/store_test.go": "package store\n\nimport \"testing\"\n\nfunc TestGet(t *testing.T) {\n\tif Get() != 2 {\n\t\tt.Fatal(\"Get() != 2\")\n\t}\n}\n",
	}
}

// attemptOrchestrator is the attempt's campaign after the model's "done":
// its one phase completed, on a real kernel evaluating the real policy,
// with a real executor so the witness actually runs.
func attemptOrchestrator(t *testing.T, ws string, a RecurseAttempt) *Orchestrator {
	t.Helper()
	kernel, err := core.NewRealKernelWithWorkspace(t.TempDir())
	if err != nil {
		t.Fatalf("new kernel: %v", err)
	}
	orch, err := NewOrchestrator(OrchestratorConfig{
		Workspace:    ws,
		Kernel:       kernel,
		LLMClient:    &MockLLMClient{},
		TaskExecutor: &MockTaskExecutor{},
		Executor:     tactile.NewDirectExecutor(),
		VirtualStore: &core.VirtualStore{},
		EventChan:    make(chan OrchestratorEvent, 64),
	})
	if err != nil {
		t.Fatalf("NewOrchestrator: %v", err)
	}
	camp := RecurseAttemptCampaign(ws, a)
	camp.Phases[0].Status, camp.Phases[0].Tasks[0].Status = PhaseCompleted, TaskCompleted
	camp.CompletedPhases, camp.CompletedTasks = 1, 1
	orch.campaign = camp
	if err := kernel.LoadFacts(camp.ToFacts()); err != nil {
		t.Fatalf("load campaign facts: %v", err)
	}
	return orch
}

func acceptanceVerdict(t *testing.T, orch *Orchestrator) string {
	t.Helper()
	rows, err := orch.kernel.Query("campaign_acceptance_result")
	if err != nil {
		t.Fatalf("query results: %v", err)
	}
	if len(rows) != 1 || len(rows[0].Args) < 3 {
		t.Fatalf("results = %+v, want one round", rows)
	}
	return fmt.Sprint(rows[0].Args[2])
}

// The model's "done" with the check still failing is not accepted: the
// witness runs, fails, and holds completion with a remediation phase.
func TestRecurseAttempt_AFailingCheckIsNotAccepted(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("no go toolchain")
	}
	ws := t.TempDir()
	files := attemptModule()
	files["store/store.go"] = "package store\n\nfunc Get() int { return 1 }\n"
	writeTree(t, ws, files)
	orch := attemptOrchestrator(t, ws, fixAttempt([]string{"go", "test", "-count=1", "./store"}))

	outcome, err := orch.settleAcceptance(context.Background())
	if err != nil || outcome != acceptanceRemediating {
		t.Fatalf("outcome = %v, err = %v; want remediating", outcome, err)
	}
	if orch.acceptanceDerived("campaign_complete") {
		t.Fatal("campaign_complete is derived over a failing witness")
	}
	if v := acceptanceVerdict(t, orch); !strings.Contains(v, "fail") {
		t.Fatalf("verdict = %q, want /fail", v)
	}
	if n := len(orch.campaign.Phases); n != 2 {
		t.Fatalf("phases = %d, want the attempt and one remediation", n)
	}
}

// The same attempt with the check passing is accepted: one round, a /pass
// fact, and completion derives.
func TestRecurseAttempt_APassingCheckIsAccepted(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("no go toolchain")
	}
	ws := t.TempDir()
	files := attemptModule()
	files["store/store.go"] = "package store\n\nfunc Get() int { return 2 }\n"
	writeTree(t, ws, files)
	orch := attemptOrchestrator(t, ws, fixAttempt([]string{"go", "test", "-count=1", "./store"}))

	outcome, err := orch.settleAcceptance(context.Background())
	if err != nil || outcome != acceptanceSatisfied {
		t.Fatalf("outcome = %v, err = %v; want satisfied", outcome, err)
	}
	if !orch.acceptanceDerived("campaign_complete") {
		t.Error("campaign_complete is not derived after the witness passed")
	}
	if v := acceptanceVerdict(t, orch); !strings.Contains(v, "pass") {
		t.Fatalf("verdict = %q, want /pass", v)
	}
}
