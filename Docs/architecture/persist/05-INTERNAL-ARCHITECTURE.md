# 05 — Internal Architecture: persist / factsnap

> Last verified against codebase: **2026-07-13**  
> Source of truth: `internal/persist/factsnap/factsnap.go`

## 1. Component diagram

```
                    ┌──────────────────────────┐
                    │   Caller (none today)    │
                    └────────────┬─────────────┘
                                 │
              Write / WriteCodec │ Read / LegacyJSON
                                 ▼
                    ┌──────────────────────────┐
                    │     factsnap public API  │
                    │  ensureExt / detectCodec │
                    └────────────┬─────────────┘
           ┌─────────────────────┼─────────────────────┐
           ▼                     ▼                     ▼
   types.Fact.ToAtom     factstore SimpleColumn   encoding/json
           │                     │                     │
           ▼                     ▼                     │
   SimpleInMemoryStore    gzip / zstd frames           │
           │                     │                     │
           └──────────► path.tmp │                     │
                                 ▼                     │
                            path.sc.* ◄────────────────┘
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
normalize codec (Auto→Gzip)
  │
  ▼
ensureExt(path, codec)
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
Create(path+".tmp")
  │ fail ──► ERROR
  ▼
compress into tmp
  │ fail ──► remove tmp ──► ERROR
  ▼
Sync + Close
  │ fail ──► remove tmp ──► ERROR
  ▼
Rename(tmp → path)
  │ fail ──► remove tmp ──► ERROR
  ▼
SUCCESS (cleanupTmp=false)
```

## 3. Read state machine

```
START
  │
  ▼
os.ReadFile(path)
  │ fail ──► ERROR
  ▼
detectCodec(suffix)
  │
  ├─ .sc.gz  → NewSimpleColumnStoreFromGzipBytes → collectFacts
  ├─ .sc.zst → NewSimpleColumnStoreFromZstdBytes → collectFacts
  └─ else    → json.Unmarshal([]types.Fact)
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
| Internal mutexes | None |
| Concurrent Write same path | Race on tmp/rename — undefined |
| Concurrent Read | Safe if OS allows concurrent readers of immutable file |
| Shared process state | None |

## 9. Failure boundaries

All errors exit as `error` with `factsnap:` prefix. No panics in production code paths. Tests assert absence of leftover `.tmp` after unknown-codec failure.
