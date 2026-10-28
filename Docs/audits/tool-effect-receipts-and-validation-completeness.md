---
doc-class: deep-dive
subsystem: tools
implementation-status: planned
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

## Appendix: Same-revision validation settlement

**Status:** source-inspected residual after the receipt implementation; no code, tests, model inference, training, or benchmarks were run. The traces and evaluations below are conditional deductions and proposed fixtures, not observed incidents.

**Inspected pins:**

- `main`: `356f65e0e6bb639adaa90ab36212daa714114871`
- `feat/conversation-brief`: `c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c`
- `codenerd-research-docs`: `1e1a893e7995c799b8b547c229a765ca4772a3c6`

The relevant `tool_effects.go`, `tool_effects.mg`, `coder_safety.mg`, and receipt-test blobs are identical on the two implementation refs. This appendix supplements this audit rather than replacing its historical baseline or reopening its explicitly deferred replay and concurrent-writer work.

### A. What this adds—and what it does not

The main branch's [**As built** section, lines 189–205](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/Docs/audits/tool-effect-receipts-and-validation-completeness.md#L189-L205) correctly reports substantial implementation: failed writes remain accounted, output survives postcondition failure, typed validation distinguishes incomplete observations, and policy derives recovery. The original output-loss and missing-write-ledger findings should **not** be repeated as current defects.

The remaining seam is narrower:

> **A later successful validation of the same bytes does not settle an earlier incomplete receipt, because publishing that validation is conditional on observing another content change.**

This is primarily a **liveness and evidence-conservation defect**, not a demonstrated false-success or authorization bypass. The executive conservatively withholds a completion claim, but it can continue withholding within the same turn after the relevant validator has actually measured the unchanged result successfully.

The audit already explicitly defers durable replay/rendezvous and concurrent-writer attribution. This appendix does not repackage those deferred topics. It concerns the implemented, in-memory policy half of recovery: associating a later observation with the existing effect revision.

### B. Exact conditional source trace

Consider a normal authorized write turn without a caller-supplied acceptance contract:

1. **First call changes a file.** A registered `write_file` writes valid JSON, for example `{"enabled":true}`, replacing different bytes. The ordinary handler does not require the new content to differ from existing content; it writes through `atomicfile.WriteFilePreservingMode`, records changed-source information, and returns success. [Handler, lines 336–365](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/tools/core/file_ops.go#L336-L365).

2. **Its call-local deadline expires at the post-validation boundary while the parent turn remains live.** Session creates a separate bounded tool context, then uses it for execution and post-validation. [Tool context, lines 2617–2627](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/executor_tools.go#L2617-L2627), [post-validation, lines 2679–2686](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/executor_tools.go#L2679-L2686). The validator registry explicitly turns cancellation before a validator into `ValidationUnavailable`; summarization produces `ToolValidationIncomplete`. [Registry, lines 225–240](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/action_validator.go#L225-L240), [summary, lines 312–347](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/action_validator.go#L312-L347). Independently, each validator receives its own 30-second child context; an unclassified failed reading after that deadline is classified unavailable. [Validator deadline and classification, lines 244–293](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/action_validator.go#L244-L293). This alternative is conditional on the validator returning that failed, unclassified reading; it is not a claim that every validator honors the deadline or that a standard validator has been observed timing out.

   These are possible orderings supported by production code, **not observed incidents or incidence estimates**. Whole-turn cancellation is not required: a child context can expire before its parent. The first trace needs only the tool-local deadline to expire after the handler writes and before a registry validation-boundary check.

3. **The changed file receives an incomplete receipt.** `settleToolEffects` records the observed change and validation state. Policy derives `/inspect` for `/incomplete`. Automatic `/revalidate` is restricted to `/not_run` at round zero; it does not cover an incomplete validation. [Receipt settlement, lines 230–283](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/tool_effects.go#L230-L283), [recovery rules, lines 96–108](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/defaults/policy/tool_effects.mg#L96-L108).

4. **A separately admitted later call writes exactly the same bytes and its validators complete successfully.** Give it a new call ID; no duplicate-identity or replay behavior is needed. All current gates must admit it. `executeToolCall` records the verified validation in the per-call log. However, the before/after content identities now match.

5. **The successful observation is discarded at settlement.** `settleToolEffects` reads that validation, accounts the successful write, then returns at `if !changed` **before** `assertToolReceipt`, `assertToolValidation`, or appending `ToolReceipts`. [Lines 236–283](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/tool_effects.go#L236-L283). Merely removing this early return would be insufficient: `assertToolReceipt` skips unchanged effects, while the policy still joins validation by the original effect's call ID. [Lines 303–318](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/tool_effects.go#L303-L318).

6. **The earlier incomplete receipt remains current.** Policy selects the latest **effect** per path, and the latest validation round for that effect's **call ID**. The second call contributes no newer effect or linked validation round. [Policy, lines 40–84](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/defaults/policy/tool_effects.mg#L40-L84). Closing supersession also does not retire the old receipt: it deliberately skips a path whose current digest equals the recorded `After` digest. [Supersession, lines 364–385](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/tool_effects.go#L364-L385).

7. **Ordinary gate-based completion remains withheld for this turn.** `turn_verified`'s write arm excludes `turn_has_unmeasured_effect`; the missing-evidence projection names `/effect_unmeasured`. Green build/test evidence does not independently settle this tool-validation obligation. [Completion, lines 676–678](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/defaults/policy/coder_safety.mg#L676-L678), [missing evidence, lines 726–734](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/defaults/policy/coder_safety.mg#L726-L734).

The example repeats a content-idempotent write only to expose the lost observation. It is **not** a recommendation to recover by rewriting files. Identical content does not imply no physical effect: an atomic replacement may still change metadata or trigger watchers.

### C. Production reachability and limits

The shared accounting wrapper is explicitly used for native and Piggyback calls; it invokes execution, then settlement before returning the result. [Main wrapper, lines 975–1045](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/executor_tools.go#L975-L1045), [feature wrapper](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/session/executor_tools.go#L980-L1050).

For the feature TUI, the delegation helper uses `ExecuteObservedWithContext`; the task executor calls `ProcessWithIntent`, which enters `runToolLoop`. [TUI helper](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/delegation.go#L61-L74), [task executor](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/session/task_executor.go#L352-L409), [loop entry](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/session/executor.go#L1321-L1327).

This proves a reachable source path, not that a particular deployed run exhibited the schedule. A valid witness must establish actual tool availability and exact admission, not fabricate a call absent from the production catalog.

**Counterexamples and boundaries:**

- **The debt is turn-local, not a permanent cross-turn lock.** `checkHollowSuccess` reads and captures the turn's verdict before deferred `cleanupTurnFacts`. A fresh turn does not inherit this turn's receipt facts through this path. Starting another turn can therefore escape this particular stale obligation, although it does not recover the lost validation observation inside the original turn. [Closure and cleanup, lines 2164–2208](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/executor_tools.go#L2164-L2208).
- A later verified write that actually changes the content can supersede the earlier effect. The existing repair test exercises that case. [Test, lines 145–185](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/tool_effects_test.go#L145-L185).
- A handler failure after a change receives the existing immediate `/not_run` revalidation. That mechanism is real and must remain. [Test, lines 192–216](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/tool_effects_test.go#L192-L216).
- A verified caller contract has a separate `turn_verified` arm. Therefore this receipt does **not universally prevent completion**. Do not manufacture or weaken acceptance contracts to escape it.
- Optional `/unvalidated` outcomes are deliberately different from `/incomplete`; this appendix does not convert optional coverage into a mandatory failure.
- Ordinary reading of the file is useful diagnosis, but the inspected producer path does not link that read to a new `turn_tool_validation` round.
- A genuinely changed revision, different resource, different validator obligation, denied call, or incomplete recheck cannot settle the original obligation merely because it occurred later.

The existing incomplete-validation test stops after confirming that `/inspect` and `turn_has_unmeasured_effect` arise. It does not exercise subsequent same-byte successful validation. [Test, lines 221–238](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/tool_effects_test.go#L221-L238). These tests were inspected, not run.

### D. Proposed contract: validation progress need not require mutation

Preserve the fail-closed rule. The correction is **not** to ignore incomplete receipts, clear them when the model says it inspected the file, or accept any later successful tool.

A proposed host-owned settlement should:

1. Identify the outstanding receipt, resources, recorded revisions, validator obligations, and owning turn.
2. Require current authority for any validation/read operation and disclosure. Historical write permission is not new permission.
3. Measure the same revision through the appropriate validators without rerunning the original effect.
4. Append a new validation observation linked to the existing receipt and revision. Preserve the earlier incomplete observation.
5. Let Mangle determine whether the new evidence settles the outstanding obligation.
6. Keep changed, unreadable, canceled, unsupported, or mismatched state explicit. Do not turn uncertainty into a pass.
7. Leave the write clock and effect count unchanged for validation-only recovery.
8. Honor parent cancellation and the remaining task budget; do not detach recovery indefinitely.

The currently documented per-call validation model matters: if one call changed multiple paths, measuring one path cannot certify all of them. A bounded first implementation can remeasure the complete original resource/obligation set rather than redesigning the entire model as per-resource receipts.

Matching digests before and after a live validation is not by itself a proof against every intervening writer or ABA sequence. Prefer validators operating on a frozen verified input where possible; otherwise state the observation/ownership limitation. This appendix does not claim to solve the deferred CAS-attribution problem.

### E. Alternatives and cost

| Alternative | Benefit | Limitation |
|---|---|---|
| Keep current conservative result | No implementation cost; no new acceptance path | Truthful but potentially stale `/effect_unmeasured`; extra model/user work |
| Carry an already-produced same-revision validation into the existing receipt | Smallest repair for the demonstrated lost observation | Needs explicit matching and complete obligation coverage; alone does not provide validation-only recovery |
| Host-driven, policy-authorized revalidation of outstanding incomplete receipts | Settles evidence without rewriting files or asking the model to manufacture a change | Additional validator I/O/CPU, revision checks, cancellation and scheduling work |
| Expose a narrowly typed revalidation operation | Explicit solver affordance | Catalog/JIT/admission surface and extra model round; must not become arbitrary execution |
| Clear debt on any later successful call or green build | Cheap | **Reject:** unrelated success does not establish the missing validation |

Measure separately: validator invocations, bytes read/hashed, host CPU, receipt/fact storage, model tokens, repeated writes, post-write gate reruns, elapsed recovery time, unresolved turns, and human interventions. A validation-only path can avoid rewrite and model-loop costs, but no savings are established here.

For `B` bytes across the affected resource set, an additional whole-content hash is `O(B)` work; validator costs are separate. Avoid blindly revalidating every receipt every round. Index unresolved current obligations and derive eligibility from new state, remaining budget, and policy. Failed and blocked tasks belong in the evaluation denominator.

### F. Primary foundations, used narrowly

- [RFC 9110 §9.2.2](https://www.rfc-editor.org/rfc/rfc9110.html#section-9.2.2) distinguishes idempotent request semantics and limits automatic retry of non-idempotent operations. The useful analogy is that repairing evidence and repeating an effect are different operations—not that file tools inherit HTTP guarantees.
- [RFC 9110 §13.1.1](https://www.rfc-editor.org/rfc/rfc9110.html#section-13.1.1) binds a conditional request to a matching representation. This motivates explicit revision matching; it does not prove that CodeNERD's current path is atomic.
- [Lee et al., *Implementing Linearizability at Large Scale and Low Latency*, SOSP 2015](https://web.stanford.edu/~ouster/cgi-bin/papers/rifl.pdf) separates returning a retained operation result from executing the operation again. Here that supports keeping effect replay outside observation repair. Its distributed-storage guarantees do not transfer to these tools merely by adding a validation record.

### G. Proposed verification and exits—none executed

1. **Core loss witness:** changed write → incomplete validation → separately admitted byte-identical write → verified validation. Assert the successful observation exists at the bridge, then inspect receipt facts and final missing evidence.
2. **Preferred recovery:** changed write → incomplete validation → authorized validation-only recovery. Require unchanged bytes, one original effect dispatch, append-only observations, and settlement only after the correct validators complete.
3. **Negative controls:** different path; different revision; missing validator; repeated incompleteness; definitive rejection; denied recovery; canceled parent; unreadable resource; stale policy/authority.
4. **Multi-file case:** one original path remains unmeasured or rejected. Revalidating a sibling must not settle the entire call.
5. **Existing behavior:** preserve immediate handler-failure revalidation, changed-byte repair, denied-call zero-effect behavior, optional-validator handling, turn-local cleanup, and the separate caller-contract route.
6. **Production paths:** native and Piggyback, real catalog/admission, real kernel, normal task-executor entry with CodeDOM/world context enabled. Use deterministic barriers or a controlled clock for the deadline schedule, not flaky short sleeps.
7. **Outcome:** a complete valid observation of the same revision can advance evidence without inventing another mutation; stale or incomplete observations cannot. Confirm the ordinary final verdict and user-visible explanation agree.
8. **Comparative evaluation:** current baseline versus retained-observation settlement versus validation-only recovery, under identical acceptance and authority. Report correctness, unresolved results, recovery effort and total costs—not just local test success.

**Bottom line:** the implemented receipt system correctly preserves failure evidence. Its next small refinement is to let **new validation evidence settle an existing unchanged revision within the owning turn**, without requiring another content mutation or weakening completion gates.
