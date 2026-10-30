# prompt — Implemented Spec (Deep-Dive)

## Verified preservation slice (2026-10-02)

**VERIFIED CURRENT:** `internal/prompt/reconciler_preservation_test.go#TestReconcilePreservation_ExactNullableTasks` (`internal/prompt/reconciler_preservation_test.go:48`) and the other named real SQLite controls exercise production `ReconcilePromptCorpus`, not a shadow updater. Root gate `artifact:.corpus-build/runs/all-features-20261002/round5-reconciler-builder.receipt.json` and the full package gate establish exact vector/task retention, Go-whitespace project ownership, canonical precedence, changed-input invalidation and transactional rollback. Consumers include factory boot, init profile and the explicit maintenance helper (`internal/system/factory.go:1647`, `internal/init/profile.go:1018`, `cmd/tools/prompt_builder/main.go:197`). The 923-row generated seed and first-boot prompt database carry the current perception content. This does not claim legacy ANN/task/model eligibility, schema-migration rollback or a successful live turn; the native deadline counterexample keeps normal-entry acceptance open.

> Last verified against codebase: **2026-07-13**  
> Status: Living Reference Document  
> Language: Go  
> Primary sources: `internal/prompt/`  
> Scale: **25** root non-test Go files; **34** root test files; **1** sync source/test pair; **333** embedded YAML files; **888** runtime-loaded atoms; **0** package-local `.mg`
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
  Acquire private KernelCompilationScope (production clones RealKernel)
  Assert compile_context/* + selector facts inside scope; defer scope close
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

### Applicability boundary

| Lane | Implemented truth |
|---|---|
| Mangle | `CompilationContext.GenerateFacts` and `AtomSelector.buildContextFacts` produce the typed selection inputs declared in `schemas_prompts.mg`; `jit_selection.mg` derives eligibility. |
| Permission and safety | Safety is skeleton guidance; this package does not derive `permitted/3`. Session tool execution remains the authoritative downstream gate. |
| Fact flow | Session intent/world state -> compilation context -> compiled prompt/config -> LLM -> downstream tool gate/articulation. |
| JIT and agents | Embedded and per-agent atoms, selector dimensions, structured-output filtering, DB registration, and agent synchronization are live. |
| Wiring | `initIntelligenceLayer`, `initShardManagement`, and `initFinalExecutors` own boot, lifecycle, and clean-session injection. |
| State and concurrency | Locks, atomics, LRU, and same-key singleflight are live; production compile facts are isolated in disposable cloned kernels. Third-party unscoped adapters remain a compatibility residual. |
| Recovery | Skeleton errors fail compilation; flesh/vector/optional stores degrade; session can fall back to a generic baseline. |
| Observability | Stats, manifests, logs, cache counters, last result, and TUI inspection exist; no durable turn-correlated receipt exists. |
| Testing | Package/validator tests plus full prompt/sync and focused production scope/cache race tests pass; no fuzz target or external-adapter conformance suite exists. |

These rows are summarized for humans in [README.md](README.md#what-exists-today)
and reconciled into explicit gaps in [03-GAP-ANALYSIS.md](03-GAP-ANALYSIS.md).

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
| `CompilationContext` facts/hash | **Implemented** | `compilation-context-v2` covers every live prompt-affecting field and canonicalizes sets |
| `ConfigFactory` / ConfigAtoms | **Implemented/partial** | Production default provider is live; SimpleRegistry catalog is test-only and drifted |
| `CompilerVectorSearcher` | **Implemented** | DB embedding cosine |
| `EvolvedAtomManager` | **Implemented** | SPL pending/promoted |
| Baseline non-JIT assembly | **Implemented** | `baseline.go` |
| `PredicateSelector` | **Implemented** | Predicate subset (parallel concern) |
| `AgentSynchronizer` | **Implemented** | `sync/` |
| Default corpus materialize | **Implemented** | `default_corpus.go` |
| Kernel policy selection rules | **Implemented (core)** | Not in this package |
| ConfigAtom ↔ live tool registry parity | **Partial** | String catalog is not generated/validated against the registered tool surface |
| Cache key completeness | **Implemented** | Real retry regression proves nudge and exact tools bypass the pre-retry cache |
| Compile fact isolation | **Implemented/partial** | Production scopes clone `RealKernel`; external adapters without scope/retract support remain unisolated |
| Atom schema validation | **Implemented** | Shared strict parser and ordered 888-ID parity across validator/filesystem/embed routes |
| Scoped package guidance | **Implemented** | `internal/prompt/agents.md` defines schema, migration, and verification contracts |

**Overall:** living production JIT with strong component coverage and unresolved
critical cache-identity, fact-isolation, and authoring-contract gaps.

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
  context.go               # CompilationContext and versioned cache identity
  atom_schema.go           # Shared strict YAML schema, migrations, vocabularies
  loader.go                # Strict AtomLoader adapter and YAML→SQLite
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
2. Clone context, resolve specialists, and perform **cache lookup** by the
   field-complete `cc.Hash()` — HIT returns stored `*CompilationResult`.
3. **singleflight.Do(cacheKey)** for MISS:  
   a. Acquire `KernelCompilationScope`; production clones the primary `RealKernel`.
      Defer scope close, then assert context facts into the private evaluator.
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

- **`injectable_context(shard, atomText)`** → one `CategoryContext` atom, `IsMandatory`, unscored importance, content bullet list.  
- **`specialist_knowledge(shard, topic, body)`** → one `CategoryKnowledge` atom, `IsMandatory`, unscored importance.  

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

**Pin selectors (empty = unpinned; non-empty = fail-closed):**  
`Providers`, `Models`. An atom declaring either is served **only** to a matching
provider/model, and is blocked on a compile that names none. Values are canonical
tokens from `pinning.go`, normalized on both the atom and the context side so a
YAML `claude-opus-4` and a runtime `anthropic/claude-opus-4-20260501` reach the
same Mangle constant. See `14-PROVIDER-MODEL-PINNING.md`.

**Composition:** `Importance`, `IsMandatory`, `IsExclusive`, `DependsOn`, `ConflictsWith`.

**Importance (as built 2026-10-28, `internal/prompt/importance.go`).** The
hand-set `priority` (0..100; 1,246 of 2,445 atoms sat at the default 30) is gone
as a decision input. `Importance` is Jev's answer, at corpus build time, to one
Score: "If the coding agent ignored `guidance`, what would most likely happen?"
(`internal/atomjudge/questions.go`). The five levels are written situations,
and those sentences are the Score's criteria word for word:

| Level | Name | Meaning |
|---|---|---|
| 1 | style | Ignoring this guidance changes only style or wording; the work is still correct. |
| 2 | convention | Ignoring this guidance breaks a project convention or makes the result harder to review, but the result still works. |
| 3 | efficiency | Ignoring this guidance wastes turns, tool calls or tokens, or causes rework, but the end result can still be correct. |
| 4 | correctness | Ignoring this guidance produces a wrong result: incorrect code, a failing test, a broken contract or a wrong answer. |
| 5 | critical | Ignoring this guidance breaks the build, loses or corrupts data, or violates safety or permissions. |

- The level is the Score's position rounded to the nearest level. The ledger
  `internal/prompt/atom_importance.json` (embedded) keeps, per atom, the content
  hash Jev judged, the level, the position and the five probabilities, plus the
  question's fingerprint and the Jev model ID.
- `go run ./cmd/tools/prompt_builder -score-importance` scores every atom the
  ledger does not cover for exactly its current content (`-rescore-all` for
  all). `-reconcile-embedded` writes the level into the corpus `importance`
  column (`NULL` = unscored); `EnsureSchema` drops the old `priority` column.
- The zero value is `ImportanceUnscored`: a missing or stale score, and every
  atom outside the embedded corpus (project, expert, evolved, strategy,
  retrieved, kernel-injected). An unscored atom gets no `atom_importance` fact,
  so no importance rule decides anything about it; Go tie-breaks put it after
  scored atoms. It is never given a stand-in level.
- Importance orders and arbitrates; it never admits. Mandatory (skeleton) atoms
  stay rule-selected. Uses: `conflict_loser` in `policy/jit_selection.mg` (the
  less important of two conflicting candidates loses; both must be scored),
  `candidate_atom` (a shard's level-4/5 atoms join the veto set), the Mangle
  mandatory cap order (`selectMangleMandatoryIDs`), the resolver's peer
  tie-break (`scoredPeerLess`) and the embedded baseline order.
- Built-in YAML may not carry `priority`; a project atom that still does parses
  through the observable `legacy-priority-ignored` migration (removal date
  2027-01-01) and the value reaches nothing. `internal/init/agents_atomize.go`
  no longer writes it.
- Coverage: `TestEmbeddedLedgerCoversEveryAtom` (`internal/atomjudge`) fails
  when an atom was edited or added without a rescore.
- First full score, 2026-10-28, `jev-1.14.0`, 2,445 atoms in 3.1 s: style 4,
  convention 201, efficiency 1,244, correctness 925, critical 71. Against the
  old hand values: of the 1,246 atoms at the default 30, 609 are correctness
  and 11 critical; of the 472 at 90..100, 56 are convention-level. 616 atoms put
  at least 0.3 on a second level, so the probabilities are kept for audit.

**Polymorphism / retrieval:** `Description`, `ContentConcise`, `ContentMin`, `Embedding`, `EmbeddingTask`.

`MatchesContext` requires **all non-empty selector dimensions** to match (AND across dimensions; frameworks/world_states/models use ANY within dimension).

### 5.3 Mangle fact projection

- `ToFact()` → `prompt_atom(ID, Category, TokenCount, IsMandatory)` — order matches `schemas_prompts.mg`; importance is `atom_importance(ID, Level)`, emitted only for a scored atom (`selector_facts.go`)  
- `ToSelectorFacts()` → `atom_selector(ID, Dimension, Value)`  
- `ToDependencyFacts()` / conflict facts similarly  

Selector path also builds string facts for batch assert (see `selector.go` `buildContextFacts`).

### 5.4 Token estimate

`EstimateTokens` ≈ `ceil(len/4)` heuristic. Used when `TokenCount` missing.

### 5.5 YAML source of truth

Embedded via `//go:embed atoms`. `atom_schema.go#AtomDefinition` and
`ParsePromptAtomYAML` are the single schema/conversion path for filesystem,
embedded, synchronization, and validation routes. Unknown fields or partial
invalid documents fail closed. Optional `content_file` is resolved relative to
the atom source. Legacy aliases are observable migrations expiring 2027-01-01;
built-ins must parse without migrations.

### 5.6 Mangle reference atoms (`atoms/mangle/`, `language/mangle*.yaml`)

The atoms that teach a model to write Mangle for the pinned engine
(`codeberg.org/TauCeti/mangle-go v0.7.0`, go.mod) and to drive it from Go. They
are ids `language/mangle/docs/<slug>`, one idea each, within the 1,500-char atom
ceiling, in topic files: `syntax`, `negation` (negation, stratification,
recursion), `aggregation`, `builtins`, `advanced` (temporal facts, merge
lattices, proofs, external predicates), `go_integration`, `errors`,
`antipatterns`, `patterns`, and `codenerd` (rules against the kernel's own
predicates). `engine_truths_pinned.yaml` is the mandatory verified-by-running
list, served when the kernel derives `/authoring_mangle`.

Nothing hand-written is trusted: `atoms_mangle_truth_test.go` asks the pinned
engine. A ```` ```mangle ```` fence must parse, analyze (strict bounds where it
declares types) and stratify; a ```` ```mangle-rejected ```` fence must be
refused by the engine, so "this is an error" is itself checked. In `codenerd`
atoms the fence is analyzed against the booted kernel's Decls, so a taught
predicate exists at that arity and with those types. Every `fn:` and `:` name
must exist in `builtin.Functions` / `builtin.Predicates` unless the line says it
does not exist; a Go identifier qualified by a mangle-go or codeNERD package must
exist in that package's source and not be marked `Deprecated:`; a ```` ```go ````
fence must parse; no file under `atoms/` may name the upstream Google import
path. Embedded Go and Mangle identifiers therefore change only with the module
version, and a bump that changes an answer fails the test instead of a prompt.

---

## 6. Deep dive — Selector (`selector.go`)

### 6.1 Skeleton vs flesh

```go
skeletonCategories = identity | protocol | safety | methodology
```

`SelectAtoms` / `SelectAtomsWithTiming`:

1. Filter structured-output-only shards (strip piggyback/reasoning protocol atoms for legislator & mangle_repair).  
2. Compute `selectMangleMandatoryIDs` when shard is legislator|mangle_repair **and** language is mangle — forces the most important mangle mandatory atoms under token/atom caps.  
3. Parallel: `loadSkeletonAtoms` + `loadFleshAtoms`.  
4. Skeleton error → fail compile. Flesh error → warn, continue.  
5. `mergeAtoms` (skeleton wins on ID collision).

### 6.2 Skeleton load

- Requires `s.kernel != nil` else CRITICAL error.  
- Filter candidates to skeleton categories.  
- Build/assert facts; query `selected_result(Atom, Source)`.  
- Debug queries: `blocked_by_context`, `mandatory_selection`.  
- All selected get LogicScore/Combined = 1.0, Source=`skeleton`.  
- Missing category → warn (does not always hard-fail if any skeleton returned),
  unless the kernel emptied it on purpose (as built 2026-10-28): the selector
  reads `mandatory_superseded(Atom)` and `unexplainedSkeletonGaps`
  (`internal/prompt/selector.go:1077`) reports only a category with no selected
  atom and no superseded candidate. The delegation judge compile
  (`/judge_implementation`, `/judge_review`) supersedes the Piggyback protocol
  and OODA methodology atoms by `conflicts_with`, so its empty protocol and
  methodology categories are its shape, not a defect
  (`TestSkeletonGaps_JudgeCompileSupersessionIsNotADefect`).
- `atom_category(AtomID, Category)` carries the category as a name constant
  (`/identity`), as its Decl declares (`selector_facts.go`, `candidateCategoryName`).

### 6.3 Flesh load

- Non-skeleton categories only.  
- Optional vector scores via `SemanticQuery` + `SemanticTopK` with timeout.
  `topKEligibleVectorScores` keeps the vector channel's own **slots** (relative
  floor, top-K).  
- Assert facts + `vector_hit(ID, Score)` for the recall pool and
  `vector_slot(ID)` for the slots. Without Jev the pool is the slots.  
- **Per-turn relevance (as built 2026-10-28, `relevance.go`,
  `internal/atomjudge`).** With `typesafe.atom_relevance` on (default when the
  section exists), `recallPool` widens the slots to up to
  `atom_relevance_pool` (default 40) eligible atoms, the next most similar
  above the absolute floor. Jev answers one Noul per pooled atom, "Does
  `guidance` apply to the work in `request`?", over the state
  `request{text, intent, target, language, frameworks, agent}`. Answers become
  `atom_relevance(ID, Percent)` plus `atom_relevance_threshold(/yes|/no, P)`
  (typesafe `judgment_yes_percent` / `judgment_no_percent`).  
- One judgment per turn: answers are cached by request and atom content
  (`relevanceCache`), so the many compiles of a turn ask once. Bounded by
  `atom_relevance_timeout` (default 4s). A failed, late or incomplete judgment
  asserts no relevance fact and keeps `vector_hit` to the slots: the compile is
  exactly the vector channel's. Mandatory atoms are never put to Jev.  
- Mangle filter or `fallbackFleshSelection` (context match + scores) if no kernel.  
- Combined score weights logic vs vector (`vectorWeight` default 0.3) for
  unjudged flesh. Judged flesh is ordered for the prompt cache, never by the
  turn's probabilities (the kernel's admission already read them): carried
  atoms at 0.9 in their carried order, new atoms at 0.5, so the resolver orders
  new atoms by importance then ID and the budget sheds them first.  
- **Carry-over (cache stability).** `carryMemory` remembers, per task (agent,
  verb, target), the flesh the last judged compile admitted, in prompt order.
  The next compile of that task adds those atoms to the recall pool and asserts
  `flesh_carried(ID)`; `flesh_admitted(Atom, /carried)` keeps them through an
  unsettled reading and drops them only when Jev reads them as not applying
  (the 30-point margin below yes). The assembler places carried flesh in
  `tierCarried`, after the mandatory tiers and before new flesh, so consecutive
  turns of one task share the cached prefix through every carried atom. A new
  task carries nothing.  
- After a judged compile the selector logs the kernel's admission table
  (`flesh_admitted`, `flesh_refused`, `logRelevanceAdmission`).

### 6.4 Mangle rule file (external)

`internal/core/defaults/jit_compiler.mg` declares and defines:

- `blocked_by_context(Atom)`  
- `mandatory_selection(Atom)`  
- `selected_result(Atom, Source)` with `/skeleton` and `/flesh` sources  
- the flesh admission table, one row per relevance band: `flesh_admitted(Atom,
  /judged_applies)` from anywhere in the recall pool;
  `/unsettled_vector_slot` and `/unjudged_vector_slot` only from a vector slot;
  `flesh_refused(Atom, /judged_does_not_apply)` even from a slot. A missing
  judgment is never a no (`internal/core/jit_atom_relevance_test.go`).  
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
- `Hash()` SHA-256 over schema `compilation-context-v2` and every prompt-affecting
  dimension, including retry/tools, budgets, search, world state, specialists,
  shard identity, and set-canonicalized frameworks/tools.

`internal/prompt/context_hash_test.go#TestCompilationContext_Hash` proves retry,
tools, specialists, budgets, search, world fields, set canonicalization, and caller
non-mutation. The production retry regression proves the output-level effect.

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

### 12.4 Persona database owners, leases and readers

`RegisterAgentDBWithJIT`, `RegisterShardDB` and shard-named `RegisterDB` calls
install an owner registration. The registry explicitly replaces or removes that
owner; ordinary task completion does neither. `CreateJITDBRegistrar` returns a
scoped acquisition callback for ShardManager, backed by `AcquireShardDB` in
`internal/prompt/compiler_shard_lifecycle.go`. Acquiring an existing normalized
persona borrows its current generation. If absent, acquisition opens a temporary
pool shared by overlapping executions. Each idempotent release is bound to its
generation, so releasing an older execution cannot delete a newer owner.

Source snapshots retain the selected persona and pushed experts until their
actual singleflight work finishes. Canceling the synchronous waiter does not
release those readers early. Vector scans, technology reads and knowledge-use
recording also retain their shard handles. Removing or replacing a registration
immediately changes the name visible to future readers, but retirement closes
the old pool only after its execution leases and readers drain. Shutdown rejects
new acquisitions and source snapshots, cancels compile work, drains counted
readers, then closes all remaining current and retired pools. LookupShardDB is
an observation API; its returned handle conveys no ownership.

Only registration changes advance the source epoch and invalidate cached
prompts. Borrowing or releasing a boot-owned generation has no such effect;
in-place source writes remain covered by source-generation capture. The
ownership contract does not change Mangle selection, relevance, budgeting,
permissions or prompt atoms. Database files are retained.

`internal/prompt/compiler_shard_lifecycle_test.go` covers owner preservation,
temporary sharing, old-generation release after replacement, explicit removal,
in-flight speaker and pushed-expert readers after caller cancellation, and
shutdown. `internal/system/shard_db_ownership_test.go` covers boot wiring and
custom-atom delivery through a subsequent real compilation. These tests are
authored, not executed; measured resource costs and runtime validation remain
open.

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
- production compile and selector facts live only in a cloned
  `KernelCompilationScope` and are discarded on close; third-party adapters that
  expose neither scope creation nor retraction remain a compatibility residual.

Kernel `permitted(...)` remains outside this package.

---

## 18. Gaps pointer

See [03-GAP-ANALYSIS.md](03-GAP-ANALYSIS.md), [TODO.md](TODO.md), [OPEN-QUESTIONS.md](OPEN-QUESTIONS.md).

Primary living gaps: durable decision receipts, external-adapter isolation policy,
dormant ConfigAtom capability drift, cache TTL/shared-result ownership, fuzz
coverage, and shadow/replay evidence.

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
