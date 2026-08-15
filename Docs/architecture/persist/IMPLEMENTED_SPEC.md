# persist — Implemented Spec (Deep-Dive)

> Last verified against codebase: **2026-08-15**  
> Status: Living Reference Document  
> Language: Go  
> Module: `codenerd`  
> Primary source: `internal/persist/` (`factsnap/`, `snapshot/`, `doc.go`)  
> Scale: **3** non-test Go files (**≈808** lines); **7** test files (**≈1,050** lines); **0** `.mg`  
> First production caller: `cmd/nerd/cmd_snapshot.go` (`nerd snapshot export|import|list`)

---

## 1. Overview

`internal/persist` hosts **fact snapshot I/O** for codeNERD, split across two subpackages:

- **`factsnap`** — the codec. Takes a slice of `types.Fact` (predicate + args) and writes a **deterministic Mangle SimpleColumn** blob, wrapped in **gzip** (default) or **zstd**, renamed into place atomically with a `.sha256` integrity sidecar.
- **`snapshot`** — the workspace store. Owns `.nerd/snapshots/`, name sanitisation, listing and bare-name resolution, so no call site invents its own paths.

`internal/persist/doc.go` is the umbrella package doc; it holds no code.

### Why it exists

JSON snapshots of fact graphs waste space: predicate names and shared atom prefixes repeat on every row. Mangle’s SimpleColumn format stores:

1. A **header** of predicate symbols, arities, and fact counts.
2. **One column per argument position**, so similar values sit adjacent and compress extremely well.

Package comment (`factsnap.go` L1–17) states the design thesis explicitly: SimpleColumn + percent-escaped constants stay text-friendly while remaining dense; gzip needs no extra runtime deps beyond stdlib; zstd (via `github.com/klauspost/compress/zstd`, already in `go.mod`) trades a dependency for smaller files.

### Key characteristics

| Property | Value |
|----------|-------|
| Package path | `codenerd/internal/persist/factsnap` |
| Go package name | `factsnap` (no root `package persist`) |
| Canonical extensions | `.sc.gz` (gzip), `.sc.zst` (zstd), legacy `.json` on read |
| Default write codec | gzip (`Write` → `WriteCodec(..., CodecGzip)`) |
| Determinism | `factstore.SimpleColumn{Deterministic: true}` |
| Atomicity | unique temp in the destination dir → `Sync` → `Close` → `chmod 0644` → `Rename` → dir fsync |
| Integrity | `<file>.sha256`, sha256sum(1) format, verified on read (`ErrIntegrity`) |
| Detection on read | sidecar verify → magic bytes (gzip `1f 8b`, zstd `28 b5 2f fd`) → suffix → legacy JSON |
| Observability | `logging.CategoryStore` debug on write/read; warn on sidecar failure and codec disagreement |
| Production importers | `cmd/nerd/cmd_snapshot.go` (via `internal/persist/snapshot`) |
| Mangle / policy | None local — pure library |
| Prompt / JIT | N/A |

### High-level flow

```
[]types.Fact
    │
    ├─ ToAtom() per fact ──► factstore.SimpleInMemoryStore
    │                              │
    │                              ▼
    │                    SimpleColumn.WriteTo (deterministic)
    │                              │
    │                              ▼
    │                    raw SimpleColumn bytes
    │                              │
    │              ┌───────────────┴───────────────┐
    │              ▼                               ▼
    │         gzip.Writer                     zstd.Writer
    │              │                               │
    │              └──────► temp (unique) ────────┘
    │                        │        │
    │                        │        └──► sha256 (teed from the same bytes)
    │                   fsync + close             │
    │                        │                    ▼
    │              chmod + rename ──► path.sc.gz  path.sc.gz.sha256
    │                        │
    │                   dir fsync
    ▼
Read(path) ← verify sidecar → sniff magic / suffix → gunzip/zstd/json
                                        → collectFacts → []types.Fact
```

---

## 2. Implementation status

| Component | Status | Notes |
|-----------|--------|-------|
| `Write` / `WriteCodec` / `WriteOptions` / `WritePath` | **Implemented** | gzip default; auto→gzip; zstd optional; `WritePath` returns the file written |
| `Read` + detection | **Implemented** | magic-byte sniff first, suffix fallback, else JSON |
| Integrity sidecar + `Verify` / `HasSidecar` | **Implemented** | written by default; `Options.NoSidecar` opts out and clears stale digests |
| Same-path write serialisation | **Implemented** | `lockPath` keyed mutex; data and digest always agree |
| `LegacyJSON` | **Implemented** | force JSON regardless of extension |
| `CanonicalPath` / `ensureExt` | **Implemented** | strips bare `.json`, appends codec suffix |
| Atomic tmp + rename | **Implemented** | cleanup on failure via deferred remove |
| Round-trip tests (gzip/zstd) | **Implemented** | 1k facts; equalish compare |
| Codec parity (100/1k/10k) | **Implemented** | byte-different, semantic-identical |
| Size vs JSON regression | **Implemented** | fails if gzip ≥ JSON size |
| Workspace store (`snapshot`) | **Implemented** | `Dir`, `Export`, `Import`, `Resolve`, `List`, `Summarize`, `SanitizeName`, `CodecFor` |
| Parent `persist` doc | **Implemented** | `internal/persist/doc.go`; import paths remain per-subpackage |
| CLI wiring | **Implemented** | `nerd snapshot export | import | list` |
| Kernel debug export | **Implemented** | EDB by default; `--predicate`, `--derived` |
| Campaign / world wiring | **Not wired** | still candidates |
| Streaming / incremental append | **Not implemented** | whole-slice rewrite only |
| Version envelope beyond SimpleColumn | **Not implemented** | see OPEN-QUESTIONS Q6 |
| `internal/logging` integration | **Implemented** | `CategoryStore`; a dedicated `CategoryPersist` is still the right home |

**Overall:** library complete and tested; product integration **live but narrow** — one caller (the CLI), no automatic call sites.

---

## 3. Source inventory

### 3.1 Tree

```
internal/persist/
  doc.go                     # umbrella package doc (18 lines, no code)
  factsnap/
    factsnap.go              # codec + durability (537 lines)
    factsnap_test.go         # round-trip gzip/zstd, legacy JSON Read, size comparison
    factsnap_codec_test.go   # CodecAuto, unknown codec cleanup
    codec_parity_test.go     # cross-codec parity + path rewriting table
    legacy_test.go           # LegacyJSON helper + error paths
    factsnap_robustness_test.go  # sniffing, sidecar, contention, empty/bool/float/name hops
  snapshot/
    snapshot.go              # workspace layout + naming + listing (253 lines)
    snapshot_test.go         # paths, containment, listing, summary
    kernel_roundtrip_test.go # package snapshot_test: domain -> kernel -> snapshot -> kernel

cmd/nerd/
  cmd_snapshot.go            # the production caller (303 lines)
  cmd_snapshot_test.go       # command behaviour
```

Callers import the subpackages; `internal/persist` itself exports nothing.

### 3.2 Production surface (complete)

| Symbol | Kind | Location | Role |
|--------|------|----------|------|
| `ExtGzip` | const | `factsnap.go:39` | `".sc.gz"` |
| `ExtZstd` | const | `factsnap.go:42` | `".sc.zst"` |
| `ExtJSON` | const | `factsnap.go:45` | `".json"` |
| `Codec` | type | `factsnap.go:48` | `int` enum |
| `CodecAuto` | const | `factsnap.go:52` | → gzip on write |
| `CodecGzip` | const | `factsnap.go:54` | gzip wrap |
| `CodecZstd` | const | `factsnap.go:56` | zstd wrap |
| `Write` | func | `factsnap.go:61` | `WriteCodec(..., CodecGzip)` |
| `WriteCodec` | func | `factsnap.go:67` | full write pipeline |
| `Read` | func | `factsnap.go:299` | verify + detect + load |
| `LegacyJSON` | func | `factsnap.go:410` | force JSON decode |
| `CanonicalPath` | func | `factsnap.go:424` | path normalization |
| `ExtSHA256` | const | `factsnap.go:61` | `".sha256"` |
| `ErrIntegrity` | var | `factsnap.go:73` | sidecar mismatch sentinel |
| `Options` | type | `factsnap.go:77` | `Codec`, `NoSidecar` |
| `WriteOptions` / `WritePath` | func | `factsnap.go:112` / `:120` | full-control writes |
| `Verify` / `HasSidecar` | func | `factsnap.go:341` / `:350` | integrity surface |
| `CodecName` | func | `factsnap.go:281` | codec → operator string |

Unexported helpers: `writeSnapshot`, `writeFileAtomic`, `lockPath`, `ensureExt`, `detectCodec`, `resolveReadCodec`, `sniffCodec`, `verifyChecksum`, `collectFacts`, `atomToFact`, `baseTermToValue`.

Full `snapshot` surface: see [06-PUBLIC-API-AND-TYPES.md](06-PUBLIC-API-AND-TYPES.md) Part B.

### 3.3 Line budget

| Path | ≈Lines | Role |
|------|-------:|------|
| `internal/persist/factsnap/factsnap.go` | 537 | codec, durability, integrity |
| `internal/persist/snapshot/snapshot.go` | 253 | workspace layout |
| `cmd/nerd/cmd_snapshot.go` | 303 | CLI caller |
| `internal/persist/factsnap/factsnap_robustness_test.go` | 316 | hardening tests |
| `internal/persist/snapshot/snapshot_test.go` | 188 | store tests |
| `internal/persist/snapshot/kernel_roundtrip_test.go` | 169 | kernel integration |
| `cmd/nerd/cmd_snapshot_test.go` | 202 | command tests |
| `internal/persist/factsnap/factsnap_test.go` | 243 | main round-trip + size |
| `internal/persist/factsnap/codec_parity_test.go` | 120 | parity + paths |
| `internal/persist/factsnap/factsnap_codec_test.go` | 54 | auto / unknown |
| `internal/persist/factsnap/legacy_test.go` | 41 | LegacyJSON |

---

## 4. Deep dive — Write path

### 4.1 Codec selection

```go
// Write always gzip
func Write(path string, facts []types.Fact) error {
    return WriteCodec(path, facts, CodecGzip)
}
```

`WriteCodec`:

1. Maps `CodecAuto` → `CodecGzip`.
2. Calls `ensureExt(path, codec)` so logical names like `"snap"` become `"snap.sc.gz"` or `"snap.sc.zst"`. Bare `.json` suffix is stripped then replaced with the codec extension (migration-friendly).
3. `os.MkdirAll(filepath.Dir(path), 0o755)`.

### 4.2 Fact → store

Each `types.Fact` is converted with `f.ToAtom()` (`internal/types/types.go`). Failures wrap as:

```text
factsnap: fact %d (%s) to atom: %w
```

Atoms land in `factstore.NewSimpleInMemoryStore()`. This is an **in-memory intermediate**, not the kernel store — factsnap never touches `internal/core`.

### 4.3 SimpleColumn encode

```go
sc := factstore.SimpleColumn{Deterministic: true}
sc.WriteTo(store, &raw)
```

`Deterministic: true` is load-bearing for reproducible snapshots and for tests that compare codec containers (bytes differ by compression, but encode order is stable).

### 4.4 Compress + atomic publish

| Step | Behavior |
|------|----------|
| Same-path lock | `lockPath(path)` — keyed mutex, so data and sidecar always describe the same write |
| Temp file | `os.CreateTemp(dir, "."+base+".tmp*")` — unique per call. A shared `<path>.tmp` let two writers interleave into one file and rename the mixture over a good snapshot |
| Gzip branch | `gzip.NewWriter` → `io.Copy` → `Close`, teed into a `sha256` hasher |
| Zstd branch | `zstd.NewWriter` → `io.Copy` → `Close`, teed the same way |
| Unknown codec | error `factsnap: unknown codec %d`; rejected before any file is touched |
| Durability | `Sync` → `Close` → `chmod 0644` → `Rename` → fsync of the destination directory |
| Sidecar | `<path>.sha256` written through the same atomic helper; a failure warns but does not fail the export, since the snapshot is already durable |
| Failure cleanup | deferred close + remove of the temp unless the rename committed |

Windows rename-over-existing semantics depend on OS; callers should treat “same final path” as “replace.”

---

## 5. Deep dive — Read path

### 5.1 Detection

`Read` verifies the sidecar first (when one exists), then picks a decoder via
`resolveReadCodec`:

1. `sniffCodec(data)` — gzip `1f 8b`, zstd `28 b5 2f fd`.
2. `detectCodec(path)` — `.sc.gz`, `.sc.zst`, else legacy JSON (`-1` sentinel).

Content wins when the two disagree, because magic bytes cannot be wrong about
the container while a suffix survives a rename that changed nothing else. The
disagreement is logged at warn level. A gzip file named `facts.bin` now decodes;
before, it was JSON-decoded and failed.

Writers must still emit canonical suffixes: `snapshot.Resolve` finds files by
name and extension, not by scanning contents.

### 5.2 Decode backends

| Codec | API |
|-------|-----|
| Gzip | `factstore.NewSimpleColumnStoreFromGzipBytes(data)` |
| Zstd | `factstore.NewSimpleColumnStoreFromZstdBytes(data)` |
| Legacy | `json.Unmarshal` into `[]types.Fact` |

### 5.3 `collectFacts`

Walks `store.ListPredicates()`, runs `GetFacts` per predicate, converts each `ast.Atom` via local `atomToFact` → `[]types.Fact`.

Order of returned facts follows store iteration, **not** original write order. Tests sort with `sortFacts` before equality checks.

### 5.4 `LegacyJSON`

Ignores extension; always JSON-unmarshals. Migration helper for paths that still emit JSON and may not end in `.json` when passed to `Read`… actually `Read` only falls through to JSON when suffix is *not* `.sc.gz`/`.sc.zst`, so a path like `dump.dat` with JSON body works via `Read` *or* `LegacyJSON`. Prefer explicit `LegacyJSON` when the file is known-JSON with a nonstandard name.

---

## 6. Deep dive — Type round-trip semantics

### 6.1 Intentional `atomToFact` fork

Comment at `factsnap.go:251–255`:

> atomToFact mirrors the conversion in `internal/core/kernel_query.go` but is duplicated here to avoid an import cycle.

**Behavioral divergence (important):**

| Constant type | `core.baseTermToValue` | `factsnap.baseTermToValue` |
|---------------|------------------------|----------------------------|
| `NameType` | plain `string` (`t.Symbol`) | `types.MangleAtom(c.Symbol)` |
| `StringType` / `BytesType` | string | string |
| `NumberType` | `NumValue` (int64) | same |
| `Float64Type` | float64 | same |
| Variables / non-constants | N/A in snapshots | `fmt.Sprintf("%v", term)` |

factsnap **preserves slash names as `MangleAtom`** so that `Write → Read → Write` stays symmetric: `ToAtom()` encodes name constants the same way next round. Core’s query path flattens names to strings (historical kernel surface).

The divergence is deliberate and now documented at the point of divergence (the
doc comment on `factsnap.baseTermToValue`) plus pinned by
`TestNameConstant_WhenRoundTripped_ShouldStayMangleAtom`. It matters because
`ToAtom` re-promotes a plain `"/x"` string to a name only via
`isValidMangleNameConstant`, a heuristic that rejects anything with a file
extension or more than two slashes — so `"/a/b/c.go"` would silently degrade
from a name constant to a string constant on the second hop. Unification is a
consumer migration inside `internal/core`, not a file move; see OPEN-QUESTIONS
Q5.

Tests encode this with `normalizeArg` / `equalishFacts`:

- `int` ↔ `int64` collapse  
- string starting with `/` ↔ `MangleAtom`

Callers comparing factsnap output to `kernel.Query` results must account for this.

### 6.2 Unsupported / degraded argument shapes

`types.Fact.ToAtom()` supports bool, time, duration, floats, etc. Round-trip through SimpleColumn is only as rich as Mangle constants allow. Bools become name constants `/true`/`/false` on encode (via `ToAtom`); on read they become `MangleAtom`/`string` name symbols, not Go `bool`. Time/duration go through numeric encodings in `ToAtom` — treat multi-hop fidelity as **best-effort** for exotic types; tests cover the common code-index / campaign-shaped facts (`code_defines`, `code_calls`, `projected_fact`, `campaign_task`).

---

## 7. Integration map

### 7.1 Upstream (imports)

| Dependency | Use |
|------------|-----|
| `codenerd/internal/types` | `Fact`, `MangleAtom`, `ToAtom()` |
| `codeberg.org/TauCeti/mangle-go/ast` | `Atom`, `Query`, constants |
| `codeberg.org/TauCeti/mangle-go/factstore` | SimpleInMemoryStore, SimpleColumn, gzip/zstd store loaders |
| `github.com/klauspost/compress/zstd` | zstd writers |
| stdlib | `compress/gzip`, `encoding/json`, `os`, `path/filepath`, `io`, `bytes` |

### 7.2 Downstream (importers)

| Consumer | Status |
|----------|--------|
| `cmd/nerd/cmd_snapshot.go` | **wired** — `snapshot` + `factsnap` |
| `internal/persist/snapshot` | **wired** — `factsnap` |
| `internal/core` | none (and must stay that way: the CLI holds the core dependency) |
| `internal/campaign` | none (uses JSON/JSONL for assault artifacts) — candidate |
| `internal/store` | none (sqlite cold path) |
| `internal/world` | none — candidate for an index freeze |
| other `internal/*` | none |

### 7.3 Fact-flow placement (as built)

```
nerd snapshot export ──► core.NewRealKernelWithWorkspace (local, no LLM)
                             │
                             ├── GetBaseFacts()  (default: EDB only)
                             ├── Query(pred)     (--predicate)
                             └── QueryAll()      (--derived)
                                     │
                                     ▼
                          snapshot.Export ──► factsnap.WritePath
                                     │
                          .nerd/snapshots/*.sc.gz (+ .sha256)
                                     │
                          snapshot.Import ──► factsnap.Read
                                     │
              ┌──────────────────────┼──────────────────────┐
              ▼                      ▼                      ▼
        summary only          --to-mangle             --assert
        (default)          (reviewable Datalog)   (in-process kernel)
```

EDB is the default export because derived facts are conclusions the kernel
recomputes; re-importing them would assert conclusions as premises.

This package does **not** sit on the hot OODA path. It is an **offline / checkpoint / export** primitive.

---

## 8. Testing summary

| Test | File | Asserts |
|------|------|---------|
| `TestRoundTripGzip` | `factsnap_test.go` | 1000 facts write/read equalish; file at canonical path |
| `TestRoundTripZstd` | `factsnap_test.go` | same for zstd |
| `TestLegacyJSONFallback` | `factsnap_test.go` | `Read` on `.json` loads marshaled facts |
| `TestSizeComparison` | `factsnap_test.go` | gzip < JSON size; logs all three sizes |
| `TestWriteCodec_Auto` | `factsnap_codec_test.go` | Auto → `.sc.gz` |
| `TestWriteCodec_UnknownCodec` | `factsnap_codec_test.go` | error; no final or tmp left |
| `TestCodecParity` | `codec_parity_test.go` | 100/1k/10k: bytes differ, facts equal, magic headers |
| `TestCodecParity_PathRewriting` | `codec_parity_test.go` | table for CanonicalPath |
| `TestLegacyJSONDirect` | `legacy_test.go` | helper + missing + bad JSON |
| `TestRead_WhenSuffixStripped_*`, `TestRead_WhenSuffixContradictsContent_*` | `factsnap_robustness_test.go` | magic-byte detection |
| `TestWrite_WhenDefaultOptions_ShouldEmitSha256Sidecar`, `TestRead_WhenSnapshotTruncated_ShouldFailIntegrityCheck`, `TestWrite_WhenNoSidecarRequested_ShouldRemoveStaleSidecar` | same | integrity sidecar |
| `TestWrite_WhenWritersRaceOnOnePath_ShouldLeaveOneReadableSnapshot` | same | 8 concurrent writers |
| `TestWrite_WhenFactsEmpty_*`, `TestBool_*`, `TestFloat_*`, `TestNameConstant_*` | same | empty / bool / float / name multi-hop |
| `snapshot_test.go` suite | `internal/persist/snapshot` | paths, containment, listing, summary, codec aliases |
| `TestSnapshotRoundTrip_WhenReloadedIntoFreshKernel_ShouldRestoreSameFacts` | `kernel_roundtrip_test.go` | domain → kernel → snapshot → fresh kernel |
| `cmd_snapshot_test.go` suite | `cmd/nerd` | export/import/list/--assert/--to-mangle behaviour |

Full inventory: [10-TESTING-ALIGNMENT.md](10-TESTING-ALIGNMENT.md).

Verify: `go test ./internal/persist/... ./cmd/nerd/`

---

## 9. Gaps pointer

See [03-GAP-ANALYSIS.md](03-GAP-ANALYSIS.md) and [TODO.md](TODO.md). Headline gaps:

1. **Narrow wiring** — one caller (the CLI). Campaign and world remain unwired.
2. **No metrics / counters** — debug log lines only, no trends.
3. **No streaming / append** — full rewrite only; the whole snapshot is buffered.
4. **atomToFact divergence** vs `internal/core/kernel_query.go` — documented and
   pinned, not unified (OPEN-QUESTIONS Q5).
5. **Whole-valued floats degrade to integers** — upstream rendering (Q8).
6. **Cross-process write contention** — in-process locking only.

Closed since 2026-07-13: zero production wiring, no content sniffing, no
logging, no integrity check, no package-level doc.

Non-gaps: codec quality, atomic writes, compression win, and test density.

---

## 10. Non-goals of this corpus

- Documenting `internal/store` cold storage (separate corpus).  
- Implementing CLI snapshot commands (code change; docs-only task).  
- Spec-doc-sprint product templates under `Docs/Spec/`.  
- Vectryx product vocabulary.

---

## 11. Maintainers’ quick reference

```go
import (
    "codenerd/internal/persist/factsnap"
    "codenerd/internal/persist/snapshot"
)

// Workspace export (preferred): .nerd/snapshots/world.sc.gz + .sha256
path, err := snapshot.Export(root, "world", facts, factsnap.CodecGzip)

// Workspace import by bare name, sidecar verified inside
facts, path, err := snapshot.Import(root, "world")

// Outside the workspace (bug report attachment)
path, err = factsnap.WritePath("/tmp/dump", facts, factsnap.Options{Codec: factsnap.CodecZstd})

// Migrate old JSON dumps
facts, err = factsnap.LegacyJSON("old_dump.json")
```

```bash
nerd snapshot export                 # kernel-YYYYMMDD-HHMMSS, EDB, gzip
nerd snapshot export idx --codec zstd -p code_defines -p code_calls
nerd snapshot list
nerd snapshot import idx --show 20 --to-mangle /tmp/idx.mg
```

---

## 12. Status table (living)

| Area | Score | Comment |
|------|------:|---------|
| Codec correctness | 5/5 | Parity tests at three sizes; multi-hop type contracts pinned |
| Atomic file protocol | 5/5 | unique temp + sync + chmod + rename + dir fsync + per-path lock |
| Integrity | 4/5 | sha256 sidecar verified on read; no version envelope yet |
| API clarity | 5/5 | Two focused subpackages plus an umbrella doc |
| Integration | 3/5 | One deliberate caller; campaign/world still open |
| Observability | 3/5 | Write/read traced with size, codec, digest, duration; no counters |
| Safety/policy | N/A | Pure serializer; policy is the caller’s job. Name containment enforced in `snapshot` |
| Test grounding | 5/5 | Unit, integration and command-level tests |

**Verdict:** Wired, and the next callers have a paved path.
