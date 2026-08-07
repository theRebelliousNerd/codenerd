// This file lives in the external test package on purpose. It needs the
// canonical verb list from internal/perception, and perception ->
// articulation -> prompt is a real import chain, so an in-package test would be
// an import cycle.
package prompt_test

import (
	"context"
	"sort"
	"strings"
	"testing"

	"codenerd/internal/perception"
	"codenerd/internal/prompt"
)

// The config-atom provider and the intent taxonomy are two hand-maintained
// lists of the same verbs, written in different packages by different changes.
// They drifted to 9-of-36 coverage without a single test failing.
//
// What made the drift invisible is that the failure is silent and inverted: an
// unregistered verb produces AllowedTools == nil, buildToolCatalogForPiggyback
// returns "" for an empty tool set, the executor logs "no tools configured" at
// DEBUG, and the model -- given no tools and a prompt telling it never to invent
// facts -- correctly answers "let me read the file first" and stops. The turn
// exits 0. Live: `nerd explain internal/types/mangle_scale.go` returned
// "Locating ... reading the file and its code structure now..." and nothing else.
func TestConfigAtoms_EveryTaxonomyVerbHasTools(t *testing.T) {
	provider := prompt.NewDefaultConfigAtomProvider()

	var missing []string
	var empty []string
	for _, def := range perception.DefaultTaxonomyData {
		atom, ok := provider.GetAtom(def.Verb)
		if !ok {
			missing = append(missing, def.Verb+" (shard "+def.ShardType+")")
			continue
		}
		if len(atom.Tools) == 0 {
			empty = append(empty, def.Verb)
		}
	}
	sort.Strings(missing)
	sort.Strings(empty)

	if len(missing) > 0 {
		t.Errorf("%d canonical taxonomy verb(s) have no config atom, so they run with zero tools:\n  %s",
			len(missing), strings.Join(missing, "\n  "))
	}
	if len(empty) > 0 {
		t.Errorf("%d verb(s) have a config atom with an empty tool set:\n  %s",
			len(empty), strings.Join(empty, "\n  "))
	}
}

// Every verb must be able to look at the codebase before describing it. This is
// the specific capability whose absence produced the stub answer.
func TestConfigAtoms_EveryTaxonomyVerbCanReadFiles(t *testing.T) {
	provider := prompt.NewDefaultConfigAtomProvider()

	var blind []string
	for _, def := range perception.DefaultTaxonomyData {
		atom, ok := provider.GetAtom(def.Verb)
		if !ok {
			continue // reported by the test above
		}
		hasRead := false
		for _, tool := range atom.Tools {
			if tool == "read_file" {
				hasRead = true
				break
			}
		}
		if !hasRead {
			blind = append(blind, def.Verb)
		}
	}
	sort.Strings(blind)
	if len(blind) > 0 {
		t.Errorf("%d verb(s) cannot read a file, so any grounded answer is impossible:\n  %s",
			len(blind), strings.Join(blind, "\n  "))
	}
}

// A verb the taxonomy routes to a persona must get that persona's tools, not
// core read-only tools. /git landing on the /none tier would mean `nerd git
// commit` silently having no git_operation tool.
func TestConfigAtoms_TaxonomyShardTypeDeterminesToolTier(t *testing.T) {
	provider := prompt.NewDefaultConfigAtomProvider()

	// One tool that is unique to each persona tier.
	sentinel := map[string]string{
		"/coder":          "edit_file",
		"/tester":         "run_tests",
		"/reviewer":       "git_diff",
		"/researcher":     "web_search",
		"/tool_generator": "run_build",
	}

	for _, def := range perception.DefaultTaxonomyData {
		want, tiered := sentinel[def.ShardType]
		if !tiered {
			continue // /none verbs are on the core tier by design
		}
		atom, ok := provider.GetAtom(def.Verb)
		if !ok {
			continue // reported by the coverage test
		}
		found := false
		for _, tool := range atom.Tools {
			if tool == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("%s is declared ShardType %s but its config atom lacks %q, so it is on the wrong tool tier",
				def.Verb, def.ShardType, want)
		}
	}
}

// An unknown verb must degrade to read-only tools, never to nothing. Before
// this, only "/consult/*" got the fallback and every other unregistered intent
// produced a nil tool set plus a WARN the caller ignored.
func TestConfigFactory_UnknownIntentFallsBackToGeneral(t *testing.T) {
	f := prompt.NewConfigFactory(prompt.NewDefaultConfigAtomProvider())

	cfg, err := f.Generate(context.Background(), &prompt.CompilationResult{Prompt: "identity"}, "/verb_invented_next_quarter")
	if err != nil {
		t.Fatalf("an unregistered intent must degrade, not fail: %v", err)
	}
	if len(cfg.AllowedTools) == 0 {
		t.Fatal("fallback produced an empty tool set; the agent would answer with no way to read anything")
	}
}
