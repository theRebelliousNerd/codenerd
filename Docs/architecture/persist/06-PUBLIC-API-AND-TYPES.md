# 06 — Public API and Types: persist / factsnap

> Last verified against codebase: **2026-07-13**  
> Import: `codenerd/internal/persist/factsnap`

## 1. Constants

| Name | Value | File |
|------|-------|------|
| `ExtGzip` | `".sc.gz"` | `factsnap.go:39` |
| `ExtZstd` | `".sc.zst"` | `factsnap.go:42` |
| `ExtJSON` | `".json"` | `factsnap.go:45` |

## 2. Types

### `type Codec int`

Selects compression on write. Defined at `factsnap.go:48`.

| Value | Meaning on write |
|-------|------------------|
| `CodecAuto` (0) | Treated as gzip |
| `CodecGzip` (1) | gzip wrap |
| `CodecZstd` (2) | zstd wrap |
| other | `WriteCodec` returns `factsnap: unknown codec %d` |

On **read**, `detectCodec` returns `CodecGzip`, `CodecZstd`, or `-1` (legacy JSON). Callers should not pass `-1` to `WriteCodec`.

## 3. Functions

### `func Write(path string, facts []types.Fact) error`

- Location: `factsnap.go:61`
- Behavior: `WriteCodec(path, facts, CodecGzip)`
- Side effects: may create parent dirs; may rewrite path extension to `.sc.gz`

### `func WriteCodec(path string, facts []types.Fact, codec Codec) error`

- Location: `factsnap.go:67`
- Full encode pipeline (see [05-INTERNAL-ARCHITECTURE.md](05-INTERNAL-ARCHITECTURE.md))
- Empty `facts` is allowed (empty store → empty-ish snapshot; still a valid file)

### `func Read(path string) ([]types.Fact, error)`

- Location: `factsnap.go:153`
- Codec from path suffix
- Returns facts in store iteration order (not original write order)

### `func LegacyJSON(path string) ([]types.Fact, error)`

- Location: `factsnap.go:184`
- Always JSON; for migration paths that still emit JSON

### `func CanonicalPath(path string, codec Codec) string`

- Location: `factsnap.go:198`
- Pure path transform; does not touch disk

## 4. Fact value contract (imported)

From `codenerd/internal/types`:

```go
type Fact struct {
    Predicate string
    Args      []any
}

type MangleAtom string
```

`Fact.ToAtom()` is the write-side encoder. Supported arg kinds include `MangleAtom`, `string`, integers, floats, bool, `time.Time`, `time.Duration` (see `internal/types/types.go`).

**On read**, name constants become `types.MangleAtom`; numbers are int64; strings remain strings. Callers must not assume Go `bool` or `time.Time` reappear as those types after SimpleColumn round-trip.

## 5. Error strings (stable greps)

| Prefix pattern | When |
|----------------|------|
| `factsnap: mkdir:` | parent dir create failed |
| `factsnap: fact N (pred) to atom:` | `ToAtom` failed |
| `factsnap: simplecolumn write:` | SC encode failed |
| `factsnap: create …` | temp file create |
| `factsnap: gzip write/close` | gzip path |
| `factsnap: zstd writer/write/close` | zstd path |
| `factsnap: unknown codec` | bad enum |
| `factsnap: sync/close/rename` | publish path |
| `factsnap: read/gzip decode/zstd decode/legacy json decode` | read path |
| `factsnap: nil store` | defensive |
| `factsnap: collect facts for` | predicate walk failed |

## 6. Non-exported API (do not call from outside)

| Func | Role |
|------|------|
| `ensureExt` | path rewrite |
| `detectCodec` | suffix → codec |
| `collectFacts` | SC store → `[]Fact` |
| `atomToFact` | atom → fact |
| `baseTermToValue` | constant typing |

## 7. Recommended call patterns

**Export default:**

```go
err := factsnap.Write(filepath.Join(dir, "slice"), facts)
// file: dir/slice.sc.gz
```

**Export zstd with known path:**

```go
p := factsnap.CanonicalPath(filepath.Join(dir, "slice"), factsnap.CodecZstd)
err := factsnap.WriteCodec(p, facts, factsnap.CodecZstd)
```

**Import:**

```go
facts, err := factsnap.Read(p)
```

**Migrate JSON:**

```go
facts, err := factsnap.LegacyJSON(oldPath)
if err == nil {
    _ = factsnap.Write(newBase, facts)
}
```
