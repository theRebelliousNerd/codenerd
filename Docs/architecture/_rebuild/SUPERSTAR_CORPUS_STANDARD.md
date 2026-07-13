# Superstar Architecture Corpus Standard

This is the quality contract for every corpus under `Docs/architecture`. It
borrows the teaching depth of Vectryx's deductive corpus while keeping codeNERD's
current state, target state, and implementation evidence rigorously distinct.

## One corpus, one authority

Each top-level `internal/<package>` has one realized corpus. `cli` owns `cmd/nerd`.
Additional source roots belong to the single semantic owner declared in
`corpus.toml`; the ordered portfolio and explicit exclusions live in
`portfolio.toml`. Source roots may not overlap.

The existing 18-file manifest remains canonical. `README.md` is the human front
door; `IMPLEMENTED_SPEC.md` and `02-CURRENT-STATE.md` own realized truth;
`01-VISION.md` owns aspiration; `03-GAP-ANALYSIS.md` bridges them; `TODO.md` alone
owns feature cards. Optional deep dives must be earned by real complexity.

## README contract

Every README uses these exact sections and package-specific content:

1. **In one minute** — feature, user, problem, and visible outcome in plain language.
2. **Its place in codeNERD** — boundary and creative-center/executive split.
3. **A representative journey** — ingress to decision, effect, response, and failure.
4. **What exists today** — concise, evidenced current behavior and honest gaps.
5. **North star** — desired outcome plus explicit non-goals.
6. **Improvement frontier** — safe truth-gap repair and bounded longer-horizon option.
7. **Choose a reading route** — 90-second orientation, 10-minute tour, deep implementation.

An explanation reusable across several unrelated packages fails review.

## Claims and evidence

Allowed claim labels are:

- `VERIFIED CURRENT` — proven live behavior.
- `PARTIAL` — names both the proven slice and absent seam.
- `PROPOSED UPLIFT` — desired behavior, never phrased as current.
- `OPEN QUESTION` — unresolved choice and consequence.
- `ASSUMPTION` — unverified premise with a validation path.
- `REJECTED` — considered alternative and reason.

Current evidence uses `path#symbol`, `path#Predicate/arity`,
`test-path#TestName`, or `artifact:path`, plus a behavioral discriminator such as
a named test, bounded command receipt, generated artifact, or observed call path.
Use `planned:` and `example:` for non-live paths. Dates, line counts, commands in
prose, and file existence alone do not prove behavior.

`verified_on` and `_progress.md` record inspection date, Git commit, and dirty-tree
fingerprint. Source changes make affected claims stale until reconciled.

## Applicability matrix

Every corpus answers every lane. `N-A` requires a package-specific reason and a
cited boundary.

| Lane | Required answer when applicable |
|---|---|
| Mangle | `Decl`, arity/types/modes, bound negation, recursion/stratification, producer and consumer |
| Permission and safety | `permitted(Action, Target, Payload)`, default deny, trust boundary, resource bound |
| Fact flow | perception/`user_intent`, kernel decision, `next_action`, execution, articulation |
| JIT and agents | prompt atom IDs, selection, context budget/truncation, visible behavior |
| Wiring | constructor, registry, boot order, dispatch, teardown, dormant/bypass paths |
| State and concurrency | owner, scope, lifetime, persistence, deduplication, race boundary |
| Recovery | cancellation, retry/idempotency, partial failure, restart/restore, rollback/degradation |
| Observability | signals, correlation, redaction, retention, operator diagnosis |
| Testing | risk-selected unit, integration, race, fuzz, adversarial, and campaign gates |

## Feature uplift contract

`TODO.md` contains the sole authoritative machine block:

```markdown
<!-- NERD_FEATURE
id: core-explain-effect-v1
owner: core
status: proposed
kind: truth-gap
depends_on: []
affects: [session, transparency]
-->
```

Statuses are `proposed`, `accepted`, `in_progress`, `verified`, `deferred`, or
`rejected`. Kinds are `truth-gap`, `leverage`, `north-star`, or `moonshot`.
The human body names value, evidence, observed gap, desired behavior, non-goals,
affected contracts, positive and negative acceptance, and rollback. `accepted`
requires a pinned decision; `verified` requires code, lifecycle wiring, regression
evidence, and a receipt.

Every corpus should expose a safe evidence-backed uplift and one bounded future
option. Prefer moving executive control into typed facts, Mangle policy, JIT
selection, and observable lifecycle gates over adding a parallel subsystem.

## Human quality rubric

Score each dimension 0 absent, 1 vague, 2 useful, or 3 exceptional:

| Dimension | A 3 teaches and proves |
|---|---|
| Human orientation | user, problem, example journey, glossary-free entry |
| North-star alignment | LLM creative center, Mangle executive, feature contribution |
| Evidence integrity | claim status and stable source/test/runtime proof |
| Architecture clarity | boundaries, ownership, diagrams, rationale, alternatives |
| Data and logic contract | types, predicates, declarations, invariants, provenance |
| Lifecycle completeness | ingress, decision, effect, response, cancellation, recovery |
| Deterministic safety | permission, default deny, hostile input, bounds, containment |
| JIT and agent behavior | atoms, selection, budgets, observable effects |
| Ecosystem wiring | consumers, registries, boot order, adapters, dormant paths |
| Operations | configuration, signals, failures, degradation, recovery |
| Verification | risk-tied unit/integration/race/fuzz/adversarial gates |
| Uplift quality | evidence-backed repair through bounded north-star leap |
| Navigation/governance | routes, IDs, links, ownership, deduplicated work |
| Consistency | no contradictory claims, terms, APIs, predicates, or status |

Passing is at least 34/42. Human orientation, evidence integrity, deterministic
safety, ecosystem wiring, verification, and consistency must each score at least
2. Record the signed score with cited examples in `_progress.md`.

## Validation and change safety

Structural checks prove schemas, coverage, canonical files, links, anchors,
evidence syntax, card shape, duplicate authority, and portfolio parity. Semantic
audit proves prose and source reconciliation. Bounded allowlisted commands produce
runtime receipts; prose never becomes executable input.

Before editing, record worktree state and target identities. Re-read before patching.
Classify legacy files as `keep`, `revise`, `harvest`, `compat`, or `remove`; do not
delete before destination review and link/citation checks. A verified product bug
needs reproduction, minimal causal change, focused regression, wiring review,
documentation reconciliation, and a rollback path.
