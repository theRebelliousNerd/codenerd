# prompt — Dependency Map

> Last verified: **2026-07-13**

## Upstream (what `internal/prompt` imports)

| Package | Use |
|---------|-----|
| `codenerd/internal/logging` | CategoryJIT/Context/Store timers and logs |
| `codenerd/internal/jit/config` | `EffectiveAgentRuntimeConfig` |
| `codenerd/internal/store` | LocalStore, LearningStore bridges |
| `codenerd/internal/embedding` | Engines, task types |
| `codenerd/internal/core` | Fact types, PredicateCorpus (predicate selector), MangleAtom |
| `codenerd/internal/sqlpragmas` | SQLite pragmas on open |
| `database/sql` + `mattn/go-sqlite3` | Corpus DBs |
| `gopkg.in/yaml.v3` | Atom YAML |
| `golang.org/x/sync/errgroup`, `singleflight` | Parallel collect / coalesce |
| stdlib | embed, crypto/sha256, sync/atomic, container/list, … |

**Avoided:** direct import of `internal/session` (session depends on prompt). Kernel accessed via **ports** (`KernelQuerier`) to reduce cycles.

## Downstream (who imports `codenerd/internal/prompt`)

| Area | Evidence |
|------|----------|
| `internal/session` | Executor/Spawner Compile + CompilationContext |
| `internal/articulation` | PromptAssembler JIT bridge + tests |
| `internal/shards` / system shards | registration, legislator, mangle_repair, planner, router, perception, world_model |
| `cmd/nerd` | campaign boot, chat session boot, ui/jit_page |
| `internal/prompt/sync` | AtomLoader |
| `tests/e2e/*` | prompt_compiler_*, session_*, orchestrator_*, piggyback_*, specialist_* |
| Autopoiesis / ouroboros | may compile tool prompts via related compilers (check `ouroboros.go` paths) |

## Sibling coupling (Mangle)

Not Go imports, but **runtime contracts**:

| Artifact | Predicate / role |
|----------|------------------|
| `schemas_prompts.mg` | Decl prompt_atom, atom_selector, compile_context, vector_hit, … |
| `jit_compiler.mg` | selected_result, mandatory_selection, blocked_by_context |
| `policy/jit_logic.mg` | Context dimension helpers |
| `policy/jit_selection.mg` | Selection policy |

## Data-plane dependencies

```
atoms YAML ──embed──► EmbeddedCorpus
atoms YAML ──builder──► prompt_corpus.db ──materialize──► .nerd/prompts/corpus.db
agents prompts.yaml ──sync──► *_knowledge.db
embeddings engine ──► vector_searcher / loader
kernel ──Assert/Query/Retract──► selector
```

## Circular import risk map

| Risk | Mitigation |
|------|------------|
| prompt ↔ core | Use `Fact` local type + KernelQuerier; limited core imports for PredicateCorpus |
| prompt ↔ session | Session imports prompt; prompt never imports session |
| prompt ↔ articulation | Articulation imports prompt; baseline can run without articulation |

## Dependency diagram

```
embedding / store / logging / jit/config / core(Predicate)
              ▲
              │
        internal/prompt
              ▲
    ┌─────────┼──────────┬────────────┐
 session  articulation  shards/sys   cmd/nerd
    ▲         ▲             ▲           ▲
    └─────────┴────── e2e ──┴───────────┘
```
