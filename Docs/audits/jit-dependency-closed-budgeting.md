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

## Post-implementation refinement (2026-10-28): section expansion must preserve policy admission

### Decision

Keep the new sectioned-atom design, whole-fragment budgeting and dependency closure. Make **section expansion an executive-policy operation**, not a second admission policy implemented by a partial Go matcher. A section may be optional without being exempt from capability, context, conflict and supersession rules. Conversely, a missing situational selector must not suppress a section when the authoritative selector deliberately permits that missing value.

The narrow opportunity is after normal selection: `compileMiss` calls `expandSections` with the full candidate corpus, then sends its enlarged result to a resolver that assumes admission has already been decided. The expansion checks `MatchesContext`, which neither checks tool requirements nor implements the same missing-dimension semantics as Mangle. A dependency-closed prompt can therefore still be policy-inconsistent. [Pipeline][section-admission-pipeline], [expansion][section-admission-expansion], [matcher][section-admission-matcher], [resolver][section-admission-resolver]

**Appendix status: proposed; static-source-inspection.**

**Evidence status:** source and test-definition inspection only. No implementation, tests, binaries, provider inference, benchmarks or failure injection were performed. This appendix is a research refinement of the as-built section above, not an adopted specification or implementation. The examples below are conditional source witnesses, not observed user incidents; no frequency, speedup or model-quality result is claimed.

### R1. Scope, pins and non-duplication

The main and feature heads were re-resolved at approximately 13:44 UTC on October 28, 2026:

- `main`: [356f65e0e6bb639adaa90ab36212daa714114871][section-admission-main].
- `feat/conversation-brief`: [c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c][section-admission-feature].
- Research branch at that check: `0de7ff76968955ad5185d8dd13b85bc05775aa98`.

The Git trees at main and feature contain identical blobs for the relevant implementation: `expandSections` in [internal/prompt/sections.go:37][section-admission-expansion] and `closeDependencies` at [internal/prompt/sections.go:121][section-admission-closure]; `compileMiss` in [internal/prompt/compiler_pipeline.go:301–336][section-admission-pipeline]; `MatchesContext` in [internal/prompt/atoms.go:427][section-admission-matcher] and `atomToolSatisfied` at [internal/prompt/atoms.go:801][section-admission-tool-check]; `Resolve` in [internal/prompt/resolver.go:48][section-admission-resolver]; `ValidatePromptAtomSet` in [internal/prompt/atom_schema.go:690][section-admission-validation]; `fitAndAssemble` in [internal/prompt/compiler_render.go:14][section-admission-render]; `filterSkeletonAtoms`/`filterFleshAtoms` in [internal/prompt/selector.go:963,1074][section-admission-filters]; `CompilationContext.Validate` in [internal/prompt/context.go:510][section-admission-context]; and `mandatory_selection`/`prohibited` in [internal/core/defaults/jit_compiler.mg:205–226][section-admission-veto]. This is shared current behavior, not a conversation-brief regression.

Root guidance assigns context inclusion to the kernel, favors quality over token reduction and requires JIT-first behavior. Prompt guidance explicitly requires tool gating to have the same meaning on every selection path and forbids granting tools to satisfy an atom. The proposed change implements those existing contracts. [Root guidance][section-admission-root], [prompt guidance][section-admission-guidance]

Existing research was read first:

| Existing work | What remains there |
|---|---|
| [JIT dependency-closed budgeting][section-admission-closure-study] | Inclusion closure, packing and ordering. Its **October 28 As built** addendum records section expansion and post-fit/post-shed closure. Do not propose that already-shipped closure as missing. |
| [JIT generation-aware optimization][section-admission-reuse-study] | Source-vector identity, shared preflight, staged metadata and rendered-fragment reuse. A fresh cached result can still violate admission. |
| [Context ledger prefix costs][section-admission-ledger-study] | Revision acquisition and post-JIT working-ledger costs. |
| [Request-visible tool contract][section-admission-tool-study] | Whether granted tools are registered/describable/offered to the model. Here the supplied catalog can be entirely correct; a later prompt transformation ignores an atom's requirement. |
| [Prompt evolution lifecycle][section-admission-evolution-study] | Candidate promotion and learning attribution. This proposal applies equally to built-in and ordinary authored atoms, without changing promotion authority. |

The closest prior audit already requires selected atoms to remain eligible. The new contribution is the concrete October 28 **post-selection expansion seam**, the two-sided policy mismatch, and a bounded contract that preserves the newly implemented functionality rather than removing it.

### R2. What is already built

1. Both normal selector partitions check `atomToolSatisfied` before their atoms enter selection; empty catalogs fail closed for explicit requirements. [Filters][section-admission-filters], [tool check][section-admission-tool-check]
2. Mangle selection applies context restrictions, prohibitions, conflicts and dependency expansion; mandatory-source conflicts can prohibit another atom. [Veto][section-admission-veto], [selection and dependency rules][section-admission-selection]
3. `compileMiss` then invokes `expandSections(scored, candidates, cc)` using the original full candidates, not just a policy-approved expansion set. [Pipeline][section-admission-pipeline]
4. `expandSections` builds the head/section map from all candidates; a selected head adds its sections and a selected section can add its head. The only new-member eligibility call is `MatchesContext`. [Expansion][section-admission-expansion]
5. `Resolve` explicitly assumes valid input. Its current body prunes missing dependencies and topologically orders; it does not reevaluate tool eligibility or conflicts. The “handle conflicts” pipeline comment is not evidence that this downstream body checks them. [Resolver][section-admission-resolver]
6. `fitAndAssemble` charges rendered fragments, closes dependencies after fitting and again after shedding, and reassembles survivors. These checks are useful and must remain. Neither closure pass asks whether a retained atom was policy-admissible. [Render][section-admission-render]

The main audit addendum reports embedded-corpus measurements from an earlier run and the section recovery they demonstrated. Those are repository-reported historical results, not measurements reproduced in this investigation. They establish why reverting expansion would discard intended capability, not that every admission boundary is covered. [As-built account][section-admission-closure-study]

### R3. Finding A: expansion can restore a tool-ineligible section

#### Source-supported conditional witness using a valid authored corpus

Consider a corpus containing:

| Atom | Properties |
|---|---|
| H | Mandatory protocol head, no tool requirements or context selectors |
| S | Optional protocol section of H, section order 1, requires `run_check`, otherwise no selectors |
| Context | Valid ample budget, no `run_check` in `AvailableTools` |

This is proposed fixture data, not a claim that a shipped built-in currently has this exact asymmetric head/section pairing.

The canonical schema supports both section metadata and tool requirements. `sectionDependsOn` adds the hard dependency on H. Complete-set validation verifies dependency existence, a non-self head, positive section order and existing conflict targets; it does not require section tool requirements to equal the head's. Thus the proposed asymmetry is not ruled out by the inspected corpus validator. [Schema conversion][section-admission-schema], [set validation][section-admission-validation]

The normal selector filters S out for its missing tool, while H remains selectable. Expansion revisits S in the complete candidates. `MatchesContext` checks selector dimensions and world states, but not `RequiresTools`; its true result admits S. H is present, so the later dependency checks do not remove S. With a fitting budget, no later inspected step discharges the missing-tool obligation. [Filters][section-admission-filters], [matcher][section-admission-matcher], [expansion][section-admission-expansion], [render][section-admission-render]

**Scope of the finding:** this breaks the model-facing capability contract and can spend tokens describing an unavailable operation. It is **not** proof of an execution-permission bypass. Exact runtime authorization remains independently necessary; never widen it to make the emitted guidance executable.

### R4. Finding B: expansion can defeat an explicit policy veto

A second valid-corpus fixture removes any ambiguity about tool metadata inheritance:

- H is an eligible mandatory head.
- S is an optional section of H.
- M is another eligible mandatory atom with `conflicts_with: [S]`.
- There are no other dependencies or incompatible selectors; the budget is ample.

Mangle's `prohibited(B)` rule derives S prohibited from M's mandatory selection and outgoing conflict. The normal candidate-selection path reads that veto. The subsequent section expansion does not: it checks only the local context matcher and adds S because H is present. The resolver is dependency-only, so H, M and S can survive together. [Conflict veto][section-admission-veto], [expansion][section-admission-expansion], [resolver][section-admission-resolver]

This fixture uses the existing directional meaning of a mandatory atom's conflict. It does not assume that all conflict edges are symmetric or that every conflict must be rewritten as a generic incompatibility relation. Those distinctions matter because the existing Mangle rules treat mandatory supersession and optional competition differently.

A local addition of `atomToolSatisfied` to expansion would fix Finding A's missing-tool witness but **not** this veto witness. A final dependency certificate also cannot establish conflict consistency. The target is preservation of authoritative admission, not merely one more field check.

### R5. Finding C: the mismatch also rejects valid inherited context

The same partial matcher is stricter than the authoritative policy in another direction:

- `GenerateFacts` omits an empty context value. [Context generation][section-admission-generate]
- Mangle deliberately leaves `/intent` and `/lang` outside its fail-closed regime dimensions. With no value present, their ordinary mismatch rule does not block the atom. [Situational policy][section-admission-situational]
- `MatchesContext` calls `matchSelector` for both dimensions. A nonempty selector plus an empty context value returns false. [Matcher][section-admission-matcher], [empty-value rule][section-admission-empty]
- `CompilationContext.Validate` checks budgets, not mandatory presence of an intent or language. [Context validation][section-admission-context]

A concrete shipped-corpus conditional witness is the researcher head `identity/researcher/tool_usage` and its `execution-protocol` section. Both carry the researcher shard selector and the same research-like intent list; the head is mandatory, the section optional. In an otherwise valid researcher compile with an unresolved/empty intent and no independent section selection, Mangle can retain the head while post-selection expansion skips its section solely because of that empty intent. This is not budget pressure and not a deliberate author-level narrower section selector. [Built-in pair][section-admission-researcher]

This establishes a supported compiler-input counterexample, not that every normal researcher caller leaves the intent empty. No live occurrence rate was measured. Language-tagged sections have the same implementation-level mismatch, but the built-in researcher pair is sufficient to make the condition concrete.

Changing all missing dimensions to permissive Go matching would be equally wrong: provider/model, persona, mode, workflow phases and world state are intentionally fail-closed. The authoritative distinction must remain explicit rather than be rediscovered in a second matcher.

### R6. Proposed contract: policy-qualified expansion with final admission evidence

All new data shapes below are proposed, not existing APIs.

For one fixed compilation source/context snapshot, the result should carry:

- Initial relevance/mandatory selection.
- Section requests implied by selected heads, and head requests implied by relevant sections.
- Authoritative disposition for each requested atom: admissible, context-blocked, tool-blocked, policy-prohibited, conflict/supersession-lost, or prerequisite-unavailable.
- Final emitted IDs and variants, with their admission reason, dependency certificate and source identity.

A successful output must satisfy:

1. Every emitted atom has a current authoritative admission witness under that compilation's policy, catalog and context.
2. Expansion cannot undo an explicit veto or supersession.
3. Section/head presence creates a **request for policy consideration**, not permission to include.
4. Situational-missing and regime-missing selectors retain the exact policy distinction.
5. Every emitted section has its head; every surviving ordinary prerequisite is present.
6. Optional sections remain optional. Required closure still fails explicitly if infeasible.
7. Final rendered prompt and complete model-request bounds still apply.
8. Cache reuse must bind the same source vector and admission result; it must not validate one generation's IDs against another generation's metadata.

#### Preferred design direction

Put section relations into the same private policy scope as candidate metadata and have Mangle own admissible section/head expansion. Go can construct typed facts, index adjacency, measure fragment cost and perform bounded IO. It must not introduce a second executive decision rule.

Do **not** blindly add a recursive selection rule: conflict resolution, negation and dependency admission interact. Specify the strata first and prove that a section does not resurrect a superseded head or create a negative cycle. Preserve the existing all-facts-before-query barrier and full competitor universe. A design that queries eligibility only against the originally selected subset can miss the very vetoing competitor it must respect.

A conservative transition can keep Go's expansion as a proposal builder, then submit the proposed set and reasons for authoritative validation in the same scoped snapshot. Publication must be gated on that result. If repair removes a head, cascade through dependent sections; if repair reaches required material, return an explicit infeasible compile instead of silently certifying an incomplete prompt.

The checker must understand admitted **section expansion**, not merely require membership in the old `selected_result`: the whole purpose of the new feature is to add optional supporting sections without demanding separate vector hits. Rechecking the old selected set alone would regress the October 28 fix.

#### Authoring checks

Validate graph-level declarations: real head, self/cycle handling, bounded positive order, deterministic duplicate-order policy and deliberate narrower section selectors. Do not force tool equality across every head and section: a generic head with an optional tool-specific example is a legitimate design, provided the example is gated correctly. Record inherited and additional requirements explicitly so they cannot drift accidentally.

Whether section order requires contiguous-prefix inclusion is a separate semantic contract. Do not smuggle that decision into this admission repair; the prior closure audit already owns ordering and closure-aware packing.

### R7. Costs, risks and observability

Building the narrow host adjacency indexes takes O(n+s) for n candidates and s section edges, plus any declared dependency/conflict edges inspected; ordinary hash-map space is linear. Sorting sections adds O(sum over heads of s_h log s_h), where s_h is the number of sections for that head, unless order is reused from the immutable corpus. Policy reevaluation and join costs depend on the rule plan and corpus and must be measured rather than assumed linear.

Costs to record:

- Additional typed relation rows and policy evaluations.
- Time and peak memory for eligibility/repair/certification.
- Candidate, proposed, admitted, vetoed and emitted section counts.
- Input tokens and useful guidance retained, not only prompt length.
- Provider cached/uncached input, output, rejected calls, retries, rehydrations, verification and human correction across the whole task.
- Prefix changes when an eligible section legitimately enters/leaves.
- Compile refusals and time spent on failed/refused compiles.

The expected benefit is not necessarily fewer tokens per request. Correct permissive admission can restore useful instructions and increase prompt size; restrictive admission can remove misleading instructions. The goal is fewer wrong or repair-heavy model turns at equal accepted-task quality.

Main risks:

- **Policy duplication:** adding selected checks in Go fixes today's witness but misses tomorrow's veto.
- **Overcorrection:** demanding an independent vector hit for every section recreates head-only prompts.
- **Conflict perturbation:** treating a proposed section as a new relevance competitor can change established winners unintentionally.
- **Unknown versus denied:** absent situational values must not become either universal permission or universal rejection.
- **Generation drift:** admissibility and rendered bytes must share one source identity.
- **Misleading diagnostics:** source inherited from a head is not the same as an independent skeleton/vector admission; expose the expansion provenance.
- **Budget optimism:** admission correctness does not replace final render accounting or mandatory-closure refusal.

### R8. Proposed experiments, none executed

Use the production private-kernel compiler path, not just direct calls to `expandSections`. Keep ordinary CodeDOM/world context and actual catalog construction enabled for later end-to-end evaluation.

| Fixture | Required observation |
|---|---|
| Eligible H; optional S requires unavailable tool | S never emitted; H remains; no authority widened. Repeat empty/available/empty catalogs. |
| Mandatory M explicitly prohibits section S of selected H | Veto survives expansion; no H+M+S success violating policy. |
| Policy-excluded category or dependent section | Expansion does not bypass category prohibition or prerequisite veto. |
| Built-in researcher head/section, empty intent | Both use the declared situational-missing policy; omission is not falsely labeled budget pressure. |
| Known nonmatching intent | Both are excluded as policy requires. |
| Missing/wrong/right provider/model and world state | Fail-closed regime gates remain unchanged. |
| Selected section with vetoed/missing head | No head resurrection; section receives explicit disposition. |
| Ample/tight budget, long placeholders | Existing whole fragments, closure and refusal behavior survive. |
| Generation change and cache hit | Admission evidence and emitted metadata remain generation-consistent. |
| Conflict-free existing corpus and stable context | No unintended selection or prefix churn; report intentional additions/removals separately. |

Existing [section test definitions][section-admission-section-tests] cover ordered head expansion, a lone section's missing head and post-budget dependency cascades. They do not contain these tool/conflict/unresolved-intent witnesses. That is a bounded description of this file, not a claim that all repository tests were exhaustively searched or that any test was run.

Later task trials should compare current pinned code, tool-only patch, full policy-qualified expansion and full expansion plus final certificate. The tool-only variant is a useful negative control: it should fail the conflict witness. Hold model, tools, corpus, tasks and total budget constant; evaluate correctness, false completion and total cost. Include tasks needing the recovered section and tasks where the unavailable-tool example would be distracting.

### R9. Research basis and adoption queue

[Translation validation, Pnueli, Siegel and Singerman (TACAS 1998)][section-admission-translation] motivates validating the result of each transformation against its source contract rather than assuming every compiler pass preserves it. Applied here, the check is much narrower than program semantic equivalence: emitted atom IDs must satisfy explicit admission and dependency obligations. It cannot prove that an LLM follows the text or that authored sections are semantically sufficient. The publication supplies a validation pattern, not evidence of CodeNERD performance or prompt quality.

The primary specification for selector semantics remains the inspected Mangle rules, not generic advice to make all selectors fail closed. [Situational policy][section-admission-situational], [veto][section-admission-veto]

| Proposed item | Narrow deliverable | Checkable exit |
|---|---|---|
| JIT-SECTION-ADMISSION-01 | Policy-qualified section/head proposals | Missing-tool and explicit-conflict fixtures cannot return success containing prohibited sections. |
| JIT-SECTION-ADMISSION-02 | Authoritative missing-dimension parity | Built-in unresolved-intent fixture and regime-gate controls satisfy one documented semantics. |
| JIT-SECTION-ADMISSION-03 | Final admission provenance/certificate | Every emitted ID has a snapshot-bound allowed reason; closure and budgeting remain enforced. |
| JIT-SECTION-ADMISSION-04 | Same-quality cost evaluation | Added validation earns its measured host/model cost or the study reports no adoption benefit. |

The first milestone is precise: **adding supporting sections must not change which policy governs the prompt.**

[section-admission-main]: https://github.com/theRebelliousNerd/codenerd/commit/356f65e0e6bb639adaa90ab36212daa714114871
[section-admission-feature]: https://github.com/theRebelliousNerd/codenerd/commit/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c
[section-admission-root]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/agents.md#the-vision-steve-2026-09-18--read-before-any-architectural-change
[section-admission-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/prompt/agents.md
[section-admission-pipeline]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/prompt/compiler_pipeline.go#L301-L336
[section-admission-expansion]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/prompt/sections.go#L37-L105
[section-admission-matcher]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/prompt/atoms.go#L427-L540
[section-admission-resolver]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/prompt/resolver.go#L43-L134
[section-admission-validation]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/prompt/atom_schema.go#L688-L718
[section-admission-schema]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/prompt/atom_schema.go#L158-L166
[section-admission-render]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/prompt/compiler_render.go#L14-L63
[section-admission-filters]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/prompt/selector.go#L963-L1089
[section-admission-tool-check]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/prompt/atoms.go#L779-L811
[section-admission-veto]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/defaults/jit_compiler.mg#L197-L244
[section-admission-selection]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/defaults/jit_compiler.mg#L246-L329
[section-admission-context]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/prompt/context.go#L510-L525
[section-admission-generate]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/prompt/context.go#L783-L842
[section-admission-situational]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/defaults/jit_compiler.mg#L73-L155
[section-admission-empty]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/prompt/atoms.go#L580-L602
[section-admission-researcher]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/prompt/atoms/identity/researcher.yaml#L87-L144
[section-admission-section-tests]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/prompt/sections_test.go#L25-L75
[section-admission-closure-study]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/Docs/audits/jit-dependency-closed-budgeting.md#as-built-2026-10-28
[section-admission-reuse-study]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/Docs/audits/jit-generation-aware-optimization.md
[section-admission-ledger-study]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/Docs/audits/context-ledger-prefix-costs.md
[section-admission-tool-study]: https://github.com/theRebelliousNerd/codenerd/blob/0de7ff76968955ad5185d8dd13b85bc05775aa98/Docs/audits/request-visible-tool-contract-and-investigation-affordances.md
[section-admission-evolution-study]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/Docs/audits/prompt-evolution-lifecycle-and-evidence-accounting.md
[section-admission-translation]: https://weizmann.elsevierpure.com/en/publications/translation-validation-2/


[section-admission-closure]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/prompt/sections.go#L121-L173
