# prompt — Internal Architecture

> Last verified: **2026-07-13**

## Component diagram

```
                    ┌──────────────────────────┐
                    │   CompilationContext     │
                    │   (context.go)           │
                    └────────────┬─────────────┘
                                 │
                    ┌────────────▼─────────────┐
                    │   JITPromptCompiler      │
                    │   (compiler.go)          │
                    │  cache │ singleflight    │
                    └───┬────┴────┬────────────┘
          collect       │         │ select/resolve/fit/assemble
    ┌───────────────────┼─────────┼───────────────────────────┐
    │                   │         │                           │
    ▼                   ▼         ▼                           ▼
 EmbeddedCorpus    AtomLoader  AtomSelector              ConfigFactory
 projectDB/shardDB  SQLite     skeleton│flesh            ConfigAtoms
 EvolvedAtoms                  KernelQuerier
 Kernel inject                 VectorSearcher
 Knowledge/Learn                   │
                                   ▼
                            DependencyResolver
                                   │
                                   ▼
                           TokenBudgetManager
                                   │
                                   ▼
                            FinalAssembler
                            TemplateEngine
                                   │
                                   ▼
                          CompilationResult
```

## Data flow (one compile)

1. **Ingress** — Session/shard builds `CompilationContext` from intent + world + persona.  
2. **Normalize** — Validate budget/reserved.  
3. **Memoize** — Hash → LRU; singleflight coalesces concurrent identical compiles.  
4. **Context assert** — `compile_context` facts for Mangle policy.  
5. **Collect** — Parallel source gather → `[]*PromptAtom`.  
6. **Select** — Parallel skeleton/flesh → `[]*ScoredAtom`.  
7. **Resolve** — Dependency order → `[]*OrderedAtom`.  
8. **Fit** — Category budgets + modes → fitted ordered atoms.  
9. **Assemble** — Category order + templates → string.  
10. **Configure** — ConfigFactory → runtime config.  
11. **Egress** — Result to session LLM call; retract context facts.

## Key state machines

### Compiler lifecycle state

```
New → (options applied) → Ready
Ready → Compile* (concurrent)
Compile → Retract compile_context (defer)
Ready → RegisterDB / RegisterAgentDB / ClearCache
Ready → Close wait via WaitGroup on in-flight
```

### Atom selection state (per atom)

```
Candidate → ContextMatch? → Scored → Ordered → Fitted(mode) → Assembled
                 │                │         │
                 drop             drop      drop (budget)
```

### Render mode state

```
standard ──(budget fail)──► concise ──(fail)──► min ──(fail)──► unselected
```

## Shared mutable state

| State | Protection |
|-------|------------|
| `cache` / `cacheList` | `cacheMu` |
| `projectDB` / `embeddedCorpus` | `dbMu` RLock snapshot |
| `shardDBs` | `shardMu` |
| `config` | `configMu` |
| `lastResult` | `atomic.Pointer` |
| `compileGroup` | singleflight |
| `TokenBudgetManager.budgets` | internal `mu` |
| `FinalAssembler` order/seps | internal `mu` |
| `EvolvedAtomManager.atoms` | internal `mu` |

## Interfaces (ports)

| Port | Methods | Implementors |
|------|---------|--------------|
| `KernelQuerier` | Query, AssertBatch | base kernel adapter and private compilation scopes |
| `KernelScopeProvider` | NewCompilationScope | production `system.KernelAdapter` via cloned `RealKernel` |
| `KernelCompilationScope` | Query, AssertBatch, Close | production per-compile adapter |
| `KernelRetracter` | Retract | compatibility cleanup; not sufficient alone for concurrent isolation |
| `VectorSearcher` | Search, EmbedQuery | `CompilerVectorSearcher` |
| `ConfigAtomProvider` | GetAtom | Default provider, SimpleRegistry |
| `AtomStore` | (interface in atoms.go) | EmbeddedCorpus etc. |

## Category pipeline vs selector skeleton

Important distinction:

- **Selector skeleton set**: identity, protocol, safety, methodology only.  
- **Assembler order / budget priorities**: broader set including hallucination, capability, etc. as non-skeleton flesh or high-priority budget.

Hallucination is **not** a skeleton category in `selector.go` but is high priority in budget and early in assembly order.

## Mangle surface (external but coupled)

Go asserts → Kernel evaluates → Go queries `selected_result`.

Primary files outside package:

- `internal/core/defaults/jit_compiler.mg`  
- `internal/core/defaults/policy/jit_logic.mg`  
- `internal/core/defaults/policy/jit_selection.mg`  
- `internal/core/defaults/schemas_prompts.mg`

## Subpackage: sync

`AgentSynchronizer` discovers `.nerd/agents/*/prompts.yaml`, parses through the
shared strict schema, transactionally replaces knowledge DB atoms under the
shards/agents dirs, and registers only successful agents. Boot attaches DBs to the
compiler through registrar functions in `compiler_db.go`.
