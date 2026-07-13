# tools — Observability

> Last verified: **2026-07-13**

## Logging channels used

| Channel | Package area | Examples |
|---------|--------------|----------|
| `logging.ToolsDebug` | registry | Registered tool; Executing tool; completed duration/success |
| `logging.VirtualStore` / `VirtualStoreDebug` | core file ops, shell | path, size, command, completion |
| `logging.Researcher` / `ResearcherDebug` / `ResearcherWarn` | research web/context7/cache/grounding | query, hits, grounding URL truncate |
| `logging.Browser` / `BrowserDebug` | research/browser.go | navigate, extract, screenshot, click |
| Session category (external) | executor_tools | modular tool execute success/fail, safety denies |

There is **no** dedicated high-level `logging.Tools` non-debug success stream for all tools; many use VirtualStore channel historically.

## Structured fields (informal)

Logs are printf-style, not structured JSON. Recurring fields:

- tool name  
- path / url / query  
- duration (registry + session)  
- success bool  
- byte sizes  

## Timing

- `ToolResult.DurationMs` set in `ExecuteTool`.  
- `logging.StartTimer(CategoryVirtualStore, "HydrateModularTools")` on hydrate.  
- Shell/command wall time bounded by timeout context.

## Metrics

No Prometheus/OTel counters in `internal/tools`. Learning hooks intended via Mangle `tool_execution` facts are declared in schemas but **not** automatically asserted from `Registry.Execute`.

## Debug workflows

1. **Tool not found:** enable session + tools debug; check hydrate log counts; `tools.Global().Names()`.  
2. **Path denied:** look for `path escapes workspace root`.  
3. **Command timeout:** shell returns explicit timeout error; session may wrap.  
4. **Research empty:** Researcher logs “no results”; context7 public access path.  
5. **Browser fail:** Browser logs start/navigate errors; ensure Chrome/Rod available.  
6. **Safety deny:** Session safety logs pending_action / permitted mismatch (outside tools).  

## Recommended future hooks (not implemented)

- Counter: tool_execute_total{name,success}  
- Histogram: tool_duration_ms  
- Assert `tool_execution` fact after Execute for Mangle learning  
- Correlate tool call ID (session has call.ID) into tools logs
