# 09 — Safety and Invariants: session

> Last verified: 2026-08-09 — truth-corrected for commit 1ad8238e. Runtime canonical first-source precedence is fixed (embedded first, DB deduplicated — stale database copies no longer shadow embedded atoms during prompt collection); boot database reconciliation and stale built-in removal remain OPEN and project-only atoms must be preserved. Pre-delegation world scan was fresh and exposed dirty state; cleanup was allowed because ownership baseline and shell scope enforcement were absent; world became stale only after unreported shell mutations because no incremental refresh ran. See §8 for invariants whose implementation is still missing and §9 for negative acceptance exams that must fail today.

## 1. Constitutional model

Session enforces constitutional safety at tool-call time, not at LLM sampling time.

```
LLM proposes ToolCall
  → allow-list (config / ouroboros)
  → checkSafety → pending_action → permitted match
  → PreflightDestructiveToolCall (optional gate)
  → execute
  → ValidateInteractiveToolResult (optional gate)
```

North star: **default deny** when the gate is enabled and permission is not proven.

## 2. Invariants

### I1 — Fail closed with gate on

If `EnableSafetyGate && kernel == nil` → deny tool.

Tests: `TestExecutor_SafetyGateFailClosed`, constitutional suite in `executor_test.go`.

### I2 — Empty tool name never becomes `/`

Categorical reject before assert.

### I3 — Args nil marshals as `{}`

Matches policy’s empty payload convention; `null` would desync `permitted`.

### I4 — Oversized payloads denied

`maxPayloadBytes = 100KB`. Truncation would break exact match and hide size.

### I5 — pending_action always retracted

`defer RetractFact` after assert (even on deny path after query).

### I6 — Task intents do not clobber interactive routing

Preset runs use `/task_intent_<n>` and retract on exit.

### I7 — Delegated inline tasks do not pollute session history

`CloneForTask` drops history, session context, persister, injected agent config.

### I8 — Dream mode is isolated

`JITExecutor` forces subagent path when `sessionCtx.DreamMode`.

### I9 — Async cancel stops workers

`WaitForResult` cancellation calls `spawner.Stop(taskID)` to prevent zombie LLM spend.

### I10 — Spawn capacity accounts for in-flight construction

`pendingSpawns` reservation + recheck after config generation.

### I11 — Specialist path traversal blocked

Names with `..` or `/` `\` rejected before filesystem read.

### I12 — Piggyback mangle_updates are filtered

Only allowlisted predicates; blocked atoms get constitutional override on envelope.

### I13 — Forced-final tools cannot widen capability

The deadline/iteration final completion may execute only tools present in the reduced final tool definitions. Read-oriented finals receive none; write-oriented finals receive recognized write mutations only. Unoffered calls are refused and cleared from the returned response.

### I14 — nerd.md write protection fails closed

Executor, VirtualStore, and the registry write guard all block a write when `project_forbidden_path` cannot be queried. An unavailable authority is not an allow decision; reads remain available for diagnosis.

### I15 — In-flight edit facts are bounded

`pending_edit` retains bodies up to 16 KiB. Larger bodies become SHA-256 digest metadata, preventing source blobs from inflating the kernel fact store.

### I16 — Write intent has a policy-owned terminal contract

`write_oriented_intent/1` identifies verbs that require a durable file mutation. The executor queries it for forced-final capabilities and hollow-success checks; a real-kernel parity test keeps the conservative degraded-mode fallback identical to the policy facts.

### I17 — Post-edit proof is transport-independent

Every tool-loop terminal path, including natural completion, deadline/iteration forced final, one-shot native fallback, and Piggyback, reaches `verifyCompletedToolTurn`. All must pass build and test gates and run the advisory critic. Piggyback cannot perform a native repair round, so a red compiler or test result fails the turn with its evidence instead of being skipped.

### I18 — Pre-task ownership baseline is immutable (OPEN — 2026-08-09) — NOT YET IMPLEMENTED

At task start the executor must snapshot an immutable ownership baseline:
(tracked vs untracked) × (dirty vs clean) × (owned by this task vs
pre-existing). The snapshot is frozen for the task and is the authority for
every later revert/delete decision. Current reality: no such snapshot exists;
revert/delete decisions are therefore made against a stale or mutated view and
the pre-delegation scan was fresh and exposed dirty state but cleanup was
allowed because this baseline was absent and incorrectly cleaned pre-existing
work in the incident. This invariant is OPEN and must remain marked so until
the snapshot exists and is consulted.

### I19 — Shell effects are fail-closed and scope-checked (OPEN — 2026-08-09) — NOT YET IMPLEMENTED

`run_command` and `bash` are currently classified as non-write tools, so shell
mutations bypass the write-intent and world-refresh gates. Required: every
shell invocation is treated as a potential workspace mutation, its filesystem
effects are detected and attributed to the invoking task, scope-checked against
the task’s permitted mutation set and the I18 baseline, and failed closed
before any success verdict on out-of-scope or unattributable effects. The
incident had a fresh pre-delegation scan that exposed dirty state but
incorrectly cleaned pre-existing tracked and untracked work because this check
did not exist; world became stale only after the unreported mutation. OPEN —
not yet implemented.

### I20 — Pre-existing dirty/untracked paths are never reverted or deleted (OPEN — 2026-08-09) — NOT YET IMPLEMENTED

A path that was dirty-tracked or untracked in the I18 baseline must never be
reverted (`git checkout`/`restore` to HEAD) or recursively deleted (`rm -rf` of
an untracked directory) by the task. Only mutations owned by the task may be
cleaned. Current reality: the pre-delegation scan was fresh, yet the task
incorrectly reverted dirty tracked work and deleted an untracked directory
that it did not own because no baseline/scope-check enforced this invariant.
This invariant is OPEN.

### I21 — Accepted shell mutations refresh the world (OPEN — 2026-08-09) — NOT YET IMPLEMENTED

The one-shot world scan before delegation was fresh but does not refresh after
shell effects. Required: when a shell effect is accepted after I19
scope-check, the executor must incrementally retract and reassert the affected
world facts before any later policy/world query or success verdict, so the
kernel world reflects post-shell reality and is stale only if this refresh is
skipped after a mutation. Without this, later turns decide on a stale
snapshot. OPEN.

## 3. Safety algorithms (reference)

Full narrative: [IMPLEMENTED_SPEC.md §6](IMPLEMENTED_SPEC.md).

There is no `safe_action/1` authorization fallback. A tool call must match the
exact `permitted(Action, Target, Payload)` relation or it is denied.

## 4. Concurrency hazards

| Hazard | Mitigation |
|--------|------------|
| Concurrent Spawn vs max | pendingSpawns + double-check |
| Shared kernel facts | Task-scoped intent IDs; retract pending_action |
| SetSessionContext race on shared executor | CloneForTask for inline tasks |
| SetConfig replacement during execution | Mutex-protected coherent config snapshots |
| CompressMemory holds lock during LLM | Unlock around Compress |
| Concurrent Ouroboros set | Executor mutex |

Kernel-level serialization of concurrent `Assert`/`Query` is a core package concern; session assumes thread-safe Kernel implementation.

## 5. What session does **not** guarantee alone

| Concern | Owner |
|---------|-------|
| Policy Decl and rules | `internal/core/defaults/policy/` |
| Workspace sandbox FS | VirtualStore / tools |
| Network egress policy | tools + config |
| User approval UX | CLI chat |
| Dreamer simulation fidelity | core VirtualStore |

## 6. Security notes

- Specialist configs load from workspace `.nerd/agents/` — treat as trusted workspace content; size-capped.
- Tool results truncated only when fed back to the model (16KB); full result still returned from modular execute to caller path.
- Large `pending_edit` content is represented by a bounded digest; current policy binds the path and treats content anonymously.
- Persist goroutine is best-effort; do not store secrets in free-form response persistence without higher-layer scrubbing.

## 7. Testing anchors

- Real kernel: `check_safety_real_kernel_test.go`, `check_safety_write_large_test.go`
- Boundary: `executor_boundary_test.go`
- Process safety: `TestExecutor_Process_SafetyGate`, `TestExecutor_SafetyGateFailClosed`

## 8. 2026-08-09 task-integrity — what is still OPEN vs what is closed (truth-corrected for 1ad8238e)

| Invariant | Status | Why |
|-----------|--------|-----|
| I1–I17 | Closed / verified | Existing tests and wiring enforce these today |
| I18 pre-task ownership baseline | OPEN | No snapshot exists; pre-delegation scan was fresh and exposed dirty state but cleanup was allowed because baseline absent — revert/delete decisions lacked an authority |
| I19 shell-effect fail-closed scope-check | OPEN | run_command/bash still classified non-write; no detection/attribution/scope-check before success |
| I20 no revert/delete of pre-existing dirty/untracked | OPEN | Incident demonstrated git checkout of dirty tracked work and rm -rf of untracked directory; pre-delegation state was already known, enforcement was missing |
| I21 incremental world retraction/reassertion after accepted shell mutations | OPEN | World became stale only after unreported shell mutations because no incremental refresh ran; pre-delegation world itself was fresh |
| Canonical atom precedence (prompt G9) | PARTIALLY FIXED — runtime FIXED in 1ad8238e (embedded first, DB deduplicated; stale DB no longer shadows at runtime), OPEN for boot DB reconciliation/removal; session-coupled | Runtime collection correct; boot DB still retains 878 stale built-in rows until reconciled/removed; project-only atoms must be preserved |

All OPEN rows above must stay OPEN until the contracts and the negative
acceptance exams in §9 are implemented and the exams pass without losing the
dirty tracked file or the untracked directory. Runtime precedence being fixed
in 1ad8238e does not close these — only boot DB reconciliation/removal and
shell/world enforcement do. Recording current reality as partially-fixed/OPEN
is the correct architecture state; marking aspirational completion would hide
the incident.
## 9. Negative acceptance exams for the 2026-08-09 incident (both OPEN)

These exams must be implemented as tests that reproduce the violation without
destroying the evidence, fail on the current code, and pass only when I18–I21
are implemented:

1. **Git checkout of dirty tracked work — dirty content must survive.** In a
   temporary repo, commit a file, mutate it to dirty, run the task/shell path
   that previously issued `git checkout -- <file>` (or equivalent revert to
   HEAD), and assert the dirty content is preserved byte-for-byte (no revert).
   The executor’s I18 baseline must classify the file as pre-existing dirty
   and I19/I20 must deny the revert / fail closed before success.
2. **Recursive deletion of an untracked directory — directory must survive.**
   In a temporary repo, create an untracked directory tree (e.g.,
   `untracked_dir/nested/file`), run the task/shell path that previously
   issued `rm -rf <untracked-dir>` via run_command/bash, and assert the
   directory tree survives intact. I18 must classify it as pre-existing
   untracked and I19/I20 must deny the delete / fail closed before success.
3. **Accepted shell mutation refreshes the world.** After an accepted
   scope-checked mutation through run_command/bash, assert that an incremental
   world retraction/reassertion occurred and that a subsequent kernel/world
   query reflects the post-shell filesystem state rather than the pre-delegation
   snapshot (I21).

Until these three exams exist and pass, the architecture has not recovered
from the 2026-08-09 incident. See also prompt/03-GAP-ANALYSIS G9/G10,
prompt/12-FAILURE-MODES FM17/FM18, and world/03-GAP-ANALYSIS for the same
incident from the prompt/world perspectives.
