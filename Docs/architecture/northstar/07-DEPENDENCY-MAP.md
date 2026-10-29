# 07 — Dependency Map: Northstar

> Last verified against codebase: 2026-07-13  
> Package: `internal/northstar`

## 1. Upstream (what Northstar imports)

| Import | Usage |
|--------|--------|
| `codenerd/internal/types` | `Fact` for `ToFacts` / `KernelClient.Assert` |
| `codenerd/internal/logging` | `CategoryNorthstar` Info/Warn/Debug |
| `codenerd/internal/sqlpragmas` | Hot profile pragmas on open |
| `database/sql` + `github.com/mattn/go-sqlite3` | Store |
| stdlib | `context`, `sync`, `path`/`filepath`, `strings`, `time`, `encoding/json`, `os`, `fmt`, `maps`, `sort` |

**Does not import:** `internal/core`, `internal/campaign`, `internal/shards`, `internal/prompt` — keeps the package a leaf library and avoids cycles.

## 2. Downstream (who imports Northstar)

Verified via reverse grep of `codenerd/internal/northstar`:

| Consumer | How used |
|----------|----------|
| `cmd/nerd/chat/session_boot.go` | `NewStore`, `NewGuardian`, `NewBackgroundEventHandler` for observer manager |
| `cmd/nerd/chat/session_shared_boot.go` | Same + `SetParentKernel(kernel)` |
| `cmd/nerd/chat/session_boot_helpers.go` | `northstarHandlerAdapter` wraps `BackgroundEventHandler` → `shards.NorthstarHandler` |
| `cmd/nerd/chat/model_helpers.go` | Ephemeral store+guardian for `/alignment` (`runAlignmentCheck`) |
| `cmd/nerd/chat/model_helpers.go` (import) | Types used for check result messaging |
| `internal/campaign/orchestrator_*.go` | `*CampaignObserver` field; phase/task/end hooks |
| `internal/campaign/orchestrator_init.go` | `SetNorthstarObserver` |
| `internal/campaign/risk_scoring.go` | `StartCampaign` as northstar risk gate |
| `internal/campaign/risk_scoring_test.go` | Builds real `CampaignObserver` fixtures |
| `internal/init/initializer.go` | Mentions/creates path for `northstar_knowledge.db` on workspace init |

**CLI note:** `cmd/nerd/cmd_northstar.go` does **not** import `internal/northstar`; it reads `.nerd/northstar.json` / `northstar.mg` directly.

## 3. Related non-import coupling

| Surface | Coupling type |
|---------|----------------|
| Mangle Decls for `northstar_*` | Schema in core defaults / workspace programs |
| Prompt atoms `internal/prompt/atoms/northstar/` | Selector `northstar_phases`; JIT for wizard |
| `articulation.prompt_assembler` | `ExtraContext["northstar_phase"]` → compile context |
| Policy rules using `northstar_capability` etc. | Consume facts if asserted |
| `logging.CategoryNorthstar` | Declared in `internal/logging/logger.go` |

## 4. Dependency diagram

```
                    ┌──────────────┐
                    │ internal/types│
                    │ internal/log  │
                    │ sqlpragmas    │
                    └──────▲───────┘
                           │
                    ┌──────┴───────┐
                    │ northstar    │
                    └──────▲───────┘
           ┌───────────────┼────────────────┐
           │               │                │
    cmd/nerd/chat     campaign          init
    (boot, /align)    (observer,       (db path)
                      risk gate)
```

## 5. Verify reverse deps

```powershell
rg "codenerd/internal/northstar" -g "*.go" --glob "!*_test.go"
```
