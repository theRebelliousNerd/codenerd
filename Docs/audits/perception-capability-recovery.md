---
doc-class: deep-dive
subsystem: perception
implementation-status: planned
last-verified: 2026-10-28
verified-against: 8cd6e1f8804709996b46e43641a3ffe464d2088a
supersedes: []
---

# Perception capability recovery: finish ownership and evidence, not dead wrappers

**Status:** source-inspected research and proposed refinement; no execution, tests, or performance results.  
**Source:** main at `8cd6e1f8804709996b46e43641a3ffe464d2088a`; comparison to main was identical on 2026-10-28 at approximately 04:11 UTC.  
**Scope:** the ten perception functions in the supplied dead-code report, plus their learning-store dependency. The report's reachability counts were supplied context, not independently rerun.

## What this subsystem is supposed to become

Perception should translate a user's current request into a constrained, context-sensitive understanding while learning reusable language mappings from confirmed corrections. Similar examples are advisory grounding; Mangle derives executive routing; neither similarity nor a learned example confers permission. The unfinished product capability is **workspace-owned, explainable semantic memory whose contribution to a decision can be traced and evaluated**.

This is not a newly invented roadmap. The current [perception TODO](https://github.com/theRebelliousNerd/codenerd/blob/8cd6e1f8804709996b46e43641a3ffe464d2088a/Docs/architecture/perception/TODO.md#L105-L180) explicitly proposes a capability/ownership receipt and a perception decision receipt, and says the ownership split remains unbuilt. The findings below refine that existing work.

## Dead-code names versus retained capabilities

| Flagged entry | Intended capability | Current source assessment |
|---|---|---|
| `NewZAIClient` | Construct configured provider transport | Configured construction is live in [client factory](https://github.com/theRebelliousNerd/codenerd/blob/8cd6e1f8804709996b46e43641a3ffe464d2088a/internal/perception/client_factory.go#L431); absence of the convenience entry does not show missing provider support. |
| `EmbeddedCorpusStore.LoadFromKernel` | Load taxonomy examples and cache their embeddings | Constructor uses `LoadFromKernelWithContext`, with caller-derived timeout. Legacy load and strict-context load differ in cancellation/cache handling; do not blindly substitute the old function. |
| `EmbeddedCorpusStore.cacheGet` | Reuse persisted embeddings | Context-aware cache method exists and is used by the context-aware loader. |
| `EmbeddedCorpusStore.cachePut` | Persist new embeddings | Context-aware write method preserves cancellation/error handling; old method is a convenience wrapper. |
| `InitSemanticClassifier` | Publish classifier after boot | Actual bootstrap uses `InitSemanticClassifierWithContext`. |
| `NewEmbeddedCorpusStoreWithCache` | Construct cached embedded corpus | Actual classifier uses the context-aware constructor. |
| `NewLearnedCorpusStore` | Retrieve workspace-learned language mappings | Actual classifier uses `NewLearnedCorpusStoreWithContext`; SQL-backed construction also has a context-aware replacement. |
| `NewSemanticClassifierFromConfig` | Construct classification pipeline | Context-aware replacement performs owned construction and cancellation cleanup. |
| `InitPerceptionLayer` | Initialize shared perception state | Production system factory invokes `InitPerceptionLayerWithContext`. |
| `isValidUnderstandingPromptContract` | Reject incompatible JIT understanding prompts | Boolean wrapper is unnecessary for the live path: `getUnderstandingPrompt` directly invokes the richer reason-returning validator and falls back. |

Evidence: [classifier construction](https://github.com/theRebelliousNerd/codenerd/blob/8cd6e1f8804709996b46e43641a3ffe464d2088a/internal/perception/semantic_classifier.go#L169-L277), [cache methods and load variants](https://github.com/theRebelliousNerd/codenerd/blob/8cd6e1f8804709996b46e43641a3ffe464d2088a/internal/perception/semantic_classifier.go#L810-L860), [learned store](https://github.com/theRebelliousNerd/codenerd/blob/8cd6e1f8804709996b46e43641a3ffe464d2088a/internal/perception/semantic_classifier.go#L1160-L1208), [shared initialization](https://github.com/theRebelliousNerd/codenerd/blob/8cd6e1f8804709996b46e43641a3ffe464d2088a/internal/perception/semantic_classifier.go#L1313-L1345), [perception bootstrap](https://github.com/theRebelliousNerd/codenerd/blob/8cd6e1f8804709996b46e43641a3ffe464d2088a/internal/perception/transducer.go#L415-L438), [system bootstrap](https://github.com/theRebelliousNerd/codenerd/blob/8cd6e1f8804709996b46e43641a3ffe464d2088a/internal/system/factory.go#L1416), [store constructor](https://github.com/theRebelliousNerd/codenerd/blob/8cd6e1f8804709996b46e43641a3ffe464d2088a/internal/store/learned_store.go#L52-L58), [prompt validation](https://github.com/theRebelliousNerd/codenerd/blob/8cd6e1f8804709996b46e43641a3ffe464d2088a/internal/perception/understanding_adapter.go#L110-L142).

**Disposition:** preserve the active contracts, then separately decide whether legacy interfaces are supported API, compatibility adapters, or removable. Reintroducing contextless calls just to make reachability green would be a regression in lifecycle control. Go's [Context guidance](https://go.dev/blog/context) explains why cancellation and deadlines should propagate through request-related operations; `Background` does not carry a caller's cancellation.

## What is genuinely wired

The [canonical understanding adapter](https://github.com/theRebelliousNerd/codenerd/blob/8cd6e1f8804709996b46e43641a3ffe464d2088a/internal/perception/understanding_adapter.go#L390-L427) gets semantic matches from the shared classifier and passes them into `LLMTransducer.Understand`. The classifier injects semantic-match facts as well as returning matches. The LLM prompt includes examples above its stricter similarity threshold: greater than 0.8, maximum eight, bounded text length. It includes verb, target, **constraint**, and similarity, so the absence of constraint in the six-argument kernel fact is not evidence that constraints vanish from the whole pipeline. See [prompt construction](https://github.com/theRebelliousNerd/codenerd/blob/8cd6e1f8804709996b46e43641a3ffe464d2088a/internal/perception/transducer_llm.go#L253-L325) and [fact injection](https://github.com/theRebelliousNerd/codenerd/blob/8cd6e1f8804709996b46e43641a3ffe464d2088a/internal/perception/semantic_classifier.go#L503-L562).

Learning also has a live path. [Consolidation](https://github.com/theRebelliousNerd/codenerd/blob/8cd6e1f8804709996b46e43641a3ffe464d2088a/internal/perception/consolidation.go#L88) calls `PersistLearnedFact`. That routine normalizes a fact, hot-loads it, stores an exemplar, calls `SharedSemanticClassifier.AddLearnedPattern`, and appends taxonomy storage. [Persistence](https://github.com/theRebelliousNerd/codenerd/blob/8cd6e1f8804709996b46e43641a3ffe464d2088a/internal/perception/learning.go#L239-L323) and [semantic addition](https://github.com/theRebelliousNerd/codenerd/blob/8cd6e1f8804709996b46e43641a3ffe464d2088a/internal/perception/semantic_classifier.go#L567-L625) show the stages. SQL-backed addition avoids a duplicate caller-side embedding. This establishes source wiring, not improved accuracy or consistent persistence under every partial failure.

The future capability is therefore not “add semantic retrieval” or “connect learning.” It is to make existing retrieval and learning **owned, attributable, revocable, and demonstrably useful**.

## Remaining ownership boundary

Four concrete observations justify the existing ownership work:

1. The classifier captures a kernel during construction ([line 188](https://github.com/theRebelliousNerd/codenerd/blob/8cd6e1f8804709996b46e43641a3ffe464d2088a/internal/perception/semantic_classifier.go#L188)).
2. Shared initialization returns immediately when a shared classifier already exists, without comparing workspace, kernel, or configuration ([1329–1331](https://github.com/theRebelliousNerd/codenerd/blob/8cd6e1f8804709996b46e43641a3ffe464d2088a/internal/perception/semantic_classifier.go#L1329-L1331)).
3. Cache and learned database destinations depend on shared taxonomy workspace state ([229–233](https://github.com/theRebelliousNerd/codenerd/blob/8cd6e1f8804709996b46e43641a3ffe464d2088a/internal/perception/semantic_classifier.go#L229-L233), [1191–1204](https://github.com/theRebelliousNerd/codenerd/blob/8cd6e1f8804709996b46e43641a3ffe464d2088a/internal/perception/semantic_classifier.go#L1191-L1204)).
4. Injection retracts the entire `semantic_match` predicate before publishing the current matches ([514](https://github.com/theRebelliousNerd/codenerd/blob/8cd6e1f8804709996b46e43641a3ffe464d2088a/internal/perception/semantic_classifier.go#L514)).

**Inference, not observed incident:** if multiple workspaces or overlapping turns share this process state, reuse and replacement can cross intended ownership or turn boundaries. Whether deployed session scheduling currently prevents that requires separate evidence. Single-workspace operation does not establish a safe multi-workspace contract.

### Proposed ownership algorithm

This is a design contract, not runnable implementation:

1. Boot constructs a semantic owner keyed by workspace identity, corpus generation, embedding model/dimension, and kernel owner. Do not use an unconstrained process singleton as the identity.
2. Publish the owner only after required initialization succeeds. Cancellation closes only resources created by this attempt.
3. Inject the owner handle into the understanding adapter. A turn obtains a generation-stable retrieval view and bounded candidate list.
4. Associate semantic evidence with `(owner, session, turn, generation)`. Publish/retract only that scope, or pass an immutable turn snapshot if the kernel interface cannot represent the scope safely.
5. Teardown waits for or cancels that owner's leases, closes its stores exactly once, and cannot close another owner's resources.
6. Reject mismatched workspace/model handles rather than silently borrowing the existing global. A legacy single-owner adapter may remain temporarily, with explicit restriction and diagnostics.

The design must decide whether concurrent readers see the previous or new corpus generation during learning. Either snapshot strategy is acceptable; mixed-generation receipts are not.

## Finish the correction-to-decision evidence loop

Build on the existing decision-receipt proposal, rather than add unrelated logging:

- **Semantic owner receipt:** opaque owner ID, workspace digest, model identity, dimension, corpus generation, supported capabilities, initialization/degradation status. No credentials.
- **Learned example record:** stable example ID, source correction/reference, applicable scope, normalized mapping, accepted/revoked status, content digest, corpus generation. Acceptance is not authorization for future actions.
- **Decision receipt:** turn correlation, input digest, owner/capability IDs, retrieved candidate IDs and scores, included-versus-filtered candidates, effective prompt-contract/fallback reason, normalized Understanding digest, routing provenance, final Intent digest, downstream outcome links.
- **Retention:** bounded metadata, deletion/revocation semantics, and access policy. No full prompts, hidden reasoning, secrets, or raw provider replies by default.

A learned example should be attributable to decisions that actually consumed it, not merely to a successful database write. Promotion or revocation should create a new corpus generation; later reconstruction must not falsely apply today's corpus to yesterday's decision.

Do not overwrite Mangle policy with a learned similarity threshold. Similarity is not calibrated intent confidence, nor proof that the current input shares the prior user's constraints. Retrieval evidence should support classification, while policy separately decides what may execute.

## Costs and fair evaluation

Use total cost per correctly handled request, not embedding cost alone:

`C_total = C_boot/cache_build amortized + C_query_embedding + C_retrieval + C_prompt_tokens + C_understanding + C_kernel + C_receipts + C_learning + C_retries + C_human_review`.

This scalar sum is meaningful only when every term is converted to the same monetary unit using disclosed prices and an explicit human-time valuation. Otherwise report the resource vector separately: input/output/cache tokens, embedding calls, CPU/GPU time, wall time, peak memory, disk/I/O, and reviewer minutes. Do not add milliseconds, tokens, and human effort as if they were interchangeable; report sensitivity to price and amortization assumptions.

Count cache/model migrations, failed initialization, candidate expansion, disk writes, retention, learning-critic calls, and any evaluation labels. Owner isolation can increase memory and reduce cross-workspace cache reuse; hashed, explicitly shareable immutable embeddings may recover reuse only if privacy and model identity permit it. Receipt IDs can reduce diagnostic prompt payloads, but this is a hypothesis, not a measured saving.

Proposed comparisons, using the same model, tasks, policies, and repair budget:

1. Current production semantic path.
2. No semantic exemplars, preserving all other context.
3. Owned/generation-scoped semantic path with the same retrieval settings.
4. Owned path plus decision receipts.
5. Learned retrieval enabled versus disabled, using held-out corrections and time-separated evaluation.

Measure intent/constraint correctness, false mutation routing, abstention/degradation, total tokens and model calls, p50/p95 latency, boot cost, memory/disk, human corrections, and cost per verified successful task. Socket-level retrieval metrics should include candidate relevance and inclusion rate, but cannot substitute for end-to-end task outcomes.

Required scenarios include sequential workspace switches, overlapping conflicting turns, cancellation during cache load/write, failed store initialization, embedding-model migration, corpus revocation, stale owner handles, and teardown during retrieval. Evaluate prompt fallback separately: malformed or conflicting contracts must trigger documented fallback without being reported as a successful JIT contract.

## Failure modes and completion criteria

- Process-global reuse masks incorrect owner selection: reject mismatches and record selected owner.
- Broad fact retraction erases another turn's evidence: scoped snapshots or serialized ownership contract.
- Stale corpus/model cache changes semantics: version model, dimension, and generation together.
- Similar examples override explicit current constraints: current-request constraints dominate; evaluate adversarial near-neighbors.
- Accepted corrections become permission: separate memory from authorization.
- Multi-stage learning persistence diverges: record per-stage outcome and repair/reconciliation status; never report all stores committed from one success.
- Receipts expose user content or grow indefinitely: digests, redaction, bounded retention.
- More evidence increases tokens without helping: fixed-budget ablations and total-cost reporting.

Completion means the live production path has explicit owners, scoped semantic evidence, traceable learning-to-decision linkage, and passing designed isolation/quality gates—not that every old constructor is called. No such gates were executed in this research pass.

