# persist — Architecture Corpus

> Last verified against codebase: **2026-08-15**  
> Status: Living Reference Document — code-grounded full corpus  
> Source: `internal/persist/`  
> Implementation: **3** non-test Go files (`doc.go`, `factsnap/factsnap.go` 537 lines, `snapshot/snapshot.go` 253 lines), **6** test files, **0** Mangle sources  
> Production caller: `cmd/nerd/cmd_snapshot.go` — `nerd snapshot export | import | list`

## Scope

`internal/persist` is a **thin, file-oriented fact snapshot layer**. Today it contains a single subpackage:

| Subpackage | Role |
|------------|------|
| [`factsnap`](../../internal/persist/factsnap/) | Serialize `[]types.Fact` to disk as Mangle **SimpleColumn** + **gzip** or **zstd**; read back with suffix and magic-byte detection; verify a `.sha256` sidecar; migrate legacy JSON snapshots |
| [`snapshot`](../../internal/persist/snapshot/) | The workspace store: `.nerd/snapshots/` layout, name sanitisation, bare-name resolution, listing, predicate summaries |

It is **not** the durable EDB / sqlite cold store (`internal/store`), **not** campaign artifact JSON (`internal/campaign`), and **not** session state. It is a **codec + atomic file write** utility aimed at portable, highly compressible fact corpora.

### Wiring status

As of 2026-08-15 the package is **wired**. `cmd/nerd/cmd_snapshot.go` is the
first production caller: it boots a workspace kernel locally (no API key, no
network, no shards), exports its EDB through `snapshot.Export`, and reads it
back through `snapshot.Import`.

```bash
nerd snapshot export                 # .nerd/snapshots/kernel-YYYYMMDD-HHMMSS.sc.gz
nerd snapshot export idx --codec zstd -p code_defines -p code_calls
nerd snapshot list
nerd snapshot import idx --show 20
nerd snapshot import idx --to-mangle /tmp/idx.mg   # reviewable Datalog
nerd snapshot import idx --assert                  # in-process kernel only
```

Campaign fact bags and a world code-index freeze remain unwired candidates, and
now have a paved path (`snapshot.Export`) rather than raw codec calls.

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
| [08-WIRING-AND-INTEGRATION.md](08-WIRING-AND-INTEGRATION.md) | Real call sites, canonical workspace paths |
| [09-SAFETY-AND-INVARIANTS.md](09-SAFETY-AND-INVARIANTS.md) | Atomic rename, determinism, type round-trips |
| [10-TESTING-ALIGNMENT.md](10-TESTING-ALIGNMENT.md) | Tests, coverage shape, commands |
| [11-OBSERVABILITY.md](11-OBSERVABILITY.md) | Logging, operator debug workflow |
| [12-FAILURE-MODES.md](12-FAILURE-MODES.md) | Concrete failures + mitigations |
| [TODO.md](TODO.md) | Prioritized backlog |
| [OPEN-QUESTIONS.md](OPEN-QUESTIONS.md) | Real open design questions |
| [_progress.md](_progress.md) | Rebuild journal |

## Verify

```powershell
# Unit tests (no CGO required for these packages)
go test ./internal/persist/...
go test ./cmd/nerd/ -run TestSnapshot

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
                              │  offline side channel (operator-driven)
                              ▼
                    snapshot.Export / Import
                              │
                     factsnap.Write / Read
                     (.sc.gz / .sc.zst + .sha256)
```

- **LLM** never talks to factsnap directly (no prompt atoms, no JIT).
- **Executive / data plane**: facts are Mangle-shaped atoms already decided by the kernel; factsnap is pure durable projection of `[]types.Fact`.
- **Constitutional safety**: factsnap does not enforce `permitted(...)`; callers that *assert* loaded facts back into the kernel remain responsible for policy. There is deliberately **no boot-time snapshot load**: `nerd snapshot import` summarises by default, `--assert` loads into a kernel that dies with the process, and adopting facts permanently means an operator moving rendered Datalog into `.nerd/mangle/` themselves.

## Related corpora

- [`types`](../types/) — `types.Fact`, `types.MangleAtom`, `ToAtom()`
- [`mangle`](../mangle/) / [`core`](../core/) — live fact stores, `atomToFact` duplication note
- [`store`](../store/) — sqlite cold storage (different durability model)
- [`campaign`](../campaign/) — JSON/JSONL assault artifacts (candidate consumer)
- [`cli`](../cli/) — where `nerd snapshot` is registered; also the quality-bar reference corpus
