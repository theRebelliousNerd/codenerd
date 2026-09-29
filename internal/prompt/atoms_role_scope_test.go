package prompt

import (
	"regexp"
	"testing"
)

var roleDeclarationRe = regexp.MustCompile(`(?m)^\s*You are (an?|the) `)

// atomHasRegimeScope reports whether the atom is fenced by at least one
// dimension jit_compiler.mg treats as a regime: an atom tagged on a regime
// dimension is blocked unless the compile context matches it, even when the
// context has no value for that dimension at all. Languages, frameworks,
// intent verbs and world states are NOT regimes — they only block when the
// context carries a different value — so they do not contain a persona.
func atomHasRegimeScope(a *PromptAtom) bool {
	return len(a.ShardTypes) > 0 ||
		len(a.OperationalModes) > 0 ||
		len(a.CampaignPhases) > 0 ||
		len(a.BuildLayers) > 0 ||
		len(a.InitPhases) > 0 ||
		len(a.NorthstarPhases) > 0 ||
		len(a.OuroborosStages) > 0 ||
		len(a.Providers) > 0 ||
		len(a.Models) > 0
}

// TestAtomCorpus_RoleDeclarationsAreScoped pins that an atom which tells the
// model who it is ("You are the ...") reaches only the agent it describes.
//
// eval/judge/task_evaluator ("You are an expert evaluator for an AI coding
// agent") and autopoiesis/meta/atom_generator were mandatory with no selector
// at all, so both were compiled into 150 of 150 recorded prompts: every coder,
// tester and reviewer turn carried a second identity contradicting its own,
// plus about 1,000 tokens instructing it how to grade tasks and author prompt
// atoms. Nothing consumed either atom on purpose; the judge reads a Go const.
func TestAtomCorpus_RoleDeclarationsAreScoped(t *testing.T) {
	corpus, err := LoadEmbeddedCorpus()
	if err != nil {
		t.Fatalf("LoadEmbeddedCorpus: %v", err)
	}
	atoms := corpus.All()
	if len(atoms) < 500 {
		t.Fatalf("embedded corpus has %d atoms; expected the full built-in library", len(atoms))
	}

	declaring := 0
	for _, a := range atoms {
		if !roleDeclarationRe.MatchString(a.Content) {
			continue
		}
		declaring++
		if !atomHasRegimeScope(a) {
			t.Errorf("%s declares a role (%q) but has no regime selector, so it is compiled into every agent's prompt",
				a.ID, roleDeclarationRe.FindString(a.Content))
		}
	}
	// The corpus has dozens of personas; zero matches means the pattern or the
	// loader broke, not that the corpus is clean.
	if declaring < 20 {
		t.Fatalf("found only %d role-declaring atoms; the detection no longer sees the corpus personas", declaring)
	}
}

// TestAtomCorpus_UnscopedMandatoryAtomsAreTheSharedSubstrate bounds the set of
// atoms sent to every agent on every call. Each entry costs its tokens times
// every request the system makes, so joining this list is a decision, not a
// default: an atom belongs here only if every agent is wrong without it.
func TestAtomCorpus_UnscopedMandatoryAtomsAreTheSharedSubstrate(t *testing.T) {
	sharedSubstrate := map[string]bool{
		"identity/base/core":                    true,
		"safety/constitution/prime_directive":   true,
		"safety/constitution/forbidden_actions": true,
		"safety/constitution/dangerous_actions": true,
		"safety/constitutional/core":            true,
		"safety/constitutional/permissions":     true,
		"protocol/piggyback/envelope":           true,
		"protocol/piggyback/thought_first":      true,
		"protocol/piggyback/mangle_updates":     true,
		"protocol/reasoning/requirements":       true,
		"protocol/reasoning/format":             true,
		"methodology/ooda/core":                 true,
		"capability/tool_thinking":              true,
		"capability/codedom_safety":             true,
	}

	corpus, err := LoadEmbeddedCorpus()
	if err != nil {
		t.Fatalf("LoadEmbeddedCorpus: %v", err)
	}
	seen := make(map[string]bool, len(sharedSubstrate))
	for _, a := range corpus.All() {
		if !a.IsMandatory {
			continue
		}
		unscoped := !atomHasRegimeScope(a) &&
			len(a.IntentVerbs) == 0 && len(a.Languages) == 0 && len(a.Frameworks) == 0 &&
			len(a.WorldStates) == 0 && len(a.RequiresTools) == 0
		if !unscoped {
			continue
		}
		seen[a.ID] = true
		if !sharedSubstrate[a.ID] {
			t.Errorf("%s is mandatory with no selector at all (~%d tokens on every request system-wide); scope it, or add it to the shared substrate deliberately",
				a.ID, a.TokenCount)
		}
	}
	for id := range sharedSubstrate {
		if !seen[id] {
			t.Errorf("%s is listed as shared substrate but is no longer an unscoped mandatory atom; remove it from the list", id)
		}
	}
}
