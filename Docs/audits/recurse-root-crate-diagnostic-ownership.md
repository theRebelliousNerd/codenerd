---
doc-class: deep-dive
subsystem: campaign-recurse
implementation-status: proposed-refinement
last-verified: 2026-11-03
verified-against: 638b74499f17140a875e39ca06c49a1148408e38
verification-method: static-source-inspection
---

# Recurse loses the owner of located root-crate diagnostics

## Decision and evidence boundary

Give recurse a typed, workspace-bound relationship between a diagnostic's source file and the node that owns it. The Rust graph producer represents a root Cargo package as one node with `Paths=["."]`; its modules remain descendants of that crate rather than separate graph nodes. The workspace finding consumer treats the same `.` path as only a root-directory package. It accepts `main.go` or `.` for the root node, but rejects a located target such as `src/main.rs`. A standard root-crate compiler failure is therefore parsed successfully, retained as failure, and then reported as unattributed instead of entering the crate's repair candidates. [Crate producer][crates] [Component projection][components] [Attribution][attribute]

This is a portability and work-selection gap, not false-green verification, an authorization bypass, or a claim that all Rust work is unreachable. Preserve the current refusal to invent an owner for genuinely unknown locations. The missing behavior is positive attribution of an admitted, known source file within a supported root crate.

This review is pinned to public main `638b74499f17140a875e39ca06c49a1148408e38`, rechecked unchanged at approximately 03:42 UTC on 2026-11-03. Only public repository source, guidance, neighboring audits and official Rust documentation were read. No repository code, test, build, gate, application, model, or behavioral fixture was run. The witness below is a conditional composition of inspected production functions, not a reproduced incident. No repository files were changed.

## Production trace

### The graph correctly admits a root Cargo package

`DeriveWorkspaceDAG` canonicalizes the workspace, refreshes membership, collects source/manifest paths, invokes the language scanners, and appends the closing stages. The collector recognizes `Cargo.toml`; `scanRustCrates` adds a node for each readable manifest containing `[package]`, using the manifest's workspace-relative directory and language `rust`. For a package manifest at the root, that directory is `.`. Component projection copies the member directory into `SubsystemNode.Paths`, giving the singleton `ID="."`, `Paths=["."]`, `Languages=["rust"]`. The graph does not make a separate node from `src/main.rs` or its module directory. [Derivation][derive] [Manifest collection][collect] [Crate producer][crates] [Component projection][components]

That representation is deliberate: the graph describes one node per crate, and the metrics consumer already distinguishes Rust by recursively counting the node's descendants. The required repair is not to rename the crate `src`, manufacture a module graph, or require the owner to move a conventional project layout. [Graph contract][graph-contract] [Recursive Rust metrics][metrics]

### The detected checks really run and the parser really locates the error

For root `Cargo.toml`, detection supplies `rust:build` (`cargo build`), `rust:test` (`cargo test`) and `rust:clippy` (`cargo clippy -- -D warnings`), all workspace-scoped. This audit assumes Cargo and the needed components are available and that no project command overrides these gates. [Detector][detect]

At the first pass baseline, `measureWorkspace` runs workspace-scoped gates and stores their results. `gates.Run` sets `Result.Node` only for a node-scoped gate, so a normal detected Cargo run leaves it empty. The Rust parser recognizes an `error[...]` line followed by its `--> source:line:column` location and constructs a finding whose target is that source path. The repository already has a parser fixture for `src/main.rs`; no invented parser format is required for the conditional witness. [Baseline][baseline] [Run contract][run] [Rust parsing][parse] [Existing parser fixture][parse-test]

The official Rust Book shows this same relative location form for an ordinary root package. It is primary evidence for the compiler's text format, not evidence that this audit ran Cargo or reproduced the CodeNERD route. [Rust compiler diagnostic example](https://doc.rust-lang.org/book/ch03-01-variables-and-mutability.html)

### The available location is lost at the node join

`measure` sends workspace findings to `attributeNode`. That function removes any test suffix, rejects explicit unattributed targets, and looks for the longest non-root path prefix. For a root path `.` it records the node ID but skips the prefix comparison. It returns that root node only when the target is exactly `.` or when the target contains no slash. Consequently `src/main.rs` matches no non-root node, is not `.`, and contains `/`: it becomes `RecurseUnattributedNodeID`. [Measurement join][measure] [Attribution][attribute]

The lookup's root restriction is understandable in its existing Go-oriented examples. A string such as an unresolved import path must not be guessed onto a root package, and a cross-cutting stage is not an ownership fallback. The defect is that a verified source-file path and an unresolved textual subject share the same untyped path treatment, while root crate coverage differs from root Go-package coverage.

`open` forwards a workspace finding only when `f.Node == node.ID`. Its current independent-gate forwarding repair does not rescue a finding that never received the physical node's ID. The policy adapter asserts `recurse_finding` only from that node's `open` slice, and the kernel chooses only from those asserted findings. All physical and closing stages can be visited while this compiler failure remains outside repair selection. [Candidate forwarding][open] [Policy inputs][policy-input] [Visit and selection][visit]

The CLI and chat both call the same `RunRecurseCycles` driver with normal campaign executors, so this is a shared production boundary rather than an unused helper. Their preceding setup and admission can still block execution independently. [CLI entry][cli] [Chat entry][chat]

## Primary conditional witness

Use a clean, admitted Git workspace containing a conventional root Cargo package with readable tracked `Cargo.toml` and `src/main.rs`. The manifest has `[package]`; the project has no Go, Python or JS source that would introduce a separate `src` node, no nested crate, and no relevant ignore or forbidden-path exclusion. Cargo is available. There is one ordinary compiler error in the root package's `src/main.rs`, and captured output is small and intact.

The existing parser fixture provides the observation shape: an E0425 diagnostic followed by a location at `src/main.rs:3:13`. This is a hypothetical observation input supported by the source fixture, not claimed execution output. Given that completed failed Cargo result:

1. Discovery produces the root Rust node `.` and the ordinary closing stages.
2. The detected workspace Cargo gate produces a failed result with empty `Result.Node`.
3. `Findings` successfully yields target `src/main.rs`.
4. `attributeNode` returns `unattributed` even though this file belongs to the discovered root crate.
5. `open` for `.` contains no copy of that finding; neither do the closing stages.
6. That failure is absent from `recurse_finding` for every visited node, so no repair attempt can be selected **for it** through the finding loop.

Other Cargo commands can report the same compilation error. This does not fix the join: each workspace-scoped result is independently attributed using the same source target. For a witness where each command reports the same root-crate source location, the result is multiple retained unattributed gate findings rather than a new eligible repair. A separate correctly attributed error could drive some other attempt, but is excluded from this narrow fixture and would not establish that this target was deliberately selected.

The global state retains the failure. `openCount` counts retained finding IDs, and `reportUnattributed` preserves their gate, target and message in logs and the result summary. The audit therefore must not say the failure is erased, the pass is green, or the user is told all checks passed. A node visit can nevertheless log zero open findings while the workspace log reports the located compiler failure as unattributed. [Retained counts and candidates][open] [Reporting][report]

Every node can also receive a metric-based improvement attempt. Such an attempt might incidentally change the defect or encounter it through inner campaign verification. The finding here is narrower: this specific measured error is not the selected repair target. The already documented workspace-test scheduling issue can separately prevent ordinary Rust improvements from satisfying their tested obligation; it is not needed to derive this attribution failure. [Visit/improvement sequencing][visit] [Earlier workspace-test audit][workspace-test]

## Adjacent Rust test limitation, not required for the witness

The same stock workspace `cargo test` route also leaves a named libtest failure without a crate owner: `Findings` combines `nodeOr(Result.Node)` with the test name, and an empty node becomes `<unattributed>`. An existing test expressly expects that conservative result. It should not be “fixed” by assigning an arbitrary root or wiring owner. A future adapter needs an admitted package/target-to-invocation relationship before a location-free test name becomes a node repair target. [Test projection][parse] [Parser fixture][parse-test]

This is a distinct observation from the primary located-file witness. The latter already has a useful path and fails the containment join; the former needs additional positive execution provenance. Cargo documents that tests are compiled into target-specific executables, and package selection can cover multiple members, so a test's module name alone is not a universal crate identity. [Cargo test command](https://doc.rust-lang.org/cargo/commands/cargo-test.html)

Keep this distinction in implementation and evaluation. Teaching the root-file join about admitted crate descendants does not by itself solve location-free libtest attribution.

## Nonduplication and existing controls

The bounded screen covered the current audit inventory and repository searches for Cargo findings, Rust attribution, unattributed recurse findings, `root crate`, and `attributeNode`. It found no exact root-crate descendant-attribution report. This is not a claim of exhaustive novelty across private discussion or every historical branch.

- **Recurse workspace test witness:** concerns an improvement admitted without scheduling the fresh test result required by the ratchet. Here a failed build check was already executed and parsed; the defect is which work item receives its located diagnostic. [Earlier audit][workspace-test]
- **Recurse gate subsumption:** concerns a correctly attributed workspace finding excluded because a node gate shared kind/language. Current `open` preserves independent gates. This witness fails before that repaired filter, at source-to-node attribution. [Earlier audit][subsumption]
- **Recurse finding census completeness:** concerns a bounded finding sample being treated as a complete census. This witness has one small diagnostic and needs neither truncation nor a cap. [Earlier audit][census]
- **Recurse node namespace identity:** concerns valid source IDs colliding with synthetic stages or delimiter encodings. The root crate and stages here have distinct IDs and a valid graph. [Earlier audit][namespace]
- **Recurse Go test dependency closure:** concerns missing test-import graph edges and work ordering. No edge or prerequisite is needed to exhibit the root crate's ownership gap. [Earlier audit][go-closure]
- **Portable recurse deliberation/crucible:** already calls for capability manifests and honest evidence limits. This report supplies a specific producer/consumer mismatch to include in that capability contract. [Portable proposal][portable]

Current attribution controls correctly keep unknown subjects away from wiring and unrelated source nodes. Existing attribution fixtures test that behavior and real Go package mapping. Preserve them. The inspected Rust graph fixture uses a virtual root workspace with nested crates, while the parser fixture stops at extracting `src/main.rs`; neither fixture composes that target with a non-virtual root crate's graph. No test result is claimed. [Attribution controls][attribute-tests] [Rust graph fixture][graph-test] [Parser fixture][parse-test]

## Proposed contract

Keep Go as observer and adapter, with Mangle owning eligibility and work selection. Specify a positive, typed attribution relation rather than broadening fallback guesses:

1. Preserve diagnostic subject kind, producer gate/invocation, location base, and normalized source-file identity separately from free-form package or test names.
2. Bind canonical source paths to the observed workspace membership and revision. Outside-workspace paths, ambiguous locations, missing provenance and unresolved imports remain unattributed.
3. Represent node ownership extent explicitly. A Go package directory and a Cargo crate with descendant modules cannot be assumed to have identical root semantics. Preserve more-specific nested member ownership and mixed-language boundaries.
4. For an admitted root-crate source file, publish its positive owner relationship so the ordinary candidate path can make the existing failed check a repair target. Keep gate argv, environment, pass semantics and snapshot identity attached through acceptance and ratchet.
5. For location-free tests, require package/target/invocation provenance. Do not infer ownership merely from there being one currently visible node or from a test-name suffix.

Cargo metadata exposes package IDs, manifest paths, target kinds and target source paths. It is a possible source of authoritative adapter data, not proof that metadata alone resolves every module or runtime test name. Adding it would require a bounded, configuration-aware collection and cache-validity design; a broad dependency resolver or new scheduler is unnecessary for the narrow positive-file witness. [Cargo metadata](https://doc.rust-lang.org/cargo/commands/cargo-metadata.html)

The minimal success condition is that a conventional root Cargo crate's admitted descendant diagnostic becomes that crate's repair candidate, while unknown subjects still do not. Spec and current-state documents should distinguish this acceptance condition from broader Rust test, coverage, feature, workspace-selection or behavioral support. Do not narrow the portability requirement merely to excuse the missing join.

## Unrun evaluation and costs

Evaluate through actual discovery, gate detection, measurement, finding extraction, attribution, policy candidate selection, attempt construction and ordinary acceptance. A direct `attributeNode` unit test alone cannot establish production reachability.

Proposed cases, all unrun:

- Conventional root crate, compiler error in `src/main.rs`, `src/lib.rs` and a nested module: correct root owner and target repair candidate.
- Root crate plus member crates: root files and member files remain distinct; the most specific admitted package relationship wins.
- Virtual workspace with nested crates: preserve current graph behavior and avoid inventing a virtual root package.
- Mixed-language repository with a root Rust crate and other source nodes: no cross-language owner theft.
- Root Go package and unresolved external import paths: preserve current conservative behavior.
- Absolute workspace paths, workspace-relative paths, platform separators, files outside the workspace, ignored files and unknown output: normalize only with sufficient provenance and retain unknowns visibly.
- A location-free libtest failure: remain unattributed until an admitted target/invocation supplies its owner; repeated test names across targets must not transfer evidence.
- Missing Cargo, canceled command, malformed manifest, missing source and graph collision: preserve independent unavailable/error/refusal semantics.
- Normal CLI and chat routes, plus mission mode: same attribution and policy behavior; no widening of writes or oracle protection.
- A successful repair and a nonrepair: the former reaches a fresh compatible witness; the latter remains failed, unverified or unresolved according to its actual evidence.

Measure attribution precision/recall over known workspace source files, repairs selected for observed failures, incorrect owner assignments, conservative unknowns, subprocess count, source/membership reads, serialized evidence bytes, time and model/tool expenditure. Reusing observed membership can make the narrow join inexpensive, but authoritative package metadata and invalidation may add work. No reliability, speed, token saving or live qualification is established by this static review.

## Pinned references

[derive]: https://github.com/theRebelliousNerd/codenerd/blob/638b74499f17140a875e39ca06c49a1148408e38/internal/campaign/recurse_workspace.go#L64-L100
[graph-contract]: https://github.com/theRebelliousNerd/codenerd/blob/638b74499f17140a875e39ca06c49a1148408e38/internal/campaign/recurse_workspace.go#L25-L47
[collect]: https://github.com/theRebelliousNerd/codenerd/blob/638b74499f17140a875e39ca06c49a1148408e38/internal/campaign/recurse_workspace.go#L269-L316
[crates]: https://github.com/theRebelliousNerd/codenerd/blob/638b74499f17140a875e39ca06c49a1148408e38/internal/campaign/recurse_workspace.go#L611-L644
[components]: https://github.com/theRebelliousNerd/codenerd/blob/638b74499f17140a875e39ca06c49a1148408e38/internal/campaign/recurse_workspace.go#L199-L234
[metrics]: https://github.com/theRebelliousNerd/codenerd/blob/638b74499f17140a875e39ca06c49a1148408e38/internal/campaign/recurse_cycle.go#L1456-L1475
[detect]: https://github.com/theRebelliousNerd/codenerd/blob/638b74499f17140a875e39ca06c49a1148408e38/internal/gates/detect.go#L146-L152
[baseline]: https://github.com/theRebelliousNerd/codenerd/blob/638b74499f17140a875e39ca06c49a1148408e38/internal/campaign/recurse_cycle.go#L654-L705
[run]: https://github.com/theRebelliousNerd/codenerd/blob/638b74499f17140a875e39ca06c49a1148408e38/internal/gates/run.go#L41-L80
[parse]: https://github.com/theRebelliousNerd/codenerd/blob/638b74499f17140a875e39ca06c49a1148408e38/internal/gates/findings.go#L89-L173
[measure]: https://github.com/theRebelliousNerd/codenerd/blob/638b74499f17140a875e39ca06c49a1148408e38/internal/campaign/recurse_cycle.go#L713-L755
[attribute]: https://github.com/theRebelliousNerd/codenerd/blob/638b74499f17140a875e39ca06c49a1148408e38/internal/campaign/recurse_cycle.go#L789-L841
[open]: https://github.com/theRebelliousNerd/codenerd/blob/638b74499f17140a875e39ca06c49a1148408e38/internal/campaign/recurse_cycle.go#L889-L938
[policy-input]: https://github.com/theRebelliousNerd/codenerd/blob/638b74499f17140a875e39ca06c49a1148408e38/internal/campaign/recurse_policy.go#L64-L100
[visit]: https://github.com/theRebelliousNerd/codenerd/blob/638b74499f17140a875e39ca06c49a1148408e38/internal/campaign/recurse_cycle.go#L958-L1012
[report]: https://github.com/theRebelliousNerd/codenerd/blob/638b74499f17140a875e39ca06c49a1148408e38/internal/campaign/recurse_cycle.go#L844-L887
[cli]: https://github.com/theRebelliousNerd/codenerd/blob/638b74499f17140a875e39ca06c49a1148408e38/cmd/nerd/cmd_campaign_recurse.go#L225-L263
[chat]: https://github.com/theRebelliousNerd/codenerd/blob/638b74499f17140a875e39ca06c49a1148408e38/cmd/nerd/chat/campaign_recurse.go#L218-L229
[parse-test]: https://github.com/theRebelliousNerd/codenerd/blob/638b74499f17140a875e39ca06c49a1148408e38/internal/gates/findings_test.go#L60-L71
[attribute-tests]: https://github.com/theRebelliousNerd/codenerd/blob/638b74499f17140a875e39ca06c49a1148408e38/internal/campaign/recurse_attribute_test.go#L30-L162
[graph-test]: https://github.com/theRebelliousNerd/codenerd/blob/638b74499f17140a875e39ca06c49a1148408e38/internal/campaign/recurse_workspace_test.go#L163-L183
[workspace-test]: https://github.com/theRebelliousNerd/codenerd/blob/638b74499f17140a875e39ca06c49a1148408e38/Docs/audits/recurse-workspace-test-witness.md
[subsumption]: https://github.com/theRebelliousNerd/codenerd/blob/638b74499f17140a875e39ca06c49a1148408e38/Docs/audits/recurse-gate-subsumption.md
[census]: https://github.com/theRebelliousNerd/codenerd/blob/638b74499f17140a875e39ca06c49a1148408e38/Docs/audits/recurse-finding-census-completeness.md
[namespace]: https://github.com/theRebelliousNerd/codenerd/blob/638b74499f17140a875e39ca06c49a1148408e38/Docs/audits/recurse-node-namespace-identity.md
[go-closure]: https://github.com/theRebelliousNerd/codenerd/blob/638b74499f17140a875e39ca06c49a1148408e38/Docs/audits/recurse-go-test-dependency-closure.md
[portable]: https://github.com/theRebelliousNerd/codenerd/blob/638b74499f17140a875e39ca06c49a1148408e38/Docs/audits/portable-recurse-deliberation-crucible.md
