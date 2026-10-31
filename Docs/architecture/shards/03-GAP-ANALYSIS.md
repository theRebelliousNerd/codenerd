# Gap analysis: shards

This document compares the verified current tree with the north star in
[01-VISION.md](01-VISION.md). It does not turn proposals into current claims.

## Executive gap matrix

| ID | Priority | Current evidence | Gap | Target | Card |
|---|---:|---|---|---|---|
| SH-G01 | CLOSED SLICE | canonical manifest now includes the full authorization envelope, drives production configs, rejects duplicate ownership, and passes exact Cortex permission tests | factory/profile/dependency descriptors still have runtime-specific overrides | one descriptor plus explicit environment enrichers | `shards-registration-contract-v1` |
| SH-G02 | CLOSED | batch consultation returns input-ordered successes plus joined partial/total errors; nil spawner fails visibly | cache identity/copy and unified lifecycle receipt remain separate concerns | preserve current honest batch contract | `shards-terminal-outcomes-v1` |
| SH-G03 | CLOSED | both unmapped modes emit one no-handler failure, consume the permission, and do not amplify on a second pass | no remaining route-consumption defect found in this audit | preserve exact terminal helper and regressions | `shards-terminal-outcomes-v1` |
| SH-G04 | CLOSED | observer manager creates fresh run contexts, separates loop/task joins, drains stale events, and passes Start/Stop/Start under race; overflow is counted and assessments are copies (2026-09-25) | nothing renders the drop counter yet | operator diagnostics | `shards-terminal-outcomes-v1` |
| SH-G05 | P1 | predicate ownership is unified; Cortex/chat wire useful but different extras | registrar plus post-registration overrides and auxiliary boot paths are manual factory/profile drift surfaces | one descriptor plus explicit environment enrichers | `shards-registration-contract-v1` |
| SH-G06 | P1 | required-JIT and optional-JIT paths exist | consultation/repair/fallback behavior is inconsistent and not inventoried | atom-first call registry with explicit fallback policy and receipts | `shards-jit-prompt-boundary-v1` |
| SH-G07 | P1 | StartSystemShards uses profiles and `activate_shard/1` | detached queue submission has no generation-wide readiness result | bounded activation plan and required-ready gate | `shards-policy-certified-activation-v1` |
| SH-G08 | P2 | logs, facts, audit, Glass Box, ToolStore, heartbeats | signals do not join boot generation, shard/task/action, readiness, cancellation, terminal state | versioned redacted lifecycle receipt | `shards-terminal-outcomes-v1` |
| SH-G09 | P2 | deterministic technology matcher is tested | path/import/content hints miss semantic fit and evidence freshness | retrieval may nominate candidates, while logic applies typed eligibility/budget gates | supporting backlog |
| SH-G10 | P2 | broad unit/race suite passes | missing negative controls line up with G01-G04 and boot readiness | risk-selected regression and campaign matrix | all cards |
| SH-G11 | CLOSED LOCALLY in `adde9087e867a8e111b59536d8d2274e5c3d7c25`; publication pending | `BaseSystemShard.beginRun` atomically initializes runtime metadata and `StartedAt` provides its snapshot (`internal/shards/system/base.go:330`, `internal/shards/system/base.go:339`). | The baseline's distinct base/derived mutexes previously raced over the embedded timestamp; current storage is private to the base. | Eight new cases, resident-shard/manager race packages, ten repetitions of the previously failing Cortex fixture, full offline suite, sqlite_vec build and vet pass. Broader Cortex race fails a separate unchanged Chrome/mock fixture also reproduced on base; required private-terms scan remains unavailable. | Required timestamp ownership in [09-SAFETY-AND-INVARIANTS.md](09-SAFETY-AND-INVARIANTS.md). |

## Verified non-gaps

- **REJECTED:** restoring large coder/reviewer/tester/researcher Go shard classes.
  Persona execution is intentionally owned by JIT atoms and session execution.
- **REJECTED:** treating profile `Permissions` as runtime authorization.
  `permitted(Action, Target, Payload)` and VirtualStore validation are the
  actual effect boundary.
- **REJECTED:** moving Mangle declarations into `internal/shards`. Core defaults
  own the program assembled by the live kernel.
- **REJECTED:** replacing deterministic route selection with an LLM. Optional
  autopoiesis may propose a route, but learned rules and actual effects stay
  validated and policy-gated.
- **VERIFIED CURRENT:** the ordinary read-file action preserves one executive
  action ID across pending, permission, routing, and execution.
- **VERIFIED CURRENT:** `security_violation` is emitted with the declared three
  arguments, and execution failures elsewhere use the declared two-argument
  `execution_error` contract.

## Dependency order

```text
G01 ownership slice CLOSED -> G05 full descriptor/boot parity
                                  |
                                  +--> G07 activation generation

G02/G03/G04 terminal defect slices CLOSED
             |
             +--> G08 unified lifecycle receipt --> G07 readiness diagnosis

G06 JIT inventory -----------------------------------> observable specialist cognition
```

The order matters. A readiness layer built before registry convergence would
certify whichever duplicate happened to boot. A lifecycle receipt built before
terminal outcomes would merely record ambiguity.

## Safe uplift versus longer horizon

### Safe truth-gap repair

`shards-registration-contract-v1` has completed its predicate-ownership slice:
production consumes the canonical manifest, uniqueness is enforced, and exact
permission target/payload mismatches are negative-tested. The remaining safe
step is a typed factory/profile/dependency descriptor around current enrichers.

`shards-terminal-outcomes-v1` has landed its three bounded fixes: consultation
errors are aggregated without losing successes, observer generations restart
cleanly, and permitted actions are consumed on every terminal route branch.
The remaining slice is a bounded cross-operation lifecycle receipt and missing
drop/retention telemetry.

### Bounded longer-horizon option

`shards-policy-certified-activation-v1` adds a finite Mangle-derived activation
plan and Go-owned boot generation. It must not equate readiness with permission
and must let optional shards degrade without blocking the creative center.

## Measurement plan

| Risk | Falsifying gate |
|---|---|
| ownership split | **PASS for predicate ownership**: manifest uniqueness + exact Cortex envelope; still compare future factory/profile descriptors across boots |
| consultation false success | **PASS**: one success + one error returns both semantics; total failures and nil spawner are non-nil errors |
| router replay | **PASS**: both modes consume exact permission and second cycle cannot amplify |
| observer false liveness | **PASS**: Start/Stop/Start delivers new events under race; still add overflow/drop diagnostics |
| JIT drift | enumerate all LLM call sites and assert atom family/fallback policy; prompt-atom validator stays green |
| readiness ambiguity | delay/fail one required shard and one optional shard; boot result fails only for required and names both outcomes |
| correlation loss | one journey retains boot generation, shard/task ID, executive action ID, and terminal status without raw prompt/secret retention |

## External findings

The code-level packets are:

- `artifact:.corpus-build/findings/shards-predicate-manifest-drift.md`
- `artifact:.corpus-build/findings/shards-consultation-error-loss.md`
- `artifact:.corpus-build/findings/shards-observer-restart.md`
- `artifact:.corpus-build/findings/shards-router-unmapped-replay.md`

The four packets now carry resolved status and the exact regressions that closed
them. They remain incident evidence; they are not open backlog. Boot readiness,
full factory/profile descriptor parity, JIT inventory, drop telemetry, cache
ownership, and result retention remain genuine residuals.


## Required contract: Cancelled synchronous waiter retention

When SpawnWithContext loses its synchronous caller to cancellation, it must atomically relinquish stored results and suppress a late result while retaining execution cleanup and JIT unregistration. Successful waiters and explicit result retrieval retain their normal semantics.

| Gap ID | Capability | Current state / deficiency | Target | Severity | Phase | Dependencies | Exit criteria |
|---|---|---|---|---|---|---|---|
| GAP-SHARDS-CANCELLED-WAITER | Cancelled synchronous waiter retention | `ShardManager.SpawnWithContext` relinquishes the cancelled waiter result and suppresses late retention (`internal/core/shards/manager_spawn.go:139`). | Required contract above | P1 | Implemented; focused tests passed; aggregate verification pending | Existing package interfaces | A gated late-returning shard leaves zero active shard, detached marker, and retained result entries after caller cancellation and execution cleanup. |

**Verified current:** `TestShardManager_CanceledSynchronousWaiterDoesNotRetainResult` (`internal/core/shards/manager_spawn_context_test.go:125`) passes repeatedly, with the full `internal/core/shards` package and race suite also passing.

## Required contract: queued waiter result disposition

A queue worker that abandons a spawned shard's completion wait because its
request context stops must relinquish that shard's retained result through the
manager's existing atomic detach operation. This must cover completion before,
during and after cancellation without skipping execution cleanup or JIT database
release. Disposition belongs to the individual spawn; unrelated active work,
retained results and detach marks must remain intact.

Consuming an execution result is different from abandoning a wait. A delivered
execution error, including `context.Canceled` or `context.DeadlineExceeded`, must
be returned as that execution's result without adding a phantom detach mark.
Ordinary attached and explicitly detached requests retain their API semantics.

| Gap ID | Current deficiency | Required exit |
|---|---|---|
| GAP-SHARDS-QUEUED-WAITER | The completion-wait cancellation branch now relinquishes ownership through `ShardManager.detach` (`internal/core/shards/spawn_queue.go:441`), which suppresses late retention or removes an already-recorded result under the manager mutex. | A real queued synchronous call that is cancelled while an agent delays completion leaves no retained result, detach mark, active shard or JIT lease after completion; already-recorded and racing completions, execution-error consumption and unrelated-work controls pass under race. |

Status: closed for queued completion-wait result disposition. The public
`SpawnWithPriority` path is exercised by
`TestSpawnQueue_CanceledSynchronousWaiterDisposesLateResult`
(`internal/core/shards/queue_cancellation_test.go:53`); completion-before-stop,
execution-error consumption and concurrent publication controls are in the same
file. This does not establish global shutdown, queue restart or live-model
qualification guarantees.

## Executive proposal metadata

GAP-SHARDS-EXECUTIVE-METADATA: syntax/schema validation must not substitute for
proposal confidence. The executive must retain confidence and rationale from
the exact successful model attempt, retain the validated/sanitized rule as the
only executable code, and keep missing, malformed, non-finite or out-of-range
confidence pending under the existing threshold. Metadata from failed attempts
must not leak into a successful retry. Status: closed locally for the executive path by deterministic
handler-to-kernel regression, positive adoption controls and package checks.
The pending fact carries integer percentage confidence, matching the declared
number slot; the proposal and threshold retain their 0..1 ratio scale.
No confidence threshold, constitutional permission, protected-head validation
or feedback retry budget may be weakened by this repair.
