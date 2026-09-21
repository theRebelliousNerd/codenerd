package campaign

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/tactile"
)

// scriptedAcceptance is an acceptance command whose rounds are written down:
// exit codes in order, the same output each time, and a record of what ran.
type scriptedAcceptance struct {
	exits  []int
	output string
	ran    []tactile.Command
}

func (s *scriptedAcceptance) Execute(_ context.Context, cmd tactile.Command) (*tactile.ExecutionResult, error) {
	exit := s.exits[min(len(s.ran), len(s.exits)-1)]
	s.ran = append(s.ran, cmd)
	return &tactile.ExecutionResult{Success: true, ExitCode: exit, Combined: s.output}, nil
}
func (s *scriptedAcceptance) Capabilities() tactile.ExecutorCapabilities {
	return tactile.ExecutorCapabilities{}
}
func (s *scriptedAcceptance) Validate(tactile.Command) error { return nil }

// acceptanceOrchestrator is a campaign whose one phase is done, on a real
// kernel evaluating the real policy: due, accepted, exhausted and blocked are
// derived in these tests, not stubbed.
func acceptanceOrchestrator(t *testing.T, accept *Acceptance, exec tactile.Executor) *Orchestrator {
	t.Helper()
	kernel, err := core.NewRealKernelWithWorkspace(t.TempDir())
	if err != nil {
		t.Fatalf("new kernel: %v", err)
	}
	orch, err := NewOrchestrator(OrchestratorConfig{
		Workspace:    t.TempDir(),
		Kernel:       kernel,
		LLMClient:    &MockLLMClient{},
		TaskExecutor: &MockTaskExecutor{},
		Executor:     exec,
		VirtualStore: &core.VirtualStore{},
		EventChan:    make(chan OrchestratorEvent, 64),
	})
	if err != nil {
		t.Fatalf("NewOrchestrator: %v", err)
	}
	orch.campaign = &Campaign{
		ID: "/campaign_accept", Title: "acceptance", Status: StatusActive,
		TotalPhases: 1, CompletedPhases: 1, TotalTasks: 1, CompletedTasks: 1,
		Acceptance: accept,
		Phases: []Phase{{
			ID: "/phase_accept_0", CampaignID: "/campaign_accept", Name: "write the docs", Status: PhaseCompleted,
			Tasks: []Task{{
				ID: "/task_accept_0_0", PhaseID: "/phase_accept_0", Description: "write", Status: TaskCompleted,
				Type: TaskTypeDocument, WriteSet: []string{"Docs/architecture/diff"},
			}},
		}},
	}
	if err := kernel.LoadFacts(orch.campaign.ToFacts()); err != nil {
		t.Fatalf("load campaign facts: %v", err)
	}
	return orch
}

// No witness declared: the phases being done is the campaign being done, and
// nothing is run. This is every campaign that existed before --accept.
func TestAcceptance_UndeclaredChangesNothing(t *testing.T) {
	exec := &scriptedAcceptance{exits: []int{1}}
	orch := acceptanceOrchestrator(t, nil, exec)

	outcome, err := orch.settleAcceptance(context.Background())
	if err != nil || outcome != acceptanceSatisfied {
		t.Fatalf("outcome = %v, err = %v; want satisfied", outcome, err)
	}
	if len(exec.ran) != 0 {
		t.Errorf("a command ran with no acceptance declared: %+v", exec.ran)
	}
	if !orch.acceptanceDerived("campaign_complete") {
		t.Error("campaign_complete is not derived for a finished campaign with no witness")
	}
}

// The defect this exists for, observed 2026-09-21: every phase done, every
// model-judged task green, "Campaign completed successfully", 17 failing
// checks. With a witness declared, done phases do not derive completion.
func TestAcceptance_AFailingWitnessHoldsCompletionAndBriefsARemediation(t *testing.T) {
	report := "diff   16 md    2 problems\n    adr/ADR-001.md: no witness line\n    03-GAP-ANALYSIS.md: row G2 has a vague exit"
	exec := &scriptedAcceptance{exits: []int{1}, output: report}
	orch := acceptanceOrchestrator(t, &Acceptance{Command: []string{"python", "scripts/check.py", "diff"}}, exec)

	if orch.acceptanceDerived("campaign_complete") {
		t.Fatal("campaign_complete is derived before the declared witness has run")
	}
	outcome, err := orch.settleAcceptance(context.Background())
	if err != nil || outcome != acceptanceRemediating {
		t.Fatalf("outcome = %v, err = %v; want remediating", outcome, err)
	}

	// The argv reached the executor as an argv, in the workspace.
	if len(exec.ran) != 1 || exec.ran[0].Binary != "python" || strings.Join(exec.ran[0].Arguments, " ") != "scripts/check.py diff" || exec.ran[0].WorkingDirectory != orch.workspace {
		t.Errorf("ran %+v", exec.ran)
	}
	if orch.acceptanceDerived("campaign_complete") {
		t.Error("campaign_complete is derived over a failed witness")
	}

	// One phase appended, pending, whose one task carries the witness's whole
	// output and is scoped to what the campaign wrote.
	if n := len(orch.campaign.Phases); n != 2 {
		t.Fatalf("phases = %d, want the original and one remediation", n)
	}
	phase := orch.campaign.Phases[1]
	if phase.Status != PhasePending || len(phase.Tasks) != 1 {
		t.Fatalf("remediation phase = %+v", phase)
	}
	task := phase.Tasks[0]
	for _, line := range strings.Split(report, "\n") {
		if !strings.Contains(task.Description, strings.TrimSpace(line)) {
			t.Errorf("the task does not carry the witness's line %q", line)
		}
	}
	if len(task.WriteSet) != 1 || task.WriteSet[0] != "Docs/architecture/diff" {
		t.Errorf("remediation write set = %v, want what the campaign wrote", task.WriteSet)
	}
	if orch.isCampaignComplete() {
		t.Error("the loop would complete the campaign with a remediation phase pending")
	}
	// The whole output is on disk, where the round says it is.
	round := orch.campaign.Acceptance.Rounds[0]
	if saved, err := os.ReadFile(filepath.Join(orch.workspace, filepath.FromSlash(round.OutputPath))); err != nil || string(saved) != report {
		t.Errorf("saved output = %q, err = %v", saved, err)
	}
}

func TestAcceptance_APassingWitnessCompletes(t *testing.T) {
	exec := &scriptedAcceptance{exits: []int{0}, output: "diff   16 md    0 problems"}
	orch := acceptanceOrchestrator(t, &Acceptance{Command: []string{"python", "scripts/check.py"}}, exec)

	outcome, err := orch.settleAcceptance(context.Background())
	if err != nil || outcome != acceptanceSatisfied {
		t.Fatalf("outcome = %v, err = %v; want satisfied", outcome, err)
	}
	if !orch.acceptanceDerived("campaign_complete") {
		t.Error("campaign_complete is not derived after the witness passed")
	}
	if len(orch.campaign.Phases) != 1 {
		t.Errorf("a passing witness appended a phase: %d phases", len(orch.campaign.Phases))
	}
	// Settled is settled: a second visit to the seam runs nothing.
	if _, err := orch.settleAcceptance(context.Background()); err != nil || len(exec.ran) != 1 {
		t.Errorf("the witness ran %d times, err = %v; want once", len(exec.ran), err)
	}
}

// The limit is the policy's (campaign_acceptance_limit). After that many failed
// rounds the kernel derives campaign_blocked /acceptance_failed and no further
// remediation is planned: a campaign that cannot satisfy its witness stops and
// says so, where it used to say "completed successfully".
func TestAcceptance_ExhaustedRoundsBlockByName(t *testing.T) {
	exec := &scriptedAcceptance{exits: []int{1}, output: "still 2 problems"}
	orch := acceptanceOrchestrator(t, &Acceptance{Command: []string{"python", "scripts/check.py"}}, exec)

	for round := 1; round <= 2; round++ {
		outcome, err := orch.settleAcceptance(context.Background())
		if err != nil || outcome != acceptanceRemediating {
			t.Fatalf("round %d: outcome = %v, err = %v; want remediating", round, outcome, err)
		}
		// The remediation phase runs and is done; the witness is due again.
		prev, err := cloneCampaign(orch.campaign)
		if err != nil {
			t.Fatal(err)
		}
		last := &orch.campaign.Phases[len(orch.campaign.Phases)-1]
		last.Status, last.Tasks[0].Status = PhaseCompleted, TaskCompleted
		if err := syncCampaignFacts(orch.kernel, prev, orch.campaign, ""); err != nil {
			t.Fatalf("sync: %v", err)
		}
	}

	outcome, err := orch.settleAcceptance(context.Background())
	if err != nil || outcome != acceptanceBlocked {
		t.Fatalf("third failed round: outcome = %v, err = %v; want blocked", outcome, err)
	}
	if reason := orch.getCampaignBlockReason(); !strings.Contains(reason, "acceptance_failed") {
		t.Errorf("block reason = %q, want /acceptance_failed", reason)
	}
	if n := len(orch.campaign.Phases); n != 3 {
		t.Errorf("phases = %d: a blocked campaign planned another remediation", n)
	}
	if len(exec.ran) != 3 {
		t.Errorf("the witness ran %d times, want 3", len(exec.ran))
	}
}

// A witness that cannot be run is a failed round with the reason as its
// output. It must not read as a pass, and it must not spin.
func TestAcceptance_ACommandThatCannotRunIsAFailedRound(t *testing.T) {
	orch := acceptanceOrchestrator(t, &Acceptance{Command: []string{"no-such-binary"}},
		&mockTactileExecutor{err: os.ErrNotExist})

	outcome, err := orch.settleAcceptance(context.Background())
	if err != nil || outcome != acceptanceRemediating {
		t.Fatalf("outcome = %v, err = %v; want remediating", outcome, err)
	}
	round := orch.campaign.Acceptance.Rounds[0]
	if round.Passed {
		t.Fatal("a command that could not run was recorded as a pass")
	}
	if !strings.Contains(orch.campaign.Phases[1].Tasks[0].Description, "could not be run") {
		t.Error("the remediation task does not say the command could not be run")
	}
}

// The witness and its rounds survive a save and a load: a resumed campaign
// neither forgets the command nor restarts its count.
func TestAcceptance_PersistsWithTheCampaign(t *testing.T) {
	c := &Campaign{ID: "/campaign_x", Acceptance: &Acceptance{
		Command: []string{"go", "vet", "./..."},
		Rounds:  []AcceptanceRound{{Round: 1, Passed: false, ExitCode: 1}},
	}}
	copied, err := cloneCampaign(c)
	if err != nil {
		t.Fatal(err)
	}
	if copied.Acceptance == nil || strings.Join(copied.Acceptance.Command, " ") != "go vet ./..." || len(copied.Acceptance.Rounds) != 1 {
		t.Fatalf("acceptance after a round trip = %+v", copied.Acceptance)
	}
	facts := copied.Acceptance.ToFacts(c.ID)
	if len(facts) != 2 || facts[0].Predicate != "campaign_acceptance" || facts[1].Predicate != "campaign_acceptance_result" {
		t.Errorf("facts = %+v", facts)
	}
}
