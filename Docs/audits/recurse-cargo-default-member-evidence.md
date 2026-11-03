# Cargo package selection and recurse test applicability

**Status: static source-derived audit and proposed contract.** Public main `519ac934b6780f67b7401fd6d4f424a1f23e2d6f`, inspected 2026-11-03. No implementation, repository write, build, test, application path, Cargo command, model call, inference or benchmark was run. The scenario below is a conditional source trace, not an executed reproduction or observed incident.

## Decision in brief

Bind recurse's mandatory test verdict to the package domain of the improvement it is judging. The current host admits a Rust node's improvement when any runnable workspace-scoped Rust test exists. After the edit it runs that test and can set `Tested=true` from a completed verdict. Detected `rust:test`, however, is plain `cargo test` at the workspace root. Cargo may select only the workspace's default members, while recurse's graph includes another admitted crate. The fresh test can therefore certify the cycle without exercising the edited crate. [Admission][admission] [After-check selection][after] [Tested projection][tested] [Detection][detect]

The narrow witness needs no stale cache, changed manifest, test deletion, output truncation, unavailable tool or receipt reuse. In a virtual Cargo workspace with independent members a and b and `default-members = ["crates/a"]`, a source-only simplification of b can reduce its line metric while breaking an unchanged test of b. The detected commands still run for a; the outer ratchet can derive keep if all its other conditions succeed. This establishes a missing package-applicability check in the outer gate contract, not an inevitable end-to-end failure of the model, judge or campaign. [Package selection](https://doc.rust-lang.org/cargo/commands/cargo-test.html#package-selection) [Ratchet][policy]

Preserve fresh workspace-test execution and the tested guard. Neither going back to omitting those tests nor removing `default-members` from an owner's project is an acceptable repair. Host observations should identify which packages a check actually covers; Mangle should decide whether that evidence meets the selected node's obligation. If no authorized covering check exists, defer the improvement with an explicit reason before mutable work.

## Intended contract and current status

Root guidance makes Mangle the executive, requires typed host execution and revision-aware evidence, and calls for verification through production paths. Campaign guidance says model prose is not test evidence. The architecture contract separates intended guarantees from what current wiring establishes. Those requirements apply to the relevance of a real observation, not only to whether a subprocess ran. [Root][guidance] [Campaign][campaignGuidance] [Architecture][architecture]

The existing `ScopeAll` field describes how often a gate runs: once for the workspace rather than once per node. It is not a collected set of Cargo package IDs. `Language="rust"` is likewise a toolchain label, not evidence that every Rust crate is selected. The gap arises when those coarse attributes are sufficient for the stronger claim that a test can verify a particular node. [Gate types][scope] [Language matching][applicable] [Admission][admission]

A relevant earlier repair is present in this main. The original workspace-test audit showed ordinary improvements could never get their mandatory test fact when all available tests were workspace-scoped. Its current-main addendum records that correction and explicitly leaves language-matching commands' actual node coverage unestablished. Current `improve` checks for such a gate, and `run` selects it for the candidate when there is no own-node Test. Its authored regression uses a Go project declaring `commands.test: go test ./...` and expects a simplification to be retained. That fixes fresh execution reachability; it does not establish the package domain of plain Cargo defaults. Do not repeat the old missing-producer finding as if it still described current source. [Earlier audit][oldWorkspace] [Current admission][admission] [Current scheduling][after] [Authored regression][workspaceTest]

## Production trace

### 1. The sweep can name a crate the default command will not select

`DeriveWorkspaceDAG` canonicalizes the root, refreshes shared membership, collects admitted source paths and Cargo manifests, then calls `scanRustCrates`. The Rust producer creates a node for every collected manifest with a package section. Its node membership is not narrowed to Cargo default members. A valid member b is therefore a Rust node even when only a is the root command's default selection. The witness uses ordinary admitted tracked members and does not rely on ignored directories, an invalid manifest or a hidden harness directory. [Derivation][graph] [Collection][collect] [Crate nodes][crates]

Focusing on b does not align Cargo's selection with b. The graph filter selects the requested node and its dependencies, but `scopeGateToNodes` rewrites workspace commands only when `Language=="go"`. Rust argv are returned unchanged. Full sweeps also retain those argv. [Focused ordering][focus] [Gate scoping][gateScope]

### 2. Detected Cargo commands use the root's default package selection

Root `Cargo.toml` produces `cargo build`, `cargo test` and `cargo clippy -- -D warnings`, all with `ScopeAll` and Rust language. The test argv has neither package selectors nor `--workspace`. `gates.Command` copies workspace argv unchanged; `WorkspaceCommand` passes non-Go arguments through and sets the process directory to the selected workspace root. There is no downstream Rust package expansion in this path. [Detector][detect] [Invocation][run] [Command construction][command]

Cargo documents that package selection without explicit selectors follows the selected manifest, which here is resolved from the root directory. A workspace root uses `workspace.default-members`. If that field is omitted, a virtual workspace selects all members, whereas a non-virtual workspace selects the root crate. `--workspace` explicitly selects all members; `--package` selects specified packages. Build has the same documented default-package distinction. Clippy documents support for the usual workspace options and package selection, including workspace path dependencies; the witness makes a and b independent to avoid that dependency route. [Cargo test package selection](https://doc.rust-lang.org/cargo/commands/cargo-test.html#package-selection) [Cargo build package selection](https://doc.rust-lang.org/cargo/commands/cargo-build.html#package-selection) [Clippy workspace behavior](https://doc.rust-lang.org/clippy/usage.html#workspaces)

The explicit-default-members witness is sufficient and easiest to isolate. A non-virtual root package with an independent member and no explicit default-members is another applicable configuration. It is not correct to claim that every virtual workspace is affected: omitting default-members there normally makes the root test select all members.

### 3. The improvement admission joins language, not selected package

Detected Rust tests are workspace-scoped, so `nodeGates` does not put them in the node's own-gate set. `hasApplicableWorkspaceTest` admits the available `rust:test` using scope, kind and `AppliesTo(node.Languages)`. `AppliesTo` compares language labels; it does not inspect Cargo membership or selected package identity. Thus b can have no own Test, have no test selected by the root command, and still pass this admission. [Own gates][nodeGates] [Admission helper][admission] [Applicability][applicable]

The ordinary angle is selected from measurable metrics. Rust nodes have recursive source-line measurement; the simplify angle asks that lines go down. An unchanged count of test functions is not evidence that those functions executed. [Metric production][metrics] [Metric reader][metricReader] [Angle and guards][metricPolicy]

### 4. Fresh execution supplies a verdict for the wrong node obligation

After a changed ordinary improvement with no own Test, `needsWorkspaceTest` is true. The workspace loop selects test gates matching the node language and measures them. `measure` runs the workspace gate with an empty node ID, through the same root command path. Nothing in this producer adds b to Cargo's selection. [After checks][after] [Measurement][measure]

During projection, any fresh after entry of kind Test that is not unverified and is not classified as no-tests sets the cycle Boolean `Tested`. The Boolean is then projected as `recurse_ratchet_tested(Cycle)`; it carries no required package domain or relation to the edited node. The witness gives a at least one actual passing test, so it does not depend on a no-tests result being misclassified. [Projection][tested] [Fact bridge][facts]

The current policy keeps a changed improvement when its metric moves, the tested fact is present, no checked gate gets worse, no forbidden path exists, and no guarded metric regresses. Under the witness's stated conditions those inputs permit keep. The keep branch commits the changed paths, updates retained gate state and increments both Kept and Improved. The policy is not comparing Cargo package sets because no such input was supplied. [Keep policy][policy] [Metric policy][metricPolicy] [Adoption][adopt]

## Conditional witness

Use a clean, admitted Git workspace with this valid project arrangement:

- A virtual root Cargo manifest declares members `crates/a` and `crates/b`, with only `crates/a` in `default-members`.
- Both crates have valid manifests and source. They are independent: a does not depend on, import, invoke or otherwise exercise b. There are no build scripts that inspect b and no shared test fixture that indirectly tests it.
- a has an ordinary passing test. b has a separately stored existing test asserting that its public `answer()` returns 42.
- b initially implements `answer` with a multi-line function that binds 42 and returns it. A candidate changes only that production function to a shorter valid function returning 0.
- No test definition, manifest, lockfile, fixture, gate declaration, environment or selection configuration changes. The replacement reduces b's measured nonblank lines and leaves the static test count unchanged.
- Cargo and the necessary tools are available; the baseline and selected a checks pass. No owner declaration replaces the native gates, no additional applicable node check or broad check detects the b regression, and optional error-free measurement is not enabled.
- The ordinary simplify visit is reached under a sufficient finite pass budget, or its exact outer boundary is isolated by a future scripted fixture. The inner executor returns the candidate and valid owned-write receipts. All independent permission, scope, budget, campaign, snapshot, Git and persistence conditions needed for the keep branch are stipulated successful.

The explicit assumption about inner execution is important. The bad replacement violates the intended simplify behavior and should be rejected by a capable worker or reviewer. The audit does not claim a model would necessarily propose it, overlook it, finish its task or pass all independent gates. The witness asks what the outer deterministic ratchet does **if that erroneous in-scope candidate reaches it**, which is exactly the kind of mistake a mandatory verification boundary should be able to catch.

| Boundary | Source-derived consequence under these assumptions |
|---|---|
| Graph and focus | b is a Rust node; it can be visited or explicitly focused. |
| Native gate inventory | Root Cargo commands remain workspace-scoped with no explicit package selector. |
| Baseline | Default member a supplies the recorded passing build/test/lint results. |
| Improvement admission | A workspace Test with matching Rust language exists, so b is admitted. |
| Candidate metrics | b lines decrease; workspace test count stays K. |
| Fresh ordinary checks | Root commands run again for default selection a. b's existing test is not selected. |
| Test fact | a's completed fresh test result sets the cycle's Tested Boolean. |
| Policy and adoption | With other guards successful, simplify can be kept and counted as improved. |
| Independent oracle | A correctly selected b test would exercise the changed return value and fail. This was not run here. |

This is not a false claim about the exit code: the process really can pass for a. The unsupported step is using that result as the mandatory test evidence for b's change. At pass end, a changed kept count triggers another workspace measurement, but its unchanged Cargo argv still select a; repetition alone does not close the domain gap. [Pass-end condition][pass]

A weaker consequence needs no erroneous patch: baseline and visit logs can show no observed b finding even when b has a defect that only b's tests expose. Such absence is not proof that b is green. The main witness stays with the stronger, conditional outer-adoption path and does not claim measured damage or frequency.

## Inner controls and limits on the finding

Both CLI and chat use the shared recurse loop and construct `RecurseAttemptCampaign` for its executor. The task receives the node write set, angle and metrics. For an improvement with no check, the constructor declares no specific acceptance command; this does **not** remove the rest of the session, orchestrator, Guardian, write guard or completion machinery. [CLI][cli] [Chat][chat] [Attempt constructor][attempt]

The mandatory improvement prompt explicitly asks for behavior-preserving simplification and passing tests. Mission prompting additionally requires production-path exercise and a test of changed behavior. A model or judge can detect the bad candidate, run a package-specific test, repair it or refuse completion. These are useful controls, not proof that the outer host's generic Tested Boolean covers b. This review does not claim every such control was bypassed or that a production model path was executed. [Prompt instructions][prompt]

Mission preparation, protected verification inputs, owned-write checks, retained evidence and resource limits remain independent safeguards. The narrow witness uses ordinary recurse and a permitted production-source change; it does not rely on modifying an oracle or strategic requirement. Mission mode should be evaluated separately with its additional admissions, rather than assumed either vulnerable end to end or universally protected by deliberation. [Mission controls][mission]

Other important counterexamples:

- A virtual workspace with no default-members restriction normally selects all members, so this particular package omission is absent.
- Explicit project commands selecting all required packages, or a legitimate node-scoped command selecting b, can supply the missing test route. Do not overwrite such owner declarations.
- If a depends on b, Cargo may compile b as a dependency. That changes the witness: it can reveal compile errors, but compiling a dependency is not automatically running that dependency's own tests. The independent-crate witness avoids that ambiguity.
- A missing tool or cancelled command cannot supply a normal successful fresh test verdict. Existing unavailable/unverified handling remains necessary.
- A failing selected gate, forbidden write, declining test count or other independent veto can prevent keep. No universal false-keep claim is made.
- Equal argv, environment, root and source freshness do not solve this case. Before and after may identify precisely the same experiment, but that experiment never covers the selected b obligation.

## Nonduplicate review

This finding is a scoped consumer-applicability follow-on to the portable capability proposal and the workspace-test audit's current-main qualification, not a new discovery that language matching fails to establish node coverage. The incremental evidence is the concrete source-composed path from native Cargo default-member selection through the repaired workspace-test schedule into an unrelated node's Tested fact. [Portable proposal][portable] [Existing qualification][oldWorkspace]

The neighboring reports answer different questions:

- **Workspace test witness:** an eligible ordinary improvement previously had no fresh-test producer. Current main adds it; the addendum explicitly leaves actual node coverage open. This report supplies a Cargo-specific deterministic consequence and validation contract for that acknowledged boundary. [Prior audit][oldWorkspace]
- **Cargo inherited dependency closure:** members lost graph prerequisite edges. This witness uses independent members and a complete, correct graph. [Prior audit][inheritAudit]
- **Root-crate diagnostic ownership:** an observed diagnostic could not be attributed to its root crate. Here the omitted b test produces no diagnostic for attribution. [Prior audit][rootAudit]
- **Workspace test receipt invocation identity:** a receipt from a different directory discharged another invocation. Here every process runs freshly at the correct root, with no reused receipt. [Prior audit][receiptAudit]
- **Gate comparison domain admission / scoped overlay:** different before/after domains or incomplete retained evidence produced incorrect comparisons. Here before and after can have identical valid package selection a; the mismatch is between that domain and b's required observation. [Comparison][comparisonAudit] [Overlay][overlayAudit]
- **Gate language domain:** a collapsed multilingual component sent a toolchain gate to the wrong member paths. Here all nodes are Rust and no SCC is needed. [Prior audit][languageAudit]

The inspected Rust graph fixture covers direct path dependencies, and the newer Cargo inheritance tests cover virtual and non-virtual graphs and focus closure. Neither establishes a Cargo default-member test invocation for an improvement. The workspace-test regression is a Go declaration fixture. The pure policy test intentionally injects Tested and verifies its requirement, not the producer's semantic scope. These test definitions were read, not run. [Graph fixture][graphTest] [Inheritance fixture][inheritTest] [Workspace fixture][workspaceTest] [Policy guard][guardTest]

Bounded public-repository searches for `default-members`, `default members` and `cargo test --workspace` returned no exact report. `Cargo package` returned the root-crate diagnostic audit. Relevant recurse, gate-comparison, overlay and receipt reports were also inspected. This is a bounded overlap check, not an exhaustive claim about every branch, discussion or private history.

## Proposed repair and acceptance contract

Specify the distinction between scheduling scope and evidence applicability before changing code. Preserve the existing loop and separation of responsibilities:

1. **Host supplies package identity and capability facts.** Resolve the admitted node to its Cargo package/manifest identity. For each candidate check retain the selected manifest, effective package selectors/default members, relevant target/feature/profile configuration, working directory and completeness disposition. A command that cannot establish a supported selection relation remains unknown, not an all-package certificate.
2. **Policy owns the obligation.** Mangle derives whether the candidate has a feasible authorized test route whose selected package domain covers the required node/behavioral scope. Matching language is necessary for many native gates but not sufficient.
3. **Execute an adequate route or defer before spending.** For native Cargo checks, explicit package selection or a genuinely authorized full-workspace check are possible approaches. Retain the owner's declared command and intended restrictions; a default-member restriction can be deliberate. If broader execution is not authorized or affordable, report the missing domain and hold the candidate rather than silently widening commands.
4. **Keep real evidence separate from its admission.** Retain a's valid pass as evidence for a, while declining to let it discharge b's obligation. Freshness and invocation identity remain required. Do not set Tested from source counts, model prose, a baseline or a structurally related package.
5. **Preserve negative and unknown results.** An unavailable, interrupted, unsupported or incompletely resolved selector cannot become covered. A package-specific failing test remains a failure. Do not weaken existing regression, no-tests, ownership or permission controls.

Blindly appending `--workspace` everywhere is not the full contract. Cargo members may exceed the admitted sweep or include deliberately excluded work, expensive targets and effectful build/test code. Conversely, a package-only check may miss required dependent integration tests. The intended obligation, authorization and supported configuration must determine selection. Package coverage is a necessary boundary here, not a claim that running one package's tests proves all behavioral requirements.

## Unrun validation and full-cost comparison

First create a disposable local fixture with real Cargo manifests, valid independent crates and unchanged tests. Drive the shared loop with a scripted executor and real kernel, recording actual command identities and selected package/test receipts. The script must not inject Tested or replace the ratchet. Compare the current native behavior against a proposed package-domain admission route. Separately exercise normal CLI/chat and, when authorized, a model/judge path; a deterministic fixture is not production-path qualification.

Required cases include:

- Virtual root with explicit default-members a, focused b and full sweep.
- Same fixture without default-members: both members selected.
- Non-virtual root with independent b and implicit root-only defaults.
- Explicit owner command selecting b or all authorized members.
- Legitimate node-scoped b test; a test fails only after b's source-only change.
- Positive behavior-preserving b simplification, to ensure the repair still permits useful work.
- b compiled as a dependency but its own tests not selected, kept separate from the independent-crate fixture.
- Unknown package identity, unavailable tool, cancellation, unsupported selector or partial metadata.
- Unchanged gate configuration versus separately authorized changes to package selection.
- Mission mode, ownership/compensation and optional additional gates as separate control arms.

The required result is not merely an extra subprocess. The host must show that the candidate's required package evidence ran, or explicitly defer before mutation; a fresh a-only result must not discharge b. Preserve the prior workspace-test repair and ordinary Go behavior.

Report false keeps, unjustified reverts, explicit deferrals, selected-package/test coverage, completed versus unverified checks, worker/verification retries, model tokens and spend when used, subprocess count, build-cache effects, wall/CPU time, bytes read/retained, cleanup failures and owner intervention. Include failed and abandoned attempts. Broader selection can increase real compilation/test cost; an admitted package check can reduce unrelated work but may provide less integration assurance. No measured gain, latency or failure frequency is claimed.

Done means a retained improvement's mandatory test evidence has an inspectable relation to the package it changed, not just the same language and a fresh successful process somewhere in its workspace.

## Revalidation before freeze

Main advanced during inspection from `27dc83bd5f881123c1c9e87fdf57d9fd20d86278` to the pinned `519ac934b6780f67b7401fd6d4f424a1f23e2d6f`. The three-commit comparison was inspected. It adds the workspace-test and no-tests audit addenda, refactors a local index expression in the Cargo manifest parser, and changes other campaign/runtime paths. It does not change native Cargo gate selection, workspace command construction, improvement admission, fresh after-test scheduling or the Tested/ratchet join cited here. The new campaign test-witness cache is an inner `/test_run` path; the ordinary outer-gate witness still executes freshly and does not rely on cache reuse. All source links below are repinned; no application or test execution was performed. [Compared revisions](https://github.com/theRebelliousNerd/codenerd/compare/27dc83bd5f881123c1c9e87fdf57d9fd20d86278...519ac934b6780f67b7401fd6d4f424a1f23e2d6f)

## Pinned public sources

[guidance]: https://github.com/theRebelliousNerd/codenerd/blob/519ac934b6780f67b7401fd6d4f424a1f23e2d6f/agents.md#L3-L23
[campaignGuidance]: https://github.com/theRebelliousNerd/codenerd/blob/519ac934b6780f67b7401fd6d4f424a1f23e2d6f/internal/campaign/agents.md#L1-L8
[architecture]: https://github.com/theRebelliousNerd/codenerd/blob/519ac934b6780f67b7401fd6d4f424a1f23e2d6f/Docs/architecture/AGENTS.md#L50-L111
[scope]: https://github.com/theRebelliousNerd/codenerd/blob/519ac934b6780f67b7401fd6d4f424a1f23e2d6f/internal/gates/gates.go#L40-L86
[detect]: https://github.com/theRebelliousNerd/codenerd/blob/519ac934b6780f67b7401fd6d4f424a1f23e2d6f/internal/gates/detect.go#L123-L151
[declarations]: https://github.com/theRebelliousNerd/codenerd/blob/519ac934b6780f67b7401fd6d4f424a1f23e2d6f/internal/gates/detect.go#L19-L90
[graph]: https://github.com/theRebelliousNerd/codenerd/blob/519ac934b6780f67b7401fd6d4f424a1f23e2d6f/internal/campaign/recurse_workspace.go#L61-L106
[crates]: https://github.com/theRebelliousNerd/codenerd/blob/519ac934b6780f67b7401fd6d4f424a1f23e2d6f/internal/campaign/recurse_workspace.go#L627-L647
[collect]: https://github.com/theRebelliousNerd/codenerd/blob/519ac934b6780f67b7401fd6d4f424a1f23e2d6f/internal/campaign/recurse_workspace.go#L270-L316
[focus]: https://github.com/theRebelliousNerd/codenerd/blob/519ac934b6780f67b7401fd6d4f424a1f23e2d6f/internal/campaign/recurse_plan.go#L38-L68
[gateScope]: https://github.com/theRebelliousNerd/codenerd/blob/519ac934b6780f67b7401fd6d4f424a1f23e2d6f/internal/campaign/recurse_cycle.go#L637-L690
[measure]: https://github.com/theRebelliousNerd/codenerd/blob/519ac934b6780f67b7401fd6d4f424a1f23e2d6f/internal/campaign/recurse_cycle.go#L759-L782
[run]: https://github.com/theRebelliousNerd/codenerd/blob/519ac934b6780f67b7401fd6d4f424a1f23e2d6f/internal/gates/run.go#L48-L99
[command]: https://github.com/theRebelliousNerd/codenerd/blob/519ac934b6780f67b7401fd6d4f424a1f23e2d6f/internal/build/workspace_command.go#L30-L60
[applicable]: https://github.com/theRebelliousNerd/codenerd/blob/519ac934b6780f67b7401fd6d4f424a1f23e2d6f/internal/gates/gates.go#L115-L127
[nodeGates]: https://github.com/theRebelliousNerd/codenerd/blob/519ac934b6780f67b7401fd6d4f424a1f23e2d6f/internal/campaign/recurse_cycle.go#L993-L1005
[admission]: https://github.com/theRebelliousNerd/codenerd/blob/519ac934b6780f67b7401fd6d4f424a1f23e2d6f/internal/campaign/recurse_cycle.go#L1181-L1224
[after]: https://github.com/theRebelliousNerd/codenerd/blob/519ac934b6780f67b7401fd6d4f424a1f23e2d6f/internal/campaign/recurse_cycle.go#L1324-L1349
[tested]: https://github.com/theRebelliousNerd/codenerd/blob/519ac934b6780f67b7401fd6d4f424a1f23e2d6f/internal/campaign/recurse_cycle.go#L1415-L1443
[facts]: https://github.com/theRebelliousNerd/codenerd/blob/519ac934b6780f67b7401fd6d4f424a1f23e2d6f/internal/campaign/recurse_policy.go#L190-L225
[metrics]: https://github.com/theRebelliousNerd/codenerd/blob/519ac934b6780f67b7401fd6d4f424a1f23e2d6f/internal/campaign/recurse_cycle.go#L1575-L1604
[metricReader]: https://github.com/theRebelliousNerd/codenerd/blob/519ac934b6780f67b7401fd6d4f424a1f23e2d6f/internal/gates/metrics.go#L88-L118
[policy]: https://github.com/theRebelliousNerd/codenerd/blob/519ac934b6780f67b7401fd6d4f424a1f23e2d6f/internal/core/defaults/policy/recurse.mg#L260-L329
[metricPolicy]: https://github.com/theRebelliousNerd/codenerd/blob/519ac934b6780f67b7401fd6d4f424a1f23e2d6f/internal/core/defaults/policy/recurse.mg#L339-L398
[adopt]: https://github.com/theRebelliousNerd/codenerd/blob/519ac934b6780f67b7401fd6d4f424a1f23e2d6f/internal/campaign/recurse_cycle.go#L1481-L1507
[pass]: https://github.com/theRebelliousNerd/codenerd/blob/519ac934b6780f67b7401fd6d4f424a1f23e2d6f/internal/campaign/recurse_cycle.go#L549-L612
[cli]: https://github.com/theRebelliousNerd/codenerd/blob/519ac934b6780f67b7401fd6d4f424a1f23e2d6f/cmd/nerd/cmd_campaign_recurse.go#L225-L265
[chat]: https://github.com/theRebelliousNerd/codenerd/blob/519ac934b6780f67b7401fd6d4f424a1f23e2d6f/cmd/nerd/chat/campaign_recurse.go#L218-L267
[attempt]: https://github.com/theRebelliousNerd/codenerd/blob/519ac934b6780f67b7401fd6d4f424a1f23e2d6f/internal/campaign/recurse_attempt.go#L34-L139
[prompt]: https://github.com/theRebelliousNerd/codenerd/blob/519ac934b6780f67b7401fd6d4f424a1f23e2d6f/internal/prompt/atoms/campaign/recurse_attempt.yaml#L24-L58
[mission]: https://github.com/theRebelliousNerd/codenerd/blob/519ac934b6780f67b7401fd6d4f424a1f23e2d6f/Docs/architecture/campaign/MISSION-IMPROVEMENT.md#L184-L211
[guardTest]: https://github.com/theRebelliousNerd/codenerd/blob/519ac934b6780f67b7401fd6d4f424a1f23e2d6f/internal/campaign/recurse_policy_test.go#L316-L332
[workspaceTest]: https://github.com/theRebelliousNerd/codenerd/blob/519ac934b6780f67b7401fd6d4f424a1f23e2d6f/internal/campaign/recurse_audits_test.go#L362-L387
[graphTest]: https://github.com/theRebelliousNerd/codenerd/blob/519ac934b6780f67b7401fd6d4f424a1f23e2d6f/internal/campaign/recurse_workspace_test.go#L163-L185
[inheritTest]: https://github.com/theRebelliousNerd/codenerd/blob/519ac934b6780f67b7401fd6d4f424a1f23e2d6f/internal/campaign/recurse_audits_test.go#L483-L569
[oldWorkspace]: https://github.com/theRebelliousNerd/codenerd/blob/519ac934b6780f67b7401fd6d4f424a1f23e2d6f/Docs/audits/recurse-workspace-test-witness.md
[inheritAudit]: https://github.com/theRebelliousNerd/codenerd/blob/519ac934b6780f67b7401fd6d4f424a1f23e2d6f/Docs/audits/recurse-cargo-inherited-dependency-closure.md
[rootAudit]: https://github.com/theRebelliousNerd/codenerd/blob/519ac934b6780f67b7401fd6d4f424a1f23e2d6f/Docs/audits/recurse-root-crate-diagnostic-ownership.md
[receiptAudit]: https://github.com/theRebelliousNerd/codenerd/blob/519ac934b6780f67b7401fd6d4f424a1f23e2d6f/Docs/audits/workspace-test-receipt-invocation-identity.md
[comparisonAudit]: https://github.com/theRebelliousNerd/codenerd/blob/519ac934b6780f67b7401fd6d4f424a1f23e2d6f/Docs/audits/gate-comparison-domain-admission.md
[overlayAudit]: https://github.com/theRebelliousNerd/codenerd/blob/519ac934b6780f67b7401fd6d4f424a1f23e2d6f/Docs/audits/scoped-gate-overlay-evidence-conservation.md
[languageAudit]: https://github.com/theRebelliousNerd/codenerd/blob/519ac934b6780f67b7401fd6d4f424a1f23e2d6f/Docs/audits/recurse-gate-language-domain.md
[portable]: https://github.com/theRebelliousNerd/codenerd/blob/519ac934b6780f67b7401fd6d4f424a1f23e2d6f/Docs/audits/portable-recurse-deliberation-crucible.md#L28-L59

