# Governed code replication on the existing CodeDOM

Status: proposed refinement, not implemented or evaluated. Initial source inspection used main commit `2ae31765adba7ee477d76e298dea3fd5a3ca0fd4`, 2026-10-28. At 04:40 UTC main had advanced 21 commits to `7d5f6f4d6406077467e9154b722890b86066b991`; the compared changes did not alter the cited CodeDOM, holographic, edit-record, orientation or predicate-consumer files. Updated prompt guidance was reread: source-vector invalidation and envelope-first capability gating must apply to proposed family context. No implementation, tests, training or inference was performed for this document.

## Decision

Do not replace CodeDOM with an agent-native code graph. CodeDOM is already that foundation. The useful next experiment is to make intentional duplication an explicit maintenance contract on top of it: allow independently located implementations while deriving which shared obligations must be reconsidered when one changes.

This is not a recommendation to copy the repository everywhere. It tests a sharper hypothesis: some dependencies exist only to share a small implementation, and the cost of understanding that abstraction may exceed the cost of maintaining two local implementations **if their required relationship is explicit and enforced**. Conversely, a shared security policy may be the worst candidate for duplication. These are empirical design choices, not consequences of cheap code generation.

The smallest useful new unit is a versioned **code family contract**, not a new source model, transaction engine or prompt format.

## Existing capabilities and overlap

| Concern | Inspected evidence | Status and implication |
|---|---|---|
| Element-centric source access | [CodeDOM tool surface](https://github.com/theRebelliousNerd/codenerd/blob/2ae31765adba7ee477d76e298dea3fd5a3ca0fd4/internal/tools/codedom/doc.go) | Implemented tools already expose elements, references, revisions, callers, callees and refactoring. Do not introduce parallel identities. |
| Expanded local understanding | [HolographicContext](https://github.com/theRebelliousNerd/codenerd/blob/2ae31765adba7ee477d76e298dea3fd5a3ca0fd4/internal/world/holographic.go#L28-L101) | Existing package, architectural, dependency and impact context includes explicit incomplete-parse accounting. A contextual projection is not a new invention here. |
| Multi-file edits | [apply_edits](https://github.com/theRebelliousNerd/codenerd/blob/2ae31765adba7ee477d76e298dea3fd5a3ca0fd4/internal/tools/codedom/apply_edits.go#L17-L40) | Implemented bounded 2–16 distinct-existing-file line edits with staging, optimistic checks and best-effort rollback. Not a universal atomic filesystem transaction or proof of behavior. |
| Change signals | [ExecutionRecord and RecordEdit](https://github.com/theRebelliousNerd/codenerd/blob/2ae31765adba7ee477d76e298dea3fd5a3ca0fd4/internal/tools/edit_record.go) | Existing edited-element recording feeds impact facts. Extend this path rather than introduce a second event bus. |
| Clone discovery and generalized cleanup | [Orientation cleanup vision](https://github.com/theRebelliousNerd/codenerd/blob/2ae31765adba7ee477d76e298dea3fd5a3ca0fd4/Docs/architecture/orient/12-DEBT-ORPHANS-AND-CLEANUP.md), [Mangle surface](https://github.com/theRebelliousNerd/codenerd/blob/2ae31765adba7ee477d76e298dea3fd5a3ca0fd4/Docs/architecture/orient/09-MANGLE-SURFACE.md) | Target-state documents already propose near-duplicate signals and generalized cleanup transactions. Do not present either as missing innovation. |
| Bounded replicas | [Predicate consumer validation](https://github.com/theRebelliousNerd/codenerd/blob/2ae31765adba7ee477d76e298dea3fd5a3ca0fd4/internal/core/predicate_consumers.go) | Existing replicas concern kernel facts and ownership, not replicated source implementations. Reuse the explicit-owner principle, not its representation blindly. |

Bounded default-branch searches for code families, source replicas and `code_near_duplicate` found no executable family-contract lifecycle; the latter appeared in orientation target documents. This is evidence for a candidate extension, not an exhaustive absence proof. Existing audit recommendations about structural completeness and holographic continuation remain related work, not additional defects asserted here.

## The missing distinction is why two pieces of code must agree

Similarity does not establish shared intent. Two implementations can look alike and correctly diverge. Two implementations can look different yet both be required to enforce the same invariant.

Define a family using a durable ID, contract revision, owner approval, members resolved to canonical CodeDOM references, and explicit relation semantics. Initial relation examples are deliberately non-exhaustive:

- **Generated:** one authoritative generator or template; member edits are forbidden except through an approved regeneration process. Preserve generator revision and inputs.
- **Synchronized obligation:** members may have different bodies but must satisfy named common requirements. A change creates review and verification obligations; it does not authorize copying text.
- **Independent fork:** shared ancestry is recorded, but automatic propagation is forbidden. Selected security advisories may still require independent review.
- **Unclassified candidate:** similarity is a discovery hint only, with no permission to synchronize, delete or refactor.

Use a read-only expanded CodeDOM view when navigation is the problem. Use a physical clone only when the implementation placement itself is worth changing. These are different interventions and must be evaluated separately.

## Proposed contract

The following is illustrative data, not a supported configuration or current schema. Reference and revision placeholders must be resolved by the actual CodeDOM adapter; never invent production reference syntax.

```json
{
  "schema_version": 1,
  "family_id": "bounded-retry-adapters",
  "contract_revision": "reviewed-contract-digest",
  "relation": "synchronized_obligation",
  "authority": "reviewed-project-policy-reference",
  "members": [
    {"ref": "resolved-adapter-a-ref", "revision": "observed-a-revision"},
    {"ref": "resolved-adapter-b-ref", "revision": "observed-b-revision"}
  ],
  "shared_requirements": ["honor-cancellation", "bounded-retries"],
  "allowed_differences": ["provider-error-classification", "backoff-policy"],
  "change_policy": "derive-review-obligations-not-text-propagation",
  "verification_scope": "per-member-and-production-entry-route"
}
```

Required additional storage fields include workspace identity, family membership epoch, policy revision, authorized editor, evidence IDs, candidate-to-approved transition, last complete membership reconciliation, and reasons for approved exclusions. Store identifiers or small facts in the kernel, not entire duplicated bodies. Read bodies through CodeDOM on demand.

Family membership is not a claim that every duplicate in the repository has been discovered. Completion is closed over the declared family at its epoch. Newly discovered members invalidate that closure; undiscovered candidates remain an explicit residual risk.

## Change algorithm and executive contract

1. Resolve the family contract and every declared member against a fresh structural snapshot. Missing, ambiguous or stale members are unresolved obligations, not silently absent peers.
2. Observe the changed member through the existing edit-record path, plus ordinary world rescans for edits outside CodeNERD. Model-mediated writes are not the only change source.
3. Join the changed reference with approved family memberships. For each shared requirement derive a review obligation for each member. A semantic impact classifier may propose exemptions but cannot grant them.
4. Present one family summary, the changed body, each relevant difference and explicit omitted members in the existing CodeDOM/Holographic context. Do not concatenate all bodies by default. JIT atoms explain the contract and expose available tools; they are not an authorization substitute.
5. The model proposes member-specific edits or an evidence-backed no-change decision. Mangle derives admitted work from current approvals, scope and unresolved obligations. Ordinary write permissions remain necessary.
6. Existing CodeDOM tools stage and apply authorized edits within their actual limits. For families beyond tool bounds, maintain explicit pending stages; do not call sequential batches atomic. Capture partial outcomes and do not publish completed status during recovery.
7. Attach verification evidence to the exact post-edit member revisions, policy revision, environment and named shared requirement. Passing a common helper test cannot substitute for evidence that each production route uses the intended implementation.
8. Re-read membership, member revisions and contract revision before family completion, then publish the completion record using an atomic compare-and-swap against that exact generation vector, or an equivalent coordinator transition. A reread followed by an unconditional write races with another worker. Any mismatch invalidates affected evidence and restarts obligation derivation. The completion record attests only to its named snapshot; later mutations revoke current release eligibility until revalidated. Report unresolved or unsupported obligations distinctly from failures. Do not hold a whole-family lock during model reasoning or verification; serialize only the final conditional transition and genuinely conflicting effects.

For family F, change epoch e and member m, let O(F,e,m) be the finite approved obligations, and V(F,e,m) the obligations discharged by current, admissible evidence. Completion requires:

`complete(F,e) iff membership_complete(F,e) and no_partial_write(F,e) and for every m in members(F,e), O(F,e,m) is a subset of V(F,e,m)`.

This is a bounded policy conclusion about the atomically published generation vector, not a theorem that arbitrary implementations are equivalent. Negative predicates must be evaluated only against completed snapshots; absence of an evidence row does not imply a failed test, and absence of a member row does not prove no peer exists.

The important design is **shared obligations with local implementations**. Sharing all source is one possible implementation; sharing none while losing the obligations is not acceptable.

## Integration boundaries

- **World and CodeDOM:** canonical reference resolution, snapshot/revision validity and changed-element observation. Refactoring moves migrate membership through an explicit identity mapping or mark it unresolved; do not recover by name alone.
- **Orientation:** candidate discovery and operator intent classification. Similarity only proposes family membership. Its planned near-duplicate score must not become a semantic equivalence score.
- **Mangle:** derive required reviews, admitted effects and completion from owned, versioned base facts. Scope any consumer replicas explicitly. A new family declaration cannot authorize its own writes or relax the existing security policy.
- **Session and tools:** retain effect receipts and failure states. A rollback receipt is not verification; an attempted edit is not a completed change.
- **JIT and broker:** compile bounded family context using existing selection and token accounting. Reuse a stable contract prefix when appropriate, but stale contract content must be invalidated regardless of cache savings.
- **Northstar and specifications:** requirements must refer to reviewed intent, not infer the project wants synchronization from similar code. Changing shared requirements is a policy edit with its own authorization.

## Massive parallel coding without invisible coupling

Physical copies can reduce write contention, but they do not make shared requirements independent. Separate the two kinds of concurrency explicitly:

- A worker may edit an independent member under a fixed immutable contract version and a declared member read/write set. It cannot change shared requirements as a side effect of a local implementation edit.
- Workers changing different members can proceed in parallel if their read sets do not assume one another's mutable behavior. CodeDOM revisions detect textual conflicts; requirement-level dependencies still require an integration check.
- A shared requirement change creates a new immutable contract version and a family migration epoch. Existing workers either finish against the old version with no claim of new-version completion or rebase their obligations. Do not edit the contract in place beneath active work.
- Global security fixes are family-wide obligations even when source edits are disjoint. Parallel workers can implement local fixes, but the family cannot claim rollout completion until every affected member has current evidence or an explicit authorized retirement. An exemption is not inferred from a lack of similar text.
- Completion aggregation joins member receipts for the same family epoch. A collection of locally successful but mutually incompatible versions is not a verified family release. Integration authority remains separate from the workers whose patches it judges.

A useful scheduler reserves the minimum write set, records the full read set and contract version, and separates independent member work from the family-wide admission barrier. It should avoid a global repository lock: lock or serialize only genuinely shared policy transitions and overlapping effects. Optimistic concurrency still requires revalidation immediately before admission, and an uncertain write outcome must be reconciled before retrying.

Independent source edits are not necessarily independent builds. Each member task also declares its package or module build closure, pinned interface and contract versions, isolated workspace and artifact ownership, dependency/environment identity, and verification inputs. Two local copies inside a coupled build may still contend over generated files, dependency manifests, caches, databases or deployment state. Worker-owned outputs and correctly keyed shared caches prevent one worker's build from becoming another worker's unexplained evidence. A local build receipt attests only to that closure and input snapshot; final integration must verify the assembled compatible versions. Five hundred callers of a stable API are not intrinsically a problem. The governing variables are mutable interface exposure, build and verification blast radius, and how much work shares contested state.

Measure verified throughput as accepted, production-integrated changes per wall-clock hour at fixed total model/tool/verification budget, not number of patches emitted. Sweep worker count and report conflict/rebase rate, blocked time, verification queue latency, stale-read rejection, duplicated work, global-fix completion lag, and cost per accepted change. Include the control where existing shared abstractions are edited with fine-grained scheduling; duplication must beat that, not an artificially serialized baseline.

## Costs and failure modes

For a lifecycle of H changes, compare total cost rather than source lines:

`C = C_setup + C_discovery + sum_over_H_changes(C_context + C_model + C_tools + C_verification + C_review + C_recovery) + C_storage + C_build + C_future_migration`.

Every C term must use the same declared accounting unit, such as dollars including an explicit labor conversion; do not add raw tokens, seconds and CPU hours. Where no defensible conversion exists, report a resource vector and its Pareto tradeoffs instead. Shared costs are attributed once, and unsuccessful work remains charged.

Report tokens by input/output/cache category separately from dollars, latency, CPU, memory and human time. Duplicates can reduce navigational work while increasing scan, index, compilation, test, review and incident cost. A generated clone is cheap to emit but not free to build, deploy or secure.

With n members and r requirements, naive obligation materialization is O(nr) per change; store compact family requirements and join lazily where possible. Candidate all-pairs clone discovery is quadratic in element count; any approximate candidate index must declare its recall limitations. Bound active family size and context expansion without treating bounded output as complete evidence.

Critical failures include accidental coupling of independent forks, lost security patches, incorrect exemptions, stale identity after moves, generator/manual-edit conflicts, tests copied with the same bug, incomplete production routing, concurrent family edits, expensive verification fanout, and contract sprawl. The mechanism must make these visible; it cannot promise that metadata eliminates them.

## Evaluation that could reject the idea

Use the same models, task distribution, verification environment, tool budgets and repository snapshots across these arms:

1. Existing CodeDOM and HolographicContext, with shared abstractions unchanged.
2. The same code with better task-context selection only.
3. Intentional local duplication without a family contract.
4. Local duplication with the proposed family obligations.
5. Generator-maintained local implementations where a generator is feasible.

Evaluate multi-change trajectories, not a single patch: first feature addition, later cross-cutting correction, deliberate divergence, security patch, rename/move, concurrent edit and rollback failure. Include structurally dissimilar implementations sharing requirements and similar implementations that must remain independent. Split by repository and family to prevent duplicated train/evaluation examples or memorized near copies from inflating accuracy.

Primary outcomes: independently verified production behavior and delayed regression rate. Secondary outcomes: total tokens and cost per verified lifecycle, time to completion, human review burden, unintended synchronization, missed members, unnecessary edits, evidence invalidations, and recovery fidelity. An arm that reduces turns but increases escaped regressions loses. Retain negative results where task projections alone deliver the benefit and physical duplication contributes only cost.

For a first bounded experiment, choose small non-security-critical adapters with explicit requirements and naturally different local behavior. Do not adopt a repository-wide duplication policy from that result. Expand only if gains persist under later maintenance and adversarial divergence cases. These experiments are proposed, not executed.

## Prior work and limits of the claim

[Toomim, Begel and Graham, Managing Duplicated Code with Linked Editing](https://www.microsoft.com/en-us/research/wp-content/uploads/2016/02/toomim-linked-editing.pdf) already explored maintaining linked source regions rather than forcing language-level abstraction. This proposal does not claim to invent synchronized duplication, and the paper's human-editor results are not evidence of CodeNERD token savings.

[Kapser and Godfrey, Cloning Considered Harmful Considered Harmful](https://plg2.cs.uwaterloo.ca/~migod/papers/2006/wcre06-clonePatterns.pdf) documents cases where cloning serves engineering purposes and argues that motivation matters before refactoring. It supports asking why a clone exists, not assuming all cloning is beneficial or automatically safe.

The CodeNERD-specific refinement is to connect clone intent, canonical element revisions, existing effect records and Mangle-derived verification obligations. The novelty claim is deliberately limited to a proposed integration to investigate. The first question remains whether existing CodeDOM views already eliminate enough navigation cost that actual duplication is unnecessary.

## As built (2026-10-28)

- **Contract file:** `.nerd/code_families.json` (`internal/codefamily`) holds `schema_version`, and per family `family_id`, `members` (CodeDOM element refs), `shared_requirements` and `allowed_differences`. The loader refuses unknown fields, a wrong version, duplicate IDs or members, a family with fewer than two members, and a family with no shared requirement. It is loaded into the kernel at boot (`system.loadCodeFamilies`); a malformed file loads nothing and is reported.
- **Executive contract** (`policy/code_families.mg`):
  - A changed member (`turn_changed_element`) derives `turn_family_obligation` for every other member and each shared requirement.
  - An obligation is settled when that peer changes in the same turn, or when the model records a reasoned `family_no_change`.
  - An open obligation withholds `turn_verified` (`coder_safety.mg`) and adds `turn_missing_evidence(/family_review_open)`.
  - Each open review is named to the model through `injectable_context`: the peer, the family and the requirement.
  - Family facts and decisions are host-witness predicates that a model cannot assert, and they are homed on the world shard with `turn_changed_element`.
- **Tool:** `family_no_change` (CodeDOM; safe, envelope `/codedom`) records `tools.FamilyDecision`. The session drains it into `turn_family_no_change`, and a reason of at least a sentence is required.
- **Prompt:** `capability/code_families` explains the review and that peers may differ.
- **Deferred:**
  - member revisions and the generation vector, with compare-and-swap on completion;
  - per-member verification evidence beyond the existing test gates;
  - candidate-family discovery;
  - multi-batch staging.

  Completion is closed over the declared family only.
