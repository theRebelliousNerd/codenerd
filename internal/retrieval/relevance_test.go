package retrieval

import (
	"context"
	"errors"
	"testing"
)

type fakeRelevance struct {
	probs map[string]float64
	err   error
	saw   []RelevanceCandidate
}

func (f *fakeRelevance) Relevance(_ context.Context, _ string, files []RelevanceCandidate) (map[string]float64, error) {
	f.saw = files
	if f.err != nil {
		return nil, f.err
	}
	return f.probs, nil
}

func candidates(paths ...string) []CandidateFile {
	out := make([]CandidateFile, len(paths))
	for i, p := range paths {
		out[i] = CandidateFile{FilePath: p, RelevanceScore: float64(len(paths) - i)}
	}
	return out
}

// The judge reorders the keyword pool and leaves out what it calls
// irrelevant; a file already added (a mention) is never judged.
func TestJudgeTier2_ReordersAndDropsNoise(t *testing.T) {
	judge := &fakeRelevance{probs: map[string]float64{"a.go": 0.2, "b.go": 0.95, "c.go": 0.6}}
	b := &TieredContextBuilder{workDir: t.TempDir(), relevance: judge, relevanceDrop: 0.3}
	got, judged := b.judgeTier2(context.Background(), "fix the retry policy", candidates("mentioned.go", "a.go", "b.go", "c.go"), map[string]bool{"mentioned.go": true})
	if len(judge.saw) != 3 {
		t.Fatalf("judged %d files, want 3 (the mention is not judged)", len(judge.saw))
	}
	if len(got) != 2 || got[0].FilePath != "b.go" || got[1].FilePath != "c.go" {
		t.Fatalf("selection = %v, want b.go then c.go with a.go dropped", got)
	}
	var dropped bool
	for _, j := range judged {
		if j.FilePath == "a.go" && !j.Kept {
			dropped = true
		}
	}
	if !dropped {
		t.Fatalf("a.go's drop is not recorded: %+v", judged)
	}
}

// A failed judgment is no evidence: the keyword ranking stands untouched.
func TestJudgeTier2_FailureKeepsKeywordRanking(t *testing.T) {
	b := &TieredContextBuilder{workDir: t.TempDir(), relevance: &fakeRelevance{err: errors.New("jev down")}, relevanceDrop: 0.3}
	ranked := candidates("a.go", "b.go")
	got, judged := b.judgeTier2(context.Background(), "x", ranked, map[string]bool{})
	if len(got) != 2 || got[0].FilePath != "a.go" || judged != nil {
		t.Fatalf("a failed judgment changed the selection: %v %v", got, judged)
	}
}

// The judgment reaches the kernel as context_relevance/3.
func TestTieredContextFacts_EmitsRelevance(t *testing.T) {
	tc := &TieredContext{Judged: []JudgedFile{{FilePath: "b.go", Probability: 0.95, Kept: true}, {FilePath: "a.go", Probability: 0.2}}}
	var n int
	for _, f := range TieredContextFacts("issue", tc, "") {
		if f.Predicate == "context_relevance" {
			n++
		}
	}
	if n != 2 {
		t.Fatalf("context_relevance facts = %d, want 2", n)
	}
}

// A request in plain English searches for its domain terms.
func TestExtractKeywords_PlainEnglishHasDomainTerms(t *testing.T) {
	cases := map[string][]string{
		"why does the approval broker open a second request for a duplicate call": {"approval", "broker", "duplicate", "request"},
		"the footer does not wrap in the TUI":                                     {"footer", "wrap", "TUI"},
		"renderFooter ignores work_mode":                                          {"renderFooter", "work_mode", "ignores"},
	}
	for text, want := range cases {
		got := map[string]bool{}
		for _, k := range ExtractKeywords(text).AllKeywords() {
			got[k] = true
		}
		for _, w := range want {
			if !got[w] {
				t.Errorf("%q: missing keyword %q (got %v)", text, w, ExtractKeywords(text).AllKeywords())
			}
		}
		for _, stop := range []string{"why", "does", "the", "for"} {
			if got[stop] {
				t.Errorf("%q: stopword %q became a keyword", text, stop)
			}
		}
	}
}
