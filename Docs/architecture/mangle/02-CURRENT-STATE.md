# Current state: the live Mangle substrate

> **VERIFIED CURRENT** on 2026-07-13 at
> `c8f21b46ec4b28529953094e0c18dac4dfd0c8eb`. The worktree was dirty; the
> fingerprint and review boundary are recorded in [_progress.md](_progress.md).

## Scope and inventory

The realized source root is `internal/mangle`. At the reviewed snapshot it has:

| Class | Count | Meaning |
|---|---:|---|
| Non-test Go files | 21 | root engine/differential/grammar/LSP/proof/schema/parse/SIMD plus feedback, synth, and transpiler packages |
| Go test files | 40 | unit, integration-like policy loading, fuzz seed, benchmark, concurrency, and torture coverage |
| Package-local Mangle files | 1 | `internal/mangle/intent_routing.mg` |
| Package-local design docs | 1 | `internal/mangle/synth/README.md` |

The largest implementation files remain `engine.go` (1,108 lines), `lsp.go`
(1,055), `differential.go` (866), and `grammar.go` (787). Counts describe the
reviewed tree; behavior claims below use symbols and tests.

```text
internal/mangle/
  engine.go                 reusable engine, facts, queries, persistence, gas
  differential.go           legacy strata mode, opt-in unified mode, snapshots
  parse_lock.go             process-wide ParseUnit and ParseAtom chokepoint
  schema_validator.go       learned-rule declarations, arity, protected heads
  grammar.go                atom validator and repair loop
  proof_tree.go             derivation trace model and renderers
  lsp.go                    .mg diagnostics and navigation server
  intent_routing.mg         package-local shadow/fixture rule corpus
  feedback/                 bounded model validation and feedback cycle
  synth/                    mangle_synth_v1 schema, validation, compiler, decoder
  transpiler/               sanitizer for atom, aggregation, and safety repair
```

## Realized components

| Component | Claim | Live evidence |
|---|---|---|
| `Engine` | **VERIFIED CURRENT** | `internal/mangle/engine.go#NewEngine` owns a concurrent fact store, configuration, schema fragments, query context, file-fact index, and optional persistence |
| Schema load and analysis | **VERIFIED CURRENT** | `internal/mangle/engine.go#Engine.LoadSchemaString` uses `ParseUnit`; `Engine.rebuildProgramLocked` calls `analysis.AnalyzeOneUnit` then `analysis.Stratify` |
| Fact insertion | **VERIFIED CURRENT** | `internal/mangle/engine.go#Engine.AddFacts` requires a loaded program, converts typed values, enforces the fact limit, inserts, and optionally evaluates |
| Full engine gas | **VERIFIED CURRENT** | `internal/mangle/engine.go#Engine.evalWithGasLimit` forwards `WithCreatedFactLimit`; `internal/mangle/engine_test.go#TestDerivedFactsGasLimit` passes |
| Query boundary | **VERIFIED CURRENT** | `internal/mangle/engine.go#Engine.Query` parses shape, validates the declaration, synthesizes all-output mode when missing, applies a deadline, and returns named bindings |
| Persistence warm-up | **VERIFIED CURRENT** | `internal/mangle/engine.go#Engine.WarmFromPersistence` restores facts with auto-eval suspended and recomputes once afterward |
| Differential engine | **PARTIAL** | `internal/mangle/differential.go#DifferentialEngine` supports legacy strata stores and an opt-in unified store. Kernel result parity, zero-config 500,000 ceiling parity, and all three direct created-fact-limit routes are covered; external/provenance options remain full-path-only |
| Snapshot | **VERIFIED CURRENT** for legacy mode | `internal/mangle/differential.go#DifferentialEngine.Snapshot`, `internal/mangle/differential_test.go#TestSnapshotIsolation`; it deep-copies strata stores rather than structural COW |
| Virtual predicates | **VERIFIED CURRENT** for bound first-key loader shape | `internal/mangle/differential.go#DifferentialEngine.RegisterVirtualPredicate`, `internal/mangle/differential_test.go#TestLazyLoading` |
| Parser lock | **VERIFIED CURRENT** | `internal/mangle/parse_lock.go#ParseUnit` and `ParseAtom` share one mutex; sanitizer, synth, core, system adapters, and tests route through it. `TestCodeUsesSerializedMangleParser` scans the whole root module for raw parser selectors and the mixed-caller integration test passes under race |
| Schema/protected-head gate | **VERIFIED CURRENT** | `internal/mangle/schema_validator.go#SchemaValidator.ValidateLearnedRule` checks protected heads, learned-fact declarations, head arity, and body declarations |
| Feedback | **VERIFIED CURRENT** | `internal/mangle/feedback/loop.go#FeedbackLoop.GenerateAndValidate` applies deadlines, retry budgets, JIT predicate selection, optional synth, sanitizer, and validator feedback |
| Synth | **VERIFIED CURRENT** | `internal/mangle/synth/decoder.go#DecodeSpec`, `internal/mangle/synth/validate.go#ValidateSpec`, `internal/mangle/synth/compile.go#Compile` |
| Proof and LSP | **VERIFIED CURRENT** as APIs | `internal/mangle/proof_tree.go#ProofTreeTracer`, `internal/mangle/lsp.go#LSPServer`; CLI query and LSP commands are downstream consumers |
| SIMD intersection | **VERIFIED CURRENT** as a tested exported utility | `internal/mangle/simd_intersect_amd64.go#IntersectSIMD`, `internal/mangle/simd_intersect_test.go`; no production caller was found in the review |

## Live kernel paths

The package does not wrap every production evaluation. The actual split is:

```text
core program rebuild
  -> internal/core/parse_serial.go#parseUnit
  -> internal/mangle/parse_lock.go#ParseUnit
  -> mangle-go analysis / stratification

normal core evaluate
  -> internal/core/kernel_eval.go#RealKernel.evaluateFullLocked
  -> mangle-go EvalStratifiedProgramWithStats directly

opt-in differential evaluate
  -> internal/core/kernel_eval.go#RealKernel.buildDiffEngineLocked
  -> internal/mangle/engine.go#Engine.LoadSchemaString
  -> internal/mangle/differential.go#NewDifferentialEngine
  -> EnableUnifiedFastPath
  -> ApplyAtomDelta
  -> CopyAllFactsTo core store
```

**VERIFIED CURRENT:** differential evaluation defaults off at compile time in
`internal/features/features.go#IsDiffEvalEnabled`. Environment or active user
configuration may opt in. Core declines the diff path when provenance is active
or a declared external predicate callback is present. Policy changes, retractions,
clear, and reset invalidate its cached differential state.

**VERIFIED CURRENT:** `buildDiffEngineLocked` copies the value from
`internal/core/kernel_init.go#RealKernel.effectiveDerivedFactLimitLocked` into
the wrapper configuration. Unset/non-positive kernel configuration resolves to
500,000 on both full and diff paths; the package-level reusable Engine default
remains 100,000. The kernel parity regression is
`internal/core/kernel_eval_test.go#TestKernelEval_ZeroConfigDerivedFactLimitParity`.
`internal/mangle/differential.go#DifferentialEngine.evalOptions` turns every
positive value into `WithCreatedFactLimit`, and unified atom, legacy atom, and
legacy fact delta paths forward it. The finite regression at
`internal/mangle/differential_test.go#TestDifferentialEngine_DerivedFactsLimit`
fails closed on all three routes.

## Model-authored logic path

`FeedbackLoop.GenerateAndValidate` is the common bounded loop, but callers choose
different protocols.

| Caller | Current synth behavior | Evidence |
|---|---|---|
| Legislator | **VERIFIED CURRENT:** `SynthModeRequire`, one clause, no declarations/package/use | `internal/shards/system/legislator.go#NewLegislatorShard` |
| Core hot-load feedback | **VERIFIED CURRENT:** default `SynthModeOff` | `internal/core/kernel_policy.go#RealKernel.HotLoadRule` constructs `NewFeedbackLoop` without `SetSynthMode` |
| Executive policy shard | **VERIFIED CURRENT:** default `SynthModeOff` | `internal/shards/system/executive.go#NewExecutivePolicyShardWithConfig` |
| Constitution gate shard | **VERIFIED CURRENT:** default `SynthModeOff` | `internal/shards/system/constitution.go#NewConstitutionGateShardWithConfig` |
| Mangle repair shard | **PARTIAL:** has synth helpers in its repair implementation, but does not use the common feedback-loop synth-mode contract | `internal/shards/system/mangle_repair.go#MangleRepairShard` |

The feedback prevalidator catches common AI errors—atom/string confusion,
Prolog-style negation, unsafe variables, and malformed aggregation—and the
sanitizer attempts selected repairs. Final parse/analyze/schema checks remain
authoritative.

## Safety boundary

The package does not own `permitted/3`. Core declares it at
`internal/core/defaults/schemas_safety.mg#permitted/3` and derives it through
`internal/core/defaults/policy/constitution.mg#permitted/3`.

`internal/mangle/schema_validator.go#forbiddenLearnedHeads` prevents learned
logic from defining:

- constitutional permission and allowlist predicates;
- admin or signed approvals;
- pending/permitted actions and permission results;
- routing, execution, and system-shard-state pipeline facts.

This is a compile/load boundary, not a complete runtime sandbox. Resource bounds,
external predicate registration, constitutional policy, VirtualStore validation,
and tool containment remain separate gates.

## Package-local intent source

**VERIFIED CURRENT:** `internal/mangle/intent_routing.mg` is read by package tests
such as `internal/mangle/intent_wiring_test.go#TestIntentWiring` and
`internal/mangle/intent_imports_test.go#TestIntentImports`. No production Go
loader references that path.

Production boot instead enumerates `schema/intent_routing.mg` from
`internal/core/intent_defaults.go#DefaultIntentSchemaFiles` and reads the embedded
copy at `internal/core/defaults/schema/intent_routing.mg` through
`internal/core/intent_loader.go#RealKernel.loadEmbeddedIntentFacts`. The two files
are not duplicates: one is 414 lines, one is 367 lines, and their SHA-256 hashes
differ. Tests of the package-local file therefore do not prove runtime intent
behavior. That is a governance and coverage gap, not proof that production
routing is currently failing.

## Configuration and state

| Setting/state | Current behavior | Evidence |
|---|---|---|
| `FactLimit` | default 100,000 EDB insertions | `internal/mangle/engine.go#DefaultConfig` |
| `DerivedFactsLimit` | default 100,000 on the reusable Engine full evaluator | `internal/mangle/engine.go#DefaultConfig`, `Engine.evalWithGasLimit` |
| `QueryTimeout` | default 30 seconds; a caller deadline wins | `internal/mangle/engine.go#Engine.Query` |
| `AutoEval` | enabled unless `MANGLE_AUTO_EVAL=0` | `internal/mangle/engine.go#DefaultConfig` |
| Differential feature | compile-time default off; env/config may opt in | `internal/features/features.go#IsDiffEvalEnabled` |
| Feedback retries | default max 3, per-rule 3, session budget 20 | `internal/mangle/feedback/types.go#DefaultConfig` |
| Feedback timeouts | loaded from configured LLM timeout values | `internal/mangle/feedback/types.go#DefaultConfig` |
| Parse serialization | one package-global mutex for exported chokepoints | `internal/mangle/parse_lock.go#parseMu` |

## Verification receipt

The reviewed command completed successfully:

```text
go test ./internal/mangle/... -count=1
ok codenerd/internal/mangle
ok codenerd/internal/mangle/feedback
ok codenerd/internal/mangle/synth
ok codenerd/internal/mangle/transpiler
```

The follow-up P0 receipt also passed:

```text
go test ./internal/mangle -run '^TestDifferentialEngine_DerivedFactsLimit$' -count=1
go test ./internal/mangle -run '^(TestDifferentialEngine_|TestNewDifferentialEngine|TestDerivedFactsGasLimit)' -count=1
```

The 2026-08-09 parser-boundary receipt passed:

```text
go test -race ./internal/mangle -run '^Test(CodeUsesSerializedMangleParser|ProductionParserCallersShareSerializedEntryPoint)$' -count=5
go test -race ./internal/core -run 'Concurrent|DreamerSingleton|DreamRouterSingleton' -count=1
```

This receipt does not claim `-race`, fuzz duration, kernel-wide, CLI, or campaign
coverage. The current decisive cross-package parity test is
`internal/core/kernel_eval_test.go#TestKernelDifferentialEval`; it compares full
and differential results for a finite workload. Differential package tests now
exercise evaluator limits; external callbacks, provenance, and unified read APIs
remain outside that core parity test.
