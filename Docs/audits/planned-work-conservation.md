---
doc-class: deep-dive
subsystem: session
implementation-status: partially-implemented (see As-built status)
last-verified: 2026-10-28
verified-against: c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c
main-compared-against: 356f65e0e6bb639adaa90ab36212daa714114871
verification-method: static-source-inspection
supersedes: []
---

# Planned-work conservation across bounded execution windows

## Decision

Bound the work admitted to an execution window without silently reducing the set of work the executive knows remains. A capped plan must be explicitly refused or retain an owned remainder; completing its selected prefix must not be represented as completing the admitted plan.

The specific current gap is in the session's **edit-step planner**, not chat's background successor chain. The parser stops at `StepPlanMaxSteps` and returns only a slice of those first steps. No overflow disposition or remainder reaches the execution ledger or the step verdict. This remains true on both current implementation branches.

**Important limit:** the original task text remains in each step anchor and in the final verification history. It has not disappeared from every model request. Outer acceptance, report auditing, test gates, or a model that notices the missing scope can still reject or repair an incomplete result. The established loss is narrower: the harness has discarded explicit planned obligations, its step-count statement is prefix-only, and its step-completion predicate cannot account for the omitted steps. This report does not claim that every over-cap task receives an overall success verdict.

**Evidence status:** static inspection only. No tests, executable fixtures, model inference, builds, benchmarks, code implementation or user-computer tasks were performed for this investigation. The traces and evaluation cases below are proposed witnesses, not reproduced incidents. This document reports research, not implemented behavior; publishing the report does not change that evidence status.

## Source freeze and branch distinctions

At 13:03 UTC on October 28, 2026, main resolved to [356f65e](https://github.com/theRebelliousNerd/codenerd/commit/356f65e0e6bb639adaa90ab36212daa714114871), `feat/conversation-brief` to [c2d39e5](https://github.com/theRebelliousNerd/codenerd/commit/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c), and the research inventory to [691d1ee](https://github.com/theRebelliousNerd/codenerd/commit/691d1ee1f5a16f694858b45cbf566481dbcbf96a). All three heads were rechecked unchanged at 13:12 UTC. Citations below use those immutable commits.

The `turn_steps.mg` policy is byte-identical between main and feature. The two `work_steps.go` files differ in their response-control admission call at line 870: main uses `processPiggybackControlPacket`, feature uses `admitResponseControls`. The parser, plan admission, anchors, step report, final-state measurement and completion handling relevant here are identical. Feature's improved control admission therefore does not repair plan overflow. [Main source](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/work_steps.go), [feature source](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/session/work_steps.go).

### Intended architecture versus implemented contract

The root [North Star](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/agents.md#north-star) assigns planning, memory, orchestration and completion to the logic executive, while the model handles problem solving. [Session guidance](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/session/agents.md) describes the existing edit-step mechanism, per-file coverage, post-edit proof and revision-bound completion. It is not a specification of a durable obligation ledger.

The implementation deliberately bounds a plan. Both the configuration comment and parser comment say a larger task is not one turn's work and the executive runs the first steps and reports. The default is twelve steps and the planning threshold is two sites. [Configuration](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/config/session.go#L35-L43), [defaults](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/config/session.go#L86-L93).

This proposal does **not** call the cap itself a defect or suggest removing resource bounds. It identifies that the reported and derived state does not distinguish a fully admitted plan from the retained prefix of a larger one. Changing that contract is prospective work, not a description of code that already exists.

**Prior art inside the repository:** Forcing and completion already describes the cap and first-steps reporting in [row 23](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/Docs/journeys/02-forcing-and-completion.md#L85-L87), and the model-prose no-change escape in [section 4.3](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/Docs/journeys/02-forcing-and-completion.md#L513-L548). Its [later proposal](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/Docs/journeys/02-forcing-and-completion.md#L942-L951) already suggests generalizing the step ledger into obligations. This report is not discovery of those mechanisms or invention of obligation ledgers. Its additional contribution is tracing the current denominator loss through parser, anchor, report and Mangle completion, and specifying a narrow overflow exit. The older journey's statement that planned-step decisions are entirely in Go and the file never touches the kernel is stale at these refs: `recordStepPass`, `stepCommitRetry`, `derivedStepOutcome` and `turn_steps.mg` now carry the measurement/decision split. The overflow gap survives that migration.

### Relationship to existing research

The current `Docs/audits` inventory and relevant existing reports were read before selecting this seam:

| Existing work | Boundary retained, not repeated |
|---|---|
| [TUI evidence-turn ownership](https://github.com/theRebelliousNerd/codenerd/blob/691d1ee1f5a16f694858b45cbf566481dbcbf96a/Docs/audits/tui-evidence-turn-ownership.md) | Describes conditional edit planning as an entry-path fact; its findings concern evidence ownership and delivery, not plan overflow. |
| [Background outcome and continuation ownership](https://github.com/theRebelliousNerd/codenerd/blob/691d1ee1f5a16f694858b45cbf566481dbcbf96a/Docs/audits/background-outcome-and-continuation-ownership.md) | A known job's successor is lost or misowned after completion. Here an inner worker's accepted plan suffix never becomes executable obligations. Repairing background handoff alone cannot recover it. |
| [Delegated-turn failure settlement](https://github.com/theRebelliousNerd/codenerd/blob/691d1ee1f5a16f694858b45cbf566481dbcbf96a/Docs/audits/delegated-turn-failure-settlement.md) | Preserves measured results on hard failure. Here a prefix-only plan may return normally; there need not be a hard error to preserve. |
| [Verification applicability and progressive evidence](https://github.com/theRebelliousNerd/codenerd/blob/691d1ee1f5a16f694858b45cbf566481dbcbf96a/Docs/audits/verification-applicability-and-progressive-evidence.md) | Concerns judgment applicability, evidence acquisition and request budgets. A better judge is defense in depth, not a replacement for conserving the executive's known remaining scope. |
| [Structure coverage and refactor preconditions](https://github.com/theRebelliousNerd/codenerd/blob/691d1ee1f5a16f694858b45cbf566481dbcbf96a/Docs/audits/structure-coverage-and-refactor-preconditions.md) | Structural discovery and mutation preconditions. No new CodeDOM index or rewrite backend is required for the overflow contract. |

This is distinct from the session spec's `GAP-SESSION-STEP-NET-STATE`. The current [gap row](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/Docs/architecture/session/03-GAP-ANALYSIS.md#L193-L199) still describes final file-state assertion as pending, but the current [host measurement](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/session/work_steps.go#L374-L428) and [post-repair call](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/session/work_steps.go#L905-L909) are present. Do not reopen that older missing-wiring claim from the stale row. Documentation should be reconciled separately with observed test status, not marked fully verified from this inspection.

## 1. Production path and what already works

The feature TUI's [delegation helper](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/delegation.go#L50-L74) uses the observed task executor when available. The JIT executor's [inline path](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/session/task_executor.go#L352-L409) invokes `ProcessWithIntent` on a task-owned clone. The session then reaches `runToolLoop`, which chooses planned execution when the planner returns more than one step. [Feature dispatch](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/session/executor_tools.go#L60-L74), [main dispatch](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/executor_tools.go#L57-L67).

Planning is conditional: a working loop must exist, the intent must be write-oriented, a continuation channel and write tool must be available, and policy must derive that the brief needs planning. A planning failure, empty completion or fewer than two parsed steps falls back to the ordinary single pass. This is not a universal planner for every chat request. [Admission and fallback](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/session/work_steps.go#L166-L230).

Each admitted step gets its own prompt focus and loop. A no-write pass can receive one commit-regime retry; the task then gets a shared post-edit gate and a final file-state measurement before the step verdict. Unknown preimages and unreadable final state fail rather than masquerading as unchanged. These safeguards must survive any plan-conservation repair. [Execution and closing gate](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/session/work_steps.go#L799-L926).

## 2. Finding: a bounded prefix becomes the entire executive plan

The loss can be followed without assuming a model failure:

1. The planner is asked for one `STEP` per requested file region; its instruction does not require it to compress its answer to twelve steps. [Planner instruction](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/session/work_steps.go#L41-L46).
2. `parseWorkSteps` parses and deduplicates valid edit steps, appends each to a slice, and immediately breaks once its length equals `maxSteps`. The return type is just `[]workStep`: no total count, overflow flag, rejected suffix or continuation token. [Parser](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/session/work_steps.go#L81-L123).
3. `planTurnSteps` accepts that slice, logs its size, and returns it. The raw planner text remains a local variable and is not supplied as the plan to `runPlannedSteps`. [Caller](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/session/work_steps.go#L196-L230).
4. The anchor says the task runs as `len(steps)` steps, calls the current step `i of len(steps)`, and instructs the solver to make exactly that step's change because remaining steps will run later. Only the retained slice is sequenced. The full user task is still included above that instruction. [Anchor](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/session/work_steps.go#L274-L296).
5. The report counts and enumerates only that slice. It has no omitted-work count or partial-plan disposition. [Report](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/session/work_steps.go#L589-L621).
6. Policy derives `turn_has_step` from existing `step_execution` rows and completion from the absence of an unresolved or reverted recorded step. There is no admitted-plan manifest to compare against and no fact for the truncated suffix. [Policy](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/core/defaults/policy/turn_steps.mg#L107-L132).

If the retained steps satisfy the existing post-edit gates and step policy, `runPlannedSteps` can return nil error after executing only that prefix. This is a **local step-verdict conclusion**, not a proof of an overall accepted task. `ProcessWithIntent` subsequently closes acceptance evidence, audits the report and checks hollow success. [Outer settlement](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/session/executor.go#L1324-L1370).

The existing parser test deliberately constructs `default + 5` valid steps and asserts that only the configured maximum is returned. This confirms the cap is intentional; it does not check that the discarded obligations are represented elsewhere. The test was read, not run. [Test definition](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/session/work_steps_test.go#L145-L172).

### Conditional source trace

Let the configured maximum be K, with K at least the planning threshold. The authorized request names K+1 independent edit sites, and the planner returns K+1 valid, distinct steps in that order. The parser admits only steps 1…K. Each pass obeys its anchor, changes only its named site, and the shared gates accept the resulting changes. No retained step is unresolved or reverted. Policy therefore derives local `/complete`, the report lists K steps, and the K+1st step has neither an execution row nor an owed-work entry.

This trace does not require a malformed planner response, a provider error, a stale file or a permission bypass. It also does not prove the final answer is wrong: the original task is still available, and independent acceptance may catch the omission. The gap is that the system relies on later semantic rediscovery for scope it had already explicitly identified.

## 3. Adjacent intentional limits: do not overstate the repair

Conserving the plan does not make a planned-step verdict a semantic proof of each requested change. These weaker guarantees are already documented, including in the forcing/completion journey above, and are not presented as new findings.

* **Per-file coverage is intentionally coarse.** A zero-write step can be covered by another step that wrote the same file; the policy does not require that the two changes are equivalent. The rule also does not order the covering step before the covered one. Its comments explicitly acknowledge per-file, not per-hunk, state comparison. [Policy contract and rules](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/core/defaults/policy/turn_steps.mg#L43-L61), [coverage relation](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/core/defaults/policy/turn_steps.mg#L91-L110). A test states that the guarantee is per-file editing and expects this behavior. [Test](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/session/work_steps_test.go#L408-L433).
* **No-change evidence is currently a nonempty model-authored explanation.** The parser checks the marker and nonempty suffix; it does not independently resolve a cited source location. [Parser](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/session/work_steps.go#L259-L271), [fact publication](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/session/work_steps.go#L875-L880). This is explicitly exercised by existing tests; it is not an undiscovered alternate tool authorization path. [Tests](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/session/work_steps_test.go#L319-L391).

Those are candidates for a later obligation-evidence contract, not necessary additions to the minimal overflow fix. Preserve their current meanings in compatibility tests rather than implying a manifest alone proves every hunk or no-change claim.

## 4. Proposed contract and implementation choices

All names and shapes in this section are design terms, not existing APIs or predicates.

### A. Minimal conservative repair: explicit plan overflow refusal

The parser reports whether a further valid distinct step exists beyond K. Before running the planned prefix, the executive derives an explicit oversized-plan disposition. It reports that the plan exceeds the admitted window and retains the original task without presenting a K-step plan as the whole task. If the system cannot retain a safe owned remainder yet, an honest refusal or request to split is sufficient.

This variant changes no execution backend and does not require durable resumability. It need not count an arbitrarily long suffix: a bounded parser can report `more than K` or `plan completeness unknown` rather than invent an exact total. It must distinguish exactly K steps from more than K, and malformed/duplicate/non-edit lines must follow an explicit counting contract. Do not merely remove the cap or ask the model to omit steps so the response fits.

### B. Stronger capability: execution windows over a retained plan

Retain the authorized admitted plan separately from the active window. A task-owned record can carry:

- Plan identity and generation, original-request identity, authorized scope and parser/admission disposition.
- Stable identities for accepted steps, their target/change descriptions and source order.
- Which steps are selected now, which are executed, which remain pending, and which have an explicit waived/superseded disposition.
- Measured evidence references and current applicability, kept separate from the model's explanation.
- Remaining-work ownership, cancellation state, and a continuation disposition rather than automatic unbounded execution.

Go supplies measured parser results, execution records and resource facts. Mangle decides whether a window can start, whether a step remains owed, whether another window is authorized, and whether the admitted plan is complete. The active count K remains a resource constraint; it no longer deletes knowledge. Count-only metadata is sufficient for explicit partial reporting, but automatic continuation requires the actual admitted suffix or a reliable owned reference to it.

**Authority:** the planner's extra lines are proposals, not new user authorization. An accepted plan must remain within the original request and effective capabilities. Keep rejected/invented steps distinct from pending authorized work. A model's incomplete plan is a separate problem: conserving every parsed step does not prove that it captured every user requirement.

**Closure:** no plan-complete claim while admitted obligations are pending, blocked, unknown, or missing disposition. A window can finish while the task remains partial. Reporting should state both the executed window and remaining scope. Build/test/acceptance requirements remain independent; an empty remainder cannot fabricate successful evidence.

**Continuation:** select only unfinished authorized work under current constraints. A stopped or canceled task does not resume because a remainder exists. Do not replay completed effects to reconstruct a missing ledger. If a later window depends on earlier results, revalidate relevant revisions and report missing evidence instead of treating retired worker recall handles as durable.

**Lifetime:** the current inline worker retires its working scopes when it returns. Therefore cross-task continuation is not obtained by storing a `recall_context` handle in the plan. It needs an explicitly owned transfer or a durable, authorized evidence store; otherwise remain within one task or stop with an honest partial result. [Current lifetime](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/session/task_executor.go#L361-L367).

**Presentation:** show the current window, a concise completed summary and the owned remainder count/handle, not the entire expanding plan on every model call. New model-facing behavior belongs in JIT atoms. Retain the original request where needed; do not conflate a smaller presentation with a smaller obligation set.

### Recommended adoption order

1. Specify and implement explicit overflow disposition before effects, with a complete-vs-prefix distinction in the report and local verdict.
2. Add a retained in-task remainder only if product requirements justify automatic windowing. Carry original scope, constraints and existing gates unchanged.
3. Add cross-task recovery only with an owned persistence/handoff contract. Integrate with, rather than duplicate, the existing task-terminal and background-continuation work.
4. Consider stronger per-step evidence and coverage separately; do not make them a hidden prerequisite for the minimal conservation repair.

## 5. Evaluation design — not executed

Use deterministic providers to establish host contracts first, then normal production entry paths with real CodeDOM/world context to measure usefulness. Do not count model compliance with a hand-supplied answer as evidence of planning quality.

| Proposed case | Required observation |
|---|---|
| Exactly K valid distinct steps | No false overflow; all admitted steps represented. |
| K+1 valid distinct steps, final one independent | Either explicit pre-effect overflow refusal, or one owned pending obligation after the first window; never a full-plan-complete disposition for the prefix. |
| K+5 plus duplicate, malformed and verification lines | Defined counting behavior; invalid lines do not inflate accepted totals and valid late steps are not silently lost. |
| K+1 planner steps but one lies outside authorized scope | Rejected proposal is not executed or recorded as an authorized debt. |
| Planner omits a user-requested site | The system must not claim conservation proves requirement completeness; measure independent acceptance separately. |
| Last admitted step fails after a write | Existing effect and failure settlement preserved; remainder neither erased nor replayed. |
| Cancellation before the next window | Remaining scope retained as canceled/blocked according to policy; no automatic dispatch. |
| Prior completed effect then restart/retired working scope | No effect replay merely to repopulate evidence; missing retained proof is explicit. |
| Earlier step changed by a later window | Evidence applicability is rechecked; completed labels are not assumed timeless. |
| Two independent regions in one file | Preserve current per-file semantics for the minimal fix; use as a separate negative control for any stronger per-obligation claim. |

The primary conditional fixture can use thirteen explicit independent text-file changes at the default K=12. Instrument the planner output, parsed plan identity, actual executed targets, remaining disposition, report, local step verdict and outer acceptance separately. Ensure the gate result is observable; do not disable it and then claim the full production path passed. A smaller configured K is valid only if it respects configuration's minimum-site relation.

Read both native and Piggyback channel paths: the planner gate supports both, and changing only one renderer must not produce two completeness contracts. Compare minimal refusal, retained windowing, and the current implementation under identical task and evidence availability. Measure first-window latency, total completion latency, calls, token cost, repeated reads, effect replays, explicit remaining-work accuracy, and whole-request acceptance. A lower initial cost achieved by doing less work is not an efficiency improvement.

## 6. Costs, risks and decision criteria

**Minimal refusal** adds little persistent state and closes ambiguous reporting quickly, but moves splitting/retry effort to the user or calling executive. It can lower automatic completion rate on large tasks. That is an honest tradeoff, not a performance win by itself.

**Retained windows** require bounded plan storage, identity/versioning, ownership, cancellation and evidence applicability. Parsing all S accepted steps costs O(S) work and storage, while the active presentation can remain O(K); a separate hard byte bound must return explicit incompleteness if full admission is impossible. A task with thousands of proposed steps must not create unbounded memory or model context.

**Semantic risk** remains: a faithfully retained plan can be a bad plan, and a measured file edit can be the wrong edit. The strongest claim of this repair is conservation of admitted obligations. Keep independent whole-task acceptance, not a completion rule based only on total step counts.

**Integration risk** is duplicate ownership: a session remainder, TUI background chain and campaign plan must not each independently dispatch the same work. Prefer one owner with typed references. A continuation request is not a fresh authorization or a new license to run completed writes.

**Adoption criterion:** an over-cap authorized task must have a truthful, machine-readable incomplete/overflow state before any full-plan completion is published. Windowing is justified only if it improves accepted whole-task completion or reduces total cost without raising replay, stale-evidence or cancellation violations. No such improvement has been measured by this report.

## 7. Primary foundations, bounded analogy and handoff

The primary evidence for the finding is the pinned production parser, executor, policy, guidance and test definitions above. External literature supplies design vocabulary, not validation of the CodeNERD finding or the proposed repair.

Theo Haerder and Andreas Reuter's *Principles of Transaction-Oriented Database Recovery* (1983) separates materialized database state, propagation strategy, checkpoints and logging techniques. Its published abstract explicitly connects those distinctions to recovery scope, speed and redundant-information cost. The useful analogy is that a selected execution window, the retained work record, and observed filesystem effects are different state: retaining one does not establish the others. The design must say what its checkpoint records and what information supports resumption. [IBM Research publication and abstract](https://research.ibm.com/publications/principles-of-transaction-oriented-database-recovery).

That is an architectural analogy, not a claim that database recovery results prove this proposal. CodeNERD's filesystem/tool effects are not automatically an atomic database transaction; the report has not established a write-ahead logging protocol, rollback, serializability, exactly-once effects or crash-safe plan persistence. For that reason the minimal repair is explicit overflow disposition, while resumability is a separate capability with ownership and replay requirements. The literature does not justify rerunning a completed external effect when its report is missing.

Bounded windowing and obligation ledgers are established ideas, including the repository's own earlier journey proposal. The contribution here is a specific missing conservation contract on the current parser-to-verdict boundary and a checkable exit, not a claim of a new recovery algorithm.

Follow the repository's [architecture documentation standard](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/Docs/journeys/09-architecture-doc-standard.md): specify the session capability and checkable exit first; implementation and test results must be recorded separately. A proposed gap row should name plan-admission completeness, not reuse the already-implemented final-file-state wiring row.

**Checkable exit:** a planner answer with more valid authorized steps than the active cap cannot silently become a complete smaller plan. It is explicitly refused or retains the unexecuted suffix with one owner and an incomplete disposition; no completed effect is replayed merely to repair the ledger. Current capability, permission, deadline, cancellation, post-edit and whole-task acceptance gates remain in force.

## As-built status

Verified and built 2026-10-28 on `main` after `c28a1212`. The parser, anchor, report and policy were unchanged from the audited pins.

### Claim ledger

| Claim | Verdict | Evidence |
|---|---|---|
| `parseWorkSteps` breaks at `maxSteps` and returns only the prefix, with no overflow signal | Real, fixed | It now keeps every valid distinct step and marks those past the bound `Beyond` (`internal/session/work_steps.go:107`; `splitPlan` `:77`) |
| `planTurnSteps` returns the slice; the planner's text is local | Real, fixed | It returns the whole plan and logs the overflow; the one-pass threshold counts runnable steps only |
| The anchor says the task runs as `len(steps)` steps | Real, fixed | `workStepAnchor` states how many steps lie beyond and that they are not to be made here (`:300`) |
| The report counts and lists only the prefix | Real, fixed | `workStepReport` counts the whole plan and lists each remaining step (`:635`) |
| Policy has no fact for the truncated suffix; `/complete` derives from recorded rows only | Real, fixed | `step_plan_remaining/3` declared and joined into `turn_has_unresolved_step` (`turn_steps.mg:80`, `:127`); asserted before any step runs (`work_steps.go:852`); named in the incomplete error (`derivedStepOutcome`, `:571`) |
| The existing parser test asserts the cap and nothing else | Confirmed, rewritten | `TestParseWorkSteps` now asserts the bound runnable and the 5 past it kept |
| Per-file coverage and model-authored no-change evidence are intentional limits | Confirmed, unchanged | `turn_steps.mg` coverage rules; `noChangeEvidence` |
| `GAP-SESSION-STEP-NET-STATE` row is stale | Confirmed stale; not reconciled here | Host measurement exists (`assertPlannedStepFileNet`) |

### What was built

- Ruling: variant A with the remainder retained in the turn, not a refusal. The window runs (useful work), the verdict is `/incomplete` while any step past the bound is owed, and the error and report name each remaining step so the caller and the user see exactly what is left. No cross-task windowing.
- `internal/session/work_steps.go`, `internal/core/defaults/policy/turn_steps.mg`; spec `Docs/architecture/session/03-GAP-ANALYSIS.md` `GAP-SESSION-STEP-PLAN-OVERFLOW`.

### Tests

`internal/session/work_steps_test.go`: `TestRunToolLoop_PlannedSteps_StepsBeyondTheBoundStayOwed` (bound 2, three steps: two edit, the third is not run, `ErrStepsIncomplete` names it, report and anchor state the remainder); `TestParseWorkSteps` (bound kept, 5 beyond retained). The other planned-step tests pass unchanged.

### Left open

- Retained windows that run the remainder in a later task (variant B) and cross-task ownership: not built; the remainder exists only in this turn's facts, report and error.
- A plan larger than the planner's own output limit cannot be counted beyond what the planner wrote.
- A new `Decl` was added: the predicate corpus (`internal/core/defaults/predicate_corpus.db`) needs the shared rebuild.
