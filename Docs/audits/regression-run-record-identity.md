# Independent identities for regression run records

**Repository:** `theRebelliousNerd/codenerd`  
**Source pin:** `643123c8de26a969c3b0fc6d78902be88b9c25f5`  
**Status:** source-supported finding and proposed contract, reviewed 2026-11-02. No tests, reproductions, builds, application runs, model calls or benchmarks were executed. No runtime code was changed.

## Finding and scope

Two regression runs saved into the same workspace that start during the same UTC second target the same persisted JSON pathname. The writer truncates an existing file rather than allocating a distinct record. Both saves can report success, but `nerd regression list` can later discover only one record for those two attempts. The overwritten run's full output, task results and timing are no longer available through that path.

The timestamp is taken at **run entry**, not at persistence. Therefore long-running suites are not a defense: two independent CLI processes can start their batteries within one second and finish minutes apart, with the later save replacing the earlier record. No simultaneous file writes, malformed battery, clock failure or adversarial input is needed. [Run start][run], [writer][save], [list discovery][list]

This is a bounded **history and evidence-integrity defect**. Each invocation still formats its own in-memory summary and returns its own failure status. The inspected code does not use saved regression JSON to authorize work, accept campaign changes or derive a Mangle success fact. It would overstate the finding to call this a CI exit-code bypass or a model safety failure. [CLI verdict][cli-run], [host boundary][package], [result-fact boundary][safety]

**Decision:** separate run identity from display time, establish non-clobbering publication, and preserve the existing operator-run verdict contract. Do not merely increase the number of timestamp digits or replace the truncating write with an atomic overwrite.

## Actual producer and consumer chain

### The command is wired and saves by default

The root command registers `regressionCmd`, and its initializer registers `run`, `init` and `list`. The run command resolves the workspace from `--workspace` or the current directory, loads the selected battery, and calls `RunBatteryWithOptions`. A custom `--file` changes the battery input, not the run-record directory. [Root registration][root], [workspace and flags][flags], [CLI entry][cli-run]

The CLI evaluates battery policy for an informational operator warning; it then executes the explicitly requested battery. That does not expose the battery as a new agent-facing tool. Once execution returns without a harness error, the CLI calls `SaveRun` unless `--no-save` was supplied. This occurs for both passing and failing task summaries. A persistence error emits a warning but intentionally does not change the task verdict. `--json` changes stdout formatting, not the default saving behavior. [CLI entry and persistence][cli-run]

There is no run-wide workspace serialization in the inspected root pre-run, command or writer. The root pre-run initializes profiling/logging; its logging initializer's mutex is process-local and is released after initialization. The documented sequential-task invariant concerns tasks **inside one battery**, not independently launched runs. The stronger scenario is two separate processes, not concurrent mutation of Cobra's package-level flag variables. [Root pre-run][pre-run], [logging initialization][logging-init], [concurrency documentation][concurrency]

### The writer maps many starts to one address

`Summary` contains `StartedAt`, aggregate counts, duration and task results, but no separate run identity. `RunBatteryWithOptions` initializes `StartedAt` with `time.Now().UTC()` before shell preflight or task execution. [Summary model][summary], [run start][run]

`SaveRun` creates `.nerd/regression/runs/`, then computes:

```text
name = StartedAt formatted as YYYYMMDD-HHMMSS + ".json"
```

It marshals the full summary and calls `os.WriteFile(path, payload, 0644)`. There is no exclusive creation, collision check, suffix allocation, previous-record validation or run identifier in that path. [Run directory and writer][save]

For two distinct starts A and B within the same second:

```text
A.StartedAt = 2026-11-02T22:20:30.100Z
B.StartedAt = 2026-11-02T22:20:30.900Z

path(A) = path(B) = .nerd/regression/runs/20261102-222030.json
```

These are synthetic values for the static argument, not timestamps from an observed incident. Their distinction may remain in their serialized `StartedAt` values, but that does not distinguish the filename.

Go specifies that `WriteFile` truncates an existing file; it also warns that failure during the multi-call operation can leave partial contents. Thus a successful second save can destroy the first complete record without returning a collision error. Partial-write handling is a related publication concern, but the primary witness assumes both writes complete normally. [Go `os.WriteFile` contract](https://pkg.go.dev/os#WriteFile)

### Listing cannot recover the lost attempt

`ListRuns` enumerates non-directory entries ending in `.json` and sorts their paths in reverse lexical order. It does not read another journal, count completed invocations, or maintain versioned contents behind one pathname. The CLI then reads each returned path, unmarshals one `Summary`, computes PASS or FAIL from that summary, and prints its counts. [List discovery][list], [list rendering][cli-list]

Accordingly, a fail-then-pass collision can leave only the passing summary visible; a pass-then-fail collision can leave only the failing one. The surviving verdict is not itself fabricated—it describes the surviving record. The defect is that the listing loses the other attempt while appearing to show the persisted run history.

The text-mode run command can print the same “Run record” address for both invocations. An operator who saved the earlier address can later read another run's data at that address. With `--json`, separately retained stdout could preserve both full summaries; ordinary text output and logging may also preserve parts of the earlier evidence. Those channels are not a version history for the JSON pathname. [CLI output][cli-run]

## Reachability and limits

A minimal source-derived scenario is:

1. Two operator-initiated CLI processes target the same workspace and both leave saving enabled.
2. Each loads a valid, nonempty battery and passes shell preflight.
3. Their `RunBatteryWithOptions` calls begin within one UTC second. The batteries may be different `--file` inputs, and their tasks may take much longer than one second.
4. A finishes and successfully saves summary A.
5. B finishes later and successfully saves summary B under the same derived filename.
6. Listing that directory returns one entry for this timestamp, and reading A's returned path now yields B's summary.

This scenario is conditional, not a measured incidence claim. Startup work may make rapid *sequential* launches uncommon, and the seeded build/vet/unit suite is not a subsecond fixture. Independent launches do not need either whole invocation to finish within the same second. The actual condition is equal formatted **start** time plus the same run directory. [Seeded suite][seed], [run start][run], [writer][save]

Counterexamples and related cases should remain explicit:

- Different formatted start seconds or different workspaces do not collide through this rule.
- `--no-save` produces no persisted history; that is an intentional opt-out, not the defect.
- A loading or shell-preflight error returns before the CLI saves. The witness uses valid runs.
- A later filesystem error can produce a warning rather than silent successful replacement; normal successful writes are sufficient.
- File-write overlap can add partial or mixed-file risks, but none is required to prove sequential replacement.
- Clock rollback can reuse an earlier filename even for starts separated in elapsed time. That is an additional clock-dependent case, not the ordinary witness.
- Programmatic callers can save equal, zero or differently zoned timestamps, because `SaveRun` neither validates nor normalizes `StartedAt`. Those broaden the API's collision domain; the production CLI already supplies UTC, so timezone ambiguity is not needed here.
- Two different battery files in one workspace share the same output namespace. Adding only a battery-name prefix would not protect repeated attempts of the same battery.

The practical severity is moderate for retained experiment and diagnostic evidence, especially when an earlier failed attempt or its full output matters. There is no measured loss rate, no claim that every run is affected, and no demonstrated downstream automatic acceptance vulnerability. Restoring evidence coverage may reveal failures previously absent from history; it must not retrospectively change a process's observed exit code.

## Existing coverage and duplicate screen

`TestSaveRun_AndListRuns` saves one fixed-time summary, reads it back, and expects one listed path. The CLI persistence test similarly runs one battery and checks that list reads one record. These are useful single-record controls, not multi-attempt identity tests. The CLI failure test explicitly disables saving, so it does not cover a failed record being replaced by a later pass. [Store/list fixture][store-test], [CLI persistence fixture][cli-test], [CLI failure fixture][failure-test]

Targeted searches for `SaveRun`, `ListRuns`, `battery.go`, `regression/runs`, timestamp and same-second/collision terms found no matching treatment of this record-identity defect in the inspected QA or audit corpus. The regression wiring document describes timestamped JSON run records, while TODO marks persistence completed. Older IMPLEMENTED_SPEC and gap-analysis text still describe persistence and CLI wiring as absent; that stale inventory must not be repeated as current behavior. [Wiring contract][wiring], [completed persistence item][todo], [older inventory][old-spec]

The repository's existing atomic-file guidance exempts reports written once to a genuinely fresh path. That distinction is useful here: the regression filename is timestamped, but the code does not establish that it is fresh. This audit refines that identity precondition rather than proposing another generic atomic-file utility. [Atomic-file guidance][atomic-guidance]

Snapshot transport/codec audits concern a different writer and consumer. Similar timestamp spellings elsewhere do not establish a duplicate finding or justify expanding this repair to all persistence. This screen is bounded to retrieved current sources and indexed QA/docs, not an exhaustive claim about all historical discussions.

## Proposed record contract

The following is a proposal, not a shipped guarantee.

**Distinct run attempts must not replace each other's successfully published records.** Start time is descriptive metadata; it is not sufficient identity. A successful save returns an address that continues to identify that run while retained.

1. **Allocate identity independently.** Establish a run identifier once for each attempt. Preserve the full start time for display and ordering. A timestamp prefix plus a collision-resistant suffix is one possible layout, not the contract itself.
2. **Enforce no clobber at the filesystem boundary.** The allocation/publication operation must refuse an existing identity or allocate another one atomically across separate processes. A preflight `Stat` followed by truncating write is not that guarantee. Neither is an in-process mutex.
3. **Specify repeated-save semantics.** Saving the same attempt again should either be explicitly idempotent after identity/content validation or return a clear conflict. Do not silently replace an existing different payload. Equal result bytes from distinct attempts must not cause content-hash deduplication to erase an attempt.
4. **Separate complete publication from name allocation.** Decide how readers distinguish a committed JSON record from an in-progress/failed write. If staging is used, stage outside the `.json` discovery set and publish only after checking write/close outcomes. Ordinary atomic replacement still overwrites an existing destination; no-clobber publication needs its own supported primitive or protocol.
5. **State durability honestly.** Successful write/close, complete visibility and power-loss durability are different promises. If crash durability is required, specify file and directory synchronization and platform/filesystem support. A repair must not claim more than its publication primitive provides.
6. **Keep verdict and recording status separate.** Preserve the existing decision that storage failure does not transform a passing task into a failed task. Still report whether the history record was committed, conflicted or unavailable, and return a usable address only for a committed record.
7. **Define listing order and compatibility.** Continue reading legacy timestamp-only JSON files. Specify newest-by-start-time with a deterministic tie-breaker, or another explicit order. Random suffix lexical order is not subsecond chronology. Do not silently label a mixed-format lexical sort chronological without verifying it.
8. **Do not add authority.** Keep this an operator harness and retain its policy warning and non-agent-callable boundary. Record identity does not require new model tools, permission rules, or campaign integration.

The standard library offers exclusive temporary-file creation across programs, but a uniquely allocated temporary name alone does not make a later fixed-destination replacement non-clobbering. The repository's `atomicfile.WriteFile` explicitly replaces its destination; swapping that helper into the current filename scheme would preserve one complete *winner*, not both attempts. [Go temporary-file allocation](https://pkg.go.dev/os#CreateTemp), [existing replacement helper][atomic-write]

RFC 9562 discusses collision resistance and local uniqueness as application design choices. A UUID-style identifier is an available building block, not a substitute for handling an existing destination or an argument for global infrastructure. This workspace requires bounded local record identity; it does not need a distributed ID service. [RFC 9562 sections 6.7 and 6.8](https://www.rfc-editor.org/rfc/rfc9562.html#section-6.7)

## Costs and alternatives

A higher-resolution timestamp reduces common collisions but does not guarantee distinct attempts under repeated clocks, low clock resolution, deterministic callers or two processes. Adding a process ID reduces some conflicts but introduces restart/reuse semantics. Content hashing incorrectly collapses identical outcomes from independent runs unless coupled to an independent attempt identity. Serializing a whole suite around a workspace lock would change operator concurrency and can add minutes of waiting for a small record-allocation problem.

An independent identifier plus exclusive publication adds small metadata and filesystem work, with retries only for actual allocation conflicts. Staging and synchronization add file operations, latency and possible debris cleanup. More correct retention consumes **more disk**, because records previously overwritten are now retained; bounded, explicit retention is a separate policy and must not silently recreate the loss.

Current discovery scans all entries and sorts paths, approximately O(n) directory enumeration and O(n log n) ordering for n records. Sorting by JSON start time instead can add O(total record bytes) reads; retaining an orderable filename prefix with a specified tie-breaker may avoid that cost. The choice needs compatibility controls rather than an unmeasured claim of zero overhead.

No additional model inference, embedding or training is intrinsic to this repair. Better retained diagnostics may prevent reruns or improve experiments, but those benefits are unmeasured. Do not claim token savings from simply dropping earlier attempts.

## Unrun validation and fair evaluation

Use deterministic synthetic summaries and controlled publication interleavings before live CLI timing experiments. Fixed equal-second timestamps make the core identity check nonflaky; a test that happens to launch twice quickly is not sufficient.

| Future control | Required observation |
| --- | --- |
| Same second with distinct fractions | Two distinct attempts remain separately readable with exact counts, outputs and original full timestamps. |
| Exactly equal timestamps | Distinct attempts still receive distinct retained identities, or an explicit non-destructive conflict is reported under the selected contract. |
| Failure then success and reverse | Both records remain visible; listing preserves each verdict rather than presenting only the final writer. |
| Inverted completion order | Address ownership is unaffected by which long-running attempt finishes first. |
| Separate processes | Contending writers to the same directory cannot replace one another; exercise the actual allocation/publication primitive, not only goroutines. |
| Forced identifier collision | Existing bytes remain unchanged; collision handling has an observable bounded outcome. Do not rely solely on random noncollision probability. |
| Repeated save of one attempt | The declared idempotency/conflict rule holds, including changed payload. |
| Read during write and injected failures | Reader sees a committed record or an explicit incomplete/unavailable state, never a claimed successful history row from partial JSON; previously committed records survive. |
| Legacy and mixed names | Old records still load; deterministic ordering is documented and verified for ties, clock rollback and new-name formats. |
| CLI boundaries | Default saving, `--json`, `--no-save`, custom `--file`, failing tasks and preflight failures retain their intended outputs and exit semantics. |
| Windows and Unix filesystems | No-clobber and reader/publication assumptions hold on supported platforms; unsupported guarantees are explicit. |

A later authorized end-to-end check should use the real CLI with harmless, independently identifiable battery outputs, retain each invocation's stdout and exit status, and reconcile attempted runs, successful publication receipts, listed identities and decoded records. Do not replace the actual CLI path with a mock-only success claim. Do not run destructive or expensive seeded suites merely to demonstrate a filename rule.

Compare baseline and candidate with identical summaries, payload sizes, concurrency and filesystem conditions. Report lost/replaced attempts, parse failures, publication errors, latency distributions, bytes retained, allocation retries and cleanup artifacts. Count all attempted evaluations when reporting a task pass rate; otherwise losing failures can bias the experiment's denominator. An improvement in record completeness is the primary result. A broader agent-quality claim would require separately controlled tasks and independent acceptance evidence.

**Closure condition:** each successfully recorded attempt remains independently addressable and attributable under same-second starts, process concurrency and explicit collision tests, while existing task verdicts and permissions remain unchanged. The current source establishes the gap; implementation and execution validation remain future work.

[package]: https://github.com/theRebelliousNerd/codenerd/blob/643123c8de26a969c3b0fc6d78902be88b9c25f5/internal/regression/battery.go#L1-L12
[summary]: https://github.com/theRebelliousNerd/codenerd/blob/643123c8de26a969c3b0fc6d78902be88b9c25f5/internal/regression/battery.go#L124-L138
[run]: https://github.com/theRebelliousNerd/codenerd/blob/643123c8de26a969c3b0fc6d78902be88b9c25f5/internal/regression/battery.go#L227-L295
[save]: https://github.com/theRebelliousNerd/codenerd/blob/643123c8de26a969c3b0fc6d78902be88b9c25f5/internal/regression/battery.go#L405-L434
[list]: https://github.com/theRebelliousNerd/codenerd/blob/643123c8de26a969c3b0fc6d78902be88b9c25f5/internal/regression/battery.go#L499-L519
[cli-run]: https://github.com/theRebelliousNerd/codenerd/blob/643123c8de26a969c3b0fc6d78902be88b9c25f5/cmd/nerd/cmd_regression.go#L37-L102
[cli-list]: https://github.com/theRebelliousNerd/codenerd/blob/643123c8de26a969c3b0fc6d78902be88b9c25f5/cmd/nerd/cmd_regression.go#L127-L162
[flags]: https://github.com/theRebelliousNerd/codenerd/blob/643123c8de26a969c3b0fc6d78902be88b9c25f5/cmd/nerd/cmd_regression.go#L164-L189
[root]: https://github.com/theRebelliousNerd/codenerd/blob/643123c8de26a969c3b0fc6d78902be88b9c25f5/cmd/nerd/main.go#L333-L348
[pre-run]: https://github.com/theRebelliousNerd/codenerd/blob/643123c8de26a969c3b0fc6d78902be88b9c25f5/cmd/nerd/main.go#L122-L168
[logging-init]: https://github.com/theRebelliousNerd/codenerd/blob/643123c8de26a969c3b0fc6d78902be88b9c25f5/internal/logging/logger.go#L172-L205
[seed]: https://github.com/theRebelliousNerd/codenerd/blob/643123c8de26a969c3b0fc6d78902be88b9c25f5/internal/regression/seed.go#L9-L40
[safety]: https://github.com/theRebelliousNerd/codenerd/blob/643123c8de26a969c3b0fc6d78902be88b9c25f5/Docs/architecture/regression/09-SAFETY-AND-INVARIANTS.md#L157-L178
[concurrency]: https://github.com/theRebelliousNerd/codenerd/blob/643123c8de26a969c3b0fc6d78902be88b9c25f5/Docs/architecture/regression/09-SAFETY-AND-INVARIANTS.md#L146-L152
[store-test]: https://github.com/theRebelliousNerd/codenerd/blob/643123c8de26a969c3b0fc6d78902be88b9c25f5/internal/regression/battery_features_test.go#L307-L339
[cli-test]: https://github.com/theRebelliousNerd/codenerd/blob/643123c8de26a969c3b0fc6d78902be88b9c25f5/cmd/nerd/cmd_regression_test.go#L114-L142
[failure-test]: https://github.com/theRebelliousNerd/codenerd/blob/643123c8de26a969c3b0fc6d78902be88b9c25f5/cmd/nerd/cmd_regression_test.go#L82-L103
[wiring]: https://github.com/theRebelliousNerd/codenerd/blob/643123c8de26a969c3b0fc6d78902be88b9c25f5/Docs/architecture/regression/08-WIRING-AND-INTEGRATION.md#L118-L133
[todo]: https://github.com/theRebelliousNerd/codenerd/blob/643123c8de26a969c3b0fc6d78902be88b9c25f5/Docs/architecture/regression/TODO.md#L15-L21
[old-spec]: https://github.com/theRebelliousNerd/codenerd/blob/643123c8de26a969c3b0fc6d78902be88b9c25f5/Docs/architecture/regression/IMPLEMENTED_SPEC.md#L104-L121
[atomic-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/643123c8de26a969c3b0fc6d78902be88b9c25f5/internal/atomicfile/atomicfile.go#L26-L41
[atomic-write]: https://github.com/theRebelliousNerd/codenerd/blob/643123c8de26a969c3b0fc6d78902be88b9c25f5/internal/atomicfile/atomicfile.go#L151-L224

