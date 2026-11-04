---
doc-class: deep-dive
subsystem: campaign-recurse-gate-operands
implementation-status: proposed-refinement
evidence-status: source-only
last-verified: 2026-11-04
verified-against: 02c6a9b6978b0d67fe836674496c2b9f838d5c49
verification-method: static-source-inspection
main-rechecked: 2026-11-04
supersedes: []
---

# Preserve literal node paths in recurse gate operands

## Finding and decision

**A discovered Python directory named `--help` becomes a request for help rather than a directory to verify. Both detected checks can exit zero without compiling that directory or running its tests, and recurse can nevertheless record affirmative testing evidence.** The detector constructs `python -m compileall -q {node}` and `python -m pytest -q {node}`. Expansion inserts the directory directly, with no option boundary or path prefix. In the inspected CPython and pytest implementations, the resulting final `--help` argument selects successful help behavior. CodeNERD keeps the original node identity, classifies the exits as passes, and treats pytest exit zero as a completed test without its known no-tests disposition. [Detection][detect] [Expansion][expand] [CPython parsing][py-compile] [Argparse help][py-help] [Pytest help][pytest-help] [Tested projection][tested]

The same expansion boundary independently strips meaningful filename whitespace: the admitted directory `pkg ` becomes the operand `pkg`. If that sibling exists, its tests can donate a passing result to the original node. These are two violations of one required contract: **an admitted node path must reach the checker as that literal path operand, not as another pathname or a control argument.** [Expansion][expand] [Measurement attribution][measure]

Correct the built-in language adapter and define the shared placeholder contract explicitly. Preserve host execution, project-declared commands, exact permissions, source identity, known no-tests handling and Mangle's decision authority. Do not solve this by asking the model to notice help output, renaming user directories, weakening tests or replacing focused verification with an indiscriminate workspace run.

**Evidence boundary:** public main was pinned to [02c6a9b6978b0d67fe836674496c2b9f838d5c49][pin]. Root/scoped guidance, campaign architecture, the recurse journey, the 385-file audit inventory, related reports, implementation and relevant authored fixtures were inspected. Retained CodeNERD bytes used here were checked against that pin's Git blob identities. External source analysis uses CPython `v3.13.0`, resolved to [60403a5409ff2c3f3b07dd2ca91a7a3e096839c7][py-pin], and pytest `8.3.3`, resolved to [d0f136fe64f9374f18a04562305b178fb380d1ec][pytest-pin]. These establish a bounded supported-tool example, not the user's installed versions. No application, repository program, interpreter fixture, test, build, gate, model inference, training or benchmark was run. All behavioral traces and evaluations below are source deductions or proposed checks, not captured outcomes. No implementation, credential, setting or permission was changed.

## Architectural role and current status

The project assigns creativity to models and executive control to the deterministic host/kernel. Campaign guidance requires actual host testing against the relevant workspace; session guidance distinguishes changed artifacts, passed checks and verified behavior. The recurse journey intends measured improvements across arbitrary supported workspaces. A successful process that answered a help request cannot establish the intended source verification obligation merely because its argv was originally derived from a directory. [Root guidance][root] [Campaign guidance][campaign-guidance] [Session guidance][session-guidance] [Journey][journey]

Both CLI and chat use `RunRecurseCycles` and normal attempt campaigns. Improvements have no single declared acceptance command: the outer driver measures their gates and metrics after the executor returns. This makes the outer observation contract necessary even when inner review and testing are also present. [CLI][cli] [Chat][chat] [Attempt contract][attempt-contract]

| Boundary | Status at the pin | Relevant limitation |
|---|---|---|
| Workspace membership and Python node discovery | Implemented | A directory containing admitted Python files becomes a node; it need not be an importable Python package identifier. [Membership][membership] [Python scanner][python-scan] |
| Separate node IDs and source `Paths` | Implemented | The witness uses one ordinary physical node, no SCC collision and no synthetic-stage name. [Node shape][node] [Component projection][components] |
| Node placeholder expansion | Implemented, incomplete fidelity | It trims the path and inserts `{node}` without a checker-specific literal-operand contract. [Expansion][expand] |
| Fresh subprocess execution and process outcomes | Implemented | Actual help execution is observed accurately; its meaning is incorrectly sufficient for verification evidence. [Runner][runner] |
| Known pytest empty collection | Implemented | Exit 5 remains a pass without affirmative test evidence. Help returns a different exit, zero. [No-tests contract][no-tests] [Outer no-tests consumption][no-tests-consumer] |
| Current source metrics and Mangle ratchet | Implemented | Metrics read the original source path while the tested fact comes from the misbound invocation. [Metrics][metrics] [Policy bridge][bridge] [Ratchet][ratchet] |
| Literal operand admission and qualified execution evidence | Proposed | No correction or passing evaluation is claimed here. |

The defect is before the policy decision. Mangle receives a host assertion that testing occurred; it is not given a distinct help invocation that it could refuse. More deliberation, another critique model or a longer prompt would not restore the missing host observation.

## How the directory reaches option parsing

### Discovery preserves a legitimate filesystem name

Use a filesystem that supports the illustrated directory names and a clean Git workspace. Membership obtains NUL-delimited file names and preserves their path components. Its admission checks reject escapes, absolute inputs and NUL, and enforce membership/explicit exclusions; they do not reject leading dashes. The workspace collector accepts Python files, and `scanPythonImports` creates one node for each containing directory before attempting import resolution. `relDir` and component projection retain `--help` as the node ID and its source path. No Python import named `--help` is required. [Git membership][git-membership] [Path admission][path-admission] [Collector][collect] [Python scanner][python-scan] [Component projection][components]

Graph validation rejects empty/duplicate IDs and invalid dependency endpoints, not this distinct node. The witness requires neither a focused CLI selector nor a user typing a command-line option: an ordinary unfiltered sweep discovers the directory from the repository. The synthetic `wiring`, `review` and `bench` nodes do not collide with it. [Graph validation][graph-validation]

### The host keeps identity but changes argument role

For a node-scoped gate, `measure` iterates the node's original `Paths` and passes each to `gates.Run`. `Result.Node` retains that input. `Command` obtains argv through `ForNode`, which expands `{node}` to the raw relative path after `strings.TrimSpace`. `{pkg}` has a separate `./` prefix convention; the Python detector uses `{node}`. [Measurement][measure] [Runner and command][runner] [Expansion][expand]

For the primary node the resulting argument vectors are:

```text
python3, -m, compileall, -q, --help
python3, -m, pytest,     -q, --help
```

`python` is used instead when that is the selected interpreter. The detector's interpreter choice does not change the final operand problem. There is no shell interpolation in this witness. The non-Go workspace command builder uses `exec.CommandContext` with separate argv elements, sets the workspace as CWD and keeps its allowlisted environment. Those are useful protections; the invoked program still parses its own options. Adding shell quote characters to an argv element would change the bytes, not establish a literal operand. [Interpreter detection][detect] [Workspace command][workspace-command]

The general distinction between command construction and unintended option interpretation is documented by MITRE's [argument-injection taxonomy][cwe88]. The concrete claim here is verification misbinding, not arbitrary command execution, exfiltration, or a demonstrated malicious repository attack.

## The external tool contracts

### CPython help exits before compilation

The pinned `compileall.main` constructs a normal `ArgumentParser`, declares `-q` and positional compilation destinations, then calls `parse_args` before its compilation dispatch. It does not disable default help. The pinned argparse constructor adds `-h` and `--help`; its help action prints help and calls `exit`, whose default status is zero. Thus `-q --help` exits during parsing rather than treating the latter string as `compile_dest`. [Compileall parser][py-compile] [Default help registration][py-help-registration] [Help action][py-help] [Default exit][py-exit]

This conclusion does not depend on old bytecode, timestamps, compiler caching, shell syntax or source contents. It also does not say that starting the interpreter executes no code: module startup and option parsing still occur. The absent operation is compilation of the selected directory.

### Pytest help is successful without a normal test session

Pytest documents `--help` as its help/configuration interface. In the pinned source, `helpconfig` registers it and its command-line main hook calls configuration/help/cleanup and returns zero. The CLI returns that code through `console_main` and the `python -m pytest` entry point. This is separate from normal test collection/execution. The witness assumes ordinary healthy plugin configuration, with no plugin overriding this behavior or raising during initialization. [Pytest invocation guide][pytest-guide] [Help registration and result][pytest-help] [CLI return][pytest-cli] [Module entry][pytest-entry]

Help can load/configure plugins. It is therefore neither an assertion that the process is side-effect-free nor a substitute for the ordinary command's permissions and cancellation. Configuration failure can produce a failure instead of the proposed successful help result. That is a useful failure control, not a guarantee that every help invocation becomes test evidence.

The exit-code distinction matters. `Gate.NoTestsExitCodes` recognizes detected pytest's exit 5. It does not recognize every way a command can return zero without tests. `FindingsAll` returns immediately for a passing result, so the retained help output contributes no repair finding. `gateRun.noTests` consults those exit markers, not the help text. The after-measurement loop consequently sets `Tested` for the completed help invocation. [Gate result semantics][no-tests] [Passing findings][findings] [No-tests consumer][no-tests-consumer] [Tested projection][tested]

## Conditional ordinary recurse witness

Use a fresh, clean, admitted POSIX workspace with the following three relevant files tracked in its initial HEAD, and no overriding `nerd.md` commands, exclusion rules, root Python sources or independent broad gate. Python and the pinned pytest profile are available; initialization, kernel, ownership, Git and cancellation conditions are healthy.

```text
pyproject.toml
--help/module.py
--help/test_module.py
```

The project marker can contain an empty `[tool.pytest.ini_options]` table. The directory name is literal, not an instruction supplied to the model. Initially `module.py` has three nonblank source lines:

```python
def answer():
    value = 42
    return value
```

An unchanged test can load the neighboring source file by path, avoiding any requirement that the directory be a Python identifier:

```python
from pathlib import Path
import runpy

def test_answer():
    module = runpy.run_path(str(Path(__file__).with_name("module.py")))
    assert module["answer"]() == 42
```

Consider a simplify attempt whose retained production change is:

```python
def answer():
    return 0
```

That is a bad behavioral change; a compliant worker should not propose it and an effective reviewer should reject it. It is a diagnostic counterexample for the host boundary, not permission to make the edit. Assume it survives those independent controls and returns through the existing retained-write channel. The outer source trace is then:

| Stage | Host observation | Missing association |
|---|---|---|
| Node discovery | Physical node and path `--help` | No naming rejection or collision occurs. |
| Baseline compile/test gates | Both help commands complete with exit zero | Neither checks the selected source or executes its test. |
| Baseline metrics | Three production lines and one test definition | These come from real source bytes, not help output. |
| Candidate | Only production source changes, three lines to two | Existing test, runner configuration and node name remain unchanged. |
| After compile/test gates | The same help commands again exit zero | Fresh processes repeat the same wrong operation. |
| After metrics and facts | Fewer lines, unchanged test count, `Tested=true`, no observed gate regression | Test activity has been inferred from successful help. |
| Ratchet and adoption | With all other guards satisfied, the improvement success/keep premises hold | The outer boundary cannot reject the bad change on this test obligation. |

[Measurements][measure] [After-measurement][after] [Metric reader][metrics] [Metric acquisition][metric-bridge] [Tested projection][tested] [Fact bridge][bridge] [Keep rule][ratchet] [Adoption][adopt]

An explicitly literal invocation against the same unchanged test would evaluate the changed function and fail the assertion. That expectation follows from the fixture's code; it has not been executed. There is no test-count inflation, test deletion, missing dependency edge, stale interpreter cache, receipt reuse, diagnostic truncation or changed exit-code contract in this example.

The unconditional static result is narrower than end-to-end adoption: the built-in invocation selects help and its completed zero exit satisfies the host's current affirmative-test projection. Actual retained bad work still requires the executor, review, permission, metric, forbidden-path and publication conditions above. No observed production frequency or model failure probability is claimed.

## The inner session is not an independent operand check

The ordinary session workspace-test route obtains written non-Go directories using their path dirname, detects the same gates, builds each node check through `ForNode`, and executes it through `gates.Command`/`gates.Run`. The original directory remains in the obligation ID and `Node`; the argv can already mean help. `classifyRun` returns `/passed` for a successful exit not marked `NoTests`, and `noteObligation` publishes that outcome. Thus merely adding the existing session workspace test plan does not independently establish literal selection. [Written-directory derivation][written-dirs] [Plan construction][session-plan] [Planned execution][session-run] [Classification][session-classify] [Outcome publication][session-publish]

Exact receipt matching is still valuable and must remain: it compares argv, working directory, environment digest and after-write ordering. But an exact match to an already-misconstructed invocation proves that the same operation occurred, not that the operation represented the intended path. The primary witness does not need reuse at all. [Receipt matching][session-receipt]

Other independent tests, source review, typed edit guards or owner-declared workspace checks can reject the candidate. This report does not claim that every task completes or that no tests run anywhere in the system. It identifies a shared host producer whose consumers should not be counted as independent defenses against that producer's own operand error.

## Whitespace gives a second operand mismatch

On a supporting filesystem, admit both literal directories `pkg ` and `pkg`, with tracked Python files and no exclusions. The former ends in one ASCII space. The scanner, node paths and source metrics retain it; `ForNode("pkg ")` becomes `pkg` before either `{node}` or `{pkg}` expansion. Both detected Python checks therefore operate on the sibling. `Result.Node`, the recurse state key and node-scoped finding attribution still refer to the original `pkg ` node. [Discovery][collect] [Python scanner][python-scan] [Expansion][expand] [Measurement][measure] [Metric reader][metrics]

If the sibling has a real passing test, the exit-5 safeguard is irrelevant: a test did execute, but for the wrong subject. If the sibling does not exist or fails, the result can instead become a misleading failure attributed to the original node. Ordinary internal spaces, as in `pkg demo`, are not removed by this helper and are an essential control. Leading/trailing whitespace and nested components need their own exact-byte cases rather than an assertion that all whitespace is broken.

This secondary witness is POSIX-scoped; it does not assume that Windows preserves trailing-space directory names through ordinary path APIs. The primary `--help` witness needs no trailing whitespace. Filesystem normalization, symlink identity and platform-specific naming restrictions remain separate requirements.

## Proposed contract

### Preserve identity before adapting syntax

Keep the admitted workspace-relative path as data. Do not trim it to validate nonemptiness or silently reinterpret an all-whitespace directory as the root. Separate an omitted-node sentinel from an existing literal path. Preserve membership, canonical workspace identity and containment checks; fidelity is not permission to address a different resource.

For built-in Python checks, use a tool-specific literal path representation and demonstrate that the intended file/directory is selected. A `./` prefix for a relative path or an explicitly placed end-of-options marker can prevent the demonstrated leading-dash interpretation where the tool supports it. Neither alone fixes `TrimSpace`; both identity and operand role need protection. Python documents the end-of-options convention, but support and placement must be qualified for the actual invoked tools. [Argparse interface][argparse-doc]

Do not blindly prepend `--` to every gate. Owner templates can contain a placeholder inside an option value, multiple operands, subsequent flags, package selectors, wrapper programs or a different command grammar. `{node}` and `{pkg}` already have distinct conventions. Specify whether raw placeholders remain compatible, whether an explicit path-operand form is added, and how supported adapters supply it. Preserve owner intent and decline unsupported representations visibly rather than quietly modifying command meaning.

Likewise, `./` is not a universal escaping scheme for every checker language. Pytest supports `@` argument files and `::` collection selectors. Qualification should include such syntax-bearing names and either prove literal selection or return an explicit unsupported/unverified result. This report establishes neither a universal encoding nor a new exploit for each of those characters. [Pytest selector grammar][pytest-guide]

### Preserve evidence meaning through consumers

Keep process success distinct from an admitted verification invocation and the testing observation it establishes. For the built-in profile, a generated help/configuration-only operation must not satisfy the node's test obligation or produce `recurse_ratchet_tested`. Preventing the wrong operation at construction is the narrow correction; stronger structured runner evidence can independently confirm selected/executed tests but needs its own versioned integration and costs.

Do not infer failure merely from English help-looking output, search for a magic phrase, change all zero exits into unknown, or special-case only a directory named `--help`. Output can be localized or plugin-modified, and real tests can print help. Preserve genuine empty collection as its current distinct disposition, ordinary failure, unavailable executable and cancellation. Let the host expose exact subject/invocation/outcome facts and let Mangle decide which evidence satisfies the obligation.

Because session planning and recurse share expansion, repair and qualify both consumers. Preserve invocation receipt identity, current-source requirements, scoped permissions, known no-tests handling, collapsed-node per-invocation records and recovery ownership. Previously recorded help or sibling observations must not be reused as evidence for a newly corrected invocation. A changed argv already prevents matching the old exact receipt; any durable/cached classification migration should be explicit rather than assumed.

## Costs and failure handling

The minimum path-adaptation repair needs no model request, additional subprocess, dependency scan, training or workspace-wide test. Its direct work is string validation/encoding proportional to operand bytes, plus carrying a faithful subject representation. It may make the actual observed workload much larger than before because a command that previously returned help now compiles and runs tests. That increase is restored verification, not a performance regression to conceal.

Measure interpreter startup, compilation, test collection/execution, output capture, receipt bytes, allocations and any extra remeasurement separately. Existing execution budgets and cancellation still apply. A previously instantaneous help result can become a long-running test, a real failure, unavailable dependency or timeout. Those states must flow to existing refusal/recovery behavior; no compensating pass is justified because the old behavior was cheap.

Structured test reporting would add parser/protocol maintenance, event volume, capture/storage cost, possible plugin loading and compatibility work. Compare it against the narrow operand repair rather than assuming it is free or automatically needed for the first correction. Do not require installing a plugin or changing the user's environment silently.

Replacing focused checks with whole-workspace checks is a separate, higher-cost design: it can repeatedly execute unrelated tests and expose additional side effects without fixing generic placeholder fidelity. Rejecting every unusual filename is cheap but unnecessarily restricts supported repositories. Prefer faithful selection where supported and an explicit, bounded unsupported disposition otherwise.

No token, latency, reliability or quality gain is measured here. Any later claim must include newly executed tests, real failures, retries and additional worker turns, not only a smaller argument-construction cost or avoided misleading repairs.

## Fair unrun evaluation

Compare the current and corrected implementations using the same source fixtures, interpreter/pytest versions, plugin configuration, environment, permissions and budgets. Separate these layers:

1. **Exact transport:** a controlled argv observer establishes the actual bytes received. This can expose trimming and boundary construction, but it does not prove how a real checker interprets them.
2. **Real tool semantics:** run actual compileall and pytest against literal directories. Verify selected files, executed test identity and test outcome, including a deliberately failing assertion. A fake exit-zero executable cannot establish this contract.
3. **Production composition:** use the shared CLI/chat recurse driver and normal session test-plan path with a scripted executor, then inspect measurements, policy facts, retained/reverted writes and the journal. Keep model calls disabled for deterministic qualification.

Proposed cases, all unrun:

| Case | Required evidence |
|---|---|
| Primary `--help` directory, bad simplify change and unchanged test | Actual node test executes and fails; help exit cannot donate affirmative evidence; candidate is not kept on that basis. |
| Same directory, legitimate line reduction preserving `answer()==42` | Correct path is compiled/tested; valid improvement can still pass ordinary policy. |
| Same path with no collected tests | Preserve the no-tests disposition; no affirmative testing fact from it. |
| `pkg ` plus passing `pkg` sibling | No sibling substitution; exact original directory is verified. |
| Whitespace path whose trimmed sibling is absent or failing | No misleading result attributed from another/missing target. |
| Ordinary name, internal spaces, nested `parent/--help`, root `.` and SCC members | Preserve supported normal behavior and per-invocation attribution. |
| Other option-like names and checker-specific `@`/`::` syntax | Demonstrate literal selection or explicit unsupported handling; no inferred universal escaping. |
| Owner-declared wrapper, embedded placeholder and flags after the operand | Preserve the declared command's semantics and explain unsupported forms rather than silently rewriting it. |
| Session plan with and without an exact earlier receipt | Correct subject/invocation identity in both fresh execution and valid reuse. |
| Plugin/configuration failure, missing runner, cancellation and large output | Existing fail/unavailable/indeterminate/truncation behavior remains visible. |
| Ignored, forbidden, outside-workspace and platform-unsupported names | Existing admission limits remain intact; path preservation cannot widen authority. |
| Restart or corrected invocation after an older receipt | No transfer of old help/sibling evidence to the corrected obligation. |

Existing authored tests assert ordinary Python argv, zero/five/one exit semantics, no-tests projection, process error handling and simplify metric behavior. They are useful controls. The inspected tests do not exercise a discovered leading-dash directory through real Python option parsing and the outer ratchet. The no-tests fixture's zero-exit assertion is appropriate only within an admitted test-running invocation, which is precisely the missing premise here. No test was rerun. [Detector fixture][detect-test] [No-tests fixture][no-tests-test] [Runner fixtures][run-tests] [Outer no-tests fixture][outer-test] [Simplify fixture][simplify-test]

**Checkable exit:** every supported admitted node path reaches the intended checker as that exact subject; names cannot change an intended verification into help/configuration behavior; a different sibling's success cannot satisfy the node; the demonstrated bad candidate cannot be kept using either false observation; valid ordinary and unusual-name projects, owner commands, cancellation, source stability, scope and existing no-tests behavior remain intact. Mark implementation and execution evidence separately when those checks actually run.

## Overlap and priority

The current recursive audit inventory, queue and nearby reports were screened. GitHub searches under `Docs/audits` found no `--help` or `option injection` report; `ForNode` matched four related documents, and leading-dash results concerned Git adoption/census. Whitespace searches led to unrelated formatting/protocol reports. Search behavior is a discovery aid, not exhaustive proof of novelty across historical branches or private discussion.

- **Python bytecode source fidelity** concerns running the selected module from old bytecode. Here no selected test runs, and a fresh cache changes nothing. The proposed cache correction does not delimit a node operand. [Prior bytecode audit][prior-bytecode]
- **Known no-tests continuity** records the repaired exit-5 consumer. Preserve it. Here zero is successful help; in the whitespace control zero can be a genuine test of a different directory. [Prior no-tests audit][prior-no-tests]
- **Windows batch argument fidelity** concerns later command-shell transformation. The primary witness uses direct POSIX argv and the called program's normal parser. No batch wrapper or environment-variable expansion is needed. [Prior batch audit][prior-batch]
- **Git literal path adoption** distinguishes NUL framing from pathspec interpretation after a keep. This is an earlier source-to-checker operand boundary. Its useful application-grammar principle carries over, but fixing Git selection does not execute the intended Python tests. [Prior Git audit][prior-git]
- **Node namespace identity**, **gate language domain** and **collapsed finding invocation identity** concern graph collisions, checker applicability and attribution among completed invocations. Here the graph is distinct, the language is correct and the retained invocation is simply the wrong operation. [Node audit][prior-node] [Language audit][prior-language] [Collapsed audit][prior-collapsed]
- **TypeScript spaced diagnostic paths** concerns decoding an already emitted diagnostic. The whitespace issue here changes the executed operand before a diagnostic exists. [Prior diagnostic audit][prior-diagnostic]

Prioritize this as a bounded host correctness requirement for portable recurse, not a new autonomous subsystem. The principal correction is small in mechanism but must be validated through real tool parsing and both existing consumers. Neither the rarity of a directory spelling nor a successful process exit establishes the test evidence the architecture requires.

## References

[pin]: https://github.com/theRebelliousNerd/codenerd/commit/02c6a9b6978b0d67fe836674496c2b9f838d5c49
[root]: https://github.com/theRebelliousNerd/codenerd/blob/02c6a9b6978b0d67fe836674496c2b9f838d5c49/agents.md#L1-L68
[campaign-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/02c6a9b6978b0d67fe836674496c2b9f838d5c49/internal/campaign/agents.md#L1-L15
[session-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/02c6a9b6978b0d67fe836674496c2b9f838d5c49/internal/session/agents.md
[journey]: https://github.com/theRebelliousNerd/codenerd/blob/02c6a9b6978b0d67fe836674496c2b9f838d5c49/Docs/journeys/10-forever-loop.md#L37-L128
[cli]: https://github.com/theRebelliousNerd/codenerd/blob/02c6a9b6978b0d67fe836674496c2b9f838d5c49/cmd/nerd/cmd_campaign_recurse.go#L204-L280
[chat]: https://github.com/theRebelliousNerd/codenerd/blob/02c6a9b6978b0d67fe836674496c2b9f838d5c49/cmd/nerd/chat/campaign_recurse.go#L218-L276
[attempt-contract]: https://github.com/theRebelliousNerd/codenerd/blob/02c6a9b6978b0d67fe836674496c2b9f838d5c49/internal/campaign/recurse_attempt.go#L34-L74
[detect]: https://github.com/theRebelliousNerd/codenerd/blob/02c6a9b6978b0d67fe836674496c2b9f838d5c49/internal/gates/detect.go#L119-L178
[expand]: https://github.com/theRebelliousNerd/codenerd/blob/02c6a9b6978b0d67fe836674496c2b9f838d5c49/internal/gates/gates.go#L129-L147
[runner]: https://github.com/theRebelliousNerd/codenerd/blob/02c6a9b6978b0d67fe836674496c2b9f838d5c49/internal/gates/run.go#L44-L101
[workspace-command]: https://github.com/theRebelliousNerd/codenerd/blob/02c6a9b6978b0d67fe836674496c2b9f838d5c49/internal/build/workspace_command.go#L32-L65
[no-tests]: https://github.com/theRebelliousNerd/codenerd/blob/02c6a9b6978b0d67fe836674496c2b9f838d5c49/internal/gates/gates.go#L75-L114
[no-tests-consumer]: https://github.com/theRebelliousNerd/codenerd/blob/02c6a9b6978b0d67fe836674496c2b9f838d5c49/internal/campaign/recurse_cycle.go#L232-L265
[findings]: https://github.com/theRebelliousNerd/codenerd/blob/02c6a9b6978b0d67fe836674496c2b9f838d5c49/internal/gates/findings.go#L112-L120
[node]: https://github.com/theRebelliousNerd/codenerd/blob/02c6a9b6978b0d67fe836674496c2b9f838d5c49/internal/campaign/recurse_dag.go#L16-L33
[components]: https://github.com/theRebelliousNerd/codenerd/blob/02c6a9b6978b0d67fe836674496c2b9f838d5c49/internal/campaign/recurse_workspace.go#L197-L234
[collect]: https://github.com/theRebelliousNerd/codenerd/blob/02c6a9b6978b0d67fe836674496c2b9f838d5c49/internal/campaign/recurse_workspace.go#L264-L350
[python-scan]: https://github.com/theRebelliousNerd/codenerd/blob/02c6a9b6978b0d67fe836674496c2b9f838d5c49/internal/campaign/recurse_workspace.go#L463-L483
[graph-validation]: https://github.com/theRebelliousNerd/codenerd/blob/02c6a9b6978b0d67fe836674496c2b9f838d5c49/internal/campaign/recurse_plan.go#L82-L109
[git-membership]: https://github.com/theRebelliousNerd/codenerd/blob/02c6a9b6978b0d67fe836674496c2b9f838d5c49/internal/workspace/git.go#L99-L246
[membership]: https://github.com/theRebelliousNerd/codenerd/blob/02c6a9b6978b0d67fe836674496c2b9f838d5c49/internal/workspace/membership.go#L299-L448
[path-admission]: https://github.com/theRebelliousNerd/codenerd/blob/02c6a9b6978b0d67fe836674496c2b9f838d5c49/internal/workspace/membership.go#L632-L660
[measure]: https://github.com/theRebelliousNerd/codenerd/blob/02c6a9b6978b0d67fe836674496c2b9f838d5c49/internal/campaign/recurse_cycle.go#L754-L839
[after]: https://github.com/theRebelliousNerd/codenerd/blob/02c6a9b6978b0d67fe836674496c2b9f838d5c49/internal/campaign/recurse_cycle.go#L1324-L1356
[metrics]: https://github.com/theRebelliousNerd/codenerd/blob/02c6a9b6978b0d67fe836674496c2b9f838d5c49/internal/gates/metrics.go#L37-L179
[metric-bridge]: https://github.com/theRebelliousNerd/codenerd/blob/02c6a9b6978b0d67fe836674496c2b9f838d5c49/internal/campaign/recurse_cycle.go#L1575-L1606
[tested]: https://github.com/theRebelliousNerd/codenerd/blob/02c6a9b6978b0d67fe836674496c2b9f838d5c49/internal/campaign/recurse_cycle.go#L1421-L1445
[bridge]: https://github.com/theRebelliousNerd/codenerd/blob/02c6a9b6978b0d67fe836674496c2b9f838d5c49/internal/campaign/recurse_policy.go#L217-L247
[ratchet]: https://github.com/theRebelliousNerd/codenerd/blob/02c6a9b6978b0d67fe836674496c2b9f838d5c49/internal/core/defaults/policy/recurse.mg#L299-L365
[adopt]: https://github.com/theRebelliousNerd/codenerd/blob/02c6a9b6978b0d67fe836674496c2b9f838d5c49/internal/campaign/recurse_cycle.go#L1481-L1510
[written-dirs]: https://github.com/theRebelliousNerd/codenerd/blob/02c6a9b6978b0d67fe836674496c2b9f838d5c49/internal/session/workspace_test_run.go#L178-L203
[session-plan]: https://github.com/theRebelliousNerd/codenerd/blob/02c6a9b6978b0d67fe836674496c2b9f838d5c49/internal/session/workspace_test_plan.go#L73-L112
[session-run]: https://github.com/theRebelliousNerd/codenerd/blob/02c6a9b6978b0d67fe836674496c2b9f838d5c49/internal/session/workspace_test_run.go#L113-L140
[session-classify]: https://github.com/theRebelliousNerd/codenerd/blob/02c6a9b6978b0d67fe836674496c2b9f838d5c49/internal/session/workspace_test_plan.go#L132-L156
[session-publish]: https://github.com/theRebelliousNerd/codenerd/blob/02c6a9b6978b0d67fe836674496c2b9f838d5c49/internal/session/workspace_test_run.go#L143-L159
[session-receipt]: https://github.com/theRebelliousNerd/codenerd/blob/02c6a9b6978b0d67fe836674496c2b9f838d5c49/internal/session/workspace_test_plan.go#L159-L183
[detect-test]: https://github.com/theRebelliousNerd/codenerd/blob/02c6a9b6978b0d67fe836674496c2b9f838d5c49/internal/gates/detect_test.go#L92-L105
[no-tests-test]: https://github.com/theRebelliousNerd/codenerd/blob/02c6a9b6978b0d67fe836674496c2b9f838d5c49/internal/gates/no_tests_test.go#L5-L30
[run-tests]: https://github.com/theRebelliousNerd/codenerd/blob/02c6a9b6978b0d67fe836674496c2b9f838d5c49/internal/gates/run_test.go#L23-L83
[outer-test]: https://github.com/theRebelliousNerd/codenerd/blob/02c6a9b6978b0d67fe836674496c2b9f838d5c49/internal/campaign/recurse_audits_test.go#L264-L306
[simplify-test]: https://github.com/theRebelliousNerd/codenerd/blob/02c6a9b6978b0d67fe836674496c2b9f838d5c49/internal/campaign/recurse_cycle_test.go#L467-L511
[prior-bytecode]: https://github.com/theRebelliousNerd/codenerd/blob/02c6a9b6978b0d67fe836674496c2b9f838d5c49/Docs/audits/recurse-python-bytecode-source-fidelity.md
[prior-no-tests]: https://github.com/theRebelliousNerd/codenerd/blob/02c6a9b6978b0d67fe836674496c2b9f838d5c49/Docs/audits/recurse-no-tests-evidence-continuity.md
[prior-batch]: https://github.com/theRebelliousNerd/codenerd/blob/02c6a9b6978b0d67fe836674496c2b9f838d5c49/Docs/audits/recurse-windows-batch-argument-fidelity.md
[prior-git]: https://github.com/theRebelliousNerd/codenerd/blob/02c6a9b6978b0d67fe836674496c2b9f838d5c49/Docs/audits/recurse-git-literal-path-adoption.md
[prior-node]: https://github.com/theRebelliousNerd/codenerd/blob/02c6a9b6978b0d67fe836674496c2b9f838d5c49/Docs/audits/recurse-node-namespace-identity.md
[prior-language]: https://github.com/theRebelliousNerd/codenerd/blob/02c6a9b6978b0d67fe836674496c2b9f838d5c49/Docs/audits/recurse-gate-language-domain.md
[prior-collapsed]: https://github.com/theRebelliousNerd/codenerd/blob/02c6a9b6978b0d67fe836674496c2b9f838d5c49/Docs/audits/recurse-collapsed-finding-invocation-identity.md
[prior-diagnostic]: https://github.com/theRebelliousNerd/codenerd/blob/02c6a9b6978b0d67fe836674496c2b9f838d5c49/Docs/audits/recurse-typescript-spaced-diagnostic-paths.md
[py-pin]: https://github.com/python/cpython/commit/60403a5409ff2c3f3b07dd2ca91a7a3e096839c7
[py-compile]: https://github.com/python/cpython/blob/60403a5409ff2c3f3b07dd2ca91a7a3e096839c7/Lib/compileall.py#L315-L387
[py-help-registration]: https://github.com/python/cpython/blob/60403a5409ff2c3f3b07dd2ca91a7a3e096839c7/Lib/argparse.py#L1750-L1801
[py-help]: https://github.com/python/cpython/blob/60403a5409ff2c3f3b07dd2ca91a7a3e096839c7/Lib/argparse.py#L1099-L1117
[py-exit]: https://github.com/python/cpython/blob/60403a5409ff2c3f3b07dd2ca91a7a3e096839c7/Lib/argparse.py#L2649-L2652
[argparse-doc]: https://docs.python.org/3.13/library/argparse.html#arguments-containing
[pytest-pin]: https://github.com/pytest-dev/pytest/commit/d0f136fe64f9374f18a04562305b178fb380d1ec
[pytest-help]: https://github.com/pytest-dev/pytest/blob/d0f136fe64f9374f18a04562305b178fb380d1ec/src/_pytest/helpconfig.py#L48-L158
[pytest-cli]: https://github.com/pytest-dev/pytest/blob/d0f136fe64f9374f18a04562305b178fb380d1ec/src/_pytest/config/__init__.py#L139-L203
[pytest-entry]: https://github.com/pytest-dev/pytest/blob/d0f136fe64f9374f18a04562305b178fb380d1ec/src/pytest/__main__.py#L1-L9
[pytest-guide]: https://docs.pytest.org/en/8.3.x/how-to/usage.html
[cwe88]: https://cwe.mitre.org/data/definitions/88.html
