# 03 — Gap Analysis: session

> Last verified: 2026-08-09 — true-up for the 2026-08-09 task-integrity incident (prompt/03-GAP-ANALYSIS G9/G10, prompt/12-FAILURE-MODES FM17/FM18, world/03-GAP-ANALYSIS, session/09-SAFETY-AND-INVARIANTS). Section 7 records current reality as OPEN; do not mark G9/G10 closed until the canonical-precedence and temp-repo negative exams exist and pass.

## 1. Spec vs reality matrix

| Desired behavior | Reality | Gap severity |
|------------------|---------|--------------|
| Universal clean loop | `Process` / `ProcessWithIntent` live | **None** |
| JIT specialization | Compiler + ConfigFactory wired | **None** |
| Constitutional default deny | `checkSafety` fail-closed | **None** |
| Interactive executive gate | Type-assert seam on VirtualStore | **None** (depends on VS implementing interface) |
| Multi-turn tools all providers | Native `ToolResultsProvider` yes; Piggyback single-round | **High** |
| No planning-only false success | `intent_requires_tool_call` + nudge | **Low** residual (kernel policy must exist) |
| Task isolation | CloneForTask + task intent IDs | **None** |
| Cross-session memory | Persister optional; atomsJSON empty | **Medium** |
| Piggyback memory → cold store | Assert/log only | **Medium** |
| Ouroboros auto-generation from missing_tool_for | Detect + log; generation elsewhere | **Low** (wiring external) |
| Spawn auto-start consistency | Spawn does not start; SpawnSpecialist/async do | **Low** |
| Completion signaling | Poll 100ms | **Low** |
| Empty AllowedTools unrestricted | Nil/empty now denies every tool | **Closed 2026-07-13** |
| Package README accuracy | Stale slogans | **Low** (docs) |
| Pre-task ownership baseline + shell-effect safety (2026-08-09) | No baseline snapshot; run_command/bash classified non-write and bypass write gates; fresh dirty state incorrectly cleaned | **OPEN — Critical (task-integrity)** |
| World refresh after shell effects (2026-08-09) | One-shot world scan; no incremental retraction/reassertion on accepted shell mutations | **OPEN — Critical (task-integrity)** |

## 2. Priority backlog (engineering)

### P0 — correctness / safety — 2026-08-09 task-integrity incident (OPEN, do first)

1. **Capture an immutable pre-task ownership baseline at task start.** Snapshot (tracked vs untracked) × (dirty vs clean) × (owned by this task vs pre-existing) and freeze it for the task. Every later revert/delete decision must consult this baseline, not a later mutated view.
2. **Detect, attribute, scope-check, and fail closed before success on every run_command/bash effect.** Treat every shell invocation as a potential workspace mutation; detect filesystem effects, attribute them to the invoking task, scope-check against the task’s permitted mutation set, and deny success on any out-of-scope or unattributable effect.
3. **Forbid revert of pre-existing dirty tracked work and delete of pre-existing untracked paths.** `git checkout`/`restore` of a dirty tracked file that was dirty in the baseline and `rm -rf` of an untracked directory that was untracked in the baseline are violations, even if “cleanup” would otherwise be convenient.
4. **Wire accepted mutations to incremental world retraction and reassertion.** When a shell effect is accepted, incrementally retract and reassert the affected world facts before any later policy/world query or success verdict, so the kernel view reflects post-shell reality. One-shot pre-delegation scan alone is insufficient.
5. **Add both negative acceptance exams in a temporary repo** — (a) git checkout of dirty tracked work must not lose the dirty content; (b) recursive deletion of an untracked directory must not delete it. Both must currently fail and pass only when 1–4 are implemented.
6. **Enforce canonical prompt-atom precedence (coupled with prompt/03-GAP-ANALYSIS G9):** boot reconciliation must replace 878 stale corpus.db built-in copies with the 888-ID embedded canonical atoms while preserving project-only atoms. Add the seeded temp-corpus.db exam.

### P0 — correctness / safety (pre-existing)

7. Ensure production VirtualStore always implements `InteractiveExecutiveGate` when destructive tools are enabled (wiring audit, not session-only).
8. Keep fail-closed tests green; never reintroduce nil-kernel allow under gate-on.

### P1 — tool protocol completeness

3. Multi-iteration Piggyback tool feedback (re-issue envelope with tool results).  
4. Preserve empty AllowedTools as “no tools”; any bootstrap capability must be an explicit validated envelope.

### P2 — memory & persistence

5. Populate `atomsJSON` on `StoreSessionTurn`.  
6. Wire memory operations to cold storage / session compressed state.  
7. Use `StoreCompressedState` when SubAgent compresses.

### P3 — lifecycle polish

8. Unify Spawn vs SpawnSpecialist start semantics (or document contract firmly).  
9. Replace Wait polling with event/channel if contention becomes hot.  
10. Align package README with Spawner/TaskExecutor reality.

## 3. Non-gaps (do not “fix”)

| Observation | Why not a gap |
|-------------|----------------|
| No local `.mg` | Correct: session is runtime, policy is global |
| Spawner exists despite “no spawn” slogan | Slogan is anti-domain-shard; Spawner is intentional |
| Baseline prompt on JIT failure | Deliberate degrade, not silent success with wrong tools |
| `safe_action/1` classification | Correctly insufficient; only exact `permitted/3` authorizes |
| Shared kernel across SubAgents | Intent IDs + retract; required for shared policy world |

## 4. Consumer asymmetry

| Path | Assembly |
|------|----------|
| Normal Cortex boot | `system.factory` `initFinalExecutors` |
| Campaign cobra | Rebuilds Executor/Spawner/JITExecutor in `cmd_campaign.go` |

**Gap:** dual assembly can drift (timeouts, token budgets, persister, ouroboros registry). Prefer shared factory helper long-term.

## 5. Test gaps

See [10-TESTING-ALIGNMENT.md](10-TESTING-ALIGNMENT.md).

Notable: Piggyback multi-turn (once built) needs e2e; real InteractiveExecutiveGate blocking path has unit coverage mainly via mocks/type-assert patterns.

## 6. Repaired gaps and remaining truth

The missing/empty capability fail-open, Ouroboros registration bypass, specialist
config validation bypass, and `safe_action/1` permission fallback are closed with
focused tests. The remaining priorities are protocol-neutral multi-turn tools,
one owned stack composition, durable state/receipts, and policy-reference registry
parity. The authoritative implementation contracts are in [TODO](TODO.md).

## 7. 2026-08-09 task-integrity incident — true-up (OPEN gaps)

> Records current reality, not aspirational completion. All items in this
> section are OPEN until the contracts and negative acceptance exams listed
> below exist and pass. This section is coupled to prompt/03-GAP-ANALYSIS.md
> G9/G10, prompt/12-FAILURE-MODES.md FM17/FM18, world/03-GAP-ANALYSIS.md, and
> session/09-SAFETY-AND-INVARIANTS.md for the same incident.

### 7.1 Session reality on 2026-08-09 — OPEN

- `run_command` and `bash` can mutate the workspace while being classified as
  non-write tools. That classification lets shell effects bypass the write-intent
  and world-refresh gates that `write_file`/`edit_*` paths go through.
- The task saw fresh dirty state but incorrectly cleaned pre-existing tracked
  and untracked work. In the incident this meant `git checkout -- <tracked>`
  reverting dirty tracked files to HEAD and `rm -rf <untracked-dir>` deleting
  an untracked directory that the task did not own.
- The one-shot world scan runs before delegation but does not refresh after
  shell effects. The pre-delegation snapshot was fresh; after an unreported
  mutation, later policy/world queries can authorize or “succeed” on a stale
  view.
- No immutable pre-task ownership baseline is snapshotted at task start. Without
  a frozen record of (tracked vs untracked) × (dirty vs clean) × (owned by
  this task vs pre-existing), later mutations cannot be attributed or
  scope-checked.

### 7.2 Required session contracts (all OPEN)

1. **Immutable pre-task ownership baseline** — at task start, snapshot the
   workspace ownership map (tracked vs untracked, dirty vs clean, task-owned
   vs pre-existing) and freeze it for the duration of the task. The baseline
   is the authority for revert/delete decisions; it must not be recomputed
   from a later mutated view.
2. **Shell effects must be detected, attributed, scope-checked, and fail
   closed before success** — every `run_command`/`bash` invocation is a
   potential workspace mutation. The executor must detect filesystem effects,
   attribute them to the invoking task/shell call, scope-check them against
   the task’s permitted mutation set, and fail closed (deny success) on any
   out-of-scope or unattributable effect before returning success.
3. **Pre-existing dirty or untracked paths must never be reverted or
   deleted** — reverting a dirty tracked file (`git checkout`/`restore`) or
   recursively deleting an untracked directory is forbidden when the path was
   dirty/untracked in the baseline. Only mutations owned by the task may be
   cleaned; pre-existing work is inviolate.
4. **Accepted mutations must trigger incremental world retraction and
   reassertion** — when a shell effect is scope-checked and accepted, the
   world must incrementally retract and reassert the affected facts before
   any later policy/world query or success verdict, so the kernel view
   reflects post-shell reality.

### 7.3 Negative acceptance exams (both OPEN)

Both must be implemented as tests that reproduce the violation without losing
the artifact and that pass only when the contracts above are implemented:

- **Exam A — git checkout of dirty tracked work:** in a temporary repo,
  commit a file, mutate it to dirty, run the task/shell path that previously
  issued `git checkout -- <file>`, and assert the dirty content is preserved
  (no revert to HEAD). The test must fail on the current code and pass only
  when baseline preservation + shell-effect scope-check are enforced.
- **Exam B — recursive deletion of an untracked directory:** in a temporary
  repo, create an untracked directory tree, run the task/shell path that
  previously issued `rm -rf <untracked-dir>` (via run_command/bash), and
  assert the directory survives intact. Must fail today and pass only when
  baseline preservation + fail-closed shell handling are enforced.

### 7.4 Coupling note

Prompt/03-GAP-ANALYSIS G9 (canonical atom precedence) and G10
(prompt/world/session coupling) and world/03-GAP-ANALYSIS and
session/09-SAFETY-AND-INVARIANTS describe the same incident from their
subsystem perspectives. Closing any one requires closing the shared contracts
and both negative exams listed here.
