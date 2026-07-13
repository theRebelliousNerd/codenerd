# prompt — JIT Deep Dive (Compiler · Atoms · Selector · Budget · Resolver)

> Last verified: **2026-07-13**  
> Companion to [IMPLEMENTED_SPEC.md](IMPLEMENTED_SPEC.md) — narrative walkthrough of the five core machines.

---

## 1. Why JIT exists

Monolithic shard prompts do not scale: they fight token limits, freeze knowledge, and force the LLM to act as executive. codeNERD replaces them with:

- **Atoms** — small, tagged, versionable fragments.  
- **Context** — structured dimensions from intent + world.  
- **Logic** — Mangle selects skeleton; vectors propose flesh.  
- **Budget** — fits reality of context windows with polymorphism.  
- **Assembly** — stable head, dynamic tail.

---

## 2. Atom as the unit of meaning

### 2.1 Identity

`PromptAtom.ID` is the global key (e.g. `protocol/piggyback/envelope`, `language/go/error_handling`). Content is markdown. `ContentHash` supports dedupe/cache invalidation. `TokenCount` is precomputed or estimated.

### 2.2 When an atom is eligible

`MatchesContext` (used in baseline and some flesh fallbacks):

```
for each non-empty selector dimension:
  context value must be ∈ selector list (normalized)
frameworks: ∃ overlap
world_states: ∃ active world flag matching
empty dimension ⇒ wildcard
```

YAML authors control fan-out by leaving selectors empty (always candidate) vs tight tags (`shard_types: [coder]`, `languages: [go]`).

### 2.3 Composition graph

- `DependsOn` — must appear earlier (resolver).  
- `ConflictsWith` — primarily Mangle exclusion.  
- `IsExclusive` — exclusion group id.  
- `IsMandatory` — budget forces inclusion attempts; Mangle mandatory selection for skeleton.

### 2.4 Polymorphism

Three bodies for one ID: full / concise / min. Budget walks modes. Description field is for **embedding**, not necessarily full content (keeps RETRIEVAL_DOCUMENT embeddings cheaper).

---

## 3. CompilationContext as the request object

Think of `CompilationContext` as the **query plan** for prompts.

| Tier | Fields | Drives |
|------|--------|--------|
| Mode | OperationalMode | Dream vs active vs debug atoms |
| Campaign | Phase, ID, Name | Campaign encyclopedia |
| Build | BuildLayer | Scaffold→integration atoms |
| Init / Northstar / Ouroboros | Phase/stage | Specialized flows |
| Intent | Verb, Target | Intent atoms + ConfigFactory |
| Shard | Type, ID, Instance, Name | Persona + DB selection + inject match |
| World | counts/flags + tool_nudge | world_state atoms |
| Stack | Language, Frameworks | language/framework flesh |
| Budget | TokenBudget, Reserved | AvailableTokens for Fit |
| Semantic | Query, TopK | Vector flesh |
| Dynamic | Specialists, Tools, Activation | Templates + future boosts |

`ToContextFacts` materializes this as Mangle `compile_context` pairs for policy-side boosting/blocking.

`Hash` emits the versioned `compilation-context-v2` identity over every
prompt-affecting field. Set-like frameworks/tools are sorted and deduplicated
without mutating the request; values are length-prefixed to prevent delimiter
collisions. **Authors of new dimensions must classify and test cache impact.**

---

## 4. Compiler orchestration

### 4.1 Cache + singleflight

```
hash = cc.Hash()
if LRU[hash]: return cached
singleflight.Do(hash):
  acquire private KernelCompilationScope
  ... pipeline against scoped kernel ...
  close scope on every exit
  LRU push (evict if >100)
```

Prevents recompilation storms when many shards share identical contexts.
Production `KernelAdapter.NewCompilationScope` clones the primary `RealKernel`,
so mixed-key compiles never share selector facts. External adapters without the
scope interface retain compatibility-only semantics.

### 4.2 Parallel collection

errgroup fans out independent sources. Only static collect failure aborts; inject/knowledge/learning are soft.

### 4.3 Soft vs hard failures

| Step | Hard fail? |
|------|------------|
| Validate context | Yes |
| Collect static | Yes on errgroup error |
| Kernel inject | No |
| Select skeleton | Yes |
| Select flesh | No |
| Resolve | Yes on error |
| Fit | Yes on error (e.g. budget < headroom) |
| Assemble | Yes |
| ConfigFactory | No (warn) |

### 4.4 Result packaging

`buildResultWithStats` correlates candidates vs scored vs fitted, fills category token maps, builds Manifest entries for selected/dropped (where tracked).

---

## 5. Selector internals

### 5.1 Bifurcation motivation

Embeddings fail. Network times out. Similarity misranks. **Safety must not depend on that.** Skeleton uses only Mangle + mandatory flags + context blocks.

### 5.2 Skeleton query loop

```
filter skeleton categories
buildContextFacts(cc, skeletonAtoms, forcedMandatory)
kernel.AssertBatch(facts)
query selected_result(Atom, Priority, Source)
map IDs → ScoredAtom{Combined:1.0, Source:skeleton}
```

Forced mandatory: for legislator/mangle_repair + language mangle, high-priority mangle-tagged mandatory atoms can be capped (`mangleMandatoryTokenCap` / atom cap / 90% budget ratio) and marked mandatory even if not originally.

### 5.3 Flesh query loop

```
filter non-skeleton
optional getVectorScores(SemanticQuery, TopK) with timeout
build facts + vector_hit(id, score)
Mangle select OR fallbackFleshSelection
score Combined = (1-w)*logic + w*vector
threshold minScoreThreshold (default 0.1)
```

### 5.4 Merge

Union by atom ID; skeleton entry wins. Preserves critical atoms if flesh also returned them with lower scores.

### 5.5 Structured-output prefilter

Before bifurcation, legislator/mangle_repair drop `protocol/piggyback/*` and `protocol/reasoning/*` atoms so system prompts do not demand piggyback envelopes.

---

## 6. Resolver internals

Kahn topological sort:

1. Count in-edges from DependsOn ∩ selected set.  
2. Seed queue with zero in-degree; sort mandatory then score.  
3. Emit nodes; decrement dependents; re-sort queue.  
4. If stuck: break cycle at highest Combined remaining node.

Outputs sequential `Order` used by assembler within categories (assembler also groups by category order — dependency order is global first, then category regrouping can interleave differently if categories reorder; dependency edges across categories still influenced initial sequential order indices).

**Author tip:** keep dependencies within or near categories to avoid surprising reorders after `SortByCategory` helpers.

---

## 7. Budget Fit internals

### 7.1 Allocation

`calculateAllocations` distributes `availableBudget` (total − headroom) across present categories using BasePercent, clamping Min/Max, respecting PriorityFirst strategy.

### 7.2 Inclusion walk

Sorted by category priority then score:

```
for each category chunk:
  for each atom:
    if mandatory:
      include if totalBudget allows (absolute)
    else:
      try standard under remaining category allocation
      else concise
      else min
      else unselected
fillRemaining with leftovers
```

### 7.3 Truncation helper

`truncateAtomToBudget` may shorten content for near-fit cases (see budget.go) — used carefully on non-mandatory paths.

### 7.4 Reports

`BudgetReport` / `CategoryUsage` support diagnostics of utilization per category.

---

## 8. Assembly and prefix caching

Order is a **product decision**, not cosmetic:

```
[static identity/safety/protocol/...][stack knowledge][phase][mode][DYNAMIC TAIL]
```

Dynamic tail last maximizes prefix cache reuse across turns when only world/context changes.

Templates rewrite placeholders using CompilationContext (tools, specialists, etc.).

---

## 9. Config half of JIT

Prompt alone is insufficient. Session needs:

```
EffectiveAgentRuntimeConfig {
  IdentityPrompt  // compiled text
  AllowedTools    // ConfigAtom tools
  Policies        // .mg files to load
  ToolLoop        // iteration caps
  Safety          // RequirePolicyEnforcement
}
```

Intent verbs and shard types map to ConfigAtoms. `/consult/foo` falls back to `/general`.

This is **executive coupling**: policies named here must exist in the policy corpus; tools must exist in VirtualStore.

---

## 10. End-to-end example (narrative)

**User:** “Fix the failing Go test in foo_test.go”

1. Perception → intent verb `/fix`, language `/go`, world `failing_tests`.  
2. Kernel routes coder agent.  
3. Executor builds CompilationContext: shard coder, mode active, semantic query = user text, budget from config.  
4. Compile:  
   - Skeleton: coder identity + piggyback protocol + constitution + methodology (TDD/debug).  
   - Flesh: go testing atoms, failing_tests world_state, exemplars if vector hits.  
   - Budget may drop low-priority exemplars; keep safety.  
5. ConfigFactory: `/fix` + `/coder` → write/edit tools + coder policy suite.  
6. LLM runs with tools; VirtualStore still checks permitted.  
7. If model replies without tools on a tool-required intent: the retry changes
   the v2 cache identity, activates `system/tool_nudge/no_tool_call_retry`, and
   renders the exact current `AvailableTools` list.

---

## 11. Authoring checklist for new atoms

1. Choose category + stable ID path.  
2. Write content + concise/min if large.  
3. Tag selectors tightly enough to avoid budget waste.  
4. Set priority / mandatory only if true skeleton necessity.  
5. Declare DependsOn carefully.  
6. Add description for embedding.  
7. Place under `example:internal/prompt/atoms/<category>/`.
8. Use canonical `shard_types`; built-ins may not depend on legacy migrations.
9. Run `validate_prompt_atoms -fail-on-warn` and ordered parity tests; regenerate
   the baked corpus if shipping defaults.
10. If new tools required: ConfigAtom update + tool registry.
11. Grep wiring before assuming shard still hardcodes prompts.

---

## 12. Debugging playbook

| Question | Instrument |
|----------|------------|
| Why missing atom? | Manifest Dropped; MatchesContext; Mangle blocked_by_context |
| Why huge prompt? | Stats tokens; which categories; mandatory mangle set |
| Why same prompt twice? | `compilation-context-v2` field classification; cache hits |
| Why no tools? | ConfigFactory intents; AllowedTools |
| Why slow? | Collect/Select/Vector ms fields |
| Why wrong persona? | ShardType/ShardID; agent DB registered? |

---

## 13. Relation to north star

| North star | Mechanism |
|------------|-----------|
| LLM creative | Receives prompt only |
| Logic executive | Mangle selection + permitted + tools |
| JIT atoms | YAML library + multi-source collect |
| No prompt drift | Context-keyed assembly + cache hash |

This deep dive is the mental model for reading `compiler.go` → `selector.go` → `resolver.go` → `budget.go` → `assembler.go` in order.
