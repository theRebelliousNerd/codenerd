# prompt — Architecture Corpus (`internal/prompt`)

> Last verified against codebase: **2026-07-13**  
> Status: Living Reference Document  
> Language: Go (module `codenerd`)  
> Primary package: `internal/prompt/` (+ `internal/prompt/sync/`, `internal/prompt/atoms/`)  
> Role: **JIT Prompt Compiler** — the transduction boundary between Mangle (executive) and the LLM (creative center)

## Scope

This corpus documents codeNERD’s **prompt JIT system**: how system prompts and agent runtime configs are compiled at runtime from **prompt atoms**, selected by **context + Mangle + vectors**, fitted to a **token budget**, and assembled for session / shard execution.

It is **not**:

- The Mangle policy corpus itself (`internal/core/defaults/policy/`, `jit_compiler.mg`)
- Session OODA loop details (`Docs/architecture/session/`)
- Articulation piggyback emission (`internal/articulation/`) — only the JIT bridge into it

## North-star placement

```
user input → perception → user_intent → kernel next_action
  → VirtualStore / shards / tools
  → Session Executor builds CompilationContext
  → JITPromptCompiler.Compile → system prompt + EffectiveAgentRuntimeConfig
  → LLM (creative) under AllowedTools + permitted(...) (executive)
  → articulation → TUI/stdout
```

**Inversion of control:** new LLM-facing behavior becomes **prompt atoms** (and ConfigAtoms for tools/policies), not ad-hoc shard prompt strings. Logic selects; the model describes.

## Document map

| Doc | Role |
|------|------|
| [IMPLEMENTED_SPEC.md](IMPLEMENTED_SPEC.md) | **Flagship** living architecture + deep dives |
| [00-ALIGNMENT-VISION-REVIEW.md](00-ALIGNMENT-VISION-REVIEW.md) | North-star alignment scores |
| [01-VISION.md](01-VISION.md) | Target product/architecture vision |
| [02-CURRENT-STATE.md](02-CURRENT-STATE.md) | On-disk inventory, hotspots |
| [03-GAP-ANALYSIS.md](03-GAP-ANALYSIS.md) | Spec vs reality, priorities |
| [04-ARCHITECTURAL-PRINCIPLES.md](04-ARCHITECTURAL-PRINCIPLES.md) | Binding design principles |
| [05-INTERNAL-ARCHITECTURE.md](05-INTERNAL-ARCHITECTURE.md) | Components, data flow, state |
| [06-PUBLIC-API-AND-TYPES.md](06-PUBLIC-API-AND-TYPES.md) | Exported types and constructors |
| [07-DEPENDENCY-MAP.md](07-DEPENDENCY-MAP.md) | Upstream/downstream packages |
| [08-WIRING-AND-INTEGRATION.md](08-WIRING-AND-INTEGRATION.md) | Boot, session, shards, CLI |
| [09-SAFETY-AND-INVARIANTS.md](09-SAFETY-AND-INVARIANTS.md) | Safety, concurrency, selection rules |
| [10-TESTING-ALIGNMENT.md](10-TESTING-ALIGNMENT.md) | Tests, commands, gaps |
| [11-OBSERVABILITY.md](11-OBSERVABILITY.md) | Stats, manifest, log categories |
| [12-FAILURE-MODES.md](12-FAILURE-MODES.md) | Failure modes + mitigations |
| [13-PROMPT-JIT-DEEP-DIVE.md](13-PROMPT-JIT-DEEP-DIVE.md) | Compiler/selector/budget/resolver end-to-end |
| [TODO.md](TODO.md) / [OPEN-QUESTIONS.md](OPEN-QUESTIONS.md) / [_progress.md](_progress.md) | Governance |

## Verify commands

```powershell
# Package tests
go test ./internal/prompt/...

# With CGO when sqlite-vec paths are exercised in broader e2e
$env:CGO_CFLAGS = "-IC:/CodeProjects/codeNERD/sqlite_headers"
go test ./internal/prompt/... ./tests/e2e/... -count=1

# Rebuild baked default corpus after atom YAML edits
go run ./cmd/tools/prompt_builder -input internal/prompt/atoms -output internal/core/defaults/prompt_corpus.db
# or: powershell -ExecutionPolicy Bypass -File build/build_prompt_corpus.ps1 -SkipEmbeddings
```

## Source anchors (start here)

| Concern | Path |
|---------|------|
| Compile entry | `internal/prompt/compiler.go` — `JITPromptCompiler.Compile` |
| Atom model | `internal/prompt/atoms.go` — `PromptAtom`, categories |
| Selection (skeleton/flesh) | `internal/prompt/selector.go` — `AtomSelector` |
| Deps / order | `internal/prompt/resolver.go` — `DependencyResolver` |
| Budget + polymorphism | `internal/prompt/budget.go` — `TokenBudgetManager.Fit` |
| Assembly | `internal/prompt/assembler.go` — `FinalAssembler` |
| Context dimensions | `internal/prompt/context.go` — `CompilationContext` |
| Config / tools | `internal/prompt/config_factory.go`, `config_defaults.go` |
| YAML → SQLite | `internal/prompt/loader.go`, `sync/synchronizer.go` |
| Embedded corpus | `internal/prompt/embedded.go`, `atoms/**/*.yaml` |
| Mangle selection rules | `internal/core/defaults/jit_compiler.mg`, `policy/jit_*.mg` |
| Session consumer | `internal/session/executor.go` |
| Articulation bridge | `internal/articulation/prompt_assembler.go` |

## Quality bar

Modeled on `Docs/architecture/cli/`: real inventories, control-flow diagrams, wiring evidence, honest gaps — **not** auto-generated stubs.
