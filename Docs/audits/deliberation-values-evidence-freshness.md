---
doc-class: deep-dive
subsystem: deliberation-session
implementation-status: proposed-refinement
evidence-status: static-source-inspection
last-verified: 2026-11-03
verified-against: a4981591c95307eab5df20f82c07132b79ef9375
supersedes: []
---

# A values answer must not renew stale deliberation evidence

## Finding and boundary

An exact answer to a saved values question can make a deliberation thread `ready` without checking whether the workspace evidence supporting its alternatives has changed. The shared service reloads the old candidates and challenger assessments, reasserts their success and blocker facts, adds the selected answer, and derives a new disposition. Neither the thread record nor that fast path carries a host-produced workspace/evidence validity identity. The chat adapter then returns the selected design or adopts it into the original request. [Resume][resume] [Round projection][projection] [Thread][thread] [Chat continuation][chat]

The narrow deterministic finding is **fresh evaluation of historical premises without factual-validity admission**. An intact, correctly owned thread and a genuine answer are sufficient; no forged record, malformed JSON, ambiguous candidate ID, missing consultation qualifier, or permission bypass is needed. The code can produce the same readiness result whether a relevant source file stayed unchanged or changed incompatibly during the pause. An actual wrong implementation, escaped permission gate, failed acceptance check or retained bad recurse improvement was **not demonstrated**.

This document is pinned to public main [a4981591c95307eab5df20f82c07132b79ef9375][pin], resolved November 3, 2026. Before freezing, main advanced to `3244509f64ebb0e20ad23f235cd95d4c7ecd55fa`; the [complete one-commit comparison](https://github.com/theRebelliousNerd/codenerd/compare/a4981591c95307eab5df20f82c07132b79ef9375...3244509f64ebb0e20ad23f235cd95d4c7ecd55fa) changed only the credential-pool audit, leaving the cited implementation unchanged. The review is based on static inspection of public guidance, architecture, implementation, existing audits and authored tests. No repository program, test, build, inference, gate, benchmark or failure injection ran. The proposed witnesses and evaluations below are unrun.

**Important scope exclusion:** a normal uninterrupted recurse mission already captures a workspace snapshot before deliberation and compares source and vision afterward before execution admission. This report does not claim that protection is absent. Its demonstrated source path is the shared saved-question continuation used by chat and design-only ideation. Recurse relevance is that the same conductor produces mission questions, and a future owner-addressed mission resume must not treat successful generic answer replay as sufficient execution admission. Current mission-owner isolation must remain. [Mission boundary][mission] [Owner isolation][owners] [Earlier ownership audit][ownership-audit]

## Existing intent and implemented behavior

Repository guidance requires stale evidence not to remain current after source changes and places executive decisions in the kernel. The session specification says readiness is a judgment about the proposed reading, not proof that an implementation passed verification. It also deliberately supports durable values questions and replay of their saved round. These goals are compatible: preserve the preference answer, but independently decide whether the factual basis still supports the design. [Root contract][guidance] [Session design][session]

The requirements interrogator is told to assess candidates against the complete request and workspace evidence. Its `success` field means the reading satisfies the requirements test against that evidence; a values-only fork may pass subject to choosing a preference. Thus the saved success assertion is not solely a record of user taste. It can depend on an API, configuration or implementation fact that later changes. [Interrogator contract][interrogator]

| Boundary | Implemented at the pin | Missing distinction |
|---|---|---|
| Persistence | Atomic thread save; load verifies thread and session identity; original intent, question, answer, rounds and owner fields survive. | Matching thread/session identity is not validity of the source observations. |
| Question construction | The conductor emits at least two policy-eligible values alternatives and one concrete question. | A valid option at question time is not necessarily viable at answer time. |
| Exact answer | Matches a candidate ID or rendered option, reasserts the saved round, adds an answer and calls `finish`. | No current workspace observation is supplied to decide whether reuse is valid. |
| Executive proof | The current kernel derives the disposition and requires recorded proof support. | A fresh derivation does not refresh the old model-authored success premise. |
| Design-only continuation | Returns the chosen brief, assumptions, success test and proof identifier without starting a development worker. | The report does not distinguish a current design from a historical design whose evidence is now stale. |
| Ordinary work continuation | Preserves the original verb/restrictions and returns through normal work admission. | A valid permission decision does not establish the adopted design's factual freshness. |

These are existing protections to retain, not reasons to replay historical executive facts wholesale at boot. [Persistence][persistence] [Question/result][outcome] [Proof][proof] [Chat][chat] [Authored permission control][values-test]

## Source path through the pause

### 1. Successful evidence-based challenge is saved

The first proposer/challenger round is collected through the configured roles. `validateRound` checks candidate IDs, required brief/success text, assessment coverage and blocker vocabulary. `assertRound` converts `Assessment.Success` into `deliberation_success` and copies citations and assessment items into typed facts. A values fork can therefore contain two genuinely viable designs whose only remaining blocker is preference. [Round collection][round] [Validation][validation] [Projection][projection]

`finish` obtains the eligible alternatives, records `awaiting_values`, sets `Asked`, saves the question and options, and publishes the outcome. `Thread` retains the full rounds but has no workspace digest, observed dependency versions or host-issued evidence-validity state. Candidate citations and assumptions are strings; they do not become a verified source binding merely because they are saved. [Question publication][outcome] [Record shape][thread]

### 2. The workspace can change while the question is pending

The thread is designed to survive session reload. A person, editor, another authorized process or a later checkout can change a relevant ordinary workspace file before the answer. The witness needs only a sequential edit completed before resume, not a race during collection or an inconsistent filesystem scan. Persistence checks the conversation identity, not that file's old and current contents. [Load][persistence] [Restart fixture][values-test]

### 3. Exact option replay does not revisit the evidence

The chat continuation loads the specified thread, checks `awaiting_values`, assigns `Answer`, and calls `Service.Run`. At lines 84–93, `Run` recognizes the exact option, reasserts the last round, asserts `deliberation_answer`, and returns directly through `finish`. It does not enter `collectRound`, call `Probe`, consult a specialist, or compare a workspace identity on this branch. [Chat continuation][chat] [Fast path][resume]

The answer disposition rule requires a values-fork decision, the selected candidate, saved success, and the absence of a non-values critical blocker. It has no source-validity predicate. If no new observation is made, an incompatible change cannot introduce a new data/design blocker into this replayed round. [Disposition rule][policy]

`prove` obtains a current proof envelope, including program digest and recorder generation, and replaces the round's proof field. Those identify logical evaluation, not the source version on which the old success assertion was based. `finish` marks the thread ready and `Note` displays the chosen brief, success test and proof ID. This is not a defect in Mangle's derivation: the omitted input is a host observation about whether the premises remain applicable. [Proof envelope][proof] [Ready note][outcome]

### 4. Design-only mode gives a complete witness without claiming unsafe work

`/deliberate` and `/innovate` create design-only threads. When such a thread becomes ready after an answer, `resumeDeliberation` returns its note immediately. No later development-worker inspection is required for that result to reach the user. This is the cleanest source-derived endpoint: **the same selected design can be marked `ready` and reported as “Decided”, without a stale-evidence qualifier, in both changed and unchanged workspaces**. [Design command][chat]

For an ordinary request, `adoptDeliberation` carries the chosen target, brief, success test and assumptions into the original intent and marks the reading resolved. Normal work admission remains downstream; an executor may discover the changed source and repair or reject the plan. That possibility limits the downstream impact, but does not validate the already-published readiness judgment. [Adoption][chat] [Normal route][process]

## Small proposed witness and controls

Use an otherwise healthy chat session in a non-yolo mode, an unchanged policy, valid role output, available proof recording, short payloads and successful storage. Start a design-only deliberation concerning retry behavior in a repository. Let a local API support both fixed-delay and exponential-delay scheduling; both designs satisfy the request, and the only remaining choice is the owner's preference.

1. At workspace state S0, the proposer supplies two candidates citing the real retry API. The challenger correctly marks both successful with only a critical values item. The conductor asks which delay policy to use.
2. Save/reload the session, or simply leave the question pending.
3. In history A, leave the API unchanged. In history B, complete an ordinary edit that removes support for variable delays while leaving fixed-delay support. The selected exponential design's factual precondition is now false. Do not modify the saved thread.
4. In both histories, answer with the exact exponential option originally offered.

For the inspected fast path, the same saved round and same answer supply the same deliberation premises in A and B. There is no read of the changed API to discriminate them. Subject to successful ordinary kernel/storage operations, it reaches the same selected `ready` design with no stale-evidence qualifier. A proof identifier may differ between evaluations; that does not supply the missing source-validity distinction. This is a source-derived control-flow witness, **not an executed reproduction or a prediction of an actual model's initial choices**. Scripted role output would establish the boundary deterministically; a later model study would measure frequency and practical harm.

Useful negative controls:

- **No source change:** fast replay is desirable after a validity check. The remedy must not require a new model call for every unchanged answer.
- **Preference-only problem with no mutable factual dependency:** its answer can remain reusable under an explicit dependency-free contract. The model merely omitting citations is not proof of that condition.
- **Free-text answer:** it follows the refinement route instead of this exact-answer shortcut. That path does not establish freshness of exact-option replay and is outside the primary witness.
- **Normal recurse mission:** pre/post source and vision admission is a real separate control. Do not report it as bypassed by this chat witness.
- **Ordinary propose mode:** the authored fixture expects a hold before mutation after answer replay. Preserve that behavior; this report is not an approval bypass.
- **A changed but irrelevant file:** whole-workspace invalidation may redo unnecessary work. It is a precision/cost control for the proposed design, not evidence that relevant edits are safe to ignore.

## Why this is distinct from existing work

The duplicate screen inspected the current audit inventory/queue and the relevant reports, and searched public repository content for deliberation freshness, saved questions and answer predicates. Search results can lag main, so source claims here use the explicitly fetched pin. This is a bounded duplicate screen, not a claim about every historical branch or private discussion.

- **Recurse values continuation ownership** addresses which owner gets a stored question, including legacy records and missing owner-addressed resumption. This witness uses one correctly owned ordinary chat question. Owner identity is correct; evidence has aged. [Ownership audit][ownership-audit]
- **Deliberation read invocation identity** concerns omitted read arguments in a role's next request. Here the original assessment and citations may be completely accurate and fully identified; their source changes afterward. [Read audit][read-audit]
- **Deliberation consultation qualifier continuity** concerns discarded caveats/references at an immediate adapter handoff. This witness needs no specialist call or lossy response projection. [Qualifier audit][qualifier-audit]
- **Consultation cache snapshot** concerns completed specialist answer reuse and failure history. This is reuse of the saved proposer/challenger judgment through the human-answer shortcut, even with no consultation cache. [Consultation audit][consult-audit]
- **Portable recurse deliberation and crucible** already proposes evidence identities and revalidation on resume. This report supplies a concrete current consumer gap and an implementable acceptance boundary for that direction; it does not claim to invent source-bound evidence or add a competing agent framework. [Portable design][portable]

## Proposed bounded contract

The following is **proposed**, not implemented by this audit.

### Separate preference durability from evidence validity

Persist the answered preference as a durable constraint tied to the original thread, question/round and selected candidate semantics. Preserve the original restrictions. Independently retain a host-produced validity record for the factual support used by the round. Resuming should be able to say:

- **Current:** required evidence identities still match; exact-answer replay may reuse the saved assessment.
- **Stale:** a relevant dependency changed; retain the preference, but withhold current readiness pending a bounded refreshed challenge.
- **Unknown/unavailable:** identity was never recorded, its coverage is insufficient, or inspection failed; do not treat this as current.
- **Historical selection:** the preference was recorded, but the old design is being shown only as a historical artifact rather than an executable/current recommendation.

Host code measures workspace/source identities and availability; Mangle derives reuse, refinement or stop from those observations. The LLM can explain changed implications but cannot certify its own missing dependency inventory as complete or decide that inspection failure means unchanged. Keep design readiness, permission and implementation acceptance as separate judgments.

### Start conservatively; name the portability boundary

For local-workspace evidence, a conservative first contract can bind the round to the canonical workspace and a content/membership/configuration identity. Git HEAD alone is insufficient for dirty or untracked changes. The repository already has `evidence.Snapshot`, which includes admitted names, modes and contents plus specific project configuration while excluding runtime state. Evaluate that existing collector for reuse; do not introduce a second incompatible definition without an explicit reason. Its current non-regular-file rejection and membership policy are capability limits, not conditions to silently waive. [Existing snapshot][snapshot]

A digest only helps if it identifies what was actually observed. Record the observation interval and use host-supported coherent generations or conservative before/after validation around collection. A pair of equal directory hashes alone is not a universal proof that every intermediate read was coherent, especially under concurrent or change-and-restore activity. The primary witness avoids that problem; a correction must state its actual guarantee rather than claim transactional snapshots it does not provide.

Local source identity also does not cover changing MCP results, external documentation, specialist knowledge or other non-file evidence. Such dependencies need their own version/availability contract, a refresh, or an explicit unknown disposition. For unsupported repositories or unavailable identity mechanisms, retain useful historical design and the user's choice without asserting current readiness. This makes the protocol portable without pretending all evidence is a Git tree.

### Revalidate without restarting the interview

When evidence changes, carry the saved preference into refreshed proposal/challenge work as a constraint. Do not ask the same values question again merely because source changed. If the selected preference is now impossible, explain the new conflict or return an unresolved result under the existing one-question policy. A materially new trade-off is not automatically covered by the old answer.

Do not silently reset round/no-progress budgets or convert a refresh failure into success. A necessary refresh that cannot fit remaining capacity needs a named stopped/blocked outcome with the preference preserved. A future mission resume must additionally establish its owner, current mission/source/vision identity, budget and ordinary execution admission; a generic `ready` thread cannot substitute for those checks.

### Treat old records honestly

Legacy threads lack an observed source binding. Successful JSON decoding and matching session IDs are not a migration proof of freshness. Their preference can be kept, but their factual support is unknown until re-established. Migration must not invent an S0 digest by hashing the current workspace and attaching that value to historical observations.

## Costs, failure modes and research basis

An unchanged fast path with full-workspace hashing trades provider work for local I/O: approximately O(B + F) inspection over admitted bytes B and files F, plus a small persistent digest/metadata record. A dependency inventory can reduce repeat I/O, but costs storage and maintenance and requires complete enough dependency tracking to justify selective reuse. Frequent irrelevant edits can cause conservative false invalidation; measuring that rate matters. Refreshing a stale assessment may require the existing proposer/challenger pair, bounded read tools and consultations, all charged to the ordinary budget. No speedup or savings is claimed.

Record refresh attempts, identity failures, bytes inspected, latency, paid role/tool calls, total tokens/cost, stale-ready prevention and unnecessary refreshes. Count blocked and abandoned cases, not only successful continuations. Avoid unlimited retained snapshots or copying entire evidence bodies into each prompt; bounded references and explicit expired/unavailable states are preferable.

Mokhov, Mitchell and Peyton Jones distinguish scheduling from the decision to reuse or rebuild a result. Their verifying-trace design records dependency values/hashes and checks them before reusing a value. This provides a useful engineering analogy for separating a preference answer from factual-validity admission; it is not a proof that an LLM assessment becomes correct when hashes match. [Build Systems à la Carte, §4.2.2](https://www.microsoft.com/en-us/research/wp-content/uploads/2018/03/build-systems-final.pdf)

W3C PROV-DM distinguishes entities, activities, use and derivation, including revision relationships. It supplies vocabulary for associating a judgment with the specific observations from which it was derived, rather than only a live path or conversation ID. It does not itself define a cache invalidation algorithm or certify the truth of a judgment. [PROV-DM](https://www.w3.org/TR/prov-dm/)

Key failure modes for the proposal are false-current results from incomplete dependency coverage, excessive invalidation from coarse snapshots, missing/denied reads mistaken for equality, applying the right digest to the wrong workspace, assigning current hashes to legacy evidence, losing the user's preference during refresh, crossing an existing budget to refresh, and treating a fresh proof recorder generation as fresh source evidence. All need explicit outcomes; none is resolved by simply requesting another unconstrained model opinion.

## Fair evaluation plan: entirely unrun

First use existing scripted-role seams and the production kernel, then exercise the normal chat/session route with world-model and CodeDOM context enabled. Do not use a live model merely to reproduce a deterministic missing comparison. Observe the persisted validity record, actual reads, asserted facts, resulting proof/status/note, worker launch count and permission disposition.

| Fixture | Required outcome under the proposed contract |
|---|---|
| Exact option; dependencies unchanged; restart | Correct preference resumes with bounded identity work and no unnecessary role call. |
| Relevant API changed while pending | Old success facts cannot alone produce current `ready`; preference survives. |
| Dirty edit with unchanged Git HEAD | Relevant change is detected. |
| Relevant file deleted/renamed or membership changed | Explicit stale/unavailable result, not equality by absence. |
| Wrong canonical workspace, same relative paths | No cross-workspace reuse. |
| Relevant project configuration changed | Currentness respects the documented configuration domain. |
| Unrelated source edit | Conservative refresh is truthful; report its cost and test any justified selective-reuse strategy separately. |
| Legacy thread with no binding | Unknown factual validity; do not manufacture a historical binding. |
| Identity read denied, cancelled or failed | No current-ready result; saved answer is recoverable. |
| External evidence changed while local source stayed fixed | Local equality cannot certify that external dependency. |
| Concurrent change during reacquisition | The declared coherence/uncertainty contract holds; no unsupported atomicity claim. |
| Selected preference now impossible | New factual/design conflict is reported without silently changing the preference. |
| Budget exhausted before refresh | Named stop with retained preference; no budget reset or fabricated success. |
| Design-only and ordinary held work | No development worker for the former; normal work approval remains required for the latter. |
| Mission-owned question | Existing ownership isolation and separate mission freshness/admission remain intact. |

The existing values fixture explicitly expects saved-round replay with no additional model call and checks that propose-mode mutation is held. Keep those as unchanged-evidence controls. Add the changed-evidence twin rather than replacing the fast path with unconditional recomputation. The authored fixture was inspected, not run. [Existing fixture][values-test]

Only after deterministic boundaries are established, compare (A) current exact-answer replay, (B) conservative identity admission and bounded refresh, and optionally (C) dependency-aware admission on a frozen, repository-disjoint set of paused design tasks. Include Go, Python, JS/TS, Rust, mixed repositories and unsupported tooling, because the first contract is about evidence identity rather than language-specific test success. Vary relevant, irrelevant and external changes independently. Hold request, permitted effects, model/provider, acceptance rubric and full-cost limits constant where possible.

Blind evaluation of the design's current factual applicability should be separate from preference fidelity and from implementation acceptance. Report false-current readiness, unnecessary refresh, unresolved fraction, preference loss, owner-question burden and full cost/latency. Include all failed, cancelled and budget-limited continuations. Cluster uncertainty by repository/task family; do not treat repeated seeds as independent repositories. A cost-negative or over-conservative result is valid evidence for revising the design, not a reason to exclude those cases.

## Acceptance and adoption

Update the owning session/deliberation specification first: an exact answer resolves a preference but renews no factual observation by itself. Add a named validity/reuse contract and legacy disposition, then implement through the existing host-measurement/Mangle-decision boundary. Preserve the read-only roles, one-question behavior, normal permissions, mission-owner isolation, mission snapshot checks and independent verification ratchet.

The checkable exit is narrow: when a relevant dependency changes during a legitimate values pause, the normal resume path retains the user's answer but cannot label the old design current-ready solely by replaying saved success facts. With unchanged admitted evidence, the inexpensive replay path remains available. This closes one concrete continuation gap; it does not establish general behavioral correctness or universal repository portability.

## Pinned source references

[pin]: https://github.com/theRebelliousNerd/codenerd/commit/a4981591c95307eab5df20f82c07132b79ef9375
[guidance]: https://github.com/theRebelliousNerd/codenerd/blob/a4981591c95307eab5df20f82c07132b79ef9375/agents.md#L17-L24
[session]: https://github.com/theRebelliousNerd/codenerd/blob/a4981591c95307eab5df20f82c07132b79ef9375/Docs/architecture/session/IMPLEMENTED_SPEC.md#L816-L895
[interrogator]: https://github.com/theRebelliousNerd/codenerd/blob/a4981591c95307eab5df20f82c07132b79ef9375/internal/prompt/atoms/system/requirements_interrogator.yaml#L14-L28
[thread]: https://github.com/theRebelliousNerd/codenerd/blob/a4981591c95307eab5df20f82c07132b79ef9375/internal/deliberation/types.go#L59-L101
[validation]: https://github.com/theRebelliousNerd/codenerd/blob/a4981591c95307eab5df20f82c07132b79ef9375/internal/deliberation/types.go#L144-L181
[round]: https://github.com/theRebelliousNerd/codenerd/blob/a4981591c95307eab5df20f82c07132b79ef9375/internal/deliberation/round.go#L10-L75
[projection]: https://github.com/theRebelliousNerd/codenerd/blob/a4981591c95307eab5df20f82c07132b79ef9375/internal/deliberation/policy.go#L10-L40
[resume]: https://github.com/theRebelliousNerd/codenerd/blob/a4981591c95307eab5df20f82c07132b79ef9375/internal/deliberation/service.go#L28-L100
[policy]: https://github.com/theRebelliousNerd/codenerd/blob/a4981591c95307eab5df20f82c07132b79ef9375/internal/core/defaults/policy/deliberation.mg#L18-L58
[proof]: https://github.com/theRebelliousNerd/codenerd/blob/a4981591c95307eab5df20f82c07132b79ef9375/internal/deliberation/policy.go#L42-L72
[outcome]: https://github.com/theRebelliousNerd/codenerd/blob/a4981591c95307eab5df20f82c07132b79ef9375/internal/deliberation/outcome.go#L16-L115
[persistence]: https://github.com/theRebelliousNerd/codenerd/blob/a4981591c95307eab5df20f82c07132b79ef9375/internal/deliberation/persistence.go#L24-L108
[owners]: https://github.com/theRebelliousNerd/codenerd/blob/a4981591c95307eab5df20f82c07132b79ef9375/internal/deliberation/persistence.go#L77-L173
[chat]: https://github.com/theRebelliousNerd/codenerd/blob/a4981591c95307eab5df20f82c07132b79ef9375/cmd/nerd/chat/deliberation.go#L53-L119
[process]: https://github.com/theRebelliousNerd/codenerd/blob/a4981591c95307eab5df20f82c07132b79ef9375/cmd/nerd/chat/process.go#L363-L419
[values-test]: https://github.com/theRebelliousNerd/codenerd/blob/a4981591c95307eab5df20f82c07132b79ef9375/cmd/nerd/chat/deliberation_test.go#L179-L219
[mission]: https://github.com/theRebelliousNerd/codenerd/blob/a4981591c95307eab5df20f82c07132b79ef9375/internal/campaign/recurse_mission.go#L212-L318
[snapshot]: https://github.com/theRebelliousNerd/codenerd/blob/a4981591c95307eab5df20f82c07132b79ef9375/internal/evidence/change.go#L169-L244
[ownership-audit]: https://github.com/theRebelliousNerd/codenerd/blob/a4981591c95307eab5df20f82c07132b79ef9375/Docs/audits/recurse-values-continuation-ownership.md
[read-audit]: https://github.com/theRebelliousNerd/codenerd/blob/a4981591c95307eab5df20f82c07132b79ef9375/Docs/audits/deliberation-read-invocation-identity.md
[qualifier-audit]: https://github.com/theRebelliousNerd/codenerd/blob/a4981591c95307eab5df20f82c07132b79ef9375/Docs/audits/deliberation-consultation-qualifier-continuity.md
[consult-audit]: https://github.com/theRebelliousNerd/codenerd/blob/a4981591c95307eab5df20f82c07132b79ef9375/Docs/audits/consultation-cache-snapshot.md
[portable]: https://github.com/theRebelliousNerd/codenerd/blob/a4981591c95307eab5df20f82c07132b79ef9375/Docs/audits/portable-recurse-deliberation-crucible.md
