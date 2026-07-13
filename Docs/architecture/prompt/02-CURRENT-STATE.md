# Prompt JIT current state

> Evidence snapshot: commit `cfc537e96495e1fbccd7efff8bb8e4001c93ca9c`,
> post-repair dirty-tree fingerprint is recorded in [_progress](_progress.md),
> inspected 2026-07-13. The shared worktree contains unrelated concurrent changes;
> current claims below were rechecked against the named symbols and focused gates.

## Executive summary

`VERIFIED CURRENT` — `internal/prompt` is a live production package, not a future
design. It embeds and loads 888 prompt atoms, collects optional database/evolved/
knowledge candidates, asks Mangle for a deterministic skeleton plus filtered
flesh, orders dependencies, fits a token budget, renders templates, and can create
an effective runtime configuration. The clean session executor, articulation
assembler, system shards, initialization flows, CLI chat, and tests consume the
package.

`VERIFIED CURRENT` — three formerly open truth defects are closed: cache identity
is field-complete and versioned, the production adapter evaluates selector facts
inside disposable cloned kernels, and authoring/embedded/filesystem/sync validation
share one strict schema with ordered 888-atom parity. `PARTIAL` — an external
`KernelQuerier` that implements neither scope creation nor retraction still uses
an unisolated compatibility path, and durable decision receipts remain future work.

## Scale and evidence

| Surface | Current measurement | Evidence |
|---|---:|---|
| Root non-test Go | 25 files | `artifact:internal/prompt` directory scan on 2026-07-13 |
| Root Go tests | 34 files / 231 listed tests | `go test -list '^Test' ./internal/prompt` on 2026-07-13 |
| Sync subpackage | 1 source + 1 test | `internal/prompt/sync/synchronizer.go#AgentSynchronizer.SyncAll` |
| Embedded YAML | 333 files | `artifact:internal/prompt/atoms` scan |
| Runtime atoms | 888 | `internal/prompt/embedded_test.go#TestLoadEmbeddedCorpus` with verbose receipt |
| Local Mangle source | 0 files | The coupled declarations/policy live under `internal/core/defaults` |
| Focused package verification | PASS: prompt 2.563s; sync 0.182s; validator tests 0.722s | `go test -count=1 -timeout=240s ./internal/prompt ./internal/prompt/sync ./cmd/tools/validate_prompt_atoms` |
| Strict atom validation | PASS: 333 files, 888 atoms, 0 issues | `go run ./cmd/tools/validate_prompt_atoms -root internal/prompt/atoms -fail-on-warn` |

## Significant source inventory

Line counts are orientation only; symbol/test evidence carries the behavioral
claim.

| File | Lines | Current responsibility | Status |
|---|---:|---|---|
| `compiler.go` | 1268 | Compiler, scoped-kernel acquisition, result/stats, LRU, singleflight, candidate pipeline, manifest, close | Working; production selection is scope-isolated |
| `selector.go` | 1238 | Skeleton/flesh selection, Mangle fact projection, vector fallback, dedupe | Working against the per-compile kernel supplied by compiler |
| `atom_schema.go` | 700 | Versioned strict schema, bounded migrations, typed vocabulary, canonical parse adapters | Working; shared by validator/runtime paths |
| `loader.go` | 745 | Shared strict YAML adapter, schema creation, transactional SQLite replace/store, agent/project load | Working; invalid documents fail whole-file |
| `budget.go` | 897 | Category allocation, standard/concise/min fit, hard caps and reports | Working and bounded |
| `predicate_selector.go` | 724 | Separate predicate-corpus selection/formatting | Working; consumer story is separate from atom JIT |
| `context.go` | 731 | Context dimensions, world states, validation, facts, versioned cache identity | Working; set-like fields canonicalized without caller mutation |
| `atoms.go` | 684 | 23 categories, atom model, selector matching, facts, validation, embedded corpus | Working |
| `assembler.go` | 634 | Ordered prompt assembly, templates, options, UTF-8-safe truncation, stats | Working |
| `compiler_db.go` | 419 | DB atom load/registration, cache clear, knowledge/learning bridge | Working; best-effort optional sources |
| `resolver.go` | 410 | Dependency validation, cycle handling, deterministic category order | Working |
| `loader_embedding.go` | 355 | Embedded-to-SQLite synchronization and embeddings | Working |
| `config_factory.go` | 352 | Live default intent-to-tool/policy config provider and config generation | Production path |
| `embedded.go` | 126 | `go:embed`, strict shared parse, duplicate/set validation | Working; fails the entire load on invalid or migration-requiring built-ins |
| `evolved_atoms.go` | 209 | Pending/promoted atom file manager and cache invalidation | Working; promotion policy outside package |
| `compiler_specialists.go` | 179 | Specialist registry discovery and short-lived cache | Working |
| `vector_searcher.go` | 178 | Embed query and scan registered atom DB embeddings | Working; linear DB scan |
| `query_expansion.go` | 154 | Derive a semantic query from context | Working |
| `default_corpus.go` | 146 | Materialize baked prompt DB and hydrate selector tags | Working |
| `baseline.go` | 114 | Embedded mandatory/context match without Mangle/vector | Working fallback helper |
| `config_defaults.go` | 88 | Alternate `SimpleRegistry` catalog | Test-only/dormant; drifted names |
| `compiler_options.go` | 72 | Constructor options | Working |
| `output_mode.go` | 43 | Strip Piggyback/reasoning atoms for structured-output shards | Working |
| `manifest.go` | 36 | In-memory selected/dropped decision data | Working but not durable/correlated |
| `config_registry.go` | 31 | Simple config registry | Test-only/dormant |
| `sync/synchronizer.go` | 196 | Discover per-agent prompts, hash, strict parse, transactional replace/prune DB atoms | Working at boot; invalid agents are not registered |

## Current compilation behavior

1. `internal/prompt/compiler.go#JITPromptCompiler.Compile` rejects nil or invalid
   contexts and calculates `internal/prompt/context.go#CompilationContext.Hash`.
2. An LRU hit returns the stored `*CompilationResult`; a miss enters a same-key
   `singleflight` group.
3. `internal/prompt/compiler.go#acquireCompilationKernel` asks a
   `KernelScopeProvider` for a private evaluator. Production
   `internal/system/factory_adapters.go#KernelAdapter.NewCompilationScope` clones
   the live `RealKernel`; scope close discards all compilation facts on every exit.
4. An errgroup collects the static sources plus kernel-injected, semantic
   knowledge, and learning atoms. Optional-source failures are logged and degrade.
5. `internal/prompt/selector.go#AtomSelector.SelectAtomsWithTiming` concurrently
   loads skeleton and flesh. Skeleton requires a kernel and fails hard; flesh may
   fall back to direct `PromptAtom.MatchesContext`.
6. The selector asserts `current_context`, `prompt_atom`, `atom_tag`, dependency,
   conflict, and vector facts into that private kernel. No compile ID is required
   because the entire evaluator is compilation-owned and disposable.
7. Resolver, budget manager, and assembler produce the final text and manifest.
   If attached, ConfigFactory adds a runtime config. The result is stored in the
   atomic last-result slot and LRU.

`VERIFIED CURRENT` tests include
`internal/prompt/compiler_test.go#TestCompileEndToEnd`,
`internal/prompt/selector_test.go#TestAtomSelector_SelectAtoms_Bifurcation`,
`internal/prompt/resolver_test.go#TestDependencyResolver_ResolveWithDependencies`,
`internal/prompt/budget_test.go#TestTokenBudgetManager_FitMandatory`, and
`internal/prompt/assembler_test.go#TestFinalAssembler_AssembleWithTemplates`.

## Atom lifecycle

### Built-in source

`internal/prompt/embedded.go#LoadEmbeddedCorpus` walks 333 embedded YAML files and
delegates to `internal/prompt/atom_schema.go#ParsePromptAtomYAML`. Unknown fields,
invalid sequence members, duplicate IDs, or compatibility migrations fail the
complete built-in load. The loaded corpus is a map plus category index in
`internal/prompt/atoms.go#EmbeddedCorpus`.

### Project and agent sources

`internal/system/factory.go#initIntelligenceLayer` materializes
`.nerd/prompts/corpus.db`, hydrates context tags, and attaches it as project DB.
`internal/prompt/sync/synchronizer.go#AgentSynchronizer.SyncAll` discovers agent
prompt YAML, replaces changed rows, and prunes removed rows in per-agent knowledge
DBs. `internal/system/factory.go#initShardManagement` registers/unregisters those
DBs with the compiler as shard lifecycle changes.

### Evolved and recalled sources

`internal/prompt/evolved_atoms.go#EvolvedAtomManager` reads promoted/pending atom
files. `internal/prompt/compiler_db.go#JITPromptCompiler.collectKnowledgeAtoms`
and `#JITPromptCompiler.collectLearningAtoms` add optional retrieval results under
bounded timeouts. These sources enrich prompt flesh; they are not constitutional
authorities.

## Logic contract

`VERIFIED CURRENT` declarations live in
`internal/core/defaults/schemas_prompts.mg#prompt_atom/5` and adjacent predicates.
Go currently produces two context dialects:

- `compile_context(Dimension, Value)` from
  `internal/prompt/context.go#CompilationContext.ToContextFacts`;
- `current_context(Dimension, Tag)` plus candidate facts from
  `internal/prompt/selector.go#AtomSelector.buildContextFacts`.

`internal/core/defaults/policy/jit_selection.mg#selected_atom/1` derives mandatory
or eligible candidates after prohibition/conflict rules. Go then re-resolves
dependencies and budgets content. The boundary is intentionally hybrid: Mangle
determines logical eligibility; Go handles text, retrieval, ordering, and bounded
assembly.

## Wiring and user-visible behavior

| Producer/consumer | Live seam | What the user experiences |
|---|---|---|
| Boot | `internal/system/factory.go#initIntelligenceLayer` | Built-ins and local agent prompt atoms are available after startup |
| Clean session | `internal/session/executor.go#Executor.ProcessWithIntent` | Intent-specific prompt and tools reach the model |
| No-tool recovery | `internal/session/executor_tools.go#Executor.retryWithNoToolNudge` | Intended one-time direct-tool correction; currently cache-colliding |
| Articulation/system shards | `internal/articulation/prompt_assembler.go#PromptAssembler.AssembleSystemPrompt` | Shard/system prompts can use JIT with configured fallback |
| Init | `internal/init/jit_integration.go#Initializer.assembleJITPrompt` | Initialization phases can compile their own context |
| Inspector | `cmd/nerd/ui/jit_page.go#JITPageModel.UpdateContent` | Operator can filter/copy included atoms and full prompt |

## Permission and trust boundaries

Prompt atoms are untrusted instructions relative to the executive. A safety atom
may improve model behavior, but authorization occurs later in
`internal/session/executor_tools.go#Executor.checkSafety`, which queries the
kernel's constitutional gate and fails closed when the kernel is absent and the
gate is enabled. `ConfigFactory` describes an allowed capability surface; tool
lookup and permission still occur outside this package.

Trust-boundary risks that remain:

- third-party kernel adapters without scope or retraction support cannot guarantee
  compile-fact isolation;
- template expansion injects dynamic strings such as specialist/tool names into
  model-visible text;
- manifests/logs contain IDs and selection metadata and need explicit retention/
  redaction before becoming durable receipts;
- optional database/evolved content must never be able to displace constitutional
  skeleton solely through vector similarity.

## State, concurrency, recovery, and operations

| Concern | Current owner/behavior | Honest limit |
|---|---|---|
| Compiler cache | `JITPromptCompiler.cache`, mutex + 100-entry LRU | `CacheTTLSeconds` is configured but not enforced; cache results remain shared read-only pointers |
| Same-key fan-in | `JITPromptCompiler.compileGroup` | Identical versioned contexts join; mixed keys receive distinct cloned production kernels |
| DB lifecycle | `dbMu`, `shardMu`, `RegisterDB`, `RegisterShardDB`, `Close` | Shared DB scan/vector cost has no persistent per-call receipt |
| Last result | atomic pointer | One process-local snapshot, overwritten by next compile |
| Skeleton failure | returns critical selection error | Clean executor replaces it with generic baseline, losing JIT-specific guidance |
| Flesh/vector failure | warn, context fallback or skeleton-only | Degradation reason is logged but not carried to the user-visible receipt |
| Cancellation | context reaches collection/search/DB operations; deferred scope close discards selector facts | Third-party unscoped adapters retain compatibility-only semantics |
| Atom sync | content hash avoids unchanged rewrites; strict parse precedes transactional replace; removed atoms are pruned | Hash metadata write failure is non-fatal and causes a safe re-sync next boot |

## Test reality

Strengths:

- package tests cover all major components and many malformed/extreme inputs;
- race-oriented tests exist for compiler, selector, budget, assembler, and config;
- integration-tag tests cover session/compiler and LLM client boundaries;
- embedded runtime load and per-agent sync are directly exercised.

Gaps:

- no `Fuzz*` entry exists under `internal/prompt`;
- full prompt/sync and focused production adapter scope/cache gates pass under
  `-race` after the shared test kernel was made concurrency-safe;
- several files named `*_gaps_test.go` log or tolerate known failure behavior,
  so their presence is not proof that each named gap is closed;
- no focused failure/panic fixture yet proves cleanup for every third-party adapter
  shape; production scope close after selector panic is covered in
  `internal/prompt/compiler_scope_test.go#TestJITPromptCompiler_CompilationScopeClosesAfterSelectorPanic`.

## Current limitations to carry forward

1. G4 durable correlated prompt decision receipt.
2. G5 dormant ConfigAtom capability catalog divergence.
3. G6 unused cache TTL and shared cached-result pointer contracts.
4. G7 fuzz coverage and external-adapter isolation policy.
5. G8 shadow/replay evidence before automated prompt evolution.
