---
doc-class: deep-dive
subsystem: context
implementation-status: proposed
last-verified: 2026-10-28
verification-method: static-source-inspection
verified-against: 356f65e0e6bb639adaa90ab36212daa714114871
feature-compared-against: c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c
research-inventory-against: 890cfb7b7d0fe77073bb09fe18aefecb62bc27d3
---

# Make remembered preferences recallable and forgetting scope explicit

The normal chat path has two incompatible meanings of long-term memory. A natural-language remember instruction writes a cold-storage row with type `learned`; the shared Piggyback promotion operation writes type `preference`, which the normal hydration pass actually loads. Both natural-language and Piggyback forget operations remove only a matching `session_note` from the kernel. They do not remove either durable row. The feature branch adds useful application dispositions, but a successful or no-match session-note retraction can still be classified as applied without identifying the memory domain it affected.

**Decision:** define one owned memory identity and lifecycle across perception, control packets, persistence, retrieval and acknowledgement. Preserve the existing safety restriction against arbitrary predicate retraction. Do not make `forget` a broad database-delete capability. Until a durable-memory operation is explicitly supported, the response contract should state that only a session note was targeted and that durable deletion was not performed.

This is a source-based design proposal, not a report of an observed user incident. No tests, build, live inference, runtime probe, failure injection or implementation was performed. The source-derived sequences below are candidates for later verification, not executed reproductions. Natural-language `remember` and `forget` are perception actions here; this report does not claim that typing literal slash commands `/remember` or `/forget` invokes these handlers.

## Scope and existing work

The [root contract][root] assigns memory and executive decisions to Mangle. [Store guidance][store-guidance] distinguishes successful persistence from recall after reopening; [prompt guidance][prompt-guidance] distinguishes saved content from content a request actually consumes.

The research inventory at the header revision includes three adjacent reports:

- [Articulation admission and control fidelity][admission-doc] owns failed or partially applied controls, parsing and replay. Its feature appendix correctly credits the new dispositions. The present finding is that a successful low-level disposition can describe the wrong memory domain, and that the two promotion producers disagree on a persistence discriminator. It remains even when every store call succeeds.
- [Chat session memory ownership and recovery][session-doc] owns session selection, compressed-state identity and coherent checkpoints. A perfectly isolated new session would still load the same filtered cold-storage types and would not make session-note deletion erase a preference.
- [Learned correction integrity and worker lifecycle][correction-doc] owns correction qualifiers, contextual durable writes and worker teardown. This report concerns explicit user memory operations, not reinforcement or embedding-worker lifetime.

The [CLI scoped-forget contract][scope-contract] is already implemented for its stated purpose: preserve executive facts while deleting only the matching session-note key. That safety repair must remain. The missing capability is not permission to retract arbitrary predicates; it is an honest, typed user-memory operation with a supported lifetime.

## Production reachability and source freeze

Chat boot takes its VirtualStore and LocalStore from the shared Cortex, rather than constructing a separate memory service. The normal turn calls `processMemoryOperations` after routing and before later context loading. The Understanding adapter maps recognized remember/forget actions to internal intent verbs and memory operations. These are actual production callers, not an isolated helper with no caller. [Shared boot][boot], [action mapping][mapping], [memory call][memory-call].

Reachability is conditional on perception producing the corresponding action and a nonempty `Scope.Target`, the request reaching this point rather than an earlier route return, and the relevant kernel/store being configured. The code does not establish how often the model chooses that interpretation. A later articulation response can also emit independent memory operations; this report does not assume those operations always occur or always repair the first path.

Pinned comparison:

| Source boundary | Main and feature relationship |
|---|---|
| Understanding memory extraction | Main lines 667–688 and feature lines 684–703 retain the same remember key/value and forget key mapping. |
| Chat memory consumer | Main lines 55–79 and feature lines 65–90 retain the `learned` discriminator and session-only forget. Feature additionally checks the returned disposition. |
| Shared memory operations | Feature adds explicit application outcomes, including partial note replacements. Both versions keep durable promotion and session-note forgetting separate. |
| Hydration, cold storage and kernel retraction | Identical blobs across the pinned heads: `virtual_store_predicates.go` 535f8403; `local_cold.go` c9638346; `kernel_facts.go` 17e1e72f. |
| Model-facing memory atom | Identical blob 9e3cb45f. It describes the array as managing long-term knowledge and describes forget broadly, while note is explicitly short-term. |

This is not a claim that the feature has been merged to main. The comparison credits its better error accounting without treating that repair as durable erasure. [Feature consumer][feature-consumer], [feature memory operations][feature-memory], [memory atom][memory-atom].

## Finding one successful remember writes do not enter normal hydration

The adapter constructs a promotion with key `preference` and the recognized target as its value. Chat then calls `StoreFact(key, [value], "learned", 10)`. By contrast, the shared operation used by Piggyback calls `StoreFact(key, [value], "preference", 10)`. [Adapter][extraction], [chat consumer][consumer], [shared promotion][memory-main].

`StoreFact` persists the supplied discriminator as `fact_type`; it does not normalize `learned` to `preference`. Its conflict identity is predicate plus argument encoding, so different values under the generic preference key are separate records, not necessarily overwrites. [Cold-storage write][store-write].

The normal `HydrateLearnings` pass loads exactly the `preference`, `user_fact` and `constraint` categories into the corresponding learned predicates. The type filter reaches an exact SQL equality. There is no `learned` branch in this pass. Chat calls hydration later in the same ordinary articulation path. [Hydration][hydrate], [type filter][type-filter], [turn hydration][turn-hydrate].

A bounded witness is a recognized natural-language remember operation, a successful store write, no additional Piggyback promotion, and a later fresh-kernel hydration. The stored row exists, but that hydration does not admit it as a learned preference. This establishes producer/consumer mismatch, **not universal irretrievability**. `QueryLearned` loads by predicate independently of type, and the unbound external query can request all fact types. Recent conversation history may also carry the original text. Those alternatives can make an individual answer correct; neither proves parity of the normal persisted-memory journey. [Broader readers][query-learned], [unbound reader][external-query].

The converse problem matters too: repairing the discriminator alone can make previously inactive memories active. A migration must be explicit about which legacy `learned` rows came from user remember operations; it must not reinterpret every record of that type as a user preference.

## Finding two forget targets a different memory domain

The adapter maps the recognized forget target to `MemoryOperation.Key`. The chat consumer passes that operation to `ApplyMemoryOperation` with no durable store. More importantly, even a nonnil store would not change the current forget implementation: it retracts only `session_note(Key, ...)`. The `MemoryStore` interface exposes writes, not deletion. [Adapter][extraction], [chat consumer][consumer], [memory implementation][memory-main].

A durable preference stored through either promotion producer is a cold-storage row, not a session note. A precise natural-language match to its content, or even supplying its exact stored key, does not redirect this operation into cold storage. A promoted `preference` row remains eligible for the next hydration; a `learned` row remains stored and accessible to broader readers. The existing `LocalStore.DeleteFact` removes an exact predicate-and-arguments row, but the inspected memory-operation route does not call it. [Exact store delete][store-delete], [hydration][hydrate].

The feature makes this mismatch observable at a useful but insufficient layer. It initializes the memory disposition as applied, calls scoped retraction, and changes the stage only on error. The underlying RealKernel returns nil when there are no matching facts. The production Cortex routes retraction to shard-local kernels and propagates errors, not a matched-record count. Thus a forget operation whose key names no session note can return applied while leaving durable memory untouched. Idempotent deletion need not be an error; the problem is a receipt that does not state *which domain was addressed* or whether the requested durable entity was resolved. [Feature disposition][feature-memory], [no-match semantics][retract], [Cortex bridge][cortex-retract], [shard bridge][shard-retract].

The source does **not** prove that the user is always told “I forgot it.” Natural-language replies are model-produced, and the feature has real warning propagation for unsuccessful dispositions. The narrower result is that this successful low-level path neither performs durable forgetting nor supplies a domain-qualified negative outcome that would require the answer to disclose that limitation. A correct answer remains possible.

The model-facing optional memory atom contributes an interface mismatch: it presents memory operations as long-term knowledge management, offers promotion to persistent storage, describes forgetting without a session-only qualification, and separately labels note as short-term. Selection of that optional atom is conditional; it is not assumed present in every request. When selected, its contract should match the operation the host can actually perform. [Memory atom][memory-atom].

## Proposed scoped memory contract

Keep the existing memory service and policy boundary rather than adding a second planner. The following are proposed concepts, not existing API names.

1. **Resolve identity before changing state.** A memory record needs a stable identifier, owner/workspace, domain, source request, value revision and lifetime. Natural-language matching proposes candidates; it does not authorize bulk deletion. A generic key such as `preference` is insufficient to identify one of several values. Unresolved or multiple matches require clarification or a bounded preview.
2. **Unify promotion semantics.** Both recognized user instructions and Piggyback promotions should enter the same typed host operation and persist the same supported domain. Mangle decides admission and applicability; Go validates identities, writes bytes and measures results. Verify stored, reopen-readable, retrieved and delivered states separately.
3. **Separate session-note removal from durable-memory removal.** Preserve narrowly scoped note retraction. Either expose a clearly named session-only operation, or require the existing operation to carry an explicit domain. Until durable removal exists, return unsupported-for-domain rather than implying it was attempted.
4. **Authorize only the requested records.** Bind any durable deletion to the user's request, resolved record IDs and current revisions. Do not permit a model to select a kernel predicate, delete all `preference` rows, or erase learned constraints merely by emitting a key. Reuse the existing exact authorization and approval machinery where the selected operation requires it.
5. **Retire live projections with durable state.** Removing an owned record must also prevent its already-hydrated projection or pending producer from republishing it as current memory. Use a generation or retirement marker at read/publication boundaries. On partial failure, preserve an explicit pending/failed settlement rather than claiming complete removal or replaying unrelated effects.
6. **Report the postcondition and scope.** Distinguish removed, already absent, unresolved identity, unsupported domain, refused, failed and partially settled. A successful no-op is acceptable when the requested record is proven already absent in the correct domain. The response should identify what no longer influences future turns and what remains elsewhere.

Generation and invalidation are established provenance concepts, not a new memory theory. W3C PROV-DM distinguishes an entity's creation and subsequent invalidation, including loss of availability for use. This supports explicit lifecycle identity; it does not prove a particular implementation's authorization, semantic matching or physical erasure. [W3C PROV-DM](https://www.w3.org/TR/prov-dm/#term-Invalidation).

## Retention and privacy boundaries

“Do not use this preference again,” “delete this saved memory,” and “erase this conversation” are different operations. The repair should make those differences visible, not silently expand a preference request into transcript, audit, checkpoint or backup deletion. A minimal outcome receipt should retain only the identity and status needed for accountability; copying the forgotten text into its receipt would create another retention path.

Logical invalidation is not physical erasure. An ordinary SQL deletion affects matched rows; it is not evidence about all copies or already-sent model requests. SQLite separately documents secure-delete behavior and limitations, including virtual-table shadow content. This note does not inspect the application's complete forensic-erasure posture or allege a measured leak. Any physical-erasure promise needs a separately scoped inventory and evidence. [SQLite DELETE](https://www.sqlite.org/lang_delete.html), [SQLite secure_delete](https://www.sqlite.org/pragma.html#pragma_secure_delete).

## Alternatives and full cost

The smallest safe interim option is to narrow prompt and response wording: session-note forgetting only, durable removal explicitly unsupported. This is inexpensive and preserves safety, but leaves the user's durable-memory task unresolved.

A typed exact-record operation is a stronger incremental option. It costs identity/schema work, candidate resolution, a bounded store transaction, live-projection invalidation, receipt storage and tests across both entry paths. Ambiguous natural-language resolution can cost an extra user turn. Query, transaction and coordination costs should be measured independently from model tokens.

Tombstones can prevent stale snapshots or asynchronous producers from resurrecting retired records, but retaining values in tombstones would undermine a deletion request. Keep only necessary opaque identity and revision data, bound retention, and define restore behavior explicitly.

A global purge or full history replay is not an acceptable shortcut. Purging expands authority and loses unrelated memory; replay can regenerate the removed preference and increases context cost. Adding embeddings alone does not repair the missing domain/identity contract and makes deletion propagation more expensive.

No efficiency gain is measured here. Evaluate operational cost alongside correctness: candidate lookup latency, affected projection count, transaction time, receipt bytes, extra clarification turns, tokens added to operation guidance, and recovery complexity. Quality constraints include no unintended deletion, no false completion and no reappearance through the supported memory readers.

## Proposed verification and checkable exits

All cases below are **unrun**. Preserve the normal production entry paths and actual kernel/store wiring; helper-only assertions are insufficient.

- Recognize a natural-language remember request, inspect its persisted discriminator, reopen storage, run fresh-kernel hydration and inspect the final relevant request. Compare against equivalent Piggyback promotion. Assert the specific delivered record, not just row count.
- Store multiple values under one key. Forget exactly one resolved record; prove the others and executive permission/intent/safety facts survive. Include semantic paraphrases, ambiguous matches and no-match requests.
- Exercise session-note deletion separately. Require a domain-qualified outcome for both actual removal and already absent state.
- Promote a durable preference, request its removal, then inspect immediate kernel state, fresh hydration, broad `query_learned` access and the actual next request. A passing low-level DELETE alone is not completion.
- Inject store failure, kernel-retirement failure and concurrent stale republishing. Keep the original failure and partial state in the receipt; never acknowledge unsupported durable erasure as applied.
- Test cancellation and retry at transaction boundaries, including an already absent record, with stable operation identity.
- Verify wording for each disposition. A deterministic host notice must not say durable memory was removed when only a session note was addressed. Keep model-written prose clearly downstream of that fact.
- Define legacy-row migration and retention expectations explicitly. Test old `learned` rows without converting unrelated records by type alone.

The minimum coherent delivery is: one promotion discriminator across both producers, explicit memory domain and entity resolution for forgetting, exact authorized scope, measured settlement with live-projection retirement, and truthful acknowledgement of retained copies outside that scope.

[root]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/agents.md
[store-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/store/agents.md
[prompt-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/prompt/agents.md
[admission-doc]: https://github.com/theRebelliousNerd/codenerd/blob/890cfb7b7d0fe77073bb09fe18aefecb62bc27d3/Docs/audits/articulation-admission-and-control-fidelity.md
[session-doc]: https://github.com/theRebelliousNerd/codenerd/blob/890cfb7b7d0fe77073bb09fe18aefecb62bc27d3/Docs/audits/chat-session-memory-ownership-and-recovery.md
[correction-doc]: https://github.com/theRebelliousNerd/codenerd/blob/890cfb7b7d0fe77073bb09fe18aefecb62bc27d3/Docs/audits/learned-correction-integrity-lifecycle.md
[scope-contract]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/Docs/architecture/cli/03-GAP-ANALYSIS.md#L72-L78
[boot]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/cmd/nerd/chat/session_shared_boot.go#L88-L110
[mapping]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/perception/understanding_adapter.go#L604-L607
[memory-call]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/cmd/nerd/chat/process.go#L291-L319
[extraction]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/perception/understanding_adapter.go#L667-L688
[consumer]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/cmd/nerd/chat/process_helpers.go#L55-L79
[feature-consumer]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/process_helpers.go#L65-L90
[memory-main]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/context/memory_ops.go#L18-L75
[feature-memory]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/context/memory_ops.go#L19-L111
[store-write]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/store/local_cold.go#L63-L94
[type-filter]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/store/local_cold.go#L166-L185
[hydrate]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/virtual_store_predicates.go#L573-L628
[turn-hydrate]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/cmd/nerd/chat/process.go#L724-L731
[query-learned]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/virtual_store_predicates.go#L25-L85
[external-query]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/virtual_store_predicates.go#L774-L796
[store-delete]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/store/local_cold.go#L215-L233
[retract]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/kernel_facts.go#L865-L914
[cortex-retract]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/cortex_kernel.go#L541-L558
[shard-retract]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/kernel_shard.go#L222-L238
[memory-atom]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/prompt/atoms/protocol/piggyback.yaml#L112-L148

