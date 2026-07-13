# 08 — Wiring and Integration: Northstar

> Last verified against codebase: 2026-07-13  
> Focus: how the library is actually registered and called

## 1. Fact-flow placement

Primary OODA path does **not** go through Northstar:

```
user_intent → kernel next_action → VirtualStore → articulation
```

Northstar is a **parallel guardian**:

```
vision/events → Guardian.CheckAlignment → SQLite + optional kernel facts
                      ↓
            campaign risk gate / UI assessment
```

Kernel facts, when injected, can influence later orientation (injectable context, policy rules that mention `northstar_*`).

## 2. Interactive boot (primary chat path)

File: `cmd/nerd/chat/session_boot.go` (~1054+)

1. `shards.NewBackgroundObserverManager(...)`
2. `RegisterObserver("northstar")`
3. `northstar.NewStore(filepath.Join(workspace, ".nerd"))`
4. `NewGuardian(store, DefaultGuardianConfig())`
5. `SetLLMClient(shardLLMClient)`
6. `Initialize()`
7. `NewBackgroundEventHandler(guardian, sessionID)`
8. `observerMgr.SetNorthstarHandler(&northstarHandlerAdapter{handler})`

**Gap:** this path does **not** call `guardian.SetParentKernel(...)` (contrast shared boot).

## 3. Shared boot path

File: `cmd/nerd/chat/session_shared_boot.go` (~331+)

Same observer wiring **plus**:

```text
if kernel != nil {
    guardian.SetParentKernel(kernel)
}
```

Prefer this pattern for any new boot assembly.

## 4. Adapter layer

File: `cmd/nerd/chat/session_boot_helpers.go`

`northstarHandlerAdapter` converts `shards.ObserverEvent` → `BackgroundEventHandler.HandleEvent` and maps `ObserverAssessment` back to `shards.ObserverAssessment`.

## 5. Manual alignment (`/alignment`)

File: `cmd/nerd/chat/model_helpers.go` — `runAlignmentCheck`

- Opens store on `.nerd`, builds guardian, optional `SetLLMClient(m.client)`, `Initialize`
- If `!HasVision` → skipped message
- Else `CheckAlignment(..., TriggerManual, subject, buildAlignmentContext())` with 60s timeout
- Formats result for TUI (`formatAlignmentCheckResult`)

Does **not** set parent kernel (ephemeral process; check is UX-only).

## 6. Campaign orchestration

| Hook | File | Call |
|------|------|------|
| Configure | `orchestrator_init.go` | `SetNorthstarObserver(*CampaignObserver)` |
| Risk preflight | `risk_scoring.go` | `StartCampaign` as `/northstar` gate; protected campaigns require configured observer |
| Phase start | `orchestrator_phases.go` | `OnPhaseStart` |
| Phase complete | `orchestrator_phases.go` | `OnPhaseComplete` |
| Task complete | `orchestrator_tasks.go` | `OnTaskComplete` with file paths |
| Campaign end | `orchestrator_execution.go` | `EndCampaign` |

Risk toggles: `NorthstarGateToggle` `/auto|/enabled|/disabled` on orchestrator config. Auto enables when observer configured.

## 7. CLI surface (partial integration)

File: `cmd/nerd/cmd_northstar.go`

Commands: `show`, `summary`, `query`, `facts`, `export`, `stats`.

All read **filesystem artifacts** (`.nerd/northstar.json`, `.nerd/northstar.mg`), not `internal/northstar.Store`.

Wizard (chat) produces those artifacts; Guardian DB is separate unless something bridges them.

## 8. Workspace init

`internal/init/initializer.go` references `northstar_knowledge.db` path under `.nerd/` so the store location is part of workspace layout.

## 9. Prompt / JIT surface (adjacent)

| Path | Role |
|------|------|
| `internal/prompt/atoms/northstar/*.yaml` | Wizard + validation atoms; `northstar_phases` selectors |
| `internal/articulation/prompt_assembler.go` | Maps `northstar_phase` session extra context into compile context |
| Guardian alignment prompts | Still **inline** in `guardian.go` |

## 10. Wiring checklist for integrators

- [ ] Store path = `{workspace}/.nerd`
- [ ] `Initialize` after SetLLM / SetParentKernel
- [ ] Kernel set if vision facts must appear in Mangle
- [ ] Campaign: construct `CampaignObserver` and `SetNorthstarObserver` before start
- [ ] Background: adapter + `SetNorthstarHandler`
- [ ] Decide which vision file is authoritative (JSON vs SQLite) for your surface
