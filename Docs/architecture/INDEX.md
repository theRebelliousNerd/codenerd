# Architecture Corpora Index

> Last updated: 2026-07-13 — **1:1 with every `internal/*` package** + deep rebuild wave

Each top-level directory under `internal/` has a matching architecture corpus.
Deep rebuild: one subagent per package using `Docs/architecture/_rebuild/SUBAGENT_INSTRUCTIONS.md`.
Decisions: [DARK-FACTORY-JOURNAL.md](DARK-FACTORY-JOURNAL.md)

## Quality standard

- **Current standard:** [`_rebuild/SUPERSTAR_CORPUS_STANDARD.md`](_rebuild/SUPERSTAR_CORPUS_STANDARD.md).
- **Machine ownership:** [`portfolio.toml`](portfolio.toml) plus one `corpus.toml` per corpus.
- **Legacy decisions:** [`_rebuild/LEGACY_MIGRATION_LEDGER.md`](_rebuild/LEGACY_MIGRATION_LEDGER.md) records every compatibility filename and deletion gate.
- **Reference depth:** `Docs/architecture/cli/` (cmd/nerd) and dense packages (core, session, mangle, campaign, prompt, …).
- **Rejected:** auto-inventory stubs without behavioral narrative.
- **Docs only** for the deep rebuild — no Go/Mangle changes in that wave.

## Realized — internal packages (37)

| Corpus | Source | Status | Inventory | SPEC size | Spec |
|--------|--------|--------|-----------|-----------|------|
| [articulation](articulation/) | `internal/articulation/` | Realized — concise 3-file corpus | 8 go / 7 tests | 11353B | [SPEC](articulation/README.md) |
| [autopoiesis](autopoiesis/) | `internal/autopoiesis/` | Realized — concise 3-file corpus | 37 go / 30 tests | 22937B | [SPEC](autopoiesis/README.md) |
| [browser](browser/) | `internal/browser/` | Realized — BPAR-1 partial, parity uplift active | 7 go / 10 tests | 18042B | [SPEC](browser/README.md) |
| [build](build/) | `internal/build/` | Realized — concise 3-file corpus | 2 go / 6 tests | 6928B | [SPEC](build/README.md) |
| [campaign](campaign/) | `internal/campaign/` | Realized — concise 3-file corpus | 32 go / 17 tests | — | [SPEC](campaign/README.md) |
| [config](config/) | `internal/config/` | Realized | 3 docs | 13458B | [README](config/README.md) |
| [context](context/) | `internal/context/` | Realized — concise 3-file corpus | 10 go / 26 tests | 8451B | [SPEC](context/README.md) |
| [core](core/) | `internal/core/` | Realized — concise 3-file corpus | 78 go / 107 tests | — | [SPEC](core/README.md) |
| [diff](diff/) | `internal/diff/` | Realized — deep corpus | 1 go / 2 tests | 15987B | [SPEC](diff/README.md) |
| [embedding](embedding/) | `internal/embedding/` | Realized — concise 3-file corpus | 6 go / 9 tests | — | [SPEC](embedding/README.md) |
| [features](features/) | `internal/features/` | Realized — concise 3-file corpus | 2 go / 6 tests | — | [SPEC](features/README.md) |
| [init](init/) | `internal/init/` | Realized — concise 3-file corpus | 17 go / 20 tests | — | [SPEC](init/README.md) |
| [jit](jit/) | `internal/jit/` | Realized — concise 3-file corpus | 1 go / 1 tests | — | [SPEC](jit/README.md) |
| [logging](logging/) | `internal/logging/` | Realized — concise 3-file corpus (README + INTERNALS + WIRING-AND-NOT-BUILT) | 9 go / 12 tests | 11537B | [SPEC](logging/README.md) |
| [mangle](mangle/) | `internal/mangle/` | Realized — concise 3-file corpus | 21 go / 39 tests | — | [SPEC](mangle/README.md) |
| [mcp](mcp/) | `internal/mcp/` | Realized — deep corpus | 10 go / 16 tests | 15870B | [SPEC](mcp/IMPLEMENTED_SPEC.md) |
| [northstar](northstar/) | `internal/northstar/` | Realized — deep corpus | 4 go / 6 tests | 12302B | [SPEC](northstar/IMPLEMENTED_SPEC.md) |
| [observability](observability/) | `internal/observability/` | Realized — deep corpus | 2 go / 3 tests | 14906B | [SPEC](observability/IMPLEMENTED_SPEC.md) |
| [perception](perception/) | `internal/perception/` | Realized — deep corpus | 50 go / 48 tests | 24208B | [SPEC](perception/IMPLEMENTED_SPEC.md) |
| [persist](persist/) | `internal/persist/` | Realized — deep corpus | 1 go / 4 tests | 15449B | [SPEC](persist/IMPLEMENTED_SPEC.md) |
| [prompt](prompt/) | `internal/prompt/` | Realized — deep corpus | 25 go / 32 tests | 24108B | [SPEC](prompt/IMPLEMENTED_SPEC.md) |
| [regression](regression/) | `internal/regression/` | Realized — deep corpus | 1 go / 1 tests | 16878B | [SPEC](regression/IMPLEMENTED_SPEC.md) |
| [retrieval](retrieval/) | `internal/retrieval/` | Realized — deep corpus | 4 go / 6 tests | 16215B | [SPEC](retrieval/IMPLEMENTED_SPEC.md) |
| [session](session/) | `internal/session/` | Realized — deep corpus | 6 go / 14 tests | 29637B | [SPEC](session/IMPLEMENTED_SPEC.md) |
| [shards](shards/) | `internal/shards/` | Realized — deep corpus | 18 go / 24 tests | 22859B | [SPEC](shards/IMPLEMENTED_SPEC.md) |
| [sqlpragmas](sqlpragmas/) | `internal/sqlpragmas/` | Realized — deep corpus | 1 go / 2 tests | 16642B | [SPEC](sqlpragmas/IMPLEMENTED_SPEC.md) |
| [store](store/) | `internal/store/` | Realized — deep corpus | 39 go / 44 tests | 25755B | [SPEC](store/IMPLEMENTED_SPEC.md) |
| [system](system/) | `internal/system/` | Realized — deep corpus | 5 go / 11 tests | 21263B | [SPEC](system/IMPLEMENTED_SPEC.md) |
| [tactile](tactile/) | `internal/tactile/` | Realized — deep corpus | 16 go / 12 tests | 20620B | [SPEC](tactile/IMPLEMENTED_SPEC.md) |
| [testing](testing/) | `internal/testing/` | Realized — deep corpus | 21 go / 8 tests | 22372B | [SPEC](testing/IMPLEMENTED_SPEC.md) |
| [tools](tools/) | `internal/tools/` | Realized — deep corpus | 25 go / 21 tests | 21378B | [SPEC](tools/IMPLEMENTED_SPEC.md) |
| [transparency](transparency/) | `internal/transparency/` | Realized — deep corpus | 8 go / 9 tests | 16178B | [SPEC](transparency/IMPLEMENTED_SPEC.md) |
| [types](types/) | `internal/types/` | Realized — deep corpus | 5 go / 4 tests | 16449B | [SPEC](types/IMPLEMENTED_SPEC.md) |
| [usage](usage/) | `internal/usage/` | Realized — deep corpus | 2 go / 4 tests | 12243B | [SPEC](usage/IMPLEMENTED_SPEC.md) |
| [ux](ux/) | `internal/ux/` | Realized — deep corpus | 4 go / 4 tests | 14363B | [SPEC](ux/IMPLEMENTED_SPEC.md) |
| [verification](verification/) | `internal/verification/` | Realized — deep corpus | 1 go / 3 tests | 16936B | [SPEC](verification/IMPLEMENTED_SPEC.md) |
| [world](world/) | `internal/world/` | Realized — deep corpus | 37 go / 31 tests | 25025B | [SPEC](world/IMPLEMENTED_SPEC.md) |

## Non-internal surfaces

| Corpus | Source | Notes |
|--------|--------|-------|
| [cli](cli/) | `cmd/nerd/` | Deep-dive quality bar (Cobra + TUI + wiring) |

## Proposed (greenfield only)

| Feature | Notes |
|---------|-------|
| — | New packages: run arch-propose, then add 1:1 corpus under Docs/architecture/<name>/ |

## Coverage rule

```
for dir in internal/*:
  require Docs/architecture/<dir>/IMPLEMENTED_SPEC.md (deep, code-grounded)
```

Validate sizes / presence:

```powershell
python .agents/skills/corpus-build/scripts/validate_architecture_corpora.py
```
