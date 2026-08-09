# mangle — Implemented Spec (Deep-Dive)

> Last verified against codebase: **2026-07-13**  
> Status: Living Reference Document  
> Language: Go  
> Module path: `codenerd/internal/mangle`  
> Subpackages: `feedback`, `synth`, `transpiler`  
> Upstream: `codeberg.org/TauCeti/mangle-go` (analysis, ast, engine, factstore, parse, unionfind, provenance used by core)  
> Scale: **21** non-test Go sources; **40** test files; **1** package-local `.mg` (`intent_routing.mg`)

## 1. Overview

`internal/mangle` is the **production-grade Google Mangle substrate** for codeNERD. It adapts the TauCeti `mangle-go` library into a Cortex-oriented API: typed facts, stratified evaluation with **inference gas limits**, optional persistence, differential evaluation, LLM generation feedback, structured synthesis, schema drift gates, proof trees, and a Mangle LSP.

It implements the **library half** of the hollow-kernel pattern:

| Layer | Responsibility |
|-------|----------------|
| `internal/core` RealKernel | Owns schemas/policy/learned strings, OODA evaluate loop, `permitted` derivation, VirtualStore externals |
| `internal/mangle` | Parsers (locked), Engine wrapper, DifferentialEngine, validators, feedback, synth |
| Policy `.mg` corpus | Declares predicates and rules that define executive behavior |

### Key characteristics

| Property | Value |
|----------|-------|
| Fact representation | `mangle.Fact{Predicate, Args []any, Line, Timestamp}` |
| Default fact limit | 100_000 |
| Default derived gas | 100_000 for direct reusable `Engine`; 500_000 for both full and differential kernel paths when kernel configuration is unset |
| Default query timeout | 30s |
| Auto-eval | On unless `MANGLE_AUTO_EVAL=0` |
| Parse concurrency | Serialized process-wide (`parseMu`) |
| Diff stratification (library) | **2-bucket**: EDB=0, IDB=1 (intentional performance choice) |
| Unified fast path | Opt-in single `EvalStratifiedProgramWithStats` |
| Feedback retries | 3 per rule hash; 20 session budget (defaults) |
| Synth format | `mangle_synth_v1` JSON |

### High-level control flow

```
                    ┌─────────────────────────────────────┐
                    │  LLM / shard / CLI / perception     │
                    └──────────────┬──────────────────────┘
                                   │ rules / facts / queries
          ┌────────────────────────┼────────────────────────┐
          ▼                        ▼                        ▼
   feedback.FeedbackLoop    Engine.AddFacts          DifferentialEngine
   + synth + sanitizer      Engine.Query             ApplyAtomDelta
          │                        │                        │
          ▼                        ▼                        ▼
   SchemaValidator          evalWithGasLimit         unifiedStore OR
   HotLoadRule / Decl       stratified fixpoint      per-stratum chains
          │                        │                        │
          └────────────────────────┴────────────────────────┘
                                   │
                                   ▼
                     kernel store / next_action / permitted
```

Fact-flow (system-wide):

```
user input → perception → user_intent → kernel next_action
  → VirtualStore / shards → articulation
```

Mangle participates as the **evaluation engine** and as the **gate** on learned rule admission.

---

## 2. Implementation status

| Component | Status | Notes |
|-----------|--------|-------|
| `Engine` load / fact / query | **Implemented** | `engine.go` |
| Stratified eval + gas | **Implemented** | `evalWithGasLimit` |
| Persistence warm / replace-by-file | **Implemented** | `Persistence` interface |
| `ParseUnit` / `ParseAtom` lock | **Implemented** | Used by engine + core |
| `DifferentialEngine` legacy path | **Implemented** | Per-stratum + `ChainedFactStore` |
| Unified fast path | **Implemented** | Kernel enables after construct |
| Snapshot COW | **Implemented** | Full fact copy (not structural COW) |
| Virtual / lazy predicates | **Implemented** | `FactStoreProxy` + `RegisterVirtualPredicate` |
| `SchemaValidator` + forbidden heads | **Implemented** | Constitutional gate for learned |
| `FeedbackLoop` | **Implemented** | Shards + kernel_policy |
| PreValidator + ErrorClassifier | **Implemented** | Regex + pattern banks |
| Sanitizer (transpiler) | **Implemented** | Atom interning, agg, safety |
| MangleSynth compile/decode | **Implemented** | Prefer/require modes in loop |
| AtomValidator / RepairLoop | **Implemented** | GCD-style atom repair |
| ProofTreeTracer | **Implemented** | Query-oriented traces |
| LSPServer | **Implemented** | Document / def / ref / diag / complete |
| SIMD intersect | **Implemented** | amd64 + generic tags |
| intent_routing.mg | **Source present** | Declarative intent routing rules |
| Diff path + external predicates | **Partial** | Kernel **falls back** to full eval |
| Diff path + created-fact gas | **Implemented** | `evalOptions` forwards the configured positive limit on unified atom, legacy atom, and legacy fact routes |
| Production parsing via ParseUnit/ParseAtom | **Implemented** | Core, sanitizer, synth, and system adapters use the shared chokepoint; source scan and mixed-caller race tests guard it |
| True delta propagation | **Not implemented** | Re-eval / unified re-eval instead |

**Overall:** living production package — **not** pre-implementation.

---

## 3. Source inventory

### 3.1 Layout

```
internal/mangle/
  engine.go differential.go grammar.go schema_validator.go
  parse_lock.go proof_tree.go lsp.go simd_intersect_*.go
  intent_routing.mg
  feedback/   # LLM validate-retry
  synth/      # structured JSON → Mangle
  transpiler/ # Sanitizer frontend
```

### 3.2 Top non-test sources (line counts ≈)

| Path | Lines | Purpose |
|------|------:|---------|
| `engine.go` | ~1100 | Engine, fact conversion, query, limits |
| `lsp.go` | ~1055 | LSP server |
| `differential.go` | ~866 | Diff eval, proxy, snapshot |
| `grammar.go` | ~787 | AtomValidator, RepairLoop |
| `proof_tree.go` | ~482 | Derivation traces |
| `feedback/loop.go` | ~476 | GenerateAndValidate |
| `feedback/prompt_builder.go` | ~446 | Retry prompts |
| `synth/compile.go` | ~424 | Spec → Mangle text |
| `schema_validator.go` | ~412 | Decl drift + forbidden heads |
| `feedback/pre_validator.go` | ~402 | Fast AI error checks |
| `transpiler/sanitizer.go` | ~379 | Multi-pass repair |
| `synth/validate.go` | ~330 | Spec validation |
| `feedback/types.go` | ~253 | Categories, budgets |
| `feedback/error_classifier.go` | ~252 | Compiler error mapping |
| `synth/schema.go` | ~213 | JSON schema |
| `synth/decoder.go` | ~169 | Response JSON extraction |
| `synth/spec.go` | ~122 | Spec types |
| `feedback/normalize.go` | ~76 | Normalize / extract |
| `simd_intersect_amd64.go` | ~51 | SIMD |
| `parse_lock.go` | ~44 | Global parse lock |

### 3.3 Test surface (representative)

| Suite | Focus |
|-------|-------|
| `engine_test.go` | Limits, concurrency, coercion, batches, unicode |
| `differential_test.go` | Stratification, incremental, snapshot, lazy load |
| `mangle_validation_test.go` | Real schemas/policy/shard GL files, safety rules |
| `grammar_*_test.go` | Validator, fuzz ParseAtom, benchmarks |
| `parse_lock_test.go` | Concurrent ParseUnit/ParseAtom |
| `feedback/*_test.go` | Loop, classifier, pre-validator, JIT selector hooks |
| `synth/*_test.go` | Compile, decode, validate |
| `transpiler/*_test.go` | Sanitizer atoms |
| `torture_test.go` | Stress / edge differential |

---

## 4. Engine deep dive (`engine.go`)

### 4.1 Types

```go
type Config struct {
    FactLimit, DerivedFactsLimit, QueryTimeout int
    AutoEval bool
    SchemaPath, PolicyPath string
}

type Engine struct {
    // ConcurrentFactStore over SimpleInMemoryStore
    // programInfo, strata, predToStratum
    // predicateIndex, schemaFragments, factCount, derivedCount
    // autoEval, persistence, fileFacts reverse index
}

type Fact struct {
    Predicate string
    Args []any
    Line int
    Timestamp time.Time
}

type Persistence interface {
    ReplaceFactsForFile(ctx, file, facts, contentHash) error
    LoadFacts(ctx) ([]Fact, error)
    GetFileStates(ctx) (map[string]string, error)
}
```

### 4.2 Lifecycle

```
NewEngine(cfg, persistence)
    │
    ├─ LoadSchema / LoadSchemaString  ──► ParseUnit ──► schemaFragments
    │                                         │
    │                                         ▼
    │                                  rebuildProgramLocked
    │                                   AnalyzeOneUnit
    │                                   analysis.Stratify  → strata cache
    │                                   QueryContext build
    │
    ├─ WarmFromPersistence (optional, autoEval off during bulk)
    │
    ├─ AddFact / AddFacts / ReplaceFactsForFile*
    │       insertFactLocked → factToAtomLocked
    │       if autoEval → evalWithGasLimit
    │
    ├─ Query / GetFacts / GetFactsSeq / EvaluateRule
    │
    └─ Clear | Reset | Close
```

### 4.3 Schema load and stratification

`LoadSchemaString` appends a `parse.SourceUnit` fragment and rebuilds:

1. Concatenate all fragments’ clauses + decls.
2. `analysis.AnalyzeOneUnit`.
3. `analysis.Stratify` on EDB/IDB/rules — full fine-grained strata for **Engine** eval.
4. Build `predicateIndex` and `QueryContext{PredToRules, PredToDecl, Store}`.

**Invariant:** no facts or queries before schema load → `errNoSchemas`.

### 4.4 Fact insertion and typing

`factToAtomLocked`:

1. Predicate must exist in `predicateIndex` (Declared).
2. Arity must match symbol arity.
3. Per-arg expected type from first Decl bounds (`/name`, `/string`, `/number`, `/float64`, …).
4. `convertValueToTypedTerm` coerces Go values:
   - NameType forces `/`-prefixed names.
   - StringType forces string constants (no auto-name).
   - Heuristic **Auto-Atomizer**: identifier-like strings become `/name` when not strict StringType.
   - Numbers, floats, bools, lists, JSON-encoded maps/structs.

**Skew risk:** Kernel facts use `types.Fact.ToAtom()` on the differential path specifically so encoding matches kernel semantics rather than Engine auto-atomizer heuristics (`ApplyAtomDelta` comment).

### 4.5 Gas-limited evaluation

`evalWithGasLimit`:

- Requires cached `strata` / `predToStratum`.
- Optional `mengine.WithCreatedFactLimit(DerivedFactsLimit)`.
- Calls `EvalStratifiedProgramWithStats`.
- Tracks derived delta via `EstimateFactCount` before/after.
- Logs Info only when `derivedThisRound >= 100` (noise control).

`RecomputeRules` forces eval with progress ticks every 30s.

### 4.6 Query

`Query(ctx, query string)`:

1. `parseQueryShape` → atom + variable bindings.
2. Requires Decl + at least one mode.
3. Applies default timeout if context has none.
4. Goroutine runs `queryContext.EvalQuery` with cancel checks.
5. Returns `QueryResult{Bindings, Duration}`.

### 4.7 File-scoped facts

`fileFacts map[string][]ast.Atom` reverse index:

- On insert, if arg0 looks like a path, index under canonical path.
- `ReplaceFactsForFile` removes prior atoms for that file, inserts new, optional persist with content hash.
- Enables world-model style “reindex this file” without full Clear.

### 4.8 Limits and warnings

| Limit | Behavior |
|-------|----------|
| FactLimit | Hard error on insert when exceeded |
| 85% capacity | One-shot warn via logging |
| DerivedFactsLimit | Eval option; errors bubble as eval failure |
| QueryTimeout | Context timeout → timed-out error |

---

## 5. Differential engine deep dive (`differential.go`)

### 5.1 Why it exists

Full rebuild of a large EDB+IDB program every OODA tick is expensive. DifferentialEngine aims to:

1. Hold facts in stratum-aware stores.
2. On delta, re-evaluate from the lowest changed stratum upward.
3. Support snapshot isolation for simulation.
4. Optionally collapse to **one** stratified eval over a unified store (fast path).

### 5.2 Structure

```go
type DifferentialEngine struct {
    baseEngine  *Engine
    programInfo *analysis.ProgramInfo
    strataStores []*KnowledgeGraph      // per-stratum facts
    predStratum  map[ast.PredicateSym]int
    strataRules  [][]ast.Clause
    strataNodesets []analysis.Nodeset   // cached for legacy loop
    strataPredMaps []map[ast.PredicateSym]int
    // Unified fast path:
    unifiedStore factstore.FactStore
    unifiedStrata []analysis.Nodeset
    unifiedPredToStratum map[ast.PredicateSym]int
}
```

### 5.3 Stratification policy: intentional 2-bucket

`computeStrata` maps:

- **EDB** (never a rule head) → stratum **0**
- **IDB** (appears as rule head) → stratum **1**

Fine-grained `analysis.Stratify` was measured to **hurt** wall time on codeNERD’s workload: EDB deltas force re-eval of all higher strata with per-call engine setup dominating. With 2 buckets, one eval over the rule set lets the engine’s seminaive evaluator work internally.

Documented experiment (2026-05-28 in source comments): fine-grained stratify caused `TestKernelDifferentialEval` timeouts (60s) vs ~1s with 2-bucket.

### 5.4 Legacy path: `ApplyDelta` / `ApplyAtomDelta`

```
for each atom:
  place in strataStores[predStratum]
  track minChangedStratum

for s = minChangedStratum .. max:
  build ChainedFactStore{base: 0..s-1, overlay: s}
  subset ProgramInfo.Rules = strataRules[s]
  EvalStratifiedProgramWithStats(subset, cached nodeset, chain)
```

`ChainedFactStore` implements `factstore.FactStore`: writes go to overlay; reads union base+overlay.

### 5.5 Unified fast path

`EnableUnifiedFastPath()`:

1. Runs full-program `analysis.Stratify` (fine-grained for the *engine’s* single call).
2. Allocates `unifiedStore`.
3. Subsequent `ApplyAtomDelta`:
   - Adds atoms to unified store only (when fast path active — skips per-stratum bookkeeping in the fast branch).
   - One `EvalStratifiedProgramWithStats` over full program on that store.

`CopyAllFactsTo` prefers unified store when set — kernel materializes results back into its own store this way.

**Idempotent** enable; used by kernel after `NewDifferentialEngine`.

### 5.6 Snapshot

`Snapshot()` deep-copies each stratum store’s facts into a new DifferentialEngine sharing `baseEngine` / `programInfo` / maps. This is **logical** isolation (copy), not zero-copy COW of the underlying map structure.

### 5.7 Query on differential

Builds a chain of all strata stores, constructs a temporary `QueryContext`, then:

1. Emits matching stored facts via `GetFacts`.
2. If rules exist for the predicate, top-down `EvalQuery`.

### 5.8 Lazy / virtual predicates

`FactStoreProxy` wraps a store; on `GetFacts` for a registered predicate, runs a loader that may populate the store.

`RegisterVirtualPredicate(predicate, loader func(string)(string,error))` wraps stratum-0 store and inserts 2-arg string atoms on miss (key → value).

### 5.9 Kernel integration contract

From `internal/core/kernel_eval.go` (not in this package, but defines real usage):

| Condition | Path |
|-----------|------|
| `features.IsDiffEvalEnabled()` false | Full rebuild |
| `proofRecorder != nil` | Full (provenance) |
| External predicates active | Full (diff does not forward externals) |
| Policy dirty / retract / clear | Invalidate diff engine |
| Else | `evaluateDiffLocked` → `ApplyAtomDelta` + `CopyAllFactsTo` |

Created-fact gas is enforced on differential calls. External predicates and
provenance still use the full path; keep those explicit fallbacks until their
option contracts are separately implemented and tested.

The kernel resolves an unset/non-positive `derivedFactLimit` through
`effectiveDerivedFactLimitLocked`: full and differential kernel evaluation both
receive 500,000. This intentionally overrides the reusable Engine's independent
100,000 package default when the Engine is constructed for the kernel diff path.
`TestKernelEval_ZeroConfigDerivedFactLimitParity` locks that boundary.

---

## 6. Feedback loop deep dive (`feedback/`)

### 6.1 Purpose

LLM-generated Mangle is **wrong by default** (Prolog habits, quoted atoms, bad aggregation, unbound negation). The feedback package:

1. Catches errors **before** and **after** compilation.
2. Auto-repairs when safe.
3. Returns structured errors into progressive prompts.
4. Enforces **budgets** so sessions cannot burn infinite tokens.

### 6.2 Core types (`types.go`)

| Type | Role |
|------|------|
| `ErrorCategory` | parse, atom_string, aggregation, missing_period, unbound_negation, undeclared, stratification, type_mismatch, prolog_negation, syntax |
| `ValidationError` | Category, line/col, wrong/correct/suggestion, AutoFixed |
| `RetryConfig` | MaxRetries, SessionBudget, timeouts, auto-repair flags |
| `ValidationBudget` | Per-rule hash + session counters (mutex) |
| `OutputProtocol` | `mangle_rule` vs `mangle_synth_json` |
| `SynthMode` | Off / Prefer / Require |

`IsAutoRepairable` marks categories the Sanitizer can fix (including partial unbound-negation via generator injection).

### 6.3 Pipeline: `GenerateAndValidate`

```
for attempt 1..MaxRetries:
  budget check (session + per-prompt hash)
  build prompt (+ feedback on retries; JIT predicates if selector set)
  LLM.Complete(per-attempt timeout)
  if synth mode:
    DecodeSpec → Compile → SingleClause (require may force retry)
  else:
    ExtractRuleFromResponse → NormalizeRuleInput
  PreValidator.Validate
  QuickFix + Sanitizer.Sanitize
  validator.HotLoadRule (sandbox parse/load)
  validator.ValidateLearnedRule (schema + forbidden heads)
  success → return GenerateResult
exhausted → error
```

Interfaces:

- `LLMClient` / optional `TracingLLMClient` (shard attribution).
- `RuleValidator` (`HotLoadRule`, `ValidateLearnedRule`, `GetDeclaredPredicates`).
- Optional `PredicateSelectorInterface` for JIT predicate lists.

### 6.4 PreValidator (`pre_validator.go`)

Fast regex bank for common AI mistakes:

- `"active"` → should be `/active` (atom vs string).
- `\+` Prolog negation → `!`.
- `fn:Count` casing / missing `|> do`.
- Missing terminating period.
- Colon atoms `:active` → `/active` (QuickFix word list).

`QuickFix` applies deterministic string rewrites without full parse.

### 6.5 ErrorClassifier (`error_classifier.go`)

Maps compiler error text to categories (stratification, type mismatch, etc.), extracts line:col, attaches wrong line from source via `ClassifyWithContext`.

### 6.6 PromptBuilder (`prompt_builder.go`)

Progressive strategy:

- Always: syntax reminders (atoms, negation, aggregation, variables, periods, bind-before-negate).
- Attempt ≥2: tighter constraints + examples.
- Final attempt: simplify suggestion.
- Lists available predicates and domain examples.

### 6.7 Integration sites

| Caller | Path |
|--------|------|
| Executive autopoiesis | `internal/shards/system/executive_autopoiesis.go` |
| Constitution gate | `constitution.go` (budget checks) |
| Legislator | `legislator.go` + synth |
| mangle_repair | `mangle_repair.go` + synth |
| Kernel policy helpers | `kernel_policy.go` |

Session start should `ResetBudget` (executive does this).

---

## 7. Synth (`synth/`)

### 7.1 Contract

LLM emits **structured JSON**, not freehand Mangle:

```json
{
  "format": "mangle_synth_v1",
  "program": {
    "clauses": [
      {
        "head": { "pred": "next_action", "args": [ { "kind": "name", "value": "/run_tests" } ] },
        "body": [
          { "kind": "atom", "atom": { "pred": "test_state", "args": [ { "kind": "name", "value": "/failing" } ] } }
        ]
      }
    ]
  }
}
```

### 7.2 Pipeline

1. `DecodeSpec` — extract JSON (fenced code, piggyback surfaces).
2. `ValidateSpec` — structural rules + options (`RequireSingleClause`, allow decls/package/use).
3. `Compile` — render Package/Use/Decl/Clause lines → `mangle.ParseUnit` → optional `AnalyzeOneUnit`.
4. `Result{Source, Clauses, Decls}` — FeedbackLoop takes `SingleClause()` when configured.

### 7.3 Design intent (`synth/README.md`)

Long-term: VirtualStore tool `mangle_synth_tool` so agents never emit raw Mangle. Current code already supports the compiler path; tool registration is product wiring.

---

## 8. Transpiler / Sanitizer

`transpiler.Sanitizer` multi-pass:

1. **Preprocess** SQL-style `Res = count(Var)` → `llm_agg(...)`.
2. **Parse** to AST.
3. **Atom interning** — `"string"` → `/atom` using schema-aware `AtomValidator`.
4. **Aggregation repair** — temp agg → `|> do fn:group_by` form.
5. **Safety injection** — unbound negation gets a generator binding when possible (`rectifySafety`).
6. **Serialize**.

`UpdateFromProgramInfo` keeps predicate type maps current after kernel rebuild.

**Verified:** `Sanitize` and `SanitizeAtoms` call `mangle.ParseUnit`; the same
process-wide mutex is shared with core, synth, system adapters, and ParseAtom.

---

## 9. Schema validator

### 9.1 Role

Prevents learned rules from:

1. Using **undeclared** body predicates (no data source → dead rules).
2. Defining **forbidden heads** that spoof the control plane.
3. Arity mismatches on declared heads.
4. Undeclared **facts** (learned EDB for unknown preds).

### 9.2 Forbidden learned heads (excerpt)

| Predicate | Reason |
|-----------|--------|
| `permitted`, `safe_action` | Constitution core-owned |
| `admin_override`, `signed_approval` | User/admin only |
| `pending_action` | Executive shard |
| `permitted_action`, `permission_check_result` | Constitution gate |
| `routing_result` | Tactile router |
| `execution_result` | VirtualStore |
| `system_shard_state` | Supervisor |

### 9.3 RuleValidator surface

`HotLoadRule` appends `.` if needed, `ParseUnit`, then `ValidateLearnedRule` — matches feedback interface so one type serves both kernel and loop.

Decl loading is **regex-based** (`^Decl\s+name\s*\(`) plus head extraction from learned text — good for standard corpus; exotic multi-line Decls may need ProgramInfo path.

---

## 10. Grammar-constrained decoding (`grammar.go`)

`AtomValidator` holds:

- Core predicates (`user_intent`, `file_topology`, `diagnostic`, …) with arity/arg types.
- Name constants map.
- Updateable from `ProgramInfo`.

`RepairLoop.ValidateAndRepair` validates atom strings and builds LLM repair prompts with syntax rules.

This is **atom-level** GCD; full rule GCD is the feedback+sanitizer stack.

---

## 11. Parse lock (`parse_lock.go`)

Problem: ANTLR-generated parser mutates **process-global** ATN/DFA prediction caches. Concurrent `parse.Unit` races (confirmed under race detector during concurrent kernel + Engine construction).

Solution:

```go
var parseMu sync.Mutex

func ParseUnit(reader io.Reader) (parse.SourceUnit, error)
func ParseAtom(s string) (ast.Atom, error)
```

**Rule:** all packages must use these entry points. Core’s `parseUnit`, sanitizer,
synth compiler, and system fact adapters delegate to `mangle.ParseUnit`. An AST
test scans the root module's Go sources, including tests and function references,
for raw parser selectors outside `parse_lock.go`,
and a mixed-caller integration test passes under the race detector.

Parsing is cheap vs eval; serialization is not a throughput bottleneck.

---

## 12. Proof trees (`proof_tree.go`)

`ProofTreeTracer`:

- Indexes rules by head from `ProgramInfo`.
- `TraceQuery` runs engine query, builds `DerivationNode` trees (EDB vs IDB).
- Caches up to 100 traces by query string.

Used for glass-box / transparency explanations. Distinct from kernel’s optional `provenance.DerivationRecorder` on the full eval path.

---

## 13. LSP (`lsp.go`)

`LSPServer` maintains open documents, definitions, references, diagnostics, hover, completions. Tied to an `Engine` for program intelligence. CLI exposes via `cmd/nerd` mangle-lsp; `internal/world/lsp` also imports the package.

---

## 14. SIMD intersect

`IntersectSIMD(a, b []uint64)` — sorted-slice intersection:

- `simd_intersect_amd64.go` under `amd64 && simd` tags.
- Generic pure-Go otherwise.

Utility for join-style workloads; not the primary OODA path.

---

## 15. intent_routing.mg

Package-local Mangle source defining:

- `intent_action_type` from `user_intent` verbs (create/modify/delete/query).
- `persona(...)` selection from intent.
- Additional routing predicates with local Decls for standalone validation.

Depends on schemas from core/world. Treat as **declarative routing corpus**; confirm load/merge into kernel program before claiming runtime effect.

---

## 16. Integration map

### 16.1 Downstream importers (evidence)

| Package | Usage |
|---------|-------|
| `internal/core` | ParseUnit, SchemaValidator, DifferentialEngine, feedback, Engine in tests |
| `internal/shards/system` | FeedbackLoop, synth (legislator, mangle_repair, executive, constitution) |
| `internal/autopoiesis` | Engine, transpiler |
| `internal/browser` | Engine for honeypot / DOM session logic |
| `internal/perception` | Engine / taxonomy |
| `internal/transparency` | Proof / explainer |
| `internal/world/lsp` | LSPServer |
| `internal/system` | Factory wiring |
| `cmd/nerd` | mangle-check, mangle-lsp, query, browser, UI splitpane, chat types |
| `internal/prompt` | PredicateSelector implements feedback interface |

### 16.2 Upstream dependencies

- `codeberg.org/TauCeti/mangle-go/*` — parse, analysis, engine, factstore, ast, unionfind, builtin, packages
- `codenerd/internal/logging` — Kernel category
- `codenerd/internal/types` — shared types (engine)
- `codenerd/internal/config` — LLM timeouts (feedback defaults)

### 16.3 Fact-flow placement

```
perception asserts user_intent facts
        │
        ▼
core.RealKernel.evaluate ──► (diff?) mangle.DifferentialEngine
        │                    or full EvalStratifiedProgramWithStats
        ▼
derived next_action / permitted
        │
        ▼
VirtualStore executes
        │
        ▼
shards may FeedbackLoop → learned rules → kernel policy dirty → rebuild
```

---

## 17. Concurrency model

| Resource | Protection |
|----------|------------|
| Engine fields | `sync.RWMutex` |
| DifferentialEngine | `sync.RWMutex` + per-`KnowledgeGraph` mutex |
| parse | process-wide `parseMu` |
| ValidationBudget | mutex |
| ProofTreeTracer | RWMutex |
| LSPServer | RWMutex |
| Query execution | timeout via context + select |

`engine_test.go` includes `TestConcurrentAccess`.

---

## 18. Gaps pointer

See [03-GAP-ANALYSIS.md](03-GAP-ANALYSIS.md) and [TODO.md](TODO.md). Critical partials:

1. Diff path does not forward external predicates or provenance; core falls back
   to full evaluation for both. Created-fact limits are forwarded and tested.
2. Sanitizer/synth may call unlocked parse.
3. intent_routing.mg runtime wiring verification.
4. Proof tree vs provenance dual systems.
5. Regex Decl parser vs full analysis.

---

## 19. Verify commands

```powershell
go test ./internal/mangle/...
go test ./internal/mangle/feedback/...
go test ./internal/mangle/synth/...
go test ./internal/mangle/transpiler/...
go test ./internal/core -run Diff -count=1
```

If evaluation crashes at kernel level, look for `debug_program_ERROR.mg` dump (written by core `rebuildProgram` on analysis failure).

---

## 20. Maintenance notes

- Do **not** remove differential header mandates without replacing performance rationale.
- Prefer `UpdateFromProgramInfo` over hardcoding predicate lists.
- New LLM-facing rule writers should use FeedbackLoop (+ synth prefer/require).
- New predicates require `Decl` in schemas before facts or learned bodies.
- Keep this IMPLEMENTED_SPEC denser than inventory tables; update deep dives when eval or feedback contracts change.
