# core — Failure Modes

> Last verified: **2026-07-13**

## Legend

| Severity | Meaning |
|----------|---------|
| **S0** | Data loss / arbitrary code without gate |
| **S1** | Hard down / no agent progress |
| **S2** | Incorrect deny/allow; partial function |
| **S3** | Perf degradation / noise |

---

## F1 — Embedded constitution fails to compile (S1)

**Symptom:** `NewRealKernel` returns error; chat/CLI cannot boot cortex.  
**Cause:** Bad Decl, syntax, stratification, or user override concatenated into program.  
**Detection:** Kernel error log; `debug_program_ERROR.mg`.  
**Mitigation:** Fix mg sources; remove bad `.nerd/mangle` overrides; never ship binary with broken embed.

## F2 — Policy Decl collision / duplicate `permitted` (S1/S2)

**Symptom:** Analyze failure or inconsistent schema.  
**Cause:** Multiple Decl lines for same predicate with incompatible bounds.  
**Mitigation:** Keep Decl ownership in schemas modules; policy only rules; rebuild debug dump.

## F3 — Boot guard left on (S2)

**Symptom:** All tools fail with “boot guard active”.  
**Cause:** Chat forgot `DisableBootGuard` after first user interaction.
**Mitigation:** Preserve chat rehydration quiescence, then clear on the first user
message. Do not copy this timing blindly to explicit command `BootCortex` mode.

## F4 — Stale next_action replay (S0/S1 risk if unguarded)

**Symptom:** On rehydrate, agent starts mutating files before user speaks.  
**Cause:** Ephemeral facts not filtered + boot guard disabled too early.  
**Mitigation:** Keep both layers; never disable guard during pure rehydrate.

## F5 — Dreamer false negative (S0)

**Symptom:** Destructive action proceeds that should panic.  
**Cause:** Missing projected_fact modeling; action not in `isDestructiveAction`; weak policy.  
**Mitigation:** Expand projection; keep fail-closed defaults; add golden panic_state cases.

## F6 — Dreamer false positive (S2)

**Symptom:** Safe actions blocked.  
**Cause:** Over-broad panic rules; stale cache after state change.  
**Mitigation:** Tighten rules; `InvalidateCache` on assert/policy change; surface reason to user.

## F7 — Kernel nil on VS (S0/S2)

**Symptom:** Actions deny with missing-kernel evidence; destructive routes cannot build Dreamer.
**Cause:** Incomplete boot DI.  
**Mitigation:** Production boot always `SetKernel` before routing. Preserve the
current fail-closed denial; never repair availability by restoring a nil shortcut.

## F8 — Eval thrash / latency (S3→S1)

**Symptom:** Multi-second hangs per turn; timeouts.  
**Cause:** Full re-eval on every assert; huge program; fact explosion.  
**Mitigation:** Batch asserts; retract carefully; fact limits; consider stable policy; ops guidance on diff-eval.

## F9 — Diff-eval incorrectness (S2)

**Symptom:** Query returns stale IDB after complex updates.  
**Cause:** Incremental path gaps; retract without full rebuild (code tries to invalidate — flag bugs possible).  
**Mitigation:** Prefer full eval when unsure (`CODENERD_DIFF_EVAL=0`); add regression tests.

## F10 — maxFacts exceeded (S2)

**Symptom:** Assert failures / refused facts.  
**Cause:** Unbounded logging facts / world model dump into EDB.  
**Mitigation:** Prune action logs; page context; raise limit only with memory budget.

## F11 — HotLoad suicide rule (S1 historical)

**Symptom:** Kernel permanently fails evaluate after bad learned rule.  
**Cause:** Invalid rule appended without sandbox (Bug #8 class).  
**Mitigation:** `HotLoadRule` sandbox compiler + unsafe negation check + heal path.

## F12 — Permission classification cache staleness (S3)

**Symptom:** Classification hint is stale; exact Mangle query still decides.
**Cause:** Cache not rebuilt after policy hot change.  
**Mitigation:** Rebuild for diagnostics/performance as needed. Never turn the cache
back into an allow path.

## F13 — Handler missing for ActionType (S2)

**Symptom:** Unknown action error or no-op.  
**Cause:** Policy emits verb without VS case.  
**Mitigation:** Table tests; keep policy atom names synced with ActionType.

## F14 — MCP / tool executor unset (S2)

**Symptom:** Research/MCP actions fail.  
**Cause:** Boot did not SetMCPClient / tool registries.  
**Mitigation:** Feature-detect; degrade gracefully with clear execution_error facts.

## F15 — Cortex predicate ownership conflict (S2)

**Symptom:** Facts land on wrong domain; last registration wins (warn log).  
**Cause:** Overlapping owned predicate sets.  
**Mitigation:** Partition Decl ownership carefully; test RegisterShard conflicts.

## F16 — Clone memory pressure (S3)

**Symptom:** GC thrash during burst simulations.  
**Cause:** Dreamer clones large kernel repeatedly; cache misses.  
**Mitigation:** Cache hits; reduce projected eval frequency; limit concurrent destructive proposals.

## F17 — Transaction left open (S2)

**Symptom:** Subsequent edits blocked or inconsistent FS.  
**Cause:** Crash mid multi-file edit.  
**Mitigation:** VS Close aborts active tx; ensure Abort on context cancel.

## F18 — Path / workspace mis-root (S2)

**Symptom:** Wrong files edited; `.nerd` not found.  
**Cause:** CWD ≠ workspace; forgot `NewRealKernelWithWorkspace` / `SetWorkspace`.  
**Mitigation:** Always pass workspace from CLI flags.

## F19 — Partial Dreamer projection silently evaluated (S0)

**Symptom:** Simulation calls an action safe after a projected fact was rejected.
**Cause:** unchecked clone assertion at fact capacity or atom-conversion failure.
**Mitigation:** retain `assertWithoutEvalChecked`; any staging rejection is unsafe.

## F20 — Result fact schema or correlation drift (S2)

**Symptom:** downstream policy cannot join a failure/result, pruning reads output
as time, or router results use a different ID from the executive action.
**Cause:** hand-built fact slots or a second route ID.
**Mitigation:** preserve `security_violation/3`, `execution_error/2`,
`execution_result/6`, slot-5 timestamp, and original action ID tests.

## F21 — Validation uses symbolic CodeDOM target (S2)

**Symptom:** an edit succeeds but semantic validation checks an element identifier
as if it were a file path.
**Cause:** result omitted concrete file metadata.
**Mitigation:** edit handlers return `Metadata["file"]`; validator prefers it.

---

## Cross-cutting recovery

| Failure class | First action |
|---------------|--------------|
| Boot | debug dump + remove user overrides |
| Deny | logs + query security_violation |
| Hang | timers + fact counts + policyDirty |
| Wrong effect | trace RouteAction layers; verify ActionType |

## Related tests

Prefer e2e boundaries: `dreamer_kernelclone`, `shadowmode_commit_safety`, `kernelquery_virtualstore`, `session_executor_kernel`.
