---
doc-class: deep-dive
subsystem: campaign-recurse-finding-admission
implementation-status: proposed-refinement
evidence-status: source-only
last-verified: 2026-11-04
verified-against: 45e9161503532cd959e3ccc242e1694fbdfc297c
verification-method: static-source-inspection
main-rechecked: 2026-11-04
supersedes: []
---

# Preserve passing Go package verdicts in recurse findings

## Finding and decision

**A passing Go package can become an ordinary recurse repair candidate because its log contains a line beginning `panic:`.** The JSON finding reader buffers that package-level output, consumes but ignores the package's explicit `pass` event, and promotes the buffered text to a package failure at end of input. An unrelated package must actually fail so the enclosing command is red and the reader runs. The passing package need not panic, skip its tests, or emit incomplete output. [Parser entry][entry] [Event dispatch][dispatch] [Panic buffer][panic] [Finalization][finish]

This is a **conditional false-positive finding-admission defect**. The demonstrated source consequence is an invented finding in the ordinary workspace gate census, attributed to the passing package's node and eligible for repair selection. If selected, the normal attempt path receives that finding and the genuine failing workspace command. This report does not establish that a model changes the package, that any such change is kept, or that this has occurred in a deployed run. [Measurement][measure] [Open findings][open] [Policy projection][projection] [Attempt input][attempt]

The bounded correction is to retain explicit package terminal verdicts within each gate invocation and prevent a completed package pass from being contradicted by the buffered panic-text fallback. Preserve real failed-package and init-panic coverage, invocation identity, exact package attribution, uncertainty handling, write guards and the existing ratchet. Do not solve this by ignoring all panic output, suppressing an entire failing invocation, or asking a model to decide whether machine-reported pass means pass.

**Evidence boundary:** public main was pinned to [45e9161503532cd959e3ccc242e1694fbdfc297c][pin]. Root and scoped guidance, campaign architecture, the complete current audit inventory, related reports, the ordinary gate/recurse path and authored tests were inspected. Cached CodeNERD sources were checked against the pin's Git blob identities. The Go producer's primary documentation and implementation at [2aae1000ea1d4b28923b96db28acf4e3b99d6aff][go-pin], resolved from `go1.27.3`, were separately inspected. No CodeNERD or Go program, test, build, gate, model call, inference, training or benchmark was run. No implementation, credential, setting or permission was changed. All fixture outcomes and costs below are proposed or source-derived, not measured results.

## Architectural role and current status

The root design puts creative problem solving in the model and executive control in deterministic logic. Campaign guidance separately requires real host execution for test evidence. A finding reader is therefore part of the admission boundary: it translates an external command's observations into facts the executive can use. Correct Mangle scheduling cannot recover a terminal verdict discarded before fact assertion. [Root guidance][root] [Campaign contracts][campaign-guidance]

The campaign mission specification preserves the existing recurse scheduler, acceptance witness, regression checks and owned compensation. This issue occurs earlier and also applies to ordinary non-mission recurse. It needs neither mission mode nor the optional error-free gate ladder. Current source, rather than an older architectural wiring description, determines the consumer path below. [Mission contract][mission] [Architecture guidance][architecture] [Campaign corpus][campaign-readme]

| Boundary | Status at the pin | Limit relevant here |
|---|---|---|
| Owner-declared Go JSON test command | Implemented | `commands.test` becomes a workspace gate and overrides the detected test kind. The built-in detected Go test command itself is not JSON. [Declaration][detect] [Defaults][defaults] |
| Whole-command verdict | Implemented | The runner derives `Passed` from actual process completion and configured acceptable exits. A failing package can make a mixed invocation red. [Runner][runner] |
| JSON action recognition | Implemented | `pass` is recognized as structured Go output and consumed. [Recognition][recognition] |
| Package terminal-pass retention in this reader | Missing | The JSON handler has no pass branch; neither package completion nor a passing named test records successful package status. [Dispatch][dispatch] [Failure handler][fail] |
| Package-level panic fallback | Implemented, too broad | Pending `panic:` output is emitted at EOF unless a failure-oriented suppression map has already covered the package. [Finalization][finish] |
| Recurse attribution and policy input | Implemented | The synthetic finding can enter the measured census and the node's `recurse_finding` relation. [Measurement][measure] [Projection][projection] |
| Per-finding source invocation | Implemented | The exact reporting invocation is retained; preserving the correct argv does not repair an incorrect finding extracted from it. [Source selection][source] |
| Positive terminal-verdict contract and new controls | Proposed | No implementation or passing evaluation is claimed here. |

The target is not to make every red command finding-free. It is to keep an invocation's genuine failure while excluding an unsupported failure assigned to a package that explicitly passed.

## Producer contract: output and verdict are separate observations

Go's `test2json` protocol distinguishes event action from output bytes. Package identity disambiguates interlaced streams; an event without a test name concerns the package. Output carries the program's stdout/stderr rather than asserting that a matching word is an actual process failure. A terminal package `pass` and a line of output containing the characters `panic:` are consequently compatible observations. [Go protocol][go-protocol]

Go permits `TestMain` to perform setup before `m.Run` and to return after it; the test wrapper uses the run result for process exit. A setup print before tests start provides an ordinary way to emit the witness's package-level text without an unrecovered panic. [Testing contract][testing]

The pinned converter reinforces that distinction. It starts with an empty active test name, emits ordinary output events, records process success as pass, clears the active name on final PASS/FAIL framing and emits a terminal package result when closing. The producer has already supplied the result the consumer needs. This inspection establishes protocol plausibility, not which Go release is installed in any user's environment. [Converter initialization][go-init] [Process exit][go-exit] [Final framing][go-final] [Output and event identity][go-output] [Terminal emission][go-close]

The report uses only this narrow producer contract. It does not assume arbitrary test logs are trustworthy instructions, equate skipped packages with tested packages, or infer success from the absence of a diagnostic.

## Source trace through the ordinary recurse path

### A supported declaration produces one mixed workspace invocation

`gates.Detect` loads `nerd.md`, converts nonempty build/test/lint command strings to gates, and omits detected gates of the declared kind. For a direct Go test command, `fromNerdMD` records `ScopeAll`, language `go`, and ID `nerd.md:test`. The runner executes that argv with the workspace command builder and records the process result. [Declaration][detect] [Runner][runner]

The primary witness uses an unfocused sweep and an owner declaration equivalent to:

```yaml
commands:
  test: go test -count=1 -json ./...
```

This is the commands portion of an otherwise valid `nerd.md`, not a claim that the fragment alone is the complete project document. The default detected `go:test` uses `-count=1 -cover` and a node package token; this report does not claim that every default recurse test invocation takes the JSON path. [Defaults][defaults]

Focus matters. For a focused Go sweep, `scopeGateToNodes` replaces `./...` with the selected package patterns. If that excludes the genuinely failing package, the invocation can pass and the reader returns immediately. The primary witness deliberately does not use such a focus. [Focus projection][focus]

### Structured pass is lost while a textual hint survives

`FindingsAll` returns no findings for a wholly passing or unverified result. Otherwise it parses the output with a fresh `goStream`. Valid JSON with a recognized action is consumed by `onJSON`, including `pass`. The handler acts only on build output, build failure, output and failure. Thus a package pass is neither mistaken for raw text nor rejected as malformed: it is recognized and discarded. [Parser entry][entry] [Recognition][recognition] [Event dispatch][dispatch]

`onOutput` examines each output line for the exact prefix `panic:`. When `Test` is empty and `Package` is present, it retains the first such line in `jsonPkgPanic[package]`. Named-test output uses a separate map and is not this witness. [Panic buffer][panic]

Two names can obscure the missing state:

- `jsonTested` is set when a **named test fails**, not when any test executes or passes.
- `jsonPkgDone` is set while processing or suppressing package **failures**; it is not set by a package pass.

At EOF, `finish` sorts the packages with buffered panic text and calls `notePackageFail` for any not covered by those maps. That helper emits a package-target finding, then marks it covered and done. A successfully tested and explicitly passing package can therefore satisfy every condition for this fallback. [Failure handler][fail] [Package fallback][package-fail] [Finalization][finish]

### The false finding becomes ordinary executive input

For each invocation, `recurseRun.measure` calls `FindingsAll`, maps each target to a node, inserts its ID into the uncapped census, retains the source invocation and keeps up to the work-selection cap. The witness has two findings, far below that cap. Target `a` maps directly to the node owning path `a`; there is no unresolved import-path guess. [Measurement][measure] [Module mapping][mapping] [Node attribution][attribute]

`open` forwards workspace findings attributed to the current node. The policy adapter asserts their IDs, kind, target and signature as `recurse_finding`. The Mangle rules rank test findings and admit eligible, unattempted, non-stalled, non-refused candidates. They receive no package-pass fact with which to reject this specific false premise. [Open findings][open] [Policy projection][projection] [Candidate rules][candidate]

If selected, `attempt` retrieves the exact source invocation and constructs a repair attempt from its output and argv. The campaign adapter describes the claimed failure, retains the whole command as acceptance and copies the selected node paths into the task write set. This is a concrete route to unnecessary repair work, not a claim that a model will obey the false diagnosis or obtain a kept commit. [Attempt input][attempt] [Source selection][source] [Campaign handoff][handoff]

The existing controls remain valuable: selection can be blocked by history or admission, acceptance can stay red because of the real failure, and the ratchet still checks resolution, changes, regressions and refusal. These limit downstream consequences; they do not make the initial false finding true. [Ratchet][ratchet]

## Concrete causal witness

### Workspace and event assumptions

Construct a fresh temporary Go module named `example.com/passguard` with independent ordinary packages `a` and `z`. Give both a production `.go` file so both appear as real subsystem nodes. Use a clean repository and the declared workspace test command above. Other required build/lint checks are healthy. Use a fresh, unfocused recurse state with no historic refusal or stall for the candidate. Keep captured output complete and well below `OutputLimit`.

Package `a` has a passing `TestOK` and a `TestMain` whose setup prints a literal diagnostic-looking message before calling `m.Run`, then returns normally. Package `z` has `TestBad`, which calls `t.Fail()`. No actual panic or recover call is required in `a`; the diagnostic text is data.

The essential test source for `a` would be:

```go
package a

import (
    "fmt"
    "testing"
)

func TestMain(m *testing.M) {
    fmt.Println("panic: expected recovery diagnostic")
    m.Run()
}

func TestOK(t *testing.T) {}
```

This code is a proposed fixture inside the document, not a file added to the repository or a reproduction that was executed. Printing before `m.Run` avoids relying on output attribution after the last named test.

The following is a **constructed event projection**, not a captured complete Go transcript. It retains the events needed by this reader; real framing output, timestamps and elapsed fields are omitted. A capture-based evaluation must retain the complete original stream rather than present this projection as runtime evidence.

```json
{"Action":"start","Package":"example.com/passguard/a"}
{"Action":"output","Package":"example.com/passguard/a","Output":"panic: expected recovery diagnostic\n"}
{"Action":"run","Package":"example.com/passguard/a","Test":"TestOK"}
{"Action":"pass","Package":"example.com/passguard/a","Test":"TestOK"}
{"Action":"pass","Package":"example.com/passguard/a"}
{"Action":"start","Package":"example.com/passguard/z"}
{"Action":"run","Package":"example.com/passguard/z","Test":"TestBad"}
{"Action":"fail","Package":"example.com/passguard/z","Test":"TestBad"}
{"Action":"fail","Package":"example.com/passguard/z"}
```

The packages may interleave while preserving each package's own order. The failure is not dependent on `a` preceding `z`: finalization examines the retained package map after the stream is consumed.

### State transition and expected observation

| Observation | Current reader state or effect |
|---|---|
| Whole command fails because `z` fails | `Result.Passed` is false; `Err` is nil, so parsing proceeds. |
| Package-level panic-prefixed output from `a` | `jsonPkgPanic[a]` stores the literal text. |
| `TestOK` passes | Recognized action, no state change in `onJSON`; `jsonTested[a]` remains false. |
| Package `a` passes | Recognized action, no state change; `jsonPkgDone[a]` remains false. |
| `TestBad` fails in `z` | A real finding targets `z::TestBad`; failure maps cover `z`. |
| Package `z` fails | Its package fallback is suppressed because the named test already explains the failure. |
| EOF | The pending text for `a` becomes a second finding targeting `a`. |
| Recurse opens node `a` | The extra finding is available to policy as kind `/test`. |

For this fixture, the source-derived current finding set is:

- `z::TestBad`, message `test failed: TestBad`, supported by a test failure event.
- `a`, message `panic: expected recovery diagnostic`, contradicted as a package failure by the explicit package pass.

The desired set contains the first finding only. The workspace gate must remain failed. A change from two findings to one is not a newly passing command and must not be reported as such.

The claim stops at admission and conditional dispatch. No assertion about the first node attempted is needed. No output truncation, lost verdict, missing test, stale snapshot, crossed invocation, digit-normalization collision, model mistake or accepted patch is required to establish the mismatch.

## Required contract and bounded refinement

### Verdict precedence

For one direct Go JSON invocation and one package identity, an observed terminal package pass must prevent buffered package-level panic text from becoming a failed-package finding. Keep that text available as diagnostic context if useful, but do not promote it into a contradiction of the terminal result. This is a precedence rule between two fields in the same evidence stream, not a general rule that positive assertions override negative evidence.

Package and test verdicts remain separate. One passing named test does not prove that its package passed; another test or `TestMain` may fail later. Conversely, a package pass must not erase real failures from a different package. Do not use the current failure-only `jsonTested` map as a substitute for an explicit package verdict.

### Completeness and ambiguity

When a package has no terminal verdict, do not invent a pass. Real init panics can lack named-test failures, and the current fallback exists to retain that evidence even after other packages have failed. Preserve that behavior for compatible inputs or represent it with an explicit incomplete/provisional disposition before changing scheduling semantics. A truncated, malformed or contradictory stream is a different condition from this report's complete, noncontradictory passing package.

For a single well-formed direct `go test -json` package stream, an explicit pass and an explicit package fail are not interchangeable observations. If wrapper commands concatenate multiple runs under the same package key, or malformed input supplies conflicting terminal events, the implementation must state how it recognizes run boundaries or surfaces ambiguity. A global “last pass wins” rule can erase a real earlier failure and is not the proposed contract. A one-time deletion of buffered text on pass is also insufficient if later output can repopulate it without a new run identity.

### Implementation boundary

The narrow candidate is package terminal-state tracking in `goStream`, consulted when deciding whether buffered package text may become a finding. Existing finding identity, module mapping, per-invocation source retention and Mangle choice remain unchanged. Terminal state must be scoped to each `FindingsAll` invocation, not cached across passes or commands. Any invalid-stream disposition must remain deterministic and inspectable.

CodeNERD already has a useful comparison in `internal/testfacts`: its parser records package and test verdicts separately, and failure extraction checks failure status before examining package panic text. That demonstrates an existing representation of the needed distinction, not that the other parser should be dropped in wholesale. Its repeated-test semantics, retained-output model and diagnostic coverage are separate contracts. [Testfacts verdicts][testfacts-verdict] [Testfacts failure extraction][testfacts-fail]

Before choosing reuse, compare exact target and ID stability, Go build-event support, output limits, missing-terminal behavior, raw text fallback and allocations. A small correction to the live reader may be safer than coupling two consumers with different purposes. A shared terminal-state helper is an option only if both consumers' contracts are made explicit and exercised.

### Controls that must remain in place

- Keep the actual whole-command exit verdict. Removing `a` must not turn the genuinely red `z` command green.
- Preserve true failed named tests, true package/init panics, build failures and their exact package targets.
- Keep the all-pass and unverified early returns, but do not misuse either to hide a mixed invocation's real failures.
- Preserve focus projection, the work-selection cap versus full census, and source invocation identity.
- Do not confer tested coverage from package skip, no-test output or a missing verdict.
- Preserve owner scope, normal campaign admission, acceptance, compensation and the ratchet. Parser confidence is not write permission.

## Failure modes and tradeoffs

**Suppressing every panic line loses real failures.** Authored tests explicitly preserve init panic findings, including when earlier packages have already failed. The correction must distinguish a confirmed pass from an absent negative event, rather than delete the fallback. [Init-panic fixtures][init-tests]

**Treating any named-test pass as package success is unsound.** A package can have both passing and failing tests. Only package-scoped terminal evidence can settle the package-level fallback in the primary witness.

**Silencing the whole invocation breaks the real failure census.** The unrelated `z::TestBad` remains a mandatory finding. One false positive is not grounds to downgrade the command's actual failed verdict.

**Regex exceptions are fragile.** Whitelisting the witness's phrase would leave other ordinary output strings exposed and could hide real panic text containing the same words. The defect is event-state handling, not an incomplete dictionary of benign messages.

**Prompt filtering moves a machine contract into a heuristic stage.** A model might notice the later pass in a retained transcript, but the false fact has already entered the census and scheduler. If the transcript is long, the attempt description can also retain the false headline while evidence presentation is bounded. The deterministic fix belongs before selection. [Attempt presentation][presentation]

**Overgeneralized terminal handling can conceal mixed-run failure.** Per-package pass state must not bleed across distinct invocations, and contradictory records need an explicit policy. The regression suite should make these boundaries visible rather than rely on whichever event happened to arrive last.

**A corrected parser does not prove completion of the task.** Even if the false candidate disappears, `z` still fails. The report does not claim a faster successful repair, improved model accuracy, safe adoption of a patch, or an end-to-end mission gain without evaluating those outcomes separately.

## Full cost account

Let `B` be captured output bytes, `E` recognized events and `P` distinct package identities in one invocation. The current reader already scans and decodes output, splits lines, maintains diagnostic maps and sorts the packages with buffered panic text. A bounded terminal-state refinement adds approximately `O(E)` state updates and `O(P)` status entries, within the existing `O(B)` parsing work; package-fallback ordering remains up to `O(P log P)`. Exact allocations depend on representation and must be measured. This is not an argument that the cost is zero.

The runner retains at most `OutputLimit` bytes of original output, currently 1 MiB, using head and tail retention; its returned string adds a truncation marker when bytes were dropped. This bounds retained original bytes, not the number of semantic facts guaranteed present after truncation. The proposed complete-stream witness is much smaller. A refactor that accumulates another full copy of every test's output can add memory even when its terminal-state logic is simpler. Compare retained bytes and allocations before sharing the larger `testfacts` accumulator. [Output bound][output-bound] [Testfacts output routing][testfacts-output]

The cost of today's false candidate is conditional but broader than one map entry:

1. The census retains an extra ID, target, message and source association; policy receives an extra candidate.
2. If selected and admitted, a normal repair campaign can consume planning/execution work, model tokens, filesystem observations and gate invocations.
3. The acceptance command still covers the whole workspace. An unnecessary attempt on `a` can therefore rerun the same genuine failure in `z` without resolving it.
4. Repeated visits can carry outcome/history state until existing stall or refusal rules prevent another attempt; those rules limit retries but do not repair the diagnosis.
5. A maintainer may spend time investigating a passing package because the host labels its output a failure.

Do not report these as measured token, money or latency savings. The minimum deterministic comparison is candidate precision while preserving true-failure recall. Later timing should separate parser CPU/allocation cost, gate wall time, number of selected/blocked/started attempts, model usage, verification work and compensation. Gate runtime or provider latency can dominate parser overhead; a smaller candidate count alone is not an end-to-end speedup result.

## Unrun evaluation plan

The evaluation below is proposed documentation. No fixture, test or benchmark was run for this report. Use temporary workspaces and the actual production parser/measurement path, with no provider calls for the deterministic checks.

### Parser controls

| Case | Required observation |
|---|---|
| Primary complete mixed stream | `a` has explicit test and package pass plus benign package panic text; `z` fails. Return only `z::TestBad`, with whole gate still failed. |
| All packages pass | Keep the existing early return: no findings, even if benign diagnostic text is present. |
| Actual init panic | Preserve the package finding and panic message with no named-test failure. |
| Actual init panic after another package fails | Preserve both independent failures; keep the existing authored fixture intent. |
| Benign package text, one named test passes, another fails | Preserve the real failed test. A named-test pass must not settle package success. |
| Panic-prefixed text on a passing named test | Do not promote test output to a failure; keep test and package state distinct. |
| Explicit package fail with no named-test failure | Preserve the package failure even if a different package passes. |
| Build failure and passing noisy package | Preserve compiler/package evidence without adding a false finding for the passed package. |
| Interleaved packages | Permute cross-package ordering while preserving each package's order; findings must not migrate or change. |
| Missing terminal verdict | Do not infer pass. Exercise the selected incomplete/fallback contract without losing real init panic evidence. |
| Conflicting terminal records or concatenated runs | Surface the documented invalid/multi-run disposition; a later pass must not silently erase an independent earlier failed run. |
| Unverified or truncated invocation | Preserve unverified behavior and explicit truncation handling; do not present this as the complete-stream success case. |

Add variant names, module-prefix near misses and package/test names shared across packages to prove identity isolation. Existing authored captures for Go JSON failures and init panic are controls to retain, not evidence that the new case already passes. [Existing JSON fixture][json-test] [Init-panic fixtures][init-tests] [Existing pass/unverified fixture][pass-test]

### Production-path oracle

1. Generate and retain the complete Go JSON output for the two-package fixture in an explicitly authorized offline evaluation. Record toolchain version, exact argv, exit code, output bytes and truncation flag. Check that `a` has a real package pass and `z` a real failure before judging CodeNERD.
2. Feed the same immutable captured result through the current and candidate `FindingsAll`. Compare exact targets, messages and stable IDs of surviving real findings, not just count.
3. Exercise `measure` and `open` with real subsystem nodes and a deterministic temporary command that returns the captured bytes and exit code through the real gate runner. Verify that the current source admits `a` while the refinement does not, and both preserve `z` and the correct source invocation. Keep capture replay distinct from the separate real-Go producer check.
4. Use the real policy adapter with fresh state to show candidate eligibility, then a recorder-only executor to observe the handoff if selected. Do not invoke a model or claim that such a recorder proves repair quality.
5. Repeat with focused `a` only using the real declared Go command rather than the fixed capture-replay command in step 3; `./...` is rewritten and the gate passes; that is a necessary negative control, not a reproduction of the primary defect.
6. Repeat invocations in one process with opposite outcomes to ensure no terminal state leaks between calls. Exercise bounded output and missing verdict separately.

The decisive acceptance condition is: **a complete, internally consistent package pass cannot acquire a package-failure candidate solely from its output prefix, while every independently failing package in the same invocation retains its warranted finding.** Successful global exit is not required because the witness deliberately includes `z`'s true failure.

### Fair cost and quality comparison

Use identical captures and ordering across the pinned reader, a narrow terminal-state correction and any shared-parser alternative. Include small normal streams, many packages, repeated output, actual init panics, mixed build/test failures and near-limit output. Report package-level false positives, true-failure coverage, candidate membership, CPU time, allocations and retained bytes. Keep any capture-generation cost separate from parser timing.

Only a separately authorized end-to-end study can compare model tokens, dollars, repair latency or kept-change quality. Hold provider, model, seed policy, budget, gate set and repository revision fixed, and report failed/blocked attempts as well as successes. No particular percentage improvement, runtime bound or model benefit is predicted here.

## Overlap and priority

The current audit inventory and nearby reports were checked before selecting this boundary. The novelty claim is bounded to the inspected source and corpus, not a claim to have searched every historical discussion.

| Existing report | Boundary it owns | Why this report remains distinct |
|---|---|---|
| [Error-free Go package-failure census][prior-g3] | The separate G3 reader suppresses real package failures when compile findings already exist. | This is ordinary `FindingsAll`, inventing a failure despite an explicit pass. No optional ladder or compile error is needed. |
| [Test verdict diagnostic coverage][prior-verdict] | A different session consumer can drop a negative test verdict when diagnostics provide no matching explanation. | This reader discards a positive package verdict and promotes unrelated output. Different producer-to-consumer boundary and opposite error. |
| [Recurse diagnostic subject identity][prior-identity] | Normalization can conflate distinct diagnostic subjects. | The primary witness has exact, different package identities and no normalization collision. |
| [Collapsed finding invocation identity][prior-invocation] | A finding could be handed the wrong member invocation. | Current `byFinding` retention is intact; this report uses one correct invocation containing a false finding. |
| [No-tests evidence continuity][prior-no-tests] | Empty collection and testing disposition. | `a` actually executes and passes a named test; there is no empty-suite inference. |
| [Finding census completeness][prior-census] | Work-selection caps and truncated output can hide existing findings. | The stream is short and complete; the error is an extra member, not a missing one. |
| [Mission acceptance scope][prior-scope] | Whole-command acceptance can exceed a node's repair scope. | That can compound the cost after selection but does not cause the false candidate and is not re-reported as a new finding. |
| [Rename effect census][prior-rename] | Git adoption can omit a rename source deletion. | Independent of test observation, finding admission and this witness. |

Prioritize the narrow parser contract and offline negative/positive controls before changing recurse policy or broadening a shared parser. Its value is precise: the executive should spend repair effort on supported failed subjects, not on a passing package whose ordinary output resembles a failure marker. Full execution, performance and end-to-end qualification remain open.

## Sources

[pin]: https://github.com/theRebelliousNerd/codenerd/commit/45e9161503532cd959e3ccc242e1694fbdfc297c
[root]: https://github.com/theRebelliousNerd/codenerd/blob/45e9161503532cd959e3ccc242e1694fbdfc297c/agents.md#L1-L20
[campaign-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/45e9161503532cd959e3ccc242e1694fbdfc297c/internal/campaign/agents.md#L1-L15
[architecture]: https://github.com/theRebelliousNerd/codenerd/blob/45e9161503532cd959e3ccc242e1694fbdfc297c/Docs/architecture/AGENTS.md#L1-L74
[campaign-readme]: https://github.com/theRebelliousNerd/codenerd/blob/45e9161503532cd959e3ccc242e1694fbdfc297c/Docs/architecture/campaign/README.md
[mission]: https://github.com/theRebelliousNerd/codenerd/blob/45e9161503532cd959e3ccc242e1694fbdfc297c/Docs/architecture/campaign/MISSION-IMPROVEMENT.md#L98-L114
[detect]: https://github.com/theRebelliousNerd/codenerd/blob/45e9161503532cd959e3ccc242e1694fbdfc297c/internal/gates/detect.go#L15-L71
[defaults]: https://github.com/theRebelliousNerd/codenerd/blob/45e9161503532cd959e3ccc242e1694fbdfc297c/internal/gates/detect.go#L120-L132
[runner]: https://github.com/theRebelliousNerd/codenerd/blob/45e9161503532cd959e3ccc242e1694fbdfc297c/internal/gates/run.go#L15-L99
[output-bound]: https://github.com/theRebelliousNerd/codenerd/blob/45e9161503532cd959e3ccc242e1694fbdfc297c/internal/gates/run.go#L102-L134
[entry]: https://github.com/theRebelliousNerd/codenerd/blob/45e9161503532cd959e3ccc242e1694fbdfc297c/internal/gates/findings.go#L112-L184
[recognition]: https://github.com/theRebelliousNerd/codenerd/blob/45e9161503532cd959e3ccc242e1694fbdfc297c/internal/gates/findings.go#L286-L348
[dispatch]: https://github.com/theRebelliousNerd/codenerd/blob/45e9161503532cd959e3ccc242e1694fbdfc297c/internal/gates/findings.go#L444-L455
[panic]: https://github.com/theRebelliousNerd/codenerd/blob/45e9161503532cd959e3ccc242e1694fbdfc297c/internal/gates/findings.go#L491-L510
[fail]: https://github.com/theRebelliousNerd/codenerd/blob/45e9161503532cd959e3ccc242e1694fbdfc297c/internal/gates/findings.go#L512-L540
[package-fail]: https://github.com/theRebelliousNerd/codenerd/blob/45e9161503532cd959e3ccc242e1694fbdfc297c/internal/gates/findings.go#L542-L552
[finish]: https://github.com/theRebelliousNerd/codenerd/blob/45e9161503532cd959e3ccc242e1694fbdfc297c/internal/gates/findings.go#L554-L588
[mapping]: https://github.com/theRebelliousNerd/codenerd/blob/45e9161503532cd959e3ccc242e1694fbdfc297c/internal/gates/findings.go#L610-L652
[measure]: https://github.com/theRebelliousNerd/codenerd/blob/45e9161503532cd959e3ccc242e1694fbdfc297c/internal/campaign/recurse_cycle.go#L759-L839
[attribute]: https://github.com/theRebelliousNerd/codenerd/blob/45e9161503532cd959e3ccc242e1694fbdfc297c/internal/campaign/recurse_cycle.go#L859-L907
[focus]: https://github.com/theRebelliousNerd/codenerd/blob/45e9161503532cd959e3ccc242e1694fbdfc297c/internal/campaign/recurse_cycle.go#L637-L689
[open]: https://github.com/theRebelliousNerd/codenerd/blob/45e9161503532cd959e3ccc242e1694fbdfc297c/internal/campaign/recurse_cycle.go#L1007-L1031
[projection]: https://github.com/theRebelliousNerd/codenerd/blob/45e9161503532cd959e3ccc242e1694fbdfc297c/internal/campaign/recurse_policy.go#L64-L100
[candidate]: https://github.com/theRebelliousNerd/codenerd/blob/45e9161503532cd959e3ccc242e1694fbdfc297c/internal/core/defaults/policy/recurse.mg#L130-L255
[attempt]: https://github.com/theRebelliousNerd/codenerd/blob/45e9161503532cd959e3ccc242e1694fbdfc297c/internal/campaign/recurse_cycle.go#L1151-L1178
[source]: https://github.com/theRebelliousNerd/codenerd/blob/45e9161503532cd959e3ccc242e1694fbdfc297c/internal/campaign/recurse_cycle.go#L1677-L1692
[handoff]: https://github.com/theRebelliousNerd/codenerd/blob/45e9161503532cd959e3ccc242e1694fbdfc297c/internal/campaign/recurse_attempt.go#L95-L141
[presentation]: https://github.com/theRebelliousNerd/codenerd/blob/45e9161503532cd959e3ccc242e1694fbdfc297c/internal/campaign/recurse_attempt.go#L144-L168
[ratchet]: https://github.com/theRebelliousNerd/codenerd/blob/45e9161503532cd959e3ccc242e1694fbdfc297c/internal/core/defaults/policy/recurse.mg#L257-L329
[testfacts-verdict]: https://github.com/theRebelliousNerd/codenerd/blob/45e9161503532cd959e3ccc242e1694fbdfc297c/internal/testfacts/parse.go#L95-L169
[testfacts-output]: https://github.com/theRebelliousNerd/codenerd/blob/45e9161503532cd959e3ccc242e1694fbdfc297c/internal/testfacts/parse.go#L133-L144
[testfacts-fail]: https://github.com/theRebelliousNerd/codenerd/blob/45e9161503532cd959e3ccc242e1694fbdfc297c/internal/testfacts/failures.go#L40-L61
[json-test]: https://github.com/theRebelliousNerd/codenerd/blob/45e9161503532cd959e3ccc242e1694fbdfc297c/internal/gates/findings_go_test.go#L160-L184
[init-tests]: https://github.com/theRebelliousNerd/codenerd/blob/45e9161503532cd959e3ccc242e1694fbdfc297c/internal/gates/findings_go_test.go#L186-L246
[pass-test]: https://github.com/theRebelliousNerd/codenerd/blob/45e9161503532cd959e3ccc242e1694fbdfc297c/internal/gates/findings_test.go#L137-L145
[go-protocol]: https://pkg.go.dev/cmd/test2json
[testing]: https://pkg.go.dev/testing#hdr-Main
[go-pin]: https://github.com/golang/go/commit/2aae1000ea1d4b28923b96db28acf4e3b99d6aff
[go-init]: https://github.com/golang/go/blob/2aae1000ea1d4b28923b96db28acf4e3b99d6aff/src/cmd/internal/test2json/test2json.go#L125-L144
[go-exit]: https://github.com/golang/go/blob/2aae1000ea1d4b28923b96db28acf4e3b99d6aff/src/cmd/internal/test2json/test2json.go#L153-L161
[go-final]: https://github.com/golang/go/blob/2aae1000ea1d4b28923b96db28acf4e3b99d6aff/src/cmd/internal/test2json/test2json.go#L251-L262
[go-output]: https://github.com/golang/go/blob/2aae1000ea1d4b28923b96db28acf4e3b99d6aff/src/cmd/internal/test2json/test2json.go#L503-L531
[go-close]: https://github.com/golang/go/blob/2aae1000ea1d4b28923b96db28acf4e3b99d6aff/src/cmd/internal/test2json/test2json.go#L404-L422
[prior-g3]: https://github.com/theRebelliousNerd/codenerd/blob/45e9161503532cd959e3ccc242e1694fbdfc297c/Docs/audits/errorfree-go-test-package-failure-census.md
[prior-verdict]: https://github.com/theRebelliousNerd/codenerd/blob/45e9161503532cd959e3ccc242e1694fbdfc297c/Docs/audits/test-verdict-diagnostic-coverage.md
[prior-identity]: https://github.com/theRebelliousNerd/codenerd/blob/45e9161503532cd959e3ccc242e1694fbdfc297c/Docs/audits/recurse-diagnostic-subject-identity.md
[prior-invocation]: https://github.com/theRebelliousNerd/codenerd/blob/45e9161503532cd959e3ccc242e1694fbdfc297c/Docs/audits/recurse-collapsed-finding-invocation-identity.md
[prior-no-tests]: https://github.com/theRebelliousNerd/codenerd/blob/45e9161503532cd959e3ccc242e1694fbdfc297c/Docs/audits/recurse-no-tests-evidence-continuity.md
[prior-census]: https://github.com/theRebelliousNerd/codenerd/blob/45e9161503532cd959e3ccc242e1694fbdfc297c/Docs/audits/recurse-finding-census-completeness.md
[prior-scope]: https://github.com/theRebelliousNerd/codenerd/blob/45e9161503532cd959e3ccc242e1694fbdfc297c/Docs/audits/recurse-mission-acceptance-scope.md
[prior-rename]: https://github.com/theRebelliousNerd/codenerd/blob/45e9161503532cd959e3ccc242e1694fbdfc297c/Docs/audits/recurse-rename-effect-census.md
