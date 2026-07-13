# tactile — Dependency Map

> Last verified: **2026-07-13**

## Imports (what tactile uses)

| Dependency | Usage |
|------------|--------|
| stdlib `context`, `os/exec`, `sync`, `time`, … | Execution core |
| stdlib `syscall` / `unsafe` (platform) | Job objects, rusage, clone flags |
| `codenerd/internal/logging` | `CategoryTactile`, timers, Tactile* helpers |

**Does not import:** `internal/core`, `internal/session`, `internal/prompt`, `mangle` engine packages.

### Subpackages

```
tactile/python  → tactile, logging
tactile/swebench → tactile, tactile/python, logging
```

## Downstream (who imports tactile)

### Core runtime

| Consumer | How |
|----------|-----|
| `internal/core/virtual_store.go` | Holds `tactile.Executor`, `AuditLogger`; `initModernExecutor`; fact inject |
| `internal/core/virtual_store_codedom.go` | `TactileFileEditorAdapter` over `*tactile.FileEditor` |
| `internal/core/virtual_store_actions*.go` | Action execution via Executor |
| `internal/core/*_test.go` | Mocks implementing `tactile.Executor` |
| `internal/core/tdd_loop*.go` | Test mocks |

### Campaign

| Consumer | How |
|----------|-----|
| `orchestrator_types.go` | Config field `Executor tactile.Executor` |
| `orchestrator_task_handlers.go` | Builds `tactile.Command` |
| `checkpoint.go` / `micro_checkpoint.go` | Shell via Executor |
| `assault_tasks.go` | DirectExecutor factory helpers |

### CLI

| Consumer | How |
|----------|-----|
| `cmd/nerd/chat/session_boot.go` | `NewDirectExecutor`, `NewFileEditor` |
| `cmd/nerd/chat/model_types.go` | Executor fields on chat model |
| `cmd/nerd/cmd_campaign.go` | DirectExecutor |
| `cmd/nerd/dom_cmd.go` / `dom_replace_cmd.go` | FileEditor + DirectExecutor |

### Tests

| Consumer | How |
|----------|-----|
| `tests/e2e/*` | Composite/Direct with kernel paths |

### Other

| Note | Detail |
|------|--------|
| `internal/logging` | Defines category used by tactile (not a reverse import of tactile) |

## Dependency graph (ASCII)

```
                    logging
                       ▲
                       │
         ┌─────────────┴──────────────┐
         │         tactile            │
         │   (+ python, swebench)     │
         └──────┬───────────┬─────────┘
                │           │
        ┌───────▼───┐   ┌───▼────────┐
        │   core    │   │  campaign  │
        │ VirtualStore  │ checkpoint │
        └───────┬───┘   └───┬────────┘
                │           │
                └─────┬─────┘
                      ▼
                 cmd/nerd
                 tests/e2e
```

## Fact-flow adjacency

```
tactile ──Fact──► core.injectTactileFact ──Assert──► kernel
                       ▲
                       │ permitted check (before motor)
                    Mangle policy
```

## Coupling assessment

| Coupling | Rating | Note |
|----------|--------|------|
| tactile → logging | Light | Correct |
| core → tactile | Medium | Expected motor dependency |
| campaign → tactile | Medium | Direct command construction |
| cmd → tactile | Medium | Boot + DOM |
| tactile → core | **None** | Cycle-breaking success |

## Change impact guide

| If you change… | Re-test / re-check |
|----------------|--------------------|
| `ExecutionResult` semantics | campaign, VS actions, e2e |
| Fact predicates | core schemas + any policy rules |
| FileEditor line math | CodeDOM adapter + DOM CLI |
| Docker defaults | security review + docker tests |
| Platform GetPlatformExecutor | OS-specific CI |
