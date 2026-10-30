# Verified delivery checks need the observed change set

## Finding and scope

The TUI's verified mutation path gives its delivery-coverage check a newly constructed observation containing only the worker's output and a done outcome. The worker's measured `Changed` paths are not carried through that construction. Delivery therefore omits its dedicated `files_changed` input for the initial verified step, even when the observed executor and the verification judge had those paths.

This is a narrow evidence-handoff omission after successful verification. It is **not** evidence that verification ran without source evidence, that every path disappears from the report, or that the later delivery judge necessarily makes the wrong decision. The verification result still contains its evidence packet. Worker output can name paths, and the host's closing report often names them too. The lost guarantee is that the delivery check receives the explicit observed change set that its existing input and recorder support.

Preserve an appropriately scoped typed change receipt from the accepted attempt into delivery, rather than relying on reconstruction from prose. Keep the original verification, permission, delivery-stopping and work-mode controls unchanged. No new model call is inherently required to transfer information already held by the host.

This report is pinned to public main [`388205138ba6d47c1cff0c5e97c896dfa642a391`](https://github.com/theRebelliousNerd/codenerd/commit/388205138ba6d47c1cff0c5e97c896dfa642a391), revalidated on 2026-10-30 at approximately 10:45 UTC. Root and relevant chat guidance, source, test definitions, the main audit inventory, and research branch scopes through `research-main-20261030-1018` were inspected. An independent source challenge corroborated the omission and emphasized the retained evidence packet and possible narrative path information. No application execution, tests, builds, inference, failure injection, or performance measurements were performed. The witnesses and evaluations below are proposed, not reproduced incidents.

## Existing evidence improvements remain effective

The [verification evidence and judge contract audit's as-built section](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/verification-evidence-and-judge-contracts.md#L192-L209) records a real typed observation and evidence-packet boundary. The judge receives kernel outcome, observed checks, changed-file evidence, findings and other structured context, with the worker narrative distinguished from host evidence. `VerificationResult` separates the kernel's accepted disposition from the judge's judgment and retains the evidence packet. [Current result type](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/verification/verifier.go#L99-L116).

Commit [`f01bfee19e997baecbe330185c3bc19309d66a4d`](https://github.com/theRebelliousNerd/codenerd/commit/f01bfee19e997baecbe330185c3bc19309d66a4d) further repaired deletion and truncated-diff evidence. The [change-evidence audit as-built section](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/verifier-change-evidence-coverage.md#L178-L205) explains that the retained representation now supports subsequent disclosure, including removed text, while applicability and reply checks remain wired. None of that work should be reverted or reported as absent.

The present finding begins later: after the verifier has accepted and returned, when chat starts a separate request-delivery check. It is distinct from the earlier [delivery restriction continuation](https://github.com/theRebelliousNerd/codenerd/blob/136e02f0a38897a40931747b1e56a6f0c14feb197/Docs/audits/delivery-continuation-restriction.md), which concerns inherited execution restrictions, and [delivery answer ownership](https://github.com/theRebelliousNerd/codenerd/blob/77069960b69aba14a67dbd682d6f65435e8ada50/Docs/audits/delivery-terminal-answer-ownership.md), which concerns terminal answer attribution. This report does not require a second worker, asynchronous answer, or missing restriction. Its boundary is the evidence supplied to the first delivery reading.

## Production chain

### The observed executor knows what was written

`observedReturn` assigns output, outcome, missing evidence, stage and snapshot from the execution result. It copies successful mutation `WrittenPaths` into `Changed`, and projects effect receipts separately. These paths are measured execution information, not a claim extracted from worker narration. [Observation construction](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/session/observed_return.go#L47-L86).

`VerifyWithRetry` requires an observed executor and receives its complete return for each attempt. It uses the outcome for policy and passes the return to the grounded judge. `buildEvidencePacket` creates observed change items from retained writes or, when their bodies are unavailable, from the changed-path list. The source therefore establishes that the first verification consumer can have the very information absent from the subsequent delivery input. [Observed attempt and judgment](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/verification/verifier.go#L253-L317), [change evidence items](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/verification/evidence_packet.go#L182-L246).

### Successful verification returns a narrower shape

On the kernel's accept move, the verifier sets `verdict.Accepted`, stores the verification, and returns `ret.Output, verdict, nil`. Its public return does not include the original observation. The verdict does retain `Evidence`; saying the change evidence is erased from all host state would be wrong. The delivery caller, however, does not project that packet into a change receipt. [Accepted return](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/verification/verifier.go#L319-L335).

Chat handles verification error, missing verdict and unaccepted verdict before starting delivery. Once accepted, it formats the verified response, opens a delivery chain, and calls:

```go
m.checkContinuation(ctx, dc, shardType, task,
    observation.Return{Output: result, Outcome: "/done"})
```

No `Changed` field is initialized. This is a new observation-shaped value, not the worker's original observation. The done outcome is not itself a demonstrated falsification: the accepted path has already passed the verifier's guards. The omitted change metadata is the finding. [Verified response and delivery construction](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/process.go#L649-L687).

### Delivery cannot recover the omitted typed list

`checkContinuation` injects result facts, records the supplied return, and asks for the next step. The delivery recorder appends `ret.Output` to its reports and adds paths only by iterating `ret.Changed`. `judgeDelivery` passes those reports and collected paths into `learnjudge.DeliveryInput`. [Continuation entry](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/process_continuation.go#L23-L37), [recording and delivery call](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/request_delivery.go#L80-L123).

The delivery-state builder includes `files_changed` only for a nonempty `Changed` list. It does not look up the verifier's evidence packet or recover paths from a different kernel relation. Thus the initial verified step's empty synthetic list produces an absent field, even if the source observation had a nonempty list. Absence is not an explicit assertion that no files changed; the current helper deliberately omits empty lists instead of sending an empty signal. [Input type and request construction](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/learnjudge/delivery.go#L24-L30), [state builder](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/learnjudge/delivery.go#L93-L113).

## Reachable witness and bounded consequence

Use an initialized idle TUI with an admitted foreground `/mutation` delegation, an available verifier and observed executor, and a permitted edit to a concrete workspace path. The worker returns a nonempty observed change list and a done outcome; the verifier accepts under its normal policy. Instruction coverage is enabled with the kernel, Jev client, configuration and nonempty request fragments needed to open a delivery chain. Successful fact operations and no earlier ordinary pending obligation allow `nextStep` to reach delivery judging. [Delivery enablement](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/request_delivery.go#L46-L69), [selection order](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/request_delivery.go#L276-L287).

Capture the state sent to Jev for that delivery reading. The source predicts a report but no `files_changed` field from the accepted step. The original request can have been fully satisfied; no false verifier acceptance, incomplete implementation or continuation offer is required for this transport witness.

The quality implication is conditional. Losing a dedicated observed path list can make delivery assessment more dependent on report wording and its bounded presentation. It might affect uncertainty, missing-part classification, or later recovery work. None of those outcomes was measured. A changed path alone also does not prove that a requested behavior was delivered; adding the list must not turn file existence or a write into automatic semantic acceptance. The current coverage question asks what the report does or answers, so any benefit from supplemental path evidence needs evaluation rather than assumption. [Coverage question](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/learnjudge/coverage.go#L87-L94).

Two counterweights are especially important:

- **Narrative can preserve the information.** The worker may name paths. For write turns without an acceptance report, `appendEvidenceSummary` appends host-observed written paths, listing up to ten and disclosing any additional count. With an acceptance report, it appends that report's summary instead. A small-write example can therefore expose every changed path in `result` despite the missing dedicated field. [Closing report](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/session/change_evidence.go#L352-L378).
- **The verification packet remains available.** `VerificationResult.Evidence` holds the packet shown to the judge. This is a missing projection into a later consumer, not proof of irrecoverable evidence destruction. Retaining a packet elsewhere is useful, but does not establish that the later Jev request consumed it.

This audit consequently establishes neither a wrong final answer nor unnecessary rewriting, hidden failure, permission bypass, or a universal lack of path information.

## Existing controls and positive paths

The verifier's acceptance rule requires a done attempt, current evidence, no rejecting judge result and no unresolved judgment. A worker failure, unavailable verifier or unaccepted result does not take the audited successful handoff. Preserve these gates and the separation between an execution's disposition and the judge's own words. [Acceptance policy](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/defaults/policy/delegation.mg#L441-L482).

Delivery itself preserves useful limits. A failed reading does not fabricate delivery success; ordinary pending obligations take precedence; cut reports cannot support a missing-part derivation; blockers and stalled rounds prevent further pushback; only the active delivery request can raise its offer. These protections constrain any downstream effect of the missing field. [Reading failure](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/request_delivery.go#L114-L138), [delivery policy](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/defaults/policy/request_delivery.mg#L58-L115).

Other producers demonstrate that the delivery recorder can receive the original observation. `executeSubtask` records a non-error observed return directly; multi-step work retains non-error returns and records them after the plan. A nil execution error is not itself a verified-success verdict. Those paths are positive transport controls, not additional instances of the same omission. [Continuation producer](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/process_continuation.go#L68-L97), [multi-step producer](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/process_dream.go#L744-L780).

## Proposed repair contract

Update the relevant verification and chat capability specifications before implementation. The narrow contract is: **the delivery observation for an accepted attempt preserves that attempt's measured changed paths, or explicitly declares the projection unavailable or partial.** It must not infer authoritative paths from output wording or treat absence of the field as measured zero writes.

One option is a small typed projection attached to the verifier's result, containing the accepted attempt's change list and its existing identity or snapshot context. Another is a typed verified-attempt envelope containing the observed return alongside the separate verification disposition. Either can reuse existing data without rereading the workspace, reexecuting the worker, or calling another model. Avoid parsing path labels out of the human-readable evidence packet when a typed producer is available.

Choose the scope deliberately. The smallest repair transports the accepted attempt's list, which is the information lost in this witness. A cumulative request-wide account across retries is a separate contract: earlier attempts may have left effects, reverted changes, or historical observations. Do not silently combine them and label the union as the accepted attempt's current write set. Preserve provenance and use the existing applicability mechanisms for any claim of present state.

Passing the entire original return into `checkContinuation` has another consequence: that helper also feeds structured findings and test debt into continuation derivation. Such a change may be desirable, but it is larger than repairing `files_changed` and can change which tasks are offered. Separate the minimal projection from that behavioral expansion, specify the intended obligation semantics, and test both. Do not assume wholesale observation substitution is a neutral one-line metadata fix. [Other consumers in the helper](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/process_continuation.go#L136-L189).

Keep presentation bounded. A path receipt need not retain full preimages or diffs, and copying large `Writes` bodies merely to transfer names is unnecessary. If a large path list needs bounded disclosure, its omitted count and availability must be explicit; silent truncation would recreate the same coverage problem at a new boundary. No automatic retry, wider authority, or new claim that changed means delivered is part of this proposal.

## Coverage and fair evaluation

The inspected delivery journey fixture deliberately declares `/instruction` so it runs in the foreground without the verification loop used by `/mutation`. It demonstrates delivery continuation, but does not test this successful verified return. The lower-level `CheckDelivery` test explicitly supplies `Changed` and verifies both its inclusion and intentional omission when empty. Preserve those tests; add the missing producer-to-consumer journey. [Journey fixture](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/request_delivery_test.go#L42-L45), [helper contract test](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/learnjudge/delivery_test.go#L35-L73). These are test-definition observations, not executed results or an exhaustive assertion about all indirect coverage.

The primary future regression should submit through normal TUI admission, select the verified mutation lane with the real policy, obtain an observed worker return, traverse `VerifyWithRetry`, and capture the actual delivery `Ask` state. A manually constructed `DeliveryInput` or direct `dc.record` test cannot establish that this producer transferred its evidence. Deterministic adapters can control worker and judge responses, but must not bypass the verifier or manufacture the final delivery state.

| Case | Required observation |
|---|---|
| Accepted verified attempt with a nonempty observed path set | Delivery receives exactly the declared accepted-attempt paths with the correct scope |
| Same attempt with complete path narration | Dedicated receipt survives without deleting or changing useful prose; do not assume a quality gain |
| Bounded report and more than ten changed paths | Capture actual report coverage; any supplemental path projection preserves or honestly discloses the remainder |
| Accepted no-write result | No invented changes; retain the intended empty/unavailable distinction |
| Rejected, unavailable, canceled or failed verification | No successful delivery handoff is manufactured; existing failure settlement remains intact |
| Multiple verifier attempts | Receipt belongs to the accepted attempt under the selected contract, not an arbitrary earlier or cumulative set |
| Ordinary continuation and multi-step producer | Existing observed-path handoffs still work; no duplicated reports or paths |
| Disabled or failing delivery judge, cut reports, blocker, stalled round | Existing stopping behavior and truthful notes remain unchanged |
| Full-return option with findings or test debt | Any new continuation is intentional and policy-admitted, not an unnoticed side effect of receipt repair |

Compare pinned main with the minimal receipt projection under matched requests, workspace revisions, permissions, models, prompts, history, verification decisions and delivery settings. Keep CodeDOM and world-model context enabled for normal-production assessment. Score exact evidence transfer first. Evaluate any subsequent change in coverage classification, unnecessary follow-up, abstention or whole-task quality separately, including cases where narrative already suffices and where a changed file still fails to satisfy the request.

No speedup or token saving is established. Transferring the receipt adds path serialization and potentially Jev input tokens; retaining a full return can prolong much larger before/after bodies. Measure allocation and lifetime costs, bytes sent, request counts, provider tokens, verification and delivery latency, additional authorized work, human recovery and final independently assessed task quality. Count failed and unresolved requests, not just completed tasks. A field-preserving repair should not need another model call or a worker rerun, but any richer evidence acquisition or cumulative retry ledger must account for its own costs.

## Acceptance boundary

Close this gap when the real successful verified mutation path transfers its accepted-attempt change receipt into the actual delivery request, while no-write and failure controls remain truthful and existing evidence/permission gates remain intact. The result establishes a faithful handoff, not that paths alone prove semantic completion. Runtime acceptance and quality effects remain unverified until the proposed normal-entry checks and evaluations are actually performed.
