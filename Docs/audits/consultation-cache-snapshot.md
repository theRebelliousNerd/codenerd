---
doc-class: deep-dive
subsystem: expert-consultation-cache
implementation-status: partial
last-verified: 2026-10-30
verified-against: fef98230c20a5751e4c32a2fd2757832a9d39cdc
verification-method: static-source-inspection
---

# Expert-consultation cache: evidence snapshot freshness

Current disposition: both defects were still present at the inspected base
above. The working-tree mitigation removes completed-answer reuse and makes
failure history depend on complete file observations. Implementation and tests
are authored; execution and observed fail-before/pass-after evidence remain
pending under this task's author-only restriction. The original source witness
below is retained as historical analysis; current rulings and design are in
“As built.”

Static source inspection of public `theRebelliousNerd/codenerd` at `d1a1b5b8de3f11bb59e4f3eeef56a467d36a7bd2` (2026-10-30). No tests, codeNERD runs, model inference or benchmarks were performed. This is a conditional source witness, not an observed bad model result.

## Conclusion

The new session consultation ladder has real freshness checks, but its stored digest map is **a post-consultation filesystem sample, not a receipt for evidence used to produce the answer**. It also silently omits files it cannot read; policy only checks files present in that stored map. Two distinct paths can therefore qualify an answer for `/reuse` without establishing the claimed “files unchanged since” condition.

This is narrower than “all cached consultation is unsafe.” Unchanged readable files reuse as designed; a subsequent change or disappearance of a previously digested file invalidates it. The issue is evidence binding and completeness at cache publication.

## Guidance and prior work

[Repository guidance](https://github.com/theRebelliousNerd/codenerd/blob/d1a1b5b8de3f11bb59e4f3eeef56a467d36a7bd2/agents.md) says stale evidence must not survive source changes as current truth, Mangle owns executive decisions, normal production paths with CodeDOM/world context are the principal evaluation, and older generated architecture prose is not implementation evidence. [Session guidance](https://github.com/theRebelliousNerd/codenerd/blob/d1a1b5b8de3f11bb59e4f3eeef56a467d36a7bd2/internal/session/README.md) preserves exact permission and capability boundaries.

The [coordination and safe-reuse audit](https://github.com/theRebelliousNerd/codenerd/blob/d1a1b5b8de3f11bb59e4f3eeef56a467d36a7bd2/Docs/audits/consultation-coordination-and-safe-reuse.md) explicitly declines completed-advice caching without an evidence-revision source. That statement still accurately describes **another mechanism**, [internal/consultation.Manager](https://github.com/theRebelliousNerd/codenerd/blob/d1a1b5b8de3f11bb59e4f3eeef56a467d36a7bd2/internal/consultation/manager.go): it coalesces in-flight work, deletes the flight on publication and retains no completed artifact. The new `session.consultCache` is separate and does retain completed expert cards. Do not conflate their behavior.

Earlier [scope](https://github.com/theRebelliousNerd/codenerd/blob/d1a1b5b8de3f11bb59e4f3eeef56a467d36a7bd2/Docs/audits/expert-card-scope-continuity.md) and [repair continuity](https://github.com/theRebelliousNerd/codenerd/blob/d1a1b5b8de3f11bb59e4f3eeef56a467d36a7bd2/Docs/audits/expert-card-repair-continuity.md) audits inspect older `486db57...` source. Current code has scoped supplements and repair dealing; those old transport findings are not evidence that the corresponding old gaps persist. Freshness is independent of those improvements.

## Producer → publication → reuse → model

1. [Executor 1588–1605](https://github.com/theRebelliousNerd/codenerd/blob/d1a1b5b8de3f11bb59e4f3eeef56a467d36a7bd2/internal/session/executor.go#L1588-L1605) assembles JIT/project/file context, invokes `withExpertConsultations`, then passes the returned system prompt to `runToolLoop`. [Step compilation 823–847](https://github.com/theRebelliousNerd/codenerd/blob/d1a1b5b8de3f11bb59e4f3eeef56a467d36a7bd2/internal/session/work_steps.go#L823-L847) independently selects and compiles experts, deals old cards, then invokes the same helper.
2. [expert_spawn.go 76–96](https://github.com/theRebelliousNerd/codenerd/blob/d1a1b5b8de3f11bb59e4f3eeef56a467d36a7bd2/internal/session/expert_spawn.go#L76-L96) narrows a need to scope not already consulted within this result, then asks `routeConsultations`. The cache is allocated per executor and shared by task clones ([executor.go 483](https://github.com/theRebelliousNerd/codenerd/blob/d1a1b5b8de3f11bb59e4f3eeef56a467d36a7bd2/internal/session/executor.go#L483), [739–743](https://github.com/theRebelliousNerd/codenerd/blob/d1a1b5b8de3f11bb59e4f3eeef56a467d36a7bd2/internal/session/executor.go#L739-L743)). A fresh ExecutionResult does not itself clear it.
3. [consult_route.go 83–102](https://github.com/theRebelliousNerd/codenerd/blob/d1a1b5b8de3f11bb59e4f3eeef56a467d36a7bd2/internal/session/consult_route.go#L83-L102) keys by normalized agent, spawn mode, verb, sorted file names and trimmed-input hash. `fileDigests` reads each named file, hashes bytes, and silently skips every read error. At routing, current digest facts and cached digest facts are asserted ([135–170](https://github.com/theRebelliousNerd/codenerd/blob/d1a1b5b8de3f11bb59e4f3eeef56a467d36a7bd2/internal/session/consult_route.go#L135-L170)); these pre-call measurements are not retained in consultPlan.
4. [expert_spawn.go 99–126](https://github.com/theRebelliousNerd/codenerd/blob/d1a1b5b8de3f11bb59e4f3eeef56a467d36a7bd2/internal/session/expert_spawn.go#L99-L126) invokes each ordinary/escalated consultation concurrently and waits for all. Production [factory.go 2752–2756](https://github.com/theRebelliousNerd/codenerd/blob/d1a1b5b8de3f11bb59e4f3eeef56a467d36a7bd2/internal/system/factory.go#L2752-L2756) wires JITExecutor. [task_executor.go 485–493](https://github.com/theRebelliousNerd/codenerd/blob/d1a1b5b8de3f11bb59e4f3eeef56a467d36a7bd2/internal/session/task_executor.go#L485-L493) runs `/consult/<agent>` via `RunUntilVerified` and returns the actual answer.
5. Only **after** the batch wait does [expert_spawn.go 136–150](https://github.com/theRebelliousNerd/codenerd/blob/d1a1b5b8de3f11bb59e4f3eeef56a467d36a7bd2/internal/session/expert_spawn.go#L136-L150) create the card, parse handoffs/mandates, and call `fileDigests` again to populate cachedCard. There is no comparison between pre- and post-call readings and no binding to the bytes the consultant actually consumed. Waiting for slower siblings broadens the interval even after a particular answer is complete.
6. [shard_experts.mg 239–261](https://github.com/theRebelliousNerd/codenerd/blob/d1a1b5b8de3f11bb59e4f3eeef56a467d36a7bd2/internal/core/defaults/policy/shard_experts.mg#L239-L261) marks a card stale when a **stored** file digest differs from the current one, or a **stored** file no longer has a current digest. An answered card with no derived stale fact is usable and routes to reuse. Missing stored coverage has no corresponding stale rule.
7. On reuse [expert_spawn.go 125–127](https://github.com/theRebelliousNerd/codenerd/blob/d1a1b5b8de3f11bb59e4f3eeef56a467d36a7bd2/internal/session/expert_spawn.go#L125-L127) copies the old card and sets its Route. [expert_cards.go 153–162](https://github.com/theRebelliousNerd/codenerd/blob/d1a1b5b8de3f11bb59e4f3eeef56a467d36a7bd2/internal/session/expert_cards.go#L153-L162) adds “reused: asked before on the same request, files unchanged since”; the answer body is appended and recorded. This is model-facing evidence, not merely a log/cache statistic.

## Two separate conditional witnesses

### A. Post-hoc snapshot binds an old answer to new bytes

A named readable file starts at revision A. The consultant receives/reads A and derives an answer specific to A. While that computation (or a slower sibling) is pending, an authorized external writer or parallel task changes the file to B. Publication samples B. A later request with the same cache key, still at B, compares B to B and receives the A-specific answer labeled reused/unchanged.

The normal sequential executor does not begin its own post-consultation edits before this wait; the witness requires an intervening writer or equivalent evidence revision change. The source establishes possible misbinding, not that every consultation races or even reads the named file. The initial current-digest sample does not protect publication because it is used for routing facts only.

### B. Missing digest coverage is treated as no counterexample

At publication, a selected file is missing or unreadable, so no map entry is stored. If a nonempty answer was returned, later creation/readability/change of that same named file produces a current digest but no old digest to compare. Other stored files may remain unchanged, leaving the card usable. The degenerate empty-map case has no file-based invalidation witness at all.

A particularly clear conditional case is a consultant that read A successfully, followed by deletion before publication: its answer can be stored with no digest for A and later reused after a different A is created. This is distinct from **deletion after successful publication**, which policy correctly detects through the cached digest and missing current measurement.

## Visibility and authority limits

The consultation prompt names the task, target and domain files; it does not embed a revision-stamped snapshot ([consultationTask](https://github.com/theRebelliousNerd/codenerd/blob/d1a1b5b8de3f11bb59e4f3eeef56a467d36a7bd2/internal/session/expert_spawn.go#L221)). Source alone does not establish which file bytes a real specialist read through CodeDOM/world context/tools, whether it later reread them, or which assertions depended on them. A mock returning an A-specific marker proves routing/transport only, not real evidence consumption. A production evaluation must capture the actual specialist requests/tool observations and returned answer.

Existing JIT knowledge, current target context, the executing model's own reads, and normal verification can compensate. [Expert critique](https://github.com/theRebelliousNerd/codenerd/blob/d1a1b5b8de3f11bb59e4f3eeef56a467d36a7bd2/internal/session/expert_critique_round.go) reads current changed-file views and calls the consultant separately; it does not use this pre-edit route cache. It is advisory and conditional, so not a universal freshness certificate. Permission, capability, write-set, mandate admission and verification gates remain separate. A cache hit is neither authorization nor proof of task correctness.

## Repair direction, not an implemented fix

Make cache eligibility depend on complete evidence provenance. At minimum, retain pre-call file measurements, record read failures explicitly, compare complete expected coverage and before/after digests, and decline completed reuse when coverage is unknown or changed. Do not silently treat an unreadable file as an empty dependency set. Fileless tasks need an explicit policy rather than accidental vacuous validity.

Before/after equality is only a conservative improvement: it cannot exclude A→B→A changes or prove which bytes a model saw. A stronger design attaches a versioned snapshot/read receipt to the consultation's evidence and checks that revision at reuse. Preserve Mangle as the decision maker: Go emits measured revisions/completeness and policy derives eligible reuse, reread, reconsult or visible unresolved status. Do not widen tool or safety permissions to accomplish it.

## Fair evaluation (proposed; not run)

Start with deterministic production-kernel route tests and a controllable consultant. Assert the actual returned prompt, old/new answer markers, route, call counts and recorded digest coverage. Include unchanged readable hit; change after publication; deletion after publication; mutation during an in-flight answer; mutation after one answer but before a sibling finishes; missing-at-publication then created; unreadable-at-publication then readable (portable injected read failure rather than root-sensitive chmod); mixed complete/incomplete file sets; empty scope; failures and escalation; a fresh result on a shared task clone. Separate same-turn scope suppression from cross-result cache reuse.

Then test through normal production boot, expert selection, JIT and model-request assembly with CodeDOM/world-model context enabled. Observe that the consultant actually consumed revision A, arrange an authorized revision-B transition at a known boundary, and capture whether the reused body and freshness claim reach the later executing-model request. Preserve normal knowledge and critique channels; restricted mocks are supplementary diagnostic evidence.

Compare current behavior, completed-reuse disabled, and the chosen provenance-aware design on matched tasks/models/budgets. Score freshness delivery separately from final quality. Account for all specialist and executor tokens, JIT context, filesystem hashing, cache memory, retries, escalation, unresolved outcomes, critique, repairs, verification and end-to-end wall-clock latency. Report total cost per verified completed task, not avoided consultation calls alone. No savings, superiority or downstream regression is established by this static audit.

## As built

Commits: none created in this author-only task. Source inspection used base
`fef98230c20a5751e4c32a2fd2757832a9d39cdc`; changes are in the working tree.

| Finding or qualification | Ruling against the inspected base | Disposition |
|---|---|---|
| A: an old answer can acquire new post-batch digests, including during a sibling wait | Real: routing discarded its initial samples and `expert_spawn.go` sampled again after `wg.Wait`. | Removed completed-answer retention and `/reuse`, including its model-facing unchanged claim. |
| B: failed reads remove dependencies from stored coverage | Real: `fileDigests` skipped errors; policy only iterated stored digests. | Every expected file has a retained read outcome; Mangle compares complete before/after/current coverage for failure history. Fileless scopes cannot vacuously suppress calls. |
| Manager versus session cache | Already correct: `internal/consultation/manager.go:93` deletes the in-flight entry on publication and retains no completed artifact. | No Manager change. Its request receipt contains no evidence provenance. |
| Unchanged readable reuse and invalidation after a successful stored digest changes or disappears | Correct description of the former policy; this does not repair A or B. | Change/deletion-after-publication remain regression controls; unchanged answered requests now consult again. |
| Executor/step callers, task clones, JITExecutor wiring, and model-facing delivery | Confirmed: `internal/session/executor.go:1590`, `work_steps.go:846`, `task_executor.go:485`, `internal/system/factory.go:2760`. | Both callers use the revised helper; clones share only attempt history (`executor.go:742`). |
| Scoped supplements, repair continuity, critique and independent authorization/verification limits | Confirmed in `expert_spawn.go:160`, `expert_cards.go`, and `expert_critique_round.go`. | Within-turn coverage and the existing gates remain separate from cross-result routing. No claim of a live bad model result or model quality improvement is made. |
| Before/after equality cannot prove evidence identity or exclude A→B→A | Correct. `ExpertConsultant` returns text only; the specialist can also consume CodeDOM and knowledge context. | Chosen design declines completed reuse rather than presenting endpoint hashes as a receipt. No finding was ruled wrong. |

Implementation: `internal/session/consult_route.go` now retains attempt outcomes
without answer bodies, including read errors. `expert_spawn.go` retains pre-call
measurements and measures each return before the sibling wait; new ordinary
attempts reset prior escalation. `expert_cards.go` removes the unchanged/reused
claim. `internal/core/defaults/schemas_shard_experts.mg` and
`policy/shard_experts.mg` declare and judge expected scope and typed observations;
`internal/shards/registration.go:545` homes the whole routing family in cortex.
The contract, current state and validation gap are in
`Docs/architecture/session/{IMPLEMENTED_SPEC,02-CURRENT-STATE,03-GAP-ANALYSIS}.md`.
No embedded prompt atoms or baseline files were changed.

Authored tests: `internal/session/consult_snapshot_test.go` covers in-call and
sibling-wait mutation, A→B→A, missing/unreadable/mixed/fileless dependencies,
explicit coverage facts and cleanup, escalation restart, shared clones and
same-result suppression. `consult_route_test.go` replaces the old unchanged-hit
expectation and retains knowledge/escalation controls.
`internal/system/consult_snapshot_boot_test.go` captures model requests through
real boot, selection and JIT with world/CodeDOM providers; a controlled consultant
reads A, writes B before publication, and must be called again for B. This is
transport evidence to be executed, not a live-specialist evaluation.

Fail-before rationale (static, not observed): the old path reuses the first
answer for unchanged, in-call, sibling-wait, missing/unreadable, mixed, fileless
and A→B→A cases, so fresh-call/new-answer assertions should fail. The old failure
cache escalates incomplete scopes and carries an escalated flag across stale
ordinary retries. Change/deletion after publication, same-result coverage,
knowledge delivery and stable failure escalation are preservation controls.

Remaining: execute the authored tests and focused race checks, the defaults
starved/unconsumed/literal budgets, the shard-join audit, and the full suite in
the root session. The Mangle lab/engine checks and live specialist/model cost
evaluation were not run; no engine verdict or empirical savings is claimed.
Only Go formatting and `git diff --check` have been performed. A repository-local
private-terms scanner was not found. The partial status records these pending
validation obligations; complete evidence receipts are intentionally outside
the chosen no-completed-reuse contract.

