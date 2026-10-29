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

## Appendix: snapshot identity must advance independently of evaluation dirtiness

**Recommendation:** keep the implemented generation-aware preflight optimization, but make its kernel identity cover every successful change to the state a compilation scope can snapshot. A mutation that deliberately avoids an immediate fixpoint is still a mutation. This is a conformance gap in the existing §2 generation contract, not a proposal to add another cache or repeat the original reset requirement.

**Evidence boundary (2026-10-28):** static source inspection at main `356f65e0e6bb639adaa90ab36212daa714114871`; no code, tests, benchmarks, inference or profiling were run. The relevant `kernel_eval.go`, `kernel_generation.go`, and `compiler_preflight.go` blobs are identical on inspected `feat/conversation-brief` tip `c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c`. The existing audit and its research-branch counterpart at `0806d57a48fba303bdc2193faf28e634798facba` were checked: they already require reset/restore invalidation, but do not identify the mutation paths below. The current audit's [as-built section][epoch-as-built] is credited: ordinary assertions/retractions, per-instance identity, atomic clone-plus-identity capture, private selection scopes, source-vector publication fences and the memo are implemented. The findings concern the exceptions to the identity funnel, not an absence of that implementation.

### 1. The exact invariant and the three exceptions

[`SnapshotIdentity`][epoch-identity] certifies reuse from `(Instance, Generation, Reusable)`. `Reusable` currently tests program dirtiness, presence of the dependency index, and absence of external-premise heads. It does not test whether every intervening EDB/store change advanced the generation. The comment says every write, retract and clear passes through `markDirtyLocked`; that helper [does increment the generation][epoch-mark]. Three inspected paths do not pass through it:

| Transition | What actually changes | What the identity misses |
|---|---|---|
| `Clear` / `Reset` | [`clearFactsLocked`][epoch-clear] replaces the EDB, atom cache, dedupe index and evaluated store, and sets `initialized=false`. It deliberately leaves `factsDirty` as-is. | Instance and generation remain unchanged; a previously reusable analyzed program still reports reusable. This remains true after a successful read re-materializes the cleared kernel. |
| Successful nonduplicate `AssertWithoutEval` | [The method][epoch-stage] adds the admitted fact to the EDB/cache/index and records its predicate in `dirtyPreds` when `dirtyAll` is false, without setting `factsDirty`. | No generation increment. [`EvaluatePending` / `Evaluate`][epoch-evaluate] subsequently evaluate and clear the dirty state but do not increment it either. The pre-batch and post-batch worlds therefore retain the same identity. Rejections and duplicate no-ops are not this case. |
| Changed timestamp for an existing heartbeat | [`assertHeartbeat`][epoch-heartbeat] replaces the stored fact, updates the evaluated store when possible, repairs the atom cache, and replaces the dedupe entry. Avoiding re-evaluation is deliberate. | No generation increment. A successful normal refresh can change a row visible through `QueryAll` while retaining the same reusable identity. Same-timestamp no-ops are not this case. |

This distinction matters because the memo is not just a cache of derived executive answers. The fresh [dynamic-snapshot computation][epoch-fingerprint] includes **every declared row returned by `QueryAll`**, together with the injected atoms, in its fingerprint. Thus even a heartbeat change that preserves all current health conclusions changes the value being memoized. Do not silently redefine a whole-snapshot fingerprint as an existence-only health fingerprint.

### 2. Why the JIT consumer cannot repair the missing identity

The production [`KernelAdapter`][epoch-adapter] forwards the real kernel's identity and captures a clone plus its base identity under one lock. That correctly prevents a torn clone/identity pair; it cannot distinguish two different states assigned the same identity.

On a memo hit, [`runPreflight`][epoch-preflight] returns the previous dynamic atoms and fingerprint without cloning. The [compiler pipeline][epoch-pipeline] then looks up the prompt cache and can return before acquiring a selection scope. A prompt-cache miss is not sufficient recovery: [`ensureSelectionScope`][epoch-scope] acquires a fresh scope but deliberately skips snapshot reconstruction when its base identity equals the memo's identity. In that branch, selection can use a current private kernel while `in.dynamicAtoms` still describes the older world.

These are conditional source-derived counterexamples, **not observed session failures**:

1. **Reset with a warm prompt cache.** Start with an analyzed kernel whose identity is reusable. Admit an `injectable_context` row for shard `/_all` with content `OLD_EVIDENCE`, using the declaration-compatible fact shape, and compile successfully so both memo and prompt entry exist. Reset that same kernel. Compile again with identical context/configuration and unchanged tracked non-kernel sources, **without any intervening generation-advancing mutation**. The memo key remains equal; if the prompt entry is retained, the old prompt can satisfy the call even though the host EDB was cleared. A fresh uncached snapshot would not contain that EDB witness. Program initial facts are intentionally retained through re-evaluation and must not be confused with the removed witness.
2. **Committed staged addition.** Warm the memo, then successfully stage a new `injectable_context` row through `AssertWithoutEval` and call `EvaluatePending` or `Evaluate`. A subsequent identical compile can reuse the pre-addition snapshot because neither operation moved the generation. This counterexample remains after explicit materialization; it is not merely an argument about whether staged rows should be visible before evaluation.
3. **Prompt miss with a retained memo.** Repeat either transition, but arrange for the prompt entry alone to be absent while the corresponding memo remains. The fresh scope has the same identity, so the scope guard does not recompute the dynamic snapshot. The old dynamic atoms can then be supplied to a new compilation. The proposed evaluation must distinguish this from the simpler complete-cache-hit case.
4. **Heartbeat fingerprint mismatch.** Warm a snapshot including one heartbeat; refresh its timestamp normally; compare memo reuse with a forced fresh snapshot under the same compilation context. The source predicts equal reported identity but different full-snapshot fingerprint. This establishes an identity-contract mismatch; it does **not** establish that the shipped prompt text or health verdict changes.

### 3. Production reachability, with bounded impact claims

* **Reset is reachable:** the [chat reset command][epoch-reset-command] calls `m.kernel.Reset()` and reports cleared facts; [Cortex reset][epoch-cortex-reset] forwards to each shard. That proves the mutation exists on a production entry path, not that ordinary next-chat compilation serves stale content. The inspected [ordinary chat state-update path][epoch-chat-update] calls `UpdateSystemFacts`, which [replaces `current_time` in a kernel transaction][epoch-time-update], as well as doing other state hydration. An intervening generation-advancing mutation breaks the counterexample's equal-identity condition. No stale post-reset user conversation is claimed here.
* **Staged writes are reachable:** [Dreamer construction and kernel replacement][epoch-dreamer-set] seed critical-path facts through [`AssertWithoutEval` plus `EvaluatePending`][epoch-dreamer-stage]. Projected facts also use staging on a private clone. Boot-before-first-compile and short-lived speculative clones need not encounter an already-populated JIT memo. The demonstrated gap is the public mutation/identity contract; a user-visible impact additionally requires reuse of a kernel/compiler lifetime with a warmed memo and no later invalidating write.
* **Heartbeat refresh is a real recurring producer:** [`BaseSystemShard.EmitHeartbeat`][epoch-heartbeat-producer] calls ordinary kernel `Assert` with the current timestamp. The shipped [health rule][epoch-health-rule] tests only heartbeat existence. This appendix does not claim a new timestamp-based health bug, and does not propose removing the no-reevaluation optimization merely to fix cache identity.

The [existing real-kernel/production-adapter integration test][epoch-integration-test] supplies a useful starting fixture: it establishes a reusable identity, warms a cache and then checks a normal `Assert` of `injectable_context`. The [generation tests][epoch-tests] cover normal assertion/retraction, clone identity/isolation, external premises and pending program changes. Neither inspected file covers reset, staged publication or heartbeat refresh. This is test-source coverage analysis, not a report that any test passed or failed during this research.

### 4. Proposed mutation contract

Keep **state revision** separate from **fixpoint scheduling**:

1. **Identity names the clone input.** Every successful change to EDB values or the analyzed-program inputs advances a monotone state revision under the existing write lock. Equality of reusable identities must imply equality of everything the current fingerprint reads, not merely equal dirty flags. New instances retain distinct incarnation IDs.
2. **Normal writes preserve existing scheduling.** Existing assertion/retraction batches can advance once per committed mutation boundary. Rejected operations and exact no-ops do not need a revision. Do not introduce extra evaluations merely to advance identity.
3. **Staging changes identity immediately.** `AssertWithoutEval` already changes the EDB a clone copies. Advance state identity on each accepted change, or introduce a genuine owned batch/publication boundary with an equivalent guarantee. Do not pretend the existing API is an invisible transaction. Explicit evaluation without any input change should not need another state revision solely to fix the staged-write omission.
4. **Reset retires the previous state atomically.** Empty the host EDB, invalidate materialization, advance the state revision and require the next evaluated read to reconstruct from retained policy/program facts. Preserve lazy behavior rather than doing an eager full fixpoint inside reset. Keep the existing compiled-program and immutable cone-plan reuse.
5. **Heartbeat identity and heartbeat evaluation are different choices.** A changed admitted timestamp advances whole-snapshot identity while retaining the current no-dirty scheduling behavior. If the extra memo invalidations are too costly, implement an explicitly narrower, analyzed fingerprint dependency projection as the existing audit proposes; do not exempt a field while continuing to hash it in the reference path. Any projection must account for newly admitted rules, negative dependencies and injected-context producers.
6. **Cache guards stay defense in depth.** Preserve clone/base-identity capture, source-vector checks, private results and downstream current permission checks. Returning `Reusable=false` on unresolved lifecycle states is a conservative fallback, not a substitute for advancing identity when the changed state becomes reusable again.

An adjacent reset lifecycle deserves a focused regression: [`ExplainEnvelope`][epoch-explain] calls `ensureEvaluated`, which [returns immediately for a clean dirty flag][epoch-ensure], whereas ordinary [query code][epoch-query-reset] has a separate `!initialized` recovery. For a previously recorded kernel with `factsDirty=false`, reset retains the old recorder but installs an empty store. This witness requires no intervening query, evaluation or dirtying mutation, and a recorded derived goal depending on a removed EDB premise. CodeNERD requires `codeberg.org/TauCeti/mangle-go v0.7.0` ([go.mod](https://github.com/theRebelliousNerd/codenerd/blob/356f65e/go.mod#L5-L8)); the linked source identifies the same module at the v0.7.0 tag. The pinned upstream [recording-based builder][epoch-proof-builder] reads recorded root events even when the new store no longer supplies their premises, marking missing support partial. Static inspection therefore predicts the possibility of an old partial derivation being returned with the prior recorder generation if an envelope was obtained before reset rather than forcing post-reset evaluation. This was not executed. Keep proof/store retirement consistent with reset, without turning this appendix into a new proof-format project.

### 5. Cost and proposed evaluation

The direct revision update is constant-size work under a lock already held by the mutation. The important cost is **additional correct invalidation**, especially for recurring heartbeats: a preflight miss can pay clone copying, context evaluation, full `QueryAll`, serialization and sorting again. A revision fix is not a speedup claim. Measure the increase rather than hiding it by omitting mutable input from the identity. Preserve the existing small-cone/full-fixpoint decisions, no-eager-evaluation staging behavior and bounded memo.

No checks in this table were executed. Use deterministic local fixtures and the production adapter; no live model is required to establish the identity invariant.

| Proposed fixture | Required observations |
|---|---|
| Warm memo + prompt; `Clear` and `Reset`, independently | Identity changes before another write; next compile does not reuse removed EDB evidence; retained program initial facts still derive. |
| Reset followed by an ordinary query, then compile | Lazy query recovery cannot restore the pre-reset identity; no old memo becomes valid again. |
| Warm memo; successful staged addition; explicit full or pending evaluation | Identity differs before/after the batch; new injected evidence appears; existing no-immediate-evaluation contract is preserved. |
| Duplicate/rejected staging controls | No false successful admission, no evidence injection, and no unnecessary invalidation claimed as a mutation. |
| Prompt entry evicted/cleared while memo remains | Miss-side scope identity cannot bless stale dynamic atoms; reference forced snapshot and compilation agree. |
| Valid changed heartbeat, same timestamp, first heartbeat, several shard names | Whole-snapshot identity/fingerprint equivalence; changed refresh does not force a live full fixpoint; exact no-op remains cheap. Do not require prompt bytes to change when only the fingerprint input changes. |
| Reset while a previous compile owns a clone | Old work may complete only for its captured identity; it cannot satisfy a post-reset identity. Keep current source-vector and cancellation ownership controls. |
| Recording evaluation, reset, then `Explain`/`ExplainEnvelope` before any ordinary query | Proof recorder and store describe one current evaluated state; no old recorder generation is advertised as current after reset. |
| Ordinary `Assert`, retract, policy replacement, external-premise program and clone isolation | Existing invalidation/fallback behavior preserved; permission and ownership semantics unchanged. |

For performance, compare the present implementation, identity-complete implementation, and a forced-fresh reference under the **same accepted facts and query semantics**. Vary world size, staged batch size, heartbeat frequency, compile frequency, prompt-cache eviction and concurrent snapshot readers. Report preflight reuse/miss counters, clone bytes, evaluations by mode, latency, allocations and peak retained memory. Separate prompt-byte changes from fingerprint changes, and separate a required refresh from an avoidable full fixpoint. Any future dependency projection must first match the complete-snapshot oracle for the inputs it claims to cover and disclose its intentionally narrower contract.

**Checkable exit:** all public mutation families have an explicit identity/scheduling classification; paired equal reusable identities produce equal reference snapshots for an unchanged context/configuration; warm-hit and memo-only-miss fixtures cannot reuse pre-reset or pre-staging evidence; heartbeat refresh retains its no-live-reevaluation performance property. Update the owning core/prompt specification and the existing audit's as-built claim only after those checks are implemented and run. No new permission cache or authorization relaxation is justified.

[epoch-as-built]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/Docs/audits/jit-generation-aware-optimization.md#as-built-2026-10-27
[epoch-identity]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/kernel_generation.go#L5-L65
[epoch-mark]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/kernel_eval_cone.go#L218-L238
[epoch-clear]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/kernel_eval.go#L472-L505
[epoch-stage]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/kernel_facts.go#L720-L747
[epoch-evaluate]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/kernel_facts.go#L750-L788
[epoch-heartbeat]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/kernel_facts.go#L564-L618
[epoch-fingerprint]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/prompt/compiler_kernel.go#L162-L218
[epoch-adapter]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/system/factory_adapters.go#L91-L135
[epoch-preflight]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/prompt/compiler_preflight.go#L38-L59
[epoch-pipeline]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/prompt/compiler_pipeline.go#L93-L147
[epoch-scope]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/prompt/compiler_preflight.go#L107-L127
[epoch-reset-command]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/cmd/nerd/chat/commands_handlers_files.go#L19-L39
[epoch-cortex-reset]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/cortex_kernel.go#L808-L817
[epoch-dreamer-set]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/dreamer.go#L37-L48
[epoch-dreamer-stage]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/dreamer.go#L103-L162
[epoch-heartbeat-producer]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/shards/system/base.go#L772-L792
[epoch-health-rule]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/defaults/policy/system_core.mg#L103-L106
[epoch-integration-test]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/system/prompt_preflight_reuse_test.go#L11-L65
[epoch-tests]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/kernel_generation_test.go
[epoch-explain]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/kernel_explanation.go#L84-L123
[epoch-ensure]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/kernel_eval.go#L430-L455
[epoch-query-reset]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/kernel_query.go#L39-L57
[epoch-proof-builder]: https://github.com/google/mangle/blob/7cd2543ac1817f8b92d42efe3c6ef82dededbeeb/provenance/recorder.go#L209-L327
[epoch-chat-update]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/cmd/nerd/chat/process.go#L713-L731
[epoch-time-update]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/kernel_sysfacts.go#L20-L38

## Post-implementation refinement: vector scores must share the compiler’s source scope

**Recommendation:** bind semantic scoring to the same captured, canonical candidate sources that determine prompt content and cache identity. A fixed searcher object is not an immutable data source when its `Search` method reads changing databases. Also propagate selector degradation into the existing publication ledger; source-collection health does not certify selection health.

**Evidence status:** static source inspection only; no implementation, tests, provider calls, profiling, or benchmarks were executed. Main was `356f65e0e6bb639adaa90ab36212daa714114871`; feature branch `feat/conversation-brief` was `c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c`. The relevant compiler-generation, source-collection, pipeline, selector, and vector-searcher files have identical blobs at those revisions. Independent source review confirmed the scope/collision/truncation chain and its controls; this is not behavioral verification.

### Scope and relationship to earlier research

The original sections already require complete cache dependency identity, warn against global top-K truncation before eligibility filtering, and require recovery from degraded compilation. This refinement does not claim those principles are new. It identifies concrete discrepancies in the implemented path: **candidate collection and generation tracking are expert-scoped, while the default vector backend searches every registered expert database; selector degradation also does not reach the new publication ledger.**

The historical literal-rendering and section-admission appendices concern different boundaries. The [embedding representation audit](https://github.com/theRebelliousNerd/codenerd/blob/c592ff223cd207fca4c29ae793d5eba57aa4781d/Docs/audits/embedding-representation-and-migration.md) addresses model/representation compatibility; the cross-expert finding remains possible with correctly stamped, same-model, equal-dimensional vectors. This appendix belongs with the existing generation-aware audit rather than a duplicate general caching proposal.

### 1. The two source scopes disagree

| Stage | Inspected behavior |
|---|---|
| Candidate bodies | Embedded atoms take precedence over project atoms, followed by the selected expert. Duplicate IDs from later sources are discarded. Other experts’ atom bodies are not collected. [Source](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/prompt/compiler_sources.go#L32-L95) |
| Source identity | `snapshotSources` captures only the expert named by `CompilationContext.ShardID`; `captureSourceVector` reads that expert’s generation and the project generation. [Source](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/prompt/compiler_generation.go#L97-L140) |
| Vector backend | `Search` independently calls `snapshotPromptDBs`, which returns the project database plus **all** registered expert databases. [Search](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/prompt/vector_searcher.go#L55-L99), [database enumeration](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/prompt/vector_searcher.go#L200-L227) |
| Score identity | The query selects atom ID, embedding, and model stamp. It checks model/dimension compatibility, then keeps the greatest score for each bare atom ID across databases. It does not compare the scored row’s content identity with the canonical candidate body. [Source](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/prompt/vector_searcher.go#L91-L138) |
| Candidate reduction | The backend sorts and truncates globally. Only afterward does the selector restrict scores to eligible flesh candidates and compute its population-dependent floor. [Backend](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/prompt/vector_searcher.go#L164-L185), [selector](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/prompt/selector.go#L227-L317) |

This is a production path when embeddings are enabled: the factory constructs and attaches `CompilerVectorSearcher`, then registers every discovered agent database. [Construction](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/system/factory.go#L1796-L1806), [attachment](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/system/factory.go#L1846-L1854), [registration](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/system/factory.go#L1913-L1926).

The source-generation comment exempts the searcher because it is a construction-time constant. That describes the object’s configuration, not the databases its method subsequently reads. [Source](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/prompt/compiler_generation.go#L15-L32)

### 2. Conditional cross-expert witnesses

These are proposed source-derived cases, not reproduced incidents.

#### A. A different expert supplies a canonical candidate’s score

Compile for expert A. Candidate `x` belongs to the embedded/project/A precedence winner. Another registered expert B contains the same atom ID but a different body and a higher-scoring embedding. Both embeddings pass model and dimension checks.

The backend’s maximum-by-ID merge can attach B’s score to the canonical body of `x`. The selector then emits `vector_hit` using that ID and uses the score in the selected atom’s combined ranking. [Vector facts](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/prompt/selector.go#L1105-L1127), [selected-score mapping](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/prompt/selector.go#L1165-L1204)

For an observable prompt difference, `x` must be an eligible, nonmandatory, score-sensitive candidate, with the changed score crossing a selection threshold/top-K boundary or affecting budget competition. A mandatory atom already retained regardless of score is a negative control.

**Bounded claim:** this is score/body provenance mismatch, not proof that B’s text is emitted. Candidate collection still prevents B-only bodies from entering this compilation. It is not an execution-permission bypass.

#### B. A changing score dependency is absent from cache identity

Warm a successful compilation for A, then update only B’s existing vector row in place. Keep registration, context, kernel fingerprint, compiler configuration, project/A sources, and other tracked sources unchanged.

B’s update is not included in A’s source vector. The cache may therefore return the earlier result before vector search runs. A forced-fresh compile can observe the changed B score under the same tracked identity. The post-compile recapture also omits B, so it cannot detect this dependency changing during compilation. [Cache hit](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/prompt/compiler_pipeline.go#L105-L125), [publication](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/prompt/compiler_pipeline.go#L341-L366)

Credit the existing guard: `RegisterShardDB` advances the global source epoch and clears the cache. Therefore the narrow witness requires **an in-place update without re-registration**, not merely adding or replacing B through that API. [Source](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/prompt/compiler_db.go#L213-L234)

Ordinary turns can change kernel/context identity; no claim is made that every next chat turn serves a stale prompt. The invariant to establish is equal tracked identity versus unequal actual scoring inputs, not an estimated incident rate.

#### C. Unselected IDs can consume the backend’s result allowance

Let `N` be the number of candidates supplied to the selector. It requests `N` scores to avoid starving eligible atoms behind ineligible candidates. However, the backend searches a larger database union. If that union contains at least `N` higher-scoring IDs absent from this compilation’s candidate set, global truncation can remove a useful current candidate before eligibility filtering.

No duplicate IDs are required for this case. The selector can discard irrelevant returned IDs, but cannot recover a candidate whose score was already truncated. The missing scores can also alter the population used for the mean-plus-standard-deviation floor. [Request size](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/prompt/selector.go#L784-L804)

The bound is conditional on the registered union actually exceeding the local allowance with sufficiently high-scoring outside IDs. A small, disjoint expert corpus that cannot fill that allowance is a negative control, not evidence against the mechanism.

### 3. Additional bounded finding: selector degradation bypasses the publication ledger

The as-built implementation correctly creates a degradation ledger and supplies it to candidate, knowledge, and learning collection. Those collectors can prevent caching when they report an outage. The selector call does not receive that ledger and returns only selected atoms, vector timing, and an error. [Collectors](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/prompt/compiler_pipeline.go#L245-L277), [selection](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/prompt/compiler_pipeline.go#L300-L310)

`getVectorScoresWithConfig` logs a warning and returns nil scores after a search error or its own subdeadline expires. The selection pipeline can subsequently return successfully. Flesh assertion/query failures have other explicit fallback paths. None of those warnings is itself an entry in the compiler’s `degradation` object. [Vector outcome erasure](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/prompt/selector.go#L917-L947), [flesh assertion fallback](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/prompt/selector.go#L843-L861), [flesh query fallback](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/prompt/selector.go#L1140-L1163)

**Proposed witness:** use healthy, generation-tracked collectors and stable context/kernel/configuration. Let only the vector search fail or exceed its subdeadline while the outer compilation context remains live; preserve a valid skeleton and successful remaining pipeline. A successful fallback result can have no recorded degraded source, satisfy the unchanged-vector publication check, and enter the normal LRU. Recover the searcher without changing any key component. An identical compile can hit that entry without probing recovery. Choose a score-sensitive atom to test prompt impact, but also assert the more direct observable: whether the recovered searcher is called. [Publication gate](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/prompt/compiler_pipeline.go#L341-L366)

This is not a claim of absent diagnostics: warnings already exist. It is an outcome-propagation gap in the existing degraded-result contract, not a new proposal to invent that contract. Outer cancellation, critical skeleton errors, unhealthy tracked source collectors, unrelated source churn, and cache eviction can prevent this exact sequence. A legitimately disabled vector searcher or a successful empty result must remain distinguishable from a transient failure.

The narrow repair is to return structured selection health alongside atoms and timing, then merge failure reasons into the existing publication ledger. Preserve optional fallback for the current caller where policy permits it, but do not represent that fallback as healthy cacheable work. The default backend also skips individual failed database queries and does not inspect `rows.Err()` after iteration; a full coverage contract must audit those lower-layer outcomes rather than assuming every backend failure reaches the top-level error. That is adjacent implementation work, not required to establish the explicit returned-error witness above. [Backend read loop](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/prompt/vector_searcher.go#L91-L139)

### 4. Proposed source contract and alternatives

For every semantic score used in a compilation:

1. Its candidate ID and content/representation identity identify the canonical candidate being ranked.
2. Its source belongs to the compilation’s permitted scoring scope.
3. Every mutable input capable of changing that score population is tracked in the captured source identity.
4. Truncation cannot discard members of the intended eligible population before that population’s threshold is computed.
5. A failed or incomplete scoring pass is distinguished from a successful empty or deliberately disabled pass, and reaches the existing cache-publication decision.

**Preferred direction:** give the vector search operation a compilation-owned scope derived from the captured sources and canonical candidate inventory. Do not have it rediscover global registrations midway through the compile. Restrict expert backing sources to the selected expert unless cross-expert retrieval is an explicit, separately specified capability.

Embedded atoms may legitimately use a synchronized project-database embedding. Require correspondence with the canonical candidate’s represented content, not necessarily the same physical storage location. Verify the existing digest’s semantics before treating it as a sufficient certificate; matching ID, model name, and dimension alone is insufficient.

Retain Mangle as the executive selector. Host filtering should establish source/candidate identity and preserve the intended score population, not replace conflict, capability, mandatory, or dependency policy with a new Go ranking rule. Preserve the complete eligible population used by the existing floor calculation; returning only a small eligible top-K before computing its mean/variance is a ranking change, not a transparent optimization.

**Alternative: deliberately retain global scoring.** Then capture and track every database it reads, qualify duplicate-ID score ownership, and prevent unrelated IDs consuming the current candidate allowance. This expands per-call generation reads and invalidation fanout. Tracking all databases alone fixes neither score/body mismatch nor truncation starvation.

**Conservative intermediate:** mark results uncacheable when scoring depends on untracked sources. This contains stale reuse but does not repair ranking provenance or unnecessary global work. A TTL is not a substitute for the dependency contract.

### 5. Costs and evaluation

The inspected backend performs a query embedding, scans vector-bearing rows across registered databases, decodes compatible vectors, computes cosine similarities, and sorts unique scored IDs. With `R` scanned rows, dimension `D`, and `U` scored IDs, a useful planning model is:

```text
T_vector ≈ T_query_embedding + T_SQL_scan(R)
           + T_decode_and_similarity(Rcompatible × D)
           + T_sort(U log U)
```

This is not a measured bottleneck ranking. Scoping can reduce irrelevant rows, decoding, sorting, and timeout exposure. If filtering happens only after SQL retrieval, it does not eliminate the full row scan. Candidate/content qualification adds bookkeeping and possible database lookup cost; batched indexed access needs its own query-plan and allocation evidence.

Tracking all experts instead increases generation-query work and invalidates unrelated prompts when any expert changes. Reusing an immutable search scope must also bound retained handles and preserve existing shutdown/cancellation ownership. Correctly marking selector degradation can increase retries and embedding-provider calls during outages; retain bounded deadlines and measure that cost. Any retry/backoff policy must preserve observable recovery rather than cache an outage indefinitely as a healthy result.

**Proposed checks; none executed:**

| Fixture | Required observation |
|---|---|
| Same ID, different body in project/A/B | Score provenance follows the canonical candidate; B cannot win merely through a larger score. |
| Warm A cache, in-place B vector update | Under scoped semantics, A is unchanged; under intentionally global semantics, dependency identity changes. |
| Register/unregister B control | Existing epoch invalidation remains effective. |
| Many higher-scoring B-only IDs | Current eligible candidate remains reachable; threshold uses the complete intended population. |
| Fewer than eight versus at least eight eligible candidates | Existing absolute/population-floor behavior is preserved. |
| Correct model/dimension but wrong candidate digest | Compatibility alone cannot qualify the vector. |
| Source replacement or update during search | Captured handles, content identity, generation checks, and result publication agree. |
| Only vector search fails, then recovers with unchanged key inputs | First result is explicitly degraded and not healthy-cacheable; later compile probes the recovered source. |
| Vector subdeadline versus outer cancellation | Optional timeout and caller cancellation remain distinct; cancellation does not become successful fallback. |
| Deliberately disabled search or successful empty result | No false transient-failure designation or needless recovery work. |
| Flesh assertion/query fallback, partial DB read, iteration error | Outcome reaches the appropriate ledger; do not infer full coverage from a nonnil result slice. |

Use deterministic fake embeddings, real SQLite fixtures, and the production kernel/adapter first; live inference is unnecessary to establish these identity and selection invariants. Exercise complete compilation, not just `Search`. Keep the actual default backend in the cross-expert tests; a replacement searcher that already honors scope would hide the finding. Existing model-compatibility and source-generation test definitions are useful controls, not evidence that these new witnesses were executed. [Model fixtures](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/prompt/vector_searcher_model_test.go), [generation fixtures](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/prompt/compiler_generation_test.go)

Then vary registered experts, unrelated rows, duplicate-ID rate, vector dimensions, candidate count, cache hit/miss ratio, source churn, and failure/recovery frequency. Report rows scanned/decoded, provider requests, source-generation reads, selected IDs, exact prompt differences, CPU, allocations, peak retained memory, and end-to-end compile latency. Preserve model, tools, budgets, policy, and held-out task acceptance when evaluating capability. Fewer scanned vectors or fewer prompt tokens alone do not establish improvement. Include failed attempts and repair cost, not only successful compiles.

**Checkable exit:** the source scope that names prompt content also names its scoring dependencies; canonical candidate identity survives vector retrieval; unrelated registered experts cannot silently alter or starve a compilation’s semantic candidate population; and selector fallback cannot bypass the existing degraded-result publication contract.

## Post implementation refinement for learned strategy health propagation

### Recommendation and evidence status

**Propagate unavailable or incomplete learned-strategy reads through the existing degradation gate.** Preserve current-turn fallback, but do not publish that fallback as a healthy compiled-cache entry. Distinguish legitimate empty input from failed input; neither mandatory strategies nor a blanket cache TTL is warranted.

This is a **static-source finding, inspected October 28, 2026**, not behavioral verification. No tests, fault injection, model calls, or benchmarks were executed; no latency or model-quality gains are established. Main `356f65e0e6bb639adaa90ab36212daa714114871`, feature `c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c`, and research `0c38d4fc987213a17f6570cc7d8dae34c87d8ace` have identical inspected blobs for `internal/prompt/{strategy_atoms,compiler_generation,compiler_pipeline,compiler_sources,compiler}.go`, `internal/autopoiesis/prompt_evolution/{strategy_atoms,strategy_store}.go`, and `internal/store/source_generation.go`.

### Relationship to the existing audits

The [as-built audit](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/Docs/audits/jit-generation-aware-optimization.md#L205-L237) broadly claims lookup errors/timeouts become degraded. This edge qualifies that claim and is distinct from its deferred cross-process StrategyStore-cache coherence issue. [Historical provider swallowing](https://github.com/theRebelliousNerd/codenerd/blob/c592ff223cd207fca4c29ae793d5eba57aa4781d/internal/prompt/strategy_atoms.go#L39-L60) and [success publication](https://github.com/theRebelliousNerd/codenerd/blob/c592ff223cd207fca4c29ae793d5eba57aa4781d/internal/prompt/compiler.go#L954-L986) predate the generation/degradation gate; that [earlier audit](https://github.com/theRebelliousNerd/codenerd/blob/c592ff223cd207fca4c29ae793d5eba57aa4781d/Docs/audits/jit-generation-aware-optimization.md#L95-L101) already specified a generic degraded-recovery contract. The [research audit](https://github.com/theRebelliousNerd/codenerd/blob/0c38d4fc987213a17f6570cc7d8dae34c87d8ace/Docs/audits/jit-generation-aware-optimization.md#L398-L408) covers selector degradation, not learned-strategy query, panic, or scan outcomes. This is a remaining outcome-propagation edge, not a new caching architecture.

### Source to cache failure chain

1. [Production wiring](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/system/factory_learning.go#L134-L141) registers the evolver provider. Its [real implementation](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/autopoiesis/prompt_evolution/strategy_atoms.go#L65-L93) logs `SelectStrategies` errors and returns nil atoms. This is not merely a fake-provider seam.
2. The [provider interface and collector](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/prompt/strategy_atoms.go#L26-L67) expose no error result; the collector also recovers provider panics, warns, and returns nil. [Source collection](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/prompt/compiler_sources.go#L73-L104) records project/shard degradation, but passes no degradation accumulator or reason through the strategy call.
3. The [publication gate](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/prompt/compiler_pipeline.go#L341-L366) rejects untracked, degraded, or changed-source results. If generation probes remain readable and unchanged, other collectors are healthy, and compilation succeeds, the erased strategy failure can nevertheless be published as healthy. There is no strategy degradation statistic/reason on this path.
4. A later [same-key hit](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/prompt/compiler_pipeline.go#L110-L125) returns before collection, skipping the recovery probe. The [compiler LRU](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/prompt/compiler_cache.go#L31-L55) is size-bounded without a TTL. Prompt-byte differences require an otherwise eligible strategy to survive selection and budget; absent re-probing is the more direct observable.

The storage layer also matters: [initial query errors](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/autopoiesis/prompt_evolution/strategy_store.go#L151-L190) are returned, then erased above. [Row scanning](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/autopoiesis/prompt_evolution/strategy_store.go#L386-L417) logs and skips scan failures without checking `rows.Err()`. `SelectStrategies` can therefore cache partial/empty results with nil error. Its [five-minute TTL](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/autopoiesis/prompt_evolution/strategy_store.go#L49-L54) and [cache helpers](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/autopoiesis/prompt_evolution/strategy_store.go#L420-L443) are independent of the compiler LRU.

### Conditions and counterexamples

Both generation probes must succeed unchanged. [Generation failures become untracked](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/prompt/compiler_generation.go#L119-L156), so closing the database/global outage is not a standalone stable-token witness. Missing-table/read-schema failures can coexist with readable generation, but schema restoration changes the schema token; malformed-data repair may increment generation. [Generation token construction](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/store/source_generation.go#L123-L165)

The provider's [generation method delegates to its store](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/autopoiesis/prompt_evolution/strategy_atoms.go#L150-L157), and [vector equality requires tracked inputs](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/prompt/compiler_generation.go#L75-L79). A useful reproduction must record these probe outcomes separately from the strategy-read outcome, rather than assuming that a database failure leaves generation tracking intact.

Cancellation or critical compilation failure, another degraded source, source churn, changed context/config/kernel, and eviction defeat the exact repeat path. No provider, legitimate disabled/no-shard input, and successful no-match queries are healthy empty cases. This finding concerns successful fallback publication, not a demonstrated turn crash or data corruption, and does not imply that every failure sticks.

### Narrow remedy and full costs

Carry structured health/error alongside atoms through **store → provider → collector**. Record recovered panics, surface scan failures, inspect iteration errors, and avoid caching incomplete store results as healthy. Feed existing degradation handling while preserving permitted current-turn fallback, scoring, budget, and healthy emptiness. This is a recommendation, not an implementation claim.

Costs include typed-status bookkeeping and provider compatibility migration. The intended healthy cached path remains unchanged. During outages, rejecting partial inner-cache and compiled fallback entries increases repeated queries/classification and whole-miss collection, selection, and assembly: allocations, warnings, concurrent work and contention, and potentially configured embedding work elsewhere. Existing generation reads remain. [Strategy classification itself is regex-based](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/autopoiesis/prompt_evolution/strategy_atoms.go#L60-L77), not an added strategy LLM call. Neither SQL complexity nor latency improvement is established. Separately evaluate bounded, observable backoff rather than silently healthy fallback; measure recovery, repeated work, contention, prompt differences, and downstream repair costs.

### Primary foundation

Go documents that [`Rows.Next`](https://pkg.go.dev/database/sql#Rows.Next) returning false can mean either exhaustion or error; [`Rows.Err`](https://pkg.go.dev/database/sql#Rows.Err) distinguishes iteration errors and remains usable after closure. **Source-derived inference:** unchanged generation tokens do not establish the successful completeness of a particular read.

### Proposed evaluation

None of the following checks was executed.

| Case | Required observation |
|---|---|
| Stable generation; isolated one-shot provider failure, then recovery | First result degraded/uncacheable; next unchanged-key call re-probes. |
| Recovered provider panic | Separate health propagation and recovery coverage. |
| Real query, per-row scan, and iteration failures | Health reaches compiler; no incomplete inner result cached as healthy. |
| Closed database negative control | Generation becomes untracked, not a stable-token witness. |
| Genuine empty/disabled input | Healthy caching remains permitted. |
| Changed generation/schema | Normal invalidation still operates. |
| Eligible versus budget-excluded strategy | Conditional prompt difference; no forced inclusion. |
| Failure pressure and concurrent same-key calls | Measure attempts, time, allocations, SQL, broader configured embedding work, and recovery latency. |

Synthetic faults establish seam propagation, not production failure frequency; lower-layer coverage requires the real provider/storage path.

### Checkable exit

Healthy emptiness is distinguishable from unavailable/incomplete input; the latter never becomes healthy-cacheable. Unchanged-key recovery is observable without unrelated source churn, with measured recovery and total-work costs.


## 2026-10-28: strategy-cache writes under a shared read lock

**Status:** source-established unsafe interleaving, not a reproduced race, crash or failed test. Main `356f65e0e6bb639adaa90ab36212daa714114871` and feature `c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c` have identical store and provider blobs; the relevant executor route remains on both.

[`SelectStrategies`](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/autopoiesis/prompt_evolution/strategy_store.go#L151-L185) holds only `ss.mu.RLock` while calling `getCached` and `setCache`. [The helpers](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/autopoiesis/prompt_evolution/strategy_store.go#L420-L433) can replace the cache map on expiry and write map entries and `cacheTime`. Go's [RWMutex contract](https://pkg.go.dev/sync#RWMutex) permits simultaneous readers; it does not serialize these mutations. Database mutation methods use the exclusive lock, so this specific issue is selection versus selection. Fully warm hit-only selection does not mutate. Disabled strategy selection or an empty shard skips this path.

A conditional production route exists: the [learning factory](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/system/factory_learning.go#L134-L141) registers the shared provider; the [spawner](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/spawner.go#L421-L429) gives workers the shared compiler, and [asynchronous subagents](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/subagent.go#L276-L339) can compile concurrently with a nonempty shard. A successful query permits concurrent cache fills; expiry can replace the map before a query. The earlier pre-run configuration compile omits shard type and is **not** this witness.

Compiler singleflight coalesces identical full prompt keys, not every strategy-store key. The [key](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/prompt/context.go#L695-L724) includes target and semantic query, so different admitted tasks can compile concurrently yet select the same strategy cache key (or different entries in the same map). Identical full-key callers may still coalesce. This is distinct from the previously noted cross-process TTL freshness concern.

**Proposed contract.** Every read, expiry replacement and write of the cache map and timestamp must share a consistent synchronization scheme. Options include a dedicated cache mutex with a documented lock order, an immutable snapshot with atomic replacement, or an exclusive store lock; compare contention and DB time rather than assuming a winner. If fetching outside a lock, attach a generation to the fetch and recheck it before publishing so an invalidated result cannot be installed late. Preserve expiration, database failure behavior, selection semantics and graceful degradation.

**Future evaluation, not run.** Coordinate cold misses, expiry/expiry, warm hit versus missing-key fill, invalidation versus in-flight query, and distinct compiler keys reaching the same or different strategy keys. Use race detection in an authorized test environment, assert source-generation correctness and failed-read disposition, and cover disabled strategies and same-key singleflight controls. Measure cold/warm throughput, tail latency, database queries, lock wait, memory, retry cost, prompt equivalence and downstream task quality at matched workloads; count the entire compilation path. No throughput or accuracy improvement is claimed.

## Post-implementation refinement: preserve literal evidence through template rendering

### Decision and scope

Keep the implemented generation-aware compiler and render-session work. Add an explicit distinction between **authored template instructions** and **runtime evidence** before rendering.

A single-pass template engine answers *how often* to interpret template syntax. It does not establish *which bytes are authorized to contain that syntax*. Current JIT rendering has solved the former problem but still conflates the latter.

This is an adjacent refinement of the template/data semantics already discussed in this audit, not a new cache-generation, dependency-budgeting or section-admission project. It concerns deterministic preservation of admitted evidence and user constraints. It does not claim arbitrary code execution, an authorization bypass, malicious exploitation, or a measured reduction in task failures.

**Source freeze:** rechecked at approximately 16:13 UTC on 2026-10-28. `main` was `356f65e0e6bb639adaa90ab36212daa714114871`; `feat/conversation-brief` was `c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c`; the research branch was `890cfb7b7d0fe77073bb09fe18aefecb62bc27d3`. The relevant assembler, knowledge/learning conversion, kernel-atom construction, selection, and compiler pipeline files are byte-identical between main and feature. The feature's Northstar wizard changed to Guardian-owned updates, but `Vision.ToFacts` still preserves constraint strings unchanged.

**Verification method:** static source inspection and primary-source research only. No runtime execution, tests, model inference, profiling or implementation was performed. The counterexamples and validation plan below are source-derived and unrun.

### Existing implementation to preserve

The present compiler has real improvements:

- Source vectors and kernel snapshots bind cache reuse to the inspected state.
- One `RenderSession` renders fragments once and shares those fragments across fitting and reassembly.
- Fit charges the greater of declared and rendered size; a final assembled-prompt check remains.
- Inserted template values are not scanned recursively.
- Mangle still governs context, capability, conflict and dependency admission.

The [current generation audit's as-built section][literal-as-built] records these improvements. The [production fit-and-assemble path][literal-fit] actually uses them. None closes the following source/data distinction.

### Producer-to-renderer evidence

| Boundary | Current behavior |
|---|---|
| Project and selected-expert knowledge | `knowledgeAtomToPromptAtom` wraps the retrieved record's concept, confidence and **raw `Content`** in a new atom, then sets `RetrievedContext=true`. [Conversion][literal-knowledge] |
| Recalled learning | The compiler hydrates stored learning content, places the returned body into atom `Content`, and sets `RetrievedContext=true`. [Learning conversion][literal-learning] |
| Kernel-injected context | The compiler reads matching `injectable_context` rows, constructs the bounded context block, and creates an atom marked `IsMandatory=true` and `KernelInjected=true`. [Kernel conversion][literal-kernel] |
| Selection | Runtime knowledge/learning gets a separate retrieval witness; Mangle still applies context and prohibition rules. This is not an unconditional inclusion path. [Witness producer][literal-witness], [Mangle admission][literal-admission] |
| Compiler integration | Dynamic, knowledge and learning atoms enter the normal candidate collection before selection and rendering. [Pipeline][literal-pipeline] |
| Rendering | `RenderSession.Fragment` obtains the selected content variant and, whenever the compilation context is nonnil, calls `expandTemplate` on that content. It does **not** distinguish source-bearing evidence spans from authored-template spans. [Fragment renderer][literal-fragment] |

The existing origin flags are explicitly host-populated and non-authorable through YAML. Their documented roles concern selection and assembly placement; neither currently denotes literal rendering. In particular, `KernelInjected` describes the transport/origin, not proof that every character is data: a kernel-derived block can mix authored guidance and source-derived text. These flags are useful inventory leads, not a complete replacement for explicit producer semantics. [Origin fields][literal-flags]

#### A concrete user-constraint producer

This does not require a hypothetical custom template function.

`Vision.ToFacts` passes each Northstar constraint string into `northstar_constraint` without templating it. The production policy derives `injectable_context("*", Desc)` from those constraints when Northstar is defined. The compiler then wraps the derived string into the normal kernel-context atom. [Vision projection][literal-vision], [Northstar policy][literal-northstar]

The feature's Guardian-owned projection also calls `vision.ToFacts`; therefore its improved strategic-edit authority does not remove this later rendering seam. [Feature Guardian projection][literal-guardian]

### Conditional counterexample

Use a valid saved constraint whose complete text is:

```text
Preserve the exact literal token `{{language}}` in the configuration template.
```

Conditions:

1. Northstar is defined and this constraint is projected into the kernel.
2. The row survives the existing injection bounds; one short constraint suffices.
3. The ordinary compile admits its kernel-context atom.
4. The compilation context has `Language="go"` or `Language="/go"`.
5. The budget is ample.

The inspected path constructs the correct literal constraint, then sends it through `expandTemplate`. The registered `language` function returns `go`. The resulting fragment therefore contains:

```text
Preserve the exact literal token `go` in the configuration template.
```

The backticks do not protect the token: the renderer recognizes brace syntax without interpreting Markdown quoting. This changes the object of the user's constraint before a model sees it. The `language` function and parser establish this transformation directly. [Default substitutions][literal-defaults], [Template scanner][literal-scanner]

An equivalent knowledge fixture is a retrieved example containing:

```text
The documented template expression is `{{language}}`.
```

Its body reaches the same renderer after retrieval and policy admission. An equivalent recalled-learning fixture uses that string in hydrated learning content.

These are **source-derived counterexamples, not executed reproductions**. They establish a deterministic transformation under the stated conditions, not evidence that any observed model subsequently edited the wrong file.

#### Why the new one-pass protection does not cover this

The existing test passes a string such as `agent says {{secret}}` **as the return value of a template lookup** and correctly verifies that it is not scanned again. [Existing test][literal-render-test]

In the counterexample, the evidence string has already become the atom's initial `Content`. The renderer therefore treats it as source text on the **first** pass. There is no recursive expansion, map-order race, stale cache or second whole-prompt pass.

The normal `Compile` entry requires a nonnil context, so the renderer's nil-context bypass is not a mitigation on this path. Unknown placeholders and whitespace-altered names are useful negative controls: names not matching a registered key remain unchanged. This is specifically a collision between literal evidence and the renderer's exact recognized vocabulary, not a claim that every brace expression is evaluated.

### Capability and efficiency consequences

The primary loss is fidelity: a correctly retrieved and admitted source can be altered during rendering.

There is also a conditional budget consequence. Literal evidence mentioning `{{available_specialists}}` or `{{available_tools}}` expands into the current roster or catalog. This can enlarge a small evidence record enough to change fitting or omission decisions. The current rendered-size charging and final enforcement remain protective; this is **not** evidence of a budget bypass.

Accidental expansion can thus:

- Replace a literal source token with an unrelated contextual value.
- Change the displayed meaning of a stored constraint or technical example.
- Increase prompt bytes and displace otherwise useful material.
- Make an unchanged literal source vary with language, catalog or roster state.

No net token saving or task-quality improvement has been measured. Preserving a literal token can also make a fragment longer when the old substitution replaced it with a short string. Quality, not a presumed universal size reduction, is the acceptance criterion.

### Proposed contract

Names below are design notation, not current APIs.

```text
AuthoredTemplate:
  validated instruction source
  explicit supported placeholder semantics

LiteralEvidence:
  source-bearing content
  never interpreted as template syntax

RenderedFragment:
  bytes + origin/render policy + applicable truncation disposition
```

#### Producer inventory before suppression

Inventory each producer that contributes runtime content, recording which spans are authored guidance, which are source-bearing data, whether substitution is intentional, and which fields that substitution may read. `KernelInjected` can mix authored and data spans, so a blanket "skip all kernel-injected atoms" patch is not established as correct. Likewise, the authored atom `capability/knowledge_discovery` legitimately uses `{{available_specialists}}`; globally disabling substitutions or disabling them for an entire category would break intended behavior. [Authored positive control][literal-authored]

Use explicit types or segments to preserve this distinction through composition. An immutable literal segment must not become a template merely because a wrapper was concatenated around it. A host-owned flag-based implementation may be a useful narrow candidate **after** the inventory establishes that a producer's entire output has one rendering policy; compare it against the typed-segment contract rather than assuming the existing origin flags already provide that guarantee.

#### Narrow implementation direction

1. **Give source-bearing runtime bodies literal semantics.** Knowledge bodies, hydrated learning bodies and user-derived constraint spans must not be interpreted as template source solely because they become `PromptAtom.Content`.
2. **Do not infer template authority from category or transport.** Some authored knowledge-category atoms legitimately are templates; kernel-origin blocks may mix different content roles.
3. **Do not let data grant itself template authority.** A host-owned origin/render policy is preferable to recognizing an opt-in marker inside a retrieved document.
4. **Retain intentional authored substitutions.** Existing built-in/project prompt templates must continue to expand their declared placeholders once.
5. **Use structured composition for mixed content.** A trusted wrapper may contain an explicit substitution node whose value is evidence; that value remains literal. A producer whose whole output is verified literal can use a simpler literal-fragment representation. A general-purpose template AST is not necessarily required for the initial fix.
6. **Make intentional runtime templating explicit.** If the producer inventory finds a real requirement, specify its authoring/admission contract rather than silently grandfathering every runtime string.
7. **Keep selection separate.** Literal treatment does not make evidence mandatory, trusted, current or authorized. Mangle context/tool/conflict rules, source generations, whole-record fitting and existing truncation disclosures remain unchanged.

Do not fix this by deleting all braces, escaping source bytes into different source code, excluding all evidence that resembles a template, or adding model instructions to ignore an already altered constraint.

A useful local invariant is:

> For an admitted, untruncated literal evidence segment, changing unrelated template context cannot alter its evidence bytes.

For bounded/truncated evidence, apply that invariant to the explicitly retained source spans; do not falsely promise preservation of bytes already omitted under an independently declared bound. Boundaries between literal and template segments also need to be structural: joining two segments must not manufacture a newly executable placeholder across their boundary.

### Proposed validation — unrun

Exercise the production private-kernel compiler route, not only `expandTemplate`.

| Fixture | Required observation |
|---|---|
| Saved Northstar constraint containing recognized `{{language}}` | Literal token survives full compile with `Language=go`; source fact and emitted evidence agree. |
| Selected project knowledge containing the same token | Retrieval and Mangle admission occur; emitted body remains literal. |
| Selected expert knowledge | Same preservation, with sibling-expert isolation unchanged. |
| Hydrated recalled learning | Stored body, not search descriptor, is preserved. |
| Kernel `specialist_knowledge` body and heading | Source-bearing spans survive their declared bounded projection literally. |
| Kernel block mixing authored instructions and source data | Explicitly authored substitutions work; data spans do not become templates. |
| Evidence inside backticks, code fences, quotes and plain prose | Quoting style does not change its literal policy. |
| Every recognized placeholder plus unknown, whitespace-altered and malformed forms | All remain literal in evidence; authored-template semantics remain as documented. |
| Intentional authored template using `{{language}}` or `{{available_specialists}}` | Still expands once; `knowledge_discovery` remains functional. |
| Placeholder returned inside an authored template's data value | Existing nonrecursive protection remains. |
| Placeholder delimiters split across segment boundaries | Composition cannot promote joined data into a new substitution. |
| Literal roster/catalog token with small and large current catalogs | No accidental expansion or budget-driven loss caused by interpreting that token. |
| Tight budget, whole-record omission and mandatory overflow | Existing fitting/refusal obligations remain unchanged. |
| Cold compile, cache hit, source update and language change | Content origin and rendering policy remain consistent; no stale-generation regression. |
| Ordinary no-placeholder evidence | No unintended byte or selection changes. |

Keep assertions on selected IDs, provenance, emitted fragment bytes, omission/refusal disposition and final budget—not merely successful compilation.

For later model trials, pair template-heavy repository tasks with ordinary-code controls. Keep model/provider, allowed tools, corpus, task budgets and verification criteria fixed. Compare current behavior with origin-aware literal rendering and retain the already-correct one-pass renderer in both arms. Compare any inventory-qualified flag-based candidate with typed mixed-segment handling rather than conflating both with global substitution removal.

Measure:

- Literal-evidence corruption and constraint preservation.
- False omission, compile refusal and source coverage.
- Verified completion, false completion and human repair.
- Total input/output and cached-token accounting, retries, rehydrations and verification.
- Compiler CPU, allocations, rendered bytes and retained memory.

A smaller prompt that omits useful evidence is not an improvement. A byte-preserving renderer also does not prove that a model follows the preserved evidence. Include producer-inventory and migration complexity in the adoption cost; a broad type redesign should earn its cost against a correctly scoped smaller implementation.

### Primary foundations and limits

[Su and Wassermann, POPL 2006][literal-su-wassermann] motivates retaining input-origin information when strings enter an interpreter rather than deciding syntactic authority from the final concatenated bytes. The applicable lesson here is provenance-aware interpretation. Their SQL injection definition, proof and measured results are **not** a proof of exploitation or performance in CodeNERD.

The [Go `text/template` documentation][literal-go-template] explicitly separates template source from supplied data and describes a trust assumption for template authors. CodeNERD uses its own smaller renderer, not that package; the reference supports the source/data distinction, **not** an arbitrary-code-execution claim or a recommendation to replace the renderer.

### Overlap and completion criterion

- **Generation audit:** reuse and render-once implementation are credited. This appendix refines the initial template-source provenance boundary adjacent to the already discussed template/data semantics.
- **Dependency/section audit:** no new selection, closure or section-admission claim is made.
- **Retrieval audit:** no claim that retrieval chose the wrong source; the witness begins with a correctly admitted source.
- **Northstar editing audit:** no loss during saving or updating is required. The loss occurs downstream while rendering a successfully preserved constraint.
- **Prompt-evolution audit:** admission of generated instructions remains a separate question; do not blanket-classify all learned/evolved authored instructions as literal evidence without inventory.

**Completion criterion:** an admitted literal evidence segment can contain every recognized placeholder name without being rewritten by compilation context, while intentionally authored templates retain their documented substitutions and all existing policy/budget gates remain intact.

[literal-as-built]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/Docs/audits/jit-generation-aware-optimization.md#as-built-2026-10-27
[literal-fit]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/prompt/compiler_render.go#L10-L63
[literal-knowledge]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/prompt/compiler_db.go#L441-L458
[literal-learning]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/prompt/compiler_db.go#L532-L550
[literal-kernel]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/prompt/compiler_kernel.go#L245-L273
[literal-witness]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/prompt/selector.go#L1128-L1137
[literal-admission]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/defaults/jit_compiler.mg#L237-L244
[literal-pipeline]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/prompt/compiler_pipeline.go#L282-L284
[literal-fragment]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/prompt/assembler.go#L227-L255
[literal-flags]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/prompt/atoms.go#L252-L261
[literal-vision]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/northstar/types.go#L144-L148
[literal-northstar]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/defaults/policy/prompt_northstar.mg#L211-L214
[literal-guardian]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/northstar/guardian.go#L223-L275
[literal-defaults]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/prompt/assembler.go#L424-L526
[literal-scanner]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/prompt/assembler.go#L561-L606
[literal-render-test]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/prompt/compiler_render_test.go#L13-L30
[literal-authored]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/prompt/atoms/capability/knowledge_discovery.yaml#L5-L22
[literal-su-wassermann]: https://people.inf.ethz.ch/suz/publications/popl06.pdf
[literal-go-template]: https://pkg.go.dev/text/template#hdr-Overview
