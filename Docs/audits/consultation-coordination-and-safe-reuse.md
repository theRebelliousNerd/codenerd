---
doc-class: deep-dive
subsystem: shards
implementation-status: planned
last-verified: 2026-10-27
verified-against: e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a
supersedes: []
---

# Consultation coordination: trustworthy reuse, lossless partial outcomes, and bounded advisory work

Status: source-grounded proposal; no implementation, tests, model calls, or benchmarks were run for this document. Source baseline: `e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a` (2026-10-27 inspection). Scope: specialist consultation and campaign advisory handoffs, not task acceptance or general retrieval.

## Required outcome

CodeNERD should spend on a specialist only when the specialist can add relevant evidence, reuse advice only for the same computational question and evidence revision, and preserve useful findings even when another specialist fails. A consultation result is evidence for the executive, not permission to act or proof of correctness.

This extends the existing shards cache-ownership and lifecycle residuals rather than replacing the architecture corpus. [Architecture guidance](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/Docs/architecture/AGENTS.md) requires JIT-first cognition, Mangle-owned executive decisions, revision-aware recoverable context, and exact default-deny authorization. [Project guidance](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/nerd.md) likewise places prompts in atoms and policy in the kernel. Proposed fields and predicates below are design contracts, not shipped APIs.

## What exists, and what does not yet follow from it

| Boundary | Inspected behavior | Consequence / remaining gap |
|---|---|---|
| Single consultation | `RequestConsultation` renders a question and context, spawns a specialist, parses advice, and caches it | Real execution path, not an unwired class; nil spawner fails visibly |
| Batch consultation | One goroutine per requested specialist; successful responses returned in input order alongside joined failures | Partial outcomes already work at this layer; do not propose their first implementation |
| Cache | Key hashes specialist, complete question, and context; five-minute freshness and 100-entry capacity | Full-question/context collision repair exists; identity, ownership, revision and concurrent-miss handling remain |
| Chat campaign adapter | Converts successful responses even when an error accompanies them | Adapter does not discard partial success |
| Alternate system campaign-runner provider | Logs per-specialist spawn errors, drops failed responses, and returns successes with `nil` error | Failure honesty differs by production entry path; successful subset can be treated as a complete panel |
| Advisory board | `ConsultAdvisors` returns `nil,error` on any batch error | Successful findings disappear at the next boundary |
| Risk-gate caller | On advisory error emits generic unapproved concern; otherwise synthesizes specific concerns | Preserving a successful rejection matters independently of whether the batch is incomplete |
| Advisory input | Request contains campaign title as `RawPlan`, plus separate phase summaries, task counts, and paths | A review here is not demonstrated review of the complete task/dependency/write-set graph |

Evidence: [consultation implementation](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/internal/shards/consultation.go#L99-L202), [chat adapter](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/cmd/nerd/chat/campaign_consultation_adapter.go#L25-L63), [advisory board](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/internal/campaign/shard_advisory_board.go#L150-L197), [risk-gate caller](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/internal/campaign/risk_scoring.go#L779-L852). The [existing gap analysis](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/Docs/architecture/shards/03-GAP-ANALYSIS.md) already identifies cache identity/copy, JIT boundaries, and lifecycle receipts as residuals. Its reported historical test passes are not newly executed evidence here.

Do not reopen fixed neighboring gaps: [campaign returns](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/internal/campaign/orchestrator_task_results.go#L36-L77) already use structured observation projections rather than blind result-head truncation. [Attempt write ownership](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/internal/campaign/orchestrator_attempt_writes.go#L116-L130) already adds dynamic leases, with [failed-attempt rollback](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/internal/campaign/orchestrator_attempt_writes.go#L138-L163). Older comments elsewhere are not sufficient evidence those mechanisms are absent.

## 1. Separate reusable content from request identity

The current cache key omits `FromSpec`, even though `buildConsultationPrompt` renders it. A second caller can receive the first caller's pointer, including the first `RequestID` and `ToSpec`. The returned response contains mutable slices and a metadata map; returning that stored pointer means caller mutation can affect future readers. This is a source-level contract defect, not a measured production incident. The normal batch path uses `FromSpec=system`, narrowing—but not eliminating—the identity issue. See [prompt and response construction](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/internal/shards/consultation.go#L238-L335) and [cache](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/internal/shards/consultation.go#L379-L432).

Proposed immutable reusable object:

`AdviceArtifact = {content_id, specialist_profile_revision, prompt_revision, model_config_id, evidence_manifest, advice, references, caveats, parse_status, confidence_claim, created_at}`.

Proposed per-consumer envelope:

`ConsultationReceipt = {request_id, requester, artifact_id, status, reuse_kind, original_compute_receipt, delivered_at, missing_evidence}`.

Use a canonical, length-delimited encoding and a full digest:

`K = H(schema_version, scope_id, specialist_revision, prompt_revision, model_config_id, requester_semantics, question, evidence_revision, context)`.

`scope_id` prevents unintended cross-workspace/user sharing. Exclude purely administrative request IDs; include requester identity while it affects the prompt. Alternatively remove requester-dependent semantics explicitly before claiming cross-requester equivalence. Evidence revisions must include relevant working-tree content, not merely HEAD: an uncommitted edit can invalidate advice. If the evidence universe cannot be determined, use a conservative workspace generation or disable reuse. TTL bounds retention; it does not establish semantic freshness.

On a hit, validate the evidence generation and authorization scope, copy the artifact's mutable fields or return immutable access, and create a fresh receipt. Never report the historical duration as new inference cost. Preserve both original compute attribution and reuse attribution.

## 2. Coalesce identical work without coupling cancellation

Current cache lookup and spawn are separate; simultaneous identical misses can both invoke the specialist. Batch construction also does not deduplicate duplicate specialist entries. These are opportunities for reducing redundant work, not established savings.

Proposed sequence:

1. Validate request and acquire its policy-derived budget/eligibility decision before spawning.
2. Compute the semantic key and check a valid completed artifact.
3. Atomically join or create an in-flight record keyed by that key. Bound unique active work, queued entries, and followers per entry.
4. Execute the producer under its own bounded context and policy-authorized scope; followers wait under their own cancellation contexts.
5. A cancelled follower detaches without cancelling siblings. When no authorized consumers remain, cancel the producer and join cleanup. Do not let the first request's short deadline own every follower's lifetime.
6. Publish only a validated immutable artifact, then generate one terminal receipt per consumer. Failure is delivered honestly; negative caching, if allowed, has an explicit short TTL and never becomes an approval artifact.

The Go project's [singleflight documentation](https://pkg.go.dev/golang.org/x/sync/singleflight) supplies duplicate-call suppression as an established mechanism. It does not by itself provide evidence-version identity, authorization scoping, consumer-specific receipts, or the cancellation/retention contract above. Those are necessary integration work, not a reason to replace the existing queue wholesale.

The manager's `defaultTimeout` and request `Priority` fields are present, but `RequestConsultation` does not apply them in the inspected implementation. The advisory wrapper supplies a batch deadline of configured timeout multiplied by advisor count; downstream spawners may impose other limits. Therefore document effective deadlines end to end rather than claiming that consultation has no timeout anywhere. Wire priority into the existing executive/queue only after defining starvation behavior; do not create a competing Go scheduling authority.

## 3. Preserve partial evidence and parse explicit decisions

Return a typed batch envelope with one entry per requested specialist: `{specialist, status, advice_artifact?, error_class?, retryable?, attempt_id}`. Missing, failed, abstaining, and rejecting are different states. Conversion must not erase successes when an error occurs. The risk boundary should receive both the incomplete-review fact and every successful substantive concern, then let existing policy classify them. This does not mean an incomplete panel may automatically approve. It means a known rejection cannot be reduced to a generic missing-review message by another advisor's transport failure.

There are two distinct defects to close, not one universal manager defect. The chat adapter retains errors and successes, then the board drops successes on error. Conversely, [`campaignRunnerConsultationManager.RequestBatchConsultation`](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/internal/shards/system/campaign_runner.go#L466-L537) logs a spawn error and returns from that worker, then returns `responses,nil` after sorting. Its comment mentions joined failures, but its code does not join them. [System campaign-runner construction](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/internal/shards/system/campaign_runner.go#L340-L365) supplies that provider to intelligence gathering and the advisory board. This is a statically traced alternate entry path, not a runtime reproduction or a claim every campaign uses it. Total failure still reaches the board's zero-response fail-closed check; partial failure is more subtle because a successful subset can proceed to vote synthesis without a missing-review error. Preserve the successful entries and aggregate failed entries on both paths.

For an unchanged plan/evidence revision, retry only failed retryable entries. Carry forward successful entries by artifact identity. A revision change invalidates affected entries; do not merge votes across incompatible plans. This avoids a whole-panel repeat solely because one member timed out, without manufacturing consensus.

The current [advisory parser](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/internal/campaign/shard_advisory_board.go#L387-L459) scans the entire advice for words including `reject`, then falls back to approval based on confidence if it finds no vote. Thus text such as an explicit approval explaining why a rejected alternative was avoided can be interpreted as rejection; missing votes can become approval. The generic consultation prompt also asks for ADVICE sections while the embedded advisory question asks for VOTE sections.

Propose one versioned JIT response contract: exact vote enum, findings with evidence references, explicit unknowns, and an optional self-reported confidence field. Parse the vote field only, reject contradictory duplicate fields, and mark absent/invalid votes unknown—not approved. Treat confidence as a model claim, not empirical calibration. The kernel decides the effect of unknown votes and required-role absence. Before changing `RequireUnanimous`/`RequireCriticalApproval` semantics, specify them: current `determineApproval` checks rejections and blocking concerns, not an explicit positive vote from every required advisor. A policy migration needs negative controls rather than silently tightening or loosening production behavior.

## 4. Make advisory coverage explicit before adding more advisors

The current caller supplies a title as `RawPlan`. More agents reviewing the same incomplete representation cannot repair missing dependency or mutation information. Construct a bounded plan manifest with task IDs, objective references, dependency edges, intended read/write sets, acceptance obligations, and evidence handles. Define which obligations each advisor must inspect, and return `coverage={examined, unavailable, not_assigned}`. Retrieve large details on demand through existing bounded observation interfaces instead of duplicating the full plan into every prompt.

This is a proposal to improve review input, not a claim the present summary is always inadequate. Evaluate summary-only versus manifest-plus-retrieval, including retrieval costs and failures. Mangle chooses required coverage and admissible reviewers; a model may propose expertise or findings but does not grant itself approval authority.

## Cost model and fair evaluation

For request cohort R, count all producers, failed attempts, coordinator calls, refresh reads, projection calls, tools, and downstream retries:

`T_total = sum_calls(input_uncached + input_cached + output) + downstream_rework_tokens`.

Report cached input separately for billing and compute interpretation; do not equate token count with money. Also report dollars, wall-clock p50/p95, CPU hashing/serialization time, peak artifact bytes, pending-entry count, tool latency, and cancellation cleanup latency. A cache hit can reduce model tokens while increasing stale-advice risk; coalescing can reduce duplication while increasing wait time. For n identical misses of cost C, the ideal avoided producer work is `(n-1)C`, but validation, storage, copying, follower receipts, cancellation, and unsuccessful reuse reduce that ideal. This is an accounting bound, not a measured speedup.

Evaluate whole campaigns at matched models, tools, evidence access, task distributions, retry caps, and total budget. Compare: current implementation; no reuse; identity-safe cache; identity-safe cache plus coalescing; typed partial-result retry; and adaptive bounded consultation. Include one strong single-agent baseline and fixed-panel baseline. A cheaper failed campaign is not an efficiency win. Primary outcomes are independently verified task success, false approval/rejection, stale-evidence use, successful findings retained under partial failure, and total cost per verified success.

For the local consultation boundary, use deterministic spawner fixtures to isolate semantics; for complete-system evaluation, use held-out repository tasks with independent outcome checking. Proposed fixture matrix: same question/different requester; mutation of returned maps/slices; uncommitted evidence change within TTL; concurrent identical misses; duplicate specialists; one follower cancelled; all followers cancelled; nil spawner; one successful rejection plus one timeout; malformed/no vote; approval prose mentioning rejection; missing critical advisor; plan change between retries; required evidence beyond bounded manifest. None were executed here.

Scale specialist requests across 1, 2, 4, 8, 16, and 64, varying unique-key fraction and dependency coupling. Count queueing and provider throttling, not only model time. [Kim et al., Towards a Science of Scaling Agent Systems, v3](https://arxiv.org/abs/2512.08296v3) finds coordination effectiveness depends on task structure under controlled comparisons; it motivates matched-budget topology ablations, not a transferable CodeNERD performance guarantee.

## Failure modes and order of work

First fix identity/copy and typed terminal semantics; then add coalescing, bounded queues, and selective retries; finally evaluate advisor selection. Optimizing before defining equivalence amplifies wrong reuse. Over-broad revision keys waste compute; under-broad keys reuse stale advice. Coalescing across authorization scopes leaks information. All-consumer cancellation without cleanup retains work. Caching self-reported confidence amplifies an uncalibrated claim. Selective retries without plan revision binding manufacture a panel that never reviewed one common plan. Evidence manifests without retrieval access give false coverage.

Acceptance requires demonstrated production reachability through chat and CLI adapters and the alternate campaign-runner provider, not merely new manager unit tests. Include the same one-rejection-plus-one-timeout fixture through each provider and the consuming board; require identical terminal-status completeness and preservation of the specific rejection. The alternate provider's construction and error loss were source-traced, but runtime parity and the proposed repairs remain unverified. Preserve existing permission, mutation, completion, and protected-path boundaries throughout. No superiority, benchmark win, or savings is claimed until the complete-system evaluation establishes it.
