package chat

import (
	"context"
	"sort"
	"strings"
	"testing"

	"codenerd/internal/config"
	"codenerd/internal/core"
	"codenerd/internal/perception"
	"codenerd/internal/prompt"
	"codenerd/internal/system"
)

// =============================================================================
// S17 — THE INTERACTIVE CHAT TURN COMPILES A JIT PROMPT
// =============================================================================
//
// Before this seam the main TUI chat turn was the only high-volume path in the
// product that received no compiled prompt at all. Its system prompt came from
// a kernel query for final_system_prompt, a predicate with a Decl and no
// producer anywhere in the repository, so the query returned zero rows on every
// turn and the whole system prompt was the architect's persona — 1,616 tokens
// of identity with no protocol, no constitution, no methodology and no
// capability atoms behind it.
//
// These tests pin the three properties that make the new shape true rather than
// merely present: the atoms arrive, they arrive BEHIND the persona, and the two
// of them together are bounded by one budget.

// newChatTestCompiler builds a JIT compiler over the SHIPPED embedded corpus —
// the same internal/prompt/atoms/**.yaml the product compiles from — with a real
// Mangle kernel doing selection.
//
// It is deliberately not a stub corpus. The question these tests exist to answer
// is "does the chat's compilation context select anything useful out of the real
// corpus", and a hand-built corpus of four atoms answers a different question.
func newChatTestCompiler(t *testing.T) (*prompt.JITPromptCompiler, *core.RealKernel) {
	t.Helper()

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

	return compiler, kernel
}

// newChatModelWithCompiler returns a chat model wired the way a booted session
// wires one, for the parts this seam reads: a kernel, a JIT compiler, a config
// carrying the JIT budget, and a user utterance in history (which is where
// buildChatCompilationContext reads the retrieval query from, because
// handleSubmit appends it before processInput runs).
func newChatModelWithCompiler(t *testing.T, utterance, verb string) Model {
	t.Helper()

	compiler, kernel := newChatTestCompiler(t)

	m := NewTestModel()
	m.kernel = kernel
	m.jitCompiler = compiler
	m.turnIntentVerb = verb
	m.Config = &config.UserConfig{
		ContextWindow: &config.ContextWindowConfig{MaxTokens: 1048576},
		JIT:           &config.JITConfig{TokenBudget: 200000, ReservedTokens: 8000},
	}
	if utterance != "" {
		m.history = append(m.history, Message{Role: "user", Content: utterance})
	}
	return m
}

// TestChatSkeleton_CompiledAtomInventory is the MEASUREMENT.
//
// It records what the chat's compilation context actually selects out of the
// shipped corpus: the atom IDs, the skeleton/flesh split, the token total and
// the budget it was compiled under. The numbers it logs are the baseline in
// Docs/journeys/impl/S17-chat-jit-prompt.md, and it fails only on the property
// that baseline has to keep: the chat turn selects a real prompt, not an empty
// one and not the whole corpus.
func TestChatSkeleton_CompiledAtomInventory(t *testing.T) {
	m := newChatModelWithCompiler(t, "why does the continuation loop stop early?", "/explain")

	cc := m.buildChatCompilationContext()
	if cc == nil {
		t.Fatal("buildChatCompilationContext returned nil under a 200k budget; " +
			"the chat turn cannot compile at all")
	}

	result, err := m.jitCompiler.Compile(context.Background(), cc)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}

	ids := make([]string, 0, len(result.IncludedAtoms))
	categories := map[string]int{}
	mandatory := 0
	for _, atom := range result.IncludedAtoms {
		ids = append(ids, atom.ID)
		categories[string(atom.Category)]++
		if atom.IsMandatory {
			mandatory++
		}
	}
	sort.Strings(ids)

	t.Logf("chat compilation context: %s", cc.String())
	t.Logf("budget: %d tokens (persona %d already subtracted, reserve %d)",
		cc.TokenBudget, architectPersonaTokens(), cc.ReservedTokens)
	t.Logf("selected: %d atoms (%d flagged mandatory), %d tokens, %.1f%% of budget",
		len(result.IncludedAtoms), mandatory, result.TotalTokens, result.BudgetUsed)
	t.Logf("skeleton/flesh by category: skeleton=%d flesh=%d",
		result.MandatoryCount, result.OptionalCount)
	t.Logf("candidates=%d selected=%d included=%d",
		result.AtomsCandidates, result.AtomsSelected, result.AtomsIncluded)

	catNames := make([]string, 0, len(categories))
	for name := range categories {
		catNames = append(catNames, name)
	}
	sort.Strings(catNames)
	for _, name := range catNames {
		t.Logf("  category %-14s %d atoms", name, categories[name])
	}
	for _, id := range ids {
		t.Logf("  atom %s", id)
	}

	if len(result.IncludedAtoms) == 0 {
		t.Fatal("the chat turn compiled ZERO atoms. This is the pre-S17 state expressed " +
			"through the new seam: a compilation context that selects nothing is the same " +
			"prompt as no compile at all.")
	}

	// The persona's own SHARD ROUTING table makes the chat an orchestrator, not
	// a shard, so no shard's private identity may leak into it. This is the
	// failure jit_compiler.mg documents: an unset /shard admitted 114 mandatory
	// atoms carrying 25+ contradictory identities, and the model answered as
	// whichever one it latched onto.
	for _, atom := range result.IncludedAtoms {
		for _, st := range atom.ShardTypes {
			st = strings.TrimSpace(st)
			if st == "" || st == "*" {
				continue
			}
			if "/"+strings.TrimPrefix(st, "/") != chatShardType {
				t.Errorf("atom %q is scoped to shard %q but reached the chat prompt; "+
					"the chat is the orchestrator, not a shard, and another persona's "+
					"atoms replace its identity rather than adding to it", atom.ID, st)
			}
		}
	}
}

// TestMainChatPrompt_CarriesPersonaThenCompiledAtoms is the behavioural pin.
//
// PROVEN TO FAIL against the branch point: with cmd/nerd/chat/persona.go,
// process.go and helpers_articulation.go restored to 0225c695,
// articulationSystemPrompt returns the persona and nothing else, so the prompt
// is exactly persona-length and the subtest fails on "carries no compiled
// skeleton at all (0 bytes after the persona)".
func TestMainChatPrompt_CarriesPersonaThenCompiledAtoms(t *testing.T) {
	m := newChatModelWithCompiler(t, "review the executor for races", "/review")

	systemPrompt := m.articulationSystemPrompt()
	persona := personaAlone()

	// 1. The persona is still first and still whole.
	if !strings.HasPrefix(systemPrompt, persona) {
		t.Fatalf("the architect persona is no longer at offset 0 of the main chat prompt "+
			"(prompt is %d bytes)", len(systemPrompt))
	}

	// 2. The compiled skeleton is behind it.
	skeleton := systemPrompt[len(persona):]
	if strings.TrimSpace(skeleton) == "" {
		t.Fatalf("the main chat system prompt carries no compiled skeleton at all "+
			"(0 bytes after the persona; whole prompt is %d bytes). The interactive turn is "+
			"the highest-volume path in the product and it is receiving no atom the harness "+
			"chose for it.", len(systemPrompt))
	}

	// 3. It is the compiler's output, not prose someone concatenated. Compile
	//    the same context directly and require the skeleton to be that text.
	cc := m.buildChatCompilationContext()
	if cc == nil {
		t.Fatal("buildChatCompilationContext returned nil under a 200k budget")
	}
	direct, err := m.jitCompiler.Compile(context.Background(), cc)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if strings.TrimSpace(skeleton) != strings.TrimSpace(direct.Prompt) {
		t.Fatalf("the text behind the persona is not the compiled prompt.\n"+
			" delivered %d bytes\n compiled  %d bytes\n"+
			"Something other than the JIT compiler is writing the chat's system prompt.",
			len(strings.TrimSpace(skeleton)), len(strings.TrimSpace(direct.Prompt)))
	}

	t.Logf("system prompt: %d bytes total = %d persona + %d compiled skeleton (%d atoms)",
		len(systemPrompt), len(persona), len(skeleton), len(direct.IncludedAtoms))
}

// TestChatPromptBudget_CountsThePersona pins the "one budget" half of the seam.
//
// The persona used to be free: it was appended to whatever the prompt already
// was, and nothing subtracted its 1,616 tokens from anything. The compile now
// runs against jit.token_budget MINUS the persona, so the two together are one
// bounded sum.
func TestChatPromptBudget_CountsThePersona(t *testing.T) {
	const (
		totalBudget = 40000
		reserved    = 4000
	)

	m := newChatModelWithCompiler(t, "explain the working meter", "/explain")
	m.Config = &config.UserConfig{
		ContextWindow: &config.ContextWindowConfig{MaxTokens: 1048576},
		JIT:           &config.JITConfig{TokenBudget: totalBudget, ReservedTokens: reserved},
	}

	cc := m.buildChatCompilationContext()
	if cc == nil {
		t.Fatalf("buildChatCompilationContext returned nil under a %d-token budget", totalBudget)
	}

	persona := architectPersonaTokens()
	if cc.TokenBudget != totalBudget-persona {
		t.Fatalf("compile budget = %d, want %d (jit.token_budget %d minus the %d-token persona). "+
			"An uncounted persona is a prompt that overruns its budget by exactly the amount "+
			"nobody is measuring.", cc.TokenBudget, totalBudget-persona, totalBudget, persona)
	}

	systemPrompt := m.articulationSystemPrompt()
	total := prompt.EstimateTokens(systemPrompt)
	if total > totalBudget {
		t.Fatalf("assembled chat system prompt is %d tokens, over the %d-token budget "+
			"(persona %d + skeleton %d)", total, totalBudget, persona, total-persona)
	}
	t.Logf("persona %d + skeleton %d = %d tokens, budget %d (reserve %d)",
		persona, total-persona, total, totalBudget, reserved)
}

// TestChatSkeleton_BudgetTooSmallKeepsThePersonaWhole is the other end of the
// budget: when jit.token_budget cannot even hold the persona, the persona is
// not what gets shed. The compile is skipped and the turn runs on identity
// alone — degraded, never anonymous.
func TestChatSkeleton_BudgetTooSmallKeepsThePersonaWhole(t *testing.T) {
	m := newChatModelWithCompiler(t, "hello", "/explain")
	m.Config = &config.UserConfig{
		ContextWindow: &config.ContextWindowConfig{MaxTokens: 1},
		JIT:           &config.JITConfig{TokenBudget: 1, ReservedTokens: 1},
	}

	if cc := m.buildChatCompilationContext(); cc != nil {
		t.Fatalf("a 1-token budget produced a compilation context with budget %d; "+
			"it should refuse rather than compile into a negative budget", cc.TokenBudget)
	}

	persona := personaAlone()
	if got := m.articulationSystemPrompt(); got != persona {
		t.Fatalf("under an impossible budget the chat prompt should be the persona alone "+
			"(%d bytes), got %d bytes", len(persona), len(got))
	}
}

// personaAlone returns the persona's exact bytes through the one sanctioned
// accessor. Naming the constant here would trip
// TestArchitectPersonaHasOneDeliverySeam, which is correct of it: the persona
// has one delivery seam, and a test asserting where the persona sits should get
// it from that seam rather than reaching past it.
func personaAlone() string { return withArchitectPersona("") }

// TestChatTurnCompilesExactlyOnce pins the cost. One compile per turn is
// affordable (~344ms, measured in S19); a compile per call site is not, and a
// compile inside a render or a retry loop is how that happens.
func TestChatTurnCompilesExactlyOnce(t *testing.T) {
	m := newChatModelWithCompiler(t, "fix the flaky test", "/fix")

	before := m.jitCompiler.GetStats()
	_ = m.articulationSystemPrompt()
	after := m.jitCompiler.GetStats()

	compiles := after.TotalCompilations - before.TotalCompilations
	if compiles != 1 {
		t.Fatalf("one main-chat turn ran %d compilations, want exactly 1", compiles)
	}
}

// TestChatCompilationContextNamesEveryFailClosedDimension.
//
// /shard, /mode, /provider and /model are fail-closed regime dimensions in
// internal/core/defaults/jit_compiler.mg: an atom constrained on one of them is
// blocked unless the compile names it. Leaving one empty is therefore not
// "unfiltered", it is "that whole population of atoms is retired" — silently,
// with no error and no log line. This test is the reminder.
func TestChatCompilationContextNamesEveryFailClosedDimension(t *testing.T) {
	m := newChatModelWithCompiler(t, "add a test for the elider", "/test")

	cc := m.buildChatCompilationContext()
	if cc == nil {
		t.Fatal("buildChatCompilationContext returned nil under a 200k budget")
	}

	if cc.ShardType != chatShardType {
		t.Errorf("ShardType = %q, want %q: an empty /shard admits every shard-gated atom "+
			"in the corpus at once, which is the 25-identity failure jit_compiler.mg records",
			cc.ShardType, chatShardType)
	}
	if cc.ShardID != chatShardID {
		t.Errorf("ShardID = %q, want %q: it is what injectable_context rows are matched against",
			cc.ShardID, chatShardID)
	}
	if cc.OperationalMode != "/active" {
		t.Errorf("OperationalMode = %q, want /active: an interactive turn is not a dream, "+
			"a shadow run or a TDD repair", cc.OperationalMode)
	}
	if cc.IntentVerb != "/test" {
		t.Errorf("IntentVerb = %q, want the turn's verb: /intent is permissive, so an unset "+
			"verb hands the model all eight intent/*/core framings at once", cc.IntentVerb)
	}
	if cc.SemanticQuery == "" {
		t.Error("SemanticQuery is empty: AtomSelector gates vector search on it, so an empty " +
			"query turns off the probabilistic half of the skeleton/flesh architecture")
	}
}

// TestSystemPromptIsNotEchoedIntoTheUserMessage.
//
// articulateWithConversation used to open the USER message with "System
// Instructions:" and the entire system prompt, and then pass the same string as
// the system argument. The bytes were sent twice in one request, and after S17
// that would have doubled the compiled skeleton as well as the persona — the
// compiler's budget spent twice, with the second copy invisible to it.
//
// PROVEN TO FAIL against the branch point: restoring helpers_articulation.go to
// 0225c695 makes the user message start with "System Instructions:" and this
// test reports the duplicated bytes.
func TestSystemPromptIsNotEchoedIntoTheUserMessage(t *testing.T) {
	const sentinel = "SENTINEL-SYSTEM-PROMPT-MUST-NOT-BE-ECHOED"

	intent := perception.Intent{Category: "/query", Verb: "/explain", Target: "budget"}
	client := NewMockLLMClient()

	// The mock returns text the piggyback parser rejects, so this errors. The
	// assertion is about what the client was HANDED, which is captured before
	// any parsing happens.
	_, _ = articulateWithConversation(
		context.Background(),
		client,
		intent,
		payloadForArticulation(intent, nil),
		nil,
		sentinel,
		nil, nil, nil,
	)

	client.mu.Lock()
	systemPrompts := append([]string(nil), client.systemPrompts...)
	userPrompt := client.lastPrompt
	client.mu.Unlock()

	if len(systemPrompts) != 1 || systemPrompts[0] != sentinel {
		t.Fatalf("system arguments = %q, want exactly one call carrying the system prompt verbatim",
			systemPrompts)
	}
	if strings.Contains(userPrompt, sentinel) {
		t.Fatalf("the system prompt is echoed into the user message. Every byte of the "+
			"persona and the compiled skeleton would be charged twice per turn.\n"+
			"user message begins: %q", firstN(userPrompt, 200))
	}
	if strings.Contains(userPrompt, "System Instructions:") {
		t.Fatalf("the user message still opens with the \"System Instructions:\" header; " +
			"the echo seam is back")
	}
}

func firstN(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
