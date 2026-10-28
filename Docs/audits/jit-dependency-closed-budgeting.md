---
doc-class: deep-dive
subsystem: prompt
implementation-status: partial
last-verified: 2026-10-28
verified-against: 4947e87e8d6a8d41e5cc7af5a71ec0675831dce2
verification-method: static-source-inspection
main-revalidated-at: 2026-10-28T03:13:40Z
supersedes: []
---

# Budget coherent instruction sets, not isolated prompt atoms

**Decision:** preserve hard atom dependencies through every budget reduction and through final assembly. The first useful improvement is a final dependency certificate and dependency-aware removal, not an expensive optimizer or a larger context window. A prompt can be within its token budget and still violate the atom author's declared composition contract.

**Evidence status:** static source review and proposed design. No implementation, tests, inference, profiling, training, or provider calls were executed. Conditional examples are unexecuted witnesses, not observed incidents. Main was inspected at the commit above; recheck it before implementation because concurrent development is active.

Freshness reconciliation: main advanced during this pass by merging the research documentation branch. The compare from the initial `2e0c54e` source baseline to `4947e8` changed documentation only; the cited implementation files are unchanged. Main was then confirmed identical to the new pin at the timestamp above.

## 1. Scope and evidence lineage

The [root guidance, “The Vision”](https://github.com/theRebelliousNerd/codenerd/blob/4947e87e8d6a8d41e5cc7af5a71ec0675831dce2/agents.md#the-vision-steve-2026-09-18--read-before-any-architectural-change) prioritizes quality and capability over token reduction and assigns executive context decisions to Mangle. The [prompt guidance, lines 48–75](https://github.com/theRebelliousNerd/codenerd/blob/4947e87e8d6a8d41e5cc7af5a71ec0675831dce2/internal/prompt/agents.md#L48-L75) requires whole retrieved context and transitive pruning when a tool-gated dependency is unavailable. The same compositional discipline must survive a later resource decision.

This is distinct from [generation-aware JIT reuse](jit-generation-aware-optimization.md), §§2–5, which addresses repeated compilation work and stale cache identity, and [context ledger costs](context-ledger-prefix-costs.md), §§3–4, which addresses archived observations and compaction. Both preserve dependency obligations as a requirement. This document investigates the actual gap between pre-budget dependency resolution and the final emitted atom set.

The repository has extensive existing dependency and budget documentation. [Prompt current implementation, dependency resolver and budget sections](https://github.com/theRebelliousNerd/codenerd/blob/4947e87e8d6a8d41e5cc7af5a71ec0675831dce2/Docs/architecture/prompt/IMPLEMENTED_SPEC.md) describes the pipeline. Its existence is not evidence that an invariant verified at one stage remains true after another stage removes atoms. This is an integration refinement, not a claim that dependencies or budgeting are absent.

## 2. What is already implemented

| Boundary | Source evidence | Existing behavior to preserve |
|---|---|---|
| Hard dependency declaration | `atom_requires`, [internal/core/defaults/schemas_prompts.mg:404–406](https://github.com/theRebelliousNerd/codenerd/blob/4947e87e8d6a8d41e5cc7af5a71ec0675831dce2/internal/core/defaults/schemas_prompts.mg#L404-L406) | The relation explicitly means the dependent requires its prerequisite to be selected. It is not merely a similarity hint. |
| Executive expansion and invalidity | `tentative`, `missing_dep`, `invalid`, [internal/core/defaults/jit_compiler.mg:265–329](https://github.com/theRebelliousNerd/codenerd/blob/4947e87e8d6a8d41e5cc7af5a71ec0675831dce2/internal/core/defaults/jit_compiler.mg#L265-L329) | Recursively includes allowed dependencies and propagates missing-dependency invalidity before returning selections. Existing vetoes must not be bypassed to rescue a budget plan. |
| Transitive pruning and ordering | `DependencyResolver.Resolve`, [internal/prompt/resolver.go:46–133](https://github.com/theRebelliousNerd/codenerd/blob/4947e87e8d6a8d41e5cc7af5a71ec0675831dce2/internal/prompt/resolver.go#L46-L133) | Default resolver prunes missing dependencies transitively, then produces ordered atoms. Do not characterize it as only a naive sort. |
| Global and category limits | `TokenBudgetManager.Fit`, [internal/prompt/budget.go:381–461,568–595](https://github.com/theRebelliousNerd/codenerd/blob/4947e87e8d6a8d41e5cc7af5a71ec0675831dce2/internal/prompt/budget.go#L381-L595) | Fits whole standard/concise/min variants, with overflow guards and a global bound as well as category allocations. |
| Mandatory refusal | `Fit`, [internal/prompt/budget.go:521–560](https://github.com/theRebelliousNerd/codenerd/blob/4947e87e8d6a8d41e5cc7af5a71ec0675831dce2/internal/prompt/budget.go#L521-L560) | For mandatory atoms reaching this branch, selects a whole fitting variant or returns an error rather than cutting the instruction. |
| Post-render enforcement | `enforceAssembledBudget`, [internal/prompt/compiler.go:1939–1970](https://github.com/theRebelliousNerd/codenerd/blob/4947e87e8d6a8d41e5cc7af5a71ec0675831dce2/internal/prompt/compiler.go#L1939-L1970) | Rechecks assembled text, sheds whole optional atoms, and refuses an oversized mandatory remainder. Preserve this final check. |
| Stable prefix-oriented assembly | `assembleWithConfig`, [internal/prompt/assembler.go:206–270](https://github.com/theRebelliousNerd/codenerd/blob/4947e87e8d6a8d41e5cc7af5a71ec0675831dce2/internal/prompt/assembler.go#L206-L270) | Deterministic tiers/categories protect prefix reuse; resolver order is retained within each tier/category. Do not replace this with arbitrary unstable ordering. |

Real authoring paths use these dependencies: `RenderAgentPromptsYAML` emits mandatory identity, optional methodology depending on identity, and optional domain guidance depending on both ([internal/system/agent_definition.go:93–185](https://github.com/theRebelliousNerd/codenerd/blob/4947e87e8d6a8d41e5cc7af5a71ec0675831dce2/internal/system/agent_definition.go#L93-L185)). This establishes practical relevance of optional prerequisites, not proof that this exact generated corpus has produced a dangling prompt in practice.

## 3. Finding: a valid selection can become dependency-incomplete after fitting

The production compiler order is explicit: selection, `Resolve`, `Fit`, assembly, `enforceAssembledBudget`, then result construction (`compile`, [internal/prompt/compiler.go:874–955](https://github.com/theRebelliousNerd/codenerd/blob/4947e87e8d6a8d41e5cc7af5a71ec0675831dce2/internal/prompt/compiler.go#L874-L955)). There is no dependency revalidation between the two removal stages and result construction in this inspected path.

`Fit` reorders the input by category priority and score, then admits or omits individual atoms. Its fit decisions inspect cost, category, mandatory status and count limits, not `DependsOn` ([internal/prompt/budget.go:433–461,486–700](https://github.com/theRebelliousNerd/codenerd/blob/4947e87e8d6a8d41e5cc7af5a71ec0675831dce2/internal/prompt/budget.go#L433-L700)). The later `ShedToFit` similarly constructs a list of individual nonmandatory victims and removes them without removing their retained dependents ([internal/prompt/budget.go:1049–1135](https://github.com/theRebelliousNerd/codenerd/blob/4947e87e8d6a8d41e5cc7af5a71ec0675831dce2/internal/prompt/budget.go#L1049-L1135)). Checking that the final text fits does not establish dependency closure.

**Proposed witness A:** two eligible optional atoms A and B, with A requiring B. Arrange category allocations, scores and whole variant sizes so A is admitted but B fits neither its allocation nor the final remaining budget. The pre-budget set can be valid, yet the emitted set contains A without B. Use a real compile fixture with a private production kernel; do not rely only on calling Fit directly.

**Proposed witness B:** both atoms initially fit, then a template expansion causes post-render overshoot. Make B the lower-priority shedding victim. Removing B alone can leave A present. A final dependency check must reject or repair this state even though the reduced text now fits.

These are source-supported conditional paths, not executed failures. Prompt dependence may encode context, terminology, procedure or a warning, and the model might still solve a particular task without it. The supported defect is failure to maintain the declared composition contract, not a claim of universal incorrect output or bypass of independent tool authorization.

Simply rerunning the current resolver after Fit is an incomplete repair: it can prune optional dependents, but a required instruction losing its prerequisite should produce an explicit infeasibility result rather than silently disappear. Likewise, a budget cap on atom count is a resource limit, not permission to drop required closure. `Fit` has both an input cap and an included-atom cap ([internal/prompt/budget.go:395–404,472–515](https://github.com/theRebelliousNerd/codenerd/blob/4947e87e8d6a8d41e5cc7af5a71ec0675831dce2/internal/prompt/budget.go#L395-L515)); the proposed certificate must cover those exits too.

## 4. Related distinction: inclusion dependency versus presentation order

The resolver calls `topologicalSort`, but final assembly groups by tier and category before using the resolver's order inside each group. Therefore a cross-group prerequisite is not guaranteed to precede its dependent in final text. This is directly visible in `assembleWithConfig:206–263`, cited above. The resolver also breaks cycles to make progress rather than failing every cyclic input (`topologicalSort`, [internal/prompt/resolver.go:177–290](https://github.com/theRebelliousNerd/codenerd/blob/4947e87e8d6a8d41e5cc7af5a71ec0675831dce2/internal/prompt/resolver.go#L177-L290)).

Do not overstate this as an LLM execution-order bug. A model receives a prompt, not a sequential program. The hard schema contract is inclusion; a strict ordering promise requires a separate product decision. Clarify the contract:

* If `DependsOn` means only joint inclusion, certify closure and retain current stable tiers unless a task evaluation justifies reordering.
* If it also means prerequisite-before-dependent, use a dependency-respecting final order and treat tier/category preference as a tie-break among ready nodes, not an overriding grouping rule.
* A cycle can be a coherent joint-inclusion bundle, but cannot satisfy strict pairwise precedence. Distinguish that case explicitly rather than treating a forced break as proof of topological validity.

This avoids sacrificing provider prefix caching to fix a requirement that was never intended, while making any genuinely required precedence checkable.

## 5. Proposed contract and minimal repair

All names below are proposed notation, not existing APIs or schema declarations.

For selected set S and prerequisite relation D, require:

```text
For every atom a in S and every prerequisite b of a: b is in S.
Every mandatory eligible atom and its transitive prerequisites is in S.
Every selected rendering is an allowed whole variant.
Every selected atom remains eligible under the same source/policy snapshot.
Rendered prompt and complete model request pass their respective budget checks.
```

Snapshot identity must include the generation semantics from the adjacent JIT study. Do not validate graph G from one corpus against rendered bytes from another. Variant content and template expansion identities belong to the same compile record.

### 5.1 Lowest-complexity safe path

1. After executive eligibility and dependency resolution, build the immutable graph of selected atom IDs and declared prerequisites. Missing or prohibited prerequisites remain unresolved; do not load an excluded dependency through a side door.
2. Compute the mandatory closure M. Dependencies of mandatory atoms are protected for this compilation even if their author-level flag is optional. Record the reason for that promotion; do not rewrite the stored atom's global mandatory status.
3. Retain current fitting as an initial candidate, then validate the result. If an optional atom has a missing prerequisite, remove it and recursively invalidate retained dependents. If removal reaches M, return a structured infeasibility result rather than a success missing an obligation.
4. Replace post-render single-victim removal with removal of the victim plus all retained dependents that require it. Never choose a removal closure intersecting M. Re-render and repeat within a bounded budget; if nothing removable can achieve a fit, return infeasible.
5. Certify closure, eligibility, required set, variant selection and final ordering contract after the last change, immediately before building/caching the successful compilation result.

A queue over reverse dependency edges can perform one cascade in O(n+e) worst case for n selected atoms and e dependency edges. Do not repeatedly rescan every edge once per missing node when a worklist can update missing counts. Repeated budget repair still adds work and may underfill the window; this baseline prioritizes semantic correctness, not optimal token utilization.

### 5.2 Better packing only after the certificate exists

A more capable policy treats the incremental prerequisite closure as the admission unit. For optional root a and current set S, charge the unique atoms in closure(a) minus S. Shared dependencies are paid once. Consider whole variant choices for every newly admitted atom, not only the root. A root that looks cheap may have an expensive support bundle.

The binary single-variant formulation illustrates the constraint:

```text
maximize a declared utility surrogate sum(u[a] * x[a])
subject to sum(c[a] * x[a]) <= available budget
           x[a] <= x[b] for every a requiring b
           x[a] = 1 for required closure
           existing conflict, capability and admission constraints
```

Multiple variants require one-of-none/standard/concise/min decisions and their actual rendering costs. Token costs need not add perfectly across joined fragments; retain the final rendered/request check. Existing score values are ranking signals, not calibrated measurements of task success, so the objective is only a surrogate. Do not call its mathematical optimum the best prompt.

Start with deterministic closure-aware greedy admission under Mangle-declared priorities, compare it to the minimal repair, and use an exact bounded offline optimizer only as an evaluation reference. Shared prerequisites, conflicts and variants make naive per-atom utility-per-token heuristics misleading. No approximation guarantee is asserted here.

Mangle owns required sets, eligibility, admissible variants, priority and validity conclusions. Go may perform bounded graph bookkeeping, rendering and resource measurement, but must not invent a new executive objective. If a host solver proposes a plan, return its selected IDs/costs to a scoped policy validation step before success. Model-authored cost or closure claims are not authoritative.

## 6. Failure handling and observability

The output should distinguish valid compilation from infeasible required closure, missing prerequisite, invalid variant, exhausted resource bound and stale source generation. Include IDs and compact reason chains without exposing unrelated private atom contents. A failed mandatory bundle must not become a generic “use fewer tokens” request to the model.

For every omitted optional atom, distinguish direct budget removal from cascading prerequisite removal. Report both estimated and rendered costs, shared prerequisites and effective mandatory closure. A closure certificate says the declared graph was satisfied; it does not prove the declarations captured every semantic dependency.

Concise/min variants are author-supplied alternatives, not mechanically proven semantic equivalents. Initially use the same prerequisite set for every variant. If future per-variant dependency sets are introduced, require explicit authoring and validation, not inference that a short variant no longer needs its supporting guidance.

Malformed graphs, duplicate IDs and cycles need explicit handling. Joint-inclusion strongly connected components can be treated atomically if policy permits; strict precedence cycles are infeasible. Preserve normal authorization and source precedence when resolving IDs. A dependency cannot name another workspace's private instruction merely because the string matches.

## 7. Costs and fair evaluation

Account separately for graph construction, closure validation, candidate packing, variant rendering, final request tokenization/estimation, policy re-evaluation, cache identity/certificate retention and failed compile recovery. Store adjacency and reverse adjacency in O(n+e); storing every transitive closure naively can require O(n²) space. Use lazy/shared closure representation or bounded recomputation when warranted, not a claim that closure-aware packing is free.

The expected benefit is avoiding tokens spent on incomplete instruction bundles and reducing downstream repair caused by such bundles. It may retain more supporting instructions, reject more infeasible compiles or spend extra host CPU. A smaller emitted prompt alone is not an improvement. Count all provider attempts, cached and uncached input correctly, output/reasoning subcounts without double counting, retries, task failures and human repair.

Compare four alternatives with identical atom corpus, model, task snapshot, tools, required instructions and total resource allowance:

1. Current pipeline, with its existing whole-atom and final-budget protections.
2. Current packing plus final closure repair/certificate.
3. Closure-aware packing plus the same certificate and final checks.
4. A bounded exact optimizer for small fixtures only, under the same explicit utility surrogate; use it to measure packing regret, not model intelligence.

For ordering, compare current tiers against dependency-constrained ready-node ordering only on cases whose contract requires precedence. Measure provider prefix reuse and accepted-task quality jointly; never improve cache hit rate by violating an actual dependency.

### Proposed experiments, none executed

| Fixture | Required observation |
|---|---|
| Optional high-score dependent and costly low-score prerequisite | Final prompt includes both or neither; tight budget cannot produce dangling instruction. |
| Template expansion triggers late shedding | Removal cascades or chooses another valid bundle; final closure rechecked. |
| Mandatory atom has optional prerequisite | Prerequisite joins effective mandatory closure; inability to fit is explicit infeasibility. |
| Two roots share one prerequisite | Shared bytes charged once; removing one root does not remove prerequisite still needed by another. |
| Chain, diamond, broad fan-out and cyclic inclusion graph | Deterministic closure behavior; cycle/order semantics explicit. |
| Tool/context veto on prerequisite | No budget rescue reintroduces it; dependent remains inadmissible. |
| Standard/concise/min and large placeholders | Whole variants only; declared dependencies preserved; final request guard retained. |
| Atom-count/resource cap intersects mandatory closure | Explicit failure, not silent loss of required content. |
| Cross-tier prerequisite with strict precedence contract | Final rendered order, not just resolver order, satisfies the contract. |
| Corpus/config changes during compile or cache reuse | Certificate and prompt share generation; stale certificate cannot validate new content. |

Use production factory → scoped selection → resolver → budget → assembler → final result/cache for integration fixtures. Direct Fit tests are useful socket tests but insufficient evidence for the full contract. At scale vary graph density, shared support, selected atom count, variant count, mandatory fraction and expansion variance independently. Record peak memory, p50/p95/p99 compile latency, repeated policy work and refusal reasons.

For capability evaluation, choose tasks where guidance genuinely composes: specialist domain plus methodology, multi-language changes, typed tool use, and verification procedures with supporting definitions. Independently evaluate behavior, false completion and repair burden on held-out tasks. Include dependency-free controls so added machinery must justify its cost. Report confidence intervals and unsuccessful tasks rather than cherry-picking a high-score packing result.

## 8. Primary foundations and adoption queue

[Boland et al., *Clique-based facets for the precedence constrained knapsack problem*, §1](https://web.maths.unsw.edu.au/~froyland/BBFFS12.pdf) formalizes item inclusion under prerequisite and capacity constraints. It supports the joint-selection model; its mining/manufacturing results are not evidence of prompt quality or CodeNERD speedups. [Kahn, *Topological sorting of large networks*](https://doi.org/10.1145/368996.369025) is the primary ordering reference. A topological procedure cannot preserve precedence if a later stage arbitrarily regroups the output; whether strict precedence is required here remains an explicit contract choice.

| Local research item | Deliverable | Checkable exit |
|---|---|---|
| JIT-CLOSURE-01 | Final emitted-set dependency certificate | Both early-fit and late-shed witnesses cannot return success with missing prerequisites. |
| JIT-CLOSURE-02 | Effective mandatory closure and cascading removal | Required support survives; infeasible closure fails explicitly; optional reverse closure removed consistently. |
| JIT-CLOSURE-03 | Clarified inclusion/order/variant semantics | Corpus validation and final rendered ordering match the chosen contract, including cycles. |
| JIT-CLOSURE-04 | Closure-aware packing study | Same-quality task study and small exact-reference fixtures show whether better packing earns its host cost. |

These are proposed local work items, not claims that the repository has adopted new specs. The first milestone is simple and testable: **a successful bounded prompt still contains the prerequisites of every instruction it retained.**
