# prompt — Current State Inventory

> Last verified: **2026-07-13**  
> Tree: `internal/prompt/`

## Package scale

| Class | Count (approx.) | Notes |
|-------|----------------:|-------|
| Non-test Go sources (`internal/prompt/*.go`) | **~25** | Excludes `*_test.go` |
| Test Go files | **~32** | Unit + gap/boundary tests |
| Subpackage Go | **2** | `sync/synchronizer.go` + test |
| Local `.mg` | **0** | Selection logic lives in `internal/core/defaults/` |
| Embedded atom YAML | **hundreds** | Under `atoms/` multi-tree |
| Package README | **1** | `internal/prompt/README.md` (strong) |

### Largest non-test sources (lines ≈)

| Path | Lines | Role |
|------|------:|------|
| `compiler.go` | ~1185 | `JITPromptCompiler`, `Compile`, stats, injection |
| `selector.go` | ~1175 | Skeleton/flesh selection, Mangle fact build |
| `loader.go` | ~907 | YAML parse, SQLite schema, store atoms |
| `budget.go` | ~897 | Category budgets, `Fit`, polymorphism |
| `predicate_selector.go` | ~724 | Predicate corpus subset for prompts |
| `context.go` | ~701 | `CompilationContext`, hash, facts |
| `atoms.go` | ~684 | Categories, `PromptAtom`, matching |
| `assembler.go` | ~634 | Final assembly, templates, order |
| `compiler_db.go` | ~419 | DB load, knowledge/learning atoms, cache clear |
| `resolver.go` | ~410 | Topological order, cycles |
| `loader_embedding.go` | ~355 | Embedding-aware load paths |
| `config_factory.go` | ~352 | Agent runtime config from intents |
| `embedded.go` | ~227 | `go:embed` corpus load |
| `evolved_atoms.go` | ~209 | SPL evolved atom manager |
| `sync/synchronizer.go` | ~193 | Agent YAML → knowledge DB |
| `compiler_specialists.go` | ~179 | Specialist injection helpers |
| `vector_searcher.go` | ~178 | Cosine search over DB embeddings |
| `query_expansion.go` | ~154 | Query expansion for semantic search |
| `default_corpus.go` | ~146 | Materialize baked corpus DB |
| `baseline.go` | ~114 | Non-Mangle mandatory baseline |
| `compiler_options.go` | ~73 | Functional options |
| `config_defaults.go` | ~90 | Registry default ConfigAtoms |
| `output_mode.go` | ~44 | Structured-output-only filter |
| `manifest.go` | ~37 | Flight recorder types |
| `config_registry.go` | ~40 | `SimpleRegistry` |

## Atom category tree (on disk)

```
internal/prompt/atoms/
  identity/          # coder, tester, reviewer, legislator, …
  protocol/          # piggyback, reasoning_trace, …
  safety/            # constitution(al)
  methodology/       # tdd, ooda, debugging, dream, refactoring
  capability/        # codedom, tools, knowledge_discovery
  hallucination/     # per-role anti-hallucination
  language/          # go, python, ts, rust, java, mangle (+ deep subtrees)
  framework/         # bubbletea, gin, react, sqlite, …
  domain/            # project_context, style
  campaign/          # planner, assault, librarian, taxonomy, …
  init/              # init phases + kb_*
  northstar/         # vision alignment atoms
  ouroboros/         # tool-generation stages
  autopoiesis/       # meta atom generator
  context/           # file/session/error/symbol context
  exemplar/          # few-shot
  reviewer/          # review methodologies
  eval/              # judge
  knowledge/         # extraction/retrieval
  build_layer/       # scaffold → integration
  intent/            # create, fix, refactor, …
  world_state/       # diagnostics, security, …
  system/            # executive, perception, tool_nudge, …
  shards/            # coder/tester/reviewer/… shard-specific
  mangle/            # large encyclopedic Mangle teaching corpus
  perception/        # transducer understanding
```

## Component status (living code)

| Component | Status | Notes |
|-----------|--------|-------|
| `JITPromptCompiler.Compile` | **Implemented** | Cache + singleflight + full pipeline |
| Embedded corpus | **Implemented** | `go:embed atoms` |
| Project / agent DBs | **Implemented** | Register + load + vector search |
| Skeleton/flesh selector | **Implemented** | Kernel required for skeleton |
| Resolver | **Implemented** | Kahn topo + cycle break |
| Budget Fit | **Implemented** | Priority categories + modes |
| Assembler + templates | **Implemented** | Prefix-cache-friendly order |
| ConfigFactory | **Implemented** | Default + registry providers |
| Baseline path | **Implemented** | No kernel/vector |
| Evolved atoms | **Implemented** | Disk pending/promoted |
| Knowledge/learning bridge | **Implemented** | Optional stores on compiler |
| PredicateSelector | **Implemented** | Separate from atom JIT |
| AgentSynchronizer | **Implemented** | Boot sync agents |
| Default corpus materialize | **Implemented** | Baked DB seed |

**Overall:** production package — **not** pre-implementation.

## Hotspots

1. **`Compile` hot path** — concurrent collect, selection, budget, assemble; cache key quality matters.  
2. **Selector Mangle coupling** — depends on `jit_compiler.mg` predicates remaining consistent with asserted facts.  
3. **Atom corpus size** — embedded load cost at process start; DB limit 10k atoms per query.  
4. **ConfigAtom dual definitions** — `config_factory.go` defaults vs `config_defaults.go` registry lists.  
5. **Mangle mandatory override** — legislator/mangle_repair + language mangle special path in `selector.go`.

## Tests present (by name pattern)

- `compiler_*_test.go`, `selector_*_test.go`, `budget_test.go`, `resolver_*_test.go`  
- `assembler_*_test.go`, `atoms_*_test.go`, `context_*_test.go`  
- `loader_*_test.go`, `config_*_test.go`, `embedded_test.go`  
- Gap/boundary suites: `*_gaps_test.go`, `compiler_boundary_test.go`  
- E2E: `tests/e2e/prompt_compiler_*`, `promptcompiler_llmclient_*`, session/orchestrator integrations
