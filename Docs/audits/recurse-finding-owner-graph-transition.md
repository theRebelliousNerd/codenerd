# Recurse finding ownership across graph generations

**Status: source-grounded integration finding and proposed contract.** Reviewed public `theRebelliousNerd/codenerd` main at **`5a1892983a535fd4756c8c40edebdfed1772c0d3`**, observed 2026-11-03 at 23:38 UTC and rechecked unchanged at 23:44 UTC. All CodeNERD links below use that immutable revision. Source and existing test definitions were read; no tests, builds, gate commands, repository programs, model inference, failure injections or application paths were executed. No implementation or repository files were changed. The witness and evaluation are unrun.

## Decision in brief

A recurse pass can correctly rediscover a changed dependency graph while continuing to use workspace findings attributed to the previous graph. A kept edit that splits an import cycle is enough: the end-of-pass measurement still uses the old collapsed node, the next pass installs the new separate nodes, and its nonempty measurement cache prevents a fresh workspace baseline. Candidate collection compares the cached owner string with each current node ID. A remaining finding can consequently be counted as open globally but offered to no node's repair policy. [Pass lifecycle][pass] [Measurement attribution][measure] [Candidate collection][open]

This is **not** a missing-import edge, an identifier collision, a claim that Mangle misorders a valid DAG, or evidence that a failing check passes. Both graph generations can be correct; the missing contract joins a measured finding to the graph generation under which its owner was derived.

The bounded next step is to make graph installation reconcile finding ownership before visits begin. Preserve the original observation and its applicability. Reproject compatible observations against the new graph or obtain a fresh measurement when compatibility is unknown. Every retained finding needs a current owner or an explicit unresolved/out-of-scope disposition. Do not silently suppress it because its former component no longer exists.

## 1. Why this matters to Socratic recurse

The repository deliberately separates creative proposals from executive control: the host observes source and gates, Mangle chooses work, and models solve the admitted task. Its root guidance also requires stale evidence not to survive a source change as current truth. [Root guidance][root]

Mission preparation occurs **after** recurse has selected a finding or improvement. It sends the chosen attempt to the design-only proposer/interrogator thread and copies the selected node's paths into the mission receipt. A workspace finding that never enters the live visit's candidate set cannot be recovered merely by improving the interrogator's questions. The model might incidentally notice the underlying defect during another task, but that is not deterministic conservation of the measured obligation. [Attempt selection][attempt] [Mission preparation][mission]

The defect affects ordinary recurse as well as the optional mission path. CLI and chat both call `RunRecurseCycles`. The proof does not require an error-free-gate configuration, a mission, paid calls, a resumed process, or a model choosing a particular repair. It concerns the adapter state presented to the next pass once the conditional topology-changing keep has occurred. [CLI entry][cli] [Chat entry][chat]

## 2. Source trace

### 2.1 Correctly changing physical components

`DeriveWorkspaceDAG` refreshes membership, reads the admitted workspace, invokes language scanners, collapses cycles and appends synthetic closing nodes. The Python scanner can represent ordinary `a -> b` and `b -> a` package imports. `collapse` sorts each strongly connected component and derives its ID by joining its member paths with `+`; it separately retains the member paths. [Derivation][derive] [Python scanner][python] [SCC projection][collapse]

Thus the initial physical component containing `a` and `b` has ID `a+b`. Removing the `b -> a` edge while retaining `a -> b` correctly yields separate nodes `a` and `b` on the next derivation. No reserved stage name, plus sign in a real path, ambiguous serialization or failed graph validation is needed. The identity change is a truthful consequence of different component membership.

### 2.2 Pass-local graph, cross-pass measurements

At the start of every `pass`, the driver rederives and validates the graph, stores `r.allNodes`, applies any configured focus and publishes the pass. It also redetects gates. The measured workspace state, however, is reused whenever `len(r.state) != 0`: the alternate branch only calls `seedSeenFromState`. That helper copies finding IDs into the seen set; it does not reconstruct their node ownership. [Pass setup][pass] [Seen-set reuse][seen]

At pass close, a workspace remeasurement occurs only when at least one attempt was kept. The call receives the same local `nodes` slice created at the start of that pass. There is no graph rederivation between the retained edit and this closing call. Consequently the closing gate can see the new source bytes while its finding owner is still projected through the old component list. [Pass close][pass-close]

### 2.3 Ownership is materialized into each finding

`measure` executes a workspace gate, parses its output and sets each finding's `Node` using `attributeNodeInRoot(..., nodes)`. Attribution matches a target path to the component whose member paths contain it. For `a/problem.py` and an old component with paths `[a,b]`, the assigned owner is `a+b`, even if the gate ran after the import cycle was removed. The parser's stable finding identity is derived separately from gate, target and sometimes signature; the owner need not be part of that identity. [Measurement][measure] [Path attribution][attribute] [Finding identity][finding-id]

`gateRun` retains the parsed finding list and raw invocation observations, but its type does not bind the `Node` projection to a graph generation. Its optional `snapshot` is populated for mission runs after source bracketing. That supports source applicability, not automatic repair of an owner projection when the next pass installs another graph. [Run state][run-state] [Measurement bracketing][measure]

### 2.4 The new graph cannot consume the old owner

Each visit refreshes its node-scoped gates, then calls `open`. For workspace gates, `open` includes a retained finding only when `f.Node == node.ID`. It does not reattribute by target. Therefore neither current node `a` nor current node `b` receives a finding still carrying `Node: "a+b"`. Synthetic closing nodes do not match that string either. [Visit][visit] [Candidate collection][open]

The policy adapter publishes only the findings supplied by `open`. Mangle's candidate rule in turn requires a `recurse_finding` matching the active visit. The engine has no missing finding fact from which it could derive the repair. This is an observation-to-policy handoff failure, not a reason to give Go a new work-selection algorithm. [Policy adapter][projection] [Candidate rule][selection]

### 2.5 The summary distinction does not catch this state

`openCount` counts IDs in retained state without requiring their owners to exist in the current graph. `reportUnattributed` reports only the explicit `unattributed` sentinel. An orphaned historical owner string is neither a current node nor that sentinel. The pass can therefore retain a positive open count while every current visit lacks this particular finding, without adding it to `RecurseCycleResult.Unattributed`. [Counts and unattributed reporting][report]

This does not establish a false green summary: the count can remain nonzero. The narrower failure is that a measured obligation loses its repair route without an explicit ownership diagnostic.

## 3. Conditional witness, not a reproduction

Use an otherwise ordinary, authorized whole-workspace sweep with at least two passes. The illustrative source layout is:

```text
a/__init__.py     # imports b
a/problem.py      # independent defect F
b/__init__.py     # imports a initially
```

Assumptions are explicit:

1. Both directories and all relevant source are admitted. The supported import forms produce the stated initial cycle. No import-resolution limitation from another audit is exercised.
2. A runnable, unchanged workspace-scoped **`audit`** gate G produces a small, parsed finding F with target `a/problem.py`. F remains genuinely unresolved throughout the transition. Use the stock per-attempt workspace ratchet kinds, `build` and `lint`; G is neither an improvement test gate nor another finding's selected target during the affected second-pass visits. Consequently unrelated per-attempt checks do not refresh G before those visits. No other successful G refresh intervenes. Node-scoped test measurements are not assumed to reproduce F. [Stock ratchet kinds][ratchet-kinds] [After-measurement selection][after]
3. During the first pass, a legitimate attempt is kept that removes the now-unneeded `b -> a` import. Its other obligations, ownership checks and ratchet conditions are satisfied. This keep is a premise, not an unexecuted claim that an arbitrary import deletion would be accepted.
4. F has at most one earlier unsuccessful attempt and is neither refused nor excluded. A visit attempts each finding once, so an unresolved F can legitimately remain for the next pass. This isolates the ownership omission from stall policy.
5. No relevant external edit occurs between the pass-closing observation and the next graph acquisition. Storage, gate execution, kernel operations and graph discovery succeed.

The source-derived transition is:

| Boundary | Graph used for ownership | F's retained owner | Effect |
|---|---|---|---|
| First pass begins | One physical component `a+b` | `a+b` | Normal initial attribution |
| Authorized topology edit is kept | The pass still holds its original `nodes` slice | `a+b` | The graph is not rederived inside the visit |
| First pass closes | Old slice containing `a+b`; gate observes post-edit source | `a+b` | G still reports F, but its owner is projected through the old graph |
| Second pass begins | New graph contains `a` and `b`, not `a+b` | `a+b` | Nonempty `r.state` skips a new workspace baseline |
| Current source nodes are visited, with no intervening G refresh under the conditions above | `b`, then `a` under the remaining dependency | `a+b` | Exact owner comparison excludes F from both candidate lists |
| Reporting | Retained state still contains F | Not the unattributed sentinel | F may remain in open counts without a current repair route |

The scheduling order is correct under the remaining edge. It cannot repair the owner-string mismatch.

The first pass need not evade the current once-per-visit behavior. For example, F can receive one no-change attempt, while another admitted fix or a later measured improvement removes the unnecessary import. Alternatively, F can be measured at pass close after its node's visit has finished. Those are scheduling conditions to arrange in a future fixture, not observed histories.

### Persistence and recovery bounds

The immediate established omission is the next pass's candidate handoff under the explicit no-intervening-G-refresh condition above. It is not necessarily permanent. In a different gate configuration, an earlier kept attempt that actually remeasures G could replace its owner before the affected visit, avoiding that omission. A later pass-closing refresh after a kept change can also restore G's ownership, and a fresh invocation starts with a new in-memory measurement cache. Those are potential escape paths, not a guarantee that F will be visited in the requested run. [After-measurement][after] [Keep publication][keep] [Initial run state][init]

If no later attempt is kept, the pass-close optimization performs no workspace remeasurement. No-change attempts do not produce the changed-tree after-measurement, and a reverted attempt does not install its temporary `after` map into `r.state`. Under those further conditions, the stale owner can survive additional passes. This matters in a bounded invocation: its remaining pass or mission budget can be spent without reconsidering an otherwise eligible known finding. No amount of rederiving the same new graph alone rewrites that cached owner. [Changed-tree measurements][after] [Keep versus revert][keep] [Pass close][pass-close]

The reverse transition also belongs in evaluation: merging two nodes into a new component can orphan findings owned by the old individual IDs. The primary finding does not depend on proving both directions.

## 4. Distinctness and existing safeguards

The bounded novelty screen read the current audit inventory and queue, root/campaign guidance, mission architecture, portable-crucible proposal, current graph/pass/measurement/selection implementation, and relevant neighboring audit bodies. Public repository searches for recurse graph attribution, topology, reattribution and `efDirty` found no dedicated report of this pass-boundary projection mechanism. Search results are supporting discovery, not proof about every historical branch or private discussion. [Audit queue][queue]

- **Node namespace identity** diagnoses collisions and discusses migration when encoding changes. This witness has no collision or encoding migration: the component legitimately changes membership during one live run. An injective new ID scheme still changes identity on a split, so that repair alone cannot reconcile cached finding owners. [Namespace audit][namespace]
- **Gate subsumption** diagnoses suppression based on a shared kind/language. Current `open` preserves independent workspace gates, but still requires exact node equality. The new omission remains after the old suppression is removed. [Subsumption audit][subsumption]
- **Collapsed finding invocation identity** distinguishes the selected finding's actual command from a component aggregate. Here G has one workspace invocation and its command/result can be entirely correct; its cached scheduling owner becomes obsolete. [Invocation audit][invocation]
- **Stall evidence applicability** already proposes explicit applicability handling for node split/merge, membership changes and graph transitions at an admitted pass boundary. That broader design requirement is prior work, not a new proposal here. Its diagnosed mechanism is historical retry suppression after a changed premise. This report identifies a different concrete consumer: the live cached `Finding.Node` projection can omit F before current visit facts reach stall policy, even when F has never stalled. Fixing failure-history applicability alone does not reproject `r.state` or supply the missing `recurse_finding`. [Stall audit, applicability contract][stall-contract] [Stall audit][stall]
- **Portable recurse deliberation/crucible** asks for capability-aware acquisition and honest behavioral evidence. This report supplies a concrete graph-lifecycle conservation condition rather than re-presenting that general proposal as new. [Portable proposal][portable]

Preserve graph validation, SCC semantics, source membership, focused-scope rules, exact gate identity, no-tests handling, independent gate obligations, all existing write restrictions and the outer ratchet. Retaining a finding for consideration must not authorize an edit, establish test coverage, or certify strategic mission satisfaction.

## 5. Required contract and bounded design

### Finding conservation across graph installation

Before a pass makes any node eligible for selection, every retained workspace finding must have one of these explicit dispositions under the installed graph:

- A current owner derived from its admitted target and a compatible source/graph observation.
- An explicitly out-of-scope owner in the full graph, preserving the distinction from unknown attribution.
- An unresolved, unsupported, retired-source or stale observation that cannot be consumed as current candidate evidence and is surfaced for the existing recovery/measurement policy.

An owner string absent from the current node set must not silently mean “no work.” Preserve stable finding identity independently of a node's grouping identity. Preserve the raw historical observation rather than rewriting its history to imply the gate ran under a graph that did not yet exist.

### A small integration repair, not another scheduler

1. Update the campaign/recurse specification to distinguish source observation, graph generation and finding-to-node projection. Define which changes invalidate only the projection and which also require another gate execution.
2. At graph installation, compare the prior attribution domain with the new component/path domain. A deterministic canonical member-path representation can establish whether ownership changed; a pass number alone cannot.
3. Where the retained measurement is demonstrably applicable to the same source state from which the new graph was acquired, reproject its target through the current graph without rerunning the external command. This is especially useful for the primary witness: the old graph label, rather than the fresh pass-closing output, is what is obsolete.
4. If applicability is unknown, acquire fresh compatible evidence through the existing gate path or expose an explicit incomplete state. Do not relabel older results as newly measured. Mission-only snapshot fields are not a universal ordinary-recurse proof.
5. Reconcile before `open` and before the resulting `recurse_finding` batch is installed. Keep policy responsible for priority, retries, execution admission and stop decisions. The host supplies the ownership/applicability observations.
6. Make reporting recognize orphaned ownership during reconciliation so positive open counts cannot silently coexist with zero possible owners for those same records.

The error-free `efOpen` projection has the analogous design consideration: it is rebuilt when `efDirty` is true, which is currently driven by snapshot changes, not graph installation. Its records also materialize `Node` from a supplied `nodes` list and `open` uses the same equality check. This report does not rely on an error-free witness or claim every such run is affected; graph-generation invalidation should nevertheless inspect that cache as part of the same owning contract. A full error-free refresh is not automatically a repair for ordinary `r.state` workspace findings. [Error-free projection][ef]

Do not fix this by keeping removed components as active scheduling nodes, assigning every orphan to `wiring`, copying an old composite's visit completion to all new components, widening model scope, retrying every finding regardless of stalls, or treating an open-count decrease as success. Those changes invent ownership or erase independent policy distinctions.

## 6. Failure modes and costs

Correctness risks of a repair include reattributing a stale diagnostic to a new file at the same path, transferring history between split/merged components, incorrectly bringing excluded source into a focused sweep, resolving a location-free result by guesswork, mutating shared cached slices, and publishing a partly reconciled graph if observation or kernel work fails. Keep historical measurements immutable and publish the new projection coherently. Concurrent external edits should cause incompatibility or remeasurement, not fabricated freshness.

A simple comparison of canonical component membership is linear in total member-path bytes after existing sorting. Reprojecting F findings with the current linear attribution helper across P member paths is roughly O(FP) path comparisons; a suitable prefix index could reduce lookup work but creates its own normalization and invalidation obligations. Additional generation/projection metadata consumes memory, journal/sidecar storage and serialization work. These are engineering estimates, not measured costs.

Conservatively rerunning every workspace gate whenever graph ownership changes adds process startup, filesystem/CPU work and potentially expensive test/audit time. Reprojection can avoid those runs only when source and invocation applicability is established. It must not be sold as free reuse. Restoring omitted work may legitimately add repair attempts, deliberation calls, tokens and wall time; spending fewer resources by omitting a known obligation is not an efficiency win.

Mokhov, Mitchell and Peyton Jones's *Build Systems à la Carte* distinguishes scheduling order from whether work must be rebuilt. That separation is a useful primary-source design lens: a fresh scheduling structure does not itself make derived cached observations reusable. Applying that lesson to this finding-owner cache is this report's engineering inference, not a proof from the paper that CodeNERD is a build system or that a proposed implementation is correct. [Author-hosted paper][paper]

## 7. Fair evaluation plan, entirely unrun

Read-only inspection found useful existing controls: the workspace test derives an import cycle as one node; ordinary cycle tests check retained fixes, rejected regressions and no-op stalls; separate fixtures check measured improvements; the independent-workspace-gates fixture checks candidate preservation for fixed node IDs. Those tests were not run. Their inspected bodies do not compose a topology-changing keep, closing measurement, next-pass graph installation and candidate projection. This is not a repository-wide assertion that no other coverage exists. [SCC test][scc-test] [Cycle controls][cycle-tests] [Measured-improvement fixtures][improvement-tests] [Fixed-owner candidate test][open-test]

Under separate implementation/execution authorization, first use deterministic executors and immutable fixture checks, with no model needed:

1. Derive the initial `a+b` component through real source discovery. Produce a small real parsed workspace finding for `a/problem.py` and verify its initial owner.
2. Arrange a separately valid retained edit that splits the component while F remains unresolved and nonstalled. Observe the actual production pass-close call, not merely a handcrafted `gateRun`.
3. Enter the next pass and assert the new graph contains `a` and `b`, not `a+b`.
4. Capture `open`, the exact `recurse_finding` facts delivered by the adapter, and the policy's next candidate. Require F to have a current admitted owner or an explicit incompatible/unresolved disposition. Do not accept merely seeing F in `openCount`.
5. Let a nonchanging follow-on pass complete and establish whether F's disposition persists correctly without relying on an unrelated successful write.

Controls should cover:

| Fixture | Required observation |
|---|---|
| Graph unchanged | Existing ownership and candidate order remain stable; no needless gate rerun |
| SCC splits | Each finding maps to its target's new component or has an explicit unresolved state |
| SCC merges | Old individual owners do not strand their findings outside the merged node |
| One directory gains a more specific admitted owner | Longest-path ownership changes are reconciled consistently |
| Finding source is removed or moves | No invented current diagnostic; stale/retired identity is visible |
| Gate command/env/pass contract changes | Projection repair is not mistaken for invocation compatibility |
| Unknown or location-free target | Existing unattributed behavior remains; no synthetic owner assignment |
| Focused sweep with outside finding | Known out-of-scope remains distinct from unresolved ownership |
| F already stalled/refused | Repairing ownership does not erase legitimate history or permission barriers |
| Changed attempt is reverted | Temporary post-attempt ownership does not overwrite accepted baseline state |
| Another kept attempt later refreshes G | Confirm the actual recovery boundary and latency rather than claiming permanent loss |
| Cancellation or observation failure at graph install | No partially current candidate batch or successful-completion claim |
| Mission mode | Selected receipt carries the current node scope; behavioral support remains distinct from mission satisfaction |
| Optional error-free projection | Snapshot and graph-generation invalidation are both accounted for, separately from ordinary workspace gates |

Compare baseline, conservative remeasurement and applicability-qualified reprojection using identical repositories, gates, admitted effects, run/pass budgets and deterministic executor responses. Count retained findings with no current owner, eligible findings lost before policy, incorrect ownership, stale evidence admitted, extra gate invocations and time until the finding becomes actionable. Track CPU, wall time, allocations, source scans, snapshot hashing, bytes retained and human reconciliation separately. Include genuine unknowns and failing checks; a treatment that suppresses them must not appear better.

Only after those controls pass should a matched end-to-end study use normal CLI/chat entry paths with world-model and CodeDOM context enabled. Hold task wording, providers, model/effort, JIT inputs, permissions, evidence budgets and checker strength fixed. Report observed repairs, invalid adoptions and resource use independently. No live experiment, benchmark improvement, runtime failure frequency or successful repair is claimed here.

## 8. Acceptance criterion

The gap is closed when a valid graph change cannot silently detach a retained workspace finding from all current visits: the actual production pass transition conserves the finding with a compatible current owner or an explicit unresolved disposition, preserves policy/permission boundaries, and records independently executed evidence. Graph freshness and finding ownership must advance together.

## Pinned references

[root]: https://github.com/theRebelliousNerd/codenerd/blob/5a1892983a535fd4756c8c40edebdfed1772c0d3/agents.md
[queue]: https://github.com/theRebelliousNerd/codenerd/blob/5a1892983a535fd4756c8c40edebdfed1772c0d3/Docs/audits/ASTRA_QUEUE.md
[derive]: https://github.com/theRebelliousNerd/codenerd/blob/5a1892983a535fd4756c8c40edebdfed1772c0d3/internal/campaign/recurse_workspace.go#L64-L102
[python]: https://github.com/theRebelliousNerd/codenerd/blob/5a1892983a535fd4756c8c40edebdfed1772c0d3/internal/campaign/recurse_workspace.go#L456-L549
[collapse]: https://github.com/theRebelliousNerd/codenerd/blob/5a1892983a535fd4756c8c40edebdfed1772c0d3/internal/campaign/recurse_workspace.go#L150-L233
[pass]: https://github.com/theRebelliousNerd/codenerd/blob/5a1892983a535fd4756c8c40edebdfed1772c0d3/internal/campaign/recurse_cycle.go#L515-L611
[seen]: https://github.com/theRebelliousNerd/codenerd/blob/5a1892983a535fd4756c8c40edebdfed1772c0d3/internal/campaign/recurse_cycle.go#L615-L625
[pass-close]: https://github.com/theRebelliousNerd/codenerd/blob/5a1892983a535fd4756c8c40edebdfed1772c0d3/internal/campaign/recurse_cycle.go#L602-L612
[run-state]: https://github.com/theRebelliousNerd/codenerd/blob/5a1892983a535fd4756c8c40edebdfed1772c0d3/internal/campaign/recurse_cycle.go#L206-L216
[measure]: https://github.com/theRebelliousNerd/codenerd/blob/5a1892983a535fd4756c8c40edebdfed1772c0d3/internal/campaign/recurse_cycle.go#L759-L839
[attribute]: https://github.com/theRebelliousNerd/codenerd/blob/5a1892983a535fd4756c8c40edebdfed1772c0d3/internal/campaign/recurse_cycle.go#L870-L914
[finding-id]: https://github.com/theRebelliousNerd/codenerd/blob/5a1892983a535fd4756c8c40edebdfed1772c0d3/internal/gates/findings.go#L116-L141
[open]: https://github.com/theRebelliousNerd/codenerd/blob/5a1892983a535fd4756c8c40edebdfed1772c0d3/internal/campaign/recurse_cycle.go#L1010-L1032
[visit]: https://github.com/theRebelliousNerd/codenerd/blob/5a1892983a535fd4756c8c40edebdfed1772c0d3/internal/campaign/recurse_cycle.go#L1052-L1104
[projection]: https://github.com/theRebelliousNerd/codenerd/blob/5a1892983a535fd4756c8c40edebdfed1772c0d3/internal/campaign/recurse_policy.go#L64-L100
[selection]: https://github.com/theRebelliousNerd/codenerd/blob/5a1892983a535fd4756c8c40edebdfed1772c0d3/internal/core/defaults/policy/recurse.mg#L222-L255
[ratchet-kinds]: https://github.com/theRebelliousNerd/codenerd/blob/5a1892983a535fd4756c8c40edebdfed1772c0d3/internal/core/defaults/policy/recurse.mg#L258-L264
[report]: https://github.com/theRebelliousNerd/codenerd/blob/5a1892983a535fd4756c8c40edebdfed1772c0d3/internal/campaign/recurse_cycle.go#L938-L991
[attempt]: https://github.com/theRebelliousNerd/codenerd/blob/5a1892983a535fd4756c8c40edebdfed1772c0d3/internal/campaign/recurse_cycle.go#L1152-L1180
[mission]: https://github.com/theRebelliousNerd/codenerd/blob/5a1892983a535fd4756c8c40edebdfed1772c0d3/internal/campaign/recurse_mission.go#L174-L325
[cli]: https://github.com/theRebelliousNerd/codenerd/blob/5a1892983a535fd4756c8c40edebdfed1772c0d3/cmd/nerd/cmd_campaign_recurse.go#L253-L263
[chat]: https://github.com/theRebelliousNerd/codenerd/blob/5a1892983a535fd4756c8c40edebdfed1772c0d3/cmd/nerd/chat/campaign_recurse.go#L218-L232
[after]: https://github.com/theRebelliousNerd/codenerd/blob/5a1892983a535fd4756c8c40edebdfed1772c0d3/internal/campaign/recurse_cycle.go#L1323-L1353
[keep]: https://github.com/theRebelliousNerd/codenerd/blob/5a1892983a535fd4756c8c40edebdfed1772c0d3/internal/campaign/recurse_cycle.go#L1477-L1534
[init]: https://github.com/theRebelliousNerd/codenerd/blob/5a1892983a535fd4756c8c40edebdfed1772c0d3/internal/campaign/recurse_cycle.go#L356-L453
[ef]: https://github.com/theRebelliousNerd/codenerd/blob/5a1892983a535fd4756c8c40edebdfed1772c0d3/internal/campaign/recurse_error_free.go#L31-L69
[namespace]: https://github.com/theRebelliousNerd/codenerd/blob/5a1892983a535fd4756c8c40edebdfed1772c0d3/Docs/audits/recurse-node-namespace-identity.md
[subsumption]: https://github.com/theRebelliousNerd/codenerd/blob/5a1892983a535fd4756c8c40edebdfed1772c0d3/Docs/audits/recurse-gate-subsumption.md
[invocation]: https://github.com/theRebelliousNerd/codenerd/blob/5a1892983a535fd4756c8c40edebdfed1772c0d3/Docs/audits/recurse-collapsed-finding-invocation-identity.md
[stall]: https://github.com/theRebelliousNerd/codenerd/blob/5a1892983a535fd4756c8c40edebdfed1772c0d3/Docs/audits/recurse-stall-evidence-applicability.md
[stall-contract]: https://github.com/theRebelliousNerd/codenerd/blob/5a1892983a535fd4756c8c40edebdfed1772c0d3/Docs/audits/recurse-stall-evidence-applicability.md#L99-L116
[portable]: https://github.com/theRebelliousNerd/codenerd/blob/5a1892983a535fd4756c8c40edebdfed1772c0d3/Docs/audits/portable-recurse-deliberation-crucible.md
[scc-test]: https://github.com/theRebelliousNerd/codenerd/blob/5a1892983a535fd4756c8c40edebdfed1772c0d3/internal/campaign/recurse_workspace_test.go#L187-L204
[cycle-tests]: https://github.com/theRebelliousNerd/codenerd/blob/5a1892983a535fd4756c8c40edebdfed1772c0d3/internal/campaign/recurse_cycle_test.go#L120-L200
[improvement-tests]: https://github.com/theRebelliousNerd/codenerd/blob/5a1892983a535fd4756c8c40edebdfed1772c0d3/internal/campaign/recurse_cycle_test.go#L422-L511
[open-test]: https://github.com/theRebelliousNerd/codenerd/blob/5a1892983a535fd4756c8c40edebdfed1772c0d3/internal/campaign/recurse_cycle_test.go#L514-L539
[paper]: https://simon.peytonjones.org/assets/pdfs/build-systems-original.pdf
