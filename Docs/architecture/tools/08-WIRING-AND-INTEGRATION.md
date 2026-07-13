# tools — Wiring and Integration

> Last verified: **2026-07-13**

## Boot wiring

### Primary hydrate

`internal/system/factory.go` calls:

```text
bctx.virtualStore.HydrateModularTools()
```

Implementation: `internal/core/virtual_store_tools.go` → `HydrateModularTools`:

1. `core.RegisterAll(registry)` + `core.RegisterAll(tools.Global())`  
2. same for `shell`, `codedom`, `research`  
3. logs counts for both registries  

VirtualStore constructs `modularTools: tools.NewRegistry()` in `virtual_store.go` constructor.

### Test impact provider

Expect boot/world path to call `codedom.RegisterTestImpactProvider(...)` so impact tools work. Without it, those two tools error at execute time (still registered).

### Workspace root env

Session/workspace startup may set `CODENERD_WORKSPACE_ROOT` for file-tool containment (`workspace_guard.go` comment).

## Session interactive path

`internal/session/executor_tools.go`:

| Step | Function / gate |
|------|-----------------|
| Tool loop | `runToolLoop` / piggyback batch |
| Allowlist | `isToolAllowed` vs `cfg.AllowedTools` |
| Constitutional | `checkSafety` → `pending_action` / `permitted` |
| Executive preflight | `InteractiveExecutiveGate.PreflightDestructiveToolCall` |
| Execute | `tools.Global().Execute` |
| Post-validate | `ValidateInteractiveToolResult` |
| Fallback | Ouroboros `ExecuteRegisteredTool` |

### ConfigFactory

`jit/config.EffectiveAgentRuntimeConfig.AllowedTools` is the runtime allowlist. Empty/nil → **allow all** modular names (documented e2e behavior). ConfigFactory is expected to populate from intent/JIT; if it fails open, tools are unrestricted at this layer.

### Prompt surface

Prompt assembly uses AvailableTools / AllowedTools for `{{available_tools}}` and tool definitions (`buildToolDefinitions`). Tool *schemas* come from modular registry tools matching allowed names.

## VirtualStore action path

VS holds `modularTools` for:

- `GetModularTools` / `RegisterModularTool`  
- Research-related action handlers that execute modular tools by name (`virtual_store_actions.go`)  

Compiled/Ouroboros tools use separate `toolRegistry` + `HydrateToolsFromDisk` / `HydrateStaticTools` — **not** this package’s RegisterAll.

## Mangle routing

`internal/mangle/intent_routing.mg` Section 4.5 defines which modular tools are allowed for which intents (read tools universal; write/shell by category; research suite for research/learn/document/verify).

Session does not currently re-query `modular_tool_allowed` per call in `isToolAllowed`; integration assumes ConfigFactory already reflected policy into AllowedTools.

## Init / campaign / autopoiesis

Direct imports of `tools/research` for:

- Context7-style knowledge hydration  
- `GroundingHelper` for Gemini search grounding  
- Decomposer/replan research assist  

These bypass the LLM tool_call loop; they call helpers/APIs as libraries.

## World

`internal/world/test_dependency.go` collaborates with codedom interfaces for impact analysis (implements analyzer used via provider).

## E2E

Tests register dummy tools on registries / set AllowedTools to prove:

- Forbidden tools blocked  
- Empty config open behavior  
- Timeout / hanging tools  
- Piggyback vs native loops  

## Wiring gaps to audit (not delete)

| Symptom | Check |
|---------|-------|
| Tool missing at runtime | HydrateModularTools called? Global vs VS? |
| Impact tools always error | RegisterTestImpactProvider? |
| Path escape | Which tool family? guard applied? |
| git tools never allowed | intent_routing + AllowedTools |
| Research works in init but not chat | Intent category / AllowedTools |
