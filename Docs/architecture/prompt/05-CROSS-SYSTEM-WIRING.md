# prompt — Cross-System Wiring

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/prompt/` (complete internal coverage)
> **Implementation: `internal/prompt/` — 25 non-test .go, 32 tests, 0 .mg**


## Owned package

`internal/prompt/`

## Integration checklist (verify before claiming live)

| Surface | Path | Notes |
|---------|------|-------|
| This package | `internal/prompt/` | Exists |
| Kernel | `internal/core/` | Facts / VirtualStore / Dreamer |
| Mangle engine | `internal/mangle/` | Evaluation / feedback |
| Schemas/policy | `internal/core/defaults/` | Global Decl/policy |
| Shard registration | `internal/shards/registration.go` | If registers shards |
| Session | `internal/session/` | Execution loop |
| Prompt JIT | `internal/prompt/` | Atoms/compiler |
| Articulation | `internal/articulation/` | Piggyback/assembly |
| Config | `internal/config/` | Settings |
| CLI | `cmd/nerd/` | User entry |
| Tools/MCP | `internal/tools/`, `internal/mcp/` | External tools |

## Honesty

Do not invent routes or registrations. Grep registration hubs and callers before asserting a wire is live for **prompt**.
