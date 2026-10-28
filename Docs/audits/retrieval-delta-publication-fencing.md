---
doc-class: research-proposal
subsystem: retrieval-context
implementation-status: proposed
last-verified: 2026-10-28
verified-against: 356f65e0e6bb639adaa90ab36212daa714114871
feature-verified-against: c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c
historical-research-inventory: c592ff223cd207fca4c29ae793d5eba57aa4781d
fresh-research-inventory: 548bb1cf93ecbcd0fd3e9c31afd4e43ea2b60c54
verification-method: static-source-inspection
---

# Fence delta refreshes before reusing keyword results

**Decision:** make incremental keyword-cache maintenance a generation-bound publication, not a key-only patch to whatever entry exists when IO finishes. The implemented full-scan writer rejects an old epoch; the implemented delta writer does not. A delayed old-domain delta can therefore alter a correctly populated new-domain cache entry and survive subsequent lookups in that new domain.

This is a focused post-implementation refinement of R1 in the existing retrieval audit. It is not a proposal to add retrieval, query-level invalidation, partial outcomes, or a new indexing system. Keep the single native scan, cache reuse, issue ownership and Mangle-controlled brief.

**Evidence boundary:** source inspection only. No application code, tests, benchmarks, model inference, failure injection or runtime probes were executed. The schedules below are unexecuted source-derived witnesses, not observed user incidents. No CodeNERD implementation or runtime state was changed.

## 1. What is new, and what is already implemented

The historical [retrieval audit][historical-audit] already asks that a scan capture an epoch and not publish after invalidation. Main and the fresh research branch include an [as-built addendum][as-built] crediting `Begin`, epoch-checked `Set`, domain changes, and incremental maintenance. Credit that repair: `Set` actually compares its supplied epoch with the current epoch before publication. [Cache publication][set].

The additional finding is the **other writer**. `beginDelta` returns only keyword strings, while `applyDelta` later obtains each entry by that string and rewrites it. Neither the original domain, the epoch, nor the identity/version of the original entry is checked at delta commit. The comment that an entry stored after the delta began is left alone is only true for a key absent from the captured keyword list; a replacement entry under an old key is not protected. [Delta methods][delta].

The historical [saturation audit][saturation] covers all-reader retirement and ripgrep truncation losing its overflow status. It does not cover late delta publication. [JIT generation work][jit] concerns compiler inputs and kernel snapshots; [ledger costs][ledger] concern revision collection and ordered compaction. Neither is this keyword-cache writer. The fresh research inventory contains the as-built retrieval audit; the historical inventory contains the separate saturation audit. They were both checked rather than treating their differing inventories as evidence that a topic is new.

The root [North Star][root] requires useful, current evidence and reduced full-task cost, with Mangle governing retrieval. This proposal changes acquisition/cache validity, not executive relevance policy.

### Source freeze

Read-only branch resolution at approximately 17:08 UTC gave the full pins in the header. The decisive implementation blobs are identical on main and the only active feature branch, `feat/conversation-brief`:

| Path | Shared blob |
|---|---|
| `internal/retrieval/keyword_cache.go` | `14dda632836724bae19dd6f13e5c23f57685971e` |
| `internal/retrieval/invalidation.go` | `bc92f7b8443057488fd6ee17cbb6a99514935593` |
| `internal/retrieval/search.go` | `bfafa8513e144657a4c8bc5fc474a70e90f303b8` |
| `internal/retrieval/decisions.go` | `91fdc38e1691ed2a1d851bacad4628da62b9021c` |
| `internal/workspace/membership.go` | `e1a4025bffdb71c2453cbd95db4064dccd0efc03` |

`facts.go` and the factory differ between branches, so their entire contents are not claimed identical. Both versions were inspected at the relevant boundaries: they share the retriever between executor/spawner, invalidate before starting the bounded builder, and use the same cache operations. [Main factory][factory], [feature factory][feature-factory], [main seed][seed], [feature seed][feature-seed].

## 2. Exact implemented chain

1. `SearchKeywords` identifies a domain from backend, exclusion patterns and workspace membership revision, calls `Begin(domain)`, and then calls `Get(keyword)` separately for each keyword. A cold scan publishes through epoch-checked `Set`. [Search][search].
2. `Begin` clears the cache and advances its epoch when the domain changes. `Get` checks key existence/TTL, copies the hits and returns the overflow flag; it accepts no requested domain or epoch. [Begin and Get][get].
3. `ApplyFileChanges` calls `beginDelta`, then refreshes membership and reads changed files outside the cache mutex. `beginDelta` advances the epoch and drops overflowed entries but leaves ordinary entries readable. It returns only their keys. [Delta acquisition][acquire], [delta methods][delta].
4. `applyDelta` later looks up each captured key in the current entry map, removes hits matching changed paths, appends the previously read replacement hits, and assigns the result. It has no comparison with the acquisition's domain, epoch or entry version. [Delta commit][delta].
5. `InvalidateFromKernel` holds `r.mu` across its event query, maintenance and cursor advancement. This correctly serializes **invalidators**. `SearchKeywords` does not take that mutex, so a search already past its own invalidation phase can overlap maintenance. The cache's smaller mutex makes each method individually synchronized; it does not make the whole acquire/read/publish sequence atomic. [Invalidator][invalidator], [seed][seed], [search][search].

This is a logical interleaving defect, not an allegation of an unsynchronized Go map access. The [Go memory model](https://go.dev/ref/mem) permits sequentially consistent interleavings of separately synchronized operations; method-level locking alone does not supply the missing multi-operation transaction. Race-detector success would not establish this invariant.

## 3. Central witness: an old delta patches a new domain

The following sequential schedule at the cache-method boundaries is sufficient; no simultaneous instruction execution or scheduler-specific memory reordering is needed:

| Step | Operation | State |
|---|---|---|
| 1 | Cache keyword `quasar` in domain D0. | An ordinary, nonoverflowed entry exists. |
| 2 | Start a delta; capture the keyword list and read an eligible changed file `scratch/x.go` containing `quasar`. Pause before `applyDelta`. | The delta holds old-domain replacement hits. Its epoch advance has already happened. |
| 3 | A search calls `Begin(D1)` for a different membership domain, then completes a full scan and `Set(quasar, empty, newEpoch)`. | D0 was cleared. The correctly populated D1 entry says no eligible file matches. |
| 4 | Resume the old `applyDelta` with its captured key and old replacement hit. | It finds the new D1 entry by the same key and appends the D0 hit. No fence rejects it. |
| 5 | Another search calls `Begin(D1)` and `Get(quasar)`. | The domain is unchanged, so no clear occurs; the contaminated entry is returned. |

The fresh `Set` did not violate its own epoch check. It was the older delta's later write that crossed the boundary. Checking only full scans is therefore insufficient.

### A realistic production interleaving

Use the shared native retriever that the factory installs, a Git workspace, and an **untracked** source file `scratch/x.go` that is initially an eligible member. The untracked qualification matters: adding an ignore pattern does not generally exclude an already tracked file.

- Task A has passed `InvalidateFromKernel` but has not yet reached the keyword search; the seed performs those operations separately.
- A recognized write changes `scratch/x.go`; task B starts invalidation, captures the keyword and reads its new matching bytes into its pending delta.
- Before B commits, the workspace operator changes the root `.gitignore` to exclude that untracked path. No kernel write event for the external ignore edit is assumed.
- Task A now obtains the new membership revision, clears D0 through `Begin(D1)`, scans the new domain excluding `scratch/x.go`, and caches the correct empty result.
- B's old delta commits into A's replacement entry. B then advances its processed write-event cursor.

`Membership.Revision` explicitly tracks the root ignore-file stamps and refreshes when needed; `searchDomain` uses that revision. Thus this is **not simply the already-documented absence of a watcher for external edits**. The new domain was detected and a correct cold result was published; a different cache writer subsequently corrupted it. [Membership revision][membership], [search domain][search].

Later D1 cache reads can continue returning the old-domain path until expiry or another invalidation replaces it. The existing TTL remains a limit; permanent corruption is not claimed. If the ignore edit also produces a kernel event that a later seed processes, that later invalidation can clear the entry sooner. The witness does not require a current request to continue using D0 after detecting D1.

The source supports possible wrong candidate paths or absence claims, not a proven bad patch, confidentiality breach, or tool-permission bypass. The tiered builder ranks cached hits, and Mangle subsequently applies its relevance floor; inclusion in a model-facing brief additionally requires that ranking/policy admit the candidate. Existing execution-time membership and permission checks are independent and must remain. [Tiered consumer][consumer], [brief policy][policy].

## 4. Two adjacent boundaries to specify, without overstating them

**In-progress reads:** `beginDelta` advances the epoch before repairing ordinary entries, and `Get` can still return them. A query begun before a concurrent write may legitimately be linearized before that write; not every overlapping old answer is wrong. But the implementation has no reader-visible pending state or read token that proves which generation a multi-keyword answer belongs to. New-generation admission must not be inferred merely from the epoch having advanced. The central post-commit D1 witness above is stronger and does not depend on a strict latest-at-return promise.

**Same-key replacement:** even without a domain change, expiry, eviction or a fresh `Set` can replace a captured key while a delta is reading. The commit cannot tell that this is a different entry. If the replacement reflects newer source bytes than the pending delta, the late patch can regress it. Replacing an entry with equivalent data is harmless; the conditional defect requires a meaningful intervening source/revision change. An epoch fence plus an entry-version rule makes the intended replacement behavior explicit rather than relying on the key list's comment.

**Controls that do not trigger the central defect:** a serial stable-domain delta; a domain change with no replacement entry under a captured keyword (the old commit finds no entry); a noncaptured keyword created during the delta; or a later invalidation that clears the bad publication. External-backend changes already take the clear-cache path and do not use the native delta. These are useful negative fixtures, not reasons to ignore the native shared-cache schedule.

## 5. Proposed minimal contract

All names below are proposed notation, not existing APIs.

```text
Delta ticket = (domain, mutation epoch, selected entry IDs/versions, changed-path set)
Commit may modify only entries selected by that ticket,
only while its domain and mutation epoch remain current,
and only if the entry version it was based on still matches.
An obsolete ticket cannot modify or delete a replacement entry.
Readers never admit a pending pre-maintenance entry as refreshed current evidence.
```

### Conservative baseline

Clear the affected cache at the recognized change boundary and use the existing epoch-fenced cold-scan path. This avoids exposing half-maintained entries and avoids late delta commits. It sacrifices useful reuse, but is a legitimate correctness baseline. Do not compare a sophisticated delta implementation only against the currently unsafe baseline.

### Generation-bound incremental path

1. Under the cache lock, capture a domain/epoch ticket and immutable references or copies of exactly the entry versions to maintain. Mark them pending or remove them from live admission; do not promote their old contents into a newly valid generation.
2. Perform bounded changed-file reads outside the lock. Retain exact existing scan, membership, count/cap and overflow semantics. Build replacement results privately.
3. On commit, compare the domain, mutation epoch and selected entry versions under the same lock. Publish valid replacements atomically; reject obsolete work without touching entries another operation installed. A domain clear or newer mutation retires the ticket.
4. If readers need a result during maintenance, let policy choose a bounded wait, a cold scan or an explicit unavailable/partial outcome. A reader that starts a cold scan must own a current generation token. Avoid holding the cache mutex across filesystem IO merely to achieve this.
5. Preserve TTL, overflow dropping, file-membership gates and issue-scoped facts. A canceled/failed refresh may safely retire cached answers, but cannot leave an unqualified old result readable as if refreshed.
6. Record whether an event was successfully maintained, conservatively invalidated or remains pending. Cursor advancement is safe after either valid maintenance or safe retirement; it should not imply success solely because a method returned an integer count.

A global mutation-epoch guard plus pending-entry handling is the narrow first implementation. Entry versions become especially important if the design permits concurrent same-epoch cold publication or per-key maintenance. Do not build unnecessary general MVCC machinery before comparing the simple baseline.

Mangle retains authority over whether retrieval runs and what qualified evidence enters the request. Go implements the deterministic cache transaction and reports its state; it must not turn a refresh failure into a creative decision that the missing file is irrelevant.

## 6. Cost and deadline implications

The correctness gain can cost extra cold scans when tickets are rejected. Ticket/version metadata is small relative to hit payloads, but copy-on-write replacement can temporarily retain both old and new results. Waiting increases tail latency; optimistic work can duplicate reads. Measure all of these, rather than calling more cache hits an efficiency win.

The present seed performs `InvalidateFromKernel` **before** creating its bounded builder context. `ApplyFileChanges` and its `scanFile` calls accept no context. Therefore the configured seed timeout is not a bound on this maintenance phase. This is source-visible placement, not a measured timeout violation. A cancellation-aware redesign should include acquisition/maintenance in the owned retrieval budget, safely retire pending entries on cancellation, and avoid merely timing out the caller while leaving publication work running. [Seed deadline][seed], [delta acquisition][acquire].

Keep separate measures: changed files/bytes; cached queries selected for maintenance; cache hits/misses; pending waits; ticket rejection causes; cold restarts; lock hold time; allocations/peak retained bytes; retrieval latency; full model input/output and cached-input usage; tool calls and recovery turns; independently verified task success. No numerical speedup or token saving is established here. The expected value is trustworthy reuse and avoiding wrong candidates/repeated investigation, not a smaller initial prompt at any cost.

## 7. Proposed verification and fair evaluation

The existing `TestCacheDelta_InFlightScanCannotOverwriteAChange` captures an old scan epoch, applies a delta, then checks that `Set` rejects the old scan. That is the already-fixed **opposite direction**. It does not schedule an old delta after a new-domain `Set`. Existing path, deletion, ignore and overflow tests remain relevant and must be retained. [Current test][test].

No checks below were run:

| Fixture | Required evidence |
|---|---|
| Old delta → D1 clear → correct D1 `Set` → old delta commit | Ticket rejected; fresh D1 bytes and metadata remain untouched. |
| Production shared retriever with the two-task/root-ignore schedule | Normal seed and membership paths reach the schedule; subsequent current-domain result matches a forced-cold oracle. Use an untracked file. |
| Same-key expiry/eviction/republication during delta | Old work cannot mutate the replacement entry; no silent regression. |
| Read while selected entries are pending | A declared old snapshot, bounded wait/miss, or qualified degraded result—not an unqualified refreshed claim. |
| Two keyword reads around a generation transition | Documented coherent-generation or explicit partial semantics; no accidental mixed-generation complete answer. |
| Stable-domain serial delta; newly created noncaptured key | Preserve current intended reuse and do not modify unrelated entries. |
| Domain transition without replacement; same-data replacement | Safe no-op or valid behavior without false corruption assertions. |
| Cancel/fail during maintenance; repeated invalidation | Retired/pending/committed state and event cursor agree; no stale publication after cancellation or dropped obligation. |
| Native overflow, deletion, ignore, external backend, no-op changes | Existing cap and clear-on-change contracts preserved. |
| Full task with world/CodeDOM enabled | Outcomes survive cache → candidates → issue facts → policy → brief; independent tool authorization remains unchanged. |

Use deterministic barriers around acquisition and publication, not timing-only sleeps. Test the state machine without a live model first; then use production task entry paths to measure actual quality/cost. A race test is supplementary to these deterministic logical-interleaving tests.

Compare (a) pinned implementation, (b) clear-on-change baseline, and (c) fenced incremental publication under identical source revisions, keyword corpus, backend, task/model/tool settings and total budget. Sweep warm/cold cache, changed fraction, keyword count, parallel tasks, TTL/eviction pressure and membership transitions. Include ordinary serial workloads to expose overhead. Count failed/refused/retried tasks and report uncertainty for stochastic task outcomes; do not evaluate only successful cached calls.

**Checkable exit:** every keyword-cache publication route consumes a valid generation-bound ticket; a superseded delta cannot alter replacement entries; pending maintenance has explicit reader/cancellation semantics; the new deterministic schedules agree with the declared current-domain oracle; and any claimed efficiency benefit is measured end to end without weakened membership or evidence rules. Follow the repository's spec-first workflow before authorized implementation. This draft closes no implementation gap.

## References

[historical-audit]: https://github.com/theRebelliousNerd/codenerd/blob/c592ff223cd207fca4c29ae793d5eba57aa4781d/Docs/audits/retrieval-freshness-coverage-and-context-value.md
[as-built]: https://github.com/theRebelliousNerd/codenerd/blob/548bb1cf93ecbcd0fd3e9c31afd4e43ea2b60c54/Docs/audits/retrieval-freshness-coverage-and-context-value.md#as-built-2026-10-27
[saturation]: https://github.com/theRebelliousNerd/codenerd/blob/c592ff223cd207fca4c29ae793d5eba57aa4781d/Docs/audits/retrieval-saturation-outcomes-and-producer-lifecycle.md
[jit]: https://github.com/theRebelliousNerd/codenerd/blob/548bb1cf93ecbcd0fd3e9c31afd4e43ea2b60c54/Docs/audits/jit-generation-aware-optimization.md
[ledger]: https://github.com/theRebelliousNerd/codenerd/blob/548bb1cf93ecbcd0fd3e9c31afd4e43ea2b60c54/Docs/audits/context-ledger-prefix-costs.md
[root]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/agents.md#north-star
[set]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/retrieval/keyword_cache.go#L94-L115
[get]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/retrieval/keyword_cache.go#L62-L92
[delta]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/retrieval/keyword_cache.go#L148-L210
[acquire]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/retrieval/invalidation.go#L97-L161
[invalidator]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/retrieval/invalidation.go#L183-L282
[search]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/retrieval/search.go#L24-L160
[membership]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/workspace/membership.go#L182-L224
[factory]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/system/factory.go#L2602-L2616
[feature-factory]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/system/factory.go#L2614-L2628
[seed]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/retrieval/facts.go#L206-L244
[feature-seed]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/retrieval/facts.go#L214-L252
[consumer]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/retrieval/tiered_context.go#L370-L406
[policy]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/defaults/policy/retrieval.mg#L25-L51
[test]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/retrieval/invalidation_test.go#L154-L175
