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
	probs, err := TriageProblems(context.Background(), c, problems, 20)
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

type queryFailer struct{}

func (queryFailer) Query(string) ([]types.Fact, error) { return nil, context.DeadlineExceeded }

func TestSplitDefects_QueryErrorAndMalformedRows(t *testing.T) {
	problems := []problemlog.Problem{
		{Signature: "a", Category: "session"},
		{Signature: "b", Category: "api"},
	}
	cases := []struct {
		name        string
		querier     Querier
		wantDefects int
		wantAround  int
		wantErr     bool
	}{
		{name: "malformed rows are ignored", querier: rowsQuerier{AroundPredicate: []types.Fact{{Predicate: AroundPredicate, Args: []any{types.MangleString("only-one")}}, {Predicate: AroundPredicate, Args: []any{types.MangleString("a"), types.MangleString("session"), types.MangleString("extra")}}, {Predicate: AroundPredicate, Args: []any{types.MangleString("a"), types.MangleString("session")}}}}, wantDefects: 1, wantAround: 1},
		{name: "no rows keeps every problem as a defect", querier: rowsQuerier{}, wantDefects: 2, wantAround: 0},
		{name: "query error is an error", querier: queryFailer{}, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			defects, around, err := SplitDefects(tc.querier, problems)
			if tc.wantErr {
				if err == nil || defects != nil || around != nil {
					t.Fatalf("SplitDefects = %v %v %v, want error and nil lists", defects, around, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(defects) != tc.wantDefects || len(around) != tc.wantAround {
				t.Fatalf("defects = %v around = %v, want %d and %d", defects, around, tc.wantDefects, tc.wantAround)
			}
		})
	}
}

func TestTriageFacts_TruncatesToShorterInput(t *testing.T) {
	problems := []problemlog.Problem{
		{Signature: "a", Category: "session"},
		{Signature: "b", Category: "api"},
		{Signature: "c", Category: "kernel"},
	}
	cases := []struct {
		name  string
		probs []float64
		want  int
	}{
		{name: "shorter probs stop the facts", probs: []float64{0.9}, want: 1},
		{name: "exact length projects every problem", probs: []float64{0.9, 0.1, 0.5}, want: 3},
		{name: "longer probs add nothing", probs: []float64{0.9, 0.1, 0.5, 0.2}, want: 3},
		{name: "no probs project nothing", probs: nil, want: 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			facts := TriageFacts(problems, tc.probs)
			if len(facts) != tc.want {
				t.Fatalf("facts = %d, want %d", len(facts), tc.want)
			}
			for _, f := range facts {
				if f.Predicate != TriagePredicate {
					t.Fatalf("predicate = %s, want %s", f.Predicate, TriagePredicate)
				}
			}
		})
	}
	if pct, _ := types.ExtractInt64(TriageFacts(problems[:1], []float64{0.5})[0].Args[2]); pct != 50 {
		t.Fatalf("percent = %d, want 50", pct)
	}
}

func TestRefuted_FiltersByReviewID(t *testing.T) {
	verdicts := []FindingVerdict{
		{Finding: Finding{File: "a.go", Line: 10, Message: "m1"}},
		{Finding: Finding{File: "b.go", Line: 20, Message: "m2"}},
	}
	cases := []struct {
		name     string
		rows     []types.Fact
		want     int
		wantFile string
	}{
		{name: "only the named review counts", rows: []types.Fact{{Predicate: "review_finding_refuted", Args: []any{types.MangleString("rev-1"), types.MangleString("a.go"), int64(10)}}, {Predicate: "review_finding_refuted", Args: []any{types.MangleString("other"), types.MangleString("b.go"), int64(20)}}}, want: 1, wantFile: "a.go"},
		{name: "malformed rows are ignored", rows: []types.Fact{{Predicate: "review_finding_refuted", Args: []any{types.MangleString("rev-1")}}}, want: 0},
		{name: "no rows refute nothing", rows: nil, want: 0},
		{name: "wrong line does not join", rows: []types.Fact{{Predicate: "review_finding_refuted", Args: []any{types.MangleString("rev-1"), types.MangleString("a.go"), int64(11)}}}, want: 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Refuted(rowsQuerier{"review_finding_refuted": tc.rows}, "rev-1", verdicts)
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != tc.want {
				t.Fatalf("refuted = %v, want %d", got, tc.want)
			}
			if tc.want == 1 && got[0].File != tc.wantFile {
				t.Fatalf("refuted file = %s, want %s", got[0].File, tc.wantFile)
			}
		})
	}
	if _, err := Refuted(queryFailer{}, "rev-1", verdicts); err == nil {
		t.Fatal("query error must be an error")
	}
}

func TestResult_NilIsNoEvidence(t *testing.T) {
	var r *Result
	if got := r.Facts(); got != nil {
		t.Fatalf("nil facts = %v, want nil", got)
	}
	if got := r.Summary(); got != "no judgments" {
		t.Fatalf("nil summary = %q, want no judgments", got)
	}
	extra := &Result{Judgments: map[string]float64{ChangesFiles: 0.9}, About: map[string]string{ChangesFiles: "s", "answered_9": "stray"}, Choices: map[string]map[string]float64{}}
	facts := extra.Facts()
	about := 0
	for _, f := range facts {
		if f.Predicate == "jev_judgment_about" {
			about++
			if got := types.ExtractString(f.Args[1]); got != "s" {
				t.Fatalf("about = %q, want s", got)
			}
		}
	}
	if about != 1 {
		t.Fatalf("about facts = %d, want 1: stray About without a judgment must not project", about)
	}
}

func TestInBatches_EmptyAndNonPositiveSize(t *testing.T) {
	cases := []struct {
		name        string
		items       []int
		size        int
		wantBatches int
	}{
		{name: "empty needs no batches", items: nil, size: 2, wantBatches: 0},
		{name: "zero size is one per batch", items: []int{1, 2, 3}, size: 0, wantBatches: 3},
		{name: "negative size is one per batch", items: []int{1, 2}, size: -5, wantBatches: 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := inBatches(tc.items, tc.size, func(b []int) ([]int, error) { return b, nil })
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != tc.wantBatches {
				t.Fatalf("batches = %d, want %d", len(got), tc.wantBatches)
			}
			flat := []int{}
			for _, b := range got {
				flat = append(flat, b...)
			}
			if len(flat) != len(tc.items) {
				t.Fatalf("items = %v, want %v in order", flat, tc.items)
			}
			for i := range flat {
				if flat[i] != tc.items[i] {
					t.Fatalf("items = %v, want %v in order", flat, tc.items)
				}
			}
		})
	}
}

func TestFindingFacts_ProjectsFileLineAndPercent(t *testing.T) {
	cases := []struct {
		name string
		prob float64
		want int64
	}{
		{name: "zero", prob: 0, want: 0},
		{name: "rounds half up", prob: 0.125, want: 13},
		{name: "one", prob: 1, want: 100},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			facts := FindingFacts("rev-7", []FindingVerdict{{Finding: Finding{File: "x.go", Line: 4, Message: "m"}, Probability: tc.prob}})
			if len(facts) != 1 {
				t.Fatalf("facts = %v", facts)
			}
			f := facts[0]
			if f.Predicate != "review_finding_check" {
				t.Fatalf("predicate = %s", f.Predicate)
			}
			if got := types.ExtractString(f.Args[0]); got != "rev-7" {
				t.Fatalf("review = %q", got)
			}
			if got := types.ExtractString(f.Args[1]); got != "x.go" {
				t.Fatalf("file = %q", got)
			}
			if line, _ := types.ExtractInt64(f.Args[2]); line != 4 {
				t.Fatalf("line = %d, want 4", line)
			}
			if pct, _ := types.ExtractInt64(f.Args[3]); pct != tc.want {
				t.Fatalf("percent = %d, want %d", pct, tc.want)
			}
		})
	}
	if got := FindingFacts("rev-7", nil); len(got) != 0 {
		t.Fatalf("no verdicts must project no facts, got %v", got)
	}
}
