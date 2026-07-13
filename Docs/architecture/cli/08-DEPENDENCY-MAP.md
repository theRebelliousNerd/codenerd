# 08 — Dependency Map: CLI → Internal Packages

> Last verified: 2026-07-13  
> Direction: `cmd/nerd` **depends on** packages below; reverse edges are “called by CLI”.

## 1. Critical boot graph

```
cmd/nerd/main.go
  ├─ internal/config
  ├─ internal/features
  ├─ internal/logging
  ├─ internal/observability
  └─ cmd/nerd/chat
        ├─ internal/core (+ coreshards)
        ├─ internal/system
        ├─ internal/session
        ├─ internal/perception
        ├─ internal/articulation (via process helpers)
        ├─ internal/prompt
        ├─ internal/shards (+ system)
        ├─ internal/store
        ├─ internal/embedding
        ├─ internal/world
        ├─ internal/campaign (via cmd + chat)
        ├─ internal/autopoiesis
        ├─ internal/browser
        ├─ internal/northstar
        ├─ internal/transparency
        ├─ internal/verification
        ├─ internal/tactile
        ├─ internal/retrieval
        ├─ internal/context
        ├─ internal/ux
        ├─ internal/types
        └─ internal/sqlpragmas
```

## 2. Command-centric dependencies

| CLI area | Primary internal packages |
|----------|---------------------------|
| `run` / instruction | `system`, `core`, `perception`, `session` |
| `query` / `why` / `logic` | `core`, `mangle` (via kernel) |
| `campaign` | `campaign`, `prompt` (JIT provider) |
| `auth` | engine clients under `perception` / config |
| `browser` | `browser` |
| `check-mangle` / `mangle-lsp` | `mangle` tooling |
| `embedding` | `embedding` |
| `dom` | codedom / world / diff pathways |
| `tool` / autopoiesis | `autopoiesis` |
| `glassbox` / transparency | `transparency`, logging |

## 3. External modules (Go)

Notable third-party (from imports in main/chat):

- `github.com/spf13/cobra`
- `github.com/charmbracelet/bubbletea` (+ glamour/lipgloss ecosystem)
- `go.uber.org/zap`
- `github.com/mattn/go-sqlite3` (CGO)

## 4. Refresh command

```powershell
# packages importing cmd/nerd (should be rare)
rg "codenerd/cmd/nerd" -g "*.go"
# what chat boots
rg "codenerd/internal/" cmd/nerd/chat/session_boot.go
```
