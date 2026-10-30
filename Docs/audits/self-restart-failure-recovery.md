---
doc-class: deep-dive
subsystem: cli-selfrestart
implementation-status: implemented
evidence-status: source-only
last-verified: 2026-10-30
verified-against: fef98230c20a5751e4c32a2fd2757832a9d39cdc
verification-method: static-source-and-test-definition-inspection
supersedes: []
---

# Recover self restart after a later rebuild

## Conclusion and scope

Bind restart failure suppression to the attempted executable identity rather than leaving it as an indefinitely latched chat-model boolean. A failed restart must not immediately retry the same failed build, but a distinct later rebuild must be allowed to reach the existing idle-state check. The current chat adapter records a failed attempt and never clears that record on its rebuild path. It can therefore announce that a later build will be used when idle while refusing the check that would perform that restart.

This is a source-supported state-transition defect, not an observed production incident. It applies to the automatic in-place restart route on a supported platform after a restart error and a subsequently delivered eligible rebuild event. It does not establish that normal first-time restarts fail, that a failed exec terminates the old process, or that all session state is lost. Windows deliberately reports a manual-restart instruction instead of taking the automatic route. [Chat failure handling](https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/cmd/nerd/chat/self_restart.go#L189-L207), [platform branch](https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/cmd/nerd/chat/self_restart.go#L99-L109), [Windows implementation](https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/selfrestart/exec_windows.go#L7-L16).

All implementation citations are pinned to public main [`fef98230c20a5751e4c32a2fd2757832a9d39cdc`](https://github.com/theRebelliousNerd/codenerd/commit/fef98230c20a5751e4c32a2fd2757832a9d39cdc), revalidated as main at approximately 05:41 UTC on October 30, 2026. This review read source and existing test definitions only. It ran no repository code, tests, model inference, training, benchmarks, executable replacement or failure injection. The remediation and evaluation below are proposed, not implemented or executed.

## The intended recovery boundary

The implementation already states a bounded suppression contract. The comment on `performRestart` says that after failure the model no longer tries until the next rebuild. The existing test describes the same boundary, then verifies that an immediate restart check does not retry. That contract is sensible: an idle timer should not repeatedly attempt a known failed executable, but a later replacement should not be permanently excluded by an earlier failure. [Contract and implementation](https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/cmd/nerd/chat/self_restart.go#L189-L207), [existing failure test](https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/cmd/nerd/chat/self_restart_test.go#L110-L129).

The route is wired into the interactive application, not merely an unused helper. Boot completion starts the executable watcher. The update switch dispatches rebuild notifications and restart-check messages into the handlers inspected here. `RunInteractiveChat` enters `runChatProgram`, whose restart-failure branch keeps the returned model and starts another TUI program around it. [Boot wiring](https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/cmd/nerd/chat/model_update.go#L868-L884), [message dispatch](https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/cmd/nerd/chat/model_update.go#L615-L624), [interactive loop](https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/cmd/nerd/chat/self_restart.go#L159-L186).

## The current state transition

Three source boundaries establish the mismatch:

1. `performRestart` calls `Restarter.Restart`. Any returned restart error sets `m.restartFailed = true`, appends the error notice and returns that model. An actual exec error is one concrete way to reach this branch; a restore-file write error can also return through it. [Chat latch](https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/cmd/nerd/chat/self_restart.go#L192-L205), [restart error paths](https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/selfrestart/selfrestart.go#L217-L234).
2. `handleBinaryRebuilt` records an accepted changed identity, appends the ordinary automatic-restart announcement and schedules another restart check. It does not clear or replace the failure latch. [Rebuild handler](https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/cmd/nerd/chat/self_restart.go#L99-L109).
3. `handleRestartCheck` returns immediately when `m.restartFailed` is true, before consulting `ShouldRestart` or `restartBusy`. The later build does not get as far as the normal idle gate. [Check handler](https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/cmd/nerd/chat/self_restart.go#L112-L130).

Reopening the TUI is not a counterexample to this chain. `runChatProgram` retains the failed model. `Model.Init` schedules boot commands, and a later boot completion can start another watcher; neither that sequence nor `handleRebuildWatch`, which replaces the restarter and channel, clears `restartFailed`. The source establishes a latch that survives this normal retry-of-the-UI path, not a claim that a completely new manually launched process inherits it. [Retained model](https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/cmd/nerd/chat/self_restart.go#L170-L186), [model initialization](https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/cmd/nerd/chat/model_lifecycle.go#L193-L205), [watcher adoption](https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/cmd/nerd/chat/self_restart.go#L76-L84).

## A precise later rebuild witness

Let A be the running process image, B a build whose restart attempt returns an error, and C a distinct subsequent rebuild at the watched executable path. This is a proposed deterministic fixture, not instructions to replace a live executable.

The witness requires all of the following:

- The platform supports in-place exec, a watcher/restarter is present, and B was admitted far enough for `performRestart` to return an error and preserve the failed model.
- After that failure, a C notification reaches `handleBinaryRebuilt`. The witness does not assume that every filesystem event is delivered or that every file is a valid executable.
- C differs from B under the implementation's identity fields and also differs from the baseline identity held by the restarter receiving the notification. At least one of size, modification time or inode differs in each relevant comparison. The baseline may have been refreshed when the reopened TUI starts another watcher, so the argument does not require it to remain A.
- The restarted check runs after the session is otherwise eligible: no foreground load, no boot and no approval view. The failure latch alone is sufficient to stop it.

These conditions deliberately use the actual identity contract. `Identity.Changed` compares size, modification time and inode; it does not compare the displayed build label, hash executable contents, or require a monotonically increasing time. `BinaryChanged` compares a notification with `r.current`, then assigns `r.pending`. [Identity comparison](https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/selfrestart/selfrestart.go#L33-L57), [pending publication](https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/selfrestart/selfrestart.go#L184-L194).

With those conditions met, C is accepted and the user is told it will be used when idle. The scheduled check then sees the still-true latch and returns without producing a restart request. Repeated idle checks do not repair the state. The consequence established by source is suppression of later automatic restart in the retained model and an inaccurate recovery announcement, not a demonstrated bad result from executing C. [Announcement](https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/cmd/nerd/chat/self_restart.go#L100-L109), [early return](https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/cmd/nerd/chat/self_restart.go#L114-L123).

## Existing protections to retain

- **No immediate failure loop.** The existing failed-exec test requires that an immediate check produces neither another restart request nor another command. Preserve that outcome for the failed identity. [Test](https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/cmd/nerd/chat/self_restart_test.go#L112-L129).
- **Busy and approval deferral.** Restart eligibility checks loading, boot and approval view. The busy-session test exercises waiting for a turn to settle before quitting the TUI. Recovering from failure must not bypass these guards. [Guard](https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/cmd/nerd/chat/self_restart.go#L114-L130), [existing turn test](https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/cmd/nerd/chat/self_restart_test.go#L26-L45).
- **Settled executable observation.** The watcher compares successive observations and requires a successful Go build-info read before publishing an identity. This is not a guarantee that exec will succeed, but it filters partial builds. [Watcher](https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/selfrestart/watch.go#L52-L70).
- **Failure leaves the old image running.** The restart mechanism reports the exec error and removes the handoff file on that error path; the chat appends the reason and reopens its UI. Do not convert recovery into forced termination. [Exec error handling](https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/selfrestart/selfrestart.go#L217-L234), [UI recovery](https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/cmd/nerd/chat/self_restart.go#L170-L205).
- **Platform distinction.** Windows keeps the manual-restart path. A fix to automatic recovery need not add Windows exec support. [Platform implementation](https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/selfrestart/exec_windows.go#L7-L16).

## Proposed bounded repair

Keep this in the existing restart mechanism and chat adapter. No new autonomous controller, provider call, prompt atom or model decision is needed for an executable-generation bookkeeping repair. The objective is to make the already documented failure boundary executable, while preserving the idle and platform conditions.

The recommended contract is:

1. Remember which observed executable identity the failed attempt targeted, including failures before exec if they share the same suppression policy.
2. Suppress automatic retries for that failed identity. An unrelated idle tick, a duplicate observation, or a watcher replacement must not erase that suppression.
3. Admit a distinct later observed identity, then use the normal busy/approval check before preparing a restart request. A later failure establishes the new suppressed identity.
4. Make the notice describe the admitted state. Do not announce a pending automatic restart while the same state is already known to prevent it.
5. Keep any deliberate retry of an unchanged build as a separate, explicit product decision. This memo does not propose a retry timer or a new command.

Simply clearing the boolean whenever `BinaryChanged` returns true is not a fully specified repair. That function compares against `current`, not against the last failed or already-pending identity. If the current baseline is A, repeated B notifications can still compare as changed. A recovery fix must distinguish a genuinely new post-failure candidate from another report of B, rather than trading permanent suppression for repeated attempts. This is an acceptance requirement for the proposed fix, not a claim that the current latched implementation already retries duplicates. [Existing comparison](https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/selfrestart/selfrestart.go#L184-L194).

## Evaluation and costs

Start with deterministic handler tests and an injected `ExecFunc`; the existing tests already provide that seam. Observe request generation, pending identity, failure state, notices and invocation count. Do not execute a real replacement merely to demonstrate this boolean transition. Then validate the reopened-program and watcher handoff in an authorized isolated fixture, because model reuse and watcher reinitialization are separate from the local handler chain. [Existing injected failure](https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/cmd/nerd/chat/self_restart_test.go#L112-L127), [exec injection seam](https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/internal/selfrestart/selfrestart.go#L169-L182).

| Proposed case | Required result |
|---|---|
| First eligible rebuild with an idle session | Existing restart-request behavior remains. |
| B fails and no new identity arrives | No immediate or periodic automatic retry of B. |
| Duplicate B notifications after failure | Suppression remains and the UI does not promise an admitted restart. |
| Distinct C arrives after B fails | C becomes eligible; an idle check creates a restart request targeting C. |
| C arrives while loading, booting or in approval view | C remains pending without quitting; eligibility resumes only when the existing guard clears. |
| C fails and later distinct D arrives | The same generation-scoped failure and recovery contract repeats. |
| Reopened TUI replaces its watcher/restarter | Failure ownership survives reinitialization; distinct later C can still recover. |
| Changed display label with unchanged identity fields | No invented generation change; preserve the current identity contract. |
| Size, modification time or inode changes independently | Each supported identity dimension can admit the intended later observation. |
| Restore-file creation fails before exec | Report failure honestly and apply the explicitly chosen suppression contract; no false exec claim. |
| Windows rebuild | Manual-restart instruction remains; no automatic exec request. |
| Successful state round trip | Existing conversation, mode, brief, held work and draft expectations remain covered by the existing round-trip test. |

The bookkeeping repair should need only a small amount of bounded state and constant-time comparisons. It does not require model inference or embedding expenditure. Deterministic tests cost local compilation and test time; integration coverage adds temporary state IO and watcher scheduling. A real exec smoke test, if later authorized, requires a disposable process/terminal fixture and should be reported separately from handler tests. No latency, memory or reliability improvement has been measured here.

The current failure test ends after proving suppression before the next rebuild; it never delivers a later changed identity. Extending that transition coverage is the decisive acceptance addition. The existing successful restore test remains useful but does not prove recovery after an error. [Failure-test endpoint](https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/cmd/nerd/chat/self_restart_test.go#L110-L129), [round-trip fixture](https://github.com/theRebelliousNerd/codenerd/blob/fef98230c20a5751e4c32a2fd2757832a9d39cdc/cmd/nerd/chat/self_restart_test.go#L47-L100).

## Related engineering guidance

[Amazon’s discussion of retries and backoff](https://aws.amazon.com/builders-library/timeouts-retries-and-backoff-with-jitter/) highlights how uncontrolled retries can amplify failure. The proposed identity-scoped suppression retains that concern while permitting a genuinely new build; the article does not validate this implementation or prescribe its exact state machine.

## Prior work and exit condition

This review found no existing self-restart audit in the inspected `Docs/audits` inventory; an indexed audit-directory search for `selfrestart` also returned no matches. Search coverage is not proof of exhaustive novelty, and the index can lag main. The narrow subject here is the newly introduced failed-attempt-to-later-rebuild handoff, not general session persistence, executable trust, all watcher lifecycle behavior or the broader mission-driven improvement proposal.

**Exit condition:** a failed attempt remains suppressed for its own executable identity, a distinct eligible later identity can reach the unchanged idle/approval gate in the retained chat session, and the UI states the actual disposition. Both the failure-to-new-build transition and duplicate-failed-build suppression must be pinned by tests before this finding is marked fixed. This memo does not mark either check passed.


## As built

Rechecked against worktree baseline `8d373fb6ca05ef03fec59b923c5f1925066d623a`
and the authored changes. The earlier analysis describes that baseline; this
section records the resulting implementation. Evidence remains source inspection
and test definitions, with no executed behavioral acceptance result.

| Finding or boundary | Ruling and implementation |
|---|---|
| A failed restart permanently excludes later builds | **Real, repaired.** `cmd/nerd/chat/model_types.go` retains the attempted `selfrestart.Identity`; `handleRestartCheck` in `cmd/nerd/chat/self_restart.go` suppresses only that identity. A distinct candidate reaches the existing idle gate. |
| Duplicate B notifications could defeat a boolean-reset repair | **Real design constraint, addressed.** `internal/selfrestart/selfrestart.go` deduplicates pending identities. The chat suppresses notices and requests for its failed identity even after restarter replacement. Size, time and inode remain the identity dimensions; display text does not admit a generation. |
| Failure ownership must survive watcher replacement | **Real, addressed.** Failure identity remains on the retained model, independently of the replacement restarter's baseline. |
| Busy/approval deferral, old-process survival, state round trip and platform distinction | **Existing protections retained.** `restartBusy`, restore-file IO, injected exec and the minimal platform stub remain in their existing production paths. Restore-write failure receives the same identity suppression as exec failure. |

`cmd/nerd/chat/self_restart_test.go` adds
`TestSelfRestart_FailureIsScopedToTheAttemptedIdentity` (including watcher
replacement, duplicate notices, repeated B/C/D failures and all idle guards)
and `TestSelfRestart_RestoreWriteFailureCanRecoverOnAnotherBuild`.
`internal/selfrestart/selfrestart_test.go` adds
`TestRestarter_TracksDistinctPendingIdentities`, including independent size,
time and inode changes, display-only changes and return to the running identity.
The existing failed-exec and restore round-trip tests remain.

Fail-before evidence is **static**, not an executed red test: the baseline's
`restartFailed` boolean remains true when a later candidate arrives, so the new
recovery assertion cannot obtain a restart request. Its `BinaryChanged` compares
only against the running identity, so the new duplicate-pending assertion also
fails by inspection. No finding was dismissed as wrong or already repaired.

The as-built contract is documented in
`Docs/architecture/cli/IMPLEMENTED_SPEC.md`. No prompt atoms or production Mangle
files changed. Builds, tests, live exec and commits were not run under the
author-only task constraint. Authorized package execution and isolated TUI/exec
acceptance remain pending; this implementation status does not claim they passed.
