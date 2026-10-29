// This file lives in the external test package on purpose. It needs the
// canonical verb list from internal/perception, and perception ->
// articulation -> prompt is a real import chain, so an in-package test would be
// an import cycle.
package prompt_test

import (
	"context"
	"slices"
	"sort"
	"strings"
	"sync"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/perception"
	"codenerd/internal/prompt"
	"codenerd/internal/types"
)

// taxonomyKernel is one booted kernel for this file. DeriveTurnTools is the
// catalog; the default atoms no longer carry a Tools slice.
var (
	taxonomyKernelOnce sync.Once
	taxonomyKernelInst types.Kernel
	taxonomyKernelErr  error
)

func taxonomyTurnKernel(t *testing.T) types.Kernel {
	t.Helper()
	taxonomyKernelOnce.Do(func() {
		taxonomyKernelInst, taxonomyKernelErr = core.NewRealKernel()
	})
	if taxonomyKernelErr != nil {
		t.Fatalf("NewRealKernel: %v", taxonomyKernelErr)
	}
	return taxonomyKernelInst
}

func taxonomyTools(t *testing.T, verb string) []string {
	t.Helper()
	tools, err := prompt.DeriveTurnTools(taxonomyTurnKernel(t), verb)
	if err != nil {
		t.Fatalf("DeriveTurnTools(%s): %v", verb, err)
	}
	return tools
}

// The config-atom provider and the intent taxonomy are two hand-maintained
// lists of the same verbs, written in different packages by different changes.
// They drifted to 9-of-36 coverage without a single test failing.
//
// What made the drift invisible is that the failure is silent and inverted: an
// unregistered verb produced an empty tool catalog, buildToolCatalogForPiggyback
// returned "" for an empty tool set, the executor logged "no tools configured" at
// DEBUG, and the model -- given no tools and a prompt telling it never to invent
// facts -- correctly answered "let me read the file first" and stopped. The turn
// exited 0. Live: `nerd explain internal/types/mangle_scale.go` returned
// "Locating ... reading the file and its code structure now..." and nothing else.
// The atom still has to exist (it anchors policies). The tools are
// turn_tool_allowed, and an empty derivation is the same disarm.
func TestConfigAtoms_EveryTaxonomyVerbHasTools(t *testing.T) {
	provider := prompt.NewDefaultConfigAtomProvider()

	var missing []string
	var empty []string
	for _, def := range perception.DefaultTaxonomyData {
		if _, ok := provider.GetAtom(def.Verb); !ok {
			missing = append(missing, def.Verb+" (shard "+def.ShardType+")")
			continue
		}
		if len(taxonomyTools(t, def.Verb)) == 0 {
			empty = append(empty, def.Verb)
		}
	}
	sort.Strings(missing)
	sort.Strings(empty)

	if len(missing) > 0 {
		t.Errorf("%d canonical taxonomy verb(s) have no config atom, so they run with no policy anchor:\n  %s",
			len(missing), strings.Join(missing, "\n  "))
	}
	if len(empty) > 0 {
		t.Errorf("%d verb(s) derive an empty turn_tool_allowed envelope:\n  %s",
			len(empty), strings.Join(empty, "\n  "))
	}
}

// Every verb must be able to look at the codebase before describing it. This is
// the specific capability whose absence produced the stub answer.
func TestConfigAtoms_EveryTaxonomyVerbCanReadFiles(t *testing.T) {
	provider := prompt.NewDefaultConfigAtomProvider()

	var blind []string
	for _, def := range perception.DefaultTaxonomyData {
		if _, ok := provider.GetAtom(def.Verb); !ok {
			continue // reported by the test above
		}
		if !slices.Contains(taxonomyTools(t, def.Verb), "read_file") {
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
		if _, ok := provider.GetAtom(def.Verb); !ok {
			continue // reported by the coverage test
		}
		if !slices.Contains(taxonomyTools(t, def.Verb), want) {
			t.Errorf("%s is declared ShardType %s but turn_tool_allowed lacks %q, so it is on the wrong tool tier",
				def.Verb, def.ShardType, want)
		}
	}
}

// An unknown verb must degrade to the read-only floor, never to nothing.
// Generate no longer carries that floor: the default atoms declare policies,
// and DeriveTurnTools is the catalog. Before this, only "/consult/*" got a
// tool fallback and every other unregistered intent produced a nil tool set
// plus a WARN the caller ignored.
func TestConfigFactory_UnknownIntentFallsBackToGeneral(t *testing.T) {
	f := prompt.NewConfigFactory(prompt.NewDefaultConfigAtomProvider())

	cfg, err := f.Generate(context.Background(), &prompt.CompilationResult{Prompt: "identity"}, "/verb_invented_next_quarter")
	if err != nil {
		t.Fatalf("an unregistered intent must degrade, not fail: %v", err)
	}
	if len(cfg.AllowedTools) != 0 {
		t.Fatalf("Generate granted tools %v; the catalog is turn_tool_allowed", cfg.AllowedTools)
	}
	if len(cfg.Policies) == 0 {
		t.Fatal("fallback produced no policies")
	}
	tools := taxonomyTools(t, "/verb_invented_next_quarter")
	if !slices.Contains(tools, "read_file") || slices.Contains(tools, "write_file") {
		t.Fatalf("unknown verb envelope = %v, want the read-only floor", tools)
	}
}
