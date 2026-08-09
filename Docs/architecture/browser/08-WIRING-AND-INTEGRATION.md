# 08 — Wiring and Integration: browser

> Last verified against codebase: 2026-08-09

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

System paths (A) and (B) share the Cortex-owned manager, whose `browserKernelSink` writes facts into the live executive kernel. Legacy chat remains conditional. Path (C) is a separate operator export workflow with a schema-loaded engine and private workspace files.

## 2. CLI wiring (`cmd/nerd`)

### Registration

`main.go` `init()`:

- `browserCmd.AddCommand(browserLaunchCmd, browserSessionCmd, browserSnapshotCmd)`
- `rootCmd.AddCommand(..., browserCmd, ...)`

### Config helper

`getBrowserConfig()` loads the top-level native `.nerd/config.json` `browser`
block, then pins `SessionStore` and workspace output policy under `<cwd>/.nerd/browser/`.

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

The current system factory (`internal/system/factory.go`) creates one workspace manager backed by `browserKernelSink`, injects it into tactile routing, and binds modular research tools with `research.SetBrowserManager`. The legacy chat boot path still contains:

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
The remaining gap is surface reachability: the existing six research tools share live facts, but multi-browser/isolation/focus lifecycle operations are not yet exposed through the progressive tool contract.

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

`getBrowserManager()` resolves the Cortex-owned manager installed by
`research.SetBrowserManager`; a nil-sink manager is constructed lazily only for
narrow standalone package use. `browser_close` calls `CloseSession` and cancels
the tab's event stream.

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
| CLI operator path | **Live** (standalone export engine) |
| Research tools effect path | **Live** (shared Cortex manager/facts after system boot) |
| System Cortex BrowserManager field | **Constructed with live-kernel adapter** |
| Legacy chat BrowserManager field | **Declared; nil** |
| Tactile SetBrowserManager | **Wired in system factory; conditional in legacy chat** |
| VS handleBrowse | **Explicit refuse** |
| VS modular browser tools | **Arg mapping + registry if registered** |
| HoneypotDetector in CLI/agent path | **Not wired as first-class command** |

## 9. Recommended integration sequence (docs-only guidance)

1. Expose the BPAR-1 lifecycle through progressive native tools and operator CLI commands.
2. Add a live modular-tool → browser event → Cortex query proof.
3. Optionally implement VS handleBrowse as a thin delegate.
4. Run SnapshotDOM after navigate before honeypot/safe click decisions.
