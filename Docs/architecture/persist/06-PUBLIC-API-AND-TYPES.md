# 06 — Public API and Types: persist

> Last verified against codebase: **2026-08-15**  
> Imports: `codenerd/internal/persist/factsnap` (codec), `codenerd/internal/persist/snapshot` (workspace store)

Call `snapshot` for anything that lives in a workspace. Call `factsnap`
directly only for files that belong outside `.nerd/snapshots/`.

---

# Part A — `factsnap`

## A1. Constants

| Name | Value | File |
|------|-------|------|
| `ExtGzip` | `".sc.gz"` | `factsnap.go:50` |
| `ExtZstd` | `".sc.zst"` | `factsnap.go:53` |
| `ExtJSON` | `".json"` | `factsnap.go:56` |
| `ExtSHA256` | `".sha256"` | `factsnap.go:61` |

## A2. Types and sentinels

### `type Codec int`

| Value | Meaning on write |
|-------|------------------|
| `CodecAuto` (0) | Treated as gzip |
| `CodecGzip` (1) | gzip wrap |
| `CodecZstd` (2) | zstd wrap |
| other | write returns `factsnap: unknown codec %d` |

### `type Options struct { Codec Codec; NoSidecar bool }`

Zero value = gzip with an integrity sidecar. `NoSidecar` also deletes any stale
sidecar at the destination, because an old digest would fail every later read.

### `var ErrIntegrity error`

Returned (wrapped) by `Read`/`Verify` when the bytes disagree with the sidecar.
Match with `errors.Is`.

## A3. Functions

| Signature | Location | Notes |
|-----------|----------|-------|
| `Write(path string, facts []types.Fact) error` | `factsnap.go:100` | gzip + sidecar |
| `WriteCodec(path string, facts []types.Fact, codec Codec) error` | `factsnap.go:106` | codec + sidecar |
| `WriteOptions(path string, facts []types.Fact, opts Options) error` | `factsnap.go:112` | full control |
| `WritePath(path string, facts []types.Fact, opts Options) (string, error)` | `factsnap.go:120` | returns the file actually written; use this when reporting a location |
| `Read(path string) ([]types.Fact, error)` | `factsnap.go:299` | verifies sidecar, then suffix + magic-byte detection |
| `Verify(path string) error` | `factsnap.go:341` | nil when no sidecar exists |
| `HasSidecar(path string) bool` | `factsnap.go:350` | |
| `CodecName(c Codec) string` | `factsnap.go:281` | `"gzip"`, `"zstd"`, `"auto"`, `"json"` |
| `LegacyJSON(path string) ([]types.Fact, error)` | `factsnap.go:410` | migration helper, ignores suffix |
| `CanonicalPath(path string, codec Codec) string` | `factsnap.go:424` | pure path transform |

Empty `facts` is allowed and round-trips to an empty slice.

## A4. Durability contract

Every write goes through `writeFileAtomic`: a uniquely named temp file in the
destination directory, `Sync`, `Rename`, then an fsync of the directory. The
temp name is unique per call — a shared `<path>.tmp` lets two writers interleave
into one file and rename the mixture over a good snapshot. Same-path writers are
additionally serialised in-process (`lockPath`) so the data file and its sidecar
always describe the same bytes.

## A5. Fact value contract

From `codenerd/internal/types`: `Fact{Predicate string; Args []any}` and
`MangleAtom string`. `Fact.ToAtom()` is the write-side encoder.

On read:

| Written | Read back | Stable from |
|---------|-----------|-------------|
| `MangleAtom("/x")`, `"/x"` (name-shaped) | `MangleAtom("/x")` | hop 1 |
| `string` | `string` (unless name-shaped — then `MangleAtom` on hop 2) | hop 2 |
| `int`, `int64` | `int64` | hop 1 |
| fractional `float64` | `float64` | hop 1 |
| whole `float64` (2.0, 0.0) | `int64` — see OPEN-QUESTIONS Q8 | hop 1 |
| `bool` | `MangleAtom("/true")` / `MangleAtom("/false")` | hop 1 |
| `time.Time`, `time.Duration` | integer constants | hop 1 |

Callers must not assume Go `bool` or `time.Time` reappear as those types.

## A6. Error strings (stable greps)

| Prefix pattern | When |
|----------------|------|
| `factsnap: mkdir:` | parent dir create failed |
| `factsnap: fact N (pred) to atom:` | `ToAtom` failed |
| `factsnap: simplecolumn write:` | SC encode failed |
| `factsnap: create temp in …` | temp file create |
| `factsnap: gzip write/close`, `factsnap: zstd writer/write/close` | codec path |
| `factsnap: unknown codec` | bad enum |
| `factsnap: sync/close/rename` | publish path |
| `factsnap: integrity check failed` | sidecar mismatch (`ErrIntegrity`) |
| `factsnap: read/gzip decode/zstd decode/legacy json decode` | read path |
| `factsnap: nil store`, `factsnap: collect facts for` | defensive / predicate walk |

## A7. Non-exported API

| Func | Role |
|------|------|
| `writeSnapshot` | write pipeline |
| `writeFileAtomic`, `lockPath` | durability |
| `ensureExt`, `detectCodec` | path ↔ codec |
| `resolveReadCodec`, `sniffCodec` | magic-byte detection |
| `verifyChecksum` | sidecar comparison |
| `collectFacts`, `atomToFact`, `baseTermToValue` | store → facts |

---

# Part B — `snapshot`

## B1. Constants and types

| Name | Location | Notes |
|------|----------|-------|
| `DirName = ".nerd/snapshots"` | `snapshot.go:31` | documentation constant; use `Dir()` to build paths |
| `type Entry` | `snapshot.go:42` | `Name, Path, Codec, Bytes, ModTime, Verifiable` |
| `type PredicateCount` | `snapshot.go:217` | `Predicate, Count` |

## B2. Functions

| Signature | Location | Notes |
|-----------|----------|-------|
| `Dir(root string) string` | `snapshot.go:34` | `<root>/.nerd/snapshots` |
| `DefaultName(prefix string) string` | `snapshot.go:59` | `prefix-YYYYMMDD-HHMMSS` |
| `SanitizeName(name string) (string, error)` | `snapshot.go:70` | containment boundary; rejects separators, dot-leading, traversal, exotic characters; strips a redundant codec suffix |
| `Export(root, name string, facts []types.Fact, codec factsnap.Codec) (string, error)` | `snapshot.go:102` | returns the written path |
| `Resolve(root, ref string) (string, error)` | `snapshot.go:114` | bare name, filename or path |
| `Import(root, ref string) ([]types.Fact, string, error)` | `snapshot.go:146` | facts + resolved path |
| `List(root string) ([]Entry, error)` | `snapshot.go:160` | newest first; missing dir is not an error |
| `Summarize(facts []types.Fact) []PredicateCount` | `snapshot.go:225` | count desc, then name |
| `CodecFor(name string) (factsnap.Codec, error)` | `snapshot.go:244` | `gzip`/`gz`/`auto`/`""` → gzip; `zstd`/`zst` → zstd |

## B3. Recommended call patterns

**Export from a workspace:**

```go
path, err := snapshot.Export(root, snapshot.DefaultName("kernel"), facts, factsnap.CodecGzip)
```

**Import by bare name:**

```go
facts, path, err := snapshot.Import(root, "kernel-20260815-140501")
```

**Export outside the workspace (bug report attachment):**

```go
path, err := factsnap.WritePath("/tmp/report", facts, factsnap.Options{Codec: factsnap.CodecZstd})
```

**Migrate JSON:**

```go
facts, err := factsnap.LegacyJSON(oldPath)
if err == nil {
    _, err = factsnap.WritePath(newBase, facts, factsnap.Options{})
}
```
