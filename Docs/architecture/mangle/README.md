# mangle — Architecture Corpus (`internal/mangle`)

> Last verified against codebase: **2026-07-13**  
> Status: Living Reference Document  
> Language: Go (module `codenerd`)  
> Upstream engine: `codeberg.org/TauCeti/mangle-go`  
> Primary package: `internal/mangle/` (+ `feedback/`, `synth/`, `transpiler/`)

## Scope

This corpus documents the **Mangle substrate** for codeNERD: the production wrapper around Google Mangle (TauCeti fork), differential evaluation, LLM-facing validation/feedback, structured synthesis (MangleSynth), grammar-constrained atom repair, schema drift gates, LSP for `.mg` files, and parse serialization.

It is **not**:

- The kernel executive itself (`Docs/architecture/core/`) — kernel *owns* evaluation orchestration and `permitted(...)`.
- The policy corpus (`.mg` under `internal/core/defaults/policy/`).
- The CLI `mangle check` / `mangle lsp` UX surface (`Docs/architecture/cli/`).

It **is** the package that:

1. Wraps `mangle-go` with gas limits, typed fact insertion, persistence hooks, and stratified eval.
2. Provides incremental / differential evaluation used by the kernel when `features.diff_eval` is on.
3. Makes LLM-generated logic *safe enough to load* via pre-validation, sanitizer, feedback retry, and schema gates.
4. Serializes all ANTLR parse calls process-wide (`ParseUnit` / `ParseAtom`).

## North-star placement

```
LLM (creative)  ──writes candidate rules / atoms──►  mangle/feedback + synth + transpiler
                                                        │
                                                        ▼
Logic executive (kernel)  ◄── Engine / DifferentialEngine ──►  VirtualStore, shards
        permitted(...) default deny lives in core policy; this package enforces
        Decl arity, forbidden heads, parse safety, and gas limits.
```

Fact-flow reminder:

```
user input → perception → user_intent → kernel next_action
  → VirtualStore / shards / tools → articulation
```

`internal/mangle` sits **under** the kernel (evaluation machinery) and **beside** shards that synthesize rules (legislator, mangle_repair, executive autopoiesis).

## Document map

| Doc | Role |
|-----|------|
| [IMPLEMENTED_SPEC.md](IMPLEMENTED_SPEC.md) | **Flagship** living architecture + deep dives (engine, differential, feedback) |
| [00-ALIGNMENT-VISION-REVIEW.md](00-ALIGNMENT-VISION-REVIEW.md) | North-star alignment scores with evidence |
| [01-VISION.md](01-VISION.md) | Target architecture vision for this package |
| [02-CURRENT-STATE.md](02-CURRENT-STATE.md) | Precise on-disk inventory |
| [03-GAP-ANALYSIS.md](03-GAP-ANALYSIS.md) | Spec vs reality, priorities, non-gaps |
| [04-ARCHITECTURAL-PRINCIPLES.md](04-ARCHITECTURAL-PRINCIPLES.md) | Binding design principles |
| [05-INTERNAL-ARCHITECTURE.md](05-INTERNAL-ARCHITECTURE.md) | Components, data flow, state machines |
| [06-PUBLIC-API-AND-TYPES.md](06-PUBLIC-API-AND-TYPES.md) | Exported types/funcs that matter |
| [07-DEPENDENCY-MAP.md](07-DEPENDENCY-MAP.md) | Upstream/downstream with evidence |
| [08-WIRING-AND-INTEGRATION.md](08-WIRING-AND-INTEGRATION.md) | Boot, kernel, shards, CLI hooks |
| [09-SAFETY-AND-INVARIANTS.md](09-SAFETY-AND-INVARIANTS.md) | Safety, concurrency, Decl gates |
| [10-TESTING-ALIGNMENT.md](10-TESTING-ALIGNMENT.md) | Tests, gaps, commands |
| [11-OBSERVABILITY.md](11-OBSERVABILITY.md) | Logging, debug dumps, stats |
| [12-FAILURE-MODES.md](12-FAILURE-MODES.md) | Concrete failures + mitigations |
| [TODO.md](TODO.md) / [OPEN-QUESTIONS.md](OPEN-QUESTIONS.md) / [_progress.md](_progress.md) | Governance |

## Verify

```powershell
# Package tests (no CGO required for pure mangle package tests)
go test ./internal/mangle/...

# Targeted deep suites
go test ./internal/mangle -run 'TestEngine|TestDifferential|TestSchema'
go test ./internal/mangle/feedback/...
go test ./internal/mangle/synth/...
go test ./internal/mangle/transpiler/...

# Kernel path that constructs DifferentialEngine
go test ./internal/core -run 'Diff|Evaluate' -count=1

# Policy corpus still parses through Engine
go test ./internal/core/defaults/policy/... -count=1
```

Build the binary only when testing CLI mangle commands:

```powershell
$env:CGO_CFLAGS = "-IC:/CodeProjects/codeNERD/sqlite_headers"
go build -o nerd.exe ./cmd/nerd
./nerd.exe mangle-check --help
```

## Package tree (summary)

```
internal/mangle/
  engine.go              # Engine, Config, Fact, Query, gas limits
  differential.go        # DifferentialEngine, strata, unified fast path
  grammar.go             # AtomValidator, RepairLoop (GCD)
  schema_validator.go    # Decl drift + forbidden learned heads
  parse_lock.go          # Process-wide ParseUnit / ParseAtom
  proof_tree.go          # Derivation traces for glass-box
  lsp.go                 # Language server for .mg
  simd_intersect_*.go    # Sorted-set intersect (amd64/generic)
  intent_routing.mg      # Declarative intent → action/persona rules
  feedback/              # LLM validate-retry loop
  synth/                 # mangle_synth_v1 JSON → Mangle text
  transpiler/            # Sanitizer (atom interning, agg, safety)
```

## Quality bar

Modeled on `Docs/architecture/cli/`: real inventories, control-flow diagrams, wiring evidence, honest gaps — **not** thin auto-inventory stubs.
