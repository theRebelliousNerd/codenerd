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
		// A ```mangle-rejected fence is run on the engine and must be refused
		// (TestMangleAtoms_FencesMatchEngine), so it may show a rejected form.
		visible := stripFences(atom.Content, mangleRejectedFenceTg)
		for _, loc := range typedDecl.FindAllStringIndex(visible, -1) {
			lead := visible[max(0, loc[0]-240):loc[0]]
			if !strings.Contains(strings.ToUpper(lead), "WRONG") {
				t.Errorf("atom %s teaches the Decl X.Type<T> form, which the pinned engine does not parse: ...%s...",
					atom.ID, visible[loc[0]:min(len(visible), loc[1]+30)])
			}
		}
	}
	for _, atom := range corpus.All() {
		content := stripFences(atom.Content, mangleRejectedFenceTg)
		for _, r := range rejected {
			if loc := r.form.FindStringIndex(content); loc != nil {
				end := loc[1] + 40
				if end > len(content) {
					end = len(content)
				}
				t.Errorf("atom %s teaches a reducer form the pinned engine rejects (%s): ...%s...",
					atom.ID, r.fix, content[loc[0]:end])
			}
		}
	}
}

// A wildcard inside a negated atom is never correct Mangle. On the v0.5.1 line
// the negation was deleted from the clause with no error, so
// `blocked(X) :- candidate_action(X), !permitted(X, _, _).` blocked every action;
// on the pinned v0.7 analysis refuses it ("variable X0 is not bound"). The
// legislator atom, which teaches codeNERD's agents to write permission policy,
// once presented the form as CORRECT. An atom may show the form only on a line
// that says it is wrong, or inside a ```mangle-rejected fence the engine refuses;
// the working idiom is a projection:
// `has_permit(X) :- permitted(X, _, _).` then `!has_permit(X)`.
func TestEmbeddedCorpus_TeachesNoWildcardNegation(t *testing.T) {
	corpus, err := LoadEmbeddedCorpus()
	if err != nil {
		t.Fatalf("LoadEmbeddedCorpus: %v", err)
	}
	wildcardNegation := regexp.MustCompile(`![a-z_][a-z_0-9:]*\(\s*_\s*[,)]|![a-z_][a-z_0-9:]*\([^)]*[, ]_\s*[,)]`)
	explains := regexp.MustCompile(`(?i)wrong|deleted|evaluates as|silently|rejected|never reported`)
	for _, atom := range corpus.All() {
		lines := strings.Split(stripFences(atom.Content, mangleRejectedFenceTg), "\n")
		for i, line := range lines {
			if !wildcardNegation.MatchString(line) {
				continue
			}
			context := line
			if i > 0 {
				context = lines[i-1] + "\n" + line
			}
			if !explains.MatchString(context) {
				t.Errorf("atom %s teaches a wildcard inside a negated atom, which the pinned engine refuses: %q (project first: has_x(X) :- x(X, _). then !has_x(X))",
					atom.ID, strings.TrimSpace(line))
			}
		}
	}
}
