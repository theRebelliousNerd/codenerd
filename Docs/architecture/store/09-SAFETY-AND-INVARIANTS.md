# store — Safety and Invariants

> Last verified: **2026-07-13**  
> Package: `internal/store/`

## Scope of safety

Store enforces **data integrity and process safety**, not **constitutional policy**. Callers must ensure writes are permitted before invoking store APIs.

## Invariants

### I1 — Single-writer process model

- `db.SetMaxOpenConns(1)` on LocalStore, ToolStore, and typical open paths.
- Package-level `sync.RWMutex` on multi-method types.
- **Violation risk:** opening the same DB from multiple OS processes without SQLite multi-process discipline.

### I2 — Unique keys where upsert matters

| Table | Uniqueness |
|-------|------------|
| cold_storage | `(predicate, args)` |
| archived_facts | `(predicate, args)` |
| knowledge_graph | `(entity_a, relation, entity_b)` |
| session_history | `(session_id, turn_number)` |
| compressed_states | `(session_id, turn_number)` |
| world_files | `path` PK |
| world_facts | `(path, depth, predicate, args)` |
| prompt_atoms | `atom_id` UNIQUE |
| learning_candidates | `(phrase, verb, target, reason)` |
| tool_executions | `call_id` UNIQUE |
| predicate vectors | partial unique on `content` when metadata kind=predicate |

### I3 — Access tracking on cold LoadFacts

`LoadFacts` updates `last_accessed` and increments `access_count`. Archival decisions depend on this; silent bypass of tracking would skew maintenance.

### I4 — Fact codec fidelity

Cold args must round-trip through `encodeFactArgs`/`decodeFactArgs`. `types.MangleAtom` must not collapse to bare string without type tag.

### I5 — ANN dual-write visibility

When `vectorExt` is true, vec_index insert errors are **logged as warnings** (ANN drift). Invariant for search quality: rowids align between `vectors` and `vec_index` after successful writes and after backfill.

### I6 — Require-vec fail-fast (tagged builds)

If `defaultRequireVec && !vectorExt`, `NewLocalStore` returns error and closes DB. Prevents “thought ANN works” production configs without extension.

### I7 — Reflection worker lifecycle

- Started only when reflection enabled **and** embedding engine set.
- Stopped on `Close`, engine nil, or config disable.
- Must not leave goroutines unbounded after Close (stop channel + done wait with timeout).

### I8 — Embedded corpus isolation

RO open (`mode=ro`), temp file extract, `ProfileReadOnly` pragmas (no WAL write attempts). Close removes temp file.

### I9 — Schema migrations are additive

`pendingMigrations` only adds columns. Missing tables skip. Do not invent destructive migrations without backup (`RunAllMigrations`).

### I10 — No import of policy engine

Store must not call kernel `permitted` or load policy `.mg`. Safety composition stays at VirtualStore/kernel.

## Concurrency checklist for new APIs

1. Take `mu` appropriately (write = Lock, pure read = RLock).
2. Do not hold lock across long network embed calls if avoidable (batch paths carefully unlock/relock as existing code does).
3. Do not spawn unbounded goroutines without stop signals.
4. Prefer transactions for multi-row vector batches.

## Threat / abuse notes (local agent)

| Risk | Mitigation |
|------|------------|
| Path traversal in world file keys | Caller sanitizes paths; store stores opaque strings |
| Unbounded tool Result size | Cleanup policies (size/runtime); not truncation on write |
| Prompt injection via stored content | Not store’s job; retrieval consumers must treat content as untrusted data |
| Disk fill | Maintenance vacuum, archival purge, tool cleanup |

## Mangle Decl

**N/A** — no local `.mg` sources. Decl obligations apply to consumers asserting hydrated graph/facts into the kernel.
