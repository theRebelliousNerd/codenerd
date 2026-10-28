package campaign

import (
	"codenerd/internal/session"
	"codenerd/internal/types"
	"context"
	"os"
	"strings"
	"testing"
)

// TestParseCheckpointVerdictAtom_AcceptsTrailingPeriod verifies the parser
// accepts both the bare atom and the kernel fact form with a trailing period
// (the emitter requires the period; FilterMangleUpdates tolerates it).
func TestParseCheckpointVerdictAtom_AcceptsTrailingPeriod(t *testing.T) {
	for _, atom := range []string{
		`checkpoint_verdict("P", /pass, "ok", 90)`,
		`checkpoint_verdict("P", /pass, "ok", 90).`,
	} {
		key, verdict, reason, confidence, ok := parseCheckpointVerdictAtom(atom)
		if !ok {
			t.Fatalf("parseCheckpointVerdictAtom(%q) = !ok, want ok", atom)
		}
		if key != "P" {
			t.Errorf("parseCheckpointVerdictAtom(%q) key = %q, want %q", atom, key, "P")
		}
		if confidence != 90 {
			t.Errorf("parseCheckpointVerdictAtom(%q) confidence = %d, want 90", atom, confidence)
		}
		if verdict != "pass" {
			t.Errorf("parseCheckpointVerdictAtom(%q) verdict = %q, want %q", atom, verdict, "pass")
		}
		if reason != "ok" {
			t.Errorf("parseCheckpointVerdictAtom(%q) reason = %q, want %q", atom, reason, "ok")
		}
	}
}

// TestCheckpointTasks_CarryTheVerdictKeyAndThePromptPhase captures the turns
// handed to the reviewer and nemesis shards: each task carries the phase and
// its verdict key as data, and each turn's context carries the checkpoint
// prompt phase that compiles the checkpoint_verdict/4 contract into the
// shard's system prompt (TestCheckpointVerdictContracts_ReachOnlyTheCheckpointTurn
// compiles it). No contract text is left in the task.
func TestCheckpointTasks_CarryTheVerdictKeyAndThePromptPhase(t *testing.T) {
	capture := func(t *testing.T, run func(cr *CheckpointRunner)) (string, string) {
		t.Helper()
		var task, phase string
		exec := &MockTaskExecutor{
			ExecuteFunc: func(ctx context.Context, req session.TaskRequest) (string, error) {
				task = req.Task
				if sc := types.GetSessionContext(ctx); sc != nil && sc.CampaignActive {
					phase = sc.CampaignPhase
				}
				return "plain prose, no verdict", nil
			},
		}
		run(NewCheckpointRunner(nil, exec, t.TempDir(), &MockKernel{}))
		if task == "" {
			t.Fatal("expected task executor to capture a task, got empty")
		}
		return task, phase
	}

	for name, tc := range map[string]struct {
		run   func(cr *CheckpointRunner)
		phase string
	}{
		"review": {func(cr *CheckpointRunner) {
			_, _, _ = cr.runShardValidationCheckpoint(context.Background(), &Phase{ID: "/phase_p", Name: "P"})
		}, CheckpointReviewPromptPhase},
		"nemesis": {func(cr *CheckpointRunner) {
			_, _, _ = cr.runNemesisGauntletCheckpoint(context.Background(), &Phase{ID: "/phase_p", Name: "P"})
		}, CheckpointNemesisPromptPhase},
	} {
		t.Run(name, func(t *testing.T) {
			task, phase := capture(t, tc.run)
			if phase != tc.phase {
				t.Errorf("the %s turn compiled under campaign phase %q, want %q", name, phase, tc.phase)
			}
			if !strings.Contains(task, `Verdict key: "phase_p"`) || !strings.Contains(task, "Phase: P") {
				t.Errorf("the %s task does not name its phase and verdict key:\n%s", name, task)
			}
			if strings.Contains(task, "checkpoint_verdict") {
				t.Errorf("the %s task carries contract text; the contract is the atom's:\n%s", name, task)
			}
		})
	}

	// assault_tasks.go spawns its nemesis stage on the same phase.
	raw, err := os.ReadFile("assault_tasks.go")
	if err != nil {
		t.Fatalf("read assault_tasks.go: %v", err)
	}
	if !strings.Contains(string(raw), "withPromptPhase(ctx, CheckpointNemesisPromptPhase)") {
		t.Error("assault_tasks.go's nemesis stage does not compile under CheckpointNemesisPromptPhase")
	}
}
