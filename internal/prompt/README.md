# Prompt System (JIT Compiler)

`internal/prompt` implements codeNERD’s JIT Prompt Compiler: prompts are assembled at runtime from **prompt atoms** based on the current `CompilationContext` plus kernel/world state.

This directory is the boundary between the deterministic executive (Mangle kernel + world model) and the creative center (LLM). Every shard LLM call should flow through this pipeline.

## Atom sources (where prompt text comes from)

The compiler can draw candidate atoms from three places:

1. **Embedded corpus (built-ins)**
   - Source: `internal/prompt/atoms/**/*.yaml`
   - Loaded via: `prompt.LoadEmbeddedCorpus()` (see `internal/prompt/embedded.go`)
   - Always available (no runtime filesystem dependency).

2. **Project corpus DB (shared / project-scoped)**
   - Convention: `.nerd/prompts/corpus.db`
   - Registered via: `compiler.RegisterDB("corpus", ".../corpus.db")` or `prompt.WithProjectDB(db)`
   - Used for: project-level atoms + semantic search over atoms with embeddings.
   - Boot seeding: when available, the baked default corpus (`internal/core/defaults/prompt_corpus.db`, embedded into the binary) is materialized to this path via `prompt.MaterializeDefaultPromptCorpus(...)`.
   - Boot ingestion: hybrid `.mg` files can emit `PROMPT:` directives which get ingested into the corpus (see `internal/system/factory.go` and `internal/core/hybrid_loader.go`).

3. **Agent / shard DBs (agent-scoped)**
   - Convention: `.nerd/shards/{agent}_knowledge.db`
   - YAML source: `.nerd/agents/{agent}/prompts.yaml`
   - Synced via:
     - `prompt.LoadAgentPrompts(...)` (see `internal/prompt/loader.go`), or
     - `internal/prompt/sync.AgentSynchronizer` (boot sync).
   - Registered via: `compiler.RegisterShardDB(agent, db)` or `prompt.RegisterAgentDBWithJIT(...)`.

## Atom format (YAML)

Atoms are defined as YAML objects (or a YAML list) with fields like:

- `id` (string): globally-unique atom ID (e.g. `protocol/piggyback/envelope`)
- `category` (string): one of `identity|protocol|safety|methodology|...` (see `internal/prompt/atoms.go`)
- `content` (string, markdown): the prompt text (or `content_file` for file-backed content)
- Optional composition: `priority`, `is_mandatory`, `is_exclusive`, `depends_on`, `conflicts_with`
- Optional selectors: `operational_modes`, `campaign_phases`, `build_layers`, `init_phases`, `northstar_phases`, `ouroboros_stages`, `intent_verbs`, `shard_types`, `languages`, `frameworks`, `world_states`
- Optional polymorphism: `description`, `content_concise`, `content_min`

See any file under `internal/prompt/atoms/` for examples.

## Persistence schema (SQLite)

When atoms are stored in SQLite (project corpus and/or agent DBs), `AtomLoader.EnsureSchema()` is the source of truth (see `internal/prompt/loader.go`). At a high level:

- `prompt_atoms`: base atom data + optional embedding
- `atom_context_tags`: normalized selector tags (`dimension`, `tag`), plus reserved dimensions for `depends_on` and `conflicts_with`

Vector search reads embeddings from `prompt_atoms.embedding` across all registered DBs (see `internal/prompt/vector_searcher.go`).

## Compilation pipeline (runtime)

At a high level, `JITPromptCompiler.Compile()` (`internal/prompt/compiler.go`) does:

1. **Collect candidates** from embedded corpus + project DB + shard DB (+ optional kernel-injected ephemeral atoms like `injectable_context`/`specialist_knowledge`).
2. **Assert context to kernel** as `compile_context/2` facts (see `CompilationContext.ToContextFacts()` in `internal/prompt/context.go`).
3. **Select atoms** via `AtomSelector` (`internal/prompt/selector.go`) using a System-2 split:
   - **Skeleton (deterministic, required)**: `identity`, `protocol`, `safety`, `methodology` selected via kernel rules.
   - **Flesh (probabilistic, degradable)**: other categories selected via vector search (if available) + kernel filter.
4. **Resolve dependencies/conflicts** (`internal/prompt/resolver.go`).
5. **Fit token budget** with polymorphic rendering (`internal/prompt/budget.go`).
6. **Assemble final prompt** (`internal/prompt/assembler.go`).

For debugging/observability, the compiler emits a `PromptManifest` “flight recorder” + `CompilationStats` (see `internal/prompt/manifest.go` and `internal/prompt/compiler.go`).

## Enabling semantic retrieval (embeddings)

Vector search operates over SQLite embeddings, not the in-memory embedded corpus. To enable semantic search over built-in atoms, sync them into the project corpus DB with embeddings:

- `prompt.SyncEmbeddedToSQLite(ctx, ".nerd/prompts/corpus.db", embeddingEngine)`

This is typically wired during boot in interactive flows (see `cmd/nerd/chat/session.go`).

## Where to add new prompt behavior

- **Core/system behavior**: add new atoms under `internal/prompt/atoms/<category>/...` (they get embedded into the binary).
- **Project/user behavior**: add atoms to `.nerd/agents/<agent>/prompts.yaml` and sync into `.nerd/shards/<agent>_knowledge.db`.
- **Hybrid Mangle files**: add `PROMPT:` directives to `.mg` hybrid files when you want “one file” that contains both logic and prompt atoms; these are ingested into `.nerd/prompts/corpus.db` at boot.

## Useful cross-refs

- `internal/prompt/compiler.go` (JIT compilation entrypoint)
- `internal/prompt/atoms/` (canonical built-in atom library)
- `internal/prompt/loader.go` + `internal/prompt/sync/` (YAML -> SQLite ingestion + sync)
- `internal/prompt/context.go` (selection dimensions)
- `internal/articulation/emitter.go` (Piggyback protocol integration)
