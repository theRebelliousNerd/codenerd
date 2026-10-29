# store — Architectural Principles

> Last verified: **2026-07-13**  
> Binding for work **inside** `internal/store/` (and for callers that depend on its contracts).

## 1. Store is substrate, not executive

Never implement next-action selection, constitutional permission, or campaign orchestration inside store. Accept structured writes; leave policy to Mangle / VirtualStore.

## 2. Tier by access pattern, not by feature fashion

Put durable logic facts in cold storage; associative recall in vectors; relations in the graph; ephemeral tool I/O in tools.db; shard self-modification in learnings DBs. Do not collapse tiers for convenience.

## 3. Single-writer SQLite process model

`SetMaxOpenConns(1)` + `sync.RWMutex` is the concurrency contract. Do not assume multi-writer multi-process access. Background work must take the same locks / respect writer serialization.

## 4. Prefer additive schema evolution

Migrations add columns / indexes; avoid destructive rewrites of production tables without backup paths (`RunAllMigrations` model). `CREATE TABLE IF NOT EXISTS` for base schema.

## 5. Dual-write honesty for ANN

If both `vectors` and `vec_index` are in play, treat vec insert failures as **real** failures of search quality. Log loudly; prefer fix paths over silent success.

## 6. Embeddings are optional; structured memory is not

Keyword / exact / cold paths must remain usable when no embedding engine is configured. Semantic features degrade; constitutional fact storage must not.

## 7. Typed fact codecs over raw JSON where Mangle cares

Cold fact args use tagged encode/decode so `types.MangleAtom` and numeric types survive. Do not casually switch to `fmt.Sprintf` for core cold storage.

## 8. Separate DBs for bloat-prone journals

Tool executions and per-shard learnings stay out of `knowledge.db` by design. New high-volume append-only logs should default to isolation unless cross-table joins are proven necessary.

## 9. Reflection is asynchronous maintenance

Descriptor/embedding refresh must not block TUI boot (vector index backfill is already background). Reflection worker is best-effort hygiene, not the only write path for truth.

## 10. Import-cycle hygiene

Store may import `embedding`, `config`, `logging`, `types`, `sqlpragmas`, `defaults` for corpora. It must **not** import `core`, `session`, or perception types that would cycle — use `any` + local mirrors (`ReasoningTrace`) and adapters in `system`.

## 11. Pragmas via leaf package

Do not re-implement SQLite pragma sets. Re-export or call `sqlpragmas` so MCP and other cycle-prone packages can import the leaf.

## 12. Wiring audit before deletion

Satellite stores and “unused” methods may be invoked only from `cmd/`, campaigns, or reflection. Grep reverse deps before removing APIs.
