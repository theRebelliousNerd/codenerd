# S17 — the interactive chat turn compiles a JIT prompt

## Status

- **last updated**: 2026-09-18, log created (first action, before any study)
- **done**: nothing yet
- **open**: everything — study, measurement, design, implementation, tests

Branch: `worktree-agent-afad2d0ae49f7aa7b`. Branch point was `0b5c69d0`, an *old* commit of
`dogfood/c2-closure`; the seam this brief describes (`cmd/nerd/chat/persona.go`,
`withArchitectPersona`, `persona_test.go`, `testdata/architect_persona.golden`) landed after it in
`5bcf0a5d` / `e2474cf8`. Fast-forwarded to `0225c695` (the `dogfood/c2-closure` tip) before doing
anything, so this work sits on the same base as the sibling seams. Pure fast-forward, no merge
commit, clean tree.

## Study

Everything below was read in this worktree at `0225c695`, not taken from the brief.

### 1. The main-chat system prompt is the persona and nothing else

`cmd/nerd/chat/persona.go:273-284` — `Model.articulationSystemPrompt()`:

```go
base := ""
if m.kernel != nil {
    if systemPrompts, err := m.kernel.Query("final_system_prompt"); err == nil && ... {
        base = types.ExtractString(systemPrompts[0].Args[0])
    }
}
systemPrompt := withArchitectPersona(base)
```

`final_system_prompt` has no producer: the only occurrences in the tree are the `Decl` at
`internal/core/defaults/schemas_reviewer.mg:145`, this query, the row in
`internal/core/defaults/testdata/query_only_predicates.txt`, and prose. So `base` is always `""`,
`withArchitectPersona("")` returns `architectPersona` verbatim (`persona.go:227-233`), and the whole
system prompt is 1,616 tokens of persona.

`cmd/nerd/chat/process.go:819` — `contextFacts, _ = m.kernel.Query("context_to_inject")`. Same
story: `Decl` at `schemas_reviewer.mg:142`, a `rule_description/2` *string* mentioning it in
`internal/core/defaults/policy/trace_logic.mg:29` (documentation of a rule that does not exist, not
the rule), this query, and the baseline row. Always zero rows, so the `Context Facts:` block at
`helpers_articulation.go:252-273` never renders.

### 2. The chat turn never calls the JIT compiler

`Compile` call sites under `cmd/nerd/chat`: `northstar_llm.go:110` (the northstar wizard) and
`process_dream_delegation.go:413`. `process.go:559`/`:911` only read `m.jitCompiler.GetLastResult()`
for the feedback-store manifest hash — which on the main path is therefore the manifest of some
*other* subsystem's last compile, or nil. The interactive turn compiles nothing.

### 3. The persona is delivered twice, as the same bytes

`helpers_articulation.go:143-148`:

```go
if systemPrompt != "" {
    sb.WriteString("System Instructions:\n")
    sb.WriteString(systemPrompt)
    sb.WriteString("\n\n")
}
```

`sb` is the **user** message. The same `systemPrompt` string is then passed as the *system*
argument at `:323` (`CompleteWithStreamingAndThoughts`), `:327` (`CompleteWithStreaming`) and
`:383` (`CompleteWithSystem`). No transformation between the two — it is the identical variable.
This is a straight second delivery of the same bytes and it is charged twice by the broker
(`internal/broker/types.go:78-82` already notes the concatenation). **Verdict: delete the echo.**
It is not "belt and braces": today it doubles a 1,616-token persona, and after this seam lands it
would double the entire compiled skeleton.

### 4. What the compiler needs from a compilation context

- `Compile(ctx, cc)` — `internal/prompt/compiler.go:575`. `cc.Validate()` then `cc.Clone()`, cache
  keyed on `cc.Hash()`, singleflight, ~344 ms per real compile (S19).
- **`ShardType` must be set.** `internal/session/executor.go:1226-1241` documents the failure mode
  in the code itself: `jit_compiler.mg`'s `blocked_by_context` only blocks a shard-gated atom when
  the context *has* the shard dimension, so an empty `ShardType` admits **every** shard-gated atom —
  a measured 114 mandatory atoms, ~60k tokens, 25+ contradictory identities, and the model answering
  as whichever it latched onto. An empty `ShardType` is not "no filter", it is "all personas at
  once".
- **`Provider`/`Model` are fail-closed** (`internal/prompt/context.go:151-168`): empty blocks every
  provider-pinned atom rather than admitting them all. Safe default, but it retires the evolved
  corpus, so the chat context should name the serving vendor.
- **`SemanticQuery` gates vector search entirely** (`executor.go:1197-1215`): `AtomSelector` needs
  `cc.SemanticQuery != ""` or the probabilistic half never runs.
- **`ShardID` is what the kernel-derived context keys on.** `collectKernelInjectedAtoms`
  (`compiler.go:1054-1103`) queries `injectable_context(ShardID, Atom)` and admits a row only when
  arg0 matches `cc.ShardID` / `cc.ShardInstanceID` / `*` / `/_all`. It renders them into a single
  mandatory `kernel/context/<hash>` atom at priority 95. So item 3 of the brief's prompt shape —
  "the kernel-derived context the compiler already places" — arrives for free the moment the chat
  turn compiles with a `ShardID`, and it arrives *inside* the one budget rather than beside it.

### 5. The budget

`config.JITConfig` (`internal/config/jit.go`): `TokenBudget` default 200000, `ReservedTokens` 8000.
`UserConfig.GetEffectiveJITConfig()` (`internal/config/user_config.go:1607-1627`) clamps
`TokenBudget` to `ContextWindow.MaxTokens` and then zeroes the reserve if it would swallow the
budget. `architectPersonaTokens()` (`persona.go:244-246`) uses `prompt.EstimateTokens`, the same
estimator the JIT budget is fitted with — so subtracting one from the other is meaningful.

### 6. The constraint the tests impose on the design

`cmd/nerd/chat/persona_test.go` must pass **unchanged**. Two consequences:

- `TestMainChatSystemPrompt_AlwaysCarriesArchitectPersona:112` calls `m.articulationSystemPrompt()`
  with **no arguments**, on a `NewTestModel()` that has no JIT compiler, under a budget of 1 token.
  So the signature cannot change, the method must tolerate `m.jitCompiler == nil`, and a budget too
  small to compile must still yield the persona whole at offset 0.
- `TestArchitectPersonaHasOneDeliverySeam:198` greps every non-`persona.go` file in the package for
  `\barchitectPersona\b`. The compile must therefore be wired *inside* `persona.go`, not bolted on
  in `process.go`.

Both constraints point the same way: `articulationSystemPrompt()` stays the one seam and grows the
compile inside it, using `m.shutdownCtx` (with a `context.Background()` fallback — the pattern
`process.go:866-869` already uses) so no signature changes.

## Measurement

`TestChatSkeleton_CompiledAtomInventory` (`cmd/nerd/chat/chat_jit_prompt_test.go`) compiles the
chat's compilation context against the **shipped** embedded corpus (`prompt.LoadEmbeddedCorpus`,
914 candidates) with a real Mangle kernel doing selection. Not a stub corpus — the question is what
the real corpus yields for this context, and four hand-built atoms answer a different question.

Run at `jit.token_budget` 200000 / reserve 8000, verb `/explain`:

```
chat compilation context: mode=/active shard=/chat intent=/explain budget(available)=190384
budget: 198384 tokens (persona 1616 already subtracted, reserve 8000)
selected: 17 atoms (17 flagged mandatory), 4451 tokens, 0.0% of budget
skeleton/flesh by category: skeleton=14 flesh=3
candidates=914 selected=18 included=17
```

The 17 atoms:

| Category | Atoms |
|---|---|
| identity | `identity/base/core` |
| protocol | `protocol/piggyback/{envelope,mangle_updates,thought_first}`, `protocol/reasoning/{format,requirements}` |
| safety | `safety/constitution/{prime_directive,forbidden_actions,dangerous_actions}`, `safety/constitutional/{core,permissions}` |
| methodology | `methodology/ooda/core` |
| capability | `capability/{tool_thinking,tool_request_protocol,knowledge_discovery,knowledge_protocol}` |
| intent | `intent/explain/core` |

**Baseline being beaten: 0 atoms, 0 tokens.** The pre-S17 chat turn compiled nothing at all, so
every one of these 17 is new information the harness now places. The interesting ones:

- **`protocol/piggyback/*`.** The chat turn *already demands* a piggyback envelope — the JSON
  schema, the THOUGHT-FIRST ordering and the "DO NOT speak to the user until AFTER you have written
  the complete control_packet" line are hand-written prose in `helpers_articulation.go:284-300`,
  sent in the user message. The corpus has authored, evolvable atoms for exactly that protocol and
  the chat turn was not receiving them.
- **`safety/constitution*` — five atoms.** The prime directive, the forbidden-action list and the
  permission model were reaching every shard and not the interactive turn. The turn a human types
  into was the one with no constitution in its window.
- **`intent/explain/core` — one, not eight.** `/intent` is permissive, so an *unset* verb admits all
  eight of `intent/{brainstorm,create,design,explain,refactor,research,review,test}/core` (all
  mandatory, all shard-agnostic) and hands the model eight task framings to choose between. Setting
  the verb is worth seven atoms of contradiction avoided.

Cost: 4,451 tokens of a 198,384-token budget — **2.2%**. Assembled system prompt 24,271 bytes =
6,461 persona + 17,810 skeleton. One compile, ~0.5 s cold in test (S19 measured ~344 ms for a
compile clone), and the prompt cache keys on `cc.Hash()` so a repeated context is a hit.

`candidates=914 selected=18 included=17` — one atom was selected and did not make the final
prompt. Not budget (2.2% used); noted in `## Open`.

## Design

**Shape of the main-chat system prompt, in order, under one budget:**

1. `architectPersona` at byte offset 0, byte-for-byte, always.
2. the JIT-compiled chat skeleton.
3. the kernel-derived context — which is *inside* (2), not a third concatenation: the compiler's
   `collectKernelInjectedAtoms` turns `injectable_context(ShardID, Atom)` rows into a mandatory
   `kernel/context/<hash>` atom at priority 95 and the selector places it. So kernel context is
   budgeted, ordered and deduplicated like everything else rather than appended past the budget.

**The budget is one sum.** `buildChatCompilationContext` sets
`cc.TokenBudget = GetEffectiveJITConfig().TokenBudget - architectPersonaTokens()`. The selector
spends `TokenBudget - ReservedTokens`, so `persona + skeleton ≤ jit.token_budget - reserve`. The
persona stopped being a free rider; it is the first line item.

**The chat is `/chat`, a shard type no atom declares — deliberately.** `/shard` is fail-closed
(`jit_compiler.mg:143-146`), so this decides by itself which half of the corpus the turn can see.
The main chat turn is the *orchestrator* that routes work to shards; the persona's own SHARD ROUTING
table says so. Giving it `/coder` would hand it the Coder's INVESTIGATE FIRST protocol for work it
never performs. Leaving it empty is worse — that is the recorded 114-atom / 25-identity failure.
`/chat` yields precisely the shard-agnostic corpus: the atoms whose authors declared no shard
because they belong to whoever is speaking. Derived from the corpus, not picked from it.

**Every fail-closed dimension is named** (`/shard`, `/mode`, `/provider`, `/model`), because an
unnamed one silently retires a whole population of atoms with no error and no log line.
`servingIdentity()` unwraps the broker layers around `m.client` for provider/model, mirroring
`Executor.servingIdentity`.

**The retrieval query is the user's own words**, read off `m.history` — `handleSubmit`
(`model_handlers.go:153`) appends the user message before `processInput` runs, so this turn's
utterance is already on the model. `AtomSelector` gates vector search on a non-empty
`SemanticQuery`, so an empty one turns the probabilistic half of the architecture off entirely.

**No signature changed.** `articulationSystemPrompt()` stays the production entry point that
`persona_test.go` pins, takes the deadline from `m.shutdownCtx`, and the verb travels on the model
as `turnIntentVerb` (set by `processInput` on its own copy after perception).

**Degradation is always toward identity, never away from it.** No compiler, no budget, or a failed
compile ⇒ the persona alone, logged. The persona is never the thing that gets shed.

**The persona stays a Go constant.** The file header used to justify that with "no atom of any kind
reaches this prompt", which this change falsifies, so the justification was rewritten to the real
one: an atom's position is the selector's decision and a mandatory atom can be superseded
(`mandatory_superseded/1`). "First, always, whole, byte for byte" is a stronger guarantee than the
corpus offers anything. Publishing the persona as an atom would trade a guarantee for a ranking.

## Implementation

_(pending)_

## Tests

_(pending)_

## Open

_(pending)_
