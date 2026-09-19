package system_test

import (
	"context"
	"slices"
	"strings"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/prompt"
	"codenerd/internal/system"
	"codenerd/internal/types"
)

// The harness predicts what a compile will need and the JIT serves it at that
// moment, and only then. For a compile aimed at a .mg file the kernel derives
// /authoring_mangle (policy/jit_needs.mg), and the Mangle core and the verified
// engine truths are in the prompt; for a Go file the kernel derives nothing and
// neither is. Measured before this existed (2026-09-18): a coder compile with
// language /mangle selected 41 atoms and none of the corpus's 119 /mangle atoms.
func TestJITServesTheMangleCorpusWhenTheKernelDerivesTheNeed(t *testing.T) {
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	embedded, err := prompt.LoadEmbeddedCorpus()
	if err != nil {
		t.Fatalf("LoadEmbeddedCorpus: %v", err)
	}
	compiler, err := prompt.NewJITPromptCompiler(
		prompt.WithKernel(system.NewKernelAdapter(kernel)),
		prompt.WithEmbeddedCorpus(embedded),
	)
	if err != nil {
		t.Fatalf("NewJITPromptCompiler: %v", err)
	}
	t.Cleanup(func() { _ = compiler.Close() })

	needs := func(language string) []string {
		facts, err := kernel.Query("target_need(" + language + ", Need)")
		if err != nil {
			t.Fatalf("target_need(%s, Need): %v", language, err)
		}
		var out []string
		for _, f := range facts {
			out = append(out, strings.TrimPrefix(types.ExtractString(f.Args[1]), "/"))
		}
		return out
	}
	compile := func(language string) (ids []string, tokens int) {
		cc := prompt.NewCompilationContext()
		cc.ShardType, cc.ShardID, cc.IntentVerb, cc.OperationalMode = "/coder", "coder", "/fix", "/active"
		cc.Language = language
		cc.TokenBudget = 200000
		cc.DerivedNeeds = needs(language)
		res, err := compiler.Compile(context.Background(), cc)
		if err != nil {
			t.Fatalf("Compile(%s): %v", language, err)
		}
		for _, a := range res.IncludedAtoms {
			ids = append(ids, a.ID)
		}
		return ids, res.TotalTokens
	}
	served := []string{"language/mangle/core", "language/mangle/engine_truths_pinned"}

	if got := needs("/mangle"); !slices.Equal(got, []string{"authoring_mangle"}) {
		t.Fatalf("the kernel derived %v for a /mangle target, want [authoring_mangle]", got)
	}
	mangleIDs, mangleTokens := compile("/mangle")
	for _, id := range served {
		if !slices.Contains(mangleIDs, id) {
			t.Errorf("a compile aimed at a .mg file did not serve %s", id)
		}
	}

	if got := needs("/go"); len(got) != 0 {
		t.Fatalf("the kernel derived %v for a /go target, want nothing", got)
	}
	goIDs, goTokens := compile("/go")
	for _, id := range served {
		if slices.Contains(goIDs, id) {
			t.Errorf("a compile aimed at a Go file was served %s; the need was never derived", id)
		}
	}
	t.Logf("/mangle compile: %d atoms, %d tokens; /go compile: %d atoms, %d tokens", len(mangleIDs), mangleTokens, len(goIDs), goTokens)
}
