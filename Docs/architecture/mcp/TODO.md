# mcp — TODO

> Last verified: 2026-07-13  
> Docs-only backlog derived from code audit (not a commitment schedule)

## P0 — Make Mangle selection real

- [ ] Include `policy_mcp.mg` in kernel policy load (or relocate under `internal/core/defaults/policy/`)
- [ ] Emit EDB on discover/save: `mcp_server_*`, `mcp_tool_registered`, capability, category, domain, affinity, condensed, analyzed
- [ ] Retract/update facts on disconnect / re-analyze
- [ ] Golden tests for `mcp_tool_selected` given fixture EDB + vector scores

## P1 — Lifecycle completeness

- [ ] Retain `MCPIntegrationBridge` on boot/system context for compile access
- [ ] Readiness signal after ConnectAll + initial discover
- [ ] Wire `CompileToolsForShard` (or equivalent) into shard/articulation JIT prompt path if product requires MCP tools in LLM context
- [ ] Fix `cmd_mangle_check` path: `internal/core/defaults/schemas_mcp.mg` (not missing `internal/mcp/schemas_mcp.mg`)
- [ ] Align package README structure section with on-disk files

## P2 — Selection quality

- [ ] Feed usage stats into selection (success rate / latency boost or penalty)
- [ ] Expose Info log when path is mangle vs fallback
- [ ] Revisit skeleton counter naming vs policy skeleton tools
- [ ] Optional re-analyze invalidation policy (schema hash change)

## P3 — Hardening

- [ ] Fake MCP server tests for HTTP list/call
- [ ] `-race` CI for manager+store
- [ ] Document/configure stdio sandbox expectations
- [ ] Secret redaction strategy for tool outputs in logs

## P4 — Product expansion (optional)

- [ ] MCP resources/prompts beyond tools capability flags
- [ ] Auth headers / token injection for HTTP transports
- [ ] Metrics exporter for call latency/error rates

## Non-goals (do not TODO as defects)

- Hosting an MCP server inside codeNERD
- Replacing `internal/tools` static tools entirely
- Vectryx-specific product features in this package
