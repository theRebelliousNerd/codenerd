package prompt

import (
	"regexp"
	"strings"
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
		// The capitalized reducers are not reducers on this engine: the analysis
		// treats them as ordinary functions over an unbound variable.
		{regexp.MustCompile(`fn:(Sum|Count|Max|Min|Avg|Collect|CollectDistinct|FloatSum|FloatMax|FloatMin|CountDistinct|CollectToMap|PickAny)\(`), "reducers are lowercase (fn:sum, fn:count(), fn:collect_distinct, fn:float:sum, ...); there is no fn:CollectToMap or count-distinct"},
	}
	// The Google 0.4.0 typed-variable declaration (Decl p(X.Type<int>)) does not
	// parse on the pinned engine; it may appear only as a marked wrong-way example.
	typedDecl := regexp.MustCompile(`Decl\s+[a-z_:][A-Za-z0-9_]*\s*\([^)]*\.Type<`)
	for _, atom := range corpus.All() {
		for _, loc := range typedDecl.FindAllStringIndex(atom.Content, -1) {
			lead := atom.Content[max(0, loc[0]-240):loc[0]]
			if !strings.Contains(strings.ToUpper(lead), "WRONG") {
				t.Errorf("atom %s teaches the Decl X.Type<T> form, which the pinned engine does not parse: ...%s...",
					atom.ID, atom.Content[loc[0]:min(len(atom.Content), loc[1]+30)])
			}
		}
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
