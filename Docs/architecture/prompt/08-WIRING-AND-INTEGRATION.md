# prompt — Wiring and Integration

> Last verified: **2026-07-13**

## Boot path (campaign / interactive)

Evidence: `cmd/nerd/cmd_campaign.go`, chat session boot patterns.

```
1. LoadEmbeddedCorpus()
2. DefaultCompilerConfig()  (+ budget from user config when available)
3. NewJITPromptCompiler(
     WithKernel(KernelAdapter),
     WithEmbeddedCorpus(...),
     WithConfig(...),
     optionally WithVectorSearcher / WithProjectDB / WithConfigFactory
   )
4. NewDefaultConfigFactory()
5. shardMgr.SetJITRegistrar(CreateJITDBRegistrar(compiler))
6. shardMgr.SetJITUnregistrar(CreateJITDBUnregistrar(compiler))
7. Pass compiler + factory into session Executor / Spawner
```

Agent sync (when used):

```
AtomLoader → AgentSynchronizer.SyncAll → register agent knowledge DBs
```

Optional:

```
MaterializeDefaultPromptCorpus → .nerd/prompts/corpus.db
SyncEmbeddedToSQLite → embeddings for vector flesh
```

## Session Executor wiring

`internal/session/executor.go`:

1. Interfaces: `JITCompiler`, `ConfigFactory` (session-local ports wrapping prompt types).  
2. On task: `buildCompilationContext(ctx, intent)` fills mode, language, shard, budget, semantic query, world flags, tools, etc.  
3. `jitCompiler.Compile(ctx, compilationCtx)` — **required** when non-nil (nil path is failure/fallback depending on version).  
4. Config: factory `Generate` with intent verb; tools enforced in tool loop (`isToolAllowed`).  

Spawner (`spawner.go`): Compile for agent persona; may retry with baseline context; config generation for named agents.

## Articulation bridge

`internal/articulation/prompt_assembler.go`:

- Holds optional `*prompt.JITPromptCompiler`.  
- `NewPromptAssemblerWithJIT`, `SetJITCompiler`, `GetJITCompiler`.  
- When JIT present, `Compile` supplies system prompt path; otherwise legacy/baseline assembly.

## System shards

Shards call JIT with persona-specific contexts and **fail closed** if atoms missing (examples from comments/errors):

| Shard | Atom anchors |
|-------|----------------|
| Planner | `atoms/campaign/planner.yaml` |
| Legislator | `identity/legislator.yaml`, `system/legislator.yaml` |
| Mangle repair | mangle language + structured output mode |
| Router / world_model / perception | `system/autopoiesis.yaml`, `system/perception.yaml` |

Registration may pull prompt package for DB registrar hooks (`internal/shards/registration.go`).

## Hybrid Mangle PROMPT: directives

Boot/hybrid loader can ingest `PROMPT:` blocks from hybrid `.mg` into project corpus (see package README + `internal/core/hybrid_loader.go` / system factory). Not implemented inside `selector.go`; ingestion is boot concern.

## UI glass box

`cmd/nerd/ui/jit_page.go` consumes `CompilationResult` / `PromptAtom` for operator visibility of selected atoms.

## E2E wiring

| Test area | Proves |
|-----------|--------|
| `prompt_compiler_llm_*` | Compile → LLM boundary |
| `jit_kernel_context_cleanup_*` | compile_context retract |
| `session_clean_loop_*` | Executor JIT loop |
| `specialist_config_boundary_*` | ConfigFactory consult fallback |
| `piggyback_executor_full_boundary_*` | Protocol atoms + execution |

## Wiring gaps (audit targets)

1. PredicateSelector call sites vs atom JIT — separate product surface.  
2. Whether all interactive boots attach vector searcher + project DB.  
3. EvolvedAtomManager attachment frequency on compiler in production boot.  
4. Dual ConfigAtom registries — which factory path each boot uses.

## Registration helpers

`compiler_db.go` exposes helpers to open/register SQLite paths and clear cache on mutation so shard lifecycle can hot-attach agent knowledge without reconstructing the compiler.
