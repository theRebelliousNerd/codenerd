---
doc-class: deep-dive
subsystem: prompt
implementation-status: partial
last-verified: 2026-10-28
verified-against: 2e0c54ef204997c36e65c91cbb90bc991e63c8c1
verification-method: static-source-inspection
main-revalidated-at: 2026-10-28T02:14:43Z
supersedes: []
---

# JIT optimization through generation aware reuse

**Decision:** optimize the work that JIT repeats, not the evidence that makes its answer trustworthy. The most useful sequence is to bind all prompt sources to explicit generations, make reuse conditional on those generations, share immutable preflight work, and only then reduce candidate and rendering work. Moving cache lookup ahead of the current private snapshot would be a correctness regression.

This is a source-grounded design for maintainers, not an implementation or benchmark report. No tests, model inference, training, profiling, or production experiments were executed. Conditional race sequences are proposed witnesses, not reproduced incidents. Claims refer to the pinned main revision; main is actively changing and must be checked again before implementation.

Freshness check: GitHub compared the pinned commit with `main` at 02:14:43 UTC on 2026-10-28 and reported identical (zero commits ahead or behind). This is a snapshot of the moving branch, not a claim about future pushes.

## 1 Scope and existing strengths

The [repository guidance](https://github.com/theRebelliousNerd/codenerd/blob/2e0c54ef204997c36e65c91cbb90bc991e63c8c1/agents.md#the-vision-steve-2026-09-18--read-before-any-architectural-change) places quality and capability ahead of token reduction and gives Mangle executive responsibility for context. The [prompt-specific guidance, lines 7–10](https://github.com/theRebelliousNerd/codenerd/blob/2e0c54ef204997c36e65c91cbb90bc991e63c8c1/internal/prompt/agents.md#L7-L10) requires cache identity, selection and injections to use the same private kernel snapshot. This proposal preserves both requirements.

The current compiler is not an uncached sequential concatenator:

| Existing mechanism | Source and consequence |
|---|---|
| Private compilation scope before cache lookup | `compile`, [internal/prompt/compiler.go:693–743](https://github.com/theRebelliousNerd/codenerd/blob/2e0c54ef204997c36e65c91cbb90bc991e63c8c1/internal/prompt/compiler.go#L693-L743), and `NewCompilationScope`, [internal/system/factory_adapters.go:80–101](https://github.com/theRebelliousNerd/codenerd/blob/2e0c54ef204997c36e65c91cbb90bc991e63c8c1/internal/system/factory_adapters.go#L80-L101): production clones the kernel, then selection uses that private scope. |
| Dynamic state and caller context in identity | `collectDynamicKernelSnapshotWithConfig`, [internal/prompt/compiler.go:1179–1233](https://github.com/theRebelliousNerd/codenerd/blob/2e0c54ef204997c36e65c91cbb90bc991e63c8c1/internal/prompt/compiler.go#L1179-L1233), and `CompilationContext.Hash`, [internal/prompt/context.go:642–756](https://github.com/theRebelliousNerd/codenerd/blob/2e0c54ef204997c36e65c91cbb90bc991e63c8c1/internal/prompt/context.go#L642-L756): current schema is v5, not the older v2 described in some historical docs. |
| Configuration generation and private results | `configSnapshot`/`SetConfig`, [internal/prompt/compiler_db.go:277–310](https://github.com/theRebelliousNerd/codenerd/blob/2e0c54ef204997c36e65c91cbb90bc991e63c8c1/internal/prompt/compiler_db.go#L277-L310); `privateResult`, [internal/prompt/compiler.go:306–388](https://github.com/theRebelliousNerd/codenerd/blob/2e0c54ef204997c36e65c91cbb90bc991e63c8c1/internal/prompt/compiler.go#L306-L388). Do not propose repairing a shared mutable result bug already addressed here. |
| LRU and same-key singleflight | `NewJITPromptCompiler` and `compile`, [internal/prompt/compiler.go:595,758–777,964–985](https://github.com/theRebelliousNerd/codenerd/blob/2e0c54ef204997c36e65c91cbb90bc991e63c8c1/internal/prompt/compiler.go#L758-L777): size-bounded reuse and duplicate-call suppression already exist. |
| Concurrent source collection and correct selection barrier | `compile`, [internal/prompt/compiler.go:804–839](https://github.com/theRebelliousNerd/codenerd/blob/2e0c54ef204997c36e65c91cbb90bc991e63c8c1/internal/prompt/compiler.go#L804-L839); `runSelection`, [internal/prompt/selector.go:698–905](https://github.com/theRebelliousNerd/codenerd/blob/2e0c54ef204997c36e65c91cbb90bc991e63c8c1/internal/prompt/selector.go#L698-L905). Both skeleton and flesh facts land before either selection query. Preserve this union-sensitive conflict/dependency behavior. |
| Canonical precedence and capability gating | `collectAtomsWithStats`, [internal/prompt/compiler.go:1396–1463](https://github.com/theRebelliousNerd/codenerd/blob/2e0c54ef204997c36e65c91cbb90bc991e63c8c1/internal/prompt/compiler.go#L1396-L1463); [internal/prompt/agents.md:55–75](https://github.com/theRebelliousNerd/codenerd/blob/2e0c54ef204997c36e65c91cbb90bc991e63c8c1/internal/prompt/agents.md#L55-L75). Embedded IDs win; a missing tool must not be granted merely to make its prompt atom eligible. |
| Stable assembly and actual rendered-budget check | `assembleWithConfig`, [internal/prompt/assembler.go:206–270](https://github.com/theRebelliousNerd/codenerd/blob/2e0c54ef204997c36e65c91cbb90bc991e63c8c1/internal/prompt/assembler.go#L206-L270); `enforceAssembledBudget`, [internal/prompt/compiler.go:1929–1970](https://github.com/theRebelliousNerd/codenerd/blob/2e0c54ef204997c36e65c91cbb90bc991e63c8c1/internal/prompt/compiler.go#L1929-L1970). Rendering uses deterministic tiers/categories, sheds whole optional atoms, and refuses an oversized mandatory remainder. |

The owning [prompt gap analysis, Dynamic evidence is part of prompt cache identity](https://github.com/theRebelliousNerd/codenerd/blob/2e0c54ef204997c36e65c91cbb90bc991e63c8c1/Docs/architecture/prompt/03-GAP-ANALYSIS.md#dynamic-evidence-is-part-of-prompt-cache-identity) already records the snapshot correctness requirement. The adjacent [Mangle kernel performance study](mangle-kernel-performance.md) addresses clone storage representation and cone preparation. This document instead addresses how often JIT requests those operations, what a reusable result depends on, and how compilation work is partitioned. The [prompt evolution study](prompt-evolution-lifecycle-and-evidence-accounting.md) owns admission/promotion of learned instructions; this proposal does not make cached evolved content trusted merely because it is versioned.

## 2 A cache hit still performs substantial preflight work

`compile` acquires the scope, asserts contextual facts, collects injections, and fingerprints the complete `QueryAll` result before looking in the LRU. The fingerprint serializes typed argument descriptions, canonicalizes/sorts the records, and hashes them (`compile`, `collectDynamicKernelSnapshotWithConfig`, citations above). Even a hit therefore pays for a clone and whole-snapshot traversal. Concurrent same-key callers each perform this preflight before reaching `DoChan` at line 761; duplicate suppression applies to the subsequent work, not the earlier preparation.

This is intentional correctness work, not evidence of a defect simply because it costs CPU. It prevents an unchanged caller context from reusing stale kernel-derived instructions. The optimization opportunity is to replace repeated reconstruction with a cheap **handle to the same immutable generation**, not to pretend the state did not change.

The present timing also has a specific scope. `snapshotStart` is at line 719, after `acquireCompilationKernel` at line 703; `stats.Duration` adds this snapshot interval to the miss-work interval at line 952. A cache hit returns a private copy of the earlier result, changing `CacheHit` but not remeasuring its original compilation duration (`privateResult`, lines 364–370). The outer legacy timer exists at lines 680–682, so this is not a claim that no timing exists. It means the per-result/miss average must not be mistaken for the complete current-call latency or total host cost of hits and joiners.

### Proposed reuse contract

Names below are design notation, not current APIs:

```text
CompilationSnapshotHandle:
  workspace_identity + kernel_identity
  program_generation + base_fact_generation
  immutable read handle or ownership lease
  external_dependency_status

PreflightKey:
  snapshot_identity + canonical caller context + compiler configuration generation
```

1. Capture an immutable kernel handle and generation atomically under the kernel's supported synchronization. All fact changes, retractions, rule/schema changes and reset/restore paths must advance the appropriate generation. Process-local sequence values need an incarnation identity to prevent reuse after reset.
2. When the same base handle and caller context recur, share the private overlay/injection/fingerprint result. Each consumer receives read-only results or its own mutation overlay; do not share a mutable selection kernel between distinct compiles.
3. If a kernel query invokes untracked external state, the handle alone is insufficient. Track that dependency explicitly or retain the current full-capture/no-reuse path. A monotonic counter is not a snapshot if values are read outside its protected epoch.
4. Keep the conservative whole-kernel generation first. Later, an analyzed dependency projection may avoid invalidation from unrelated predicates. Such a projection must include negative dependencies, dynamic rules, injected-context producers and schema changes; unknown dependency sets fall back to broad invalidation.
5. Share preflight only after identity is trustworthy. A singleflight key based merely on `cc.Hash()` before its snapshot fingerprint is attached could incorrectly join two different kernel states.
6. Retain existing shutdown joining and independent shared-work cancellation. A canceled caller must release its handle without destroying work still owned by other callers. Bound retained generations and pending work; cache hit rate alone does not justify unlimited memory retention.

The first implementation may use a generation number merely to memoize the expensive fingerprint, while still creating private scopes for selection. That captures part of the opportunity with less concurrency redesign. A more ambitious immutable base plus overlay should be justified by measurements, not assumed superior.

## 3 Nonkernel sources need generations too

The current cache key combines the compilation context, dynamic kernel fingerprint and `configRevision` (`promptCacheKey`, [internal/prompt/compiler.go:384–388](https://github.com/theRebelliousNerd/codenerd/blob/2e0c54ef204997c36e65c91cbb90bc991e63c8c1/internal/prompt/compiler.go#L384-L388)). Project/shard atoms, knowledge and learning retrieval are collected only after a miss (`compile`, lines 804–858). Their independent database/file contents are not themselves encoded in that key.

Database registration does clear the cache, which must be credited. However, `clearPromptCache` replaces the map without advancing a source generation, and `RegisterDB`/`RegisterShardDB` call it after replacement ([internal/prompt/compiler_db.go:138–200,217–235](https://github.com/theRebelliousNerd/codenerd/blob/2e0c54ef204997c36e65c91cbb90bc991e63c8c1/internal/prompt/compiler_db.go#L138-L235)). A compilation's later publish at lines 964–985 does not verify that the source generation it loaded remains current.

**Proposed race witness:** compile A loads project-only atom content and pauses before result insertion. Replace/register the project corpus and clear the LRU. Resume A. It can insert its old result under the same context/configuration key; a later identical call can reuse it if the kernel snapshot is unchanged. This is a source-derived interleaving, not an executed race. Changing `SetConfig` is not the same case: its revision already separates old and new keys.

An in-place knowledge update has a related conditional risk: if no hashed kernel/context input changes and no separate invalidation occurs, the cache hit bypasses retrieval and can retain old material. This review does not claim every producer follows that path or that every turn has identical context. The contract gap is that independent source changes are not uniformly represented at the compiler boundary.

### Generation bound publication

Extend the compilation dependency identity with a source vector:

```text
SourceVector:
  embedded_corpus_digest
  project_atom_generation
  selected_expert_atom_generation
  knowledge_generation + learning_generation
  evolved_and_strategy_generation
  embedding_model_and_index_generation
  source_availability_or_degradation_state
```

Each captured generation must refer to the actual data read, not a counter sampled before an unrelated later read. Use immutable source records or transactional reads with an owned generation row. A replacement changes identity even if a path/name is reused. Commit mutations and their source generation in one database transaction where possible; retain source ownership and current read authorization.

Cache insertion is namespaced by the captured vector. An old compile may finish for a caller whose declared contract permits that captured snapshot, but it cannot populate the new generation's cache. If the consumer requires latest-at-return semantics, revalidate the vector and return a stale result or bounded retry. Be explicit about which semantics are offered; incessant background changes must not cause an invisible infinite recompilation loop.

Missing sources and failed optional lookups also need a contract. `collectAtomsWithStats` logs project/shard read errors and continues, and knowledge/vector collection can degrade (source citations in §§1 and 4). A successful degraded prompt can then enter the normal LRU. Recovery from a transient dependency outage must be able to invalidate that result or trigger a bounded reprobe; otherwise identical context can keep serving degraded output until eviction. This does not require adding a blanket TTL to every correct result. Separate complete-generation reuse from explicitly degraded fallback reuse and its retry policy.

SQLite `PRAGMA data_version` is useful only with its documented limits: comparisons are meaningful on the same connection, and same-connection commits do not change it. It is not a portable global generation to sample on arbitrary `sql.DB` pool connections. Prefer an owned transactional version row or a documented pinned-connection observer plus explicit local-write invalidation. File mtime alone similarly does not certify database content or multi-file consistency.

## 4 Compile invariant atom metadata once per source generation

On a miss, `collectAtomsWithStats` loads project and selected-shard atoms and applies canonical first-source precedence. `loadAtomsFromDB` reads bodies and metadata through a tag join, limiting base rows to 10,000 ([internal/prompt/compiler_db.go:17–97](https://github.com/theRebelliousNerd/codenerd/blob/2e0c54ef204997c36e65c91cbb90bc991e63c8c1/internal/prompt/compiler_db.go#L17-L97)). The selector then rebuilds atom/category/priority/tag/dependency facts as strings in `buildContextFacts` ([internal/prompt/selector.go:1350–1510](https://github.com/theRebelliousNerd/codenerd/blob/2e0c54ef204997c36e65c91cbb90bc991e63c8c1/internal/prompt/selector.go#L1350-L1510)). These facts cross an adapter that parses strings into core facts (`KernelAdapter.AssertBatch`, [internal/system/factory_adapters.go:120–147](https://github.com/theRebelliousNerd/codenerd/blob/2e0c54ef204997c36e65c91cbb90bc991e63c8c1/internal/system/factory_adapters.go#L120-L147)).

Much of that metadata is invariant across caller contexts. A generation-owned atom catalog can retain immutable validated bodies, canonical source precedence, static typed facts, and prepared templates. Each compile supplies only contextual facts, effective capability facts, dynamic atoms, vector scores and context-dependent mandatory overrides. Do not accidentally precompute `is_mandatory` as wholly static: `forcedMandatory` changes it for a compilation in the cited builder.

This is staging, not replacing Mangle selection with a Go heuristic. Mangle remains the final selector. Any coarse database/index prefilter must be a proven superset of eligible candidates and their dependency/conflict closure. In particular, the current selector deliberately builds the complete skeleton/flesh fact union before querying; an optimization that omits conflict competitors can change the winner. A candidate cap must be observable, deterministic and incapable of silently excluding required skeleton/dependency material. The present SQL limit is not evidence that 10,000 atoms is an adequate future quality limit.

Start with the simple version: reuse a complete typed catalog by source vector and add a private dynamic overlay. Only if corpus size warrants it should the host add selector-compatible indexes or lazy body hydration. Lazy hydration must validate that body/version still matches the selected metadata. Unsupported metadata or an unknown selector dimension must not become a false negative.

### Vector work cannot be reduced by blindly asking for fewer results

`runSelection` requests scores with `topK=len(atoms)` because a small global top-K can be consumed by mandatory or context-ineligible atoms ([internal/prompt/selector.go:781–803](https://github.com/theRebelliousNerd/codenerd/blob/2e0c54ef204997c36e65c91cbb90bc991e63c8c1/internal/prompt/selector.go#L781-L803)). `topKEligibleVectorScores` then filters, computes a mean-plus-1.5-standard-deviation floor when at least eight eligible scores exist, sorts and caps ([internal/prompt/selector.go:227–317](https://github.com/theRebelliousNerd/codenerd/blob/2e0c54ef204997c36e65c91cbb90bc991e63c8c1/internal/prompt/selector.go#L227-L317)). The exact backend search cost is separate; this source establishes the request size and selector work, not that every backend scans every embedding.

An eligibility-aware vector interface could avoid transferring/scoring ineligible rows, but must preserve the same eligible score population and threshold computation. Fetching only the top few eligible scores changes the mean/variance population and therefore changes policy. Treat that as a separately evaluated ranking change, not a semantics-preserving speed patch. If an approximate index cannot provide the baseline score population, label the approximation and measure missed useful atoms and downstream repairs. Existing no-vector/degraded behavior and capability checks remain controls.

## 5 Render once and budget the actual fragments

The compiler already checks the assembled prompt, which is important. The remaining opportunity is to reduce repeated work and estimation mismatch. `assembleSectionWith` expands per-atom templates, while `assembleWithConfig` applies final template processing again ([internal/prompt/assembler.go:268–270,304–323](https://github.com/theRebelliousNerd/codenerd/blob/2e0c54ef204997c36e65c91cbb90bc991e63c8c1/internal/prompt/assembler.go#L268-L323)). `ShedToFit` can call assembly again over surviving atoms on successive bounded passes ([internal/prompt/budget.go:1049–1135](https://github.com/theRebelliousNerd/codenerd/blob/2e0c54ef204997c36e65c91cbb90bc991e63c8c1/internal/prompt/budget.go#L1049-L1135)). These are bounded loops, not an unbounded quadratic claim.

Proposed staged renderer:

1. Compile atom templates into immutable plans at catalog creation. Keep their source hash and parser/version identity.
2. Declare the context fields each plan reads. Render each selected atom/mode once for the captured context; reuse that fragment during budget shedding rather than invoking template functions repeatedly.
3. Account for headers, separators and tier order. Keep whole-atom/dependency obligations and mandatory-skeleton refusal. Do not trim arbitrary bytes to manufacture a fit.
4. Preserve or deliberately migrate the current second-pass template semantics. `TemplateEngine.Process` iterates the registered function map and substitutes placeholders ([internal/prompt/assembler.go:496–517](https://github.com/theRebelliousNerd/codenerd/blob/2e0c54ef204997c36e65c91cbb90bc991e63c8c1/internal/prompt/assembler.go#L496-L517)); nested placeholders or placeholders inside inserted data need an explicit rule. An optimization must not quietly turn source text into instructions or change expansion order.
5. Validate the final serialized request independently. The JIT helper `EstimateTokens` is `(len(content)+3)/4`, a heuristic rather than model tokenization ([internal/prompt/atoms.go:265–274](https://github.com/theRebelliousNerd/codenerd/blob/2e0c54ef204997c36e65c91cbb90bc991e63c8c1/internal/prompt/atoms.go#L265-L274)). A provider/tokenizer-specific count or a documented conservative bound must include chat wrappers, tools, history and output reservation at the request owner. This review does not assert that no downstream request-budget checks exist.

Per-fragment token counts are useful estimates but may not sum exactly under a tokenizer that merges across boundaries. Retain a final full-request check; cache counts by exact bytes and tokenizer/model identity. Distinguish local compilation-cache hits from provider prompt-prefix-cache hits. Stable atom ordering is already implemented, and nothing here warrants rearranging instruction priority merely to maximize a provider cache metric.

## 6 Full cost model and adoption order

Let F be snapshot facts, B their serialized bytes, A candidate atoms, T tags/dependency edges, R rendered bytes, J simultaneous same-identity callers, and P budget reassembly passes. A source-derived planning model for the current path is:

```text
per call preflight = clone + context assertion/evaluation + QueryAll
                  + serialization(B) + canonical record sort(F) + specialist resolution
cache miss work   = source loading(A,T) + knowledge/learning lookup + vector work
                  + fact construction/parsing + Mangle selection
                  + dependency resolution + fit + assembly/reassembly(P,R)
return work       = private result copying
```

With J identical callers, singleflight can share miss work while preflight remains paid per caller. This is not a measured J-fold speedup opportunity: memory bandwidth, locking, context differences and source churn determine what is shareable. A generation/catalog design adds publication, reference counting, invalidation, retained-memory and cold-build costs. Count those explicitly.

Record separate metrics for caller latency, shared-work CPU, clone bytes, snapshot rows and bytes, source-query rows, fact parsing, kernel evaluations, vector requests, rendered bytes, reassembly count, allocations, peak retained generations and cancellation cleanup. Report cache misses, hits and joiners separately. Attach this to the existing manifest/decision-receipt direction rather than inventing a rival telemetry system.

At the complete-system level count all provider input/output and retries, keeping cached/reasoning subcounts non-additive where provider totals already include them. Include embedding requests, host CPU, memory, user waiting, repair turns, failed tasks and human review. Fewer prompt tokens can lose if the missing guidance costs another incorrect edit or test failure.

Recommended sequence:

| Research item | Narrow deliverable | Why first or later |
|---|---|---|
| JIT-REUSE-01 | Source generation vector and old-work publication isolation | Correctness prerequisite; prevents optimizing stale results. |
| JIT-REUSE-02 | Full per-call preflight metrics and cold/hit/joiner separation | Establishes which cost dominates rather than assuming Mangle or vectors do. |
| JIT-REUSE-03 | Immutable generation catalog and cached typed metadata | Reuses context-independent work while preserving Mangle authority. |
| JIT-REUSE-04 | Shared snapshot/fingerprint preparation with owned overlays | Larger lifecycle change; retain conservative fallback and existing isolation. |
| JIT-REUSE-05 | Rendered-fragment reuse with final request accounting | Avoids repeated expansion and makes budget tradeoffs legible. |
| JIT-REUSE-06 | Eligibility-aware vector retrieval | Adopt only with threshold-population and recall evidence. |

These IDs are proposal-local work items, not assertions that owning specs have adopted them. No code or spec originals are changed by this document.

## 7 Proposed evaluation and failure controls

No evaluation below was run. Compare the current production path with one change at a time, then the combined design. Keep selected atoms, render modes, effective tools, model/version, permissions, source snapshots and task-verification criteria fixed for semantics-preserving comparisons.

| Fixture | Required evidence |
|---|---|
| Identical context and immutable source vector | Prompt bytes/selected set match; repeated work falls in measured counters, not merely a renamed cache hit. |
| Same context, changed/retracted kernel evidence | No old evidence served as current; fallback if snapshot unavailable. |
| Old compile paused before cache insertion, corpus replaced | Old generation cannot populate or satisfy the new generation cache. |
| In-place project/expert/knowledge/learning change | Relevant source generation changes and refresh occurs; selected expert never leaks sibling data. |
| Optional source outage then recovery | Degraded result marked; bounded recovery probe can replace it without requiring unrelated context churn. |
| Configuration/capability revocation during work | No new-generation hit on old settings; downstream current authorization remains mandatory. |
| J same-key callers with mixed cancellation | Correct singleflight ownership, no leaked scopes; one caller cannot cancel another's required work. |
| New rule introduces a previously untracked predicate dependency | Projection invalidates or falls back; no stale negative/conjunction result. |
| Large corpus with mandatory and conflicting candidates | Staging/prefilter preserves complete obligations and winners; caps are observable. |
| Ineligible atoms dominate global vector ranks | Eligible relevant atom remains reachable; new backend does not repeat the old global-top-K starvation. |
| Score population around eight-candidate threshold | Exact-baseline mode preserves threshold; approximate mode reports selection changes. |
| Long placeholders, Unicode/code, nested placeholders, tiny budgets | Final output obeys declared counting contract; mandatory instructions whole or compilation fails. |
| Reset/restart/source replacement reuses small counter values | Incarnation/source identity prevents generation aliasing. |

Sweep snapshot size and corpus size independently; vary cold/warm state, mutation rate, same-key versus mixed-key concurrency, selected expert count, retrieval latency and template expansion. Do not measure only an empty fake kernel or a repeated one-line prompt. Use registered factory/scope/compiler/assembler integration plus the actual request boundary.

For capability, pair tasks requiring different languages, tool envelopes, phase-specific guidance, expert knowledge and changed code evidence. Include multi-turn feature wiring, test repair and large-refactor tasks. Randomize paired assignments, report uncertainty and failures, and hold total budgets fixed. Primary quality measures are verified completion, false completion and human repair; efficiency measures are total resource cost and latency at that quality. A faster compiler whose smaller candidate set misses required guidance is a failed optimization.

## 8 Primary foundations and limits

* [Mokhov, Mitchell and Peyton Jones, Build Systems à la Carte, §4.2.2](https://simon.peytonjones.org/assets/pdfs/build-systems-original.pdf): verifying traces associate reusable results with dependency information. This motivates staged JIT dependency identity, not a claim that the compiler is already a correct incremental build system.
* [Go singleflight documentation](https://pkg.go.dev/golang.org/x/sync/singleflight): duplicate suppression is scoped to the keyed function invocation. It does not share work done before that call, certify the key, or provide persistent caching.
* [SQLite PRAGMA data_version documentation](https://www.sqlite.org/pragma.html#pragma_data_version): its connection-local comparison semantics constrain invalidation design. A pooled connection handle is not automatically a stable source-generation oracle.

These are established mechanisms applied to concrete CodeNERD seams. No novelty, measured speedup, token reduction or superiority claim is established. The goal is to make the same trustworthy JIT decision with less repeated work, and to improve the decision only through a separately measured quality contract.

## As built (2026-10-27)

Verified against current source first: §§2–5 still held. The cache key had no source generation, `clearPromptCache` cleared without fencing in-flight publishes, every hit cloned and walked the kernel, project/expert atoms were re-joined on every miss, selector metadata was rendered to Mangle text and re-parsed per compile, and placeholders were expanded per atom, again over the whole prompt, and again on every shed pass. `compiler.go` (1,971 lines) is now split into `compiler{,_cache,_generation,_kernel,_pipeline,_preflight,_render,_result,_sources,_stats}.go`.

**Implemented**

| Item | Where | What |
|---|---|---|
| JIT-REUSE-01 source vector | `internal/store/source_generation.go#ReadSourceGeneration`; `internal/prompt/compiler_generation.go#captureSourceVector`; `compiler_pipeline.go#compileMiss` | Owned, trigger-maintained version rows that commit with the write, from any connection or process. Each token includes the schema version, and the read self-repairs missing triggers. The vector covers: a registration epoch, the project DB, the selected expert DB (its prompt atoms plus knowledge), LocalStore knowledge and vectors, every LearningStore shard, the evolved-set generation, and `StrategyProvider.SourceGeneration`. The key carries the vector captured before collection; the result is published only if a re-capture matches, every source is tracked, and nothing degraded. |
| Degraded/untracked contract | `compiler_generation.go#degradation`; `compiler_db.go#collectKnowledgeAtomsFrom`, `#collectLearningAtomsFrom`; `compiler_sources.go` | Lookup errors and timeouts (not empty results) mark the compile degraded. A degraded result is returned but not cached, so every identical call re-probes. An unreadable generation means no lookup and no publish. |
| JIT-REUSE-02 metrics | `compiler_stats.go#compilerCounters`, `CompilerStats`, `CompilationStats.{PreflightMs,PreflightReused,DegradedSources,Uncacheable}`; `cmd/nerd/cmd_advanced.go` (`nerd jit`) | Hits, joiners, preflights (count and mean), memo reuses, catalog reuses, uncacheable/degraded/churn counts, all separate from the compile average. |
| JIT-REUSE-03 catalog + typed metadata | `compiler_generation.go#atomCatalog`; `selector_facts.go#appendCandidateFacts` | Project and expert atoms are reused per handle and generation, installed only after revalidation. Candidate facts are typed `Fact`s whose name constants are `types.MangleAtom`; static facts are cached per immutable atom, and only `is_mandatory`/`prompt_atom` are built per compile. `KernelAdapter.AssertBatch` takes them without parsing. Mangle still selects. |
| JIT-REUSE-04 shared preflight | `internal/core/kernel_generation.go#SnapshotIdentity`, `#CloneWithIdentity`; `internal/system/factory_adapters.go#SnapshotIdentity`; `internal/prompt/compiler_preflight.go` | The kernel generation advances in `markDirtyLocked`, alongside a per-kernel instance id. The identity is read under the same lock as the clone. It is `Reusable` only when no rule reads an external predicate and the program is analysed. The dynamic snapshot is memoized by identity, context facts, shard and config revision, so a hit over an unchanged kernel clones nothing. A miss acquires its scope and re-snapshots if the scope's identity differs. |
| JIT-REUSE-05 render once | `assembler.go#RenderSession`, `#expandTemplate`; `budget.go#TokenCounter`; `compiler_render.go#fitAndAssemble` | One session per compile renders each (atom, mode) fragment once. Fit and ShedToFit charge `max(declared, rendered)`, and reassembly reuses fragments. The final whole-prompt check is kept. |

**Ruled differently**

- **No snapshot handle or ownership lease.** This follows the doc's own "first implementation": a generation number memoizes the fingerprint, and selection still runs on private scopes.
- **Source generations come from trigger-maintained rows, not `PRAGMA data_version`.** This is the doc's preference, and it covers writes from other processes.
- **Latest-at-return semantics.** A result whose sources moved during the compile is returned to its caller, marked uncacheable. There is no retry, so a source that changes constantly cannot cause an endless recompile loop.
- **The embedded corpus and vector searcher are not in the vector.** They are construction-time constants of a compiler instance.
- **Template placeholders expand in one pass.** It is deterministic and runs left to right over source text. Inserted values are never re-scanned, which closes the map-order-dependent re-expansion of placeholders inside data such as the specialist roster. The assembler's second whole-prompt pass is removed.
- **No new knob.** Memo, catalog and fact-cache bounds are memory ceilings, not tunables, and an evicted entry is recomputed rather than served wrong.
- **Found along the way:** `core.sanitizeFactForNumericPredicates` coerced numeric arguments in the caller's `Args` slice. Once typed fact slices were shared across concurrent compiles, that became a data race. It now coerces a private copy (`internal/core/kernel_facts_ownership_test.go`).

**Tests**

- `internal/store/source_generation_test.go`
- `internal/core/kernel_generation_test.go`
- `internal/prompt/compiler_generation_test.go`: in-place write, catalog, degraded→recovery, mid-compile churn, untracked, registration epoch.
- `internal/prompt/compiler_preflight_test.go`: no clone on hit, kernel write, non-reusable, memo→miss.
- `internal/prompt/compiler_render_test.go`: no re-expansion, render once, rendered charge, typed facts equal their parsed text over the whole shipped corpus.
- `internal/system/prompt_preflight_reuse_test.go`: production adapter and real kernel, end to end.

**Deferred**

- **JIT-REUSE-06 eligibility-aware vector retrieval.** The doc requires threshold-population and recall evidence first, and gathering it needs live embedding backends.
- **Analyzed dependency projection** (§2.4). Whole-kernel generation stays the conservative identity.
- **Provider-tokenizer counting at the request owner** (§5.5). It is outside this package.
- **The §7 capability and latency sweeps.** They need live models and workloads.
- **Cross-process staleness in `StrategyStore`'s 5-minute in-memory cache.** It predates this change: the source vector moves, but the store may answer from its cache until the TTL expires.
