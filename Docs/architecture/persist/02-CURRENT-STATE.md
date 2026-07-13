# 02 — Current State: persist

> Last verified against codebase: **2026-07-13**

## 1. Package layout (complete)

```
internal/persist/                 # directory only — no Go files at root
  factsnap/
    factsnap.go                   # 287 lines — all production logic
    factsnap_test.go              # 243 lines
    codec_parity_test.go          # 120 lines
    factsnap_codec_test.go        # 54 lines
    legacy_test.go                # 41 lines
```

| Metric | Count |
|--------|------:|
| Non-test `.go` | 1 |
| Test `.go` | 4 |
| Mangle `.mg` | 0 |
| YAML/config | 0 |
| README in package | 0 |

## 2. File roles

| File | Role | Hotspots |
|------|------|----------|
| `factsnap.go` | API + codec pipeline | `WriteCodec` (L67–148), `Read` (L153–180), `collectFacts` / `atomToFact` (L234–287) |
| `factsnap_test.go` | Behavioral core | sampleFacts, equalishFacts, size comparison |
| `codec_parity_test.go` | Cross-codec contract | magic bytes, 10k scale |
| `factsnap_codec_test.go` | Edge codecs | Auto, unknown cleanup |
| `legacy_test.go` | Migration helper | LegacyJSON errors |

## 3. Exported inventory (exhaustive)

Constants: `ExtGzip`, `ExtZstd`, `ExtJSON`, `CodecAuto`, `CodecGzip`, `CodecZstd`.

Types: `Codec`.

Funcs: `Write`, `WriteCodec`, `Read`, `LegacyJSON`, `CanonicalPath`.

Nothing else is exported.

## 4. Behavioral inventory

| Behavior | Present? | Notes |
|----------|----------|-------|
| Gzip write | Yes | Default |
| Zstd write | Yes | klauspost |
| Auto codec on write | Yes | → gzip |
| Suffix detect on read | Yes | No magic sniff |
| Legacy JSON read | Yes | Via `Read` fallback + `LegacyJSON` |
| Atomic publish | Yes | tmp/sync/rename |
| Dir create | Yes | `MkdirAll` 0o755 |
| Deterministic columns | Yes | flag on SimpleColumn |
| Concurrent multi-writer lock | No | caller must serialize |
| Progress / logging | No | — |
| Checksums | No | compression framing only |

## 5. Dependency inventory

**Direct:**

- `codenerd/internal/types`
- `codeberg.org/TauCeti/mangle-go/ast`
- `codeberg.org/TauCeti/mangle-go/factstore`
- `github.com/klauspost/compress/zstd`
- stdlib: bytes, compress/gzip, encoding/json, errors, fmt, io, os, path/filepath, strings

**Reverse (production):** none.

## 6. Hotspots / complexity

For a 287-line file, complexity concentrates in:

1. **WriteCodec switch** — two compression stacks + shared tmp lifecycle.  
2. **atomToFact / baseTermToValue** — type mapping that must stay symmetric with `ToAtom`.  
3. **ensureExt** — path rewriting edge cases (covered by table tests).

No concurrency primitives, no interfaces, no registration hooks.

## 7. Historical thin-corpus note

Earlier auto-inventory stubs described this package generically as “persistence helpers bridging stores and runtime.” That phrasing is **too broad**: the code only does **fact snapshot files**, not general store bridging. This rebuild replaces that vagueness.
