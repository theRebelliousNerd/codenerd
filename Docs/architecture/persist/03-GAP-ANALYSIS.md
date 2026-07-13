# 03 — Gap Analysis: persist

> Last verified against codebase: **2026-07-13**

## 1. Spec vs reality matrix

| Desired capability | Reality | Gap? |
|--------------------|---------|------|
| Compact fact file format | SimpleColumn + gzip/zstd implemented | **No** |
| Round-trip fidelity for common facts | Tests at 1k–10k with equalish compare | **No** |
| Atomic writes | tmp + sync + rename | **No** |
| Legacy JSON migration | `Read` + `LegacyJSON` | **No** |
| Production export path | No importers | **Yes — P0** |
| Production import / rehydrate | No importers | **Yes — P0** |
| CLI operator surface (`nerd snapshot …`) | Absent | **Yes — P1** |
| Content sniff if extension wrong | Suffix-only `detectCodec` | **Yes — P2** |
| Shared atom conversion with core | Duplicated; NameType differs (MangleAtom vs string) | **Yes — P2** (document + optional shared helper later) |
| Logging category | None | **Yes — P3** |
| Streaming / append snapshots | Full rewrite only | **Yes — P3** (may stay non-goal) |
| Integrity hash (SHA of raw SC) | None | **Yes — P3** |
| Root `package persist` re-exports | Directory only | **Yes — P3** (cosmetic) |
| Concurrency-safe multi-writer | None | **Non-gap** if callers serialize |
| Policy enforcement inside factsnap | None | **Non-gap** (correct boundary) |

## 2. Priority backlog (from gaps)

### P0 — Wire or explicitly demote

Without a caller, the package cannot earn its keep in the platform story. Candidates (docs-only recommendation; not implemented here):

1. Campaign phase / assault **fact bag** export alongside JSONL results.  
2. World scan freeze of `code_*` facts.  
3. Kernel debug dump alternative to ad-hoc JSON.  
4. CLI: `nerd facts export|import` style verb.

Until then, mark in audits as **“clean but unwired”** (matches `AUDIT.md` “clean” row for `internal/persist/factsnap`).

### P1 — Operator path

Even a thin CLI wrapping `Write`/`Read` would make the library real for humans and scripts.

### P2 — Hardening

- Optional magic-byte detection when suffix is missing.  
- Comment/link in both `core` and `factsnap` atom conversion noting intentional NameType divergence.  
- Integration test once a first caller lands.

### P3 — Polish

- Logging: `CategoryStore` or a dedicated category on write size / duration.  
- SHA-256 sidecar or header.  
- Streaming writer if multi-million fact dumps appear.

## 3. Non-gaps (do not “fix”)

| Item | Why not a gap |
|------|----------------|
| No Mangle Decl | Not a logic package |
| No prompt atoms | No LLM surface |
| Only one source file | Appropriate size |
| Uses third-party zstd | Already in `go.mod`; gzip default avoids hard dependency at call sites that only use `Write` |
| Order of `Read` output unstable | Documented; tests sort; SimpleColumn is set-oriented |

## 4. Risk if ignored

Leaving the package unwired forever creates:

1. **Maintenance drag** without product value.  
2. **Temptation to delete** despite good tests (wiring-audit anti-pattern).  
3. **Parallel reinvention** — other subsystems keep writing JSON fact dumps.

## 5. Recommended decision

**Keep + wire deliberately.** Prefer one high-value caller (campaign or world) over expanding the codec surface.
