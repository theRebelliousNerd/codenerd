---
doc-class: research-proposal
subsystem: retrieval-context
implementation-status: proposed
last-verified: 2026-10-28
verified-against: 356f65e0e6bb639adaa90ab36212daa714114871
feature-verified-against: c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c
verification-method: static-source-inspection
---

# Retrieval saturation must be an outcome, not a timeout or a complete answer

## Decision

Give a bounded keyword scan an explicit **cap-reached outcome and a joined producer lifecycle**. The current native scanner can exhaust every keyword's hit allowance, retire all readers, and leave the producer waiting for its deadline. The optional ripgrep adapter has the opposite reporting error: it truncates at the same cap, but the caller treats its result as complete.

These are two concrete post-implementation boundary defects, not a request to add retrieval or provenance from scratch. Preserve the existing one-pass native scan, domain/epoch cache, membership filters, retained positive hits, and Mangle-controlled retrieval brief. Go should report the actual acquisition outcome; Mangle continues deciding what evidence and qualifications enter context.

**Evidence limit:** this is source inspection and conditional control-flow reasoning. No implementation code was changed or executed, no tests or benchmarks were run, and no model inference was invoked. The proposed witnesses below were not executed. No production incident, measured delay, or task-success improvement is claimed.

## 1. Source freeze and relationship to earlier audits

Read-only GitHub connector inspection at approximately 14:37–14:43 UTC resolved:

| Branch | Pin |
|---|---|
| `main` | [`356f65e0e6bb639adaa90ab36212daa714114871`][main] |
| `feat/conversation-brief` | [`c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c`][feature] |
| `codenerd-research-docs` | [`50d79ff092254fba36338ed560ba0bae60340c18`][research] |

Both recursive implementation trees were available without truncation. The exact blob identities match on main and feature for the decisive files:

| Path | Shared blob |
|---|---|
| `internal/retrieval/scan_native.go` | `f4e6ab020327909ab60111054b9647b21a1b14b5` |
| `internal/retrieval/search.go` | `bfafa8513e144657a4c8bc5fc474a70e90f303b8` |
| `internal/retrieval/backend.go` | `1b16b62b494ebc3fb7575cbd966821ae0cd17272` |
| `internal/retrieval/keyword_cache.go` | `14dda632836724bae19dd6f13e5c23f57685971e` |
| `internal/retrieval/outcome.go` | `696e69d3f25c0b600a05a351da648482d732d27b` |
| `internal/retrieval/decisions.go` | `91fdc38e1691ed2a1d851bacad4628da62b9021c` |

All three branch heads were re-resolved unchanged at approximately 14:49 UTC before handoff.

The factory, tiered builder and seed file have other branch changes, so their whole-file identity is **not** claimed. Their feature versions were also inspected: the factory still installs the default native retriever; seeding still shares a five-second default deadline; the keyword/import/semantic stages still run sequentially. [Feature factory][feature-factory], [feature seed][feature-seed], [feature stages][feature-stages].

The [root contract][guidance] requires accurate evidence, bounded requests, and reduced end-to-end turns rather than token savings alone. Scoped [prompt][prompt-guidance] and [session][session-guidance] guidance was read; no implementation or architectural instruction was changed.

### Non-duplication

The earlier [retrieval freshness and context-value audit][prior-retrieval] covers negative-result/domain invalidation, semantic coverage/reuse, and first-class degraded outcomes. Importantly, **main's current version includes an “As built (2026-10-27)” addendum** crediting their implementation. This audit does not repeat the old “errors are logged then returned as nil” claim or propose wiring a missing task retriever. It examines the implemented cap-to-outcome boundary that the old audit did not trace.

The [JIT generation audit][prior-jit] concerns prompt-source generations/preflight work; the [ledger-cost audit][prior-ledger] concerns revision observation/prefix accounting; the [workspace search-navigation audit][prior-navigation] concerns lexical evidence becoming structural relations/addresses. None of those is the native producer's all-keywords-saturated transition or the optional adapter's loss of cap status. Existing overflow test definitions are credited below, but they do not exercise either acquisition boundary.

## 2. Existing production journey and protections

1. The system factory constructs `NewSparseRetriever(DefaultSparseRetrieverConfig(...))` and attaches a shared task retriever to both executor and spawner. Default `Backend` is nil, meaning **native**, not ripgrep. [Factory][factory], [configuration][config].
2. `TaskRetriever.Retrieve` checks `issue_retrieval_wanted`, then passes its shared retriever into `SeedIssueFacts`. [Task retrieval][task].
3. Seeding creates a child context with a five-second default budget. The builder executes mentions, keyword search, import expansion, and semantic expansion in order under that context. [Seed deadline][seed], [stage ordering][stages].
4. `SearchKeywords` scans all uncached native keywords in one walk. It retains positive hits when a scan fails, returns partial/unavailable provenance, and does not cache failed keywords. A successful native cap-reached result is intended to be cached with `overflow=true`, so later reads retain partialness. [Search][search], [cache][cache].
5. Outcomes become issue-scoped facts. Policy derives partial/unavailable caveats, and the task brief renders them beside ranked paths. The brief already says these are starting points, not an exhaustive answer. [Outcome facts][outcomes], [policy][policy], [brief][brief].

The default production path has cancellation and returns retained hits. The finding below is therefore **not an unbounded production hang** and not wholesale loss of the retrieval result. A direct search using `DefaultSparseRetrieverConfig` has a 30-second search timeout; a normal task seed imposes the shorter shared five-second deadline. The effective wait is bounded by whichever applicable deadline/cancellation occurs first, not a universal fixed delay. [Configuration][config], [seed deadline][seed].

The optional adapter is reachable through explicit `nerd retrieve --ripgrep`; it is not selected by the inspected default factory. [CLI selection][cli].

## 3. Finding S1: reaching every native hit cap can strand the producer

### Exact source chain

The scanner permits 500 recorded hits per file and 20,000 per keyword. `full(i)` checks the latter bound under the shared mutex. The worker computes whether any keyword remains open and **returns** when none does. Its input channel has capacity 1,000. [Limits][limits], [worker lifecycle][workers].

The synchronous `WalkDir` producer has no corresponding “all keywords full” observation or signal. For each eligible file it selects between sending a path and `ctx.Done()`. It closes the channel and joins workers only **after the walk returns**. [Producer][producer].

Consequently, if all workers have exited and enough eligible files remain to fill the channel, no receiver can free a slot. The producer remains in its send select until the external context is canceled or expires. A buffered channel only postpones this state; it is not a completion protocol.

After timeout, `searchKeywordsNative` returns the accumulated hits with a deadline error. `SearchKeywords` marks every missing keyword failed for a failed shared walk. Its failed-keyword branch precedes the overflow calculation and cache insertion, so those cap-reached hits become `partial/timeout`, not reusable `partial/overflow`. [Scanner termination][termination], [classification][classification].

### Bounded, unexecuted witness

Use an eligible test workspace, one keyword, native backend, and `Parallelism=1`:

- Forty lexically early files each contain 500 word-bounded occurrences of `quasar`; each is below the 4 MiB cutoff and is nonbinary.
- At least 2,000 later eligible files contain no match.
- Search for the one keyword with a deadline long enough to reach the cap before it expires.

After the fortieth matching file, the stored count is 20,000. On its next received path the only worker sees no open keyword and exits. More than the channel's 1,000 remaining slots are needed by the producer; it therefore cannot finish its walk without deadline/cancellation. This construction is deliberately independent of scheduler timing after the one-reader cap is reached. All required files must pass the actual membership and purpose filters.

The application can extract a single lowercase quoted identifier from task prose, so this is not limited to a test-only malformed keyword structure. For example, the quoted identifier in `repair the "quasar" bug` uses the tertiary extractor. [Extraction][extract]. This is a reachability explanation, not a claim that this task or repository was run.

### Effect and limits

- Native retrieval can spend the remainder of its seed budget waiting after its own cap policy says no reader will perform useful work.
- An unchanged repeat query cannot hit a cache entry for those failed keywords, because none was published by this pass. The same condition can repeat while the corpus/query still saturates.
- In the ordinary seed path the keyword stage consumes the shared deadline. Import expansion immediately checks that deadline and stops; later semantic work receives the expired context. This can reduce the candidate coverage available to the same task, beyond the waiting cost. It does not prove a particular missing file or failed user task. [Import check][imports].
- With a directly supplied noncancelable context, the internal helper has no remaining release event under the witness. Public search wrappers install deadlines; describe that internal-helper consequence separately, not as production behavior.

**Counterexamples/controls:** no all-keyword saturation means at least one reader keeps draining; saturation near the end can leave fewer pending files than the channel can hold, allowing the walk to finish; caller cancellation releases the blocked send; the external backend does not use this native queue. One saturated keyword plus another unsaturated keyword must continue scanning for the latter.

## 4. Finding S2: optional ripgrep truncation is labeled complete

`RipgrepBackend.Search` parses and membership-filters a successful subprocess result, then truncates the retained slice to 20,000 entries and returns nil error. The interface returns only `[]KeywordHit` and error; it does not convey whether truncation occurred. [Adapter contract and cap][backend].

`SearchKeywords` states that the external backend has no cap and computes overflow only when `r.backend == nil`. A successful capped ripgrep result therefore keeps the initial `complete/none` status and is cached with `overflow=false`. [Classification][classification].

### Bounded, unexecuted witness

Select the actual ripgrep adapter, run successfully, and have more than 20,000 eligible parsed hits after membership filtering. Place at least some hits in files whose only hits fall beyond the retained prefix. The adapter returns 20,000 and nil; the caller has no source-provided truncation bit and reports complete. A second unchanged query can repeat that classification from the cache until invalidation/expiry.

**Exactly 20,000 is ambiguous** under the present slice-only interface: it might be an exhaustive result or a truncated larger result. A conservative `len >= cap` rule may report partial safely but cannot prove which case happened. An actual `> cap` witness establishes the concrete defect without relying on this ambiguity.

The distinction reaches the brief/report: policy creates caveats for partial/unavailable outcomes, not complete. The generic “places to start reading” hedge remains useful, but does not repair a machine-readable false completeness claim. This does not establish that a model inferred absence or made a bad edit.

**Important limit:** external-backend in-domain changes clear the cache rather than applying the native incremental delta. This audit does **not** claim that the false `overflow=false` value defeats the external change-invalidation path. The established error is false completeness on the original result and its unchanged cache reuse. Main's as-built addendum explicitly documents the external clearing behavior. [Current implementation account][as-built].

**Counterexamples/controls:** a successful adapter result below the cap does not hit this global truncation path; a subprocess failure/deadline follows the existing failure branch; default factory retrieval remains native. This finding does not assert parity of every ripgrep/native matching or per-file-count semantic.

## 5. Proposed acquisition and lifecycle contract

### 5.1 Report terminal state at the producer boundary

Keep a bounded result envelope per keyword, with retained hits and explicit terminal state:

| State | Meaning |
|---|---|
| Exhausted | The declared acquisition domain was traversed under the backend's configured filters/bounds. |
| Cap reached | Retention stopped at its allowance; additional matches may exist. |
| Deadline/canceled | Caller time/authorization ended before the acquisition could finish. |
| Failed | Membership, IO/backend, or protocol failure interrupted the acquisition. |

“Exhausted” must not silently promise coverage of files deliberately excluded by the scanner's declared domain. Do not turn a result count into proof of the backend state. Retain valid hits across partial outcomes.

Carry this fact from `ScanBackend` instead of assuming all non-native adapters are uncapped. A compatible adapter can preserve existing callers while the richer result travels through keyword classification. A conservative cap-length check is a narrower interim repair with explicit false-positive partialness at exact equality, not the final proof of exhaustive acquisition.

### 5.2 Join intentional native early completion

Once every missing keyword has reached its allowance, the native stage must notify the producer and remaining readers of internal bounded completion. The producer must observe that notification both while walking and while blocked in a send. Only the owner of the producer channel closes it; all readers are joined before the scan returns.

Keep intentional saturation distinct from cancellation of the caller's task. A private stop channel or a derived context with an explicit internal cause are mechanisms, not a mandate. Simply canceling the current context and reusing its `Err()` path would relabel overflow as cancellation again. Define precedence when genuine caller cancellation races with saturation, and retain enough provenance not to disguise cancellation or backend failure as successful bounded acquisition.

Do not stop when only one of several keywords is full. No reader is required to wait for new work after all results are capped, but a producer must never wait for consumers that the completion protocol has retired. Raising the queue capacity only changes the corpus size at which the defect appears. Having workers drain-and-discard would avoid the blocked send but still walk the entire unnecessary remainder; it is a correctness baseline, not an efficiency claim.

### 5.3 Preserve current cache and Mangle decisions

- Cap-reached data may be reused only as cap-reached data, with the current domain/epoch rules and existing invalidation restrictions.
- Real failed/canceled scans must not become cacheable complete answers.
- Continue emitting issue-owned outcome facts and deriving the model-facing qualifier in Mangle. Do not hardcode a different selection policy in the scanner.
- Keep bound constants and configured deadlines distinct: a count cap is an intentional coverage decision, not evidence of slowness.
- No wider filesystem membership, tool authority, or embedding provider access is required.

## 6. Primary foundations and limits

The [Go language specification](https://go.dev/ref/spec#Send_statements) explains why a send cannot progress when a buffered channel is full and no receiver remains; its [select semantics](https://go.dev/ref/spec#Select_statements) account for the deadline case releasing this producer. These language rules support the source-derived conditional trace, not its frequency in CodeNERD deployments.

The official [Go pipeline/cancellation guidance](https://go.dev/blog/pipelines) treats downstream early exit as an upstream completion problem and cautions that a larger fixed buffer is not a general solution. That is the transferable lifecycle principle here; no replacement of CodeNERD's Mangle executive is implied.

The standard-library [context documentation](https://pkg.go.dev/context#WithCancelCause) provides a way to retain a cancellation cause if a derived-context design is chosen. Cause propagation alone does not define the application's cap/deadline precedence or join its workers; those remain explicit contracts to design and verify.

## 7. Costs and validation, not measured results

The expected value is not merely fewer scanner milliseconds: it is preserving the remaining retrieval stages' opportunity to supply useful candidates, avoiding unchanged rescans, and keeping completeness claims trustworthy.

Measure separate quantities: eligible/visited/read files; bytes read; retained hits per keyword; cap transitions; queue-block time; terminal cause; cache reads/writes/refusals; seed budget remaining by tier; backend process output bytes; peak memory; full task latency, model turns/tokens, verification success, and human recovery. Do not add these unlike units into a fictitious cost score. Monetary figures require actual provider usage and current pricing; none are estimated here.

An early-stop signal adds synchronization/branching. An outcome envelope changes adapters and fixtures. External truncation currently occurs after subprocess output is buffered, so truthful status alone does not make external acquisition memory-bounded by 20,000 hits; changing that acquisition strategy would require its own cancellation/parsing contract. Benchmark normal small queries as a regression control, not just the saturation case.

### Existing test definitions credited

- `TestMultiKeywordPassMatchesSingleKeywordWalks` compares four terms across five small files; it does not saturate a keyword. [Definition][multi-test].
- `TestSearch_ShouldCapHitsPerFile` establishes one-file retention bounds, not all-reader retirement with a long remaining queue. [Definition][bound-test].
- `TestSearchKeywords_OverflowIsPartial` seeds a cache entry already marked overflow, then tests its readback; it does not acquire a cap-reached native or ripgrep result. [Definition][overflow-test].
- Existing outcome tests cover failed-keyword propagation and task-brief caveats. Those protections should remain; passing their source-defined cases would not establish the new boundary cases. No tests were run in this review.

### Proposed deterministic acceptance cases

1. One native keyword saturates with far more than one channel-capacity of eligible files remaining: terminate from cap completion before caller cancellation; join producer/readers; retain 20,000 hits; report partial/overflow.
2. Repeat unchanged search: same partial qualification survives cache reuse without another full scan.
3. Two keywords, only one saturated: the other continues to the end, including a hit in the last eligible file.
4. All keywords saturate under several worker counts and competing shared searches: no blocked producer, data race, send-after-close, or abandoned reader.
5. Real cancellation before saturation, during send, and racing with the final hit: preserve documented precedence and join behavior; never report exhaustive completion.
6. Exactly-at-cap and cap-plus-one external results: the actual adapter conveys the truncation distinction, or a conservative interim classifier explicitly reports uncertainty at equality.
7. External result beyond the cap, then unchanged cache read: no complete status or missing caveat; actual `--ripgrep` selection is exercised, not only a fake backend with small results.
8. Cap-reached cached data followed by source/membership changes: preserve current domain/epoch invalidation, and preserve external clear-on-change behavior.
9. Production `TaskRetriever` with world/CodeDOM enabled: the native saturation stage leaves a live budget for downstream import/semantic work where time genuinely remains. Trace outcomes through facts to the brief.
10. Small corpus, no match, below-cap matches, and a saturated query near end of corpus: results/ordering/cancellation remain compatible with the declared contract.

Prefer deterministic barriers or controllable traversal seams for lifecycle assertions over timing-only sleeps. A timeout in a test is a failure guard, not the expected success path.

### End-to-end study and stopping criteria

Compare the pinned system, lifecycle-only repair, adapter-outcome-only repair, and both at identical tasks, model/tool configuration, repository state and total budget. Include large high-frequency repositories and ordinary low-frequency tasks. Count unsuccessful and timed-out tasks, not just completed runs. Show candidate recall/coverage, verified task correctness, total latency and resource use; no retrieval microbenchmark alone establishes fewer user-facing turns.

Before implementation, follow the repository's spec-first package/gap workflow. Completion requires source plus executed evidence that (a) bounded completion cannot strand acquisition, (b) actual cap outcomes survive backend/cache/kernel/brief boundaries, (c) cancellation and membership guarantees remain, and (d) the normal task path demonstrates any claimed quality/cost benefit. This research closes none of those implementation or validation obligations.

## Source references

[main]: https://github.com/theRebelliousNerd/codenerd/commit/356f65e0e6bb639adaa90ab36212daa714114871
[feature]: https://github.com/theRebelliousNerd/codenerd/commit/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c
[research]: https://github.com/theRebelliousNerd/codenerd/commit/50d79ff092254fba36338ed560ba0bae60340c18
[guidance]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/agents.md
[prompt-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/prompt/agents.md
[session-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/agents.md
[prior-retrieval]: https://github.com/theRebelliousNerd/codenerd/blob/50d79ff092254fba36338ed560ba0bae60340c18/Docs/audits/retrieval-freshness-coverage-and-context-value.md
[as-built]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/Docs/audits/retrieval-freshness-coverage-and-context-value.md
[prior-jit]: https://github.com/theRebelliousNerd/codenerd/blob/50d79ff092254fba36338ed560ba0bae60340c18/Docs/audits/jit-generation-aware-optimization.md
[prior-ledger]: https://github.com/theRebelliousNerd/codenerd/blob/50d79ff092254fba36338ed560ba0bae60340c18/Docs/audits/context-ledger-prefix-costs.md
[prior-navigation]: https://github.com/theRebelliousNerd/codenerd/blob/50d79ff092254fba36338ed560ba0bae60340c18/Docs/audits/workspace-search-navigation-fidelity.md
[factory]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/system/factory.go#L2602-L2616
[config]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/retrieval/sparse.go#L56-L121
[task]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/retrieval/decisions.go#L294-L370
[seed]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/retrieval/facts.go#L183-L279
[stages]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/retrieval/tiered_context.go#L178-L228
[search]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/retrieval/search.go#L41-L160
[cache]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/retrieval/keyword_cache.go#L21-L115
[outcomes]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/retrieval/outcome.go#L128-L161
[policy]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/defaults/policy/retrieval.mg#L38-L51
[brief]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/retrieval/decisions.go#L222-L256
[cli]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/cmd/nerd/cmd_retrieve.go#L70-L120
[limits]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/retrieval/scan_native.go#L31-L49
[workers]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/retrieval/scan_native.go#L213-L257
[producer]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/retrieval/scan_native.go#L260-L304
[termination]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/retrieval/scan_native.go#L305-L322
[classification]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/retrieval/search.go#L103-L160
[extract]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/retrieval/sparse.go#L249-L267
[imports]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/retrieval/tiered_context.go#L414-L443
[backend]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/retrieval/backend.go#L51-L142
[multi-test]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/retrieval/multi_keyword_pass_test.go#L14-L79
[bound-test]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/retrieval/bounds_test.go#L108-L131
[overflow-test]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/retrieval/invalidation_test.go#L310-L323
[feature-factory]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/system/factory.go#L2614-L2628
[feature-seed]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/retrieval/facts.go#L201-L279
[feature-stages]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/retrieval/tiered_context.go#L206-L257

## As-built status

Verified and built 2026-10-28 on `main` (after `3daf1420`). `scan_native.go`, `search.go` and `backend.go` were unchanged from the audited blobs.

### Claim ledger

| Claim | Verdict | Evidence |
|---|---|---|
| S1: readers return once every keyword is full, but the producer has no saturation signal and blocks on the full channel until the deadline | Real, fixed (confirmed: the witness took 20.0 s, the full deadline, before the fix) | `scan_native.go:226` `saturated`, selected at `:298` and `:321`; `:344` returns a saturated walk without error |
| S1: the deadline error marks every keyword failed, so the capped result is `partial/timeout` and not cached | Real, fixed | the walk now ends without error, and `SearchKeywords` reports `partial (overflow)` and caches it; `saturation_test.go` reads it back from the cache |
| S1: a single saturated keyword must not stop the walk for an open one | Confirmed and pinned | `TestSearchKeywordsNative_AnOpenKeywordKeepsTheWalkGoing` (the open keyword's only hit is in the last file) |
| S1: the seed's shared deadline is spent by the stranded keyword stage, starving import/semantic expansion | Real consequence; removed with S1 | the keyword stage now returns when saturated |
| S2: ripgrep truncates to 20,000 and returns nil; `SearchKeywords` computes overflow only for the native backend | Real, fixed | `backend.go:150` returns `CapReached: true`; `search.go:143` uses each backend's own report |
| S2: exactly 20,000 is ambiguous from a slice | Confirmed; the adapter now reports the cut itself (`> cap`), the native path reports `>= cap` (conservative) | `backend.go`, `scan_native.go` `searchSingleKeyword` |
| External-backend change invalidation clears the cache | Confirmed; unchanged | `keyword_cache.go` |
| Default factory retriever is native; ripgrep only via `nerd retrieve --ripgrep` | Confirmed | `cmd/nerd/cmd_retrieve.go:91` |

### What was built

- Ruling (S1): a private `saturated` channel, closed once under the readers' mutex when the last keyword fills, distinct from the caller's context so saturation is never relabeled as cancellation. Precedence: a walk with every keyword at its cap is a finished bounded acquisition and returns without error even if the deadline also passed (its hits are exactly what a longer walk would keep); otherwise cancellation and walk errors keep their existing reporting. The channel owner still closes `files` and joins every reader.
- Ruling (S2): the backend contract carries the cap outcome (`BackendResult{Hits, CapReached}`); the caller no longer assumes non-native backends are uncapped. All callers and test fakes moved to the new signature.
- Spec: `Docs/architecture/retrieval/IMPLEMENTED_SPEC.md` §5.1 "Cap outcome and producer lifecycle".

### Tests

`internal/retrieval/saturation_test.go`: the audit's witness (40 files filling the cap, 2,500 empty files behind them, one reader) finishes far inside the deadline with 20,000 hits, `partial (overflow)`, no error detail, and a cached overflow on repeat (failed before the fix after 20.0 s); a second open keyword is still found in the last file; a backend reporting `CapReached` is partial on both the fresh and the cached read. `go test -race ./internal/retrieval` passes.

### Left open

- The ripgrep adapter still buffers the whole subprocess output before cutting (§7); bounding its memory needs its own streaming/cancellation contract.
- Acceptance cases 4-5 (several worker counts under competing searches; cancellation racing the final hit) and 9 (production `TaskRetriever` budget left for import/semantic stages) were not built as separate fixtures; the end-to-end study was not run.
