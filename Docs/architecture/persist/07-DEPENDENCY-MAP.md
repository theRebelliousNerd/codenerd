# 07 — Dependency Map: persist

> Last verified against codebase: **2026-08-15**

## 1. Upstream (what the packages import)

```
factsnap
  ├── codenerd/internal/types          # Fact, MangleAtom, ToAtom
  ├── codenerd/internal/logging        # CategoryStore debug/warn lines
  ├── codeberg.org/TauCeti/mangle-go/ast
  ├── codeberg.org/TauCeti/mangle-go/factstore
  ├── github.com/klauspost/compress/zstd
  └── stdlib (bytes, compress/gzip, crypto/sha256, encoding/hex, encoding/json,
              errors, fmt, io, os, path/filepath, strings, sync, time)

snapshot
  ├── codenerd/internal/persist/factsnap
  ├── codenerd/internal/types
  └── stdlib (fmt, os, path/filepath, sort, strings, time)
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
| `internal/core` | Import cycle risk; atom conversion duplicated (see OPEN-QUESTIONS Q5). The CLI holds the core dependency instead |
| `internal/store` | Different durability model |
| `internal/config` | No runtime config |

## 2. Downstream (who imports factsnap)

### Production

```
cmd/nerd/cmd_snapshot.go      ──► persist/snapshot, persist/factsnap
internal/persist/snapshot     ──► persist/factsnap
```

`cmd_snapshot.go` is the first production caller (kernel debug export; see
[08-WIRING-AND-INTEGRATION.md](08-WIRING-AND-INTEGRATION.md)). Nothing else
imports either package as of 2026-08-15.

### Tests

- `internal/persist/factsnap/*_test.go` — package-internal codec tests
- `internal/persist/snapshot/snapshot_test.go` — workspace store
- `internal/persist/snapshot/kernel_roundtrip_test.go` — `package snapshot_test`,
  imports `internal/core`; the only place persist code meets a real kernel
- `cmd/nerd/cmd_snapshot_test.go` — command behaviour end to end

### Docs / audit

- `Docs/architecture/persist/*` (this corpus)
- `AUDIT.md` row: `internal/persist/factsnap | clean`

## 3. Sibling durability systems (not dependents)

| System | Path | Relationship |
|--------|------|--------------|
| SQLite cold store | `internal/store` | Parallel durability; not shared code |
| Campaign JSON/JSONL | `internal/campaign` | Ad-hoc JSON files; candidate future snapshot user |
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
rg "persist/snapshot" -g "*.go"
```

If a new importer appears, update [08-WIRING-AND-INTEGRATION.md](08-WIRING-AND-INTEGRATION.md) and this map.

## 6. Diagram (as-is vs intended)

```
AS-IS (2026-08-15):
  [cmd/nerd/cmd_snapshot.go] ──► [core kernel]  (booted locally, no LLM)
           │                          │
           │                          ▼
           └────► [snapshot] ──► [factsnap] ──► .nerd/snapshots/*.sc.gz(+.sha256)
                       ▲
                       └── operator: import / --assert / --to-mangle

STILL INTENDED:
  [campaign artifacts | world index freeze]
           │
           ▼
       [snapshot]
```
