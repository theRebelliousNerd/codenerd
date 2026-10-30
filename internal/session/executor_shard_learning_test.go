package session

import (
	"context"
	"errors"
	"strings"
	"testing"

	"codenerd/internal/perception"
	"codenerd/internal/store"
	"codenerd/internal/types"
)

func personaStore(t *testing.T) *store.LearningStore {
	t.Helper()
	ls, err := store.NewLearningStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ls.Close() })
	return ls
}

func learnedPredicates(t *testing.T, ls *store.LearningStore, persona string) map[string]string {
	t.Helper()
	rows, err := ls.Load(persona)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, r := range rows {
		parts := make([]string, 0, len(r.FactArgs))
		for _, a := range r.FactArgs {
			parts = append(parts, types.ExtractString(a))
		}
		out[r.FactPredicate] = strings.Join(parts, " | ")
	}
	return out
}

// The coder, tester, reviewer and researcher stores held zero learnings: the
// personas run as session-executor turns, and nothing on that path wrote
// their stores, though their profiles enable learning. A turn's settlement
// now records what it taught into its own persona's store: here a write a
// validator rejected and a later write repaired.
func TestPersonaLearning_ACorrectedTurnTeachesItsPersona(t *testing.T) {
	if p := personaForVerb("/fix"); p != "coder" {
		t.Fatalf("/fix runs as %q, want coder", p)
	}
	k := realKernel(t)
	gate := &scriptedGate{answers: []types.ToolValidation{
		{State: types.ToolValidationRejected, Reason: "invalid JSON"},
		{State: types.ToolValidationVerified},
	}}
	e, cfg, _ := effectFixture(t, k, gate, nil)
	ls := personaStore(t)
	e.SetShardLearnings(ls)
	e.SetLearningPolicy(func(persona string) bool { return persona == "coder" })

	result := &ExecutionResult{Intent: perception.Intent{Verb: "/fix", Category: "/mutation"}}
	e.executeToolBatch(context.Background(), []types.ToolCall{writeCall("c1", "a.json", `{"a":`)}, cfg, result)
	e.executeToolBatch(context.Background(), []types.ToolCall{writeCall("c2", "a.json", `{"a":1}`)}, cfg, result)
	_ = e.checkHollowSuccess(result)
	e.persistTurn(context.Background(), "fix the config file", result.Intent, result, turnTelemetry{})

	got := learnedPredicates(t, ls, "coder")
	if c := got["correction_pattern"]; !strings.Contains(c, "a.json") || !strings.Contains(c, "invalid JSON") {
		t.Fatalf("the coder learned no correction from the repaired write: %v", got)
	}
}

// A failed turn teaches its persona why it failed; a persona whose profile
// disables learning records nothing, and its turns are not handed to the
// prompt-evolution recorder either (the same enable_learning gate).
func TestPersonaLearning_TheProfileGateIsHonoured(t *testing.T) {
	e := NewExecutor(nil, nil, nil, nil, nil, nil)
	ls := personaStore(t)
	e.SetShardLearnings(ls)
	sink := &recordingSink{}
	e.SetTurnRecorder(sink)
	e.SetLearningPolicy(func(persona string) bool { return persona != "reviewer" })

	failed := &ExecutionResult{TurnOutcome: "/hollow", MissingEvidence: []string{"/test_run"}, Error: errors.New("hollow success")}
	e.persistTurn(context.Background(), "add a retry test", perception.Intent{Verb: "/test"}, failed, turnTelemetry{})
	if f := learnedPredicates(t, ls, personaForVerb("/test"))["failure_pattern"]; !strings.Contains(f, "missing evidence /test_run") {
		t.Fatalf("the failed turn taught its persona nothing: %q", f)
	}

	review := &ExecutionResult{TurnOutcome: "/failed", Error: errors.New("review crashed")}
	before := len(sink.all())
	e.persistTurn(context.Background(), "review auth", perception.Intent{Verb: "/review"}, review, turnTelemetry{})
	if got := learnedPredicates(t, ls, "reviewer"); len(got) != 0 {
		t.Fatalf("a persona with learning disabled recorded %v", got)
	}
	if len(sink.all()) != before {
		t.Fatal("a persona with learning disabled was handed to the turn recorder")
	}
}
