# mcp — Corpus Rebuild Progress

## 2026-07-13 — Full rebuild (SUBAGENT_INSTRUCTIONS)

**Mode:** Docs only under `Docs/architecture/mcp/`  
**Source audited:** `internal/mcp/` (10 non-test Go, 16 tests, `policy_mcp.mg`, README)  
**Wiring audited:** `internal/system/factory.go`, `factory_adapters.go`, `internal/config/integrations.go`, `internal/core/virtual_store*.go`, `internal/core/defaults/schemas_mcp.mg`, campaign consumers, e2e MCP VS tests  

**Quality bar:** `Docs/architecture/cli/` depth (narratives, diagrams, honest gaps — not inventory stubs)

### Produced set

| File | Status |
|------|--------|
| README.md | Rewritten |
| IMPLEMENTED_SPEC.md | Flagship rewrite |
| 00-ALIGNMENT-VISION-REVIEW.md | Rewritten |
| 01-VISION.md | New (contract name) |
| 02-CURRENT-STATE.md | New |
| 03-GAP-ANALYSIS.md | New |
| 04-ARCHITECTURAL-PRINCIPLES.md | New |
| 05-INTERNAL-ARCHITECTURE.md | New |
| 06-PUBLIC-API-AND-TYPES.md | New |
| 07-DEPENDENCY-MAP.md | Rewritten |
| 08-WIRING-AND-INTEGRATION.md | New |
| 09-SAFETY-AND-INVARIANTS.md | New |
| 09-MANGLE-SURFACE.md | Deep rewrite |
| 10-TESTING-ALIGNMENT.md | New |
| 11-OBSERVABILITY.md | New |
| 12-FAILURE-MODES.md | New |
| TODO.md | Rewritten |
| OPEN-QUESTIONS.md | Rewritten |
| _progress.md | This file |

### Key findings captured

1. Client + JIT compiler are real and well-tested.  
2. Boot wires adapters into VirtualStore when integrations configured.  
3. `policy_mcp.mg` and EDB fact emission are the main north-star gaps.  
4. Schema lives in defaults; package README path list is slightly stale.  

### Out of scope

- No Go/Mangle/code changes  
- No files outside `Docs/architecture/mcp/`  

### Prior thin stubs (legacy names)

Earlier auto-inventory files under alternate names (`01-DOMAIN-MODEL.md`, `02-CURRENT-STATE-MCP.md`, etc.) are **superseded** by the contract set above. Prefer the new map linked from `README.md`.
