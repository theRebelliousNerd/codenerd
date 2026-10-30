---
doc-class: deep-dive
subsystem: verification
implementation-status: proposed
last-verified: 2026-10-30
verified-against: fef98230c20a5751e4c32a2fd2757832a9d39cdc
verification-method: static-source-inspection and independent source challenge
supersedes: []
---

# Verifier change evidence coverage

**Decision:** preserve the delegation judge's restricted evidence authority, validated reply contract, currentness checks and kernel-owned disposition. Extend the retained change representation so a judge can inspect deleted-file preimages and removed hunks omitted from a bounded diff. This is a proposed capability extension to the deliberately post-change-oriented evidence policy, not a claim that the earlier progressive-disclosure repair regressed.

**Finding:** a normal delegated attempt can retain complete before-and-after observations upstream while the judge receives only a deletion marker, or a bounded diff whose omitted removals cannot be recovered through post-change expansion. The missing information can matter to deciding whether a removal satisfied the request. Source proves the projection and recovery limitation; it does not prove that a judge accepted an incorrect deletion or that this caused a reported production failure.

**Evidence boundary:** public main was resolved again at approximately 05:45 UTC on 2026-10-30 to [`fef98230c20a5751e4c32a2fd2757832a9d39cdc`][main], the same head inspected earlier in this pass. No codeNERD runtime, tests, model inference, training, benchmark or failure injection was run. No implementation was changed. All behavioral witnesses and evaluation schedules below are unexecuted.

## Research context

[Liu and colleagues’ long-context study](https://arxiv.org/abs/2307.03172) shows that access to relevant context can vary with its position in evaluated tasks. It motivates checking both coverage and actual judge requests rather than assuming retained bytes guarantee use; it does not establish any particular model verdict in CodeNERD.

## Existing policy and prior coverage

The repository's North Star makes the model the creative center, the Mangle kernel the executive, and the host the producer and transporter of observations. Relevant source guidance requires revision-bound completion, distinct artifact/check/behavior stages, and caller-provided rather than model-invented acceptance. The proposed extension follows that division: Go retains and measures evidence; Mangle continues to decide admission, continuation and disposition; the judge evaluates only the resulting authorized packet. [Repository guidance][guidance] [Session guidance][session-guidance]

Two existing audits already own most of this area:

- [Verification evidence and judge contracts][judge-audit] documents the implemented typed packet, strict judge reply validation, explicit worker-reported narrative, evidence-backed judgments and policy-mediated additional evidence. Its as-built section explicitly restricts requests to elided packet items, specifically retained post-change content. No arbitrary path reads are implied. This report does not call that intentional restriction an accidental omission or claim to have discovered that the judge lacks general tools.
- [Verification applicability and progressive evidence][applicability-audit] documents the implemented workspace applicability check, range cursor, no-progress disposition and complete packet-framing budget. It fixes repeating the same postimage prefix. The finding here is different: progressing through every byte of the selected postimage still cannot disclose bytes that were removed from it.

The [delegated verdict handoff audit][handoff-audit] concerns preserving a verifier disposition for a later task. This report concerns the evidence supplied before the current judgment, not subsequent handoff status. The existing [test evidence audit][test-audit] covers test-obligation completeness and truthful completion summaries; it does not establish that a mechanical pass proves the semantics of every removal.

Public branch enumeration during this pass listed main, three `lane/worktree-agent-*` branches and two QA branches, with no current branch named as a research branch. The audit documents above were inspected on the pinned main. That observation says nothing about unpublished work or deleted historical research branches. Repository search was used only to discover possible duplicates: its index returned an older revision, so current source claims below use direct reads at the pinned SHA.

## The production evidence path

The ordinary TUI mutation delegation is not an unverified prose-only path. It refuses a mutation without the derived verification obligation, refuses a required but unwired verifier, calls `VerifyWithRetry`, and requires `verdict.Accepted` before formatting verified success. [TUI delegation][tui]

The verifier invokes the observed task executor, asserts the worker's outcome, and asks the kernel whether a judge is due. Policy judges only an attempt whose outcome is `/done`. The judge can withhold that attempt; it cannot promote an unresolved worker attempt to done. [Observed invocation][verifier-entry] [Judge policy][judge-policy]

The inline observed executor calls `withTurnWrites` after execution. That adapter attaches the workspace, each changed path, the preimage recorded before the first write, and the final file state. A missing final path is represented as known and absent, not as an unreadable file. The inspected test definition explicitly includes a deleted file with a retained known preimage. [Executor bridge][executor-bridge] [Write observations][writes] [Write observation test][writes-test]

`buildEvidencePacket` receives those observations and creates a change item per write. The packet is then rendered into the actual judge user request. Further evidence rounds append only the packet's admitted expansions; the judge is not a general source-reading tool loop. [Packet construction][change-items] [Actual judge request][judge-request] [Judge prompt contract][judge-prompt]

The established seam is therefore:

```text
Observed return with before and after states
  -> bounded change item and selected expansion representation
  -> actual judge request and later packet expansions
  -> validated judgment and currentness measurement
  -> kernel delegation disposition
```

## Whole file deletion loses retained source detail

For a write whose final state is known and absent, the packet body is the short text `the attempt deleted the file`, enclosed in parentheses. Its label retains the before-state digest and the after-state label `absent`, but the branch does not render a deletion diff and does not retain `Before.Content` as expansion backing. [Deletion branch][change-items]

When that short marker fits the body allocation, `admit` records no omitted bytes and clears expansion backing. A later request for the item is classified as already fully shown. Even if an unusually tight allocation cuts the marker, the fallback can retain only that marker, not the discarded preimage. A digest identifies content; it does not let the judge recover or inspect it. [Admission][admit] [Expansion availability][unexpandable]

### Unexecuted deleted file witness

Construct an observed `/done` attempt with one permitted whole-file deletion. Its known preimage contains a unique decision-relevant sentinel; its final state is known and absent. The original task, worker narrative, findings and notes do not repeat that sentinel. Use a budget that comfortably fits the deletion marker and normal packet labels.

The source-derived expectation is precise: the judge request contains the deletion identity and marker but not the sentinel; requesting the change item cannot supply the retained deleted content. The missing body can prevent adjudicating whether the file was actually obsolete or whether its removal discarded a requirement the task intended to preserve. This witness does not require a fabricated passing test claim, a source race or an unauthorized deletion.

This is not a claim that every deletion needs its complete preimage in the initial prompt. A request naming one exact file to delete can sometimes be settled from the path and observed absence alone. The proposed capability is to disclose decision-relevant retained detail when needed, with honest coverage and bounded cost.

## A postimage cannot recover an omitted removed hunk

For a surviving file, the initial body is a rendered diff, while `full` is set to the post-change content. If admission truncates the diff and the postimage is nonempty, the existing nonempty `full` remains. Subsequent disclosures walk that postimage, not the omitted tail of the diff. [Change item representation][change-items] [Admission fallback][admit]

That is useful for additions and for inspecting the resulting implementation. It is insufficient to recover a removed line that appears only in an omitted diff hunk. Once the entire postimage has been disclosed, the cursor reports exhaustion even though the judge has never seen those removed bytes. This is a coverage difference between representations, not a broken cursor. [Progressive disclosure][disclosure]

### Unexecuted truncated removal witness

Use a modified file with a large multi-hunk diff and a nonempty final body. Place the only decision-relevant removed sentinel beyond the prefix allocated to the initial diff. Ensure the sentinel is absent from the postimage, narrative and other evidence. Request every available range of the change item, with enough permitted rounds to exhaust its retained postimage.

The expected source-level result is that no expansion can reveal the sentinel. The test must establish both trigger conditions: the removed hunk really lies outside the initial admitted prefix, and the postimage remains nonempty. A small complete diff is not a valid reproducer for this limitation.

### Independent challenge and boundaries

An independent source challenge confirmed the two mechanisms and required these qualifications:

- **Existing empty postimage:** when the surviving file's content is empty, `full == ""` allows the admission fallback to retain the original truncated diff and its prefix offset. That case can recover removed content and is an essential positive control, not part of the affected partial-removal claim.
- **True file deletion:** absence takes its own marker-only branch before diff rendering. It is different from leaving an empty file on disk.
- **Deliberate authority restriction:** the judge's lack of arbitrary tools is an existing, intentional safety boundary. The extension must operate on retained, authorized observations, not silently add general read access.
- **Earlier audit status:** post-change-only disclosure is explicitly documented in the earlier as-built policy. This report proposes richer representations and coverage, rather than calling the current behavior a regression against an already-promised preimage API.
- **Honest abstention:** an insufficient-evidence judgment can fail the delegation closed. Evidence loss is not itself proof of false acceptance.

## Existing safeguards remain necessary

The strict reply parser and grounding path remain useful. The host measures file identity and the workspace snapshot after every judge round. The acceptance rule requires a done attempt, current evidence, no rejecting judgment and no unresolved judgment. Historical or unknown applicability does not authorize acceptance or a worker retry. [Applicability measurement][applicability] [Disposition policy][accept-policy]

The proposed witnesses hold applicability current; they do not reopen the earlier stale-pass finding. They also do not demonstrate bypass of deletion permission, removed-test checks, behavioral pinning, ordinary build/tests or a caller acceptance contract. Those gates can independently prevent an incorrect result. Conversely, those checks do not generally establish every semantic claim about removed code or prose, which is one reason the quality judge exists.

A cautious judge may request the missing representation and reach an honest unavailable result, while another might judge the available evidence sufficient. The current source does not determine how often either happens. Do not describe an unexecuted packet witness as a measured user-visible misjudgment, security exploit, data loss incident or end-to-end completion failure.

## Proposed representation contract

Extend the selected contract before implementation, in the owning verification and observation specifications. Preserve the existing APIs' authority and kernel decision ownership.

1. **Name representations explicitly.** Distinguish a diff, a preimage and a postimage, with their source state, existence, knownness, content identity and byte coverage. An omitted diff byte count is not a statement about coverage of the postimage.
2. **Preserve the evidence the packet omitted.** A bounded diff's omitted removed hunks must remain recoverable as part of the retained diff or an explicitly linked preimage representation. A known deleted-file preimage must not be represented as wholly disclosed merely because its deletion marker was shown.
3. **Keep initial prompts bounded.** Do not inject every full preimage by default. Retain compact change identity and coverage, then let policy-mediated evidence requests disclose bounded useful ranges. Existing whole-packet and per-disclosure accounting must include any new labels and framing.
4. **Keep unknowns honest.** If the preimage was not recorded, say it is unknown. Do not reconstruct it from model prose, a guessed Git revision or a current file read. Absent, empty, unknown and binary states need distinct semantics.
5. **Keep currentness separate from historical provenance.** The preimage is evidence of what the attempt changed, not a claim that those bytes still exist. Its attempt and before-state identity must remain bound to the same after-state and workspace applicability checks used for disposition.
6. **Do not broaden authority.** Disclose only observations already retained under the existing task and path authorization. No arbitrary file paths, shell commands, new mutation permissions or hidden worker reruns are part of this proposal. If a future design needs new acquisition, give it a separate explicit permission contract.
7. **Represent unavailable detail truthfully.** An unsupported representation or exhausted budget stays unavailable or unresolved. It must not become a synthetic pass, and a judge limitation must not trigger effect-repeating worker recovery.

A conservative option is to retain the omitted diff as the first expansion source, including complete deletion diffs. A richer option exposes bounded diff/preimage/postimage views separately. The simpler option may require fewer protocol changes; the richer option can support more context-sensitive review but introduces representation selection, coverage metadata and retention complexity. Neither is automatically the better whole-system tradeoff.

## Evaluation before claiming improvement

### Deterministic qualification

Exercise the real packet builder, `VerifyWithRetry`, reply validation, policy and actual captured judge requests. A renderer-only fixture with manually supplied missing content would skip the seam. Existing progressive-disclosure tests are useful controls: they demonstrate reaching added code beyond a first range, cursor exhaustion and packet budgets. They were inspected, not run here, and do not establish the proposed removal-specific contracts. [Existing disclosure tests][disclosure-tests]

| Case | Required observation |
|---|---|
| Known whole-file deletion with preimage-only sentinel | Sentinel is recoverable under the retained deletion evidence identity, or its unavailability is explicitly represented; marker is not misreported as full source coverage. |
| Truncated diff with removed sentinel and nonempty postimage | Omitted removed detail is reachable without arbitrary source acquisition; exhaustively reading only the postimage is not credited as complete diff coverage. |
| Same change with an empty surviving postimage | Existing diff fallback remains correct and distinguishable from actual absence. |
| Small complete diff and newly created file | No needless expansion or regression in the successful simple path. |
| Known empty preimage, unknown preimage and binary representation | Distinct truthful states without invented content or completeness. |
| Deleted path recreated or surviving file changed before judgment completes | Existing superseded/unknown applicability behavior remains fail closed. |
| Unicode boundaries, long lines, many changed files and tight budgets | Bounded complete serialization, honest remaining coverage and no repeated range presented as new evidence. |
| Judge requests unavailable evidence | Named limitation and unresolved disposition, not a worker rerun or fabricated acceptance. |
| Legitimate deletion and unintended removal with identical mechanical outcomes | Distinguish evidence availability from the judge's actual decision quality. |
| Original normal TUI mutation delegation | Actual request includes the intended evidence; permission, worker verdict and verifier outcome retain their existing authority. |

### Paired production evaluation

Compare three alternatives under the same tasks, starting revisions, model/version, permissions, ordinary world/CodeDOM context and total resource budgets:

- current post-change-oriented packet;
- a complete-change reference containing the full relevant diff, with explicitly larger evidence cost if needed;
- the proposed bounded representation-aware packet and progressive disclosure.

Use both benign removal and harmful over-removal tasks, including documentation deletion, code refactoring, mixed additions/removals and large multi-file changes. Keep original task wording and independent expected behavior fixed. Randomize treatment order, include warm/cold caches, and evaluate held-out tasks rather than tuning exclusively to sentinel fixtures.

Report false acceptance, false rejection, justified abstention, unresolved obligations, correct recovery and human repair separately. A design that simply refuses more work is not automatically more capable; one that accepts more work is not automatically more accurate. Mechanical controls should remain equal across arms so extra tests or relaxed permissions are not mistaken for a packet improvement.

### Cost accounting

Measure actual serialized request bytes/tokens, judge round count, full provider-reported usage including unsuccessful rounds, worker repair/reverification cost when legitimately triggered, wall latency, time to useful resolution and human recovery. Do not double-count cached-input or reasoning subtotals already included in provider totals.

On the host, measure retained observation lifetime, extra diff/representation memory, serialization CPU, identity and snapshot I/O, metadata volume, cleanup and any persistence cost. Before/after bodies are already present in the observed return, so this proposal need not acquire new source bytes; retaining additional views can nevertheless increase lifetime memory and prompt cost. Storing an additional diff or another string view has different allocation behavior, which requires measurement rather than an assumption that reuse is free.

No net token saving, speedup, prevalence, cost reduction or accuracy gain is established by this static audit. The first checkable outcome is narrower: **a judge can recover relevant removed content already held by the authorized execution observation, and the packet reports exactly which representation and ranges it has shown.**

## Source references

[main]: https://github.com/theRebelliousNerd/codenerd/commit/fef98230c20a5751e4c32a2fd2757832a9d39cdc
[guidance]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/agents.md
[session-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/session/agents.md
[judge-audit]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/Docs/audits/verification-evidence-and-judge-contracts.md
[applicability-audit]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/Docs/audits/verification-applicability-and-progressive-evidence.md
[handoff-audit]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/Docs/audits/delegated-verdict-handoff.md
[test-audit]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/Docs/audits/test-evidence-capability.md
[tui]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/cmd/nerd/chat/process.go#L599-L669
[verifier-entry]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/verification/verifier.go#L283-L318
[judge-policy]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/core/defaults/policy/delegation.mg#L390-L418
[executor-bridge]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/session/task_executor.go#L443-L472
[writes]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/session/turn_writes.go#L20-L45
[writes-test]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/session/turn_writes_test.go#L13-L48
[change-items]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/verification/evidence_packet.go#L223-L250
[admit]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/verification/evidence_packet.go#L281-L324
[unexpandable]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/verification/evidence_disclosure.go#L18-L38
[disclosure]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/verification/evidence_disclosure.go#L41-L132
[judge-request]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/verification/verifier.go#L360-L396
[judge-prompt]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/prompt/atoms/eval/delegation_judge.yaml
[applicability]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/verification/applicability.go#L52-L84
[accept-policy]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/core/defaults/policy/delegation.mg#L440-L467
[disclosure-tests]: https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/verification/evidence_disclosure_test.go
