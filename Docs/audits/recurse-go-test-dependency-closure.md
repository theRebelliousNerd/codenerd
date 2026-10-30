---
doc-class: research-proposal
subsystem: campaign-recurse-planning
implementation-status: proposed
last-verified: 2026-10-30
verified-against: 6cbe3962fbdb24a1639b26e71f8096e4f4330048
verification-method: static-source-inspection
---

# Recurse dependency closure omits Go test imports

## Decision

Include the Go test-import relationships that the recurse node's test gate actually depends on when deriving its dependency closure and sweep order. The current Go package projection keeps only `Imports`, dropping `TestImports` and `XTestImports` from the existing `go list -e -json ./...` response. A package imported exclusively by another package's tests is therefore a workspace node but not a dependency of that importer. A focused sweep can omit it, and an unfocused sweep can visit the importer first. [Go projection][projection], [focus closure][filter], [ordering policy][ordering].

This is a **planning and work-selection mismatch, not false-green test evidence**. Go still compiles the test dependencies when the selected node's tests run. Workspace gates, session verification and the recurse ratchet remain capable of detecting failures. The missing graph edge means the planner does not deliberately visit the corresponding prerequisite first, not that the prerequisite becomes invisible to every verifier. [Detected gates][gates], [Go test semantics][go-test], [ratchet][ratchet].

The narrow repair can consume fields already returned by the current Go listing without adding a subprocess or model call. It must preserve workspace containment, build configuration, deterministic policy ordering and cycle handling. Adding test edges can enlarge a focused sweep and form valid test-dependency cycles, so scope, identity and cost changes must be explicit rather than treated as a free optimization.

**Evidence boundary:** this is static source research with an independently challenged conditional witness. No CodeNERD application, Go command, test, benchmark, model inference or failure injection was executed. Production frequency, token savings and task-quality effects are unmeasured.

## Source freeze and novelty

Initial inspection used `388205138ba6d47c1cff0c5e97c896dfa642a391`; revalidation at approximately 17:18 UTC on 2026-10-30 resolved main to [`6cbe3962fbdb24a1639b26e71f8096e4f4330048`][main]. Root and campaign guidance were reread and are unchanged. The audit inventory was screened through [`research-main-20261030-1651`, `90c84648725aa263c8a1ba2458390b572402e4de`][research]. Its recent completion-storage, generated-test-authority, knowledge-synthesis and prior-reason-budget reports address different producer/consumer boundaries. The three intervening main commits were inspected: they repair restart/policy-reload behavior and recurse gate contracts/subsumption, then rebuild the predicate corpus. The Go graph producer, focus helper, graph validator and recurse ordering policy are byte-identical to the initial pin, so the missing test-import edge remains. Implementation links below now name the revalidated head; changed cycle/attempt anchors were refreshed. This is source revalidation, not an executed result. [Main changes][main-changes], [root guidance][guidance], [campaign guidance][campaign-guidance].

The earlier [test-gate memoization audit][memo-prior] explicitly recognizes that importer measurements include production, same-package-test and external-test imports. That is relevant prior art, not a defect being reasserted: the current session importer discovery already reads all three lists. This report concerns the separate recurse DAG producer and its dependency-closure consumer, not memo-key identity or missing importer verification. [Implemented importer discovery][importers].

The [recurse gate contract][gate-prior] and [gate subsumption][subsumption-prior] repairs are now landed in `5efbc6782`. The current source copies a measured gate's environment, argv and passing exits into a private workspace-bound runtime contract; restored recurse checks without that contract refuse execution. It also removes kind/language-only suppression, forwarding every measured workspace finding attributed to a visited node. These are real implemented protections to preserve. They do not add a missing graph edge or create a visit for a node excluded by focus. The [gate-scoped recurse addendum][gate-addendum] remains historical context; this proposal does not reopen those repaired findings. [Runtime contract guard][runtime-contract], [current candidate admission][current-open].

The entire `internal/campaign/recurse_workspace.go` file was byte-identical to main on all five inspected active code/QA heads. This is a file comparison, not whole-branch equivalence:

| Inspected branch | Head |
|---|---|
| `jules-orchestrator-acceptance-qa-12283394416863483039` | `51cfd42649552866d4aa54c53b68acf5740eb09b` |
| `lane/worktree-agent-a70bb621a19639eb0` | `a53cfc308e8836c72370adaa46deba3f7b00180d` |
| `lane/worktree-agent-a893b7e3344915557` | `80af1863d3a3aa46a722ac8b0c8a0985582d4995` |
| `lane/worktree-agent-acd8bb412e996d54c` | `aa7988dec4c62116656ea722ff73b687a8509908` |
| `qa-cartographer-boundary-analysis-16707629379015549107` | `767a3291d6287b238417f2466592a9002810d05a` |

The primary Go references are separately pinned to the official `go1.26.0` tag's commit, [`d90b98e65320778f3b1f99a6951ab20f04d218b3`][go-pin]. They establish command semantics; they are not evidence that a particular local toolchain was run.

## What the contract already promises

A `SubsystemNode` represents a slice of the tree and the nodes whose soundness it assumes. `DependsOn` lists nodes that must sweep first. `FilterDAG` retains requested subsystems plus their transitive dependencies, specifically so a focused sweep visits prerequisites before dependents. The CLI's `--subsystem` description likewise says dependencies are pulled in. [Node and closure contract][filter], [CLI][cli].

The Go graph is derived from the workspace rather than a hard-coded package table. The actual driver re-derives that graph each pass, validates it before applying focus, publishes its graph facts, and lets the kernel choose the next ready node. The planner does not run a separate Go-side substitute ordering algorithm when policy cannot order the graph. [Workspace derivation][derive], [live pass][pass], [graph validation][graph-check], [kernel ordering][ordering].

The current default Go node work includes `go vet {pkg}` and `go test -count=1 -cover {pkg}`; the workspace also gets a build gate. Project-declared commands can replace detected kinds, so the concrete witness assumes ordinary detected Go gates remain available. [Detection and overrides][gates].

Those facts make test-import relationships relevant to the node's actual work. The stronger alternative interpretation—“this is intentionally a production-import-only visualization”—does not explain the focused repair contract or the gates consuming that focus. If production-only topology is desired for architectural display, it should be distinguished from the prerequisite closure for test-bearing work.

## Producer omission

The official Go listing schema separates:

- `Imports`: the package's ordinary imports;
- `TestImports`: imports from same-package test files;
- `XTestImports`: imports from external-package test files.

The listing implementation resolves the latter two lists for its output. These fields are available without switching to `go list -test`, which has a different purpose: it adds test binaries and test-specific package variants to the reported package set. [Go listing fields][go-fields], [listing resolution][go-resolution], [test variant flag][go-test-flag].

CodeNERD currently decodes only:

```text
Dir
ImportPath
Imports
```

It first creates nodes and the import-path-to-directory lookup for listed packages within the root. It then adds edges exclusively by iterating `p.Imports`. JSON members omitted from this struct cannot donate graph edges later; no subsequent pass fills test imports. The listing already uses `build.GoInvocation`, so this is not a claim that recurse ignores the configured Go environment or build flags. [Complete Go producer][projection].

The existing containment and lookup are important safeguards: imported third-party packages do not become workspace sweep nodes merely because their names appear in a test. The proposed repair should join the added import lists to that same admitted local package map, not blindly add every test import as a task.

## Deterministic conditional witness

Use a normal module with the following structure and no relevant excluded build tags:

```text
go.mod                module example.com/m
a/a.go                package a; no import of z
a/a_test.go           package a_test; imports testing and example.com/m/z
z/z.go                package z; exports Value
```

The test in `a/a_test.go` calls `z.Value`. Both directories have ordinary Go source and are returned in the successful listing. Neither package imports the other in production, and no other source creates an indirect production-import path from `a` to `z`.

The normal listing represents `example.com/m/z` under `a.XTestImports`, not `a.Imports`. Therefore:

1. The graph contains both `a` and `z`, but no `a → z` dependency.
2. A focus on `a` keeps `a` alone; `z` is not in its recorded transitive closure.
3. In the unfocused graph, both ordinary nodes are initially ready. The existing lexical tie-break selects `a` before `z`, although `a`'s test binary consumes `z`.
4. The `a` visit still runs its Go test gate, which compiles the external test package and its dependency. No test failure has been suppressed merely by omitting the edge.

This trace is derived from the decoder, closure and policy, not from a run. A same-package `a/a_test.go` importing `z` produces the analogous `TestImports` witness. An ordinary production import of `z` is a control: that edge is already handled and the omission does not occur. [Projection][projection], [closure][filter], [lexical ready-node rule][ordering], [Go test compilation][go-test].

For a practical repair variant, let `z` contain a defect exercised by both its own tests and `a`'s external test. The focused sweep can diagnose a failure while omitting the dependency's own visit. In an unfocused sweep it can spend work on the dependent before the dependency has been repaired. The extra attempts, model behavior and eventual result are conditional—not deterministic counts and not a claim of inevitable nontermination.

## Downstream scope and attribution

There are two materially different cases.

**Ordinary recurse can sometimes compensate.** A node-scoped measurement assigns its returned findings to the node whose gate ran, even when a diagnostic concerns a dependency. The attempt then receives that node's task description, acceptance command and initial write set. However, the ordinary campaign write guard can acquire a lease for an undeclared path; it is not a universal prohibition on all writes outside the initial write set. A worker may identify and repair `z` opportunistically, subject to normal policy, leases, validation and observed outcomes. This is a significant limit on the impact claim. [Measurement attribution][measure], [attempt construction][attempt], [campaign work declaration][campaign-build], [write lease guard][write-guard].

**Mission scope is stricter.** A mission receipt copies the current node's paths as its scope. The mission write check resolves a candidate path, measures whether it is inside that scope and whether it is protected, and asks policy for admission. A direct `z/z.go` repair from a receipt scoped only to `a` is outside scope and cannot be admitted by that rule. This does not justify bypassing the scope guard; it supports scheduling the real dependency as its own authorized node. Mission design, snapshot, budget and protection checks must also admit the attempt, so no mission is assumed to reach execution automatically. [Receipt scope][receipt], [mission write check][mission-scope], [mission policy][mission-policy].

Workspace measurements and unattributed-finding reporting are further counterevidence against any “silent all-green” description. A workspace-wide build failure in an omitted package can still be measured and reported; its target can be unattributed in the narrowed node set rather than visited. The newly repaired `open` admits every workspace finding attributed to the selected node rather than suppressing it by kind/language. It does not admit an omitted node into the sweep. Actual gate failures remain failures. The current per-node test and session importer gates may still expose the same defect independently. [Workspace measurement and attribution][measure], [unattributed reporting][unattributed], [session importer discovery][importers].

Finally, planning validates the graph it actually receives. A missing semantic edge does not create a structural duplicate, unknown endpoint or cycle for that validation to reject. This is different from the reserved synthetic-node identity collision: the witness uses unambiguous ordinary directory names and a structurally valid graph.

## Existing protections and tests

Preserve all of the following:

- Canonical root handling and exclusion of package directories outside it.
- Shared Go invocation configuration and failure propagation from the listing or JSON decoder.
- Joining only to known local packages; dropping self-edges and non-workspace edges.
- Strongly connected component collapse and policy-authoritative deterministic ordering.
- Whole-graph soundness checking before focus, and re-derivation on later passes.
- Unknown-subsystem refusal, finite-pass/stop/cancellation controls, and mission limits.
- Actual package test execution, observed task verdicts, write leases, mission scope and the keep/revert ratchet.

The inspected Go workspace fixture already verifies production-import leaf ordering, disconnected local nodes and exclusion of fixture/dependency directories. The common helper compares the kernel order against its topological oracle. Python-cycle coverage also exercises component collapse. These are useful controls, but a topology oracle given the same incomplete graph cannot establish that every Go test edge was acquired. The inspected Go fixture does not include the test-only dependency witness above. Tests were read, not run; this is not an exhaustive claim about every fixture in the repository. [Workspace tests][tests], [graph construction and collapse][collapse].

## Repair shape and tradeoffs

1. Extend the decoded Go dependency projection to retain same-package and external-test imports from the existing listing. Resolve/deduplicate relationships through the same local package lookup, preserving ordinary production imports.
2. Define whether the planner stores edge kind separately or derives a combined prerequisite relation for test-bearing work. The kernel should continue to own ordering and focus decisions; this does not call for a second scheduler.
3. Keep the existing invocation unless a proven requirement calls for `-test`. Blindly adding that flag introduces synthetic package identities and requires separate normalization. The missing lists already exist in the current output.
4. Handle valid cycles formed by test edges explicitly. For example, package A may depend on B in production while B's external tests import A. Such a package-directory relation can be cyclic even though the Go test program is legitimate. Existing SCC collapse is a possible mechanism, not a reason to reject the repository as having a production import cycle.
5. Review the identity and authority effects of any new SCC. Merged component IDs, multiple `Paths`, focused subsystem naming, journal/resume keys and mission scope can change. Display the actual admitted closure and do not silently widen a receipt already issued for an earlier graph.
6. Update the package contract and focused-run tests before declaring the behavior repaired. Do not disable dependency test execution or weaken the scope guard to avoid the mismatch.

The current session importer reader provides a local precedent for the three-list vocabulary. It is not a drop-in graph implementation: its goal is one-hop reverse importer verification, whereas recurse needs dependency edges, closure, ordering and component identities. [Session implementation][importers].

## Complete cost model

The missing fields are already present in the JSON bytes returned by `go list`; retaining them need not add another process, filesystem traversal or model request. The direct incremental work is decoding/storing their strings, joining and deduplicating test edges, and carrying those edges through graph facts and ordering. An edge-count model should count distinct local test relationships, not all standard-library imports as new tasks.

The larger cost lies downstream. A formerly narrow focus may gain several legitimate dependencies. Those nodes can add gate runs, context retrieval, deliberation in mission mode and worker attempts. Test edges can also merge SCCs, reducing node count while increasing each node's path scope and gate work. There is no justified claim that the repair is always faster, cheaper or smaller.

The proposed benefit is fewer misordered or missing prerequisite visits and less dependent work before its inputs are sound. Go's existing build/test caching can already reduce repeated compilation, while CodeNERD uses a noncached test-result invocation for its detected node test gate. Neither fact provides a token-saving number. Measure all attempts, gate work and model usage, including the newly admitted prerequisites, rather than reporting only avoided failed calls.

## Fair evaluation plan not yet run

### Producer and policy checks

- Construct a real temporary module for the external-test-only witness and a separate same-package-test-only witness. Exercise the actual Go listing, `DeriveWorkspaceDAG`, `RecurseSweepOrder` and live focus selection, not only a canned struct.
- Assert the local test-only edge, dependency-first order and inclusion of `z` in an `a` focus. Keep an ordinary-production-import control and a genuinely unrelated package that remains outside the focus.
- Check standard-library and external-module imports, excluded build-tag tests, configured build flags, duplicate imports and self-import relationships. Verify the effective selected build domain rather than assuming every test file is active.
- Exercise a valid external-test dependency cycle. Verify a deliberate SCC/edge-kind policy, stable visible identities, correct paths, focus behavior and resume handling rather than demanding a DAG of raw package names.

### Production consequence checks

- Run the real recurse driver with deterministic task execution and ordinary detected gates. Record node visitation and the exact scoped test command. Confirm that planning changes do not alter the truth of a failing test or mint a successful task verdict.
- Add the dependency-defect variant and capture both ordinary and mission behavior. Ordinary undeclared-path leasing remains an allowed potential rescue; mission outside-scope writes remain denied. The correct repair schedules an admitted dependency visit, not a broader tool permission.
- Retain malformed-listing, unknown-focus, graph-soundness, cancellation, stop and bounded-mission controls. A failed dependency acquisition must not be reported as a complete measured closure.

### End-to-end quality and efficiency

In a separately authorized paired live evaluation, fix workspace revisions, prompts, model/slot, build environment, policies, budgets and retrieval settings. Include both test-only-dependent repairs and repositories where the extra prerequisites are already healthy. Measure useful completed repairs, scope correctness, unnecessary edits, kept/reverted outcomes, attempts before dependency repair, total gate processes and CPU time, model calls, total input/output and cached/uncached tokens, context retrieval work and elapsed time.

Keep world/CodeDOM context and ordinary safety gates enabled. Report the extra admitted work as well as any avoided retries. Do not call a smaller token total an improvement if the task lost dependency coverage, and do not claim a universal speedup from the deterministic graph correction.

## Bottom line

The verifier can still see Go test dependencies that the recurse planner omits. Conserving those existing listing fields in the dependency-closure contract aligns focused work and ordering with the tests the nodes actually run. The fix should preserve current verification and authority checks, not replace them.

[main]: https://github.com/theRebelliousNerd/codenerd/tree/6cbe3962fbdb24a1639b26e71f8096e4f4330048
[research]: https://github.com/theRebelliousNerd/codenerd/tree/90c84648725aa263c8a1ba2458390b572402e4de/Docs/audits
[guidance]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/agents.md
[campaign-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/campaign/agents.md
[projection]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/campaign/recurse_workspace.go#L338-L405
[derive]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/campaign/recurse_workspace.go#L49-L82
[collapse]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/campaign/recurse_workspace.go#L115-L226
[filter]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/campaign/recurse_dag.go#L16-L77
[graph-check]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/campaign/recurse_plan.go#L38-L150
[ordering]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/core/defaults/policy/recurse.mg#L405-L485
[pass]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/campaign/recurse_cycle.go#L459-L550
[gates]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/gates/detect.go#L15-L133
[cli]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/cmd/nerd/cmd_campaign_recurse.go#L81-L91
[measure]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/campaign/recurse_cycle.go#L560-L690
[unattributed]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/campaign/recurse_cycle.go#L693-L736
[attempt]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/campaign/recurse_cycle.go#L905-L933
[campaign-build]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/campaign/recurse_attempt.go#L94-L138
[write-guard]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/campaign/orchestrator_attempt_writes.go#L137-L156
[receipt]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/campaign/recurse_mission.go#L183-L219
[mission-scope]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/campaign/recurse_mission_scope.go#L13-L82
[mission-policy]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/core/defaults/policy/mission.mg#L8-L33
[ratchet]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/core/defaults/policy/recurse.mg#L301-L329
[importers]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/session/importer_packages.go#L31-L73
[tests]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/campaign/recurse_workspace_test.go#L28-L203
[memo-prior]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/Docs/audits/test-gate-memoization-identity.md
[gate-prior]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/Docs/audits/recurse-gate-contract-continuity.md
[subsumption-prior]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/Docs/audits/recurse-gate-subsumption.md
[gate-addendum]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/Docs/audits/gate-scoped-recurse-current-main-addendum.md
[go-pin]: https://github.com/golang/go/commit/d90b98e65320778f3b1f99a6951ab20f04d218b3
[go-fields]: https://github.com/golang/go/blob/d90b98e65320778f3b1f99a6951ab20f04d218b3/src/cmd/go/internal/list/list.go#L116-L133
[go-resolution]: https://github.com/golang/go/blob/d90b98e65320778f3b1f99a6951ab20f04d218b3/src/cmd/go/internal/list/list.go#L743-L750
[go-test-flag]: https://github.com/golang/go/blob/d90b98e65320778f3b1f99a6951ab20f04d218b3/src/cmd/go/internal/list/list.go#L204-L214
[go-test]: https://github.com/golang/go/blob/d90b98e65320778f3b1f99a6951ab20f04d218b3/src/cmd/go/internal/test/test.go#L64-L85
[main-changes]: https://github.com/theRebelliousNerd/codenerd/compare/388205138ba6d47c1cff0c5e97c896dfa642a391...6cbe3962fbdb24a1639b26e71f8096e4f4330048
[runtime-contract]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/campaign/acceptance_runtime.go#L11-L45
[current-open]: https://github.com/theRebelliousNerd/codenerd/blob/6cbe3962fbdb24a1639b26e71f8096e4f4330048/internal/campaign/recurse_cycle.go#L762-L786

