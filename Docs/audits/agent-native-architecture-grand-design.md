# Agent native architecture for correct parallel software maintenance

**Research design for CodeNERD — 28 October 2026.** This document synthesizes Steve’s architecture questions, the supplied GPT 6 Pro discussion, and inspected CodeNERD source. It proposes a program of experiments, not an implementation plan already approved for execution. No software, tests, models or benchmarks were run for this investigation. Examples and schemas are illustrative, not supported APIs. Source evidence is pinned to `main` commit `19e40566a4aff304fd00ca252821be74bde3d3f2`; the current main head was rechecked at 05:03 UTC on 28 October 2026 and was unchanged. The companion [governed code replication study](codedom-governed-code-replication.md) supplies narrower lifecycle detail; this document is the overarching architecture synthesis.

## Reader map

Read the central bet and first principles for the argument, then follow the question you need to answer:

- **What already exists in CodeNERD?** [Source-grounded foundations](#what-codenerd-already-supplies).
- **Should we change folders or the agent view?** [Physical locality](#physical-locality-and-progressive-disclosure) and [change packets](#compile-a-change-packet-over-codedom).
- **How can many workers proceed without hidden conflicts?** [Parallelism budget](#architecture-as-a-parallelism-budget) and [worked workflows](#three-worked-parallel-workflows).
- **When should we duplicate code?** [Declared replication relationships](#treat-duplication-as-a-declared-relationship).
- **How do Mangle and verification keep claims honest?** [Maintained evidence graph](#make-evidence-a-maintained-dependency-graph).
- **How do we avoid enormous build and test gates?** [Rust build and maintenance boundaries](#rust-workspaces-separate-maintenance-and-build-bottlenecks).
- **Which alternatives should compete, and what would reject them?** [Architecture options](#architectural-options-worth-competing), [research design](#a-research-program-that-can-distinguish-the-mechanisms), and [pilot](#a-decisive-pilot-without-a-repository-rewrite).

## The central bet

**Design software around the cost of making its next correct change, not around the cost of typing its current implementation.**

When many reasoning workers can produce code cheaply, source compression is no longer an adequate proxy for maintainability. The scarce resources become locating the right behavior, recovering the assumptions behind it, coordinating changes to shared meaning, and producing trustworthy evidence for the assembled result. Cheap generation does not make those costs disappear. It can multiply them.

The ambitious proposal is an architecture with **small local working worlds for coding agents, explicit maintenance boundaries for parallel work, and a globally consistent account of contracts, authority and evidence**. Locality is something the harness must construct and verify, not something the model should assume because its prompt is short.

That suggests three independently selectable interventions:

1. **Change the working representation.** Compile task-specific source views from CodeDOM, contracts, wiring and current evidence. Make the worker begin with a justified next action rather than a repository scavenger hunt.
2. **Change the maintenance architecture.** Organize capabilities so common changes have small owned read/write/effect surfaces and independently verifiable implementations. Permit specialization where the implementation boundary genuinely reduces coordination.
3. **Change the production of evidence.** Treat verification as a revision-bound dependency graph with bounded local feedback, explicit integration obligations and an authoritative acceptance transition.

These interventions can reinforce one another, but they are not interchangeable. Better views do not remove shared-write contention. More physical copies do not remove shared requirements. More tests do not establish that the tested implementation is actually wired into production.

**The bold alternative to DRY is not “copy everything.” It is to make the unit of reuse a behavioral obligation and its evidence, while choosing source sharing, generated specialization or independent implementation according to the work.** This is a research hypothesis with established ingredients, not a claim to have invented modularity or proved agent superiority.

## First principles from the worker experience

The following is a design analysis of the work interface, not a claim about subjective experience or empirical measurements of this model.

A newly admitted worker has several uncertainties: what the request actually requires, which source is authoritative, which facts remain fresh, what it may change, which other changes are in flight, how to distinguish a plausible patch from a completed capability, and what to do when its understanding is incomplete. A large context window increases carrying capacity; it does not settle any of those questions. A hundred workers can repeat the same uncertainty a hundred times.

The useful architecture should exploit what language-model workers can do: reason over heterogeneous representations, produce alternative mechanisms, translate an explicit behavioral requirement into different local implementations, and investigate independent hypotheses concurrently. It should avoid making them do repeatedly what a host can determine once: enumerate a known closure, resolve source identities, compare revisions, schedule exclusive resources, or decide whether an evidence record refers to the current code.

### The work unit should be a decision not a bag of files

A worker needs enough evidence to decide its next consequential action. For a display-label change, that may be one element and a presentation contract. For a five-line retry change, it may include idempotency, transaction state, deadlines, cancellation and ambiguous external effects. Token allocation should follow the decision’s dependencies, not lines changed or one universal prompt template.

The desired interface answers:

- What behavioral difference is requested, and what is explicitly not requested?
- What source and contracts establish the current behavior?
- What can be safely assumed without reopening an implementation?
- What authority is available now, and what actions remain forbidden?
- What evidence would falsify the proposed change?
- What must be revisited if this task discovers a new dependency?

This is deliberately more demanding than retrieving similar code. Relevant context and sufficient context are different properties.

### Parallelism is an architectural resource

Worker count is not throughput. Useful concurrency requires independently productive tasks, enough model capacity, independent edit surfaces, adequate build/test capacity, and integration mechanisms that do not collapse into one coordinator’s serial reasoning.

A helper with 500 callers does not prevent compilation or necessarily harm parallel development. If its contract is stable, it may reduce everyone’s work. A volatile helper whose behavior is implicitly reinterpreted by each task is a coordination hotspot. The objective is to eliminate avoidable shared negotiation, not to eliminate all shared code.

There should not automatically be one resident agent per module. Stateless workers can receive reusable structural preparation and bounded task state when needed. Resident specialists are an option only if the measured benefit of retained knowledge outweighs startup, idle, stale-memory, coordination and lifecycle costs. Shared structural caches do not imply shared model reasoning or shared billed prompt caches.

### A shorter prompt can still be a worse architecture

If omitted context returns later as a regression, a mistaken assumption, a human clarification or a giant integration repair, the system moved cost rather than removed it. Likewise, a clean small module may be a poor agent boundary when its behavior depends on hidden state across ten others.

The real objective is **low total cost per correctly integrated change over a maintenance horizon**, under explicit safety and quality constraints. “Fewest tokens” is a resource objective within that constraint, not permission to deliver less capability.


## What CodeNERD already supplies

| Inspected evidence | What it establishes | What it does not establish |
|---|---|---|
| [`internal/tools/codedom/doc.go`](https://github.com/theRebelliousNerd/codenerd/blob/19e40566a4aff304fd00ca252821be74bde3d3f2/internal/tools/codedom/doc.go), package tool inventory | Declaration-oriented reading/search, callers/callees, imports, Mangle predicate outlines, transactional edits and impacted-test interfaces already belong to CodeDOM. | That capability cells, semantic conflict scheduling or every desired operation are implemented. |
| [`internal/world/codemodel/model.go`](https://github.com/theRebelliousNerd/codenerd/blob/19e40566a4aff304fd00ca252821be74bde3d3f2/internal/world/codemodel/model.go), `Element` | Elements retain byte spans, structural identity and hashes of their own bytes; unrelated file edits need not invalidate every element revision. | That independent element writes preserve cross-element behavioral invariants. |
| [`Docs/architecture/orient/12-DEBT-ORPHANS-AND-CLEANUP.md`](https://github.com/theRebelliousNerd/codenerd/blob/19e40566a4aff304fd00ca252821be74bde3d3f2/Docs/architecture/orient/12-DEBT-ORPHANS-AND-CLEANUP.md), target-state header and §§1–4 | Existing design covers debt facts, near-duplicate sensing, durable classification and generalized cleanup transactions. | These target-state statements are not fresh runtime evidence; this document does not repeat their older implementation-gap claims as current facts. |
| [`agents.md`](https://github.com/theRebelliousNerd/codenerd/blob/19e40566a4aff304fd00ca252821be74bde3d3f2/agents.md) | Structural world knowledge, bounded context, Mangle executive authority, typed operations, verification and spec-first discipline are existing project commitments. | Permission for agents to redefine safety policy or regard their own prose as proof. |

Current [`repoint.go` lines 20–38 and 75–94](https://github.com/theRebelliousNerd/codenerd/blob/19e40566a4aff304fd00ca252821be74bde3d3f2/internal/tools/codedom/repoint.go#L20-L94) already expresses the principle that a blast-radius operation belongs in a structural tool rather than many model-directed text edits. Its Go entry path refuses incomplete use observations and exposes language-specific limits. That is inspected implementation evidence, not proof of every runtime path or of semantic equivalence. The historical tool-call counts in its comments are repository-reported motivation, not measurements from this investigation.

The companion [governed code replication proposal](codedom-governed-code-replication.md) treats clone-family mechanics. Here duplication is one architectural choice, not the main mechanism.

## A cell is an ownership boundary, not a microservice

A capability cell contains its public contract, local implementation decisions, state ownership, lifecycle rules, adapters, verification obligations and human-facing rationale. It may compile into the same Go process as every other cell. Network deployment is optional and often an unnecessary cost.

Illustrative layout—not a prescription to reorganize CodeNERD now:

```text
capabilities/
  INDEX.md                    # bounded top-level capability map
  research/
    CAPABILITY.md             # purpose, non-goals, ports, ownership, next map
    contract/                 # stable inputs, results, effects, compatibility
    workflow/                 # orchestration local to this capability
    adapters/                 # provider-specific translations
    verification/             # contract and real-path obligations
  patch_delivery/
    CAPABILITY.md
    contract/
    workflow/
    adapters/
    verification/
authority/                    # shared security/authorization semantics
composition/                  # registration and cross-cell integration
```

There is no required directory count, fixed taxonomy or files-per-cell threshold. A small capability may use three files. A large one may have nested maps. Splitting by “models/services/utils” alone often puts one change across every layer; putting all layers in a cell can localize change, but cannot eliminate genuine cross-cutting obligations.

**Three graphs must remain distinct:**

1. Runtime dependency: what calls, imports or communicates with what.
2. Maintenance dependency: what must change together to satisfy a requirement.
3. Understanding dependency: what an agent must inspect before changing something safely.

A stable executable contract can reduce understanding dependency without removing runtime dependency. Local implementations can reduce maintenance dependency while increasing code volume. A directory move can improve discoverability while changing none of these semantics. Report each effect separately.

### Boundary contract

Each cell declares: stable ID and owner; accepted inputs and returned outcomes; effects and authority requirements; state schema/ownership; cancellation/error/retry semantics; compatibility policy; production entry paths; outbound contracts; security and correctness invariants; evidence producers; revision-scoped checks; and unresolved assumptions.

The contract is deliberately stronger than a function signature. “Returns an error” is not enough when one implementation has already charged a card, persisted a partial result or launched a background job. Boundary claims that cannot yet be checked remain explicit assumptions, not green checks.

## Physical locality and progressive disclosure

The repository should not require a model to consume a thousand-name directory listing before finding a subsystem. Physical layout and tools should cooperate:

1. A bounded root map lists capability IDs, purpose, ownership and child-map locations, not every file.
2. A capability map exposes its contract, ordinary edit surfaces and next-level maps.
3. CodeDOM resolves the selected symbols and revision-bound source bodies.
4. A separate completeness query retrieves all affected consumers, effects and verification obligations before mutation or acceptance.

**Discovery can be approximate; safety closure cannot silently be approximate.** A relevant-looking top-k result must never imply “these are all consumers.” Results should carry `scope`, `revision`, `continuation`, `truncated`, and `completeness = complete | partial | unknown`. A small page is compatible with exhaustive host-side computation; it is not evidence that the host did little work.

Maps should be generated or checked against structural indexes plus explicit human-owned responsibility descriptions. Static analysis cannot infer purpose reliably. Unknown files must appear in an unresolved bucket; a stale map must retain its snapshot identity. Missing an entry is not proof it is unused. Case changes, symlinks, generated trees, plugins and configuration-driven registration need explicit inclusion rules.

Do not rigidly optimize directory breadth. A narrow hierarchy can replace one huge listing with fifteen dependent calls. Measure total navigation depth, bytes transmitted, host enumeration, index refresh, latency and missed-item rates. Cache maps by tree/content revisions and update affected ancestors; avoid rewriting every map for an unrelated source edit. Preserve direct symbol search as a bypass when identity is known.

Human maintainers must still navigate with an ordinary editor, review ownership and reproduce evidence without relying on a proprietary agent memory. The repository remains intelligible when CodeNERD is unavailable.

### Shared preparation without shared stale assumptions

Parallel startup creates its own bottleneck. If `n` workers independently pay orientation cost `O`, the work is approximately `nO`; shared structural preparation `S` plus per-task adaptation `o_i` changes the accounting to `S + sum(o_i)`, but may add a queue at the shared service. This is an accounting hypothesis, not a promised saving. Measure index contention, filesystem enumeration, embedding retrieval, model admission limits, warmup and cache misses. Admit workers progressively when startup would otherwise saturate the same resources; speculative unused packets are wasted work too.

The product requirement is a justified next action, not a repository scavenger hunt. Evaluate time and tokens to first useful edit, wrong-subsystem starts, missed constraints, redundant discovery, context-reset recovery and integrated correctness. A fast first edit that later requires widespread rework is not success.

## Architecture as a parallelism budget

One hundred agents do not create one hundred independent changes. A volatile shared file or contract can serialize work; a stable library with high fan-in need not. Compilation is a different bottleneck: the same dependency may be cheap to cache yet expensive to change collaboratively.

For task `t`, maintain estimated read set `R_t`, write set `W_t`, contract-change set `K_t`, and invariant/effect set `I_t`, all revision-scoped. Build a conservative conflict graph with tasks as vertices. Add an edge for write/write or write/read overlap, incompatible contract evolution, or competing effects/invariants. Symbol-disjoint changes can still conflict semantically.

```text
For each admitted task:
  discover its capability and boundary contracts
  derive a preliminary read/write/contract/effect footprint
  expand unknown dependencies or mark the footprint incomplete
  schedule a nonconflicting set, bounded by compute and verification capacity
  permit local work against the pinned contracts
  on new dependency or changed intent: revise footprint and reschedule
  before acceptance: refresh dependencies, verify combined integration state
  atomically compare the expected dependency/contract generations and publish
  if comparison fails: reject acceptance, refresh and reverify affected obligations
```

Acceptance must be a compare-and-swap (CAS) or equivalent locked transaction over the verified source, dependency, contract and evidence generations. A separate “check freshness, then publish” sequence permits a check/use race. Receipts remain bound to the verified revision; concurrent change cannot silently inherit them. If the backend cannot atomically enforce the required scope, acceptance remains pending rather than claiming safety.

The conflict graph governs independently acceptable changes, not a blanket ban on concurrent thought. A reader can explore a pinned snapshot while another task changes its dependencies; isolated speculative edits can also proceed when authorized. Write/read overlap means the result cannot claim independent acceptance until the changed read dependencies are revalidated or the work is replanned. This distinction preserves useful parallel exploration without spending stale evidence twice.

This is scheduling evidence, not a proof that inferred footprints are complete. Unknown footprints may require isolation or targeted exploration. Conflicts must not be hidden by “different branches.” A contract-change task becomes a migration wave with explicit consumer obligations; other cells may continue under the old version only when compatibility rules permit it.

For a candidate architecture `A` and realistic task distribution `D`, propose minimizing:

\[
J(A)=E_{t\sim D}[C_{discover}+C_{change}+C_{verify}+C_{integrate}+C_{repair}]
 + C_{migration}/H + C_{platform}/H
\]

subject to required behavioral, security and human-maintainability thresholds. `H` is the explicitly estimated number of maintenance tasks in the declared horizon, so migration and platform costs are amortized per task consistently with the expectation. Report sensitivity to that estimate rather than assuming an infinite horizon. Costs remain a vector of tokens, money, machine time and human time until explicit conversion weights are chosen.

For concurrency `n`, use an accounting model, not a speedup claim:

\[
T_n \geq \max(T_{critical\ path}, W/n, T_{shared\ verification}).
\]

Under this idealized lower-bound model, `W` is total work on the modeled worker resource, `n` is its parallel capacity, and the other terms are unavoidable critical-path and serialized verification durations. Coordination and conflict repair extend measured work and dependency paths; do not add their overlapping elapsed times again as separate lower-bound terms. Instrument the actual execution DAG and resource timeline. The conflict graph's independent sets describe available safe work under the estimated footprint, not guaranteed throughput. Pairwise graphs also miss higher-order conflicts; include group-level invariants and composition tests.

## Stable contracts, local variation, shared authority

Do not make every shared helper configurable until every consumer edits the same giant policy switch. Prefer local adapters behind stable boundary contracts when consumers genuinely evolve independently. Conversely, do not replicate authorization policy merely to avoid coordination: global consistency there is the feature.

| Decision | Useful condition | Cost or danger |
|---|---|---|
| Shared implementation | Same invariant and synchronized evolution | Shared changes may create migration/coordination waves |
| Local implementation behind common contract | Different mechanisms, same externally required behavior | More verification and security-fix propagation |
| Generated local variants | Explicit parameterized variation, reproducible generator | Generator becomes a shared dependency; regeneration and debug costs |
| Independent fork | Semantics intentionally diverge | Must sever false synchronization expectations and own future fixes |
| Context-only CodeDOM projection | Structure is sound; discovery is the main cost | Does not remove shared-write or shared-contract contention |

A repeated ten-line conversion may be cheap to own locally; repeated interpretation of an authorization policy may be unacceptable. Source similarity alone cannot decide. The replication companion specifies lifecycle handling; this proposal requires every copy to carry a declared consistency obligation or explicit independence.

## Integration is a link obligation, not an afterthought

A cell can pass local tests while never being registered or called. Its completion contract should bind:

`requirement → production root → registration → invocation → authority → effect/state → observed outcome → evidence revision`.

Missing links are not automatically defects—some capabilities are deliberately dormant—but they prevent claiming production availability. Evidence must distinguish static reachability, executed behavior, inferred intent and declared design.

An illustrative manifest records IDs, producer/consumer contracts, required scenario IDs, effects, lifecycle transitions, evidence freshness dependencies and allowed deployment modes. It does not contain a self-certified `verified: true`. External evidence producers establish individual obligations; an executive policy derives readiness for the exact integrated revision and environment.

For CodeNERD, proposed responsibilities are:

* **CodeDOM/world:** structural footprints, element revisions and dependency queries, preserving existing mechanisms.
* **Orientation:** explicit capability ownership, unresolved mapping and intended-vs-observed integration distinctions.
* **Mangle:** derive scheduling exclusions, compatibility and unresolved verification obligations from typed facts. Declare every predicate and make snapshot scope explicit; absence is not safety when discovery is incomplete.
* **JIT:** disclose the cell contract, relevant implementation, boundary changes and outstanding obligations in stages; never spend the entire context on a directory dump.
* **Host executors:** enforce mutation preconditions and produce receipts. A model must not certify its own completeness or broaden authority.

These are proposed allocation points, not findings that the named features are absent. Before implementation, compare them with current target-state specs and production owners; extend existing representations rather than inventing competing stores.


## Compile a change packet over CodeDOM

The proposed change packet is a task-level composition of existing observations, not another authoritative source store. **Canonical storage, agent working view and executable organization need not be identical.** A shared implementation can appear in several local working views without creating several implementations. A compiler can inline or specialize execution without changing the ownership of source. A physical copy creates a different maintenance obligation and must be labeled accordingly.

The analogy is to a materialized read view: denormalize the information presented to the worker before denormalizing the authority behind it. Unlike an ordinary read view, a coding view must preserve explicit edit mappings, source revisions, policy scope and unresolved dependencies.

Illustrative packet, not current CodeNERD JSON:

```json
{
  "packet_id": "provider-cancellation-task-42",
  "source_snapshot": "resolved-source-revision",
  "intent": {
    "change": "Preserve partial usage when a stream is canceled",
    "non_goals": ["Change retry policy", "Broaden network authority"]
  },
  "capability": "provider-a-stream-decoding",
  "contract_epoch": "reviewed-stream-contract-3",
  "editable": [
    {"element_ref": "resolved-decoder-ref", "revision": "observed-digest"}
  ],
  "required_behavior": [
    "Cancellation stops further reads",
    "Observed partial usage remains available",
    "Incomplete streams are not reported as complete"
  ],
  "production_path": ["registration-ref", "wrapper-ref", "decoder-ref"],
  "evidence_obligations": ["cancel-fixture", "usage-contract", "real-route-integration"],
  "unresolved": ["Dynamically chosen transport requires further observation"],
  "closure": {"scope": "declared-provider-route", "state": "partial"},
  "authority": "reference-to-current-host-checked-policy-not-a-grant"
}
```

A source fragment shown in five packets still has one canonical identity. The packet is not an editable concatenated file. Every proposed edit targets a canonical reference with a revision precondition. Conflicting proposals to different appearances of the same element must be reconciled, not accepted independently. A quoted contract is not permission to edit it.

### Packet construction contract

1. Resolve the requested behavioral change, owner and non-goals. Preserve ambiguity when intent is not established.
2. Resolve production roots, registration/configuration, relevant elements and effects through current indexes and explicitly scoped discovery.
3. Attach boundary contracts whose preconditions match the task. A contract can stop expansion only when it answers the decision at hand; otherwise retrieve implementation or mark the question unresolved.
4. Derive evidence obligations, including wiring, failure paths and relevant feature/platform configurations. Do not let the proposed patch choose its own acceptance oracle.
5. Produce a compact first view plus typed expansion handles. Retain full evidence host-side; include scope and completeness on every closure claim.
6. Register source, configuration, inventory, contract, toolchain and oracle dependencies. Record why each item was selected.
7. Before every governed action, recheck live authority and relevant preconditions. Before acceptance, recompute changed obligations and atomically bind the decision to the verified generation vector.

There are two different completeness problems. **Presentation completeness** asks whether the worker has enough information for its next decision. **Safety closure completeness** asks whether every applicable obligation in the declared domain has been accounted for. Neither can be inferred from retrieval ranking. The host may compute a complete closure while showing a small page; a model can also receive a huge packet whose closure is unknown.

A useful packet includes an unresolved ledger rather than a synthetic confident overview. For example: “static route resolved; runtime plugin selection not observed; production availability not established.” The same ledger should survive context resets and handoffs, retaining observations, hypotheses, rejected explanations and pending obligations separately. It should not store invented private reasoning or treat prose confidence as evidence.

### A packet compiler also needs a quality contract

Measure wrong-owner selection, omitted relevant dependencies, stale fragments, duplicate context, misleading completeness claims, cost to refresh, and the rate at which workers must escape the packet. A mandatory escape is not automatically a failure: a good packet should expose genuinely novel information. Repeated undocumented escapes for ordinary tasks indicate a bad boundary or poor packet construction.

No packet should be considered correct because its authoring model says it is comprehensive. Evaluate against independent task outcomes and deliberately planted dependencies. Run a strong control in which the existing architecture receives the same source and contracts without specialized editing APIs. That separates information value from orchestration value.

## Treat duplication as a declared relationship

The question is not whether two bodies look alike. It is whether they must continue satisfying the same evolving requirement.

| Relationship | Meaning | Maintenance policy |
|---|---|---|
| Repeated presentation | One source, several task views | Preserve identity and invalidate all dependent views |
| Generated specialization | One generator and explicit inputs create variants | Change the generator or approved inputs; reproduce and verify outputs |
| Independent local implementation | Similar code serves independently owned behavior | Allow divergence; do not infer synchronization from similarity |
| Shared obligation family | Different implementations owe the same named behavior | Propagate counterexamples and revalidation obligations |
| Unclassified clone candidate | Similarity or ancestry is known, intent is not | Investigate; do not auto-delete, synchronize or exempt |

**Duplicate mechanics when useful; do not accidentally duplicate ownership of truth.** Provider wire-format decoding may reasonably differ. Permission policy, cryptographic primitives and transaction authority should not become a constellation of handwritten alternatives merely to increase apparent parallelism. A local adapter can call centrally enforced authority without opening its implementation for every ordinary change.

A managed family records member identities and revisions, owner, approved relationship, contract versions, allowed variation, lifecycle state, generator lineage where applicable, environment scope, evidence producers and last complete membership reconciliation. Newly discovered members invalidate a prior membership closure. A missing reference is unresolved, not quietly removed. An independent fork needs explicit ownership of future fixes and any retained security obligations.

### Propagate a counterexample rather than a patch

Suppose cancellation loses the final usage update in provider A. Turn that failure into a reviewed behavioral requirement and reproduction. Derive obligations for all relevant family members. Provider B may already satisfy it; provider C may require a different state-machine change; provider D may be intentionally outside this contract. The harness needs evidence for each conclusion. Mechanically copying A’s patch could break B or impose A’s wire protocol on C.

This is where cheap reasoning can enable something more interesting than cheap copying: many workers can independently adapt a shared requirement to genuinely different mechanisms, while one authoritative family inventory prevents forgotten consumers. The benefit depends on reliable obligations, not agreement among workers. Correlated models can all miss the same edge case; compare against independently specified properties, adversarial examples and external expectations.

Deterministic generation is preferable when variation is purely mechanical. The model may design the transformation, but every generated instance need not pay for a fresh interpretation. Conversely, a template with hundreds of conditional branches can recreate the shared hotspot it was supposed to remove. Include generator edits and regeneration in conflict and build graphs.

## Make evidence a maintained dependency graph

A boundary can expose more than a signature: input conditions, effects, outcome categories, cancellation semantics, state ownership, compatibility and the evidence supporting those claims. Consider a persistence boundary whose failure can mean no write, successful rollback or unknown outcome. Collapsing all three into an undifferentiated error forces callers to infer crucial behavior from internals.

Tests and traces are bounded evidence, not universal proofs. Formal methods may strengthen selected claims, but no naming convention turns a test receipt into proof of arbitrary correctness. The contract must state its scope and assumptions. A module’s self-report is not an admissible evidence producer unless a trusted policy explicitly recognizes its mechanism and provenance.

Proposed dependency chain:

```text
source + configuration + inventory + toolchain
    → structural observations and boundary assumptions
    → applicable obligations and selected checks
    → revision-scoped evidence receipts
    → candidate readiness
    → atomic admission of the verified integrated revision
```

Invalidation must account for modifications and deletions, not only new facts. Negated conclusions depend on the domain searched: “no additional provider exists” depends on the membership inventory, so adding a file can invalidate the result even if no previously matched source changes. Unregistered reads make selective reuse unsound. [Skyframe’s documentation](https://bazel.build/reference/skyframe) provides an engineering precedent for explicit dependencies and incremental recomputation; it does not establish that CodeNERD currently has this proposed evidence lifecycle.

### Mangle should derive the obligations rather than remember them in prompts

The following is **illustrative rule notation**, not a tested Mangle schema or ready-to-install policy:

```prolog
needs_recheck(G, Impl, Contract, Version, Revision) :-
    replica(G, Family, Impl, Revision),
    required_contract(G, Family, Contract, Version),
    !check_passed(G, Impl, Contract, Version, Revision).
```

The variables in the negative premise are bound by positive premises. That is necessary but not enough: declarations, types, stratification, provenance, host producers, consumers and invalidation semantics still need design and validation in CodeNERD’s Mangle engine. Mangle is the project’s deductive database language; this example does not reduce the executive to a prompt convention.

Missing evidence means outstanding verification, not an observed test failure. No outstanding obligations means ready **only if** inventory and obligation derivation are complete within the declared scope, all required evidence is admissible and current, no partial effects remain, and current authority permits the transition.

Let `M_e` be the closed member inventory at epoch `e`, `O(e,m)` the applicable obligations and `V(e,m)` those discharged by admissible receipts. A necessary readiness condition is:

`inventory_complete(e) ∧ obligations_complete(e) ∧ no_partial_effects(e) ∧ ∀m∈M_e: O(e,m) ⊆ V(e,m)`.

This is policy-scoped readiness, not mathematical equivalence of all members. It must additionally be tied to an atomic acceptance transition against source, dependencies, contracts, inventory, oracle and evidence generations. Checking freshness and later writing “done” without a conditional transition leaves a race. If the storage/host boundary cannot enforce the needed atomic scope, completion remains pending. Existing bounded edit operations must not be described as universal multi-file atomic transactions.

### Avoid replacing one bottleneck with an evidence bottleneck

Store compact relationships and receipts rather than full source bodies in kernel facts. Reuse canonical CodeDOM identities. Partition immutable historical evidence from current admissibility indexes. Recompute affected obligations only when dependency registration supports that conclusion; otherwise widen conservatively. Compact or archive obsolete generations while retaining the provenance required for audit and rollback. Bounded active state must not mean silently deleting unresolved obligations.

These are extension points to examine against the live architecture, not findings that every mechanism is absent. The host provides facts and performs admitted effects; executive policy derives decisions. Do not move completion or delegation logic into a new Go packet-builder merely because it is convenient.

## Three worked parallel workflows

### Twenty provider adaptations under one stable contract

A coordinator obtains approval for a contract epoch defining cancellation and partial usage. Shared structural preparation identifies twenty provider cells and their production paths. Each worker receives only its local decoding mechanism, shared requirements and relevant evidence obligations. Workers produce local candidates in isolated workspaces, with separately budgeted build/test resources.

Worker 7 discovers an external side effect can occur before cancellation. It records the counterexample and proposes a boundary change; it cannot silently revise the global contract. Work independent of that assumption can continue. Dependent candidates lose current admissibility when the contract changes. A new epoch creates a targeted migration wave if the dependency inventory is complete, or a broader reconciliation otherwise.

The final assembled state must satisfy all applicable provider and composition obligations. Twenty passing local suites do not establish a correct registry, mutually compatible feature flags, or global usage aggregation. Integration is its own evidence obligation, not a ceremonial merge.

### A shared security fix during a feature campaign

Fifty workers are improving separate cells when a security invariant changes. Physical independence does not authorize continuing under revoked policy. The executive invalidates affected grants immediately, identifies scope through current membership and dependencies, and admits a repair wave under a new policy revision. Speculative reading may continue where permitted; protected actions cannot rely on the old packet.

A family member is not exempt merely because it has different text. Retirement requires explicit authority and evidence that no supported production route still uses it. Release eligibility waits for all affected supported members or an approved deployment boundary that excludes them. Measure the resulting consistency lag and repair cost: this is where unmanaged duplication may lose despite winning routine tasks.

### An unknown interaction despite a clean merge

Two workers modify separate files. One changes retry classification; another changes persistence cancellation. Both satisfy local checks, and their diffs merge cleanly. Together they can retry an operation whose side effect is uncertain.

A correct task model includes shared idempotency and effect-ordering obligations, not just filenames. If those relations were unknown, the system must discover them through broader integration scenarios or explicit uncertainty, not claim the conflict graph proved independence. The repair should improve the boundary/evidence model, so later tasks do not repeat the same hidden coupling. A failure record that merely says “try again” wastes the most useful architectural signal.

## Architectural options worth competing

The proposal is not one compulsory layout. Different bottlenecks imply different designs.

| Option | Strong hypothesis | Main cost | Discriminating experiment |
|---|---|---|---|
| Shared source with compiled task views | Most wasted work is repeated discovery | Packet construction and invalidation | Same code and worker, current navigation versus packets |
| Capability-oriented modular monolith | Physical ownership reduces routine change propagation | Boundary design and migration | Same packet system, old versus cell layout |
| Generated local specializations | Mechanical variation should not require repeated reasoning | Generator hotspot and output verification | Same contracts, generator versus shared helper versus handwritten variants |
| Managed independent implementations | Local mechanisms genuinely need different evolution | Mandatory-fix propagation and consistency lag | Long campaigns with common security/contract updates |
| Versioned composition graph | Stable contract epochs permit independent development lanes | Compatibility and integration state | Shared in-place API change versus staged epoch migration |
| Disposable candidate implementations | Several competing mechanisms can be evaluated cheaply | Rejected work and oracle demand | One candidate versus bounded portfolio under equal total budget |

The last option is deliberately aggressive. Keep the behavioral contract and independent acceptance oracle stable while several workers explore implementations; retain only an accepted candidate and its evidence. This differs from permanent duplication. Its value is search diversity, not parallel maintenance. It is inappropriate when evaluation cannot distinguish candidates, runtime costs are difficult to measure, or the workers can modify the oracle. Count all rejected candidates and duplicated discovery. More samples with correlated mistakes may add cost without information.

A versioned composition graph is similarly not a license for unlimited versions. Give migration epochs retirement conditions and compatibility limits. If all consumers remain forever pinned to old contracts, the architecture has exchanged compile contention for permanent protocol debt.

## Optimize architecture from maintenance evidence

Agent navigation and change traces can suggest where boundaries are wrong. Repeated opening of the same four modules may signal a missing behavioral boundary. Repeated identical fixes across copies may favor a canonical implementation or generator. Repeated independent specialization of a shared helper may favor local ownership. But traces can also reflect poor tools, incomplete indexing, formatting commits or the current model’s habits.

Treat an architecture proposal as a hypothesis with a reversible migration and a held-out maintenance forecast. Its author should state which task classes improve, which become harder, what source/evidence gets duplicated, and what observation would reject the change. Do not let the proposing model grade its own architectural taste.

A proposed architecture ledger can record a boundary’s expected edit closure, required context, contract fan-out, build invalidation closure, mandatory-fix propagation scope and observed exceptions. It is a measured operational profile, not a universal complexity score. Cyclomatic complexity remains one risk input; it cannot describe hidden effects, semantic coupling, global authority or test-oracle adequacy by itself.


## Failure modes and stopping rules

* **False independence:** contracts omit shared state, ordering or resource budgets. Test composition and effect conflicts, not only imports.
* **False confidence from clean merges:** independent textual edits interact behaviorally. SAM studies test-generated partial specifications for semantic conflicts; such tests are useful evidence, not complete proof. [Da Silva et al.](https://arxiv.org/abs/2310.02395)
* **Contract bureaucracy:** every local change needs shared approval. Keep internal changes local; version actual boundary changes and measure gate delay.
* **Task-distribution overfit:** co-change history reflects formatting, commits or team habits, not inherent coupling. Combine history with domain invariants and held-out changes.
* **Central generator bottleneck:** local files conceal one frequently edited source. Include generator changes and synchronized regeneration in conflict footprints.
* **Map blindness:** a directory index omits files or stale dependencies. Completeness state and unresolved inventory must remain visible.
* **Evidence explosion:** maintaining mappings and obligations costs more than the avoided exploration. Index incrementally; measure rather than assert savings.
* **Human displacement:** agents understand generated structures but operators cannot diagnose outages. Require ordinary source maps, rationale and reproducible investigation.

Stop expanding a pilot if it reduces local edit cost but increases integrated failure, repair cost or mandatory-fix latency. Retain a reversible source/path mapping and migrate one capability first. No global rewrite is justified by this proposal.

### Adversarial and operational failure modes

Repository prose, issue text, generated maps and imported contracts can contain misleading instructions. Treat them as evidence with provenance, not as permission to override the executive. A hostile source comment must not expand a packet’s write scope or replace its acceptance oracle. Family membership approval and policy revision remain governed transitions.

Cross-worker caches must preserve workspace and authorization isolation. Do not leak private source through globally reusable packets. A cache key needs the relevant identity and policy scope as well as content revisions; revocation can invalidate access even when bytes are unchanged. Preserve auditability without retaining sensitive payloads indefinitely.

A malicious or mistaken worker can try to remove obligations, weaken tests, classify a required member as independent, or hide an incomplete inventory. Separate who proposes these changes from who admits them. Record provenance and require current policy for each transition. A stalled verifier is unresolved, not permission to lower the quality floor. Avoid both failure modes: endless retry without new evidence and premature completion after an arbitrary attempt count.

## Rust workspaces: separate maintenance and build bottlenecks

Steve's Neurolog example motivates this question, but its historical size, test count and bottlenecks are user-reported context, not a current repository measurement. He clarified that “single-threaded” was hyperbole and that substantial adjustments have already been made. Nothing here diagnoses Neurolog's present build or claims it literally uses one CPU thread. No comparison with Zed is needed without inspecting its current source and workload.

Cargo exposes package selection, parallel-job controls and timing reports; workspace members are packages managed together, not automatically independent compilation tasks. Dependency order still constrains scheduling. Rust compiler code generation also has its own parallelism and incremental settings, so “one crate means one thread” is the wrong model. [Cargo build](https://doc.rust-lang.org/cargo/commands/cargo-build.html), [profiles](https://doc.rust-lang.org/cargo/reference/profiles.html), [workspaces](https://doc.rust-lang.org/cargo/reference/workspaces.html).

The architectural question is whether boundaries reduce the **invalidated build/test closure** for common edits and expose enough independent work without creating a long chain of tiny crates. Split along stable contracts and ownership, not arbitrary line counts. A volatile foundational API can invalidate many dependents; moving code behind the same volatile API does not remove that cost. Extra crate boundaries can introduce public API obligations, dependency management, code-generation/linking tradeoffs and duplicated generic instantiations. Do not claim a speedup without measuring the resulting graph and actual compiler phases.

A proposed verification ladder separates rapid feedback from release assurance:

1. **Local feedback:** selected package checks and narrowly relevant tests, clearly labeled partial evidence.
2. **Integration admission:** changed contracts, transitive impacted consumers, shared state/effect scenarios and required feature configurations.
3. **Release gate:** the project's full required suite and supported configuration/platform matrix, plus mandatory safety checks, on the integrated release candidate. Retain broader periodic checks when impact analysis is incomplete.

A parallel build lane needs more than a separate source directory. Give each candidate an explicit build closure, pinned compatible interfaces and an isolated output namespace. Share immutable dependency artifacts through correctly keyed caches, not mutable build directories whose writes race across workers. Admission should reserve CPU, memory and I/O as well as source ownership. The final integrated candidate still needs verification against the versions actually assembled; evidence from individually compatible snapshots cannot simply be unioned after their dependencies change.

The fast lane never converts a subset pass into a full-suite pass. A full test inventory may be sharded when resource isolation and ordering assumptions allow, but completion means all required shards passed at compatible revisions. Failing, canceled or absent shards remain unresolved. Cached results need sound keys covering sources, dependencies, features, toolchain, environment and relevant fixtures; otherwise they are hints, not reusable acceptance evidence. Contract changes can deliberately invalidate a wide set.

Future evaluation should distinguish cold builds, warm no-op builds, private implementation edits, shared interface edits, macro/generated-code edits and release builds. Record critical path, CPU/memory/I/O saturation, cache hit validity, code-generation/link time, test compile time, execution time and queue delay. At 100 agents, unconstrained build workers can reduce throughput through contention even when the source architecture is excellent. Schedule build and test capacity as shared resources separately from source-write ownership.


## A research program that can distinguish the mechanisms

A single patch benchmark will systematically underprice duplication’s future obligations. Use maintenance campaigns over the same initial behavior and requirements, with ordered sequences of independent enhancements, shared changes, failures and retirements. Preserve an untouched reference implementation where useful for differential checks, while recognizing that a reference can also contain bugs.

### Separate the variables instead of comparing bundles

Use a factorial core with three factors: compiled packets on/off; physical capability localization on/off; and managed family bookkeeping on/off where replicated implementations exist. Add separate arms for shared implementations, generated variants and independent local variants. The attachment’s four-arm A/B/C/D design is a useful initial pilot, but D combines packet and family management, so that contrast alone cannot attribute gains to either.

Retain strong baselines: current CodeNERD with CodeDOM, a conventional well-modularized architecture, and a generic editor receiving the same relevant information. Do not handicap the baseline with a raw 400,000-file dump. Match acceptance criteria, model versions, tools, hardware budgets and supported configurations. Where a treatment changes runtime behavior or source-language semantics, evaluate that explicitly rather than calling it a layout-only intervention.

Run concurrency levels of 1, 2, 5, 10, 25, 50 and 100 workers under both fixed-total-resource and fixed-per-worker budgets. The latter measures scaling when spending grows; it does not demonstrate efficiency. Include admission queues, failed startup, API capacity, build contention and coordinator work. Use repeated runs and randomized task orders; report uncertainty and effect heterogeneity rather than only an aggregate score.

Hold out task families as well as individual tasks. Include boundary-crossing tasks the proposed layout makes inconvenient. Incorporate security fixes, schema migration, common cancellation bugs, stale configuration, dynamic registration, broken indexes, revoked permissions, interrupted work, partial writes, rollback, and obsolete-member removal. Some failures should appear only after several apparently successful tasks.

### Independent acceptance and evidence challenges

Acceptance checks must be inaccessible to candidate weakening. Distinguish functional outcomes, production wiring, structural requirements, runtime resource constraints and maintainability outcomes. An agent may propose new tests, but cannot declare them sufficient merely because its patch passes them. Counterfactual fault injection and targeted mutation can assess whether evidence rejects plausible mistakes. Mutation scores depend on operator quality and equivalent mutants; they are not a correctness probability.

Test stale-evidence handling directly: alter a contract after local verification, add a previously absent family member, change feature configuration, replace a toolchain, and revoke an authority lease. Verify that old receipts cease to support current readiness and that unrelated receipts remain reusable only where dependencies justify it. This examines the evidence mechanism, not just the generated code.

### A scorecard for maintenance independence

| Measure | Useful interpretation | Anti-gaming check |
|---|---|---|
| Context required before a correct decision | Understanding cost | Include omitted dependencies discovered during repair |
| Boundary escapes per task | Contract sufficiency | Distinguish legitimate new scope from hidden ordinary dependencies |
| Change propagation closure | Shared maintenance burden | Count contracts, configuration and generated outputs, not just files |
| Accepted integrated tasks per wall-clock hour | End-to-end throughput | Include queues, failed tasks and final integration |
| Mandatory-fix consistency lag | Cost of managed duplication | Track every supported member and unresolved exclusion |
| Evidence rejection of plausible defects | Verification strength | Independent faults and nontrivial mutation operators |
| Semantic conflicts after clean text merges | Hidden coupling | Evaluate combined behavior, not only diff compatibility |
| Human diagnosis and review effort | Maintainability outside the harness | Fresh reviewers with ordinary tooling |
| Cost per accepted change over the campaign | Whole-system efficiency | Charge migration, rejected candidates, repair and retirement |

No single metric is the target. A proposal must pass explicit capability and safety floors and then show its tradeoff across this scorecard. Lower context size obtained by hiding risk is a regression.

### Full cost accounting

Keep a resource vector before converting to a single score:

`C = (input tokens, output tokens, model calls, CPU time, accelerator time, wall time, peak memory, storage, network, human time)`.

If a monetary comparison is desired, declare the price schedule and conversion assumptions. Do not add seconds to tokens or price hosted inference as though it were measured compute. Count:

- one-time source migration, contract creation, map/index construction and generator development;
- worker orientation, duplicated exploration, prompt/JIT assembly, cache misses and reasoning;
- candidate generation, failed attempts, discarded portfolios and conflict repair;
- local builds, test compilation, test execution, integration and release checks;
- evidence storage, dependency registration, invalidation and completeness reconciliation;
- runtime/deployment overhead, mandatory propagation, human review and incident recovery;
- decommissioning old variants and undoing an unsuccessful architectural experiment.

For horizon `H` maintenance tasks, compare total campaign cost and cost per accepted integrated task. Report unsuccessful and abandoned work separately rather than dropping it from the denominator without disclosure. Quality thresholds are prerequisites, not terms that a low token bill can buy away.

### What would change our mind

- If packets improve results but physical localization adds nothing, retain the existing source architecture and improve its working view.
- If cells improve routine edits but lose on cross-cutting tasks, identify the distribution under which that tradeoff is worthwhile rather than declaring a universal architecture.
- If generated variants beat independent variants, prefer deterministic specialization; do not keep agents in the loop for its own sake.
- If managed duplication wins early but loses on mandatory fixes, reject it for that family or strengthen the consistency mechanism before expanding.
- If total verification cost dominates despite fast edits, invest in correct incremental evidence and build boundaries rather than increasing worker count.
- If humans cannot diagnose the resulting system without the harness, the maintainability claim fails even if agents score well.

The aim is not to ensure the proposal wins. It is to discover which source organization, working representation and evidence mechanism jointly make future correct changes cheaper.

## Research relationships and important counterevidence

These sources inform the hypotheses, not claims of implemented CodeNERD performance.

**CodePlan** treats repository-wide changes as a planning problem involving incremental dependency and impact analysis. It supports moving mechanically recoverable change relationships out of repeated model exploration, not a conclusion that duplication is preferable. [Primary publication](https://www.microsoft.com/en-us/research/publication/codeplan-repository-level-coding-using-llms-and-planning-2/).

**Agent-Native Reusable Code Primitives**, an October 2026 preprint, packages implementations with contracts, dependency closure, tests and provenance. Its Appendix G.4 reports that the remaining difference between its full system and a generic editor supplied the same component fields has a confidence interval including zero. That motivates an information-matched control rather than assuming resident agents supply the benefit. Its Python-oriented reconstruction setting and acknowledged provenance/contamination limitations do not establish long-horizon maintenance benefits. [Paper and Appendix G.4](https://arxiv.org/html/2610.09079v1).

**Evaluating AGENTS.md**, revised September 2026, reports that repository context files did not generally improve success in its settings and increased inference cost. This argues against equating additional repository prose with useful preparation. It does not test or disprove every revision-bound task packet. [Primary paper](https://arxiv.org/abs/2602.11988v3).

**CodeTaste** separates executing specified refactorings from discovering the refactorings human developers selected, finding the latter harder for evaluated agents. Its combined functional and structural evaluation is relevant; human preference matching is not proof of the best architecture for agents. [Primary paper](https://arxiv.org/abs/2603.04177v2).

**Practical Mutation Testing at Scale** describes change-focused mutation, filtering and operator selection in Google’s setting. It motivates economical challenges to verification strength rather than indiscriminate whole-repository mutation. It does not make passing mutants a proof of correctness. [Primary publication](https://research.google/pubs/practical-mutation-testing-at-scale-a-view-from-google/).

## A decisive pilot without a repository rewrite

Begin with a behavior family whose local mechanisms differ but whose shared obligations are clear, such as provider stream decoding. Keep production authority centralized. Establish current task, context, build, integration and repair costs before changing the source layout.

1. Specify a packet over existing CodeDOM identity and revision mechanisms. Include real production wiring, evidence scope and unresolved dependencies. Compare it with existing tools on unchanged source.
2. Specify evidence-carrying behavioral boundaries for cancellation, incomplete results and partial usage. Reuse current ownership and executive representations wherever they already express these ideas.
3. In an isolated research treatment, compare a shared implementation, deterministic specialization and independently maintained variants under the same contract. Include propagation and retirement tasks, not just creation.
4. Add conflict-aware admission and independent build closures only where measured contention justifies them. Keep speculative reading separate from final acceptance.
5. Expand only if the whole maintenance campaign meets the agreed correctness floor and improves the chosen cost/throughput tradeoff. Otherwise preserve the useful representation work and reject the unnecessary structural change.

No new programming language, permanent agent per directory, blanket microservice migration or repository-wide clone campaign is required to test the thesis. Those remain separate options with substantial obligations of their own.

**The north star is software whose behavior is easy to locate, whose ordinary changes can proceed independently, whose shared obligations cannot be forgotten, and whose completion claims remain tied to the code that was actually checked.** Cheap code generation makes this architecture worth investigating. It does not excuse us from building it carefully.
