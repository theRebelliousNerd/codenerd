---
doc-class: deep-dive
subsystem: campaign-recurse-recovery
implementation-status: proposed
evidence-status: source-only
last-verified: 2026-11-04
verified-against: 25891ce9a81241875506104845cb40058f16a5e7
verification-method: static-source-inspection
supersedes: []
---

# Recurse visit completion must be revalidated after a stopped workspace changes

## Finding and decision

A fresh recurse process can correctly rediscover the current graph, run a fresh workspace check, and attribute a newly failing behavior to the right node, yet never visit that node during its requested pass. Recovery treats the previous interrupted pass's `visit_done` node strings as current completion evidence. It does not bind them to a source revision or check contract. A clean owner commit that changes an already visited node is enough to make that skip stale. [Replay][replay] [Pass admission][pass] [Readiness rules][rules]

The bounded consequence is **omitted work in the resumed pass**, including a `--waves 1` invocation that consumes its only pass completing the historical suffix. This does not mean a failed check becomes a pass, a bad patch is committed, or a remaining finding disappears from the ledger. The finding can remain visibly open. The defect is that fresh evidence does not reopen the node that owns it. [Pass budget][budget] [Candidate forwarding][open] [Visit entry][visit]

The proposed correction is to qualify reuse of unfinished-pass visits before asserting them into the current scheduling facts. If continuity cannot be established, preserve the history but start a fresh scheduling generation or explicitly invalidate the affected completed visits. A conservative fresh pass is preferable to silently importing old completion into a changed workspace. Keep Mangle responsible for eligibility, ordering and pass completion; the host should supply identity and validity observations.

**Evidence boundary.** Public `theRebelliousNerd/codenerd` main was resolved to [`25891ce9a81241875506104845cb40058f16a5e7`][pin] on November 4, 2026, and rechecked unchanged at 00:14 UTC. Source, guidance, existing audits and test definitions were inspected. No repository implementation was changed. No tests, builds, gate commands, repository programs, model inference, failure injections or benchmarks were run. Every witness and evaluation below is conditional and unrun.

## Why this is a separate boundary

The root contract requires revision-aware context and says stale evidence must not survive a source change as current truth. Campaign guidance distinguishes host-observed verification from model prose, and architecture guidance requires implementation, intention and behavioral evidence to remain distinct. A journaled visit is itself an executive observation: using it removes a node from eligibility and contributes to completion. Its historical truth does not alone establish present applicability. [Root][root] [Campaign][campaign-guidance] [Architecture][architecture-guidance]

Several nearby reports already own important parts of this area:

- **Finding ownership across graph generations** concerns a long-lived in-memory measurement whose owner names an old collapsed component. This witness uses a fresh process, an unchanged graph, unchanged node IDs and a fresh correctly attributed finding. Reprojecting finding owners therefore does not repair it. [Graph-transition audit][prior-graph]
- **Recovery attempt identity** concerns recognizing an uncertain commit by a reused cycle number. This witness has no in-flight attempt, uncertain commit, cycle collision or missing acknowledgement. That report expressly distinguishes `visit_done` from a recovered ratchet and does not establish this skip. [Attempt audit][prior-recovery]
- **Journal replay coverage** concerns missing retired history or malformed tails. Here the journal is complete, small and valid. Replaying its bytes perfectly reproduces the stale visit facts rather than curing them. [Journal audit][prior-journal]
- **Stall evidence applicability** concerns two failed attempts suppressing a finding after prerequisites change. Here there need be no prior attempt for the new finding, no refusal and no stall. Suppression occurs one level earlier, before a visit supplies candidates. [Stall audit][prior-stall]
- **Node namespace identity** warns that changing identifier encodings needs migration of visit history. The witness holds the entire graph and identifiers fixed; only source behavior changes. [Identity audit][prior-namespace]
- **Campaign generation admission** and the portable recurse proposal already motivate qualified recovery and source/contract identity. This report does not claim that design principle as new. It identifies the concrete ordinary recurse completed-visit consumer that does not enforce it. [Campaign recovery][prior-campaign] [Portable proposal][portable]

The current audit inventory, all `recurse-*` report titles and relevant bodies, the existing queue, and targeted searches for `visit_done`, resume, visited nodes and source snapshots were checked. No existing report with this exact complete-journal, stable-graph, changed-source skip was found. This is a bounded novelty screen, not a claim about every historical branch or private discussion. [Queue][queue]

## The production trace

### A clean checkout can contain different committed behavior

Both CLI and chat enter `RunRecurseCycles`; the CLI passes `--waves` through to the cycle config. Restarting is explicitly described as resuming the ledger. This is a live path, not merely the planner helper. [CLI][cli] [CLI contract][cli-contract] [Chat][chat]

Startup constructs a new `recurseRun` with an empty measurement-state map, takes the workspace lock, reads history, settles any in-flight attempt, prepares the checkout, rereads the journal and calls `resume`. With no in-flight file, settlement returns immediately. [Startup][startup] [No-flight branch][no-flight]

`recurseGit.prepare` rejects tracked uncommitted changes, then checks the branch name and returns if it is already on the configured branch. It never compares the current commit to a commit recorded for the interrupted pass. The witness makes a normal owner commit on that same branch while no recurse process is running, so startup need not encounter any dirty state or branch switch. [Preparation][prepare]

Git's own contract for status is comparison of worktree/index/current `HEAD`, not comparison with a prior application checkpoint. A clean result after a commit is therefore not evidence that the source is unchanged since the prior visit. This distinction is standard Git semantics, not a Git defect. [Git status documentation][git-status]

### Recovery promotes historical node strings directly

A `visit_done` row is written only after the visit's finding loop and improvement step return. Its recorded fields here are step, pass, node and open count. The journal schema also has outcome/commit fields for other row kinds, but no visit-source identity, gate-contract fingerprint or scheduling generation. [Visit completion][visit] [Journal schema][journal]

`resume` keeps `done[h.Node] = true` for each `visit_done` in the unfinished pass. It resets that map for another pass or an ended pass, not for source changes. The method receives history alone and does not read the workspace or compare a current revision. If the last pass ended, it correctly starts the next pass with no done set. [Replay][replay]

The new pass path derives and validates the graph, optionally focuses it, asserts the current pass, then asserts every recovered node as `recurse_node_visited(Pass, ID)`. Gate detection and the initial fresh workspace measurement happen afterward. The graph soundness probe uses a separate pass number and retracts its own probe visits; it does not validate the recovered visits against current source. [Pass setup][pass] [Visit fact][visit-fact] [Soundness probe][probe]

The Mangle rules consequently derive the node as visited now. Readiness expressly requires that a node is not visited; pass completion counts visited IDs in the current graph. These rules faithfully implement their supplied facts. The missing evidence is the applicability of the recovered visit, not the topological algorithm. [Rules][rules]

### Fresh findings do not revoke completion

Because the new process starts with empty `state`, it runs workspace gates before scheduling. Measurement parses findings and attributes them to the current nodes. The witness's owner is correct and current. [Initialization][startup] [Measurement][measurement] [Attribution][attribution]

That observation does not retract any `recurse_node_visited` fact. The driver asks for the next unvisited node. Only entering `visit` calls `open` for that node and projects its findings into the visit's `recurse_finding` facts. Findings for another node are not borrowed by an unrelated visit. [Visit dispatch][close] [Candidate collection][open] [Fact projection][finding-facts]

When the remaining nodes are done, the historical visits and new visits jointly satisfy pass completion. The driver appends `pass_end`, increments this invocation's finished-pass counter and checks the requested budget. At one wave it stops without beginning a fresh pass. [Closing path][close] [Invocation budget][budget] [Completion rules][rules]

## A conditional witness with an independent behavioral check

Use a small owner-controlled Python repository with ordinary admitted directories `a` and `z`, both holding Python files. Their imports, directories and node identities remain unchanged throughout. No Python project marker is needed for graph discovery; its source scanner creates directory nodes from admitted Python files. Declare one runnable workspace-scoped check in `nerd.md`, with a fixed test population and a direct behavior assertion for a function in `a`. The declaration and its test source remain unchanged. [Python graph][python-graph] [Declared checks][declared-gates]

A particularly bounded fixture declares that check as an `audit` gate and declares no test gate. It runs a fixed external checker or an ordinary test runner; gate kind here affects scheduling classification, not whether the checker can make a real assertion. With no applicable test gate, the existing improvement guard skips discretionary improvements. This removes incidental model edits from the minimal scheduling witness. The defect also applies to test-kind findings; this fixture deliberately isolates it from unrelated improvement attempts. [Improvement admission][improvement-admission]

The checker contract should be concrete: it reads/calls the same function and requires a specific return value. At revision S0, `a.value()` returns 0 and the assertion expects 0. At S1, an owner commit changes only the production implementation to return 1. On S1 the unchanged checker exits nonzero and emits a small supported located failure such as `FAILED a/test_value.py::test_value - AssertionError`. If that filename is the actual admitted test, the parser produces a stable located test identity and the workspace attribution selects `a`. The example is an input contract for a future fixture, not a transcript from a run. [Failure parser][finding-parser] [Extraction][finding-extract] [Attribution][attribution]

If the test file exists inside `a`, it is source for the same directory node, so it does not add a third node. Avoid `pytest.ini`, `pyproject.toml` and other detector markers in this isolated fixture, or explicitly account for all detected gates. Use the **full sweep**, not focused audit skipping. Stipulate the checker and its dependencies are installed, source is readable, membership admits the files, output is complete and the check has no external mutation.

The sequence is:

1. **S0 visit completes.** A normal recurse pass p visits `a` while the assertion passes. It writes `visit_done(p, a)`. No refused or stalled finding is needed.
2. **The pass is interrupted between nodes.** Stop after the completed-visit row and before the pass ends, with at least `z` still unvisited. The journal is valid and there is no in-flight attempt. A process interruption at this boundary suffices; a controlled future fixture should synchronize on the observed completion boundary rather than race arbitrary sleeps. [Completion write][visit] [Boundary][boundary]
3. **The owner changes behavior.** With recurse stopped, commit the return-value change on the configured recurse branch. Leave the graph, gate declaration, test source, permissions and journal unchanged. The checkout is clean.
4. **Restart for one wave.** Use a fresh process/kernel, the same workspace and scope, ordinary non-mission mode, and one pass. Startup admits the clean checkout and restores `done[a]`.
5. **The fresh check sees the defect.** Its failure is measured at S1, owned by `a`, and counted open. No cached S0 gate result is involved.
6. **Scheduling skips `a`.** The recovered completion excludes `a` from `recurse_next_node`. `z` and the closing stages cannot offer a finding owned by `a` to their own visit policies. With the stated check set, no unrelated improvement repairs it.
7. **The invocation finishes its one pass.** Its remaining visits complete p; it records a nonzero open count, increments `Passes` and stops. The broken behavior remains. No attempt for the fresh `a` finding was dispatched during this invocation. [Replay][replay] [Readiness][rules] [Fresh baseline][pass] [Budget][budget]

This proves a conditional omission in source logic, not observed latency or production harm. Its practical significance is that a user can commit work while the assistant is stopped, request a bounded sweep, and receive a completed pass that did not revisit the changed node.

A future stronger behavioral fixture should place the oracle outside the executor's write scope and use a deterministic executor capable of repairing only `a`. The principal assertion is that S1 must be revisited or explicitly held for revalidation, not that a stochastic model always finds the repair.

## What is and is not established

- **The failed check remains failed.** The pass-start/end open count can expose it. The report does not call the result all-green or claim that the UI says every behavior passed. The summary reports passes and attempts; status separately reports the open-count trend. [Summary][summary] [Status][status]
- **A next full pass can recover.** If the run is unbounded, has enough remaining waves, or is invoked again after the stale pass ends, the new pass has no inherited done set and can visit `a`. This is a bounded-run coverage failure, not proven permanent starvation. [Pass loop][budget] [Ended-pass recovery][replay]
- **Unchanged-context resume is valuable.** The present skip avoids repeating work when context truly remains compatible. A repair should retain that optimization where it is justified.
- **A dirty uncommitted tracked change is rejected.** The witness deliberately uses a clean committed change. Startup's existing refusal is real and should remain. [Preparation][prepare]
- **This does not require graph churn, wrong attribution, truncated output or missing logs.** Those separate defects should not be used to explain away this stable-identity case.
- **Deliberation cannot recover an attempt that was not selected.** Mission preparation takes an already selected attempt and its node scope. Ordinary mode suffices for the witness; no false mission satisfaction is claimed. The relevance to Socratic recurse is candidate conservation upstream of proposing and questioning, not a mandate to make the model override the scheduler. [Attempt-to-mission boundary][mission] [Mission preparation][mission-prepare]

## Proposed contract and implementation choices

The contract should distinguish historical completion from reusable completion:

> An unfinished pass may reuse a completed visit only when recovery establishes that its continuation context is compatible with the context under which the visit was completed. Otherwise the visit remains historical evidence and cannot discharge current scheduling coverage.

Specify this in campaign/recurse architecture before implementation. This is a proposed contract, not a description of an existing predicate.

### Minimum admissible evidence

Record or derive a durable continuation checkpoint for the pass containing:

- Workspace and ledger incarnation, current scheduling generation, graph identity and selected scope.
- The settled source frontier expected at restart, including committed and any relevant admitted untracked content.
- The effective gate/verification contract and its interpretation version, with relevant toolchain/environment identity where the capability claims depend on them.
- Completed visits and the finalized attempt/commit lineage that advanced the pass to that frontier.
- Whether recovery coverage is complete and all uncertain effects have been reconciled.

Do not place credentials or raw secret environment values in the journal. Use safe descriptors/digests with an explicit scope. Missing evidence, failed reads, unsupported schema or unstable observation should yield unknown continuity, not equality.

A single branch name is insufficient: it can point at another commit. A single `HEAD` equality check is useful containment for the committed-change witness but not a complete workspace contract. Recurse admits preexisting untracked files, and relevant configuration or tool state may change outside tracked source. The existing snapshot helper illustrates a broader local content boundary, including admitted untracked content and selected configuration, but its capability and cost must be reviewed before reusing it for ordinary recovery. [Preparation][prepare] [Existing snapshot][snapshot]

### Conservative restart first

For an interrupted pass with unknown or changed continuation context, preserve historical records and start a fresh scheduling generation over the requested current graph. Do not reset kept commits, delete audit history or replay side effects. Preserve separate refusal and stall semantics according to their own contracts.

This is a small containment option. It may repeat checks or revisit improvements but need not repeat a previously kept change: fresh measurements determine current findings, and the executor remains behind normal admission. Use an explicit transition record so a future replay does not merge the old `visit_done` rows back into the restarted pass.

The new generation should have a distinct identity even if the user-visible pass label remains stable. Otherwise the current parser's same-pass `pass_start` behavior preserves the old done map. Likewise, an in-process restart must retire or supersede the old generation's relevant visit facts. Merely clearing a local Go map while reusing the same `(pass, node)` facts is not sufficient. [Replay reset semantics][replay] [Visit projection][visit-fact] [Current rules][rules]

Define budget accounting explicitly: rejected historical coverage must not consume a requested current sweep. Preserve recorded historical progress while distinguishing a completed resumed suffix from a complete sweep under a newly admitted context.

### Qualified selective reuse later

A more efficient design can preserve visits whose dependencies, source/check domain and permissions remain applicable, invalidate the changed nodes and dependent obligations, and publish those validity facts to Mangle. This is harder than comparing each visit's entire-workspace snapshot with current source.

A normal pass intentionally changes the workspace after earlier visits. Comparing all visit-time snapshots with the last snapshot would invalidate legitimate earlier visits after every later kept edit. The design therefore needs an acknowledged pass frontier and expected internal effect lineage, or a proven per-visit dependency domain, not an unconditional whole-tree equality at every node.

A dependency-aware optimization is only as sound as graph acquisition and gate scope. The existing adapter limitations prevent assuming perfect semantic dependency closure. Where dependency coverage is incomplete, fall back to conservative invalidation or an explicit unsupported reuse disposition. Do not broaden model write authority to compensate for missing reuse evidence.

The useful analogy is incremental builds: scheduling order and the decision to reuse/rebuild are separate concerns. Mokhov, Mitchell and Peyton Jones distinguish those choices in *Build Systems à la Carte*. This motivates evaluating visit reuse independently of topological correctness; their model is not a proof of CodeNERD's recovery behavior or cost. [Primary paper][build-paper]

## Costs and failure modes

**Conservative restart cost.** Repeated graph detection, workspace gates, node gates and possibly improvement attempts add time and may add model spend. Measure these separately. A smaller attempt count obtained by skipping the newly failing node is not an efficiency benefit.

**Qualified reuse cost.** Content hashing reads source bytes; graph/configuration fingerprints add serialization and comparison work; durable frontier updates add writes and recovery states. Incremental identities may reduce repeated I/O but require invalidation correctness. No numeric cost or savings is asserted.

**Checkpoint publication.** A crash between effects, visit completion and continuation checkpoint publication must not manufacture reusable coverage. Preserve uncertain state and reconcile effects independently before deciding what can be skipped.

**Expected edits versus outside edits.** Source changes from previously acknowledged recurse commits must not be mislabeled as external merely because the pass-start revision differs. Conversely, an owner commit, branch movement or copied ledger must not inherit coverage because the path and node names match.

**Contract changes.** Changed tests, command arguments, environment, adapter version, policy version, membership or focus can invalidate reuse even with unchanged production bytes. Preserve exact permission restrictions; revalidation is not new authority.

**Partial or unreadable observations.** An unavailable checksum/manifest or unknown tool identity should not become an empty fingerprint shared by unrelated states. Distinguish unknown, compatible and incompatible.

**Legacy records.** Keep old logs inspectable. Visits lacking the required identity cannot be retroactively labeled source-validated; conservative fresh scheduling avoids inventing provenance. Do not weaken existing refusal, ownership, no-tests or recovery-error controls.

## Fair proposed evaluation

No evaluation below was run. Compare pinned current main, conservative restart, and any qualified-reuse alternative on the same fixtures, requested scope, budgets, gates, policy and deterministic executor.

The source-inspected tests provide useful controls:

- The sweep test explicitly verifies that already asserted visits are skipped and that a new pass starts unvisited. It assumes the validity of those visit assertions; it does not establish restart/source compatibility. [Sweep fixture][sweep-test]
- The stop/resume test checks a stop after a judged repair and a later fix in another node. It does not commit a new defect into a completed node between runs. [Stop fixture][stop-test]
- Killed-attempt and missed-commit-acknowledgement tests exercise effect settlement. They are not completed-visit freshness tests. [Recovery fixtures][recovery-tests]

The minimum test matrix is:

| Case | Required observation |
|---|---|
| Complete journal, stopped pass, owner commit changes completed node, same graph/checks | Changed node is revisited or held for explicit revalidation; old completion does not discharge its current visit. |
| Same-context cold restart | Valid visits can be reused if the design claims that optimization; kept effects are not repeated. |
| Old pass has a recorded `pass_end` | Existing next-pass behavior remains correct. |
| Source changes only through acknowledged kept commits before the stop | Validate expected frontier rather than blindly comparing every visit to the pass-start revision. |
| Untracked admitted source changes while stopped | A `HEAD`-only optimization must not claim complete continuity. |
| Same source, changed gate/test/configuration | Reuse observes the changed check contract or reports uncertainty. |
| Changed focus, dependency graph or component identity | Current requested coverage is explicit; old node strings alone do not discharge it. |
| One wave versus two waves versus unbounded | Separate the first resumed suffix from the next fresh pass and measure time to consideration. |
| Restart after admission failure or interrupted checkpoint write | No false reusable visit; no lost durable history or repeated uncertain effect. |
| Dirty tracked workspace | Preserve startup refusal and ownership protections. |
| CLI and chat production entry paths | Both carry the same recovery decision and explain why progress was reused or invalidated. |

For the main fixture, capture source identity S0/S1, complete journal rows, graph/scope identity, the actual gate invocation and failure, attributed owner, recovered visit facts, next-node decisions, fresh `visit` rows, executor-dispatch records and budget-stop facts. Assert the oracle is broken at S1 independently of the application summary. Use a fresh kernel/process so a warm cache cannot accidentally hide recovery behavior.

Avoid a test that simply invokes `resume` with fabricated history and asserts the same done map: it would encode the faulty contract. Small unit fixtures can validate serialization and policy projection, but the acceptance case must traverse actual persistence, restart admission, fresh measurement, node selection and bounded completion.

Report at least: changed-node consideration rate, unsupported reuse decisions, wrongly repeated settled effects, valid-resume reuse rate, checks executed, pass count, gate time, restart latency, snapshot bytes read/written and peak memory. Separate node visits from model attempts and from successful repairs.

Only after deterministic acceptance should an optional matched mission study compare behavioral repair quality and cost. Hold model/provider, prompts, limits, source revisions, check contracts and scopes fixed; count no-change, blocked, unverified, refused, reverted and retained outcomes. Evaluate independent behavior, not only increased test count or lower open count. No paid experiment, performance gain or mission-quality improvement is claimed here.

## Decision threshold

The source supports a bounded recovery correction: **do not reuse incomplete-pass visit completion across an unqualified source/check transition**. It does not justify redesigning the scheduler, deleting the journal, disabling safe resume, weakening gates, broadening permissions or claiming every previous visit must always be repeated. The exit criterion is that fresh behavioral evidence for a changed completed node reaches current scheduling under the declared bounded-run contract, while unchanged-context recovery and effect ownership remain intact.

## Sources

All CodeNERD links above and below are immutable at the reviewed main revision. Primary Git documentation supports the distinction between a clean checkout and unchanged historical input. The incremental-build paper supplies a comparison framework only.

[pin]: https://github.com/theRebelliousNerd/codenerd/commit/25891ce9a81241875506104845cb40058f16a5e7
[root]: https://github.com/theRebelliousNerd/codenerd/blob/25891ce9a81241875506104845cb40058f16a5e7/agents.md#L1-L25
[campaign-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/25891ce9a81241875506104845cb40058f16a5e7/internal/campaign/agents.md#L1-L15
[architecture-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/25891ce9a81241875506104845cb40058f16a5e7/Docs/architecture/AGENTS.md#L48-L76
[journey]: https://github.com/theRebelliousNerd/codenerd/blob/25891ce9a81241875506104845cb40058f16a5e7/Docs/journeys/10-forever-loop.md#L162-L175
[cli]: https://github.com/theRebelliousNerd/codenerd/blob/25891ce9a81241875506104845cb40058f16a5e7/cmd/nerd/cmd_campaign_recurse.go#L247-L273
[cli-contract]: https://github.com/theRebelliousNerd/codenerd/blob/25891ce9a81241875506104845cb40058f16a5e7/cmd/nerd/cmd_campaign_recurse.go#L45-L91
[chat]: https://github.com/theRebelliousNerd/codenerd/blob/25891ce9a81241875506104845cb40058f16a5e7/cmd/nerd/chat/campaign_recurse.go#L196-L284
[startup]: https://github.com/theRebelliousNerd/codenerd/blob/25891ce9a81241875506104845cb40058f16a5e7/internal/campaign/recurse_cycle.go#L375-L450
[prepare]: https://github.com/theRebelliousNerd/codenerd/blob/25891ce9a81241875506104845cb40058f16a5e7/internal/campaign/recurse_git.go#L50-L77
[journal]: https://github.com/theRebelliousNerd/codenerd/blob/25891ce9a81241875506104845cb40058f16a5e7/internal/campaign/recurse_journal.go#L11-L64
[replay]: https://github.com/theRebelliousNerd/codenerd/blob/25891ce9a81241875506104845cb40058f16a5e7/internal/campaign/recurse_cycle.go#L1840-L1884
[pass]: https://github.com/theRebelliousNerd/codenerd/blob/25891ce9a81241875506104845cb40058f16a5e7/internal/campaign/recurse_cycle.go#L515-L570
[budget]: https://github.com/theRebelliousNerd/codenerd/blob/25891ce9a81241875506104845cb40058f16a5e7/internal/campaign/recurse_cycle.go#L489-L512
[close]: https://github.com/theRebelliousNerd/codenerd/blob/25891ce9a81241875506104845cb40058f16a5e7/internal/campaign/recurse_cycle.go#L572-L612
[visit-fact]: https://github.com/theRebelliousNerd/codenerd/blob/25891ce9a81241875506104845cb40058f16a5e7/internal/campaign/recurse_plan.go#L153-L186
[rules]: https://github.com/theRebelliousNerd/codenerd/blob/25891ce9a81241875506104845cb40058f16a5e7/internal/core/defaults/policy/recurse.mg#L400-L485
[probe]: https://github.com/theRebelliousNerd/codenerd/blob/25891ce9a81241875506104845cb40058f16a5e7/internal/campaign/recurse_plan.go#L318-L350
[measurement]: https://github.com/theRebelliousNerd/codenerd/blob/25891ce9a81241875506104845cb40058f16a5e7/internal/campaign/recurse_cycle.go#L700-L840
[attribution]: https://github.com/theRebelliousNerd/codenerd/blob/25891ce9a81241875506104845cb40058f16a5e7/internal/campaign/recurse_cycle.go#L870-L935
[open]: https://github.com/theRebelliousNerd/codenerd/blob/25891ce9a81241875506104845cb40058f16a5e7/internal/campaign/recurse_cycle.go#L1007-L1032
[visit]: https://github.com/theRebelliousNerd/codenerd/blob/25891ce9a81241875506104845cb40058f16a5e7/internal/campaign/recurse_cycle.go#L1052-L1106
[finding-facts]: https://github.com/theRebelliousNerd/codenerd/blob/25891ce9a81241875506104845cb40058f16a5e7/internal/campaign/recurse_policy.go#L66-L90
[candidate]: https://github.com/theRebelliousNerd/codenerd/blob/25891ce9a81241875506104845cb40058f16a5e7/internal/core/defaults/policy/recurse.mg#L237-L249
[boundary]: https://github.com/theRebelliousNerd/codenerd/blob/25891ce9a81241875506104845cb40058f16a5e7/internal/campaign/recurse_cycle.go#L452-L476
[no-flight]: https://github.com/theRebelliousNerd/codenerd/blob/25891ce9a81241875506104845cb40058f16a5e7/internal/campaign/recurse_cycle.go#L1783-L1806
[summary]: https://github.com/theRebelliousNerd/codenerd/blob/25891ce9a81241875506104845cb40058f16a5e7/internal/campaign/recurse_cycle.go#L144-L204
[status]: https://github.com/theRebelliousNerd/codenerd/blob/25891ce9a81241875506104845cb40058f16a5e7/internal/campaign/recurse_control.go#L91-L177
[declared-gates]: https://github.com/theRebelliousNerd/codenerd/blob/25891ce9a81241875506104845cb40058f16a5e7/internal/gates/detect.go#L19-L90
[finding-parser]: https://github.com/theRebelliousNerd/codenerd/blob/25891ce9a81241875506104845cb40058f16a5e7/internal/gates/findings.go#L65-L78
[finding-extract]: https://github.com/theRebelliousNerd/codenerd/blob/25891ce9a81241875506104845cb40058f16a5e7/internal/gates/findings.go#L112-L166
[python-graph]: https://github.com/theRebelliousNerd/codenerd/blob/25891ce9a81241875506104845cb40058f16a5e7/internal/campaign/recurse_workspace.go#L459-L478
[improvement-admission]: https://github.com/theRebelliousNerd/codenerd/blob/25891ce9a81241875506104845cb40058f16a5e7/internal/campaign/recurse_cycle.go#L1184-L1206
[mission]: https://github.com/theRebelliousNerd/codenerd/blob/25891ce9a81241875506104845cb40058f16a5e7/internal/campaign/recurse_cycle.go#L1251-L1283
[mission-prepare]: https://github.com/theRebelliousNerd/codenerd/blob/25891ce9a81241875506104845cb40058f16a5e7/internal/campaign/recurse_mission.go#L174-L248
[snapshot]: https://github.com/theRebelliousNerd/codenerd/blob/25891ce9a81241875506104845cb40058f16a5e7/internal/evidence/change.go#L169-L246
[sweep-test]: https://github.com/theRebelliousNerd/codenerd/blob/25891ce9a81241875506104845cb40058f16a5e7/internal/campaign/recurse_sweep_test.go#L81-L147
[stop-test]: https://github.com/theRebelliousNerd/codenerd/blob/25891ce9a81241875506104845cb40058f16a5e7/internal/campaign/recurse_control_test.go#L52-L109
[recovery-tests]: https://github.com/theRebelliousNerd/codenerd/blob/25891ce9a81241875506104845cb40058f16a5e7/internal/campaign/recurse_cycle_test.go#L257-L356
[prior-graph]: https://github.com/theRebelliousNerd/codenerd/blob/25891ce9a81241875506104845cb40058f16a5e7/Docs/audits/recurse-finding-owner-graph-transition.md
[prior-recovery]: https://github.com/theRebelliousNerd/codenerd/blob/25891ce9a81241875506104845cb40058f16a5e7/Docs/audits/recurse-recovery-attempt-identity.md
[prior-journal]: https://github.com/theRebelliousNerd/codenerd/blob/25891ce9a81241875506104845cb40058f16a5e7/Docs/audits/recurse-journal-replay-coverage.md
[prior-stall]: https://github.com/theRebelliousNerd/codenerd/blob/25891ce9a81241875506104845cb40058f16a5e7/Docs/audits/recurse-stall-evidence-applicability.md
[prior-namespace]: https://github.com/theRebelliousNerd/codenerd/blob/25891ce9a81241875506104845cb40058f16a5e7/Docs/audits/recurse-node-namespace-identity.md
[prior-campaign]: https://github.com/theRebelliousNerd/codenerd/blob/25891ce9a81241875506104845cb40058f16a5e7/Docs/audits/campaign-generation-admission-and-recovery.md
[portable]: https://github.com/theRebelliousNerd/codenerd/blob/25891ce9a81241875506104845cb40058f16a5e7/Docs/audits/portable-recurse-deliberation-crucible.md
[queue]: https://github.com/theRebelliousNerd/codenerd/blob/25891ce9a81241875506104845cb40058f16a5e7/Docs/audits/ASTRA_QUEUE.md
[git-status]: https://git-scm.com/docs/git-status
[build-paper]: https://www.microsoft.com/en-us/research/wp-content/uploads/2018/03/build-systems-final.pdf

