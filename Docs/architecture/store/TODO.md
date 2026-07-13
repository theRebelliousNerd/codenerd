# store — TODO

> Last verified: **2026-07-13**  
> Prioritized backlog for `internal/store` (and docs-only notes where code change is optional).

## P1 — Correctness / search quality

1. **ANN drift healer** — periodic or startup reconcile of `vectors` vs `vec_index` rowids; surface count of drifted rows in stats/logs.
2. **Failure-injection tests** for vec_index insert failures (assert warning path + recovery).
3. **Documented dim-change procedure** — force re-embed after embedding model swap (ops runbook; partially covered by `ReembedAllDBsForce`).

## P2 — API / observability

4. **Expand `GetStats`** to include `reasoning_traces`, `prompt_atoms`, `task_verifications`, `review_findings`, `archived_facts`.
5. **Reflection backlog gauge** — log or stats for pending trace/learning embedding candidates.
6. **Optional thin interfaces** at consumer edges if cross-package mocks become painful (do not force premature abstraction).

## P3 — Ergonomics / hygiene

7. **Rename/clarify “Shard B/C/D” comments** in `local_core.go` to match expanded tier map (docs done; code comments lag).
8. **Prompt atom test density** for polymorphism columns and selector JSON round-trips.
9. **World fingerprint race tests** under concurrent scan + read.

## P4 — Horizon (do not start without need)

10. Remote/blob archival tier for huge cold sets.
11. Multi-reader pragma profile for read-mostly analytics processes.
12. OpenTelemetry metrics export.

## Explicitly not TODO

- Implementing `permitted(...)` inside store.
- Merging tools.db into knowledge.db.
- Adding package-local Mangle `.mg` files.
