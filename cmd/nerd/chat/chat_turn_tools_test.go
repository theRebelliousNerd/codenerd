package chat

import (
	"context"
	"slices"
	"strings"
	"testing"

	"codenerd/internal/config"
	"codenerd/internal/prompt"
)

// C2 — A CHAT TURN COMPILES AGAINST THE TOOLS IT IS OFFERED.
//
// The selector drops every atom whose requires_tools are not all in
// CompilationContext.AvailableTools (internal/prompt/atoms.go
// atomToolSatisfied). Commit a7e584e5 fixed the spawner, which compiled
// with AvailableTools unset and so stripped all tool-gated guidance from
// every subagent. The main chat turn compiled the same way: its context
// never named the turn's catalog, so interactive turns lost every
// tool-gated atom (CodeDOM, structure queries).
//
// The turn's catalog is the kernel derivation prompt.DeriveTurnTools
// (commit 34c5eac8), the same one the session executor and the spawner
// use. These tests pin that the chat turn compiles against it.

// TestChatSkeleton_IncludesToolGatedAtomsOfTurnCatalog is the failing-test
// pin: a /fix turn's catalog offers find_symbol and get_element (the
// /coder persona's /codedom envelope), so the skeleton must carry
// capability/codedom_core, which requires exactly those two tools.
func TestChatSkeleton_IncludesToolGatedAtomsOfTurnCatalog(t *testing.T) {
	m := newChatModelWithCompiler(t, "fix the race in the executor", "/fix")

	// Precondition: the catalog really does offer the atom's tools. If the
	// policy envelope changes, this fails instead of the assertion below,
	// so the failure names the catalog, not the compile.
	catalog, err := prompt.DeriveTurnTools(m.kernel, "/fix")
	if err != nil {
		t.Fatalf("DeriveTurnTools(/fix): %v", err)
	}
	for _, tool := range []string{"find_symbol", "get_element"} {
		if !slices.Contains(catalog, tool) {
			t.Fatalf("/fix catalog lacks %q (catalog: %v); the test's "+
				"precondition is gone, not the compile", tool, catalog)
		}
	}

	cc := m.buildChatCompilationContext()
	if cc == nil {
		t.Fatal("buildChatCompilationContext returned nil under a 200k budget")
	}
	result, err := m.jitCompiler.Compile(context.Background(), cc)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}

	for _, atom := range result.IncludedAtoms {
		if atom.ID == "capability/codedom_core" {
			return
		}
	}
	t.Fatalf("the /fix chat skeleton lacks capability/codedom_core although "+
		"the turn's catalog offers its tools %v; the compile is running "+
		"against an empty AvailableTools", catalog)
}

// TestChatCompilationContext_SetsAvailableToolsFromTurnCatalog pins that
// the context carries the derivation itself — not a copy that can drift,
// and not a second query answered differently.
func TestChatCompilationContext_SetsAvailableToolsFromTurnCatalog(t *testing.T) {
	m := newChatModelWithCompiler(t, "fix the race in the executor", "/fix")

	cc := m.buildChatCompilationContext()
	if cc == nil {
		t.Fatal("buildChatCompilationContext returned nil under a 200k budget")
	}
	want, err := prompt.DeriveTurnTools(m.kernel, "/fix")
	if err != nil {
		t.Fatalf("DeriveTurnTools(/fix): %v", err)
	}
	if !slices.Equal(cc.AvailableTools, want) {
		t.Fatalf("AvailableTools = %v, want the turn's catalog %v", cc.AvailableTools, want)
	}
}

// TestChatCompilationContext_FailClosedWithoutKernel pins the executor's
// rule (resolveAvailableTools): a derivation that cannot run compiles
// against none. No tools is not all tools.
func TestChatCompilationContext_FailClosedWithoutKernel(t *testing.T) {
	compiler, _ := newChatTestCompiler(t)

	m := NewTestModel()
	m.jitCompiler = compiler
	m.turnIntentVerb = "/fix"
	m.Config = &config.UserConfig{
		ContextWindow: &config.ContextWindowConfig{MaxTokens: 1048576},
		JIT:           &config.JITConfig{TokenBudget: 200000, ReservedTokens: 8000},
	}
	// m.kernel stays nil: NewTestModel wires no kernel.

	cc := m.buildChatCompilationContext()
	if cc == nil {
		t.Fatal("buildChatCompilationContext returned nil under a 200k budget")
	}
	if len(cc.AvailableTools) != 0 {
		t.Fatalf("AvailableTools = %v without a kernel; a failed derivation "+
			"must fail closed", cc.AvailableTools)
	}
}

// TestNorthstarCompilationContext_SetsAvailableTools pins that the
// /research compile names the /research envelope, so the /researcher prompt
// keeps its tool-gated research guidance.
func TestNorthstarCompilationContext_SetsAvailableTools(t *testing.T) {
	_, kernel := newChatTestCompiler(t)

	m := NewTestModel()
	m.kernel = kernel

	cc := m.buildNorthstarCompilationContext("requirements")
	want, err := prompt.DeriveTurnTools(kernel, "/research")
	if err != nil {
		t.Fatalf("DeriveTurnTools(/research): %v", err)
	}
	if !slices.Equal(cc.AvailableTools, want) {
		t.Fatalf("AvailableTools = %v, want the turn's catalog %v", cc.AvailableTools, want)
	}
	// find_symbol is in the /researcher /codedom_read envelope and absent
	// from the /general floor: this distinguishes the envelope from a
	// collapsed-to-/general fallback.
	if !slices.Contains(cc.AvailableTools, "find_symbol") {
		t.Fatalf("AvailableTools = %v, want the /research envelope (with find_symbol)",
			cc.AvailableTools)
	}
}

// TestNorthstarCompilationContext_FailClosedWithoutKernel is the northstar
// half of the fail-closed rule: no kernel, no catalog, no panic.
func TestNorthstarCompilationContext_FailClosedWithoutKernel(t *testing.T) {
	m := NewTestModel()

	cc := m.buildNorthstarCompilationContext("requirements")
	if len(cc.AvailableTools) != 0 {
		t.Fatalf("AvailableTools = %v without a kernel; a failed derivation "+
			"must fail closed", cc.AvailableTools)
	}
}

// TestTranslatorPrompt_CompilesAgainstEmptyCatalog pins the deliberate
// exception: the /translate translator never calls tools — its answer is
// user-facing prose — so its compile names no catalog and carries no
// tool-gated atoms. "Refs Name Code" is capability/codedom_core's heading,
// unique to that atom, which requires find_symbol+get_element.
func TestTranslatorPrompt_CompilesAgainstEmptyCatalog(t *testing.T) {
	m := newChatModelWithCompiler(t, "what is the main issue?", "/review")

	systemPrompt, _ := m.buildShardInterpretationPrompt(
		context.Background(),
		"what is the main issue?", "reviewer", "review foo.go",
		"found 3 issues: an unchecked error, a data race, a typo")
	if strings.HasPrefix(systemPrompt, personaAlone()) {
		t.Fatal("the translator fell back to the persona; the test cannot " +
			"observe the JIT catalog it compiled against")
	}
	if strings.Contains(systemPrompt, "Refs Name Code") {
		t.Fatal("the translator prompt carries capability/codedom_core, which " +
			"requires find_symbol+get_element: its catalog is supposed to stay empty")
	}

	// Contrast: the same regime WITH a catalog selects the atom, so the
	// absence above is the empty catalog, not the /translate regime. The
	// chain mirrors buildShardInterpretationPrompt's.
	tokenBudget, reservedTokens := translatorJITBudget(m.Config)
	cc := prompt.NewCompilationContext().
		WithOperationalMode("/active").
		WithIntent("/translate", "").
		WithShard("/analysis_translator", "analysis_translator", "Analysis Translator").
		WithTokenBudget(tokenBudget, reservedTokens).
		WithSemanticQuery("Translate reviewer shard output into actionable summary", 8)
	var err error
	if cc.AvailableTools, err = prompt.DeriveTurnTools(m.kernel, "/fix"); err != nil {
		t.Fatalf("DeriveTurnTools(/fix): %v", err)
	}
	res, err := m.jitCompiler.Compile(context.Background(), cc)
	if err != nil {
		t.Fatalf("contrast Compile: %v", err)
	}
	if !strings.Contains(res.Prompt, "Refs Name Code") {
		t.Fatal("the translator regime plus the /fix catalog still lacks " +
			"codedom_core: the regime blocks the atom some other way and the " +
			"emptiness assertion above proves nothing")
	}
}
