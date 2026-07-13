# 03 — Gap Analysis: Autopoiesis

> Last verified against codebase: **2026-07-13**  
> Compared: package vision / north star vs `internal/autopoiesis` reality

## 1. Spec vs reality matrix

| Expected capability | Reality | Gap severity |
|---------------------|---------|--------------|
| Full Ouroboros for all tool commits | Chat `generate_tool` may call `GenerateTool` only; `ExecuteAction` also light path | **High** |
| Kernel-only triggers for generation | `delegate_task` works; also direct chat/CLI helpers | **Medium** |
| Parent kernel sees all Ouroboros internal facts | Register/hot-load/learning synced; full internal state machine facts stay local | **Low–Medium** (by design) |
| Campaign start from autopoiesis | Analysis emits `ActionStartCampaign`; `ExecuteAction` no-ops | **None** (correct ownership) if callers handle |
| Persistent agents always run | Specs + memory files written; no full scheduler here | **Medium** (scope) |
| Unified execute sandbox | Binary exec primary; Yaegi alternate; not one policy switch | **Medium** |
| Thunderdome always decisive | If PanicMaker fails or no attacks, loop continues | **Low–Medium** |
| SPL → JIT always | Wired in chat/boot; depends on config AutoPromote | **Medium** (ops) |
| Learning fact gas | `MaxLearningFacts` + retract patterns present | **Low** |
| Restore tools to kernel on boot | `SetKernel` → `syncExistingToolsToKernel` batch | **Closed** |
| Hot-reload parent facts | `assertToolHotReloaded` (GAP-019 fix in comments) | **Mostly closed** |
| Safety policy embedded | `go_safety.mg` via core defaults | **Closed** if content present; warn if load fails |

## 2. Priority backlog (engineering)

### P0 — correctness / safety

1. **Unify generation highways** — route chat `generate_tool` and `ExecuteAction` through `ExecuteOuroborosLoop` (or explicit named “unsafe diagnostic” path).  
2. **Fail closed when `go_safety.mg` missing** — today checker may run with empty policy string after warn.  
3. **Document/guard `AllowExec: true`** default — operators may assume no subprocess.

### P1 — integration

4. Prefer kernel query `ShouldGenerateTool` / refine predicates over ad-hoc chat heuristics where policy exists.  
5. Ensure VirtualStore tool executor and Orchestrator registry stay single-source after hot reload.  
6. Campaign pregen: confirm Ouroboros vs light generate consistent with risk scoring.

### P2 — product polish

7. Agent scheduler bridge (or explicit handoff doc to shards/user agents).  
8. Yaegi path productization or demotion to test-only.  
9. Richer e2e with fake LLM covering full stage transitions + Thunderdome kill/retry.  
10. SPL promotion requires optional human-in-the-loop flag defaulting stricter.

## 3. Non-gaps (do not “fix”)

| Observation | Why not a gap |
|-------------|----------------|
| No `.mg` files inside package | Policy is embedded from core defaults by design |
| Campaign not executed here | Ownership is `internal/campaign` |
| Dual Mangle engines (Ouroboros local + parent) | Isolates state machine; parent only needs durable outcomes |
| Heuristic QuickAnalyze without LLM | Required for chat latency |

## 4. Risk register

| Risk | Likelihood | Impact | Mitigation in code |
|------|------------|--------|--------------------|
| Unsafe tool slips past empty policy | Low if embed ok | Critical | Embed load + tests on checker |
| Tool spam | Medium | Medium | MaxToolsPerSession, confidence |
| Infinite Ouroboros | Low | High | MaxIters, shouldHalt, retries |
| Learning store races | Low | Medium | Mutexes; race tests in feedback_test |
| Prompt atom pollution | Medium | Medium | Confidence thresholds, AutoPromote flag |

## 5. Measurement recommendations

- Log ratio: light GenerateTool vs Ouroboros Execute per session.  
- Count safety rejections vs Thunderdome kills.  
- After boot: query `tool_registered` count vs registry.List length (parity check).
