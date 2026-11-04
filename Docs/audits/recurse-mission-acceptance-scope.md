# Recurse mission acceptance can demand repairs outside the admitted node

**Status: source-grounded integration finding and proposed contract; no execution validation.** Public main was inspected at **`45a51a3b66232a42e018b3fa070370286d22f851`** on November 4, 2026 and rechecked unchanged at approximately 03:47 UTC. All repository references below are pinned to that revision. No tests, builds, gate commands, application paths, model inference or fixtures were executed. No implementation, repository, credential or setting was changed.

## Decision

Give a recurse fix one coherent, host-owned acceptance objective. The outer recurse policy accepts incremental repair: the selected finding disappears, no gate worsens and the other guards hold. The inner attempt campaign instead requires the entire command that reported that finding to return a passing exit. When that command checks several independent nodes, another baseline defect can make the inner objective impossible within the selected mission's immutable node scope. A correctly repaired target is still followed by a failing `run_check` or acceptance round because a different node remains broken. [Outer contract][ratchet] [Attempt contract][attempt-contract] [Mission scope][mission-prepare]

This is an **objective-and-scope mismatch**, not a wrong command, lost environment, incorrect exit-code rule, false passing gate or missing dependency. The reporting command can be copied and executed perfectly. Its whole-workspace success condition is nevertheless stronger than the single-finding repair admitted to the mission.

The bounded recommendation is to specify a typed incremental-repair acceptance contract, shared by the task, campaign and outer ratchet, or explicitly admit a larger atomic work item before dispatch. Do not quietly broaden write permission, turn a nonzero exit into a pass, remove `run_check`, or ask the model to decide which failures may be ignored.

## 1. Intended behavior and current composition

Root guidance assigns creativity to models and executive decisions to Mangle. The mission specification says repair priority and dependency order remain recurse policy; deliberation selects a design inside an already admitted work item. The requirements-interrogator skill asks for explicit scope, lifecycle, failure behavior and falsifying tests. Those requirements favor detecting an incompatible success condition before paying for a design or editing attempt. [Guidance][guidance] [Mission specification][mission-spec] [Interrogator][interrogator]

The relevant contracts currently differ:

| Layer | Actual contract at the pin |
|---|---|
| Recurse selection | Findings from a workspace gate are attributed to source nodes. A visit supplies only its owned findings to policy. |
| Attempt construction | Copy the selected finding's original reporting argv, environment and passing-exit rule into a one-node campaign. |
| Mission admission | Copy `Node.Paths` into `MissionReceipt.Scope`; reject writes outside that scope. |
| Editing turn | A net-writing turn offered `run_check` owes a passing result after its last write. |
| Campaign acceptance | The copied command must return a passing exit. Failure can append remediation using the campaign tasks' existing write set. |
| Outer recurse ratchet | A fix succeeds when its selected finding is gone. A failed gate with no increase in findings is not, on that basis alone, worse. |

The table describes source behavior, not executed outcomes. [Attribution][measure] [Candidate collection][open] [Selected check][attempt] [Campaign construction][campaign] [Turn check][turn-check] [Acceptance execution][acceptance-run] [Remediation][remediation] [Outer contract][ratchet]

### The selected finding and its whole command are not equivalent obligations

`attempt` obtains the selected finding's correct original invocation through `sourceOf` and copies `Argv`, `OKExitCodes` and `Env`. This preserves the recent invocation and configuration repairs. `RecurseAttemptCampaign` then places that full argv into `Acceptance.Command`, sets the task write set to `a.Node.Paths`, and tells the task to fix the named target so the command passes. The acceptance descriptor carries no target-finding identity or compatible baseline census with which to express partial success. Its `GateID` identifies the reporting gate, not the selected deficit. [Selected check][attempt] [Campaign construction][campaign] [Acceptance descriptor][acceptance-type]

The distinction matters for an ordinary workspace checker that reports two independent, located defects. Both defects were produced by the same correct invocation. Replaying that invocation after repairing one should still fail because the second remains. The command has not malfunctioned, and the second failure is not a regression.

### Mission scope makes the conflict enforceable

Mission preparation copies the selected node paths into the receipt before handing the attempt to the design-only deliberation thread. `checkMissionWrite` resolves a candidate path, determines whether it lies within the receipt scope, and asks policy for write admission. The outer ratchet independently treats paths outside that scope as forbidden. A model's decision to repair the other node therefore cannot legitimately resolve this mismatch by itself. [Mission preparation][mission-prepare] [Pre-write admission][mission-write] [Post-write checks][mission-outer]

This is specifically stronger than the ordinary campaign `WriteSet`. Ordinary `writeGuard` can acquire leases on additional paths; its original write set is not a universal immutable scope ceiling. The primary witness below consequently uses mission mode. For ordinary recurse the same objective mismatch exists, but a wider task, a later replan or independently allowed edits may change the outcome. Do not generalize the mission's hard boundary to every campaign. [Ordinary guard][write-guard]

### The stronger objective is enforced twice

`spawnTask` attaches the campaign check to the task context and calls `RunUntilVerified`. `campaignCheck` faithfully copies the full command, pass rule and environment. `run_check` refuses model-supplied arguments and records the actual command result. Policy requires a passing after-write observation for the offered check; a remaining sibling failure cannot satisfy that arm. Without another verification arm, it yields `/check_not_green`. The existing host-acceptance transaction is a separate verification arm, so this is not a claim that the check arm is the only possible way any turn could be verified. [Task handoff][task-check] [Governed tool][run-check] [Check policy][turn-check] [Check obligation][check-obligation] [Verification arms][verified] [Missing check evidence][missing-check]

Even if an initial task reaches phase completion through another admitted path, `settleAcceptance` runs the campaign's witness. `runAcceptanceRound` computes pass from the command exit and declared successful exits, without comparing the selected finding or baseline. A failure is a failed round. If budget remains, `appendAcceptanceRemediation` asks to make the same command pass using the campaign write-set union. Mission scope remains unchanged through remediation. [Settlement][settle] [Round judgment][acceptance-run] [Remediation][remediation]

By contrast, outer policy explicitly permits an already failing gate to remain failing without an increased finding count; it derives fix success from the target's resolution. The host computes that ordinary target from absence of the selected stable ID in the complete captured finding census, subject to unverified, no-tests and truncated-failure guards. Thus a two-to-one finding reduction can meet the outer objective while never satisfying the inner whole-command exit requirement. [Target calculation][target] [Ratchet policy][ratchet]

## 2. Conditional witness, not a reproduction

Use an otherwise authorized whole-workspace mission recurse run with two disjoint admitted source nodes, `a` and `b`. Neither depends on the other; neither is a collapsed component or the generic root node. A runnable, owner-declared workspace lint or audit command G checks both nodes and emits these small, parseable diagnostics while exiting 1:

```text
a/widget.js:3:1: forbidden unused local alpha
b/widget.js:3:1: forbidden unused local beta
```

These are stipulated output records for a hypothetical checker, not claimed output from a particular installed linter. G's documented success rule is exit 0 only. It reads each file independently; repairing `a/widget.js` cannot change whether `b/widget.js` violates its rule. Its executable, declaration, environment and oracle remain unchanged. Both files belong to real acquired nodes. [Declared workspace gates][detect] [Finding extraction][findings] [Attribution][measure]

Assume additionally:

1. The initial gate measurement completes without truncation; both findings are acquired and neither is refused or stalled.
2. Policy selects the finding in `a`. Choosing `b` first gives the symmetric case.
3. Mission design is admitted for `a`, giving scope `["a"]`; the checker and its protected inputs cannot be changed to remove the obligation.
4. A legitimate edit inside `a` removes only its defect. All independent permissions, test/evidence obligations, metrics and nonregression checks are satisfied or held constant. No external writer repairs `b` during this attempt.
5. G's after-output contains only the `b` finding and still exits 1.

The source-derived observations are:

| Boundary | Consequence under the stated premises |
|---|---|
| Selected work | The `a` target and its correct whole-workspace G invocation are handed to the attempt. |
| Mission scope | An edit to `b` lacks mission admission. |
| After-write `run_check`, when this arm is owed | The executed G remains failing, so this check cannot provide passing evidence for the turn. |
| Campaign acceptance, if phases reach completion | G produces another failed round despite the selected target being gone. |
| Remediation, if allowed by round budget | The same global passing-exit requirement is reissued; original node scope does not acquire `b`. |
| Independent outer comparison, if the retained repair reaches it | G is fail→fail with findings 2→1, and the selected target is absent. These facts can satisfy the outer target/nonregression predicates, subject to every other guard. |

The deterministic finding is the incompatible set of obligations. The source alone does **not** establish how many model turns occur, that a particular model repairs `a`, that every execution deadlocks, or that a correct change is always discarded.

### Rollback and recovery must be measured, not assumed

The task wrapper retains successful writes and otherwise considers partial-progress preservation before undo. Current `canPreservePartialFileModifyProgress` has explicit cases and typed-signal restrictions. In its `ErrTaskNotDone` branch, `/check_not_green` is not an allowed signal. A controlled failure containing that signal therefore does not qualify through that branch. Other outcome shapes and the separate verified-compile-progress branch need their own checks. If undo runs successfully, the legitimate `a` repair can be lost before the outer ratchet sees it; this is a conditional joined-path result, not an unconditional consequence of any acceptance failure. [Task transaction][transaction] [Partial-progress rules][partial] [Signal production][signals] [Undo][undo]

CLI and chat return orchestrator-owned kept writes to the recurse driver. The driver records executor errors but independently remeasures retained changes. This is a real escape path: a retained legitimate repair can still be kept even if its campaign reported a failure. It does not make the contradictory inner requirement coherent, nor eliminate the potential extra checks and remediation. [CLI adapter][cli] [Chat adapter][chat] [Outer execution boundary][outer-execute]

An external repair of `b`, an explicitly admitted larger work item, a checker whose actual domain is only `a`, or a target-specific witness can also avoid the conflict. None is guaranteed by the inspected handoff.

## 3. Nonduplication and counterevidence

The bounded screen inspected current guidance, mission architecture, the audit inventory and queue, relevant report bodies and targeted public code searches. It found no dedicated report of **a correctly preserved broad reporting command demanding a different mission node's pre-existing repair**. This is not a claim about all history or private discussion. [Queue][queue]

- **Gate-contract continuity** concerns lost environment and successful-exit semantics. This witness keeps them identical and uses ordinary zero-only success. Correct copying is a prerequisite, not the repair. [Prior contract audit][prior-contract]
- **Collapsed finding invocation identity** concerns giving B the command that actually ran for A. Here there is one correct G invocation for both findings and two separate nodes, not an aggregate carrying the wrong invocation. [Prior invocation audit][prior-invocation]
- **Finding-census completeness** treats campaign acceptance as an independent countercontrol and explicitly preserves incremental outer semantics. This witness requires a complete two-finding census and asks whether that countercontrol's success condition is attainable inside the selected mission scope. It does not dispute the census audit's absence-of-evidence finding. [Prior census audit][prior-census]
- **Campaign acceptance case identity** concerns path-case deduplication. The witness uses simple lowercase, disjoint paths. [Prior case audit][prior-case]
- **Portable recurse/deliberation crucible** already calls for typed behavioral acceptance and explains that outer nonregression is not universal green. This report supplies a concrete current producer/consumer contradiction, not a new overall agent architecture. [Prior portability proposal][portable]

Existing tests explicitly ensure a failing selected gate cannot be accepted and a repaired single-package gate can pass. Those controls remain necessary. The inspected fixture uses one package with one defect; it does not establish the behavior of a broader gate with independent baseline failures outside a mission's scope. No authored test was run. [Existing acceptance tests][tests]

## 4. Proposed contract and bounded choices

Before dispatch, distinguish three concepts: **the selected deficit**, **the full reporting command**, and **the admitted repair scope**. Their identities must not be collapsed into one argv.

For incremental recurse fixes, the specification should define a host-owned acceptance descriptor containing the selected finding or approved group, source-bound compatible baseline observation, reporting invocation identity, parser/capture coverage and nonregression obligations. Preserve environment and pass rules privately through the existing runtime contract. The same descriptor must reach the editing turn, campaign settlement and outer adoption decision.

Two defensible designs should be decided explicitly:

1. **Incremental target acceptance:** a compatible complete fresh observation proves the selected target resolved and satisfies the required nonregression predicate, while honestly recording that G still fails for other known work. A separately admitted target witness may establish resolution, but cannot replace the whole-domain nonregression evidence. This aligns with the existing outer policy.
2. **Atomic full-gate repair:** derive a larger work item that contains every required repair domain before design/execution, with explicit host admission and all existing permission/budget checks. If it cannot be authorized or bounded, report an unattainable current work item instead of dispatching repeated narrow attempts.

Do not silently switch between these meanings. An ordinary user-declared campaign whose requested result is a completely passing acceptance command must retain that requirement. The incremental interpretation belongs to an explicitly typed recurse contract, not a global weakening of `Acceptance`.

Host adapters should measure and bind observations; Mangle should decide scope, progress, continuation and acceptance. A model's statement that the remaining failures are unrelated is not evidence. Unknown/truncated/ambiguous output, changed check definitions, mismatched snapshots, no-tests results and incompatible before/after domains must remain incomplete or blocked. Preserve original failed exits instead of rewriting them as zero.

This contract also gives the proposer/interrogator a precise preflight question: “Can this admitted scope satisfy the selected acceptance objective?” A design-only answer can identify a mismatch, but cannot authorize a sibling write or make a broad failed check pass.

## 5. Failure modes, cost and unrun evaluation

An incremental implementation could incorrectly waive a newly introduced sibling failure, accept disappearance caused by parser loss or renamed identities, reuse stale baseline output, misclassify no-tests as resolution, or leak mutable invocation configuration across attempts. A broadening implementation could exceed mission scope or costs, recreate a monolithic whole-repository repair and reduce useful partial progress. Both require source/gate/domain identity and honest unknown states. Existing independent write, verification and ratchet controls remain necessary.

Reusing a complete baseline already acquired by recurse can avoid another baseline launch, but fresh after-observation and census comparison still cost tool time, CPU, memory and parsing. Retaining target/coverage metadata increases state and recovery complexity. Narrow witnesses can add an extra process. Broader atomic groups can increase context, model calls, verification time and rollback footprint. No speedup, saved-token figure or production prevalence was measured.

Proposed deterministic evaluations, **all unrun**:

- Two disjoint nodes, two honest baseline findings from one workspace gate; select each node in turn. Verify the target becomes resolved while the gate retains a failing exit for the other node.
- Carry the case through the normal mission builder, real scope policy, governed `run_check`, task continuation, campaign settlement and outer ratchet. Do not stop at comparing mocked counts.
- Record whether valid writes are retained or undone for `/check_not_green`, self-reported incompleteness, verified compile progress, explicit blocker, cancellation and exhausted campaign rounds. Preserve each existing outcome's ownership rules.
- One-node/single-defect control; node-scoped check; whole-workspace task whose scope really includes both nodes; grouped error-free witness; two independent gates rather than two findings in one gate.
- Real regression in `b` caused by the `a` edit; disappearing target with a new unrelated finding; opaque/truncated/unverified output; changed executable/argv/environment/exit rule; stale source; no tests; zero net change.
- Ordinary user-authored campaign with a whole-command pass requirement must still require that pass. No “incremental” exemption should leak into that route.
- CLI and chat adapters, mission and ordinary mode, optional error-free gates enabled and disabled. Both normal entry paths must convey the same objective.

Use an independently retained oracle for both defects, matched source and checker versions, identical permissions and budgets, and the same scripted repair. Compare current behavior, target-acceptance design and explicitly broader atomic work. Measure correct repairs retained, unrelated obligations conserved, false acceptance, blocked/rolled-back correct work, turn/check/remediation counts, model tokens, host time and total cost per verified change. Charge census maintenance, fresh checks and any broader verification to the proposed design. A lower cost obtained by skipping G or allowing unauthorized writes is failure, not improvement.

**Exit criterion:** a selected mission repair must never be required to make an unchanged independent out-of-scope baseline defect disappear merely because both were reported by one command. The system must either admit a compatible incremental proof, admit the larger repair explicitly, or identify the scope/objective conflict before repeated execution. It must still retain and report the unresolved sibling obligation.

## Source references

[guidance]: https://github.com/theRebelliousNerd/codenerd/blob/45a51a3b66232a42e018b3fa070370286d22f851/agents.md#L3-L36
[interrogator]: https://github.com/theRebelliousNerd/codenerd/blob/45a51a3b66232a42e018b3fa070370286d22f851/.agents/skills/requirements-interrogator/SKILL.md
[mission-spec]: https://github.com/theRebelliousNerd/codenerd/blob/45a51a3b66232a42e018b3fa070370286d22f851/Docs/architecture/campaign/MISSION-IMPROVEMENT.md#L198-L231
[ratchet]: https://github.com/theRebelliousNerd/codenerd/blob/45a51a3b66232a42e018b3fa070370286d22f851/internal/core/defaults/policy/recurse.mg#L258-L327
[attempt-contract]: https://github.com/theRebelliousNerd/codenerd/blob/45a51a3b66232a42e018b3fa070370286d22f851/internal/campaign/recurse_attempt.go#L34-L54
[measure]: https://github.com/theRebelliousNerd/codenerd/blob/45a51a3b66232a42e018b3fa070370286d22f851/internal/campaign/recurse_cycle.go#L759-L840
[open]: https://github.com/theRebelliousNerd/codenerd/blob/45a51a3b66232a42e018b3fa070370286d22f851/internal/campaign/recurse_cycle.go#L1010-L1032
[attempt]: https://github.com/theRebelliousNerd/codenerd/blob/45a51a3b66232a42e018b3fa070370286d22f851/internal/campaign/recurse_cycle.go#L1152-L1179
[campaign]: https://github.com/theRebelliousNerd/codenerd/blob/45a51a3b66232a42e018b3fa070370286d22f851/internal/campaign/recurse_attempt.go#L55-L141
[acceptance-type]: https://github.com/theRebelliousNerd/codenerd/blob/45a51a3b66232a42e018b3fa070370286d22f851/internal/campaign/orchestrator_acceptance.go#L21-L48
[mission-prepare]: https://github.com/theRebelliousNerd/codenerd/blob/45a51a3b66232a42e018b3fa070370286d22f851/internal/campaign/recurse_mission.go#L174-L256
[mission-write]: https://github.com/theRebelliousNerd/codenerd/blob/45a51a3b66232a42e018b3fa070370286d22f851/internal/campaign/recurse_mission_scope.go#L13-L75
[mission-outer]: https://github.com/theRebelliousNerd/codenerd/blob/45a51a3b66232a42e018b3fa070370286d22f851/internal/campaign/recurse_cycle.go#L1361-L1399
[write-guard]: https://github.com/theRebelliousNerd/codenerd/blob/45a51a3b66232a42e018b3fa070370286d22f851/internal/campaign/orchestrator_attempt_writes.go#L138-L157
[task-check]: https://github.com/theRebelliousNerd/codenerd/blob/45a51a3b66232a42e018b3fa070370286d22f851/internal/campaign/orchestrator_task_handlers.go#L64-L148
[run-check]: https://github.com/theRebelliousNerd/codenerd/blob/45a51a3b66232a42e018b3fa070370286d22f851/internal/tools/shell/run_check.go#L48-L159
[turn-check]: https://github.com/theRebelliousNerd/codenerd/blob/45a51a3b66232a42e018b3fa070370286d22f851/internal/core/defaults/policy/coder_safety.mg#L538-L604
[check-obligation]: https://github.com/theRebelliousNerd/codenerd/blob/45a51a3b66232a42e018b3fa070370286d22f851/internal/core/defaults/policy/coder_safety.mg#L678-L687
[verified]: https://github.com/theRebelliousNerd/codenerd/blob/45a51a3b66232a42e018b3fa070370286d22f851/internal/core/defaults/policy/coder_safety.mg#L778-L800
[missing-check]: https://github.com/theRebelliousNerd/codenerd/blob/45a51a3b66232a42e018b3fa070370286d22f851/internal/core/defaults/policy/coder_safety.mg#L806-L827
[settle]: https://github.com/theRebelliousNerd/codenerd/blob/45a51a3b66232a42e018b3fa070370286d22f851/internal/campaign/orchestrator_acceptance.go#L97-L136
[acceptance-run]: https://github.com/theRebelliousNerd/codenerd/blob/45a51a3b66232a42e018b3fa070370286d22f851/internal/campaign/orchestrator_acceptance.go#L156-L225
[remediation]: https://github.com/theRebelliousNerd/codenerd/blob/45a51a3b66232a42e018b3fa070370286d22f851/internal/campaign/orchestrator_acceptance.go#L228-L301
[target]: https://github.com/theRebelliousNerd/codenerd/blob/45a51a3b66232a42e018b3fa070370286d22f851/internal/campaign/recurse_cycle.go#L1421-L1463
[detect]: https://github.com/theRebelliousNerd/codenerd/blob/45a51a3b66232a42e018b3fa070370286d22f851/internal/gates/detect.go#L15-L92
[findings]: https://github.com/theRebelliousNerd/codenerd/blob/45a51a3b66232a42e018b3fa070370286d22f851/internal/gates/findings.go#L116-L187
[transaction]: https://github.com/theRebelliousNerd/codenerd/blob/45a51a3b66232a42e018b3fa070370286d22f851/internal/campaign/orchestrator_task_transaction.go#L55-L121
[partial]: https://github.com/theRebelliousNerd/codenerd/blob/45a51a3b66232a42e018b3fa070370286d22f851/internal/campaign/orchestrator_task_transaction.go#L123-L194
[signals]: https://github.com/theRebelliousNerd/codenerd/blob/45a51a3b66232a42e018b3fa070370286d22f851/internal/campaign/orchestrator_task_handlers.go#L150-L174
[undo]: https://github.com/theRebelliousNerd/codenerd/blob/45a51a3b66232a42e018b3fa070370286d22f851/internal/campaign/orchestrator_attempt_writes.go#L159-L185
[cli]: https://github.com/theRebelliousNerd/codenerd/blob/45a51a3b66232a42e018b3fa070370286d22f851/cmd/nerd/cmd_campaign_recurse.go#L225-L265
[chat]: https://github.com/theRebelliousNerd/codenerd/blob/45a51a3b66232a42e018b3fa070370286d22f851/cmd/nerd/chat/campaign_recurse.go#L218-L265
[outer-execute]: https://github.com/theRebelliousNerd/codenerd/blob/45a51a3b66232a42e018b3fa070370286d22f851/internal/campaign/recurse_cycle.go#L1295-L1354
[queue]: https://github.com/theRebelliousNerd/codenerd/blob/45a51a3b66232a42e018b3fa070370286d22f851/Docs/audits/ASTRA_QUEUE.md
[prior-contract]: https://github.com/theRebelliousNerd/codenerd/blob/45a51a3b66232a42e018b3fa070370286d22f851/Docs/audits/recurse-gate-contract-continuity.md
[prior-invocation]: https://github.com/theRebelliousNerd/codenerd/blob/45a51a3b66232a42e018b3fa070370286d22f851/Docs/audits/recurse-collapsed-finding-invocation-identity.md
[prior-census]: https://github.com/theRebelliousNerd/codenerd/blob/45a51a3b66232a42e018b3fa070370286d22f851/Docs/audits/recurse-finding-census-completeness.md
[prior-case]: https://github.com/theRebelliousNerd/codenerd/blob/45a51a3b66232a42e018b3fa070370286d22f851/Docs/audits/campaign-acceptance-case-identity.md
[portable]: https://github.com/theRebelliousNerd/codenerd/blob/45a51a3b66232a42e018b3fa070370286d22f851/Docs/audits/portable-recurse-deliberation-crucible.md
[tests]: https://github.com/theRebelliousNerd/codenerd/blob/45a51a3b66232a42e018b3fa070370286d22f851/internal/campaign/recurse_attempt_acceptance_test.go#L83-L177
