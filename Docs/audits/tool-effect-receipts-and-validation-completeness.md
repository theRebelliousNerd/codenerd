---
doc-class: deep-dive
subsystem: tools
implementation-status: partial
last-verified: 2026-10-27
verified-against: e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a
supersedes: []
---

# Tool effect receipts: preserve changed state when validation fails

**Status:** source-inspected proposal, not an implementation or measured result. No code, tests, model inference, training, or benchmarks were run. Baseline: `e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a`. All examples below are proposed fixtures or static deductions, not observed incidents.

## Decision

Separate four facts currently compressed at the modular-tool boundary: **permission to attempt**, **what changed**, **whether the result was validated**, and **whether the requested task is complete**. A validator can reject a result after bytes have changed. Returning an error must not erase the change or authorize a blind retry.

Prioritize a host-issued, revision-bound effect receipt carried from the handler through session accounting and recovery. Preserve all existing capability, constitutional, workspace, project, and Dreamer gates. A receipt is evidence, never authority. This is a wiring refinement of existing receipts and validation, not a claim of a new distributed-systems primitive.

## 1. Scope and relationship to existing research

The root North Star requires typed policy-mediated operations and logic-owned executive decisions (`agents.md:5–43`, **North Star** and **The Vision**). Session guidance explicitly separates process, validation, and durable-feedback outcomes for generated tools (`internal/session/agents.md:3–17`, **Session contributor guidance**). Tool guidance requires declared effects and mandatory executive gates (`internal/tools/agents.md:1–8`, **Tool boundaries**). This proposal extends that separation to ordinary modular tools.

The architecture documentation standard calls for source-backed present state, explicit proposed state, and checkable exits (`Docs/journeys/09-architecture-doc-standard.md:1`, **The standard the architecture corpus has to meet**). Current main and the research branch were read before selecting this seam. Historical tool gap prose is not treated as current evidence: the tool gap matrix's **Spec vs reality matrix** and **Remaining — wiring** sections (`Docs/architecture/tools/03-GAP-ANALYSIS.md:25,59`) predate substantial execution changes. Source below is authoritative for this proposal.

This is distinct from the existing research notes on provider usage, retrieval freshness, consultation reuse, and delegation judging. In particular, [verification-evidence-and-judge-contracts](verification-evidence-and-judge-contracts.md) concerns the later advisory judge. This document concerns the earlier **handler → post-validator → turn write ledger** boundary. It neither redesigns the judge nor duplicates token metering.

## 2. What is already implemented

All source links below are pinned to the inspected baseline.

| Boundary | Source witness | Status and implication |
|---|---|---|
| Effective capability | [`internal/session/executor_tools.go:2354–2363`, `executeToolCall`](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/internal/session/executor_tools.go#L2354-L2363) | Implemented: JIT allowlist checked before backend dispatch. This proposal adds no capability. |
| Constitutional/project/executive gates | [`internal/session/executor_tools.go:2424–2488,2532–2560`, `executeToolCall`](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/internal/session/executor_tools.go#L2424-L2560) | Implemented: conditional constitutional check, write guards, declared effect, and mandatory non-read executive-gate check. Do not infer a global permission bypass from a later accounting defect. |
| Effect classification | [`internal/tools/effects.go:18–70`, `BuiltinEffect`, `DeclaredEffect`, `LookupEffect`](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/internal/tools/effects.go#L18-L70) | Implemented reviewed read/write/execute/external declarations. These describe possible effects, not proof that a specific invocation changed anything. |
| Handler outcomes and observations | [`internal/tools/registry.go:342–455`, `Registry.ExecuteTool`](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/internal/tools/registry.go#L342-L455) | Implemented: argument validation, allowlist, guard, handler, audit, metrics, and fact sink. Handler success is determined from its error before session post-validation. |
| Changed-element record | [`internal/tools/edit_record.go:10–39,61`, `ExecutionRecord`, `EditedElement`, `RecordEdit`](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/internal/tools/edit_record.go#L10-L69) | Implemented element observations, but not a complete admission/effect/validation receipt carrying before/after revisions. |
| Kernel observation | [`internal/core/virtual_store_tool_facts.go:45–77`, `toolFactSink`](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/internal/core/virtual_store_tool_facts.go#L45-L77) | Implemented handler-success facts and successful changed-element facts. This is useful evidence already present, not an empty learning loop. |
| Post-validator wiring | [`internal/core/virtual_store.go:237–270,374–380`, `NewVirtualStoreWithConfig`, `initValidators`](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/internal/core/virtual_store.go#L237-L380); [`internal/core/validator_registry.go:13–36`, `RegisterAllValidators`](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/internal/core/validator_registry.go#L13-L36) | Implemented constructor registration of file, syntax, execution, and CodeDOM validators. |
| Generated-tool replay separation | [`internal/system/factory_tool_executor.go:66–164,184–228`, `ExecuteGenerated`, `RetryGeneratedFeedback`](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/internal/system/factory_tool_executor.go#L66-L228) | Implemented live-call identity/fingerprint rendezvous and feedback-only retry. This is a useful local precedent; it is not evidence of crash-proof exactly-once execution for every backend. |

## 3. Source-grounded gap: a failed postcondition is not proof of no mutation

The modular route invokes the handler, then validates its reported successful result. On validator failure, it returns **empty output** with a post-action error ([`internal/session/executor_tools.go:2599–2629`, `executeToolCall`](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/internal/session/executor_tools.go#L2599-L2629)).

The accounting wrapper records the working result, then returns immediately on error. Its successful-write count, write sequencing, test-witness invalidation, written-path registration, and last-written-content record occur only afterward ([`internal/session/executor_tools.go:978–1043`, `executeAndRecordToolCall`](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/internal/session/executor_tools.go#L978-L1043)).

Consequently, **when a modular handler changes a file and a later validator rejects it**, that call does not reach this wrapper's successful-write accounting. Registry-level observations may still exist. The error is surfaced, but the two layers describe different aspects using success/error rather than carrying the distinction explicitly.

This matters because the repair boundary snapshots `WrittenPaths` ([`internal/session/executor_tools.go:447–470`, `verifyCompletedToolTurn`](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/internal/session/executor_tools.go#L447-L470)); change closure and evidence summary consult `SuccessfulWriteTools` ([`internal/session/change_evidence.go:70–117,324–351`, `closeChangeEvidence`, `appendEvidenceSummary`](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/internal/session/change_evidence.go#L70-L117)). This is a precise accounting concern, not proof that every later completion gate accepts an incorrect task.

### Concrete proposed witness

Use an authorized temporary JSON target and a payload containing incomplete JSON. The ordinary write handler checks Go syntax only, writes bytes, records changed source, and returns success ([`internal/tools/core/file_ops.go:296–365`, `executeWriteFile`](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/internal/tools/core/file_ops.go#L296-L365); [`internal/tools/go_syntax_guard.go:17–26`, `RejectUnparseableGo`](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/internal/tools/go_syntax_guard.go#L17-L26)). The syntax validator includes JSON and can reject the written content at full confidence ([`internal/core/validator_syntax.go:31–43,67–146,163–174`, `NewSyntaxValidator`, `SyntaxValidator.Validate`, `validateJSONSyntax`](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/internal/core/validator_syntax.go#L31-L174)).

The fixture must establish that all earlier gates admitted this exact harmless request; a preflight refusal would be a different result. It should assert actual bytes and all ledgers after rejection. No fixture was run here.

## 4. Adjacent gap: incomplete validation is not affirmative validation

The validator registry returns a failed, zero-default-confidence result when its context is canceled between validators ([`internal/core/action_validator.go:176–203`, `ValidatorRegistry.Validate`](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/internal/core/action_validator.go#L176-L203)). The interactive bridge returns an error only when the **first** failed result has confidence at least 0.8; nil explicitly means “verified or no opinion” ([`internal/core/virtual_store_interactive_gate.go:263–323`, `ValidateInteractiveToolResult`](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/internal/core/virtual_store_interactive_gate.go#L263-L323)).

That contract is implemented, but it does not communicate validation completeness to the session. A low-confidence failure preceding a later high-confidence failure is also worth a dedicated fixture: the registry can continue after the former, while the bridge inspects `FirstFailure` ([`internal/core/action_validator.go:225–251`, `Validate`, `FirstFailure`](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/internal/core/action_validator.go#L225-L251)). This is a conditional source deduction, not a claim that a standard-validator production sequence has been reproduced.

Do not fix this by interpreting every “no opinion” as a definitive defect. Unsupported validators, canceled validators, and failed validators need distinct evidence states. Later task acceptance remains a separate decision.

## 5. Proposed receipt and executive contract

The following fields and relation names are **design notation, not shipped APIs**.

A receipt is host-issued and immutable. Publish an initial immutable effect receipt, then append separately immutable validation and recovery records linked by receipt identity and effect revision. Never mutate a published effect receipt as validation arrives. A consumer may instead use versioned immutable snapshots, provided each names its predecessor and the effect revision, and incomplete versions cannot masquerade as complete ones. The following shape describes the assembled view, not a single record rewritten in place:

```text
identity: task, turn, call, tool-registration-version, canonical-arguments-digest
admission: denied | admitted; policy/evidence epoch; denial reason
execution: not-started | started | returned | interrupted | unknown
effects[]: resource identity, before revision, after revision, changed | unchanged | unknown
validation[]: resource identity, validator identity/version, measured resource revision, pass | fail | skipped | unavailable
output: bounded summary + recoverable body handle
recovery: inspect | revalidate | repair | retry-eligible | human-required
```

Separate facts such as `effect_observed(call,resource,before,after)`, `validation_observed(call,validator,revision,state)`, and `effect_uncertain(call,resource)` feed Mangle. Go reports host observations; Mangle derives obligations and allowed recovery actions. The model cannot assert completion or relax gates through these fields.

Required invariants:

1. **Admitted is not executed; executed is not changed; changed is not correct.**
2. Every observed changed path enters the turn's effect set even when handler or validator fails. A separate successful-tool count may remain for reporting, but it cannot define the write set.
3. Changed or uncertain resources invalidate prior dependent witnesses. Uncertainty must not become a fabricated revision or a passing validator.
4. Preserve captured handler output alongside post-validation errors, with recoverable handles for large output. Do not require a model to rerun an effect merely to rediscover its result.
5. For mandatory validators, acceptance requires completed passes on the current revision. Skipped optional validators remain visible without automatically failing the task.
6. Admission checks still run for every **new effect** and every attempted scope expansion. A historical receipt grants no fresh authority. Before replaying stored output or exposing its recoverable body, check current read authorization for the receipt's resources and output scope; historical permission to execute is not permission to disclose now.
7. A before/after difference establishes changed state, not authorship. If concurrent editing can occur during execution or readback, attribution remains uncertain unless an observed ownership mechanism, such as a successful revision compare-and-swap under exclusive ownership, proves the call produced that revision. Do not charge another writer's bytes to this call.

A proposed acceptance relation is:

```text
tool_result_verified(c,r) =
  effect_state_known(c,r)
  AND for every required validator v: validation(c,v,r)=pass
  AND no validation(c,*,r)=fail
```

This is a tool-local statement, not task completion. The exact required set is policy-derived for the operation and resource type.

### Recovery algorithm

1. Freeze the exact request identity and revisions after normal authority checks.
2. Execute once; collect output and host-side effects even on an error path.
3. Publish the immutable effect receipt before reducing it to model-facing text. Attach unknown effects when observation fails.
4. Run validators against the receipt's resulting revision; append immutable linked validation records, recording cancellation/absence separately. Derive completeness only from the required observation set for that exact revision; append recovery decisions rather than rewriting earlier records.
5. Merge effects into the turn ledger before returning any post-action error.
6. Derive recovery:
   - denied/not-started: never auto-bypass the denial;
   - changed + failed validation: inspect or repair the changed artifact, not repeat blindly;
   - changed + unavailable validation: revalidate if currently permitted;
   - interrupted/unknown: reconcile state first;
   - known no effect + transient execution failure: retry only if policy permits;
   - completed call replay: check current read authorization, then return the authorized retained receipt/output, not re-execute; withhold inaccessible content without synthesizing a new effect.
7. Before revalidation, compare resource identities and revisions. Every validation record names both. A concurrent modification requires a new observation and uncertain attribution unless ownership evidence proves authorship, not validation of the old snapshot or automatic attribution to the tool.

Use compare-and-swap or equivalent revision ownership for compensating edits; never roll back over concurrent user work. Arbitrary subprocess or remote effects may be irreconcilable. Mark them unknown and require narrower inspection or a human, rather than promising atomic rollback.

## 6. Literature and limitations

- [Saltzer and Schroeder, *The Protection of Information in Computer Systems*, basic principles](https://web.mit.edu/saltzer/www/publications/protection/Basic.html) motivates complete mediation and fail-safe defaults. Application here: receipts and cached outcomes must not widen current authority.
- [Lee et al., *Implementing Linearizability at Large Scale and Low Latency* (SOSP 2015)](https://web.stanford.edu/~ouster/cgi-bin/papers/rifl.pdf) identifies request identity, durable completion records, retry rendezvous, and metadata lifetime as essential to exactly-once RPC behavior. The transferable idea is replaying an outcome instead of an effect. CodeNERD's arbitrary filesystem/process tools do not inherit RIFL's guarantees merely by adding a receipt; crash consistency requires coupling mutation and receipt storage or explicit uncertain recovery.
- [RFC 9110 §9.2.2](https://www.rfc-editor.org/rfc/rfc9110.html#section-9.2.2) conditions automatic retry of non-idempotent requests on knowledge that replay is safe or that the original was not applied. This is an analogy for tool retry design, not a claim that local tools are HTTP methods.

No novelty, speedup, accuracy superiority, or exactly-once system guarantee is claimed.

## 7. Full cost accounting and alternatives

Compare at equal task correctness and unchanged authorization:

```text
C_task = C_model(all attempts, cached/uncached input, output, reasoning)
       + C_tool + C_validation + C_receipt_IO + C_reconciliation
       + C_repair + C_human
```

Record wall time, CPU, bytes hashed/read/written, receipt storage, model tokens, and human intervention separately. Do not add unlike units without explicit prices. Include failed tasks, not only completed ones.

For n affected files totaling B bytes and v validators, naive repeated reads can cost O(vB). A shared immutable readback can reduce duplicated IO only if it preserves validator semantics and revision identity; it does not justify skipping independent checks. Receipt metadata is O(n+q) per invocation plus archived output, where q is the actual number of resource-validator observations; q can be O(nv) when every validator measures every resource. Append-only revalidation and recovery records add storage proportional to their number, and retention, redaction, and garbage collection have operational costs. Metadata-only receipts cannot prove a remote process's external side effects.

Evaluate three ablations:

- **A — current path:** string/error plus current observations and all existing gates.
- **B — lossless output and effect accounting only:** preserve output, reconcile changed paths on every terminal path; no durable replay ledger.
- **C — typed receipts and policy-derived recovery:** B plus completeness states and replay/revalidation distinction.

B may deliver most benefit with less complexity. C is justified only if measured duplicate effects, repair waste, or ambiguity warrant durability and recovery costs. Smaller model prompts are not a win if the receipt adds more validator IO or human work than it avoids.

## 8. Failure modes and validation plan

Proposed deterministic fixtures, not executed:

| Fixture | Required outcome |
|---|---|
| Authorized JSON write followed by syntax rejection | Changed bytes remain attributed; handler output retained; failing validator recorded; no successful validation claim. |
| Denied call | No handler invocation, no invented mutation; denial preserved. |
| Handler succeeds, validator canceled | Effect known; validation unavailable; no mandatory-completeness pass. |
| Earlier low-confidence failure, later high-confidence failure | Any definitive mandatory failure remains visible; ordering cannot hide it. |
| Write succeeds, receipt persistence fails | Effect retained in memory if possible; durable status explicitly unknown/failed; no replay to repair logging. |
| Same identity and same arguments replayed | Current read authorization checked before retained output disclosure; retained authorized result or pending join; no second effect. |
| Same identity with different arguments | Conflict, no effect. |
| External edit during execution/readback or before revalidation/compensation | Uncertain attribution absent ownership proof; stale validation invalidated; concurrent bytes preserved. |
| Partial multi-resource effect | Each resource classified independently; no all-or-nothing claim without transactional evidence. |
| Unsupported optional validator | Explicit skipped state, not false defect; required coverage deficit remains visible. |
| Native and Piggyback routes | Equivalent evidence, accounting, and recovery; provider pairing preserved. |
| Generated-tool route | Preserve existing process/validation/durable-feedback distinctions; feedback repair never starts a second process. |

Then run production-entry evaluations over bounded real coding tasks: file edits, multi-file changes, malformed configuration repair, build/test failures, cancellation, and interrupted effects. Use identical task snapshots, model/version, seed policy, budgets, tool catalog, and human approvals across variants. Randomize variants and report paired confidence intervals. Grade final artifacts with held-out acceptance criteria plus human review of attribution and false success. Report validator false rejection, untracked changed paths, repeated effects, recovery turns, unresolved tasks, and total cost per verified task. A tool-local fixture passing does not qualify the full coding harness.

## 9. Build queue and checkable exits

These research IDs are local to this proposal, not declarations that the architecture corpus already contains them.

| ID | Proposed work | Exit |
|---|---|---|
| TOOL-EFFECT-01 | Preserve handler output and changed-path accounting across post-validator error | Authorized write/reject fixture observes the bytes, output, turn effect set, and invalidated witnesses through normal session dispatch. |
| TOOL-EFFECT-02 | Carry validation completeness separately from confidence | Cancellation, skip, and mixed-confidence fixtures produce distinct states; mandatory missing coverage cannot derive a validated result. |
| TOOL-EFFECT-03 | Separate replay, revalidation, and repair | Duplicate identity, conflict, publication failure, and revision-change fixtures show no unrequested second effect and no overwrite of concurrent work. |
| TOOL-EFFECT-04 | Qualify complete-system value | Paired production-entry evaluation reports correctness, false rejection, unresolved tasks, full costs, and confidence intervals; retain simpler B if C adds no justified value. |

Before implementation, reconcile the owning session/tools/core specs without replacing protected guidance or changing permissions. The immediate useful outcome is accurate effect accounting—not stronger prose about accuracy.


## As built (2026-10-27)

**Implemented (TOOL-EFFECT-01, -02, and the policy half of -03):**

- Typed validation states: `types.ToolValidation` / `ToolValidationState` (`internal/types/tool_validation.go`: verified, rejected, incomplete, unvalidated, not_run). `core.ValidationResult.State`/`Validator`, `core.classifyValidation` (a deferral is skipped, a timed-out failure unavailable), `core.SummarizeValidation` (rejected when *any* reading fails at `DefinitiveValidationConfidence`, so a low-confidence objection can no longer hide a later definitive failure). `ValidateAll`/`FirstFailure` are deleted; RouteAction and the interactive bridge both use the summary.
- `VirtualStore.ValidateInteractiveToolResult` returns `types.ToolValidation` instead of `error`; `apply_edits` merges every file's reading. Canceled validation emits `action_validation_unavailable`, a deferral `action_validation_skipped` (no longer `action_verified` at 0); both feed `side_effect_attempted`/limbo in `policy/validation.mg`, never the self-healing failure arms. `validation_method_used` now carries a `/name` (the raw string never satisfied its schema).
- Receipts: `session.ToolEffectReceipt`, `snapshotEffectTargets`/`observeEffects`, `Executor.settleToolEffects` (`internal/session/tool_effects.go`), called from `executeAndRecordToolCall` on every terminal path before an error returns. A rejected postcondition is `*PostconditionError` and carries the handler's output. A failed write that observably changed a file joins `WrittenPaths`, advances the write clock (invalidating earlier test witnesses), and records what it left for the restore.
- Executive contract: `policy/tool_effects.mg` (`turn_tool_effect`, append-only `turn_tool_validation` rounds, `turn_effect_current`, `turn_effect_unsettled`, `tool_effect_recovery` = /revalidate, /repair, /inspect, /reconcile). `coder_safety.mg` withholds `turn_verified` and names `/effect_rejected` / `/effect_unmeasured`. The host carries out /revalidate once; the other moves are said to the model beside the output. `Executor.assertSupersededEffects` marks, at closure, receipts an unobserved writer replaced. All receipt and validation predicates are host-witness-only (`core.hostWitnessPredicates`).

**Ruled differently:**

- `SuccessfulWriteTools` was renamed `WriteToolEffects` and widened to every write the turn must account for (completed, or failed after an observed change), rather than adding a parallel counter: every reader meant "this turn wrote", and two counters would drift. `ToolReceipts` says which calls failed.
- A failure below 0.8 confidence stays an objection the result stands over (visible in the observations, not incomplete), matching the existing threshold; making it block `/done` would regress turns on weak validators.
- Validation state is per call, not per (call, resource): a multi-file call's rejection marks every path it changed until a later write supersedes it.
- Effects are observed for write-mutation tools only. Shell and generated-tool effects keep their existing gates and receipts.

**Deferred:** the durable replay/rendezvous ledger and duplicate-identity conflict detection (ablation C / TOOL-EFFECT-03's replay half). The doc itself rates B as possibly sufficient, and replay needs crash-coupled storage. Also deferred: attribution under concurrent writers (no exclusive-ownership CAS exists to prove authorship), and the TOOL-EFFECT-04 production-entry evaluation, which needs live models.

## As-built status

Re-verified 2026-10-28 on `main` (after `4be8b4a8`). The 2026-10-27 "As built" section above still holds; this closes the open receipt-projection item and records the re-check.

### Claim ledger

| Claim | Verdict | Evidence |
|---|---|---|
| §3: a post-validator rejection returned empty output and skipped write accounting | Real, fixed earlier; still fixed | `executor_tools.go:1045` calls `settleToolEffects` before any error returns; `PostconditionError` carries the output (`tool_effects.go`) |
| §4: the bridge inspected only the first failure, and cancellation read as no opinion | Real, fixed earlier; still fixed | `SummarizeValidation` (`action_validator.go:312`), `classifyValidation` (`:283`); `FirstFailure`/`ValidateAll` no longer exist in `internal/` |
| Receipts feed Mangle; acceptance withholds `turn_verified` on a rejected or unmeasured effect | Confirmed | `policy/tool_effects.mg`; `coder_safety.mg:730`, `:734` |
| `SuccessfulWriteTools` defined the write set | Fixed earlier (renamed `WriteToolEffects`, widened) | `change_evidence.go:75` |
| Open: receipts did not cross to the parent through `observation.Return` | Real, fixed | `Return.Effects` (`observation/subagent.go`), filled by `projectReceipts` (`observed_return.go`); unsettled receipts projected by `unsettledEffects` into `structuralUncertainty` and `PartialState` |
| TOOL-EFFECT-03 replay half (durable rendezvous, duplicate-identity conflict) | Still not built | No crash-coupled receipt store exists |
| Attribution under concurrent writers | Still open | `commitMu` now covers codedom line verbs (see `edit-observation-precondition-continuity.md`), but non-codedom writers have no ownership protocol |

### What was built

- Ruling on the identity/redaction/retention question the delegated-turn work left open: a parent receives every receipt with call id, tool, write-clock seq, per-path before/after content identities (`sha256:`/`absent`/unknown), validation state and reason, handler error and the policy's recovery. Identities only, never content, so the receipt is safe to serialize and retain with the return and nothing is cut.
- `observation.EffectReceipt` / `ResourceEffect` (`internal/observation/effects.go`). A receipt with a recovery is projected as `tool effect not settled: <tool> (<call>) changed <paths>; validation <state> (<reason>); owed recovery <move>` in the parent's `unsettled` list and in `PartialState`; a measured receipt adds nothing.
- Spec: `Docs/architecture/session/IMPLEMENTED_SPEC.md` Phase I.

### Tests

`internal/observation/effects_test.go` (unsettled receipt reaches the projection and the partial state, an unchanged resource is not named, a measured receipt adds no uncertainty, JSON round trip); `internal/session/observed_return_test.go` `TestObservedReturn_ProjectsToolReceipts` (executor receipt to parent text). `go test ./internal/observation ./internal/session -run ObservedReturn` pass.

### Left open

- Replay/rendezvous ledger and duplicate-identity detection (TOOL-EFFECT-03 replay half): needs crash-coupled storage; ablation B remains the shipped point.
- TOOL-EFFECT-04 production-entry evaluation: needs live models; not run.
