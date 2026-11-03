---
doc-class: research-proposal
subsystem: campaign-recurse-planning
implementation-status: proposed
last-verified: 2026-11-03
verified-against: fe7a7507e2b8a2673dbae1ce4d306884ffd94ae1
verification-method: static-source-inspection
---

# Recurse omits Cargo workspace-inherited dependency edges

## Decision

Preserve Cargo workspace inheritance when producing recurse's local dependency graph. At the inspected main, a member crate that inherits a local dependency through `workspace = true` becomes a node, but the inherited dependency does not become its edge. A focused sweep can therefore omit a prerequisite that its contract says it includes; a full sweep can visit the dependent before that prerequisite. The defect is at the host's graph observation, before the kernel orders work. [Producer][producer] [Closure contract][closure] [Ordering][ordering]

This is a **planning and work-selection gap, not proof of false-green verification**. Cargo still resolves and compiles its real dependencies when an applicable gate runs. Existing gate exits, campaign acceptance, write admission and the keep/revert ratchet retain their authority. The missing behavior is deliberate scheduling of an admitted local prerequisite, not a claim that Rust compilation ignores it.

The most bounded initial repair is to obtain a versioned, host-owned Cargo metadata observation, join declared local dependency paths to admitted package identities, and state whether the result is a declared prerequisite envelope or a configuration-resolved graph. Do not substitute an LLM's guessed dependency list, silently treat failed metadata as an empty complete graph, or widen mission write authority to compensate for missing visits.

**Evidence boundary:** this report is static source inspection and a conditional source trace. No application, Go or Cargo command, test, benchmark, model call, failure injection or implementation was run. Frequency, latency and task-quality effects are unmeasured.

## Source freeze and prior-art screen

GitHub's main branch resolved at 04:08 UTC on 2026-11-03 to [`fe7a7507e2b8a2673dbae1ce4d306884ffd94ae1`][main], committed at 03:57:08 UTC. All implementation links below use that pin. Root guidance, the public-repository rule, campaign guidance, campaign architecture README/internals/wiring/mission documentation, the audit queue and all current `recurse-*.md` audits were inspected. Older architecture descriptions were treated as intent or historical context, not as substitutes for current code. [Guidance][guidance] [Campaign guidance][campaign-guidance] [Architecture][architecture]

A bounded GitHub audit search returned no `workspace.dependencies` or `cargoPathDeps` matches. The single `scanRustCrates` match was the root-crate diagnostic-ownership audit. The eight Cargo-matching audit files were inspected, including the portable-crucible proposal, workspace-test receipt and recurse workspace-test witness reports. This is a documented search boundary, not a claim to have read every historical branch or discussion.

Relevant prior work deserves explicit credit:

- **Portable recurse deliberation/crucible** already warns that graph discovery is not full semantic dependency analysis and calls for explicit capability limits. Its examples include dynamic imports and external dependencies. The present witness needs neither: both crates and their ordinary local production relationship are explicitly declared in admitted Cargo manifests. This report supplies a concrete conformance case for that broader capability proposal. [Portable proposal][portable]
- **Go test dependency closure** establishes the adjacent principle that recurse's prerequisite graph must preserve relationships its work consumes. Current main now decodes and iterates `Imports`, `TestImports` and `XTestImports`; that omission is not being reopened. Rust inheritance uses a different producer and needs a cross-manifest join, not another Go JSON field. [Prior Go audit][prior-go] [Current Go producer][go-current]
- **Root-crate diagnostic ownership** concerns assigning a source diagnostic to a root crate. This witness uses unambiguous nested member directories, no `.` crate, and demonstrates the missing edge before any diagnostic exists. [Prior root audit][prior-root]
- **Workspace-test receipt invocation identity** concerns reusing evidence from a different execution directory. No receipt reuse, successful test, or foreign invocation is required here. [Prior receipt audit][prior-receipt]

## Contract and production route

`SubsystemNode.DependsOn` names nodes that must sweep first. `FilterDAG` follows these edges recursively so focus includes a node's transitive prerequisites. The CLI advertises `--subsystem` as a focus with dependencies pulled in. Both CLI and chat invoke the shared `RunRecurseCycles`; this is not only a display helper. [Closure contract][closure] [CLI flag][cli-flag] [CLI dispatch][cli] [Chat dispatch][chat]

Each live pass re-derives the graph, checks structural soundness over the full graph, stores it as `allNodes`, then applies focus and asserts the resulting graph. Planning uses the same derivation and closure machinery. The host turns every recorded dependency into `subsystem_depends(Node, Dep)`. The kernel marks a node ready when no recorded dependency remains unvisited and chooses the lexically least ready identity. [Live pass][pass] [Plan path][plan] [Fact projection][facts] [Ordering][ordering]

The direction is explicit: a recorded `a → z` means **a depends on z**, so z must visit first. It does not mean a must run before z. The graph's `addEdge(from, to)` records the target in the source node's dependency set and drops self-edges or endpoints not already admitted as nodes. [Graph edge][edge]

## Producer omission

Rust discovery is currently a textual two-pass scan:

1. Read collected `Cargo.toml` files. Only a file matching a `[package]` header becomes a crate node and enters the absolute-directory-to-node map.
2. Revisit the manifests. If a manifest's directory is not in that crate map, skip it completely.
3. In each remaining manifest, find every literal double-quoted `path = "..."` match. Resolve it relative to that manifest and add an edge only when the target directory is another admitted crate. [Complete Rust producer][producer]

Consequently, a virtual workspace's root manifest has no source node and is skipped in the second pass even if it defines `[workspace.dependencies]`. A member's `z.workspace = true` or `z = { workspace = true }` has no literal path for the regex to find. No map from inherited dependency names to workspace definitions exists in this producer, and no later graph stage fills the missing relation.

Cargo explicitly supports declaring dependencies in the workspace table and inheriting them in member dependency tables. This is ordinary supported manifest syntax, including normal, development and build dependencies, not an inferred runtime call or an unknown build system. [Cargo workspace dependencies](https://doc.rust-lang.org/cargo/reference/workspaces.html#the-dependencies-table) [Cargo inheritance](https://doc.rust-lang.org/cargo/reference/specifying-dependencies.html#inheriting-a-dependency-from-a-workspace)

This report isolates the missing inherited edge. The regex has other semantic limitations, such as not interpreting TOML sections or conditional activation; those are evaluation controls and design constraints below, not additional proven incident claims bundled into this finding.

## Conditional witness: equivalent Cargo dependency, different recurse graph

Use a valid, admitted, entirely local virtual workspace with no other source languages, no ignored or forbidden member directories and no additional relationship between the two crates:

```toml
# Cargo.toml
[workspace]
members = ["crates/a", "crates/z"]
resolver = "2"

[workspace.dependencies]
z = { path = "crates/z" }
```

```toml
# crates/a/Cargo.toml
[package]
name = "a"
version = "0.1.0"
edition = "2021"

[dependencies]
z.workspace = true
```

```toml
# crates/z/Cargo.toml
[package]
name = "z"
version = "0.1.0"
edition = "2021"
```

Let `crates/a/src/lib.rs` call a public function in `crates/z/src/lib.rs`. Both member manifests contain `[package]`; the root does not. The file contents are a proposed fixture, not an executed reproduction.

Static trace:

1. Collection admits all three manifests. Rust discovery creates `crates/a` and `crates/z`, but no root crate.
2. The root's local dependency definition is skipped because the root is not in the crate map. Neither member contains a literal `path` dependency. Therefore the graph contains no a-to-z edge.
3. `FilterDAG(..., ["crates/a"])` keeps a alone. It does not pull in z, because it only follows the absent `DependsOn` relation. Cross-cutting closing nodes are dependents, not prerequisites of a, so they are not pulled into this focused set either.
4. In a fresh unfocused pass, a and z are both ready. Their identity ranks put `crates/a` before `crates/z`, so the kernel visits the dependent first. A later z visit and the normal cross-cutting close remain possible.
5. Replace only a's dependency declaration with `z = { path = "../z" }`. That direct declaration denotes the same local dependency for this fixture, but now the current producer records a-to-z. Focus includes z and the kernel must visit z first. [Producer][producer] [Closure][closure] [Cross-cutting construction][close] [Facts][facts] [Ordering][ordering]

The direct-path variant is a strong control: the defect does not arise from unsupported Rust, missing files, an unavailable compiler, a cycle, or unknown node identity. The manifest representation alone changes the planner's dependency evidence.

## Downstream effect and limits

The minimal deterministic consequence is the wrong planned closure/order. It does not depend on the model choosing a particular edit, a gate passing, or mission admission succeeding.

The focused execution path can further turn the omission into lost repair selection. Detected Rust build, test and clippy gates are workspace-scoped. `scopeGateToNodes` only rewrites Go gates, so those Cargo argv remain unchanged by Rust focus. During finding attribution, however, a finding outside the focused set that belongs to another known node in `allNodes` is deliberately dropped as out of scope. A diagnostic located at `crates/z/src/lib.rs` can therefore be measured by Cargo yet excluded from the focused a sweep's work list, even though z is a real prerequisite of a. This consequence requires a parsed, correctly located z diagnostic; it is not assumed for location-free output. [Detected Rust gates][rust-gates] [Gate scoping][gate-scope] [Focused attribution][attribution]

That filtering is reasonable for genuinely unrelated subsystems. The remedy is not to indiscriminately retain every out-of-scope finding. It is to supply the missing dependency so the promised closure is accurate before filtering. The existing `open` helper still forwards each retained workspace finding attributed to the current node; it cannot recreate an omitted node or an already discarded finding. [Candidate handoff][open]

Do not equate diagnostic omission with a passing process: the stored result still carries Cargo's exit, `Passed`, output and errors. Independent host checks and the ratchet can reject an attempt. In the full sweep, z remains a node and is eventually visitable. A worker may also discover the dependency while diagnosing a, subject to its actual authority. No inevitable nontermination, acceptance of broken code, exact token loss or production incident is established.

Mission mode makes scope fidelity especially important without weakening this evidence boundary. An admitted attempt's mission receipt copies the current node's paths; the write guard measures containment and asks policy for admission. The policy requires an inside, unprotected path. Missing a z visit does not authorize an a-scoped attempt to write z. Preserve that guard and schedule the prerequisite explicitly rather than using scope expansion as a workaround. [Mission receipt][receipt] [Mission write check][write-check] [Mission policy][mission-policy]

## Existing protections and authored coverage

Preserve:

- Canonical workspace roots, membership refresh, hidden-harness exclusions and project-forbidden paths.
- Joining edges only to admitted local nodes rather than turning registry or out-of-workspace packages into writable targets.
- Self-edge handling, strongly connected component collapse and stable node identity rules.
- Full-graph structural validation before focus, kernel-owned readiness/order and re-derivation on later passes.
- Unknown-subsystem refusal, gate availability reporting, genuine process status and project-declared gate authority.
- Campaign acceptance, mission limits/protected inputs, write leases, effect ownership, cancellation and keep/revert rules. [Discovery][derive] [Collection][collect] [Edge handling][edge] [Graph validation][validate] [Pass][pass]

Structural validation checks the graph it receives for empty/duplicate IDs, unknown endpoints and self-dependencies. It cannot reject a semantic relationship that was never represented. Likewise, a topological oracle operating on that same incomplete graph can confirm ordering consistency without confirming acquisition completeness.

The authored Rust workspace test places a direct `path = "../core"` declaration in a member manifest and expects core before CLI. It also excludes the virtual root and build-output crate. Those are valuable controls; it contains no inherited-dependency case. Tests were read, not run, and this is not an assertion that every Rust-related test in the repository was searched exhaustively. [Current Rust fixture][test]

## Bounded metadata-based repair proposal

This section is a proposal, not a completed or evaluated implementation.

1. **Define the observation's meaning.** For a first version, produce a declared local prerequisite envelope from Cargo's normalized package metadata. Distinguish that from a resolved, feature/target-specific graph. Retain edge kind, optional/target conditions and the configuration identity needed to interpret it. A conservative union can include extra work, but it must not be presented as the exact executed dependency set.
2. **Use a versioned Cargo adapter.** Prefer machine-readable metadata over a larger regex bank. Specify format version and an explicit admitted manifest. A member-only, no-dependency-fetch mode is a candidate for this narrow join: inspect each package's local dependency path and manifest identity, including inheritance. Do not expect its `resolve` field to contain a graph; Cargo documents that field as null with `--no-deps`. A configuration-resolved graph is a separate, potentially more expensive acquisition. [Cargo metadata](https://doc.rust-lang.org/cargo/commands/cargo-metadata.html)
3. **Keep acquisition within authority.** Metadata collection is still a host subprocess, not permission to fetch, modify a lockfile, install a toolchain or execute arbitrary project commands. Use the authorized executable/environment, explicit directory, cancellation, bounded output and elapsed-time controls. Choose offline/lock-preserving behavior appropriate to the supported contract, and report cache or lock failures honestly. Do not retry by removing restrictions or deleting a lockfile.
4. **Admit paths before edges.** Canonicalize manifest and dependency locations against the selected workspace, apply current membership and project restrictions, join by admitted identity, and preserve renamed-package information. Do not infer membership from a dependency's display name or parse an opaque package ID as a filesystem path. External or excluded dependencies should be represented as boundary observations, not silently promoted into writable scope.
5. **Represent incomplete acquisition.** Tool absence, malformed output, unreadable inputs, unresolved inheritance or resource limits must not become a supposedly complete graph with zero dependencies. Retain a typed completeness/capability disposition. Policy can choose a disclosed conservative sweep, hold a dependency-complete claim, or request a narrower supported operation. Useful generic work remains possible without claiming full closure.
6. **Bind and refresh.** Associate the observation with manifest/configuration identity and source generation. Refresh or invalidate it when those inputs change. Keep the existing per-pass refresh; do not mutate an already admitted attempt's scope or reinterpret a saved visit under a new graph without an explicit transition. Adding edges may alter SCCs, node IDs, focus closure and mission scope.
7. **Leave executive control in Mangle.** Go acquires and validates package/dependency facts. Mangle retains readiness, ordering, admission and stop decisions. No second scheduler or model-authored dependency proof is required.

Cargo's metadata documentation distinguishes declared package dependencies from its resolved graph, provides local paths and manifest paths, and gives no-deps/format/lock/network controls. It does not by itself establish that a new adapter is correct under every toolchain or repository configuration. Adapter behavior needs the evaluation below. [Cargo metadata](https://doc.rust-lang.org/cargo/commands/cargo-metadata.html)

An alternate implementation can parse TOML and resolve workspace inheritance directly. That avoids a Cargo subprocess but must explicitly implement workspace-root lookup, dependency tables and aliases, path bases, invalid/missing definitions, and supported configuration limits. Simply scanning the virtual root and adding all its paths to every member would invent dependencies for members that never inherit them.

## Costs and failure modes

Metadata adds subprocess startup, JSON parsing, memory and potential cold-toolchain/cache failures. Fully resolved metadata can require a broader dependency acquisition than local declared relationships; do not conceal that cost or silently substitute one graph meaning for another. A declared union may schedule optional or inactive-target prerequisites and enlarge focused work. A narrower configuration graph can omit obligations relevant to a later test profile unless its identity and applicability are carried forward.

Canonicalization, membership and project exclusions remain necessary even for metadata emitted by a trusted executable, because manifests can name external locations. Treat new schema fields tolerantly while refusing unsupported identity semantics. Avoid joining by package name alone when names are renamed or repeated. Preserve clear failures for invalid Cargo manifests instead of falling back to an attractive but incomplete order.

No model call is inherently needed to repair this boundary. Any reduction in wasted model work is unmeasured and must be weighed against acquisition cost, larger closure, extra gate runs, scope changes and conservative holds.

## Fair evaluation plan — unrun

Use identical source fixtures and invocation profiles for the current producer and proposed adapter. Start with independently specified expected local edges, then compare planned closure and actual visited nodes through normal CLI/chat paths. A hand-injected graph or a topological oracle over the producer's own result is insufficient evidence of acquisition correctness.

| Case | Required observation |
|---|---|
| Primary virtual workspace, normal inherited local dependency | a-to-z edge, focused a includes z, z visits first. |
| Semantically equivalent direct member `path` declaration | Same admitted relationship and closure as inherited form. |
| Workspace root also has a package | Inherited relations belong to the members that declare them, not automatically to the root package. |
| An unused workspace dependency definition | No edge from a member that never inherits it. |
| Inherited development/build dependency | Preserve declared kind and documented prerequisite policy. |
| Renamed dependency and both inheritance spellings | Correct target identity, not name-only matching. |
| Optional/target-specific dependency | Truthful declared-envelope versus resolved-profile semantics. |
| Two-hop inherited local chain | Full transitive focused closure. |
| Genuine cycles and same-node references | Existing SCC/self-edge policy retained; no accidental identity collision. |
| Dependency outside root, ignored, forbidden or unavailable | No unauthorized writable node; explicit boundary or incomplete disposition. |
| Metadata unavailable, malformed, cancelled or oversized | No complete-empty success; useful recoverable outcome. |
| Manifest changes between acquisition, admission and next pass | Stale observation rejected/refreshed; existing scope not silently expanded. |
| Focused a with correctly located z compile diagnostic | z is an admitted prerequisite; diagnostic reaches its proper repair visit. |
| Focused a with unrelated x diagnostic | Existing out-of-scope behavior preserved. |
| Failing Cargo exit with incomplete/unlocated findings | Failure remains failure; no invented owner or successful verification. |
| Mission receipt and ordinary campaign controls | Scope, protection, leases and ratchet continue to constrain edits. |

For production-path verification, use an authorized disposable repository with valid local Cargo fixtures. Do not claim the application path passed merely because a unit fixture produced the expected edge. Separately test plan output, live focus/visit behavior, diagnostic routing and unchanged verification outcomes. Include initially passing and failing gates, and retain failed/abandoned runs in denominators.

Report missing/extra edges, focus membership, visit order, false out-of-scope classification, gate dispositions, added invocations, CPU/wall time, peak memory, output bytes and model-call/attempt counts. Separate deterministic planner correctness from model success. No benchmark or expected percentage improvement is supplied here.

**Completion condition:** equivalent direct and workspace-inherited local dependencies produce the same declared prerequisite relation, focused recurse includes the promised local closure, and verification authority remains independently grounded in executed evidence.

## Pinned source references

[main]: https://github.com/theRebelliousNerd/codenerd/commit/fe7a7507e2b8a2673dbae1ce4d306884ffd94ae1
[guidance]: https://github.com/theRebelliousNerd/codenerd/blob/fe7a7507e2b8a2673dbae1ce4d306884ffd94ae1/agents.md
[campaign-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/fe7a7507e2b8a2673dbae1ce4d306884ffd94ae1/internal/campaign/agents.md
[architecture]: https://github.com/theRebelliousNerd/codenerd/blob/fe7a7507e2b8a2673dbae1ce4d306884ffd94ae1/Docs/architecture/campaign/README.md
[producer]: https://github.com/theRebelliousNerd/codenerd/blob/fe7a7507e2b8a2673dbae1ce4d306884ffd94ae1/internal/campaign/recurse_workspace.go#L611-L644
[derive]: https://github.com/theRebelliousNerd/codenerd/blob/fe7a7507e2b8a2673dbae1ce4d306884ffd94ae1/internal/campaign/recurse_workspace.go#L60-L101
[collect]: https://github.com/theRebelliousNerd/codenerd/blob/fe7a7507e2b8a2673dbae1ce4d306884ffd94ae1/internal/campaign/recurse_workspace.go#L263-L319
[edge]: https://github.com/theRebelliousNerd/codenerd/blob/fe7a7507e2b8a2673dbae1ce4d306884ffd94ae1/internal/campaign/recurse_workspace.go#L131-L147
[close]: https://github.com/theRebelliousNerd/codenerd/blob/fe7a7507e2b8a2673dbae1ce4d306884ffd94ae1/internal/campaign/recurse_workspace.go#L235-L258
[closure]: https://github.com/theRebelliousNerd/codenerd/blob/fe7a7507e2b8a2673dbae1ce4d306884ffd94ae1/internal/campaign/recurse_dag.go#L16-L77
[plan]: https://github.com/theRebelliousNerd/codenerd/blob/fe7a7507e2b8a2673dbae1ce4d306884ffd94ae1/internal/campaign/recurse_plan.go#L38-L67
[validate]: https://github.com/theRebelliousNerd/codenerd/blob/fe7a7507e2b8a2673dbae1ce4d306884ffd94ae1/internal/campaign/recurse_plan.go#L82-L107
[facts]: https://github.com/theRebelliousNerd/codenerd/blob/fe7a7507e2b8a2673dbae1ce4d306884ffd94ae1/internal/campaign/recurse_plan.go#L109-L133
[ordering]: https://github.com/theRebelliousNerd/codenerd/blob/fe7a7507e2b8a2673dbae1ce4d306884ffd94ae1/internal/core/defaults/policy/recurse.mg#L420-L446
[pass]: https://github.com/theRebelliousNerd/codenerd/blob/fe7a7507e2b8a2673dbae1ce4d306884ffd94ae1/internal/campaign/recurse_cycle.go#L469-L515
[gate-scope]: https://github.com/theRebelliousNerd/codenerd/blob/fe7a7507e2b8a2673dbae1ce4d306884ffd94ae1/internal/campaign/recurse_cycle.go#L591-L643
[attribution]: https://github.com/theRebelliousNerd/codenerd/blob/fe7a7507e2b8a2673dbae1ce4d306884ffd94ae1/internal/campaign/recurse_cycle.go#L708-L777
[open]: https://github.com/theRebelliousNerd/codenerd/blob/fe7a7507e2b8a2673dbae1ce4d306884ffd94ae1/internal/campaign/recurse_cycle.go#L913-L937
[rust-gates]: https://github.com/theRebelliousNerd/codenerd/blob/fe7a7507e2b8a2673dbae1ce4d306884ffd94ae1/internal/gates/detect.go#L146-L153
[receipt]: https://github.com/theRebelliousNerd/codenerd/blob/fe7a7507e2b8a2673dbae1ce4d306884ffd94ae1/internal/campaign/recurse_mission.go#L212-L222
[write-check]: https://github.com/theRebelliousNerd/codenerd/blob/fe7a7507e2b8a2673dbae1ce4d306884ffd94ae1/internal/campaign/recurse_mission_scope.go#L13-L74
[mission-policy]: https://github.com/theRebelliousNerd/codenerd/blob/fe7a7507e2b8a2673dbae1ce4d306884ffd94ae1/internal/core/defaults/policy/mission.mg#L30-L32
[cli-flag]: https://github.com/theRebelliousNerd/codenerd/blob/fe7a7507e2b8a2673dbae1ce4d306884ffd94ae1/cmd/nerd/cmd_campaign_recurse.go#L81-L88
[cli]: https://github.com/theRebelliousNerd/codenerd/blob/fe7a7507e2b8a2673dbae1ce4d306884ffd94ae1/cmd/nerd/cmd_campaign_recurse.go#L253-L263
[chat]: https://github.com/theRebelliousNerd/codenerd/blob/fe7a7507e2b8a2673dbae1ce4d306884ffd94ae1/cmd/nerd/chat/campaign_recurse.go#L218-L236
[test]: https://github.com/theRebelliousNerd/codenerd/blob/fe7a7507e2b8a2673dbae1ce4d306884ffd94ae1/internal/campaign/recurse_workspace_test.go#L163-L182
[go-current]: https://github.com/theRebelliousNerd/codenerd/blob/fe7a7507e2b8a2673dbae1ce4d306884ffd94ae1/internal/campaign/recurse_workspace.go#L376-L463
[portable]: https://github.com/theRebelliousNerd/codenerd/blob/fe7a7507e2b8a2673dbae1ce4d306884ffd94ae1/Docs/audits/portable-recurse-deliberation-crucible.md#L23-L57
[prior-go]: https://github.com/theRebelliousNerd/codenerd/blob/fe7a7507e2b8a2673dbae1ce4d306884ffd94ae1/Docs/audits/recurse-go-test-dependency-closure.md
[prior-root]: https://github.com/theRebelliousNerd/codenerd/blob/fe7a7507e2b8a2673dbae1ce4d306884ffd94ae1/Docs/audits/recurse-root-crate-diagnostic-ownership.md
[prior-receipt]: https://github.com/theRebelliousNerd/codenerd/blob/fe7a7507e2b8a2673dbae1ce4d306884ffd94ae1/Docs/audits/workspace-test-receipt-invocation-identity.md
