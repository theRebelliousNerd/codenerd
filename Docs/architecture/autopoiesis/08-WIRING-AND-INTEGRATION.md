# 08 — Wiring and Integration: Autopoiesis

> Last verified against codebase: **2026-07-13**  
> Honest wiring journal — only paths found in source

## 1. Boot wiring (Cortex)

**File:** `internal/system/factory.go` — `initAutopoiesisAndBrowser`

```text
DefaultConfig(workspace)
  → NewOrchestrator(llmClient, config)
  → NewAutopoiesisBridge(kernel)
  → poiesis.SetKernel(bridge)
  → if GetOuroborosLoop() != nil:
        virtualStore.SetToolGenerator(ouroborosLoop)
        virtualStore.SetToolExecutor(ouroborosLoop)
  → Cortex.Orchestrator = poiesis
```

Boot context field: `poiesis *autopoiesis.Orchestrator` (also exposed on Cortex as `Orchestrator`).

**Not wired at this step (optional later):** `SetPromptAssembler` — chat/session boot attaches JIT assembler when available (`session_boot.go` / shared boot patterns).

## 2. Chat model wiring

| Concern | Location |
|---------|----------|
| Model field `autopoiesis *Orchestrator` | `cmd/nerd/chat/model_types.go` |
| Prompt evolution field | same + `prompt_evolution` import |
| Listener cancel | `model_lifecycle.go` (`autopoiesisCancel`, `autopoiesisListenerCh`) |
| QuickAnalyze / generate_tool | `process.go` |
| List/execute/generate helpers | `helpers_tools.go` |
| Slash / tool commands | `commands_tools.go`, `commands_handlers.go` |
| Evolution commands | `commands_evolution.go` |
| Delegation records for SPL | `delegation.go` |
| Alt+A dashboard | `model_key_handler.go` + `ui/autopoiesis_page.go` |

## 3. CLI wiring

| Command path | Behavior |
|--------------|----------|
| Instruction / run style | `cmd_instruction.go` calls `cortex.Orchestrator.ProcessKernelDelegations` |
| Systems | `cmd_systems.go` prints Autopoiesis status section / subcommands |
| Campaign cobra | Comments note Ouroboros not fully wired in some CLI campaign modes — campaign package has own Orchestrator |

## 4. VirtualStore integration

Ouroboros implements generator/executor interfaces expected by VirtualStore (via `ToolSynthesizer` / loop methods: generate, execute tool by name). Exact interface satisfaction is in core VirtualStore setters:

- `SetToolGenerator`
- `SetToolExecutor`

This allows **kernel-derived actions** that need a missing tool to invoke generation without chat-only coupling.

## 5. Fact-flow participation

```
user_intent (perception)
  → kernel derives next_action / delegate_task / generate_tool signals
  → Orchestrator processes (chat branch or ProcessKernelDelegations)
  → asserts tool_registered / missing_tool_for / learnings
  → later next_action may route tool use through VirtualStore → RuntimeTool.Execute
  → articulation reports results
```

## 6. Campaign integration

`internal/campaign/tool_pregenerator.go` and `intelligence_gatherer.go` import autopoiesis to prepare tools or reason about needs **without** owning Ouroboros stages themselves. Risk scoring tests may construct package types.

## 7. Verification integration

`internal/verification/verifier.go` imports autopoiesis — treat as a peer consumer for validation workflows (do not assume full Orchestrator lifecycle).

## 8. Partial / dormant wires (audit notes)

| Item | Status |
|------|--------|
| `StartKernelListener` | Implemented; chat lifecycle supports cancel — confirm always started in all boot paths |
| `SetPromptAssembler` | Implemented; depends on chat attaching after Cortex boot |
| Light `GenerateTool` in process.go | Live but shallower than Ouroboros |
| CLI campaign + Ouroboros | Comment in `cmd_campaign.go` indicates incomplete CLI-mode Ouroboros wiring |
| YaegiExecutor | Implemented; not primary commit path of Ouroboros |
| `ActionDelegateToShard` | Enum exists; primary delegation is kernel `delegate_task` |

Before deleting “unused” methods: grep chat, factory, campaign, e2e, and VirtualStore.

## 9. UI surface

`cmd/nerd/ui/autopoiesis_page.go`:

- Tabs for patterns and learnings  
- `UpdateContent(patterns, learnings)` from Orchestrator getters  
- Keyboard nav tests in `keyboard_navigation_test.go` / `pages_test.go`

## 10. How to add a new integration safely

1. Prefer asserting a Mangle fact and handling in `ProcessKernelDelegations` or policy.  
2. Or call Orchestrator methods from chat/CLI after nil-check.  
3. Always `SetKernel` before expecting fact side effects.  
4. Prefer `ExecuteOuroborosLoop` for new tool creation entry points.  
5. Run package tests + targeted e2e Autopoiesis suite.
