---
doc-class: research-proposal
subsystem: chat-knowledge-store-prompt
implementation-status: proposed
last-verified: 2026-10-30
verified-against: 388205138ba6d47c1cff0c5e97c896dfa642a391
verification-method: static-source-inspection
---

# Consultation knowledge: durable rows without semantic admission

## Finding and limits

A successful main-chat specialist consultation can be saved in the project's `knowledge_atoms` table without becoming a candidate for the semantic search that `/knowledge search` and normal JIT project-knowledge retrieval use. The chat persistence function calls the row-only `StoreKnowledgeAtom`, even when an embedding engine is already attached. The semantic reader searches `vectors`, not `knowledge_atoms`; neither reader manufactures the missing vector. This is a producer-to-consumer wiring gap, not evidence that storage failed or that every future answer loses the knowledge. [Chat writer][writer], [row writer][row], [semantic reader][semantic].

The immediate synthesis still receives the consultation's response. The current session retains the results; `/knowledge` and numeric detail have row-based recovery; JIT can fall back to lexical lookup when semantic lookup returns no atoms or errors. Other experts, previously indexed copies, history, direct tools, or a new consultation can supply equivalent information. The narrow consequence is **missing semantic eligibility for this newly persisted consultation**, with conditional search and future-answer quality effects—not universal invisibility, guaranteed model error, or a permission bypass. [Live handoff][update], [synthesis][synthesis], [list/detail][listing], [JIT fallback][jit].

Source pin: main `388205138ba6d47c1cff0c5e97c896dfa642a391`, rechecked 2026-10-30 at approximately 15:14 UTC. This is static research: no application, tests, inference, benchmarks, or provider calls were run. All witnesses and evaluation plans below remain unexecuted.

## Scope, prior work, and controls already present

The review read root and relevant chat, perception, articulation, store, and prompt guidance, main's audit inventory, and research inventories through `research-main-20261030-1450` (`ac3013965dd2ce27d66f04ab833638ac8f4db788`). Store guidance explicitly separates durable recall from row counts; prompt guidance requires actual compiled inclusion, bounded queries, and preserved source identity. These are reasons to evaluate the consuming route, not to make every saved atom mandatory prompt content. [Store guidance][store-guide], [prompt guidance][prompt-guide].

This continues existing knowledge-delivery work at a different boundary:

- **Terminal knowledge synthesis grounding** already identified lost parent evidence and incomplete consultation dispositions. Current main freezes parent context and renders successful, failed, empty, and not-run outcomes in the final request. The immediate response route is not the finding here. That audit also records the intentional persistence filter that excludes failed results; this proposal does not turn failures into knowledge. [Grounding audit][grounding], [current synthesis][synthesis].
- **Knowledge consultation literal identity** covers pre-execution deduplication of specialist/query/purpose. Its repair preserves distinct requests; this finding begins after a nonempty result has returned and has been inserted. [Literal identity audit][literal].
- **Embedding representation and migration** addresses vector identity, compatible read eligibility, and migration. **Knowledge index repair/cache freshness** addresses an existing canonical vector whose derived index changes. Here the consultation writer never creates that canonical vector, so a correctly stamped index or a cache invalidation cannot by itself supply it. [Embedding audit][embedding-audit], [index audit][index-audit].
- **Project knowledge lexical term budget** provides an important fallback control: task terms precede routing expansions in bounded lexical retrieval. This proposal preserves that control rather than treating an embedding service as the only way to remember. [Lexical audit][lexical-audit], [JIT collector][collector].

There is already a dual-write API and an explicit missing-vector backfill API, with tests defining both contracts. Agent creation and agent synchronization use those facilities to repair per-agent knowledge databases. The novel contribution is the still-row-only chat producer and the fact that these agent repairs do not cover its normal project database. It is not a proposal to invent a second embedding system. [Dual write][dual], [backfill][backfill], [existing tests][backfill-tests], [agent repair scope][agent-repair].

The five active non-research heads were checked at `51cfd42649552866d4aa54c53b68acf5740eb09b`, `a53cfc308e8836c72370adaa46deba3f7b00180d`, `80af1863d3a3aa46a722ac8b0c8a0985582d4995`, `aa7988dec4c62116656ea722ff73b687a8509908`, and `767a3291d6287b238417f2466592a9002810d05a`. Each retains `StoreKnowledgeAtom` in `persistKnowledgeResults`; the older four use the older signature without a source argument. This is a check of that producer, not a claim that all branch code or retrieval behavior is identical.

## Production trace

### 1. The result reaches a real persistence consumer

The ordinary articulation path enters consultation handling when the envelope contains knowledge requests and the turn is not already awaiting knowledge. Governed control effects are settled before that branch. The returned `knowledgeGatheredMsg` reaches `handleKnowledgeGatheredMsg`, which saves the full results in `pendingKnowledge` and `knowledgeHistory`, calls `persistKnowledgeResults`, and schedules terminal synthesis with the original request, parent context, and results. No manually injected completion message is needed to establish this route. [Entry][entry], [Update consumer][update].

`persistKnowledgeResults` skips non-nil errors. For each remaining result it constructs a `session/<query>/<specialist>/<timestamp>` concept and calls:

```go
m.localDB.StoreKnowledgeAtom(concept, kr.Response, 0.85, "chat/knowledge_request")
```

The comment above the function says this enables future semantic search and JIT compilation. A successful call, however, proves only the row insertion: `StoreKnowledgeAtom` computes a content hash, inserts concept/content/confidence/hash/source into `knowledge_atoms`, and returns. It performs no embedding, vector insertion, or scheduled publication. The witness uses a nonblank response and successful insertion, avoiding the distinct empty-result and write-error cases. [Writer][writer], [row implementation][row].

### 2. The same project store is wired to consumers

Factory boot opens `.nerd/knowledge.db`, attaches the embedding engine when available, and installs that LocalStore on the JIT compiler. Shared chat boot takes `cortex.LocalDB`; Update installs that component on the model. This is not a comparison between unrelated databases. [Store construction][factory-store], [engine attachment][factory-engine], [JIT wiring][factory-jit], [shared boot][shared-boot], [model installation][install].

The adjacent `StoreKnowledgeAtomWithEmbedding` makes the distinction explicit. It calls the row writer, then, if an engine exists and content is nonblank, calls `StoreVectorWithEmbedding` with `content_type=knowledge_atom`, concept, and confidence metadata. It preserves the row if embedding fails, records a failure counter, and warns; its nil return alone is not a promise of vector readiness. [Dual-write implementation][dual].

### 3. Search and JIT cannot rank a row absent from their corpus

`/knowledge search <query>` calls `SearchKnowledgeAtomsSemantic(ctx, query, 5)` directly. Empty results and errors produce the same no-matching-entries display; that command has no lexical fallback. Conversely, `/knowledge` and `/knowledge <n>` can read `session/` rows when in-memory history is unavailable. This yields a useful observable contrast: an entry can be listed and opened while its new body never entered the semantic candidate corpus. The command's error/no-match conflation is not required for this witness: use a healthy engine and a successful empty search. [Search command][search], [list/detail alternatives][listing].

`SearchKnowledgeAtomsSemantic` checks for an engine and invokes `VectorRecallSemanticFiltered` with `content_type=knowledge_atom`. That function embeds the query and uses the vector-index or brute-force vector route. The semantic reader reconstructs atoms directly from returned vector content and metadata; it does not join against `knowledge_atoms`. A new row in `knowledge_atoms` is not a candidate in either route. An existing independently indexed equivalent can still answer the query; that is recovery, not evidence that this writer published its response. [Semantic reader][semantic], [vector route][vector-route].

Normal JIT project collection first invokes that semantic reader. If it returns any atoms without error, the collector returns them immediately. Only otherwise does it search `knowledge_atoms` lexically. Consequently, the consultation may be recovered by lexical terms when semantic lookup is empty; but a nonempty semantic result set from existing vectors prevents that fallback from inspecting it at all. After retrieval, ordinary relevance, dependency, policy, and token-budget selection still determine actual compiled inclusion. Admission to the candidate corpus is necessary for this route, not sufficient to guarantee delivery. [JIT collector and fallback][collector].

### 4. Existing healing is real, but scoped elsewhere

`BackfillKnowledgeAtomVectors` can discover nonblank `knowledge_atoms` lacking a concept/content-matched knowledge vector and publish them. Its tests explicitly write row-only atoms, invoke backfill, and then search. Current production `EnsureAllAgentKnowledge` enumerates `.nerd/shards/*_knowledge.db`; agent synchronization similarly opens the agent-specific path. Neither is the normal `.nerd/knowledge.db` chat store. [Backfill][backfill], [tests][backfill-tests], [agent repair][agent-repair], [synchronizer][sync].

The inspected generic engine-attachment/index-healing route reads existing `vectors`. General force reembedding processes vectors, prompt atoms, and traces, rather than first projecting all project `knowledge_atoms` into vectors. LocalStore open backfills content hashes. These controls repair their own representations; they do not establish automatic repair for this absent project knowledge vector. An explicit call to the backfill API on the project store would be a valid recovery. [Index backfill][index-backfill], [force reembed][reembed], [store-open maintenance][open].

## Bounded witness and conditional impact

1. Use normal chat entry with an available specialist and a healthy, configured embedding engine. Let articulation request a consultation and let it return nonblank response `K` with nil error. Choose an answer not already represented by an equivalent project knowledge vector; otherwise that separate copy is a valid recovery control.
2. Allow the ordinary Update consumer to persist it successfully. Verify the original request is answered from the live consultation and the `session/` row remains readable. This isolates durable retrieval from the already-fixed immediate synthesis route.
3. Issue `/knowledge search` with a relevant query. `K` has no vector from this writer, even if the query uses identical words. If no other indexed knowledge matches, the command reports no match; otherwise it can return other entries but not this newly inserted representation.
4. For the future-JIT consequence, move beyond the live result's available context or reopen a session, and use a subsequent compilation that executes project-knowledge collection. A query without useful lexical overlap can miss `K` when fallback runs. Alternatively, a nonempty semantic set from other vectors bypasses the lexical table even when its terms could have found `K`. Then condition on the answer actually needing `K`, no other source recovering it, and a repaired vector being ranked and admitted under the same budgets.

The source establishes the missing candidate. It does not establish how frequently these conditions occur, that a particular provider will rank `K` highly after repair, or that any model will answer incorrectly. Plausible costs are repeated specialist work, a confusing list-versus-search experience, or weaker use of prior project-specific findings. Existing vectors remain searchable, and all already-retained row/live-history evidence remains available through its respective route.

## Proposal: one durable artifact, explicit retrieval readiness

**Publication contract.** For nonblank successful consultations selected for durable retention, preserve the existing row and producer provenance, and explicitly record semantic state: pending, ready for an identified embedding space, unavailable/no engine, or retryable failure. A row-write acknowledgement must not be reported as semantic readiness. Keep failed and not-run consultations as dispositions, not authoritative knowledge bodies.

**Reuse current mechanisms.** The narrow eager repair can reuse `StoreKnowledgeAtomWithEmbedding`; the resilient repair should pair durable row insertion with owned, bounded embedding work and a restart-safe backlog. Do not simply add provider latency inside Bubble Tea's synchronous Update handler. Move optional provider work into a command or lifecycle-owned worker with cancellation, a concurrency bound, and shutdown joining. The foreground answer should remain usable if embeddings are slow or unavailable. The current dual-write method swallows individual embedding failure after warning, so a typed readiness result or independently inspectable durable state is needed for reliable retries.

**Repair old project rows.** Reuse the existing backfill logic against the project store with explicit ownership, source/content identity, a resumable cursor or bounded batches, and compatible embedding-space rules. Avoid opening a second uncontrolled connection or reembedding the entire store on every message. Preserve no-text skipping and deduplication. A completed backfill's row count is not enough: verify the right concept/body is eligible under the attached engine and visible through the real consumer.

**Search degradation.** Consider bounded lexical fallback or hybrid retrieval for `/knowledge search`, with honest no-engine/error/partial-coverage status. A fallback improves usability but does not make the semantic projection complete. Conversely, adding the missing projection need not change JIT's current semantic-first policy; any broader hybrid merge should be a separate, evaluated choice with deduplication and relevance controls.

**No authority expansion.** Retrieved consultation text remains evidence with provenance and existing policy treatment. More searchable text is not a grant to execute tools, override user restrictions, or promote all specialist claims to verified fact. Preserve the current request cap, answer ownership, consultation dispositions, source isolation, content integrity, and whole-prompt budget rules.

## Costs and fair evaluation

Publishing vectors adds provider calls, tokens, latency, vector/index storage, and repair complexity. Durable queuing adds state transitions and crash recovery; eager publication can make the TUI noticeably slower. A larger candidate corpus can introduce irrelevant or stale knowledge and displace useful context. Measure those effects rather than counting more hits as an unconditional improvement. No-engine operation must remain a supported row/lexical state, not an initialization failure.

Recommended evaluation, not run in this audit:

- **Normal-entry regression:** use the real submission → articulation consultation → worker result → Update → persistence path, not direct insertion alone. Confirm the final answer still receives `K`; then exercise actual slash search and a later compilation against the same project store.
- **Reopen:** close and reopen through normal boot, clear in-memory consultation history, and verify persisted detail and semantic state independently. Test legacy row-only project records and the existing per-agent repair as separate cases.
- **Deterministic retrieval fixture:** a controlled embedding engine can establish exact corpus membership, metadata, identity, and ranking without conflating this defect with provider quality. Add a negative query and an already-indexed equivalent as controls. A real-provider quality trial is separate and should report ranking uncertainty.
- **JIT path:** cover empty-semantic lexical recovery, nonempty-semantic fallback suppression, successful vector retrieval subsequently rejected by policy/budget, and admitted compiled inclusion. Inspect the actual compiled prompt and source provenance; do not stop at a database row count or helper result.
- **Failure matrix:** no engine, empty content, failed consultation, failed row write, embedding failure, cancellation, restart before publication, repeated retry, model-space change, and shutdown. Preserve a readable row without claiming readiness; never double-publish on retry or block terminal answer delivery indefinitely.
- **Cost/quality comparison:** hold source snapshot, request set, relevance judgments, and prompt budget constant. Compare useful search/compiled recall, answer correctness when the stored fact matters, unnecessary consultations, unrelated-atom admission, provider spend, p50/p95 foreground latency, backlog age, and storage growth. Retain examples where lexical or existing indexed knowledge already succeeds so the baseline receives fair credit.

Acceptance is not “every saved atom appears in every prompt.” It is that an intentionally retained consultation has an explicit, recoverable semantic-publication state; eligible records reach the intended search corpus; and the ordinary retrieval and selection controls can make an informed choice.

[writer]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/process_knowledge.go#L369-L400
[row]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/store/local_knowledge.go#L43-L71
[dual]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/store/local_knowledge.go#L379-L424
[semantic]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/store/local_knowledge.go#L426-L483
[entry]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/process.go#L1101-L1109
[update]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/model_update.go#L980-L1028
[synthesis]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/process_knowledge.go#L277-L329
[listing]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/commands_handlers_files.go#L277-L459
[search]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/commands_handlers_files.go#L356-L403
[factory-store]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/system/factory.go#L1136-L1144
[factory-engine]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/system/factory.go#L1753-L1771
[factory-jit]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/system/factory.go#L1886-L1891
[shared-boot]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/session_shared_boot.go#L104-L117
[install]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/model_update.go#L695-L735
[vector-route]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/store/vector_store.go#L584-L646
[collector]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/prompt/compiler_db.go#L321-L423
[jit]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/prompt/compiler_db.go#L389-L423
[backfill]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/store/knowledge_backfill.go#L12-L92
[backfill-tests]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/store/knowledge_backfill_test.go#L33-L99
[agent-repair]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/init/agents_knowledge_repair.go#L15-L65
[sync]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/prompt/sync/synchronizer.go#L115-L175
[index-backfill]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/store/vector_store.go#L1048-L1065
[reembed]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/store/reembed_all.go#L190-L227
[open]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/store/local_core.go#L136-L153
[store-guide]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/store/agents.md
[prompt-guide]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/prompt/agents.md
[grounding]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/terminal-knowledge-synthesis-grounding.md
[literal]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/knowledge-consultation-literal-identity.md
[embedding-audit]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/embedding-representation-and-migration.md
[index-audit]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/knowledge-index-repair-cache-freshness.md
[lexical-audit]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/project-knowledge-lexical-term-budget.md
