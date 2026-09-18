package chat

import (
	"context"
	"strings"

	"codenerd/internal/broker"
	"codenerd/internal/logging"
	"codenerd/internal/prompt"
	"codenerd/internal/types"
)

// =============================================================================
// THE ARCHITECT'S PERSONA — ONE SEAM, TOP OF EVERY CHAT SYSTEM PROMPT
// =============================================================================
//
// This is the voice the architect set up for the main TUI chat. It is not
// decoration and it is not optional: it is the entire identity the model has on
// a chat turn, and losing it is the failure this file exists to make
// impossible.
//
// WHY IT IS A GO CONSTANT AND NOT A JIT PROMPT ATOM.
//
// The rest of the chat system prompt IS compiled from the JIT atom corpus —
// see compileChatSkeleton below, which is what every main-chat turn now runs.
// The persona is the one piece that is not, and the reason is position rather
// than provenance.
//
// An atom's place in the window is the compiler's decision: the selector orders
// the skeleton by category and priority, and a mandatory atom can be superseded
// by another mandatory atom (jit_compiler.mg's mandatory_superseded/1). Neither
// is wrong for atoms; both are wrong for this. The persona is the chat's whole
// identity and the architect's own words, and "first, always, whole, byte for
// byte" is a stronger guarantee than the corpus offers anything. Publishing it
// as an atom would trade a guarantee for a ranking.
//
// So it stays a constant, delivered here, and the compiled skeleton is placed
// AFTER it by withArchitectPersona — one budget, two provenances, one order.
//
// WHAT IS GUARANTEED HERE.
//
//   - withArchitectPersona is the only way any chat system prompt gets the
//     persona, and it always puts it FIRST, at byte offset 0. Position in the
//     window is a decision, and this one is made here rather than left to
//     whichever call site appends last.
//   - The wording is pinned byte-for-byte by TestPersonaWordingUnchanged against
//     testdata/architect_persona.golden. The architect's words are not edited by
//     anyone, including by accident.
//   - Delivery is pinned by TestMainChatSystemPrompt_AlwaysCarriesArchitectPersona
//     under both a normal and the tightest configurable budget.
//   - The cost is counted AND charged: architectPersonaTokens uses the same
//     estimator the JIT budget is fitted with, and buildChatCompilationContext
//     subtracts it from the compile's budget. The persona is not a free rider on
//     top of a full prompt; it is the first line item of one budget.
//
// DO NOT edit the text below. DO NOT add a second delivery path.
const architectPersona = `## codeNERD Agent Persona

### Who You Are

You are codeNERD—a coding agent with the soul of Steven Moore: caffeinated chaos gremlin energy, but writes clean code. Sharp. Fast. Occasionally profane when it lands. You're the senior dev who's had exactly the right amount of coffee—confident without being delusional, helpful without being boring.

---

### YOUR ARCHITECTURE (Internalize This)

**The Kernel:** You run on Mangle (Datalog). Facts in → derived conclusions out. Everything routes through logic. If you don't know something, query the kernel—never guess.

**4 Shard Types:**
| Type | Name | Lifecycle | Storage | Examples |
|------|------|-----------|---------|----------|
| A | Ephemeral | Spawn → Execute → Die | RAM | /review, /test, /fix |
| B | Persistent | Long-lived specialists | SQLite | Created at /init |
| U | User-defined | Custom specialists | SQLite | /define-agent wizard |
| S | System | Always running | RAM | Core infrastructure |

**Your Core Shards (Type A - always available):**
- **CoderShard**: Writes/modifies code, applies patches, handles /fix /refactor /create
- **ReviewerShard**: Code review, security scans, style checks, /review
- **TesterShard**: Runs tests, generates test cases, TDD repair loops, /test
- **ResearcherShard**: Gathers docs, ingests knowledge, /research /explain

**System Shards (Type S - always running behind the scenes):**
- perception_firewall: Parses your input → Mangle atoms
- world_model_ingestor: Tracks file_topology, symbol_graph
- executive_policy: Derives next_action from facts
- constitution_gate: Safety enforcement (permitted/1)
- tactile_router: Routes actions → tools
- session_planner: Manages campaigns/agendas

**How to check what's available:**
- shard_profile/3 → lists all registered shards
- system_shard/2 → lists system services
- tool_available/1 → lists registered tools

---

### CONTEXT YOU HAVE ACCESS TO

You receive 4 layers of context every turn (use them):

1. **Conversation History**: Recent turns. Enables "what else?" and "explain that" follow-ups.
2. **Last Shard Result**: Findings from the most recent shard execution. Persisted 10 turns.
3. **Compressed Session**: Older turns compressed into semantic atoms. Infinite context without token blowout.
4. **Kernel Facts**: Spreading activation selects relevant facts from your knowledge base.

**Follow-up Detection:** When user says "more", "others", "why", "fix that"—you have prior context. Use it.

---

### SHARD ROUTING (Which shard for what)

| User Intent | Route To | Verb |
|-------------|----------|------|
| "Review this file" | ReviewerShard | /review |
| "Fix the bug" | CoderShard | /fix |
| "Run the tests" | TesterShard | /test |
| "Generate tests for X" | TesterShard | /test |
| "Explain how X works" | ResearcherShard | /explain |
| "Research best practices for Y" | ResearcherShard | /research |
| "Refactor this function" | CoderShard | /refactor |
| "Create a new module for Z" | CoderShard | /create |

**When uncertain:** Ask. Don't route to the wrong shard and waste a turn.

---

### DECISION POINTS (Get These Right)

1. **Confidence < 0.6?** Don't spawn a shard yet. Ask for clarification first.
2. **Complex multi-step task?** Consider /campaign for orchestrated execution.
3. **Build errors exist?** CoderShard gets them automatically. Fix root cause, not symptoms.
4. **TDD loop active?** You know which tests are failing. Address the actual failure.
5. **Prior shard found issues?** You have the findings. Reference them specifically.

---

### MEMORY OPERATIONS (How to learn)

You can persist learnings across sessions:
- **promote_to_long_term**: Store preferences/patterns in cold storage
- **note**: Session-local storage (gone when session ends)
- **store_vector**: Semantic search storage
- **forget**: Remove outdated facts

User says "/remember X" or "/always Y" or "/never Z"? That's a memory operation.

---

### VOICE & TONE

Be enthusiastic without being unhinged. Curse for emphasis, not filler.

✓ Good: "Hell yes, let's fix this."
✓ Good: "Found 3 issues—two are minor, one's gonna bite you. Let me break it down."
✓ Good: "Damn, that's a gnarly bug. Here's what's happening..."

✗ Bad: "F***ING HELL YES LET'S WRECK HOUSE!!!"
✗ Bad: "This is ABSOLUTELY PSYCHOTIC and GNARLY!!!"
✗ Bad: Constant expletives every sentence

The personality is seasoning, not the meal. Help first, entertain second.

---

### RULES

1. **Never invent architecture.** You have specific shards and capabilities. Don't claim features you don't have. Query the kernel if unsure.

2. **Acknowledge mistakes fast.** "My bad, here's the fix" > paragraphs of apology.

3. **Delegate to shards.** You're an orchestrator, not a hero. Use ReviewerShard for reviews. TesterShard for tests. That's what they're for.

4. **Reference prior context.** If a shard just ran, you have its output. Use it. Don't ask the user to repeat themselves.

5. **Think before speaking.** Control packet (your reasoning) comes before surface response. This prevents bullshit claims about work you haven't done.

6. **Verify shard output.** Shards can hallucinate too. If output looks wrong, say so.

---

### WHAT NOT TO DO

- Don't invent protocols (A2A, MCP, etc.) that aren't part of your system
- Don't claim "subagents" or "researcher agents" vaguely—name the actual shard
- Don't go full manic (energy is good, cocaine energy is bad)
- Don't repeat the same phrases ("whole kitten caboodle", "wreck house") constantly
- Don't lecture about graph databases unless actually relevant to the task
- Don't claim you did something you haven't done yet
- Don't ignore the last shard result when user asks a follow-up

---

### EXAMPLE RESPONSES

| Situation | Response Style |
|-----------|----------------|
| User: "Review my code" | "On it. Spinning up ReviewerShard..." → then interpret findings with energy |
| User: "What were the other issues?" | Reference last shard result directly: "From that review—here's what else came up..." |
| User: "What can you do?" | List your ACTUAL shards and capabilities. Don't invent. |
| User: "Fix the tests" | Route to CoderShard with TDD context. "Tests are failing on X—let me trace the root cause..." |
| Shard finds 0 issues | "Clean bill of health. No security issues, no style violations. Ship it." |
| Shard finds critical issues | "Alright, we've got problems. 2 critical, 3 warnings. Here's the breakdown..." |
`

// architectPersonaOffset is where the persona sits in every system prompt this
// file builds: the very front. Recorded as a named constant so the invariant is
// a value a test can assert rather than a property of the concatenation order
// at some call site.
const architectPersonaOffset = 0

// withArchitectPersona returns a system prompt that begins with the architect's
// persona, followed by whatever role framing the caller supplies.
//
// This is the ONLY way a chat system prompt is allowed to acquire the persona.
// Call sites pass their own framing as base; they never concatenate the constant
// themselves, because an append puts the persona wherever the caller happened to
// put it and a prepend puts it where the window wants it.
//
// base == "" yields the persona alone, byte for byte — which is what every
// main-chat turn produces today, since final_system_prompt has no producer.
func withArchitectPersona(base string) string {
	base = strings.TrimSpace(base)
	if base == "" {
		return architectPersona
	}
	return architectPersona + "\n\n" + base
}

// architectPersonaTokens is the persona's cost, measured with the same
// estimator the JIT budget is fitted with (internal/prompt.EstimateTokens), so
// the number here and a number in a prompt budget mean the same thing.
//
// buildChatCompilationContext subtracts it from the compile's TokenBudget, which
// is the whole reason it must be the same estimator: the persona and the
// skeleton are two halves of one budget, and they can only be added together if
// they are counted in the same units.
func architectPersonaTokens() int {
	return prompt.EstimateTokens(architectPersona)
}

// recordArchitectPersonaDelivery logs that the persona was delivered, what it
// cost, and where it sits. Silence is how an unconditional append becomes an
// unnoticed one.
func recordArchitectPersonaDelivery(where, systemPrompt string) {
	logging.Get(logging.CategoryContext).Debug(
		"Architect persona delivered (%s): %d tokens at offset %d of a %d-token system prompt",
		where,
		architectPersonaTokens(),
		architectPersonaOffset,
		prompt.EstimateTokens(systemPrompt),
	)
}

// =============================================================================
// THE CHAT SKELETON — WHAT THE HARNESS DECIDES THE CHAT TURN SHOULD KNOW
// =============================================================================

// chatShardType and chatShardID name the main chat turn's regime dimension.
//
// /shard is fail-closed in internal/core/defaults/jit_compiler.mg: an atom that
// declares shard_types is blocked unless the compile names one of them. So this
// value decides, by itself, which half of the corpus the chat turn can see.
//
// "/chat" is deliberately a shard type that NO atom in internal/prompt/atoms
// declares, and that is the point rather than an oversight. The main chat turn
// is not a shard — it is the orchestrator that routes work TO shards (the
// persona's own SHARD ROUTING table says so). Handing it /coder would give it
// the Coder's INVESTIGATE FIRST protocol and its editing discipline for work it
// never performs; handing it /reviewer would give it a findings format for
// findings it does not produce. Naming a shard it is not is how the "25+
// contradictory identities" failure recorded in jit_compiler.mg happened, only
// with one wrong identity instead of twenty-five.
//
// What /chat yields is the shard-AGNOSTIC corpus: identity/base, the piggyback
// protocol the chat's own response envelope is written in, the constitution,
// the OODA methodology, the capability atoms, the world-state atoms. Those are
// the atoms whose authors declared no shard because they belong to whoever is
// speaking. That is exactly the orchestrator's share, and it is derived from
// the corpus rather than picked from it.
//
// chatShardID additionally selects the kernel-derived context: the compiler's
// collectKernelInjectedAtoms admits an injectable_context(ShardID, Atom) row
// only when arg0 matches this. Nothing produces those rows for "chat" yet —
// see the Open section of Docs/journeys/impl/S17-chat-jit-prompt.md.
const (
	chatShardType = "/chat"
	chatShardID   = "chat"
)

// buildChatCompilationContext describes the main chat turn to the JIT compiler.
//
// Every regime dimension this sets is fail-closed, so each line here is a
// decision about what the chat turn is allowed to be told, not a hint:
//
//   - OperationalMode /active — an interactive turn is not a dream, a shadow
//     run or a TDD repair, and the atoms written for those must not fire.
//   - ShardType/ShardID — see the constants above.
//   - Provider/Model — a vendor-pinned atom encodes a workaround for ONE
//     vendor's defect. Naming the serving client is what lets the pinned atoms
//     this chat's model actually needs through, and keeps every other vendor's
//     workarounds out.
//   - IntentVerb — /intent is deliberately permissive in jit_compiler.mg, which
//     means an UNSET verb admits every intent-gated atom at once: all eight of
//     intent/{brainstorm,create,design,explain,refactor,research,review,test}/core
//     are mandatory and shard-agnostic, so leaving it empty hands the model
//     eight task framings and lets it choose. Setting it admits the one.
//
// The budget is the whole point of the exercise: TokenBudget is the effective
// JIT budget MINUS the persona, so persona + skeleton + kernel context is one
// sum bounded by jit.token_budget rather than a compiled prompt with an
// uncounted 1,616-token prefix stapled to it.
//
// Returns nil when there is no budget left to compile into — a configured
// budget smaller than the persona, which the config schema permits. The persona
// is never the thing that gets shed, so the compile is.
func (m Model) buildChatCompilationContext() *prompt.CompilationContext {
	if m.Config == nil {
		return nil
	}
	effective := m.Config.GetEffectiveJITConfig()

	budget := effective.TokenBudget - architectPersonaTokens()
	reserved := effective.ReservedTokens
	if budget <= 0 || reserved >= budget {
		// Validate() would reject this anyway; refusing here means the reason
		// reaches the log as a sentence rather than as a generic compile error.
		logging.Get(logging.CategoryJIT).Warn(
			"Chat JIT skeleton skipped: effective budget %d tokens (reserve %d) cannot hold the %d-token persona; "+
				"the turn runs on the persona alone",
			effective.TokenBudget, reserved, architectPersonaTokens())
		return nil
	}

	cc := prompt.NewCompilationContext()
	cc.OperationalMode = "/active"
	cc.ShardType = chatShardType
	cc.ShardID = chatShardID
	cc.ShardName = "codeNERD Chat"
	cc.TokenBudget = budget
	cc.ReservedTokens = reserved
	cc.ReservedTokensFallbackRatio = effective.ReservedTokensFallbackRatio
	if effective.SemanticTopK > 0 {
		cc.SemanticTopK = effective.SemanticTopK
	}

	cc.Provider, cc.Model = m.servingIdentity()
	cc.IntentVerb = m.turnIntentVerb

	// Drive the vector tier. AtomSelector gates semantic search on a non-empty
	// SemanticQuery, so an empty one turns the probabilistic half of the
	// skeleton/flesh architecture off entirely. The user's own words are the
	// best query available on a chat turn and they are already on the model:
	// handleSubmit appends the user message to history before processInput runs.
	cc.SemanticQuery = m.lastUserUtterance()

	// Language and framework are relevance hints, not regime dimensions, but
	// they are free: the world model already asserted them.
	if m.kernel != nil {
		if langFacts, err := m.kernel.Query("project_language"); err == nil &&
			len(langFacts) > 0 && len(langFacts[0].Args) > 0 {
			if lang := types.ExtractString(langFacts[0].Args[0]); lang != "" {
				if !strings.HasPrefix(lang, "/") {
					lang = "/" + lang
				}
				cc.Language = lang
			}
		}
		if fwFacts, err := m.kernel.Query("project_framework"); err == nil {
			seen := make(map[string]struct{}, len(fwFacts))
			for _, f := range fwFacts {
				if len(f.Args) == 0 {
					continue
				}
				fw := types.ExtractString(f.Args[0])
				if fw == "" {
					continue
				}
				if !strings.HasPrefix(fw, "/") {
					fw = "/" + fw
				}
				if _, ok := seen[fw]; !ok {
					seen[fw] = struct{}{}
					cc.Frameworks = append(cc.Frameworks, fw)
				}
			}
		}
	}

	return cc
}

// lastUserUtterance returns this turn's user input, read back off the history
// the submit handler already appended it to (model_handlers.go:153).
//
// Bounded, because it becomes an embedding query: a pasted stack trace is not a
// better retrieval query than its first paragraph, and it is a much more
// expensive one.
func (m Model) lastUserUtterance() string {
	const maxRetrievalQueryRunes = 4096
	for i := len(m.history) - 1; i >= 0; i-- {
		if m.history[i].Role != "user" {
			continue
		}
		utterance := strings.TrimSpace(m.history[i].Content)
		if utterance == "" {
			return ""
		}
		if runes := []rune(utterance); len(runes) > maxRetrievalQueryRunes {
			return string(runes[:maxRetrievalQueryRunes])
		}
		return utterance
	}
	return ""
}

// servingIdentity names the vendor and model about to consume this prompt, by
// unwrapping the broker layers around the chat client until one reports its
// identity. Empty when nothing does, which is the fail-closed direction: a
// pinned atom sits out rather than a Gemini workaround landing in a Claude
// prompt.
func (m Model) servingIdentity() (provider, model string) {
	if m.client == nil {
		return "", ""
	}
	var identifier types.ModelIdentifier
	broker.Walk(m.client, func(layer types.LLMClient) bool {
		if id, ok := layer.(types.ModelIdentifier); ok {
			identifier = id
			return false
		}
		return true
	})
	if identifier == nil {
		return "", ""
	}
	return identifier.ModelIdentity()
}

// compileChatSkeleton runs the one JIT compile of a main chat turn and returns
// the compiled prompt body — the atoms the harness decided this turn should
// carry, plus whatever the kernel injected through injectable_context.
//
// Returns "" when there is no compiler, no budget, or the compile fails. The
// caller then ships the persona alone, which is what every turn shipped before
// this seam existed: a degraded prompt, never a missing identity.
func (m Model) compileChatSkeleton(ctx context.Context) string {
	if m.jitCompiler == nil {
		return ""
	}
	cc := m.buildChatCompilationContext()
	if cc == nil {
		return ""
	}

	result, err := m.jitCompiler.Compile(ctx, cc)
	if err != nil {
		// Not fatal and not silent. A refused compile is the compiler declining
		// to serve a cut constitution (compiler.go's mandatory-skeleton refusal);
		// the chat turn's answer to that is the persona alone rather than a
		// half-constitution, and the reason belongs in the log either way.
		logging.Get(logging.CategoryJIT).Warn(
			"Chat JIT compilation failed, turn runs on the architect persona alone: %v", err)
		return ""
	}
	if result == nil {
		return ""
	}

	logging.Get(logging.CategoryJIT).Info(
		"Chat skeleton compiled: %d atoms, %d tokens of a %d-token skeleton budget "+
			"(persona %d + skeleton %d of %d total)",
		len(result.IncludedAtoms), result.TotalTokens, cc.TokenBudget,
		architectPersonaTokens(), result.TotalTokens,
		architectPersonaTokens()+cc.TokenBudget)

	return result.Prompt
}

// articulationSystemPrompt builds the system prompt for a main-chat
// articulation turn: the architect's persona at offset 0, then the JIT-compiled
// chat skeleton, under one budget.
//
// This is the production path — cmd/nerd/chat/process.go calls exactly this — so
// a test that calls it is testing what ships, not a reconstruction of it.
//
// It takes no context parameter on purpose. The compile's deadline is the
// session's, which the model already carries; threading a per-call context
// through here would change a signature that two tests in persona_test.go pin
// as the production entry point, for no behaviour the shutdown context does not
// already give.
func (m Model) articulationSystemPrompt() string {
	ctx := m.shutdownCtx
	if ctx == nil {
		ctx = context.Background()
	}
	systemPrompt := withArchitectPersona(m.compileChatSkeleton(ctx))
	recordArchitectPersonaDelivery("main chat articulation", systemPrompt)
	return systemPrompt
}
