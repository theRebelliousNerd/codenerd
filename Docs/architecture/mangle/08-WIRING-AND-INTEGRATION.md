# 08 — Wiring and Integration: mangle

> Last verified: 2026-07-13  
> How mangle is registered, constructed, and invoked at runtime.

## 1. Kernel boot

```
NewRealKernel / NewRealKernelWithWorkspace
  → loadMangleFiles()          # schemas, policy, learned from disk/embed
  → loadPredicateCorpus()
  → filterBootFacts (ephemeral strip)
  → evaluate()                 # first fixpoint
```

### Schema validator

`kernel_init.go` constructs:

```go
k.schemaValidator = mangle.NewSchemaValidator(k.schemas, learnedText)
```

Refreshed when learned policy changes (`refreshSchemaValidatorLocked`).

### Parse funnel

All kernel parse goes through:

```
core.parseUnit → mangle.ParseUnit → parse.Unit  (under parseMu)
```

### Evaluation wiring

`kernel_eval.go` `evaluate()`:

1. Policy dirty → `rebuildProgram` (parse+analyze+stratify) + `invalidateDiffEngineLocked`.
2. If `features.IsDiffEvalEnabled()` && no proof recorder && no external predicates:
   - `evaluateDiffLocked` builds `mangle.Engine` + `NewDifferentialEngine` + `EnableUnifiedFastPath`.
   - Applies atom deltas via `ApplyAtomDelta`.
   - `CopyAllFactsTo` into kernel store.
3. Else full path: fresh store + `EvalStratifiedProgramWithStats` with gas + optional externals + provenance.

Env / config:

- `CODENERD_DIFF_EVAL` / `.nerd/config.json` `features.diff_eval` via `features.IsDiffEvalEnabled()`.

## 2. System shards — generation path

### ExecutivePolicy

- Owns `*feedback.FeedbackLoop` created with `DefaultConfig()`.
- Resets validation budget at session start.
- Autopoiesis (`executive_autopoiesis.go`) calls `GenerateAndValidate` when proposing learned rules.
- Early-exits if `IsBudgetExhausted` / `CanRetryPrompt` false.

### ConstitutionGate

- Own FeedbackLoop for related generation gates.
- Budget checks before autopoiesis-adjacent work.

### Legislator / mangle_repair

- FeedbackLoop + `synth` modes for structured rule emission and repair.
- Align with JIT prompt atoms / predicate selection when wired.

## 3. CLI

| Command | Wiring |
|---------|--------|
| `nerd mangle-check` | `cmd/nerd/cmd_mangle_check.go` imports mangle; validates corpus |
| `nerd mangle-lsp` | LSPServer over Engine / documents |
| `nerd query` | May exercise Engine query paths |

## 4. Autopoiesis / ouroboros

`internal/autopoiesis/ouroboros.go` depends on `mangle` + `transpiler` for self-modification loops (PRD header in differential.go references Autopoiesis and Reasoning shards as consumers).

## 5. Browser / honeypot

Browser package constructs or uses `mangle.Engine` for logic over session/honeypot facts (tests exercise integration).

## 6. Perception / transparency / world

| System | Integration |
|--------|-------------|
| Perception taxonomy | Stores/queries via Engine-shaped APIs |
| Transparency explainer | Proof trees / mangle types |
| world/lsp | Wraps `LSPServer` |

## 7. Prompt JIT selector

`internal/prompt/predicate_selector.go` implements `SelectForContext` matching `feedback.PredicateSelectorInterface`. When set on FeedbackLoop, retry prompts get **context-relevant** predicates instead of full Decl dump.

## 8. Wiring audit checklist (before “unused” claims)

| Artifact | Check |
|----------|-------|
| `intent_routing.mg` | Grep load sites in core defaults / policy merge |
| `IntersectSIMD` | Grep call sites outside tests |
| `ProofTreeTracer` | Transparency + CLI glass-box |
| `RegisterVirtualPredicate` | Diff consumers (lazy file content etc.) |
| `Persistence` implementations | Store packages implementing interface |
| Sanitizer unlocked parse | Treat as known partial until fixed |

## 9. Fact-flow end-to-end (with mangle)

```
User message
  → perception transducers assert user_intent / focus facts (kernel EDB)
  → kernel evaluate (mangle full or differential)
  → IDB: next_action, permitted, …
  → VirtualStore dispatches permitted actions
  → shards may generate new Mangle via FeedbackLoop
  → learned rules merge → policyDirty → rebuildProgram
  → next OODA cycle
```

Constitutional gate: even if mangle derives candidate actions, **execution** requires `permitted` in core policy — default deny.
