---
doc-class: deep-dive
subsystem: campaign-recurse
implementation-status: proposed-refinement
evidence-status: static-source-inspection
last-verified: 2026-11-03
verified-against: ef9fc03a591bdbcc368a2e0f6ce949c4f58397ee
supersedes: []
---

# Preserve Python package initializer prerequisites in focused recurse

## Finding and decision

A fully recognized Python dotted import can still produce an incomplete recurse dependency closure. The scanner records one dependency on the **deepest directory** that matches the imported module. It does not retain the regular-package ancestors whose initializers participate in importing that module. Those ancestors can already exist as admitted graph nodes yet be omitted from a focused sweep. [Scanner][scan] [Resolver][resolve] [Focus closure][focus]

The smallest witness uses `import zzpkg.child.leaf`, with an ordinary `__init__.py` at each package level. The source-derived graph gives the consumer one edge to `zzpkg/child/leaf`. Focusing on the consumer consequently retains that leaf but not `zzpkg` or `zzpkg/child`. No `from ... import ...` parsing, alias, dynamic import, namespace ambiguity, source-read failure or cyclic graph is needed.

**Recommendation:** define and implement a package-initialization prerequisite contract in the existing Python graph adapter. Preserve regular-package ancestor identities when constructing a supported local import closure, retain explicit uncertainty outside that domain, and keep Mangle in charge of scheduling. Do not ask the proposer to compensate by widening an attempt's write scope.

This is a source-grounded portability defect in dependency acquisition. The focus result follows from the inspected resolver and closure code; its runtime reproduction and downstream behavioral effects are unrun. There is no claim of a failed test accepted as passing, unauthorized mutation, or a defective commit being retained.

## Source and execution boundary

Public main resolved to **ef9fc03a591bdbcc368a2e0f6ce949c4f58397ee**, committed November 3, 2026 at 15:11:31 UTC, when checked at approximately 15:37 UTC and remained unchanged at the 15:48 UTC recheck. The source snapshots used here were matched byte-for-byte, after removing only a connector-added final newline where necessary, against blob identities in that commit's Git tree. All repository links below use this pin. [Commit][pin]

Inspection covered root and architecture guidance, current campaign mission and session specifications, the requirements-interrogator contract, the Python scanner, graph construction and focus, production planning and run consumers, mission scope, authored tests and relevant existing audits.

No repository program, test, build, package installation, model inference, gate, benchmark or failure injection ran. No implementation, credential, configuration or setting changed. Only scratch research material and this Markdown report were written. The illustrative fixture below was not created or imported.

## Why this matters before Socratic design

The mission path already supplies the desired proposer and requirements interrogator. They receive the selected recurse attempt, original mission, North Star and previous outcome. The proposer must preserve the attempt's existing scope and gate. The mission specification likewise says the participants cannot choose a different write scope or confer permission. These are useful controls, not defects. [Mission specification][mission-spec] [Proposer contract][proposer]

That division makes prerequisite acquisition consequential. When the host omits a prerequisite from the focused work graph, the ensuing creative exchange operates inside an incomplete set of selected work items. A capable interrogator might discover an initializer and explain the problem; noticing it does not create a permitted repair target. More deliberation rounds or a stronger model cannot establish the missing host-side edge.

The repository's north star calls for codebase reasoning grounded in actual structure, with executive decisions in Mangle. The requirements-interrogator guidance asks for explicit lifecycle, failure, compatibility and falsifying-test contracts. Here the relevant requirement is concrete: a focused Python package task should retain the supported local initialization prerequisites on which that package depends. [Root guidance][guidance] [Interrogator guidance][interrogator] [Architecture contract][architecture]

This is not a proposal to make every directory above a file a dependency. Import roots, namespace packages and ordinary filesystem parents have different meanings. The first correction should cover a declared, checkable regular-package domain.

## Implemented behavior and the missing relation

### Source acquisition already sees the initializers

`DeriveWorkspaceDAG` canonicalizes the workspace root, refreshes membership, collects admitted source files and calls `scanPythonImports`. The collector includes admitted `.py` files, so ordinary readable `__init__.py` files are not excluded by their name. [Discovery][discovery] [Collection][collection]

The Python scanner first adds a node for every directory containing a collected Python file. In the witness, every ancestor initializer directory is therefore present. This differs from a missing-node or failed-discovery case. [Node acquisition][scan]

Existing membership and forbidden-path boundaries remain in force. A stronger adapter must distinguish an admitted node whose dependency evidence is unreadable from one whose dependency set is known to be empty. These cases are outside the minimal witness and require an explicit unsupported or incomplete disposition.

### The whole import is parsed but only one owner survives

For an ordinary `import zzpkg.child.leaf` statement, `pyImport` captures the complete dotted name. `pythonImportTargets` preserves it in one `pyImportTarget.Module`. No missing imported-name list is needed. [Parsing][parse]

`resolvePythonModule` then:

1. Splits the module into components.
2. Tries the workspace root and `src` bases for an absolute import.
3. Walks from the longest component prefix to shorter prefixes.
4. Returns immediately on the first directory in `pyDirs`.

The return type carries a single directory and a Boolean. There is no returned initialization chain, no package-kind classification and no completeness result. `scanPythonImports` calls `g.addEdge(from, to)` once for that returned target. [Resolver][resolve] [Edge construction][scan]

The distinction matters because Python's ordinary import machinery initializes regular parent packages as part of importing their descendants. The official reference gives the parent/subpackage example and identifies `__init__.py` execution explicitly. This establishes the language premise, not codeNERD's end-to-end behavior. [Python regular packages][python]

### Graph collapse cannot restore missing prerequisites

`workspaceGraph.addEdge` ignores self-edges and references to unknown nodes. Its component collapse preserves the edges it received and merges strongly connected components. It does not infer containment or package initialization. Cross-cutting nodes depend on the graph's top-level consumers; they do not add an edge from a selected leaf back to an omitted package ancestor. [Graph operations][graph] [Cross-cutting close][crosscut]

`FilterDAG` starts at each requested node and follows only `DependsOn`. It correctly refuses unknown requested IDs. It has no filesystem or Python-resolution phase. Consequently a valid graph with an absent dependency gives a valid but incomplete focused set. The scheduler need not be wrong for this to occur. [Focus closure][focus]

### Both plan and execution use that closure

`RecurseSweepOrder` derives and checks the graph, applies `FilterDAG` for named subsystems and asks the kernel to order the focused result. The actual run's `pass` separately rederives the graph, stores the full node set, applies the same filter and asserts the resulting sweep graph. This is a production planning/execution input, not an unused helper. [Planning][plan] [Run path][run]

`sweepGraphFactList` projects `DependsOn` into `subsystem_depends`. The Mangle sweep rules consume those supplied edges. Their obligation is to order the admitted graph; they cannot distinguish an intentionally independent node from a prerequisite omitted by a language adapter. [Fact projection][facts] [Sweep rules][schedule]

### Deliberation receives the already selected scope

Mission preparation serializes the attempt into the design request and copies `a.Node.Paths` into the mission receipt's `Scope`. The write guard later measures containment against that receipt and asks policy for admission. A missing ancestor is not granted by a successful proposal or a convincing explanation. [Mission preparation][mission] [Scope guard][guard]

There is a separate, conditional diagnostic consequence. For a focused **workspace-level** gate, `measure` discards a finding when its target belongs to a known full-graph node but no selected node. Thus a correctly parsed finding at `zzpkg/__init__.py` can be treated as outside the focused sweep if that package node was omitted. This is conditional on that gate producing such a path-owned finding; the report does not claim every Python exception produces one. Node-scoped results follow a different attribution branch. [Finding filter][filter]

The raw gate result and gate-level failure remain separate. Filtering one finding does not establish that the gate passes or that the ratchet accepts a change. The principal finding needs no diagnostic at all: the ancestor nodes are absent from the selected prerequisite closure.

## Minimal unrun witness

Assume a temporary workspace with these four admitted, readable files and no other source imports:

| File | Contents |
|---|---|
| `aa_app/use.py` | `import zzpkg.child.leaf` |
| `zzpkg/__init__.py` | `ROOT_READY = True` |
| `zzpkg/child/__init__.py` | `CHILD_READY = True` |
| `zzpkg/child/leaf/__init__.py` | `VALUE = 1` |

Use an ordinary import environment in which this workspace root uniquely supplies `zzpkg`. Exclude import hooks, competing installed packages, `__path__` mutation, namespace portions and source changes. All paths pass existing membership and forbidden-path checks.

The predicted current source trace is:

1. Node acquisition yields `aa_app`, `zzpkg`, `zzpkg/child` and `zzpkg/child/leaf`, plus the existing synthetic close nodes.
2. Parsing `aa_app/use.py` yields the complete target `zzpkg.child.leaf`.
3. The resolver's first successful directory is `zzpkg/child/leaf`; it returns without retaining shorter prefixes.
4. The only ordinary source dependency is `aa_app -> zzpkg/child/leaf`. The initializer constants produce no separate import edges.
5. Focusing on `aa_app` retains `aa_app` and `zzpkg/child/leaf`. It omits both regular-package ancestor nodes. Focusing directly on the leaf likewise does not pull them in.
6. An attempted repair of `zzpkg/__init__.py` from either retained node is outside that node's ordinary mission scope.

Steps 1 through 5 are a static walk through deterministic code, not recorded tool output. Step 6 is conditional on reaching a mission attempt and requesting that write; none was attempted.

A companion fixture can give `zzpkg/__init__.py` its own import of another admitted local package. The current graph can record that parent's dependency correctly while still leaving the entire parent branch outside the consumer's focused closure. That variation tests transitive loss, not a new parsing bug.

### What this witness does not establish

- It does not show that the simple full sweep has a bad visitation order. Lexical tie-breaking can put ancestors first by coincidence. No such order is claimed here.
- It does not show that omitted initializers are never read by CodeDOM, world context, a model or a broad gate.
- It does not show that selecting the parent node forbids child work. A parent `Paths` root may cover descendants; the direction relevant here is the opposite: selecting only a descendant does not cover its parent.
- It does not require meaningful initializer side effects to establish closure loss. Real initialization errors or configuration effects would make the missing node operationally important, but their incidence is unmeasured.
- It does not claim every `.py` directory is a package, every Python execution initializes ancestors, or a warm `sys.modules` cache reexecutes each initializer.
- An explicitly selected ancestor, another recorded import path to it, or a broader permitted sweep can retain it. Those are valid controls, not repairs to the automatic focused-closure contract.

## Distinctness from existing audits

The pinned audit inventory, queue and nearby report bodies were screened. This is a bounded novelty check.

**Python from-import prerequisites** identifies names after `import` being dropped from a `from` statement, including a missing child subpackage. This witness uses ordinary `import`, preserves the whole name and successfully resolves the leaf. The lost objects are implicitly required ancestors, not unparsed descendants. Parsing every name in the earlier report would not by itself repair this single-target return. Both should inform one coherent Python adapter design rather than competing resolvers. [Earlier Python audit][prior-python]

**JavaScript workspace prerequisite identity**, **Go workspace acquisition** and the Cargo dependency audits concern other resolution mechanisms. Their shared lesson about truthful prerequisite closure is prior work; this report adds the regular-package initialization relation and its focused-scope witness. [JavaScript audit][prior-js] [Go audit][prior-go]

**Node namespace identity** concerns graph-ID collisions. All ordinary nodes here have distinct unambiguous IDs, and the primary witness has no SCC. **Gate-language domain** concerns language ownership inside a collapsed component. No mixed-language component is needed. [Namespace audit][prior-namespace] [Gate-language audit][prior-language]

**Portable recurse deliberation crucible** already proposes capability-aware adapters and stronger requirement/evidence contracts. This is a concrete current adapter gap under that proposal, not a new architecture or a claim that the general portability idea is novel. [Portable proposal][portable]

The recent deliberation audits concern evidence admission, objections, review handoff, proof ownership, context capacity and values continuation. Their corrections cannot invent a prerequisite node in an already focused attempt. Conversely, correcting this graph does not validate a model's design, retire an objection, admit a citation or establish a behavioral test result.

## Proposed contract and ownership

Update the owning campaign and workspace capability/specification before implementation. The following is a proposed contract, not a description of shipped APIs.

### Define the supported resolution domain

For a uniquely resolved local regular-package import under a declared import-root interpretation, retain:

- The importing source and parsed module reference.
- The resolved leaf source/package owner.
- Each admitted local regular-package ancestor initializer on that resolution path.
- The distinction between exact resolution, conservative possible dependency and unresolved/unsupported resolution.
- The source/membership generation needed to decide whether the observation is still applicable.

A source index may supply this information, or the existing scanner may be extended. The missing requirement is the relation and its evidence, not the choice of parser library.

Within the minimal supported domain, absence of an explicit `import parent` statement must not erase initialization prerequisites. Outside that domain, lack of a known ancestor must not be silently advertised as proof that there is no dependency.

### Preserve prerequisite closure and the right kind of order

A supported package import should retain all known initialization prerequisites in its focused closure. The graph may represent child-to-parent edges, a typed initialization relation that policy projects, or another explicitly specified equivalent. Directly focusing a supported child should not depend on whether some unrelated consumer happened to import it.

Keep actual interpreter import sequencing distinct from coarse sweep scheduling. Directory nodes contain more than an initializer; an initializer can itself import a descendant. Adding valid ancestor relations can therefore create real coarse-grained cycles. Use the existing SCC mechanism where the declared scheduling semantics require it; do not drop the new relation merely to preserve an acyclic-looking graph.

The design should say whether the required result is ancestor-first order where possible, joint component treatment where dependencies cycle, or another conservative plan. A package initialization dependency is evidence for planning, not proof that every module in a directory must execute in that sequence.

### Keep host measurement and policy decisions separate

Go should acquire and validate paths, package markers, module ownership, resolution support and graph evidence. Mangle should continue to derive admitted work and visitation order over those facts. The LLM may explain uncertainty or suggest a needed adapter improvement; it must not turn an unresolved dependency into permission to edit elsewhere.

The normal membership, forbidden-path, canonical-root, scope, oracle protection and ratchet controls remain in force. A resolved external or forbidden prerequisite should remain visibly unavailable within the local sweep, not be silently imported into its writable graph. Correcting discovery may legitimately enlarge the dependency set pulled by a focused command; that expansion must remain visible and within the command's declared dependency/permission contract.

### Treat namespace and shadowing cases explicitly

PEP 420 defines namespace packages that lack a regular initializer and can span multiple locations. A universal “add every parent directory” rule would be unsound. The adapter needs evidence of package kind and import-root identity, and must not fabricate an initializer or assume every same-named directory is the resolved parent. [Namespace package specification][pep420]

The first implementation can deliberately support a narrower exact domain and surface uncertainty for namespace portions, custom hooks, generated modules, extension modules, ambiguous root/`src` layouts and `__path__` manipulation. It should not claim general Python import equivalence until those cases have an explicit contract and evidence.

Source changes need rederivation or generation invalidation. The current run already rederives its graph each pass; any proposed caching must preserve that behavior. A failed read, disappeared initializer or changed membership cannot reuse a prior “complete” relation without checking validity. [Per-pass derivation][run]

## Failure modes and costs

A bounded correction adds ancestor discovery and edge work, but the largest costs may occur after planning.

Let `F` be admitted Python files, `I` recognized import references and `D` the maximum module depth. A cached package-marker index can limit repeated filesystem checks; a straightforward ancestor walk can inspect up to order `I × D` prefixes. This is a design estimate, not a measured implementation complexity or a claim that all resolver work is linear. Hashing, canonicalization, graph projection and SCC/fixpoint costs depend on the chosen representation and workload.

Measure the complete consequences:

- Source enumeration, reads, package-marker checks, canonicalization, parsing, allocations and retained index bytes.
- Number of additional prerequisite nodes/edges, SCC merges, focused-plan size and changed scope descriptions.
- Kernel fact assertion, ordering/fixpoint time and graph rebuilds after source changes.
- Additional visits, gates, source snapshots, proof/evidence records and journal writes.
- All proposer, interrogator and specialist requests, tool reads, JIT work, repeated context bytes, input/output/reasoning usage, provider retries and known/unknown spend.
- Authorized implementation attempts, validation, compensation, unresolved stops and owner effort.
- End-to-end latency and total cost per independently acceptable result, including failed and deferred runs.

The baseline may look cheaper because it omits work. The correction may also over-include work, merge a large package hierarchy or spend more without improving the final result. Neither fewer model calls nor a larger dependency graph is a success criterion by itself.

Principal risks to design and evaluate:

| Risk | Required behavior |
|---|---|
| Namespace directory treated as a regular initializer | Package-kind evidence or an explicit unresolved disposition; no invented source. |
| A parent initializer imports a child | Preserve the relation and handle the resulting component honestly; no endless walk or edge deletion to force order. |
| Parent scope subsumes many children | Expose scope growth, deduplicate work where allowed and preserve the existing write-admission contract. |
| Root and `src` both contain a matching package | Bind to the declared resolution environment or report ambiguity; do not call the first textual match exact. |
| Forbidden, missing or unreadable initializer | Preserve the limitation without bypassing membership or using an empty success. |
| Initializer created, removed or edited between passes | Reacquire applicable relations and distinguish earlier historical evidence. |
| Additional nodes exhaust a mission envelope | Stop truthfully within the original limits; do not silently raise cycles, time, spend or permissions. |
| A broad gate still checks outside the focused set | Keep gate verdict, finding attribution and selected work separate. |
| Package already initialized in one runtime | Do not confuse a warm runtime shortcut with a complete static prerequisite relation for future clean runs. |

## Fair evaluation plan with no claimed results

### Deterministic contract checks

Start with the exact four-file witness through real `DeriveWorkspaceDAG`, `FilterDAG`, graph projection and production Mangle scheduling. Inspect the resulting node identities, edges and focused set. A test that manually supplies the desired edges checks the consumer, not this producer defect.

Required fixtures:

1. The ordinary dotted-import witness: current baseline's predicted omitted ancestors versus a treatment retaining the declared initialization closure.
2. Direct focus on the leaf package with no consumer selected.
3. A leaf module file inside a regular child package, so same-directory ownership is not mistaken for a separate missing node.
4. Several regular ancestors and a parent initializer with its own local dependency.
5. Existing explicit import paths that already retain an ancestor, to avoid duplicate work.
6. Empty initializers as well as initialization code; syntactic emptiness does not silently change the declared package contract.
7. Namespace-only parents, mixed namespace/regular chains and unrelated filesystem parents, with expected exact/partial/unsupported distinctions.
8. Root and `src` layouts, competing package names, case-sensitive and case-insensitive hosts, and paths containing non-ASCII characters.
9. A parent initializer importing a descendant: expected SCC identity, finite closure, all member paths and preserved scope restrictions.
10. Ignored, forbidden, missing or unreadable ancestors and source-generation changes.
11. A workspace-level diagnostic naming an omitted initializer versus a node-scoped diagnostic, keeping their different attribution rules explicit.
12. Already supported ordinary Go, Rust and JavaScript graph fixtures, plus the earlier Python from-import cases, to check that the shared graph consumer did not regress.

Existing authored Python tests include regular package initializers and assert selected explicit-import ordering. Their assertions do not establish initialization closure for the four-file witness or direct leaf focus. Those fixtures should be reviewed when adding implicit relations because a new parent relation may change an existing component's topology. No current test is claimed to fail, and no repository-wide absence-of-tests claim is made. [Existing tests][tests]

### Production entry and behavioral qualification

After an authorized implementation exists, first exercise the normal focused planning entry and a bounded mission run with world-model and CodeDOM context enabled. Record the graph and scope actually consumed, the design request, permitted writes, gate outcomes and terminal receipt. Do not treat a unit graph snapshot alone as proof of end-to-end capability.

A later behavioral fixture can place a known, independently specified initialization defect in an ancestor and ask for a bounded consumer behavior that depends on it. Compare whether the system identifies the right prerequisite, obtains an admissible work item and completes or truthfully blocks without patching an unrelated child. Use an independent oracle for the desired behavior and clean interpreter state where import execution is evaluated. Such interpreter execution is for future authorized validation, not graph discovery in this report.

Compare:

- Pinned baseline.
- The smallest regular-package-aware treatment.
- A conservative broader-sweep alternative, measured separately rather than presented as free correctness.

Keep tasks, model/provider, JIT corpus, permissions, toolchain, gates and total resource envelopes matched. Include successful, blocked, failed and budget-exhausted runs. Report prerequisite recall and precision against independently reviewed module ownership, scope expansion, valid design rate, correctly localized repairs, unauthorized-write refusals, verified behavior and full cost. A treatment that refuses every ambiguous case may improve a narrow omission count while reducing useful task completion; both belong in the result.

### Primary literature and limits

Mokhov, Mitchell and Peyton Jones' **Build Systems à la Carte** separates dependency-driven scheduling from decisions about rebuilding. It supports evaluating the dependency producer independently from an otherwise correct scheduler and accounting for additional work. codeNERD's adaptive mission loop is not the paper's build-system model; the paper does not establish an LLM quality gain, a measured speedup or this specific defect. [Primary paper][build-systems]

Python's reference and PEP 420 supply the language semantics and package-kind distinction. They do not prescribe codeNERD's node granularity, write scope, SCC policy or benchmark. Those remain explicit design decisions to test.

## Acceptance boundary

A focused recurse task over the declared supported Python regular-package domain must retain the admitted local initialization prerequisites of its selected packages, or explicitly report that the prerequisite relation is unresolved. A correctly parsed leaf name and a valid DAG alone do not establish that contract.

The first change should extend the existing adapter and its source-to-policy evidence, not add another agent framework or relax mission scope. Preserve the distinction between a more complete work graph, a ready design, a passing gate and fulfillment of the owner's mission.

The report establishes the missing single-target-to-initialization-chain relation and its focused-closure consequence. Runtime incidence, implementation correctness, semantic repair quality and performance remain unmeasured.

## Pinned references

[pin]: https://github.com/theRebelliousNerd/codenerd/commit/ef9fc03a591bdbcc368a2e0f6ce949c4f58397ee
[guidance]: https://github.com/theRebelliousNerd/codenerd/blob/ef9fc03a591bdbcc368a2e0f6ce949c4f58397ee/agents.md#L3-L51
[architecture]: https://github.com/theRebelliousNerd/codenerd/blob/ef9fc03a591bdbcc368a2e0f6ce949c4f58397ee/Docs/architecture/AGENTS.md#L48-L77
[interrogator]: https://github.com/theRebelliousNerd/codenerd/blob/ef9fc03a591bdbcc368a2e0f6ce949c4f58397ee/.agents/skills/requirements-interrogator/SKILL.md
[mission-spec]: https://github.com/theRebelliousNerd/codenerd/blob/ef9fc03a591bdbcc368a2e0f6ce949c4f58397ee/Docs/architecture/campaign/MISSION-IMPROVEMENT.md#L98-L110
[proposer]: https://github.com/theRebelliousNerd/codenerd/blob/ef9fc03a591bdbcc368a2e0f6ce949c4f58397ee/internal/prompt/atoms/system/deliberation_proposer.yaml#L54-L70
[discovery]: https://github.com/theRebelliousNerd/codenerd/blob/ef9fc03a591bdbcc368a2e0f6ce949c4f58397ee/internal/campaign/recurse_workspace.go#L61-L102
[collection]: https://github.com/theRebelliousNerd/codenerd/blob/ef9fc03a591bdbcc368a2e0f6ce949c4f58397ee/internal/campaign/recurse_workspace.go#L269-L316
[scan]: https://github.com/theRebelliousNerd/codenerd/blob/ef9fc03a591bdbcc368a2e0f6ce949c4f58397ee/internal/campaign/recurse_workspace.go#L461-L481
[parse]: https://github.com/theRebelliousNerd/codenerd/blob/ef9fc03a591bdbcc368a2e0f6ce949c4f58397ee/internal/campaign/recurse_workspace.go#L456-L510
[resolve]: https://github.com/theRebelliousNerd/codenerd/blob/ef9fc03a591bdbcc368a2e0f6ce949c4f58397ee/internal/campaign/recurse_workspace.go#L513-L544
[graph]: https://github.com/theRebelliousNerd/codenerd/blob/ef9fc03a591bdbcc368a2e0f6ce949c4f58397ee/internal/campaign/recurse_workspace.go#L134-L235
[crosscut]: https://github.com/theRebelliousNerd/codenerd/blob/ef9fc03a591bdbcc368a2e0f6ce949c4f58397ee/internal/campaign/recurse_workspace.go#L238-L260
[focus]: https://github.com/theRebelliousNerd/codenerd/blob/ef9fc03a591bdbcc368a2e0f6ce949c4f58397ee/internal/campaign/recurse_dag.go#L16-L77
[plan]: https://github.com/theRebelliousNerd/codenerd/blob/ef9fc03a591bdbcc368a2e0f6ce949c4f58397ee/internal/campaign/recurse_plan.go#L40-L67
[facts]: https://github.com/theRebelliousNerd/codenerd/blob/ef9fc03a591bdbcc368a2e0f6ce949c4f58397ee/internal/campaign/recurse_plan.go#L109-L134
[schedule]: https://github.com/theRebelliousNerd/codenerd/blob/ef9fc03a591bdbcc368a2e0f6ce949c4f58397ee/internal/core/defaults/policy/recurse.mg#L400-L453
[run]: https://github.com/theRebelliousNerd/codenerd/blob/ef9fc03a591bdbcc368a2e0f6ce949c4f58397ee/internal/campaign/recurse_cycle.go#L515-L549
[filter]: https://github.com/theRebelliousNerd/codenerd/blob/ef9fc03a591bdbcc368a2e0f6ce949c4f58397ee/internal/campaign/recurse_cycle.go#L754-L805
[mission]: https://github.com/theRebelliousNerd/codenerd/blob/ef9fc03a591bdbcc368a2e0f6ce949c4f58397ee/internal/campaign/recurse_mission.go#L182-L248
[guard]: https://github.com/theRebelliousNerd/codenerd/blob/ef9fc03a591bdbcc368a2e0f6ce949c4f58397ee/internal/campaign/recurse_mission_scope.go#L13-L83
[tests]: https://github.com/theRebelliousNerd/codenerd/blob/ef9fc03a591bdbcc368a2e0f6ce949c4f58397ee/internal/campaign/recurse_workspace_test.go#L113-L137
[prior-python]: https://github.com/theRebelliousNerd/codenerd/blob/ef9fc03a591bdbcc368a2e0f6ce949c4f58397ee/Docs/audits/recurse-python-from-import-prerequisites.md
[prior-js]: https://github.com/theRebelliousNerd/codenerd/blob/ef9fc03a591bdbcc368a2e0f6ce949c4f58397ee/Docs/audits/recurse-js-workspace-prerequisite-identity.md
[prior-go]: https://github.com/theRebelliousNerd/codenerd/blob/ef9fc03a591bdbcc368a2e0f6ce949c4f58397ee/Docs/audits/recurse-go-workspace-acquisition.md
[prior-namespace]: https://github.com/theRebelliousNerd/codenerd/blob/ef9fc03a591bdbcc368a2e0f6ce949c4f58397ee/Docs/audits/recurse-node-namespace-identity.md
[prior-language]: https://github.com/theRebelliousNerd/codenerd/blob/ef9fc03a591bdbcc368a2e0f6ce949c4f58397ee/Docs/audits/recurse-gate-language-domain.md
[portable]: https://github.com/theRebelliousNerd/codenerd/blob/ef9fc03a591bdbcc368a2e0f6ce949c4f58397ee/Docs/audits/portable-recurse-deliberation-crucible.md
[python]: https://docs.python.org/3/reference/import.html#regular-packages
[pep420]: https://peps.python.org/pep-0420/#specification
[build-systems]: https://www.microsoft.com/en-us/research/wp-content/uploads/2018/03/build-systems-final.pdf

