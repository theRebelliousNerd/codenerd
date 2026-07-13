# 08 — Wiring and Integration: browser

> Last verified against codebase: 2026-07-13

## 1. Fact-flow placement

```
user_intent
  → perception
  → kernel derives next_action / tool choice
  → (A) VirtualStore modular tools  OR  (B) TactileRouterShard routes  OR  (C) CLI
  → SessionManager effects + optional EngineSink facts
  → Mangle world facts (if sink/engine wired)
  → policy: is_honeypot / safe_interactable
  → further next_action / articulation
```

Today, path (A) works without facts; (B) is registered but manager usually nil; (C) uses throwaway engines and workspace files.

## 2. CLI wiring (`cmd/nerd`)

### Registration

`main.go` `init()`:

- `browserCmd.AddCommand(browserLaunchCmd, browserSessionCmd, browserSnapshotCmd)`
- `rootCmd.AddCommand(..., browserCmd, ...)`

### Config helper

`getBrowserConfig()` → `DefaultConfig()` +  
`SessionStore = <cwd>/.nerd/browser/sessions.json`

### `nerd browser launch`

1. `mangle.NewEngine(DefaultConfig(), nil)`  
2. `NewSessionManager` + `Start`  
3. Write control URL → `.nerd/browser/control.txt`  
4. Block on SIGINT/SIGTERM → Shutdown, remove control file  

### `nerd browser session <url>`

1. If control.txt exists, set `DebuggerURL`  
2. Start manager, `CreateSession`  
3. Print session ID/target/URL  
4. **Does not Shutdown** (leaves browser for snapshot)

### `nerd browser snapshot <session-id>`

1. Requires control.txt  
2. Load session from store; if `detached` + TargetID, `Attach`  
3. `SnapshotDOM` + optional `ReifyReact`  
4. Dump selected predicates via `engine.GetFacts` → `.nerd/browser/snapshots/<id>_<unix>.mg`

## 3. Chat / Cortex boot

`session_boot.go` (and shared boot):

```go
// Browser Manager is created on-demand when needed (not at boot)
// This avoids spawning Chrome during normal TUI usage
var browserMgr *browser.SessionManager // nil until needed
```

When registering `tactile_router`:

```go
if browserMgr != nil {
    shard.SetBrowserManager(browserMgr)
}
```

`Cortex` / chat model carry `BrowserManager *browser.SessionManager` fields (`model_types.go`).  
**As of verification, no on-demand constructor path was found in boot that assigns non-nil** — wiring hook exists; production of the manager is the gap.

## 4. TactileRouterShard

`internal/shards/system/router.go`:

- Field: `BrowserManager *browser.SessionManager`  
- `SetBrowserManager(mgr)`  
- Tool routes (examples):

| ActionPattern | ToolName | RequiresSafe | Timeout |
|---------------|----------|--------------|---------|
| `browse` | `browser_tool` | true | 60s |
| `browser_navigate` | `browser_tool` | true | 60s |
| `browser_screenshot` | `browser_tool` | false | 30s |
| `browser_read_dom` | `browser_tool` | false | 30s |

## 5. VirtualStore

`virtual_store_types.go` defines:

- `ActionBrowserNavigate`, `Extract`, `Screenshot`, `Click`, `Type`, `Close`

`handleBrowse` **always fails** with message that browser must run via TactileRouterShard, asserting fact `browser_routing(operation, /requires_shard)`.

`handleModularTool` maps ActionBrowser* payloads to tool args (`url`, `session_id`, `selector`, `text`).

## 6. Modular research tools

`internal/tools/research/browser.go`:

| Tool name | Execute |
|-----------|---------|
| `browser_navigate` | Start + CreateSession or Navigate |
| `browser_extract` | Page content / selector text |
| `browser_screenshot` | base64 PNG |
| `browser_click` | Click selector |
| `browser_type` | Type text |
| `browser_close` | Close session path |

Uses `getBrowserManager()` → `NewSessionManager(DefaultConfig(), **nil**)` once.  
**No DOM fact stream; no honeypot.**

## 7. Mangle program load

`kernel_init.go` includes `"schemas_browser.mg"`.  
Policy modules `browser.mg` and `browser_honeypot.mg` ship under defaults/policy (loaded with policy corpus).

Constitution:

```
safe_action(/browser_navigate).
safe_action(/browser_screenshot).
safe_action(/browser_read_dom).
```

Routing: `routing_table(/browser, /browser_tool, /high)`.  
Delegation: `tool_capabilities(/browser, /navigate|/click|/type)`.

Intent routing (`internal/mangle/intent_routing.mg`): modular browser tools allowed under research/verify intent categories.

## 8. Integration journal (honest)

| Wire | Status |
|------|--------|
| Schemas in kernel | **Live** |
| Honeypot policy | **Live** (when engine has facts) |
| CLI operator path | **Live** (isolated engine) |
| Research tools effect path | **Live** (no facts) |
| Chat BrowserManager field | **Declared** |
| Boot construct manager | **Not at boot; on-demand not completed** |
| Tactile SetBrowserManager | **Conditional; usually skipped** |
| VS handleBrowse | **Explicit refuse** |
| VS modular browser tools | **Arg mapping + registry if registered** |
| HoneypotDetector in CLI/agent path | **Not wired as first-class command** |

## 9. Recommended integration sequence (docs-only guidance)

1. Construct manager with live kernel engine on first browser tool call.  
2. Set into cortex struct + tactile router.  
3. Point research `getBrowserManager` at the same instance (or deprecate singleton).  
4. Optionally implement VS handleBrowse as thin delegate.  
5. Run SnapshotDOM after navigate before honeypot/safe click decisions.
