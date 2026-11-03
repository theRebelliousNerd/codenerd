---
doc-class: deep-dive
subsystem: campaign-recurse-planning
implementation-status: proposed
last-verified: 2026-11-03
verified-against: 440276a52c52cbc8b003b5830ca70a5d708e5126
verification-method: static-source-inspection
supersedes: []
---

# Recurse loses Python from import submodule prerequisites

## Decision and evidence boundary

Preserve the imported names in Python `from ... import ...` observations before deriving recurse's prerequisite graph. The current scanner retains only the module before `import`. When an imported name denotes a local subpackage, its directory becomes a node but its dependency edge disappears. A focused sweep can omit that prerequisite's separate visit, and a full sweep can visit its dependent first. The host loses the relationship before Mangle receives the graph. [Python producer][python] [Focus closure][focus] [Ordering rules][ordering]

The bounded correction belongs in graph acquisition and its capability contract. Keep the kernel's ordering, cycle handling, scope checks and retention rules. Python permits an imported name to be an attribute or a submodule; a repair must preserve that distinction or explicitly report a conservative dependency envelope. It should not turn every imported symbol into an unquestioned package edge.

This is a planning and work-selection defect. It does not establish a false-green test result, an improperly retained change, an authorization bypass, or a measured cost increase. An applicable Python gate may recursively exercise the omitted directory through a containing package. That incidental execution is useful but does not restore its own scheduled visit or the promised prerequisite order.

**Source freeze:** public main [`440276a52c52cbc8b003b5830ca70a5d708e5126`][main], rechecked unchanged during drafting at approximately 05:11 UTC on 2026-11-03. The review read public guidance, source, architecture, neighboring audits, existing fixtures and the Python language reference. No repository code, tests, builds, gates, application paths, models, benchmarks or reproductions were run. No implementation was made. The examples below are conditional source traces, not observed incidents.

## The contract and the current production route

Root guidance assigns executive decisions to Mangle and requires source-grounded, spec-first changes. The recurse journey describes a sweep of the workspace's own dependency graph from leaves to entry points. The CLI advertises import-derived ordering for Python and a `--subsystem` focus whose dependencies are pulled in. `SubsystemNode.DependsOn` is explicitly the set of nodes that must sweep first. [Guidance][guidance] [Journey][journey] [CLI contract][cli] [Node contract][node]

`DeriveWorkspaceDAG` canonicalizes the workspace root, refreshes membership, collects admitted sources, scans the recognized languages, collapses strongly connected components and appends the closing stages. Python source discovery does not require the same project marker that gate detection uses. Every admitted directory containing a Python file becomes a Python node. [Derivation][derive] [Collection][collect] [Python producer][python]

For each Python file, `scanPythonImports` calls `pythonImportTargets`, resolves each returned target, and adds an edge between known directories. The relevant observation has only two fields: `Up`, the number of relative-import dots, and `Module`, the module text preceding `import`.

The loss is explicit:

1. `pyFromImport` matches leading dots and the optional module up to the `import` keyword. It does not capture the imported names.
2. `pythonImportTargets` appends that pair and immediately continues to the next source line. There is no subsequent parse of the names, aliases or parenthesized import list.
3. `resolvePythonModule` searches the recorded module's path prefixes and returns the deepest matching Python directory. It cannot recover a name that was never retained.
4. A relative import with an empty recorded module resolves to its package directory. `addEdge` correctly drops a self-edge, but that leaves no relationship to an imported child directory. [Parser and resolver][python] [Edge admission][edges]

Component collapse preserves the edges that exist; it does not discover missing ones. `sweepGraphFactList` emits `subsystem_depends` only for each node's recorded `DependsOn` values. Mangle then selects the lexically least unvisited node whose recorded dependencies have been visited. The policy is acting faithfully on an incomplete observation. [Collapse][collapse] [Fact projection][facts] [Ordering rules][ordering]

Both planning and live execution use this graph. `RecurseSweepOrder` derives and validates the whole graph before applying `FilterDAG`. A live pass likewise re-derives, checks graph soundness, and filters focused runs before selecting visits. `FilterDAG` follows dependency edges; it does not include every descendant of a selected directory. [Planning][plan] [Live pass][pass] [Focus closure][focus]

## Witness A an ordinary absolute import loses a subpackage

Consider a clean workspace containing these readable, admitted files:

| Path | Relevant contents |
|---|---|
| `acli/__init__.py` | Empty |
| `acli/main.py` | `from lib import engine` |
| `lib/__init__.py` | Empty |
| `lib/engine/__init__.py` | `VALUE = 1` |

There are no other imports, ignored or forbidden paths, import hooks, `__getattr__` definitions, generated modules or name-shadowing assignments. An ordinary root Python project marker can be added when exercising gate detection; it is unnecessary for the graph observation itself. The example uses ordinary, nonreserved node names and no collapsed cycle.

Python's documented `from` semantics first load the module in the `from` clause, then process each imported name. If the package lacks the named attribute, Python attempts the corresponding submodule. With an empty `lib/__init__.py` and an existing `lib/engine/__init__.py`, `engine` names the `lib.engine` subpackage. This is static, explicit source structure, not a dynamic-import or external-dependency example. [Python import semantics][python-reference]

The inspected producer yields nodes `acli`, `lib` and `lib/engine`. For `acli/main.py`, it records `{Up: 0, Module: "lib"}` and produces `acli -> lib`. It produces no `acli -> lib/engine` edge, and the empty `lib` initializer produces no edge to `lib/engine` either.

That yields two concrete scheduling consequences:

- **Focused pass:** a focus on `acli` includes `acli` and `lib`, but not the separate `lib/engine` node. Repeating the same pass over unchanged source repeats the same omission.
- **Full pass:** initially `lib` and `lib/engine` are ready, and `lib` has the smaller identity key. Once `lib` is visited, `acli` becomes ready and sorts before `lib/engine`. The resulting source-node order begins `lib`, `acli`, `lib/engine`; the dependent precedes the subpackage it imports. Synthetic closing stages remain correctly constrained by the graph they received. [Focus closure][focus] [Lexical projection][facts] [Ordering rules][ordering]

These consequences concern which node is visited and when. They do not mean Python will stop importing `lib.engine`, or that a build/test command will ignore it. The interpreter and runners retain their actual language semantics.

## Witness B a relative child import becomes a discarded self edge

Use the following independent fixture:

- `pkg/__init__.py`: empty.
- `pkg/main.py`: `from . import child`.
- `pkg/child/__init__.py`: `VALUE = 1`.

Python documents this form directly: the leading dot denotes the current package and the imported name identifies its submodule. Under the stated empty-initializer assumptions, the relationship is `pkg -> pkg/child`. [Python relative import semantics][python-reference]

The scanner retains `{Up: 1, Module: ""}`. The resolver begins at `pkg`, finds that directory in `pyDirs`, and returns it. `addEdge("pkg", "pkg")` drops the self-edge as designed. A focus on `pkg` therefore contains only the `pkg` node, while the full sweep's lexical order can place `pkg` before `pkg/child`. The bug is upstream of the self-edge safeguard; removing that safeguard would create an invalid dependency instead of recovering the child. [Parser and resolver][python] [Edge admission][edges]

This witness also shows why merely keeping a containing-package edge does not repair the absolute case. Imported names are required to distinguish a same-directory attribute from a separate child package.

## Existing protections and the claims they limit

Preserve these controls:

- Canonical root identity, shared workspace membership, hidden-directory exclusions and project forbidden paths. A discovered name must not admit an outside or excluded file.
- Known-node edge joining, legitimate self-edge removal and strongly connected component collapse.
- Whole-graph structural validation before focus, explicit refusal of unknown focus names, and graph re-derivation on later passes.
- Kernel-authoritative readiness, deterministic ordering, pass budgets, stop and cancellation behavior.
- Actual gate execution, unavailable/failed dispositions, task acceptance, write ownership, mission restrictions and the ordinary keep/revert ratchet.

Structural graph validation rejects duplicates, unknown endpoints and invalid self-dependencies. It does not certify that every source relationship was observed. The witness graph has none of those structural faults. Re-derivation cannot restore a consistently discarded imported name. [Validation][validation] [Derivation][derive]

Conversely, visiting the containing `lib` or `pkg` node can have broader effects than a single directory label suggests. Detected Python `compileall` and pytest gates receive the directory argument and may inspect descendants. `SubsystemNode.Paths` also describes roots for task write scope, so the parent node's allowed root may encompass the child. This report makes no claim that omitted children necessarily escape all checking, cannot be edited, or produce an accepted defective commit. Gate or scope coverage and scheduled prerequisite visits are different contracts. [Python gate detection][detect] [Node contract][node]

The current Python tests are useful controls. The ordering fixture uses forms such as `from shop.models.item import Item`, `from ..models import item` and `from .api import views`. In those examples the relevant directory is already named before `import`, so the current parser can recover it. The cycle fixture also names the other package before `import`. Neither inspected fixture supplies a directory solely through an imported name. The test helper compares kernel order with a topological oracle derived from the same graph, which checks ordering agreement but cannot prove source-edge completeness. These tests were read, not run. [Existing Python fixture][fixtures] [Existing cycle fixture][cycle-fixture]

## Bounded nonduplication screen

The review read root and campaign guidance, campaign architecture, the recurse journey, the audit queue, the portable recurse proposal, and the relevant dependency, identity, gate and evidence reports. Targeted searches restricted to `Docs/audits` returned no matches for `pythonImportTargets`, `resolvePythonModule` or `from-import`. A Python/dependency search returned neighboring reports that were screened for this mechanism. This is a bounded public-main search, not a claim to have exhausted every branch, issue or historical discussion.

- **Portable recurse deliberation/crucible** already states that graph discovery is not complete semantic dependency analysis and proposes capability reporting. This report contributes a narrow counterexample using an ordinary explicit local import, rather than reintroducing its broad concern about dynamic imports, generated code or external dependencies. [Portable proposal][portable]
- **Go test dependency closure** and **Cargo inherited dependency closure** establish the neighboring requirement to retain prerequisites consumed by the work. They concern different observation producers. Current source already reads Go test import lists and Cargo inherited dependencies; neither repairs the Python imported-name loss. [Go report][go-audit] [Cargo report][cargo-audit] [Current Go producer][go-current] [Current Cargo producer][cargo-current]
- **Gate language domain** concerns correctly discovered Python edges collapsing a heterogeneous component, after which invocation dispatch loses per-path languages. The present witnesses are Python-only and require no component collapse or wrong-toolchain dispatch. [Language report][language-audit]
- **Node namespace identity** concerns collisions with synthetic stage names or component IDs. These examples have ordinary distinct IDs. **Collapsed finding invocation identity** concerns a selected finding receiving another member's argv. This finding arises before gates or diagnostics exist. [Namespace report][namespace-audit] [Invocation report][invocation-audit]

## Proposed observation contract

The host should provide a faithful, revision-bound Python import observation. Mangle should continue deciding scheduling and eligibility over that observation.

1. **Retain the syntax needed to resolve the relationship.** Preserve the import form, relative level, preceding module, imported names and aliases, and the source file/revision. A structured parser is preferable to repeatedly extending the current line expression. Do not execute project modules to discover imports.
2. **Resolve within admitted source membership.** For the bounded examples, identify both the containing package and the existing local subpackage named by the import. Join to actual workspace nodes rather than inventing directories from arbitrary symbols.
3. **State the graph's meaning.** An exact runtime dependency graph and a conservative static prerequisite envelope are different products. A package attribute can shadow a same-named submodule, and import hooks or module attributes can be dynamic. Preserve ambiguity explicitly or conservatively include a possible local prerequisite with that documented meaning; do not claim it is a proven runtime import.
4. **Preserve supported controls.** Ordinary `import x`, already supported qualified `from` forms, aliases, third-party imports, exclusions and self-edge handling must retain their intended behavior. Missing or unreadable evidence should not quietly certify a complete empty dependency set.
5. **Respect graph identity changes.** Recovered edges can create a real SCC and change component IDs, member paths, focus names and journal/resume identities. Revalidate a resumed or already admitted plan against the new graph. Do not silently expand mission authority or reuse evidence admitted for an incompatible component.
6. **Keep execution and scheduling separate.** The fix should not run gates during discovery, add a second Go scheduler, bypass policy to force visits, or weaken gate evidence because a graph is incomplete.

For an initial bounded implementation, require correct acquisition for ordinary imports over explicit admitted package files, retain an honest unresolved/ambiguous disposition elsewhere, and extend the owning specification before changing code. Whether existing CodeDOM import observations can serve this consumer needs a separate wiring and identity review; their existence alone is not proof of a safe replacement.

## Fair evaluation plan all unrun

No test result or performance number is claimed. Proposed evaluation should first isolate observation correctness, then exercise production composition under separate authorization.

### Deterministic producer and consumer fixtures

| Fixture or control | Required observation or outcome |
|---|---|
| Witness A with an empty parent initializer | Preserve the local subpackage prerequisite; focused closure includes its separate node; the dependent cannot precede it. |
| Witness B with an empty parent initializer | Recover the child prerequisite rather than retaining an invalid self-edge. |
| Existing qualified absolute and relative imports | Existing relationships and order remain correct. |
| Aliased, multiple-name and parenthesized `from` imports | Each supported module candidate retains its own identity; aliases do not replace source names. |
| Attribute-only import with no matching submodule | Do not manufacture a nonexistent node or edge. |
| Parent attribute shadowing an existing submodule | Follow the declared exact/conservative/ambiguous contract; do not label a possible edge as a proven runtime dependency. |
| Admitted `src/` layout and ordinary packages | Resolve relative levels and roots consistently with the existing supported layout. |
| Third-party, ignored, forbidden or outside-root candidates | No membership or write-authority expansion. |
| Newly exposed import cycle | Preserve the real component and dependency order without ID collisions or stale resume assumptions. |
| No parser result, unreadable source or unsupported dynamic form | Preserve a truthful capability limitation rather than claiming complete independence. |

Use hand-specified expected dependency relations derived from the fixture's language semantics. Comparing two sorting algorithms over the same generated graph is insufficient. Exercise actual `DeriveWorkspaceDAG -> FilterDAG -> sweepGraphFactList -> recurse_next_node` composition, not only a helper receiving preconstructed targets. Then verify the normal plan and live recurse entry paths against the same expected node closure. [Plan route][plan] [Live route][pass]

Separate graph assertions from gate assertions. Include one check intentionally restricted to each node directory and one ordinary recursive Python gate. The former distinguishes missing scheduled work; the latter ensures the audit does not mistake incidental descendant checking for a direct visit. Preserve real gate failures, unavailable tools, cancellation, scope refusal and retention semantics. Production-path fixtures can use a deterministic executor; paid model behavior is unnecessary to prove this graph boundary.

### Cost and task usefulness

Measure before and after on the same source snapshots and execution profiles. Report source files and bytes read, parse time, allocations, membership lookups, graph nodes/edges/components, focus size and scheduling overhead. Include small projects, deeply nested packages, repositories with many imported symbols, repeated imports, and mixed-language projects. Keep development fixtures separate from the held-out repositories used to assess generality.

Recovered dependencies may make a focused sweep larger and more expensive. That is a possible cost of honoring its contract, not automatically a regression or a speedup. Conservative over-approximation may also add unnecessary work. Count false-positive prerequisite visits, duplicate gate executions, retained/reverted attempts, model calls and tokens if an authorized end-to-end study is later undertaken, wall time and total cost per independently verified repair. Include failures, blocked cases and budget exhaustion in the denominator.

For a later task study, hold model/provider, task instructions, permission scope, gate definitions and total budgets fixed. Compare the existing and corrected producer on matched tasks with independent acceptance checks. Do not equate more graph edges or more visits with better repair quality. A result that fixes closure but adds disproportionate cost should prompt narrower resolution or better capability reporting, not a hidden relaxation of the prerequisite promise.

## Acceptance boundary

The bounded change is complete when the ordinary explicit imports above preserve their local subpackage prerequisites through real graph acquisition, focus and policy ordering; symbol-only and excluded candidates remain controlled; graph changes retain identity and authority checks; and normal execution still reports only what its gates observed.

That establishes a specific Python portability boundary. It does not establish complete Python semantics, universal dependency discovery, a general task-quality improvement or a measured cost benefit.

## Pinned references

[main]: https://github.com/theRebelliousNerd/codenerd/commit/440276a52c52cbc8b003b5830ca70a5d708e5126
[guidance]: https://github.com/theRebelliousNerd/codenerd/blob/440276a52c52cbc8b003b5830ca70a5d708e5126/agents.md
[journey]: https://github.com/theRebelliousNerd/codenerd/blob/440276a52c52cbc8b003b5830ca70a5d708e5126/Docs/journeys/10-forever-loop.md#L3-L9
[cli]: https://github.com/theRebelliousNerd/codenerd/blob/440276a52c52cbc8b003b5830ca70a5d708e5126/cmd/nerd/cmd_campaign_recurse.go#L56-L85
[node]: https://github.com/theRebelliousNerd/codenerd/blob/440276a52c52cbc8b003b5830ca70a5d708e5126/internal/campaign/recurse_dag.go#L15-L33
[derive]: https://github.com/theRebelliousNerd/codenerd/blob/440276a52c52cbc8b003b5830ca70a5d708e5126/internal/campaign/recurse_workspace.go#L59-L100
[collect]: https://github.com/theRebelliousNerd/codenerd/blob/440276a52c52cbc8b003b5830ca70a5d708e5126/internal/campaign/recurse_workspace.go#L264-L317
[python]: https://github.com/theRebelliousNerd/codenerd/blob/440276a52c52cbc8b003b5830ca70a5d708e5126/internal/campaign/recurse_workspace.go#L456-L543
[edges]: https://github.com/theRebelliousNerd/codenerd/blob/440276a52c52cbc8b003b5830ca70a5d708e5126/internal/campaign/recurse_workspace.go#L131-L146
[collapse]: https://github.com/theRebelliousNerd/codenerd/blob/440276a52c52cbc8b003b5830ca70a5d708e5126/internal/campaign/recurse_workspace.go#L148-L238
[facts]: https://github.com/theRebelliousNerd/codenerd/blob/440276a52c52cbc8b003b5830ca70a5d708e5126/internal/campaign/recurse_plan.go#L108-L134
[ordering]: https://github.com/theRebelliousNerd/codenerd/blob/440276a52c52cbc8b003b5830ca70a5d708e5126/internal/core/defaults/policy/recurse.mg#L403-L446
[focus]: https://github.com/theRebelliousNerd/codenerd/blob/440276a52c52cbc8b003b5830ca70a5d708e5126/internal/campaign/recurse_dag.go#L35-L76
[plan]: https://github.com/theRebelliousNerd/codenerd/blob/440276a52c52cbc8b003b5830ca70a5d708e5126/internal/campaign/recurse_plan.go#L38-L67
[pass]: https://github.com/theRebelliousNerd/codenerd/blob/440276a52c52cbc8b003b5830ca70a5d708e5126/internal/campaign/recurse_cycle.go#L515-L595
[validation]: https://github.com/theRebelliousNerd/codenerd/blob/440276a52c52cbc8b003b5830ca70a5d708e5126/internal/campaign/recurse_plan.go#L83-L107
[detect]: https://github.com/theRebelliousNerd/codenerd/blob/440276a52c52cbc8b003b5830ca70a5d708e5126/internal/gates/detect.go#L134-L142
[fixtures]: https://github.com/theRebelliousNerd/codenerd/blob/440276a52c52cbc8b003b5830ca70a5d708e5126/internal/campaign/recurse_workspace_test.go#L111-L133
[cycle-fixture]: https://github.com/theRebelliousNerd/codenerd/blob/440276a52c52cbc8b003b5830ca70a5d708e5126/internal/campaign/recurse_workspace_test.go#L185-L202
[python-reference]: https://docs.python.org/3.13/reference/simple_stmts.html#the-import-statement
[portable]: https://github.com/theRebelliousNerd/codenerd/blob/440276a52c52cbc8b003b5830ca70a5d708e5126/Docs/audits/portable-recurse-deliberation-crucible.md
[go-audit]: https://github.com/theRebelliousNerd/codenerd/blob/440276a52c52cbc8b003b5830ca70a5d708e5126/Docs/audits/recurse-go-test-dependency-closure.md
[cargo-audit]: https://github.com/theRebelliousNerd/codenerd/blob/440276a52c52cbc8b003b5830ca70a5d708e5126/Docs/audits/recurse-cargo-inherited-dependency-closure.md
[go-current]: https://github.com/theRebelliousNerd/codenerd/blob/440276a52c52cbc8b003b5830ca70a5d708e5126/internal/campaign/recurse_workspace.go#L368-L449
[cargo-current]: https://github.com/theRebelliousNerd/codenerd/blob/440276a52c52cbc8b003b5830ca70a5d708e5126/internal/campaign/recurse_workspace.go#L627-L733
[language-audit]: https://github.com/theRebelliousNerd/codenerd/blob/440276a52c52cbc8b003b5830ca70a5d708e5126/Docs/audits/recurse-gate-language-domain.md
[namespace-audit]: https://github.com/theRebelliousNerd/codenerd/blob/440276a52c52cbc8b003b5830ca70a5d708e5126/Docs/audits/recurse-node-namespace-identity.md
[invocation-audit]: https://github.com/theRebelliousNerd/codenerd/blob/440276a52c52cbc8b003b5830ca70a5d708e5126/Docs/audits/recurse-collapsed-finding-invocation-identity.md
