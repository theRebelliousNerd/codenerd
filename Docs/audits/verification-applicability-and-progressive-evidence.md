---
doc-class: deep-dive
subsystem: verification
implementation-status: built (see As-built status)
last-verified: 2026-10-28
verified-against: 356f65e0e6bb639adaa90ab36212daa714114871
verification-method: static-source-inspection
supersedes: []
---

# Verification applicability and progressive evidence

**Decision:** preserve the implemented judge contract and kernel-owned delegation loop. Close the remaining gap between evidence that was valid when a worker finished and evidence that can support the result delivered now. Give every judgment a host-measured applicability disposition, make evidence acquisition disclose new ranges rather than repeat prefixes, and account for the complete serialized request rather than only selected bodies.

The expected capability is straightforward: a delegated change should not be presented as currently accepted because a judge passed an earlier revision, and a judge asking to see omitted code should actually be able to reach it. These are refinements to the live verification path, not a proposal for a new verifier, unrestricted judge tools, or an LLM executive.

**Source freeze:** branch heads were resolved again at approximately 10:12 UTC on 2026-10-28. Main was [`356f65e0e6bb639adaa90ab36212daa714114871`](https://github.com/theRebelliousNerd/codenerd/commit/356f65e0e6bb639adaa90ab36212daa714114871), feature `feat/conversation-brief` was [`c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c`](https://github.com/theRebelliousNerd/codenerd/commit/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c), and research branch `codenerd-research-docs` was [`66837f868a149c0d035942ba11a40cb2b4d4f3b7`](https://github.com/theRebelliousNerd/codenerd/commit/66837f868a149c0d035942ba11a40cb2b4d4f3b7). The inspected `internal/verification/evidence_packet.go`, `judge_contract.go`, and `verifier.go` have identical blob SHAs on main and the feature branch. This report is frozen to those revisions.

**Verification limit:** source inspection only. No codeNERD runtime, tests, inference, benchmark, failure injection, implementation, or GitHub write was performed. The schedules below are source-derived witnesses to validate, not reproduced incidents or measured defect frequencies. The only new local output is this report.

## 1 Reconcile the existing audit before adding work

The [verification evidence and judge contracts audit on main](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/Docs/audits/verification-evidence-and-judge-contracts.md#L192-L209) includes an **As built 2026-10-27** appendix. The [research-branch copy](https://github.com/theRebelliousNerd/codenerd/blob/66837f868a149c0d035942ba11a40cb2b4d4f3b7/Docs/audits/verification-evidence-and-judge-contracts.md) inspected here lacks that appendix and still describes the original proposal as planned. Reading only that copy would incorrectly turn completed work back into a backlog.

Preserve the original proposal as historical rationale, retain main's implementation reconciliation, and add the narrower residual findings below. In particular, these capabilities are implemented source paths, not missing work:

| Capability | Current source evidence |
|---|---|
| Semantic reply validation including required fields, duplicate keys, known statuses, bounded replies and contradictory combinations | [`judge_contract.go`](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/verification/judge_contract.go#L96-L197) |
| Typed host evidence packet with labeled narrative, check observations, changed-file content and closing snapshot | [`evidence_packet.go`](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/verification/evidence_packet.go#L151-L245) |
| Judge only a kernel-done attempt and preserve separate kernel disposition and judge judgment | [`verifier.go`](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/verification/verifier.go#L79-L96), [`delegation.mg`](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/defaults/policy/delegation.mg#L354-L368) |
| Policy-mediated additional evidence rounds without rerunning the worker for a judge limitation | [`verifier.go`](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/verification/verifier.go#L321-L359), [`delegation.mg`](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/defaults/policy/delegation.mg#L385-L445) |
| Re-measure final-edit mechanical evidence and verify the acceptance transaction before closing the worker | [`change_evidence.go`](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/change_evidence.go#L70-L118) |

Implemented here means an inspected source path exists; it does not assert that this pass executed it successfully.

### Narrative as artifact is an intentional exception

Main's appendix explicitly records narrative-only grounding under **Ruled differently**. For the review rubric, the narrative is the artifact being evaluated. The [review atom](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/prompt/atoms/eval/delegation_judge.yaml#L49-L81) makes that purpose explicit. A blanket prohibition on reported evidence would damage that capability. This report does not classify narrative-only grounding as an overlooked implementation bug.

The distinction still matters conceptually: observing that a review contains a statement is not proving that statement about the repository true. Any future claim-specific evidence refinement should preserve that distinction, but it is not a prerequisite for the three bounded findings here.

### Scope relative to neighboring audits

This report concerns a normally returned, kernel-done worker attempt and its subsequent adjudication. It does not duplicate [failed delegated turn settlement](https://github.com/theRebelliousNerd/codenerd/blob/66837f868a149c0d035942ba11a40cb2b4d4f3b7/Docs/audits/delegated-turn-failure-settlement.md), [package-qualified test evidence](https://github.com/theRebelliousNerd/codenerd/blob/66837f868a149c0d035942ba11a40cb2b4d4f3b7/Docs/audits/test-evidence-capability.md), or [background outcome and continuation ownership](https://github.com/theRebelliousNerd/codenerd/blob/66837f868a149c0d035942ba11a40cb2b4d4f3b7/Docs/audits/background-outcome-and-continuation-ownership.md). Their lifecycle and attribution work remains independently valuable.

## 2 Follow the production request through acceptance

The ordinary TUI delegation path requires the verification obligation for a mutation and refuses to fall through to an ordinary response when the verifier is unavailable. It passes the task, persona, retry limit, session context and priority into `VerifyWithRetry`. It then checks `verdict.Accepted` before selecting `formatVerifiedResponse`. This is not an audit of an unused helper. See [main process.go](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/cmd/nerd/chat/process.go#L455-L528) and the corresponding [feature path](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/process.go#L520-L591).

Each attempt is executed through `ObservedTaskExecutor`. The return carries the kernel outcome, and the verifier asserts that outcome before asking whether a judge is due. [`VerifyWithRetry`](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/verification/verifier.go#L224-L288).

The session's observation adapter carries the closing snapshot, choosing a verified acceptance report's after-snapshot when present. The live task executor also attaches written-file preimages and the contents read after execution; that attachment is not absent just because it occurs outside `observedReturn`. See [`observed_return.go`](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/observed_return.go#L47-L84), [`task_executor.go`](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/task_executor.go#L388-L403), and [`turn_writes.go`](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/turn_writes.go#L19-L42).

The judge receives a packet built from those observations. Its parsed reply is grounded against packet items, published as a judgment fact, and consumed by Mangle's delegation rules. The accepted result finally renders a green **Accepted** label, preserving the judge status and confidence separately. [`judge`](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/verification/verifier.go#L328-L359), [`formatVerifiedResponse`](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/cmd/nerd/chat/helpers.go#L329-L373).

## 3 V1 Applicability is checked for cited defects but not accepted passes

`EvidencePacket.usable` checks the current file identity when an item is file-backed. A non-file item returns usable immediately. The packet snapshot is displayed in the prompt but is not itself validated by that method. [`evidence_packet.go` lines 77 to 96](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/verification/evidence_packet.go#L77-L96).

`Judgment.ground` visits citations and requested items. A pass has neither: the reply contract rejects citations on a pass. Consequently, a valid pass reaches judgment publication without invoking a file freshness check. A defect with at least one usable citation can remain a defect even when other citations are stale, which is already an explicit behavior in the grounding tests. [`judge_contract.go` lines 170 to 235](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/verification/judge_contract.go#L170-L235), [`judge_contract_test.go` lines 73 to 105](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/verification/judge_contract_test.go#L73-L105).

The executive acceptance rule joins the recorded `/done` attempt with absence of judge rejection and unresolved evidence. It contains no separate applicability requirement. The Go `/accept` branch then returns `Accepted=true` without checking the packet snapshot. [`delegation.mg` lines 406 to 423](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/defaults/policy/delegation.mg#L406-L423), [`verifier.go` lines 290 to 314](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/verification/verifier.go#L290-L314).

### Source derived schedule

1. The worker closes its gates and acceptance at revision S and returns `/done`.
2. The host freezes a packet describing that attempt.
3. While the judge request is in flight, an external writer changes a file relevant to the verified behavior.
4. The judge returns a contract-valid pass based on the old packet.
5. Grounding checks no cited file because the pass has no citations.
6. The existing acceptance rule can derive `/accept`, and the TUI displays **Accepted** for a workspace no longer represented by S.

This is not a claim that the original evidence was fabricated or that the original worker outcome was wrong. It is a claim about the absent status-independent applicability check after a potentially long judge call. The same conceptual issue affects low-confidence objections that policy accepts and unreferenced dependencies that change while an otherwise current citation remains available.

The existing [stale citation test](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/verification/verifier_delegation_test.go#L275-L300) already models a writer replacing a file during the judge call. It returns a defect citing that file and verifies downgrade to insufficient evidence. It does not cover a pass over the same schedule.

## 4 V2 Acquisition can repeat a prefix without reaching omitted evidence

`expand` takes `cutUTF8(it.full, budget)` from the start of each requested file's retained post-change contents. It has no range, offset, cursor, or record of what was already disclosed. `expandable` continues to examine the original omitted count and retained content. The verifier appends an expansion to the previous user prompt before asking again. [`evidence_packet.go` lines 92 to 118](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/verification/evidence_packet.go#L92-L118), [`verifier.go` lines 337 to 359](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/verification/verifier.go#L337-L359).

Consider a changed file whose decisive function lies beyond the first B bytes of post-change content, with the initial bounded diff also omitting that function. The judge requests the file item. Expansion returns the first B bytes, still without the decisive function. Requesting the item again can return the same prefix. The host may spend another permitted round without increasing evidence coverage, eventually declaring the attempt unverifiable.

There is an existing round limit, so this is not a claim of an infinite loop. The gap is failure to make progress within those rounds. The [existing acquisition test](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/verification/verifier_delegation_test.go#L227-L273) deliberately selects an expansion budget large enough to fit the full file. It demonstrates useful acquisition, but not access to content beyond an expansion boundary.

The result is truthful unavailability in this schedule, not false acceptance. That still matters: a valid result can become needlessly unverifiable because the advertised evidence route cannot navigate its own retained content.

## 5 V3 The body budget is not a complete request budget

Packet construction deducts a quarter of the configured budget for narrative and shares the remainder across diff bodies. Acceptance summaries, check details, failed names, findings, notes, path labels and per-item formatting are added outside that accounting. `render` emits every item and its labels. Expansion charges content bytes but not its framing; the round loop retains previous expansions. [`packet construction`](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/verification/evidence_packet.go#L153-L245), [`rendering`](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/verification/evidence_packet.go#L121-L147).

The [configuration](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/config/delegation.go#L15-L50) documents an evidence-packet and per-expansion budget. The precise finding is that those body allowances are not a bound on the serialized packet or the total request submitted to the model. The independently compiled system prompt also consumes context. No claim is made here about a provider's ultimate context ceiling or the frequency of exceeding it.

For initial request evidence length P and retained expansion lengths E1 through ER, the total evidence text submitted across R further calls is:

`(R + 1) * P + sum((R - i + 1) * Ei, i = 1..R)`

This describes input presentation, not billed uncached tokens: provider cache accounting may change price, and tokenization changes the byte-to-token relationship. It makes the accumulation cost explicit, including repeated prefixes that supplied no new evidence.

## 6 Proposed capability contract

### Applicability before every disposition

Keep one host-produced manifest for the attempt: attempt identity, task or contract identity, relevant source snapshot, evidence items and rubric version. Preserve unavailable identities as unavailable. Toolchain, configuration and environment identities per check are already deferred by the earlier audit; do not pretend this report discovers or implements them.

The host measures applicability as `current`, `superseded`, or `unknown`. Mangle derives whether the current evidence supports acceptance, needs remeasurement, or must remain historical. No model field may self-assert current applicability. The prerequisite applies to pass, defect and policy-accepted low-confidence objections, not only evidence citations.

On supersession, preserve the original outcome as evidence about S. Do not label a concurrent edit a defect in the previous worker, and do not automatically rerun a mutating worker to repair a verification limitation. Derive the smallest authorized evidence refresh needed. If current applicability cannot be established, say so and retain the historical result without displaying it as present completion.

A final hash is not a permanent lock against external writers. Define the acceptance linearization point and show the verified revision or receipt in the result. Where internal writers can coordinate, an epoch, lease, or compare-and-set can make the acceptance publication coherent. External changes after that point remain subsequent changes, not retroactive falsification of historical evidence.

Checking only written files is not sufficient when unchanged dependencies, configuration or other behavior-relevant inputs have changed. Reuse an appropriate existing workspace snapshot where its scope is sufficient; otherwise represent missing coverage explicitly. A narrower dependency manifest can reduce work only after its completeness is validated.

### Progressive disclosure with measurable progress

Extend the request contract to address an item and a host-validated range or cursor over retained immutable content. Keep source identity, disclosed intervals, remaining spans and a manifest of omissions. The host supplies only authorized packet content; no arbitrary paths, shell, write capability, or new source acquisition is implied.

Deduplicate identical requests and ranges already available in the active requested view. A next acquisition must disclose useful evidence that is new to that view, validly rehydrate evidence evicted from it, or return an explicit no-progress reason. Previously disclosed does not mean permanently unnecessary: the coverage ledger must distinguish historical disclosure from present availability, so deduplication cannot disable recovery. Repeating a prefix that is still present is not progress; restoring a needed evicted range can be. Do not equate a positive original omitted count with proof that another identical expansion can advance the judge. Keep enough local context around selected ranges to interpret them, and surface when a requested context window cannot fit.

This is bounded context with progress, not permission for unlimited calls. Preserve applicable resource constraints and make any exhaustion visible. Changes to request shape and recovery language belong in versioned JIT atoms, following the repository's [North Star and runtime contract](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/agents.md#north-star).

### Complete request admission

Budget the final serialized request, reserving the task and mandatory judge-contract overhead. Charge labels, metadata, omission markers and framing as well as content. Keep body-size bounds as useful subordinate limits, not a substitute for request admission.

Build each follow-on request from the stable manifest, the currently selected evidence and coverage state rather than blindly accumulating repeated expansions. Information may be evicted from the active prompt only when the omitted ranges remain explicitly recoverable. If critical evidence cannot be supplied within the allowed request, preserve insufficient evidence; do not trade an unknown for a pass.

## 7 Costs risks and fair alternatives

The objective is independently verified task quality at acceptable complete-system cost, not minimum judge tokens in isolation:

`C_total = execution + gates + acceptance + identity + packet + JIT + judge_calls + evidence_reads + repair_and_reverification + persistence`

Count every failed, canceled, superseded and successful attempt. Include provider input/output and exposed reasoning accounting; distinguish cached reads and writes without double-counting them in input totals. Include hashing, read and diff CPU, bytes read, peak memory, retained pre/post content, fact publication and fixpoint work, storage, lock contention, wall time, and latency to trustworthy acceptance.

Freshness checks can add substantial I/O on large repositories. Broader snapshots can reduce false currentness but increase delay and invalidate attempts for irrelevant changes. Narrow manifests are cheaper but can miss dependencies. Range acquisition can reduce duplicate input while increasing call count or depriving the judge of needed surrounding context. Strict request budgets can increase abstentions. Leases can reduce races but create contention and recovery obligations. None is a measured win yet.

Compare at least these alternatives:

1. Existing implementation with its current body budgets and round cap.
2. Status-independent applicability alone, preserving acquisition behavior.
3. Progressive range acquisition alone, preserving existing applicability semantics for experimental isolation.
4. Complete-request admission alone.
5. Combined proposal, with all added refreshes, reads and abstentions charged.

Avoid letting a variant take credit for extra tests, a stronger model, more evidence permissions or a larger total budget that the baseline did not receive. Report quality-cost Pareto curves rather than a single token percentage. Keep a no-judge ablation only as an experiment for incremental value, not a recommendation to remove the existing gates.

## 8 Evaluation plan not executed results

### Deterministic boundary cases

Replay identical immutable observations into current and proposed contracts with independently specified expected dispositions:

- Concurrent source change during a judge pass, a high-confidence defect, a low-confidence defect and an acquisition request.
- A changed dependency or configuration while the cited changed file remains identical.
- Unknown identity, unreadable files, deletion and a source change between applicability measurement and acceptance publication.
- A historical valid result after supersession, without an automatic mutation retry.
- Decisive code before and after each packet or expansion boundary; a large single file, many files, reordered requests and repeated identical requests.
- Unicode boundaries, empty content, binary representations and unknown preimages without invented detail.
- Large check details, long failed-test lists, findings, path labels, notes and acceptance summaries independent of diff size.
- Verification that every submitted serialized request fits its declared allowance and all omitted critical evidence remains visible as an obligation.
- Review tasks whose narrative is the artifact, retaining the intended narrative-grounding behavior.
- Existing invalid-reply, non-done worker, missing-client and judge-unavailable cases, preserving fail-closed behavior without redundant worker execution.

Measure stale acceptance, false supersession, unresolved applicability, novel bytes per acquisition, duplicate bytes and ranges, request-admission violations, grounded-defect precision and recall, unnecessary worker retries and replay cost. A case that remains unresolved is not a verified success.

### Production path evaluation

Use the normal TUI `RouteDelegate` journey with CodeDOM, world-model context and ordinary permissions enabled. Pair initial repository snapshots, user requests, model version, sampling, tools and total budget. Include both single-file and multi-file tasks, large source artifacts, baseline-broken repositories, varying dependency surface, and concurrent edits under a controlled schedule.

Independent hidden acceptance checks and reviewers must evaluate the final revision, not merely the frozen packet or quality of the explanation. Split by repository and task family to keep calibration separate from held-out evaluation. Verify the result shown in the TUI as well as the internal disposition, including historical versus current wording.

Report verified completions, escaped regressions, false vetoes, unverified outcomes, canceled work, added remeasurement, total tokens and cost, read/hash resources and latency distributions. Use paired uncertainty intervals. Do not claim general harness superiority from one model, language or small fixture set.

## 9 Checkable handoff

Update (2026-10-28): the research-branch verification audit was reconciled with main's as-built appendix in commit `62e90b2836b0afe15cee49b05950ff1fa97d20d3`, preserving the narrative-as-artifact decision and deferred work. The remaining documentation step is to add explicit gap rows to the verification, observation, session and core specifications for the narrower contracts above. No package-wide redesign is necessary to state these exits.

Acceptance criteria for future authorized work:

1. A valid pass over a superseded relevant revision cannot become current acceptance solely because it has no citations.
2. Historical verification remains inspectable without being confused with the selected workspace's current state.
3. Another evidence acquisition adds evidence to the active requested view, validly rehydrates needed evicted evidence, or returns an explicit no-progress disposition; it does not silently replay an already-present prefix as progress.
4. The complete serialized request honors its admission contract, with explicit omissions and recoverable retained evidence.
5. Unknown applicability or evidence exhaustion never becomes pass, and judge limitations alone never trigger an ungrounded mutation retry.
6. Review-narrative evaluation, current kernel completion gates, permission constraints and existing error semantics remain intact.
7. End-to-end evaluation reports the added costs and quality tradeoffs before any efficiency or reliability claim.

The stopping point of this research is the frozen source-grounded proposal. No implementation or test execution accompanies it.

## As-built status

Verified and built 2026-10-28 on main (after `c28a1212`). Every claim was re-read against current main; none had been fixed since the source freeze.

### Claim ledger

| Claim | Verdict | Evidence |
|---|---|---|
| Section 1: judge/v2 contract, typed packet, judge-only-done, evidence rounds, final-edit remeasurement are implemented | Confirmed | `judge_contract.go` `parseJudgment`; `evidence_packet.go` `buildEvidencePacket`; `delegation.mg` `delegation_judge_due`, `judge_may_acquire`; `change_evidence.go:70` |
| Narrative-as-artifact is an intentional exception | Confirmed, kept | `ground` still accepts a narrative citation |
| V1: a pass cites nothing, so no freshness check runs before `/accept` | Real, fixed. Before: `usable` was the only check (`evidence_packet.go:81`), reached only through citations; `delegation_attempt_accepted` had no applicability conjunct | `applicability.go:52` measures after every round; `verifier.go:382` asserts `judge_evidence_applicability`; `delegation.mg:408-467` |
| V1: an unreferenced dependency can change while the cited file stays current | Real, fixed | The workspace snapshot (`evidence.Snapshot`) is re-measured, not only the written files |
| V2: `expand` returns the first B bytes every time; a repeat request re-sends the prefix | Real, fixed. Before: `cutUTF8(it.full, budget)` (`evidence_packet.go:110`) | `evidence_disclosure.go:73`: per-item `disclosed` cursor, next range only, line-aligned |
| V2: no explicit no-progress disposition | Real, fixed | `unexpandable` reasons (`evidence_disclosure.go:19`) become `Judgment.Unsupplied` and reach `unverifiableReason` |
| V3: body budget is not a request budget (labels, details, findings, notes, framing uncharged) | Real, fixed | `admit` (`evidence_packet.go:286`) charges header, labels and worst-case markers first; bodies share the rest (`shareFairly`); disclosure charges its framing |
| V3: follow-on requests accumulate repeated prefixes | Real, fixed by V2: requests still accumulate, but no range is disclosed twice, so each request is at most (1+rounds) budgets plus the task | `config/delegation.go` `JudgeEvidenceBytes` doc |

### What was built

- `observation.Return.Workspace`, set by `withTurnWrites` (`turn_writes.go:21`): the tree the turn's snapshot and writes describe. (The field landed in lane F's commit `c3f9333a`, which committed `subagent.go` whole.)
- `Applicability` (`/current`, `/superseded`, `/unknown`), measured by the host after every judge round: each changed file by identity, then the workspace snapshot against the bound one. When the attempt bound no snapshot, the tree is measured when the packet is frozen, so the judge window is still covered.
- Policy: `judge_evidence_current` is required for `delegation_attempt_accepted`; `judge_evidence_historical` (anything not current) derives `/unverifiable`, is excluded from `delegation_attempt_failed` (no worker retry) and from `judge_may_acquire` (no evidence round). Ruling: no new move atom; historical evidence is unverifiable for the workspace now, and the reason says the judgment stands for the attempt as it was.
- An accepted delegation's reason names the snapshot it was current for.
- Progressive disclosure: ranges from a cursor; a cut check detail, acceptance summary, finding, note or narrative is expandable from where its body stopped. Ruling: host-owned sequential cursor, no reply-contract change (random-access ranges would need judge/v3).
- Request admission: labels are never cut; a packet whose labels alone overflow `judge_evidence_bytes` is refused and the judge is unavailable (fail closed). Long lists (untested paths, changed paths, finding messages, notes) moved from labels to bodies so they are cuttable and recoverable.
- JIT atom `eval/delegation_judge/contract` teaches range requests.

### Tests

`applicability_test.go` (pass over a superseded changed file and over a changed dependency, unknown workspace, current control naming the snapshot); `evidence_disclosure_test.go` (decisive code past the first range reached in three rounds with each line disclosed once, unsuppliable requests named, cursor exhaustion, whole rendered packet within budget, label overflow refused); `verifier_delegation_test.go` broken reply after an evidence round.

### Left open

- Section 7 cost comparison and section 8 production-path evaluation: need live models and held-out tasks.
- Leases or compare-and-set at acceptance publication: external writers are outside codeNERD's control; the linearization point is the last applicability measurement, and the accepted reason records the snapshot.
- Remeasurement on supersession (a fresh packet for the new tree): not built; the attempt is unverifiable and the user sees why. A refresh would rerun gates, which is the worker's turn closure, not the judge's.
- The embedded prompt corpus needs a rebuild for the atom change.
