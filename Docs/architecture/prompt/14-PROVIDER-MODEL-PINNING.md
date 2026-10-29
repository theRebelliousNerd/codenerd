# Provider and model pinning

> Corpus: `prompt` | Live owner: `internal/prompt`, `internal/autopoiesis/prompt_evolution` | Added: 2026-08-17

## 1. The problem

A prompt atom can encode a claim about **one model's behavior**. Not about coding,
not about the user's project — about how a specific model responds to a specific
instruction. Examples of the shape:

- a workaround for a tokenizer quirk that mangles a particular fence style
- a refusal pattern one vendor exhibits and another does not
- a tool-call encoding one vendor gets wrong without an explicit reminder
- a context-window behavior that only appears past some depth on one model

The prompt evolution system (`internal/autopoiesis/prompt_evolution`) manufactures
exactly this kind of atom. Its loop is Execute → Judge → Evolve → Integrate: it
takes failures, asks an LLM to write atoms that would have prevented them, and
splices the survivors into future system prompts.

Before this change that loop had no notion of which model produced a failure.
`ExecutionRecord` recorded shard, problem type, actions, and verdict — never the
serving provider or model. Failures from every configured vendor were grouped by
`problem_type:shard_type` alone, one atom was generated per group, and that atom
was served to every model thereafter. **An atom learned from Gemini's failure
modes was delivered verbatim to Claude.** This is worse than a wasted atom: it
spends budget teaching one model to route around another model's defect, in the
imperative voice the corpus uses for real rules.

## 2. What already existed, and why it did nothing

`PromptAtom` had carried `Models []string` and `Providers []string` since the
original schema. They parsed from YAML, normalized, cloned, and projected into
`atom_selector(ID, /model, V)` and `atom_selector(ID, /provider, V)` facts.

They were inert at every other layer:

| Layer | State before |
|---|---|
| `CompilationContext` | No `Provider`/`Model` field, so no `compile_context(/provider, …)` was ever asserted |
| `jit_logic.mg` | Match rules for 11 dimensions; none for `/model` or `/provider` |
| `jit_compiler.mg` | Neither dimension in `regime_dimension`, and no tag emitted to gate on |
| `selector.go` `addTags` | Emitted 12 dimensions; not these two |
| `MatchesContext` | Checked 11 dimensions; skipped both |
| `prompt_evolution` | Never recorded provider/model at all |

So an atom declaring `providers: [/anthropic]` was silently served to every
vendor. The field looked like enforcement and was decoration — the failure mode
this document's mechanism is designed to make impossible.

## 3. The contract

An atom that declares `providers:` and/or `models:` is **pinned**: it is admitted
only on a compile whose `CompilationContext` names a matching provider/model. An
atom that declares neither is **unpinned** and is unaffected — this is the
overwhelming majority of the corpus, and pinning must never touch it.

Both dimensions are **fail-closed**. A pinned atom on a compile that never named a
provider/model is blocked, not admitted.

That choice deserves its justification, because the permissive reading is the
default elsewhere in the selector. A pinned atom asserts "this is true of model
X". A compile that cannot say which model it is serving cannot show that claim
holds. The two failure directions are not symmetric:

- **Strict (chosen).** A pinned atom sits out a compile that forgot to set
  `Provider`. The cost is bounded, local, and visible in the manifest.
- **Permissive (rejected).** Every vendor-specific workaround in the corpus lands
  in every prompt. The cost is unbounded, silent, and grows with the corpus —
  and the evolution loop manufactures pinned atoms continuously, so it grows on
  its own.

This mirrors the reasoning already recorded for `regime_dimension` in
`jit_compiler.mg`, and pinning joins that list rather than inventing a parallel
mechanism.

## 4. Canonical tokens

The mechanism rests entirely on both sides of a pin converging on one Mangle name
constant. This is the part most likely to break silently, so it is centralized in
`internal/prompt/pinning.go`.

The hazard is concrete. An atom's YAML says `models: [claude-opus-4]`. The runtime
reports `anthropic/claude-opus-4-20260501`. `factBuilder.writeAtom` preserves `-`
and `.` and treats `/` as a path separator, so `claude-opus-4` and
`claude_opus_4` are **two distinct constants** to the kernel. A pin that never
fires is worse than no pin, because it reads as enforcement.

Every value on both sides therefore passes through normalizers that emit only
`[a-z0-9_]` — and are consequently fixpoints of `writeAtom`:

| Function | Input | Output |
|---|---|---|
| `NormalizeProviderToken` | `"Anthropic"`, `"/openai"` | `anthropic`, `openai` |
| `NormalizeModelToken` | `"anthropic/claude-opus-4-20260501"` | `claude_opus_4_20260501` |
| | `"anthropic.claude-sonnet-4-v1:0"` | `claude_sonnet_4_v1` |
| `ModelFamilyToken` | `claude_opus_4_20260501` | `claude_opus_4` |
| `ModelPinTokens` | `"anthropic/claude-opus-4-20260501"` | `[claude_opus_4_20260501, claude_opus_4]` |

`ModelPinTokens` returns the exact token **and** the family token, and the
compilation context asserts both. An atom may therefore pin at either
granularity: an exact snapshot when snapshots are known to differ, or a family
when the guidance should survive the vendor's next release.

`ModelFamilyToken` strips only unambiguous release markers — a `yyyymmdd` stamp, a
`yyyy[_mm[_dd]]` run, or `latest`/`preview`/`exp`/`beta`/`stable`. A bare numeric
segment is never stripped, because in practice it is part of the name
(`claude_opus_4`, `gpt_4o`, `gemma4`) far more often than it is a version.
Stripping it would collapse distinct models onto one family and let a pin for one
match the other. When nothing is strippable, family equals exact and both are
emitted anyway, so a heuristic miss costs only exact-granularity matching.

## 5. Enforcement

### 5.1 Live path (Mangle)

`internal/prompt/selector.go#buildContextFacts` emits, per compile:

```text
current_context(/provider, /anthropic)
current_context(/model, /claude_opus_4_20260501)
current_context(/model, /claude_opus_4)
atom_tag("evolved/methodology/abc", /provider, /anthropic)
atom_tag("evolved/methodology/abc", /model, /claude_opus_4)
```

`internal/core/defaults/jit_compiler.mg` declares both as regime dimensions:

```prolog
regime_dimension(/provider).
regime_dimension(/model).
```

which arms the fail-closed rule already present in that file:

```prolog
blocked_by_context(Atom) :-
    regime_dimension(Dim),
    has_constraint(Atom, Dim),
    !satisfied_constraint(Atom, Dim).
```

`internal/core/defaults/policy/jit_logic.mg` gains `atom_has_provider_match/1` and
`atom_has_model_match/1` for scoring parity, so a pinned atom is scored as having
matched the dimension it was pinned to rather than looking like it matched on
nothing. Those rules are positive-match only; the binding enforcement is the
`regime_dimension` block above.

### 5.2 Fallback path (Go)

`PromptAtom.MatchesContext` applies the same rule for
`fallbackFleshSelection`, the path used when Mangle is unavailable. Provider uses
`matchSelector` directly (fail-closed on an empty context value); model matches if
**any** context token — exact or family — is in the atom's list.

Keeping the two paths in agreement is a standing hazard in this package; the
`shard_types` history in `jit_selection.mg` records what divergence cost last
time. `TestShardGating_RegimeDimensionsMatchKernelPolicy` couples the Go-side
list to the `.mg` file so a removed `regime_dimension` fact fails a test rather
than silently going permissive.

## 6. Where the serving identity comes from

`types.ModelIdentifier` is an optional interface on LLM clients:

```go
type ModelIdentifier interface {
    ModelIdentity() (provider, model string)
}
```

Implemented by all eight clients (`AnthropicClient`, `GeminiClient`,
`OllamaClient`, `OpenAIClient`, `OpenAICompatClient`, `OpenRouterClient`,
`XAIClient`, `ZAIClient`).

**Config is deliberately not the source.** `UserConfig.Model` is an optional
override, and each client substitutes a vendor default when it is empty, so on
the majority of setups the config value is `""` while a real model serves the
turn. Worse, `OpenAICompatClient.normalizeModel` can rewrite the model on the way
out (see `normalizeMetaModel`), so config can name a model the client never
calls. Pinning on config would attach atoms to the wrong identity — and the JIT
compiler enforces pins fail-closed, so a wrong pin is not a soft miss.

Two call sites populate the context:

- `internal/session/executor.go#servingIdentity` routes through `llmForVerb`, so a
  reasoning-intensive verb served by the planner slot compiles against the
  **planner's** identity. Compiling against the main client would pin-match atoms
  for a model that never sees them and block the ones the planner needs.
- `internal/session/spawner.go#servingIdentity` reads the main client, because
  `generateConfig` compiles one config for the subagent before any verb is in
  hand.

A client that does not implement the interface yields empty strings, which leaves
pinned atoms blocked and unpinned atoms untouched. It is logged, because the
symptom is otherwise a quietly smaller prompt.

## 7. Evolution provenance

| Type | Added |
|---|---|
| `ExecutionRecord` | `Provider`, `Model` (raw vendor spelling) |
| `JudgeVerdict` | `Provider`, `Model`, carried from the record being judged |
| `GeneratedAtom` | `Provider`, `Model`, `PinScope` |
| `EvolverConfig` | `AtomPinScope` |

`JudgeVerdict` carries provenance separately because the atom generator receives
only `[]*JudgeVerdict` — the record is gone by then. It is distinct from
`EvaluatedBy`: that names the model that *graded* the work, this names the model
that *produced* it, and the atom is pinned to the latter.

### 7.1 Pin scope

`PinScope` selects how tightly a generated atom is bound. The tradeoff is transfer
versus attribution, and how far a failure generalizes is not something the loop
can infer from the failure alone — so it is policy, not inference.

| Scope | Pins to | Use when |
|---|---|---|
| `model_family` *(default)* | provider + model family | Default. Behavior is stable across dated snapshots far more often than across models |
| `model` | provider + exact model | Snapshots are known to differ behaviorally |
| `provider` | provider only | Vendor-level traits: API envelope, tool-call encoding, refusal style |
| `none` | nothing | Restores pre-pinning behavior. Provenance is still recorded for review |

An omitted `atom_pin_scope` defaults to `model_family` rather than `none`: a
config written before pinning existed must not silently opt out of it. An
explicitly unknown value is a config error.

### 7.2 Grouping must match pinning

`FeedbackCollector.GetFailuresByProblemType(minCount, scope)` takes the scope and
groups by `FailureGroupKey{ProblemType, ShardType, Provider, Model}`, using the
same normalizers the generator pins with.

The coupling is the point. One atom generalizes exactly one group. If groups were
coarser than pins, a single atom would be generated from failures spanning two
different models and then pinned to whichever record happened to come first —
attributing one model's failures to another, with the authority of an enforced
pin. `TestGroupTokenEqualsGeneratedSelector` asserts the group's token and the
generated atom's selector are equal for every scope.

The meta-prompt also names the serving model, since the atom will only ever be
served back to it: guidance specific to that model is in scope and preferred over
vendor-neutral hedging.

## 8. Migration

`execution_records` gains `provider` and `model` via the existing
`ensureExecutionRecordColumns` pattern. Rows written before pinning keep `NULL`,
group under the empty pin, and yield unpinned atoms — the pre-pinning behavior,
applied only to pre-pinning data.

`CompilationContext.Hash` schema moves to `compilation-context-v3`. Pins are
hashed as canonical tokens, so `gpt-4o` and `openai/GPT-4O` share a cache entry
while two providers never do.

Evolved atoms on disk gain `provider`, `model`, and `pin_scope` in their YAML
wrapper. Existing files lack them, load as unpinned, and are unaffected.

## 9. Verification

| Test | Asserts |
|---|---|
| `internal/core/jit_pinning_test.go` | The **real kernel** blocks a pinned atom on a different provider, on a different model, and on a compile that names no provider; admits it on a match |
| `internal/prompt/pinning_test.go` | Token convergence across spellings, family derivation, Mangle-fixpoint safety, fact emission, cache identity |
| `internal/prompt/atom_pinning_test.go` | `MatchesContext` fail-closed semantics; unpinned atoms unaffected |
| `internal/autopoiesis/prompt_evolution/pinning_test.go` | Scope behavior, graceful degradation, group/pin granularity equality |
| `internal/prompt/shard_gating_test.go` | `regime_dimension(/provider)` and `(/model)` still present in `jit_compiler.mg` |

The kernel tests are non-vacuous: removing `regime_dimension(/provider).` from
`jit_compiler.mg` fails `TestPinnedAtomBlockedWhenProviderUnset` and passes the
rest, which is precisely the case the declaration adds.

## 10. Known limits

- **Family derivation is heuristic.** It handles the shapes vendors ship today.
  An unrecognized versioning scheme degrades to exact-only matching, never to a
  wrong match.
- **The spawner pins to the main client**, so a subagent that later routes a verb
  to the planner compiles its *config* against the main model. Per-turn
  compilation inside the subagent's own executor is correctly routed.
- **`atom_context_boost/1` has no producer**, so `atom_matches_context/2` in
  `jit_logic.mg` fires only its mandatory branch. This predates pinning and is
  why §5.1 describes the parity rules as scoring-only.
