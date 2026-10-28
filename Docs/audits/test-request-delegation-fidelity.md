# Test-request fidelity across chat delegation

**Status:** source-derived finding and proposed design, not runtime reproduction. Main `356f65e0e6bb639adaa90ab36212daa714114871`; feature `c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c`. The branches differ materially: the feature improves operation fidelity for background queries but still loses request-owned selection.

## The contract at stake

“Run existing tests in package A” carries both an operation and a selection. A tester being launched is not sufficient if the handoff changes run into authoring, or if packages A and B produce the same task description. The latter can waste compute and leave the user's requested scope unverified. Source inspection establishes a lossy handoff, not what any particular model ultimately runs.

## What the source does

1. Perception distinguishes `verify` (run existing tests) from implementing tests, maps it to `/test` and classifies it as a query: [understanding contract](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/perception/understanding_adapter.go#L764-L800), [mapping](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/perception/understanding_adapter.go#L575-L589).
2. On main, the [delegation formatter](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/cmd/nerd/chat/delegation.go#L518-L522) emits `run_tests` when the target contains the literal substring `run` or is `codebase`, otherwise `write_tests for <target>`. A prior coder result can also produce a write-tests-after-fix task: [context formatter](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/cmd/nerd/chat/delegation.go#L129-L133). This can turn an admitted verification request into a request to author tests.
3. The feature branch intentionally corrects that operation for background `/test` queries, returning `run_tests` plus the constraint. However, it omits the target and scope file: [background task](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/background.go#L233-L242). Preserve that non-writing improvement.
4. The feature's background job doesn't carry the original intent as a structured selection; Update starts it with the rewritten task, and the `TaskRequest` construction sets the verb and task but not a target: [job](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/background.go#L27-L36), [Update](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/model_update.go#L168-L173), [request](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/delegation_routing.go#L352-L358). The cloned worker starts without conversation history and executes a preset instead of re-perceiving the original user sentence: [clone](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/session/executor.go#L585-L638), [preset](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/session/task_executor.go#L361-L392).

With equal empty constraints and the normal confident tester/background route, requests targeted at `internal/runtime` and `internal/runqueue` both become `run_tests` on both heads: main matches the substring inside both names and feature substitutes it for every background `/test`. A target such as `internal/auth` produces `write_tests for internal/auth` on main, while the feature preserves run semantics but drops the named selection.

## Important limits and existing safeguards

The parent and child share a kernel, so the original current-intent facts can still appear in incidental context. A constraint can also repeat the package. Neither provides a request-owned, immutable selection in the job and preset. Therefore this finding is **not** that the target is erased everywhere or that the worker necessarily executes the whole suite.

The existing [typed verification tool](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/tools/shell/verification.go#L36-L126) supports a working directory, Go packages and test pattern, validates workspace containment and rejects custom commands. Without an explicit package selection it can default broadly or derive selection from writes; that behavior is not proof of what a particular request executes. Preserve permission, restriction, verification and completion gates.

## Proposed request and receipt contract

Carry an immutable descriptor from admission through the background job, `TaskRequest`, first worker request, typed runner and execution receipt:

- execute existing tests versus author or modify tests;
- request and owner identity;
- explicitly selected package/path, test selector and working directory, with unknown or ambiguous states;
- user constraints and independent effect restrictions;
- actual selected scope and any explicit policy-authorized widening.

A substring in a package name or the presence of a prior coder result must not determine operation. Preserve feature's execute-only correction, but carry the selection alongside it. Let Mangle arbitrate admission and any widening. A receipt should record what actually ran, not merely what was requested.

[Go's command documentation](https://pkg.go.dev/cmd/go) distinguishes package arguments from test flags, and [testing documentation](https://pkg.go.dev/testing) explains name filtering. They are useful references for typed selection, not evidence that this CodeNERD path executed a particular command.

## Evaluation and end-to-end cost

Future deterministic fixtures should drive the real process-to-Update-to-background-command path with two different named packages, a named test, explicit whole-suite selection, prior coder/no prior coder, ambiguous scope, and explicit test-authoring controls. Capture the descriptor, first worker request, actual typed-runner arguments and receipt; ensure different requests remain distinguishable, execution never silently becomes authoring, and a broader run is reported as broader. Exercise shared-kernel context, concurrent tasks, missing target, unsupported selectors, safety restrictions and failure/cancellation.

Compare current and proposed flows with fixed projects and matched quality gates. Measure selection fidelity, incorrect scope and missed requested checks, false widening, user-visible completion accuracy, test/build CPU, wall time, tool calls, input/output tokens, retries, descriptor storage and extra verification cost. Full-suite runs remain useful as a separate explicit gate; a targeted run must not be represented as complete repository verification. No speedup or reliability gain is claimed.
