---
doc-class: research-proposal
subsystem: prompt-mangle-predicate-retrieval
implementation-status: proposed-not-implemented
last-verified: 2026-10-28
verified-against: 356f65e0e6bb639adaa90ab36212daa714114871
verification-method: static-source-inspection-only
---

# Predicate JIT must retrieve the current vocabulary, not certify it by row count

## Decision

Keep the bounded predicate selector and the feature branch's embedding-space safeguards. Add a **corpus-qualified inventory contract**: every semantic candidate must identify a current predicate and its current represented content; the authoritative corpus supplies the signature. Persisted retrieval metadata must never override current arity.

The strongest source-supported witness is a restart with a changed predicate corpus and a retained LocalStore, **using the same embedding model**. The selector can treat an obsolete inventory as complete and can advertise an old arity alongside the current predicate's description. This is a model-input and retrieval-quality gap, not evidence of invalid rule persistence or an execution-authority bypass.

## 1. Scope, pins, and evidence limits

Inspected branch tips:

| Branch | Source pin | Role |
|---|---|---|
| `main` | [`356f65e0e6bb639adaa90ab36212daa714114871`](https://github.com/theRebelliousNerd/codenerd/tree/356f65e0e6bb639adaa90ab36212daa714114871) | Current runtime baseline |
| `feat/conversation-brief` | [`c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c`](https://github.com/theRebelliousNerd/codenerd/tree/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c) | Active feature branch, 27 commits ahead of this main pin |
| `research-main-20261028` | [`cc0e7c60d292cd0286a3e410a937c0e63b3286cf`](https://github.com/theRebelliousNerd/codenerd/tree/cc0e7c60d292cd0286a3e410a937c0e63b3286cf) | Current research deduplication |
| `codenerd-research-docs` | [`c592ff223cd207fca4c29ae793d5eba57aa4781d`](https://github.com/theRebelliousNerd/codenerd/tree/c592ff223cd207fca4c29ae793d5eba57aa4781d) | Historical research deduplication |

`internal/prompt/predicate_selector.go` has the identical blob `906e34e3e9a642dcf39481b119885619ae0d4160` on all four pins. The current core corpus loader and legislator wiring also have identical blobs across them. Store behavior differs on the feature branch and is explicitly credited below.

The review searched 133 distinct versions of root audit documents and prompt/JIT/Mangle architecture documents across those branches, then inspected the relevant source. Existing research already covers atom-JIT cache generations, global vector-source scope, selector degradation, embedding-space identity, and resumable migration. This proposal does not repeat those mechanisms. It concerns the separate `PredicateSelector` surface: **same-space corpus membership/content freshness and authoritative signature hydration**. The architecture's [PredicateSelector section][arch-selector] already identifies this as separate from atom JIT; the current source establishes its production consumer, rather than relying on old prose about its product role.

No implementation, tests, benchmarks, codeNERD execution, inference, provider calls, or implementation changes were performed. The sequences below are proposed deterministic witnesses derived from source, not observed incidents. No user LocalStore was inspected, and no prevalence or performance improvement is claimed.

## 2. The existing production path and protections

1. [`NewPredicateCorpus`][corpus-load] extracts the embedded database to a new read-only temporary SQLite file. A fresh binary can therefore bring a new corpus while the workspace's LocalStore remains persistent; this finding does not require mutating the read-only corpus inside a running process.
2. [`LegislatorShard.SetParentKernel`][legislator-wire] constructs `PredicateSelector` from the kernel's corpus, attaches `VirtualStore.GetLocalDB()`, and hands the selector to its feedback loop. [`generateRule`][legislator-generate] uses that loop for rule generation.
3. [`FeedbackLoop.GenerateAndValidate`][feedback-input] requests predicate selection for the initial prompt and again for retries. [`selectPredicates`][feedback-select] accepts a successful nonempty JIT result; it does not independently reconcile those signatures against the validator's declared set. Retry prompts call the list “Available Predicates (use ONLY these).” [Prompt builder][feedback-render]
4. Domain selection still reads the current corpus and preserves bounded core/domain fallbacks. The semantic path also checks that a retrieved name exists in the current corpus; deleted names are skipped. These are real protections, but existence by name does not establish signature or represented-content identity.
5. Generated rules still go through sanitization, sandbox `HotLoadRule`, and `ValidateLearnedRule`, with attempt/time budgets. Preserve these gates. A wrong prompt reference may cause rejection, auto-repair, retries, or failure; it does not establish that a wrong rule passes them. [Validation path][feedback-validation]

### Credit the active feature branch's embedding fix

On `feat/conversation-brief`, [`CountVectorsByMetadata` and `VectorContentsByMetadata`][feature-count] constrain rows to the attached engine's eligible embedding space. [`vectorEligibleClause`][feature-space] requires an embedding with the matching model stamp; production writes stamp the producing engine. This prevents differently stamped/legacy vectors from being counted or ranked as current-space data.

That is not a predicate-corpus inventory check. A vector can be fully eligible for the **same** model while representing a predicate removed, renamed, re-described, or re-arityed in the current corpus. The witness below holds the model identity fixed and uses correctly stamped vectors.

The feature's [`TestPredicateCorpusReindexesAfterASpaceChange`][feature-test] checks model-filtered counts, manually writes the replacement batch, and checks recall/row count. It does not instantiate a `PredicateSelector` or exercise a same-space corpus upgrade. This describes the test source's scope; it is not a report of a test run or a criticism of its useful store-level guarantees. No general hot-model-switch claim is required here.

## 3. Finding A: inventory cardinality is not current-corpus coverage

[`indexPredicateVectors`][index] compares the total corpus predicate count with the number of stored rows whose metadata says `kind=predicate`. If the latter is at least the former, it returns ready **before loading the current predicate inventory**. The selector then retains a `vectorIndexReady` Boolean for its lifetime. If counts are below the threshold, it loads existing content strings and appends missing current representations; this function does not retire obsolete representations.

An equal count cannot establish equal membership:

```text
old corpus: {p/1, q/1}
new corpus: {p/2, r/1}
persisted same-space rows: {representation(p/1), representation(q/1)}
stored_count = current_count = 2
```

With these conditions, a fresh selector declares indexing complete despite lacking the current `p/2` and `r/1` representations. A description-only change has the same freshness problem even without an arity change. These examples are synthetic inventories, not a claim that these specific names occur in the repository.

The defect is not limited to equal-size upgrades. When the current corpus grows and indexing runs, the selector can add current contents without removing obsolete ones. The feature branch's writer replaces identical predicate **content**, not every historical representation sharing a predicate name; changed arity/description changes [`predicateVectorContent`][representation]. The writer's index cleanup also names the current content. [Batch writer][feature-batch]

**Bounded recall consequence:** retrieval is limited before current-name hydration. Enough highly ranked stale or removed representations can consume the candidate allowance. Skipping missing names afterwards cannot recover a current predicate excluded upstream. This requires those stale rows to rank within the finite allowance; it is not an assertion that every upgraded workspace loses relevant predicates.

## 4. Finding B: stale retrieval metadata overrides authoritative arity

The semantic hydration code performs three consequential operations in sequence:

1. Parse name and arity from persisted vector metadata/content.
2. Fetch the current `PredicateInfo` by that name.
3. Initialize `selectedArity` from `info.Arity`, then replace it with the retrieved positive arity.

It returns that arity with the **current** name/domain/category/description. [Exact source][hydrate]

In the proposed two-corpus witness, a retrieved old `p/1` row therefore becomes a selected `p/1` carrying `p/2`'s current description. [`mergePredicatesWithVector`][merge] keys candidates by `name/arity`, so the stale entry need not replace a correctly selected `p/2`: both can coexist and compete for the cap. [`SelectForContext`][format] formats the selected arity directly into the model-facing signature.

Exact additional conditions for a visible wrong-signature witness:

- The current corpus still contains the name, but its authoritative positive arity differs from the persisted positive arity.
- A same-model eligible old representation survives in the LocalStore and is returned by filtered semantic recall.
- The stale candidate survives the final relevance/cap ordering; use a deterministic fake embedding and adequate cap in a future fixture to make this explicit.
- The caller uses the semantic selector (`SelectForContext` or `SelectForRepair`), rather than only current-corpus domain selection.

Name deletion is a negative control: the current hydration check drops the row rather than advertising the removed name. It can still contribute to the separate pre-hydration recall loss above. A clean new LocalStore and an unchanged corpus are also necessary controls.

### Related bounded edge: valid arity zero is rejected by semantic parsing

[`parsePredicateMeta`][meta] rejects `arity <= 0`, although the current schema contains valid zero-argument declarations such as [`is_multi_step()`][zero-schema]. A correctly indexed zero-arity candidate cannot pass this semantic branch. Domain selection can still include it, so this is **not total unavailability**. Evaluate with a zero-arity predicate absent from the chosen base domains to isolate semantic recovery. Missing/malformed metadata must remain distinguishable from valid integer zero; merely accepting every missing arity as zero is not a repair.

## 5. Proposed inventory and lifecycle contract

### Reader invariant first

Every model-facing predicate signature must come from a current authoritative catalog entry. A vector row is a retrieval handle and scoring witness, not a source of schema authority. Reject an arity/content mismatch or map a stale name to the current entry with explicitly degraded scoring provenance; do not advertise the historical signature as current.

The minimum containment is to hydrate arity only from the authoritative corpus and admit valid zero arity. This prevents the wrong-signature output but does **not** certify index completeness or cure candidate starvation. A current-corpus-only selector is a useful conservative fallback while the index is stale or unavailable.

### Name an inventory, not just a count

Use an inventory identity covering:

```text
corpus identity / schema version
canonical predicate signature and represented-content digest
predicate representation version
embedding-space identity and applicable transformation identity
backing store identity
```

Compute the immutable embedded corpus's digest once rather than hashing it on every query. Preserve the feature branch's embedding identity work; corpus identity is an additional dependency, not a replacement for it. Coordinate with the embedding audit's unresolved full transformation identity rather than inventing a conflicting model identifier.

Certify readiness from exact current membership and successful representation coverage. A raw count, an old completion bit, or a provider-success return is insufficient. Reuse unchanged content's valid vectors, embed only new/changed representations, and report any missing/failed members. Do not reinterpret a partial rebuild as a complete corpus because obsolete rows make the aggregate count large enough.

### Qualify reads before top-K

The current caller's corpus membership must constrain the search population **before its finite result allowance is consumed**. Possible implementations include a corpus-qualified metadata scope or a join/allowed-ID set against the current inventory. Post-top-K filtering is a containment layer, not full recall equivalence.

Retirement should be scoped to owned predicate representations. Do not clear unrelated knowledge vectors, or globally delete all `kind=predicate` rows without establishing which consumers/corpus versions share the store. Obsolete representations may remain stored for reuse/history, but must not rank as members of the current inventory.

### Lifecycle and publication

Bind readiness to `(store, corpus, representation, embedding space)` rather than an unqualified Boolean. A new process must check its current inventory against persisted data; a setter that changes the backing identity must invalidate the corresponding readiness certificate. The production corpus is normally immutable during a selector lifetime, so avoid unnecessary per-query corpus polling.

Index builds should be bounded, cancelable, coalesced for the same identity, and published only for the inventory they actually built. Existing indexing uses a per-selector atomic running flag and `context.Background()`; any redesign must give shared work a clear lifetime rather than accidentally tying it to one canceled caller or leaving it unbounded. A current domain fallback may continue when appropriate, with an explicit incomplete semantic-coverage outcome. Do not widen the set of executable operations or weaken rule validation to compensate for retrieval failure.

A related primary foundation is [Build Systems à la Carte](https://www.microsoft.com/en-us/research/wp-content/uploads/2018/03/build-systems-final.pdf): reuse depends on tracked inputs, not merely the quantity of stored artifacts. Applying that principle to predicate representations is a design inference, not empirical evidence of a CodeNERD performance win.

## 6. Full costs, alternatives, and risks

Let `N` be current corpus members, `R` retained predicate representations, `Delta` changed/new representations, `D` embedding dimensions, and `K` the retrieval allowance.

| Choice | Benefit | Cost / remaining limitation |
|---|---|---|
| Authoritative hydration only | Stops old arity being advertised | Small per-candidate comparison; unchanged coverage/starvation risk |
| Current domain fallback on stale inventory | Simple correctness baseline, no new provider work on the request | Lower semantic recall outside base domains; possible more model retries |
| Full current-inventory rebuild | Simple coverage reference | Up to `N` embedding inputs, startup latency, write/index IO, migration failures |
| Exact inventory reconciliation with vector reuse | Embeds only `Delta`; excludes obsolete candidates | `O(N + R)` planning/diff work, digest and ownership storage, conditional publication complexity |
| Namespaced generations retained concurrently | Avoids destructive shared-store transitions | Storage/retirement overhead; caller-scoped filtering required to avoid mixed generations |

These are planning bounds, not measured timings. Reconciliation may need to read every retained row unless indexes support the intended access path. Predicate representations are short but provider call latency, minimum billing, batch behavior, and index writes can dominate. Strict failure reporting can increase retry/rebuild work; backoff and coalescing must not hide permanent incompleteness. More accurate guidance can use **more** initial tokens or expose more useful predicates; fewer tokens alone is not success.

Measure full-task costs: corpus enumeration/digests, SQL reads, embeddings attempted/succeeded/reused, provider input and latency, vector/index writes, scanned/scored rows, retained bytes, prompt tokens, generation attempts, validator work, repair retries, unresolved outcomes, and human correction. Include cold startup, warm unchanged selection, upgrades, partial failures, and cancellation. No speedup or “most efficient harness” claim is justified from source alone.

## 7. Proposed evaluation — none executed

### Deterministic contract fixtures

Use real SQLite inventory/store fixtures, deterministic fake embeddings, the actual `PredicateSelector`, and capture the prompt passed through the production feedback interface. A mock selector that already emits canonical signatures would miss this defect.

| Fixture | Required observation |
|---|---|
| Same model, equal-size corpus replacement | Current membership is established despite unchanged row count; removed member never ranks in current scope |
| Same name changes arity | Only the authoritative current signature reaches initial/retry prompts; no stale/current signature pair |
| Description/domain/category change with same signature | Scoring representation is current or explicitly degraded; current prose is not paired silently with an obsolete scoring certificate |
| Growing corpus with old representations retained | Exact coverage, not `stored >= expected`; stale rows cannot hide missing new members |
| Removed-name rows outrank current candidates and fill `K` | Current scoped top-K matches a cold current-inventory reference; name skipping alone must fail this control |
| Valid zero arity outside base domains | Semantic recovery works; missing/negative/fractional metadata is not silently treated as a valid signature |
| Unchanged corpus/model/store | Reuses valid vectors, preserves bounded deterministic selection, does not repeatedly rebuild |
| New store or newly constructed selector after restart | Readiness names the actual backing inventory, not an unrelated earlier completion bit |
| Partial embedding/write failure; interruption then resume | Coverage remains incomplete, valid completed work may be reused, no false ready certificate |
| Model-space mismatch and legacy unstamped controls | Existing feature safeguards remain effective; no guessed stamp or foreign-space ranking |
| Downstream rule generated with wrong arity deliberately | Existing sandbox/schema rejection remains intact; prompt repair is not achieved by weakening validation |

Count both newly correct candidates and false current-signature assertions. Verify exact captured prompt signatures before asserting a model benefit. Include build-tag/indexed and brute-force paths where supported, and preserve candidate/cap semantics when comparing results.

### Task-level study, only if separately authorized

Compare the pinned harness, authoritative-hydration containment, stale-index domain fallback, full rebuild, and exact reconciliation at equal total budget. Hold model/version, prompts, tools, corpus version, seeds, and rule-validation budget fixed. Include rule generation and repair tasks requiring changed signatures, newly introduced predicates, non-core semantic discovery, and zero-arity predicates. Independently validate intended rule behavior, not merely successful compilation.

Primary outcomes: correct current vocabulary in actual prompts, verified rule/task correctness, unresolved tasks, and complete task cost. Secondary outcomes: candidate recall at fixed cap, retries, latency distributions, allocations, embedding reuse, storage growth, and replay stability. Count failures/timeouts and report uncertainty across repeated stochastic trials. Reject an optimization that cuts embedding calls while increasing wrong guidance, missing candidates, or failed repairs.

**Checkable exit:** the selector's readiness certificate names the current inventory; same-space persisted corpus drift cannot silently alter the advertised vocabulary; retrieval metadata cannot override authoritative arity; valid zero-arity candidates can participate; and all existing execution/schema gates remain unchanged.

[arch-selector]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/Docs/architecture/prompt/IMPLEMENTED_SPEC.md#L557-L559
[corpus-load]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/predicate_corpus.go#L73-L109
[legislator-wire]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/shards/system/legislator.go#L147-L170
[legislator-generate]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/shards/system/legislator.go#L330-L344
[feedback-input]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/mangle/feedback/loop.go#L147-L195
[feedback-select]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/mangle/feedback/loop.go#L453-L472
[feedback-render]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/mangle/feedback/prompt_builder.go#L83-L91
[feedback-validation]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/mangle/feedback/loop.go#L300-L328
[feature-count]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/store/vector_store.go#L1143-L1210
[feature-space]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/store/vector_identity.go#L75-L85
[feature-test]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/store/vector_identity_test.go#L579-L617
[index]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/prompt/predicate_selector.go#L565-L663
[representation]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/prompt/predicate_selector.go#L665-L684
[feature-batch]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/store/vector_store.go#L306-L383
[hydrate]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/prompt/predicate_selector.go#L520-L562
[merge]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/prompt/predicate_selector.go#L486-L517
[format]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/prompt/predicate_selector.go#L449-L483
[meta]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/prompt/predicate_selector.go#L686-L720
[zero-schema]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/defaults/schemas_intent.mg#L45-L48
