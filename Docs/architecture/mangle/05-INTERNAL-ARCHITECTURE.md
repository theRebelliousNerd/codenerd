# 05 — Internal Architecture: mangle

> Last verified: 2026-07-13

## Component diagram

```
┌──────────────────────────────────────────────────────────────────┐
│                     internal/mangle                                │
│                                                                    │
│  parse_lock ──► ParseUnit/ParseAtom ──► mangle-go/parse            │
│       ▲                                                            │
│       │                                                            │
│  Engine ── LoadSchema ──► analysis.AnalyzeOneUnit + Stratify       │
│     │  AddFacts / Query / evalWithGasLimit                         │
│     │                                                              │
│  DifferentialEngine ── wraps Engine.programInfo                    │
│     │  strataStores[] or unifiedStore                              │
│     │  ApplyAtomDelta / ApplyDelta / Snapshot / Query              │
│     │  FactStoreProxy (lazy)                                       │
│                                                                    │
│  SchemaValidator ── Decl map + forbidden heads + HotLoadRule       │
│  AtomValidator / RepairLoop ── GCD atoms                           │
│  ProofTreeTracer ── DerivationTrace                                │
│  LSPServer ── documents / defs / diags                             │
│                                                                    │
│  feedback.FeedbackLoop                                             │
│     ├─ PreValidator                                                │
│     ├─ ErrorClassifier                                             │
│     ├─ PromptBuilder                                               │
│     ├─ ValidationBudget                                            │
│     ├─ transpiler.Sanitizer                                        │
│     └─ synth.DecodeSpec/Compile (optional)                         │
└──────────────────────────────────────────────────────────────────┘
```

## Data flows

### A. Schema load (Engine)

```
.mg text → ParseUnit → SourceUnit fragments[]
  → AnalyzeOneUnit → ProgramInfo
  → Stratify → strata, predToStratum
  → predicateIndex, QueryContext
```

### B. Fact assert + auto-eval (Engine)

```
Fact → factToAtomLocked (Decl + arity + types)
  → ConcurrentFactStore.Add
  → factCount++ / fileFacts index
  → [autoEval] evalWithGasLimit → EvalStratifiedProgramWithStats
```

### C. Kernel differential eval (cross-package)

```
RealKernel.evaluate
  if policyDirty → rebuildProgram → invalidateDiffEngine
  if diff enabled && no proof && no externals:
      buildDiffEngineLocked:
        NewEngine → LoadSchemaString(schemas+policy+learned)
        NewDifferentialEngine → EnableUnifiedFastPath
      ApplyAtomDelta(factsSinceLastEval)
      CopyAllFactsTo(kernel store)
  else:
      evaluateFullLocked (fresh store + full stratified eval + options)
```

### D. LLM rule admission

```
Shard/LLM
  → FeedbackLoop.GenerateAndValidate
      → (synth?) DecodeSpec/Compile
      → PreValidator / Sanitizer
      → RuleValidator.HotLoadRule
      → ValidateLearnedRule (forbidden heads + Decl)
  → accepted rule string → kernel learned policy → policyDirty
```

## Key state machines

### Engine readiness

```
New → (no programInfo) ──LoadSchema──► Ready
Ready ──Reset──► New
Ready ──Clear──► Ready (facts empty, program kept)
```

### Differential validity (kernel)

```
nil ──build──► live
live ──retract/clear/policy rebuild──► nil (invalidate)
live ──successful ApplyAtomDelta──► live
```

### Feedback budget

```
fresh (sessionUsed=0)
  ──RecordAttempt──► ... until sessionBudget or maxPerRule
  ──Reset──► fresh
IsSessionExhausted → callers skip generation
```

## KnowledgeGraph

Per-stratum container: `store factstore.FactStore`, `isFrozen`, mutex. Used as layer in `strataStores`.

## ChainedFactStore read/write semantics

| Op | Behavior |
|----|----------|
| Add | Overlay only |
| GetFacts / Contains | Overlay then bases |
| ListPredicates | Union with de-dupe |
| Merge | Into overlay |

## Error taxonomy (feedback)

Categories map to repair strategies (auto vs re-prompt). See `feedback.ErrorCategory` and `IsAutoRepairable`.

## Threading assumptions

- Engine/Differential: external callers serialize via package mutexes; do not share unlocked program mutation.
- Parse: global lock.
- FeedbackLoop: one loop instance may be session-scoped; budget is mutex-safe; do not assume LLMClient is concurrent-safe unless documented.
