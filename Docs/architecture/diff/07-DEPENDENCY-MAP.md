# 07 — Dependency Map: `internal/diff`

> Last verified against codebase: 2026-07-13

## 1. Upstream (what this package imports)

### Standard library

| Package | Usage |
|---------|-------|
| `strings` | Split lines, `IndexByte` for NUL |
| `sync` | `sync.Map` cache |
| `time` | `diffTimeout` duration |

### Third-party

| Module | Version (go.mod) | Usage |
|--------|------------------|-------|
| `github.com/sergi/go-diff` | v1.4.0 | `diffmatchpatch` algorithm engine |

### Internal

**None.** `internal/diff` does not import any other `codenerd/internal/*` package.

```
internal/diff
  ├── stdlib: strings, sync, time
  └── github.com/sergi/go-diff/diffmatchpatch
```

## 2. Downstream (who imports this package)

### Production

| Importer | Path | How |
|----------|------|-----|
| CLI UI | `cmd/nerd/ui/diffview.go` | Types, `NewEngine`, `ComputeDiff`, `ComputeWordLevelDiff` |

### Tests

| Importer | Path |
|----------|------|
| UI word-diff tests | `cmd/nerd/ui/word_diff_test.go` |
| Package self-tests | `internal/diff/*_test.go` |

### Negative scan

No imports under (non-exhaustive but grepped):

- `internal/core/**`
- `internal/session/**`
- `internal/tools/**`
- `internal/shards/**`
- `internal/prompt/**`
- `internal/perception/**`
- `internal/articulation/**`
- `internal/campaign/**`
- other `cmd/` packages beyond `cmd/nerd/ui`

## 3. Dependency diagram

```
github.com/sergi/go-diff
          ▲
          │
   internal/diff  ◄──── cmd/nerd/ui (diffview.go, word_diff_test.go)
          │
          └── (no further internal fans-out)
```

## 4. Module pin note

`go.mod` lists:

```
github.com/sergi/go-diff v1.4.0
```

Upgrades should re-run the full `./internal/diff` suite and UI word-diff tests; semantic
cleanup / timeout behavior is library-defined.

## 5. Audit command

```powershell
rg "codenerd/internal/diff" -g "*.go"
```

## 6. Architectural implication

Because fan-out is tiny, API breaks are localized to the TUI. Conversely, the package is
**easy to over-engineer** — resist growing it until a second production consumer appears
(e.g. non-interactive patch reports). Keep general-use features only.
