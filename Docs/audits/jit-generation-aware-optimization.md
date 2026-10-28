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
