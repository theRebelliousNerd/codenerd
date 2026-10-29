# Expert Critique: Preserve Domain Scope Through Bounded Review

## Decision summary

Preserve expert-domain assignments and report what each expert actually received and reviewed. Do **not** remove the deliberate build-critic context cap or make every missing advisory opinion a failed turn.

The current expert critique path is wired and has meaningful safeguards. Its integration gap is narrower: selection knows which task files belong to an expert, but the critique registration preserves only the expert identity. Every selected expert then receives the same bounded, generic source packet. A specialist's assigned file can be outside that packet without a corresponding domain-coverage receipt.

The opportunity is to make bounded context **domain-directed and accountable**, using existing JIT, source projection, consultation and Mangle machinery. This is also a concrete integration seam for plan-bound expert decks: an advice card's identity is insufficient if its subject scope disappears before the reviewing expert receives evidence.

## Scope, evidence and status

Source snapshot: CodeNERD main `4d984f9c9b719b013a6400f0c88deb48d2e5eb82`, inspected 2026-10-29. Static source inspection only. No tests, benchmarks, inference, builds or implementation changes were performed. Proposed schemas and evaluation cases below are designs, not shipped behavior or measured improvements.

| Capability | Status at this snapshot |
|---|---|
| Expert selection includes canonical domain files | Implemented |
| Selected experts become turn-level critique obligations | Implemented |
| Expert consultation and severity-based repair triage | Implemented |
| Changed-region projection and post-repair checks | Implemented |
| Domain file assignment retained in critique registration | Not present in inspected path |
| Per-expert evidence inclusion/omission and review-status receipt | Proposed |
| Runtime probability of missing a relevant file | Unverified |
| Savings from domain-specific packets | Unmeasured hypothesis |

## Source-grounded producer–consumer chain

### Selection has the domain information

`experts.Need.Files` is documented as the sorted canonical task files covered by the agent's domain. Selection populates it from policy results. This is stronger than merely choosing an expert name: it is already a useful assignment seed. It is not necessarily a complete final change scope, because work can create new paths or discover new domains after selection. [Selection source](https://github.com/theRebelliousNerd/codenerd/blob/4d984f9c9b719b013a6400f0c88deb48d2e5eb82/internal/experts/select.go#L51-L58)

### Critique registration retains identity, not that scope

`recordExpertCritics` loops over `sel.Needs` and, for `need.Critique`, asserts `turn_expert_critic(Turn, Agent)`. It does not retain `need.Files` in this registration. The round is real, but the explicit selected-domain assignment has been reduced to an agent identity. [Registration source](https://github.com/theRebelliousNerd/codenerd/blob/4d984f9c9b719b013a6400f0c88deb48d2e5eb82/internal/session/expert_critique_round.go#L35-L47)

### Every expert receives the same bounded packet

`reviewWithShardExperts` constructs `views`, `removals` and `windows` once from `readWrittenFilesForReview(workspace, result.WrittenPaths)`. It then calls each expert sequentially with those same maps. The expert's name changes, but the initial source selection is not specialized by `Need.Files`. The prompt asks the expert to review defects in its domain. [Packet assembly and dispatch](https://github.com/theRebelliousNerd/codenerd/blob/4d984f9c9b719b013a6400f0c88deb48d2e5eb82/internal/session/expert_critique_round.go#L108-L126)

The reused build-critic selector takes at most six readable non-test source files. It deliberately excludes tests by naming/directory convention and excludes extensions outside its source whitelist. Unreadable or deleted files are skipped. These choices are not inherently wrong for a generic advisory build critic; the issue is inheriting them as implicit domain-review coverage without carrying the domain contract. [Selector and exclusions](https://github.com/theRebelliousNerd/codenerd/blob/4d984f9c9b719b013a6400f0c88deb48d2e5eb82/internal/session/critic.go#L246-L326)

The cap's comment says “newest-written first,” but the selector iterates the supplied write list forward. The normal `addWrittenPath` producer deduplicates existing paths and appends newly encountered paths. On that path, selection favors first-seen rather than newest-written files. This is a source/comment mismatch, not proof every producer has the same ordering. [Write-set producer](https://github.com/theRebelliousNerd/codenerd/blob/4d984f9c9b719b013a6400f0c88deb48d2e5eb82/internal/session/tool_effects.go#L294-L301)

### Views are already more sophisticated than file-head truncation

The existing machinery projects changed spans with CodeDOM-derived structural context. Large files show changed windows rather than only their first bytes. Keep this machinery. The byte threshold is a soft boundary checked between windows, not a hard guarantee that every rendered packet is under 24,000 bytes: a whole window can exceed it, numbered output adds bytes, and removals and request context add further input. [Projection implementation](https://github.com/theRebelliousNerd/codenerd/blob/4d984f9c9b719b013a6400f0c88deb48d2e5eb82/internal/session/critic.go#L394-L471)

## Minimal counterexample and what it does not prove

Consider a turn with seven readable source files in its normal first-seen write order. A selected specialist's relevant domain file is the seventh. Six earlier files consume the shared selector's cap. Every expert's injected packet omits the seventh file, including the expert selected for that domain.

This establishes an initial evidence-routing omission. It does **not** establish that the expert is unable to review the omitted file. The consultation is executed through a subagent path; permitted tools may retrieve additional source. The original request may identify the file, and the expert may discover it independently. Any evaluation must count that rescue rather than assuming the initial packet is the entire observation history. [Consultation execution](https://github.com/theRebelliousNerd/codenerd/blob/4d984f9c9b719b013a6400f0c88deb48d2e5eb82/internal/session/task_executor.go#L472-L518)

The problem is that recovery depends on rediscovery, while this critique interface has no explicit per-expert manifest saying which relevant subjects were supplied, omitted or subsequently retrieved. A successful call or zero parsed findings cannot establish complete assigned-domain coverage.

## Existing safety and correctness rescues

The critique round is explicitly advisory. A failed consultation is logged and does not fail the turn. Findings are filtered, asserted and triaged by policy before becoming a repair request. The writer is told to reject mistaken findings rather than blindly change code. A repair that breaks build/tests is subject to `recheckUplift`, and later verification rounds still measure the resulting code. This audit does not claim the omission bypasses those independent gates. [Expert critique implementation](https://github.com/theRebelliousNerd/codenerd/blob/4d984f9c9b719b013a6400f0c88deb48d2e5eb82/internal/session/expert_critique_round.go)

Finding-location filtering is also deliberately permissive for unrecognized paths: `findingsOnChange` retains them because they may be alternate spellings of real reviewed files. That tolerance can preserve useful tool-discovered findings, but it is not proof of source coverage or canonical subject identity. A new receipt must not mistakenly infer that every accepted finding came from the original injected view. [Finding filter](https://github.com/theRebelliousNerd/codenerd/blob/4d984f9c9b719b013a6400f0c88deb48d2e5eb82/internal/session/critic.go#L480-L509)

## Proposed contract: domain-scoped review receipts

The unit of accountability should be a review assignment, not “expert X ran.” Suggested fields below describe a proposed contract; they are not existing JSON or Mangle declarations.

```json
{
  "assignment_id": "turn-42/expert-storage/revision-7",
  "turn_id": "turn-42",
  "expert_id": "storage",
  "subject_revision": "revision-7",
  "policy_revision": "policy-3",
  "authority": "advisory",
  "subjects": [
    {
      "path": "internal/storage/transaction.go",
      "role": "domain_change",
      "content_digest": "example-content-digest",
      "delivery": "included",
      "windows": [[81, 137]]
    },
    {
      "path": "internal/api/transaction.go",
      "role": "shared_contract",
      "content_digest": "example-other-digest",
      "delivery": "omitted_budget",
      "retrieval_handle": "example-scoped-handle"
    }
  ],
  "review_status": "partial",
  "findings": [],
  "coverage_receipts": []
}
```

### Assignment formation

Start from selected domain files, then reconcile with the actual final write set and known affected boundary contracts. Do not pretend the pre-execution selection is complete. Retain reasons for inclusion: directly changed domain source, shared interface, dependency invariant, test evidence, migration or configuration contract.

Avoid giving every expert every transitive dependency. Prefer the smallest evidence slice that can support the assigned review question, and explicit handles for omitted relevant subjects. When ownership is ambiguous, use a shared-boundary assignment rather than hiding the ambiguity or duplicating the entire repository context.

### Delivery and observation

Reuse changed-region projection and the existing JIT/context budget. Emit a small manifest containing included, omitted-budget, unreadable, deleted and unsupported-surface subjects. A deleted file needs before-state/change evidence rather than a failed attempt to read a nonexistent current file.

Tool retrieval should add source-bound observation receipts, including revision and windows. An expert's claim that it read a file is not sufficient evidence by itself. Conversely, delivery of bytes is not proof of reasoning quality: receipts certify availability and provenance, not comprehension.

### Outcome

Keep result status independent of findings. Valid states should distinguish a completed scoped review with no findings, partial review, failed consultation, cancelled review and unparseable response. A partial advisory review may remain a legitimate advisory result. It should not silently become a completed mandate.

This distinction matters because the current parser returns `nil` for both explicit `NO FINDINGS` and wholly unparseable output, although its comment claims a distinction. A `NO FINDINGS` line also suppresses other finding-shaped lines. A structured status envelope would expose contradiction or parse failure without treating every advisory anomaly as a hard turn failure. [Current parser](https://github.com/theRebelliousNerd/codenerd/blob/4d984f9c9b719b013a6400f0c88deb48d2e5eb82/internal/session/critic.go#L187-L244)

### Mangle's role

Let policy derive missing assignment coverage, retrieval priority, escalation eligibility and whether a requirement remains owed. Keep deterministic bookkeeping out of extra model calls. Facts should distinguish selection, delivered evidence, observed evidence, result status and authorized obligation. Scope joins must include assignment/turn and subject revision so a previous review cannot satisfy a changed artifact's obligation.

The model contributes domain reasoning; it does not promote its own advisory findings into mandatory authority. A steering atom can remain useful partial guidance. A mandate must originate from authorized policy and define the evidence that discharges it. Missing knowledge can trigger scoped retrieval before a costly stronger-model consultation, but cached expert knowledge is not automatically a substitute for a required review of the current change.

## Options

| Option | Benefit | Cost / failure mode |
|---|---|---|
| Keep packet, add explicit coverage manifest | Smallest contract repair; reveals omitted domain subjects | Does not itself improve source allocation; manifests consume context |
| Domain-aware packets using existing projections | Reduces irrelevant repeated source; targets assigned questions | Misclassification can hide cross-domain interactions |
| Shared boundary packet plus domain-specific slices | Preserves interface context while specializing internals | Requires explicit boundary selection and versioning |
| On-demand scoped retrieval from manifests | Pays for detail only when needed | More tool turns, startup latency, permission failures |
| Bounded parallel expert dispatch | May reduce critical-path latency | More simultaneous model load, cancellation complexity and potentially duplicate findings; separate change from coverage repair |

Recommended order: make coverage observable first, then specialize packets, then evaluate scheduling independently. This prevents a claimed performance win from merely doing less review.

## Full cost model

With E experts and shared initial source packet V, repeated initial source input is approximately E × V before provider caching and tokenization effects. Current expert dispatch is sequential, so consultation latency contributes approximately the sum of individual consultation latencies, plus assembly, policy and repair work. These are structural cost terms, not measured numbers.

Domain packets replace E × V with a sum of expert-specific slices plus shared boundary context, manifests and retrieval responses. Savings depend on overlap. When every expert genuinely needs the same boundary code, duplication may be justified and cache-friendly. When routing is wrong, narrower packets may increase rediscovery calls or miss defects.

Account for source loading/projection, assignment derivation, metadata storage, digest computation, cache invalidation, consultation startup, model input/output, retrieval calls, parsing/triage, duplicate finding reconciliation, repair rounds and re-verification. Track peak memory and concurrent load if parallel dispatch is considered. The current file count and byte threshold alone do not bound total prompt or end-to-end cost.

## Evaluation design — proposed, not executed

Compare current shared packets, manifest-only packets and domain-aware packets under equal model choices, task corpus, permissions and total allowed resource budgets. Preserve tool rescue in every arm. Report both quality and cost; do not advertise lower initial prompt size as an end-to-end win.

Include these cases:

1. Six versus seven-plus changed files, with the expert-relevant file early and late in write order.
2. A late rewrite to an already-seen path; ensure ordering and subject revision are understood separately.
3. Multiple domains sharing an API invariant, where independently correct local reviews miss the interaction.
4. Large changed windows and removed code, measuring actual rendered payload rather than nominal byte constants.
5. Deleted files, config-only changes and tests that carry a relevant domain contract; preserve intentional source filtering unless the assignment explicitly requires these surfaces.
6. Omitted source successfully retrieved, denied, unreadable or stale by the time of retrieval.
7. No findings, malformed response and contradictory response; verify they do not collapse into the same coverage claim.
8. Advisory expert failure versus an explicitly authorized mandatory review requirement.
9. A repair changing the reviewed artifact; the old receipt must not certify new bytes.

Measure assigned-subject coverage, domain defects detected/escaped, false-positive repair churn, source retrieval success, model tokens, cache effects, startup and wall time, total tool calls, repeated verification work and evidence storage. Include a human/source-grounded adjudication of coverage and defect validity rather than using another model's confidence as the sole oracle.

## Relationship to prior work

This is not a repeat of initial consultation continuity: `planned-step-expert-consultation-continuity` concerns advice lost when step prompts replace the initial prompt. This audit concerns the later review round and the loss of domain file assignment before evidence selection.

It is not the UI feedback-identity issue in `review-feedback-identity-and-current-verdicts`, nor the delegated judge packet/acceptance contract in `verification-evidence-and-judge-contracts`. `verification-applicability-and-progressive-evidence` supplies an analogous principle—source applicability and omission must be explicit—but this document locates a separate producer–consumer seam in expert critique. `consultation-coordination-and-safe-reuse` addresses consultation lifecycle/reuse; it does not substitute for domain-specific evidence coverage.

The plan-bound expert deck proposal can consume these receipts instead of inventing a parallel expert execution path. A step or stage may bind steering and mandates to an assignment, while current consultation, JIT and Mangle systems perform the delivery and accounting.

## Recommended next decision

Treat expert identity, domain scope and reviewed evidence as separate objects. Preserve the existing bounded context strategy and advisory safety posture. Add enough provenance to answer: **which expert reviewed which revision of which assigned subjects, what was omitted, and what remains unknown?** Only then optimize source placement and scheduling against measured end-to-end cost.
