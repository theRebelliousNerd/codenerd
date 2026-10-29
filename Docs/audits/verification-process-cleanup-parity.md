---
doc-class: deep-dive
subsystem: session
implementation-status: proposed
last-verified: 2026-10-29
verified-against: 95654a64562f02c00040ac28ca9cda4733c96fd9
verification-method: static-source-inspection
supersedes: []
---

# Verification process cleanup parity

## Finding and evidence boundary

Session-owned build and test verification bypasses the subprocess lifecycle wrapper already used by typed verification tools. The session runners construct `exec.CommandContext` commands but call raw `cmd.CombinedOutput()`. Consequently, canceling a verification context does not establish the process-group or Windows job cleanup supplied by `processutil`.

This is a **source-supported, conditional failure witness**, not a measured incident or an implemented fix. Cancellation reaches the verification context in the scenario below; the gap is lower down, at subprocess ownership. A canceled Go driver can leave an already-running test binary alive on a platform where nothing independently terminates that child. Neither driver exit nor UI settlement proves descendant termination. The proposed contract is parity with the existing cleanup mechanism, not a promise of universal tree containment.

CodeNERD references are pinned to the revision above. The pinned Go sources establish reference API and implementation behavior, not the runtime version used by any particular CodeNERD installation.

## The omitted lifecycle boundary

[`verifyBuildRunner` and `verifyTestRunner`](https://github.com/theRebelliousNerd/codenerd/blob/95654a64562f02c00040ac28ca9cda4733c96fd9/internal/session/verify_outcome.go#L67-L78) set command directory and environment. Build verification calls `cmd.CombinedOutput()` directly. Test verification calls `processutil.NonInteractive(cmd).CombinedOutput()`: that expression still invokes the raw command method. `NonInteractive` only supplies finite empty stdin when stdin is unset; it does not install cancellation, a kill scope, or a pipe-wait bound.

The existing alternative is [`processutil.CombinedOutput`](https://github.com/theRebelliousNerd/codenerd/blob/95654a64562f02c00040ac28ca9cda4733c96fd9/internal/processutil/command.go#L12-L65). It captures output through `Run`, whose owned start path installs `WaitDelay`, noninteractive stdin, and a platform kill scope, and releases that scope after waiting. The package's [agent instructions](https://github.com/theRebelliousNerd/codenerd/blob/95654a64562f02c00040ac28ca9cda4733c96fd9/internal/processutil/agents.md#L1-L7) explicitly require `CommandContext` commands to use these wrappers rather than raw command methods. The [typed verification tool](https://github.com/theRebelliousNerd/codenerd/blob/95654a64562f02c00040ac28ca9cda4733c96fd9/internal/tools/shell/verification.go#L211-L225) already does so. This is therefore session-runner drift, not a defect in every build or test tool.

Go's reference [`CommandContext`](https://github.com/golang/go/blob/6e676ab2b809d46623acb5988248d95d1eb7939c/src/os/exec/exec.go#L475-L486) defaults cancellation to `Process.Kill` and leaves `WaitDelay` unset. [`Process.Kill`](https://github.com/golang/go/blob/6e676ab2b809d46623acb5988248d95d1eb7939c/src/os/exec.go#L327-L331) kills the process itself, not its children. Setting stdin alone does not change that ownership boundary.

## A reachable foreground path

The ordinary TUI turn derives a cancelable context from its shutdown context and records its cancel function without adding another turn timeout. The normal mutation route passes that context into `VerifyWithRetry` ([turn setup](https://github.com/theRebelliousNerd/codenerd/blob/95654a64562f02c00040ac28ca9cda4733c96fd9/cmd/nerd/chat/process.go#L76-L92), [delegation](https://github.com/theRebelliousNerd/codenerd/blob/95654a64562f02c00040ac28ca9cda4733c96fd9/cmd/nerd/chat/process.go#L560-L571)). The verifier invokes observed execution with that context; execution errors can return as `ExecutionError`, so reaching this boundary does not require an affirmative judge verdict ([verifier](https://github.com/theRebelliousNerd/codenerd/blob/95654a64562f02c00040ac28ca9cda4733c96fd9/internal/verification/verifier.go#L286-L293)).

For an admitted, authorized Go edit through the ordinary `/coder`-to-`/fix` route, common observed execution reaches an isolated or inline session and calls `ProcessWithIntent` with the same context ([executor](https://github.com/theRebelliousNerd/codenerd/blob/95654a64562f02c00040ac28ca9cda4733c96fd9/internal/session/task_executor.go#L350-L441)). This witness assumes the intended route was admitted; it does not assert that every input receives that classification.

The [shared terminal verification gate and drivers](https://github.com/theRebelliousNerd/codenerd/blob/95654a64562f02c00040ac28ca9cda4733c96fd9/internal/session/executor_tools.go#L229-L236) dispatch build/test verification through the [session drivers](https://github.com/theRebelliousNerd/codenerd/blob/95654a64562f02c00040ac28ca9cda4733c96fd9/internal/session/executor_tools.go#L484-L550). With tests enabled and the requisite configuration/result, [`gateTests`](https://github.com/theRebelliousNerd/codenerd/blob/95654a64562f02c00040ac28ca9cda4733c96fd9/internal/session/build_verify.go#L338-L359) leads to runnable-package selection and either coverage or ordinary tests ([gate](https://github.com/theRebelliousNerd/codenerd/blob/95654a64562f02c00040ac28ca9cda4733c96fd9/internal/session/build_verify.go#L537-L555), [coverage delegation](https://github.com/theRebelliousNerd/codenerd/blob/95654a64562f02c00040ac28ca9cda4733c96fd9/internal/session/coverage_profile.go#L242-L259)). Given a valid workspace, nonempty package selection, and a Go toolchain, [`verifyTests`](https://github.com/theRebelliousNerd/codenerd/blob/95654a64562f02c00040ac28ca9cda4733c96fd9/internal/session/test_verify.go#L256-L307) runs `go test -json` through `runVerificationCommand` and the raw test runner.

## Conditional witness: stop reaches cancellation, not the child

Use a foreground turn with no earlier parent deadline. Let an actual, uncached test binary start quiet, long-running work, still within its own test timeout. Then deliver Ctrl+X while the turn is loading. The [key handler](https://github.com/theRebelliousNerd/codenerd/blob/95654a64562f02c00040ac28ca9cda4733c96fd9/cmd/nerd/chat/model_key_handler.go#L60-L79) requests stop; the [owner-checked turn stop](https://github.com/theRebelliousNerd/codenerd/blob/95654a64562f02c00040ac28ca9cda4733c96fd9/cmd/nerd/chat/side_turn.go#L163-L183) latches the request and calls the actual cancel function. This is not a stale-owner or display-only stop. Normal Ctrl+X input is not an operating-system process-group signal.

The [verification supervisor](https://github.com/theRebelliousNerd/codenerd/blob/95654a64562f02c00040ac28ca9cda4733c96fd9/internal/session/verify_outcome.go#L102-L148) skips pre-canceled work, otherwise creates an independently budgeted context and starts the runner in a goroutine. In the ordinary parent-canceled branch it cancels that budget context, receives the runner result, and returns `VerifyCanceled`. Choose cancellation first, without an earlier deadline, so delivery is unambiguous. Joining the runner does not establish that all descendants were joined or killed.

Raw `CommandContext` cancellation kills the Go driver. If its test child is not externally killed or coupled to parent death, the child can outlive that stop and potentially finish test work later. Go's [JSON output arrangement](https://github.com/golang/go/blob/6e676ab2b809d46623acb5988248d95d1eb7939c/src/cmd/go/internal/test/test.go#L1439-L1459) and [test-child command](https://github.com/golang/go/blob/6e676ab2b809d46623acb5988248d95d1eb7939c/src/cmd/go/internal/test/test.go#L1625-L1656) include their own converter and cancellation machinery. Killing the driver prevents its ordinary cleanup code from executing; it does not directly cancel the driver's managed child context. The binary may nevertheless finish normally, hit its timeout, encounter a broken pipe, or be terminated by an external group/job action. No permanent survivor or universal hang is claimed.

A separate, conditional risk concerns retained output descriptors. With `WaitDelay == 0`, inherited pipes can keep output-copy goroutines waiting for EOF ([Go pipe semantics](https://github.com/golang/go/blob/6e676ab2b809d46623acb5988248d95d1eb7939c/src/os/exec/exec.go#L277-L301), [goroutine wait](https://github.com/golang/go/blob/6e676ab2b809d46623acb5988248d95d1eb7939c/src/os/exec/exec.go#L952-L974)). That requires proving retention of CodeNERD's outer capture descriptor. The ordinary `go test -json` witness does **not** establish that descriptor topology and is not evidence of a blocked capture join.

## Existing cleanup contract and its limits

On non-Windows platforms, the [existing scope](https://github.com/theRebelliousNerd/codenerd/blob/95654a64562f02c00040ac28ca9cda4733c96fd9/internal/processutil/proc_tree_other.go#L16-L32) starts a new process group and cancels by killing that group plus the direct process. This covers descendants remaining in the group, not demonstrated containment of descendants that escape it.

On Windows, the [job implementation](https://github.com/theRebelliousNerd/codenerd/blob/95654a64562f02c00040ac28ca9cda4733c96fd9/internal/processutil/proc_tree_windows.go#L15-L79) documents a post-start job-assignment race and fallback limitations. Cancellation uses job termination or bounded `taskkill` plus direct process termination; release closes the handle. It deliberately does not enable `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE`, preserving background children on normal exit. Reuse must preserve that behavior rather than turn successful command completion into blanket descendant termination.

The wrapper's five-second `PipeWaitDelay` bounds pipe waiting in its applicable paths. It is not a universal five-second cancellation wall clock including cancellation hooks, fallback helpers, and operating-system behavior. Tree cancellation and bounded waiting serve different purposes: `WaitDelay` alone can permit survivors, while tree killing alone does not cover every retained-descriptor shape.

## Novelty and current test evidence

The earlier [verification-cancellation-after-deadline audit](https://github.com/theRebelliousNerd/codenerd/blob/566290cb86e4fb2dd45ce0e87e7626d88c2a5af0/Docs/audits/verification-cancellation-after-deadline.md) explicitly excluded descendant cleanup. Here cancellation reaches `budgetCtx`; no deadline-prefix behavior is required. This proposal neither supersedes nor fixes that separate context-propagation defect. Turn ownership and side-turn ordering are also separate boundaries.

The current-main screen covered 92 audit Markdown files without this exact session-runner omission. A known [architecture gap analysis](https://github.com/theRebelliousNerd/codenerd/blob/95654a64562f02c00040ac28ca9cda4733c96fd9/Docs/architecture/tools/03-GAP-ANALYSIS.md#L90-L107) already identified subprocess ownership for impacted tests; [`runGoTests`](https://github.com/theRebelliousNerd/codenerd/blob/95654a64562f02c00040ac28ca9cda4733c96fd9/internal/tools/codedom/run_impacted_tests.go#L441-L459) now uses `processutil.CombinedOutput` and avoids a pass receipt on cancellation. This proposal extends that parity, not the repository's lifecycle design.

Existing [processutil tests](https://github.com/theRebelliousNerd/codenerd/blob/95654a64562f02c00040ac28ca9cda4733c96fd9/internal/processutil/cancellation_test.go#L15-L99) exercise tree cancellation and normal exit. The [session cancellation test](https://github.com/theRebelliousNerd/codenerd/blob/95654a64562f02c00040ac28ca9cda4733c96fd9/internal/session/verify_outcome_test.go#L435-L467) instead uses a scripted runner reacting to `ctx.Done`, not an operating-system descendant. Tests were read, not run; that scripted test cannot prove this boundary.

## Proposed repair contract and costs

Specify that every session-owned verification command uses existing `processutil` ownership: start, cancellation scope, bounded pipe wait, and release. Preserve argv, environment, directory, captured output, and verification outcomes. This is Go lifecycle plumbing, not new model-tool authority, free-form shell access, planner behavior, verification budgets, or count caps. Each operation must own only its own scope, never the TUI or sibling work.

Retain partial output and canceled, indeterminate, and known-failure distinctions. A naive runner swap is insufficient: a successful direct-process exit with retained pipes can newly produce `exec.ErrWaitDelay` when bounded capture closes those pipes ([Go contract](https://github.com/golang/go/blob/6e676ab2b809d46623acb5988248d95d1eb7939c/src/os/exec/exec.go#L295-L301)). The [current supervisor](https://github.com/theRebelliousNerd/codenerd/blob/95654a64562f02c00040ac28ca9cda4733c96fd9/internal/session/verify_outcome.go#L136-L143) treats a nonnil error with a live budget as `VerifyFailed`. Specify incomplete capture/cleanup separately, for example as indeterminate, rather than inventing a compiler/test failure, repair trigger, or pass. Cancellation remains canceled. The current zero-`WaitDelay` runner does not emit this newly introduced error through that bound. Platform degradation must remain visible; the owner-context deadline fix stays separate.

Implementation plumbing is small; cross-platform qualification is moderate. Costs include per-command group/job setup and release, degraded Windows cancellation helpers, and pipe-wait latency. Bounded pipe closure can lose unflushed output tails, which must be treated as partial. Killing work interrupts fixture cleanup and does not undo prior effects. No additional model or embedding call is needed; performance improvements remain unmeasured.

## Fair prospective evaluation

No evaluation below was run. Compare current raw runners, a `WaitDelay`-only variant, and existing-wrapper reuse under identical configuration:

- Use controlled self-reexecuting parent/quiet-child fixtures with PID handshakes and markers, not sleeps as primary ordering. Cover capture, no capture, separated pipes, and explicitly demonstrated outer-descriptor inheritance.
- Exercise a real uncached `go test -json` fixture: observe the test child starting, then stop through the normal TUI path or a deterministic model double admitting a Go edit into the actual post-edit gate. Check driver **and** descendant exit, absence of subsequent markers, and survival of unrelated queued/sibling work. Cached success is an invalid child-lifecycle control.
- Compare Native and Piggyback post-edit paths; use the already-wrapped typed verification tool as a good control. Include normal pass/failure, pre-cancellation, no child, no parent deadline, no operation timeout, and the distinct late-parent-deadline gap expected to remain. For exit-zero plus retained capture pipes, require incomplete-output classification and no false repair.
- Include independent child timeout/completion and external process-group termination as controls: eventual termination is not proof that CodeNERD cancellation owned cleanup. Qualify Windows job success, fallback, and assignment races. Document escaping POSIX process groups as a limit, not a silent pass.

Record cancellation-to-driver-exit, descendant-exit, and return latency distributions; live process/handle/goroutine retention; post-stop workspace effects; partial-output and outcome fidelity; normal verified success; CPU/memory; and model-token/tool counts. Receipts should identify PIDs, scopes, revision, platform, and configuration. Fast return fails if the scoped child remains alive. Conclusions must stay within assessed platforms and descriptor shapes, not claim blanket proof of zero orphans.
