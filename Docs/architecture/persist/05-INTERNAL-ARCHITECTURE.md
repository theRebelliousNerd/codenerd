# 05 — Internal Architecture: persist / factsnap

> Last verified against codebase: **2026-08-15**  
> Source of truth: `internal/persist/factsnap/factsnap.go`, `internal/persist/snapshot/snapshot.go`

## 1. Component diagram

```
                    ┌──────────────────────────┐
                    │ cmd/nerd/cmd_snapshot.go │
                    └────────────┬─────────────┘
                                 │ Export / Import / List
                                 ▼
                    ┌──────────────────────────┐
                    │   snapshot (workspace)   │
                    │ Dir / SanitizeName /     │
                    │ Resolve / List           │
                    └────────────┬─────────────┘
                                 │
       WritePath / WriteOptions  │ Read / Verify / LegacyJSON
                                 ▼
                    ┌──────────────────────────┐
                    │     factsnap public API  │
                    │ ensureExt / detectCodec  │
                    │ sniffCodec / verifyChecksum │
                    └────────────┬─────────────┘
           ┌─────────────────────┼─────────────────────┐
           ▼                     ▼                     ▼
   types.Fact.ToAtom     factstore SimpleColumn   encoding/json
           │                     │                     │
           ▼                     ▼                     │
   SimpleInMemoryStore    gzip / zstd frames           │
           │                     │                     │
           └──────► unique temp ──│                     │
                        + sha256 tee ▼                     │
                            path.sc.* ◄────────────────┘
                            path.sc.*.sha256
                                 │
                                 ▼
                         collectFacts
                         atomToFact
                                 │
                                 ▼
                           []types.Fact
```

There is no long-lived process, no goroutine pool, no registry. Every call is a **pure request/response** against the filesystem.

## 2. Write state machine

```
START
  │
  ▼
normalize codec (Auto→Gzip); reject unknown before touching disk
  │
  ▼
ensureExt(path, codec)
  │
  ▼
lockPath(path)   (serialises same-path writers so data and digest agree)
  │
  ▼
MkdirAll(dir)
  │
  ▼
for each fact: ToAtom → store.Add
  │ fail ──► ERROR (no file)
  ▼
SimpleColumn.WriteTo(buffer)
  │ fail ──► ERROR
  ▼
CreateTemp(dir, "."+base+".tmp*")
  │ fail ──► ERROR
  ▼
compress into temp, teed into sha256
  │ fail ──► remove temp ──► ERROR
  ▼
Sync + Close + Chmod 0644
  │ fail ──► remove temp ──► ERROR
  ▼
Rename(temp → path) + fsync(dir)
  │ fail ──► remove temp ──► ERROR
  ▼
write <path>.sha256 through the same helper
  │ fail ──► warn only (snapshot already durable)
  ▼
SUCCESS
```

## 3. Read state machine

```
START
  │
  ▼
os.ReadFile(path)
  │ fail ──► ERROR
  ▼
verifyChecksum(path, data)      (skipped when no sidecar exists)
  │ mismatch ──► ErrIntegrity
  ▼
resolveReadCodec = sniffCodec(data) else detectCodec(suffix)
  │
  ├─ gzip → NewSimpleColumnStoreFromGzipBytes → collectFacts
  ├─ zstd → NewSimpleColumnStoreFromZstdBytes → collectFacts
  └─ else → json.Unmarshal([]types.Fact)
```

## 4. Key types (runtime)

| Type | Ownership | Lifetime |
|------|-----------|----------|
| `[]types.Fact` | caller | request |
| `factstore.SimpleInMemoryStore` | write path | ephemeral |
| `bytes.Buffer` raw SC | write path | ephemeral |
| `*factstore.SimpleColumnStore` | read path | ephemeral until collectFacts finishes |
| `Codec` | caller / detect | value enum |

## 5. Path rewriting rules (`ensureExt`)

| Input | Codec | Output |
|-------|-------|--------|
| `snap` | Gzip | `snap.sc.gz` |
| `snap` | Zstd | `snap.sc.zst` |
| `snap.sc.gz` | Gzip | unchanged |
| `snap.sc.zst` | Zstd | unchanged |
| `snap.json` | Gzip | `snap.sc.gz` |
| `snap.json` | Zstd | `snap.sc.zst` |
| `snap` | Auto (after map) | `snap.sc.gz` |

`CanonicalPath` is a pure wrapper around `ensureExt`.

## 6. Data layout on disk (conceptual)

```
[ gzip or zstd frame ]
   └── SimpleColumn payload
         ├── predicate dictionary (symbol, arity, count)
         └── columnar argument streams (percent-escaped constants)
```

Exact binary layout is owned by `mangle-go/factstore`; factsnap does not reimplement it.

## 7. Conversion pipeline detail

```
types.Fact{Predicate, Args}
        │ ToAtom()
        ▼
ast.Atom
        │ store.Add
        ▼
SimpleInMemoryStore
        │ SimpleColumn.WriteTo
        ▼
[]byte (SC)
        │ compress
        ▼
file

file
        │ decompress + SC parse
        ▼
SimpleColumnStore
        │ ListPredicates + GetFacts
        ▼
ast.Atom
        │ atomToFact / baseTermToValue
        ▼
types.Fact{Predicate, Args}
```

## 8. Concurrency model

| Concern | Behavior |
|---------|----------|
| Internal mutexes | `pathLocks`, a `sync.Map` of per-path mutexes taken for the duration of a write |
| Concurrent Write same path (one process) | Serialised; last writer wins with a matching sidecar |
| Concurrent Write same path (two processes) | Each file lands atomically, but data and digest can come from different writers — use distinct names |
| Concurrent Read | Safe if OS allows concurrent readers of immutable file |
| Shared process state | Only `pathLocks`, which grows one entry per distinct path written |

## 9. Failure boundaries

All errors exit as `error` with a `factsnap:` or `snapshot:` prefix. No panics in
production code paths. Tests assert the absence of leftover temp files after an
unknown-codec failure and after eight writers contend on one path.

## 10. The `snapshot` layer

`snapshot` adds no I/O of its own beyond `os.ReadDir`/`os.Stat`; it is the
place where a workspace turns into paths:

| Concern | Where |
|---------|-------|
| Where snapshots live | `Dir(root)` → `<root>/.nerd/snapshots` |
| What a name may contain | `SanitizeName` — a containment boundary, since names arrive from operators |
| How a reference becomes a file | `Resolve` — full path, filename, or logical name tried against each codec extension |
| What is on disk | `List` — skips sidecars, temp residue and dotfiles; newest first |
| What a snapshot holds | `Summarize` — predicate histogram, so nobody prints 100k facts to decide |

Keeping these out of `factsnap` means the codec never has to know what a
workspace is, and keeping them out of the CLI means the next caller inherits the
same naming rules for free.
