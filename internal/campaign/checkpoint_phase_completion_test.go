package campaign

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"codenerd/internal/config"
	"codenerd/internal/core"
	"codenerd/internal/observation"
	"codenerd/internal/session"
	"codenerd/internal/tactile"
)

// A failing checkpoint must not complete the phase.
//
// The whole point of a phase checkpoint is that "all tasks finished" and "the
// phase is done" are different claims. If a failed verification still marked
// the phase completed, the campaign would advance to the next phase on top of
// unverified work, and the completed-phase count shown to the operator would
// mean nothing. runPhase does the right thing today; this pins it, because the
// failure path and the success path converge two statements apart in
// orchestrator_tasks.go and an early `completePhase` there would be silent.
//
// The bounded escape hatch is not an exception to that: at
// campaign.max_checkpoint_attempts the phase stops spinning failure -> replan
// -> re-checkpoint and closes /unverified -- not completed -- so the phases
// built on it stay blocked and a resume re-arms the checkpoint. What a failed
// checkpoint leads to is the policy's (phase_ckpt_move), so the orchestrator
// runs on the real corpus with the campaign section published, as Run does.
func newCheckpointRegressionOrchestrator(t *testing.T, review string) (*Orchestrator, chan OrchestratorEvent) {
	t.Helper()
	return newCheckpointPolicyOrchestrator(t, review, nil)
}

func newCheckpointPolicyOrchestrator(t *testing.T, review string, edit func(*config.CampaignConfig)) (*Orchestrator, chan OrchestratorEvent) {
	t.Helper()

	kernel, err := core.NewRealKernelWithWorkspace(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	events := make(chan OrchestratorEvent, 64)
	// The phase's completed task wrote Docs/out.md; the file is there.
	workspace := t.TempDir()
	if err := os.MkdirAll(filepath.Join(workspace, "Docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "Docs", "out.md"), []byte("# out\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	orch, err := NewOrchestrator(OrchestratorConfig{
		Workspace: workspace,
		Kernel:    kernel,
		LLMClient: &MockLLMClient{},
		// A failed checkpoint appends a remediation task the next attempt
		// runs; the executor edits the phase's file, as the task asks, and
		// reports the write as a production executor does.
		TaskExecutor: &MockTaskExecutor{
			ExecuteObservedFunc: func(ctx context.Context, req session.TaskRequest) (observation.Return, error) {
				out := filepath.Join(workspace, "Docs", "out.md")
				before, err := os.ReadFile(out)
				if err != nil {
					return observation.Return{}, err
				}
				after := string(before) + "remediated\n"
				if err := os.WriteFile(out, []byte(after), 0o644); err != nil {
					return observation.Return{}, err
				}
				return observation.Return{Output: "remediated", Outcome: "/done", Writes: []observation.FileWrite{{
					Path:   out,
					Before: observation.FileState{Known: true, Exists: true, Content: string(before)},
					After:  observation.FileState{Known: true, Exists: true, Content: after},
				}}}, nil
			},
		},
		Executor:     tactile.NewDirectExecutor(),
		VirtualStore: &core.VirtualStore{},
		EventChan:    events,
		Campaign:     testCampaignConfig(edit),
	})
	if err != nil {
		t.Fatalf("NewOrchestrator: %v", err)
	}
	orch.publishPolicyParams()

	// The checkpoint runs a /shard_validation review through the task executor.
	// Returning a verdict string makes pass/fail deterministic with no LLM.
	orch.checkpoint = NewCheckpointRunner(nil, &MockTaskExecutor{
		ExecuteFunc: func(ctx context.Context, req session.TaskRequest) (string, error) {
			return review, nil
		},
	}, orch.workspace, policyKernel(t, nil))

	orch.campaign = &Campaign{
		ID:          "/campaign_ckpt",
		Title:       "checkpoint regression",
		Status:      StatusActive,
		TotalPhases: 1,
		TotalTasks:  1,
		Phases: []Phase{{
			ID:         "/phase_ckpt_0",
			CampaignID: "/campaign_ckpt",
			Name:       "verified phase",
			Status:     PhaseInProgress,
			Objectives: []PhaseObjective{{
				Type:               ObjectiveCreate,
				Description:        "produce the thing",
				VerificationMethod: VerifyShardValidate,
			}},
			Tasks: []Task{{
				ID:       "/task_ckpt_0",
				PhaseID:  "/phase_ckpt_0",
				Status:   TaskCompleted,
				Type:     TaskTypeFileCreate,
				WriteSet: []string{"Docs/out.md"},
			}},
		}},
	}
	// Run loads the campaign's facts before any phase runs; the kernel derives
	// the phase's completion and its checkpoint moves from them.
	if err := kernel.LoadFacts(orch.campaign.ToFacts()); err != nil {
		t.Fatalf("load campaign facts: %v", err)
	}
	return orch, events
}

func drainEventTypes(ch chan OrchestratorEvent) map[OrchestratorEventType]int {
	seen := make(map[OrchestratorEventType]int)
	for {
		select {
		case ev := <-ch:
			seen[ev.Type]++
		default:
			return seen
		}
	}
}

func TestRunPhase_WhenCheckpointFails_ShouldNotCompletePhase(t *testing.T) {
	orch, events := newCheckpointRegressionOrchestrator(t, "FAIL: the artifact was never written")

	if err := orch.runPhase(context.Background(), &orch.campaign.Phases[0]); err != nil {
		t.Fatalf("runPhase returned error: %v", err)
	}

	phase := orch.campaign.Phases[0]
	if phase.Status == PhaseCompleted {
		t.Fatal("phase was marked completed despite a FAILED checkpoint: unverified work would be treated as verified")
	}
	if orch.campaign.CompletedPhases != 0 {
		t.Fatalf("CompletedPhases = %d after a failed checkpoint, want 0", orch.campaign.CompletedPhases)
	}
	if phase.CheckpointFailures != 1 {
		t.Fatalf("CheckpointFailures = %d, want 1", phase.CheckpointFailures)
	}
	if len(phase.Checkpoints) != 1 || phase.Checkpoints[0].Passed {
		t.Fatalf("expected one recorded FAILED checkpoint, got %+v", phase.Checkpoints)
	}

	seen := drainEventTypes(events)
	if seen[EventCheckpointFailed] == 0 {
		t.Fatalf("expected a %s event so the operator learns why the phase stayed open; got %v", EventCheckpointFailed, seen)
	}
	if seen[EventPhaseCompleted] != 0 {
		t.Fatalf("a phase_completed event was emitted for a failed checkpoint; got %v", seen)
	}
}

func TestRunPhase_WhenCheckpointPasses_ShouldCompletePhase(t *testing.T) {
	orch, _ := newCheckpointRegressionOrchestrator(t, `{"control_packet": {"mangle_updates": ["checkpoint_verdict(\"phase_ckpt_0\", /pass, \"everything verified\", 95)"]}, "surface_response": "done"}`)

	if err := orch.runPhase(context.Background(), &orch.campaign.Phases[0]); err != nil {
		t.Fatalf("runPhase returned error: %v", err)
	}

	if orch.campaign.Phases[0].Status != PhaseCompleted {
		t.Fatalf("phase status = %s after a PASSING checkpoint, want %s",
			orch.campaign.Phases[0].Status, PhaseCompleted)
	}
	if orch.campaign.CompletedPhases != 1 {
		t.Fatalf("CompletedPhases = %d, want 1", orch.campaign.CompletedPhases)
	}
}

// exhaustCheckpoints runs the phase until its checkpoint attempts are spent.
func exhaustCheckpoints(t *testing.T, orch *Orchestrator) {
	t.Helper()
	max := orch.policy.MaxCheckpointAttempts
	for attempt := 1; attempt <= max; attempt++ {
		if err := orch.runPhase(context.Background(), &orch.campaign.Phases[0]); err != nil {
			t.Fatalf("runPhase attempt %d returned error: %v", attempt, err)
		}
		if orch.campaign.Phases[0].Status == PhaseCompleted {
			t.Fatalf("phase completed on attempt %d of %d with every checkpoint failing", attempt, max)
		}
		if attempt < max && orch.campaign.Phases[0].Status == PhaseUnverified {
			t.Fatalf("phase closed unverified on attempt %d, before the cap of %d", attempt, max)
		}
	}
}

// phaseStatusRows is every campaign_phase status the kernel holds.
func phaseStatusRows(t *testing.T, orch *Orchestrator) []string {
	t.Helper()
	facts, err := orch.kernel.Query("campaign_phase")
	if err != nil {
		t.Fatal(err)
	}
	var rows []string
	for _, f := range facts {
		if len(f.Args) > 4 {
			rows = append(rows, fmt.Sprint(f.Args[4]))
		}
	}
	return rows
}

// External audit N03 (2026-09-19): the cap on failed checkpoints used to
// complete the phase -- /completed in the kernel, a completed-phase count, a
// success to the Northstar observer -- so a known failed checkpoint unlocked
// the phases built on it. The phase closes /unverified now: announced, and not
// completed anywhere.
func TestRunPhase_WhenCheckpointExhausted_ThePhaseClosesUnverified(t *testing.T) {
	orch, events := newCheckpointRegressionOrchestrator(t, "FAIL: still broken")
	exhaustCheckpoints(t, orch)

	phase := orch.campaign.Phases[0]
	if phase.Status != PhaseUnverified {
		t.Fatalf("after %d failed checkpoints status = %s, want %s", orch.policy.MaxCheckpointAttempts, phase.Status, PhaseUnverified)
	}
	if orch.campaign.CompletedPhases != 0 {
		t.Fatalf("CompletedPhases = %d, want 0: an unverified phase is not a completed one", orch.campaign.CompletedPhases)
	}

	seen := drainEventTypes(events)
	if seen[EventCheckpointExhausted] == 0 {
		t.Fatalf("closing a phase unverified must be announced with %s; got %v", EventCheckpointExhausted, seen)
	}
	if seen[EventPhaseCompleted] != 0 {
		t.Fatalf("a phase_completed event was emitted for a phase whose checkpoint never passed; got %v", seen)
	}

	rows := phaseStatusRows(t, orch)
	if !slices.Contains(rows, "/unverified") || slices.Contains(rows, "/completed") {
		t.Fatalf("kernel campaign_phase statuses = %v, want /unverified and never /completed", rows)
	}

	// Every checkpoint on record failed, and that record is what the report
	// and the operator see.
	if len(phase.Checkpoints) == 0 {
		t.Fatal("no checkpoint records survived; a phase closed unverified with no evidence of why")
	}
	for _, cp := range phase.Checkpoints {
		if cp.Passed {
			t.Fatalf("a passing checkpoint appeared on a phase whose verification never passed: %+v", cp)
		}
	}
}

// A resume re-arms the checkpoint: the phase is back in progress with a fresh
// attempt budget, still not completed -- the debt survives -- and only a
// passing checkpoint completes it.
func TestPrepareResume_ReArmsAnUnverifiedPhase(t *testing.T) {
	orch, _ := newCheckpointRegressionOrchestrator(t, "FAIL: still broken")
	exhaustCheckpoints(t, orch)
	// What the loop does when campaign_blocked(/phase_unverified) derives.
	orch.campaign.Status = StatusFailed

	if err := orch.PrepareResume(); err != nil {
		t.Fatalf("PrepareResume: %v", err)
	}
	phase := orch.campaign.Phases[0]
	if phase.Status != PhaseInProgress || phase.CheckpointFailures != 0 {
		t.Fatalf("after resume status = %s, CheckpointFailures = %d; want %s with a fresh budget", phase.Status, phase.CheckpointFailures, PhaseInProgress)
	}
	if orch.campaign.CompletedPhases != 0 {
		t.Fatalf("CompletedPhases = %d after resume, want 0", orch.campaign.CompletedPhases)
	}

	orch.checkpoint = NewCheckpointRunner(nil, &MockTaskExecutor{
		ExecuteFunc: func(ctx context.Context, req session.TaskRequest) (string, error) {
			return `{"control_packet": {"mangle_updates": ["checkpoint_verdict(\"phase_ckpt_0\", /pass, \"fixed\", 95)"]}, "surface_response": "done"}`, nil
		},
	}, orch.workspace, policyKernel(t, nil))
	if err := orch.runPhase(context.Background(), &orch.campaign.Phases[0]); err != nil {
		t.Fatalf("runPhase after resume: %v", err)
	}
	if orch.campaign.Phases[0].Status != PhaseCompleted || orch.campaign.CompletedPhases != 1 {
		t.Fatalf("a passing checkpoint after resume must complete the phase: status = %s, CompletedPhases = %d",
			orch.campaign.Phases[0].Status, orch.campaign.CompletedPhases)
	}
}

// A resume must reach the kernel: it derives what blocks a campaign from its
// facts, and Run does not reload them. PrepareResume re-armed the phase in Go
// only, the kernel still held it /unverified, and the resumed campaign blocked
// on /phase_unverified before any work (campaign 7b853890, 2026-09-26).
func TestPrepareResume_TheKernelNoLongerBlocksARearmedPhase(t *testing.T) {
	orch, _ := newCheckpointRegressionOrchestrator(t, "FAIL: still broken")
	exhaustCheckpoints(t, orch)
	orch.campaign.Status = StatusFailed
	orch.campaign.BlockReason = "/phase_unverified"
	if got := orch.getCampaignBlockReason(); got != "/phase_unverified" {
		t.Fatalf("precondition: the kernel's block reason is %q, want /phase_unverified", got)
	}

	if err := orch.PrepareResume(); err != nil {
		t.Fatalf("PrepareResume: %v", err)
	}
	if got := orch.getCampaignBlockReason(); got != "" {
		t.Fatalf("after resume the kernel still blocks the campaign: %q", got)
	}
	rows := phaseStatusRows(t, orch)
	if !slices.Contains(rows, "/in_progress") || slices.Contains(rows, "/unverified") {
		t.Fatalf("kernel campaign_phase statuses after resume = %v, want /in_progress", rows)
	}
}

// How many failed checkpoints close a phase is the user's
// (campaign.max_checkpoint_attempts), read by the policy: with 1, the first
// failure closes it. A Go const of 3 decided this while the config key was
// published to the kernel and read by nothing (sweep finding F3).
func TestRunPhase_TheCheckpointCapIsTheUsersConfig(t *testing.T) {
	orch, events := newCheckpointPolicyOrchestrator(t, "FAIL: still broken", func(c *config.CampaignConfig) {
		c.MaxCheckpointAttempts = 1
	})
	if err := orch.runPhase(context.Background(), &orch.campaign.Phases[0]); err != nil {
		t.Fatalf("runPhase: %v", err)
	}
	if got := orch.campaign.Phases[0].Status; got != PhaseUnverified {
		t.Fatalf("with max_checkpoint_attempts=1 one failed checkpoint left the phase %s, want %s", got, PhaseUnverified)
	}
	if drainEventTypes(events)[EventCheckpointExhausted] == 0 {
		t.Fatal("closing the phase unverified was not announced")
	}
}

// A structured /fail verdict's reason is one line; the reviewer's report names
// the files. Until 2026-09-26 only the line reached the checkpoint, so the
// remediation could not see what was at fault (campaign 7b853890: rounds 4
// and 5 failed identically on files its brief never named).
func TestRunPhase_AFailedVerdictBriefsTheRemediationWithTheReviewersReport(t *testing.T) {
	review := `{"control_packet": {"mangle_updates": ["checkpoint_verdict(\"phase_ckpt_0\", /fail, \"missing front-matter\", 90)."]}, ` +
		`"surface_response": "Verdict: FAIL. Docs/INTERNALS.md and Docs/WIRING.md lack the required front-matter."}`
	orch, _ := newCheckpointRegressionOrchestrator(t, review)
	if err := orch.runPhase(context.Background(), &orch.campaign.Phases[0]); err != nil {
		t.Fatalf("runPhase: %v", err)
	}
	phase := orch.campaign.Phases[0]
	if len(phase.Checkpoints) != 1 || phase.Checkpoints[0].Passed {
		t.Fatalf("want one failed checkpoint, got %+v", phase.Checkpoints)
	}
	if len(phase.Tasks) != 2 {
		t.Fatalf("the phase has %d tasks, want the remediation added", len(phase.Tasks))
	}
	for _, want := range []string{"missing front-matter", "Docs/INTERNALS.md and Docs/WIRING.md lack the required front-matter"} {
		if !strings.Contains(phase.Tasks[1].Description, want) {
			t.Errorf("the remediation brief lacks %q:\n%s", want, phase.Tasks[1].Description)
		}
	}
}

// Whether a failed checkpoint gives the phase remediation work is the user's
// (campaign.replan_on_checkpoint_failure, read by phase_ckpt_move). Off, the
// phase stays open for its checkpoint to run again over the same work.
func TestRunPhase_ReplanOnCheckpointFailureIsTheUsersConfig(t *testing.T) {
	for _, tc := range []struct {
		replan bool
		want   int
	}{{true, 2}, {false, 1}} {
		orch, _ := newCheckpointPolicyOrchestrator(t, "FAIL: still broken", func(c *config.CampaignConfig) {
			c.ReplanOnCheckpointFailure = &tc.replan
		})
		if err := orch.runPhase(context.Background(), &orch.campaign.Phases[0]); err != nil {
			t.Fatalf("replan=%v: runPhase: %v", tc.replan, err)
		}
		if got := orch.campaign.Phases[0].Status; got != PhaseInProgress {
			t.Fatalf("replan=%v: one failed checkpoint below the cap left the phase %s, want %s", tc.replan, got, PhaseInProgress)
		}
		if got := len(orch.campaign.Phases[0].Tasks); got != tc.want {
			t.Fatalf("replan_on_checkpoint_failure=%v: the phase has %d task(s), want %d", tc.replan, got, tc.want)
		}
	}
}

// A phase whose tasks declare no file gives a remediation no scope, and a file
// task without a target is refused before it runs: none is appended, so the
// phase is not blocked on a task that can never run.
func TestRunPhase_ACheckpointRemediationNeedsADeclaredTarget(t *testing.T) {
	orch, events := newCheckpointRegressionOrchestrator(t, "FAIL: still broken")
	orch.campaign.Phases[0].Tasks[0].WriteSet = nil

	if err := orch.runPhase(context.Background(), &orch.campaign.Phases[0]); err != nil {
		t.Fatalf("runPhase: %v", err)
	}
	if got := len(orch.campaign.Phases[0].Tasks); got != 1 {
		t.Fatalf("the phase has %d tasks, want no remediation without a declared target", got)
	}
	if drainEventTypes(events)[EventReplanFailed] == 0 {
		t.Fatal("the refused remediation was not announced")
	}
}

// A failed checkpoint's findings become the phase's work. The /replan move
// called the replanner with an empty reason and a bare /checkpoint_failed
// trigger, so the findings reached no one: on campaign 7b853890 (2026-09-24)
// the reviewer named the exact defects, nothing was added, the phase
// re-checked unchanged work and closed unverified after three attempts. Now
// the phase gains one task briefed with the findings whole -- past the 500
// characters the summary line keeps -- and its objectives, scoped to what the
// phase's tasks write, and the phase stays open.
func TestRunPhase_WhenCheckpointFails_ItsFindingsBecomeThePhasesWork(t *testing.T) {
	findings := "FAIL: the corpus is not assembled.\n- [CRITICAL] Docs/a.md:22: claims a slot the file does not have\n" +
		strings.Repeat("- [MINOR] Docs/b.md: a sentence the standard does not allow\n", 12) +
		"- [CRITICAL] Docs/INTERNALS.md: no front-matter (the last finding)"
	if len(findings) <= 500 {
		t.Fatalf("the findings must be longer than the 500-character summary to prove they arrive whole")
	}
	orch, _ := newCheckpointRegressionOrchestrator(t, findings)
	orch.campaign.Phases[0].Tasks[0].WriteSet = []string{"Docs/b.md", "Docs/a.md"}
	orch.campaign.Phases[0].Tasks[0].Artifacts = []TaskArtifact{{Type: "/doc", Path: ".nerd/campaigns/ckpt/artifacts/report.md"}}
	docs := normalizeAbsolutePath(orch.workspace, "Docs")

	if err := orch.runPhase(context.Background(), &orch.campaign.Phases[0]); err != nil {
		t.Fatalf("runPhase: %v", err)
	}
	phase := orch.campaign.Phases[0]
	if phase.Status == PhaseCompleted || phase.Status == PhaseUnverified {
		t.Fatalf("phase status %s after one failed checkpoint; it stays open", phase.Status)
	}
	if len(phase.Tasks) != 2 {
		t.Fatalf("the phase has %d tasks after a failed checkpoint, want the remediation task added", len(phase.Tasks))
	}
	task := phase.Tasks[1]
	if task.Status != TaskPending || task.Type != TaskTypeFileModify {
		t.Fatalf("remediation task %+v: want a pending file modification", task)
	}
	for _, want := range []string{"(the last finding)", "Docs/a.md:22", "produce the thing", docs} {
		if !strings.Contains(task.Description, want) {
			t.Errorf("the remediation brief lacks %q:\n%s", want, task.Description)
		}
	}
	// The scope is the directory the phase wrote into -- a checkpoint judges
	// files next to the declared ones -- and never a task's report under .nerd/.
	if !slices.Equal(task.WriteSet, []string{docs}) {
		t.Errorf("write set %v, want the phase's directory %s", task.WriteSet, docs)
	}
	rows, err := orch.kernel.Query("campaign_task")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, f := range rows {
		if len(f.Args) > 3 && fmt.Sprint(f.Args[0]) == task.ID && strings.Contains(fmt.Sprint(f.Args[3]), "pending") {
			found = true
		}
	}
	if !found {
		t.Errorf("the kernel holds no pending campaign_task for %s: the scheduler would never run it", task.ID)
	}
}
