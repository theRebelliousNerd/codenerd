# 07 — Dependency Map (`internal/logging`)

> Last verified: **2026-07-13**

## 1. Upstream (what logging imports)

**Only Go standard library:**

| Package | Use |
|---------|-----|
| `encoding/json` | Config parse, structured/audit lines |
| `fmt` | Formatting, stderr warnings |
| `log` | Category file writers |
| `math/rand` | Performance sampling |
| `os` | Files, MkdirAll, stderr |
| `path/filepath` | Paths under workspace |
| `strings` | LLM I/O builders, escapeString |
| `sync` | Once, Mutex, RWMutex |
| `time` | Timestamps, timers, date filenames |

**No** `internal/*` imports. This is intentional isolation.

## 2. Downstream (who imports logging)

High fan-in. Non-exhaustive but evidence-based classes:

### CLI / chat (`cmd/nerd`)

| Site | Role |
|------|------|
| `cmd/nerd/main.go` | `Initialize`, `CloseAll`, early boot |
| `cmd/nerd/chat/session_boot.go` | Workspace init |
| `cmd/nerd/chat/session_shared_boot.go` | Shared boot path |
| `cmd/nerd/chat/process*.go` | Turn processing diagnostics |
| `cmd/nerd/chat/delegation*.go` | Multistep/delegation |
| `cmd/nerd/chat/campaign.go` | Campaign UX |
| `cmd/nerd/cmd_browser.go`, `cmd_advanced.go`, `cmd_mangle_lsp.go`, `cmd_init_scan.go` | Command-level logs |

### Agent / runtime (`internal/*`)

| Package | Typical category |
|---------|------------------|
| `internal/mangle` | `kernel` |
| `internal/world` | `world` |
| `internal/init` | `boot`, `store` timers |
| `internal/embedding` | `embedding` |
| `internal/articulation` | `articulation` |
| `internal/context` | `context` |
| `internal/autopoiesis` | `autopoiesis` |
| `internal/campaign` | `campaign` |
| `internal/browser` | `browser` |
| `internal/build` | `build` |
| `internal/config` | `boot` (feature summary) |

Additional packages under tools/shards/core may import logging; grepping `"codenerd/internal/logging"` is authoritative when auditing new call sites.

## 3. Sibling (config mirror, no import edge)

```
internal/config.LoggingConfig  ──schema parallel──►  logging.loggingConfig
        │                                                      │
        │ written by loaders                                   │ read only from config.json
        ▼                                                      ▼
   operator config                                    IsCategoryEnabled / loadConfig
```

`config.LoggingConfig.IsCategoryEnabled` duplicates semantics for callers that hold config objects without going through this package.

## 4. Related observability stack (not deps)

```
internal/logging          file categories, audit, LLM I/O
internal/observability    metrics / flight recorder
cmd/nerd zap              console CLI logger
transparency / glass box  live operator UX
```

None of these form a compile-time cycle with logging as a bottom leaf.

## 5. Dependency rules for new code

| Allowed | Forbidden |
|---------|-----------|
| stdlib only inside logging | `internal/core`, `mangle`, `config` imports into logging |
| Callers import logging | Logging imports callers |
| Mirror config fields carefully | Silent schema drift without doc update |

## 6. Impact radius of breaking API changes

Renaming a `Category` constant or convenience function is a **repo-wide** change. Prefer additive categories. Removing audit helpers requires call-site audit across autopoiesis/campaign/session stacks.

## 7. Diagram: leaf package

```
                    [ many internal/* + cmd/nerd ]
                              │ import
                              ▼
                      [ internal/logging ]
                              │ import
                              ▼
                         [ stdlib only ]
```
