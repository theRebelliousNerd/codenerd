---
doc-class: deep-dive
subsystem: gates-tools-go-build-coverage
implementation-status: proposed-refinement
last-verified: 2026-11-04
verified-against: 582cfc954afeeb3cf5e433a0db6fac77c82bdd2a
verification-method: static-source-inspection
---

# Discarded Go build output silently narrows compilation to executable roots

## Finding and decision

Two production build adapters protect the workspace from generated binaries by adding `-o <existing temporary directory>` to a Go build. That transformation changes more than the output destination. In the supported Go toolchain's directory-output branch, the build action includes only selected `main` packages and their dependencies. An independent selected library is not compiled. Both codeNERD adapters retry without the directory only when Go reports that there are no main packages. A mixed command-and-library module therefore falls between their two handled cases. [G1 adapter][g1-run] [Typed adapter][typed-run] [Go build action][go-action]

Consequently, a compile-breaking edit in an unimported library can leave **G1 at pass with zero findings** and make an admitted **`run_build` request for `./...` return exit zero**. The process exit and reported argv are real; the false assurance is that the successful transformed command discharged the requested compilation domain. This is not a missing-diagnostic parser problem: the relevant compiler action was not scheduled.

Preserve output hygiene, typed arguments, existing permissions and Mangle's verdict ownership. Specify that redirecting or discarding build artifacts must conserve the selected compilation domain, then use one qualified implementation contract across both adapters.

## Evidence boundary and architectural fit

Public main was read at **582cfc954afeeb3cf5e433a0db6fac77c82bdd2a**, committed November 4, 2026 at 08:20:53 UTC. All codeNERD links below use that pin. The module requires Go 1.27.3; that release tag resolved to **2aae1000ea1d4b28923b96db28acf4e3b99d6aff** in the official Go repository, and its build-driver source was inspected directly. This establishes the reference toolchain behavior, not the Go version installed on any particular user's computer. [Module][module] [Go release][go-release]

The review read root and public-repository guidance, campaign/tool instructions, campaign and build architecture, the implementation chain, authored tests, the audit inventory/queue and adjacent reports. Architecture prose was used as intent and navigation, not as proof of current code behavior. No repository program, compiler, test, gate, model inference, benchmark or failure injection ran. No implementation, repository file, credential or setting was changed. The fixture and outcomes are conditional source derivations, and all evaluations below remain unrun.

The repository's model/logic split assigns creative work to the model and executive decisions to Mangle. The host must therefore supply evidence for the compilation obligation it says was measured. An LLM guess that a library is unused, or a policy workaround that ignores this build failure, would not repair the producer. The current campaign design uses error-free snapshots as additional nonregression evidence; that design needs a faithful build observation rather than a second verdict engine in Go. [Root guidance][guidance] [Architecture standard][architecture] [Campaign contract][campaign]

## 1. The two production paths share the same loss

### G1: whole-module build measurement

`ErrorFreeMeasurers` creates G1 with binary `go`, arguments `build`, packages `./...`, the compile reader, and `buildOutput: true`. The measurement records the same requested package selectors. [G1 construction][g1-construction]

The build-output path creates an existing temporary directory, appends `-o` and that directory, then appends the original package selectors. It executes under `build.GoInvocation`, preserving the workspace build environment and configured Go flags. Its only output-related retry checks a failed invocation's stderr for `no main packages to build`; that case runs the plain original build. [G1 execution][g1-run] [Environment][invocation]

After a successful transformed invocation, `readCompile` finds no positioned compile error, and `commandGate.Measure` assigns `MeasurePass`. The outer measurer assigns a zero count. No incomplete-domain flag is emitted: `Unenumerated` concerns a nonzero exit that produced no findings, not a successful invocation that skipped requested library compilation. [Classification][g1-classify] [Count][count]

This measurement reaches users through `nerd gates measure`, which saves the snapshot and renders it. The CLI permits `--only build`, so the incorrect build claim is independently observable without requiring every other gate to miss the same defect. `nerd gates diff` loads the snapshots and asks the shared comparison policy. [CLI][cli]

### Typed `run_build`: the model's project build operation

`RunBuildTool` declares workspace-relative package arrays, defaulting to `./...`, and calls `executeTypedVerification(..., false)`. Shell registration includes that tool, and the registry's normal validation/allowlist/executive guard runs before tool execution. The witness assumes those ordinary prerequisites succeed; it requires no permission bypass or free-form command. [Tool definition][tool-definition] [Registration][registration] [Registry][registry]

The typed verifier detects the Go runner, obtains the package selectors, defaults an omitted selection to `./...`, validates containment and prepares the shared build environment. Its build branch then creates a temporary directory, appends `-o <directory>`, and executes through the existing process-ownership wrapper. It has the same no-main-only fallback. [Selection][typed-selection] [Containment][containment] [Typed build][typed-run]

The returned JSON retains the directory, exit code and output; the argv uses a disclosed temporary-directory placeholder. If the transformed invocation exits zero, the tool returns no execution error. Nothing in this path enumerates the library packages omitted by Go's directory-output branch. The result is truthful about the command it ran but insufficient for the tool's selected build obligation.

This is distinct from the already documented loss of explicit `.` selectors. The main witness uses an explicit `["./..."]` array, whose selector survives that normalization. It also applies to omitted packages under the build default, without using the test-only written-package scope. [Selector audit][prior-selector]

## 2. Why the Go flag changes coverage

The official Go 1.27.3 build driver first loads packages and checks package-loading errors. With an existing directory supplied as output, it then constructs an action by iterating the package list and skipping packages whose name is not `main`. Each retained executable root gets an install action, after which the driver executes that action and returns. Its ordinary no-output branch instead adds a build action for every selected package. [Go loading and branching][go-action]

Thus, for selected package set P and its selected executable roots M, the directory-output branch's compilation roots are M, not P. Libraries in the dependency closure of M still compile. Selected libraries outside that closure do not acquire a build action merely because `./...` matched them. This is a derivation from the driver, not a measured coverage result.

The distinction between package loading and compilation is important. Loading can still reject missing imports, invalid package metadata or other load errors before this branch; the report does not claim every defect in an omitted library is hidden. The witness uses a valid package/import section and an undefined identifier inside a function body. Go's loading error check reports already recorded loading errors, and the source-header reader parses imports rather than type-checking function bodies. [Loading errors][go-load-errors] [Header reader][go-reader]

Go's build command documentation also distinguishes its normal multi-package checking behavior from explicit output generation. The reference implementation, rather than an assumption that output flags are semantically inert, is decisive here. [Go build command][go-doc]

## 3. Conditional witness: a healthy command beside a broken library

This hypothetical module was **not created or run** during the review:

```text
go.mod
cmd/app/main.go
lib/value.go
```

- `go.mod` declares module `example.invalid/buildscope`, with a toolchain-compatible Go version.
- `cmd/app/main.go` contains `package main` and an empty `func main() {}`; it imports no local library.
- Before, `lib/value.go` contains `package lib` and `func Value() int { return 1 }`.
- After, only that function body changes to `func Value() int { return MissingIdentifier }`; no declaration of that identifier exists.

Preconditions:

1. Both packages are selected by `./...` in the same recognized module and admitted workspace.
2. The command and its ordinary dependencies compile, and the library has no independent loading error.
3. The library is not imported, directly or transitively, by any selected main package.
4. Use the ordinary build mode and stable relevant environment/configuration, without an owner override changing output or package semantics.
5. The temporary directory is created successfully; there is no cancellation, unsupported executable, incomplete capture, concurrent source edit or external tool failure.
6. The typed request is admitted through the registry and uses the array `["./..."]`; the G1 request uses its built-in selector.

The expected source-derived boundary results are:

| Boundary | Before | After |
|---|---|---|
| Ordinary compilation of both selected packages | Both compile | Library's function fails compilation |
| Main-package compilation closure | Healthy command only | Same healthy command only |
| Directory-output branch | Schedules the command | Schedules the same command |
| No-main fallback | Not entered | Not entered |
| G1 observation | Pass, zero findings | Pass, zero findings |
| Typed build result | Exit zero, no tool execution error | Exit zero, no tool execution error |
| Recorded package selectors | `./...` | `./...` |
| Correct full-domain build outcome | Pass | Fail |

The expected library diagnostic is not asserted as captured output. The test oracle must first establish it by an independent full-domain compilation in an authorized evaluation environment.

For standalone G1-only snapshots, the root and recorded package domain match. The facts supplied to the kernel are pass/zero on both sides, with no new finding. The current policy therefore has the inputs for an unchanged, keepable comparison. Its pass-to-fail rule would reject the pair if the producer had actually measured the broken library. [Snapshot projection][snapshot] [Comparison policy][policy]

A second useful, unrun metamorphic case starts with a broken library-only module: the no-main fallback can expose its error. Adding an unrelated healthy command, without fixing the library, can move the adapter to the main-only branch and turn G1 into apparent improvement. The fixed-package-set witness above is primary because it isolates the hidden regression without changing the set of packages.

### Bound the recurse consequence

The recurse CLI's optional `--gates` callback calls this same error-free measurer before work and after changed attempts. It narrows designated lint gates through `ScopedPackages`, but G1 remains constructed over `./...`. Snapshot facts enter the recurse ratchet directly. This false G1 pass can therefore remove the additional veto that G1 is meant to supply. [Recurse callback][recurse-cli] [Baseline][baseline] [After measurement][after] [Ratchet projection][ratchet]

That is not proof that a bad attempt will be retained. Ordinary declared/detected build and test gates, G2/G3, other analyzers, target-resolution requirements, mission evidence, scope guards and improvement conditions can independently reject it. Ordinary `gates.Run` executes its gate's argv and does not itself add this G1 temporary-directory transformation. A normal ordinary `go build ./...` gate is a meaningful protective control, not another alleged affected reader. [Ordinary runner][ordinary]

The typed tool result and standalone G1 claim are substantive without inventing a scenario in which every independent safeguard is bypassed.

## 4. Counterexamples, tests and novelty

### Cases that do not establish this defect

- **Library-only selection:** the current no-main fallback can compile the libraries normally. Its presence does not cover a mixed selection.
- **Library imported by a selected command:** compilation of that dependency can reveal the error.
- **Loading failure in an otherwise skipped package:** the driver's earlier loading checks may reject the entire command.
- **Healthy mixed module:** exit zero alone cannot prove which selected packages compiled; a compile-breaking sentinel is necessary.
- **Explicit command-only request:** excluding unrelated libraries is intended when they are not selected.
- **A failed or cancelled process:** no false successful-exit witness is claimed.
- **Other build modes or overrides:** these require separate qualification; the minimal witness uses default build semantics.

### Existing authored coverage does not test the mixed boundary

`TestErrorFree_MeasuresRealGatesAndRefusesARegression` builds a library-only fixture, so G1's no-main fallback is sufficient for that case. Its planted errors concern vet and formatting, not an unimported library beside a command. [G1 fixture][g1-test]

`TestTypedVerification_BuildLeavesNothingOnDisk` builds a single-main fixture and checks that no binary remains in the workspace. That protects an important output-hygiene contract but never adds a separately selected broken library. The written-package/default-build test asserts the selected `./...` spelling; that is not evidence that the transformed Go command compiled all matched packages. These tests were read, not run. [Output-hygiene fixture][tool-test] [Default selection fixture][default-test]

### Bounded novelty screen

The current audit inventory/queue and adjacent reports were inspected, with repository searches for the exact fallback text, `buildOutput`, temporary-directory build terms and unimported-package references. The fallback appeared in the two production adapters, not in a prior audit found by this screen. This is not a claim about private reports or all historical branches. [Audit queue][queue]

Related work remains distinct:

- **Gate comparison domain admission** concerns whether before/after measurements have compatible declared domains. Here the same declared domain is supplied on both sides but the invocation does not compile that whole domain.
- **Explicit package selector fidelity** concerns loss of a valid selector during argument normalization. Here `./...` survives intact; the host-added output option changes its effective compilation.
- **Error-free Go test package failure census** concerns suppression of an observed terminal failure by G3's reader. Here G1 never schedules the missing package's compilation.
- **Legacy build diagnostic evidence** concerns parsing an already unsuccessful legacy invocation. This witness has a successful process and no omitted library diagnostic to parse.
- **Verification process cleanup parity** concerns cancellation ownership, not the selected build action graph.

[Domain audit][prior-domain] [Selector audit][prior-selector] [G3 audit][prior-g3] [Legacy audit][prior-legacy] [Cleanup audit][prior-cleanup]

## 5. Proposed build contract and bounded repair direction

Specify this invariant in the owning build/tool and campaign measurement contracts:

> A build adapter may redirect or discard artifacts, but it must preserve compilation of every package selected by the admitted request under the effective build configuration. An artifact-suppressed success is full-domain evidence only after that equivalence is established.

The contract should require:

1. **Conserve compilation scope.** Handle libraries, executable packages, and mixed selections together. Dependencies of selected roots are not a substitute for independently selected roots.
2. **Preserve output hygiene.** The correction must not restore binaries, Windows executable rotation or output-directory litter in the workspace.
3. **Use one qualified behavior across adapters.** G1 and typed `run_build` should not maintain subtly different artifact-discard workarounds. A shared helper is one option; shared conformance fixtures are required even if adapters remain separate.
4. **Retain invocation identity.** Preserve the selected package array, canonical working directory, effective build flags, environment policy and relevant toolchain semantics. Record any deliberately narrowed operation as narrowed rather than leaving a full-domain label.
5. **Preserve outcomes and executive ownership.** The host reports the actual command/coverage/error. Mangle continues deciding regression and adoption. Do not suppress the corrected library errors because a previous G1 implementation missed them.
6. **Keep old evidence honest.** Existing pass snapshots from the directory-output implementation do not establish the strengthened guarantee for mixed modules. Remeasure where that assurance is needed; do not silently upgrade historical evidence.
7. **Retain failure behavior.** Unavailable tools, loading errors, compile errors, cancellation, capture failure and artifact cleanup problems remain separately attributable. This change is not permission to execute arbitrary user/model commands.

A concrete candidate worth qualifying is Go's explicit null-output handling. In the pinned driver, a recognized null destination is cleared before output-branch selection and execution falls through to all-package build actions. The initial default-single-main naming step does not replace a nonempty explicit destination. The official helper recognizes the platform's null device and Windows's case-insensitive `NUL`. This is a source-backed candidate, **not an implemented or validated recommendation to substitute an unqualified literal on every platform**. Default mode, supported platforms, configured flags and artifact behavior must be tested. [Go null branch][go-action] [Null identity][go-null]

Other designs can compile an exact resolved package set with appropriate unique/discarded outputs. They carry enumeration, subprocess and freshness costs. A second plain build after every successful directory-output build might expose the library error, but blindly doing that can restore the single-main workspace-artifact problem and duplicate work. Deleting the output flag without preserving that requirement is not a complete repair.

## 6. Costs, failure modes and fair unrun evaluation

Correct compilation can be slower than the current incomplete check because it must compile independently selected libraries that were previously skipped. That is necessary work, not a performance regression to disguise. Warm Go caches can reduce repeated compilation cost; they cannot justify omitting a changed selected library. A suitable one-invocation discard mode may avoid temporary executable generation and the library-only retry, but no latency or disk saving has been measured.

Charge package loading, dependency graph work, compilation/linking appropriate to the chosen contract, cache reads/writes, output creation/cleanup, capture/parsing, snapshot persistence, fact projection and downstream retries. Include the cost of correcting a falsely trusted build, and any remeasurement needed for older snapshots. No additional LLM or embedding call is inherently required.

Repair risks include preserving only the obvious G1 path while leaving the model tool wrong, breaking single-main artifact hygiene, hardcoding a platform-specific null path, changing owner build flags, mistaking loading success for compilation coverage, double-running expensive builds, and claiming a successful migration for old snapshots without fresh evidence.

All of the following evaluations are **proposed and unrun**:

| Layer | Required evaluation |
|---|---|
| Core semantic witness | Fixed main plus independent library, valid headers and an undefined identifier only in the library's function body. Compare the actual adapter result with a separately confirmed full-domain compile oracle. |
| Metamorphic check | Broken library-only fixture before/after adding an unrelated healthy command. Adding that command must not erase the existing library failure. |
| Dependency controls | Move the same broken library into a command's import closure; library-only and explicit-library selections; healthy mixed selection; several commands and several disconnected libraries. |
| Load-versus-compile control | Separately inject missing imports or invalid package metadata. Do not count an early loader rejection as coverage of the function-body case. |
| Typed production route | Invoke admitted `run_build` through normal registry dispatch with explicit `["./..."]` and omitted packages. Verify returned exit/error/argv/directory, with unchanged authorization and environment behavior. |
| G1 production route | Use actual `nerd gates measure --only build`, save/load snapshots, then real-kernel `gates diff`. Require pass-to-fail and a visible library diagnostic for the introduced error. |
| Recurse integration | Enable optional G1 through the ordinary CLI, world model and CodeDOM active. Observe the corrected G1 veto and separately identify ordinary gates that already protect the attempt. No credit for an unrelated guard firing first. |
| Output hygiene | Single main, library-only and mixed selections on supported Unix and Windows; no binary creation/replacement/rotation inside the workspace, with preexisting user executables preserved. |
| Configuration | Default mode first, then supported build flags/tags, platform target settings, nested module working directory and configured environment. Report unsupported combinations explicitly. |
| Error paths | Tool unavailable, temporary-output failure for any retained directory strategy, compiler error, cancellation and partial capture. No partial work may become a complete pass. |
| Cost | Cold/warm caches, package counts, compile actions, wall/CPU time, bytes written, temporary disk, duplicate invocations and false-pass rate under identical selected domains. |

The oracle must establish compilation of the disconnected library, not just successful execution of a Go command containing `./...`. A mocked argv assertion is useful plumbing coverage but cannot prove flag semantics. A helper-only test cannot prove the normal registry or CLI returned the right conclusion. Run the same fixture through both adapters and preserve existing output-hygiene tests.

**Closure criterion:** adding or retaining a selected executable cannot make an independently selected library's compile error disappear from an artifact-suppressed build, and both production adapters establish that property without writing build artifacts into the workspace.

## Pinned references

[guidance]: https://github.com/theRebelliousNerd/codenerd/blob/582cfc954afeeb3cf5e433a0db6fac77c82bdd2a/agents.md
[architecture]: https://github.com/theRebelliousNerd/codenerd/blob/582cfc954afeeb3cf5e433a0db6fac77c82bdd2a/Docs/architecture/AGENTS.md
[campaign]: https://github.com/theRebelliousNerd/codenerd/blob/582cfc954afeeb3cf5e433a0db6fac77c82bdd2a/Docs/architecture/campaign/INTERNALS.md#L138-L149
[module]: https://github.com/theRebelliousNerd/codenerd/blob/582cfc954afeeb3cf5e433a0db6fac77c82bdd2a/go.mod#L1-L3
[g1-construction]: https://github.com/theRebelliousNerd/codenerd/blob/582cfc954afeeb3cf5e433a0db6fac77c82bdd2a/internal/gates/errorfree.go#L109-L135
[g1-run]: https://github.com/theRebelliousNerd/codenerd/blob/582cfc954afeeb3cf5e433a0db6fac77c82bdd2a/internal/gates/errorfree.go#L222-L245
[g1-classify]: https://github.com/theRebelliousNerd/codenerd/blob/582cfc954afeeb3cf5e433a0db6fac77c82bdd2a/internal/gates/errorfree.go#L247-L309
[count]: https://github.com/theRebelliousNerd/codenerd/blob/582cfc954afeeb3cf5e433a0db6fac77c82bdd2a/internal/gates/errorfree.go#L179-L193
[invocation]: https://github.com/theRebelliousNerd/codenerd/blob/582cfc954afeeb3cf5e433a0db6fac77c82bdd2a/internal/build/invocation.go#L12-L42
[cli]: https://github.com/theRebelliousNerd/codenerd/blob/582cfc954afeeb3cf5e433a0db6fac77c82bdd2a/cmd/nerd/cmd_gates.go#L37-L115
[tool-definition]: https://github.com/theRebelliousNerd/codenerd/blob/582cfc954afeeb3cf5e433a0db6fac77c82bdd2a/internal/tools/shell/execute.go#L508-L542
[registration]: https://github.com/theRebelliousNerd/codenerd/blob/582cfc954afeeb3cf5e433a0db6fac77c82bdd2a/internal/tools/shell/register.go#L7-L30
[registry]: https://github.com/theRebelliousNerd/codenerd/blob/582cfc954afeeb3cf5e433a0db6fac77c82bdd2a/internal/tools/registry.go#L330-L405
[typed-selection]: https://github.com/theRebelliousNerd/codenerd/blob/582cfc954afeeb3cf5e433a0db6fac77c82bdd2a/internal/tools/shell/verification.go#L36-L112
[containment]: https://github.com/theRebelliousNerd/codenerd/blob/582cfc954afeeb3cf5e433a0db6fac77c82bdd2a/internal/tools/shell/verification.go#L264-L287
[typed-run]: https://github.com/theRebelliousNerd/codenerd/blob/582cfc954afeeb3cf5e433a0db6fac77c82bdd2a/internal/tools/shell/verification.go#L113-L165
[snapshot]: https://github.com/theRebelliousNerd/codenerd/blob/582cfc954afeeb3cf5e433a0db6fac77c82bdd2a/internal/gates/errorfree_snapshot.go#L100-L134
[policy]: https://github.com/theRebelliousNerd/codenerd/blob/582cfc954afeeb3cf5e433a0db6fac77c82bdd2a/internal/core/defaults/policy/gates.mg#L29-L154
[recurse-cli]: https://github.com/theRebelliousNerd/codenerd/blob/582cfc954afeeb3cf5e433a0db6fac77c82bdd2a/cmd/nerd/cmd_campaign_recurse.go#L348-L371
[baseline]: https://github.com/theRebelliousNerd/codenerd/blob/582cfc954afeeb3cf5e433a0db6fac77c82bdd2a/internal/campaign/recurse_error_free.go#L27-L49
[after]: https://github.com/theRebelliousNerd/codenerd/blob/582cfc954afeeb3cf5e433a0db6fac77c82bdd2a/internal/campaign/recurse_cycle.go#L1331-L1444
[ratchet]: https://github.com/theRebelliousNerd/codenerd/blob/582cfc954afeeb3cf5e433a0db6fac77c82bdd2a/internal/campaign/recurse_policy.go#L257-L280
[ordinary]: https://github.com/theRebelliousNerd/codenerd/blob/582cfc954afeeb3cf5e433a0db6fac77c82bdd2a/internal/gates/run.go#L47-L110
[g1-test]: https://github.com/theRebelliousNerd/codenerd/blob/582cfc954afeeb3cf5e433a0db6fac77c82bdd2a/internal/gates/errorfree_test.go#L26-L109
[tool-test]: https://github.com/theRebelliousNerd/codenerd/blob/582cfc954afeeb3cf5e433a0db6fac77c82bdd2a/internal/tools/shell/verification_test.go#L117-L163
[default-test]: https://github.com/theRebelliousNerd/codenerd/blob/582cfc954afeeb3cf5e433a0db6fac77c82bdd2a/internal/tools/shell/verification_test.go#L171-L199
[queue]: https://github.com/theRebelliousNerd/codenerd/blob/582cfc954afeeb3cf5e433a0db6fac77c82bdd2a/Docs/audits/ASTRA_QUEUE.md
[prior-domain]: https://github.com/theRebelliousNerd/codenerd/blob/582cfc954afeeb3cf5e433a0db6fac77c82bdd2a/Docs/audits/gate-comparison-domain-admission.md
[prior-selector]: https://github.com/theRebelliousNerd/codenerd/blob/582cfc954afeeb3cf5e433a0db6fac77c82bdd2a/Docs/audits/explicit-package-selector-fidelity.md
[prior-g3]: https://github.com/theRebelliousNerd/codenerd/blob/582cfc954afeeb3cf5e433a0db6fac77c82bdd2a/Docs/audits/errorfree-go-test-package-failure-census.md
[prior-legacy]: https://github.com/theRebelliousNerd/codenerd/blob/582cfc954afeeb3cf5e433a0db6fac77c82bdd2a/Docs/audits/legacy-build-project-diagnostic-evidence.md
[prior-cleanup]: https://github.com/theRebelliousNerd/codenerd/blob/582cfc954afeeb3cf5e433a0db6fac77c82bdd2a/Docs/audits/verification-process-cleanup-parity.md
[go-release]: https://github.com/golang/go/commit/2aae1000ea1d4b28923b96db28acf4e3b99d6aff
[go-doc]: https://github.com/golang/go/blob/2aae1000ea1d4b28923b96db28acf4e3b99d6aff/src/cmd/go/internal/work/build.go#L33-L60
[go-action]: https://github.com/golang/go/blob/2aae1000ea1d4b28923b96db28acf4e3b99d6aff/src/cmd/go/internal/work/build.go#L460-L561
[go-load-errors]: https://github.com/golang/go/blob/2aae1000ea1d4b28923b96db28acf4e3b99d6aff/src/cmd/go/internal/load/pkg.go#L3138-L3167
[go-reader]: https://github.com/golang/go/blob/2aae1000ea1d4b28923b96db28acf4e3b99d6aff/src/go/build/read.go#L265-L314
[go-null]: https://github.com/golang/go/blob/2aae1000ea1d4b28923b96db28acf4e3b99d6aff/src/cmd/go/internal/base/path.go#L81-L93

