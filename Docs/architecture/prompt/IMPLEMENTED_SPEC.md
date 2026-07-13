# prompt — Implemented Spec (Deep-Dive)

> Last verified against codebase: **2026-07-13**  
> Status: Living Reference Document  
> Language: Go  
> Primary sources: `internal/prompt/`  
> Scale: **~25** non-test Go files; **~32** test files; **0** package-local `.mg`; **hundreds** of embedded atom YAMLs under `atoms/`  
> Mangle selection rules (external): `internal/core/defaults/jit_compiler.mg`, `policy/jit_logic.mg`, `policy/jit_selection.mg`, `schemas_prompts.mg`

## 1. Overview

`internal/prompt` is codeNERD’s **JIT Prompt Compiler** and **ConfigFactory** surface. It is the production boundary between:

- the **executive** (Mangle kernel + world model + constitutional `permitted(...)`), and  
- the **creative center** (LLM system prompt + allowed tool surface).

Prompts are not static strings. They are **assembled at runtime** from **prompt atoms** using:

1. Multi-source candidate collection (embedded, project DB, agent DB, evolved, kernel-injected, knowledge/learning).  
2. Context assertion as `compile_context` facts.  
3. **Skeleton/flesh** selection (System-2 bifurcation).  
4. Dependency topological ordering.  
5. Token budget fitting with **polymorphic** content.  
6. Category-ordered assembly (prefix-cache friendly).  
7. Optional **EffectiveAgentRuntimeConfig** generation (tools + policies).

### Key characteristics

| Property | Value |
|----------|-------|
| Entry API | `JITPromptCompiler.Compile(ctx, *CompilationContext)` |
| Atom unit | `PromptAtom` (`atoms.go`) |
| Skeleton categories | identity, protocol, safety, methodology |
| Default token budget | 200_000 (override from config) |
| Vector weight default | 0.3 (70% logic / 30% vector in config) |
| Cache | LRU map+list, limit 100, keyed by `CompilationContext.Hash()` |
| Concurrency | `singleflight` per cache key; errgroup for collect; parallel skeleton/flesh |
| Logging | `logging.CategoryJIT`, `CategoryContext`, `CategoryStore` |
| Downstream consumers | `internal/session` Executor/Spawner, `internal/articulation`, system shards, `cmd/nerd` boot |

### High-level control flow

```
CompilationContext
        │
        ▼
┌───────────────────┐     cache HIT?
│ Compile(ctx, cc)  │───────────────► return *CompilationResult
└─────────┬─────────┘
          │ MISS + singleflight
          ▼
  Assert compile_context/*  ──defer Retract("compile_context")
          │
          ├─► collectAtoms (embedded + project + shard + evolved)
          ├─► collectKernelInjectedAtoms
          ├─► collectKnowledgeAtoms
          └─► collectLearningAtoms
          │
          ▼
  AtomSelector.SelectAtomsWithTiming
     ├ skeleton (Mangle only) — CRITICAL on fail
     └ flesh (vector + Mangle) — degrade OK
          │
          ▼
  DependencyResolver.Resolve  (topo order)
          │
          ▼
  TokenBudgetManager.Fit  (modes: standard|concise|min)
          │
          ▼
  FinalAssembler.Assemble (+ TemplateEngine)
          │
          ▼
  ConfigFactory.Generate → EffectiveAgentRuntimeConfig
          │
          ▼
  *CompilationResult { Prompt, IncludedAtoms, Manifest, Stats, Config }
```

Fact-flow placement:

```
user_intent → kernel next_action → VirtualStore / session
  → buildCompilationContext
  → JIT.Compile → LLM under AllowedTools + policies
```

---

## 2. Implementation status

| Component | Status | Notes |
|-----------|--------|-------|
| `JITPromptCompiler` | **Implemented** | Full pipeline + cache + stats |
| `PromptAtom` model + categories | **Implemented** | 20+ categories |
| Embedded corpus (`go:embed`) | **Implemented** | `embedded.go` |
| Project / agent SQLite atoms | **Implemented** | loader schema + compiler_db |
| Skeleton/flesh `AtomSelector` | **Implemented** | Requires kernel for skeleton |
| `DependencyResolver` | **Implemented** | Cycle break by highest score |
| `TokenBudgetManager` | **Implemented** | PriorityFirst default |
| `FinalAssembler` + templates | **Implemented** | Static head / dynamic tail |
| `CompilationContext` facts/hash | **Implemented** | `ToContextFacts`, `Hash` |
| `ConfigFactory` / ConfigAtoms | **Implemented** | Default + SimpleRegistry |
| `CompilerVectorSearcher` | **Implemented** | DB embedding cosine |
| `EvolvedAtomManager` | **Implemented** | SPL pending/promoted |
| Baseline non-JIT assembly | **Implemented** | `baseline.go` |
| `PredicateSelector` | **Implemented** | Predicate subset (parallel concern) |
| `AgentSynchronizer` | **Implemented** | `sync/` |
| Default corpus materialize | **Implemented** | `default_corpus.go` |
| Kernel policy selection rules | **Implemented (core)** | Not in this package |
| Full ConfigAtom ↔ tool registry parity | **Partial** | Dual lists can drift |
| Cache key completeness | **Partial** | See gaps |
| Agents.md package doc | **Missing** | Root Agents.md points to `internal/prompt/agents.md` but file may be absent — package README is the live doc |

**Overall:** living production JIT — **~90%** mature package with known partials.

---

## 3. Source inventory

### 3.1 Package layout

```
internal/prompt/
  compiler.go              # Compile pipeline, injection, result build
  compiler_db.go           # DB hydrate, knowledge/learning, cache clear, registrars
  compiler_options.go      # WithKernel, WithProjectDB, …
  compiler_specialists.go  # Specialist-related helpers
  selector.go              # AtomSelector skeleton/flesh
  resolver.go              # DependencyResolver
  budget.go                # TokenBudgetManager
  assembler.go             # FinalAssembler, TemplateEngine
  atoms.go                 # PromptAtom, categories, matching, ToFact
  context.go               # CompilationContext
  loader.go                # AtomLoader YAML→SQLite
  loader_embedding.go      # Embedding sync helpers
  embedded.go              # go:embed atoms
  baseline.go              # Mandatory-only baseline
  config_factory.go        # ConfigFactory + DefaultConfigAtomProvider
  config_defaults.go       # RegisterDefaultConfigAtoms
  config_registry.go       # SimpleRegistry
  vector_searcher.go       # CompilerVectorSearcher
  query_expansion.go       # Semantic query expansion
  evolved_atoms.go         # EvolvedAtomManager
  predicate_selector.go    # Predicate corpus selection
  output_mode.go           # Structured-output-only atom filter
  manifest.go              # PromptManifest types
  default_corpus.go        # MaterializeDefaultPromptCorpus
  atoms/                   # Canonical YAML atom library
  sync/                    # AgentSynchronizer
  README.md
```

### 3.2 Public type map (core)

| Type | File | Role |
|------|------|------|
| `JITPromptCompiler` | `compiler.go` | Orchestrator |
| `CompilerConfig` | `compiler.go` | Budgets, vector, cache, timeouts |
| `CompilationResult` | `compiler.go` | Prompt + stats + config |
| `CompilationStats` | `compiler.go` | Phase timings, counts, modes |
| `KernelQuerier` / `KernelRetracter` | `compiler.go` | Kernel ports |
| `VectorSearcher` / `SearchResult` | `compiler.go` | Semantic port |
| `PromptAtom` / `AtomCategory` | `atoms.go` | Atom model |
| `EmbeddedCorpus` / `AtomStore` | `atoms.go` | In-memory store |
| `CompilationContext` | `context.go` | Selection dimensions |
| `AtomSelector` / `ScoredAtom` | `selector.go` | Selection |
| `DependencyResolver` / `OrderedAtom` | `resolver.go` | Ordering |
| `TokenBudgetManager` / `CategoryBudget` | `budget.go` | Fitting |
| `FinalAssembler` / `TemplateEngine` | `assembler.go` | Assembly |
| `ConfigFactory` / `ConfigAtom` | `config_factory.go` | Tools/policies |
| `AtomLoader` | `loader.go` | Persistence |
| `PromptManifest` | `manifest.go` | Flight recorder |
| `EvolvedAtomManager` | `evolved_atoms.go` | SPL atoms |
| `CompilerVectorSearcher` | `vector_searcher.go` | Default searcher |
| `PredicateSelector` | `predicate_selector.go` | Predicate JIT |
| `AgentSynchronizer` | `sync/synchronizer.go` | Agent sync |

---

## 4. Deep dive — Compiler (`compiler.go`)

### 4.1 Construction

```go
NewJITPromptCompiler(opts ...CompilerOption) (*JITPromptCompiler, error)
```

Defaults via `DefaultCompilerConfig()`:

- `DefaultTokenBudget: 200000`  
- `EnableVectorSearch: true`  
- `VectorSearchWeight: 0.3`  
- `MaxAtomsPerCategory: 10`  
- `EnableCaching: true`  
- `CacheTTLSeconds: 300` (note: LRU size-limited; TTL field present in config)  
- `KnowledgeSearchTimeout: 10s`  

Subcomponents always created: `AtomSelector`, `DependencyResolver`, `TokenBudgetManager`, `FinalAssembler`, LRU cache (limit 100).

**Options** (`compiler_options.go`): `WithEmbeddedCorpus`, `WithProjectDB`, `WithKernel`, `WithVectorSearcher`, `WithConfig`, `WithDefaultTokenBudget`, `WithConfigFactory`.

`WithKernel` also wires `selector.SetKernel`. `WithVectorSearcher` wires selector searcher.

### 4.2 Compile algorithm (behavioral)

1. **Validate** context (`TokenBudget > 0`, reserved < budget).  
2. **Cache lookup** by `cc.Hash()` — HIT returns stored `*CompilationResult`.  
3. **singleflight.Do(cacheKey)** for MISS:  
   a. If kernel set: `AssertBatch(cc.ToContextFacts())`; if `KernelRetracter`, **defer** `Retract("compile_context")`.  
   b. **errgroup** parallel:  
      - `collectAtomsWithStats` — embedded + project DB + shard DB + evolved  
      - `collectKernelInjectedAtoms` — `injectable_context`, `specialist_knowledge`  
      - `collectKnowledgeAtoms` — LocalStore semantic bridge  
      - `collectLearningAtoms` — LearningStore  
   c. Merge candidates; fill stats source counts.  
   d. `selector.SelectAtomsWithTiming` → scored.  
   e. `resolver.Resolve` → ordered.  
   f. `budgetMgr.Fit(ordered, AvailableTokens)` → fitted with render modes.  
   g. `assembler.Assemble` → prompt string.  
   h. `buildResultWithStats` → manifest + counts.  
   i. Optional `configFactory.Generate` for `IntentVerb` + `ShardType`.  
   j. Store `lastResult` atomic; insert LRU cache.  
   k. Log stats (`CategoryJIT`); optional debug manifest dump.  
4. Return result (shared singleflight joins log).

### 4.3 Candidate collection sources

| Source | Condition | Notes |
|--------|-----------|-------|
| Embedded | `embeddedCorpus != nil` | Always first; in-memory |
| Project DB | `projectDB != nil` | Full hydrate via JOIN tags, LIMIT 10000 |
| Shard/agent DB | `cc.ShardID` in `shardDBs` | Per-agent knowledge |
| Evolved | `evolvedAtomMgr` | `.nerd/prompts/evolved/*` |
| Kernel inject | kernel Query | Mandatory context/knowledge blocks |
| Knowledge | `localDB` + embed | Timeout-bounded |
| Learning | `learningStore` | Semantic then lexical fallback |

### 4.4 Kernel-injected atoms

`collectKernelInjectedAtoms`:

- **`injectable_context(shard, atomText)`** → one `CategoryContext` atom, `IsMandatory`, priority 95, content bullet list.  
- **`specialist_knowledge(shard, topic, body)`** → one `CategoryKnowledge` atom, priority 90.  

Shard match: exact `ShardID`, `ShardInstanceID`, or `*` / `/_all`.

### 4.5 Result shape

`CompilationResult` carries:

- `Prompt`, `IncludedAtoms`  
- Token / category stats  
- `Manifest *PromptManifest`  
- `EffectiveAgentRuntimeConfig`  
- `Stats *CompilationStats` (phase ms, skeleton/flesh, render mode counts, cache flags)

---

## 5. Deep dive — Atoms (`atoms.go` + YAML)

### 5.1 Categories

Canonical constants include: identity, protocol, safety, methodology, capability, hallucination, language, framework, domain, campaign, init, northstar, ouroboros, autopoiesis, context, exemplar, reviewer, eval, knowledge, build_layer, intent, world_state, **system**.

`AllCategories()` enumerates for ordering helpers.

### 5.2 PromptAtom fields

**Identity:** `ID`, `Version`, `Content`, `TokenCount`, `ContentHash`, `Category`, `Subcategory`.

**Selectors (empty = match any for that dimension):**  
`OperationalModes`, `CampaignPhases`, `BuildLayers`, `InitPhases`, `NorthstarPhases`, `OuroborosStages`, `IntentVerbs`, `ShardTypes`, `Languages`, `Frameworks`, `WorldStates`.

**Composition:** `Priority`, `IsMandatory`, `IsExclusive`, `DependsOn`, `ConflictsWith`.

**Polymorphism / retrieval:** `Description`, `ContentConcise`, `ContentMin`, `Embedding`, `EmbeddingTask`.

`MatchesContext` requires **all non-empty selector dimensions** to match (AND across dimensions; frameworks/world_states use ANY within dimension).

### 5.3 Mangle fact projection

- `ToFact()` → `prompt_atom(ID, Category, Priority, TokenCount, IsMandatory)` — order matches `schemas_prompts.mg`  
- `ToSelectorFacts()` → `atom_selector(ID, Dimension, Value)`  
- `ToDependencyFacts()` / conflict facts similarly  

Selector path also builds string facts for batch assert (see `selector.go` `buildContextFacts`).

### 5.4 Token estimate

`EstimateTokens` ≈ `ceil(len/4)` heuristic. Used when `TokenCount` missing.

### 5.5 YAML source of truth

Embedded via `//go:embed atoms`. Schema fields mirrored in `embeddedYAMLAtom` and loader YAML types. Optional `content_file` for external content (loader path).

---

## 6. Deep dive — Selector (`selector.go`)

### 6.1 Skeleton vs flesh

```go
skeletonCategories = identity | protocol | safety | methodology
```

`SelectAtoms` / `SelectAtomsWithTiming`:

1. Filter structured-output-only shards (strip piggyback/reasoning protocol atoms for legislator & mangle_repair).  
2. Compute `selectMangleMandatoryIDs` when shard is legislator|mangle_repair **and** language is mangle — forces high-priority mangle mandatory atoms under token/atom caps.  
3. Parallel: `loadSkeletonAtoms` + `loadFleshAtoms`.  
4. Skeleton error → fail compile. Flesh error → warn, continue.  
5. `mergeAtoms` (skeleton wins on ID collision).

### 6.2 Skeleton load

- Requires `s.kernel != nil` else CRITICAL error.  
- Filter candidates to skeleton categories.  
- Build/assert facts; query `selected_result(Atom, Priority, Source)`.  
- Debug queries: `blocked_by_context`, `mandatory_selection`.  
- All selected get LogicScore/Combined = 1.0, Source=`skeleton`.  
- Missing category → warn (does not always hard-fail if any skeleton returned).

### 6.3 Flesh load

- Non-skeleton categories only.  
- Optional vector scores via `SemanticQuery` + `SemanticTopK` with timeout.  
- Assert facts + `vector_hit(ID, Score)` strings.  
- Mangle filter or `fallbackFleshSelection` (context match + scores) if no kernel.  
- Combined score weights logic vs vector (`vectorWeight` default 0.3).

### 6.4 Mangle rule file (external)

`internal/core/defaults/jit_compiler.mg` declares and defines:

- `blocked_by_context(Atom)`  
- `mandatory_selection(Atom)`  
- `selected_result(Atom, Priority, Source)` with `/skeleton` and `/flesh` sources  
- uses `vector_hit`, context blocks, exclusivity/dependency rules as implemented in policy modules  

Go must assert atoms/selectors/context consistent with these Decls.

---

## 7. Deep dive — Resolver (`resolver.go`)

`Resolve([]*ScoredAtom) ([]*OrderedAtom, error)`:

1. Drop nil/invalid.  
2. **Topological sort** (Kahn): edges from DependsOn where dep present.  
3. Zero in-degree queue ordered by **mandatory first**, then **Combined** score.  
4. Cycle: if queue empties early, pick remaining highest-score node, log warn, force progress.  
5. Emit `OrderedAtom{RenderMode: "standard"}`.  

Also: `ValidateDependencies`, `DetectCycles` (iterative DFS, stack cap 100000), `SortByCategory`.

**Note:** Conflict filtering is assumed upstream (Mangle); resolver primarily orders.

---

## 8. Deep dive — Budget (`budget.go`)

### 8.1 Defaults

`TokenBudgetManager` default strategy: **PriorityFirst**. Reserved headroom: **500** tokens (cleared if budget ≤250 and headroom not explicit).

Category priorities (illustrative):

| Priority | Categories (examples) |
|----------|----------------------|
| Mandatory | safety, identity |
| High | protocol, methodology, capability, hallucination |
| Medium | language, framework, domain, context, knowledge |
| Conditional | campaign, init, northstar, ouroboros, autopoiesis, build_layer, intent, world_state, eval |
| Low | exemplar, reviewer |

Each has `BasePercent`, `MinTokens`, `MaxTokens`, optional `CanExceedMax`.

### 8.2 Fit algorithm

`Fit(atoms, totalBudget)`:

1. Cap input at `maxAtomsInput` (100000) by truncation.  
2. Clone atoms; ensure TokenCount.  
3. Sort by category priority → score.  
4. Compute per-category allocations from present set.  
5. Per category chunk:  
   - **Mandatory**: include if fits **absolute totalBudget** (reject single atom > budget; skip if cumulative saturated).  
   - **Optional**: try **standard** → **concise** → **min** against category allocation; optional truncate helper.  
6. `fillRemaining` may place unselected into leftover budget.  
7. Cap included atoms at 5000.  
8. Set `RenderMode` on each included `OrderedAtom`.

### 8.3 Polymorphism

Assembler / budget select content variant:

| Mode | Field |
|------|-------|
| standard | `Content` |
| concise | `ContentConcise` if non-empty |
| min | `ContentMin` if non-empty |

---

## 9. Deep dive — Assembler (`assembler.go`)

### 9.1 Category order (prefix caching)

Static **head** first, dynamic **tail** last:

1. Identity → Safety → Protocol → Hallucination → Methodology → Capability  
2. Language → Framework → Domain → Knowledge  
3. Campaign → Northstar → Init → BuildLayer → Ouroboros → Autopoiesis  
4. Reviewer → Eval → Exemplar  
5. **Intent → WorldState → Context** (JIT working memory tail)

### 9.2 Assemble steps

Group by category → sort by resolver Order → section join → `TemplateEngine.Process(prompt, cc)`.

Optional section headers (default off). Inject available specialists if empty via `InjectAvailableSpecialists`.

### 9.3 Templates

`TemplateEngine` supports context-driven substitutions (e.g. available tools/specialists placeholders used by capability atoms). Functions registered for dynamic fragments.

---

## 10. Deep dive — Context (`context.go`)

### 10.1 Dimension tiers

Operational mode, campaign phase/id/name, build layer, init phase, northstar phase, ouroboros stage, intent verb/target, shard type/id/instance/name, world model flags/counts, language/frameworks, token budget/reserved, semantic query/topK, activation scores, AvailableSpecialists/Tools, PreviousAttemptNoToolCall.

### 10.2 WorldStates derivation

From flags: `failing_tests`, `diagnostics`, `large_refactor`, `security_issues`, `new_files`, `high_churn`, `reflection_hits`, `no_tool_call_retry`.

### 10.3 Facts & cache

- `ToContextFacts()` → `compile_context(Dimension, Value).` style batch  
- `Hash()` SHA-256 over major dimensions (mode, campaign, shard, language, frameworks, intent, phases, semantic query, budget, test/diag counts, refactor/security/reflection flags)  

**Not fully hashed (gap risk):** e.g. `AvailableTools`, `PreviousAttemptNoToolCall`, activation maps — can cause stale cache hits if those change alone.

---

## 11. ConfigFactory

### 11.1 ConfigAtom

`Tools []string`, `Policies []string`, `Priority int`; `Merge` dedupes tools/policies, max priority.

### 11.2 Generate

Given intents (session passes verb + shard type):

- Lookup provider atoms; merge found.  
- `/consult/*` falls back to `/general`.  
- Build `EffectiveAgentRuntimeConfig`: IdentityPrompt=compiled prompt, AllowedTools, Policies, ToolLoop defaults (MaxIterations 5, MaxTotalCalls 50), Safety RequirePolicyEnforcement true.

### 11.3 Providers

- **`DefaultConfigAtomProvider`**: rich in-package tool maps (read/search/CodeDOM/etc.).  
- **`RegisterDefaultConfigAtoms(SimpleRegistry)`**: coder/tester/reviewer/researcher with policy file lists (coder_*.mg suite).  
- **`NewDefaultConfigFactory()`**: convenience constructor.

Tool name strings must match VirtualStore/tool registration — **audit when adding tools**.

---

## 12. Persistence & sync

### 12.1 Schema (`AtomLoader.EnsureSchema`)

Tables:

- `prompt_atoms` — content, hash, tokens, category, composition, embedding BLOB, polymorphism columns  
- `atom_context_tags` — (atom_id, dimension, tag, is_exclusion)

### 12.2 Paths

| Artifact | Convention |
|----------|------------|
| Project corpus | `.nerd/prompts/corpus.db` |
| Baked seed | `internal/core/defaults/prompt_corpus.db` via `MaterializeDefaultPromptCorpus` |
| Agent YAML | `.nerd/agents/{agent}/prompts.yaml` |
| Agent DB | `.nerd/agents|{shards}/{agent}_knowledge.db` |
| Evolved | `.nerd/prompts/evolved/{pending,promoted}` |

### 12.3 Vector search

`CompilerVectorSearcher` embeds query (RETRIEVAL_QUERY when available), scans registered DBs’ embedding columns, cosine similarity, returns top atom IDs.

Embedded-only corpora lack vectors until `SyncEmbeddedToSQLite` (or builder with embeddings).

---

## 13. Baseline path (`baseline.go`)

`AssembleEmbeddedBaselinePrompt(cc)`:

- Load cached embedded corpus  
- Filter structured-output atoms  
- Select **mandatory** + `MatchesContext`  
- Resolve + Fit + Assemble **without** Mangle/vector  

Used when JIT/kernel unavailable (articulation legacy/fallback).

---

## 14. PredicateSelector (`predicate_selector.go`)

Separate from atom JIT: selects ~50–100 predicates from `core.PredicateCorpus` for prompt injection (core domain first, then shard domains, optional vector). Caps defaults 100 / vector 200. Attach LocalStore for embeddings.

---

## 15. Integration map

| Consumer | How |
|----------|-----|
| `internal/session/executor.go` | `buildCompilationContext` → `jitCompiler.Compile` → tools from ConfigFactory / result config |
| `internal/session/spawner.go` | Compile for spawned agents; baseline retry |
| `internal/articulation/prompt_assembler.go` | Optional `JITPromptCompiler`; `SetJITCompiler` |
| System shards | Router, planner, legislator, mangle_repair, world_model, perception — compile with shard-specific contexts |
| `cmd/nerd` campaign/chat boot | `LoadEmbeddedCorpus`, `WithKernel`, `NewDefaultConfigFactory`, shard DB registrars |
| `cmd/nerd/ui/jit_page.go` | Displays `CompilationResult` / atoms for glass-box |
| E2E tests | Multiple `tests/e2e/prompt_*` and session integrations |

### Session interface ports

```go
// session abstracts the concrete type
type JITCompiler interface {
  Compile(ctx, *prompt.CompilationContext) (*prompt.CompilationResult, error)
}
```

---

## 16. Observability surface

- `CompilationStats.String()` / `ToLogFields()`  
- `PromptManifest` selected/dropped entries  
- `lastResult` atomic pointer  
- Timers: `CategoryJIT` / `CategoryContext` / `CategoryStore`  
- Cache hit/miss counters  
- DebugMode manifest logging  

See [11-OBSERVABILITY.md](11-OBSERVABILITY.md).

---

## 17. Safety notes

- Skeleton includes **safety** category; budget treats safety/identity as mandatory priority.  
- Config always requests policy enforcement flag.  
- Structured-output-only shards strip piggyback protocol atoms to avoid format conflict.  
- Mandatory atoms cannot silently exceed total budget.  
- compile_context retracted after selection (when retracter available).  

Kernel `permitted(...)` remains outside this package.

---

## 18. Gaps pointer

See [03-GAP-ANALYSIS.md](03-GAP-ANALYSIS.md), [TODO.md](TODO.md), [OPEN-QUESTIONS.md](OPEN-QUESTIONS.md).

Primary living gaps: cache-key completeness, dual ConfigAtom catalogs, predicate selector wiring clarity, agents.md path referenced from root, vector-only embedded without sync.

---

## 19. Verify

```powershell
go test ./internal/prompt/...
go test ./internal/session/ -count=1  # consumer
# rebuild baked corpus after atom edits:
go run ./cmd/tools/prompt_builder -input internal/prompt/atoms -output internal/core/defaults/prompt_corpus.db
```

---

## 20. Related docs in this corpus

- Deep narrative of the full pipeline: [13-PROMPT-JIT-DEEP-DIVE.md](13-PROMPT-JIT-DEEP-DIVE.md)  
- API catalog: [06-PUBLIC-API-AND-TYPES.md](06-PUBLIC-API-AND-TYPES.md)  
- Wiring: [08-WIRING-AND-INTEGRATION.md](08-WIRING-AND-INTEGRATION.md)
