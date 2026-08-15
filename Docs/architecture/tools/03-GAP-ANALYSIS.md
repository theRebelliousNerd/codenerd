# tools — Gap Analysis

> Last verified: **2026-08-15**

## Spec vs reality matrix

| Desired property | Reality | Severity | Priority |
|------------------|---------|----------|----------|
| All path tools contained in workspace | **Closed** — file ops, search, codedom | High | Done |
| Shell cannot cwd outside workspace | **Closed** — `shell.resolveWorkingDir` | High | Done |
| Default deny for tools | **Closed in tools** — `Registry.SetAllowlist`; session wiring pending | High | Wiring |
| Mangle lists every registered tool | **Closed** — golden test in both directions | Medium | Done |
| Single allowlist source | Config + soft FilterByIntent + Mangle | Medium | P1 |
| Categories have tools | **Closed** — `Tool.AltCategories` | Low | Done |
| codedom doc matches register | **Closed** — golden test pins doc lists | Low | Done |
| Workspace root not env-global | **Closed in tools** — `Registry.SetWorkspaceRoot` → ctx; factory wiring pending | Medium | Wiring |
| Tool results always feed multi-turn | Piggyback single pass; non-TRP clients | Medium | P1 (session) |
| Search uses coerceInt for max_results | **Closed** — one shared `tools.CoerceInt` | Medium | Done |
| Full AST codedom | Regex only | Low (by design) | — |
| Persistent research cache | **Closed** — `.nerd/cache/research`; boot wiring pending | Low | Wiring |
| Metrics on tool success rates | **Closed** — `Registry.AllMetrics()` | Low | Done |

## Non-gaps (do not “fix”)

| Observation | Why not a gap |
|-------------|---------------|
| No local `.mg` in package | Correct — policy lives in core/mangle corpora |
| Dual VS + Global hydrate | Intentional for session vs VS consumers |
| GroundingHelper not a Tool | Correct library design |
| Ouroboros separate registry | Different lifecycle (compiled binaries) |
| delete_file refuses directories | Intentional safety |

## Priority backlog (compressed)

Items 1-12 below are closed in `internal/tools`. What remains is **wiring**:
each mechanism exists and is tested, but three of them have no caller yet, and
a mechanism with no caller enforces nothing.

### Remaining — wiring, owned outside this package

- `internal/session`: call `tools.Registry.SetAllowlist(&tools.Allowlist{Enforced: cfg.EnableSafetyGate, Names: cfg.AllowedTools})` when the effective config changes, so the registry-level envelope binds to the JIT config.
- `internal/system/factory.go`: call `tools.SetGlobalWorkspaceRoot(abs)` beside the existing `os.Setenv("CODENERD_WORKSPACE_ROOT", abs)`, so containment stops depending on process-global state.
- `internal/core`: install a `tools.FactSink` that asserts `tool_execution(ToolName, Success, Timestamp)`, and call `research.EnableDiskCache(workspaceRoot)` at boot.
- `internal/core/virtual_store_tools.go`: `HydrateModularTools` still registers every family into two registries; collapsing that removes the last dual-map drift risk.

### Closed

1. `resolveWorkspacePath` applied to glob/grep/codedom path args.  
2. shell and git `working_dir` (and git pathspec) contained to the workspace.  
3. Registry fails closed on an enforced-but-empty allowlist.  
4. `intent_routing.mg` synced with the full RegisterAll catalog, pinned by a golden test.  
5. One shared `tools.CoerceInt` for every numeric tool argument.  
6. Workspace root threaded via registry -> context; env demoted to a fallback.  
7. `Tool.AltCategories` so `/review` and `/attack` resolve to real tools.  
8. `core/doc.go` and `shell/doc.go` realigned, pinned by a golden test.  
9. `logging.Tools*` used throughout the package.  
10. Disk-backed research cache under `.nerd/cache/research`.  
11. `Registry.SetFactSink` emits one record per completed execution.  
12. `Registry.Metrics` / `AllMetrics` track calls, successes, failures, durations.

## Gap ownership

Many “tool safety” gaps are **session/core contracts**, not pure tools bugs. Fixes may land in:

- `internal/tools/core` (containment)  
- `internal/session` (allowlist closed default)  
- `internal/mangle` (catalog)  
- `internal/jit/config` (AllowedTools population)
