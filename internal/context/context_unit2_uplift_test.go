package context

import (
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/types"
)

// Scoring must be idempotent: the graph rebuild used to re-add last call's
// derived edges and derive them again, so dependency scores grew every call.
func TestActivation_ScoringIsIdempotent(t *testing.T) {
	ae := NewActivationEngine(DefaultConfig())
	facts := []core.Fact{
		{Predicate: "dependency_link", Args: []any{"a.go", "b.go", "import"}},
		{Predicate: "symbol_graph", Args: []any{"Sym", "func", "pub", "a.go", "sig"}},
		{Predicate: "file_topology", Args: []any{"a.go", "h", "/go", int64(1), "/false"}},
	}
	first := ae.ScoreFacts(facts, nil)
	totals := func(ss []ScoredFact) []float64 {
		out := make([]float64, len(ss))
		for i, s := range ss {
			out[i] = s.Score*1000 + s.DependencyScore
		}
		return out
	}
	want := totals(first)
	for i := 0; i < 5; i++ {
		got := totals(ae.ScoreFacts(facts, nil))
		for j := range want {
			if got[j] != want[j] {
				t.Fatalf("score drift on call %d: first=%v now=%v", i+2, want, got)
			}
		}
	}
	if n := len(ae.reverseDependencies["b.go"]); n != 1 {
		t.Errorf("reverseDependencies[b.go] has %d entries after 6 scores, want 1", n)
	}
}

// Explicit edges survive the rebuild exactly once, however often they are
// recorded or the engine scores.
func TestActivation_ExplicitEdgesSurviveAndDedup(t *testing.T) {
	ae := NewActivationEngine(DefaultConfig())
	a := core.Fact{Predicate: "p", Args: []any{"a"}}
	b := core.Fact{Predicate: "p", Args: []any{"b"}}
	ae.AddDependency(a, b)
	ae.AddDependency(a, b)
	facts := []core.Fact{a, b}
	before := ae.ScoreFacts(facts, nil)
	after := ae.ScoreFacts(facts, nil)
	for i := range before {
		if before[i].Score != after[i].Score || before[i].DependencyScore != after[i].DependencyScore {
			t.Fatalf("explicit edge scores drifted: %+v vs %+v", before, after)
		}
	}
	var depScore float64
	for _, s := range after {
		if s.Fact.Args[0] == "b" {
			depScore = s.DependencyScore
		}
	}
	if depScore != 5.0 {
		t.Errorf("dependent's dependency score = %v, want exactly 5.0 (one dependent)", depScore)
	}
}

// GetState/SetState must not alias engine memory in either direction, and
// ClearState must clear every context — including the back-reference one it
// used to leave behind.
func TestActivation_StateIsolationAndClear(t *testing.T) {
	ae := NewActivationEngine(DefaultConfig())
	in := ActivationState{
		ActiveIntent:   &core.Fact{Predicate: "user_intent", Args: []any{"a", "b", "/fix", "t"}},
		FocusedPaths:   []string{"main.go"},
		FocusedSymbols: []string{"Sym"},
		RecentFacts:    []core.Fact{{Predicate: "p", Args: []any{"x"}}},
	}
	ae.SetState(in)
	in.FocusedPaths[0] = "mutated.go"
	in.ActiveIntent.Args[2] = "mutated"
	in.RecentFacts[0].Args[0] = "mutated"
	got := ae.GetState()
	if got.FocusedPaths[0] != "main.go" || got.ActiveIntent.Args[2] != "/fix" || got.RecentFacts[0].Args[0] != "x" {
		t.Fatalf("SetState aliased caller memory: %+v", got)
	}
	got.FocusedPaths[0] = "mutated.go"
	got.ActiveIntent.Args[0] = "mutated"
	again := ae.GetState()
	if again.FocusedPaths[0] != "main.go" || again.ActiveIntent.Args[0] != "a" {
		t.Fatalf("GetState aliased engine memory: %+v", again)
	}

	ae.SetBackReferenceContext(&BackReferenceActivationContext{ReferencedTurnIDs: []int{1}})
	ae.SetCampaignContext(&CampaignActivationContext{CampaignID: "c"})
	ae.ClearState()
	if ae.backReferenceContext != nil {
		t.Error("ClearState left the back-reference context installed")
	}
	if ae.campaignContext != nil || ae.issueContext != nil {
		t.Error("ClearState left a campaign/issue context installed")
	}
	if len(ae.explicitDeps) != 0 || len(ae.explicitRevDeps) != 0 {
		t.Error("ClearState left explicit dependency edges installed")
	}
}

// Equal scores order by fact string, independent of input order: the context
// block must not flap when nothing changed.
func TestActivation_SortTiesAreDeterministic(t *testing.T) {
	mk := func() []core.Fact {
		return []core.Fact{
			{Predicate: "zz_custom", Args: []any{"c"}},
			{Predicate: "zz_custom", Args: []any{"a"}},
			{Predicate: "zz_custom", Args: []any{"b"}},
		}
	}
	ae := NewActivationEngine(DefaultConfig())
	first := ae.ScoreFacts(mk(), nil)
	for _, s := range first {
		if s.Score != 50.0 {
			t.Fatalf("fixture score = %v, want the 50.0 default (test setup broken)", s.Score)
		}
	}
	order := []string{
		first[0].Fact.Args[0].(string),
		first[1].Fact.Args[0].(string),
		first[2].Fact.Args[0].(string),
	}
	if order[0] != "a" || order[1] != "b" || order[2] != "c" {
		t.Fatalf("tie order = %v, want [a b c]", order)
	}
	rev := []core.Fact{mk()[0], mk()[2], mk()[1]}
	second := NewActivationEngine(DefaultConfig()).ScoreFacts(rev, nil)
	for i := range first {
		if first[i].Fact.String() != second[i].Fact.String() {
			t.Fatalf("order depends on input order: %v vs %v", first, second)
		}
	}
}

// A Go-constructed intent carrying MangleAtom args scores exactly like the
// same intent with plain strings: the verb boosts and target match must not
// depend on which Go type carried the text.
func TestActivation_MangleAtomIntentScoresLikeStrings(t *testing.T) {
	facts := []core.Fact{
		{Predicate: "diagnostic", Args: []any{"/error", "auth.go", int64(1), "E1", "boom"}},
		{Predicate: "file_topology", Args: []any{"auth.go", "h", "/go", int64(1), "/false"}},
	}
	strIntent := &core.Fact{Predicate: "user_intent", Args: []any{"id", "/query", "/fix", "auth.go", ""}}
	atomIntent := &core.Fact{Predicate: "user_intent", Args: []any{
		types.MangleAtom("/current_intent"), types.MangleAtom("/query"), types.MangleAtom("/fix"), "auth.go", "",
	}}
	strScores := NewActivationEngine(DefaultConfig()).ScoreFacts(facts, strIntent)
	atomScores := NewActivationEngine(DefaultConfig()).ScoreFacts(facts, atomIntent)
	if len(strScores) != len(atomScores) {
		t.Fatalf("lengths differ: %d vs %d", len(strScores), len(atomScores))
	}
	for i := range strScores {
		if strScores[i].RelevanceScore != atomScores[i].RelevanceScore {
			t.Errorf("fact %d relevance: strings=%v atoms=%v; want equal",
				i, strScores[i].RelevanceScore, atomScores[i].RelevanceScore)
		}
	}
	if atomScores[0].RelevanceScore <= 0 {
		t.Error("atom intent earned no relevance; verb boosts did not fire")
	}
}

// The kernel-override path substitutes kernel scores, keeps the heuristic for
// uncovered facts, and sorts like ScoreFacts instead of returning input order.
func TestScoreFactsWithKernelOverride_SortedAndMixed(t *testing.T) {
	ae := NewActivationEngine(DefaultConfig())
	low := core.Fact{Predicate: "zz_custom", Args: []any{"low"}}
	high := core.Fact{Predicate: "zz_custom", Args: []any{"high"}}
	facts := []core.Fact{low, high}
	out := ae.ScoreFactsWithKernelOverride(facts, nil, map[string]float64{factKey(high): 95.0})
	if len(out) != 2 {
		t.Fatalf("got %d facts, want 2", len(out))
	}
	if out[0].Fact.Args[0] != "high" || out[0].Score != 95.0 {
		t.Errorf("first = %v @ %v, want high @ 95.0 (sorted, kernel score kept)", out[0].Fact, out[0].Score)
	}
	if out[1].Fact.Args[0] != "low" || out[1].Score != 50.0 {
		t.Errorf("second = %v @ %v, want low @ 50.0 (heuristic fallback)", out[1].Fact, out[1].Score)
	}
	plain := ae.ScoreFactsWithKernelOverride(facts, nil, nil)
	if len(plain) != 2 || plain[0].Score < plain[1].Score {
		t.Errorf("nil kernel scores must behave like ScoreFacts (sorted): %+v", plain)
	}
}
