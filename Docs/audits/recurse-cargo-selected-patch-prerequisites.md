---
doc-class: deep-dive
subsystem: campaign-recurse-planning
implementation-status: proposed-refinement
evidence-status: static-source-inspection
last-verified: 2026-11-03
verified-against: e3a4705258b80affec27bca47eddce54ba11ec07
supersedes: []
---

# Preserve selected local Cargo patches in recurse prerequisites

## Finding and decision

Recurse discovers local Rust crates, but it connects a consumer to another crate only through a direct dependency path or workspace-inherited path. It does not observe Cargo's selected package after a root `[patch]` override. An ordinary version dependency that Cargo resolves to an admitted local patched crate can therefore have no recurse edge to that crate. A focused sweep can omit the actual prerequisite, and an unfocused sweep can schedule the consumer first. [Rust edge acquisition][rust-scan] [Manifest parser][parser] [Focused closure][closure] [Ordering policy][order]

**Classification: a source-grounded portability and planning-contract gap, conditional on Cargo selecting the local patch.** The scanner implements its stated direct/inherited-path subset; this report proposes extending that subset or exposing its incompleteness before making a dependency-complete focus claim. It is not a claim that Cargo builds the wrong dependency, that an unused patch must always be scheduled, that an implementation test passed incorrectly, or that a production failure was observed.

The recommended boundary is the **selected dependency identity**, joined to already admitted local crate paths. A patch declaration alone is not sufficient. Preserve the existing Mangle scheduler, independent gates, mission write restrictions and keep/revert decision. Do not ask the proposer to repair the host's missing dependency by widening its own scope.

### Source and execution boundary

Public main resolved to **e3a4705258b80affec27bca47eddce54ba11ec07**, committed November 3, 2026 at 12:24:28 UTC, and was rechecked unchanged at 12:47 UTC before freezing this report. Repository references below use that immutable pin. This review inspected public guidance, architecture, source, neighboring audits and authored tests. It ran no repository program, build, test, Go or Cargo command, model inference, benchmark, gate or failure injection. No implementation, repository file, credential, permission or setting was changed. The manifests, contracts and evaluations below are proposed and unrun. [Source commit][pin]

## 1. Why this matters to the Socratic requirements path

The design deliberately separates creative reasoning from executive control. Root guidance assigns synthesis to the model and planning, context and safety to Mangle. The mission contract gives the proposer and requirements interrogator the selected node, mission and evidence, but forbids them from choosing another write scope or replacing a gate. Dependency order remains recurse policy. These are useful boundaries, not restrictions to bypass when a dependency is missing. [Root guidance][guidance] [Mission contract][mission-contract]

The production order makes the distinction concrete:

1. Recurse derives the workspace graph and filters it to the requested focus plus recorded prerequisites.
2. The kernel selects a node and the driver creates its attempt.
3. Mission preparation sends that attempt to the existing deliberation service, then checks the returned design and source/vision admission before execution.

The mission receipt copies the already selected node's paths. Its write guard separately checks canonical containment and protected inputs. An excellent critique of an `a`-scoped design can discover that a local patched library needs attention, but it cannot create the missing prerequisite visit or authorize writing the library from that attempt. [Pass/focus][pass] [Attempt-to-deliberation dispatch][dispatch] [Mission preparation][mission-prepare] [Write guard][write-guard]

The practical requirements question is therefore: **Which local implementation does this declared dependency actually select under the admitted Cargo configuration?** Supply that observation before focusing work. More proposer/challenger rounds are not a substitute for supplying the scheduler's missing premise.

The existing portable-crucible proposal already calls for explicit dependency capabilities and unknowns. This report provides a concrete local-resolution witness and a bounded acceptance contract for that proposal, rather than another agent framework. [Portable crucible][portable]

## 2. Preserve the repairs already present

The earlier Cargo inheritance report has a current-main addendum: the original `workspace = true` omission is addressed in source. Current Rust discovery parses crate/workspace dependency records, resolves inherited local paths, and joins them to admitted crate nodes. Its ordinary inherited-edge fixture checks focused inclusion, dependency-first order and exclusion of an unused workspace declaration. Do not reopen that original defect. [Inheritance addendum][inheritance] [Current resolver][rust-scan] [Authored inheritance fixture][inheritance-test]

The new boundary is different:

- **Workspace inheritance** obtains a dependency declaration from another manifest table. The current code has that join.
- **Source patching** changes which package satisfies a declared source/version dependency. The current graph has no selected-package observation for that join.
- **Cargo default-member evidence** asks whether a command actually tested a member. This report establishes missing planning closure before any gate result exists. Restoring an edge does not settle command test coverage. [Default-member audit][default-member]
- **Root-crate diagnostic ownership** concerns assigning a diagnostic to a `.` crate. This witness uses two unambiguous nested crate directories. [Root ownership audit][root-owner]
- **Node namespace identity** concerns collisions among already discovered nodes. The proposed names here are distinct and do not collide with cross-cutting nodes. [Namespace audit][namespace]

The public audit inventory and neighboring recurse reports were screened. A bounded GitHub search for `patch.crates-io` under `Docs/audits` returned no matches; a broader Cargo/patch search returned fourteen files, whose patch-related passages were inspected. That search supports a scoped non-duplication assessment, not an assertion about every historical branch or discussion. The inheritance addendum already warns that the custom parser is not full Cargo resolver conformance. This report narrows that acknowledged limit to a selected-local-patch counterexample.

## 3. Exact acquisition boundary

### 3.1 Local crate membership survives

`DeriveWorkspaceDAG` canonicalizes the root, refreshes workspace membership, collects admitted sources and calls `scanRustCrates`. The collector respects hidden-directory and project-forbidden exclusions. Rust discovery reads the admitted manifests and adds a node for each parsed `[package]`. The two member crates in the witness are therefore both discoverable; the failure is not missing source membership. [Derivation][derive] [Collection][collect] [Rust nodes][rust-nodes]

### 3.2 The edge producer has only two local-target sources

For each crate dependency, `scanRustCrates` sets a target from `dep.path`, otherwise from `dep.workspace` through `resolveWorkspaceDepTarget`. If neither yields a target, it continues without an edge. Even when both local crates are already in the node map, their package names do not establish a relationship. [Edge loop][rust-edges]

That restraint is partly correct: a same-name local directory is not proof that Cargo uses it. The missing input is the resolver's selected package identity, not a fallback name guess.

The data types retain a dependency key, path, package name and inheritance flag. The parser recognizes package, workspace, workspace dependencies, ordinary/dev/build dependency tables, and target-specific dependency tables. `[patch.crates-io]` falls through as `secOther`; its entries are not retained. A consumer declaration such as `audit_local_z = "=0.1.0"` creates no direct path or inheritance flag. No subsequent stage resolves that declaration through the patch. [Dependency representation][representation] [Section handling][sections] [Attribute and entry handling][entries]

### 3.3 The missing relationship reaches the normal planner

`FilterDAG` recursively follows only `DependsOn`. `sweepGraphFactList` projects only those stored edges to `subsystem_depends`. The policy considers a node ready when none of its recorded prerequisites remains unvisited, then selects the lowest lexical rank among ready nodes. Structural validation can reject malformed graph identities or endpoints; it cannot discover a semantic relationship absent from its input. [Closure][closure] [Fact projection][projection] [Ordering][order] [Graph validation][graph-validation]

Both the plan path and live pass use the same derivation and focus closure. The CLI advertises focus with dependencies pulled in; the CLI and chat dispatch the shared cycle driver. This is not a cosmetic graph-rendering discrepancy. [Plan path][plan] [CLI entry][cli] [Chat entry][chat]

## 4. Conditional witness: same selected local package, different closure

Use an admitted Rust-only workspace with no ignored/forbidden member, no conflicting node identity, and no other edge between the two crates:

```toml
# Cargo.toml
[workspace]
members = ["crates/a", "crates/z"]
resolver = "2"

[patch.crates-io]
audit_local_z = { path = "crates/z" }
```

```toml
# crates/a/Cargo.toml
[package]
name = "audit_consumer_a"
version = "0.1.0"
edition = "2021"

[dependencies]
audit_local_z = "=0.1.0"
```

```toml
# crates/z/Cargo.toml
[package]
name = "audit_local_z"
version = "0.1.0"
edition = "2021"
```

Provide ordinary library targets: `crates/z/src/lib.rs` exports a function used by `crates/a/src/lib.rs`. The fixture qualification must establish that the pinned Cargo invocation and prepared lock/cache state actually select `crates/z` for a's dependency. Merely seeing the patch declaration is not qualification. No lockfile, registry access, Cargo execution or fixture was created in this review.

Cargo documents local path patches at the workspace root and their use for dependency development, including unpublished versions. Selection still depends on the dependency requirement and resolution state. These documented mechanisms support the conditional fixture; they do not replace the proposed execution qualification. [Cargo patch reference][cargo-patch]

Static trace once that selection premise holds:

1. Discovery creates `crates/a` and `crates/z` nodes.
2. The root patch is ignored by the parser. A's version-only dependency has neither a direct path nor inheritance, so the graph has no a-to-z edge.
3. Focusing on `crates/a` retains a and omits z. Cross-cutting nodes do not rescue this: they depend on the package nodes, rather than being prerequisites pulled into a's closure.
4. In a fresh full pass, both a and z are initially ready. Their lexical identities let a be selected first. Z is still available for a later full-pass visit.
5. For the control, replace a's declaration with `audit_local_z = { path = "../z", version = "=0.1.0" }` and remove the now-unneeded patch. Qualify that Cargo selects the same local package/version. The current producer then records a-to-z, focus includes z, and z must precede a. [Rust acquisition][rust-edges] [Closure][closure] [Cross-cutting edges][cross-cutting] [Policy][order]

The two inputs differ in how they designate the same effective local dependency. The proposed oracle compares qualified selected-package identity and expected local edges, not just whether two manifests contain the same package name.

### Downstream diagnostic consequence, kept separate

The built-in Rust build, test and lint gates are marked `ScopeAll` and invoked from the workspace root; their commands do not explicitly select every workspace member. The focus rewriting helper only rewrites Go argv. A Cargo invocation can therefore still report a failure at `crates/z/src/lib.rs`. During focused attribution, however, a finding outside the selected nodes that belongs to another node in the full graph is deliberately omitted as out of scope. With the missing edge, a genuine prerequisite diagnostic can take this path. [Detected Rust gates][rust-gates] [Gate scoping][gate-scope] [Finding filter][finding-filter]

This extra consequence requires an actually emitted, correctly parsed diagnostic and an applicable invocation. It is not needed to establish the planning gap. Cargo process failure, output and independent verification controls remain distinct from the filtered repair work list. Do not describe this as a false-green test or assume a mission necessarily accepts broken code.

The filter should remain for unrelated crates. Correct the prerequisite observation before filtering rather than indiscriminately broadening every focused sweep.

## 5. Proposed contract: selected, potential and unknown are different

Update the owning campaign capability/current-state documentation before implementing a change. The following is a proposed contract, not an existing API or a completed fix. [Architecture documentation guidance][architecture]

| Boundary | Required observation or decision | Must not imply |
|---|---|---|
| Workspace admission | Canonical package manifest and directory admitted by existing membership/project rules | Every resolved external package becomes writable |
| Resolution identity | Admitted manifest, lock/configuration identity, toolchain, feature/target domain and successful resolver observation | A patch declaration is necessarily selected |
| Selected local edge | Consumer package ID resolves to a package ID whose manifest belongs to an admitted local node | Matching package names alone establish identity |
| Potential dependency | Declared candidate relationship without selection evidence, labeled as potential | Exact effective prerequisite closure |
| Unavailable/incomplete | Resolution refused, failed, cancelled, truncated, stale or unsupported, with a reason | Empty complete graph or “no local dependency” |
| Focus/schedule | Policy derives permitted treatment from the graph's declared coverage and selected/potential relations | A model can widen an already admitted attempt |
| Reuse | Observation belongs to the same effective resolver inputs and graph generation | Same source tree alone proves same Cargo configuration |

The narrow invariant is: **if the admitted resolution selects a local patched package as a direct dependency of an admitted consumer, dependency-complete focus on that consumer includes the corresponding local prerequisite, subject to the existing SCC representation.** A patch that is not selected must not be presented as a selected edge. This is a planning guarantee, not a claim that every source-level, generated or cross-service dependency is modeled.

Choose explicitly whether the graph is configuration-resolved or a conservative prerequisite envelope. A union may be useful, but must retain the reason for extra edges and must not be mislabeled as the exact build graph. If multiple gate profiles matter, one narrow profile cannot silently stand for all of them. The first implementation can reject the stronger completeness claim for unsupported profiles while leaving disclosed limited work possible.

## 6. Bounded implementation direction

No implementation is included here.

1. **Obtain resolver output through the host.** Use `cargo metadata --format-version=1` under a recorded toolchain and the already admitted workspace and invocation context. Join resolved dependency package IDs to package manifest paths, then apply existing local membership. Retain renamed-dependency and edge-kind information where needed. Do not parse opaque source IDs into filesystem paths or infer a selected package by name.
2. **Do not mistake `--no-deps` for resolution.** Cargo's metadata contract leaves `resolve` null with that option; manifest dependency entries are not a substituted selected graph. A local declaration-only mode may help the existing inherited-path subset, but it cannot by itself certify this patch selection. The resolver domain also needs explicit feature/target meaning. [Cargo metadata contract][cargo-metadata]
3. **Keep acquisition within authority.** Metadata is a subprocess and may need dependency state. Use the authorized toolchain, bounded output, deadline and cancellation, with a defined lock-preserving/no-network mode where required. `--frozen` combines locked and offline behavior; an unavailable cache or incompatible lock must become unavailable evidence, not permission to fetch or rewrite it. [Cargo invocation restrictions][cargo-metadata]
4. **Publish one complete observation.** Accumulate and validate the package-ID/manifest joins before publishing the graph generation. Preserve error and coverage state. Do not publish a prefix as complete if decoding, path admission or resource bounds fail.
5. **Integrate before focus.** Extend the existing graph observation/fact path rather than introduce a parallel scheduler. Preserve Mangle readiness, lexical tie-breaking, SCC collapse and structural checks. The same semantics must feed planning and execution.
6. **Invalidate on effective-input changes.** Bind the observation to relevant manifest, lock and configuration/toolchain inputs. Keep per-pass refresh. A changed selection should produce a new graph generation; it must not mutate an executing mission's scope or reinterpret a saved visit without an explicit transition.
7. **Expose a bounded requirements receipt.** Let the proposer/interrogator see whether the current work has selected-local prerequisite coverage, a conservative envelope or an unknown. Give omitted details a genuinely accessible reference. This is evidence for critique, not new permission, a new success oracle or an invitation to run Cargo from a read-only role.

An incremental alternative is to recognize unsupported patch/configuration forms and report graph coverage as incomplete. That is a truthful capability improvement even before resolution is available. It does not recover the missing work. A direct patch-table parser plus name matching is not an equivalent full repair: it can invent edges for inactive, incompatible or ignored overrides.

Do not impose a Cargo metadata subprocess on Go/Python/JS-only workspaces. Retain existing working direct/inherited Rust cases, and publish the exact supported Cargo surface rather than promising universal manifest semantics.

## 7. Costs and failure modes

There are no measured speed, token, success-rate or cost improvements. The proposal adds costs before it can save model work:

- **Discovery:** toolchain startup, lock/cache inspection, resolution, stdout/stderr handling, parsing and canonicalization. Record cold and warm behavior separately. Metadata dependency state can be much larger than the local workspace.
- **Host computation:** retaining the admitted graph needs work proportional to relevant package/edge records; traversing and decoding the complete producer response can cost more. Streaming or early filtering must not erase unresolved relationships while claiming complete coverage.
- **Storage and invalidation:** receipts, profile identity, source digests and retained diagnostic detail consume space and I/O. Deduplicate by validated observation identity and define retention/expired states rather than copying every full metadata document into every deliberation frame.
- **Additional useful work:** restoring z to focus adds its visits, gates and possibly proposer/challenger/executor calls. A correct repair can cost more while fulfilling the requested closure. “Fewer calls” alone is therefore the wrong endpoint.
- **Conservative overhead:** potential-edge unions can schedule unused crates, enlarge SCCs and reduce parallelism. Exact-profile resolution can instead omit another profile's obligations if used too broadly.
- **Failure containment:** blocked offline resolution, absent toolchain, malformed metadata, unknown schema semantics, path escape, inaccessible manifests, source changes during acquisition, cancellation and unsupported configuration all need explicit dispositions. Retrying with weaker restrictions is not evidence repair.
- **Maintenance:** Cargo schema/version and configuration evolution, cross-platform path normalization, fixture qualification and migration of saved graph references add ongoing work. Legacy journals must not be stamped with a newly inferred dependency profile.

The full-cost ledger should include resolver time/bytes, host CPU and peak memory, artifact writes, every gate and model call including failed/retried calls, token categories and known/unknown spend, wall time, user interventions and independently accepted outcomes. Count failures and extra valid prerequisite visits, not only the final successful design.

## 8. Evaluation plan, entirely unrun

### A. Qualify dependency semantics independently

Prepare disposable public/synthetic fixtures with pinned Cargo/toolchain, manifests, lock/cache state and configuration. Establish the expected selected package using Cargo's resolved observation under that same profile. Keep the selection artifact separate from the recurse producer under evaluation. If the fixture does not select the local patch, classify it as a failed fixture precondition rather than a recurse counterexample.

| Fixture | Required distinction |
|---|---|
| Primary direct version dependency resolved to root local patch | A-to-z selected edge; focus includes z; z precedes a |
| Equivalent direct path dependency | Same selected local prerequisite and focused closure |
| Existing ordinary workspace-inherited path | Preserve source repair and exclude unused workspace declarations |
| Patch declared but not selected | No claim of selected local edge; disclose any intentionally conservative inclusion |
| Patch only in a member manifest | Respect the effective root-level source configuration; do not infer selection from textual presence |
| Renamed dependency, distinct package name | Join selected IDs/manifest paths, not dependency-key string equality |
| Two versions/sources of a package | Associate the actually selected local package, not every same-name node |
| Feature/target domains differ | Bind each result to its stated profile; no cross-profile completeness claim |
| Patch location outside admitted workspace or forbidden path | Preserve the external/excluded boundary; no new writable node |
| Missing cache, incompatible lock, unavailable toolchain or denied acquisition | Explicit incomplete/unavailable result; no silent complete empty graph |
| Truncated/malformed output or cancellation during decode | No partial graph published as complete |
| Config-only or lock-only selection change | Invalidate the prior observation even if source-file bodies are unchanged |
| No patch; unrelated ecosystem; ordinary direct Rust paths | Preserve existing semantics without unjustified discovery overhead |

These controls must distinguish false negatives from false positives. Always attaching every patch path to every member would pass the primary inclusion check while failing the contract.

### B. Exercise the actual work-selection route

After fixture qualification, exercise normal plan, CLI and chat entry paths with ordinary world-model/CodeDOM infrastructure enabled. Capture the acquired graph, projected dependency facts, policy-selected order, focused set and attempted nodes. An injected graph or a topological check over the producer's own graph cannot establish acquisition completeness.

Then introduce a controlled diagnostic in z and verify that it remains in the focused prerequisite work list while an unrelated admitted crate's diagnostic stays excluded. Check process failure independently from finding attribution. For mission mode, capture the attempt supplied to deliberation and its scope receipt; ensure only the correctly admitted node's paths become writable. No changed dependency observation should silently broaden an already-running attempt.

Existing tests were read, not run. They cover direct Rust paths and ordinary inherited closure, not this selected-patch witness. The inheritance report separately qualifies a parser-only renamed-inheritance fixture as not establishing Cargo-valid syntax; do not use that fixture as the resolver oracle here. [Direct-path test][direct-test] [Inheritance tests/addendum][inheritance-test] [Qualification][inheritance]

### C. Compare fairly before broader rollout

Compare (A) current acquisition, (B) incomplete-capability reporting, and (C) selected-resolution acquisition on the same independently qualified corpus. Hold model/provider, JIT corpus, task, permissions, gate profile and full budgets fixed wherever relevant. Include repositories with and without selected local patches, broken repositories, mixed-language workspaces and unavailable-resolution cases. Separate tuning cases from held-out cases and report disagreements and negative findings.

Primary host outcomes are selected-edge precision/recall within the declared domain, correct focused closure/order, truthful unknown handling, unnecessary inclusion and unchanged authority. End-to-end outcomes are independently supported prerequisite repairs and task completion at matched total cost, not the challenger's confidence or the number of graph edges. Report quality at equal budget and cost at equal accepted outcome. A cost-neutral or negative result is a valid reason not to enable full resolution by default.

## 9. Primary literature and limits of the analogy

Mokhov, Mitchell and Peyton Jones, *Build Systems à la Carte* (ICFP 2018), separates dependency scheduling from deciding whether work must be rebuilt. Its treatment of static/dynamic dependencies and correctness is useful here: a valid ordering of an acquired graph is different from acquiring the relationships the work actually needs. The transfer to recurse is this audit's engineering inference. Recurse is an improvement scheduler, not a build system proven correct by that paper. No performance result from the paper is claimed for CodeNERD. [Author-hosted paper][build-systems]

Cargo's own patch and metadata documentation supplies the concrete package-resolution contract. Together with the pinned CodeNERD producer and consumers, it supports the narrow conclusion: **qualified local patch selection must either enter prerequisite evidence or remain an explicit capability gap.** It does not justify stronger claims of universal dependency completeness, safe autonomous scope expansion or mission satisfaction.

## Pinned source references

[pin]: https://github.com/theRebelliousNerd/codenerd/commit/e3a4705258b80affec27bca47eddce54ba11ec07
[guidance]: https://github.com/theRebelliousNerd/codenerd/blob/e3a4705258b80affec27bca47eddce54ba11ec07/agents.md#L1-L65
[architecture]: https://github.com/theRebelliousNerd/codenerd/blob/e3a4705258b80affec27bca47eddce54ba11ec07/Docs/architecture/AGENTS.md
[mission-contract]: https://github.com/theRebelliousNerd/codenerd/blob/e3a4705258b80affec27bca47eddce54ba11ec07/Docs/architecture/campaign/MISSION-IMPROVEMENT.md#L98-L112
[portable]: https://github.com/theRebelliousNerd/codenerd/blob/e3a4705258b80affec27bca47eddce54ba11ec07/Docs/audits/portable-recurse-deliberation-crucible.md
[inheritance]: https://github.com/theRebelliousNerd/codenerd/blob/e3a4705258b80affec27bca47eddce54ba11ec07/Docs/audits/recurse-cargo-inherited-dependency-closure.md#L11-L19
[default-member]: https://github.com/theRebelliousNerd/codenerd/blob/e3a4705258b80affec27bca47eddce54ba11ec07/Docs/audits/recurse-cargo-default-member-evidence.md
[root-owner]: https://github.com/theRebelliousNerd/codenerd/blob/e3a4705258b80affec27bca47eddce54ba11ec07/Docs/audits/recurse-root-crate-diagnostic-ownership.md
[namespace]: https://github.com/theRebelliousNerd/codenerd/blob/e3a4705258b80affec27bca47eddce54ba11ec07/Docs/audits/recurse-node-namespace-identity.md
[derive]: https://github.com/theRebelliousNerd/codenerd/blob/e3a4705258b80affec27bca47eddce54ba11ec07/internal/campaign/recurse_workspace.go#L64-L100
[collect]: https://github.com/theRebelliousNerd/codenerd/blob/e3a4705258b80affec27bca47eddce54ba11ec07/internal/campaign/recurse_workspace.go#L264-L316
[representation]: https://github.com/theRebelliousNerd/codenerd/blob/e3a4705258b80affec27bca47eddce54ba11ec07/internal/campaign/recurse_workspace.go#L611-L625
[rust-nodes]: https://github.com/theRebelliousNerd/codenerd/blob/e3a4705258b80affec27bca47eddce54ba11ec07/internal/campaign/recurse_workspace.go#L627-L647
[rust-scan]: https://github.com/theRebelliousNerd/codenerd/blob/e3a4705258b80affec27bca47eddce54ba11ec07/internal/campaign/recurse_workspace.go#L627-L733
[rust-edges]: https://github.com/theRebelliousNerd/codenerd/blob/e3a4705258b80affec27bca47eddce54ba11ec07/internal/campaign/recurse_workspace.go#L675-L704
[parser]: https://github.com/theRebelliousNerd/codenerd/blob/e3a4705258b80affec27bca47eddce54ba11ec07/internal/campaign/recurse_workspace.go#L735-L904
[sections]: https://github.com/theRebelliousNerd/codenerd/blob/e3a4705258b80affec27bca47eddce54ba11ec07/internal/campaign/recurse_workspace.go#L789-L837
[entries]: https://github.com/theRebelliousNerd/codenerd/blob/e3a4705258b80affec27bca47eddce54ba11ec07/internal/campaign/recurse_workspace.go#L768-L787
[closure]: https://github.com/theRebelliousNerd/codenerd/blob/e3a4705258b80affec27bca47eddce54ba11ec07/internal/campaign/recurse_dag.go#L36-L76
[projection]: https://github.com/theRebelliousNerd/codenerd/blob/e3a4705258b80affec27bca47eddce54ba11ec07/internal/campaign/recurse_plan.go#L109-L133
[graph-validation]: https://github.com/theRebelliousNerd/codenerd/blob/e3a4705258b80affec27bca47eddce54ba11ec07/internal/campaign/recurse_plan.go#L84-L107
[order]: https://github.com/theRebelliousNerd/codenerd/blob/e3a4705258b80affec27bca47eddce54ba11ec07/internal/core/defaults/policy/recurse.mg#L420-L446
[cross-cutting]: https://github.com/theRebelliousNerd/codenerd/blob/e3a4705258b80affec27bca47eddce54ba11ec07/internal/campaign/recurse_workspace.go#L237-L257
[plan]: https://github.com/theRebelliousNerd/codenerd/blob/e3a4705258b80affec27bca47eddce54ba11ec07/internal/campaign/recurse_plan.go#L38-L66
[pass]: https://github.com/theRebelliousNerd/codenerd/blob/e3a4705258b80affec27bca47eddce54ba11ec07/internal/campaign/recurse_cycle.go#L515-L539
[cli]: https://github.com/theRebelliousNerd/codenerd/blob/e3a4705258b80affec27bca47eddce54ba11ec07/cmd/nerd/cmd_campaign_recurse.go#L80-L89
[chat]: https://github.com/theRebelliousNerd/codenerd/blob/e3a4705258b80affec27bca47eddce54ba11ec07/cmd/nerd/chat/campaign_recurse.go#L210-L244
[dispatch]: https://github.com/theRebelliousNerd/codenerd/blob/e3a4705258b80affec27bca47eddce54ba11ec07/internal/campaign/recurse_cycle.go#L1250-L1288
[mission-prepare]: https://github.com/theRebelliousNerd/codenerd/blob/e3a4705258b80affec27bca47eddce54ba11ec07/internal/campaign/recurse_mission.go#L174-L324
[write-guard]: https://github.com/theRebelliousNerd/codenerd/blob/e3a4705258b80affec27bca47eddce54ba11ec07/internal/campaign/recurse_mission_scope.go#L13-L74
[rust-gates]: https://github.com/theRebelliousNerd/codenerd/blob/e3a4705258b80affec27bca47eddce54ba11ec07/internal/gates/detect.go#L146-L152
[gate-scope]: https://github.com/theRebelliousNerd/codenerd/blob/e3a4705258b80affec27bca47eddce54ba11ec07/internal/campaign/recurse_cycle.go#L666-L689
[finding-filter]: https://github.com/theRebelliousNerd/codenerd/blob/e3a4705258b80affec27bca47eddce54ba11ec07/internal/campaign/recurse_cycle.go#L780-L807
[direct-test]: https://github.com/theRebelliousNerd/codenerd/blob/e3a4705258b80affec27bca47eddce54ba11ec07/internal/campaign/recurse_workspace_test.go#L163-L184
[inheritance-test]: https://github.com/theRebelliousNerd/codenerd/blob/e3a4705258b80affec27bca47eddce54ba11ec07/internal/campaign/recurse_audits_test.go#L483-L571
[cargo-patch]: https://doc.rust-lang.org/cargo/reference/overriding-dependencies.html
[cargo-metadata]: https://doc.rust-lang.org/cargo/commands/cargo-metadata.html
[build-systems]: https://simon.peytonjones.org/assets/pdfs/build-systems-original.pdf
