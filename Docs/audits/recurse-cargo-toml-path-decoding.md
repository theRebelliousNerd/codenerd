---
doc-class: deep-dive
subsystem: campaign-recurse-cargo-path-decoding
implementation-status: partial
evidence-status: source-only
last-verified: 2026-11-04
main-rechecked: 2026-11-04T22:14Z
verified-against: 7278d0a6e6194a95c8df9e0861aea5244ac3a60f
verification-method: static-source-inspection
supersedes: []
---

# Decode Cargo path strings before constructing recurse prerequisites

## Finding and decision

Recurse's Cargo parser removes the quotes around a dependency path but leaves TOML escapes encoded. A valid direct dependency such as `z = { path = "../\u007a" }` denotes `../z` to Cargo. The graph producer instead tries to match the literal backslash-u spelling against admitted crate directories. With ordinary directories `a` and `z`, it discovers both nodes but omits the edge from a to z. Focusing a then omits z from the prerequisite closure, and the full sweep can select a before z. [String helper][decode] [Path acquisition][attrs] [Exact directory join][join] [Focused closure][filter]

Decode supported TOML strings according to their actual string kind before filesystem resolution. Keep the owner’s manifest bytes unchanged. A producer that cannot interpret a relevant declaration must expose that limitation rather than quietly equating an unresolved value with no local dependency. Preserve Mangle's scheduling authority, existing membership exclusions and all write, gate and adoption controls.

This is a concrete refinement of the earlier Cargo inheritance audit's acknowledged incomplete TOML semantics. It does not reopen the repaired ordinary inheritance join, introduce a new resolver requirement, or depend on selected patches. The new witness uses a directly supported dependency field and two equivalent spellings of the same value. [Inheritance addendum][prior-inherit] [Patch audit][prior-patch]

**Evidence boundary:** the review is pinned to public main [7278d0a6e6194a95c8df9e0861aea5244ac3a60f][pin]. Root and public-repository guidance, campaign guidance, campaign architecture, the recurse journey, the full audit-path inventory and neighboring reports were inspected before selecting this issue. Retained source bytes were checked against the pinned Git tree. This is static inspection and a conditional derivation. No application, repository test, build, Cargo command, gate, model call, training, benchmark or behavioral reproduction ran. No implementation, credential or setting changed.

## Intended behavior and current support

Root guidance makes the host responsible for observing the codebase and Mangle responsible for executive choices. The recurse graph's own contract says a node sweeps after its dependencies, and `FilterDAG` promises a focused set containing transitive prerequisites. The live driver rederives that graph each pass and asks the kernel for the next node. The intended fix belongs in the host's transduction of a manifest, not in a model's discretionary inference about which crate probably matters. [Root guidance][root] [Graph contract][graph-entry] [Node contract][node] [Live pass][pass]

| Capability | Status at the pin | Evidence boundary |
|---|---|---|
| Acquire admitted Cargo manifests and create crate nodes | Implemented | A `[package]` section causes the containing directory to become a Rust node. [Collection][collect] [Nodes][nodes] |
| Resolve direct local dependency paths | Partial | Ordinary unescaped values join correctly; the helper does not decode escaped basic strings. [Attributes][attrs] [Join][join] |
| Join workspace-inherited local dependencies | Implemented for supported parsed forms | Existing workspace joins remain present. Their use of the same string helper does not supply full TOML conformance. [Inheritance][inherit] |
| Keep prerequisites in a focused sweep | Implemented over the acquired graph | Closure cannot traverse an edge that acquisition omitted. [Filter][filter] |
| Order nodes through Mangle | Implemented over the acquired graph | Readiness and tie-breaking use asserted dependency and ordinal facts. [Projection][facts] [Rules][order] |
| Report a scalar-decoding failure or unresolved local-path deficit | Missing on this path | The parser returns an info object without an error; a failed directory lookup simply adds no edge. [Parser return][parser-return] [Join][join] |
| Conforming decoded-value admission with qualified incomplete results | Proposed here | No implementation or executed verification is claimed. |
| Frequency and practical task-quality impact | Unverified | No real workload census or runtime trial was performed. |

The graph is a work-selection model. Cargo's build dependency resolution and CodeNERD's gate results are separate. Missing a deliberate recurse visit does not mean Cargo will omit compilation of that dependency, bypass a failing gate, or make a bad change inevitable. A fresh full graph that is internally acyclic can still be incomplete relative to the source declarations it claims to represent.

## The producer to consumer chain

### Quotes are recognized without decoding their contents

`DeriveWorkspaceDAG` canonicalizes the workspace, refreshes membership, collects eligible source files and passes the admitted Cargo manifests to `scanRustCrates`. That scanner first reads each manifest and records every parsed package directory in `crates`, independently of its dependency edges. This is why both witness nodes can exist while their relationship is lost. [Entry][graph-entry] [Collection][collect] [Nodes][nodes]

The custom parser recognizes ordinary dependency tables and routes an inline `path` attribute to `applyAttr`. Its lexing helpers track whether a backslash escapes a following character while splitting keys and fields. They preserve those bytes; they do not turn a Unicode escape into its scalar value. `unquoteTOMLString` subsequently removes one leading and trailing double or single quote and returns the interior unchanged. [Dependency tables][tables] [Inline parser][inline] [Scalar helper][decode]

For a direct path, `applyAttr` stores that interior in `cargoDepSpec.path`. `scanRustCrates` applies `filepath.FromSlash`, `Join` and `Clean`, then adds an edge only when the resulting path is a key in the previously acquired `crates` map. Filesystem path normalization is not TOML decoding. A lookup miss carries neither an error nor an unresolved-dependency record. [Path acquisition][attrs] [Directory join][join]

Cargo documents its manifests as TOML and permits local dependency paths. TOML basic strings support Unicode escapes, whereas single-quoted literal strings preserve their contents without escape processing. Thus the required transformation is determined by the string kind, before path interpretation. These primary specifications establish the input's meaning, not an executed Cargo result from this review. [Cargo manifest format][cargo-manifest] [Cargo path dependencies][cargo-path] [TOML strings][toml]

### The omitted edge remains omitted through a sound graph walk

`FilterDAG` starts with the requested IDs and recursively follows their `DependsOn` slices. It correctly refuses a named subsystem absent from its index. The witness names an existing a, so this safeguard is satisfied. There is no missing dependency ID to reject: the parser never emitted that edge. [Filter][filter]

`validateSweepGraph` rejects empty or duplicate node IDs, unknown dependency IDs and self-dependencies. Those structural checks also pass for two independent nodes. `sweepGraphFactList` assigns lexicographic ordinals and emits only observed edges. Mangle makes a node ready when no asserted dependency remains unvisited, then chooses the least ready ordinal. It cannot distinguish an actually independent node from one whose manifest relationship was lost upstream. [Graph validation and facts][facts] [Ordering rules][order]

The same acquisition feeds the dry plan and the live pass. `RecurseSweepOrder` first orders the full derived graph to check soundness, then filters and orders the focused graph. The live `pass` separately derives it, checks graph soundness, applies the focus, and selects each next node through policy. This is therefore not merely a display discrepancy. Both CLI and chat invoke the shared cycle driver. [Plan][plan] [Pass][pass] [CLI][cli] [Chat][chat]

## Minimal conditional witness

Use an ordinary admitted Rust workspace on a POSIX filesystem, with no `go.mod`, no excluded paths, no symlinks, no patches, no inherited dependencies and no scan or read failures. All files are stable and readable. The directory layout is:

```text
Cargo.toml
a/Cargo.toml
a/src/lib.rs
z/Cargo.toml
z/src/lib.rs
```

The root manifest is:

```toml
[workspace]
members = ["a", "z"]
resolver = "2"
```

The a manifest is:

```toml
[package]
name = "a"
version = "0.1.0"
edition = "2021"

[dependencies]
z = { path = "../\u007a" }
```

The z manifest contains the corresponding package metadata with `name = "z"`. Its library exports `pub fn value() -> u8 { 1 }`; a's library may call `z::value()`. These are proposed fixture contents only. The a manifest contains a literal backslash followed by `u007a`, not a pre-decoded z and not a doubled backslash.

Compare two otherwise byte-identical fixtures: **plain** has `path = "../z"`; **escaped** has the path shown above. The TOML value is identical in both. The source-derived observations are:

| Boundary | Plain spelling | Escaped spelling at the pin |
|---|---|---|
| Admitted package nodes | a and z | a and z |
| Helper result | `../z` | literal `../\u007a` |
| Join target | admitted directory z | a distinct spelling absent from the crate map |
| Acquired edge | a depends on z | no a-to-z edge |
| Focused a closure | z and a | a only |
| First ordinary node in a fresh full pass | z | a, by lexicographic tie-break |

This follows from the inspected functions and rules, not from running either fixture. The virtual workspace root contributes no package node; cross-cutting closing nodes do not repair the missing relationship. With unescaped paths, the existing direct-path design can produce the intended ordering. [Nodes and join][nodes] [Component graph][components] [Focused closure][filter] [Ordering][order]

A useful second control uses `path = '../z'`: it should also resolve to z. By contrast, `path = '../\u007a'` is a literal-string value, so its backslash-u sequence must remain literal. A blanket replacement of `\u007a` everywhere would make that negative control wrong. The correction must preserve decoded-value identity without merging distinct literal values. [TOML strings][toml]

For the full-pass consequence, both ordinary nodes initially have no visited facts and a sorts before z. For the focused consequence, omission is deterministic without relying on that tie-break. If the operator explicitly includes both nodes, z remains eligible and will be selected if the pass completes, although their prerequisite order remains wrong. If another correctly acquired edge independently brings z into the focus or blocks a, it can mask the particular manifestation. Neither possibility restores the missing direct edge.

## Scope and overlap

The audit directory contained 388 Markdown paths at the pin. The inventory, audit queue and targeted Cargo, dependency, path-identity and recurse reports were screened; relevant current bodies were read. This is a bounded overlap review, not a claim to have exhausted every historical discussion.

- **Cargo inherited dependency closure:** its original absence-of-inheritance finding has a current-main repair. Its addendum already warns that the custom parser does not establish complete TOML semantics. This report supplies the precise escaped-path witness and a finite decoding acceptance boundary for that open qualification. It does not claim the broader concern is new. [Earlier report][prior-inherit]
- **Cargo selected patch prerequisites:** that report requires resolution of package-source overrides. Here Cargo has only a direct local path; a conforming scalar decode is enough to recover the edge without adding a patch resolver. [Patch report][prior-patch]
- **Cargo default-member evidence:** that report concerns whether a real workspace test covers the selected crate. Here the defect occurs before gate selection, and no successful test is used as evidence. [Applicability report][prior-members]
- **Node namespace and operand fidelity:** those reports concern node-ID collisions or a correct node path changing command-line meaning. This witness uses plain a and z directory names and no ambiguous executable operand. The loss is manifest value decoding before directory matching. [Namespace report][prior-namespace] [Operand report][prior-operands]
- **Passing coverage capture completeness:** the immediately preceding report concerns metric extraction from a bounded retained log. This finding uses complete, tiny manifest input and no metric or process output. [Coverage report][prior-coverage]

The same helper is used for workspace path attributes, package values and quoted key segments. Those uses merit regression coverage, but this report does not claim an independently demonstrated failure at every call site. In particular, a bad explicit workspace path could be masked by ancestor lookup, and a package value need not participate in a direct-path join. The direct-path witness needs none of those interactions. [Other helper callers][other-callers] [Workspace lookup][inherit]

## Proposed contract and bounded repair

Add a named campaign specification gap for decoded Cargo path identity, with the following exit: equivalent valid supported TOML encodings produce the same admitted graph, focused closure and kernel order; literal values remain distinct; malformed or unsupported relevant declarations cannot masquerade as complete empty relationships. The architecture documentation standard calls for that checkable distinction between target behavior and current evidence. [Documentation standard][doc-standard]

The producer contract should be explicit:

1. **Decode first.** Parse the field as the declared TOML string kind. Return the decoded value plus source location, or an error/unsupported disposition. Do not return raw encoded interior as a successful semantic value.
2. **Resolve second.** Resolve a direct dependency relative to the owning manifest; resolve an inherited workspace declaration relative to its workspace declaration. Preserve this already-correct distinction. [Current direct/workspace bases][attrs]
3. **Admit without widening scope.** Join the decoded resolved path to admitted crate identity. External, forbidden or excluded targets stay outside the sweep. Record enough reason to distinguish that intentional boundary from a failed decode or unresolved expected local crate.
4. **Publish qualified observations.** Graph structural validity and acquisition completeness remain separate. A partial manifest observation must not imply all prerequisites were observed. Carry precise diagnostics to plan/live entry points rather than asking the model to guess missing edges.
5. **Retain policy ownership.** Feed corrected nodes and edges into the existing fact producer and Mangle order. No new Go-selected execution order, extra write scope, altered gate or relaxed mission oracle is needed.

A conforming TOML parser used for the relevant manifest fields is the preferred bounded repair. It handles language semantics in one place, preserves source diagnostics, and avoids accumulating ad hoc escape replacements. Its accepted TOML version and relevant Cargo field rules still need to be specified and tested. If a temporary subset parser remains, unsupported valid strings must be reported explicitly; adding only the one illustrated Unicode spelling is not a sufficient conformance claim.

Do not substitute a generic Go or JSON string unquoter without qualifying its accepted and rejected syntax against TOML. Do not pre-decode the whole file before tokenization, because an escape may represent punctuation that should remain within its original string. Do not normalize all string kinds alike or rewrite the owner’s manifest as a workaround.

Cargo metadata is a stronger alternative when the desired contract expands to actual package resolution. Its documented JSON includes workspace/package information, dependency paths and resolution data; its versioned format should be requested explicitly. It is a different operational choice, not a zero-cost substitute for local parsing. Offline/locked operation, missing packages, toolchain availability, feature/target scope and incomplete results require an explicit acquisition policy. This audit ran no metadata command. [Cargo metadata][cargo-metadata]

The conceptual boundary follows Mokhov, Mitchell and Peyton Jones, *Build Systems à la Carte*: scheduling and dependency/rebuild decisions are separately meaningful parts of a build system. Applied here, a correct order over the host's graph cannot establish that the graph preserved the source's dependency semantics. That is an engineering application of the paper's separation, not a theorem about CodeNERD or a measured benefit. [Primary literature][build-systems]

## Costs and failure modes

Let B be the total bytes of admitted Cargo manifests, V the admitted package nodes, and E the retained local edges. A conforming parser adds decoding/validation work and possibly an object representation of the manifests. For a conventional linear parser, expected parse work is proportional to B; its actual allocation constants, peak memory and diagnostic storage must be measured. Decoded-string storage and graph edges add their own byte cost. Neither that model nor the source review establishes a wall-time improvement.

The existing graph pipeline still pays for workspace membership refresh, filesystem traversal, file reads, path resolution, component construction, sorting, fact assertion and kernel queries. Full reparsing per pass charges B again. A future cache must include manifest bytes and parser/schema identity and preserve completeness; no cache is required for this repair. Do not attribute omitted prerequisite visits to an optimization win.

Correct closure can increase the number of visits, gate executions and model attempts in a focused run. Those are previously omitted obligations, not free work. If k additional prerequisite visits occur, account for each visit's source acquisition, baseline/after checks, prompt compilation, model input/output, retries, acceptance checks and possible compensation. There is no justified fixed token or dollar saving. Fewer misordered repairs or repeated passes are hypotheses requiring paired experiments.

A metadata alternative additionally pays for process startup, Cargo graph/resolver work, output acquisition and decoding, possible lock/index/cache work and any allowed network activity. Cold and warm costs must be separated. Cancellation must join owned acquisition; a failed or partial metadata result cannot become an empty complete dependency set.

Important regressions to avoid are double-decoding literal paths, accepting invalid escapes, mixing quoted-key identity with raw syntax, path normalization before decoding, accidental read/sweep admission outside the workspace, loss of ordinary inheritance, and reporting a syntactically valid graph as semantically complete after parse failure. Full resolver fidelity, feature activation, Cargo membership equivalence and symlink identity remain separate qualifications. Repairing scalar semantics alone should not be advertised as solving them.

## Evaluation plan with no results claimed

The inspected direct-path fixture asserts dependency-first ordering for an ordinary `../core` value. The inheritance fixture asserts focused closure and order for plain workspace paths. They are useful positive controls, but neither quoted source range exercises the escaped-basic-string witness. Their assertions were read, not run. [Direct-path fixture][test-direct] [Inheritance fixtures][test-inherit]

Compare three implementations on identical input bytes and workspace policy: current code, the proposed decoded-manifest adapter, and a separately qualified reference derived from conforming TOML values. Use a Cargo metadata oracle only in a later authorized environment with a pinned toolchain and declared operational mode. Keep declared dependency-envelope comparisons separate from feature-resolved graph comparisons.

| Proposed control | Required observation |
|---|---|
| Plain versus Unicode-escaped direct path | Same decoded path, node set, a-to-z edge, focused closure and order |
| Single-quoted plain versus escape-looking literal | Correct string-kind distinction; no accidental Unicode decoding of the literal |
| Inline versus dependency subtable and dotted attribute | Equivalent supported forms preserve the same relationship |
| Ordinary workspace inheritance with an escaped workspace path value | Preserve declaration-relative resolution and the existing inheritance control |
| Invalid escape or unfinished string | Specific failed/unsupported acquisition, never complete empty edges |
| Excluded or external crate | No expanded read, visit or mutation authority |
| No local dependency and unrelated crate | No invented edge and no unnecessary prerequisite visit |
| Reordered fields, comments and manifest enumeration | Stable decoded identity and ordering |
| Manifest change between passes | Fresh graph or explicit stale/incomplete status; no reused old edge claim |
| Plan and live driver | Both expose the same admitted prerequisites; actual node-selection facts agree |

Start with parser and graph fixtures using only temporary manifest/source files. Then exercise `DeriveWorkspaceDAG`, `FilterDAG`, fact projection and the real ordering policy. A later complete-cycle fixture should record selected nodes with a scripted executor and truthful checks, without treating a script's prose as empirical evidence. Preserve the normal gate and authorization contracts; do not disable them to obtain an expected visit order.

Report missing and invented edges, prerequisite inclusion, order violations, explicit incompleteness, stable-node identity, parse failures and false refusals before comparing performance. Charge all acquisition, reference-oracle, kernel, gate, retry, model and persistence work to the relevant strategy. Hold the task, model, permissions, feature/target configuration and overall budget fixed in any later quality trial. Publish observed counts and uncertainty rather than inferring quality from a larger graph or a lower token total.

The exit is a source-semantics invariant with production-path evidence: equivalent supported encodings cannot change which admitted prerequisites recurse visits. A passing scalar unit test alone does not establish that the live loop consumed its corrected graph.

[pin]: https://github.com/theRebelliousNerd/codenerd/commit/7278d0a6e6194a95c8df9e0861aea5244ac3a60f
[root]: https://github.com/theRebelliousNerd/codenerd/blob/7278d0a6e6194a95c8df9e0861aea5244ac3a60f/agents.md#L1-L63
[graph-entry]: https://github.com/theRebelliousNerd/codenerd/blob/7278d0a6e6194a95c8df9e0861aea5244ac3a60f/internal/campaign/recurse_workspace.go#L25-L100
[collect]: https://github.com/theRebelliousNerd/codenerd/blob/7278d0a6e6194a95c8df9e0861aea5244ac3a60f/internal/campaign/recurse_workspace.go#L265-L316
[nodes]: https://github.com/theRebelliousNerd/codenerd/blob/7278d0a6e6194a95c8df9e0861aea5244ac3a60f/internal/campaign/recurse_workspace.go#L627-L704
[attrs]: https://github.com/theRebelliousNerd/codenerd/blob/7278d0a6e6194a95c8df9e0861aea5244ac3a60f/internal/campaign/recurse_workspace.go#L768-L787
[join]: https://github.com/theRebelliousNerd/codenerd/blob/7278d0a6e6194a95c8df9e0861aea5244ac3a60f/internal/campaign/recurse_workspace.go#L675-L704
[inherit]: https://github.com/theRebelliousNerd/codenerd/blob/7278d0a6e6194a95c8df9e0861aea5244ac3a60f/internal/campaign/recurse_workspace.go#L649-L733
[tables]: https://github.com/theRebelliousNerd/codenerd/blob/7278d0a6e6194a95c8df9e0861aea5244ac3a60f/internal/campaign/recurse_workspace.go#L789-L887
[parser-return]: https://github.com/theRebelliousNerd/codenerd/blob/7278d0a6e6194a95c8df9e0861aea5244ac3a60f/internal/campaign/recurse_workspace.go#L871-L895
[inline]: https://github.com/theRebelliousNerd/codenerd/blob/7278d0a6e6194a95c8df9e0861aea5244ac3a60f/internal/campaign/recurse_workspace.go#L1077-L1139
[decode]: https://github.com/theRebelliousNerd/codenerd/blob/7278d0a6e6194a95c8df9e0861aea5244ac3a60f/internal/campaign/recurse_workspace.go#L1142-L1154
[other-callers]: https://github.com/theRebelliousNerd/codenerd/blob/7278d0a6e6194a95c8df9e0861aea5244ac3a60f/internal/campaign/recurse_workspace.go#L999-L1045
[components]: https://github.com/theRebelliousNerd/codenerd/blob/7278d0a6e6194a95c8df9e0861aea5244ac3a60f/internal/campaign/recurse_workspace.go#L148-L261
[node]: https://github.com/theRebelliousNerd/codenerd/blob/7278d0a6e6194a95c8df9e0861aea5244ac3a60f/internal/campaign/recurse_dag.go#L5-L33
[filter]: https://github.com/theRebelliousNerd/codenerd/blob/7278d0a6e6194a95c8df9e0861aea5244ac3a60f/internal/campaign/recurse_dag.go#L36-L76
[facts]: https://github.com/theRebelliousNerd/codenerd/blob/7278d0a6e6194a95c8df9e0861aea5244ac3a60f/internal/campaign/recurse_plan.go#L82-L132
[order]: https://github.com/theRebelliousNerd/codenerd/blob/7278d0a6e6194a95c8df9e0861aea5244ac3a60f/internal/core/defaults/policy/recurse.mg#L425-L446
[plan]: https://github.com/theRebelliousNerd/codenerd/blob/7278d0a6e6194a95c8df9e0861aea5244ac3a60f/internal/campaign/recurse_plan.go#L36-L67
[pass]: https://github.com/theRebelliousNerd/codenerd/blob/7278d0a6e6194a95c8df9e0861aea5244ac3a60f/internal/campaign/recurse_cycle.go#L515-L603
[cli]: https://github.com/theRebelliousNerd/codenerd/blob/7278d0a6e6194a95c8df9e0861aea5244ac3a60f/cmd/nerd/cmd_campaign_recurse.go#L253-L278
[chat]: https://github.com/theRebelliousNerd/codenerd/blob/7278d0a6e6194a95c8df9e0861aea5244ac3a60f/cmd/nerd/chat/campaign_recurse.go#L218-L266
[test-direct]: https://github.com/theRebelliousNerd/codenerd/blob/7278d0a6e6194a95c8df9e0861aea5244ac3a60f/internal/campaign/recurse_workspace_test.go#L163-L183
[test-inherit]: https://github.com/theRebelliousNerd/codenerd/blob/7278d0a6e6194a95c8df9e0861aea5244ac3a60f/internal/campaign/recurse_audits_test.go#L483-L571
[doc-standard]: https://github.com/theRebelliousNerd/codenerd/blob/7278d0a6e6194a95c8df9e0861aea5244ac3a60f/Docs/journeys/09-architecture-doc-standard.md#L10-L83
[prior-inherit]: https://github.com/theRebelliousNerd/codenerd/blob/7278d0a6e6194a95c8df9e0861aea5244ac3a60f/Docs/audits/recurse-cargo-inherited-dependency-closure.md#L11-L21
[prior-patch]: https://github.com/theRebelliousNerd/codenerd/blob/7278d0a6e6194a95c8df9e0861aea5244ac3a60f/Docs/audits/recurse-cargo-selected-patch-prerequisites.md
[prior-members]: https://github.com/theRebelliousNerd/codenerd/blob/7278d0a6e6194a95c8df9e0861aea5244ac3a60f/Docs/audits/recurse-cargo-default-member-evidence.md
[prior-namespace]: https://github.com/theRebelliousNerd/codenerd/blob/7278d0a6e6194a95c8df9e0861aea5244ac3a60f/Docs/audits/recurse-node-namespace-identity.md
[prior-operands]: https://github.com/theRebelliousNerd/codenerd/blob/7278d0a6e6194a95c8df9e0861aea5244ac3a60f/Docs/audits/recurse-node-operand-fidelity.md
[prior-coverage]: https://github.com/theRebelliousNerd/codenerd/blob/7278d0a6e6194a95c8df9e0861aea5244ac3a60f/Docs/audits/recurse-passing-coverage-capture-completeness.md
[cargo-manifest]: https://doc.rust-lang.org/cargo/reference/manifest.html
[cargo-path]: https://doc.rust-lang.org/cargo/reference/specifying-dependencies.html#specifying-path-dependencies
[toml]: https://toml.io/en/v1.0.0#string
[cargo-metadata]: https://doc.rust-lang.org/cargo/commands/cargo-metadata.html
[build-systems]: https://www.microsoft.com/en-us/research/wp-content/uploads/2018/03/build-systems-final.pdf
