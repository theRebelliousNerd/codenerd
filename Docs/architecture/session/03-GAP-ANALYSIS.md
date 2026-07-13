# 03 — Gap Analysis: session

> Last verified: 2026-07-13

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
| Empty AllowedTools unrestricted | Documented; safety still gates | **Medium** (defense in depth) |
| Package README accuracy | Stale slogans | **Low** (docs) |

## 2. Priority backlog (engineering)

### P0 — correctness / safety

1. Ensure production VirtualStore always implements `InteractiveExecutiveGate` when destructive tools are enabled (wiring audit, not session-only).  
2. Keep fail-closed tests green; never reintroduce nil-kernel allow under gate-on.

### P1 — tool protocol completeness

3. Multi-iteration Piggyback tool feedback (re-issue envelope with tool results).  
4. Consider treating empty AllowedTools as “no tools” when ConfigFactory failed vs intentional unrestricted bootstrap.

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
| safe_action payload fallback | Documented mitigation for Mangle normalization |
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
