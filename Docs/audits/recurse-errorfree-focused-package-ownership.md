---
doc-class: deep-dive
subsystem: campaign-recurse-focused-selection
implementation-status: proposed
evidence-status: source-only
last-verified: 2026-11-04
verified-against: 055a8576f8b20d17143037578cffda5225e29515
verification-method: static-source-inspection
supersedes: []
---

# Error-free package findings can escape a focused root-package sweep

## Finding and required outcome

**A root-package-only recurse sweep can select a repair in an independent top-level package.** With `--subsystem . --gates vet`, the dependency filter correctly excludes an unrelated package `lib`. The optional error-free measurement still observes the whole module, as designed. Its projection changes package `./lib` into target `lib` and asks a generic attribution helper to resolve it using only the focused graph. Because that graph contains the root but not `lib`, the helper treats the slash-free package directory as a root file. The resulting finding is offered to the root visit, while its acceptance command still checks `./lib`. [Projection][projection] [Attribution][attribution] [Dispatch][dispatch]

The defect is **package ownership and requested-work fidelity**, not broad measurement itself. A global regression baseline can be valuable during a focused run. Observing a finding outside the selected dependency closure should not silently make that finding work for a different node.

Preserve the subject's kind, resolve its owner against the complete admitted graph, and then apply the selected-node boundary. A known outside owner, an unresolved owner and a selected owner must remain distinct. Keep the complete observation for regression checks and reporting without manufacturing an in-scope repair.

This is a conditional static finding at [main revision `055a8576f8b20d17143037578cffda5225e29515`][pin], inspected November 4, 2026 and rechecked unchanged at 17:20 UTC. No application, test, build, gate, fixture, model inference, training or benchmark was run. No implementation, credential or setting changed. Runtime incidence, actual token costs and whether a particular model would complete the misdirected repair are unverified.

## Contract, architectural role and implementation status

The CLI describes `--subsystem` as focusing nodes with dependencies pulled in. The recurse journey makes the same distinction, and `FilterDAG` implements transitive dependency closure rather than recursive filesystem containment. Root package `.` is therefore a graph node, not shorthand for selecting every independent package below the repository directory. [CLI flags][flags] [Journey][journey] [Filter][filter]

The root architecture assigns work selection to the deterministic executive, with the model solving admitted tasks. Mission deliberation is downstream of the selected attempt. A wrong owner asserted into the candidate facts is consequently not repaired merely by better proposer/challenger reasoning. Architecture documentation also requires present implementation and proposed guarantees to stay separate. [Root guidance][root] [Architecture guidance][architecture] [Mission preparation][mission]

| Capability | Status at the pin | Boundary |
|---|---|---|
| Discover Go package nodes and import dependencies | Implemented | Production, in-package-test and external-test import lists become graph edges. [Go discovery][go-graph] |
| Focus visits on named nodes and their prerequisites | Implemented | The driver retains the full graph and installs a filtered visit graph. [Pass setup][pass] |
| Measure optional error-free gates across the module | Implemented | Initial refresh passes nil package scope; vet defaults to `./...`. [Projection][projection] [Measurer][measurer] |
| Preserve package kind through error-free owner attribution | Missing at this boundary | A package directory is passed to the generic target/root-file heuristic. [Projection][projection] [Attribution][attribution] |
| Reject out-of-focus error-free groups before repair candidacy | Partial | Correct ownership normally prevents a match, but this root fallback fabricates a selected owner. [Candidate collection][open] |
| Proposed typed owner and explicit scope-disposition contract | Not implemented by this report | Acceptance criteria below are unrun. |
| Runtime reproduction or measured quality/efficiency improvement | Unverified | This report provides source deductions only. |

The normal root-file behavior is legitimate: a diagnostic naming `main.go` belongs to the root package. The error is using that fallback for an input already known to be a **package directory**. Deleting root-file attribution altogether would repair the witness by breaking a valid control.

## Production trace

### Independent options reach the same driver

The CLI constructs the optional `recurseGateMeasure` callback, then passes both `Subsystems` and `MeasureGates` into `RunRecurseCycles`. The callback transfers its package argument into `MeasureOptions.ScopedPackages`; it does not independently intersect that argument with CLI subsystem focus. The initial nil argument is therefore materially different from an explicit selected-package list. [CLI wiring][cli] [Callback][callback]

The ordinary chat adapter shown at this pin does not supply this optional callback. The concrete entry claimed here is the CLI with `--gates`, not every recurse invocation or every chat run. Omitting `--gates` leaves the callback nil and bypasses this projection. [Chat adapter][chat] [Nil guard][projection]

### The graph and focus can both be correct

Go discovery maps admitted package directories to workspace-relative node paths and adds only imports whose target is another discovered workspace package. If root and `lib` are independent, the graph contains both nodes but no root-to-lib edge. `FilterDAG(nodes, ["."])` keeps the root and its actual dependencies; it does not include `lib` merely because its directory is beneath the root directory. The synthetic closing stages are dependents of physical nodes, not automatically prerequisites of the root. [Go discovery][go-graph] [Filter][filter] [Closing stages][closing]

The pass driver saves the complete graph in `r.allNodes`, applies focus to the local `nodes` slice, then supplies that slice to each visit. Thus the complete ownership information exists at the same time that the narrower projection loses it. [Pass setup][pass] [Visit dispatch][visit-dispatch]

### A package directory enters a file-shaped fallback

At the first visit, `refreshGates` has no baseline, so it calls `MeasureGates(ctx, nil)`. For vet, nil `ScopedPackages` selects the normal full `./...` measurement. This can report `lib/bad.go` even though only root is being visited. [Refresh][projection] [Measurer][measurer]

The gate reader passes the workspace root into `lintreport.VetFindings`. That parser preserves the analyzer name and normalizes in-workspace source positions to relative file paths, so an absolute position under the real `lib/bad.go` can reach the group builder as `File="lib/bad.go", Check="printf"`. No fabricated package label is required. [Reader binding][reader] [Diagnostic parser][parser]

`FindingGroups` obtains the package from `path.Dir(f.File)`. A supported `printf` finding in `lib/bad.go` therefore produces:

- package: `./lib`;
- group identity: based on gate, check and that package;
- witness: `go vet -printf ./lib`;
- source finding: the original located `lib/bad.go` diagnostic.

The group already has enough semantic information to avoid treating its subject as a file. [Grouping][groups] [Witness construction][witness]

The recurse adapter nevertheless strips `./` and calls `attributeNode("lib", focusedNodes)`. That helper reserves the `.` node for later fallback, finds no exact or prefix match because `lib` is absent from the focus, then returns the root for a target without `/`. Its comment explains that branch in terms of a root **file**. The string `lib` satisfies the syntax test despite being a known package directory. [Projection][projection] [Attribution][attribution]

With the full graph, exact matching would have selected the actual `lib` node before the fallback. With an outside package such as `internal/lib`, the slash prevents this particular root fallback. The inconsistent behavior depends on the spelling depth of an outside package, not on whether it is in the selected dependency closure.

### Wrong ownership becomes real repair work

`open` adds an error-free finding when its projected `Node` equals the active node. The wrong root owner therefore admits the `lib` group into the root visit. `recursePolicy.visit` asserts that owner alongside the group shape, and Mangle's candidate rule joins on the active visit's node. A fresh single-file `printf` group has a normal lint rank and is not one of the excluded unused-code checks. With no other competing finding, previous refusal, stall or already-attempted marker, it is selected. [Candidate collection][open] [Fact projection][facts] [Rank and exclusions][rank] [Selection][selection]

The group dispatch then constructs an attempt from the original group, preserving the `./lib` witness, while `r.run` assigns the selected root node to the attempt. The ordinary CLI executor constructs a campaign from this attempt. Its goal and evidence name the outside subject, its phase names the root, and its task write set is copied from the root node's paths. This is more than a mislabeled summary: the wrong work can be handed to the repair executor. [Dispatch][dispatch] [Attempt node][attempt-node] [Group specification][group-spec] [CLI executor][cli-execute] [Campaign construction][campaign]

That is the end of the primary proof. A successful mutation or retained commit is not required to establish the work-selection failure.

### The ordinary path does not rescue the error-free path

Ordinary gate results are projected elsewhere. For focused workspace findings that remain unattributed in the narrowed graph, `measure` consults `r.allNodes` and omits findings known to belong to another subsystem. A located `lib/bad.go` remains slash-bearing and can use that route. Error-free groups do not pass through it; they have already been reduced to package target `lib`. [Ordinary projection][ordinary]

This ordinary path is a useful existing control, not a general proof that all focused attribution is correct. Merely copying its “if currently unattributed” conditional would also be insufficient here: the false root assignment is already a seemingly resolved owner. Ownership must be resolved in the right domain **before** focus is applied.

## Conditional witness, not an executed reproduction

Use a fresh, admitted Go module in a disposable clean Git repository, with `.nerd/` ignored and no interrupted recurse history:

```text
go.mod
root.go
root_test.go
lib/bad.go
```

The root package has a passing test and no import of the `lib` package, including through its tests. The outside package contains a small exported function with a format/argument mismatch, such as `fmt.Printf("%d", "text")`. No source, test or project configuration changes during graph acquisition and measurement. The toolchain and normal host/kernel initialization are available, all reads succeed, and vet's complete report identifies the `printf` diagnostic at the real `lib/bad.go` location.

The Go project's documentation describes directory package patterns separately from lists of source files, and documents vet's format/argument check and JSON reporting. These primary references support the proposed input and package-domain distinction; they are not evidence that this fixture was run or that vet establishes whole-program correctness. [Go package patterns][go-packages] [Go vet][go-vet]

The future invocation is:

```text
nerd campaign recurse --subsystem . --gates vet --waves 1
```

The source-derived transition is:

| Stage | Observation or state | Result |
|---|---|---|
| Discovery | Nodes `.` and `lib`, no dependency between them | Both have correct owners |
| Focus | Selected closure contains only `.` | `lib` is outside requested visits |
| Ordinary root checks | Root package remains clean | No root repair competes |
| Error-free baseline | Full vet reports `lib/bad.go`, check `printf` | Group package is `./lib` |
| Projection | Target becomes `lib`; only root is available for attribution | Root-file fallback sets `Node="."` |
| Policy | The single group matches the active root visit | It is eligible and selected |
| Dispatch | Root node plus outside-group evidence and witness | Executor receives an outside-package repair as root work |

The first decisive assertion should inspect the dispatched `RecurseAttempt`: `Node.ID=="."` while `Finding.Target=="lib"` and `Check` targets `./lib`. A deterministic future fixture can record that tuple and cancel before mutation. It does not need a stochastic model to make an unwanted edit.

The example requires the configured check to produce the stated complete diagnostic and no unrelated failure to abort the run. Those are explicit preconditions, not fabricated test results. No dependency edge is omitted, no graph generation changes, no compiler record is truncated and no prior journal is reused.

## Consequences, countercontrols and limits

- **The focus promise is violated at candidate selection.** The user requested the root node and its prerequisites; the independent outside package is neither. A fully correct repair of that outside package would still be the wrong selected work.
- **This is not new filesystem authority.** Root `Paths=["."]` is already broad as a containment scope. Campaign construction copies that scope. Do not assume downstream write-set checks will reinterpret it as “root package files only,” and do not describe the finding as a proven permission bypass. Ordinary constitutional permission, write ownership, review and ratchet requirements still apply. [Node contract][node-contract] [Campaign construction][campaign]
- **No false green is established.** The group witness remains aimed at `./lib`, and the independent after-check/ratchet controls are unchanged. The problem is choosing the outside repair, not a demonstrated successful lie about its outcome. [Witness execution][witness-run]
- **Global observations are not inherently out of bounds.** The `--gates` feature deliberately maintains a regression baseline. The fix should separate measured coverage from repair eligibility, not hide outside regressions or automatically narrow every gate.
- **The bug is optional and spelling-sensitive.** With no error-free callback, this route is absent. Without a selected root, the exact fallback is absent. In a full sweep, the actual `lib` node wins. An actual root-to-lib import legitimately includes `lib` in the selected prerequisite closure.
- **Mission reasoning is not an owner oracle.** The ordinary non-mission witness suffices. In mission mode, the selected attempt is already formed before deliberation; the model could notice a mismatch, but that does not restore deterministic focus fidelity. No mission acceptance or bypass claim is made. [Mission preparation][mission]

Saltzer and Schroeder's original protection paper motivates minimizing the authority exercised for a job and checking object identity at mediation boundaries. Applied here as a design analogy, observation of the whole module should not itself authorize a new repair subject, and a general path spelling should not replace a known object kind. This literature supports the separation of responsibilities; it neither proves this source trace nor turns a package-selection error into a demonstrated security exploit. [Saltzer and Schroeder, 1975][saltzer]

## Required contract and bounded design

The following is proposed, not shipped:

1. **Preserve subject kind.** Error-free groups carry a package-directory subject. File diagnostics, test selectors, package directories and unknown targets must not share an unqualified root-file fallback.
2. **Resolve the owner in the complete admitted graph.** Use the current graph's canonical member paths, including collapsed components. A selected subset is a work filter, not a new ownership universe.
3. **Project focus explicitly.** Each group must be in one of: selected owner, known outside owner, unresolved/unsupported owner. Only the first becomes a repair candidate for this run. Keep outside findings in the baseline and expose their disposition without inflating the selected work list.
4. **Join policy on observed scope facts.** The host supplies typed subject/owner/focus membership; Mangle remains responsible for ranking, exclusions, stalls, selection and stopping. Do not replace its selection with a second Go scheduler.
5. **Bind derived ownership to graph changes.** If reused, ownership projections need invalidation when graph or focus changes. This is shared groundwork with the existing graph-transition audit, not a second novelty claim.
6. **Keep the witness's domain intact.** Do not rewrite `./lib` into `.` to make the command appear compatible with the wrong owner. That would turn a scope error into verification of a different package.
7. **Do not infer scope from directory prefix alone.** Root physical containment includes nested packages; root graph ownership does not automatically include every descendant package. Preserve dependency closure and legitimate SCC member ownership.

An exact package-path-to-node map plus a selected-node set is one bounded implementation option. It can be built from the graph already obtained by the pass. A missing mapping should remain unresolved rather than fall back to root. This is a design option, not a mandated new API or evidence of implementation.

## Costs and failure modes

Let `V` be graph nodes, `P` their total member paths, `G` grouped findings, and `F` individual findings.

- Current group construction already traverses findings, groups them and sorts the resulting work. Current owner attribution scans focused node paths for each group. A canonical exact package-owner map costs approximately `O(P)` additional construction and storage, then expected `O(1)` lookup per group; a selected-node set costs `O(V)` in the worst case. Canonicalization and string lengths still contribute. These are algorithmic estimates, not benchmarks.
- Keeping the full baseline leaves its tool execution, compiler/cache activity, report parsing, snapshot storage and retained evidence costs intact. Fixing candidate scope alone promises no reduction in baseline measurement cost.
- Suppressing an erroneous repair can avoid downstream prompt construction, model calls, tool actions, reviews, after-gates, witness runs and journal/compensation work. The avoided work is conditional on that repair otherwise being selected. No token, dollar, latency or quality improvement has been measured.
- Explicit outside/unresolved dispositions add state and reporting work. Keep compact identifiers and counts in routine context, with located evidence recoverable from the original snapshot; do not copy all outside findings into every task prompt.
- Overaggressive filtering can lose a genuinely selected dependency, a root-package finding or a collapsed component member. A raw exact-node-ID lookup alone is insufficient when the subject identifies a member path of a collapsed node.
- Graph/focus reuse can reintroduce stale ownership even after first-measurement correction. Error-free observations must remain bound to the graph against which their owner was determined.
- Unsupported or malformed package paths should remain unverified/unresolved for selection, with a reason. Silently dropping their baseline observations would weaken regression coverage.
- Limiting all measurement to the focus could hide regressions caused outside that focus. Treat measurement policy changes as a separate design decision and preserve the current baseline contract unless deliberately revised.

## Fair evaluation plan, all unrun

Use the same source, toolchain/build profile, graph, focus, snapshots and deterministic execution responses for current and proposed behavior. Record actual tool versions and command domains. Do not credit a candidate that “passes” only because it stopped measuring outside regressions.

1. **Pure projection witness.** Feed a complete vet snapshot containing one `lib/bad.go`/printf finding, full nodes `.` and `lib`, and selected root only through the real group-to-candidate path. The candidate must retain owner `lib` and be outside selection, not become root or disappear from the baseline.
2. **Real acquisition and driver.** In an authorized disposable fixture, acquire the graph and actual vet report through the normal CLI callback and run the real kernel/driver with an observation-only executor. Assert that no outside-group attempt is dispatched. Inspect the full candidate/focus facts, not only final pass count.
3. **Spelling control.** Move the independent package from `lib` to `internal/lib` with equivalent behavior. Both must be outside root focus for the same semantic reason; path depth must not determine eligibility.
4. **Root positive control.** Put a supported finding in a real root source file. Root focus must still offer that repair, and ordinary root-file attribution must remain valid.
5. **Dependency positive control.** Keep the diagnostic open through preceding ordinary vet/test attempts using an observation-only, non-mutating executor, or assert error-free candidacy and ownership rather than unconditional dispatch. Add a real root-to-lib dependency. Focus closure must include `lib`, and the group must be dispatched as `lib` work with its unchanged `./lib` witness.
6. **Full-sweep and focused-lib controls.** Apply the same observation-only or candidacy qualification, since ordinary gates may repair the finding first. Both should select the real `lib` owner normally. Check single- and multiple-file groups and preserve unused-code exclusions.
7. **Unavailable and incomplete observation controls.** Missing vet, report parse failure, cancellation and an unresolved package must not become affirmative selected work. Preserve unverified status and the original observation.
8. **Graph/focus transition controls.** Reuse a baseline across a changed graph or focus only with revalidated ownership; test SCC member mapping where supported. Coordinate this with the existing ownership-generation contract.
9. **Optional end-to-end repair.** Only after the deterministic boundary is fixed, use a scripted repair in an authorized fixture to verify correct candidate dispatch, campaign scope, witness, ratchet, journals and retained source. Live-model evaluation, if separately authorized, should compare task success and unintended edits as well as tokens and elapsed time.

Authored tests already cover error-free group selection/witness behavior and unused-check exclusions. The inspected selection fixture uses a full two-package sweep without root-only focus; the policy fixture supplies already-attributed findings. Those are valuable controls but do not exercise this producer's package-to-root conversion. Their source was read, not run. [Selection fixture][pick-test] [Policy fixture][policy-test]

Acceptance requires **zero out-of-focus group dispatches in the negative fixtures**, continued dispatch of legitimate root/dependency/full-sweep work, unchanged baseline regression information, and preserved handling of incomplete evidence. Lower context or runtime cost alone is not acceptance.

## Overlap and novelty boundary

The current audit inventory and queue, relevant architecture, source and authored fixtures were reviewed. Closest existing reports address different causal boundaries:

- [Finding ownership across graph generations][prior-graph] concerns retained owners after a topology change. This witness uses a fresh first baseline, correct fixed graph and no stale cache.
- [Root-crate diagnostic ownership][prior-rust] concerns missing Rust descendant-file ownership. Here a known Go package directory receives the wrong owner through a file fallback.
- [Mission error-free witness receipts][prior-receipt] concerns the evidence retained after a group repair is selected. This report fails before that selection should occur and requires no mission.
- [Mission acceptance scope][prior-acceptance] concerns the scope of an acceptance command relative to an already selected objective. Here the package-specific command stays correct for its subject; that subject should never have been assigned to the selected root.
- [Diagnostic subject identity][prior-identity] concerns distinct messages collapsed into one finding ID. This witness has one intact, correctly identified finding.
- [Checker runtime applicability][prior-runtime] concerns language filtering of project wrappers. This witness uses native Go vet over Go source.

The reusable principle that evidence, ownership, focus and authority are separate is not new. The distinct finding is the live first-measurement conversion of an out-of-focus single-segment package into a root repair through `refreshGates → attributeNode → open`.

## Sources

[pin]: https://github.com/theRebelliousNerd/codenerd/commit/055a8576f8b20d17143037578cffda5225e29515
[root]: https://github.com/theRebelliousNerd/codenerd/blob/055a8576f8b20d17143037578cffda5225e29515/agents.md#L1-L34
[architecture]: https://github.com/theRebelliousNerd/codenerd/blob/055a8576f8b20d17143037578cffda5225e29515/Docs/architecture/AGENTS.md#L1-L76
[flags]: https://github.com/theRebelliousNerd/codenerd/blob/055a8576f8b20d17143037578cffda5225e29515/cmd/nerd/cmd_campaign_recurse.go#L81-L91
[journey]: https://github.com/theRebelliousNerd/codenerd/blob/055a8576f8b20d17143037578cffda5225e29515/Docs/journeys/10-forever-loop.md#L180-L190
[filter]: https://github.com/theRebelliousNerd/codenerd/blob/055a8576f8b20d17143037578cffda5225e29515/internal/campaign/recurse_dag.go#L36-L76
[node-contract]: https://github.com/theRebelliousNerd/codenerd/blob/055a8576f8b20d17143037578cffda5225e29515/internal/campaign/recurse_dag.go#L16-L34
[go-graph]: https://github.com/theRebelliousNerd/codenerd/blob/055a8576f8b20d17143037578cffda5225e29515/internal/campaign/recurse_workspace.go#L392-L448
[closing]: https://github.com/theRebelliousNerd/codenerd/blob/055a8576f8b20d17143037578cffda5225e29515/internal/campaign/recurse_workspace.go#L237-L258
[pass]: https://github.com/theRebelliousNerd/codenerd/blob/055a8576f8b20d17143037578cffda5225e29515/internal/campaign/recurse_cycle.go#L514-L550
[visit-dispatch]: https://github.com/theRebelliousNerd/codenerd/blob/055a8576f8b20d17143037578cffda5225e29515/internal/campaign/recurse_cycle.go#L574-L603
[projection]: https://github.com/theRebelliousNerd/codenerd/blob/055a8576f8b20d17143037578cffda5225e29515/internal/campaign/recurse_error_free.go#L30-L70
[measurer]: https://github.com/theRebelliousNerd/codenerd/blob/055a8576f8b20d17143037578cffda5225e29515/internal/gates/errorfree.go#L114-L139
[groups]: https://github.com/theRebelliousNerd/codenerd/blob/055a8576f8b20d17143037578cffda5225e29515/internal/gates/errorfree_groups.go#L18-L80
[witness]: https://github.com/theRebelliousNerd/codenerd/blob/055a8576f8b20d17143037578cffda5225e29515/internal/gates/errorfree_groups.go#L85-L132
[attribution]: https://github.com/theRebelliousNerd/codenerd/blob/055a8576f8b20d17143037578cffda5225e29515/internal/campaign/recurse_cycle.go#L851-L916
[open]: https://github.com/theRebelliousNerd/codenerd/blob/055a8576f8b20d17143037578cffda5225e29515/internal/campaign/recurse_cycle.go#L1006-L1033
[facts]: https://github.com/theRebelliousNerd/codenerd/blob/055a8576f8b20d17143037578cffda5225e29515/internal/campaign/recurse_policy.go#L62-L96
[rank]: https://github.com/theRebelliousNerd/codenerd/blob/055a8576f8b20d17143037578cffda5225e29515/internal/core/defaults/policy/recurse.mg#L136-L186
[selection]: https://github.com/theRebelliousNerd/codenerd/blob/055a8576f8b20d17143037578cffda5225e29515/internal/core/defaults/policy/recurse.mg#L236-L255
[dispatch]: https://github.com/theRebelliousNerd/codenerd/blob/055a8576f8b20d17143037578cffda5225e29515/internal/campaign/recurse_cycle.go#L1151-L1181
[attempt-node]: https://github.com/theRebelliousNerd/codenerd/blob/055a8576f8b20d17143037578cffda5225e29515/internal/campaign/recurse_cycle.go#L1237-L1266
[group-spec]: https://github.com/theRebelliousNerd/codenerd/blob/055a8576f8b20d17143037578cffda5225e29515/internal/campaign/recurse_error_free.go#L81-L101
[cli]: https://github.com/theRebelliousNerd/codenerd/blob/055a8576f8b20d17143037578cffda5225e29515/cmd/nerd/cmd_campaign_recurse.go#L248-L263
[callback]: https://github.com/theRebelliousNerd/codenerd/blob/055a8576f8b20d17143037578cffda5225e29515/cmd/nerd/cmd_campaign_recurse.go#L348-L366
[cli-execute]: https://github.com/theRebelliousNerd/codenerd/blob/055a8576f8b20d17143037578cffda5225e29515/cmd/nerd/cmd_campaign_recurse.go#L225-L245
[chat]: https://github.com/theRebelliousNerd/codenerd/blob/055a8576f8b20d17143037578cffda5225e29515/cmd/nerd/chat/campaign_recurse.go
[campaign]: https://github.com/theRebelliousNerd/codenerd/blob/055a8576f8b20d17143037578cffda5225e29515/internal/campaign/recurse_attempt.go#L54-L142
[ordinary]: https://github.com/theRebelliousNerd/codenerd/blob/055a8576f8b20d17143037578cffda5225e29515/internal/campaign/recurse_cycle.go#L750-L802
[mission]: https://github.com/theRebelliousNerd/codenerd/blob/055a8576f8b20d17143037578cffda5225e29515/internal/campaign/recurse_mission.go#L174-L233
[witness-run]: https://github.com/theRebelliousNerd/codenerd/blob/055a8576f8b20d17143037578cffda5225e29515/internal/campaign/recurse_cycle.go#L1400-L1445
[pick-test]: https://github.com/theRebelliousNerd/codenerd/blob/055a8576f8b20d17143037578cffda5225e29515/internal/campaign/recurse_error_free_pick_test.go#L26-L110
[policy-test]: https://github.com/theRebelliousNerd/codenerd/blob/055a8576f8b20d17143037578cffda5225e29515/internal/campaign/recurse_error_free_policy_test.go#L9-L54
[prior-graph]: https://github.com/theRebelliousNerd/codenerd/blob/055a8576f8b20d17143037578cffda5225e29515/Docs/audits/recurse-finding-owner-graph-transition.md
[prior-rust]: https://github.com/theRebelliousNerd/codenerd/blob/055a8576f8b20d17143037578cffda5225e29515/Docs/audits/recurse-root-crate-diagnostic-ownership.md
[prior-receipt]: https://github.com/theRebelliousNerd/codenerd/blob/055a8576f8b20d17143037578cffda5225e29515/Docs/audits/mission-errorfree-witness-receipts.md
[prior-acceptance]: https://github.com/theRebelliousNerd/codenerd/blob/055a8576f8b20d17143037578cffda5225e29515/Docs/audits/recurse-mission-acceptance-scope.md
[prior-identity]: https://github.com/theRebelliousNerd/codenerd/blob/055a8576f8b20d17143037578cffda5225e29515/Docs/audits/recurse-diagnostic-subject-identity.md
[prior-runtime]: https://github.com/theRebelliousNerd/codenerd/blob/055a8576f8b20d17143037578cffda5225e29515/Docs/audits/recurse-checker-runtime-subject-applicability.md
[go-packages]: https://pkg.go.dev/cmd/go#hdr-Package_lists_and_patterns
[go-vet]: https://pkg.go.dev/cmd/vet
[saltzer]: https://web.mit.edu/saltzer/www/publications/protection/Basic.html


[reader]: https://github.com/theRebelliousNerd/codenerd/blob/055a8576f8b20d17143037578cffda5225e29515/internal/gates/errorfree.go#L261-L269
[parser]: https://github.com/theRebelliousNerd/codenerd/blob/055a8576f8b20d17143037578cffda5225e29515/internal/lintreport/lintreport.go#L61-L145
