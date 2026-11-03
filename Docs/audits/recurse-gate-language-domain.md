---
doc-class: deep-dive
subsystem: campaign
implementation-status: proposed
last-verified: 2026-11-03
verified-against: 7ad7edf3ca82fc9c211b2173af41bfea9f7636c6
verification-method: static-source-inspection
supersedes: []
---

# Recurse gate dispatch loses each path's language domain

## Decision and evidence boundary

Preserve the workspace-derived DAG and its collapsed import cycles, but carry the language membership of each constituent path into gate dispatch. The current graph stores a collapsed node's paths and the union of their languages separately. A gate admitted for any language on that node then runs on every path, including directories that do not belong to its toolchain. A second consumer, focused workspace-Go scoping, converts every selected node path into a Go package pattern without consulting language membership at all.

This is a concrete portability integration gap, not a request for another scheduler or a claim that the whole polyglot architecture is missing. In a valid mixed Go/Python project, an ordinary Python import cycle can make recurse run `go vet` and `go test` on a Python-only directory. Independently, focusing a Python-only node can rewrite a valid workspace `go build ./...` into `go build ./pycore`. These are source-derived conditional outcomes. No tests, builds, repository programs, models, gate executions, or reproductions were run. No bad retained commit or measured performance loss is claimed.

The source freeze is public main [`7ad7edf3ca82fc9c211b2173af41bfea9f7636c6`](https://github.com/theRebelliousNerd/codenerd/commit/7ad7edf3ca82fc9c211b2173af41bfea9f7636c6), checked again during this review. This report concerns the ordinary recurse `gates.Set` route. It does not depend on the optional error-free `MeasureGates` snapshot/overlay mechanism.

## The producer has the relation, then discards it

`DeriveWorkspaceDAG` accumulates multiple language scanners in one graph. `workspaceGraph` keys nodes by directory, and `addNode(dir, lang)` merges languages onto that directory. The Go scanner adds actual Go package directories, while the Python scanner adds directories containing Python sources and import edges. A directory can legitimately contain both. [Graph entry][graph] [Directory-language producer][producer] [Go producer][go-producer] [Python producer][python-producer]

The strongly connected component pass then builds a `SubsystemNode` containing:

- `Paths`: every constituent directory.
- `Languages`: the union of the languages found anywhere in the component.

It does not retain which member directory supplied each language. The exported node type likewise has no path-to-language relation. The union is suitable for a display label or for asking whether a node includes a language, but not for deciding that every member is a package of that language. [Component projection][collapse] [Node contract][node]

This does not require an invalid Go import cycle. The cycle can be entirely Python, with Go source merely sharing one of its directories.

## Witness A: a Python cycle sends Go checks to a Python-only directory

Use a clean, admitted repository with an available Go toolchain and the following source layout. The file bodies are an illustrative proposed fixture, not executed code:

- `go.mod`: an ordinary local module, such as `module example.com/poly`.
- `shared/bridge.go`: a valid non-main Go package, `package shared`.
- `shared/__init__.py`: empty.
- `shared/bridge.py`: `import pycore`.
- `pycore/__init__.py`: `import shared.bridge`.
- No Go files in `pycore`.
- No relevant ignored, forbidden, hidden, nested-module or generated directories, and no custom declaration replacing the detected Go gates.

The Python imports need no model inference or dynamic resolution: both use the scanner's supported absolute-module syntax. They give `shared -> pycore` and `pycore -> shared`. Go contributes a package at `shared`. The resulting component is `pycore+shared`, with paths `[pycore, shared]` and languages `[go, python]`. The Python marker file is not required to demonstrate the Go misdispatch: graph scanning reads admitted Python sources independently of Python gate detection. A root `pyproject.toml` and actual Python tests can be added as a control. [Scanners][graph] [Python resolution][python-resolution] [Collapse][collapse]

The ordinary production route is:

1. Go detection supplies workspace `go:build`, node `go:vet`, and node `go:test`. The latter two use `{pkg}`. [Detection][detect]
2. `nodeGates` asks `Gate.AppliesTo(node.Languages)`. The collapsed union includes `go`, so both node Go gates are admitted. This language check is real but only node-wide. [Node selection][node-gates] [Applicability][applies]
3. `measure` reads every `n.Paths` value and invokes `gates.Run` for each, with no per-path language filter. Thus the Go gate runs both on `./pycore` and on `./shared`. `Gate.ForNode` faithfully expands each path as a package pattern; the runner faithfully executes the resulting command. [Dispatch][measure] [Expansion][expand] [Runner][runner]
4. A Python-only directory is not a Go package. The explicit `./pycore` request produces a package-selection error, conventionally “no Go files,” rather than testing Python. This is different from a workspace wildcard that discovers its actual Go packages. Go's official command documentation defines explicit directory patterns as the package contained in that directory. [Go package semantics][go-docs]
5. A completed nonzero process is recorded as failure, not an unavailable tool. If its output contains no recognized located diagnostic, `FindingsAll` preserves a fallback finding whose target is the invocation's node path. `measure` then sets that finding's node to `pycore+shared`; the failure participates in the combined gate result. [Runner][runner] [Fallback finding][fallback] [Attribution and combination][measure]
6. `open` forwards those node-gate findings. The normal visit asserts them for the kernel's selection policy. If selected under the ordinary eligibility/stall rules, `sourceOf` correctly preserves the particular invocation and `attempt` passes its `go ... ./pycore` argv to the attempt. The invocation-identity repair therefore faithfully carries the wrong-domain check; it does not prevent the dispatch defect. [Open/visit][visit] [Invocation lookup][source] [Attempt][attempt]

The host has manufactured a repair obligation by applying a tool to the wrong domain. This can spend attempts on an intrinsically inappropriate check and leave misleading open/stalled findings. It does not follow that any model will respond by adding dummy Go files, or that such a change would survive all guards; those stronger claims are unnecessary.

## Witness B: focused Go workspace checks use every selected language

A simpler fixture needs no cycle: a valid root Go module with `shared/bridge.go` plus a Python-only `pycore` directory, with no import edge between them. Request a focused sweep of `pycore`.

`FilterDAG` retains the chosen subsystem and its real dependencies. With a named focus and no synthetic cross-cutting node, `isFocusedSweep` is true. The full discovered gate set is still measured, including detected workspace `go:build`. [Focus construction][pass] [Focused predicate][focused]

`scopeGateToNodes` does check that the *gate* has language `go`, workspace scope, and a focused sweep. But `goPackagePatterns` only excludes cross-cutting nodes; it never checks `n.Languages`. It converts `pycore` into `./pycore`, and the caller replaces the original `./...` argument. Even a correct node-level language check would not fully repair witness A, because a collapsed component still contains heterogeneous paths. [Focused rewrite][focused]

This focused workspace invocation has no node. For a bare “no Go files” error with no recognized located diagnostic, the fallback target remains `<unattributed>`; attribution preserves that state and the summary reports it. It is therefore incorrect to claim that witness B alone necessarily sends a repair to the Python node. Its established consequence is a false workspace-gate failure and diagnostic pollution from an invalid rewritten domain. Witness A establishes the node-owned repair route. [Fallback][fallback] [Unattributed safeguard][unattributed]

The same focused transformation can affect a declared Go workspace command containing the exact `./...` token, including test or lint commands. It does not rewrite other selectors, non-Go gates, unfocused passes, or a focused selection containing a cross-cutting node.

## Existing protections and why they do not close this seam

- The graph is validated and the kernel orders the collapsed DAG. The defect is not a lost cycle, duplicate ID or Go scheduling decision replacing Mangle.
- Node-scoped language filtering works for a single-language, single-path node. It also works for a heterogeneous component if every path genuinely belongs to the selected gate's language.
- The runner preserves argv, environment, declared successful exits, cancellation and process failure. No pass is fabricated here.
- The complete finding census, per-finding invocation map, no-tests checks, metrics, forbidden-write rules and mission ownership checks remain useful. None reconstructs the lost path-language relation before launching the command.
- Unlocated workspace findings are reported rather than assigned to an arbitrary source node. This contains witness B's remediation consequence.
- A language-neutral owner-declared checker can intentionally accept every component path. Do not infer its domain merely from file extensions or remove it because the runtime does not recognize its executable.
- Keeping a change still requires target/improvement success and the independent ratchet guards. A synthetic red finding is not itself proof of an incorrectly accepted change. [Ratchet][ratchet]

## Bounded nonduplication screen

Read root/scoped guidance, the audit queue, the portable recurse proposal, and the relevant existing recurse and verification reports before treating this as new.

- `portable-recurse-deliberation-crucible` already proposes capability manifests and honest graph/tooling limits. This report supplies a narrower, currently reachable dispatch defect: recognized language membership is available at the producer but erased before consumers use it.
- `recurse-collapsed-finding-invocation-identity` concerns a selected finding being paired with another member's argv. The current `byFinding`/invocation lookup fixes that association. Here the correctly associated argv was never applicable to its path.
- `recurse-node-namespace-identity` concerns source/stage and joined-ID collisions. These fixtures use ordinary nonreserved paths and distinct component IDs.
- `recurse-go-test-dependency-closure` and `recurse-cargo-inherited-dependency-closure` concern missing dependency edges. The proposed fixture uses supported, correctly discovered Python edges.
- `gate-scoped-recurse-current-main-addendum`, `gate-comparison-domain-admission` and `scoped-gate-overlay-evidence-conservation` concern optional error-free measurement domains, comparison and overlay. Both witnesses work with `MeasureGates == nil`, before those transformations.
- `explicit-package-selector-fidelity` concerns erasing an explicit dot selector in the verification-tool backend. Here recurse itself invents explicit Go selectors from non-Go paths before the ordinary gate runner.

This is a bounded prior-art screen, not a claim that every branch or repository sentence was examined. Existing Go-only, Python-only and Python-cycle fixtures were read; they establish the building blocks without exercising this mixed-member dispatch boundary. [Existing graph fixtures][fixtures]

## Proposed contract

Keep scheduling identity separate from invocation domain. Preserve an immutable per-member language/capability relation when collapsing a component, or retain an equivalent adapter-produced package-domain descriptor that can answer which paths a gate applies to.

1. For node-scoped toolchain gates, select the component's applicable members, then produce one invocation descriptor per selected member. Keep the full component for dependency order, permissible task scope and reporting.
2. For focused Go workspace gates, derive patterns only from admitted Go package members in the focused closure, using the same build profile as discovery. Do not merely filter by component-level language, which leaves witness A.
3. Define zero applicable members explicitly. Preserve the original workspace-wide check if its contract requires it, or record a not-applicable/empty-domain disposition if narrowed execution is authorized. Do not silently convert no applicable execution into evidence that a required check passed; do not drop arguments and accidentally execute the root package.
4. Preserve owner-declared gate semantics. Unknown executables remain generic unless an explicit adapter/domain contract says otherwise. Keep exact environment, exit-code and invocation identity.
5. The host should produce faithful membership and invocation facts; Mangle should continue to decide work eligibility and retention. A domain mismatch is a host verification-contract problem, not a new product bug to ask the creative agent to repair.

A minimal fix that filters `goPackagePatterns` by `n.Languages` addresses the simple focused witness but is insufficient for collapsed heterogeneous nodes. An untyped “skip no Go files” output rule is worse: it hides real package-selection problems after execution and weakens evidence fidelity.

## Evaluation and cost plan, all unrun

First add deterministic producer-to-consumer fixtures:

- The Go/Python two-directory component above, through actual DAG derivation, node-gate selection, argv construction and finding admission. Go checks must target only `shared`; Python checks can target both Python members.
- An unfocused control with independent Go and Python nodes.
- A component where both directories contain Go and Python; both Go members must still be checked.
- Focus a Python-only node in a mixed repository, focus a Go-only node, focus both, and focus a mixed component. Assert explicit zero-domain policy and exact command descriptors.
- Generic declared node check over all members; preserve its original behavior.
- Existing valid Go failure in `shared`; it must still enter the repair queue with the correct invocation. Cancellation and unavailable tools remain unverified.
- Full production recurse route with optional error-free gates off, then on as a composition control; CLI/TUI use the same driver. Check that synthetic findings disappear without suppressing genuine findings or claiming unchecked success.

These are suggested future authorized checks, not commands run by this review. Measure command count, tool wall time/CPU, graph/domain memory, membership lookup cost, diagnostics, retries, model tokens, completed valid repairs and total cost per verified outcome. Charge any added discovery work and repeated graph refresh. A per-member relation is linear in the admitted member-language incidences and ordinarily reuses information already produced; that is an implementation expectation, not a measured speedup.

Acceptance means that adding Python source/import structure cannot turn a Python-only directory into a Go package obligation, while real Go checks, generic declared checks, dependency order and all existing safety/ratchet guards remain intact.

## Pinned references

[graph]: https://github.com/theRebelliousNerd/codenerd/blob/7ad7edf3ca82fc9c211b2173af41bfea9f7636c6/internal/campaign/recurse_workspace.go#L59-L100
[producer]: https://github.com/theRebelliousNerd/codenerd/blob/7ad7edf3ca82fc9c211b2173af41bfea9f7636c6/internal/campaign/recurse_workspace.go#L103-L145
[go-producer]: https://github.com/theRebelliousNerd/codenerd/blob/7ad7edf3ca82fc9c211b2173af41bfea9f7636c6/internal/campaign/recurse_workspace.go#L393-L449
[python-producer]: https://github.com/theRebelliousNerd/codenerd/blob/7ad7edf3ca82fc9c211b2173af41bfea9f7636c6/internal/campaign/recurse_workspace.go#L460-L481
[python-resolution]: https://github.com/theRebelliousNerd/codenerd/blob/7ad7edf3ca82fc9c211b2173af41bfea9f7636c6/internal/campaign/recurse_workspace.go#L490-L549
[collapse]: https://github.com/theRebelliousNerd/codenerd/blob/7ad7edf3ca82fc9c211b2173af41bfea9f7636c6/internal/campaign/recurse_workspace.go#L199-L235
[node]: https://github.com/theRebelliousNerd/codenerd/blob/7ad7edf3ca82fc9c211b2173af41bfea9f7636c6/internal/campaign/recurse_dag.go#L17-L33
[detect]: https://github.com/theRebelliousNerd/codenerd/blob/7ad7edf3ca82fc9c211b2173af41bfea9f7636c6/internal/gates/detect.go#L123-L141
[node-gates]: https://github.com/theRebelliousNerd/codenerd/blob/7ad7edf3ca82fc9c211b2173af41bfea9f7636c6/internal/campaign/recurse_cycle.go#L993-L1004
[applies]: https://github.com/theRebelliousNerd/codenerd/blob/7ad7edf3ca82fc9c211b2173af41bfea9f7636c6/internal/gates/gates.go#L114-L127
[measure]: https://github.com/theRebelliousNerd/codenerd/blob/7ad7edf3ca82fc9c211b2173af41bfea9f7636c6/internal/campaign/recurse_cycle.go#L759-L839
[expand]: https://github.com/theRebelliousNerd/codenerd/blob/7ad7edf3ca82fc9c211b2173af41bfea9f7636c6/internal/gates/gates.go#L129-L147
[runner]: https://github.com/theRebelliousNerd/codenerd/blob/7ad7edf3ca82fc9c211b2173af41bfea9f7636c6/internal/gates/run.go#L48-L110
[fallback]: https://github.com/theRebelliousNerd/codenerd/blob/7ad7edf3ca82fc9c211b2173af41bfea9f7636c6/internal/gates/findings.go#L117-L184
[go-docs]: https://pkg.go.dev/cmd/go#hdr-Package_lists_and_patterns
[visit]: https://github.com/theRebelliousNerd/codenerd/blob/7ad7edf3ca82fc9c211b2173af41bfea9f7636c6/internal/campaign/recurse_cycle.go#L1007-L1118
[source]: https://github.com/theRebelliousNerd/codenerd/blob/7ad7edf3ca82fc9c211b2173af41bfea9f7636c6/internal/campaign/recurse_cycle.go#L1677-L1701
[attempt]: https://github.com/theRebelliousNerd/codenerd/blob/7ad7edf3ca82fc9c211b2173af41bfea9f7636c6/internal/campaign/recurse_cycle.go#L1152-L1178
[pass]: https://github.com/theRebelliousNerd/codenerd/blob/7ad7edf3ca82fc9c211b2173af41bfea9f7636c6/internal/campaign/recurse_cycle.go#L515-L572
[focused]: https://github.com/theRebelliousNerd/codenerd/blob/7ad7edf3ca82fc9c211b2173af41bfea9f7636c6/internal/campaign/recurse_cycle.go#L637-L689
[unattributed]: https://github.com/theRebelliousNerd/codenerd/blob/7ad7edf3ca82fc9c211b2173af41bfea9f7636c6/internal/campaign/recurse_cycle.go#L851-L914
[ratchet]: https://github.com/theRebelliousNerd/codenerd/blob/7ad7edf3ca82fc9c211b2173af41bfea9f7636c6/internal/core/defaults/policy/recurse.mg#L258-L329
[fixtures]: https://github.com/theRebelliousNerd/codenerd/blob/7ad7edf3ca82fc9c211b2173af41bfea9f7636c6/internal/campaign/recurse_workspace_test.go#L83-L215

