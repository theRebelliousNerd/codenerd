---
doc-class: deep-dive
subsystem: context-session
implementation-status: partial
last-verified: 2026-10-28
verified-against: 2e0c54ef204997c36e65c91cbb90bc991e63c8c1
verification-method: static-source-inspection
supersedes: []
---

# Keep the context policy; stop rebuilding its expensive intermediate work

**Decision:** optimize the working-context ledger's revision collection and ordered compaction computation without changing which observations the policy retains. Two concrete opportunities deserve measurement: read/parse each source file once per ledger observation epoch, and replace the all-prior candidate relation with an exact ordered-prefix computation. Neither requires weaker freshness, arbitrary context truncation, or turning executive policy back over to the model.

**Evidence status:** source inspection and proposed design only. No implementation, tests, profiling, inference, training or failure injection were executed. Complexity statements below describe source-visible work or specified algorithms, not measured wall-clock gains. The inspected main revision is the full commit in the header; check main again before implementation.

## 1. Scope: context after JIT, not another compiler cache

The [root guidance, North Star](https://github.com/theRebelliousNerd/codenerd/blob/2e0c54ef204997c36e65c91cbb90bc991e63c8c1/agents.md#north-star) requires Mangle to govern retention and eviction, omitted evidence to remain recoverable, and stale evidence not to masquerade as current. The working ledger is where those requirements meet the actual model request.

The [JIT generation-aware study](jit-generation-aware-optimization.md) covers compilation scopes, source generations, selection and rendering reuse. The [prompt-evolution study](prompt-evolution-lifecycle-and-evidence-accounting.md) covers instruction admission and learning. This document instead covers the repeated request-building path after the system prompt is compiled: archived tool observations, current source identity, compaction policy and the complete input window. It does not duplicate the kernel's general cone/index work in [Mangle kernel performance](mangle-kernel-performance.md).

The context documentation already says its old `WorkingSet.Select` funnel was replaced by the ledger. Its [TODO status table](https://github.com/theRebelliousNerd/codenerd/blob/2e0c54ef204997c36e65c91cbb90bc991e63c8c1/Docs/architecture/context/TODO.md#L10-L36) credits recall handles, revision checks, whole-kernel-or-whole-fallback selection and policy-derived retention. Those older open-gap bodies must not be used to claim these mechanisms remain missing.

## 2. What already works and must survive optimization

| Boundary | Inspected source | Contract to preserve |
|---|---|---|
| Production installation | `beginWorkingLoop`, [internal/session/working_context.go:198–250](https://github.com/theRebelliousNerd/codenerd/blob/2e0c54ef204997c36e65c91cbb90bc991e63c8c1/internal/session/working_context.go#L198-L250) | A declared workspace opens a task-private working set and installs recall. This is not a missing-host proposal. |
| Private policy scope | `NewWorkingSet`, [internal/context/working_set.go:46–100](https://github.com/theRebelliousNerd/codenerd/blob/2e0c54ef204997c36e65c91cbb90bc991e63c8c1/internal/context/working_set.go#L46-L100) | Loads schemas, config and working policy; refuses missing required configuration. Observations do not assert permission into the action kernel. |
| Scope-local durable evidence | `Records`, [internal/context/working_store.go:319–335](https://github.com/theRebelliousNerd/codenerd/blob/2e0c54ef204997c36e65c91cbb90bc991e63c8c1/internal/context/working_store.go#L319-L335), and `Read`, [:367–390](https://github.com/theRebelliousNerd/codenerd/blob/2e0c54ef204997c36e65c91cbb90bc991e63c8c1/internal/context/working_store.go#L367-L390) | Ledger metadata reads do not load bodies; recall is scoped and paginated. Do not trade that property for a full-body cache. |
| Element-level freshness | `Revision` and `sameWorkingRevision`, [internal/context/working_set.go:173–235](https://github.com/theRebelliousNerd/codenerd/blob/2e0c54ef204997c36e65c91cbb90bc991e63c8c1/internal/context/working_set.go#L173-L235) | Element identity includes its own parsed bytes; unrelated edits need not stale every element. Equal absent/unavailable sentinels do not prove freshness. |
| Exact round replacement | `Ledger`, [internal/context/working_set.go:268–352](https://github.com/theRebelliousNerd/codenerd/blob/2e0c54ef204997c36e65c91cbb90bc991e63c8c1/internal/context/working_set.go#L268-L352); `ReplaceControlFacts`, [internal/mangle/engine.go:595–620](https://github.com/theRebelliousNerd/codenerd/blob/2e0c54ef204997c36e65c91cbb90bc991e63c8c1/internal/mangle/engine.go#L595-L620) | Round facts replace rather than accumulate, and the engine rederives before answering. Old recall/eviction conclusions must not leak into a new round. |
| Policy-driven compaction | `working_shed_key`, `working_evict`, [internal/context/working_set.mg:286–394](https://github.com/theRebelliousNerd/codenerd/blob/2e0c54ef204997c36e65c91cbb90bc991e63c8c1/internal/context/working_set.mg#L286-L394) | Stale/superseded rows go first; aged candidates are ranked by liveness/hotness, round and arrival; live recalled results are protected by the policy. |
| Stable request prefix | `prepareWorkingRequest`, [internal/session/working_context.go:577–604](https://github.com/theRebelliousNerd/codenerd/blob/2e0c54ef204997c36e65c91cbb90bc991e63c8c1/internal/session/working_context.go#L577-L604) | Existing history is resent without casually rewriting old payloads. Compaction substitutes recall handles; the compiled system prompt remains separate from source observations. |
| Final request safety net | `prepareWorkingRequest`, [:606–640](https://github.com/theRebelliousNerd/codenerd/blob/2e0c54ef204997c36e65c91cbb90bc991e63c8c1/internal/session/working_context.go#L606-L640) | Complete request accounting includes system prompt, tool definitions and messages. Oversize bodies become explicit archived references, or the request fails; no silent truncation. This final limit can require additional archival beyond policy's soft ledger preferences. |

Recent main changes already precompute sort keys and per-fact views in activation scoring. They are not proposed again here, and commit-comment timings are not measurements from this study.

## 3. Opportunity A: one source read and parse per file per observation epoch

`Ledger` deduplicates current-revision work by **entity** (`seenEntity`, `working_set.go:293–313`), not by file. `Revision` splits an element entity into file and element key, reads the entire file, invokes `codemodel.Parse`, and searches its elements (`:200–224`). Therefore q different element entities in one file can cause q whole-file reads and parses during a single ledger pass. A whole-file entity is separately hashed. This is a source-visible repetition, not an assertion that it dominates Steve's actual workload.

The session also asks for the focused file's revision, and asks for revisions when recording restatement state (`updateLedger`, [working_context.go:791–810](https://github.com/theRebelliousNerd/codenerd/blob/2e0c54ef204997c36e65c91cbb90bc991e63c8c1/internal/session/working_context.go#L791-L810)). These calls do not all have to share a snapshot today. Optimizing them must not conceal concurrent source changes behind a permanent path cache.

### Proposed narrow algorithm

All names here are design notation, not existing APIs.

1. Resolve the ledger's unique entities under existing workspace/path authorization. Group them by canonical file identity while retaining each entity key. Do not combine paths merely because their strings look alike.
2. Within this one observation epoch, read each file into one immutable buffer. Compute its file digest and, only if an element revision is required, parse that buffer once using the same parser and version used by current revision semantics.
3. Build a temporary element-key → revision lookup. Return each entity's exact current result: content revision, absent element, unavailable source, or explicit nonfile identity. Never treat two failed reads as evidence of unchanged content.
4. Supply the resulting revision map to ledger fact construction. If session restatement bookkeeping reuses it, explicitly bind that bookkeeping to the same observation epoch; do not label it “latest at return.” A later round must observe again.
5. Close files and release buffers/maps after the round unless a separately authorized content-addressed cache owns them. Cancellation must be checked during work, not only before a large batch. Preserve current read-error and unsupported-language outcomes; do not convert every syntax error into unavailable evidence.

**Partial-model contract:** `codemodel.Parse`'s boolean means the language has a model, not that the whole file parsed successfully. Its `File` can retain well-formed declarations and syntax-error elements while `Err` is set or `Parsed` is false ([model.go, `File` and `Parse`](https://github.com/theRebelliousNerd/codenerd/blob/2e0c54ef204997c36e65c91cbb90bc991e63c8c1/internal/world/codemodel/model.go)). Current `Revision` checks the supported-model boolean, then searches elements; it can therefore return a retained element's valid revision from a partially parsed file. The batch must preserve that exact result, including the existing absent-element outcome when the requested key is not retained. A blanket `model.Err != nil` or `!model.Parsed` rejection would change behavior and make repair context less useful.

For a file f with size Bf and qf referenced elements, the current source path can incur roughly qf times its read/parse cost. Grouping targets one read plus one parse plus O(qf) lookups per file. This does not remove parsing, path checks or hashing, and a single very large file can still be expensive. It also does not create an atomic multi-file filesystem snapshot. Under arbitrary external writers the observation is an interval; a stronger coherence requirement needs immutable source input or the project's cooperative mutation protocol.

Start with per-call grouping, not a long-lived watcher cache. Cross-round reuse adds invalidation, parser-version, symlink/identity and permission obligations. File metadata alone is not content identity. The existing CodeDOM cache can be reused only if it exposes an immutable result for the exact bytes and equivalent element-revision semantics; “there is already a cache” is not sufficient compatibility evidence.

## 4. Opportunity B: compute prefix totals without all prior pairs

During over-ceiling compaction, policy derives candidates with key `(Rank, Round, Seq)` and byte cost. `working_shed_prior` derives each earlier candidate for each later candidate through three lexicographic rules; `working_shed_prior_bytes` then sums those earlier costs ([working_set.mg:321–366](https://github.com/theRebelliousNerd/codenerd/blob/2e0c54ef204997c36e65c91cbb90bc991e63c8c1/internal/context/working_set.mg#L321-L366)).

For m valid candidates with unique ordering keys, this relation contains exactly m(m−1)/2 distinct prior pairs. That is a combinatorial cardinality, not a measured engine allocation count or CPU bound. Indexing may accelerate joins, but cannot make a materialized all-pairs result contain fewer pairs. This work is conditional: below the ceiling `working_still_over` is false, so no shed-key relation is derived. Do not claim quadratic work on every ordinary request.

The decision only needs the exclusive byte prefix, not the full all-prior relation. Let candidates in policy order be c0…c(m−1), with nonnegative byte costs bi. Define:

```text
P0 = 0
Pi = sum of bj for j < i
Need = RemainingBytes - Ceiling
Shed(ci) iff Pi < Need, while the policy says the ledger remains over ceiling
```

This matches the current first-candidate rule plus later prefix test when Need > 0. Zero-byte candidates, exact equality at the threshold, insufficient candidate bytes, and integer overflow must be specified, not left to a shortcut. If all eligible candidates are exhausted while the request remains large, preserve the final full-request guard; do not quietly make protected rows eligible.

### Keep the executive in Mangle

The goal is **not** a Go eviction heuristic. Mangle must still derive candidate eligibility, rank, recency protection, recall protection, ceiling and final eviction. There are two plausible implementation families, neither executed here:

* An engine-supported ordered aggregate/scan whose input key and semantics are declared by policy. Its availability and binding behavior in the exact upstream version need separate confirmation; this document does not claim such a builtin already exists.
* A narrowly typed deterministic host calculation over a frozen, policy-derived candidate relation. Phase one derives the candidate set. The host validates it, sorts by the declared key and computes exact prefix measurements. Phase two asserts an epoch-bound `candidate_prefix` relation, and Mangle applies the unchanged threshold/eviction rule.

The second option adds an integration boundary and possibly a second evaluation. A host sort/scan is just an implementation of the specified relational calculation; it must not add candidates, alter ranking, drop constraints or decide whether evidence is dispensable. Schema ownership must prohibit generated/model-authored prefix facts. Expose the candidate digest, policy/program digest and epoch with the measurements so a stale scan cannot join a new ledger.

### Two-phase contract

```text
CandidateEpoch = hash(policy_version, round_inputs, canonical_candidate_rows)
PrefixMeasurement = (CandidateEpoch, Call, Key, BytesBefore)
Admission requirements:
  one candidate row per Call; unique complete order key
  nonnegative representable Bytes; checked accumulation
  exact same candidate epoch for selection and scan
  no old candidate/prefix facts after round replacement
```

A malformed duplicate-key input should produce an explicit contract error or use a separately specified stable tie-break. Do not silently invent call-ID ordering, since current host facts deliberately measure arrival order. The current session emits one sequential arrival index per ledger entry (`updateLedger`, `working_context.go:744–763`; `Ledger`, `working_set.go:280–287`). A general reusable API still needs to validate its assumptions.

Sort plus sequential prefix is O(m log m) comparison work and O(m) prefix work/storage under the stated data model. If order is already safely maintained, the scan alone is linear. These are planning bounds for the proposed primitive, not an end-to-end speedup: candidate derivation, control-fact replacement, metadata reads and evaluation remain. Another relation such as supersession may become the next bottleneck. Retain the original policy as the semantic reference for permitted future equivalence experiments.

## 5. Do not optimize the wrong budget

The ledger's ceiling measures rendered result bytes; it is not the model's entire token window. `prepareWorkingRequest` evaluates the whole request and can archive more results until it fits or refuse when the remaining instructions/task/catalog cannot fit. Prefix scan optimization must not remove this final guard or reinterpret byte equality as tokenizer equality.

Likewise, reducing host CPU does not necessarily reduce model tokens. If policy decisions and emitted bytes are identical, immediate token counts should be identical too. The expected first benefit is lower request-preparation work. A second-order reduction in timeouts or retries is a hypothesis to measure, not something to count as saved tokens in advance.

Preserve request-prefix stability: do not reorder historical tool-call/result messages simply because the compaction candidate sort has a different order. That order chooses which bodies become handles; it is not the serialization order. Keep current scope-local recall IDs and explicit stale notices. A faster operation that causes more recalls, loses current evidence or breaks provider cache prefixes is not an improvement.

## 6. Complete cost ledger and alternatives

Measure separately:

* IO: authorized path resolutions, source reads, bytes hashed, bytes parsed and archive metadata/body bytes.
* Host: parse count by file and entity, lookup/sort/scan CPU, allocations, working-set lock duration and cancellation latency.
* Logic: control rows replaced, candidate rows, prior-pair rows, prefix rows, evaluation passes and peak fact-store size.
* Request: serialized bytes, token estimates, provider-reported input/output totals and non-additive cached/reasoning subcounts, prefix-cache reuse and full-request guard actions.
* Outcome: verified task completion, required-evidence availability, recalls and their success, retries, latency and human repair.

Compare A current code, B file-grouped revision collection only, C prefix computation only, and D both. Include the extra phase, digesting and relation publication in C/D. If common ledgers have tiny m, the simpler existing logic may win despite its asymptotic shape. If almost every referenced element is in a different file, grouping may buy little. Reject a complex design whose full costs exceed its benefit.

Use the same policy, provider/model, authorized tools, task snapshots, acceptance checks and resource allowance. A run that quietly evicts more evidence is not a fair performance comparison. Before measuring live tasks, compare exact `Evict` and `Restate` decisions and request bytes on deterministic fixtures. Validity takes precedence over cost.

## 7. Proposed validation program, not executed

| Fixture | Required observation |
|---|---|
| Many element observations in one file | One read/parse per observed file epoch; same per-element revision outcomes as the reference on stable input. |
| File and several of its elements observed together | Shared bytes produce the whole-file digest and all requested element revisions without changing their meanings. |
| Edit unrelated element; remove requested element; unsupported language; read error; partially parsed file | Preserve unchanged element identity and existing absent/unavailable outcomes. A retained element in a broken file must keep its current valid revision; a non-retained key must keep the current absent-element result. Syntax errors alone must not make every element unavailable. |
| Mutation during collection | No claim of atomic snapshot; bounded conflict/unstable handling under the declared source model. |
| Ledger below ceiling | No compaction candidates/scan work; no unnecessary second evaluation if the design can safely avoid it. |
| Candidate ranks/rounds/arrival order mixed | Exact current lexicographic ordering and identical eviction set. |
| Threshold equality, zero cost, large costs, exhausted candidates | Exact boundary behavior; no integer wrap; protected evidence not silently admitted as candidates. |
| Recalled live, hot live, stale and superseded observations | Preserve policy priorities and exclusion, including stale/superseded treatment. |
| Duplicate keys, duplicate calls, stale prefix epoch | Reject or apply explicit specified semantics, never mix old measurements into new policy state. |
| Oversized complete request despite ledger selection | Existing handle archival/refusal remains; task and instructions are not silently cut. |
| Restart, cancellation, parser-version change | No stale cross-epoch cache reuse, orphaned work or false current-source label. |

Sweep m over small and large candidate populations; independently vary distinct files, elements per file, file sizes, changed fraction and source-read failures. Report p50/p95/p99 preparation latency, peak memory, lock hold time and relation cardinality. Use paired randomized task assignments and report uncertainty, failures and correct refusals. Task fixtures should include multi-function refactoring, repeated diagnosis in one large file, cross-file integration, and long-running sessions with recalls after writes.

The existing `working_ledger_test.go` is a useful source of policy examples, not proof this proposal passes them. No test was run for this document.

## 8. Primary foundations and build queue

[Blelloch, *Prefix Sums and Their Applications*, §1.1](https://www.cs.cmu.edu/~guyb/papers/Ble93.pdf) defines prefix and exclusive-prefix operations and their linear sequential implementation. That supports the arithmetic replacement for a materialized all-prior relation; it does not certify Mangle integration or imply parallel execution is necessary.

[Mokhov, Mitchell and Peyton Jones, *Build Systems à la Carte*, §4.2.2](https://simon.peytonjones.org/assets/pdfs/build-systems-original.pdf) explains dependency-based verifying traces. Its relevant lesson is that reuse must bind to the inputs actually observed. It does not turn a path-keyed parse cache into a coherent source snapshot.

| Proposed work item | First checkable exit |
|---|---|
| LEDGER-PERF-01 | Revision batch returns identical results on stable input while reading/parsing once per distinct file; failure semantics preserved. |
| LEDGER-PERF-02 | Prefix implementation yields identical policy decisions across ranked/boundary fixtures without materializing all-prior pairs. |
| LEDGER-PERF-03 | Epoch/version admission rejects stale prefix measurements and protects host-owned facts. |
| LEDGER-PERF-04 | Registered request path demonstrates reduced full preparation cost without altered prompt bytes, weaker freshness or extra recall burden. |

These IDs are local research proposals, not adopted repository gaps. The owning context/session specs must be reconciled before any authorized implementation. The guiding principle is simple: retain the policy's meaning while making the data preparation and intermediate representation cheaper.

## As built (2026-10-27)

**Implemented**

* LEDGER-PERF-01 — one read/parse per file per observation epoch. `internal/context/working_revision.go:WorkingSet.revisions` resolves each entity once, groups by resolved path, streams the digest when only the whole file is asked for, otherwise reads the file once and parses it once per language its spellings name, then answers every element by key lookup. Outcomes are exactly the old per-entity `Revision` (digest, element revision, `absent`, `unavailable`, `nonfile`), including a partly parsed file's retained elements; a canceled epoch leaves unread files `unavailable`. `Revision` is now that batch of one. `WorkingSet.Ledger` observes the ledger's entities plus `LedgerRound.Observe` in one call and returns them as `LedgerDecision.Revisions`; `session.updateLedger` binds the focus view and `loop.restated` to that epoch instead of re-reading files after the decision.
* LEDGER-PERF-02 — prefix totals without all prior pairs. `working_shed_prior`, `working_shed_prior_bytes`, `working_shed_has_prior` and `working_shed_first` are gone from `working_set.mg`. `working_shed_run(Pos, Before)` is an exclusive prefix sum along one chain of `3N` positions (`Rank*N + Seq`, N = `working_ledger_len`), stepping by a candidate's bytes or by 0; `working_shed_before` reads it and `working_shed` keeps the unchanged `Before < Need` test (the first candidate has `Before = 0 < Need`, so no separate first rule). It derives `3N+1` rows only when `working_still_over()`. Policy stays in Mangle; no host sort/second evaluation.
* LEDGER-PERF-03 (host contract) — `LedgerRound.validate` refuses an empty or repeated call, a negative size, or a round that decreases along arrival order, the assumptions the scan relies on. `session.mergeRepeatedCalls` carries a reused call id as one row (bytes summed, latest arrival and round) because the request evicts by id.
* Tests: `working_ledger_prefix_test.go` (60 randomized ledgers against the ordered-prefix reference, threshold equality, zero bytes, stale rows, linear row count, nothing below the ceiling, contract refusals), `working_revision_test.go` (batch equals the old per-entity algorithm on digest/element/absent/unsupported/missing/escape/partial-parse fixtures, unrelated-edit identity, cancellation, epoch returned by `Ledger`, benchmark), `session/working_ledger_merge_test.go`.

**Ruled differently**

* Option 1 vs 2 of §4: neither a new engine aggregate nor a host scan. The chain is plain recursive Mangle (`fn:plus`/`fn:mult`), so there is no second phase, no epoch-stamped `candidate_prefix` relation and no host-owned fact to protect; LEDGER-PERF-03's stale-prefix admission is moot. The ordering key `(Rank, Round, Seq)` is computed as `(Rank, Seq)` because the host now guarantees rounds never decrease along `Seq`.
* Duplicate call keys are not a contract error at the session: they are merged where they arise, since eviction is per id.

**Deferred**

* LEDGER-PERF-04 (registered-request cost measurement, A/B/C/D comparison, p50/p95/p99) needs live sessions; not run. No wall-clock speedup is claimed.
* Cross-round parse reuse (a content-addressed CodeDOM cache) was not added: per-call grouping only, per §3.

## As-built note (2026-10-28)

Re-verified on main: LEDGER-PERF-01..03 hold (`working_revision.go`, chain-prefix `working_shed_run`; `go test ./internal/context -run Ledger|Revision|Prefix` passes). Measured LEDGER-PERF-04 A against B on 200 elements of one file: 94.2 ms per-entity reference vs 3.05 ms batched (`BenchmarkWorkingRevisions_*`, same fixture, 20 iterations).
Ruled out: nothing new. Left: C/D end-to-end request-preparation percentiles and recall-burden comparison need live sessions; the old all-pairs shed relation is gone, so C has no reference to benchmark against.
