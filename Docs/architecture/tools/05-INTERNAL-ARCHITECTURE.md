# tools — Internal Architecture

> Last verified: **2026-07-13**

## Component diagram

```
┌─────────────────────────────────────────────────────────────┐
│                     internal/tools                          │
│  types.go ── Tool / Schema / Category / Result              │
│  registry.go ── Registry (map + byCategory + RWMutex)       │
│  errors.go ── sentinels                                     │
│  Global() ── process singleton                              │
└───────────────┬─────────────────┬───────────────┬───────────┘
                │                 │               │
     ┌──────────▼──────┐  ┌───────▼──────┐  ┌────▼──────────┐
     │ core            │  │ shell        │  │ codedom       │
     │ file_ops        │  │ execute      │  │ elements      │
     │ search          │  │ (exec/git)   │  │ lines         │
     │ workspace_guard │  │              │  │ impact+DI     │
     └────────┬────────┘  └──────┬───────┘  └────┬──────────┘
              │                  │                │
              └──────────────────┼────────────────┘
                                 │
                    ┌────────────▼────────────┐
                    │ research                │
                    │ context7 web browser    │
                    │ cache tools             │
                    │ GroundingHelper         │
                    │ ThinkingHelper          │
                    └────────────┬────────────┘
                                 │
              ┌──────────────────┼──────────────────┐
              ▼                  ▼                  ▼
      internal/browser    net/http OS      types.LLMClient
```

## Data flow — registration

```
RegisterAll(r)
  for tool in constructors():
    if r.Has(name): skip
    r.Register(tool)
      Validate Name+Execute
      tools[name] = tool
      byCategory[cat] append tool
```

Boot:

```
VirtualStore.HydrateModularTools
  RegisterAll → modularTools
  RegisterAll → tools.Global()
```

## Data flow — execution (session path)

```
executeToolCall(name, args)
  isToolAllowed?
  checkSafety? → pending_action → permitted
  PreflightDestructiveToolCall?
  timeout context
  if Global.Has(name):
    Registry.Execute
      validateArgs
      tool.Execute(ctx, args) → string
      ToolResult{DurationMs}
    ValidateInteractiveToolResult?
  else if Ouroboros:
    ExecuteRegisteredTool
```

## Data flow — VirtualStore modular execute

VS may call `modularTools.Execute` for research-related actions (see `virtual_store_actions.go` registry lookups). Same Tool handlers, different entry.

## State machines

### Browser tools (implicit)

```
[no mgr] → getBrowserManager Once → NewSessionManager
  → Start(ctx)
  → CreateSession(url) | Navigate(session_id, url)
  → extract/click/type/screenshot on session
  → browser_close (mark close)
```

No multi-tenant isolation; one manager per process.

### Research cache

```
Set → (if full: evictOldest) → entry with ExpiresAt
Get → miss if absent or expired (lazy expire on read)
Clear → empty map
```

### Test impact tools

```
[provider nil] → error
provider set → parse args → maybe Query(plan_edit)
  → analyzer.Build → GetImpactedTests
  → dry_run? list : runGoTests packages
```

## Key type relationships

```
Tool ──has──► ExecuteFunc
Tool ──has──► ToolSchema ──has──► map[string]Property
Registry ──owns──► map[string]*Tool
Registry.Execute ──returns──► *ToolResult
```

## Concurrency model

- Registry: RWMutex around maps; Execute holds RLock only for Get, not for whole Execute (validate + Execute run without holding registry lock after Get) — tool Execute may re-enter registry carefully.  
- ResearchCache: own RWMutex.  
- browserMgrMu / Once for manager.  
- Tool bodies: not required to be reentrant if they mutate shared browser/cache — concurrent browser sessions may race at mgr level (SessionManager is expected to serialize).

## Extension points

| Extension | How |
|-----------|-----|
| New built-in tool | Constructor `*tools.Tool` + append RegisterAll |
| Dynamic tool at runtime | `Register` / `VirtualStore.RegisterModularTool` |
| Impact analysis backend | `RegisterTestImpactProvider` |
| Gemini grounding | `NewGroundingHelper(client)` |
