# 07 — Dependency Map: persist

> Last verified against codebase: **2026-07-13**

## 1. Upstream (what factsnap imports)

```
factsnap
  ├── codenerd/internal/types          # Fact, MangleAtom, ToAtom
  ├── codeberg.org/TauCeti/mangle-go/ast
  ├── codeberg.org/TauCeti/mangle-go/factstore
  ├── github.com/klauspost/compress/zstd
  └── stdlib (bytes, compress/gzip, encoding/json, errors, fmt, io, os, path/filepath, strings)
```

| Dependency | Why | Risk |
|------------|-----|------|
| `internal/types` | Shared fact IR across codeNERD | Stable core type |
| `mangle-go/factstore` | SimpleColumn + store loaders | Fork-coupled; format owned upstream |
| `mangle-go/ast` | Atom/query types for collect | Same fork |
| `klauspost/compress/zstd` | Zstd writer | Listed in `go.mod` as `github.com/klauspost/compress v1.18.5` |
| `compress/gzip` | Default codec | stdlib |

### Intentionally **not** imported

| Package | Reason |
|---------|--------|
| `internal/core` | Import cycle risk; atom conversion duplicated |
| `internal/logging` | Library kept silent (gap, not dependency) |
| `internal/store` | Different durability model |
| `internal/config` | No runtime config |

## 2. Downstream (who imports factsnap)

### Production

```
(none)
```

Evidence: repository-wide search for `codenerd/internal/persist` and `persist/factsnap` under `*.go` returns **only** files inside `internal/persist/factsnap/` itself.

### Tests

Package-internal tests only (`package factsnap`).

### Docs / audit

- `Docs/architecture/persist/*` (this corpus)
- `AUDIT.md` row: `internal/persist/factsnap | clean`

## 3. Sibling durability systems (not dependents)

| System | Path | Relationship |
|--------|------|--------------|
| SQLite cold store | `internal/store` | Parallel durability; not shared code |
| Campaign JSON/JSONL | `internal/campaign` | Ad-hoc JSON files; candidate future factsnap user |
| Browser session JSON | `internal/browser` | Unrelated session metadata |
| Config files | `internal/config` | Unrelated |

## 4. External format coupling

factsnap **is** coupled to the Mangle fork’s SimpleColumn on-disk layout. Upgrading `mangle-go` may change:

- `SimpleColumn.WriteTo`
- `NewSimpleColumnStoreFromGzipBytes`
- `NewSimpleColumnStoreFromZstdBytes`

Mitigation: keep `go test ./internal/persist/...` in CI; treat format breakage as a hard fail.

## 5. Verify reverse deps (maintainer command)

```powershell
rg "codenerd/internal/persist" -g "*.go"
rg "persist/factsnap" -g "*.go"
```

If either shows a new importer, update [08-WIRING-AND-INTEGRATION.md](08-WIRING-AND-INTEGRATION.md) and this map.

## 6. Diagram (as-is vs intended)

```
AS-IS:
  [types] ──► [factsnap] ──► (disk)
                 ▲
                 └── tests only

INTENDED:
  [campaign|world|core dump|cli]
           │
           ▼
        [factsnap] ──► .nerd/**/*.sc.gz
           │
           ▼
     [core assert] (policy)
```
