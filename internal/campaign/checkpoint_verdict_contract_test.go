package campaign

import (
	"codenerd/internal/core"
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
		phase, verdict, reason, ok := parseCheckpointVerdictAtom(atom)
		if !ok {
			t.Fatalf("parseCheckpointVerdictAtom(%q) = !ok, want ok", atom)
		}
		if phase != "P" {
			t.Errorf("parseCheckpointVerdictAtom(%q) phase = %q, want %q", atom, phase, "P")
		}
		if verdict != "pass" {
			t.Errorf("parseCheckpointVerdictAtom(%q) verdict = %q, want %q", atom, verdict, "pass")
		}
		if reason != "ok" {
			t.Errorf("parseCheckpointVerdictAtom(%q) reason = %q, want %q", atom, reason, "ok")
		}
	}
}

// TestCheckpointPrompts_VerdictExampleEndsWithPeriod captures the prompts
// handed to the reviewer/nemesis shards and asserts the example atom carries
// the required trailing period, plus the explicit period sentence. It also
// asserts the assault_tasks.go prompt site carries the same contract.
func TestCheckpointPrompts_VerdictExampleEndsWithPeriod(t *testing.T) {
	capturePrompt := func(t *testing.T, run func(exec *MockTaskExecutor, cr *CheckpointRunner)) string {
		t.Helper()
		var captured string
		exec := &MockTaskExecutor{
			ExecuteFunc: func(ctx context.Context, req session.TaskRequest) (string, error) {
				captured = req.Task
				return "plain prose, no verdict", nil
			},
		}
		cr := NewCheckpointRunner(nil, exec, t.TempDir(), &MockKernel{})
		run(exec, cr)
		if captured == "" {
			t.Fatal("expected task executor to capture a prompt, got empty")
		}
		return captured
	}

	shardPrompt := capturePrompt(t, func(exec *MockTaskExecutor, cr *CheckpointRunner) {
		_, _, _ = cr.runShardValidationCheckpoint(context.Background(), &Phase{Name: "P"})
	})
	if !strings.Contains(shardPrompt, `checkpoint_verdict(\"my-phase\", /pass, \"all objectives met\", 95).`) {
		t.Errorf("shard-validation prompt example must end with period; got:\n%s", shardPrompt)
	}
	if !strings.Contains(shardPrompt, "The atom must end with a period; it is asserted into the kernel as a fact.") {
		t.Errorf("shard-validation prompt must state the period contract; got:\n%s", shardPrompt)
	}
	if !strings.Contains(shardPrompt, "Free-text PASS/FAIL is not accepted") {
		t.Errorf("shard-validation prompt must keep the free-text sentence; got:\n%s", shardPrompt)
	}

	nemesisPrompt := capturePrompt(t, func(exec *MockTaskExecutor, cr *CheckpointRunner) {
		_, _, _ = cr.runNemesisGauntletCheckpoint(context.Background(), &Phase{Name: "P"})
	})
	if !strings.Contains(nemesisPrompt, `checkpoint_verdict(\"my-phase\", /pass, \"no weaknesses found\", 95).`) {
		t.Errorf("nemesis prompt example must end with period; got:\n%s", nemesisPrompt)
	}
	if !strings.Contains(nemesisPrompt, "The atom must end with a period; it is asserted into the kernel as a fact.") {
		t.Errorf("nemesis prompt must state the period contract; got:\n%s", nemesisPrompt)
	}
	if !strings.Contains(nemesisPrompt, "Free-text PASS/FAIL is not accepted") {
		t.Errorf("nemesis prompt must keep the free-text sentence; got:\n%s", nemesisPrompt)
	}

	// assault_tasks.go builds its nemesis prompt inline in runAssaultStage;
	// assert the file carries the same contract so the third site cannot drift.
	raw, err := os.ReadFile("assault_tasks.go")
	if err != nil {
		t.Fatalf("read assault_tasks.go: %v", err)
	}
	src := string(raw)
	if !strings.Contains(src, "no weaknesses found") || !strings.Contains(src, "95).") {
		t.Error("assault_tasks.go prompt example must end with period (95).")
	}
	if !strings.Contains(src, "The atom must end with a period; it is asserted into the kernel as a fact.") {
		t.Error("assault_tasks.go prompt must state the period contract.")
	}
}

// TestShardValidationCheckpoint_RetractsStaleVerdictBeforeSpawn ensures a
// verdict asserted earlier in the phase cannot pre-approve the checkpoint:
// the runner retracts any existing checkpoint_verdict for the phase before
// spawning the reviewer, so prose-only review must fail closed.
func TestShardValidationCheckpoint_RetractsStaleVerdictBeforeSpawn(t *testing.T) {
	kernel := &MockKernel{Facts: []core.Fact{
		{Predicate: "checkpoint_verdict", Args: []any{"P", types.MangleAtom("/pass"), "stale", int64(99)}},
	}}
	exec := &MockTaskExecutor{
		ExecuteFunc: func(ctx context.Context, req session.TaskRequest) (string, error) {
			return "plain prose, no verdict", nil
		},
	}
	cr := NewCheckpointRunner(nil, exec, t.TempDir(), kernel)
	passed, details, err := cr.runShardValidationCheckpoint(context.Background(), &Phase{Name: "P"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if passed {
		t.Fatalf("expected FAIL when only a stale pre-spawn verdict exists (it must be retracted); got PASS details=%q", details)
	}
	lower := strings.ToLower(details)
	if !strings.Contains(lower, "could not be determined") {
		t.Errorf("fail-closed details should say verdict could not be determined; got %q", details)
	}
	if !strings.Contains(details, "reviewer control packet carried no checkpoint_verdict/4 for this phase") {
		t.Errorf("fail-closed details should distinguish kernel-never-received; got %q", details)
	}
}
