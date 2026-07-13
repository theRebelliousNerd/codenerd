# diff — Architecture Corpus (`internal/diff`)

> Last verified against codebase: 2026-07-13  
> Status: Living Reference Document  
> Language: Go (module `codenerd`)  
> Primary package: `internal/diff/`  
> Scale: **1** non-test Go file ≈ **379** lines; **2** test files ≈ **949** lines; **0** `.mg`

## Scope

This corpus documents **`internal/diff`**: a small, battle-tested text-diff utility that wraps
[`github.com/sergi/go-diff/diffmatchpatch`](https://github.com/sergi/go-diff) and produces
structured `FileDiff` / `Hunk` / `Line` values for consumers (primarily the interactive
diff-approval TUI in `cmd/nerd/ui/diffview.go`).

It is **not**:

- A Git porcelain / unified-diff parser  
- A kernel, shard, VirtualStore route, or Mangle policy surface  
- The TUI that *renders* diffs (`cmd/nerd/ui/` — see `Docs/architecture/cli/`)

## Document map

| Doc | Role |
|-----|------|
| [IMPLEMENTED_SPEC.md](IMPLEMENTED_SPEC.md) | Authoritative living architecture + inventory |
| [00-ALIGNMENT-VISION-REVIEW.md](00-ALIGNMENT-VISION-REVIEW.md) | North-star alignment scores |
| [01-VISION.md](01-VISION.md) | Target product/architecture vision |
| [02-CURRENT-STATE.md](02-CURRENT-STATE.md) | Precise on-disk inventory |
| [03-GAP-ANALYSIS.md](03-GAP-ANALYSIS.md) | Spec vs reality, priorities, non-gaps |
| [04-ARCHITECTURAL-PRINCIPLES.md](04-ARCHITECTURAL-PRINCIPLES.md) | Binding design principles |
| [05-INTERNAL-ARCHITECTURE.md](05-INTERNAL-ARCHITECTURE.md) | Components, data flow, algorithms |
| [06-PUBLIC-API-AND-TYPES.md](06-PUBLIC-API-AND-TYPES.md) | Exported types and functions |
| [07-DEPENDENCY-MAP.md](07-DEPENDENCY-MAP.md) | Upstream/downstream with evidence |
| [08-WIRING-AND-INTEGRATION.md](08-WIRING-AND-INTEGRATION.md) | How callers integrate |
| [09-SAFETY-AND-INVARIANTS.md](09-SAFETY-AND-INVARIANTS.md) | Bounds, concurrency, binary gates |
| [10-TESTING-ALIGNMENT.md](10-TESTING-ALIGNMENT.md) | Tests, gaps, commands |
| [11-OBSERVABILITY.md](11-OBSERVABILITY.md) | Logging/metrics (mostly none) |
| [12-FAILURE-MODES.md](12-FAILURE-MODES.md) | Concrete failure modes + mitigations |
| [TODO.md](TODO.md) / [OPEN-QUESTIONS.md](OPEN-QUESTIONS.md) / [_progress.md](_progress.md) | Governance |

## Fact-flow position

```
user_intent → kernel → next_action → VirtualStore → (file write / propose edit)
                                                      │
                                                      ▼
                         old/new strings ──► internal/diff.ComputeDiff
                                                      │
                                                      ▼
                         FileDiff ──► cmd/nerd/ui DiffApprovalView ──► human y/n
```

`internal/diff` is a **pure library** on the Act/presentation edge. It does not assert
Mangle facts, does not consult `permitted(...)`, and does not talk to the LLM.

## Verify

```powershell
go test ./internal/diff/...
go test -race ./internal/diff/...
go test ./cmd/nerd/ui/ -run 'Diff|Word'
```

Benchmarks (optional):

```powershell
go test ./internal/diff/ -bench=. -benchmem
```

## Quality bar

Modeled on `Docs/architecture/cli/`: real file inventory, control-flow diagrams, reverse-dep
evidence, and honest gaps — **not** auto-generated inventory stubs.
