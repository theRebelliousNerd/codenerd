# tools — Architecture Corpus

> Last verified against codebase: **2026-07-13**  
> Status: Living Reference Document — **code-grounded full corpus**  
> Mode: 1:1 with `internal/tools/`  
> **Implementation:** 25 non-test `.go`, 21 tests, 0 local `.mg`  
> Heuristic completeness: **~85–90%** living package

## Role

Modular tool registry and built-in tool library for the JIT Clean Loop: filesystem, shell/git, lightweight CodeDOM, and research/browser tools. Tools execute **effects**; the Mangle kernel and session safety gates remain the **executive**.

## Source location

| Path | Role |
|------|------|
| `internal/tools/` | Root types, Registry, errors, global singleton |
| `internal/tools/core/` | Filesystem + search |
| `internal/tools/shell/` | Command, bash, build, tests, git |
| `internal/tools/codedom/` | Elements, line edits, test impact |
| `internal/tools/research/` | Context7, web, browser, cache, grounding/thinking helpers |
| `internal/tools/README.md` | In-package overview (v2.0.0 JIT) |
| `internal/mangle/intent_routing.mg` | `modular_tool_allowed` rules |
| `internal/core/defaults/schemas_tools.mg` | Tool-related Decls |
| `internal/core/virtual_store_tools.go` | HydrateModularTools |
| `internal/session/executor_tools.go` | Execute path + safety |

## Full document set

| Doc | Purpose |
|-----|---------|
| [IMPLEMENTED_SPEC.md](IMPLEMENTED_SPEC.md) | **Flagship** dense living spec |
| [00-ALIGNMENT-VISION-REVIEW.md](00-ALIGNMENT-VISION-REVIEW.md) | North-star alignment scores |
| [01-VISION.md](01-VISION.md) | Target product/architecture vision |
| [02-CURRENT-STATE.md](02-CURRENT-STATE.md) | Precise inventory and hotspots |
| [03-GAP-ANALYSIS.md](03-GAP-ANALYSIS.md) | Spec vs reality matrix |
| [04-ARCHITECTURAL-PRINCIPLES.md](04-ARCHITECTURAL-PRINCIPLES.md) | Binding package principles |
| [05-INTERNAL-ARCHITECTURE.md](05-INTERNAL-ARCHITECTURE.md) | Components, data flow, state |
| [06-PUBLIC-API-AND-TYPES.md](06-PUBLIC-API-AND-TYPES.md) | Exported API surface |
| [07-DEPENDENCY-MAP.md](07-DEPENDENCY-MAP.md) | Upstream / downstream |
| [08-WIRING-AND-INTEGRATION.md](08-WIRING-AND-INTEGRATION.md) | Boot, session, Mangle, VS |
| [09-SAFETY-AND-INVARIANTS.md](09-SAFETY-AND-INVARIANTS.md) | Safety + concurrency |
| [10-TESTING-ALIGNMENT.md](10-TESTING-ALIGNMENT.md) | Tests and gaps |
| [11-OBSERVABILITY.md](11-OBSERVABILITY.md) | Logging and debug hooks |
| [12-FAILURE-MODES.md](12-FAILURE-MODES.md) | Concrete failures + mitigations |
| [TODO.md](TODO.md) | Prioritized backlog |
| [OPEN-QUESTIONS.md](OPEN-QUESTIONS.md) | Open design questions |
| [_progress.md](_progress.md) | Rebuild progress log |

## North star (context)

- LLM = creative center; Mangle kernel = executive  
- Constitutional safety: `permitted(...)`, default deny at policy layer  
- JIT prompt atoms for LLM-facing behavior (tool *selection* prose), not ad-hoc shard tool embeds  
- Wiring audit before declaring tools unused  

## Verify

```powershell
go test ./internal/tools/...
```

Optional related:

```powershell
go test ./internal/session/ -count=1 -run 'Tool|Safety'
go test ./tests/e2e/ -count=1 -run Tool
```

## Quick fact-flow

```
user_intent → ConfigFactory.AllowedTools → LLM tool_calls
  → session.Executor (allowlist + permitted + executive gate)
  → tools.Global().Execute → string result → model / articulation
```
