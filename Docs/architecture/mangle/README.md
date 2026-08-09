# Mangle: the executable boundary between ideas and actions

> Corpus owner: `mangle`
>
> Realized source root: `internal/mangle`
>
> Source review: 2026-07-13 at `c8f21b46ec4b28529953094e0c18dac4dfd0c8eb`

## In one minute

An LLM can suggest a rule, but it cannot make that rule true by saying it
confidently. Mangle is where codeNERD turns typed facts and declared rules into
deterministic conclusions. The user-visible payoff is simple: the same facts and
policy should produce the same decision, unsafe actions should remain denied,
and a malformed learned rule should fail before it can become executive state.

Think of this package as three things working together:

- a **compiler boundary** that parses and analyzes Mangle programs;
- a **deductive database adapter** that stores facts and evaluates to a fixpoint;
- a **logic-generation gate** that gives models structured feedback instead of
  accepting invented predicates, wrong arities, unsafe negation, or malformed
  aggregation.

`internal/mangle` is not the entire executive. Core owns the live constitution,
the OODA decision loop, and `permitted/3`. This package supplies reusable engine,
incremental-evaluation, validation, synthesis, proof, LSP, and parse-safety
machinery beneath and beside that executive.

## Its place in codeNERD

```text
creative center                         deterministic executive
LLM / system shard
  proposes a rule or structured spec
          |
          v
feedback -> synth -> sanitizer -> schema/protected-head gates
          |                         internal/mangle
          v
schemas + policy + EDB facts -> analysis -> fixpoint -> derived facts
          ^                                      |
          |                                      v
perception / world                         next_action + permitted/3
                                                 |
                                                 v
                                    VirtualStore -> articulation -> user
```

The important boundary is that prose never becomes authority directly. A
candidate rule must parse, use declared predicates with the right arity, remain
stratifiable, and avoid protected control-plane heads. Runtime permission is
then decided by core policy, not by the generation stack.

| Responsibility | Owner | Boundary evidence |
|---|---|---|
| Parse serialization, reusable engine, differential engine | `internal/mangle` | `internal/mangle/parse_lock.go#ParseUnit`, `internal/mangle/engine.go#Engine`, `internal/mangle/differential.go#DifferentialEngine` |
| Learned-rule schema and protected-head checks | `internal/mangle` | `internal/mangle/schema_validator.go#SchemaValidator.ValidateLearnedRule` |
| Full kernel program assembly and normal fixpoint | `internal/core` | `internal/core/kernel_eval.go#RealKernel.evaluateFullLocked` |
| Constitutional permission | core policy | `internal/core/defaults/schemas_safety.mg#permitted/3`, `internal/core/defaults/policy/constitution.mg#permitted/3` |
| Prompt-atom selection and assembly | prompt and articulation | `internal/prompt/compiler.go#JITPromptCompiler.Compile`, `internal/articulation/prompt_assembler.go#PromptAssembler.AssembleSystemPrompt` |
| External effects | VirtualStore and registered tools | `internal/core/virtual_store.go#VirtualStore` |

## A representative journey

Suppose a user asks codeNERD to fix a failing Go test.

1. **Ingress.** Perception turns the request into structured `user_intent` facts.
   Core also loads the embedded intent corpus enumerated by
   `internal/core/intent_defaults.go#DefaultIntentSchemaFiles`; natural-language
   similarity is resolved before Mangle, not by exact string matching inside it.
2. **Program construction.** Core combines schemas, constitutional policy, and
   learned rules. Every core parse goes through
   `internal/core/parse_serial.go#parseUnit`, which delegates to the process-wide
   lock at `internal/mangle/parse_lock.go#ParseUnit`.
3. **Decision.** The ordinary path evaluates the assembled program through
   `internal/core/kernel_eval.go#RealKernel.evaluateFullLocked`. When differential
   evaluation is explicitly enabled and its compatibility gates pass, core builds
   `internal/mangle/differential.go#DifferentialEngine` and applies only new EDB
   atoms through `internal/core/kernel_eval.go#RealKernel.evaluateDiffLocked`.
4. **Safety.** Policy may derive a `next_action`, but execution still requires a
   positive `permitted(ActionType, Target, Payload)` conclusion. The declaration
   and default-deny rules live in core, while
   `internal/mangle/schema_validator.go#forbiddenLearnedHeads` prevents a learned
   rule from defining `permitted`, `safe_action`, approvals, or pipeline results.
5. **Effect and response.** The kernel exposes the selected action to the
   VirtualStore, which performs the registered effect. Articulation explains the
   result to the user; proof-tree types from
   `internal/mangle/proof_tree.go#DerivationTrace` can support a logic view.
6. **Failure path.** A malformed model-generated rule is normalized, prechecked,
   optionally compiled from `mangle_synth_v1`, sanitized, and validated by
   `internal/mangle/feedback/loop.go#FeedbackLoop.GenerateAndValidate`. Attempt and
   session budgets stop repair thrash. A parse, schema, arity, stratification, or
   protected-head failure is returned rather than silently becoming policy.

**PARTIAL:** the journey still has one important semantic seam. Unified fast-path
Query/Snapshot reads do not use the store populated by ApplyAtomDelta. The parser
boundary is now **VERIFIED CURRENT**: sanitizer, synth, core, and system adapter
production calls enter the shared parse lock, with a source guard and mixed-caller
race regression preventing bypass.
Differential created-fact gas parity is now **VERIFIED CURRENT** by
`internal/mangle/differential_test.go#TestDifferentialEngine_DerivedFactsLimit`.
Kernel zero-config parity is separately guarded by
`internal/core/kernel_eval_test.go#TestKernelEval_ZeroConfigDerivedFactLimitParity`:
both full and diff kernel paths use 500,000, rather than letting diff inherit the
reusable Engine's 100,000 default.

## What exists today

The realized package contains 21 non-test Go files, 45 Go test files, and one
package-local `.mg` file. `go test ./internal/mangle/... -count=1` passed on the
reviewed tree. This is meaningful package evidence, not proof of every kernel,
race, or long-horizon path.

### Component status

| Component | Claim | Evidence and discriminator |
|---|---|---|
| Engine wrapper, fact conversion, persistence warm-up, queries | **VERIFIED CURRENT** | `internal/mangle/engine.go#Engine`; `internal/mangle/engine_test.go#TestEngineQuery`, `internal/mangle/engine_test.go#TestDerivedFactsGasLimit` |
| Full-path gas limit | **VERIFIED CURRENT** | `internal/mangle/engine.go#Engine.evalWithGasLimit` forwards `WithCreatedFactLimit`; its focused regression passes |
| Differential engine and kernel opt-in | **PARTIAL** | `internal/mangle/differential.go#DifferentialEngine`, `internal/core/kernel_eval_test.go#TestKernelDifferentialEval`; result, positive-limit enforcement, and zero-config kernel ceiling parity are tested, while external/provenance options remain unsupported and use full fallback |
| Process-wide parser lock | **VERIFIED CURRENT** | `internal/mangle/parse_lock.go#ParseUnit` and `ParseAtom` are the only raw parser calls; `internal/mangle/parse_lock_test.go#TestCodeUsesSerializedMangleParser` scans the whole root Go module, including tests and function references, while `internal/mangle/parse_callers_integration_test.go#TestProductionParserCallersShareSerializedEntryPoint` passes under race |
| Learned-rule protection | **VERIFIED CURRENT** | `internal/mangle/schema_validator.go#SchemaValidator.ValidateLearnedRule`; protected heads include permissions, approvals, and runtime pipeline facts |
| Structured Mangle synthesis | **VERIFIED CURRENT** | `internal/mangle/synth/compile.go#Compile`, `internal/mangle/synth/validate.go#ValidateSpec`; legislator requires the single-clause schema at `internal/shards/system/legislator.go#NewLegislatorShard` |
| Feedback and retry budgets | **VERIFIED CURRENT** | `internal/mangle/feedback/loop.go#FeedbackLoop.GenerateAndValidate`, `internal/mangle/feedback/types_test.go#TestValidationBudget_Concurrency` |
| Proof tree and LSP | **VERIFIED CURRENT** as APIs; product reach varies | `internal/mangle/proof_tree.go#ProofTreeTracer`, `internal/mangle/lsp.go#LSPServer`, `cmd/nerd/cmd_query.go#runWhyViaHollowTracer` |
| Package-local `intent_routing.mg` | **PARTIAL / SHADOW SOURCE** | package tests load `internal/mangle/intent_routing.mg`, while production boot enumerates `internal/core/defaults/schema/intent_routing.mg` through `internal/core/intent_loader.go#RealKernel.loadEmbeddedIntentFacts`; the files differ |

### Applicability matrix

| Lane | Status | Package-specific answer |
|---|---|---|
| Mangle | **PARTIAL** | Directly applicable. Declarations and arities are analyzed by `internal/mangle/engine.go#Engine.rebuildProgramLocked`; learned rules are screened by `internal/mangle/schema_validator.go#SchemaValidator`; synth models explicit `do`/`let` transforms at `internal/mangle/synth/spec.go#TransformStmtSpec`. Positive body atoms must bind variables before negation, recursive programs must stratify and terminate over a finite domain, atoms such as `/active` remain distinct from strings, and aggregation uses the `\|> do ... let ...` pipeline. Regex prechecks are advisory; the upstream parser/analyzer is decisive. Producers are core, perception, world, and shards; consumers are core queries, CLI tooling, browser analysis, and system shards. |
| Permission and safety | **VERIFIED CURRENT** for created-fact bounds; **PARTIAL** overall | `permitted/3` is intentionally **N-A as an owned predicate** because core policy owns default deny (`internal/core/defaults/policy/constitution.mg#permitted/3`). The package protects that boundary with `internal/mangle/schema_validator.go#forbiddenLearnedHeads`, fact/query limits, differential `evalOptions`, and bounded feedback retries. External/provenance differential options remain explicit full fallbacks. |
| Fact flow | **PARTIAL** | Perception and system state produce EDB facts; core derives `next_action` and `permitted/3`; VirtualStore executes and articulation responds. `internal/mangle` owns the differential adapter and reusable engine, but core's full path evaluates directly through mangle-go at `internal/core/kernel_eval.go#RealKernel.evaluateFullLocked`. |
| JIT and agents | **PARTIAL** | The feedback loop can request context-selected predicates through `internal/mangle/feedback/loop.go#PredicateSelectorInterface`. Legislator requires structured synth; executive, constitution, and kernel hot-load feedback loops are constructed with synth off unless configured. Prompt text and token budgeting remain owned by prompt/articulation, not this package. |
| Wiring | **PARTIAL** | Core uses the parse lock, schema validator, and opt-in differential engine. CLI/browser use the reusable engine; query UX uses proof types; mangle-lsp exposes the LSP. The package-local intent file is not the runtime boot authority, and unified fast-path read APIs have an unguarded mode boundary. |
| State and concurrency | **PARTIAL** | `Engine` and `DifferentialEngine` guard mutable stores with mutexes; every production parse call routes through the process-wide `ParseUnit`/`ParseAtom` lock and a source guard enforces that boundary. Snapshots deep-copy stratum stores rather than providing structural copy-on-write, and unified read semantics remain incomplete. |
| Recovery | **PARTIAL** | Feedback attempts have per-attempt and total deadlines plus session budgets; core invalidates differential state on policy changes/retract/clear and falls back to full evaluation; `Engine.WarmFromPersistence` restores EDB facts. There is no general automatic retry for evaluator failures, and recovery receipts are not unified. |
| Observability | **PARTIAL** | Logs expose parse/evaluation activity, `Engine.GetStats` reports fact counts, and proof/LSP diagnostics exist. There is no single bounded evaluation receipt containing path, policy/fact fingerprints, options, fallback reason, created facts, and proof correlation. |
| Testing | **VERIFIED CURRENT** for the package and parser boundary; **PARTIAL** system-wide | 45 test files cover engine behavior, differential result and gas parity, parser concurrency, fuzz seeds, schema gates, feedback, synth, sanitizer, LSP, and torture cases. Mixed ParseUnit/ParseAtom/sanitizer/synth callers pass under race, and the core concurrency slice passes under race. Unified-mode Query/Snapshot behavior remains the decisive missing gate. |

## North star

Every path from model-proposed logic to an external effect should be governed by
one inspectable contract:

- the same schema, atom/string, arity, stratification, external-predicate, gas,
  and provenance rules apply on full and differential evaluation paths;
- every parser caller uses the same process-safe chokepoint;
- structured synth is the default for model-authored logic, with free-form
  repair retained only as an explicit compatibility path;
- every accepted rule and every evaluation can emit a bounded, redacted receipt
  explaining inputs, policy identity, evaluation mode, derived result, and
  fallback or rejection reason;
- proof, provenance, CLI diagnostics, and operator telemetry describe the same
  derivation instead of maintaining parallel stories.

Non-goals are equally important. Mangle will not perform fuzzy natural-language
matching, become a string-processing language, own tool execution, grant its own
permissions, or replace the LLM as the creative problem solver. Embeddings and
perception map language into structured facts; Mangle evaluates those facts.

## Improvement frontier

The first safe slice of **evaluation-option parity** is complete: every direct
differential evaluator call now receives the configured created-fact limit, and
legacy/unified regressions prove fail-closed behavior. The verified receipt is
`mangle-diff-eval-option-parity-v1` in [TODO.md](TODO.md). External predicates and
provenance remain deliberate full-path fallbacks rather than silently dropped
options.

The bounded longer-horizon move is an **explainable replay receipt**. A redacted,
size-capped artifact would fingerprint the program and EDB, identify full or diff
mode and its options, record the derived result and proof correlation, and replay
without performing an external action. That proposal is
`mangle-explainable-replay-v1` in [TODO.md](TODO.md).

Other evidence-backed gaps remain dependency-ordered in
[03-GAP-ANALYSIS.md](03-GAP-ANALYSIS.md): close the unified read-contract bug,
make the package-local intent source's status explicit, unify
proof/provenance, and only then pursue true delta propagation.

## Choose a reading route

**90-second orientation**

1. Read this page through the representative journey.
2. Scan [02-CURRENT-STATE.md](02-CURRENT-STATE.md) for the live boundary.
3. Read the P0 rows in [03-GAP-ANALYSIS.md](03-GAP-ANALYSIS.md).

**10-minute architecture tour**

1. [01-VISION.md](01-VISION.md) — desired experience and non-goals.
2. [05-INTERNAL-ARCHITECTURE.md](05-INTERNAL-ARCHITECTURE.md) — components and state.
3. [08-WIRING-AND-INTEGRATION.md](08-WIRING-AND-INTEGRATION.md) — boot and consumers.
4. [09-SAFETY-AND-INVARIANTS.md](09-SAFETY-AND-INVARIANTS.md) — rule and resource boundaries.
5. [10-TESTING-ALIGNMENT.md](10-TESTING-ALIGNMENT.md) — verification surface.
6. [TODO.md](TODO.md) — authoritative uplift cards.

**Deep implementation route**

| Document | Responsibility |
|---|---|
| [IMPLEMENTED_SPEC.md](IMPLEMENTED_SPEC.md) | Flagship detailed implemented specification |
| [00-ALIGNMENT-VISION-REVIEW.md](00-ALIGNMENT-VISION-REVIEW.md) | North-star alignment review |
| [01-VISION.md](01-VISION.md) | Target architecture and non-goals |
| [02-CURRENT-STATE.md](02-CURRENT-STATE.md) | Source-grounded current truth |
| [03-GAP-ANALYSIS.md](03-GAP-ANALYSIS.md) | Current-to-target deltas |
| [04-ARCHITECTURAL-PRINCIPLES.md](04-ARCHITECTURAL-PRINCIPLES.md) | Binding design principles |
| [05-INTERNAL-ARCHITECTURE.md](05-INTERNAL-ARCHITECTURE.md) | Components, stores, and flows |
| [06-PUBLIC-API-AND-TYPES.md](06-PUBLIC-API-AND-TYPES.md) | Exported contracts |
| [07-DEPENDENCY-MAP.md](07-DEPENDENCY-MAP.md) | Upstream and downstream dependencies |
| [08-WIRING-AND-INTEGRATION.md](08-WIRING-AND-INTEGRATION.md) | Constructors, boot, dispatch, teardown |
| [09-SAFETY-AND-INVARIANTS.md](09-SAFETY-AND-INVARIANTS.md) | Safety and logic invariants |
| [10-TESTING-ALIGNMENT.md](10-TESTING-ALIGNMENT.md) | Test evidence and missing gates |
| [11-OBSERVABILITY.md](11-OBSERVABILITY.md) | Signals and diagnostics |
| [12-FAILURE-MODES.md](12-FAILURE-MODES.md) | Failure, degradation, and recovery |
| [OPEN-QUESTIONS.md](OPEN-QUESTIONS.md) | Unpinned choices |
| [_progress.md](_progress.md) | Review receipts and corpus history |
