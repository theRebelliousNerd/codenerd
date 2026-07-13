# store — Vision

> Last verified: **2026-07-13**  
> Package: `internal/store/`

## Product / architecture vision

codeNERD’s agent must **remember across sessions** without collapsing all memory into the next prompt. The store package is the **tiered durable memory** that makes inversion of control real over time:

- The **LLM** invents approaches and language.
- The **Mangle kernel** decides what is permitted and what runs next.
- The **store** keeps the durable substrate: facts, graph edges, embeddings, world projections, learnings, traces, and tool journals.

## Target properties

1. **Tiered by access pattern** — hot associative recall (vectors), relational graph, cold structured facts, archival demotion, separate tool/learning DBs so knowledge stays lean.
2. **Typed fact fidelity** — Mangle atoms and primitive args survive encode/decode without string soup where possible (`fact_codec`).
3. **Semantic + exact** — same knowledge atom discoverable by concept prefix **and** embedding similarity.
4. **Self-learning loop** — reasoning traces and shard learnings re-embed in the background so past work becomes retrievable.
5. **World rehydration without full rescan** — fingerprint-aware world_files / world_facts.
6. **Build-honest vector support** — sqlite-vec when CGO/headers present; explicit degrade path otherwise.
7. **Ops-friendly evolution** — additive migrations, backups on major version bumps, force re-embed across DBs after model changes.

## Non-vision (explicit)

| Not store’s job | Owner |
|-----------------|-------|
| `next_action` / `permitted` | Kernel + policy `.mg` |
| Prompt compilation / atom selection | `internal/prompt` |
| Embedding model HTTP/local engines | `internal/embedding` |
| Tool policy / sandbox | tools + VirtualStore |
| UX for browsing memory | CLI / chat |

## Success criteria (vision)

An operator can:

1. Restart `nerd` and retain cold facts, world fingerprints, session compressed state, and learnings.
2. Query knowledge semantically after embeddings backfill.
3. Archive rarely-used cold facts and restore on demand.
4. Re-embed after swapping embedding models without hand-SQL.
5. Inspect tool executions and prune by policy without touching `knowledge.db`.

## Horizon (aspirational, not claimed implemented)

- Automated ANN drift repair job
- Optional multi-reader concurrency profile
- Unified metrics surface (counts per tier in one `GetStats`)
- Pluggable remote object store for cold archival blobs (if local SQLite size becomes a hard limit)

These are backlog, not current claims — see `03-GAP-ANALYSIS.md` and `TODO.md`.
