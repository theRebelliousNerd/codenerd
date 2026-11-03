---
doc_class: audit-addendum
subsystem: usage persistence
status: proposed
last_verified: 2026-11-02
verified_commit: 0ed4ff2116f63572ece9aa1aec180dee865f232e
method: static source and documentation inspection only
supersedes: []
---

# Usage persistence retries: keep peer increments out of the local delta

## Finding and scope

**A failed usage save can turn a peer process's already-published usage into this process's apparent new contribution. A subsequent successful retry can then publish inflated aggregates.** The narrow trigger is a successful locked read/merge followed by a persistence failure, while the same tracker remains alive and retries. This does not require a broken lock, concurrent writes during the critical section, another model request, or a duplicate normalized usage report.

At the verified commit, `saveLocked` computes `disk + (current − baseline)` and assigns the result to the live aggregate **before** marshaling and writing the replacement file. Only after successful replacement does it advance the baseline. A failure between those steps leaves the next subtraction operating on imported peer increments as well as locally recorded usage. The save error is explicit; the residual defect is the accounting performed on recovery, not a claim that the failed write reports success. [Merge, early live assignment, persistence, and baseline advancement](https://github.com/theRebelliousNerd/codenerd/blob/0ed4ff2116f63572ece9aa1aec180dee865f232e/internal/usage/usage_tracker.go#L534-L629)

This is a source-derived conditional witness, not a reproduced incident, measured frequency, or billing discrepancy. No tests, application execution, model inference, or repository modifications were performed. This addendum is deliberately narrower than the existing usage audit: it concerns retry ownership of aggregate deltas, not provider report normalization, broker settlement, cache-field omission, or receipt export completeness.

## Existing protections and prior audit boundary

The current implementation already contains important repairs:

- **Process-local shared ownership.** Chat and Cortex can hold distinct, independently closable handles to a shared tracker rather than overwriting one another with independent in-process copies. Sharing is keyed by the absolute workspace path; this is not a promise that every filesystem alias resolves to one identity. [Shared tracker registry](https://github.com/theRebelliousNerd/codenerd/blob/0ed4ff2116f63572ece9aa1aec180dee865f232e/internal/usage/usage_tracker.go#L95-L156) [Per-handle close ownership](https://github.com/theRebelliousNerd/codenerd/blob/0ed4ff2116f63572ece9aa1aec180dee865f232e/internal/usage/usage_tracker.go#L159-L194)
- **Serialized cross-process read/merge/write.** A sibling advisory lock protects the merge and replacement when acquisition succeeds. The non-Windows implementation uses an exclusive `flock`; Windows uses `LockFileEx`. Both acquire synchronously. [Non-Windows locking](https://github.com/theRebelliousNerd/codenerd/blob/0ed4ff2116f63572ece9aa1aec180dee865f232e/internal/filelock/lock_other.go#L17-L34) [Windows locking](https://github.com/theRebelliousNerd/codenerd/blob/0ed4ff2116f63572ece9aa1aec180dee865f232e/internal/filelock/lock_windows.go#L26-L46)
- **Deep baselines and complete merge fields.** Aggregate maps are copied rather than aliased, and cache, reasoning, and partial-report fields participate in addition/subtraction. The prior audit explicitly records the cache/reasoning merge repair and normalized-report/broker controls; this finding does not reopen those defects. [Deep copying and field arithmetic](https://github.com/theRebelliousNerd/codenerd/blob/0ed4ff2116f63572ece9aa1aec180dee865f232e/internal/usage/usage_tracker.go#L302-L378) [Existing as-built usage audit](https://github.com/theRebelliousNerd/codenerd/blob/0ed4ff2116f63572ece9aa1aec180dee865f232e/Docs/audits/detailed-usage-wiring-and-verified-efficiency.md#L113-L138)
- **Replacement rather than truncation.** The writer creates a sibling temporary file, writes and syncs it, closes it, applies its mode, and invokes the platform replacement helper. A successful save then deep-copies the baseline. These protections prevent a different class of partially written JSON failure; they cannot make an earlier mutation of in-memory ownership transactional. [Publication sequence](https://github.com/theRebelliousNerd/codenerd/blob/0ed4ff2116f63572ece9aa1aec180dee865f232e/internal/usage/usage_tracker.go#L586-L629)

Lock acquisition failure and unreadable/unparseable disk state intentionally fall back to writing the process's own data without the merge. Those availability choices have separate tradeoffs; the witness below assumes the lock and disk read both succeed. Neither removing the lock nor weakening these existing repairs is a proposed solution. [Fallback branches](https://github.com/theRebelliousNerd/codenerd/blob/0ed4ff2116f63572ece9aa1aec180dee865f232e/internal/usage/usage_tracker.go#L543-L583)

The older QA document also discusses generic cross-process lost updates. That is not evidence that today's protected healthy merge is defective. Current code and current tests, rather than historical architecture assertions, establish this narrower failure/retry boundary. [Earlier QA concern](https://github.com/theRebelliousNerd/codenerd/blob/0ed4ff2116f63572ece9aa1aec180dee865f232e/.quality_assurance/2026-09-28_QA_usage_tracker.md#L376-L381) [Repository guidance on stale architecture claims](https://github.com/theRebelliousNerd/codenerd/blob/0ed4ff2116f63572ece9aa1aec180dee865f232e/agents.md#L60-L65)

## Production path and observable surfaces

This is a reachable persistence component, not only a standalone helper:

1. Interactive session construction calls `usage.Shared`; Cortex construction also obtains a shared tracker and installs it on its execution machinery. [Chat construction](https://github.com/theRebelliousNerd/codenerd/blob/0ed4ff2116f63572ece9aa1aec180dee865f232e/cmd/nerd/chat/session.go#L138-L147) [Cortex construction](https://github.com/theRebelliousNerd/codenerd/blob/0ed4ff2116f63572ece9aa1aec180dee865f232e/internal/system/factory.go#L1034-L1043) [Executor injection](https://github.com/theRebelliousNerd/codenerd/blob/0ed4ff2116f63572ece9aa1aec180dee865f232e/internal/system/factory.go#L2636-L2643)
2. The chat processing context carries that tracker and session identity. A concrete producer is the OpenRouter non-streaming response path, which sends parsed usage through `reportUsage`. That adapter helper forwards one normalized per-attempt report to `RecordFromContext`. [Tracked processing context](https://github.com/theRebelliousNerd/codenerd/blob/0ed4ff2116f63572ece9aa1aec180dee865f232e/cmd/nerd/chat/process.go#L111-L120) [Concrete provider producer](https://github.com/theRebelliousNerd/codenerd/blob/0ed4ff2116f63572ece9aa1aec180dee865f232e/internal/perception/client_openrouter.go#L199-L214) [Normalized report boundary](https://github.com/theRebelliousNerd/codenerd/blob/0ed4ff2116f63572ece9aa1aec180dee865f232e/internal/perception/usage_track.go#L71-L103)
3. `Record` updates aggregate dimensions and the separate per-turn map, then marks the tracker dirty. The autosave timer is five seconds. A failed autosave restores dirty state, schedules another attempt while open, and logs a warning. Thus the retry can happen without a new report. [Record and dirty state](https://github.com/theRebelliousNerd/codenerd/blob/0ed4ff2116f63572ece9aa1aec180dee865f232e/internal/usage/usage_tracker.go#L637-L709) [Autosave interval](https://github.com/theRebelliousNerd/codenerd/blob/0ed4ff2116f63572ece9aa1aec180dee865f232e/internal/usage/usage_tracker.go#L34-L35) [Automatic retry](https://github.com/theRebelliousNerd/codenerd/blob/0ed4ff2116f63572ece9aa1aec180dee865f232e/internal/usage/usage_tracker.go#L764-L802)
4. Explicit `Flush` retains dirty state on failure but stops its timer and does not itself re-arm it. A later explicit save/flush, new report, handle release, or already applicable autosave path can provide the next attempt. Last-handle release attempts a flush and then closes the tracker even on error; repeated close on the same handle is not a retry mechanism. Cortex calls the tracker close during teardown. [Flush and release behavior](https://github.com/theRebelliousNerd/codenerd/blob/0ed4ff2116f63572ece9aa1aec180dee865f232e/internal/usage/usage_tracker.go#L805-L859) [Idempotent handle close](https://github.com/theRebelliousNerd/codenerd/blob/0ed4ff2116f63572ece9aa1aec180dee865f232e/internal/usage/usage_tracker.go#L177-L194) [Cortex close wiring](https://github.com/theRebelliousNerd/codenerd/blob/0ed4ff2116f63572ece9aa1aec180dee865f232e/internal/system/cortex_close.go#L78-L85)
5. The usage page reads `Stats`; the usage CLI constructs a tracker from workspace storage and exposes aggregates, including JSON output. Therefore a wrong aggregate can reach current-process UI and later readers, without corrupting JSON syntax. [Usage page](https://github.com/theRebelliousNerd/codenerd/blob/0ed4ff2116f63572ece9aa1aec180dee865f232e/cmd/nerd/ui/usage_page.go#L46-L68) [Persisted usage reader](https://github.com/theRebelliousNerd/codenerd/blob/0ed4ff2116f63572ece9aa1aec180dee865f232e/cmd/nerd/cmd_usage.go#L48-L75)

Two independent CodeNERD processes using the same workspace can each have a tracker and baseline despite correct in-process sharing. The minimal schedule only needs the peer's save to finish before the failing process's locked read.

## Minimal arithmetic witness

Use one monotonic integer component, with consistent report dimensions, no counter reset, session pruning, integer overflow, or intervening reports. Let A's last successful baseline be 100. A records 10 locally; peer B independently publishes 20, taking the file to 120.

| Step | Disk | A current | A baseline | Meaning |
|---|---:|---:|---:|---|
| A loads initial state | 100 | 100 | 100 | No pending local contribution |
| A records local 10 | 100 | 110 | 100 | Pending local delta is 10 |
| B successfully publishes peer 20 | 120 | 110 | 100 | Peer contribution is already on disk |
| A reads 120 under the lock and merges | 120 | 130 | 100 | Live assignment is `120 + (110 − 100)` |
| A's temporary-file creation fails | 120 | 130 | 100 | Error returned; replacement never attempted |
| A retries successfully, with no new usage | 150 | 150 | 150 | It publishes `120 + (130 − 100)` |

The correct combined total is **130**, not 150. Immediately after the first failure, A's current value of 130 is a numerically correct combined snapshot; the problem is that its representation no longer separates the imported 20 from the local 10. It is the unchanged baseline that makes the next delta 30. With a fixed peer increment (P), local increment (L), and (f) failed merge attempts before success, this restricted schedule yields `baseline + L + (f + 1)P`.

The strongest fault is a known pre-publication failure such as `CreateTemp` returning an error after the successful read/merge. The code has not called replacement, so this witness does not assume a filesystem's rename failure semantics. Similar pre-replacement branches merit evaluation, but an error from the replacement operation must be analyzed separately: it should not automatically be treated as proof that no publication occurred.

**Countercontrols:** with no peer advance, this particular duplication disappears; with no failing save, the normal merge gives 130; after a successful retry advances the baseline, further healthy saves with no new changes do not continue increasing the total. The defect is not unconditional growth on every save. [Exact state transitions](https://github.com/theRebelliousNerd/codenerd/blob/0ed4ff2116f63572ece9aa1aec180dee865f232e/internal/usage/usage_tracker.go#L579-L629)

## Affected accounting, and what is not established

The same ownership error can affect every additive field in `TokenCounts`: `Input`, `Output`, `Total`, estimated `Cost`, `CacheRead`, `CacheWrite`, `Reasoning`, and `PartialReports`. It applies to `TotalProject` and the credential/provider/model/shard-type/shard-name/operation/session maps, plus `UnpricedTokens`. A peer-only map key can also be counted again: once imported into current but absent from baseline, its entire value is treated as local contribution. Actual changes depend on which fields and keys the peer incremented. Cache/reasoning subcounts are not additional total tokens. [Aggregate schema and semantics](https://github.com/theRebelliousNerd/codenerd/blob/0ed4ff2116f63572ece9aa1aec180dee865f232e/internal/usage/usage_types.go#L54-L97) [Contribution and merge implementation](https://github.com/theRebelliousNerd/codenerd/blob/0ed4ff2116f63572ece9aa1aec180dee865f232e/internal/usage/usage_tracker.go#L350-L512)

The following distinctions matter:

- **Per-turn accounting is separate.** The in-memory turn map is updated from reports and not merged from the persisted aggregate. This finding does not establish inflated `TurnTokens`. Session aggregate queries are a different surface. [Turn ownership](https://github.com/theRebelliousNerd/codenerd/blob/0ed4ff2116f63572ece9aa1aec180dee865f232e/internal/usage/usage_tracker.go#L59-L67) [Turn versus session accessors](https://github.com/theRebelliousNerd/codenerd/blob/0ed4ff2116f63572ece9aa1aec180dee865f232e/internal/usage/usage_tracker.go#L974-L1034)
- **Broker receipts receive original reports.** `RecordFromContext` sends the same report independently to the tracker and observer. Saving an aggregate does not re-emit that report. There is no demonstrated duplicate broker charge, provider request, or model execution here. [Report fan-out](https://github.com/theRebelliousNerd/codenerd/blob/0ed4ff2116f63572ece9aa1aec180dee865f232e/internal/usage/usage_tracker.go#L1038-L1050)
- **The event ring is not a repair ledger.** Cross-process event rings are deliberately not merged, and event retention is optional and bounded. Do not subtract guessed duplicates from an aggregate by assuming that ring is a complete history. [Non-merged events](https://github.com/theRebelliousNerd/codenerd/blob/0ed4ff2116f63572ece9aa1aec180dee865f232e/internal/usage/usage_tracker.go#L554-L558) [Optional bounded event recording](https://github.com/theRebelliousNerd/codenerd/blob/0ed4ff2116f63572ece9aa1aec180dee865f232e/internal/usage/usage_tracker.go#L684-L720)
- **Estimated cost is not invoice truth.** Inflated project estimates can mislead usage review, but the source explicitly labels cost as estimated. This is not evidence of a vendor overcharge, a policy bypass, or actual user harm.

Restart is not automatic reconciliation. Before publication of the inflated retry, restarting can discard A's still-unpersisted local increment; after publication, a fresh tracker simply loads the inflated aggregate as its baseline. Source does not preserve enough independent provenance to reconstruct the true sum generally. [Load and baseline establishment](https://github.com/theRebelliousNerd/codenerd/blob/0ed4ff2116f63572ece9aa1aec180dee865f232e/internal/usage/usage_tracker.go#L250-L273)

## Proposed contract and repair boundary

**Contract:** a save that is known not to have published must preserve the process's pending local contribution exactly. Retrying it against any later valid disk snapshot must not reinterpret imported peer usage as new local usage. A successful publication must establish matching current and baseline snapshots before another local operation can observe them.

The minimal design is a detached publication candidate:

1. While retaining the existing tracker mutex and advisory lock, read disk and calculate the local contribution from the unchanged current/baseline pair.
2. Build and serialize a detached `UsageData` candidate whose aggregate contains the merged value. Ensure candidate map mutation cannot alias either owned current or baseline.
3. On known pre-publication failure, discard the candidate, preserve current and baseline, and retain appropriate retry/error state.
4. After confirmed successful replacement, install the candidate aggregate and its deep baseline together while the tracker mutex remains held.

This is transaction ownership, not just a different subtraction formula. **Advancing the baseline on failure is not a safe shortcut:** it can erase a real pending local contribution without publishing it. Adding another mutex also cannot repair the existing serialized state transition.

The proposed boundary resembles the all-or-nothing staging described in [Go's transaction guidance](https://go.dev/doc/database/execute-transactions), but this tracker uses JSON files, not SQL; a database migration is not required for the narrow repair. If I/O is later moved outside the tracker mutex, the design must separately preserve reports arriving during the save, for example with an owned pending generation. Publishing an old candidate over newer local records would introduce a different loss.

**Ambiguous publication requires an explicit policy.** Staging fixes the demonstrated pre-publication case; it does not alone provide exactly-once retries after an uncertain replacement result. The non-Windows helper delegates to `os.Rename`, while Windows uses a distinct bounded-retry replacement implementation. Linux documents an NFS case where rename failure does not prove the rename did not occur. Do not infer an unknown outcome solely from an error's presence or absence across all platforms. [Non-Windows replacement](https://github.com/theRebelliousNerd/codenerd/blob/0ed4ff2116f63572ece9aa1aec180dee865f232e/internal/atomicfile/replace_other.go#L1-L11) [Windows replacement flow](https://github.com/theRebelliousNerd/codenerd/blob/0ed4ff2116f63572ece9aa1aec180dee865f232e/internal/atomicfile/replace_windows.go#L86-L152) [Linux rename semantics](https://man7.org/linux/man-pages/man2/rename.2.html)

If support requires recovery from ambiguous outcomes, the publication protocol needs identifiable ownership, such as persisted writer/generation contributions with deduplication, or an explicit unknown-outcome reconciliation state. Comparing totals alone cannot establish whose increment was committed. Such a scheme needs restart identity, retention, migration, and compaction rules; it is a larger proposal, not a capability of today's bounded event ring. [AWS's primary account of retry-safe APIs](https://aws.amazon.com/builders-library/making-retries-safe-with-idempotent-APIs/) explains why ambiguous outcomes need intent/identity and why recording a deduplication token must be atomic with its effect. That is design context, not evidence that CodeNERD is a distributed API or that idempotency keys are mandatory for the known pre-publication staging fix.

Atomic namespace replacement also differs from power-loss durability. Linux notes that syncing a file does not necessarily sync the containing directory entry. Neither this addendum nor candidate staging promises universal crash durability, filesystem compatibility, or reconstruction of already inflated records. [File versus directory sync](https://man7.org/linux/man-pages/man2/fsync.2.html)

## Costs, failure handling, and proposed evaluation

The narrow repair adds ownership discipline, not model work. It still reads and parses the whole persisted file, merges maps, serializes JSON, writes/syncs a replacement, and holds the tracker mutex and cross-process lock through I/O. Record/UI callers may wait behind this work; peer writers also serialize. Detached staging retains candidate memory alongside owned state and payload. Existing merge code already allocates copied maps, so an extra full copy is not necessarily unavoidable, but allocation count and peak retained bytes should be measured rather than assumed negligible. Broader deduplication metadata would increase disk size, parsing work, schema maintenance, and lifecycle complexity.

Retries consume I/O and lock time even when no new tokens arrive. Disabling retries avoids neither lost pending usage nor accurate recovery. Backoff or health reporting may help operations but does not fix arithmetic. A durable-directory-sync requirement would add platform-specific code and latency; it should have its own stated contract. Any observability should distinguish recorded, pending, published, and failed/unknown publication without logging prompts or credentials.

**Proposed evaluations only; none were run for this audit:**

| Case | Controlled setup | Acceptance evidence |
|---|---|---|
| Healthy controls | One writer; two writers without faults; repeated saves without new reports | Exact sum; no changes after settled baseline; current performance recorded |
| Primary witness | Start at 100; A records 10; B publishes 20; inject one post-merge `CreateTemp` failure, then recover | Failed save leaves disk 120 and A's local delta 10; retry publishes 130 |
| Repeated failure | Same witness, several known pre-publication failures | Local contribution invariant across every failure; no multiplication by retry count |
| Changing state | Peer saves and A records additional reports between retries | Each distinct report contributes once; no lost locally pending reports |
| Field/key coverage | Shared keys and peer-only keys; all eight fields, all aggregate dimensions, unpriced counts | Exact integer counts; specified floating-point tolerance for estimated cost |
| Lifecycle | Deterministic autosave scheduling, explicit flush, non-final handle release, final close failure | Error/dirty/timer behavior documented; no assumption that repeated close retries |
| Isolation | Two independently started processes, plus an in-process Shared-handle control | Real lock/ownership behavior verified; same-process test not mislabeled cross-process |
| Publication boundary | Separate faults before replacement from platform-specific uncertain replacement outcomes | Known failure and unknown outcome handled according to distinct contracts |
| Consumer separation | Observe aggregate UI/CLI, per-turn totals, and broker reports/receipts for the same fixed input | Correct aggregate recovery without duplicate report emission or altered turn accounting |
| Recovery and cost | Fresh reader after failure/success; map-size and writer-count sweep | Recovery limits explicit; wall time, lock wait, allocations, peak memory, bytes and sync count reported |

Use identical predetermined reports, key cardinalities, fault stages, and filesystem conditions for current and candidate implementations. Compare healthy overhead as well as failure correctness; report OS/filesystem and lock behavior. Keep pruning/reset/overflow out of the minimal arithmetic control, then evaluate their interaction separately rather than allowing them to obscure the retry witness.

Existing tests already cover healthy peer advancement, load/save baselines, cache/reasoning preservation, replacement durability properties, flush/close ownership, and concurrent local recording/flushing. The inspected cases do not combine a successful peer import with a known post-merge publication failure and retry. Extend that coverage rather than discarding it, and retain an independent-process acceptance test in addition to deterministic fault injection. [Existing merge and baseline tests](https://github.com/theRebelliousNerd/codenerd/blob/0ed4ff2116f63572ece9aa1aec180dee865f232e/internal/usage/crossprocess_test.go#L82-L209) [Cache/reasoning preservation test](https://github.com/theRebelliousNerd/codenerd/blob/0ed4ff2116f63572ece9aa1aec180dee865f232e/internal/usage/report_test.go#L87-L117) [Close/flush controls](https://github.com/theRebelliousNerd/codenerd/blob/0ed4ff2116f63572ece9aa1aec180dee865f232e/internal/usage/durability_test.go#L233-L323) [Concurrent local recording/flush control](https://github.com/theRebelliousNerd/codenerd/blob/0ed4ff2116f63572ece9aa1aec180dee865f232e/internal/usage/durability_test.go#L496-L529)

**Exit criterion:** a known failed publication cannot change the identity or amount of the pending local contribution, and the first successful recovery publishes each writer's contribution once while preserving existing report, lock, baseline, and lifecycle protections. No efficiency improvement is claimed until this correctness contract and its measured costs are evaluated.

