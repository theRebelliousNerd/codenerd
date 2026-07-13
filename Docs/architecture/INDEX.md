# Architecture Corpora Index

> Last updated: 2026-07-13 — dark-factory autonomous run

Pre-implementation design corpora and **code-grounded living-package corpora** live under
`Docs/architecture/<name>/`. Skills: `arch-propose` (design), `corpus-build` (realize/wiring),
`spec-doc-sprint` (product Spec templates under `Docs/Spec/`).

Decisions: [DARK-FACTORY-JOURNAL.md](DARK-FACTORY-JOURNAL.md)

## Realized (living code — code-grounded corpora)

| Corpus | Source | Status | Tier | Inventory | Spec |
|--------|--------|--------|------|-----------|------|
| [core](core/) | `internal/core/` | Realized — living code | T3 | 78 go / 107 tests | [IMPLEMENTED_SPEC](core/IMPLEMENTED_SPEC.md) |
| [mangle](mangle/) | `internal/mangle/` | Realized — living code | T3 | 21 go / 39 tests | [IMPLEMENTED_SPEC](mangle/IMPLEMENTED_SPEC.md) |
| [perception](perception/) | `internal/perception/` | Realized — living code | T3 | 49 go / 47 tests | [IMPLEMENTED_SPEC](perception/IMPLEMENTED_SPEC.md) |
| [articulation](articulation/) | `internal/articulation/` | Realized — living code | T2 | 8 go / 7 tests | [IMPLEMENTED_SPEC](articulation/IMPLEMENTED_SPEC.md) |
| [prompt](prompt/) | `internal/prompt/` | Realized — living code | T3 | 25 go / 32 tests | [IMPLEMENTED_SPEC](prompt/IMPLEMENTED_SPEC.md) |
| [session](session/) | `internal/session/` | Realized — living code | T2 | 6 go / 14 tests | [IMPLEMENTED_SPEC](session/IMPLEMENTED_SPEC.md) |
| [shards](shards/) | `internal/shards/` | Realized — living code | T3 | 18 go / 24 tests | [IMPLEMENTED_SPEC](shards/IMPLEMENTED_SPEC.md) |
| [campaign](campaign/) | `internal/campaign/` | Realized — living code | T2 | 44 go / 29 tests | [IMPLEMENTED_SPEC](campaign/IMPLEMENTED_SPEC.md) |
| [config](config/) | `internal/config/` | Realized — living code | T2 | 17 go / 4 tests | [IMPLEMENTED_SPEC](config/IMPLEMENTED_SPEC.md) |
| [store](store/) | `internal/store/` | Realized — living code | T2 | 39 go / 44 tests | [IMPLEMENTED_SPEC](store/IMPLEMENTED_SPEC.md) |
| [tools](tools/) | `internal/tools/` | Realized — living code | T2 | 25 go / 21 tests | [IMPLEMENTED_SPEC](tools/IMPLEMENTED_SPEC.md) |
| [cli](cli/) | `cmd/nerd/` | Realized — living code | T3 | 113 go / 55 tests | [IMPLEMENTED_SPEC](cli/IMPLEMENTED_SPEC.md) |

## Proposed (pre-implementation only)

| Feature | Corpus | Status | Notes |
|---------|--------|--------|-------|
| — | — | none this run | New greenfield features register here via `/arch-propose` |

## Explicitly out of scope (leaves)

`internal/diff`, `internal/types`, `internal/sqlpragmas`, `internal/ux`, `internal/usage`, `internal/verification`, `internal/regression`, `internal/features`, `internal/build`, `internal/observability`, `internal/northstar`, `internal/persist`, `internal/tactile`, `internal/retrieval`, `internal/transparency`, `internal/testing`, `internal/init`, `internal/browser`, `internal/embedding`, `internal/world`, `internal/mcp`, `internal/system`, `internal/autopoiesis`, `internal/context`, `internal/logging`, `internal/jit` — index-only / parent-folded; not full corpora.

## Pipeline

```
arch-propose (greenfield) → corpus-build (implement) → living code
existing packages → code-grounded corpus generation (this run)
```
