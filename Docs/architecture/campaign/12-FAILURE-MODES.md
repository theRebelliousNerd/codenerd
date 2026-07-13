# 12 — Failure Modes: campaign

> Last verified: **2026-07-13**  
> Concrete failures observed or designed for in source, with mitigations.

## F1 — No campaign loaded

**Symptom:** `Run` returns `"no campaign loaded"`.  
**Cause:** Forgot `SetCampaign`/`LoadCampaign`.  
**Mitigation:** CLI start path must set campaign before Run.

## F2 — Already running

**Symptom:** `"campaign already running"`.  
**Cause:** Double Run without Stop.  
**Mitigation:** Single control plane; Stop cancels.

## F3 — Missing DI dependencies

**Symptom:** `NewOrchestrator` errors with `ErrInvalidConfig` / `ErrNilDependency`.  
**Cause:** nil kernel/llm/executor/virtual_store or neither TE nor SM.  
**Mitigation:** validateOrchestratorConfig lists missing fields.

## F4 — Empty goal / empty source paths

**Symptom:** `ErrEmptyGoal` or invalid SourcePaths.  
**Cause:** Bad DecomposeRequest.  
**Mitigation:** Decomposer rejects early.

## F5 — Decomposition LLM failure

**Symptom:** plan proposal error wrapped.  
**Cause:** provider outage / parse failure.  
**Mitigation:** retries in planning helpers; fail closed to caller (no half-active campaign unless partially saved by caller).

## F6 — Validation issues after load

**Symptom:** `PlanValidationIssue` list non-empty.  
**Cause:** cycles, unreachable tasks, ambiguous structure.  
**Mitigation:** `refinePlan` path; operator review for low confidence (`plan_needs_review` rules).

## F7 — Risk gate blocks start

**Symptom:** `runRiskPreflight` error; Run never schedules.  
**Cause:** high score + enabled gates / force_block.  
**Mitigation:** lower scope, fix safety warnings, or explicit mode override (ops-controlled).

## F8 — Kernel eligibility empty forever

**Symptom:** loop cannot schedule; may hit `campaign_blocked` or spin phase start failures.  
**Cause:** missing campaign rules in program, facts not loaded, all tasks blocked.  
**Mitigation:** ensure policy load; inspect `campaign_blocked` reason; fix deps.

## F9 — Task timeout

**Symptom:** context deadline in task goroutine; failure handler.  
**Cause:** default 30m task timeout or hung TE.  
**Mitigation:** `DisableTimeouts` for extreme long-horizon; or raise TaskTimeout.

## F10 — Campaign timeout

**Symptom:** parent context deadline (default 4h).  
**Cause:** campaign too large for budgeted wall clock.  
**Mitigation:** pause/resume across sessions; DisableTimeouts; increase CampaignTimeout.

## F11 — Max retries exceeded

**Symptom:** task `/failed`; possible replan if AutoReplan thresholds met.  
**Cause:** persistent logic/tool errors.  
**Mitigation:** diagnostic repro insertion for logic failures; replan; human intervention.

## F12 — Checkpoint failed

**Symptom:** phase not completed; `replan_trigger` `/checkpoint_failed`.  
**Cause:** tests/build/nemesis failed.  
**Mitigation:** Replanner adapts tasks; do not force completePhase.

## F13 — Write-set lock timeout

**Symptom:** task not scheduled this tick; `ErrWriteSetLockTimeout`.  
**Cause:** overlapping mutators.  
**Mitigation:** wait; reduce parallelism; split write sets.

## F14 — Coder shard success without file write

**Symptom:** fallback LLM path after Stat missing.  
**Cause:** shard returned without tool call.  
**Mitigation:** explicit verify + fallback; still path-guarded.

## F15 — Pause deadlock misconceptions

**Symptom:** no new tasks while paused.  
**Cause:** designed pause.  
**Mitigation:** Resume closes pauseCh.

## F16 — Corrupt journal tail

**Symptom:** sequence recovery skips bad lines.  
**Cause:** crash mid-append.  
**Mitigation:** recoverJournalSequence ignores corrupt tail; checksums on events.

## F17 — Snapshot write failure

**Symptom:** save error; campaign may be unsafe to resume.  
**Cause:** disk full, permission, checksum mismatch.  
**Mitigation:** temp+verify+rename; journal request without commit signals incomplete write.

## F18 — Assault no targets

**Symptom:** discover error `no targets discovered`.  
**Cause:** scope/include/exclude filters too tight or non-Go tree.  
**Mitigation:** adjust AssaultConfig filters/scope.

## F19 — Assault missing batch artifact

**Symptom:** batch task fails hard.  
**Cause:** task created without artifact or file deleted.  
**Mitigation:** fail closed; re-run discover if needed (idempotent if batches exist).

## F20 — Northstar blocks phase

**Symptom:** `northstar alignment failed` on startNextPhase.  
**Cause:** phase drifts from vision.  
**Mitigation:** adjust plan or vision; risk toggle may disable gate.

## F21 — Intelligence gather failure

**Symptom:** warn log; planning continues.  
**Cause:** optional systems unavailable.  
**Mitigation:** non-fatal by design; risk snapshot may be thinner.

## F22 — Rolling-wave replan fails

**Symptom:** warn + `replan_failed` event; campaign continues with old next phase.  
**Cause:** LLM/kernel issues post-phase.  
**Mitigation:** inspect event; manual replan later.

## F23 — Cancellation mid-run

**Symptom:** status `/paused`, ctx error returned.  
**Cause:** SIGINT / Stop / timeout.  
**Mitigation:** save on cancel path; resetInProgress on next Run.

## Recovery checklist

1. Read campaign JSON status + LastError-ish fields on tasks.  
2. Tail journal for last committed snapshot.  
3. Query kernel for `campaign_blocked`, failed tasks.  
4. For assault, inspect `assault/results` and triage.  
5. Resume with LoadCampaign + Run after fixing root cause.

## Related

- [09-SAFETY-AND-INVARIANTS.md](09-SAFETY-AND-INVARIANTS.md)  
- [11-OBSERVABILITY.md](11-OBSERVABILITY.md)  
- [TODO.md](TODO.md)
