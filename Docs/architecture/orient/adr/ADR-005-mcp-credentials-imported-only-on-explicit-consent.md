---
doc-class: governance
subsystem: orient
implementation-status: accepted-not-implemented
last-verified: 2026-09-29
verified-against: e056692c
supersedes: []
---

# ADR-005: MCP Credentials Are Imported Only on Explicit Consent into a Guarded Config Merge

## Context

Developers working with AI coding tools frequently maintain active Model Context Protocol (MCP) server definitions in other tool configuration files: `.mcp.json`, `.claude/settings.json`, `.claude/settings.local.json`, `.cursor/mcp.json`, or user-scoped `~/.claude.json` and `~/.codex/config.toml`. These configurations contain sensitive connection parameters, authentication headers (`X-API-Key`, `Authorization: Bearer ...`), and custom subprocess environment variables.

Two major failure modes threaten system integrity:
1. **Silent Credential Exfiltration**: Automatically copying external credentials into repository files or asserting them into shared fact databases risks accidental git commits, credential leakage into logs, or unauthorized tool access.
2. **Configuration Overwrite Corruption**: Standard Go `json.Unmarshal` followed by `json.MarshalIndent` strips developer comments, reorders keys alphabetically, and reformats whitespace across `.nerd/config.json`. Because `.nerd/config.json` is user-owned, wholesale serialization is destructive and unacceptable.

## Decision

1. **Sensory Isolation of Secrets**:
   The Go sensor (`internal/orient/mcp.go`) scans external configurations but asserts **only metadata and key names** into Mangle facts:
   - `mcp_server_seen(Name, Tool, Scope, Transport, Path)`.
   - `mcp_server_credential(Name, /header, "X-API-Key")`.
   - Secret credential values are strictly omitted from facts, logs, and orientation reports.
2. **Explicit Operator Consent Required**:
   Policy file `internal/orient/mcp.mg` derives `mcp_import_candidate(Name, Why)`. For any candidate containing credentials or residing in user scope (`Scope == /user`), the system derives an `orient_question` requiring explicit approval on first TUI boot.
3. **Guarded, Byte-Preserving Configuration Merge**:
   Upon operator approval, codeNERD executes a surgical JSON patch (`internal/config/merge.go`):
   - Decodes the original file bytes into a DOM structure.
   - Slices and inserts strictly the `integrations.servers.<name>` object path.
   - Preserves all surrounding keys, original formatting, and comments byte-for-byte outside the modified path.
   - Validates the merged result in memory using `config.LoadUserConfig` and `Check` before writing to disk; on any validation error, the write is aborted.

## Consequences

- **Positive**: Zero risk of silent credential theft; external developer tooling is easily integrated; operator configuration files remain clean and preserved.
- **Negative**: Adds a first-boot prompt step before external MCP tools become active in codeNERD.
- **Risks**: Syntax deviations in external non-standard JSON formats must be handled gracefully without panicking.

## Witness

**Witness:** `test:TestGuardedConfigMerge_PreservesExistingFields` and `symbol:internal/config.GuardedMerge`

| Claim | Witness (Code Evidence as of 2026-09-29) |
|---|---|
| Integrations config struct defined | `IntegrationsConfig` in `internal/config/integrations.go:12-45`. |
| Target surgical merge helper | `GuardedMerge(originalBytes []byte, serverKey string, server config.MCPServerIntegration) ([]byte, error)` in `internal/config/merge.go`. |
| Proving regression test | `TestGuardedConfigMerge_PreservesExistingFields` asserting that custom formatting and comments in `.nerd/config.json` survive MCP server insertion. |

## Status (Derived, Not Asserted)

**`accepted-not-implemented`**. Derivation: The decision is approved, but `internal/config/merge.go` is not yet implemented in commit `e056692c`. Status flips to `implemented` once the merge helper passes verification tests.
