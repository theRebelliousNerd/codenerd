# mcp — TODO

> Last verified: 2026-08-15 (previous pass: 2026-07-13)  
> Docs-only backlog derived from code audit (not a commitment schedule)

> Re-audited on 2026-08-15 — nearly all of the backlog was already implemented and each closed box cites its evidence so the next audit can re-check rather than re-trust.

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
- [ ] Wire `CompileToolsForShard` (or equivalent) into shard/articulation JIT prompt path if product requires MCP tools in LLM context — genuinely open, gated on the "if"
  CompileToolsForShard exists at integration.go:257 and the bridge is retained, but Cortex.MCPBridge() has no consumers and the only non-test caller of the compile path is the nerd mcp select CLI at cmd/nerd/cmd_mcp_select.go, so MCP tools never reach an LLM prompt; closing it is a product decision, not a wiring oversight.
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
- [ ] `-race` CI for manager+store — blocked rather than deferred by choice
  this repository has no CI configuration at all so there is no pipeline to add a -race job to; go test -race ./internal/mcp/... runs locally today, and the box cannot close until CI exists
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

Everything is closed except two items, and neither is ordinary implementation work: (1) should MCP tools appear in LLM prompts at all — the whole selection stack is built and tested but its only consumer is a CLI command, and wiring it into the articulation/JIT prompt path would spend prompt budget every turn, which is why the original item hedged with "if product requires", a condition never decided; (2) -race CI cannot be actioned until the repository has CI.

## Non-goals (do not TODO as defects)

- Hosting an MCP server inside codeNERD
- Replacing `internal/tools` static tools entirely
- Vectryx-specific product features in this package
