---
title: Graph path results must preserve the search horizon
subsystem: store-graph-query
status: source-supported; proposed contract; evaluations unrun
source_commit: 4e599cb4f1c9e8a20847ff2281be3c2e1f5a8b95
review_date: 2026-11-05
---

# Graph path results must preserve the search horizon

## Finding and decision

**The graph adapter exposes a five-hop search miss as an unqualified Boolean path result.** `TraversePath` correctly documents its `ErrNoPath` as absence *within maxDepth*. `LocalStoreGraphAdapter.QueryGraph("path", ...)` fixes that depth at five, maps the sentinel to `false, nil`, and describes its result as “path existence as bool.” Neither the input contract nor the returned Boolean carries the horizon. A stored directed chain of six edges therefore has a path but receives the same successful `false` result as a genuinely disconnected pair. [Bounded sentinel][sentinel] [Adapter contract and implementation][adapter]

The narrow defect is **loss of the bounded proposition at the adapter boundary**, not a broken BFS, a storage failure, or proof of an incorrect autonomous decision. If the intended API is “reachable in at most five hops,” its Boolean is correct for that proposition; the missing piece is an explicit, enforceable public contract. It is not a sound unbounded reachability answer.

**Proposed decision:** preserve bounded work, but make the bound part of the query meaning and distinguish a completed negative search from a cutoff whenever callers ask about general reachability. Do not fix this by silently increasing the constant or running an unbounded search. Let the host report what was searched; let Mangle decide whether that evidence is sufficient or further work is justified.

This report inspects public main [`4e599cb4f1c9e8a20847ff2281be3c2e1f5a8b95`][pin]. It is documentation-only, based on source and test-definition inspection. No application, test, build, benchmark, inference, training, or runtime reproduction was executed. All witnesses and evaluation plans below are source-derived and unrun. Production frequency, downstream harm, and performance effects remain unverified.

## Existing work and distinct scope

Root guidance makes the model the creative center and Mangle the executive, and requires spec-first changes. Store guidance explicitly calls for query limits, provenance, cancellation and missing-result behavior. The store architecture lists a depth-parameterized BFS and its VirtualStore adapter as existing capabilities. This proposal retains those capabilities and their resource protections. [Root contract][root] [Spec-first requirement][spec-first] [Store guidance][store-guidance] [Store architecture][store-spec]

The earlier VirtualStore/GraphQuery QA report already identifies the adapter's fixed depth of five while discussing traversal cost. The bound is therefore not a new discovery. This audit isolates a different consequence: the helper's qualified negative becomes a Boolean with no bound, demonstrated at the first excluded path length. That older report's broad cascade hypotheses are not treated as runtime evidence here. In particular, the current visited map already prevents repeated vertex expansion; cycles do not require the depth cap to avoid revisiting vertices in a finite graph. [Earlier QA, section 14][old-qa] [Current visited map][bfs]

The existing retrieval audit addresses coverage and degraded observations in the task-retrieval pipeline, including subsequent implemented outcome facts. The KNN audit addresses backend result limits and suppressed iteration failures in a different query path. Those establish related evidence principles, not this graph adapter's hop-bound contract. The present six-edge witness requires neither stale evidence, missing rows, failed SQL, vector search nor model behavior. [Retrieval audit][retrieval-audit] [KNN audit][knn-audit]

The current `Docs/audits` inventory was checked before selecting this issue. No inspected audit addresses this `TraversePath` sentinel-to-Boolean boundary.

## Implemented, partial and proposed behavior

| Status | Behavior and qualification |
| --- | --- |
| Implemented | Directed BFS over stored outgoing `knowledge_graph` edges, with a visited/predecessor map and a configurable helper depth. |
| Implemented | The adapter's path query always passes depth five; it accepts `from->to` and returns a Boolean. |
| Implemented | Clean bounded misses are distinguished from reported storage failures at the helper and adapter boundaries. |
| Implemented | System boot attaches the LocalStore adapter to VirtualStore when a local database exists; direct external kernel queries have a live VirtualStore route. |
| Partial | The helper's within-depth qualification is not represented in the adapter's query parameters, result or general “path existence” description. |
| Proposed | An explicit bounded-reachability contract, plus found / complete-absence / incomplete outcomes for callers needing general reachability. |
| Not established | A current autonomous policy that consumes this particular path Boolean, an affected user task, frequency of paths beyond five hops, or measured quality/cost changes. |

## Exact producer-to-consumer boundary

### The traversal has a meaningful bounded contract

`TraversePath(from, to, maxDepth)` holds the LocalStore read lock, normalizes nonpositive depth to five, and starts a BFS at depth zero. It records each discovered vertex once in `cameFrom`. On dequeue it first checks whether the current vertex is the target; only afterward does it skip expansion at `current.depth >= maxDepth`. Thus a target at exactly the bound is accepted. [Traversal][bfs]

For vertices below the bound, the helper queries outgoing edges through `queryLinksLocked`, avoiding nested read locking. It records the first reported expansion failure and, if no target is found, returns a wrapped storage error instead of `ErrNoPath`. If no expansion error was reported, it returns the sentinel. The sentinel's comment explicitly says “unreachable within maxDepth.” This is a useful distinction and must be retained. [Expansion and termination][bfs-end]

The helper does not currently distinguish these two successful-search histories:

1. The reachable component was fully exhausted and the target was absent.
2. Search reached an unexpanded depth boundary, beyond which the target may exist.

Both are legitimate *within-bound misses*. They are not equivalent evidence for unrestricted absence.

### The adapter discards the qualification

The `"path"` switch arm parses `params["arg"]`, invokes `TraversePath(from, to, 5)`, and handles `errors.Is(err, ErrNoPath)` by returning `false, nil`. Other errors remain errors. A found nonempty path becomes `true`. The adapter accepts no depth setting in this arm and returns no visited depth, cutoff flag or coverage status. [Path arm][adapter-path]

The fixed five could be a deliberate product policy. That does not make it a completeness bound for the stored graph. A policy choosing how much work to do and evidence proving that nothing exists are different things. Even giving callers a depth parameter would not by itself solve this: a Boolean must still be defined as bounded reachability, or accompanied by completion information when answering the broader question.

### This is an installed interface, not an invented application route

`initExecutionLayer` creates VirtualStore, attaches the executive kernel, and installs `NewLocalStoreGraphAdapter(bctx.localDB)` when a local database is available. `SetKernel` connects VirtualStore back to the selected RealKernel, including the primary RealKernel of a CortexKernel. [Factory wiring][factory] [Kernel attachment][attachment] [Primary-kernel selection][primary-kernel]

`query_graph/3` is declared external. VirtualStore dispatches it to `getQueryGraphAtoms`, which extracts the query type and argument, calls the adapter and converts a Go Boolean to `ast.TrueConstant` or `ast.FalseConstant`. Its returned fact carries the original query arguments and that Boolean, with no horizon. `RealKernel.QueryContext` explicitly resolves direct external queries through this VirtualStore and returns live results instead of cached external rows. [Declaration][schema] [Dispatch][dispatch] [Bridge][bridge] [Direct query invocation][kernel-invoke] [Direct kernel query][kernel-query]

Accordingly, a direct `query_graph("path", "A->G", R)` against an initialized RealKernel with this real adapter can carry the same unqualified negative. This source trace does not require assuming that a generated rule compiles, that an autonomous planner chooses this predicate, or that a false result authorizes an action. No such downstream consequence was established in this review. The immediate finding is the installed query interface's semantics.

## Minimal causal witness

Use a healthy, unchanged LocalStore containing exactly these directed edges, with valid nonempty entity/relation fields and finite weights:

```text
A -> B -> C -> D -> E -> F -> G
```

Give every edge the same ordinary relation label. There are no shortcuts, concurrent writes, malformed rows, database failures or alternate paths. Query the actual LocalStore adapter with `QueryGraph("path", {"arg": "A->G"})`. This is a proposed fixture, not an executed result.

| Step | Source-derived state |
| --- | --- |
| Start | `A` is queued at depth 0. |
| Expand A through E | Each successful outgoing query discovers the next vertex; `F` is queued at depth 5. |
| Dequeue F | It is not `G`; the depth guard skips its expansion. The stored `F -> G` edge is never read. |
| Queue drains | No reported expansion error exists, so the helper returns wrapped `ErrNoPath`. |
| Adapter returns | `errors.Is` matches; the result is `false, nil`. No limit or incomplete status accompanies it. |

The graph contains a path of length six throughout. The result is correct only for the proposition “no path of at most five edges was found,” not “no path exists.” The cause is the combination of the fixed argument, depth guard and sentinel conversion, rather than graph indexing, ordering or storage reliability. [Bound and conversion][adapter-path] [Depth guard and completion][bfs-end]

Required controls sharpen the interpretation:

- **Five-edge positive:** the same graph queried from `A` to `F` succeeds because target equality is checked before the cutoff. This is not an off-by-one claim about the helper.
- **True negative:** an absent target `Z` queried from a shallow component whose entire reachable frontier is expanded also produces `false, nil`. The adapter does not distinguish this stronger evidence from the six-edge case.
- **Larger explicit helper bound:** direct `TraversePath("A", "G", 6)` should find the six-edge path. This isolates the adapter's fixed policy without implying that six is generally sufficient.
- **Cycle:** adding a back edge does not require repeated expansion of an already visited vertex. Preserve visited-state behavior rather than describing BFS as enumeration of all paths.
- **Storage failure:** closing the store remains an error at the adapter, not a healthy false result. Repairing cutoff semantics must not regress that distinction.

All controls are proposed and unrun. The scope is reachability in the stored directed graph, following all outgoing relation labels. Even a fully exhaustive negative in that graph would not establish that a real repository has no dependency or call path if its graph projection is incomplete. Relation filtering, weighted paths, graph-ingestion completeness and reflexive zero-hop semantics are separate contracts; this witness avoids them.

## What a sound result contract needs

### Option A: explicitly bounded reachability

If the supported question really is “is there a path within H hops?”, keep a Boolean but put H in the semantic contract: an explicit input or a clearly named, versioned fixed-horizon operation. Carry the effective bound across the external interface and diagnostics rather than hiding a default in the adapter. Distinguish a caller-supplied zero bound from “use default” if zero-hop queries are admitted; do not inherit the helper's nonpositive-default behavior accidentally.

Under this contract, a false answer for the six-edge fixture at H=5 is valid. Consumers must not promote it to unrestricted absence. This is the smallest compatibility-oriented repair when every legitimate caller only needs local connectivity. Merely updating a comment while leaving consumers unaware of the bound is insufficient for callers that require the broader proposition.

### Option B: general reachability with honest incomplete outcomes

For a query about general path existence under a resource budget, define at least:

| Outcome | Evidence required | Permitted conclusion |
| --- | --- | --- |
| `found` | A valid path within the declared stored-edge domain and graph observation. | A path exists in that domain; optional witness length/provenance supports inspection. |
| `not_found_complete` | All reachable eligible successors were exhaustively accounted for, or a valid completeness bound for this graph/query was met, in a declared consistent graph observation. | No path exists in that declared domain and observed state. |
| `incomplete` | Search stopped with unresolved frontier, depth/work budget, cancellation or unsupported coverage. | No conclusion of unrestricted absence; report reason and effective scope. |
| `error` | Storage/protocol failure prevented the promised observation. | Do not publish a healthy complete negative. |

These names are illustrative, not an implemented API. A depth boundary with unexpanded vertices conservatively requires `incomplete` unless other evidence certifies closure. Queue emptiness after deliberately discarding boundary expansions is not closure. An optional boundary-adjacency check could certify that no unvisited successor remains, but that is real additional database work and must count against the budget. The current reader also skips individual row-scan failures; a future completeness claim must account for topology-bearing rows that were skipped, not merely rely on every query returning nil error. [Reader behavior][link-reader]

The proposed result must identify what graph state its claim describes. If multiple reads or resumed passes can observe changes, use an appropriate snapshot, validated revision or explicitly weaker observation contract. An instance-local read lock is not by itself a cross-connection snapshot guarantee. This qualifies both complete negatives and positive path witnesses; it is not evidence of a current mutation race. The causal fixture above deliberately holds the graph unchanged.

The current BFS can return a found path despite an earlier unrelated expansion error. A valid positive witness can still establish existence, while shortest-path optimality or full search completeness may remain unknown. Do not discard useful positive evidence or promise stronger optimality as a side effect of adding statuses.

### Preserve the distinction through Mangle

An adapter-only typed cutoff error is not a complete integration repair: `getQueryGraphAtoms` currently logs adapter errors and returns no atoms with nil error. Zero facts do not expose a typed cutoff to the executive. Likewise, returning a new Go struct through the existing generic conversion would stringify it, not automatically create structured Mangle evidence. [Error handling and conversion][bridge]

Choose a declared predicate/result representation and test it through the real bridge. For example, use explicit outcome and effective-bound fields, or companion outcome facts keyed to the same query. Update declarations and typed conversion together. Preserve error visibility or explicitly represent failure; a negated empty relation must not silently stand in for complete graph absence. This is a required consequence of the proposed contract, not a new claim that a current policy already misuses negation.

For compatibility, introduce a versioned operation or explicitly retain the old Boolean as bounded-only while migrating callers that need general reachability. Do not silently change the type of `query_graph/3` results and assume existing rules will continue to unify. No new automatic broadening, retries, policy decisions or graph writes are authorized by a result envelope alone.

## Primary research grounding

Biere, Cimatti, Clarke and Zhu's *Symbolic Model Checking without BDDs* (TACAS 1999), section 5, distinguishes checking a particular bound from establishing a sufficient bound for a complete conclusion. Definition 19 and Theorem 20 connect reachability completeness to graph diameter. The applicable lesson is narrow: a bounded search can supply a positive witness, but a negative result needs its bound or a completeness argument. A convenient fixed depth is not that argument. [Primary paper][bmc]

CodeNERD's explicit BFS is not the paper's SAT-based model checker. This citation neither proposes replacing BFS with SAT nor transfers the paper's performance results to CodeNERD. Computing graph diameter merely to answer each path query would introduce substantial extra work; frontier exhaustion or honest bounded/incomplete semantics is the relevant local design choice.

## Full costs and failure modes

The present traversal performs one outgoing SQL query for each expanded vertex before success or cutoff. It materializes that vertex's outgoing links, including weights and decoded metadata, although reachability primarily needs adjacency and predecessors. The schema has an index on `entity_a`; no particular query plan or latency was measured. [Reader][link-reader] [Schema and indexes][graph-schema]

For inspected vertices V and adjacency rows E, the graph-bookkeeping component is O(V + E), with a visited/predecessor map and queue proportional to discovered vertices, plus materialized adjacency and metadata bytes. SQL round trips, index lookup/row scanning, decoding, allocation and logging add costs not captured by that graph bound. The five-hop cap limits distance, not the number of neighbors, visited vertices, SQL rows or elapsed time. A high-degree graph may still be large.

| Design choice | Cost or risk that must be counted |
| --- | --- |
| Explicit bound plus outcome | Small result/metadata overhead; schema, consumer and compatibility work. No automatic increase in search coverage. |
| Detect cutoff without expanding boundary | Cheap conservative accounting, but some actually disconnected graphs remain `incomplete`. This is honest uncertainty, not a proven regression in correctness. |
| Inspect boundary adjacency | Additional queries/rows and visited checks; can reduce unnecessary incomplete outcomes but consumes budget. |
| Larger or exhaustive search | More visited state, database work, allocations and latency; may approach the whole reachable graph. Raising H only moves the finite-chain counterexample if the result remains unqualified. |
| Repeat from the root at a larger bound | Re-reads previous levels and repeats decoding; count all attempts, not just the final successful pass. |
| Retain a resumable frontier | Saves repeated work but adds memory/lifetime ownership, cancellation, continuation validation and graph-state consistency obligations. It is optional, not required for the narrow repair. |
| Cache a result or completeness certificate | Requires graph identity/revision and invalidation semantics; a negative or diameter certificate cannot survive arbitrary edge changes by assumption. No new cache is needed for this proposal. |

`TraversePath` holds a read lock for the entire traversal; same-instance graph writes take the write lock. More search can therefore increase writer waiting. The graph interface has no context argument and the reader uses `db.Query`, so an expanded design cannot simply inherit a hard cancellation guarantee from a caller's context. Do not introduce unbounded traversal while leaving those ownership and cancellation questions implicit. [Traversal lock][bfs] [Write lock][writes] [Graph interface][interface]

Avoid fixing this by treating every miss as unknown forever: fully exhausted shallow components can support a complete negative. Conversely, do not let missing telemetry, a capped queue, skipped rows or a swallowed error manufacture completion. Define exactly which observations establish each status.

No inference is needed to repair or test this query contract. There is no measured token or dollar saving. If a later task study uses the result for reasoning, count status rendering, follow-on graph reads, evidence expansion, model calls, retries and verification. A shorter run that incorrectly declares absence is not an efficiency improvement.

## Evaluation plan, not executed

Existing tests deserve credit. The store suite distinguishes a clean miss from a closed-database failure and checks one-edge adapter true/false semantics. Another test asks for a depth-one search across two edges and checks for an error. These definitions protect bounded traversal and storage failure handling, but do not check whether the adapter exposes its horizon. [Miss/error and adapter tests][store-tests] [Depth-limit test][depth-test]

The existing VirtualStore multi-step test configures a mock graph backend that returns true for `A->C`, then directly queries the kernel; its commented hypothetical policy is not compiled. It checks the bridge's Boolean round trip, not the real LocalStore cutoff. Retain that coverage and add the real backend rather than counting the mock as a six-edge regression test. [Integration test][integration-test]

Proposed comparisons must state the question each treatment answers:

1. **Current adapter:** fixed depth five, Boolean. Record its result as current behavior, not as a correct unrestricted-reachability oracle.
2. **Explicit bounded Boolean:** same depth and stored graph; check equivalence for the bounded proposition, plus presence of the bound at every consumer boundary.
3. **Budgeted outcome contract:** same initial work budget; check sound positive and complete-negative claims, and truthful incomplete outcomes. Do not penalize honest unknowns as false negatives without also measuring resolved coverage.
4. **Optional exhaustive reference:** use finite, immutable fixture graphs with an independently specified expected reachability relation. This is an oracle for correctness and a separately measured cost baseline, not a production requirement to exhaust every graph.

| Fixture or route | Required assertion |
| --- | --- |
| Chains of 0, 1, 4, 5, 6 and longer edges | Define zero-hop semantics explicitly; preserve exactly-at-bound positives; never report unrestricted absence solely at cutoff. |
| Disconnected component exhausted below the bound | Permit a genuine complete negative without unnecessary broadening. |
| Unexpanded boundary with only back edges versus a new successor | Conservative incomplete is allowed; any claimed closure must be backed by the actual checked adjacency. |
| Cycles, self-loops, diamonds, multiple relation labels and high fan-out | No duplicate vertex expansion; declared edge domain and cost counters remain correct. |
| Direct helper, actual adapter, VirtualStore and initialized RealKernel | Effective horizon and outcome survive each real conversion; no mock substitutes for the adapter in the cutoff test. |
| Closed database, iterator failure and topology-row scan failure | Errors/incompleteness remain distinct from complete absence; retain valid positive-witness behavior under the chosen contract. |
| Real rules, if any consumer is introduced or migrated | Compile the actual rule and verify that incomplete/error outcomes cannot satisfy its complete-absence condition. A comment describing a rule is not this test. |
| Reopen the persisted fixture | Same graph and query contract after reopening; no accidental dependency on an in-memory mock or stale external row. |
| Larger-bound retry or continuation, if implemented | Count repeated work; preserve the same graph/query identity or explicitly restart on change. |
| Writer pressure and cancellation, if added | Measure reader/writer wait and cleanup; cancellation never returns complete absence. |

Report complete-negative precision, positive-witness validity, incomplete/error rate, resolved-query coverage, visited vertices, expanded vertices, rows read, SQL calls, metadata bytes, peak allocations, reader lock hold, writer wait and total query latency. Compare like-for-like budgets and count all failures, timeouts and retries. Report the workload's path-length and fan-out distributions before choosing a new default depth; no such distribution was measured here.

The completion gate is semantic, not “all queries return an answer”: every admitted operation states whether it answers bounded reachability or general reachability, and every result preserves the evidence needed for that claim. A six-hop path may remain outside a five-hop budget, but its exclusion must not become proof that the path does not exist. Before implementation, update the owning store/core capability and gap specifications; this document does not mark that work implemented or tested.

[pin]: https://github.com/theRebelliousNerd/codenerd/commit/4e599cb4f1c9e8a20847ff2281be3c2e1f5a8b95
[root]: https://github.com/theRebelliousNerd/codenerd/blob/4e599cb4f1c9e8a20847ff2281be3c2e1f5a8b95/agents.md#L1-L4
[store-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/4e599cb4f1c9e8a20847ff2281be3c2e1f5a8b95/internal/store/agents.md#L1-L18
[store-spec]: https://github.com/theRebelliousNerd/codenerd/blob/4e599cb4f1c9e8a20847ff2281be3c2e1f5a8b95/Docs/architecture/store/IMPLEMENTED_SPEC.md#L215-L225
[sentinel]: https://github.com/theRebelliousNerd/codenerd/blob/4e599cb4f1c9e8a20847ff2281be3c2e1f5a8b95/internal/store/local_graph.go#L11-L16
[adapter]: https://github.com/theRebelliousNerd/codenerd/blob/4e599cb4f1c9e8a20847ff2281be3c2e1f5a8b95/internal/store/local_graph_query.go#L26-L101
[adapter-path]: https://github.com/theRebelliousNerd/codenerd/blob/4e599cb4f1c9e8a20847ff2281be3c2e1f5a8b95/internal/store/local_graph_query.go#L81-L97
[bfs]: https://github.com/theRebelliousNerd/codenerd/blob/4e599cb4f1c9e8a20847ff2281be3c2e1f5a8b95/internal/store/local_graph.go#L173-L231
[bfs-end]: https://github.com/theRebelliousNerd/codenerd/blob/4e599cb4f1c9e8a20847ff2281be3c2e1f5a8b95/internal/store/local_graph.go#L229-L260
[link-reader]: https://github.com/theRebelliousNerd/codenerd/blob/4e599cb4f1c9e8a20847ff2281be3c2e1f5a8b95/internal/store/local_graph.go#L104-L160
[writes]: https://github.com/theRebelliousNerd/codenerd/blob/4e599cb4f1c9e8a20847ff2281be3c2e1f5a8b95/internal/store/local_graph.go#L76-L101
[factory]: https://github.com/theRebelliousNerd/codenerd/blob/4e599cb4f1c9e8a20847ff2281be3c2e1f5a8b95/internal/system/factory.go#L1654-L1670
[attachment]: https://github.com/theRebelliousNerd/codenerd/blob/4e599cb4f1c9e8a20847ff2281be3c2e1f5a8b95/internal/core/virtual_store.go#L503-L522
[schema]: https://github.com/theRebelliousNerd/codenerd/blob/4e599cb4f1c9e8a20847ff2281be3c2e1f5a8b95/internal/core/defaults/schemas_memory.mg#L68-L72
[dispatch]: https://github.com/theRebelliousNerd/codenerd/blob/4e599cb4f1c9e8a20847ff2281be3c2e1f5a8b95/internal/core/virtual_store.go#L655-L658
[bridge]: https://github.com/theRebelliousNerd/codenerd/blob/4e599cb4f1c9e8a20847ff2281be3c2e1f5a8b95/internal/core/virtual_store_graph.go#L21-L114
[kernel-query]: https://github.com/theRebelliousNerd/codenerd/blob/4e599cb4f1c9e8a20847ff2281be3c2e1f5a8b95/internal/core/kernel_query.go#L337-L367
[interface]: https://github.com/theRebelliousNerd/codenerd/blob/4e599cb4f1c9e8a20847ff2281be3c2e1f5a8b95/internal/types/interfaces.go#L245-L252
[graph-schema]: https://github.com/theRebelliousNerd/codenerd/blob/4e599cb4f1c9e8a20847ff2281be3c2e1f5a8b95/internal/store/local_core.go#L173-L185
[store-tests]: https://github.com/theRebelliousNerd/codenerd/blob/4e599cb4f1c9e8a20847ff2281be3c2e1f5a8b95/internal/store/local_store_unit_c_uplift_test.go#L37-L95
[depth-test]: https://github.com/theRebelliousNerd/codenerd/blob/4e599cb4f1c9e8a20847ff2281be3c2e1f5a8b95/internal/store/local_graph_extra_test.go#L48-L67
[integration-test]: https://github.com/theRebelliousNerd/codenerd/blob/4e599cb4f1c9e8a20847ff2281be3c2e1f5a8b95/tests/e2e/virtualstore_graphquery_integration_test.go#L666-L713
[old-qa]: https://github.com/theRebelliousNerd/codenerd/blob/4e599cb4f1c9e8a20847ff2281be3c2e1f5a8b95/.e2e_quality_assurance/2026-07-19_2018_EST_VirtualStore_GraphQuery_integration_analysis.md#L244-L252
[retrieval-audit]: https://github.com/theRebelliousNerd/codenerd/blob/4e599cb4f1c9e8a20847ff2281be3c2e1f5a8b95/Docs/audits/retrieval-freshness-coverage-and-context-value.md
[knn-audit]: https://github.com/theRebelliousNerd/codenerd/blob/4e599cb4f1c9e8a20847ff2281be3c2e1f5a8b95/Docs/audits/knn-recall-limit-error-contract.md
[bmc]: https://cca.informatik.uni-freiburg.de/papers/BiereCimattiClarkeZhu-TACAS99.pdf
[spec-first]: https://github.com/theRebelliousNerd/codenerd/blob/4e599cb4f1c9e8a20847ff2281be3c2e1f5a8b95/agents.md#L92-L108
[primary-kernel]: https://github.com/theRebelliousNerd/codenerd/blob/4e599cb4f1c9e8a20847ff2281be3c2e1f5a8b95/internal/core/virtual_store.go#L586-L595
[kernel-invoke]: https://github.com/theRebelliousNerd/codenerd/blob/4e599cb4f1c9e8a20847ff2281be3c2e1f5a8b95/internal/core/kernel_query.go#L138-L143
