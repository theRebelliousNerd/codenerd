---
doc-class: research-proposal
subsystem: campaign-recurse-context
implementation-status: proposed
last-verified: 2026-10-30
verified-against: 388205138ba6d47c1cff0c5e97c896dfa642a391
verification-method: static-source-inspection
---

# Recurse prior attempt reasons escape the history byte budget

## Decision

Bound the entire rendered prior-attempt section, not just its patch strings. `writePriorAttempts` promises newest-first history within `recurseEvidenceLimit`, but it charges only `PriorAttempt.Tried`. It writes every reason and cycle header without charging them. When prior attempts have empty patches, its 12,000-byte balance never decreases, so every retained reason is copied into the next task. This is reachable through ordinary no-net-change improvement attempts, not only through a synthetic oversized string. [Renderer][renderer], [patch producer][diff], [retention][retention].

The existing session request guard substantially limits the consequence. It counts the assembled request and refuses an irreducible oversized task locally; this is **not** a way to send unlimited context to a provider. Before that boundary, the growing history occupies input that could serve current work. At the boundary, historical evidence embedded in the mandatory task can make the next request locally inadmissible. Fix the upstream history accounting while preserving that downstream refusal and the useful reasons for earlier reversions. [Working request admission][admission], [input window][window].

This is a static finding and an unimplemented proposal. No CodeNERD application, tests, benchmark, compiler, model inference or reproduction was run. Formulas below describe logical rendered bytes and repeated presentations, not measured tokens, allocations, RSS, latency, bills, production incidence or quality improvements.

## Source freeze and prior work

Read-only connector inspection resolved and rechecked main on 2026-10-30 at [`388205138ba6d47c1cff0c5e97c896dfa642a391`][main]. Guidance read included the root [agent contract][guidance] and [campaign contract][campaign-guidance]. The prior-audit inventory was reviewed through [`research-main-20261030-1619`, `3ef34bc061b497b9cfc471f4cc46f5df595b299d`][research], including the new acceptance-case identity, rollback-permission mode, compressed-control outcome and generated-learning event-domain reports. None of those mechanisms is the history renderer addressed here.

This is a residual in an existing capability, not a proposal to invent recurse memory. The historical **Q-17 recurse reflexion memory** initiative explicitly called for carrying reverted approaches and reasons into subsequent attempts, retaining the finding stall rule, and keeping the note bounded. Current code implements that continuity. Its incomplete accounting is the remaining issue. Q-17 also identifies the real quality tradeoff: prior approaches can prevent repeated mistakes but can anchor later work. [Q-17][q17].

The [native keyword allocation audit][byte-prior] already establishes the general principle that item counts or partial payload budgets do not bound all materialized evidence bytes. This report applies that principle to a different live producer and consumer: reason-only recurse journal entries become campaign task text and then mandatory worker input. It is not another scanner allocation, lexical window, context seed scheduling, gate contract or rollback-ownership report. The existing [campaign seed ownership][seed-prior], [recurse gate continuity][gate-prior], and [gate-scoped current-main addendum][gate-addendum] remain relevant controls, not defects being reasserted here.

The `writePriorAttempts` function was also compared with the five inspected non-research branch heads below and was byte-identical in each. This is a narrow function comparison, not a claim that the branches' entire files, policies or runtimes are identical:

| Branch | Inspected head |
|---|---|
| `jules-orchestrator-acceptance-qa-12283394416863483039` | `51cfd42649552866d4aa54c53b68acf5740eb09b` |
| `lane/worktree-agent-a70bb621a19639eb0` | `a53cfc308e8836c72370adaa46deba3f7b00180d` |
| `lane/worktree-agent-a893b7e3344915557` | `80af1863d3a3aa46a722ac8b0c8a0985582d4995` |
| `lane/worktree-agent-acd8bb412e996d54c` | `aa7988dec4c62116656ea722ff73b687a8509908` |
| `qa-cartographer-boundary-analysis-16707629379015549107` | `767a3291d6287b238417f2466592a9002810d05a` |

## Contract and accounting discrepancy

The local contract is explicit: `writePriorAttempts` records what earlier attempts tried and why they were reverted, newest first, within `recurseEvidenceLimit`. That constant is 12,000. The implementation does the following for each prior record while `budget > 0`:

1. Append the entire cycle header and `p.Why`.
2. Clamp `p.Tried` if its length exceeds the remaining budget.
3. Subtract only the resulting `len(tried)`.
4. If the patch has non-whitespace content, append its label and Markdown fences, also without charging them.

Thus even nonempty patches can exceed the stated section budget by their reasons and wrappers. Empty patches are the cleaner witness because the loop makes no budget progress at all. A single long reason can also exceed the limit; no such long reason is needed to establish the production recurrence below. [Constant][limit], [complete renderer][renderer].

For `n` prior records with `Tried == ""`, define:

```text
h_i = byte length of the formatted cycle header, Why_i, and terminating newline
H_n = sum(h_i, i = 1..n)
```

The renderer emits exactly `H_n` bytes for this prior-attempt section, regardless of the nominal 12,000-byte balance. Whenever `H_n > 12000`, the section violates its local bound. This is not a claim that the whole task was intended to fit in 12,000 bytes: current metrics, the task identity, the optional north star, and other campaign context have separate ownership. The proposed assertion should measure the history section, not erroneously apply its limit to the entire task.

## Real producer and recurrence

### A no-change improvement creates a reason-only prior record

`run` passes the executor an attempt whose `Prior` slice comes from the work key. For improvement work, that key is `improve:<node>:<angle>`. After the attempt, the loop observes actual changed paths. The ratchet requires a change to keep an attempt; a no-change result is reverted rather than accepted as an improvement. On the revert path, `attemptDiff` immediately returns an empty string when its changed-path list is empty. The loop still constructs `Why` from the outcome, available metrics and optional details. It is nonempty even without an executor error. [Work key and reason construction][retention], [attempt construction][improve-run], [ratchet][ratchet], [patch producer][diff], [settlement][settlement].

`rememberRevert` skips a record only when **both** its patch and reason are empty. A normal reason-only record therefore remains in the relevant prior slice. Successful journal settlement precedes this in-memory retention. Resume reconstructs the same non-kept priors from journal records, so a restart is not a history-budget reset. [Retention][retention], [settlement][settlement], [resume][resume].

This does not require a `/file_modify` task to be falsely accepted with no edits. An inner campaign may correctly fail its modification or verification obligation and undo its work. If the outer recurse attempt settles with no net changed paths, successful cleanup and a recorded reason, the outer history entry has precisely the shape that defeats the renderer's debit. Actual observed changes and cleanup outcomes remain authoritative.

### The finding stall rule does not eliminate improvement recurrence

There is an important existing protection: the policy stops retrying a finding after its last two failures have the same signature and no intervening kept node change. Refusals also remove findings from candidacy. This protects the ordinary repeated-finding case. [Finding selection and stall][stall].

Improvement work is a separate route. Each eligible non-cross-cutting node gets the pass's measurable improvement angle; the default angles rotate through stabilize, harden, simplify and extend. `improve` does not consult a finding stall, and failed improvement settlement does not add a finding attempt. The same node and angle can therefore recur in later passes, with its full accumulated reason-only prior slice. A node with no relevant measurable metric is skipped, so measurability and continuing eligibility are explicit prerequisites. [Improvement policy][angles], [driver][improve-run], [finding versus improvement recording][settlement].

The ordinary CLI supports a finite pass count or `--waves 0` until stopped. The driver also checks cancellation, explicit stop requests and mission admission. The witness needs enough authorized passes; it does not disable any of those controls. Bounded missions can terminate well before a large history develops, so an unbounded-mission claim would be wrong. [CLI flags][cli-flags], [boundary and pass loop][passes].

### Conditional production witness

Use ordinary, non-mission recurse on an admitted workspace with a real non-cross-cutting node and a repeatedly measurable angle. Keep the node identity and work key stable. Let successive occurrences of that angle settle with no net changed paths and nonempty reasons; cleanup and journal operations succeed, and intervening work does not remove the node or make its angle ineligible. Continue for enough authorized passes that the sum of that key's reason headers exceeds 12,000 bytes.

The next `RecurseAttempt` contains those reason-only records, and the renderer includes all of them. The byte-bound violation is deterministic given those conditions. How often a real model or repository produces the conditions, how many cycles are affordable, and whether the overall request is still admitted are unmeasured. A targeted test should drive the real policy with a deterministic executor rather than spend hundreds of live model attempts merely to manufacture history.

## Production consumer and the downstream guard

The normal CLI callback turns each `RecurseAttempt` into a one-task campaign and runs that plan through a new orchestrator. `RecurseAttemptCampaign` chooses the fix or improve task renderer and installs its output as the `/file_modify` task description. The improvement renderer appends the prior section after current metrics and, for extend, the north star. [CLI execution][cli-execute], [campaign construction][campaign-build], [history render calls][renderer].

`buildTaskInput` uses the task description when no explicit shard input exists, then appends campaign context; it does not reduce the description's historical subsection. The file handler forwards that input to `spawnTask`, which retains the normal write guard, observed executor and `RunUntilVerified` contract. `TaskRequest.TaskText` preserves the task text, apart from trimming and optional routing additions. Both its inline and isolated routes carry that text onward. [Input construction][task-input], [file handler][file-handler], [observed execution][spawn], [task text][task-text], [task routing][task-route], [isolated request][task-isolated].

This is why the final request budget is a mitigation rather than a repair. The production tool loop opens a working set with the task as its anchor and records the serving client's input cap. The request preparation path counts assembled messages, allows the ledger to compact, and archives large tool results if needed. It does **not** discard arbitrary portions of the task. Once there is no tool result left to archive, an oversized request returns `ErrInputBudgetExceeded`. The single-shot path also checks the request. The effective window is lowered to the serving slot's cap when smaller than the session budget. [Loop entry][loop-entry], [task anchor][anchor], [request admission][admission], [window and single-shot guard][window].

Consequently:

- Below the actual input limit, the same old reasons can take an increasing share of the worker request.
- Above that limit, the guard can refuse the request locally rather than expose unlimited provider context.
- Refusal does not undo the upstream history rendering or make the history optional again. A request that fails admission may already have incurred orchestration, retrieval, prompt construction and other preparatory work; this report does not assert that every auxiliary call is absent before refusal.
- This is not false verification, unsafe acceptance, loss of write scope, or a permission bypass. The normal observed verdict, ratchet, compensation and scope controls remain in place.

## Existing tests and alternative interpretations

The inspected task-render tests are useful. The fix fixture checks short priors and newest-first order; the improvement fixture supplies an over-limit patch and expects a truncation marker, along with correct metrics and phase-specific north-star behavior. They exercise patch truncation, not total rendered history bytes or a long series of empty-patch reasons. They were read, not run. No claim is made that the entire repository lacks any other history fixture. [Render tests][tests].

One possible interpretation is that 12,000 was intended only as a patch budget. The constant's original comment concerns gate evidence, and a patch quota can be defensible. But `writePriorAttempts` expressly promises the combined tried-and-why section within that limit; there is no separate reason quota in the function. Even if the comment were revised to make patch-only budgeting intentional, the repeated reason-only prompt growth would still need an explicit design decision and a separately enforced history budget.

Another interpretation is that all repeated failures stall after two attempts. That is true only for the particular finding-policy conditions described above, not this independent improvement path. A short pass limit, cancellation, loss of measurability, failed cleanup or journal error can prevent the witness; they are legitimate stopping controls, not evidence that the accounting is complete.

Finally, the retained reasons are useful context, not necessarily waste. Reducing them indiscriminately could cause the worker to repeat a failed approach. The repair must preserve enough causal evidence and evaluate quality alongside resource savings.

## Proposed repair

1. **Define one complete history-section budget.** Charge cycle identifiers, reason text, patch text, labels, fences, separators and omission markers. Use a remaining-byte writer before materializing/appending each component, rather than constructing an unbounded section and truncating it afterward. Handle multibyte text and markers without invalid strings or silent cuts.
2. **Preserve the most useful recent rejection.** Keep newest-first semantics and retain a meaningful reason even when its patch is empty. Give reasons and patches explicit allocations within the total, or use a policy-selected bounded projection. An empty patch must not imply free unlimited reasons; it also must not cause the reason to disappear entirely.
3. **Keep the durable evidence.** The journal can retain the complete record while the prompt carries a bounded projection. Include an honest omission count or cycle range. If older history is to become model-retrievable, implement an actual scoped, revision-aware read path and handle contract; a printed path or cycle number is not automatically a usable retrieval capability.
4. **Keep optional history out of mandatory task identity where practical.** A broader design could represent prior attempts as typed historical evidence eligible for working-context selection, while the current obligation remains intact. That must preserve provenance and admission semantics; the narrow accounting repair does not require this larger refactor or an additional model call.
5. **Retain all safety and progress controls.** Do not weaken the input-window refusal, write guard, observed verdict, ratchet, compensation, finding stall, explicit pass budget, mission limits or cancellation in order to make a history-heavy task proceed. Follow spec-first updates for the touched package contract.

The bounded prompt projection does not itself bound total journal size or the cost of replaying complete journal history on resume. Those are separate lifecycle questions and are not claimed fixed by this change.

## Costs and expected benefit

For a continuing sequence of `n` occurrences of the same improvement key, with one new reason-only record per completed occurrence, record `i` appears in each later task. The number of prior-record presentations across those task constructions is `n(n-1)/2`; byte volume is `sum((n-i) * h_i, i=1..n)`. This arithmetic concerns one task construction per occurrence and only the history section. Inner task retries or repeated worker requests can add presentations, while stops, eligibility changes and local input refusal limit the realized provider traffic.

A bounded renderer reduces the history bytes it assembles and offers per task. That does not establish a specific allocation, RSS or wall-time reduction: other context, copying behavior, journal reads, tokenization, prompt caching, model serving and gate execution remain. Bytes are not tokens, and cached input is not the same as uncached billed input. No dollar estimate is justified from this source trace alone.

The narrow repair can be deterministic and require no new model calls. Its costs include budget bookkeeping, boundary/UTF-8 handling, omission metadata and tests. A retrieval-backed design adds scoped storage access, context/tool tokens and potentially extra turns when older history matters. Summarization by a model would add inference and correctness risk and should not be assumed necessary. The most important possible regression is losing an old reason that prevents a repeated bad patch; successful evaluation must account for that, not optimize prompt size alone.

## Fair evaluation plan not yet run

### Deterministic contract checks

- Measure only the prior section and assert its complete rendered byte length is within the chosen bound, including markers and wrappers.
- Cover no priors, a short reason and patch, many empty patches with ordinary reasons, a single oversized reason, mixtures of empty and nonempty patches, exact-boundary input, multibyte text, and a newest record that cannot fit in full.
- Check useful newest reason retention, cycle identity, honest omission signaling, valid text and stable newest-first order. Preserve current metrics and north-star behavior outside this section.

### Real policy and persistence integration

- Build a temporary workspace with an actual admitted node and measurable angle. Drive `RunRecurseCycles` with the real policy and a deterministic no-net-change executor; do not manufacture only a helper's slice and call that production coverage.
- Confirm reason-only settled records arise, that the same node-angle recurs on its rotation, and that each next task respects the section budget after the repair. Close and reopen the journal and verify the bound still holds after resume.
- Include controls for finite passes, cancellation/stop, mission admission, no measurable angle, cross-cutting nodes, kept changes, and the ordinary finding-stall rule. Cleanup or persistence failure must continue to stop safely rather than be hidden by the projection.

### Consumer and quality checks

- Exercise normal campaign task construction and the observed worker route with world/CodeDOM context enabled. Capture assembled task and request sizes without needing live inference for the accounting assertions.
- Check both below-window admission and above-window local refusal for message-channel and single-shot clients. The repair must not relax mandatory-task admission or the serving slot cap.
- In a separately authorized paired live evaluation, compare fixed workspace states, requests, model/slot, policy, budgets and retrieval settings. Include cases where an older rejection is genuinely needed, not just repeated identical reasons.
- Report completed useful improvements, repeated rejected approaches, kept/reverted outcomes, local budget refusals, total attempts, model calls, input/output and cached/uncached tokens, retrieval calls, gate work, journal I/O and resume cost, wall time and measured memory. Attribute any benefit to the entire run, not only a shorter prompt fragment. Preserve output quality and all existing safety checks as acceptance criteria.

## Bottom line

The implementation retains and replays useful recurse failure memory, but its history limit charges only patches. A legitimate series of no-net-change improvement attempts therefore defeats that local bound. Repair the complete evidence projection upstream; keep the downstream request refusal and the authoritative work-verification machinery unchanged.

[main]: https://github.com/theRebelliousNerd/codenerd/tree/388205138ba6d47c1cff0c5e97c896dfa642a391
[research]: https://github.com/theRebelliousNerd/codenerd/tree/3ef34bc061b497b9cfc471f4cc46f5df595b299d/Docs/audits
[guidance]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/agents.md
[campaign-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/agents.md
[limit]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/recurse_attempt.go#L17-L20
[renderer]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/recurse_attempt.go#L130-L205
[campaign-build]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/recurse_attempt.go#L54-L134
[diff]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/recurse_git.go#L149-L179
[retention]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/recurse_cycle.go#L256-L294
[improve-run]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/recurse_cycle.go#L944-L1003
[settlement]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/recurse_cycle.go#L1239-L1289
[resume]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/recurse_cycle.go#L1530-L1563
[ratchet]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/defaults/policy/recurse.mg#L301-L329
[stall]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/defaults/policy/recurse.mg#L188-L245
[angles]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/defaults/policy/recurse.mg#L331-L370
[passes]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/recurse_cycle.go#L398-L454
[cli-flags]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/cmd_campaign_recurse.go#L81-L91
[cli-execute]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/cmd_campaign_recurse.go#L222-L260
[task-input]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_task_results.go#L164-L194
[file-handler]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_task_handlers.go#L578-L603
[spawn]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/orchestrator_task_handlers.go#L31-L112
[task-text]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/session/task_executor.go#L44-L66
[task-route]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/session/task_executor.go#L375-L449
[task-isolated]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/session/task_executor.go#L521-L545
[loop-entry]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/session/executor_tools.go#L84-L100
[anchor]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/session/working_context.go#L130-L215
[admission]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/session/working_context.go#L549-L611
[window]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/session/working_context.go#L650-L709
[tests]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/campaign/recurse_attempt_task_test.go#L17-L139
[q17]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/journeys/11-idea-corpus-2026-09-26/quality.md#L774-L805
[byte-prior]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/native-keyword-context-allocation-and-byte-budgets.md
[seed-prior]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/campaign-context-seed-scheduler-ownership.md
[gate-prior]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/recurse-gate-contract-continuity.md
[gate-addendum]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/gate-scoped-recurse-current-main-addendum.md
