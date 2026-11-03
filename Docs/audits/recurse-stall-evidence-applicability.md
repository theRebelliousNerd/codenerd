---
doc-class: deep-dive
subsystem: campaign-recurse-policy
implementation-status: proposed
evidence-status: source-only
last-verified: 2026-11-03
verified-against: 3e9dd4acf97bd1fa04a466a2f27d1741a1e502ea
verification-method: static-source-inspection
---

# Recurse stall evidence must expire when its prerequisites change

## Decision

Keep repeated-failure suppression, but qualify it by the state in which those failures were observed. Current recurse can correctly discover and visit a repaired prerequisite, freshly measure a dependent's changed failure, and still exclude that dependent from repair because two old attempts failed before the prerequisite changed.

The immediate mechanism is narrow: `finding_stalled` compares historical attempt signatures and looks only for a newer kept change attributed to the **same node**. It does not consult the current finding's signature, the known dependency edges, or a source/check generation. A stable test identity can therefore carry a stale suppression decision across a materially changed problem.

This is a **source-supported conditional liveness finding**, not an executed reproduction, false-acceptance claim, permission bypass, or assertion that every stalled finding should be retried. The local rule is explicitly documented and implemented consistently. The issue is its limited applicability contract in a dependency-driven, continuously changing workspace, not an accidental deviation from that local specification.

No application, tests, model inference, benchmark, failure injection, implementation, repository write, credential access, or settings change was performed. This review used public source, public primary literature, and a scratch report only.

## 1. Scope and novelty

Public main resolved to [`3e9dd4acf97bd1fa04a466a2f27d1741a1e502ea`][baseline] during this review and was rechecked unchanged at approximately 22:13 UTC on November 3, 2026. Implementation and documentation references below use that commit.

Reviewed material includes [root guidance][root], [campaign guidance][campaign-guidance], the campaign architecture's [responsibility split][architecture] and [candidate-admission contract][admission], the [forever-loop journey][journey], the current driver/policy/parser/graph implementation, production CLI/chat callbacks, and relevant test bodies. Root guidance says that stale evidence must not survive a source change as current truth and that Mangle owns executive decisions. This proposal keeps those responsibilities.

The [audit inventory][queue], repository searches for `finding_stalled`, `recurse_node_kept`, and stall/dependency terms, and the neighboring reports did not identify this precise mechanism. That is a bounded novelty screen, not a claim about every branch or historical discussion.

Relevant prior work remains distinct:

- [Cargo prerequisite acquisition][prior-dag] and the analogous language-graph audits concern **missing edges**. This witness deliberately supplies a simple, correctly discovered edge. Fixing acquisition does not connect that edge to stall applicability.
- [Recovery attempt identity][prior-recovery] concerns a foreign commit being mistaken for a local kept attempt. Here all attempt identities and commits are correct.
- [Journal replay coverage][prior-journal] concerns forgetting required history after corruption or retirement. Here the history is complete; the old failures persist **too broadly**, including in an uninterrupted process.
- [Prior-reason budgeting][prior-reason] concerns model-facing history allocation and already explains that improvement attempts are distinct from finding retries. This report preserves that distinction.
- [Portable recurse deliberation][prior-portable] already proposes versioned evidence and dependency-based validity in a broader behavioral evaluation design. This report supplies a concrete existing selection consumer and counterexample, rather than claiming the general versioning idea as new.

## 2. Existing production contract

### Scheduling already knows the prerequisite

Each pass derives the workspace graph again, checks its soundness, optionally takes the focused dependency closure, publishes the pass, and detects gates. The graph projection includes `subsystem_depends(Node, Dep)`; the readiness rules require dependencies to have been visited. [Pass setup][pass], [graph facts][graph-facts], [readiness][readiness].

Python discovery, for example, creates a node for each admitted directory containing Python and records an edge when an import resolves to another such directory. A simple `import codec` from an `app` directory into a `codec` directory is enough for the witness; none of the previously audited from-import, initializer, module-file, or scan-completion gaps is required. [Python scanner][python-graph].

### Selection carries a fresh observation

A visit reruns the node's own gates, gathers its current open findings, clears the once-per-visit attempted set, and publishes fresh `recurse_finding` facts. Those facts include the current normalized signature. The kernel then supplies `recurse_next`; the driver does not ask the model to override an excluded finding. [Visit][visit], [fact projection][projection], [selection adapter][selection].

Detected Python projects have a node-scoped pytest gate. The parser deliberately constructs test IDs from gate ID and test target, without including the message. A pytest error suffix contributes to `Signature`, so the same test target can have a new signature without a new finding ID. Stable identity is useful and should not be removed to sidestep this issue. [Detected gate][pytest-gate], [finding identity][finding-identity].

### Stall applicability does not consume either input

The policy derives the last two failed attempts for an ID. Two matching historical signatures on the same recorded node, with no newer kept change on that node, derive `finding_stalled(ID)`. The candidate rule excludes every such ID. Neither `subsystem_depends` nor current `recurse_finding(..., Signature)` participates in the stall derivation. [Stall and candidate rules][stall].

A kept result calls `nodeKept` for the attempted node. That stores its latest kept cycle; it does not propagate invalidation to dependent nodes. Failed attempts remain historical facts. [Outcome projection][outcomes], [driver settlement][settlement].

The ordinary attempt failure signature is also not simply the current finding signature: it can contain `no change:`, `open:`, `worse:`, `no_tests:`, or `unresolved:` classifications. A correction must distinguish attempt disposition from current symptom identity; directly comparing those raw strings would introduce another contract error. [Failure signature][failure-signature].

Both the [CLI][cli] and [chat][chat] run this same driver and construct normal attempt campaigns. The finding is not confined to a test-only policy helper.

## 3. Conditional witness: a prerequisite fix reveals a new consumer failure

Use ordinary non-mission recurse in an admitted Python workspace, with runnable tools, enough authorized passes, successful storage, and no unrelated permission or cancellation blocker.

1. The graph contains distinct `codec` and `app` nodes, with `app` importing `codec`. Their IDs and that edge remain stable. The pytest finding is `app/test_consumer.py::test_contract` under the same gate ID throughout.
2. Two settled repair attempts for this finding, in separate visits, are reverted with the same failure signature. The test currently fails because the prerequisite lacks a needed decoding behavior. No kept change to `app` occurs after the first of these attempts. Improvements on `app` either are skipped or fail to produce a kept change.
3. A later authorized attempt on `codec` lands a valid fix. Its ordinary ratchet permits adoption, and it records a kept change on `codec`, not `app`. This prerequisite fix need not repair the consumer completely.
4. The next `app` visit genuinely runs its test after the dependency change. The same test target now reaches a later assertion, exposing a consumer defect. For an illustrative accepted parser shape, the prior summary could end in `ValueError: unsupported decoder`, and the fresh one in `AssertionError: wrong envelope`. These are hypothetical fixture outputs, not observed logs.
5. The parser retains the same finding ID but produces a different current signature. `visit` publishes it.
6. The two historical failed attempts still match one another. The kept `codec` cycle does not satisfy `recurse_node_kept_after("app", ...)`. The current signature and dependency edge do not affect that derivation. The finding remains stalled and absent from `recurse_next`.

The exact source-level consequence is **suppression of the current repair candidate despite the changed prerequisite and current observation**. Whether a newly admitted attempt would succeed is a separate hypothesis for evaluation, not proved here. The changed problem might still be difficult or beyond authority.

This witness does not require a malformed graph, stale measurement, external concurrent edits, a missing journal row, refusal, a changed gate command, or a model defeating guards. The prerequisite's keep is an explicit condition, not a claim that every proposed upstream fix survives acceptance. The default node-scoped Python test route makes it possible to keep prerequisite progress without first repairing every existing consumer test; the witness assumes all other relevant ratchet obligations permit it.

The same selection mechanism can suppress a dependent when the prerequisite changes but its visible test signature stays the same. That broader case is not proof that every upstream edit matters. It motivates bounded validity tracking instead of either permanent local suppression or a retry after every workspace commit.

### A compact policy argument

Let the last two failure facts be for finding F, node N, cycles 10 and 20, with the same attempt signature S. Let the graph contain N → P, and let the loop keep P at cycle 30. Let the current measurement of F carry signature T, where T differs from the earlier symptom.

If there is no `recurse_node_kept(N, C)` with C > 10, all premises of the existing stall rule remain satisfied. Adding P's kept fact, the dependency edge, or T changes none of those premises. The candidate rule still negates `finding_stalled(F)`. These numbers are explanatory labels, not an executed kernel experiment.

## 4. Counterevidence and limits

- The policy correctly prevents blind repetition on an unchanged node. Removing stalls wholesale would lose a useful cost and progress control.
- A same-node kept fix or improvement newer than the earlier failure lifts the stall. The visit always considers its separate improvement route, subject to measurability and test-gate conditions. That route may repair the problem or incidentally unlock the finding. It is an escape path, not a guarantee that the selected repair is reconsidered. [Improvement driver][improve].
- If the prerequisite change makes the test pass completely, no open finding remains and this witness has no lost repair. If the target or gate ID changes, the historical finding may not match. Those are negative controls.
- Some diagnostic IDs include normalized messages, unlike the test-ID example. A changed diagnostic may therefore get a new ID. The report does not generalize the stable-test witness to every parser.
- A refusal is not a failed repair hypothesis. Current policy intentionally leaves refusal lifting to the owner; dependency changes must not silently reauthorize forbidden work.
- Mission admission can stop an invocation for independent reasons. The minimal witness is ordinary recurse; this report does not assert unlimited missions.
- A correct graph is not a complete semantic read set. Imports may omit generated inputs, fixtures, configuration, external services, and toolchains. Dependency-only invalidation must disclose that limit.
- Replay reconstructs failed attempts and same-node kept observations, so a clean restart with complete history does not itself repair the applicability gap. This is separate from the known replay-loss audit. [Resume][resume].

The documentation already states the same-node rule. Treat remediation as a spec-first policy refinement with a measurable liveness/cost tradeoff, not as permission to silently rewrite documented behavior.

## 5. Proposed applicability contract

A failed attempt is durable historical evidence. A stall is a **current derived decision** about whether that evidence remains applicable. Keep these separate.

An admissible contract should establish:

1. **Stable subject, versioned conditions.** Retain the finding ID, gate/invocation identity and node identity. Alongside each attempted failure, record a host-produced applicability generation covering the relevant retained source, admitted prerequisite envelope, gate contract and required configuration/toolchain identity. Do not put secrets or raw secret environment values in policy facts, prose, or logs.
2. **Changes are observations, not success.** A dependency change or materially different current symptom can retire the old suppression basis. It cannot mark the target resolved, erase failed history, grant a write, or bypass current ranking, permission, verification, ratchet or mission checks.
3. **New evidence receives a bounded chance.** Partition repeated-failure evidence by applicable conditions. After a relevant generation changes, the next eligible visit may try the finding under ordinary policy. If two attempts fail identically under unchanged new conditions, the normal stall returns. Preserve once-per-visit exclusion; a fresh observation is not permission for an unbounded inner retry loop.
4. **Current symptom and attempt disposition are typed separately.** Preserve the normalized current symptom, failure class, affected gate IDs and relevant condition generation. A changed `open:` symptom, a newly available tool, and a `worse:` regression are not interchangeable strings. A formatting-only diagnostic change should not automatically replenish paid attempts.
5. **Known dependencies participate.** At minimum, use the admitted graph to invalidate affected dependents after a relevant kept prerequisite change. Include transitive prerequisites. Base retained-change observations on actual admitted changes, not merely the selected attempt's label. Reverted/no-op attempts do not advance a retained-source generation.
6. **Unknown applicability is explicit.** Unknown or changed graph coverage, node split/merge, membership change, toolchain change, or a legacy record with no conditions cannot be silently called equivalent. Policy should choose a disclosed bounded re-evaluation or hold. It must not reset every pass merely because metadata is absent.
7. **Mangle keeps the decision.** The host measures versions, dependencies, current findings and outcomes. Mangle derives whether previous failures apply, whether a candidate is eligible, and whether budgets or explicit holds stop work. No second Go scheduler or model-authored invalidation decision is needed.
8. **Replay preserves decisions.** Persist enough generation and dependency provenance to recover the same eligibility at a settled restart. A valid checkpoint must preserve applicable failure windows and owner refusals. Do not solve this by dropping old journal rows; the existing replay-coverage concern remains independently necessary.

A minimal first correction could admit a carefully classified new current symptom and propagate known kept-dependency invalidations. A stronger version can bind content identities to an admitted prerequisite envelope. Compare them empirically. A global generation that changes on every commit is simple but potentially expensive; a supposedly exact dependency hash over an incomplete graph is precise-looking but unsound.

Graph changes during a pass need an explicit transition. Do not retroactively relabel earlier attempts or replay visits under a new graph as though their conditions had always matched. Safe choices include retiring affected applicability at the next admitted pass boundary or remeasuring a frozen scope before admission. The choice must preserve existing effect ownership and current-pass scheduling.

## 6. Full costs and failure modes

No savings or success-rate improvement is measured.

The correction can **increase** model spend by reconsidering work the current system suppresses. Its benefit must be demonstrated as useful recovered progress, not more attempts or more commits.

Account for:

- Source/configuration identity acquisition and hashing, graph scans, and incomplete-adapter handling.
- Dependency reachability and kernel rederivation. A traversal over one affected graph can be O(V + E); eagerly materializing all transitive pairs can require O(V²) relation storage. These are design bounds, not measurements of the current Mangle runtime.
- Evidence metadata, durable writes, checkpoint/replay time, schema migration and retained history. Existing full-history retention is not made free by adding a small generation field.
- Rerun gates, actual test wall time, subprocess output and storage; invalidating a stall should not duplicate measurements already valid for that admitted visit.
- New campaign setup, retrieval/JIT work, model input/output tokens, provider calls, verification and compensation for failed retries.
- Operator attention for unknown scope or unresolved legacy evidence; diagnosis and recovery after partial publication.
- Foregone useful work when a noisy dependency or unstable environment repeatedly reopens a low-value finding.

A content fingerprint must describe retained applicable state. Advancing it for a temporary attempt that is later reverted defeats stall protection. Conversely, relying only on a selected node's keep label misses external/admitted source changes and cross-node writes. Uncertain commit or publication outcomes need reconciliation, not blind generation increments.

Additional failure cases include unavailable graph acquisition, toolchain drift, test flakiness, toggling diagnostics, deleted/recreated nodes, SCC split/merge, moved tests, changed gate IDs, cycles in an unsupported dependency view, cancellation between invalidation and observation, and restart after only part of the new metadata becomes durable. Unknown conditions must not turn into a successful keep or an automatically lifted refusal.

## 7. Fair, unrun evaluation

Read, not run: the [policy tests][policy-tests] establish different historical failure signatures, matching-last-two stalls, same-node keep resets and refusal behavior. The [production-Cortex parity test][cortex-test] feeds the same facts to the shipped domain Cortex and a single-store kernel. They are valuable controls. The inspected cases do not place a known prerequisite keep and changed current symptom between an old stall and candidate selection. This is not a claim that no uninspected test covers any related behavior.

Start with deterministic fixtures and the real parsers, graph projection, visit adapter and both production kernel configurations. A direct rule fixture alone is not sufficient integration evidence. Then, if separately authorized, evaluate CLI and chat through their ordinary attempt campaigns with world-model and CodeDOM context enabled. Do not spend live model calls merely to synthesize the two old failed facts.

Required cases:

| Condition | Required distinction |
|---|---|
| Two matching failures, same conditions | Still stalls; no indiscriminate extra attempt |
| Same-node kept change | Existing reset preserved |
| Direct prerequisite kept; stable test ID, changed symptom | Old failures do not permanently suppress the fresh repair |
| Transitive prerequisite changed | Dependency closure reaches the dependent |
| Unrelated branch changed | No reset when a complete admitted scope proves irrelevance |
| Prerequisite changed but visible signature unchanged | Exercise generation validity separately from message parsing |
| Changed text with unchanged semantics or flaky output | No unbounded fresh-attempt credit |
| Upstream repair fully resolves test | No unnecessary fix attempt |
| Temporary/reverted/no-op upstream attempt | No retained-source reset |
| Refused finding plus any dependency change | Remains refused without owner action |
| Missing gate, unavailable measurement, malformed or incomplete graph | Explicit unknown/hold; no fabricated equivalent generation |
| SCC split/merge, node rename, configuration/toolchain change | Explicit migration/invalidation, not accidental ID reuse |
| Settled restart, partial publication, legacy journal | Live/replayed decisions agree or admission explicitly holds |
| Cancellation, mission budget exhausted | Existing stop controls retain authority |

Compare current main, symptom-only applicability, dependency-only applicability, and the combined contract on the same repositories, starting snapshots, gate definitions, tools, permissions and scripted executor outcomes. For model-backed follow-on work, hold model/provider, prompt treatment and full cost budget fixed except for the actual applicability change; do not credit a larger budget or different model to this mechanism.

Primary outcomes should include independently verified target repairs after relevant changes, missed eligible repairs, redundant retries on unchanged conditions, time to reconsideration, unchanged-condition stall rate, incorrect refusal lifting, regression/revert rate, and live/replay eligibility agreement. Count failed, inconclusive, unavailable and abandoned attempts. Report total cost to the same accepted endpoint and quality at equal full budgets. Repository/task families, not repeated random seeds alone, should define independence for uncertainty reporting.

A useful correction must recover changed-condition work while retaining the unchanged-condition stall and all adoption safeguards. Increased throughput of attempted fixes is not sufficient evidence.

## 8. Primary literature and design basis

[Mokhov, Mitchell and Peyton Jones, *Build Systems à la Carte* (ICFP 2018)][build-paper] separates task ordering from deciding whether a prior result remains reusable. Its verifying traces retain dependency identities/hashes, and its dynamic-dependency discussion explains why ordering alone does not establish applicability. The transfer here is limited: recurse has a dependency-aware scheduler but its negative reuse decision lacks a corresponding validity boundary. Repair outcomes are nondeterministic judgments, not pure build outputs; the paper does not prove a recurse retry policy or its cost benefit.

[Acar, Blume and Donham, *A Consistent Semantics of Self-Adjusting Computation*][sac-paper] formalizes reuse with change propagation after mutation. It motivates preserving evidence provenance while revalidating applicability. Its correctness theorem applies to that formal semantics, not to this agent system or a partially known import graph.

These references support a design principle, not empirical evidence that this implementation fails at a measured frequency. The source-level witness above establishes the narrower concern independently.

## Recommended disposition

Record a current policy-applicability gap. First amend the campaign/recurse specification to distinguish durable failed history from currently applicable stall evidence. Then evaluate a bounded, host-observed and Mangle-derived invalidation contract. Preserve stable finding identity, owner refusals, once-per-visit exclusion and the existing outer ratchet.

[baseline]: https://github.com/theRebelliousNerd/codenerd/commit/3e9dd4acf97bd1fa04a466a2f27d1741a1e502ea
[root]: https://github.com/theRebelliousNerd/codenerd/blob/3e9dd4acf97bd1fa04a466a2f27d1741a1e502ea/agents.md#L1-L67
[campaign-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/3e9dd4acf97bd1fa04a466a2f27d1741a1e502ea/internal/campaign/agents.md
[architecture]: https://github.com/theRebelliousNerd/codenerd/blob/3e9dd4acf97bd1fa04a466a2f27d1741a1e502ea/Docs/architecture/campaign/README.md#L17-L33
[admission]: https://github.com/theRebelliousNerd/codenerd/blob/3e9dd4acf97bd1fa04a466a2f27d1741a1e502ea/Docs/architecture/campaign/INTERNALS.md#L152-L178
[journey]: https://github.com/theRebelliousNerd/codenerd/blob/3e9dd4acf97bd1fa04a466a2f27d1741a1e502ea/Docs/journeys/10-forever-loop.md#L140-L160
[queue]: https://github.com/theRebelliousNerd/codenerd/blob/3e9dd4acf97bd1fa04a466a2f27d1741a1e502ea/Docs/audits/ASTRA_QUEUE.md
[prior-dag]: https://github.com/theRebelliousNerd/codenerd/blob/3e9dd4acf97bd1fa04a466a2f27d1741a1e502ea/Docs/audits/recurse-cargo-inherited-dependency-closure.md
[prior-recovery]: https://github.com/theRebelliousNerd/codenerd/blob/3e9dd4acf97bd1fa04a466a2f27d1741a1e502ea/Docs/audits/recurse-recovery-attempt-identity.md
[prior-journal]: https://github.com/theRebelliousNerd/codenerd/blob/3e9dd4acf97bd1fa04a466a2f27d1741a1e502ea/Docs/audits/recurse-journal-replay-coverage.md
[prior-reason]: https://github.com/theRebelliousNerd/codenerd/blob/3e9dd4acf97bd1fa04a466a2f27d1741a1e502ea/Docs/audits/recurse-prior-reason-budget.md
[prior-portable]: https://github.com/theRebelliousNerd/codenerd/blob/3e9dd4acf97bd1fa04a466a2f27d1741a1e502ea/Docs/audits/portable-recurse-deliberation-crucible.md
[pass]: https://github.com/theRebelliousNerd/codenerd/blob/3e9dd4acf97bd1fa04a466a2f27d1741a1e502ea/internal/campaign/recurse_cycle.go#L515-L553
[graph-facts]: https://github.com/theRebelliousNerd/codenerd/blob/3e9dd4acf97bd1fa04a466a2f27d1741a1e502ea/internal/campaign/recurse_plan.go#L110-L154
[readiness]: https://github.com/theRebelliousNerd/codenerd/blob/3e9dd4acf97bd1fa04a466a2f27d1741a1e502ea/internal/core/defaults/policy/recurse.mg#L420-L446
[python-graph]: https://github.com/theRebelliousNerd/codenerd/blob/3e9dd4acf97bd1fa04a466a2f27d1741a1e502ea/internal/campaign/recurse_workspace.go#L456-L540
[visit]: https://github.com/theRebelliousNerd/codenerd/blob/3e9dd4acf97bd1fa04a466a2f27d1741a1e502ea/internal/campaign/recurse_cycle.go#L1052-L1107
[projection]: https://github.com/theRebelliousNerd/codenerd/blob/3e9dd4acf97bd1fa04a466a2f27d1741a1e502ea/internal/campaign/recurse_policy.go#L54-L89
[selection]: https://github.com/theRebelliousNerd/codenerd/blob/3e9dd4acf97bd1fa04a466a2f27d1741a1e502ea/internal/campaign/recurse_policy.go#L92-L100
[pytest-gate]: https://github.com/theRebelliousNerd/codenerd/blob/3e9dd4acf97bd1fa04a466a2f27d1741a1e502ea/internal/gates/detect.go#L134-L141
[finding-identity]: https://github.com/theRebelliousNerd/codenerd/blob/3e9dd4acf97bd1fa04a466a2f27d1741a1e502ea/internal/gates/findings.go#L116-L162
[stall]: https://github.com/theRebelliousNerd/codenerd/blob/3e9dd4acf97bd1fa04a466a2f27d1741a1e502ea/internal/core/defaults/policy/recurse.mg#L188-L246
[outcomes]: https://github.com/theRebelliousNerd/codenerd/blob/3e9dd4acf97bd1fa04a466a2f27d1741a1e502ea/internal/campaign/recurse_policy.go#L103-L129
[settlement]: https://github.com/theRebelliousNerd/codenerd/blob/3e9dd4acf97bd1fa04a466a2f27d1741a1e502ea/internal/campaign/recurse_cycle.go#L1529-L1549
[failure-signature]: https://github.com/theRebelliousNerd/codenerd/blob/3e9dd4acf97bd1fa04a466a2f27d1741a1e502ea/internal/campaign/recurse_cycle.go#L1713-L1736
[cli]: https://github.com/theRebelliousNerd/codenerd/blob/3e9dd4acf97bd1fa04a466a2f27d1741a1e502ea/cmd/nerd/cmd_campaign_recurse.go#L226-L263
[chat]: https://github.com/theRebelliousNerd/codenerd/blob/3e9dd4acf97bd1fa04a466a2f27d1741a1e502ea/cmd/nerd/chat/campaign_recurse.go#L218-L250
[improve]: https://github.com/theRebelliousNerd/codenerd/blob/3e9dd4acf97bd1fa04a466a2f27d1741a1e502ea/internal/campaign/recurse_cycle.go#L1181-L1205
[resume]: https://github.com/theRebelliousNerd/codenerd/blob/3e9dd4acf97bd1fa04a466a2f27d1741a1e502ea/internal/campaign/recurse_cycle.go#L1840-L1886
[policy-tests]: https://github.com/theRebelliousNerd/codenerd/blob/3e9dd4acf97bd1fa04a466a2f27d1741a1e502ea/internal/campaign/recurse_policy_test.go#L64-L130
[cortex-test]: https://github.com/theRebelliousNerd/codenerd/blob/3e9dd4acf97bd1fa04a466a2f27d1741a1e502ea/internal/campaign/recurse_policy_cortex_test.go#L13-L56
[build-paper]: https://www.microsoft.com/en-us/research/wp-content/uploads/2018/03/build-systems.pdf
[sac-paper]: https://arxiv.org/abs/1106.0478

