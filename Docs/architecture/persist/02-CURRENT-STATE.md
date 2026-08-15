# 02 — Current State: persist

> Last verified against codebase: **2026-08-15**

## 1. Package layout (complete)

```
internal/persist/
  doc.go                          # 18 lines — umbrella package doc, no code
  factsnap/
    factsnap.go                   # 537 lines — codec, durability, integrity
    factsnap_test.go              # 243 lines
    factsnap_robustness_test.go   # 316 lines
    codec_parity_test.go          # 120 lines
    factsnap_codec_test.go        # 54 lines
    legacy_test.go                # 41 lines
  snapshot/
    snapshot.go                   # 253 lines — workspace layout + naming
    snapshot_test.go              # 188 lines
    kernel_roundtrip_test.go      # 169 lines (package snapshot_test)

cmd/nerd/
  cmd_snapshot.go                 # 303 lines — the production caller
  cmd_snapshot_test.go            # 202 lines
```

| Metric | Count |
|--------|------:|
| Non-test `.go` under `internal/persist` | 3 |
| Test `.go` under `internal/persist` | 6 |
| Mangle `.mg` | 0 |
| YAML/config | 0 |
| README in package | 0 (see `doc.go`) |

## 2. File roles

| File | Role | Hotspots |
|------|------|----------|
| `doc.go` | Umbrella package doc; explains the factsnap/snapshot split | — |
| `factsnap.go` | Codec + durability + integrity | `writeSnapshot` (L124–212), `writeFileAtomic` (L241–279), `Read` (L299–339), `resolveReadCodec`/`sniffCodec` (L382–408), `collectFacts`/`atomToFact`/`baseTermToValue` (L460–537) |
| `snapshot.go` | Workspace store | `SanitizeName` (L70–100), `Resolve` (L114–144), `List` (L160–215) |
| `cmd/nerd/cmd_snapshot.go` | Operator surface | `runSnapshotExport`, `collectSnapshotFacts`, `runSnapshotImport`, `writeFactsAsMangle` |
| `factsnap_test.go` | Behavioral core | sampleFacts, equalishFacts, size comparison |
| `codec_parity_test.go` | Cross-codec contract | magic bytes, 10k scale |
| `factsnap_codec_test.go` | Edge codecs | Auto, unknown cleanup |
| `legacy_test.go` | Migration helper | LegacyJSON errors |
| `factsnap_robustness_test.go` | Hardening | sniffing, sidecar, contention, empty/bool/float/name hops |
| `snapshot_test.go` | Workspace store | containment, listing, resolution |
| `kernel_roundtrip_test.go` | Integration | real kernel on both ends |

## 3. Exported inventory (exhaustive)

`factsnap` constants: `ExtGzip`, `ExtZstd`, `ExtJSON`, `ExtSHA256`, `CodecAuto`, `CodecGzip`, `CodecZstd`.

`factsnap` types and vars: `Codec`, `Options`, `ErrIntegrity`.

`factsnap` funcs: `Write`, `WriteCodec`, `WriteOptions`, `WritePath`, `Read`, `Verify`, `HasSidecar`, `CodecName`, `LegacyJSON`, `CanonicalPath`.

`snapshot` constants and types: `DirName`, `Entry`, `PredicateCount`.

`snapshot` funcs: `Dir`, `DefaultName`, `SanitizeName`, `Export`, `Resolve`, `Import`, `List`, `Summarize`, `CodecFor`.

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
