# shards — Constitutional Safety

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded corpus**
> Mode: dark-factory autonomous generation via arch-propose/corpus-build port
> **Implementation: present under `internal/shards/` (18 non-test .go, 24 tests, 1 .mg)**


## Rules

- Default deny; actions require `permitted(...)`
- Dreamer precog lives under core (`internal/core/dreamer.go` when present)
- Dangerous shell/file ops must remain gated

## Package role

Domain/system shard implementations and registration
