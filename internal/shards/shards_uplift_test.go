package shards

import (
	"strings"
	"testing"

	"codenerd/internal/core"
)

// The additive match weights sum past 1.0 (0.5 + 0.3 + 0.4 + 0.3), but the
// SpecialistMatch contract is 0.0-1.0: the kernel scales scores onto 0-100
// for specialist_match/3, and an unclamped 1.5 would assert confidence 150.
func TestCalculateMatchScore_CappedAtOne(t *testing.T) {
	tech := TechnologyPattern{
		ShardName:    "mangle",
		FilePatterns: []string{".mg"},
		ImportHints:  []string{"mangle:"},
		ContentHints: []string{"decl", "query"},
	}
	score, matched := calculateMatchScore(
		"mangle: something", "policy/shards.mg", "decl x query y", ".mg", tech)
	if !matched {
		t.Fatal("expected a match")
	}
	if score != 1.0 {
		t.Errorf("all-weights score = %v, want exactly 1.0", score)
	}
}

// MatchFacts clamps defensively: scores arrive in a struct anyone can fill,
// and the rules compare against 0-100, so out-of-range values assert a
// confidence the scale cannot mean.
func TestMatchFacts_ClampsConfidence(t *testing.T) {
	facts := MatchFacts("task", TaskComplexityNormal, []SpecialistMatch{
		{AgentName: "Coder", Score: 1.5},
		{AgentName: "Tester", Score: -0.2},
		{AgentName: "Reviewer", Score: 0.755},
	})
	got := map[core.MangleAtom]int64{}
	for _, f := range facts {
		if f.Predicate == "specialist_match" {
			got[f.Args[0].(core.MangleAtom)] = f.Args[2].(int64)
		}
	}
	if got["/coder"] != 100 {
		t.Errorf("1.5 -> %d, want 100", got["/coder"])
	}
	if got["/tester"] != 0 {
		t.Errorf("-0.2 -> %d, want 0", got["/tester"])
	}
	if got["/reviewer"] != 76 {
		t.Errorf("0.755 -> %d, want 76 (rounded)", got["/reviewer"])
	}
}

// The advisor list feeds user-visible delegation output: map order would
// shuffle the consultation phase from run to run.
func TestGetStrategicAdvisorsFor_Sorted(t *testing.T) {
	for i := 0; i < 5; i++ {
		advisors := GetStrategicAdvisorsFor("coder")
		for j := 1; j < len(advisors); j++ {
			if advisors[j-1] > advisors[j] {
				t.Fatalf("advisors not sorted: %v", advisors)
			}
		}
	}
}

// CONFIDENCE arrives as 0-100 per the prompt, but models also write ratios
// and out-of-range values: the first must not parse as zero and the second
// must not escape the documented 0-1 scale.
func TestParseConfidence_Matrix(t *testing.T) {
	cases := []struct {
		in   string
		want float64
		ok   bool
	}{
		{"85", 0.85, true},
		{"85%", 0.85, true},
		{"0.85", 0.85, true},
		{"100", 1.0, true},
		{"150", 1.0, true},
		{"-20", 0.0, true},
		{"high", 0, false},
		{"", 0, false},
	}
	for _, c := range cases {
		got, ok := parseConfidence(c.in)
		if got != c.want || ok != c.ok {
			t.Errorf("parseConfidence(%q) = (%v, %v), want (%v, %v)", c.in, got, ok, c.want, c.ok)
		}
	}
}

// The cache key must separate what the response depends on: distinct
// questions sharing a 100-char prefix, and one question under two contexts,
// previously collided and served each other's advice.
func TestCacheKey_SeparatesQuestionAndContext(t *testing.T) {
	m := NewConsultationManager(nil)
	prefix := strings.Repeat("x", 120)
	a := m.cacheKey("adv", prefix+"-one", "ctx")
	b := m.cacheKey("adv", prefix+"-two", "ctx")
	if a == b {
		t.Error("questions differing past char 100 share a cache key")
	}
	c := m.cacheKey("adv", "same question", "context one")
	d := m.cacheKey("adv", "same question", "context two")
	if c == d {
		t.Error("same question under different contexts shares a cache key")
	}
}

// finalizeMatches sorts score-descending with a name tiebreak: scores are
// quantized weight sums, so ties are common, and the input map iterates
// randomly — without the tiebreak, truncation at MaxSpecialists would keep a
// different specialist from run to run.
func TestFinalizeMatches_TiebreakDeterministic(t *testing.T) {
	mk := func(name string, score float64) *SpecialistMatch {
		return &SpecialistMatch{AgentName: name, Score: score}
	}
	in := map[string]*SpecialistMatch{
		"b": mk("b", 0.9),
		"a": mk("a", 0.9),
		"c": mk("c", 0.7),
		"d": mk("d", 0.9),
	}
	for i := 0; i < 10; i++ {
		got := finalizeMatches(in, 2)
		if len(got) != 2 || got[0].AgentName != "a" || got[1].AgentName != "b" {
			t.Fatalf("run %d: got %v, want [a b]", i, got)
		}
	}
}

// The assessment Score documents 0-100: a model writing SCORE: 150 or
// SCORE: -20 must be clamped onto the scale, not stored off it.
func TestParseAssessment_ScoreClamped(t *testing.T) {
	m := NewBackgroundObserverManager(nil)
	event := ObserverEvent{Type: EventTaskCompleted, Target: "x"}
	if got := m.parseAssessment("obs", event, "SCORE: 150").Score; got != 100 {
		t.Errorf("150 -> %d, want 100", got)
	}
	if got := m.parseAssessment("obs", event, "SCORE: -20").Score; got != 0 {
		t.Errorf("-20 -> %d, want 0", got)
	}
	if got := m.parseAssessment("obs", event, "SCORE: 75").Score; got != 75 {
		t.Errorf("75 -> %d, want 75", got)
	}
}

// GetActiveObservers reads a map registry: the list must come out sorted,
// not in iteration order.
func TestGetActiveObservers_Sorted(t *testing.T) {
	m := NewBackgroundObserverManager(nil)
	for _, name := range []string{"zebra", "alpha", "northstar"} {
		m.observers[name] = &ObserverState{Name: name, Active: true}
	}
	m.observers["sleepy"] = &ObserverState{Name: "sleepy", Active: false}
	for i := 0; i < 5; i++ {
		got := m.GetActiveObservers()
		if len(got) != 3 || got[0] != "alpha" || got[1] != "northstar" || got[2] != "zebra" {
			t.Fatalf("got %v, want [alpha northstar zebra]", got)
		}
	}
}
