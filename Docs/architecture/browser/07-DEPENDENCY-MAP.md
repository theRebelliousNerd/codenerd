# 07 — Dependency Map: browser

> Last verified against codebase: 2026-07-13

## 1. Package imports (upstream — what browser needs)

From `internal/browser/*.go` (non-test):

| Dependency | Use |
|------------|-----|
| `codenerd/internal/logging` | CategoryBrowser, timers, Browser* helpers |
| `codenerd/internal/mangle` | Fact, Engine (adapter + honeypot) |
| `codenerd/internal/types` | `ExtractString` for rule results (honeypot) |
| `github.com/go-rod/rod` | Browser, Page, Element, Eval |
| `github.com/go-rod/rod/lib/launcher` | Launch Chrome |
| `github.com/go-rod/rod/lib/launcher/flags` | Flag parsing for Launch slice |
| `github.com/go-rod/rod/lib/proto` | CDP messages (nav, network, DOM, emulation) |
| `github.com/google/uuid` | Session IDs |
| stdlib | context, encoding/json, errors, fmt, os, path/filepath, strings, sync, time |

**Does not import:** `internal/core`, `internal/session`, `internal/shards`, `cmd/*` — keeps browser leaf-ish relative to Cortex assembly.

## 2. Downstream importers (who imports browser)

| Importer | Path evidence | How used |
|----------|---------------|----------|
| CLI | `cmd/nerd/cmd_browser.go` | NewSessionManager, Start, CreateSession, SnapshotDOM, ReifyReact |
| Chat model | `cmd/nerd/chat/model_types.go` | Field types `*browser.SessionManager` |
| Chat boot | `cmd/nerd/chat/session_boot.go`, `session_shared_boot.go` | Declares/passes browserMgr (often nil) into shards |
| Tactile router | `internal/shards/system/router.go` | `BrowserManager *browser.SessionManager`, `SetBrowserManager` |
| Research tools | `internal/tools/research/browser.go` | Shared SessionManager for modular tools |

## 3. Soft dependencies (no Go import, semantic coupling)

| Artifact | Coupling |
|----------|----------|
| `internal/core/defaults/schemas_browser.mg` | Decl must match emitted predicates |
| `internal/core/defaults/policy/browser.mg` | Consumes interactable/geometry/computed_style |
| `internal/core/defaults/policy/browser_honeypot.mg` | Consumes css_property/position/attribute; extracted from honeypot.go comments |
| `internal/core/kernel_init.go` | Loads schemas_browser into programs |
| `internal/core/virtual_store_types.go` | ActionBrowser* constants name-aligned with tools |
| `internal/core/virtual_store_actions.go` | Documents shard ownership; modular tool arg mapping |
| `internal/core/defaults/policy/constitution.mg` | safe_action browser subset |
| `internal/mangle/intent_routing.mg` | modular_tool_allowed browser_* for research/verify intents |
| `internal/prompt/config_*.go` | Tool allowlists include browser_navigate |
| Workspace `.nerd/browser/*` | Operator persistence contract |

## 4. Layer diagram

```
cmd/nerd (browser cobra + chat fields)
        │
        ├──► internal/browser  ◄── internal/tools/research
        │         │
        │         ├──► internal/mangle
        │         ├──► internal/logging
        │         └──► go-rod → Chrome
        │
internal/shards/system (holds *SessionManager)
        │
internal/core (schemas, policy, VS action types — no import of browser package)
```

## 5. Cyclic risk

No import cycle involving `internal/browser` observed. Core/policy files mention browser in comments and Decl names only.

## 6. Version-sensitive external surface

go-rod CDP proto types (`proto.PageFrameNavigated`, network timing fields) may change across Chrome versions. Lifecycle tests Skip when Chrome unavailable; production Start returns launch/connect errors.
