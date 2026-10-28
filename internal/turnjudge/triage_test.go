package turnjudge

import (
	"context"
	"testing"

	"codenerd/internal/problemlog"
)

func TestTriageProblems_SplitsDefectsFromSurroundings(t *testing.T) {
	c, _ := fakeJev(t, map[string]float64{"problem_0": 0.92, "problem_1": 0.08, "problem_2": 0.5})
	problems := []problemlog.Problem{
		{Level: "error", Category: "session", Sample: "nil session in executor dispatch"},
		{Level: "error", Category: "api", Sample: "429 Too Many Requests from provider"},
		{Level: "warn", Category: "kernel", Sample: "slow evaluation"},
	}
	probs, err := TriageProblems(context.Background(), c, problems)
	if err != nil {
		t.Fatal(err)
	}
	defects, around := SplitDefects(problems, probs, 0.3)
	if len(around) != 1 || around[0].Category != "api" {
		t.Fatalf("around = %+v, want the rate limit alone", around)
	}
	if len(defects) != 2 || defects[0].Category != "session" || defects[1].Category != "kernel" {
		t.Fatalf("defects = %+v, want the nil session then the unsettled warning", defects)
	}
}
