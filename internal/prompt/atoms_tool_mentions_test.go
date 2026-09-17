package prompt

import (
	"regexp"
	"sort"
	"strings"
	"testing"
)

// inheritedRequiredTools returns the tools an atom requires, directly or
// through its depends_on chain. The resolver prunes dependents of a blocked
// atom transitively, so a requirement on an ancestor gates the descendant too.
func inheritedRequiredTools(a *PromptAtom, byID map[string]*PromptAtom, seen map[string]bool) map[string]bool {
	req := make(map[string]bool)
	for _, t := range a.RequiresTools {
		req[t] = true
	}
	for _, dep := range a.DependsOn {
		if seen[dep] {
			continue
		}
		seen[dep] = true
		if parent, ok := byID[dep]; ok {
			for t := range inheritedRequiredTools(parent, byID, seen) {
				req[t] = true
			}
		}
	}
	return req
}

// TestAtomCorpus_OptionalToolsAreNamedOnlyByAtomsThatRequireThem pins the
// capability-gating contract from the atom side.
//
// A tool that any atom lists in requires_tools is an optional-envelope tool:
// it is present in some catalogs and absent from others. An atom that names
// such a tool without requiring it is delivered on envelopes where the tool
// does not exist, and teaches the model to call something it was not given.
//
// Measured before this test existed: five mandatory CodeDOM atoms told every
// coder to run `get_impacted_tests` / `run_impacted_tests` - tools required
// only by capability/codedom_test_tools and registered nowhere in the coder's
// catalog - while capability/tool_steering, in the same prompt, said "Do NOT
// invent tools that don't exist".
func TestAtomCorpus_OptionalToolsAreNamedOnlyByAtomsThatRequireThem(t *testing.T) {
	corpus, err := LoadEmbeddedCorpus()
	if err != nil {
		t.Fatalf("LoadEmbeddedCorpus: %v", err)
	}
	atoms := corpus.All()
	byID := make(map[string]*PromptAtom, len(atoms))
	optional := make(map[string]bool)
	for _, a := range atoms {
		byID[a.ID] = a
		for _, tool := range a.RequiresTools {
			optional[tool] = true
		}
	}
	// The vocabulary is derived from the corpus; if it collapses the test
	// would pass by checking nothing.
	if len(optional) < 5 {
		t.Fatalf("only %d tools appear in any requires_tools list; expected the CodeDOM bundle at least", len(optional))
	}

	mention := make(map[string]*regexp.Regexp, len(optional))
	for tool := range optional {
		mention[tool] = regexp.MustCompile("`" + regexp.QuoteMeta(tool) + "`")
	}

	checked := 0
	for _, a := range atoms {
		text := a.Content + "\n" + a.ContentConcise + "\n" + a.ContentMin
		req := inheritedRequiredTools(a, byID, map[string]bool{a.ID: true})
		var missing []string
		for tool, re := range mention {
			if re.MatchString(text) {
				checked++
				if !req[tool] {
					missing = append(missing, tool)
				}
			}
		}
		if len(missing) > 0 {
			sort.Strings(missing)
			t.Errorf("%s names optional tool(s) %s without requiring them (requires_tools, directly or via depends_on): it is delivered on catalogs where they do not exist",
				a.ID, strings.Join(missing, ", "))
		}
	}
	if checked < 20 {
		t.Fatalf("matched only %d tool mentions across the corpus; the mention detection no longer sees the CodeDOM guidance", checked)
	}
}
