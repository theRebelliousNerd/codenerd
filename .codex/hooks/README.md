# codeNERD Codex hooks

The owning representation is `.codex/hooks.json`.

## Active hooks

- `inject-subagent-memory-context.py`: on `SubagentStart`, injects the live
  `.claude/agent-memory/<agent>/MEMORY.md` content when present. The shared
  Claude/Codex memory corpus remains Claude-owned on disk; no parallel Codex
  journal is fabricated.
- `corpus-build/corpus-fleet-start.ps1`: records tracked corpus fleet starts.
- `corpus-build/corpus-token-meter.ps1`: records tracked corpus fleet stops
  and explicit runtime usage fields when available.

Corpus telemetry is gated by
`.corpus-build/ledger/<session_id>.active`. Untracked subagent activity exits
quietly.

## Deliberate non-port

The Claude-side compile and write-scope enforcement hooks remain under
`.claude/hooks/corpus-build/` and are not active Codex hooks. Their current
payload assumptions and blocked-path policy are Claude-specific and include
stale source-repo constraints. Enabling them would conflict with codeNERD's
documented `go test ./...` and `go build ./cmd/nerd` workflow.

Codex uses bounded agent packets, sandbox settings, config registration,
constitutional Mangle policy, review gates, and tests for enforcement. Hooks are
guardrails, not a complete security boundary.

