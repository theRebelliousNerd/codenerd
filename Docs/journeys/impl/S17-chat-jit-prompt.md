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

_(pending)_

## Design

_(pending)_

## Implementation

_(pending)_

## Tests

_(pending)_

## Open

_(pending)_
