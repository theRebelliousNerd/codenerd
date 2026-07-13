# persist — Implemented Spec (Deep-Dive)

> Last verified against codebase: **2026-07-13**  
> Status: Living Reference Document  
> Language: Go  
> Module: `codenerd`  
> Primary source: `internal/persist/factsnap/`  
> Scale: **1** non-test Go file (**287** lines); **4** test files (**≈458** lines); **0** `.mg`

---

## 1. Overview

`internal/persist` hosts **fact snapshot I/O** for codeNERD. The only implemented subpackage is **`factsnap`**: a library that takes a slice of `types.Fact` (predicate + args) and writes a **deterministic Mangle SimpleColumn** blob, wrapped in **gzip** (default) or **zstd**, then renames into place atomically.

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
| Atomicity | write `path + ".tmp"` → `Sync` → `Close` → `Rename` |
| Production importers | **None** (as of 2026-07-13) |
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
    │              └───────────► path.tmp ─────────┘
    │                              │
    │                         fsync + close
    │                              │
    │                         rename → path.sc.gz | path.sc.zst
    ▼
Read(path) ← suffix detect → gunzip/zstd/json → collectFacts → []types.Fact
```

---

## 2. Implementation status

| Component | Status | Notes |
|-----------|--------|-------|
| `Write` / `WriteCodec` | **Implemented** | gzip default; auto→gzip; zstd optional |
| `Read` + suffix detect | **Implemented** | `.sc.gz`, `.sc.zst`, else JSON |
| `LegacyJSON` | **Implemented** | force JSON regardless of extension |
| `CanonicalPath` / `ensureExt` | **Implemented** | strips bare `.json`, appends codec suffix |
| Atomic tmp + rename | **Implemented** | cleanup on failure via deferred remove |
| Round-trip tests (gzip/zstd) | **Implemented** | 1k facts; equalish compare |
| Codec parity (100/1k/10k) | **Implemented** | byte-different, semantic-identical |
| Size vs JSON regression | **Implemented** | fails if gzip ≥ JSON size |
| Parent `persist` façade | **Absent** | directory only; import path is `.../persist/factsnap` |
| Kernel / campaign / CLI wiring | **Not wired** | zero reverse imports |
| Streaming / incremental append | **Not implemented** | whole-slice rewrite only |
| Checksum / version header beyond SimpleColumn | **Not implemented** | relies on SimpleColumn + compression framing |
| `internal/logging` integration | **Not implemented** | errors only via `fmt.Errorf` wrappers |

**Overall:** library **complete and tested**; product integration **0%**. Heuristic package health as a *utility*: **high**. Heuristic *platform integration*: **dormant**.

---

## 3. Source inventory

### 3.1 Tree

```
internal/persist/
  factsnap/
    factsnap.go              # all production code (287 lines)
    factsnap_test.go         # round-trip gzip/zstd, legacy JSON Read, size comparison
    factsnap_codec_test.go   # CodecAuto, unknown codec cleanup
    codec_parity_test.go     # cross-codec parity + path rewriting table
    legacy_test.go           # LegacyJSON helper + error paths
```

There is **no** `persist.go` at `internal/persist/` root. Callers must import the subpackage.

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
| `Read` | func | `factsnap.go:153` | load + auto codec |
| `LegacyJSON` | func | `factsnap.go:184` | force JSON decode |
| `CanonicalPath` | func | `factsnap.go:198` | path normalization |

Unexported helpers: `ensureExt`, `detectCodec`, `collectFacts`, `atomToFact`, `baseTermToValue`.

### 3.3 Line budget

| Path | ≈Lines | Role |
|------|-------:|------|
| `internal/persist/factsnap/factsnap.go` | 287 | sole source |
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
| Temp file | `path + ".tmp"` |
| Gzip branch | `gzip.NewWriter` → `io.Copy` → `Close` |
| Zstd branch | `zstd.NewWriter` → `io.Copy` → `Close` |
| Unknown codec | error `factsnap: unknown codec %d`; no final file |
| Durability | `f.Sync()` then `f.Close()` then `os.Rename(tmp, path)` |
| Failure cleanup | deferred `os.Remove(tmp)` while `cleanupTmp == true`; cleared only after successful rename |

Windows rename-over-existing semantics depend on OS; callers should treat “same final path” as “replace.”

---

## 5. Deep dive — Read path

### 5.1 Detection

`detectCodec(path)` is **suffix-only** (not content sniffing):

| Suffix | Codec |
|--------|-------|
| `.sc.gz` | `CodecGzip` |
| `.sc.zst` | `CodecZstd` |
| anything else | treated as legacy JSON (`-1` sentinel) |

Implication: a gzip file named `facts.bin` will be JSON-decoded and fail. Callers must use canonical extensions (or `CanonicalPath`).

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
| `cmd/nerd` | **none** |
| `internal/core` | **none** |
| `internal/campaign` | **none** (uses JSON/JSONL for assault artifacts) |
| `internal/store` | **none** (sqlite cold path) |
| `internal/world` | **none** |
| other `internal/*` | **none** |

### 7.3 Intended (not claimed as present) fact-flow placement

```
kernel / world / campaign  ──export──►  []types.Fact
                                              │
                                              ▼
                                       factsnap.Write
                                              │
                                     .nerd/.../*.sc.gz
                                              │
                                       factsnap.Read
                                              │
                                              ▼
                               assert / rehydrate (caller + policy)
```

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

Verify: `go test ./internal/persist/...`

---

## 9. Gaps pointer

See [03-GAP-ANALYSIS.md](03-GAP-ANALYSIS.md) and [TODO.md](TODO.md). Headline gaps:

1. **Zero production wiring** — largest product gap.  
2. **No content-type sniffing** — wrong extension → wrong decoder.  
3. **No logging / metrics** — silent utility.  
4. **No streaming / append** — full rewrite only.  
5. **atomToFact drift risk** vs `internal/core/kernel_query.go`.  
6. **No package-level façade** — import path longer; no re-export root.

Non-gaps: codec quality, atomic writes, compression win, and test density are already strong for the library’s size.

---

## 10. Non-goals of this corpus

- Documenting `internal/store` cold storage (separate corpus).  
- Implementing CLI snapshot commands (code change; docs-only task).  
- Spec-doc-sprint product templates under `Docs/Spec/`.  
- Vectryx product vocabulary.

---

## 11. Maintainers’ quick reference

```go
import "codenerd/internal/persist/factsnap"

// Write default gzip
_ = factsnap.Write(".nerd/snapshots/world", facts)

// Explicit zstd
path := factsnap.CanonicalPath(".nerd/snapshots/world", factsnap.CodecZstd)
_ = factsnap.WriteCodec(path, facts, factsnap.CodecZstd)

// Read (auto by suffix)
facts, err := factsnap.Read(path)

// Migrate old JSON dumps
facts, err = factsnap.LegacyJSON("old_dump.json")
```

---

## 12. Status table (living)

| Area | Score | Comment |
|------|------:|---------|
| Codec correctness | 5/5 | Parity tests at three sizes |
| Atomic file protocol | 5/5 | tmp + sync + rename + cleanup |
| API clarity | 4/5 | Small surface; missing package doc at persist root |
| Integration | 1/5 | Unwired |
| Observability | 1/5 | Errors only |
| Safety/policy | N/A | Pure serializer; policy is caller’s job |
| Test grounding | 5/5 | Tests outnumber source lines |

**Verdict:** Keep, wire deliberately, do not delete.
