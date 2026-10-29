# 07 — Dependency Map: `internal/types`

> Last verified: **2026-07-13**

## 1. Outbound (what `types` imports)

| Dependency | Used for | Files |
|------------|----------|-------|
| `context` | Session attach; LLM/shard signatures | `types.go`, `interfaces.go` |
| `encoding/json` | Container args in `ToAtom` | `types.go` |
| `fmt` | Stringification / errors | `types.go`, `extract.go` |
| `strings` | Name validation, join, trim | `types.go`, `extract.go` |
| `time` | Time/Duration args; tool timestamps | `types.go`, `extract.go`, `shard.go` |
| `codeberg.org/TauCeti/mangle-go/ast` | `ToAtom`, name parse | `types.go` |
| `codeberg.org/TauCeti/mangle-go/analysis` | `ProgramInfo` on Kernel | `interfaces.go` |
| `codenerd/internal/logging` | Warn before panic in `NewKernelTx` | `transaction.go` |

### Forbidden / absent (by design)

`core`, `session`, `shards`, `perception`, `articulation`, `campaign`, `world`, `store`, `cmd/nerd`.

## 2. Inbound (who imports `codenerd/internal/types`)

### Tier A — implementers & aliases

| Package | Relationship |
|---------|----------------|
| `internal/core` | Implements `Kernel`, Transactor, bridges; aliases `Fact`, `Kernel`, `LLMClient` |
| `internal/perception` | Aliases `LLMClient`; clients satisfy LLM interfaces |
| `internal/world` | Aliases `Fact`; implements/uses `GraphQuery` shape |
| `internal/core/shards` | `BaseShardAgent` DI against `types.Kernel` / LLM |
| `internal/store` | Learning store, fact codec |
| `internal/persist/factsnap` | Fact snapshot codec round-trips |

### Tier B — heavy consumers

| Package | Primary uses |
|---------|--------------|
| `internal/articulation` | Fact query, Extract*, SessionContext |
| `internal/autopoiesis` | KernelInterface, ToolInfo, LearningStore |
| `internal/campaign` | KernelTx, facts, session paging |
| `internal/session` | Executor contracts (via tests/integration) |
| `internal/init` | Agent registration with shard types |
| `internal/browser` | ExtractString on honeypot facts |
| `internal/context` | Mocks / session-related tests |
| `internal/shards/system` | System shards against types |

### Tier C — CLI / product surface

| Package | Primary uses |
|---------|--------------|
| `cmd/nerd` | Spawn, campaign, query, instruction |
| `cmd/nerd/chat` | Boot, process, delegation, dream, session context |
| `cmd/nerd/ui` | Shard pages |

### Tier D — e2e / integration

`tests/e2e/*` — extensive `types.Kernel` mocks, SessionContext isolation, piggyback boundaries, campaign/session integration.

## 3. Import direction diagram

```
                 mangle-go (ast, analysis)
                          ▲
                          │
                   internal/types
                          ▲
     ┌───────────┬────────┼────────┬───────────┬──────────┐
     │           │        │        │           │          │
   core      perception  world  articulation campaign  store/…
     │           │        │        │           │
     └───────────┴────────┼────────┴───────────┘
                          ▲
                    cmd/nerd, tests
```

`types` sits **below** all Cortex subsystems: they depend on it; it does not depend on them.

## 4. Cycle-break stories (evidence)

| Historical cycle | Mechanism |
|------------------|-----------|
| core ↔ articulation / autopoiesis | Shared interfaces & facts in `types` |
| core ↔ world (GraphQuery) | Interface moved to `types` (`interfaces.go` comment) |
| VirtualStore concrete in core | Marker interface in `types` |

## 5. Compatibility aliases (downstream)

| Downstream | Alias |
|------------|-------|
| `core.Fact` | `= types.Fact` |
| `core.Kernel` | `= types.Kernel` |
| `core.LLMClient` | `= types.LLMClient` |
| `perception.LLMClient` | `= types.LLMClient` |
| `world.Fact` | `= types.Fact` |

These keep older call sites compiling without importing `types` explicitly everywhere — but new code should prefer `types.` for clarity at boundaries.
