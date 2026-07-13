# world — Wiring and Integration

> Last verified: **2026-07-13**  
> How world is registered and called. No invented routes.

## Boot / system factory

`internal/system/factory.go` constructs:

```go
world.NewScannerWithConfig(world.ScannerConfig{ ... })
```

Scanner lives on boot context for components that need workspace topology.

## HolographicCodeScope bridge

`internal/system/holographic_code_scope.go`:

- Wraps `world.NewFileScope(projectRoot)`
- On `Open`/`Refresh`, calls `world.EnsureDeepFacts` when LocalStore available
- Retracts/loads deep facts into `core.Kernel`
- Implements CodeScope surface VirtualStore expects **without** `core` importing `world`

This is the primary wiring for **deep holographic EDB** during scoped code work.

## Chat session

| File | Wire |
|------|------|
| `session_boot.go` | Builds `world.NewScanner()` for session |
| `helpers_scan.go` | Incremental scan → `ApplyIncrementalResult`; deep Go facts via `EnsureDeepFacts` |
| `process_sync.go` | After edits, apply incremental |
| `process_dream_delegation.go` | Apply incremental post dream work |
| `model_types.go` | Fields `scanner *world.Scanner` |

Steady-state fact freshness for chat **depends on these helpers**, not continuous FS watches inside world itself.

## Init & CLI scan

| File | Wire |
|------|------|
| `internal/init/initializer.go` | Holds `*world.Scanner`, scans during init |
| `cmd/nerd/cmd_init_scan.go` | Explicit scan command path |
| `cmd/nerd/cmd_instruction.go` | ApplyIncrementalResult into cortex kernel |

## Campaign

| File | Wire |
|------|------|
| `cmd/nerd/cmd_campaign.go` | `NewScanner` + `NewHolographicProvider` |
| `internal/campaign/intelligence_gatherer.go` | Same pair for intelligence |
| `internal/campaign/edge_case_detector.go` | Scanner-driven edge cases |
| `cmd/nerd/chat/campaign*.go` | Holographic for assault/campaign UI |
| `internal/shards/system/campaign_runner.go` | Shard-side campaign uses scanner + holographic |

## System shard: world_model_ingestor

`internal/shards/system/world_model.go`:

- On-demand system shard
- Owns `*world.ASTParser`
- Continuous/tick-style maintainance of topology & symbols (separate from chat incremental)
- Permissions: read file, exec, code graph

**Integration note:** dual path with chat scan — operators should treat shard as optional/long-running ingestor, chat helpers as session-local sync.

## LSP

| File | Wire |
|------|------|
| `cmd/nerd/cmd_mangle_lsp.go` | `world/lsp.NewManager`, Initialize, ServeStdio |
| `lsp/manager.go` | Projects facts for kernel/shards when asked |

Not auto-started on every chat boot; opt-in via CLI / explicit manager use.

## Fact-flow position

```
[world emitters] → Kernel.LoadFacts / Retract*
        → policy derives file_exists, impact, context_priority, …
        → next_action / shards / VirtualStore tools
        → articulation / holographic prompt injection
```

World is **precondition** for high-quality Orient, not part of Act side-effects (except fact load).

## Registration checklist (for new world features)

1. Implement emitter in `internal/world`.
2. Decl predicate in `schemas_world.mg` (or module).
3. Add to `WorldPredicates` if full-replace must clear it.
4. Wire caller (chat helper / scope / campaign / CLI).
5. Persist path if incremental needs DB cache (`store` APIs).
6. Tests at unit + at least one integration caller if boot-critical.
