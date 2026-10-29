package learnjudge

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"codenerd/internal/types"
	"codenerd/internal/typesafe"
)

// autopoiesisAsker records the request and answers every noul with a fixed
// probability per name, or fails.
type autopoiesisAsker struct {
	state     any
	questions map[string]typesafe.Question
	answers   map[string]float64
	err       error
}

func (a *autopoiesisAsker) Ask(_ context.Context, state any, questions map[string]typesafe.Question) (*typesafe.Response, error) {
	a.state, a.questions = state, questions
	if a.err != nil {
		return nil, a.err
	}
	out := map[string]any{}
	for name := range questions {
		out[name] = map[string]any{"type": "noul", "noul": a.answers[name]}
	}
	raw, _ := json.Marshal(map[string]any{"model": "jev-test", "answers": out, "usage": map[string]any{"input_tokens": 1, "output_tokens": 1}})
	var resp typesafe.Response
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// One request asks the four request judgments over the request and its target;
// the facts carry each as a whole percent.
func TestJudgeRequestScope_RequestAndFacts(t *testing.T) {
	a := &autopoiesisAsker{answers: map[string]float64{ScopeSeveralDeliverables: 0.91, ScopeOrderedStages: 0.12, ScopeManyFiles: 0.5, ScopeContinuesAfter: 0.03}}
	got, err := JudgeRequestScope(context.Background(), a, "add an endpoint with tests and docs", "internal/api")
	if err != nil {
		t.Fatal(err)
	}
	if len(a.questions) != 4 || len(got) != 4 {
		t.Fatalf("asked %d, answered %d; want 4 and 4", len(a.questions), len(got))
	}
	state := a.state.(map[string]any)
	if state["request"] != "add an endpoint with tests and docs" || state["target"] != "internal/api" {
		t.Fatalf("state = %v", state)
	}
	facts := RequestScopeFacts(got)
	if len(facts) != 4 || facts[3].Predicate != RequestJudgmentPredicate {
		t.Fatalf("facts = %v", facts)
	}
	if pct, _ := types.ExtractInt64(facts[3].Args[1]); pct != 91 {
		t.Fatalf("several_deliverables percent = %d, want 91", pct)
	}

	if _, err := JudgeRequestScope(context.Background(), a, "", ""); err == nil {
		t.Fatal("an empty request must be refused before it costs a call")
	}
	noTarget := &autopoiesisAsker{}
	_, _ = JudgeRequestScope(context.Background(), noTarget, "watch the build", "none")
	if _, ok := noTarget.state.(map[string]any)["target"]; ok {
		t.Fatal("a target of none must not reach the state")
	}
	failing := &autopoiesisAsker{err: errors.New("down")}
	if j, err := JudgeRequestScope(context.Background(), failing, "watch the build", ""); err == nil || j != nil {
		t.Fatal("a failed call must be an error and no judgments")
	}
}

// A long tool output is bounded with the omission stated, never cut silently.
func TestJudgeToolOutput_BoundsLongOutputAndSaysSo(t *testing.T) {
	a := &autopoiesisAsker{answers: map[string]float64{OutputReportsError: 0.9}}
	long := make([]byte, toolOutputLimit*2)
	for i := range long {
		long[i] = 'x'
	}
	got, err := JudgeToolOutput(context.Background(), a, "dump", "", string(long))
	if err != nil || got[OutputReportsError] != 0.9 || len(a.questions) != 5 {
		t.Fatalf("got %v err %v questions %d", got, err, len(a.questions))
	}
	state := a.state.(map[string]any)
	if n, _ := state["output_omitted_bytes"].(int); n <= 0 {
		t.Fatalf("state = %v keys, want output_omitted_bytes", len(state))
	}
	if _, ok := state["input"]; ok {
		t.Fatal("an empty input must not reach the state")
	}
}
