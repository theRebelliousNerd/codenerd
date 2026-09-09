# mcp — TODO

> Last verified: 2026-08-16 (previous pass: 2026-07-13)  
> Docs-only backlog derived from code audit (not a commitment schedule)

> Re-audited on 2026-08-16 — nearly all of the backlog was already implemented and each closed box cites its evidence so the next audit can re-check rather than re-trust.

## P0 — Make Mangle selection real

- [x] Include `policy_mcp.mg` in kernel policy load (or relocate under `internal/core/defaults/policy/`)
  closed by the relocate option: file is internal/core/defaults/policy/policy_mcp.mg, the //go:embed defaults/policy/*.mg directive covers it, kernel_init.go lines 354-373 sweep every .mg in that directory
- [x] Emit EDB on discover/save: `mcp_server_*`, `mcp_tool_registered`, capability, category, domain, affinity, condensed, analyzed
  internal/mcp/facts.go FactEmitter: EmitServer, EmitServerStatus, EmitTool, EmitToolUsage, EmitResources, EmitPrompts
- [x] Retract/update facts on disconnect / re-analyze
  facts.go replace() retracts the previous key set before asserting, plus RetractTool and RetractServer; covered by facts_lifecycle_test.go
- [x] Golden tests for `mcp_tool_selected` given fixture EDB + vector scores
  policy_golden_test.go TestMCPPolicy_WhenFixtureEDBLoaded_ShouldMatchGoldenSelection against testdata/mcp_selection_coder.golden, plus skeleton and errored-server cases

## P1 — Lifecycle completeness

- [x] Retain `MCPIntegrationBridge` on boot/system context for compile access
  built at internal/system/factory.go:1146, stored on Cortex.mcpBridge, exposed by Cortex.MCPBridge() at factory.go:475
- [x] Readiness signal after ConnectAll + initial discover
  FactEmitter.EmitReady(serverCount, toolCount) in facts.go
- [x] Get MCP capability into the LLM prompt — closed 2026-09-09 by the control
  plane, and the answer was to change what gets rendered rather than to render
  the old thing. The item hedged with "if product requires" because injecting
  rendered tool blocks spends prompt budget on every turn whether or not the
  agent touches MCP, and nobody wanted to sign that recurring bill.

  What ships instead is a fixed five-verb surface — `mcp_map`, `mcp_probe`,
  `mcp_call`, `mcp_expand`, `mcp_context` in `internal/tools/mcpctl/` — whose
  size does not depend on how many servers or tools are connected. It rides the
  existing `AvailableTools` path (added to `coreTools` in
  `internal/prompt/config_factory.go`), so both prompt paths pick it up:
  `buildToolCatalogForPiggyback` and `buildToolDefinitions` each resolve
  `cfg.AllowedTools` through `tools.Global()`. No new rendering path in prompt
  assembly was needed, which is what made the original item look expensive.

  Measured on a 26-tool fixture server: a compact atlas is 667 bytes against
  10,905 for the equivalent catalog dump (16.3x), and result shaping took a
  74,723-byte payload to 1,008 bytes (74x) while retaining the remainder under
  an expandable handle.

- [x] Fix `cmd_mangle_check` path: `internal/core/defaults/schemas_mcp.mg` (not missing `internal/mcp/schemas_mcp.mg`)
  cmd/nerd/cmd_mangle_check.go lines 180-182 load the internal/core/defaults/ path and record why the package-local path was wrong
- [x] Align package README structure section with on-disk files
  closed 2026-08-15; headers.go, metrics.go, redact.go and resources.go existed but were absent from the tree block, all 15 non-test sources are now listed exactly once

## P2 — Selection quality

- [x] Feed usage stats into selection (success rate / latency boost or penalty)
  policy_mcp.mg mcp_tool_success_rate, mcp_tool_usage_boost_candidate, mcp_tool_usage_boost, mcp_tool_usage_penalty_candidate, fed by EmitToolUsage
- [x] Expose Info log when path is mangle vs fallback
  compiler.go:120 logs path=%s in the JIT Tool Compiler summary, and :179 warns explicitly when a Mangle query fails and the Go fallback is used
- [x] Revisit skeleton counter naming vs policy skeleton tools
  mangleSkeletonSet() at compiler.go:245 sources the count from the policy own mcp_tool_skeleton(ToolID), so the skeleton=/flesh= stats name the policy concept rather than a separate Go notion
- [x] Optional re-analyze invalidation policy (schema hash change)
  ToolSchemaHash in types.go, compared on re-discovery at client.go lines 374-409, persisted via the schema_hash column added in store.go:127

## P3 — Hardening

- [x] Fake MCP server tests for HTTP list/call
  fake_server_test.go plus transport_http_test.go
- [x] `-race` CI for manager+store — closed 2026-08-16. `.github/workflows/ci.yml` adds a `race` job running `go test -race -tags sqlite_vec ./internal/mcp/... ./internal/store/...` on windows-latest. Verified locally before it was committed: both packages pass under the race detector. Scoped to those two packages deliberately rather than the whole tree, because -race is slow and these are the ones carrying concurrent manager state worth pinning.
- [x] Document/configure stdio sandbox expectations
  internal/mcp/README.md:179 section "stdio sandbox expectations" states a stdio server runs as a subprocess with the user privileges and that there is no sandbox
- [x] Secret redaction strategy for tool outputs in logs
  redact.go plus redact_test.go

## P4 — Product expansion (optional)

- [x] MCP resources/prompts beyond tools capability flags
  resources.go, surfaced to the kernel by EmitResources / EmitPrompts
- [x] Auth headers / token injection for HTTP transports
  headers.go ExpandHeaderValues resolves ${VAR} / $VAR from the environment so a token is never committed in a workspace config file
- [x] Metrics exporter for call latency/error rates
  metrics.go plus metrics_test.go, surfaced by nerd mcp metrics at cmd/nerd/cmd_mcp_select.go:190

## Open question for the owner

The prompt-budget question that blocked P1 is answered: capability reaches the
model through a constant-size control plane rather than a rendered catalog, so
the standing per-turn cost no longer scales with the number of connected tools.

One judgement is worth a look rather than a decision: all five verbs are routed
for every intent (`intent_routing_rules.mg`) and marked `safe_action`
(`constitution.mg`), with blast radius gated one level down per remote tool by
`mcp_tool_gated`. Scoping the verbs by `verb_category` was tried and rejected —
the categories are `/code`, `/test`, `/git`, `/research`, `/learn`, `/document`
and `/verify`, so any meaningful scoping leaves a configured server unreachable
from whole regions of the taxonomy, and unreachable-by-routing fails silently.
If that trade should go the other way, the change is two files.

## Still open

- `ToolSelectionConfig` (`types.go`) is not reachable from `.nerd/config.json`.
  `SetToolSelectionConfig` and `SetConfig` exist and nothing calls them, so every
  runtime uses the hardcoded defaults. `LogicWeight`, `VectorWeight` and
  `SkeletonThreshold` are declared and never read — the compiler hardcodes
  `*7/10` and `*3/10`. Either wire them or delete the dead fields; carrying a
  knob that does nothing is worse than not having one.
- Campaign-side `MCPToolStore` injection remains optional.

## Non-goals (do not TODO as defects)

- Hosting an MCP server inside codeNERD
- Replacing `internal/tools` static tools entirely
- Vectryx-specific product features in this package
