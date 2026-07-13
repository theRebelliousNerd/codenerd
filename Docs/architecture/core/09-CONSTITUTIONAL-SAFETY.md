# core — Constitutional Safety

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/core/` (complete internal coverage)
> **Implementation: `internal/core/` — 78 non-test .go, 107 tests, 129 .mg**


- Default deny; `permitted(...)`
- Dreamer: `internal/core/dreamer.go` when present
- Package role: Mangle kernel, VirtualStore, Dreamer, facts, API scheduler, shard manager plumbing
