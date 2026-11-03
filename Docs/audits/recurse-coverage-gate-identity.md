# Recurse coverage gate identity and the improvement ratchet

## Current-main addendum: baseline gate coverage is compared

Static review at public main `a4fd84e66accf2780b3ddfcc6a77bef7d16def5d`, November 3, 2026. The first-parseable-gate witness below is historical. [Commit 97a4245](https://github.com/theRebelliousNerd/codenerd/commit/97a4245a50f1159901d2f89bff375fe41e7d5998) collects per-gate coverage keys; the policy bridge pairs each baseline key with its after value, treating a missing after value as -1. One gate's rise therefore no longer hides another baseline gate's fall in the stated witness. [Collection](https://github.com/theRebelliousNerd/codenerd/blob/a4fd84e66accf2780b3ddfcc6a77bef7d16def5d/internal/campaign/recurse_cycle.go#L1575-L1604) [Projection](https://github.com/theRebelliousNerd/codenerd/blob/a4fd84e66accf2780b3ddfcc6a77bef7d16def5d/internal/campaign/recurse_policy.go#L226-L248)

The authored fixture checks rising, falling and missing reports with constructed results and a real-kernel helper; it does not run checks or the full adoption path. Pairing still lacks explicit domain/toolchain identity, and combined output does not preserve separate invocation domains. No tests ran in this review. [Fixture](https://github.com/theRebelliousNerd/codenerd/blob/a4fd84e66accf2780b3ddfcc6a77bef7d16def5d/internal/campaign/recurse_audits_test.go#L88-L191)

**Status: source-derived audit and proposed contract.** Inspected public CodeNERD revision `1a179efba7deb5c6149aea1da4970e66f92592b5` on 2026-11-03. No implementation, build, test, gate, application path, model call, inference or benchmark was run. The witness below is a conditional derivation from the pinned source, not an executed reproduction or an observed user incident.

## Decision in brief

Keep the existing recurse loop and Mangle ratchet. Repair the coverage observation boundary between them. Recurse executes independent node test gates, but `metrics` exports only the first parseable coverage report in gate-ID order as one `coverage` scalar without its gate identity. A later gate's measured coverage regression is therefore absent from the metric facts used to decide improvement and nonregression. The loss occurs after measurement, not because the later command was skipped. [Gate ordering][order] [Measurement][visit] [Metric projection][metrics]

A clean conditional witness needs no malformed output, missing command, changed gate declaration or failing test: gate A reports 50% then 60%; gate B reports 80% then 40%; both complete successfully on both sides. If A sorts first, the harden angle sees `coverage 5000->6000`. Given a changed permitted source tree, unchanged test count and the other independent guards succeeding, current policy can derive `/keep`; the host then enters its normal commit path and counts the attempt as improved. B's decrease never becomes a coverage fact. This is narrower than a claim that arbitrary regressions bypass every guard, and broader than a cosmetic journal issue. [Projection][metrics] [Facts][metric-facts] [Policy][keep-policy] [Adoption][adoption]

The proposed correction is a named collection of coverage observations with an explicit comparison domain and required-observation inventory. Host code measures and preserves those observations; Mangle determines comparability, required nonregression and improvement. Do not replace alphabetical selection with an unexplained minimum or average across unrelated domains. A deliberately configured primary metric is a different, defensible contract only when the other measurements are explicitly informational and the narrower promise is visible.

## Evidence boundary and architectural fit

This audit concerns the existing local `recurseRun.metrics` path for non-cross-cutting nodes with more than one applicable node-scoped test gate. It does not assert a universal portability defect, a behavioral correctness theorem, or that a mission has falsely satisfied its North Star. Coverage remains a proxy: preserving every reported percentage would not prove that the tests exercise the intended behavior.

The repository's root guidance assigns executive decisions to Mangle, asks for production-path validation and requires spec-first changes. Those are the constraints for a repair, not reasons to move adoption decisions into a new Go heuristic or ask an LLM to judge whether a coverage loss is acceptable. The portable recurse proposal already distinguishes proxy metrics from requirement evidence and proposes gate IDs, command/configuration identities and coverage domains. This audit supplies a concrete existing projection gap for that design; it does not claim to invent the broader identity contract. [Root guidance][guidance] [Spec-first contract][spec-first] [Portable boundary][portable-boundary] [Proposed manifest][portable-manifest]

“Measured” below means the source retains a runner result. “Projected” means that result contributes a fact to the policy under review. “Kept” means the ordinary ratchet/adoption outcome. These are separate properties. None alone means independently demonstrated behavioral improvement.

## The production call chain

### Declared checks reach the shared loop

The CLI constructs a production executor around `RecurseAttemptCampaign`, the campaign orchestrator and `executeCampaignPlan`, then passes that executor into `RunRecurseCycles`. Mission construction is optional; the coverage witness does not require it. This is not a dead parser or an isolated test helper. [CLI executor and driver][cli]

`Detect` loads project declarations. Commands override detected gates of the same kind; explicit gate entries preserve their own IDs, kind and scope. A declared entry with `scope: node` becomes a node gate whose ID is prefixed with `nerd.md:`. The resulting runnable gate set is sorted by ID. `nodeGates` filters that ordered slice for node scope and language applicability without changing its order. [Declarations][detect] [Declared node gates][declared] [Ordering][order] [Node selection][node-gates]

On a visit, the driver measures every applicable node gate and stores the result under its gate/node key. The current `open` path also forwards independent findings rather than declaring one command to subsume another merely because their kind or language matches. The coverage problem must not be confused with that already-repaired selection boundary. [Visit and independent findings][visit-open]

### Both measurements exist before the scalar is formed

The improvement step first asks policy which angle is measurable. A cross-cutting node is skipped by this path. `run` computes the before metrics and supplies them on the attempt; after a nonempty change it remeasures every gate in `v.own`, merges those results with the prior state and computes after metrics. Therefore, under the witness's completed before/after measurements, the second test gate has run and its output is present on both sides before the scalar projection. [Improvement selection][improve] [Before metrics][before] [Remeasurement][after]

`metrics` counts workspace tests and, when applicable, node source lines. For coverage it iterates `own`, skips non-test gates and calls `gates.Coverage` on each keyed result. At the first successful parse it writes `m[gates.MetricCoverage]` and breaks. Gate ID, command and coverage domain are not carried in that value. The first parseable report wins, not necessarily the first test gate and not the least-covered gate across all runs. [Metric projection][metrics]

`Coverage` recognizes Go statement-coverage lines and pytest-cov `TOTAL` summaries, returns basis points, and takes the minimum when several recognized percentages occur **within the one supplied output string**. The corresponding test covers that within-output minimum. Neither establishes aggregation or independent nonregression across multiple gate results. [Coverage parser][coverage] [Parser tests][coverage-tests]

### The kernel receives the narrowed evidence

`ratchetInput.Before` and `.After` are `map[string]int`. The bridge emits `recurse_metric(Cycle, Metric, Before, After)` only when the same metric name exists on both sides. For this path the name is `/coverage`, with no gate or domain argument. In contrast, the separate gate-verdict rows preserve each gate/node key and its pass/fail/unverified state and finding counts. [Input shape and scalar facts][metric-facts] [All gate verdicts][verdicts]

Policy allows `/harden` to improve through increasing coverage **or** increasing test count. It blocks an improvement whose projected coverage falls, and blocks any attempt whose projected test count falls. It also retains the independent gate, refusal and changed-tree conditions. These rules are consistent over their inputs; the host has already discarded the second coverage observation before the rules can inspect it. [Angle and metric guards][metric-policy] [Gate and keep rules][keep-policy]

Existing policy test definitions support this distinction: `TestRecursePolicy_ImprovementKeptOnlyOnAMeasuredMove` expects a coverage-decreasing improvement to revert and a harden coverage increase to keep. It constructs one scalar metric map directly, however, so it does not exercise the preceding selection among independently measured gate outputs. This is evidence of the intended scalar guard, not a claim that those tests were run or that no other test exists. [Existing policy fixture][policy-tests]

The journey describes a coverage decrease on an improvement as grounds for reverting. The current first-report projection gives that sentence a narrower operational meaning than “none of the independently measured node test coverage domains decreased.” A repair must choose and document which promise is intended. [Journey contract][journey]

## A deterministic conditional witness

### Preconditions

Use one ordinary Python source node whose single path is `example_pkg`, not a collapsed multi-path cycle or a cross-cutting node. Its owner declares two applicable node-scoped test gates:

| Gate | Fixed check population | Unchanged declared `run` template |
|---|---|---|
| `nerd.md:a-unit` | Unit suite A measuring the admitted package | `pytest tests/unit --cov={node} --cov-report=term` |
| `nerd.md:z-integration` | Integration suite B measuring the admitted package | `pytest tests/integration --cov={node} --cov-report=term` |

The declaration IDs are `a-unit` and `z-integration`, both with `kind: test` and `scope: node`; detection supplies the `nerd.md:` prefix shown above. Node-scoped declaration validation requires `{node}` or `{pkg}` in `run`. These templates satisfy that requirement, and `ForNode("example_pkg")` expands their coverage argument to `--cov=example_pkg`. The actual argv are therefore `pytest tests/unit --cov=example_pkg --cov-report=term` and the integration equivalent, unchanged before and after. [Declaration validation][gate-validation] [Placeholder expansion][node-expansion] [Declared gates][declared]

These are illustrative command descriptors, not executed commands or a tested fixture. Stipulate that their tools and dependencies are installed, the admitted configuration permits the reported percentages without a failing coverage threshold, each check actually collects tests, and both return exit 0. The exact command text need not change to produce the witness. Detection recognizes `pytest` as Python, and declarations preserve its independent node-gate identity. Any other applicable checks succeed and do not sort ahead of A with a parseable coverage report. [Language classification][language] [Declared gates][declared]

Compare each gate only to **itself**: its test population, package/statement domain, command, toolchain, environment and runner configuration remain comparable before and after. A and B are independent observations; their percentages are not substitutes for one another. Source revision changes are intentional. For this narrow witness even the reported statement denominator is held at 100 on each side.

The `/harden` attempt makes a permitted production-source change, not a test, gate, fixture or configuration edit. Workspace test count is unchanged. The executor completes; all ordinary scope, permission, gate, optional snapshot and adoption prerequisites succeed. Outputs are small and intact, excluding truncation as a cause. The stipulated reports are:

| Observation | Before output summary | After output summary | Same-gate delta |
|---|---|---|---|
| A | `TOTAL 100 50 50%` | `TOTAL 100 40 60%` | 5000 to 6000 basis points |
| B | `TOTAL 100 20 80%` | `TOTAL 100 60 40%` | 8000 to 4000 basis points |
| A and B process verdicts | Exit 0, pass, no findings | Exit 0, pass, no findings | No process or finding regression |
| Workspace test count | K | K | No count regression |

The source parser accepts all four coverage summaries. This table is a hypothetical observation input, not fabricated runtime output represented as real evidence. Whether a particular source patch and real suites produce it remains an unrun fixture obligation. [Parser][coverage]

### Source-derived outcome

1. Both gates are measured before the attempt and after the source change.
2. A sorts before B. `metrics` parses A, sets coverage to 5000 before and 6000 after, and breaks on each call.
3. The policy bridge therefore receives `recurse_metric(Cycle, /coverage, 5000, 6000)`, plus unchanged test count and any other available metrics. It does not receive a B coverage row.
4. Both gate-verdict rows are pass-to-pass with no findings. They do not trigger the existing gate-worse rules. The observed test verdict also supplies `recurse_ratchet_tested`.
5. Coverage increasing satisfies `/harden`. No supplied guard metric declines. With the stipulated independent prerequisites, `/keep` is derivable.
6. On that verdict the host invokes its normal commit operation. If adoption succeeds, it updates gate state and increments both kept and improved counts. A journal metric delta can show `coverage 5000->6000` while B's 8000-to-4000 decrease is absent from that summary. [Measurement][visit] [After measurement][after] [Projection][metrics] [Facts][metric-facts] [Verdicts][verdicts] [Policy][keep-policy] [Metric rules][metric-policy] [Adoption][adoption]

This establishes a conditional path to keeping a change with a measured secondary coverage decrease. It does not claim a commit was produced during this review or that a concrete behavioral bug escaped the test suites.

### Order sensitivity without changing an attempt contract

For a second, otherwise equivalent invocation, configure the same two checks under IDs that put B first **before the invocation starts**. Keep those declarations fixed through that invocation. Now the selected scalar is 8000 to 4000: `/harden` has no positive coverage movement, unchanged tests provide no alternative improvement, and the coverage guard declines. The policy reverts under the same other assumptions.

This comparison is between separately admitted naming arrangements, not an attempt that edits `nerd.md` mid-flight. It avoids conflating the issue with declaration protection or command-contract continuity. The gate identifier thereby selects which independent measurement governs retention, even when the only difference between these separately admitted configurations is naming. No source contract inspected here explicitly appoints the lexicographically earliest parseable gate as an owner-chosen primary coverage metric.

## What evidence is retained and what is durable

`gateRun` retains a `gates.Result`; the runner result includes gate, node, expanded argv, exit code, output, pass state, error and duration. `measure` stores the first result for a node and combines outputs when a collapsed node contains several paths. The witness uses one path so that combination does not obscure the gate-level loss. These retained results are sufficient to distinguish A and B before `metrics` drops that distinction. [Run state][run-state] [Measurement][measure] [Runner result][runner]

“Output retained” does not mean unlimited raw logs. `gates.Run` keeps bounded combined stdout/stderr, with a default 1 MiB head/tail buffer and a dropped-middle marker. For multi-path nodes, `measure` appends the per-run bounded outputs. The proposed metric contract must state whether the retained evidence is complete enough to interpret; it cannot assume that a parsable percentage proves an entire report survived. [Output bounds][runner] [Multi-path combination][measure]

For mission runs, `recordChecks` writes before/after maps keyed by the original gate/node key to a checks sidecar. Each row includes snapshot, verdict, argv, allowed exits, exit code, retained output and any error; the sidecar also records the source-after identity and optional error-free snapshots. The call happens after the ratchet verdict is obtained and before the scalar journal record is constructed. Thus the observation loss is not necessarily loss of all forensic evidence: a successful mission sidecar write can preserve both bounded reports even though B had no coverage-policy influence. [Sidecar schema and persistence][sidecar] [Call order][mission-order]

The ordinary recurse journal has a scalar `Metrics` string, outcome and detail fields; it is not a per-gate coverage ledger. This review does not claim that every ordinary non-mission invocation durably saves every gate's output. Nor should the mission sidecar be described as magically fixing an earlier policy projection. [Journal record][journal] [Metric rendering][metric-rendering]

Mission evidence is a separate boundary. The host initializes the mission witness to `/unknown` and changes it to `/resolved` only for a finding-backed resolution meeting its snapshot conditions. The improvement-only witness has no such finding, so this audit does not establish false mission satisfaction. [Mission witness conditions][mission-order]

## Distinctions and counterexamples

- **This is not a skipped-gate finding.** Both commands execute, both results remain in keyed state, and both verdicts reach the ratchet. The lost dimension is coverage identity at metric projection.
- **A secondary pass-to-fail or unverified result is different.** Its verdict row is still projected and can veto keeping through existing gate rules. The witness deliberately keeps both gates passing; an exit status is not a continuous coverage-nonregression check. [Verdicts][verdicts] [Gate rules][gate-rules]
- **One parseable test-gate report has no cross-gate winner conflict.** Its current scalar behavior can remain compatible, although missing data and report validity still need explicit semantics.
- **The parser's within-output minimum does not solve the cross-gate problem.** Nor does changing `break` to a global minimum: A 50% to 60% and B 80% to 70% makes the minimum rise from 50% to 60% while B still regresses. A weighted or unweighted average likewise needs an admitted meaning and can conceal an individual required-domain decrease.
- **All coverage reports disappearing is an adjacent boundary, not this witness.** Scalar facts are emitted only for metric names present on both sides. A repair should test missing and truncated observations explicitly, but neither is needed for the two complete-report case. [Scalar intersection][metric-facts]
- **Optional error-free snapshot admission is separate.** Current recurse policy has vetoes for snapshot regressions and unadmitted comparisons. They do not add the discarded per-test-gate coverage observations to `recurse_metric`. The witness either omits that optional measurement or assumes its independent checks admit the comparison. [Snapshot vetoes][snapshot-policy]
- **Improvement coverage and fix coverage are not identical rules.** The existing coverage guard is attached to improvement attempts; this report does not recast it as a coverage veto on every finding fix. The unchanged test-count guard remains independent. [Metric guards][metric-policy]
- **A percentage decrease is not proof of a behavioral regression.** This is a mismatch between the measured nonregression contract and its projection. Its practical severity must be evaluated, not inferred from dramatic percentages alone.

## Proposed observation and policy contract

Start in the owning campaign and gates specifications, then change the projection and consumers together. The minimal invariant is: **for every required admitted coverage observation, before and after remain paired by the same declared measurement identity, and any incomparable or absent required observation remains explicit rather than disappearing from the decision domain.**

An observation should carry at least:

| Field | Required meaning |
|---|---|
| Observation identity | Gate ID, logical node/domain identity and stable observation key; gate display order is not priority. |
| Invocation identity | Admitted command/arguments, working scope, toolchain and relevant configuration/environment identity, with secrets excluded. |
| Measurement definition | Parser/schema version, reported unit, coverage kind and the test/package/statement domain the percentage describes. |
| Source relation | Before and after source identities plus the admitted change relation; different source revisions are expected, not automatically incomparable. |
| Value and disposition | Basis points when available; otherwise missing, unsupported, truncated, unverified or incomparable with a reason. Never synthesize zero from absence. |
| Provenance | Reference to the relevant bounded runner result and its completeness/retention status. |
| Obligation | Whether this observation is required for nonregression, eligible to establish the angle's improvement, or informational under the admitted contract. |

Do not use gate ID alone as proof of comparability, or require byte-identical source trees that would make improvement impossible. Conversely, do not treat equal percentages or equal command strings as proof that test population, toolchain and configuration remained comparable. The portable proposal's manifest is the natural place to define these meanings, not a parallel scheduler. [Portable manifest][portable-manifest]

Host code should collect and parse already-measured results, preserve provenance, validate input shape and assert an explicit inventory. Mangle should derive which pairs are admitted, whether any required observation regressed or remains unknown, whether an eligible metric improved, and the final ratchet outcome. A missing required row must not become approval through the absence of a regression fact. Predicate names and schema details are implementation choices to be settled in the spec; none are claimed to exist now. [Executive boundary][guidance] [Existing keep rules][keep-policy]

There are three meaningful alternatives:

1. **Retain the current scalar as an explicitly configured primary metric.** The owner or admitted policy names the primary domain. All other measured percentages remain visible and explicitly informational. This is smaller but narrows the coverage promise; silently treating alphabetic order as configuration is not sufficient.
2. **Require per-domain nonregression.** Preserve all admitted required coverage pairs and let an eligible increase establish hardening only if no required domain declines or becomes unknown. This directly addresses the witness but can reject trade-offs an owner might consider acceptable. Make any tolerance or exception an explicit policy contract, not a hidden averaging rule.
3. **Add a reviewed aggregate alongside per-domain obligations.** An aggregate may be useful for a genuinely shared denominator or deliberate trade-off objective. It needs an adapter-defined interpretation and should not erase required per-domain vetoes. Cross-ecosystem percentages are not automatically commensurate.

Preserve all existing process-verdict, finding-count, test-count, scope, permission, optional snapshot, ownership and adoption guards. A richer coverage contract is additional evidence, not authority to weaken them or let the creative model redefine the verifier.

## Costs and rollout constraints

This repair need not launch an additional test process: both gate results in the witness are already measured. That is not a claim of zero cost. Current parsing stops at a prefix of the ordered gates; parsing all G relevant outputs adds work proportional to their retained bytes, up to all G reports. The host needs more observations, policy facts and comparisons; journals or sidecars need schema/version and retention handling; status displays need explanations that distinguish missing, informational and vetoing evidence.

Domain identity may require configuration inventories, hashing and adapter maintenance. Legacy records cannot be silently upgraded into comparable evidence they never contained. Rebaselining or returning unknown adds owner and execution cost. Rejected attempts and their compensation still consume campaign/model time. More accurate nonregression could reduce wasted acceptance or increase false reverts; both are empirical outcomes. No token savings, net latency improvement or overall reliability gain is measured here.

A bounded rollout can first emit observation-only per-gate metrics beside the existing decision, then review real contract differences before enabling a veto. Shadow output must say that it is not yet enforced. Version the evidence schema and preserve access to the underlying results across mission reload. Keep any execution, implementation or repository publication separately authorized; this audit itself performs none.

## Unrun validation and fair comparison

### Deterministic tests before model experiments

The first evidence should be a real-kernel/shared-loop fixture with a scripted executor and real declared check commands. The script changes only admitted source; it does not replace the outer ratchet or inject a preselected keep verdict. Real check fixtures must produce the before/after reports from that source state. This separates deterministic host/policy correctness from whether a model can discover a useful patch. The first pass requires no inference.

Proposed acceptance cases, all unrun:

| Case | Evidence to require |
|---|---|
| Two complete passing reports with A rising and B falling | Both observations reach policy; per-domain mode vetoes the decrease; current behavior is recorded rather than assumed. |
| Gate order and display-name permutation across separately admitted runs | Same policy outcome for equivalent obligations; no mid-attempt declaration edits. |
| Each gate's domain unchanged versus changed | Same-domain pairs compare; a changed test population/configuration is explicit unknown or requires re-admission. |
| A rises while B falls but remains above A | Reject the global-minimum shortcut demonstrated above. |
| Secondary gate fails, cannot start or is cancelled | Existing independent verdict veto remains effective. |
| Coverage absent, unsupported, malformed or critically truncated | Required metric remains unknown; no zero substitution or silent cross-gate replacement. |
| Single gate and multiple recognized lines within one result | Preserve intended one-gate behavior; test within-output aggregation separately from cross-gate obligations. |
| Go and pytest-cov outputs, plus unsupported formats | Adapter limits remain explicit; percentages are not averaged merely because they parse. |
| Multi-path collapsed node | Establish what the combined output represents and whether every required subdomain remains covered. |
| Initially failing or contradictory baseline evidence | Preserve deliberate incremental-repair semantics while refusing unsupported metric claims. |
| Mission persistence and reload | Named observations and bounded source results remain linked; improvement does not acquire a false resolved-finding witness. |
| Forbidden path, test-count decrease, snapshot-admission veto and adoption failure controls | New metric handling does not bypass any independent guard or claim a commit that did not complete. |

Then validate the same behavior through normal CLI and chat entry paths with the production configuration expected by the repository. A parser unit test alone cannot establish that the shared loop projects every required observation or that a persisted decision remains inspectable. Tests of the actual boundary should accompany focused owning-package and policy-corpus checks; no such checks were run for this document.

### Matched outcome and cost study

Compare current first-parseable selection, configured-primary semantics and per-domain semantics on the same frozen gate declarations, task/source baselines, models if later authorized, permission boundaries and full-cost budgets. Include one-gate repositories, multiple suites on one package, mixed ecosystems, initially failing repositories and unsupported/truncated reports. Prespecify the admitted nonregression contract before labeling outcomes; otherwise “false keep” merely encodes a preference introduced after seeing the result.

Report false keeps and false reverts against that independent contract, with denominators for all eligible attempts; supported improvements per attempt; required-observation completeness; unknown/incomparable fraction; name/order sensitivity; gate/process invocations; parser/host CPU and wall time; model calls/tokens/spend where applicable; retained bytes; compensation/recovery cost; and owner-review burden. Separate reported-coverage retention from independent behavioral acceptance. Include rejected, failed, abandoned and budget-exhausted attempts rather than pricing only kept ones.

Hold the gate inventory fixed where comparing algorithms. If a variant requires extra checks, account for their full cost rather than attributing their information to a better scalar rule. Publish repository/task-family sample limits and uncertainty; no result is promised in advance. A primary-only contract may be the right fit for some owners, while per-domain nonregression may be too strict for deliberate trade-offs. The study should expose that choice rather than declare one universally safe.

## Bounded duplication review

The nearest inspected material already covers important neighboring contracts:

- The portable recurse proposal names limited metric adapters, proxy semantics and a future capability/domain manifest. This audit refines that proposal with the concrete first-parseable gate projection and an order-sensitive witness. [Portable limits][portable-boundary] [Manifest][portable-manifest]
- The gate-subsumption audit's as-built section records the repair that forwards independent findings. Here those findings and commands need not be suppressed at all; the later metric reduction is the distinct boundary. [Subsumption as built][subsumption]
- The gate-contract-continuity audit's as-built section records environment and allowed-exit continuity repairs. The witness keeps both commands, environments and successful exits unchanged and does not resurrect those findings. [Continuity as built][continuity]
- The coverage source-identity audit concerns assigning a profile block to the wrong source file during session element-witness attribution. The present case can have correctly attributed, intact summaries from both commands; it concerns their later selection as a recurse metric, not filename-suffix matching. [Coverage attribution scope][source-identity]
- The gate-comparison-domain admission audit concerns optional error-free snapshot sets, scopes and finding-enumeration meaning. Its identity principle is relevant, but its snapshot comparisons do not supply per-node coverage observations to the local metric map. Current snapshot vetoes remain present. [Earlier comparison scope][comparison-audit] [Current vetoes][snapshot-policy]
- The recurse journey states the intended coverage regression guard without documenting first-report precedence. [Journey][journey]

This is a bounded comparison with the named materials and relevant current source, not an exhaustive novelty claim over every branch, conversation or historical audit. The actionable increment is small: retain the already-measured coverage identities through the existing policy boundary, make the selected contract explicit and test its production consumers.

## Definition of done

For one retained improvement an owner should be able to identify every required coverage domain, inspect its comparable before/after observations, see which positive measurement established improvement, see which regressions or unknowns could veto it, and trace the verdict to the actual adoption outcome. Changing only gate names between otherwise equivalent, newly admitted configurations must not silently change those obligations. Ordinary checks and Mangle authority must remain intact.

That would close this projection gap. It would not prove that the tests are adequate, that coverage is the correct strategic objective, or that recurse is portable to every repository.

## Pinned public source references

[guidance]: https://github.com/theRebelliousNerd/codenerd/blob/1a179efba7deb5c6149aea1da4970e66f92592b5/agents.md#L3-L23
[spec-first]: https://github.com/theRebelliousNerd/codenerd/blob/1a179efba7deb5c6149aea1da4970e66f92592b5/agents.md#L92-L108
[portable-boundary]: https://github.com/theRebelliousNerd/codenerd/blob/1a179efba7deb5c6149aea1da4970e66f92592b5/Docs/audits/portable-recurse-deliberation-crucible.md#L28-L44
[portable-manifest]: https://github.com/theRebelliousNerd/codenerd/blob/1a179efba7deb5c6149aea1da4970e66f92592b5/Docs/audits/portable-recurse-deliberation-crucible.md#L46-L59
[cli]: https://github.com/theRebelliousNerd/codenerd/blob/1a179efba7deb5c6149aea1da4970e66f92592b5/cmd/nerd/cmd_campaign_recurse.go#L211-L263
[detect]: https://github.com/theRebelliousNerd/codenerd/blob/1a179efba7deb5c6149aea1da4970e66f92592b5/internal/gates/detect.go#L15-L43
[declared]: https://github.com/theRebelliousNerd/codenerd/blob/1a179efba7deb5c6149aea1da4970e66f92592b5/internal/gates/detect.go#L51-L90
[gate-validation]: https://github.com/theRebelliousNerd/codenerd/blob/1a179efba7deb5c6149aea1da4970e66f92592b5/internal/projectdoc/nerdmd.go#L518-L548
[node-expansion]: https://github.com/theRebelliousNerd/codenerd/blob/1a179efba7deb5c6149aea1da4970e66f92592b5/internal/gates/gates.go#L129-L146
[language]: https://github.com/theRebelliousNerd/codenerd/blob/1a179efba7deb5c6149aea1da4970e66f92592b5/internal/gates/detect.go#L93-L117
[order]: https://github.com/theRebelliousNerd/codenerd/blob/1a179efba7deb5c6149aea1da4970e66f92592b5/internal/gates/gates.go#L171-L195
[node-gates]: https://github.com/theRebelliousNerd/codenerd/blob/1a179efba7deb5c6149aea1da4970e66f92592b5/internal/campaign/recurse_cycle.go#L899-L910
[visit]: https://github.com/theRebelliousNerd/codenerd/blob/1a179efba7deb5c6149aea1da4970e66f92592b5/internal/campaign/recurse_cycle.go#L958-L965
[visit-open]: https://github.com/theRebelliousNerd/codenerd/blob/1a179efba7deb5c6149aea1da4970e66f92592b5/internal/campaign/recurse_cycle.go#L913-L969
[improve]: https://github.com/theRebelliousNerd/codenerd/blob/1a179efba7deb5c6149aea1da4970e66f92592b5/internal/campaign/recurse_cycle.go#L1087-L1107
[before]: https://github.com/theRebelliousNerd/codenerd/blob/1a179efba7deb5c6149aea1da4970e66f92592b5/internal/campaign/recurse_cycle.go#L1110-L1144
[after]: https://github.com/theRebelliousNerd/codenerd/blob/1a179efba7deb5c6149aea1da4970e66f92592b5/internal/campaign/recurse_cycle.go#L1208-L1237
[metrics]: https://github.com/theRebelliousNerd/codenerd/blob/1a179efba7deb5c6149aea1da4970e66f92592b5/internal/campaign/recurse_cycle.go#L1454-L1475
[metric-rendering]: https://github.com/theRebelliousNerd/codenerd/blob/1a179efba7deb5c6149aea1da4970e66f92592b5/internal/campaign/recurse_cycle.go#L1478-L1486
[coverage]: https://github.com/theRebelliousNerd/codenerd/blob/1a179efba7deb5c6149aea1da4970e66f92592b5/internal/gates/metrics.go#L171-L204
[coverage-tests]: https://github.com/theRebelliousNerd/codenerd/blob/1a179efba7deb5c6149aea1da4970e66f92592b5/internal/gates/metrics_test.go#L48-L63
[policy-tests]: https://github.com/theRebelliousNerd/codenerd/blob/1a179efba7deb5c6149aea1da4970e66f92592b5/internal/campaign/recurse_policy_test.go#L206-L243
[metric-facts]: https://github.com/theRebelliousNerd/codenerd/blob/1a179efba7deb5c6149aea1da4970e66f92592b5/internal/campaign/recurse_policy.go#L179-L231
[verdicts]: https://github.com/theRebelliousNerd/codenerd/blob/1a179efba7deb5c6149aea1da4970e66f92592b5/internal/campaign/recurse_cycle.go#L1303-L1317
[gate-rules]: https://github.com/theRebelliousNerd/codenerd/blob/1a179efba7deb5c6149aea1da4970e66f92592b5/internal/core/defaults/policy/recurse.mg#L260-L281
[snapshot-policy]: https://github.com/theRebelliousNerd/codenerd/blob/1a179efba7deb5c6149aea1da4970e66f92592b5/internal/core/defaults/policy/recurse.mg#L283-L294
[keep-policy]: https://github.com/theRebelliousNerd/codenerd/blob/1a179efba7deb5c6149aea1da4970e66f92592b5/internal/core/defaults/policy/recurse.mg#L260-L329
[metric-policy]: https://github.com/theRebelliousNerd/codenerd/blob/1a179efba7deb5c6149aea1da4970e66f92592b5/internal/core/defaults/policy/recurse.mg#L356-L398
[adoption]: https://github.com/theRebelliousNerd/codenerd/blob/1a179efba7deb5c6149aea1da4970e66f92592b5/internal/campaign/recurse_cycle.go#L1336-L1382
[run-state]: https://github.com/theRebelliousNerd/codenerd/blob/1a179efba7deb5c6149aea1da4970e66f92592b5/internal/campaign/recurse_cycle.go#L205-L220
[measure]: https://github.com/theRebelliousNerd/codenerd/blob/1a179efba7deb5c6149aea1da4970e66f92592b5/internal/campaign/recurse_cycle.go#L708-L777
[runner]: https://github.com/theRebelliousNerd/codenerd/blob/1a179efba7deb5c6149aea1da4970e66f92592b5/internal/gates/run.go#L15-L78
[sidecar]: https://github.com/theRebelliousNerd/codenerd/blob/1a179efba7deb5c6149aea1da4970e66f92592b5/internal/campaign/recurse_mission_storage.go#L80-L121
[mission-order]: https://github.com/theRebelliousNerd/codenerd/blob/1a179efba7deb5c6149aea1da4970e66f92592b5/internal/campaign/recurse_cycle.go#L1321-L1336
[journal]: https://github.com/theRebelliousNerd/codenerd/blob/1a179efba7deb5c6149aea1da4970e66f92592b5/internal/campaign/recurse_journal.go#L11-L59
[journey]: https://github.com/theRebelliousNerd/codenerd/blob/1a179efba7deb5c6149aea1da4970e66f92592b5/Docs/journeys/10-forever-loop.md#L140-L159
[subsumption]: https://github.com/theRebelliousNerd/codenerd/blob/1a179efba7deb5c6149aea1da4970e66f92592b5/Docs/audits/recurse-gate-subsumption.md#L158-L190
[continuity]: https://github.com/theRebelliousNerd/codenerd/blob/1a179efba7deb5c6149aea1da4970e66f92592b5/Docs/audits/recurse-gate-contract-continuity.md#L183-L225
[comparison-audit]: https://github.com/theRebelliousNerd/codenerd/blob/1a179efba7deb5c6149aea1da4970e66f92592b5/Docs/audits/gate-comparison-domain-admission.md#L3-L17
[source-identity]: https://github.com/theRebelliousNerd/codenerd/blob/1a179efba7deb5c6149aea1da4970e66f92592b5/Docs/audits/coverage-source-identity-witness.md#L11-L29
