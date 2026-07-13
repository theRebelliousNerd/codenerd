# persist — Architecture Corpus

> Last verified against codebase: **2026-07-13**  
> Status: Living Reference Document — code-grounded full corpus  
> Source: `internal/persist/`  
> Implementation: **1** non-test Go file (`factsnap/factsnap.go`, 287 lines), **4** test files, **0** Mangle sources  
> Package name on disk: subpackage only — `package factsnap` under `internal/persist/factsnap/`

## Scope

`internal/persist` is a **thin, file-oriented fact snapshot layer**. Today it contains a single subpackage:

| Subpackage | Role |
|------------|------|
| [`factsnap`](../../internal/persist/factsnap/) | Serialize `[]types.Fact` to disk as Mangle **SimpleColumn** + **gzip** or **zstd**; read back with suffix auto-detect; migrate legacy JSON snapshots |

It is **not** the durable EDB / sqlite cold store (`internal/store`), **not** campaign artifact JSON (`internal/campaign`), and **not** session state. It is a **codec + atomic file write** utility aimed at portable, highly compressible fact corpora.

### Critical wiring status

As of 2026-07-13, **no production package imports** `codenerd/internal/persist/factsnap` (grep across `*.go` finds only the package itself and tests). The API is complete and well-tested; integration into kernel dump, campaign export, world-model freeze, or CLI snapshot commands is still open. Treat this as a **dormant integration point**, not dead code — do not delete without a wiring audit.

## Document map

| Doc | Role |
|-----|------|
| [IMPLEMENTED_SPEC.md](IMPLEMENTED_SPEC.md) | Flagship living spec: inventory, write/read flows, codecs, gaps |
| [00-ALIGNMENT-VISION-REVIEW.md](00-ALIGNMENT-VISION-REVIEW.md) | North-star scores with evidence |
| [01-VISION.md](01-VISION.md) | Target product/architecture role for fact snapshots |
| [02-CURRENT-STATE.md](02-CURRENT-STATE.md) | Precise file inventory and hotspots |
| [03-GAP-ANALYSIS.md](03-GAP-ANALYSIS.md) | Spec vs reality, non-gaps, priorities |
| [04-ARCHITECTURAL-PRINCIPLES.md](04-ARCHITECTURAL-PRINCIPLES.md) | Binding design principles |
| [05-INTERNAL-ARCHITECTURE.md](05-INTERNAL-ARCHITECTURE.md) | Components, data flow, state machine |
| [06-PUBLIC-API-AND-TYPES.md](06-PUBLIC-API-AND-TYPES.md) | Exported API with file refs |
| [07-DEPENDENCY-MAP.md](07-DEPENDENCY-MAP.md) | Upstream / (missing) downstream |
| [08-WIRING-AND-INTEGRATION.md](08-WIRING-AND-INTEGRATION.md) | Intended call sites; current zero wiring |
| [09-SAFETY-AND-INVARIANTS.md](09-SAFETY-AND-INVARIANTS.md) | Atomic rename, determinism, type round-trips |
| [10-TESTING-ALIGNMENT.md](10-TESTING-ALIGNMENT.md) | Tests, coverage shape, commands |
| [11-OBSERVABILITY.md](11-OBSERVABILITY.md) | Logging (none today) / debug hooks |
| [12-FAILURE-MODES.md](12-FAILURE-MODES.md) | Concrete failures + mitigations |
| [TODO.md](TODO.md) | Prioritized backlog |
| [OPEN-QUESTIONS.md](OPEN-QUESTIONS.md) | Real open design questions |
| [_progress.md](_progress.md) | Rebuild journal |

## Verify

```powershell
# Unit tests (no CGO required for this package)
go test ./internal/persist/...

# Verbose size comparison (informational)
go test ./internal/persist/factsnap/ -v -run TestSizeComparison

# Codec parity at 100 / 1k / 10k facts
go test ./internal/persist/factsnap/ -v -run TestCodecParity
```

```bash
go test ./internal/persist/...
```

## North star placement

```
user_intent → kernel → next_action → VirtualStore → articulation
                              │
                              │  (intended, not yet wired)
                              ▼
                     factsnap.Write / Read
                     (.sc.gz / .sc.zst files)
```

- **LLM** never talks to factsnap directly (no prompt atoms, no JIT).
- **Executive / data plane**: facts are Mangle-shaped atoms already decided by the kernel; factsnap is pure durable projection of `[]types.Fact`.
- **Constitutional safety**: factsnap does not enforce `permitted(...)`; callers that *assert* loaded facts back into the kernel remain responsible for policy.

## Related corpora

- [`types`](../types/) — `types.Fact`, `types.MangleAtom`, `ToAtom()`
- [`mangle`](../mangle/) / [`core`](../core/) — live fact stores, `atomToFact` duplication note
- [`store`](../store/) — sqlite cold storage (different durability model)
- [`campaign`](../campaign/) — JSON/JSONL assault artifacts (candidate consumer)
- [`cli`](../cli/) — quality-bar reference corpus
