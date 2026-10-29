---
doc-class: deep-dive
subsystem: campaign
implementation-status: proposed
evidence-status: source-only
last-verified: 2026-10-29
verified-against: 9dcb01b9736bf4763aa3ac391d29f133d73c6611
scope: recurse compensation consumer conformance
supersedes: []
---

# Recurse compensation must preserve unrelated user writes

## Conclusion and scope

The live TUI can accept an explicit `/write` while its background `/recurse` attempt is running. If the user then invokes local `/recurse stop`, the outer cancellation cleanup can restore that unrelated tracked file to `HEAD`, even when the attempt never touched it. The collector identifies workspace differences, not their owner. This is a conditional source deduction with a concrete production-route interleaving, not an observed incident or an executed reproduction.

Treat this as a **scoped consumer-conformance addendum** to the existing tool-effect-receipts contract, not a new discovery that concurrent-write attribution matters. That contract already distinguishes differences from authorship, requires ownership or compare-and-swap for compensation, and explicitly leaves concurrent attribution open. The missing consumer boundary here is the outer recurse Git ratchet. Its executor returns an error, not an owned mutation set. No durable replay redesign is needed to establish this finding. [Existing contract][contract] [Open attribution][open-attribution] [Executor boundary][executor]

All repository citations below are pinned to `9dcb01b9736bf4763aa3ac391d29f133d73c6611` in the public `theRebelliousNerd/codenerd` repository. This review performed source inspection only: no code changes, tests, runtime, inference, training, or benchmark execution. The proposed changes preserve permission checks, risk preflight, write guards, verification, campaign acceptance, and the Mangle executive. The user authorizes `/write`; the defect is a later compensation adopting another operation's write, not a permission bypass or a false task-completion claim.

Main and active heads were rechecked on 2026-10-29 at 19:19 UTC. The eight-file admission, write, and recurse-cleanup slice inspected here is byte-identical on the Mangle/store (`a53cfc308e8836c72370adaa46deba3f7b00180d`), session (`80af1863d3a3aa46a722ac8b0c8a0985582d4995`), and OAuth (`aa7988dec4c62116656ea722ff73b687a8509908`) lanes. Jules QA (`51cfd42649552866d4aa54c53b68acf5740eb09b`) matches seven files; its otherwise-different command-handler file retains the same recurse block. Research-1018 at `ff603b35653abf15148d22e617cb1824d869e534` adds the separate campaign-seeding and JIT-learning audits, not a repair of this path. Root, chat, campaign, and public-repository guidance were read; this note contains only public CodeNERD sources and generic fixtures.

## A reachable same TUI interleaving

Assume a healthy, admitted recurse loop in ordinary `ChatView`, with no foreground turn, approval modal, or other input mode intercepting submission. The repository has a clean tracked working tree at loop admission; `HEAD` remains stable afterward. The selected node is a directory such as `pkg/a`. A tracked `notes.txt` is inside the workspace but outside every declared and snapshot root of this attempt, initially matches `HEAD`, and is never touched by the attempt. These are witness conditions, not claims about every campaign.

The attempt must reach its untracked-name snapshot and enter `cfg.Execute`. It remains in flight while the user writes and requests cancellation; a context-cooperative executor/provider is sufficient. No particular model answer or mistaken model behavior is required. For the concluding overwrite, require the callback and its closure cleanup to return, the user's bytes still to exist when outer cleanup collects changes, and Git diff, reset, tree inspection, and checkout to succeed. There is no intervening commit or unrelated reset.

| Step | Production route and evidence |
|---|---|
| 1 Start recurse | `/recurse` aliases `/campaign recurse`. Startup checks kernel, client, shard manager, task executor, existing recurse state, and DAG arguments. The loop receives `m.workspace`; its callback builds an attempt campaign, creates a fresh orchestrator, calls `SetCampaign`, and runs it. [Command routing][commands] [Startup and callback][start] |
| 2 Keep the chat usable | On `recurseStartedMsg`, the model sets `m.recurse`, clears `isLoading`, and schedules the loop. This route does not enroll a foreground turn. Enter submits when `!m.isLoading && m.turn.current() == 0`; slash dispatch precedes ordinary processing. The approval view is modal, which is why the witness excludes it. [Background lifecycle][lifecycle] [Enter gate][enter] [Modal guard][modal] [Slash dispatch][submit] |
| 3 Begin an actual attempt | `RunRecurseCycles` obtains the recurse lock and prepares Git before running passes. The attempt snapshots untracked names, persists an in-flight record and journal entry, then calls `cfg.Execute`. [Admission][admission] [Attempt and cancellation][cancel] |
| 4 Submit the separate write | The user submits `/write notes.txt new-owner-text`. `handleCmdWrite` parses the payload and synchronously calls `writeFileContent`, reporting success if it succeeds. That helper resolves the path, creates its parent directory if needed, and calls `os.WriteFile`; neither function acquires recurse admission or the campaign write lease. [Write dispatch][write-dispatch] [Write handler][write] [File helper][write-helper] |
| 5 Cancel locally | The same TUI receives `/recurse stop`. The handler uses `stopRecurse`, which calls `m.recurse.cancel()`. This is not the external stop-file path that waits for an attempt boundary. [Stop routing][stop-routing] [Local cancellation][local-stop] |
| 6 Collect and compensate | Once `Execute` returns, `ctx.Err() != nil` enters cleanup before normal remeasurement or a ratchet verdict. With a cancellation-free context, it calls `changedByAttempt`, then `git.revert`; it removes the in-flight record only after a successful revert, and returns the context error. [Attempt and cancellation][cancel] |

The decisive set is `git diff --name-only --relative -z HEAD -- .`, plus currently untracked, nonignored names absent from the initial untracked-name set. `changedByAttempt` removes only `.nerd/recurse/` entries; it does not filter by attempted node, observed effect receipt, or writer identity. Therefore the changed tracked `notes.txt` enters the set despite having a different owner. [Collector][collector] [Only filter][filter]

`revert` resets the selected index paths, checks membership in `HEAD`, removes selected paths absent from `HEAD`, and checks out selected tracked paths from `HEAD`. Under the stated conditions, `notes.txt` becomes its committed version, replacing `new-owner-text`. Git documents `diff <commit>` as a comparison with the working tree and tree-plus-path checkout as replacing those paths from that tree; branch-switch dirty-worktree protections do not supply ownership for this explicit path restoration. [Revert implementation][revert] [Git diff documentation](https://git-scm.com/docs/git-diff) [Git checkout documentation](https://git-scm.com/docs/git-checkout)

This establishes loss of the user's current working-tree version, not permanent irrecoverability: backups, editor history, or another retained copy may exist. A newly created, nonignored `notes.txt` after the initial name snapshot is likewise selected and removed if it still exists at outer cleanup. Conversely, an untracked name already in that snapshot is excluded. The tracked, outside-node witness is primary because it avoids attributing the outcome to a lower-level snapshot rollback or new-file sweep.

The same broad set also feeds normal ratchet commit and revert branches. Adoption there is conditional on the corresponding measurements and verdict; this review predicts neither a passing acceptance check nor a particular commit. The canceled path already suffices and does not depend on either. [Normal ratchet][ratchet]

## Existing protections and the remaining boundary

The clean-checkout refusal is useful: `prepare` rejects tracked dirt at startup. It does not prove exclusive ownership of subsequent edits. Similarly, the advisory OS lock prevents another recurse loop from sharing that workspace; it is not a lock all writers acquire. The local `/write` path above does not participate, and `recurseRunning` is a status observation rather than general writer admission. [Git preparation][prepare] [Recurse lock and status][lock]

The journey document's sequential guarantee is explicitly about execution **inside** the loop. It cannot exclude user input outside it. Its claim that the ratchet touches exactly the attempt's paths needs an ownership precondition or a narrower current-state description; journaling and restart durability do not convert a time interval into authorship. [Journey guarantees][journey]

There is already substantial inner machinery, which a repair must preserve:

- `RecurseAttemptCampaign` creates one modifying task with `WriteSet: a.Node.Paths` and carries the finding's acceptance command when provided. A planned scope is not a receipt of actual authorship. [Attempt campaign][attempt-campaign]
- `spawnTask` requires an `ObservedTaskExecutor`, installs `WithWriteGuard`, uses `RunUntilVerified`, and records `ret.Writes`. This is not a system with no observed writes. [Observed task path][observed-task]
- The existing F4/C4 repair leases additional paths at write time and records writes. Its failed-attempt handling separately undoes recorded writes outside the snapshot, refuses unknown prior state or a changed resulting version, then handles the snapshot. These protections matter for campaign siblings. The prior external audit explicitly credits the landed rollback and write-time lease repairs; this addendum does not reopen that bounded finding. [Existing ownership repair][inner-writes] [Prior repair credit][f4]
- The lease manager holds a per-instance ownership map and is constructed for each orchestrator. The independent `/write` operation neither acquires that map nor returns its mutation as the attempt's write. [Lease map][lease-map] [Initialization][lease-init]

Those facts do not close the outer boundary. `RecurseExecutor` returns only `error`, while the outer collector reconstructs ownership from workspace state. Choosing a tracked path outside the inner snapshot and all attempt writes isolates that defect even if the inner machinery behaves correctly. The existing version comparison also must not be advertised as an atomic filesystem CAS against arbitrary external editors: an unlocked read followed by a restore can race. Preserve the repair while extending the ownership contract at its consumers.

This addendum concerns filesystem compensation only. It does not combine campaign context-seed EDB ownership, campaign-generation admission and journals, foreground slash-operation slots, or shutdown cancellation into a second finding.

## Proposed compensation contract

Before selecting bytes for commit, deletion, or restoration, the outer consumer needs host evidence binding an operation and attempt to a workspace epoch, canonical resource identity, prior state, resulting state, outcome, and any uncertainty. Preserve identity through the observed return and cancellation path; do not reduce it to `error` before the consumer decides what it may undo. Metadata alone does not supply the prior bytes required for restoration.

An owned mutation ledger is evidence, **not authority**. Every fresh effect or scope expansion still needs current admission. Stopping may restore only an owned version that remains the version the attempt left, under a mechanism that makes the conditional restoration real. Unknown attribution, a foreign writer, changed content, an incompatible workspace epoch, or missing restoration evidence means preserve the current file and derive a hold or inspection obligation. Do not relabel foreign bytes as the attempt's failed work.

The host should report identities, revisions, admission outcomes, conflicts, and IO results; Mangle should derive recovery and hold obligations. The model need not make another call and receives no new authority. Denied operations remain denied. Validation, campaign acceptance, and policy-derived completion remain separate from ownership. Opaque subprocess or remote effects cannot inherit a general rollback promise from a filesystem ledger.

Two implementation strategies deserve comparison rather than an assumed universal redesign:

**Cooperative writer admission plus owned receipts.** Serialize, queue, or visibly refuse mutating TUI, session, and campaign producers while an attempt owns the shared workspace. Keep read-only chat and status usable. Admission must latch before the mutation window, close before cancellation, join admitted writers, settle their effects, and hand ownership over only after cleanup. A local `m.recurse != nil` check cannot coordinate another process; the existing advisory recurse lock cannot stop an arbitrary editor. Under uncooperative writers, uncertain or changed versions must still be held rather than overwritten. Integration requires enumerating real producers, not only patching `/write`.

**A dedicated attempt working tree.** Bind an attempt worktree to an exact base, with a separate working directory, index, and `HEAD`. Route kernel/world-model state, CodeDOM, JIT context, write guards, tools, and command working directories to that identity; measurements and acceptance must certify the same snapshot. Promote only exact, owned results after current-authority and conflict checks against the destination revision. Never blindly apply a result to a different base. Cancellation should clean up only the attempt's own worktree. Git worktrees share repository objects and most refs, so they are not security sandboxes; shared metadata and promotion still need coordination. [Git worktree documentation](https://git-scm.com/docs/git-worktree)

Filtering only the planned `WriteSet` is insufficient: the existing repair deliberately accounts for legitimate out-of-plan writes, while a foreign writer can edit the same planned file. Rechecking cleanliness once still cannot prove later authorship. Removing rollback would abandon cancellation recovery rather than fix it. A receipt-only approach without enforceable ownership must conservatively hold conflicts, not claim atomic compensation.

## Existing tests and the missing interleaving

Read, but not run: `TestRecurseCycles_StoppedMidAttemptRevertsBeforeReturning` covers restoration of writes made by its executor fixture. `TestRecurseCycles_ResumeSettlesAKilledAttempt` deliberately lists the owner's preexisting untracked file in `UntrackedBefore`. The dirty-checkout test protects work present before admission. These are valuable controls, not evidence about a different writer admitted during the attempt. [Cycle tests][cycle-tests] [Dirty admission test][dirty-test]

`TestRecurseGit_ChangedKeepCommitRevert` exercises edits, deletion, creation, staging, preexisting untracked files, and ignored output; `WorkspaceInASubdirectory` protects paths outside the workspace. Their fixture mutations do not identify two concurrently admitted owners. Chat tests cover local cancellation and progress/completion handling, but not `/write` interleaved with an executing attempt. [Git tests][git-tests] [Chat tests][chat-tests]

The first proposed test should traverse the normal TUI production route with real Cortex/world-model and CodeDOM wiring, while substituting a deterministic observed task executor parked after the attempt snapshot. Use barriers, not timing sleeps: confirm attempt entry, submit `/write`, observe its exact successful bytes, submit `/recurse stop`, release the executor on cancellation, and join the callback and cleanup. Inspect exact file bytes, index state, journal/in-flight state, and owned receipts. A standalone collector test is useful but cannot certify this wiring.

The required result is that the foreign version survives unchanged, remains unattributed to the attempt, and is neither committed nor removed by it. Owned attempt mutations must still be restored or visibly held on conflict. Cover these dimensions in boundary tests before any separately authorized real-model evaluation:

| Cases | Required distinction |
|---|---|
| Tracked file outside node roots; new untracked file; preexisting untracked file | New owner writes survive; temporal absence is not authorship. |
| Ignored output; paths outside the configured workspace | Preserve existing exclusions without claiming they cover arbitrary effects. |
| Owned writes; a user changes a file after the attempt wrote it | Restore only an owned unchanged resulting version; preserve and report conflicts. |
| Denied write; Git failure; disk or receipt-persistence failure | No invented mutation or successful cleanup; retain explicit uncertainty and recovery evidence. |
| Second recurse; begin/stop race; canceled task draining | Correct admission, cancellation, join, lease release, and ownership handoff. |
| Case aliases and symlinks; restart with an in-flight record | Canonical identities and conservative recovery, not name-based ownership assumptions. |
| Keep, revert, refuse; stable and moved `HEAD` | No foreign adoption on any verdict; base changes invalidate stale promotion or restoration. |

## Cost and acceptance criteria

No loss rate, latency improvement, token saving, or implementation estimate has been measured. Account for each producer's integration work, workspace-identity and cache separation, lease failure and crash recovery, receipt durability and retention, and conflict handling. Hashing or reading affected content costs O(B) for B bytes per pass; per-resource receipt metadata costs O(N) for N observations, with retained prior bytes and repeated observations adding storage. Worktrees add checkout IO, disk use, potentially cold caches and extra build work. Serialization adds writer wait time and reduces overlapping throughput. Promotion conflicts can add retries and human review; avoided lost work is an unmeasured benefit, not a free offset.

Compare the current path, cooperative admission plus owned receipts, and the dedicated-worktree design at equal permissions, context, verification obligations, and task quality. Include successful, failed, and canceled attempts separately. Record wall time, CPU, IO bytes, disk growth, all model tokens, recovery turns, writer waiting, and human intervention; do not combine unlike units without explicit prices. Include no-mutation controls so a design that prevents all useful work cannot look successful merely because it preserves files.

The exit is a deterministic production-route ownership test plus preserved existing cancellation and admission behavior, followed only if separately authorized by bounded real coding evaluations with the same task snapshots and acceptance criteria. Update the owning campaign and chat specifications before implementation, and report source-derived expectations separately from executed results. The immediate recommendation is narrow: stop treating the outer recurse workspace diff as proof that the attempt owns every selected write.

[contract]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/Docs/audits/tool-effect-receipts-and-validation-completeness.md#L67-L122
[open-attribution]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/Docs/audits/tool-effect-receipts-and-validation-completeness.md#L205-L221
[executor]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/campaign/recurse_cycle.go#L86-L97
[commands]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/commands.go#L236-L239
[start]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/campaign_recurse.go#L120-L205
[lifecycle]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/campaign_recurse.go#L291-L312
[enter]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/model_key_handler.go#L247-L259
[submit]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/model_handlers.go#L104-L128
[admission]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/campaign/recurse_cycle.go#L245-L315
[cancel]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/campaign/recurse_cycle.go#L850-L894
[write-dispatch]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/commands.go#L145-L146
[write]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/commands_handlers_analysis.go#L350-L371
[write-helper]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/helpers_files.go#L317-L324
[stop-routing]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/commands_handlers.go#L662-L686
[local-stop]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/campaign_recurse.go#L254-L279
[collector]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/campaign/recurse_git.go#L79-L122
[filter]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/campaign/recurse_cycle.go#L1090-L1104
[revert]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/campaign/recurse_git.go#L182-L222
[ratchet]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/campaign/recurse_cycle.go#L896-L1019
[prepare]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/campaign/recurse_git.go#L50-L76
[lock]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/campaign/recurse_control.go#L12-L117
[journey]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/Docs/journeys/10-forever-loop.md#L59-L197
[attempt-campaign]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/campaign/recurse_attempt.go#L54-L132
[observed-task]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/campaign/orchestrator_task_handlers.go#L34-L90
[inner-writes]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/campaign/orchestrator_attempt_writes.go#L16-L213
[lease-map]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/campaign/write_set_lock_manager.go#L24-L38
[lease-init]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/campaign/orchestrator_init.go#L48-L70
[cycle-tests]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/campaign/recurse_cycle_test.go#L230-L307
[dirty-test]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/campaign/recurse_cycle_test.go#L387-L400
[git-tests]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/campaign/recurse_git_test.go#L81-L176
[chat-tests]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/campaign_recurse_test.go#L71-L113
[modal]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/model_key_handler.go#L29-L34
[f4]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/Docs/journeys/07-external-audit-2026-09-19.md#L415-L418
