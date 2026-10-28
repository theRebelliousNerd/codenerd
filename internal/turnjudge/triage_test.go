package turnjudge

import (
	"context"
	"testing"

	"codenerd/internal/problemlog"
	"codenerd/internal/types"
)

type rowsQuerier map[string][]types.Fact

func (q rowsQuerier) Query(p string) ([]types.Fact, error) { return q[p], nil }

// Jev's triage reaches the kernel as one self_problem_triage per problem, and
// the split follows what the kernel derives, not the probabilities.
func TestTriageProblems_FactsAndKernelSplit(t *testing.T) {
	c, seen := fakeJev(t, map[string]float64{"problem_0": 0.92, "problem_1": 0.08, "problem_2": 0.5})
	problems := []problemlog.Problem{
		{Signature: "nil session", Level: "error", Category: "session", Sample: "nil session in executor dispatch"},
		{Signature: "429", Level: "error", Category: "api", Sample: "429 Too Many Requests from provider"},
		{Signature: "slow", Level: "warn", Category: "kernel", Sample: "slow evaluation"},
	}
	probs, err := TriageProblems(context.Background(), c, problems)
	if err != nil {
		t.Fatal(err)
	}
	if len((*seen)["questions"].(map[string]any)) != 3 {
		t.Fatal("not one question per problem")
	}
	facts := TriageFacts(problems, probs)
	if len(facts) != 3 || facts[1].Predicate != TriagePredicate {
		t.Fatalf("facts = %v", facts)
	}
	if pct, _ := types.ExtractInt64(facts[1].Args[2]); pct != 8 {
		t.Fatalf("the rate limit's percent = %d, want 8", pct)
	}

	k := rowsQuerier{AroundPredicate: {{Predicate: AroundPredicate, Args: []any{"429", "api"}}}}
	defects, around, err := SplitDefects(k, problems)
	if err != nil {
		t.Fatal(err)
	}
	if len(around) != 1 || around[0].Category != "api" {
		t.Fatalf("around = %+v, want the rate limit alone", around)
	}
	if len(defects) != 2 || defects[0].Category != "session" || defects[1].Category != "kernel" {
		t.Fatalf("defects = %+v, want the nil session then the unsettled warning", defects)
	}
}
