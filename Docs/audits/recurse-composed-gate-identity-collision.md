---
doc-class: deep-dive
subsystem: campaign-recurse-gate-admission
implementation-status: proposed
evidence-status: source-only
last-verified: 2026-11-04
verified-against: a4203f2bb38d577e560be2fb39edb46e2e42f2d5
verification-method: static-source-inspection
supersedes: []
---

# Recurse gate declarations need an unambiguous producer identity

## Finding and decision

**Two independently declared checks can receive the same gate ID, even though the project document passes validation.** `commands.lint` becomes `nerd.md:lint`; an additional `gates:` entry with `id: lint` becomes the same ID. Validation rejects duplicates within the `gates:` list but does not compare that list with generated command IDs. Detection retains both declarations. The distinction survives in each gate's `Source`, command and scope, but the consumers below identify them by `ID`. [Validation][validation] [Composition][composition] [Partition][partition]

This produces two related, concrete integration failures:

1. **Same scope:** both workspace checks execute, but their results are assigned to the same state key. One independently measured failure disappears from the retained measurement state, global finding count and explicit repair candidates. This is not just an unclear display name. [Baseline][baseline] [Candidates][candidates] [Count][count]
2. **Different scopes:** workspace and node results occupy different keys, but attempt construction can pair the workspace failure's correct check with the node result's key. The after-check target test then consults a passing node check rather than the workspace check that still reports the finding. Under the explicit changed-attempt conditions below, the host can derive target resolution from the wrong producer. [Attempt][attempt] [Source lookup][source] [Target evaluation][target-evaluation]

The immediate correction is a **composed gate-set identity admission boundary**, plus an indivisible source handle from finding selection to target judgment. Reject an ambiguous set before executing its checks or give the two declaration kinds genuinely distinct identities. Do not silently choose a winner, merge their results, or treat equal labels as equivalent verification contracts. Preserve Mangle's ownership of selection and verdicts; supply it faithful, independently attributable observations.

This is a source-supported conditional finding, not a reproduced incident. The omission witness requires no model call or edit. The target-judgment witness additionally requires an executor to return a net source change, as stated explicitly below; it does not claim that every production campaign would leave that change available for the outer ratchet.

## Evidence boundary and prior work

Public `theRebelliousNerd/codenerd` main resolved to [`a4203f2bb38d577e560be2fb39edb46e2e42f2d5`][pin] on November 4, 2026, and was rechecked unchanged at 00:46 UTC. This review inspected root and campaign guidance, campaign architecture and mission contracts, the forever-loop journey, the current audit inventory, source, and relevant authored tests. Source artifacts used here were checked against the pinned tree's Git blob identities. All CodeNERD links below use that immutable commit. [Root guidance][root] [Campaign guidance][campaign-guidance] [Architecture][architecture] [Journey][journey]

No repository implementation was changed. No test, build, gate, repository program, model inference, benchmark or failure injection was run. Source-artifact processing and this report are the only local work products. All witnesses, expected fixture observations and evaluations in this document remain unrun.

The targeted duplicate screen found no exact treatment of this producer collision in the current audit inventory or neighboring reports. That is a bounded screen, not a claim to have exhausted all historical discussion. Relevant prior work must retain credit:

| Earlier audit | Its boundary | What remains distinct here |
|---|---|---|
| [Gate subsumption][prior-subsumption] | A same-kind/language heuristic excluded an independently identified workspace finding from repair selection. The current conservative correction forwards it. | Equal generated IDs collapse state before candidate selection, or misidentify the target's scope. Removing the old filter does not establish identity uniqueness. |
| [Coverage gate identity][prior-coverage] | Multiple correctly keyed check results were reduced to one coverage metric. | The complete result can already be overwritten; there need be no coverage output or improvement attempt. |
| [Collapsed finding invocation identity][prior-invocation] | A collapsed node's finding needed its producing path-specific invocation rather than the aggregate's first result. | `sourceOf` can now select the right invocation, while `sourceNode` independently assigns the wrong state key because two declarations have one ID. The witness uses a single ordinary node. |
| [Finding census completeness][prior-census] | Work-selection/output limits did not establish an exhaustive finding census. | Each checker returns one short, fully captured diagnostic. The loss is an identity alias, not truncation or saturation. |
| [Node namespace identity][prior-node] | Physical components and synthetic stages shared node IDs; graph validation rejected collisions. | The colliding objects here are gate declarations. The gate pipeline lacks the analogous admission error and can silently consume ambiguous data. |
| [Gate contract continuity][prior-contract] | Environment and allowed-exit semantics had to travel with the selected check. | Both descriptors and their environments can remain intact. Their identities are conflated when storing or locating results. |

The [portable recurse/deliberation proposal][portable] already calls for accountable check identity and evidence. This report supplies a concrete producer counterexample and two falsifiable consumer consequences, not a new agent framework or a new claim to the general identity principle.

## 1. Source trace

### 1.1 Valid local declarations are not a valid composed identity domain

The project schema exposes canonical `commands.build/test/lint` and additional `gates:` declarations. `GateSpec.ID` is documented as the stable name on which findings and the recurse ledger key. `Spec.validate` starts an empty `seenGate` map and populates it only from `s.Gates`. It checks each entry's nonempty ID, uniqueness within that list, kind, command, scope and allowed exits. A node-scoped entry must include `{node}` or `{pkg}`. None of those checks reserves `build`, `test` or `lint` when the corresponding command exists. [Schema][schema] [Validation][validation]

`fromNerdMD` creates command gate IDs as `nerd.md:` plus the command kind and additional gate IDs as `nerd.md:` plus the entry's ID. It does retain separate `Source` values for the two origins. `Detect` suppresses **detected default** gates for kinds represented in `commands`, but it does not remove explicit `gates:` entries. `split` partitions by runnable availability and sorts by ID; it does not validate uniqueness. [Composition][composition] [Detection][detection] [Partition][partition]

The existing authored parser test rejects two list entries both named `a`. The existing detection test combines `commands.test` with differently named `deadcode` and `node-lint` entries. Those are useful positive controls, but neither crosses the generated-command/list-entry name boundary. This is a coverage observation about the inspected tests, not an assertion that those tests fail. [Parser tests][parser-tests] [Detection tests][detection-tests]

### 1.2 The baseline retains one value for one key

Every live pass calls `gates.Detect`. `measureWorkspace` collects all workspace-scoped gates, executes them with at most four concurrent workers, and stores each returned `gateRun` in an indexed `runs` slice. It joins all workers before serially assigning each result to `r.state[gateKey(g.ID, "")]`. `gateKey` uses only gate ID and node. For two workspace gates with equal IDs, the second assignment to that key replaces the first value. [Pass][pass] [Baseline][baseline] [Key][key]

This is **not** a race between checker completions. The indexed result collection and join are sound. The loss occurs in the later deterministic projection to a key space too small to distinguish the declarations. Go maps have unique keys; adding a mutex would not preserve two different values under the same key. [Go map contract](https://go.dev/ref/spec#Map_types)

It is also unnecessary to claim which declaration always wins. The sort compares only `ID`, and Go does not promise stable ordering for equal elements with `sort.Slice`. In either order, one result survives and one is replaced. A fixture should record the resulting order rather than depend on an incidental stable tie. [Partition][partition] [Go sorting contract](https://pkg.go.dev/sort#Slice)

`openCount` counts unique findings in retained state. `open` looks up that same state once for each workspace gate and forwards findings attributed to the current node. With duplicate same-scope IDs it can append the surviving finding twice; that does not recover the overwritten finding. The policy adapter receives only the supplied `open` findings. [Count][count] [Candidates][candidates] [Visit][visit]

### 1.3 A correct check can be paired with the wrong after-result key

With an all/node pair, measurement keys differ: for node `app`, the workspace result is stored under `nerd.md:lint@` and the node result under `nerd.md:lint@app`. This avoids the baseline overwrite but not the identity ambiguity. [Key][key] [Baseline][baseline] [Visit][visit]

When the workspace failure is selected, `sourceOf` first checks whether the node's result actually contains that finding. If the node check passed, it does not, so `sourceOf` correctly returns the workspace run and its producing invocation. Attempt construction copies that run's output, argv, allowed exits and environment. However, it computes `targetKey` separately with `sourceNode`. That helper returns the current node ID whenever **any** node-scoped gate in the set has the finding's gate ID. It does not ask which result produced this finding. [Source lookup][source] [Attempt][attempt]

After an executor returns a nonempty net change, the loop remeasures node gates and workspace build/lint gates, plus a selected workspace target. It builds before/after ratchet rows for the result keys. It sets `TargetOK` only at `k == spec.targetKey`, using that after-result's verification status, no-tests disposition, truncation status and target-ID membership. Those guards are useful, but they cannot establish that the result belongs to the selected obligation if `targetKey` already names the other check. [Remeasurement][remeasurement] [Target evaluation][target-evaluation]

The policy treats a resolved target as cycle success. A failing gate with unchanged finding count is deliberately not “worse”; a resolved target, actual change and absence of other vetoes permit a keep. This rule supports incremental repair of already-failing projects. It is not appropriate to rewrite that rule to conceal an upstream producer-identity error. [Policy projection][projection] [Ratchet policy][ratchet-policy]

## 2. Conditional witness A: two failures, one retained obligation

Use a clean Git workspace with an ordinary admitted Python directory `app`, containing `__init__.py`, `a.py` and `b.py`. Avoid a root Python marker for this minimal fixture, so detector-added Python checks do not add unrelated obligations. The dependency scanner can still discover these source files. Supply two project-owned executable fixture checkers and this complete frontmatter:

```yaml
---
schema: nerd/v1
commands:
  lint: ./checks/a
gates:
  - id: lint
    kind: lint
    run: ./checks/b
    scope: all
---
```

The checker contract, not an executed implementation, is:

| Checker | Exit | Complete output |
|---|---:|---|
| A | 1 | `app/a.py:1:1: alpha requirement failed` |
| B | 1 | `app/b.py:1:1: beta requirement failed` |

Stipulate a supported native execution environment, executable files present, successful process startup, no cancellation, no truncation, stable source/configuration, healthy kernel, and no prior refused/stalled state. The fixture checkers are read-only. The relative-command admission path accepts workspace-relative executables; their ability to run remains an explicit fixture precondition, not an observation from this review. The ordinary positioned diagnostics are recognized, and their distinct target paths produce distinct finding identities even though their gate IDs match. [Relative programs][partition] [Finding extraction][findings] [Diagnostic parser][diagnostics] [Source discovery][discovery]

The source-derived trace is:

1. The parser accepts the declarations: there is only one item in `gates:`.
2. Detection returns two lint gates named `nerd.md:lint`, both with scope `all`.
3. Both gates execute and each produces one attributed failure in `app`.
4. Both complete `gateRun` values enter `runs`; only one remains at `state["nerd.md:lint@"]` after assignment.
5. `openCount` retains one distinct finding, not two. `open(app, ...)` cannot forward the discarded failure, even though it may append the surviving failure twice.
6. The lost finding cannot be selected by the explicit finding-to-fix policy from this state. Another task might incidentally repair it, but that is not preservation of the measured obligation.

The proof does not depend on A or B winning, a flaky checker, arbitrary map iteration, a model choosing a bad edit, a large diagnostic stream, a graph-generation change or a restart. A separate live progress line can disclose that each check failed; this report does not claim that every possible log loses every trace. The consequential loss is in the retained measurement state and downstream decision inputs.

Changing the list ID from `lint` to `extra-lint` is the essential negative control: same commands, outputs, scopes, source files and process conditions; now the two state keys are distinct. Removing `commands.lint` is another control. Repeating identical bytes under two labels is not needed for this witness and must not be substituted for the two independent obligations.

## 3. Conditional witness B: passing local check judges an unresolved workspace target

Keep the same workspace but change the extra entry to:

```yaml
gates:
  - id: lint
    kind: lint
    run: ./checks/b {node}
    scope: node
```

Let workspace checker A report its single `app/a.py` failure before and after the proposed change. Let node checker B run successfully and return exit 0 before and after. Both reports are complete. There is no independent error-free snapshot configuration for this minimal host-path witness, no forbidden write, no test-count decrease, no existing refusal/stall and no other regression. The change is an ordinary authorized tracked source edit under `app`; it neither edits the checkers nor changes `nerd.md`, gate definitions, tests or permissions.

**Additional precondition:** after the selected attempt is handed off, the executor returns a nonempty retained change and an accurate owned-write receipt. This is a conditional input at the documented `RecurseExecutor` boundary. Whether an actual model-driven campaign produces or preserves that input is a separate integration question. It must not be assumed from this static audit. [Executor contract][executor-contract]

The key trace is independent of equal-key sort order:

| Stage | Workspace A | Node B |
|---|---|---|
| Baseline | One failure F at `nerd.md:lint@` | Pass at `nerd.md:lint@app` |
| Candidate | F is forwarded for `app` | No failure |
| `sourceOf(F)` | Correctly supplies A's command/output | Rejected as F's source because it does not contain F |
| `sourceNode(F)` | Not consulted as provenance | Its matching node-scoped ID makes the helper return `app` |
| After changed attempt | Still fails with the same single F | Still passes |
| Target test | Not used for `TargetOK` | F absent, so `TargetOK` becomes true |

The two ratchet comparisons are A fail-to-fail with one finding on both sides and B pass-to-pass. Neither is worse under the documented policy. With the stated actual-change and no-other-veto conditions, the host presents `/resolved` for F and the policy derives `/keep`; the keep branch then attempts Git adoption. This is a **conditional source trace through the host ratchet**, not a statement that a commit was made during review. Git or any other subsequent error can still prevent publication. [Remeasurement][remeasurement] [Target evaluation][target-evaluation] [Ratchet policy][ratchet-policy] [Keep][keep]

The workspace failure remains in state in this second witness. Thus this is not a disappearing-candidate case or a claim that the whole workspace becomes green. It is the narrower and more serious mismatch between the chosen target's check and the result used to declare that target resolved for this attempt.

The production CLI and chat wrap attempts in campaigns. Those campaigns may independently run the correctly supplied acceptance check and reject or compensate a non-fix. Preserve that control. Both entry paths nevertheless hand their executor and results to this same outer loop, whose contract explicitly remeasures rather than trusting the executor's success/error. An executor error is recorded as detail, not an independent ratchet veto. Accordingly, an evaluation must distinguish (a) the source-proven outer target-key mismatch, (b) the executor's actual retained-change behavior, and (c) any eventual Git adoption. Do not promote (a) into a measured claim about (c). [CLI adapter][cli] [Chat adapter][chat] [Campaign acceptance][acceptance] [Executor contract][executor-contract] [Error recording][error-recording]

Renaming B to `node-lint` is again a direct negative control: the target key stays workspace-scoped, F remains present in its after-result, and `TargetOK` must remain false under otherwise identical conditions. A legitimate repair of A must still be eligible for normal acceptance.

## 4. Why deliberation and existing guards do not establish this contract

Mission preparation receives an already-selected `RecurseAttempt` and copies its finding, check and node paths into the mission receipt before running design-only deliberation. A better proposer/interrogator exchange does not restore a workspace result that the host already replaced. Conversely, a well-reasoned design does not prove that an after-result belongs to its named target. Source identity must be reliable before and after that exchange. [Mission preparation][mission]

Mission check sidecars project the before/after maps that the loop supplies. In witness A, they cannot reconstruct the overwritten gate run from the surviving value's invocation list: that list belongs to that one measured gate run, not to every declaration that shared its ID. In witness B, both keyed measurements can remain available for forensic inspection. This is not a claim that all mission evidence vanishes, or that the mission's additional scope/protected-input checks necessarily permit the conditional edit. [Sidecar][sidecar]

Several existing controls remain sound and should remain unchanged:

- Malformed project frontmatter is an error rather than a silent fallback to detected commands.
- Explicit command kinds replace detected defaults; extra list declarations remain additional checks.
- Unavailable programs are reported separately, not called passes.
- The baseline joins all checker workers before publishing their state.
- The target test guards against unverified, no-tests and failed-truncated results.
- Mangle retains selection, nonregression, forbidden-write and keep/revert authority.
- Campaign acceptance, owned-write compensation, mission scope and Git publication checks remain independent controls.

The defect is the missing identity relation between those controls. Neither successful process execution nor a fresh source snapshot distinguishes two producers if the application has already assigned them the same logical name.

## 5. Proposed contract and bounded correction

Start in the owning project-document/gates and campaign specifications. State these invariants before changing implementation:

1. **Composed uniqueness.** Every admitted gate in a detected set has one unambiguous identity across canonical commands, explicit entries and detected defaults. This property is checked after composition, not inferred from uniqueness inside one input list.
2. **No implicit equivalence.** Matching user labels, kind, language or argv does not silently authorize merging checks. An intentional alias needs an explicit relationship and a demonstrated common execution contract.
3. **One provenance handle.** Finding selection returns the producing gate identity, source state key and specific invocation together. Attempt evidence, acceptance command, target remeasurement and mission evidence all refer to that same handle. A later scan for “any matching ID” must not invent its scope.
4. **Conservation.** Every required admitted measurement is retained independently or receives an explicit unavailable/invalid/held disposition. A consumer must not quietly lose a run during map projection.
5. **Decision separation.** The host validates representations and reports observations; Mangle derives eligibility and verdicts. A collision is not evidence that the target was fixed or that two obligations are redundant.

Two implementation paths deserve comparison rather than an unexamined mandatory migration:

**Smallest containment:** reject a composed set containing duplicate IDs with an error identifying both declaration locations and scopes. Validate the full composed set before availability partitioning so a temporarily absent executable cannot hide ambiguity until a later run. Reject before gate dispatch in the shared path; optionally surface the same error during planning. Do not say this necessarily occurs before all live-command side effects: startup currently acquires ownership and prepares Git before entering the pass. This option preserves existing IDs for unambiguous configurations and requires the owner to disambiguate an ambiguous declaration. [Startup][startup]

**Representation repair:** give command fields, explicit gates and detected defaults separate typed producer identities with a deterministic, injective encoding. Keep friendly labels and declaration paths separate. Use a structured gate/node key internally, and retain the producing invocation/key instead of reconstructing it from a label. Merely adding scope to a display string fixes neither the same-scope collision nor arbitrary encoding ambiguity. Preserve current replacement precedence and expected command semantics.

The representation repair affects finding identity: `FindingsAll` hashes gate ID with target and sometimes message signature. Changing all gate IDs will therefore change finding IDs, not just presentation. Historical attempts, refusals, stalls, prior diffs and mission receipts require an explicit compatibility decision. Map legacy identities only when the old declaration and scope are uniquely recoverable; preserve ambiguous history as historical evidence rather than assigning it to whichever current producer happens to match. Do not silently erase a prior refusal or transfer it to another check. A narrow rejection path avoids much of this migration cost while full identity support is designed. [Finding identity][findings] [Attempt history][history]

Whichever option is chosen, add a defensive consumer invariant: the selected source handle must resolve to exactly one required invocation and remain compatible with the after-check contract. An absent or ambiguous source is a visible hold/error, not target absence. Returning the source state key with `sourceOf` would remove the current independent scope reconstruction, but that alone does not repair the earlier same-scope overwrite. The producer and consumer contracts must close together.

Do not fix this by making sorting stable, selecting first-wins, dropping one declaration, concatenating both outputs under one gate, running only the smaller check, or weakening the incremental nonregression rule. Those alternatives make a choice deterministic without demonstrating that the chosen check covers the omitted obligation.

## 6. Costs and failure modes

An admission-only uniqueness check needs expected O(G) time and O(G) identity storage for G composed gates. It adds no model call and need not add any gate process. A corrected representation retains O(G) result slots instead of accidentally sharing slots; retaining all actual findings uses the storage the independent obligations require. These are algorithmic estimates, not benchmarks.

The user-visible cost of rejection is real: a configuration currently accepted becomes blocked until names are disambiguated. The error should identify the canonical command and extra entry, not merely say “duplicate.” A distinct-identity implementation avoids that interruption but expands migration, fact, journal, receipt and display work. Restored findings may increase necessary repair attempts and model consumption. Lower apparent open counts or fewer attempted fixes on the current path are not efficiency wins if they reflect lost checks.

Failure modes to guard against include validating only available gates, validating only the project list, fixing storage without target lookup, fixing target lookup without storage, treating an empty-language custom checker as interchangeable with another, applying legacy refusal evidence to the wrong new identity, and changing detected-default precedence during a naming repair. Scope changes or changed executable/argv/environment/pass semantics still need the separate invocation-applicability contract described by earlier audits; a unique producer ID is necessary but does not prove those semantics are unchanged.

Persisting provenance must not expose environment secrets in prompts, public facts or report text. Use trusted runtime descriptors and safe declaration references. No new external authority, model-side shell access or autonomous tool installation is needed to detect or repair the identity boundary.

## 7. Proposed evaluation, explicitly unrun

Compare the pinned baseline, a duplicate-set admission fix, and a distinct-producer implementation using the same workspaces, checker behaviors, source changes, kernel policy and execution budgets. An error/hold is a valid safe result for the admission fix; do not score it as if it preserved two runnable identities. Distinguish prevention, complete support and end-to-end repair.

### Deterministic boundary cases

| Case | Required observation |
|---|---|
| Witness A: all/all, two short distinct failures | Baseline exposes both subprocess results but retains one key. Corrected support preserves two obligations, or rejects before dispatch with both declarations identified. |
| A with one failure and one pass, both orders | No passing run can conceal the other producer's required failure. Do not depend on equal-element sort stability. |
| A with one result unable to start or complete | The incomplete producer cannot disappear behind the other's pass; availability and runtime error are separately represented. |
| Witness B: all/node, unchanged workspace failure, passing node result | Check argv, source key and after-result producer must agree. No `/resolved` fact may be derived from the unrelated local pass. |
| B with a real workspace repair | Legitimate target resolution still works, with other guards unchanged. |
| Distinct IDs with equal kind/language | Preserve current conservative candidate admission and all independent measurements. |
| `build`, `test` and `lint` collisions separately | The repair covers every generated canonical-command name, not only the illustrative lint case. |
| Duplicate IDs wholly inside `gates:` | Existing parser rejection remains intact. |
| Canonical command absent, list ID `lint` alone | No invented ambiguity or unnecessary reservation of a harmless name under the chosen contract. |
| Same target and message from distinct producers | No accidental finding-ID conflation after representation repair. |
| Unavailable collision member later becomes available | No latent identity change silently changes the winner or transfers history. |
| Different node/language, collapsed-node and root-node variants | Preserve producer identity independently of attribution and per-path invocation identity. |

For witness A, first exercise the actual project parser and detector, then real fixture checkers through `measureWorkspace` and the node candidate projection. Merely constructing an already-invalid map would bypass the producer defect. Inspect retained gate identities, finding IDs, unique counts, candidate facts and any sidecar projection, not only progress text.

For witness B, use the production `RunRecurseCycles` host with a real policy kernel and a deterministic executor returning the stipulated source edit plus accurate receipt. Measure the actual check invocations and after-results. Record the selected check, `targetKey`, `TargetOK`, asserted ratchet facts, derived verdict, retained change and commit outcome separately. No paid inference is needed to establish this host boundary.

Then cover normal CLI/chat campaign wiring using deterministic task-execution fixtures. Specifically determine whether a failed inner acceptance leaves any retained edit for the outer ratchet, compensates it, or returns no change. A passing inner guard narrows exposure; it does not excuse an incorrect outer target identity. Do not manufacture a production keep claim by substituting a permissive executor and omitting that substitution from the result.

For a namespace migration, add restart cases with unique legacy records and deliberately ambiguous old `nerd.md:lint` histories. Confirm either a justified mapping or a visible reconciliation outcome, never accidental refusal/stall transfer. Exercise cancellation, checker error, no-tests results, truncated output and Git failure as independent controls. Nothing about collision repair should weaken existing behavior in those cases.

Record admitted declarations, attempted and completed checker invocations, retained result identities, projected unique findings, selected obligations, target-source agreement, safe holds, wrong resolutions, ratchet outcomes, compensation and publication outcomes. Also record validation CPU/allocations, state/receipt size, added remeasurements, actual model calls and total task cost where later authorized. Report failures and blocked cases, not just successful runs. This review supplies no numerical reliability improvement, token saving, latency gain or incident frequency.

## Pinned primary references

[pin]: https://github.com/theRebelliousNerd/codenerd/commit/a4203f2bb38d577e560be2fb39edb46e2e42f2d5
[root]: https://github.com/theRebelliousNerd/codenerd/blob/a4203f2bb38d577e560be2fb39edb46e2e42f2d5/agents.md
[campaign-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/a4203f2bb38d577e560be2fb39edb46e2e42f2d5/internal/campaign/agents.md
[architecture]: https://github.com/theRebelliousNerd/codenerd/blob/a4203f2bb38d577e560be2fb39edb46e2e42f2d5/Docs/architecture/campaign/INTERNALS.md
[journey]: https://github.com/theRebelliousNerd/codenerd/blob/a4203f2bb38d577e560be2fb39edb46e2e42f2d5/Docs/journeys/10-forever-loop.md
[portable]: https://github.com/theRebelliousNerd/codenerd/blob/a4203f2bb38d577e560be2fb39edb46e2e42f2d5/Docs/audits/portable-recurse-deliberation-crucible.md
[prior-subsumption]: https://github.com/theRebelliousNerd/codenerd/blob/a4203f2bb38d577e560be2fb39edb46e2e42f2d5/Docs/audits/recurse-gate-subsumption.md
[prior-coverage]: https://github.com/theRebelliousNerd/codenerd/blob/a4203f2bb38d577e560be2fb39edb46e2e42f2d5/Docs/audits/recurse-coverage-gate-identity.md
[prior-invocation]: https://github.com/theRebelliousNerd/codenerd/blob/a4203f2bb38d577e560be2fb39edb46e2e42f2d5/Docs/audits/recurse-collapsed-finding-invocation-identity.md
[prior-census]: https://github.com/theRebelliousNerd/codenerd/blob/a4203f2bb38d577e560be2fb39edb46e2e42f2d5/Docs/audits/recurse-finding-census-completeness.md
[prior-node]: https://github.com/theRebelliousNerd/codenerd/blob/a4203f2bb38d577e560be2fb39edb46e2e42f2d5/Docs/audits/recurse-node-namespace-identity.md
[prior-contract]: https://github.com/theRebelliousNerd/codenerd/blob/a4203f2bb38d577e560be2fb39edb46e2e42f2d5/Docs/audits/recurse-gate-contract-continuity.md
[schema]: https://github.com/theRebelliousNerd/codenerd/blob/a4203f2bb38d577e560be2fb39edb46e2e42f2d5/internal/projectdoc/nerdmd.go#L78-L200
[validation]: https://github.com/theRebelliousNerd/codenerd/blob/a4203f2bb38d577e560be2fb39edb46e2e42f2d5/internal/projectdoc/nerdmd.go#L518-L548
[composition]: https://github.com/theRebelliousNerd/codenerd/blob/a4203f2bb38d577e560be2fb39edb46e2e42f2d5/internal/gates/detect.go#L46-L90
[detection]: https://github.com/theRebelliousNerd/codenerd/blob/a4203f2bb38d577e560be2fb39edb46e2e42f2d5/internal/gates/detect.go#L19-L43
[partition]: https://github.com/theRebelliousNerd/codenerd/blob/a4203f2bb38d577e560be2fb39edb46e2e42f2d5/internal/gates/gates.go#L167-L196
[parser-tests]: https://github.com/theRebelliousNerd/codenerd/blob/a4203f2bb38d577e560be2fb39edb46e2e42f2d5/internal/projectdoc/nerdmd_test.go#L293-L331
[detection-tests]: https://github.com/theRebelliousNerd/codenerd/blob/a4203f2bb38d577e560be2fb39edb46e2e42f2d5/internal/gates/detect_test.go#L182-L232
[key]: https://github.com/theRebelliousNerd/codenerd/blob/a4203f2bb38d577e560be2fb39edb46e2e42f2d5/internal/campaign/recurse_cycle.go#L351
[pass]: https://github.com/theRebelliousNerd/codenerd/blob/a4203f2bb38d577e560be2fb39edb46e2e42f2d5/internal/campaign/recurse_cycle.go#L515-L569
[baseline]: https://github.com/theRebelliousNerd/codenerd/blob/a4203f2bb38d577e560be2fb39edb46e2e42f2d5/internal/campaign/recurse_cycle.go#L699-L751
[count]: https://github.com/theRebelliousNerd/codenerd/blob/a4203f2bb38d577e560be2fb39edb46e2e42f2d5/internal/campaign/recurse_cycle.go#L983-L991
[candidates]: https://github.com/theRebelliousNerd/codenerd/blob/a4203f2bb38d577e560be2fb39edb46e2e42f2d5/internal/campaign/recurse_cycle.go#L1006-L1031
[visit]: https://github.com/theRebelliousNerd/codenerd/blob/a4203f2bb38d577e560be2fb39edb46e2e42f2d5/internal/campaign/recurse_cycle.go#L1052-L1095
[attempt]: https://github.com/theRebelliousNerd/codenerd/blob/a4203f2bb38d577e560be2fb39edb46e2e42f2d5/internal/campaign/recurse_cycle.go#L1151-L1178
[source]: https://github.com/theRebelliousNerd/codenerd/blob/a4203f2bb38d577e560be2fb39edb46e2e42f2d5/internal/campaign/recurse_cycle.go#L1677-L1702
[remeasurement]: https://github.com/theRebelliousNerd/codenerd/blob/a4203f2bb38d577e560be2fb39edb46e2e42f2d5/internal/campaign/recurse_cycle.go#L1322-L1351
[target-evaluation]: https://github.com/theRebelliousNerd/codenerd/blob/a4203f2bb38d577e560be2fb39edb46e2e42f2d5/internal/campaign/recurse_cycle.go#L1413-L1444
[projection]: https://github.com/theRebelliousNerd/codenerd/blob/a4203f2bb38d577e560be2fb39edb46e2e42f2d5/internal/campaign/recurse_policy.go#L203-L271
[ratchet-policy]: https://github.com/theRebelliousNerd/codenerd/blob/a4203f2bb38d577e560be2fb39edb46e2e42f2d5/internal/core/defaults/policy/recurse.mg#L260-L329
[findings]: https://github.com/theRebelliousNerd/codenerd/blob/a4203f2bb38d577e560be2fb39edb46e2e42f2d5/internal/gates/findings.go#L98-L184
[discovery]: https://github.com/theRebelliousNerd/codenerd/blob/a4203f2bb38d577e560be2fb39edb46e2e42f2d5/internal/campaign/recurse_workspace.go#L89-L105
[executor-contract]: https://github.com/theRebelliousNerd/codenerd/blob/a4203f2bb38d577e560be2fb39edb46e2e42f2d5/internal/campaign/recurse_cycle.go#L95-L105
[error-recording]: https://github.com/theRebelliousNerd/codenerd/blob/a4203f2bb38d577e560be2fb39edb46e2e42f2d5/internal/campaign/recurse_cycle.go#L1461-L1464
[keep]: https://github.com/theRebelliousNerd/codenerd/blob/a4203f2bb38d577e560be2fb39edb46e2e42f2d5/internal/campaign/recurse_cycle.go#L1480-L1507
[cli]: https://github.com/theRebelliousNerd/codenerd/blob/a4203f2bb38d577e560be2fb39edb46e2e42f2d5/cmd/nerd/cmd_campaign_recurse.go#L225-L264
[chat]: https://github.com/theRebelliousNerd/codenerd/blob/a4203f2bb38d577e560be2fb39edb46e2e42f2d5/cmd/nerd/chat/campaign_recurse.go#L196-L241
[mission]: https://github.com/theRebelliousNerd/codenerd/blob/a4203f2bb38d577e560be2fb39edb46e2e42f2d5/internal/campaign/recurse_mission.go#L174-L251
[sidecar]: https://github.com/theRebelliousNerd/codenerd/blob/a4203f2bb38d577e560be2fb39edb46e2e42f2d5/internal/campaign/recurse_mission_storage.go#L103-L151
[startup]: https://github.com/theRebelliousNerd/codenerd/blob/a4203f2bb38d577e560be2fb39edb46e2e42f2d5/internal/campaign/recurse_cycle.go#L356-L467
[history]: https://github.com/theRebelliousNerd/codenerd/blob/a4203f2bb38d577e560be2fb39edb46e2e42f2d5/internal/campaign/recurse_cycle.go#L1840-L1884

[diagnostics]: https://github.com/theRebelliousNerd/codenerd/blob/a4203f2bb38d577e560be2fb39edb46e2e42f2d5/internal/gates/findings.go#L340-L375
[acceptance]: https://github.com/theRebelliousNerd/codenerd/blob/a4203f2bb38d577e560be2fb39edb46e2e42f2d5/internal/campaign/recurse_attempt.go#L95-L115
