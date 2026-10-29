# retrieval — Safety and Invariants

> Last verified: **2026-07-13**

## 1. Threat model (package scope)

| Asset | Risk | Mitigations present |
|-------|------|---------------------|
| Host FS | Read many files under workDir | Scoped to `workDir` walk; exclude patterns |
| Memory | Huge hit lists / large files | Cache size; result limits; **no** max file size |
| CPU | Pathological extract / scan | Empty short-circuit; timeout; ReDoS-minded tests |
| Integrity of ranking | Cache aliasing | Slice clones on Get/Set |
| Agent authority | Accidental writes | Package is read-only (os.ReadFile/Open/Stat/Walk) |

Constitutional `permitted(...)` is **orthogonal**: this package never requests tools.

## 2. Invariants

### I1 — Read-only filesystem

No `WriteFile`, `Remove`, `Mkdir` in package sources. Violation = critical defect.

### I2 — Context cancellation observed

`searchSingleKeyword` returns error on `ctx.Err()` after workers stop. Callers must not treat partial hits as complete on error.

### I3 — Word-boundary matches only

Hits that fail `isWordBoundary` are dropped. Ranking must not reintroduce raw substring noise without review.

### I4 — Cache isolation

`Get`/`Set` clone `[]KeywordHit`. Tests cover concurrent Get/Set.

### I5 — Tier budgets non-negative

`maxTierN` derived from float budgets × MaxTotal; zero MaxTotal becomes 50 in builder ctor.

### I6 — Path comparison normalized

Mentioned-file tier and extract paths use `normalizePathSeparators`.

### I7 — No kernel side effects inside package

Package must not call `LoadFacts` or Mangle; callers own EDB mutation.

### I8 — Default deny of common noise dirs

Default exclude list skips VCS and package manager trees; operators may override via config (can weaken safety if they clear excludes).

## 3. Concurrency invariants

- Cache: single mutex serializes mutations; safe for parallel keyword searches sharing one retriever.
- Hit accumulation in search: mutex around append to shared `hits`.
- `SparseRetriever.mu` is currently **unused** — do not assume it guards workDir mutation; treat config as immutable after `NewSparseRetriever`.

## 4. Input safety

| Input | Behavior |
|-------|----------|
| Empty issue text | Empty keywords |
| Null bytes in text | Must not crash (tested) |
| Very long strings | ExtractKeywords time-bounded in tests (1s for 100k `a`) |
| Windows paths in extract | Normalized to `/` |
| `parseRipgrepOutput` + drive letters | Documented field-split pitfall |

## 5. Resource bounds checklist

| Bound | Present? |
|-------|----------|
| SearchTimeout | yes (default 30s) |
| MaxResults / Rank limit | yes |
| CacheSize / TTL | yes |
| Parallelism | yes |
| Max file bytes | **no** |
| Max total walk bytes | **no** |
| Max hits absolute | **no** (only ranking limit) |
| Symlink cycle guards | WalkDir default (can loop if cycles allowed by OS/walk) |

## 6. Mangle Decl relevance

When wiring producers, every asserted predicate **must** match Decl in `schemas_knowledge.mg`. Example required shapes:

```
issue_keyword(IssueID, Keyword, Weight)
file_mentioned(File, IssueID)
tiered_context_file(IssueID, File, Tier, Relevance, TokenCount)
candidate_file(File, RelevanceScore)
keyword_hit(File, Keyword, Count)
```

Tier names in seed today use `/tier1` (name atom style), consistent with Decl `bound [..., /name, ...]`.

## 7. Security notes for multi-tenant futures

If workDir can point at hostile trees:

- Prefer symlink evaluation policy
- Cap read size
- Avoid logging full file contents from hits (today Context is single line — acceptable)

## 8. Failure safety vs silent degradation

`BuildContext` logs Tier2 errors and continues. That is **availability over correctness**. For high-assurance modes, callers should inspect logs or wrap APIs that surface hard errors.
