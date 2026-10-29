# sqlpragmas — Vision

> Last verified: **2026-07-13**  
> Target product/architecture vision for SQLite pragma governance in codeNERD.

## Problem the package solves

codeNERD opens **many** SQLite databases:

- Project LocalStore / knowledge
- Learned shards and tool stores
- Prompt atom / compiler caches
- MCP server registry store
- Northstar / feedback / strategy stores
- Predicate corpus (often read-only)
- One-shot builder tools and chat ingest paths

Without a shared preset:

1. Each open invents (or forgets) WAL, busy_timeout, cache, mmap.
2. Import cycles appear when mid-layer packages try to “borrow” store helpers.
3. Driver differences (mattn vs modernc) produce noisy open failures.
4. Read-only opens spam logs trying to set write pragmas.

## Target vision

### Single leaf of truth

All intentional SQLite opens in the product either:

- call `sqlpragmas.ApplyDefaultPragmas`, or  
- call the thin `store` re-export of the same function,

with a **profile that matches the workload**, immediately after `sql.Open`.

### Profile vocabulary is product vocabulary

| Profile | Mental model |
|---------|----------------|
| Hot | “This handle lives for a session or service lifetime.” |
| BulkBuild | “This process fills a DB then exits (or is a bulk migration).” |
| Query | “Short tool: read, maybe light write, exit.” |
| ReadOnly | “I promised not to write; do not try.” |

New open sites pick from this vocabulary rather than inventing ad-hoc PRAGMA lists.

### Best-effort portability

Pragma application must never prevent a database from opening. Tuning is opportunistic: workstation gets full benefit; constrained hosts and strict drivers degrade gracefully with Debug traces.

### Explicit, rare exceptions

- FK enforcement remains **opt-in** at schema-owning sites until a coordinated data migration.
- Packages that cannot import `store` import `sqlpragmas` (leaf).
- Custom one-off PRAGMAs after apply are allowed but should be rare and documented.

## Non-goals (vision)

- Becoming a general DB connection factory.
- Encoding Mangle policy about which DB may open.
- Auto-detecting “best” profile from path heuristics.
- Supporting non-SQLite engines.

## Success criteria

1. **Grep hygiene:** nearly every `sql.Open` for product SQLite is followed by `ApplyDefaultPragmas` (or a justified comment).
2. **Zero import cycles** involving store ↔ mcp ↔ prompt mid-layers for pragma reasons.
3. **Tests green** on CI for all four profiles on the primary CGO driver.
4. **No Debug spam** on read-only opens in normal operation.
5. **Documented profile choice** at each major open site (comment or architecture map).

## Evolution direction (aspirational, not claimed implemented)

| Idea | Why |
|------|-----|
| Optional config overrides for cache/mmap | Laptops vs 128 GB workstations |
| `ApplyDefaultPragmas` connection hook helper | Safer multi-conn pools |
| Dual-driver integration test build tag | Catch modernc rejects early |
| `EnableForeignKeys(db)` helper | Opt-in without copy-paste |

These stay out of the leaf until needed; vision prefers **stability of the apply contract** over feature growth.
