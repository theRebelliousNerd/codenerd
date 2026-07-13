# 06 — Public API and Types: mangle

> Last verified: 2026-07-13  
> Exported surface that **matters** for integrators. File refs are absolute under repo root.

## Package `codenerd/internal/mangle`

### Construction & config

| Symbol | File | Role |
|--------|------|------|
| `Config` | `internal/mangle/engine.go` | FactLimit, DerivedFactsLimit, QueryTimeout, AutoEval, paths |
| `DefaultConfig()` | same | Production defaults; AutoEval via `MANGLE_AUTO_EVAL` |
| `NewEngine(cfg, persistence)` | same | Construct engine |
| `Persistence` | same | ReplaceFactsForFile / LoadFacts / GetFileStates |
| `ErrDerivedFactsLimitExceeded` | same | Sentinel (declared; limit errors may also be engine strings) |

### Engine methods

| Method | Role |
|--------|------|
| `LoadSchema` / `LoadSchemaString` | Parse+analyze program fragments |
| `WarmFromPersistence` | Bulk hydrate EDB |
| `AddFact` / `AddFacts` / `AddFactsContext` | Insert EDB |
| `ReplaceFactsForFile` / `WithHash` | File-scoped replace + optional persist |
| `ToggleAutoEval` / `RecomputeRules` | Bulk insert control |
| `Query` | Top-down / eval query with timeout |
| `GetFacts` / `GetFactsSeq` / `QueryFacts` / `PushFact` | Fact access helpers |
| `EvaluateRule` | Iterator over derived for predicate |
| `GetStats` / `GetDerivedFactCount` / `ResetDerivedFactCount` | Telemetry |
| `GetProgramInfo` | Read-only ProgramInfo pointer |
| `GetPersistence` | Accessor |
| `Clear` / `Reset` / `Close` | Lifecycle |

### Value types

| Type | Role |
|------|------|
| `Fact` | Predicate + Args + metadata; `String()` Datalog form |
| `QueryResult` | Bindings + Duration |
| `Stats` | TotalFacts, PredicateCounts, LastUpdate |

### Differential

| Symbol | File | Role |
|--------|------|------|
| `KnowledgeGraph` | `differential.go` | Per-stratum store |
| `NewKnowledgeGraph` | same | |
| `DifferentialEngine` | same | Incremental wrapper |
| `NewDifferentialEngine(base *Engine)` | same | Requires loaded schema |
| `EnableUnifiedFastPath` / `UnifiedFastPathEnabled` | same | Single-eval mode |
| `ApplyDelta` / `ApplyAtomDelta` / `AddFactIncremental` | same | Deltas |
| `CopyAllFactsTo` | same | Materialize union |
| `Snapshot` | same | Isolated copy |
| `Query` | same | Query across strata |
| `RegisterVirtualPredicate` | same | Lazy EDB |
| `ChainedFactStore` | same | Overlay+base FactStore |
| `FactStoreProxy` / `NewFactStoreProxy` | same | Lazy loaders |

### Parse

| Symbol | File | Role |
|--------|------|------|
| `ParseUnit` | `parse_lock.go` | **Process-wide** unit parse |
| `ParseAtom` | same | **Process-wide** atom parse |

### Schema validation

| Symbol | File | Role |
|--------|------|------|
| `SchemaValidator` | `schema_validator.go` | Decl + forbidden heads |
| `NewSchemaValidator(schemas, learned)` | same | |
| `LoadDeclaredPredicates` | same | Regex Decl extract |
| `ValidateRule` / `ValidateLearnedRule` / `ValidateRules` | same | |
| `HotLoadRule` | same | Parse + learned validation |
| `CheckArity` / `GetArity` / `SetPredicateArity` | same | Arity helpers |

### Grammar / GCD

| Symbol | File | Role |
|--------|------|------|
| `AtomValidator` | `grammar.go` | Predicate specs + validation |
| `NewAtomValidator` | same | Loads core predicates |
| `PredicateSpec` / `ArgSpec` / `ArgType` | same | |
| `ValidationResult` / `ValidationError` / `ErrorSeverity` | same | Atom-level (distinct from feedback types) |
| `RepairLoop` / `NewRepairLoop` | same | ValidateAndRepair + prompts |
| `UpdateFromProgramInfo` | validator & repair | Refresh from analysis |

### Proof trees

| Symbol | File | Role |
|--------|------|------|
| `ProofTreeTracer` / `NewProofTreeTracer` | `proof_tree.go` | |
| `TraceQuery` / `IndexRules` | same | |
| `DerivationNode` / `DerivationTrace` / `RuleSpec` | same | |
| `DerivationSource` (EDB/IDB) | same | |

### LSP

| Symbol | File | Role |
|--------|------|------|
| `LSPServer` / `NewLSPServer` | `lsp.go` | |
| `Document`, `Definition`, `Reference`, `Diagnostic` | same | |
| `CompletionItem` / kinds / severities | same | |
| `LSPRequest` / `LSPResponse` / `LSPError` | same | Wire protocol shapes |
| Document open/close, defs, refs, diags, complete | same | Methods |

### SIMD

| Symbol | File | Role |
|--------|------|------|
| `IntersectSIMD(a, b []uint64)` | `simd_intersect_*.go` | Sorted intersection |

---

## Package `codenerd/internal/mangle/feedback`

| Symbol | Role |
|--------|------|
| `ErrorCategory` + constants | Classification enum |
| `ValidationError` / `ValidationResult` | Structured feedback |
| `RetryConfig` / `DefaultConfig` | Loop configuration |
| `ValidationBudget` | Session/rule budgets |
| `FeedbackContext` / `OutputProtocol` / `SynthMode` | Prompt + synth mode |
| `LLMClient` / `TracingLLMClient` | LLM ports |
| `RuleValidator` | HotLoad + schema interface |
| `PredicateSelectorInterface` / `PredicateCatalogProvider` | JIT predicates |
| `FeedbackLoop` / `NewFeedbackLoop` | Main orchestrator |
| `GenerateAndValidate` / `ValidateOnly` / `PreValidateOnly` | Entry points |
| `SetPredicateSelector` / `SetSynthMode` / `UpdateFromProgramInfo` | Configuration |
| `GetBudget` / `ResetBudget` / `CanRetryPrompt` / `IsBudgetExhausted` | Budget API |
| `BuildEnhancedSystemPrompt` | Prompt helper |
| `NewPreValidator` / `NewErrorClassifier` / `NewPromptBuilder` | Subcomponents |
| `NormalizeRuleInput` / extract helpers | `normalize.go` |
| `ExtractPredicateFromError` / `FormatErrorForFeedback` | Classifier helpers |

---

## Package `codenerd/internal/mangle/synth`

| Symbol | Role |
|--------|------|
| `FormatV1` | `"mangle_synth_v1"` |
| `Spec` / `ProgramSpec` / clause/atom/expr specs | JSON model |
| `Options` / `DefaultOptions` / `Result` | Compile options/output |
| `Compile` / `ValidateSpec` / `DecodeSpec` / `FromResponse` | Pipeline |
| `SpecError` / `NewSpecError` | Structured errors |
| `ErrEmptyResponse` / `ErrMissingJSON` | Decode sentinels |

---

## Package `codenerd/internal/mangle/transpiler`

| Symbol | Role |
|--------|------|
| `Sanitizer` / `NewSanitizer` | Multi-pass repair |
| `Sanitize` / `SanitizeAtoms` | Entry points |
| `UpdateFromProgramInfo` | Schema refresh |

---

## Not exported (but important)

- `computeStrata`, `parseQueryShape`, `convertValueToTypedTerm`, `parseMu` — internal mechanics documented in IMPLEMENTED_SPEC.
- Kernel-only eval orchestration lives in `internal/core`, not here.
