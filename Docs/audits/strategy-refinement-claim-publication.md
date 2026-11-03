---
doc-class: deep-dive
subsystem: autopoiesis
implementation-status: proposed-refinement
evidence-status: static-source-and-independent-challenge
last-verified: 2026-10-30
verified-against: 388205138ba6d47c1cff0c5e97c896dfa642a391
scope: generation claim disposition to strategy refinement purchase and publication
---

# Strategy refinement must retain publication ownership

## Decision and evidence boundary

Stop a generation-claim loser before it purchases strategy refinement, and make strategy publication conditional on the authority and strategy version that authorized that refinement. The existing atom-publication fence works: a displaced worker cannot complete the claim or store its candidate atoms through that path. The missing handoff is immediately afterward. `publishGeneration` returns no disposition, so its caller can continue into strategy refinement after `ErrClaimNotOwned`; the eventual strategy update checks only the strategy ID. [Handoff and atom fence][handoff], [strategy update][store-update]

The source-derived consequence is bounded but material: an evolution worker that has already lost publication ownership can spend a remaining refinement allowance and replace a live strategy. With coordinated concurrent refinements, an older strategy snapshot can overwrite a replacement worker's intervening update. This does not establish that the generated text is harmful or that task quality necessarily declines.

Baseline: public main [`388205138ba6d47c1cff0c5e97c896dfa642a391`][baseline], re-resolved on 2026-10-30 at approximately 09:14 UTC. Independent source challenge corroborated the claim-loser continuation under the explicit conditions below. No repository program, test, model inference or benchmark was run; no implementation was changed. Proposed fixtures are not reproduced incidents or passing test results.

**Important qualification:** the default claim lease is 600 seconds, while both automatic cycles and the TUI `/evolve` command use 120-second contexts. This report does not claim an ordinarily canceled default cycle outlives that lease. The concrete witness uses a supported shorter lease, two evolver instances sharing a workspace, and successful calls completed before either context expires. Configuration accepts any positive `claim_lease_seconds`; one second is sufficient for a controlled schedule. [Lease defaults and validation][lease-config], [automatic deadline value][deadline-value], [automatic deadline application][automatic], [manual deadline][manual]

## Relationship to existing work

The [prompt-evolution lifecycle audit][lifecycle] already calls for cross-process generation ownership, evidence consumption, bounded spending and nonblocking recording. Its as-built section describes the implemented claim fence. This report is a scoped continuation of that contract at the later strategy-write boundary, not a proposal to replace the lifecycle or rediscover that claims are needed.

Other inspected work owns different seams:

- [Strategy refinement scope before limit][scope-audit] concerns eligible rows excluded by a global candidate limit before group filtering. Here the matching strategy is explicitly within the returned candidate window.
- [JIT strategy context ownership][context-audit] concerns classification context, usage and cancellation while compiling a prompt. The current provider preserves that context; this report does not undo the repair or claim all adaptation requests are unmetered.
- [JIT generation-aware optimization][cache-audit] already discusses cross-process strategy-cache freshness and additional cache-health/synchronization gaps. This finding is an unauthorized durable write, not delayed visibility of an authorized write. Correcting either boundary does not automatically correct the other.
- [Session learning verb and persona identity][role-audit] concerns conflating the intent verb with the serving role. The witness below requires a genuinely matching group; it does not pretend a session `/fix` group matches a `/coder` strategy.

## Existing protections to preserve

`RunEvolutionCycle` uses a per-evolver cycle mutex and one cycle-wide budget. It checks cancellation and remaining generation/atom allowances before dispatching groups, orders groups by oldest unconsumed evidence, and charges generation and refinement against the same generation-call allowance. Generation performs pin, cohort, screening and claim admission before the model call. Refinement checks cancellation before each purchase and passes the cycle context into its model call. None of these controls should be weakened. [Cycle and ordering][cycle], [group admission][group], [refinement][refine], [generator context][generator]

The ledger transaction is also correct at its own boundary. Claim takeover replaces the owner only for an expired active claim, or an explicitly retryable released/publication-failed claim. `Complete` updates only a matching active owner; otherwise it returns `ErrClaimNotOwned` before consuming evidence. `publishGeneration` recognizes that result, counts discarded atoms, and returns without storing them. [Ledger ownership][ledger], [atom publication][publication]

The mutexes are instance-local, not cross-process locks. Two processes opening the same `.nerd` directory obtain separate evolvers/stores while sharing the on-disk learning databases. The source explicitly identifies generation claims as the cross-process mechanism. No single-instance parallel-cycle race is needed. [Lock contract and construction][construction], [strategy database][strategy-db]

## The lost disposition and durable consumer

After candidate admission, `generateForGroup` calls the void `publishGeneration`. It then checks only whether strategies are enabled, the store exists and generation-call allowance remains. An ownership rejection is recorded inside the callee, but cannot prevent this continuation through a returned success/failure value. [Handoff][handoff]

`updateStrategies` selects at most three globally eligible rows, filters by the current problem type and shard, decrements the shared allowance, and asks `AtomGenerator.RefineStrategy` to revise the selected strategy. The generator receives the selected content and returns a proposed next version. The caller passes only `strategy.ID` and `refined.Content` to storage. [Refinement selection][refine], [proposal generation][generator]

`StrategyStore.RefineStrategy` updates `content`, increments the current database version, updates `last_refined`, and sets `source='evolved'`, with `WHERE id = ?`. Neither the generation owner nor the version read before the model call is a precondition. Its mutex serializes methods of that store instance but does not serialize another process's read-model-write sequence. [Storage][store-update]

This is not an unused database. The factory registers the evolver's strategy provider with JIT. That provider selects matching rows and renders their current content and version as methodology atoms. Actual prompt inclusion still depends on compilation selection and budget; replacing a row does not prove it was immediately served or selected for every task. The write invalidates the updating instance's local strategy cache. Other instances' cache freshness is a separate, already documented concern. [Factory registration][factory-provider], [provider and rendering][consumer], [local invalidation][store-update]

## Concrete conditional schedule

### Preconditions

Use two functioning evolver instances A and B on the same workspace, with the same configuration and stable failure records. Explicitly run their cycles, or enable automatic evolution; automatic spending remains opt-in. Let `claim_lease_seconds` be a positive short interval such as one second, and keep every step below both 120-second cycle deadlines.

Provide at least the configured minimum of eligible, unconsumed failures with valid verdict versions and a valid common serving pin. Use identical group/cohort inputs. Disable the shared-cause atom check for this fixture, or run without a configured Jev attachment: `screenCohort` then uses the stable `count-fallback` identity. Do not assume separately attached Jev instances have equal screening epochs and therefore identical purchase keys. The deterministic cohort and this fallback make the two generation claim keys equal. [Cohort identity][cohort], [fallback identity][fallback], [claim construction][group]

Have a strategy S matching the group's problem type and exact shard string, with sufficient uses and success rate below `0.6`, and ensure S is within the global top-three candidate window. For example, an already-used `ProblemDebugging` strategy with shard `/coder` can qualify; the seeded row alone does not meet the default ten-use threshold. Keep a generation-call allowance available after the initial atom-generation call. The default allowance of three is enough for one generation plus one refinement per instance. [Defaults][defaults], [candidate query][candidate-query]

Matching `/coder` evidence is not dependent on fixing the separate session-recording defect first. The TUI handler accepts the explicit `/spawn /coder ...` spelling, forwards it unchanged, and `recordShardExecution` retains that string when learning is enabled. Task execution separately resolves the persona to its intent. Such records still require actual eligible judged failures and valid provenance before evolution admits them. Ordinary strategy-use attribution can accumulate by emitted strategy ID independently of this group's role spelling. This establishes a producer route, not a claim that every default session meets all witness conditions. [Spawn handler][spawn-handler], [task normalization][normalization], [recording][recording], [strategy attribution][attribution]

### Interleaving

1. A selects the admitted cohort, acquires generation claim K, charges one generation call, and waits for its atom-generation result.
2. After A's configured lease expires, while A remains inside its live cycle deadline, B selects the unchanged unconsumed cohort and takes over K. B begins its own charged generation call.
3. A receives a valid generation result. Its call to `Complete(K, ownerA, ...)` is rejected because B now owns K. A's atom publication is correctly discarded; A does not consume the evidence through that rejected transaction.
4. A nevertheless returns to `generateForGroup`, enters `updateStrategies`, selects S at version v, charges its remaining allowance and begins refinement. The ownership loss has not canceled A's otherwise-live context.
5. For the minimal witness, let A's valid refinement return and its id-only update succeed. The displaced worker has now published a strategy despite the earlier fence rejection.
6. To demonstrate an intervening-update overwrite, hold A's refinement response after it read v. Let B finish generation, successfully complete K, and refine S from v to body B at database version v+1. Then release A's response derived from v. A's id-only update replaces body B with body A and increments the database version to v+2. That new version number does not mean A incorporated B's revision.

The strengthened schedule needs successful, nonempty refinement replies and successful storage; it is not a deterministic claim about arbitrary model output. Both workers remain within their own cycle call budgets. No noncooperative provider, cancellation failure, atom-quarantine bypass or default-lease overrun is needed. The observed authority discrepancy exists at step 5 even if the two proposed strategy bodies happen to be equally useful.

## Proposed ownership and version contract

### Make generation disposition explicit

Return a typed publication disposition from `publishGeneration`, distinguishing accepted publication, lost ownership, storage/publication failure and other errors. A lost owner must not proceed into refinement. Define the storage-failure case explicitly instead of letting a void return accidentally authorize more work; conservatively stop downstream refinement when the parent publication was not accepted.

A Boolean or typed early-return fix is a small, useful containment for the demonstrated continuation. It does not by itself make every later strategy write safe: two valid generation claims for different groups or cohorts can target the same strategy row, and a strategy can change after it was read.

### Bind refinement to the resource it changes

Treat refinement authorization as an explicit receipt naming the admitted evidence/cohort, strategy ID, expected strategy version, refinement procedure and operation owner. A group-generation claim and a strategy row are different resources; do not assume their keys have a one-to-one relationship.

At publication, require both the current refinement owner and the expected strategy version, and consume that publication authority together with the strategy update. Use a transactional compare-and-update or an equivalent atomic mechanism. Report a lost owner or version conflict as discarded/superseded work, not `StrategiesRefined`. A preflight owner check followed by an unconditional update leaves another race window.

The current claim ledger and strategy store use separate storage components. A design must state where the refinement authority resides and how atomicity is achieved; merely checking one database and writing the other is not a transaction. One bounded option is to admit refinement only from a successful parent disposition, then hold the refinement operation/owner and strategy-version precondition in the strategy database so their final check and update share one transaction. Preserve an auditable link to the parent claim/cohort without inventing cross-database atomicity.

Keep model calls outside metadata locks and transactions. A version conflict must not trigger an unbounded retry or receive a fresh per-group allowance: any deliberate reanalysis is separately identified, charged against remaining cycle budget and still subject to cancellation. Preserve the existing Mangle-derived evidence and screening admission; the proposed host-side comparison enforces durable ownership, not a new Go decision about whether evidence merits learning.

### Account for unsuccessful publication honestly

Continue recording already purchased generation/refinement work even when publication is rejected. Add distinguishable stale-owner, version-conflict and storage-failure publication outcomes without counting the same provider call twice. Retain the winner's consumed-evidence record and reject late losers without returning winner-owned evidence to the pool. There is no reason to roll back a valid atom publication merely because a later optional refinement conflicts.

## Costs and rollout

The immediate disposition fix adds a small control-flow contract and no new provider call. A durable refinement-owner/version contract adds schema/API migration, a short claim/admission operation, a conditional update transaction, conflict diagnostics and retention decisions for operation receipts. Compare-and-update failures intentionally discard already-purchased model output; that is a cost to expose, not hide behind a successful version increment.

Transactions may increase contention when many groups target one strategy. Long model calls must not hold the write lock. Preserve bounded candidate queries and the shared spend cap rather than loading every strategy or serializing all evolution globally. Do not claim a latency, token or quality improvement without measurement.

Existing strategy versions can seed the expected-version check; this proposal does not require rewriting historical content or counts. A rolling deployment must address older writers that still issue id-only updates. Until those writers are drained or rejected by a compatible storage/version protocol, new-reader logic alone cannot guarantee safe publication. Coordinate migration and document any temporary single-writer restriction. Retain existing cache invalidation/source-generation behavior, but track cross-process cache repair separately.

## Fair evaluation and checkable exit

The existing `TestEvidenceLedger_ClaimFencing` directly checks live-lease exclusion, expired-lease takeover, stale-owner rejection and completed-claim non-reacquisition. It is a useful control, but it never traverses the subsequent strategy refinement path. Extend coverage through the complete cycle, not by weakening that ledger test. [Existing fixture][fence-test]

Proposed checks, all unrun:

| Fixture | Required observation |
|---|---|
| Two evolvers, same cohort and count-fallback, configured short lease, delayed A generation | B takes over; A stores no atoms and makes no downstream refinement purchase after losing ownership. |
| Default 600-second lease with normal 120-second deadlines | No assertion that ordinary lease expiry occurs; cancellation and no-takeover controls remain effective. |
| Eligible matching strategy versus nonmatching or below-threshold strategy | The witness reaches refinement only for admitted candidates; no false failure due to the separate global-limit or role-identity issues. |
| Two admitted refinement owners read the same strategy version | At most one conflicting expected-version publication succeeds; the other is observable and does not overwrite it. |
| Refinement owner takeover while a model call is pending | Late output cannot publish, even if no competing content has yet been written. |
| Context canceled before refinement purchase or during its call | No new purchase after the check; the context is preserved and publication follows the declared cancellation contract. |
| Failed/empty generation, failed refinement, failed storage and exhausted shared budget | Existing early exits, bounded costs and explicit error dispositions remain. |
| Successful single-owner refinement | Exactly one content/version transition and publication receipt; no duplicate model-call accounting. |
| Successful atom publication followed by refinement conflict | Atom evidence remains correctly consumed; no accidental release or duplicate atom purchase. |
| Restart and mixed-version deployment | Publication authority remains reconstructible; legacy writers cannot silently bypass the deployed contract. |

Use deterministic barriers and controlled replies with two evolvers sharing real temporary SQLite stores. Prefer a controllable clock/lease seam over flaky sleeps; verify the actual production claim-key construction, candidate filtering and caller continuation. Separately inspect the TUI recording and session attribution routes so direct fixture setup is not mistaken for proof of production reachability. These are suggestions for an authorized future implementation pass, not permission to execute tests in this audit.

Measure stale writes admitted/rejected, version conflicts, provider purchases, discarded outputs, duplicate publication, evidence consumption, lock/database latency and receipt retention. Check both cold and warm strategy consumers, while reporting cache freshness as its separate issue. If evaluating task quality later, hold tasks, models, tool access, budgets and acceptance criteria fixed; include failed and discarded refinements and total repair cost. Fewer writes alone is not evidence of better strategies.

**Checkable exit:** a worker rejected by the parent generation fence cannot purchase downstream refinement; every accepted strategy revision has current publication authority and the expected prior version; stale output cannot overwrite a newer revision; budgets, cancellation, valid atom publication and spend accounting retain their existing guarantees.

[baseline]: https://github.com/theRebelliousNerd/codenerd/commit/388205138ba6d47c1cff0c5e97c896dfa642a391
[lifecycle]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/prompt-evolution-lifecycle-and-evidence-accounting.md#as-built-2026-10-27
[scope-audit]: https://github.com/theRebelliousNerd/codenerd/blob/77069960b69aba14a67dbd682d6f65435e8ada50/Docs/audits/strategy-refinement-scope-before-limit.md
[context-audit]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/jit-strategy-context-ownership.md
[cache-audit]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/jit-generation-aware-optimization.md
[role-audit]: https://github.com/theRebelliousNerd/codenerd/blob/07bdcb50873498d22fce498f599073df00ac5750/Docs/audits/session-learning-verb-persona-identity.md
[handoff]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/prompt_evolution/evolver_cycle.go#L265-L298
[publication]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/prompt_evolution/evolver_cycle.go#L277-L319
[cycle]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/prompt_evolution/evolver_cycle.go#L34-L173
[group]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/prompt_evolution/evolver_cycle.go#L175-L243
[refine]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/prompt_evolution/evolver_cycle.go#L373-L403
[ledger]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/prompt_evolution/evidence_ledger.go#L338-L403
[generator]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/prompt_evolution/atom_generator.go#L299-L344
[store-update]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/prompt_evolution/strategy_store.go#L234-L255
[candidate-query]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/prompt_evolution/strategy_store.go#L297-L318
[strategy-db]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/prompt_evolution/strategy_store.go#L20-L57
[defaults]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/prompt_evolution/evolver.go#L86-L124
[construction]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/prompt_evolution/evolver.go#L216-L326
[lease-config]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/config/prompt_evolution.go#L42-L130
[automatic]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/system/factory_learning.go#L374-L407
[deadline-value]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/system/factory_learning.go#L79-L83
[manual]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/commands_evolution.go#L33-L44
[cohort]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/prompt_evolution/evidence_cohort.go#L31-L127
[fallback]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/prompt_evolution/screening_ledger.go#L75-L108
[factory-provider]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/system/factory_learning.go#L112-L173
[consumer]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/prompt_evolution/strategy_atoms.go#L64-L123
[spawn-handler]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/commands_handlers_analysis.go#L538-L563
[normalization]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/session/task_executor.go#L165-L220
[recording]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/delegation.go#L659-L715
[attribution]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/prompt_evolution/evolver_recording.go#L87-L112
[fence-test]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/autopoiesis/prompt_evolution/lifecycle_evidence_test.go#L239-L269
