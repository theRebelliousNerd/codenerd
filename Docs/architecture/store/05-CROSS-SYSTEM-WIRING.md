# store — Cross-System Wiring

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/store/` (complete internal coverage)
> **Implementation: `internal/store/` — 39 non-test .go, 44 tests, 0 .mg**


## Owned package

`internal/store/`

## Integration checklist (verify before claiming live)

| Surface | Path | Notes |
|---------|------|-------|
| This package | `internal/store/` | Exists |
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

Do not invent routes or registrations. Grep registration hubs and callers before asserting a wire is live for **store**.
