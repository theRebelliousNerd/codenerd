# tools — Gap Analysis

> Last verified: **2026-07-13**

## Spec vs reality matrix

| Desired property | Reality | Severity | Priority |
|------------------|---------|----------|----------|
| All path tools contained in workspace | File ops yes; search/codedom no | High | P0 |
| Shell cannot cwd outside workspace | working_dir unrestricted | High | P0 |
| Default deny for tools | Empty AllowedTools → allow all | High | P0 |
| Mangle lists every registered tool | git_*, some cache ops missing | Medium | P1 |
| Single allowlist source | Config + soft FilterByIntent + Mangle | Medium | P1 |
| Categories have tools | /review /attack empty | Low | P2 |
| codedom doc matches register | doc lists open_file etc. | Low | P2 |
| Workspace root not env-global | CODENERD_WORKSPACE_ROOT + cwd | Medium | P1 |
| Tool results always feed multi-turn | Piggyback single pass; non-TRP clients | Medium | P1 (session) |
| Search uses coerceInt for max_results | Uses bare `.(int)` — LLM float64 may miss defaults | Medium | P1 |
| Full AST codedom | Regex only | Low (by design) | — |
| Persistent research cache | In-memory TTL | Low | P3 |
| Metrics on tool success rates | DurationMs only | Low | P3 |

## Non-gaps (do not “fix”)

| Observation | Why not a gap |
|-------------|---------------|
| No local `.mg` in package | Correct — policy lives in core/mangle corpora |
| Dual VS + Global hydrate | Intentional for session vs VS consumers |
| GroundingHelper not a Tool | Correct library design |
| Ouroboros separate registry | Different lifecycle (compiled binaries) |
| delete_file refuses directories | Intentional safety |

## Priority backlog (compressed)

### P0

1. Apply `resolveWorkspacePath` to glob/grep/codedom path args.  
2. Contain shell `working_dir` (and git working_dir) to workspace.  
3. Fail closed when AllowedTools empty **and** safety gate on (session + config contract).

### P1

4. Sync `intent_routing.mg` with full RegisterAll catalog (git_*, research_cache_clear/stats).  
5. Use coerceInt consistently in search tool arg parsing.  
6. Thread workspace root via context/registry, retire env-only design.  
7. Document or enforce ConfigFactory always non-empty AllowedTools from Mangle.

### P2

8. Align codedom `doc.go` with registered tools.  
9. Decide fate of CategoryReview/CategoryAttack (tools or remove from Filter map).  
10. Optional: register research_cache tools for /general read-only.

### P3

11. Optional disk-backed research cache.  
12. Emit `tool_execution` facts from registry.Execute for learning.

## Gap ownership

Many “tool safety” gaps are **session/core contracts**, not pure tools bugs. Fixes may land in:

- `internal/tools/core` (containment)  
- `internal/session` (allowlist closed default)  
- `internal/mangle` (catalog)  
- `internal/jit/config` (AllowedTools population)
