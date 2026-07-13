# context — Architecture Corpus (`internal/context`)

> Last verified against codebase: 2026-07-13  
> Status: Living Reference Document  
> Language: Go (module `codenerd`)  
> Primary package: `internal/context/`  
> Scale: **9** non-test Go files ≈ **4,700** lines; **11** test files; **0** package-owned `.mg` (Mangle surface lives in `internal/core/defaults/`)

## Scope

This corpus documents **semantic compression and spreading activation** — the subsystem that makes “infinite context” real for long-horizon agent sessions.

It covers:

1. **Spreading activation** — 9-component scoring that ranks kernel facts for the current intent.
2. **Semantic compression** — discard surface text; retain logical atoms + rolling history.
3. **Token window management** — reserves (core / atoms / history / working), thresholds, hard limit errors.
4. **Serialization** — facts → Mangle notation context blocks for LLM injection.
5. **Context feedback learning** — SQLite-backed third feedback loop (helpful vs noise predicates).
6. **Kernel co-derivation** — optional `should_include_context` / observation masking via policy rules.

It is **not** the chat TUI (`Docs/architecture/cli/`), not the kernel (`Docs/architecture/core/`), and not retrieval/RAG (`internal/retrieval/`).

## Document map

| Doc | Role |
|-----|------|
| [IMPLEMENTED_SPEC.md](IMPLEMENTED_SPEC.md) | Authoritative living architecture + inventory + deep dives |
| [00-ALIGNMENT-VISION-REVIEW.md](00-ALIGNMENT-VISION-REVIEW.md) | North-star alignment scores |
| [01-VISION.md](01-VISION.md) | Target product/architecture vision |
| [02-CURRENT-STATE.md](02-CURRENT-STATE.md) | Precise on-disk inventory |
| [03-GAP-ANALYSIS.md](03-GAP-ANALYSIS.md) | Spec vs reality matrix |
| [04-ARCHITECTURAL-PRINCIPLES.md](04-ARCHITECTURAL-PRINCIPLES.md) | Binding design principles |
| [05-INTERNAL-ARCHITECTURE.md](05-INTERNAL-ARCHITECTURE.md) | Components, data flow, state machines |
| [06-PUBLIC-API-AND-TYPES.md](06-PUBLIC-API-AND-TYPES.md) | Exported types and constructors |
| [07-DEPENDENCY-MAP.md](07-DEPENDENCY-MAP.md) | Upstream/downstream packages |
| [08-WIRING-AND-INTEGRATION.md](08-WIRING-AND-INTEGRATION.md) | Boot, chat process, prompt JIT hooks |
| [09-SAFETY-AND-INVARIANTS.md](09-SAFETY-AND-INVARIANTS.md) | Safety facts, concurrency, caps |
| [10-TESTING-ALIGNMENT.md](10-TESTING-ALIGNMENT.md) | Tests, gaps, commands |
| [11-OBSERVABILITY.md](11-OBSERVABILITY.md) | `CategoryContext` logging + metrics |
| [12-FAILURE-MODES.md](12-FAILURE-MODES.md) | Concrete failure modes + mitigations |
| [TODO.md](TODO.md) / [OPEN-QUESTIONS.md](OPEN-QUESTIONS.md) / [_progress.md](_progress.md) | Governance |

## Fact-flow placement

```
user input → perception → user_intent → kernel next_action
  → VirtualStore / shards / tools → articulation
       ↑                                    │
       │         internal/context           │
       │  BuildContext / GetContextString   │
       │  ProcessTurn (async compress)      │
       └──────── inject compressed block ───┘
```

The package sits **beside** the OODA loop: it does not choose `next_action`, but it decides **which logical state** the LLM sees on the next turn.

## Build / verify

```powershell
go test ./internal/context/...
go test -race ./internal/context/...
```

Integration / long-horizon:

```powershell
# context harness (separate package)
go test ./internal/testing/context_harness/...

# CLI context stress entry
# nerd test-context  (cmd/nerd/cmd_test_context.go)
```

Binary build (when exercising chat path):

```powershell
$env:CGO_CFLAGS = "-IC:/CodeProjects/codeNERD/sqlite_headers"
go build -o nerd.exe ./cmd/nerd
```

## Quality bar

Modeled on `Docs/architecture/cli/`: real inventories, control-flow diagrams, wiring evidence, honest gaps — **not** auto-generated stubs.

## Related corpora

- `Docs/architecture/core/` — kernel, `should_include_context` consumers
- `Docs/architecture/perception/` / `articulation/` / `prompt/` — control packets + JIT activation scores
- `Docs/architecture/cli/` — chat boot + ProcessTurn + budget UI
- `Docs/architecture/store/` — `StoreCompressedState`, `LogActivation`
- `internal/core/defaults/policy/context_compilation.mg` — Mangle C1/C3/C4 rules
- `internal/context/README.md` — package-local overview (may lag this corpus)
