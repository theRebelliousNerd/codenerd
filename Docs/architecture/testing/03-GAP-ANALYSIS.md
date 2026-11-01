# testing — Gap Analysis

## GAP-TESTING-SYNTHETIC-STATE-DECLARATIONS

Synthetic kernel-state fixtures must declare their own predicates, at the
asserted arity, before expecting ordinary queries to return their facts. Use
`RealKernel.AppendSchema` on a fresh fixture kernel so existing production
schemas remain intact. Do not change production query visibility, permit
undeclared predicates globally, replace the schema corpus, or introduce action
grants to make a fixture pass.

The forty legacy `..._ScenarioN` cases assert `scenario_N_baseline/0` without a
declaration, then label an empty normal query as lost state. Establish the
declaration/visibility distinction with controls on unchanged production code,
retain their original assertions and timer/context boundary, and supply only
the matching fixture declaration. Check wrong arity, still-undeclared state,
independent fresh kernels and unchanged action authority.

Status: the identical final control file passes five controls on unchanged main,
while all forty old numbered cases fail. With their matching private declarations,
all 45 affected checks pass, including three race-enabled repetitions (135 pass
events). All 561 existing assertion/log/subtest/deadline expressions are unchanged.
The wrapper remains separately failing.

These cases currently exercise kernel state retention only. Their legacy names
are not evidence of Session/Executor/VirtualStore/autopoiesis wiring, and making
their schema setup valid does not close GAP-TESTING-08-01. The wrapper's separate
typed/string parsing setup must be diagnosed separately, not silently folded
into this repair. Exit requires before/control comparison, the affected cases,
race/full offline checks and an honest remaining tagged-suite census.

## GAP-TESTING-VERIFIED-ORCHESTRATOR-FIXTURE

A positive orchestrator smoke fixture must provide one coherent owned workspace:
the executor's root, the edited file, and the package tests must name the same
module. Its real test must fail on the initial source and pass after the fixture
model's bounded edit. Keep production build/test/critic/audit gates enabled and
install the role compiler that boot supplies. A write and a completion sentence
cannot stand in for a revision-bound post-write test witness.

Use an executor-owned modular registry and exact precompiled envelope for the
fixture; do not swap or broaden the process-global registry. Preserve normal
Mangle authorization, Dreamer preflight, workspace containment and tool-effect
validation. A deterministic model may propose the fixture edit, but it must not
assert gate facts, synthesize receipts, repeatedly invent fresh writes, or
declare its own acceptance.

Status: the representative inline smoke uses this coherent fixture. The same
final controls pass on unchanged production code while the old smoke setup
fails; the adapted smoke and controls pass, including concurrent private
workspaces and repeated race checks. Actual native acceptance reports retain a
failed pre-edit witness and a passing post-edit witness bound to the final
workspace snapshot. Existing smoke assertions remain intact.

Exit: repair the representative existing smoke without removing its assertions;
show the fail-before test, actual native build/test evidence and verified task
return; retain missing-role, red-test, unfinished-report and isolation controls.
Distinguish the existing policy arms: green mechanical gates alone cannot
overrule an unfinished-work admission; an independently verified caller-owned
behavior contract still verifies its declared scope. Neither arm lets model
prose create acceptance or a passing test receipt.
Other legacy fixture families and the preserved role-wiring trial remain
separate work until their own execution and verification obligations are met.

## GAP-TESTING-PROMPT-FACT-ADAPTER

The e2e prompt-kernel adapter must accept the fact representation emitted by
`CompilationContext.ToContextFacts`: complete Mangle clauses with terminating
periods. `core.ParseFactString` accepts an atom without its clause terminator
and appends that terminator itself. Normalize only surrounding whitespace and
one final clause period at this boundary; preserve quoted content, typed facts,
name constants, schema validation and rejection before batch mutation on a
parse/type error. The production system adapter already handles clause syntax;
this mismatch is a fixture defect, not a production parser failure.

Status: the producer/adapter boundary and unchanged compiler smoke pass, with
generated/string/name/typed parity and invalid-batch controls. The identical
final controls fail on the unchanged baseline and pass after normalization,
including ten race-enabled repetitions. Other fixture composition and actual
execution obligations remain open.

Exit: reproduce the unchanged prompt-compiler smoke failure, establish parity
with the production adapter for generated, dotted, bare and typed context facts,
retain malformed/unsupported input refusal controls, and exercise the actual
compiled-prompt/LLM-fixture flow. Run focused race, complete tagged and default
offline suites; report unrelated runtime failures without weakening their
assertions or treating fixture compatibility as full-recurse qualification.

## GAP-TESTING-INTEGRATION-FIXTURE-CONTRACTS

The `sqlite_vec,integration` e2e fixtures must compile against the current
runtime interfaces while preserving the behavior each fixture supplies and
every meaningful assertion. A whole-completion streaming mock returns its
original text/error and the grounding URLs it actually has (`nil` for no
sources), through `types.StreamWhole`. It must not invent grounding or turn an
error into a successful terminal result. Delegating mocks must retain their
existing failure and cancellation paths.

Budget fixtures must supply the current `Fit` token-counter argument. Use a
fresh native render-session counter for each fit, including declared count
floors and the selected render-mode text; do not substitute a zero counter,
duplicate the fitter or relax the existing resource and mutation assertions.
Legacy atom-level `Priority` fields must use current ownership: category
priority comes from the budget manager, while a fixture explicitly declaring
mandatory atoms sets `IsMandatory`. Do not invent atom importance levels or
discard mandatory behavior to obtain compilation.

Campaign fixture limits belong to caller contexts under the current lifecycle
contract. Do not restore removed orchestrator-wide campaign/task timeout knobs,
discard existing caller deadlines or widen policy/permissions to make tests
run. Keep ignored legacy fixtures and unrelated production contracts separate.

Status: fixture compilation restored. Active streaming mocks now implement the
three-result callback, removed campaign timeout fields defer to unchanged caller
deadlines, and budget fixtures use the native counter and current mandatory
flag. `TestE2E_FixtureStreamContract_*` and
`TestE2E_FixtureBudgetCounter_DeclaredRenderedAndMandatory` exercise the adapted
contracts offline; ten race-enabled repetitions pass.
The default offline `sqlite_vec` suite passes (22,446 pass and 80 skip events),
as do the build and default/tagged vet checks.

The complete tagged suite was executed, not merely compiled: 789 top-level tests
passed and 90 failed (906 pass and 95 fail events including subtests). It is
not a green integration gate. Examples requiring separate repairs are campaign
fixture plans rejected by inclusion constraints, executors lacking the
`final_report_auditor` role-prompt compiler, and generated-tool fixtures failing
current catalog admission. Preserve
these failures and the existing assertions; do not bypass the production
admission guards to obtain a passing fixture. The unchanged baseline package
before the fixture-contract repair could not compile, so that earlier baseline's
runtime equivalence was not demonstrated. Timing and deadline cases vary across
full runs; retain their failure provenance rather than count an incidental pass
as another repair or widen thresholds to suppress failures.

Exit: compile the complete tagged package, exercise the affected offline
fixtures and streaming success/error/cancellation controls, and run relevant
race/default regression gates. Record actual behavioral failures separately
from compilation; a compilable fixture is not proof its named pipeline works.

## Accepted pipeline acceptance target (2026-10-02)

**PARTIAL — GAP-TESTING-08-01:** this corpus owns tests/e2e in corpus.toml, but `tests/e2e/Session_Executor_VirtualStore_Autopoiesis_integration_test.go` does not exercise its named pipeline. Root reproduced Scenario1 failure in artifact:.corpus-build/runs/all-features-20261002/baseline-e2e.log. Forty equivalent scenarios now declare their private zero-arity state and query it after a timer; they still instantiate only the kernel and do not exercise the named pipeline. `internal/core/kernel_undeclared.go` continues to exclude undeclared predicates from ordinary schema queries; the original empty results were fixture visibility failures, not evidence of kernel data loss. The wrapper still ignores a fact-string parse error and lacks its own fixture declaration. Declaration repair does not close this pipeline acceptance obligation. Ten named cases also contain unused cancellation, duplicate flood assertions, absent panic injection and logging-only bodies. The source-grounded obligation map is artifact:.corpus-build/runs/all-features-20261002/08-verification/handoff-3.json, not a passing receipt.

**PROPOSED UPLIFT:** preserve smoke, malformed control, admitted-work cancellation/join, concurrent distinct state, actual resource bounds, articulation/pipeline panic isolation, failure/recovery, exact baseline integrity, five-turn state/isolation and partial-failure obligations. Replace synthetic repeats only after explicit mapping and decisive stronger receipts. A recorder panic is supplemental, not a substitute for articulation panic isolation. Pin missing resource/concurrency contracts and build required seams; do not invent thresholds, skip cases or silently narrow the target.

Acceptance uses a temporary workspace, real Cortex/kernel/session/VirtualStore and production feedback adapters, deterministic model inputs, typed declared baselines, actual permitted effects and correlated receipts. Automatic generated-tool learning differs from turn/prompt learning; manual RecordExecution proves neither route. Disconnecting effects or feedback must fail the respective positive assertions. Runtime bridge prerequisites remain open. Preserve historical failures and meaningful original assertions until replacement coverage is verified. Root integration, race, suite and normal-entry receipts are required; this accepted target establishes no pipeline completion.

> Last verified: 2026-07-13  
> Against vision in `01-VISION.md` and package-local README claims

## Legend

| Status | Meaning |
|--------|---------|
| **Done** | Implemented and exercised |
| **Partial** | Code exists; incomplete wiring or fidelity |
| **Missing** | Vision / docs claim without implementation |
| **Non-gap** | Intentionally out of scope |

---

## Spec vs reality matrix

| Capability | Status | Evidence | Priority |
|------------|--------|----------|----------|
| Dual-mode ContextEngine | **Done** | `MockContextEngine`, `RealIntegrationEngine`, interface in `engine_interface.go` | — |
| Scenario registry (mock + integration) | **Done** | `scenarios.go`, `scenarios_integration.go` | — |
| Turn simulation loop | **Done** | `SessionSimulator.RunScenario` | — |
| Checkpoint precision/recall | **Done** | `validateCheckpoint` + fuzzy match | — |
| Metrics + reporter | **Done** | `metrics.go`, `reporter.go` | — |
| CLI `nerd test-context` | **Done** | `cmd/nerd/cmd_test_context.go` | — |
| File-based glass-box logs | **Done** | `FileLogger` + tracers | — |
| Live LLM piggyback feedback | **Partial** | `GenerateAssistantResponse` + `executeLiveLLMTurn`; requires real mode + API | P2 |
| Real ActivationEngine scoring | **Done** (retrieve) | `RealIntegrationEngine.RetrieveContext` → `ScoreFacts` | — |
| Real Compressor / LLM summary compression | **Partial** | Compressor field set; `CompressTurn` uses metadata facts + token estimate | P1 |
| `ValidateActivation` at checkpoints | **Partial** | Type + method on validation struct; **not called** by simulator | P1 |
| `ValidateCompression` at checkpoints | **Partial** | Type only; not enforced | P1 |
| `ValidateFeedback` at checkpoints | **Partial** | Type only; not enforced | P1 |
| Category filter CLI | **Partial** | Flag + `ScenariosByCategory`; CLI never filters | P2 |
| `GetScenario("context-feedback-learning")` | **Missing** | In `IntegrationScenarios`, absent from `GetScenario` map | P2 |
| Isolated per-scenario kernel | **Partial** | Factory exists; CLI uses shared Cortex kernel | P1 |
| `FactSeeder.Clear` | **Partial** | No-op stub | P3 |
| Minimal schema load in factory | **Partial** | Schema names listed, not loaded | P3 |
| Adversarial scenarios | **Missing** | README “future” | P3 |
| Replay from `.nerd/logs` | **Missing** | README “future” | P3 |
| Real JIT compiler traces | **Partial** | Tracer + mock atoms only | P2 |
| CI job running harness | **Missing** as package-owned | README suggests YAML; not verified in-repo here | P2 |
| Full OODA / tool execution tests | **Non-gap** | Session/campaign domain | — |
| Constitutional `permitted` tests | **Non-gap** | Core policy domain | — |
| Production import of harness | **Non-gap** | Must stay test-side | — |

---

## Priority narratives

### P1 — Enforce advanced checkpoint validators

Scenarios already attach `ValidateActivation`, `ValidateCompression`, and `ValidateFeedback` on integration checkpoints. The simulator only scores `MustRetrieve` / `ShouldAvoid`. That means integration scenarios can “pass” while campaign boosts, budget compression triggers, and feedback learning never run their assertions.

**Fix shape:** extend `validateCheckpoint` to call existing validators when fields are non-nil; fail `CheckpointResult` with component-specific reasons.

### P1 — Real compression fidelity

`NewRealIntegrationEngine` builds `internalcontext.NewCompressor(...)` but `CompressTurn` does not call compressor LLM paths; it mirrors mock-style fact extraction. Real mode still improves **retrieval** fidelity; compression metrics remain enrichment estimates (~20 tokens/fact).

### P1 — Scenario isolation

`RunAll` reuses one Cortex kernel + one engine without guaranteed `Reset` between scenarios (engine has `Reset()`, harness does not always call it between runs). Cross-scenario fact contamination is a real risk.

### P2 — CLI category filter + GetScenario completeness

Operators following Long help (`--category=integration`) get no filtering. Feedback-learning scenario is invisible to name lookup via `GetScenario`.

### P2 — Mock JIT/prompt honesty

Tracer logs look production-like but are simulator-generated. Document as **synthetic** unless wired to `internal/prompt`.

### P3 — Factory / seeder polish

`randomID`, schema loading, `Clear()` — quality-of-life for package-level tests, not CLI path.

---

## Non-gaps (do not “fix”)

1. **Empty parent package** — intentional anchor; imports target `context_harness`.  
2. **No Mangle files in package** — harness asserts facts into kernel; rules live in core defaults.  
3. **Not a general eval platform** — scoped to context memory quality.  
4. **Enrichment ratios &lt; 1.0** — by design for short simulated messages; do not “fix” by demanding 5:1 without changing scenario text length.

---

## Documentation drift

| Claim | Reality |
|-------|---------|
| Package README architecture tree lists `integration.go` | File is `scenarios_integration.go` |
| README sample metrics show 5.43x compression | Code expects enrichment ratios &lt; 1.0 for mock scenarios |
| README lists 4 mock scenarios | Code has **8** mock scenarios |
| README lists 6 integration scenarios | Code has **7** (includes feedback learning) |
| CLI Long lists 6 integration scenarios | Omits `context-feedback-learning` |

Current-state inventories must track actual source rather than older sample
reports. Intended testing capabilities remain specification obligations even
when no current source exercises them.

## Production pipeline witnesses, without retiring unproved obligations

GAP-TESTING-08-01 requires real BootCortexWithConfig and Executor.Process tests,
with actual kernel, VirtualStore, generated execution, validation, and factory
learning persistence. A deterministic injected model is acceptable; replacing
those product components or manually recording feedback is not. Keep the ten
original obligations and their existing assertions until replacement behavior is
independently accepted. Forty duplicate undeclared-fact cases do not substitute
for actual turn execution. Check exact declared baseline tuples before and after.

Prove successful literal effect plus reopened learning, malformed-control refusal,
admitted cancellation and drain, distinct concurrent request isolation, actual
oversized-input refusal, panic containment at a real component seam, failure then
recovery, exact data integrity, five distinct turns, and partial failure preserving
baseline. Count effects and acknowledgments; disconnect execution/feedback to
prove the witness fails rather than passing vacuously. Do not weaken schemas,
permissions, production limits, or existing assertions. Unproved concurrency,
oversize, and articulation-panic contracts stay open instead of being replaced by
easier recorder/component tests. Test authoring alone does not close this gap.

The production witnesses in
`tests/e2e/production_generated_pipeline_test.go:722` remain red. The normal
write-intent turn omits the registered generated tool from its compiled catalog,
so the positive process/effect/learning witness does not execute. Cancellation
and concurrency cases cannot reach native admission, and the articulation panic
injection is not reached. This is an upstream reachability gap, not evidence
that those downstream contracts passed or failed after admission. Preserve all
obligations and controls; repair the real catalog path before judging downstream
behavior. Component Boot/Process acceptance with an explicitly configured
analysis persona does not substitute for this write-intent entry path.
