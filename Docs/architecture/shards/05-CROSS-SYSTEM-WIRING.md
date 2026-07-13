# shards — Cross-System Wiring

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded corpus**
> Mode: dark-factory autonomous generation via arch-propose/corpus-build port
> **Implementation: present under `internal/shards/` (18 non-test .go, 24 tests, 1 .mg)**


## Declared source

`internal/shards/`

## Wiring checklist (claims only for real paths)

| Surface | Path | Claim |
|---------|------|-------|
| Package root | `internal/shards/` | Exists |
| Kernel defaults | `internal/core/defaults/` | Global schemas/policy when needed |
| Shard registration | `internal/shards/registration.go` | Check if this package registers shards |
| VirtualStore | `internal/core/virtual_store.go` | Effectful routes if any |
| Session | `internal/session/` | Execution loop consumer/producer |
| CLI | `cmd/nerd/` | User-facing entry if any |

## Honesty

Do not invent routes or registrations. Grep before asserting a wire is live.
