package prompt

import (
	"regexp"
	"testing"
)

// The corpus teaches codeNERD's own agents to write Mangle, so every reducer
// form it shows must be one the pinned engine accepts. On 2026-09-18 it taught
// three that fail analysis on codeberg.org/TauCeti/mangle-go
// v0.5.1-0.20260413190942-4dcaa582c6d3, on 23 lines of 9 atoms:
//
//	fn:count(X)           fn:count takes no argument        -> fn:count()
//	fn:mean(X)            no such function                  -> fn:avg(X)
//	fn:count_distinct(..) declared in symbols, never wired  -> project, then fn:count()
//
// Each was established by running it through `nerd check-mangle`
// (symbols/symbols.go gives the arities: Count 0, Avg 1, CountDistinct 0 and
// "unknown function" at analysis in every spelling).
func TestEmbeddedCorpus_TeachesOnlyReducersThePinnedEngineAccepts(t *testing.T) {
	corpus, err := LoadEmbeddedCorpus()
	if err != nil {
		t.Fatalf("LoadEmbeddedCorpus: %v", err)
	}
	rejected := []struct {
		form *regexp.Regexp
		fix  string
	}{
		{regexp.MustCompile(`fn:count\([A-Za-z]`), "fn:count() takes no argument"},
		{regexp.MustCompile(`fn:mean\(`), "there is no fn:mean; the reducer is fn:avg(X)"},
		// A sentence that says the reducer does not work may name it; a call may not.
		{regexp.MustCompile(`=\s*fn:count_distinct\(`), "no count-distinct reducer works; project to the distinct columns, then fn:count()"},
	}
	for _, atom := range corpus.All() {
		for _, r := range rejected {
			if loc := r.form.FindStringIndex(atom.Content); loc != nil {
				end := loc[1] + 40
				if end > len(atom.Content) {
					end = len(atom.Content)
				}
				t.Errorf("atom %s teaches a reducer form the pinned engine rejects (%s): ...%s...",
					atom.ID, r.fix, atom.Content[loc[0]:end])
			}
		}
	}
}
