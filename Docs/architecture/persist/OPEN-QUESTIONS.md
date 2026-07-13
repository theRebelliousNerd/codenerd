# persist — Open Questions

> Last verified: **2026-07-13**

## Q1 — Who owns the first call site?

Campaign assault already writes JSON/JSONL under `.nerd/campaigns/…`. World scan already materializes code facts. Kernel debug dumps need something denser than ad-hoc dumps.

**Options:** campaign | world | core/debug CLI | none (keep library-only).

**Impact:** Determines whether factsnap becomes platform infrastructure or stays a spare wheel.

## Q2 — Default codec for production exports?

Library default is **gzip** (portability, stdlib). Parity tests show zstd is smaller. Should first callers hardcode zstd for large world indexes?

**Tradeoff:** size vs dependency story (zstd already in module via klauspost).

## Q3 — Rehydrate policy UX?

When importing a snapshot, should facts:

- A) Assert immediately through normal kernel APIs,  
- B) Land in a shadow/dream store first,  
- C) Require explicit operator confirmation?

Constitutional safety argues against silent boot-time load (B or C preferred).

## Q4 — Should `Read` sniff magic bytes?

Suffix-only detection is simple and tested. Magic sniff would recover misnamed files but can mis-identify.

**Status:** open; not required until operators complain.

## Q5 — Unify atom conversion with core?

Duplication avoids cycles but NameType behavior **already diverges** (string vs MangleAtom). Unification needs a neutral home and a careful migration of kernel query consumers.

## Q6 — Snapshot versioning?

SimpleColumn has its own header. Do we need an outer envelope with `codenerd_snapshot_version` for future migrations?

**Status:** open; defer until first format-breaking mangle-go upgrade hurts.

## Q7 — Directory naming: `persist` vs `factsnap`?

Import path is `internal/persist/factsnap` while the directory implies a broader persistence umbrella. If no siblings appear, should the package move to `internal/factsnap`?

**Status:** low priority; avoid churn without second subpackage.

## Resolved / non-questions

| Topic | Resolution |
|-------|------------|
| Is the package pre-implementation? | **No** — full library + tests exist |
| Does factsnap enforce `permitted`? | **No** — out of scope |
| Are gzip/zstd semantically different? | **No** — proven by `TestCodecParity` |
