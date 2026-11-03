# Recurse gate executable selection under declared environments

**Status: conditional source finding and bounded design proposal.** Inspected public main at [`7460225f77d1b14d43f61f4c4391a4cf62e573c1`][pin] on November 3, 2026. No repository implementation, build, test, gate, application, model inference, training or benchmark was run. The examples below are source-derived witnesses, not reproduced incidents. Only public repository material and public primary documentation informed this report.

## Finding and recommendation

A workspace can declare the environment its checks require, including `PATH`, but **the executable is selected from codeNERD's ambient process environment before that declared environment is applied**.

Two consumers expose the same split:

1. Gate admission asks ambient `exec.LookPath` whether a bare program exists, without consulting the gate's `Env`.
2. Gate execution calls `exec.CommandContext` before constructing and overlaying the effective environment. On the POSIX witness below, that has already selected the ambient executable.

Consequently a project-local test tool can be reported unavailable even though its declared search path contains it. If an ambient tool has the same name, the gate can instead execute that tool while handing it the project's declared `PATH`. This is a portability and verification-configuration boundary, not just a missing display field. The primary witness uses two distinct fixed-behavior native checkers so that interpreter startup, wrappers and virtual-environment discovery do not obscure which program actually ran.

**Recommended correction:** resolve a command's executable under the same admitted environment and working-directory contract used for execution, and carry that resolution through admission, actual dispatch and evidence. Retain existing declaration precedence, environment filtering, process controls, task admission and Mangle verdict ownership. Do not activate environments by changing the parent process, install tools automatically, or substitute a shell.

## Scope and relation to existing work

The current source already preserves important parts of the gate contract. Declared command environment reaches `Gate.Env`; an ordinary recurse fix copies it into `RecurseAttempt.CheckEnv`; the attempt campaign stores a private runtime contract; campaign acceptance checks contract consistency and includes the environment in the tactile command. The earlier [gate contract continuity audit][prior-contract] owns the missing-environment and missing-exit-code handoff. Those source repairs are present and should be credited, not reopened. [Declaration][detect] [Attempt handoff][attempt-handoff] [Campaign construction][attempt-campaign] [Runtime contract][runtime-contract] [Acceptance dispatch][acceptance]

The present witness occurs **before or independently of that handoff**. An intact environment map does not undo executable selection performed earlier. Both the original gate and a direct-backed acceptance command can consistently select the wrong ambient executable. Fixing only the copied environment fields therefore does not settle this boundary. [Workspace builder][workspace-command] [Direct executor][direct]

The [workspace test receipt audit][prior-receipts] already proposes an execution-profile identity containing executable/toolchain selection and a shared resolver for the required and actual invocation. Those design ideas are prior work, not new contributions of this report. Its principal witness concerns reuse of evidence from a different invocation; the present witness executes a fresh command and needs no receipt reuse. This report contributes the narrower, still-present lookup-before-overlay mechanism and its producer-level witnesses. [Prior resolver and identity contract][prior-resolver]

The [portable recurse design proposal][prior-portable] already calls for explicit toolchain capabilities and honest limits. The present finding supplies a concrete remaining failure in its project-declared-command escape hatch. It does not propose a new agent framework or claim universal language support.

The audit ledger, campaign/build guidance, the two neighboring audits, and targeted public code searches for `LookPath`, executable selection and `commands.env` were checked. No exact prior report of this lookup-before-overlay mechanism was found. This is a scoped novelty check, not proof that no historical discussion exists.

## Intended contract and current production route

The project schema describes `commands` as canonical invocations and `Env` as prerequisites that must be set for those commands. It does not reserve `PATH` for ambient-only behavior. The build layer explicitly includes toolchain search and environment variables in its allowlist. A declared environment is therefore not merely advisory prompt text on this path. [Schema][schema] [Allowed variables][allowed]

Both CLI and chat recurse enter `RunRecurseCycles`. At each pass the loop derives its workspace graph, calls `gates.Detect`, records unavailable gates and measures applicable gates. `Detect` loads the project document, builds declared descriptors and suppresses detected gates of a kind for which the project declared a canonical command. It then partitions the combined set with `split`. [CLI entry][cli] [Chat entry][chat] [Pass admission][pass] [Detection precedence][detect]

For a nonempty, bare program name, `split` calls `lookPath(prog)`; the variable is `exec.LookPath`. The function receives neither workspace root nor an effective-environment resolver, and does not read `g.Env`. Workspace-relative scripts containing a separator take a separate path and remain runnable until execution; they are a useful unaffected control. [Availability partition][split]

Only `set.Gates`, not `set.Unavailable`, enters workspace measurement. The unavailable list is retained for reporting. If a node has neither an applicable node test gate nor an applicable runnable workspace test gate, its otherwise measurable improvement is skipped with an explicit reason. Thus this report does not claim that unavailable gates are silently counted as passes. The error is making the availability decision using a different executable-search contract from the one the project declared. [Unavailable reporting and measurement][measure-workspace] [Improvement prerequisite][improve]

For runnable gates, `gates.Command` expands node placeholders and delegates to `build.WorkspaceCommand`. The builder:

- prepares Go environment and flags when the original program is `go`, or normalizes an explicit workspace-relative program otherwise;
- calls `exec.CommandContext`;
- assigns the base environment and working directory;
- overlays sorted `commands.env` entries afterward.

No later path in `processutil.Run` reselects the executable from that overlay. Its role is noninteractive startup, bounded pipe waiting and process-tree cancellation. The outer gate route is `gates.Run` → `processutil.Run`, not tactile execution. Campaign acceptance is a separate route through the injected tactile executor. The inspected direct tactile backend has the same lookup-before-environment ordering, but this does not mean the outer baseline receives that backend's validation, auditing, sandbox selection or default timeout. Preserve the controls each route actually has rather than claiming those paths are already unified. [Gate command][gate-command] [Workspace builder][workspace-command] [Process handling][process] [Acceptance dispatch][acceptance] [Direct executor][direct]

Go documents that `Command` resolves a separator-free name with `LookPath`, while `Cmd.Env` supplies the child's environment; `CommandContext` builds on `Command`. The public implementation stores the lookup result in `Cmd.Path` during construction. This report uses that documented distinction to derive the POSIX witness, rather than assuming that setting `Env` later reruns search. [Go API][go-exec] [Go implementation][go-source]

## Witness one selects a different checker

Consider an otherwise admitted POSIX workspace at `/work/project`:

- codeNERD's ambient `PATH` locates `/opt/ambient-tools/projectcheck`;
- a separate prepared project tool directory contains `/work/project/tools/bin/projectcheck`;
- these are distinct native executables, not symlinks, interpreter scripts or wrappers; for the same arguments, the ambient fixture prints `ambient checker` and exits 1, while the project fixture prints `project checker` and exits 0;
- the fixtures have fixed behavior independent of `argv[0]`, environment, working directory and any source file;
- `nerd.md` declares `commands.test: projectcheck --suite regression` and an exact literal `commands.env.PATH` beginning `/work/project/tools/bin`;
- the declared path contains absolute directories, uses the host separator correctly, and includes any other prerequisites the project needs;
- the files, permissions and environment remain stable, and no process startup, cancellation, policy or unrelated gate failure intervenes.

There is no shell expansion in this example. The declaration supplies the actual path string, not `$PATH`, a tilde or an unsupported placeholder. No malicious program, credential, network access or concurrently replaced executable is needed. These fixed checkers isolate selection and exit interpretation; their output is not evidence that any real project's regression suite passed. They are a proposed deterministic witness, not executables created or run during this review.

The source-derived sequence is:

| Stage | Effective observation |
|---|---|
| Declaration | The test command and its `PATH` are both retained in the gate. |
| Availability | Ambient lookup finds `/opt/ambient-tools/projectcheck`, so the declared test gate is runnable. |
| Construction | `exec.CommandContext` resolves the bare `projectcheck` using the same ambient search path. |
| Overlay | The child environment receives the declared project-tool-first `PATH`, but the command's selected executable is already the ambient checker. |
| Execution | The ambient checker is invoked with `--suite regression`; its fixed output is `ambient checker` and its exit is 1. |
| Verdict | On that normal exit, `gates.Run` records a failed gate rather than an unverified startup. The checker selected by the declared path would instead have printed `project checker` and exited 0. |

The narrowly established outcome is **selection of the ambient checker despite the declared project search environment**, with an observable exit difference under the stated fixed-behavior fixture. A nonpassing exit can produce a fallback finding because finding extraction suppresses passes and unverified runs but otherwise retains at least one diagnostic. Whether a particular finding is attributed to a node, the model later attempts remediation, or a cycle is kept remains contingent on later controls. No end-to-end false keep or measured wasted-call count is asserted. [Exit classification][gate-run] [Finding fallback][findings]

Real project and ambient versions of a checker can differ in supported flags, plugins, rules or behavior. Their specific consequence requires a fixture with that tool; the fixed-checker witness establishes the host selection defect without assuming those differences. This matters to the portable design funnel because its behavioral evidence must identify the checker actually invoked, not merely the logical name in the declaration.

## Witness two rejects an available project checker

Use a declared test command `projectcheck --suite regression` with a correct, literal `commands.env.PATH` pointing to a prepared tool directory. Let `projectcheck` be executable there and absent from the ambient path.

The descriptor is valid, but `split` consults only ambient lookup and places the declared gate in `Unavailable`. Because canonical command precedence was already applied, a detected test command of the same kind is not reintroduced as a fallback. That precedence is correct and should remain: running a different test command would not repair the intended invocation. [Detection precedence][detect] [Availability][split]

The loop reports the unavailable gate. If it was the node's only applicable test, the improvement prerequisite prevents an improvement attempt even though the project supplied a runnable checker. No process execution or particular model response is necessary for this witness; admission has already made the wrong availability classification. [Measurement][measure-workspace] [Improvement guard][improve]

Changing only `split` to use the declared search path would be incomplete. The builder would still resolve `projectcheck` under ambient lookup before receiving that environment, producing a startup error unless it also adopts the effective resolver. Conversely, fixing the builder alone leaves this gate excluded before construction. Both consumers need the same rule.

### Interpreter applications require additional evidence

Python virtual environments are relevant portability cases, but an ambient binary path alone does not establish a missing-pytest or wrong-package outcome. CPython 3.13's path initialization can derive its program name from `orig_argv[0]`, search the child's `PATH`, and use the resulting location when looking for `pyvenv.cfg`. Consequently an ambient-selected process image can still discover the project's virtual-environment configuration. The behavior depends on platform, interpreter version, launch arguments and environment. [CPython program and executable discovery][cpython-discovery] [CPython environment discovery][cpython-venv]

Any Python extension of this audit must separately observe the selected process image, invocation path and `argv[0]`, interpreter version, prefixes and package environment. Neither `sys.executable` nor a path canonicalized through a symlink alone proves that full contract. No missing-module, incompatible-package or wrong-prefix effect is claimed from the unrun Python scenario. The native fixed-checker and unavailable-checker witnesses do not depend on CPython startup behavior.

## Controls and limits

- If codeNERD was launched from the correctly activated environment, the ambient and declared executable may coincide. That is a passing control, not evidence of a mismatch.
- If a declaration explicitly names `./tools/bin/projectcheck` or the absolute checker path, the bare-name witness does not apply. The existing builder resolves relative paths against the workspace. This is a possible owner-controlled workaround, not a repository change made by this review. Interpreter invocation paths and symlinks need their own controls rather than being collapsed to the underlying binary path.
- A declared `PATH` is passed to the child today. Child subprocess lookup can use it. The finding is specifically that it does not govern the already selected top-level executable.
- A wrapper executable may deliberately choose its own interpreter or toolchain. Its semantics need separate inspection; it is not automatically affected.
- The main witness is POSIX. Windows extension lookup, `PATHEXT`, environment-key case handling, batch wrappers and process construction require explicit Windows fixtures before claiming parity.
- Project declarations still require existing admission. This finding does not establish privilege escalation, arbitrary extra authority, a secret leak, or a bypass of forbidden-path guards.
- Missing tools, startup errors, timeouts and unavailable gates must remain distinct from test failures. Do not fix a configuration mismatch by weakening acceptance or marking an unrun gate as passed.
- The direct executor has the same construction ordering, but Docker, Firejail and other backends were not comprehensively audited here. An executor-specific corrected behavior must not be generalized to every backend. [Direct ordering][direct]

## Proposed command resolution contract

This is a proposed application of the earlier receipt audit's shared-resolver and executable-identity contract to the concrete gate-admission and launch boundary. It is not a new receipt architecture or a claim that the following API already exists. [Prior contract][prior-resolver]

### One effective invocation before admission

Construct the effective invocation in a deterministic host layer before selecting an executable:

1. Preserve the admitted logical program and argument array, workspace identity, gate ID, scope, pass rule and declaration identity.
2. Compute the environment using existing precedence and allowlisting, including Go-specific build profile behavior when applicable.
3. Resolve an explicit path under its current rules; resolve a bare name using the effective search environment and documented host-specific behavior. Preserve the selected invocation path, symlink spelling and admitted `argv[0]`; do not automatically replace them with a canonical realpath.
4. Return either a resolution with the selected executable identity or a specific unavailable/unsupported/error disposition.
5. Give availability reporting and actual dispatch that same resolution contract.

Do not mutate the parent process environment or working directory while resolving. Recurse deliberately measures up to four workspace gates concurrently. A process-global `PATH` swap would introduce cross-project contamination and races that the current environment slices avoid. [Concurrent measurement][measure-workspace]

Keep logical and resolved identities separate. The logical name is relevant to gate-language classification, configured Go flags and user-facing explanation. The selected invocation path is relevant to dispatch, and any separately observed underlying image identity is additional evidence. They are not interchangeable: invocation paths and `argv[0]` can affect interpreter environment discovery. Replacing `go` with an absolute path too early must not accidentally bypass the current Go invocation branch.

### Execute the admitted result through the existing routes

A resolved path is not new permission to execute it. The outer gate runner must retain its existing `processutil` startup, output bounds and cancellation behavior. Acceptance commands must retain the validation, auditing, timeout and backend behavior provided by their actual tactile route. Do not claim that sharing a resolver gives the outer gate tactile controls it does not currently have. Any proposal to unify execution routes is separate architectural work, not necessary to establish this finding.

Where those routes validate execution authority, retain the applicable logical and selected identities in that decision. Dispatch should use the admitted invocation path and argument identity without performing a second independent ambient-name search or substituting the symlink's realpath. For a container backend, the executable and search environment belong to the container namespace; a host absolute path is not valid evidence of its container equivalent. A backend that cannot establish the requested contract should return an honest unsupported or unavailable result.

No new LLM or JEV classification is needed for ordinary executable resolution. Go should report facts about resolution and process outcome; Mangle retains responsibility for the existing attempt, gate and completion decisions. The model may explain a configuration hold but must not decide that two different toolchains are equivalent.

### Evidence and freshness

Preserve at least the logical argv including `argv[0]`, effective working directory, environment identity, resolution method, selected invocation path and actual startup result. Record a separately observed image identity when the claimed guarantee needs it, without overwriting the invocation path. Do not publish raw environment values merely to prove which tool ran. The current host-only environment digest is a useful pattern, but its directory-plus-environment API does not include executable identity; it should not be described as a complete toolchain certificate. This is an application of the prior receipt proposal, not a separate reproduced reuse finding. [Current identity helper][identity] [Prior executable-profile contract][prior-resolver]

Choose and document freshness boundaries. Environment, declaration, workspace or executable changes must invalidate a cached resolution or force reconciliation. An absolute path alone does not prove the file was not replaced. Full content hashing may be excessive for every gate; if stronger integrity is deferred, state the exact guarantee rather than promising atomic executable identity. Do not add automatic `--version` probes to every tool: they are extra executions, not inert metadata reads, and cannot be assumed harmless or standardized.

An ambiguous legacy acceptance contract must retain the current explicit refusal rather than inventing an empty overlay or a guessed executable. Repairing this issue must not undo the existing resume-safety behavior. [Runtime refusal][runtime-contract]

## Costs and failure modes of the correction

The bounded change is one effective-environment resolver shared by gate admission and supported execution paths, not a universal environment manager.

Expected host costs include merging E environment entries and probing up to P search directories for each newly resolved bare program. Across G independent gate descriptors, an uncached upper-level model is proportional to G times the merge-and-search work. Filesystem latency and platform lookup rules dominate the constants; no latency or memory improvement was measured here. Avoid a global cache initially unless profiling justifies its invalidation and ownership complexity.

Retaining a selected executable identity adds small per-invocation metadata. Strong binary digests add reads and may cost much more for large executables, scripts with interpreters and wrapper chains. Metadata-only identity is cheaper but must disclose its weaker replacement guarantees. None of this removes test-run cost.

There is no necessary increase in model calls, embeddings or inference. The expected benefit is avoiding wrong-environment diagnostics and unusable project checks; the magnitude depends on task and environment distributions. Correct resolution can also expose real failures that the ambient tool did not exercise, increasing legitimate repair work. More failures observed under the correct contract are not automatically a regression.

Principal repair hazards are:

- Using `os.Setenv` around lookup, which races with concurrent commands.
- Correcting discovery but leaving dispatch on ambient lookup, or vice versa.
- Resolving a command in the host namespace and executing it in a container.
- Canonicalizing an interpreter symlink or rewriting `argv[0]`, thereby changing environment discovery while claiming to preserve execution identity.
- Dropping Go flags, accepted exits, declaration provenance or runtime refusal when changing descriptor shapes.
- Expanding `$PATH`, tildes or shell syntax without an explicit supported contract.
- Silently accepting a current-directory or relative-search result that existing Go safety behavior would refuse. Platform search hardening needs an explicit decision, not a convenience fallback.
- Logging raw environments, credentials or an unnecessary inventory of local files.
- Installing, activating or rewriting project environments automatically to make a check pass.
- Treating a selected path as immutable or caching it across incompatible workspaces.
- Letting corrected invocation metadata authorize a denied command or broader write scope.

## Fair evaluation plan with no execution in this review

### First establish the producer contract

Read tests already cover tool availability using a stubbed `lookPath`, declaration precedence, copied environment values, workspace-relative scripts, environment allowlisting, unstartable processes and cancellation. Those are valuable controls. They do not establish agreement between an ambient binary and a different declared-path binary. In particular, the declaration test uses a simple `CGO_ENABLED` value rather than an alternative executable directory. These test definitions were inspected, not run. [Detection fixtures][detect-tests] [Runner fixtures][run-tests]

The first future deterministic fixture should put the two distinct, benign native marker executables from witness one in separate temporary directories. Supply one through the test process's ambient path and the other through the admitted command environment. Use absolute search directories and capture the selected invocation path, fixed child marker and exit. This isolates lookup semantics without interpreter startup behavior, Python packages or model invocation.

Use production command construction and `gates.Run` → `processutil.Run` for the outer execution fixture. Separately exercise acceptance through its tactile executor, including that route's validation and lifecycle behavior. A helper-only test that resolves the desired path cannot prove that either consumer used it. Separately test `Detect` to establish that discovery and launch agree.

| Case | Expected distinction |
|---|---|
| Bare checker only in declared path | Runnable and selected under that environment; no false unavailable disposition. |
| Same name in ambient and declared paths | Selected declared tool, with exact marker/path evidence. |
| Declared path equals ambient path | Existing behavior retained. |
| No override | Existing permitted ambient behavior retained. |
| Explicit absolute and workspace-relative executables | Existing path semantics retained; no spurious search. |
| Interpreter symlink and meaningful `argv[0]` | Invocation spelling and interpreter environment discovery preserved; realpath is not silently substituted. |
| Empty or malformed effective search path | Specific refusal/unavailable result; no silent ambient fallback. |
| Missing, non-executable or removed selected file | Honest current failure; no fabricated test verdict. |
| Paths containing spaces and non-ASCII characters | Argument/path identity preserved without a shell. |
| Two simultaneous gates with different paths | No resolution or environment bleed between commands. |
| Go logical name with configured build flags | Correct selection without losing Go-specific configuration. |
| Child starts but test fails; child cannot start; cancellation | Existing outcome distinctions retained. |
| Windows names/extensions and container namespace | Separate platform/backend acceptance, not inferred from POSIX. |

Do not change the whole test process's environment concurrently. Use isolated child processes or a pure injected search seam for competing environment cases. Executable fixtures should have fixed known behavior, no provider access and no dependency downloads.

### Then evaluate the recurse route

Use ordinary CLI and chat entry adapters and the real host/policy bridge with a deterministic authorized task executor. Keep the workspace, declarations, permissions and input source identical across arms. Capture the gate set, selected executable, process outcome, finding attribution, improvement skip reason and any acceptance invocation.

For optional interpreter integration controls, pre-provision identical environments across arms and record the operating-system-observed process image separately from invocation path, `argv[0]`, interpreter-reported executable, prefixes and package environment. For Python, explicitly cover the CPython startup behavior above. Do not count package installation, activation scripts or an unrecorded wrapper as the fix, or infer missing pytest merely from ambient image selection. A green test result or `sys.executable` alone is insufficient evidence of exact image selection.

Compare three alternatives:

1. Current behavior at the pinned revision.
2. The owner's explicit-checker-path declaration, as a legitimate control.
3. The proposed shared effective-environment resolution.

Hold declarations and tests constant between the current and proposed implementation arms. The explicit-path arm is a separately labeled workaround because its command changes. Equalize process budgets and backend settings. Record false unavailable results, wrong-executable selections, real startup failures, valid test verdicts, host lookup latency and any extra model/tool turns separately.

Only after deterministic parity is established should a separately authorized real-task study ask whether this reduces failed work. Include repositories where ambient selection is already correct, not only fixtures chosen to favor the proposal. Report failure classes and unresolved backends as well as success rates. No model experiment or numerical result is claimed here.

### Acceptance and stopping condition

The defect is closed for a supported backend when discovery and dispatch use the same admitted executable-search contract, the two witnesses have exact executable evidence, unaffected controls retain behavior, and normal authorization/cancellation/error semantics still hold. Stop and report unsupported behavior when the backend cannot establish its namespace or search semantics. Do not call the broader portable-recurse design complete because these command fixtures pass.

## Primary source index

All CodeNERD links below use the inspected commit. API documentation explains platform semantics; it is not evidence of the installed runtime or a successful execution.

[pin]: https://github.com/theRebelliousNerd/codenerd/commit/7460225f77d1b14d43f61f4c4391a4cf62e573c1
[schema]: https://github.com/theRebelliousNerd/codenerd/blob/7460225f77d1b14d43f61f4c4391a4cf62e573c1/internal/projectdoc/nerdmd.go#L197-L208
[allowed]: https://github.com/theRebelliousNerd/codenerd/blob/7460225f77d1b14d43f61f4c4391a4cf62e573c1/internal/build/workspace_command.go#L12-L28
[detect]: https://github.com/theRebelliousNerd/codenerd/blob/7460225f77d1b14d43f61f4c4391a4cf62e573c1/internal/gates/detect.go#L19-L90
[split]: https://github.com/theRebelliousNerd/codenerd/blob/7460225f77d1b14d43f61f4c4391a4cf62e573c1/internal/gates/gates.go#L168-L195
[workspace-command]: https://github.com/theRebelliousNerd/codenerd/blob/7460225f77d1b14d43f61f4c4391a4cf62e573c1/internal/build/workspace_command.go#L30-L60
[gate-command]: https://github.com/theRebelliousNerd/codenerd/blob/7460225f77d1b14d43f61f4c4391a4cf62e573c1/internal/gates/run.go#L84-L99
[gate-run]: https://github.com/theRebelliousNerd/codenerd/blob/7460225f77d1b14d43f61f4c4391a4cf62e573c1/internal/gates/run.go#L45-L81
[process]: https://github.com/theRebelliousNerd/codenerd/blob/7460225f77d1b14d43f61f4c4391a4cf62e573c1/internal/processutil/command.go#L11-L50
[cli]: https://github.com/theRebelliousNerd/codenerd/blob/7460225f77d1b14d43f61f4c4391a4cf62e573c1/cmd/nerd/cmd_campaign_recurse.go#L224-L269
[chat]: https://github.com/theRebelliousNerd/codenerd/blob/7460225f77d1b14d43f61f4c4391a4cf62e573c1/cmd/nerd/chat/campaign_recurse.go#L218-L271
[pass]: https://github.com/theRebelliousNerd/codenerd/blob/7460225f77d1b14d43f61f4c4391a4cf62e573c1/internal/campaign/recurse_cycle.go#L515-L565
[measure-workspace]: https://github.com/theRebelliousNerd/codenerd/blob/7460225f77d1b14d43f61f4c4391a4cf62e573c1/internal/campaign/recurse_cycle.go#L692-L750
[improve]: https://github.com/theRebelliousNerd/codenerd/blob/7460225f77d1b14d43f61f4c4391a4cf62e573c1/internal/campaign/recurse_cycle.go#L1184-L1205
[findings]: https://github.com/theRebelliousNerd/codenerd/blob/7460225f77d1b14d43f61f4c4391a4cf62e573c1/internal/gates/findings.go#L116-L185
[attempt-handoff]: https://github.com/theRebelliousNerd/codenerd/blob/7460225f77d1b14d43f61f4c4391a4cf62e573c1/internal/campaign/recurse_cycle.go#L1164-L1179
[attempt-campaign]: https://github.com/theRebelliousNerd/codenerd/blob/7460225f77d1b14d43f61f4c4391a4cf62e573c1/internal/campaign/recurse_attempt.go#L95-L112
[runtime-contract]: https://github.com/theRebelliousNerd/codenerd/blob/7460225f77d1b14d43f61f4c4391a4cf62e573c1/internal/campaign/acceptance_runtime.go#L10-L44
[acceptance]: https://github.com/theRebelliousNerd/codenerd/blob/7460225f77d1b14d43f61f4c4391a4cf62e573c1/internal/campaign/orchestrator_acceptance.go#L158-L180
[direct]: https://github.com/theRebelliousNerd/codenerd/blob/7460225f77d1b14d43f61f4c4391a4cf62e573c1/internal/tactile/direct.go#L121-L176
[identity]: https://github.com/theRebelliousNerd/codenerd/blob/7460225f77d1b14d43f61f4c4391a4cf62e573c1/internal/processutil/identity.go#L13-L48
[detect-tests]: https://github.com/theRebelliousNerd/codenerd/blob/7460225f77d1b14d43f61f4c4391a4cf62e573c1/internal/gates/detect_test.go#L158-L224
[run-tests]: https://github.com/theRebelliousNerd/codenerd/blob/7460225f77d1b14d43f61f4c4391a4cf62e573c1/internal/gates/run_test.go#L70-L98
[prior-contract]: https://github.com/theRebelliousNerd/codenerd/blob/7460225f77d1b14d43f61f4c4391a4cf62e573c1/Docs/audits/recurse-gate-contract-continuity.md
[prior-receipts]: https://github.com/theRebelliousNerd/codenerd/blob/7460225f77d1b14d43f61f4c4391a4cf62e573c1/Docs/audits/workspace-test-receipt-invocation-identity.md
[prior-resolver]: https://github.com/theRebelliousNerd/codenerd/blob/7460225f77d1b14d43f61f4c4391a4cf62e573c1/Docs/audits/workspace-test-receipt-invocation-identity.md#L119-L128
[prior-portable]: https://github.com/theRebelliousNerd/codenerd/blob/7460225f77d1b14d43f61f4c4391a4cf62e573c1/Docs/audits/portable-recurse-deliberation-crucible.md
[go-exec]: https://pkg.go.dev/os/exec#Command
[go-source]: https://go.dev/src/os/exec/exec.go
[cpython-discovery]: https://github.com/python/cpython/blob/v3.13.0/Modules/getpath.py#L244-L291
[cpython-venv]: https://github.com/python/cpython/blob/v3.13.0/Modules/getpath.py#L341-L374

