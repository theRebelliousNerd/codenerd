package system

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	pe "codenerd/internal/autopoiesis/prompt_evolution"
	nerdconfig "codenerd/internal/config"
	"codenerd/internal/prompt"

	"gopkg.in/yaml.v3"
)

type stubJudgeLLM struct{}

func (stubJudgeLLM) Complete(context.Context, string) (string, error) { return "", nil }
func (stubJudgeLLM) CompleteWithSystem(context.Context, string, string) (string, error) {
	return `{"verdict":"PASS","explanation":"ok","category":"CORRECT"}`, nil
}

// End to end through the production pieces: the evolver's trial envelope is
// materialized by trialFactSink into the real kernel, the shipped rule
// (policy/prompt_trials.mg) derives prompt_trial_admitted in a compile scope,
// and the evolved-atom source serves the quarantined candidate only while the
// envelope is live. Exhausting the exposure budget withdraws admission.
func TestPromptTrialAdmission_KernelDecidesAndExposureBudgetEndsIt(t *testing.T) {
	kernel, adapter := newPromptScopeTestKernel(t)
	nerdDir := t.TempDir()

	candidate := prompt.NewPromptAtom("evolved/trial_candidate", prompt.CategoryMethodology, "trial guidance")
	// An evolved atom is pinned to the exact model and the source role it was
	// learned on; one with no pin is quarantined at load.
	candidate.ShardTypes = []string{"/coder"}
	candidate.Providers = []string{"anthropic"}
	candidate.Models = []string{"claude-opus-5-5"}
	pendingDir := filepath.Join(nerdDir, "prompts", "evolved", "pending")
	if err := os.MkdirAll(pendingDir, 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := yaml.Marshal(struct {
		Atom      *prompt.PromptAtom `yaml:"atom"`
		ShardType string             `yaml:"shard_type"`
		Provider  string             `yaml:"provider"`
		Model     string             `yaml:"model"`
	}{candidate, "/coder", "anthropic", "claude-opus-5-5"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pendingDir, "evolved_trial_candidate.yaml"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	section := nerdconfig.DefaultPromptEvolutionConfig()
	section.TrialMaxExposures = 1
	evolver, err := pe.NewPromptEvolver(nerdDir, stubJudgeLLM{}, pe.EvolverConfigFrom(section), nil, nerdconfig.JITConfig{})
	if err != nil {
		t.Fatal(err)
	}
	defer evolver.Close()
	evolver.SetTrialFactSink(trialFactSink(kernel))
	if err := kernel.UpdateSystemFacts(); err != nil {
		t.Fatal(err)
	}
	mgr := prompt.NewEvolvedAtomManager(nerdDir)

	served := func() []string {
		t.Helper()
		scope, err := adapter.NewCompilationScope()
		if err != nil {
			t.Fatal(err)
		}
		defer scope.Close()
		rows, err := scope.Query(prompt.PromptTrialAdmittedPredicate)
		if err != nil {
			t.Fatal(err)
		}
		admitted := map[string]string{}
		for _, row := range rows {
			admitted[fmt.Sprint(row.Args[0])] = fmt.Sprint(row.Args[1])
		}
		cc := prompt.NewCompilationContext()
		cc.ShardType = "/coder"
		cc.Provider = "anthropic"
		cc.Model = "claude-opus-5-5"
		var ids []string
		for _, atom := range mgr.GetMatching(cc, admitted) {
			ids = append(ids, atom.ID)
		}
		return ids
	}

	if got := served(); len(got) != 0 {
		t.Fatalf("quarantined candidate served without a trial: %v", got)
	}
	if _, err := evolver.ApproveTrial(candidate.ID, time.Hour, 0); err != nil {
		t.Fatal(err)
	}
	if got := served(); len(got) != 1 || got[0] != candidate.ID {
		t.Fatalf("approved trial not admitted by the kernel: %v", got)
	}

	// One recorded exposure exhausts a budget of one.
	if err := evolver.RecordExecution(&pe.ExecutionRecord{
		TaskID: "trial-exposure-1", SessionID: "s", ShardType: "/coder",
		Timestamp: time.Unix(1700000000, 0), AtomIDs: []string{candidate.ID},
	}); err != nil {
		t.Fatal(err)
	}
	if got := served(); len(got) != 0 {
		t.Fatalf("exhausted trial still admitted: %v", got)
	}
}

// The production recorder goes through the owned queue: a turn submitted
// before Close is persisted by Close's drain, and one after is rejected.
func TestTurnEvolutionRecorder_DrainsThroughEvolverClose(t *testing.T) {
	nerdDir := t.TempDir()
	evolver, err := pe.NewPromptEvolver(nerdDir, stubJudgeLLM{}, pe.DefaultEvolverConfig(), nil, nerdconfig.JITConfig{})
	if err != nil {
		t.Fatal(err)
	}
	recorder := newTurnEvolutionRecorder(evolver)
	recorder.RecordTurn(turnRec("/done"))
	if err := evolver.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	stats := evolver.GetStats().Recording
	if stats.Accepted != 1 || stats.Persisted != 1 {
		t.Fatalf("recording after drain = %+v, want one accepted and persisted", stats)
	}
	recorder.RecordTurn(turnRec("/done"))
	if got := evolver.GetStats().Recording.RejectedClosed; got != 1 {
		t.Fatalf("turn after Close: rejected_closed = %d, want 1", got)
	}
}
