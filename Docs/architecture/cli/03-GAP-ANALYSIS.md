---
doc-class: shipped-with-future
subsystem: cli
implementation-status: partial
last-verified: 2026-10-02
verified-against: db1d4b7e24d0f9440c8627b2561ca64a25876954
supersedes: []
---

# CLI gap analysis

This matrix records accepted, source-grounded implementation obligations. It is an incremental census; the remaining CLI portfolio is still being audited and is not declared complete.

## Required contract: Local search evidence

The slash search and the residual natural-language search must carry the literal
query, workspace membership domain, matching file paths, result limit, acquisition
coverage, read-failure count and sampled diagnostic, and terminal error. A cap
stops acquisition only when another admitted file is encountered; it establishes
unknown remaining coverage, not a known omitted match count. A completed walk with
no failed reads may state absence within the admitted domain. Partial acquisition
preserves positive paths and cannot establish absence. Search paths are lexical
evidence, distinct from file bodies. The answering request receives the same
outcome the slash command displays; a no-file fallback must not contradict it.
Existing Mangle routing and successful independent execution results retain their
authority. Acquisition checks cancellation and streams file bytes; the result
limit does not promise a total byte or elapsed-time bound, and membership refresh
and individual filesystem operations remain synchronous. Incomplete searches
remain explicitly unresolved; no continuation handle or automatic retry is implied.

| Gap ID | Capability | Current deficiency | Target | Phase | Exit criteria |
|---|---|---|---|---|---|
| GAP-CLI-SEARCH-EVIDENCE | Local search coverage and answer delivery | Typed acquisition and both consumers are wired in `cmd/nerd/chat/search_evidence.go:20`, `commands_handlers_analysis.go:367`, `process_helpers.go:123`, and `turn_evidence.go:168`. | Required contract above | Implemented in source; root-run verification pending | `search_evidence_test.go`: slash boundary cases 0/19/20/21 and residual 0/9/10/11; admitted read failure and partial traversal preserve coverage and paths; actual `processInput` request contains a unique discovered path and coverage under a low-confidence gather route with system rescue absent; literal punctuation, membership, cancellation, and confident delegation remain correct. Tests are authored, not executed; this row remains open pending behavioral evidence. |

## Other CLI obligations

### GAP-CLI-PROGRESS-FIXTURE-JOIN

The offline chat progress fixtures own their transcript workers. Their release
callback must wait for `RunAgent` and the final transcript publication before
returning; test cleanup must perform the same idempotent join before removing
the store or temporary workspace. This is a test lifecycle requirement and
changes no production inference or transcript behavior.

Status: implemented in `cmd/nerd/chat/running_work_test.go:28`. The release and
cleanup callbacks join the worker after its final transcript. Unchanged baseline
`4de8c37` reproduced two temporary-directory cleanup failures in ten runs of
`TestSideTurn_AQuestionNeverTouchesTheBrief`; the renderer candidate reproduced
one in ten. `TestProgressModel_ReleaseJoinsFinalTranscript` (`:54`) also failed
before the join, observing unfinished progress immediately after release.
Exit: that regression and both existing progress controls pass 20 normal and
10 race repetitions. The complete offline suite is the final aggregate gate;
no tests or production recording paths are disabled to avoid cleanup failures.

The background job progress fixture (`cmd/nerd/chat/job_progress_test.go:29`)
has the same obligation: join `RunAgent` before removing the temporary session.
The aggregate gate exposed its unjoined release independently of the foreground
fixture; unchanged baseline reproduced six cleanup failures in 100 repetitions.
It now defers release and join together, and passes 100 repetitions. The related
delegated-progress fixture already joins its worker and retains that behavior.

| Gap ID | Capability | Current state | Target state | Severity | Phase | Blocking dependencies | Exit criteria |
|---|---|---|---|---|---|---|---|
| GAP-CLI-01 | Explicit headless-chat command deadline | **PARTIAL:** `cmd/nerd/cmd_chat.go#runChatWith` (`cmd/nerd/cmd_chat.go:168`) now derives commandContext before boot and shares cancellation/deadline through turns and idle input. Root filtered behavioral CLI gate passes: `artifact:.corpus-build/runs/all-features-20261002/round2-cli.log`. Baseline dogfood-stop.json records the former 616-second overrun. Fresh production boot/shutdown exit and bounded cleanup remain unqualified. | **PROPOSED UPLIFT:** [headless chat constraints](06-HEADLESS-CHAT-CONSTRAINTS.md) applies an explicit caller deadline to boot, turns, input waiting, and cancellation-aware shutdown while preserving unlimited duration when no deadline is selected. | High | Partial; lifecycle tests pass, production exit/cleanup open | CLI owner; shared shutdown or boot changes require separate ownership if necessary | Root-run behavioral regression proves explicit deadline and inherited cancellation reach boot/turn processing and prevent later turns; zero or negative timeout adds no deadline. Production `nerd chat --timeout` exits under the chosen bound plus documented bounded cleanup and independently preserves completed edits. |

The generic command helper already defines the intended positive/non-positive semantics: `cmd/nerd/operation_context.go#operationContext` (`cmd/nerd/operation_context.go:19`) and `#commandContext` (`cmd/nerd/operation_context.go:32`). Their existence does not prove headless chat uses them.

**Accepted GAP-CLI-01 refinement — inherited idle stdin:** on Linux, cancellation through `context.AfterFunc` and `Stdin.Close` left `syscall.Read` blocked when fd 0 was an inherited pipe whose peer remained open. The required behavior is a prompt typed context error with all command-owned reader work joined, without requiring the peer to close. Argument-driven input bypasses stdin; partial input, blank lines, EOF, and input errors retain their existing behavior. Do not detach a goroutine to abandon the blocked read, alter the peer's shared descriptor blocking flags, or treat a longer watchdog/skip as a fix. Current command wiring is `cmd/nerd/cmd_chat.go:168` (`runChatWith`); this refinement does not close GAP-CLI-01's separate boot/shutdown timing obligations.

**Verified current — inherited-idle-stdin refinement:** Unix `newChatInputReader` polls inherited file descriptors before synchronous reads, checks cancellation, and leaves the inherited file open (`cmd/nerd/cmd_chat_stdin_unix.go:17-73`). The production child-process regression `TestChatLifecycle_ProductionStdinIdleDeadline` (`cmd/nerd/cmd_chat_lifecycle_test.go:320`) passed three repeated runs, returning the deadline error under the unchanged watchdog. The focused lifecycle tests also passed for cancellation/join, deadline, partial and blank lines, and positional-argument bypass (`:253`, `:303`, `:354`, `:579`); the full chat lifecycle test group passed. Unix runtime behavior is verified; the fallback helper cross-compiles for Windows, where inherited-stdin cancellation behavior remains unverified.

Close a gap only after its exit evidence exists. Preserve the row and cite the closing revision and measured commands; source edits or a passing helper test alone cannot close the production deadline obligation.


## Required contract: Chat campaign runtime control

Slash campaign pause and resume must invoke the live orchestrator control and reflect its real progress state. A missing orchestrator or active campaign cannot claim a completed transition. Paused scheduling must start no new task until resume.

| Gap ID | Capability | Current state / deficiency | Target | Severity | Phase | Dependencies | Exit criteria |
|---|---|---|---|---|---|---|---|
| GAP-CLI-CAMPAIGN-PAUSE | Chat campaign runtime control | `handleCampaignCommand` now reads matching live orchestrator state, invokes `Pause`/`Resume`, and confirms the resulting status before syncing the chat view (`cmd/nerd/chat/commands_handlers.go:697-765`). | Required contract above | P1 | Implemented; focused tests passed; aggregate verification pending | Existing package interfaces | Slash-dispatch tests with real in-process orchestrator block task starts on pause, release on resume, and reject missing-orchestrator/no-active false success. |


## Required contract: Reembedding command context

Embedding reembed must derive its full operation from commandContext(cmd), refuse canceled context before engine boot, and propagate caller cancellation/global timeout through ReembedAllDBsForce as a nonzero command error.

| Gap ID | Capability | Current state / deficiency | Target | Severity | Phase | Dependencies | Exit criteria |
|---|---|---|---|---|---|---|---|
| GAP-CLI-REEMBED-CONTEXT | Reembedding command context | `runEmbeddingReembedWithOps` derives `commandContext` before initialization, passes it to engine creation and `ReembedAllDBsForce`, and joins close failures (`cmd/nerd/embedding_cmd.go:139-190`). | Required contract above | P1 | Implemented; focused tests passed; aggregate verification pending | Existing package interfaces | Canceled Cobra parent exits before initialization; positive command timeout reaches store operation and prevents later database work. |


## Required contract: Slash command payload integrity

`/write`, `/append` and `/patch` carry file content. Only the envelope (verb, one whitespace-free path token, one separator: space, tab, `\n` or `\r\n`) is parsed; the payload reaches the writer byte for byte. Content the text transport cannot carry (control characters other than tab, CR, LF) and an empty payload are refused before the file is opened. `/append` appends a line: the payload, then `\n` unless it already ends with one. Patch mode keeps every submission untrimmed (one trailing line ending dropped per submission), ends only on a whole submission of exactly `--END--`, and applies with the workspace as working directory. Replies state the bytes written or the backend's own failure output.

| Gap ID | Capability | Current state / deficiency | Target | Severity | Phase | Dependencies | Exit criteria |
|---|---|---|---|---|---|---|---|
| GAP-CLI-SLASH-PAYLOAD | Slash command payload integrity | `splitPayloadCommand`, `checkTextPayload`, `patchChunk`, `ingestPatchSubmission` (`cmd/nerd/chat/command_payload.go:31-105`); `handleSubmit` passes the untrimmed submission (`model_handlers.go:106-128`); `handleCommand` keeps raw for `/write`/`/append` (`commands.go:45`, `:146`, `:158`); `runApplyPatch` runs in the workspace and reports git/patch output (`helpers_files.go:71`). The bubbles v1 textarea still expands a typed or pasted tab to four spaces before submission (no public sanitizer option). | Required contract above | P1 | Implemented except tab transport through the textarea widget | bubbles textarea input layer | `cmd/nerd/chat/command_payload_test.go`: repeated spaces, JSON, trailing whitespace and multiline reach the file exactly through `handleSubmit`; tabs/CRLF exact at the dispatcher; control byte, empty and missing payload leave the file unchanged; append newline rule; line-by-line patch with context markers, blank context and trailing spaces applies in the workspace from another cwd; chunked and line-by-line ingestion agree. |

## Required contract: Conversation history at the compression handoff

Perception renders no compressed history, so it receives the whole conversation and its classification policy selects once (window, budget, and an omission notice that counts everything left out). Articulation, once compression is active, keeps the recent window; when the compressed view cannot be built, its compressed-history block states how many earlier messages are not in the context and why, instead of being silently empty.

| Gap ID | Capability | Current state / deficiency | Target | Severity | Phase | Dependencies | Exit criteria |
|---|---|---|---|---|---|---|---|
| GAP-CLI-HISTORY-HANDOFF | Conversation history coverage at compression | `processInput` passes all of `m.history` to perception; `articulationHistoryView` builds the articulation block or the coverage statement (`cmd/nerd/chat/process.go`, `cmd/nerd/chat/history_view.go`). Turns answered on the perception fast path or delegated are still not fed to the compressor. | Required contract above | P2 | Implemented for H1/H2; H3 (route-independent recording) open | Compressor feed in `processInput` | `TestPerception_GetsTheWholeConversationOnceCompressionIsActive` (20 of 20 prior messages reach perception with an active compressor); `TestArticulationHistoryView_AFailedBuildIsStatedNotSilent`. |

## Required contract: Session ownership of conversation memory

A session owns its transcript, its compressed memory, and the conversational facts the kernel holds for it (brief, `session_note`, held proposal, shard results for follow-ups). Selecting a session (`/new-session`, load, boot) saves the outgoing one synchronously, detaches its conversational facts in one transaction, resets the compressor to the selected id, and records the open session. A compressed row whose embedded session id differs from the requested one is refused; a compressor holding another session's memory is not stored under this id; a deferred save for a session no longer open writes nothing.

| Gap ID | Capability | Current state / deficiency | Target | Severity | Phase | Dependencies | Exit criteria |
|---|---|---|---|---|---|---|---|
| GAP-CLI-SESSION-OWNERSHIP | Session ownership of conversation memory | `selectedSession`, `detachConversation` (`cmd/nerd/chat/session_owner.go`); owner checks in `saveSessionState` and `hydrateCompressorForSession` (`session_persistence.go`); `/new-session` (`commands_handlers_files.go`). Same-session save generations are not fenced; JSON writes are not atomic across the three stores. | Required contract above | P1 | Implemented for M1, M2 and the cross-session part of M3 | Existing compressor and store | `cmd/nerd/chat/session_owner_test.go`: `/new-session` resets the compressor and detaches brief, note and held work after saving A; loading B detaches A; a row under B holding A's memory is refused; a save deferred under A that runs after the switch leaves A's newer history and does not store B's memory under A. |

## Required contract: Grounded knowledge synthesis

### Terminal answer ownership

The originating request ID must cross consultation and terminal synthesis on
success, provider error, cancellation and recovered panic. Final content must be
owned in raw history and the SQLite exchange independently of interim content,
queued submissions or unrelated output. Consultation does not settle the request
or count as a successful answer. The terminal result releases the consultation
guard; failure is retained as failure through the existing error path. A request
with identity zero remains unowned. No positional or text-based historical repair
is permitted.

| Gap ID | Capability | Current state / deficiency | Target | Severity | Phase | Dependencies | Exit criteria |
|---|---|---|---|---|---|---|---|
| GAP-CLI-KNOWLEDGE-ANSWER-IDENTITY | Owned terminal synthesis | Explicit request handoff in `request_answers.go`, `model_types.go`, `model_update.go` and `process_knowledge.go`; terminal result uses the normal owned assistant/error handlers. | Required contract above | P1 | Implemented in source; execution pending | Existing session projection and persistence | `knowledge_answer_identity_test.go` must prove final ownership through accepted input, consultation, Update, raw history, SQLite and `QuerySession`, including errors, cancellation, panic, queues and unrelated output. Root must establish fail-before and pass-after evidence; source edits alone do not close validation. |

### Grounding and consultation coverage

The request that writes the final answer after specialist consultations carries the answering request's grounding, frozen when the consultations start (recent conversation, workspace evidence rendered with its coverage, compressed history), and every consultation's outcome: answered, failed, returned nothing, or not run because of the per-turn cap, each with its query and purpose. First-call controls are not replayed and synthesis stays one model call.

| Gap ID | Capability | Current state / deficiency | Target | Severity | Phase | Dependencies | Exit criteria |
|---|---|---|---|---|---|---|---|
| GAP-CLI-KNOWLEDGE-GROUNDING | Grounded knowledge synthesis | `knowledgeParent`/`knowledgeParentOf`, `errKnowledgeNotRun`, `knowledgeSynthesisPrompt` (`cmd/nerd/chat/process_knowledge.go`); carried on `knowledgeGatheredMsg.Parent` from `processInput` to `synthesizeWithKnowledge`. Synthesis is not JIT-compiled for a terminal consumer role. | Required contract above | P2 | Implemented except the JIT terminal role | Prompt compiler consumer roles | `cmd/nerd/chat/knowledge_grounding_test.go`: a turn that reads `app.log` partly and asks a specialist sends the sentinel and PARTIAL coverage in the synthesis request; mixed success, failure and empty outcomes are all named; a third request is reported not run. |

## Required contract: Chat information read containment

The legacy /fs_read information-gathering action must enforce the same canonical workspace containment as typed read_file, rejecting traversal, outside absolute paths and symlink escapes before reading or asserting file_content. In-root reads remain supported.

| Gap ID | Capability | Current state / deficiency | Target | Severity | Phase | Dependencies | Exit criteria |
|---|---|---|---|---|---|---|---|
| GAP-CLI-CHAT-READ-CONTAINMENT | Chat information read containment | `readFileContent` resolves through `tools.ResolveWorkspacePath` before `os.ReadFile`, and `executeInfoGatheringActions` emits `file_content` only after a successful read (`cmd/nerd/chat/helpers_files.go:89-105`; `process_helpers.go:83-105`). | Required contract above | P1 | Implemented; focused tests passed; aggregate verification pending | Existing package interfaces | Production info-gathering tests deny outside absolute/traversal/symlink targets without file_content and admit in-root reads. |


## Required contract: Control packet application with knowledge requests

Knowledge consultation must not discard co-emitted memory operations, allowed Mangle observations or self-correction. Apply governed control effects exactly once before the knowledge branch and preserve them across synthesis; do not duplicate them in subsequent turns.

| Gap ID | Capability | Current state / deficiency | Target | Severity | Phase | Dependencies | Exit criteria |
|---|---|---|---|---|---|---|---|
| GAP-CLI-CONTROL-BEFORE-KNOWLEDGE | Control packet application with knowledge requests | `processInput` applies the governed articulation controls and records the sanitized packet before the knowledge-request return (`cmd/nerd/chat/process.go:896-935`); `ProcessTurnWithAppliedControls` retains that metadata without replaying effects (`internal/context/compressor_turns.go:33-43`). | Required contract above | P1 | Implemented; focused tests passed; aggregate verification pending | Existing package interfaces | Scripted production articulation with combined knowledge request and memory/observation controls retains governed effects once before specialist synthesis. |


## Required contract: Model observation trust boundary

Every model-authored Mangle update must pass ModelObservationPolicy before any kernel LoadFacts/Assert or compressor path. Model-authored permission grants, user intent, host witnesses and unsupported executive facts must never enter EDB or influence authorization. Candidate actions may enter only through their existing governed proposal path, never as authorization.

| Gap ID | Capability | Current state / deficiency | Target | Severity | Phase | Dependencies | Exit criteria |
|---|---|---|---|---|---|---|---|
| GAP-CLI-MODEL-OBSERVATION-BOUNDARY | Model observation trust boundary | `applyArticulationControlPacket` partitions candidate proposals, policy-filters ordinary model observations before assertion, and records only successfully governed facts (`cmd/nerd/chat/process.go:963-1014`; candidate proposal validation at `:1016-1045`). | Required contract above | P1 | Implemented; focused tests passed; aggregate verification pending | Existing package interfaces | Compressor-on production packet with safe observation and disallowed permitted/user_intent/host-witness admits only safe observation; querying base/derived facts confirms no injected authority or changed permission. |


## Required contract: Perceived forget memory scope

A perceived forget operation may retract only matching user/session memory key facts. It must never interpret a user/model key as a kernel predicate to retract. Preserve executive intent, permission and safety witnesses.

| Gap ID | Capability | Current state / deficiency | Target | Severity | Phase | Dependencies | Exit criteria |
|---|---|---|---|---|---|---|---|
| GAP-CLI-FORGET-MEMORY-SCOPE | Perceived forget memory scope | Chat forget now delegates to `memorycontext.ApplyMemoryOperation` through the scoped `MemoryKernel` interface (`cmd/nerd/chat/process_helpers.go:55-79`). | Required contract above | P1 | Implemented; focused tests passed; aggregate verification pending | Existing package interfaces | Forget requests naming permitted, user_intent and security_violation preserve all executive facts while deleting only the matching session-note key. |


## Required contract: Degraded delegated completion honesty

A delegated mutation requiring verification cannot be rendered as completed when no verifier is wired or its returned evidence is unverified. Refuse the unwired route or return an explicit unverified outcome while preserving observations and recovery.

| Gap ID | Capability | Current state / deficiency | Target | Severity | Phase | Dependencies | Exit criteria |
|---|---|---|---|---|---|---|---|
| GAP-CLI-UNVERIFIED-DELEGATION | Degraded delegated completion honesty | Mutation delegation refuses when the verification obligation or verifier is absent and reports no completion for an unsuccessful verdict (`cmd/nerd/chat/process.go:450-464`, `:497-513`). | Required contract above | P1 | Implemented; focused tests passed; aggregate verification pending | Existing package interfaces | Nil-verifier mutation and unverified outcome tests emit no completion header/checkmark and report the missing verification obligation. |

## Required contract: Autonomous self-repair

Specified in [08-AUTONOMOUS-SELF-REPAIR.md](08-AUTONOMOUS-SELF-REPAIR.md): with debug-self on, the kernel derives what is due, the chat starts the diagnosis-then-fix chain unasked, a self-checked rebuild replaces the binary, the restart resumes cut work, and the new process judges the fix.

| Gap ID | Capability | Current state / deficiency | Target | Severity | Phase | Dependencies | Exit criteria |
|---|---|---|---|---|---|---|---|
| GAP-CLI-SELF-REPAIR-01 | Derived trigger | `self_repair_due` (`internal/core/defaults/policy/self_repair.mg:237-238`); asked at turn boundaries and on an idle timer by `selfRepairStep` (`cmd/nerd/chat/self_repair.go:171`), never beside the user's work (`:155`). | Contract item 1, 3 | P1 | Implemented; focused tests passed | problemlog, kernel | TestSelfRepair_TheIdleTimerStartsTheChainUnasked, TestSelfRepair_NeverStartsBesideTheUsersWork (`cmd/nerd/chat/self_repair_test.go:81`, `:136`); TestSelfRepair_HotProblemsAreDue, TestSelfRepair_GatedByDebugSelfAndConfig (`internal/core/self_repair_policy_test.go:80`, `:100`). |
| GAP-CLI-SELF-REPAIR-02 | Optimize lane | `self_timing` rows from this process's audit log (`internal/problemlog/timing.go:69`); the optimize-due rule in `internal/core/defaults/policy/self_repair.mg` picks the costliest non-external recurring operation. | Contract item 2 | P2 | Implemented; focused tests passed | logging audit | TestSelfRepair_OptimizesTheCostliestOperationOnceNothingIsHot (`cmd/nerd/chat/self_repair_test.go:472`); TestSelfRepair_OptimizeTheCostliestRecurringOperation (`internal/core/self_repair_policy_test.go:275`). |
| GAP-CLI-SELF-REPAIR-03 | Attempt lifecycle across the restart | `RepairLedger` (`internal/problemlog/repair.go:94`) persists attempts; verdicts derived and adopted by `adoptSelfRepairVerdicts` (`cmd/nerd/chat/self_repair_plan.go:250`). | Contract item 4 | P1 | Implemented; focused tests passed | selfrestart | TestSelfRepair_ARecurringProblemRegressesAndTheRetryIsToldSo (`cmd/nerd/chat/self_repair_test.go:323`); TestSelfRepair_VerdictsAfterTheRestart, TestSelfRepair_HeldTargetsAndTheSlot (`internal/core/self_repair_policy_test.go:190`, `:117`). |
| GAP-CLI-SELF-REPAIR-04 | Self-checked install and resuming restart | `rebuildAndInstall` (`cmd/nerd/chat/self_restart.go:460`) builds aside, self-checks, then renames; `resumeRestoredWork` (`:390`) restarts `State.Resume` work (`internal/selfrestart/selfrestart.go:125`). | Contract items 5, 6 | P1 | Implemented; focused tests passed | build | TestSelfRepair_ABrokenBuildNeverReplacesTheRunningBinary, TestSelfCheckBinary_ABinaryThatDoesNotStartIsRefused (`cmd/nerd/chat/self_repair_test.go:358`, `:386`); TestWriteState_ReadState_CarriesResumedWork (`internal/selfrestart/resume_test.go:13`). |
| GAP-CLI-SELF-REPAIR-05 | Safety deny during a fix turn | `self_repair_fixing()` turns `self_repair_protected_path` into `project_forbidden_path` (`internal/core/defaults/policy/self_repair.mg:300-324`). | Contract item 7 | P0 | Implemented; focused tests passed | projectdoc gate | TestSelfRepair_FixTurnNeverTouchesTheSafetyGate (`internal/core/self_repair_policy_test.go:331`). |
| GAP-CLI-SELF-REPAIR-06 | Live proof | Live 2026-11-12: trigger, in-place restart, unasked diagnosis, verdict and fix turn all ran in a real yolo session; no fix has yet ended verified and installed. The whole walk is proven only with fakes (TestSelfRepair_WalksTriggerJobFixRebuildRestoreResumeVerify, `cmd/nerd/chat/self_repair_test.go:197`). | A live yolo session repairs a real hot problem, re-execs, resumes and records `verified`. | P1 | Open | Live model | `.nerd/self_repair/ledger.json` of a live session holds a `verified` attempt whose build differs from the one that logged the problem. |
| GAP-CLI-SELF-REPAIR-07 | Verdict before the fix | Jev judges a finished diagnosis per target (`JudgeDiagnosis`, `internal/turnjudge/verdict.go:56`). The kernel rules in `internal/core/defaults/policy/self_diagnosis.mg:58` and `:67-68` (no-defect and fix-due) decide whether the fix turn runs; the fix is a chain step that is never re-perceived or decomposed (`cmd/nerd/chat/self_diagnosis_verdict.go:280`). Closes the live defect where a correct "nothing to change" diagnosis ran a fix turn that failed as a hollow success. | Contract items 7, 8 | P1 | Implemented; focused tests passed; not yet proven in a live session | Jev | TestSelfRepair_NoDefectVerdictStartsNoFixTurn, TestSelfRepair_NoVerdictRunsTheFixTurnAsBefore, TestChainStep_IsOneDelegatedFixNotADecomposedPlan (`cmd/nerd/chat/self_diagnosis_verdict_test.go:88`, `:152`, `:224`); TestSelfDiagnosisVerdict_TruthTable, TestSelfRepair_NoDefectIsHeldUntilTheEvidenceGrows (`internal/core/self_diagnosis_verdict_policy_test.go:80`, `:123`). |
| GAP-CLI-SELF-REPAIR-08 | Yielding a running fix turn to the user | A queued autonomous step gives way to the user's queued messages (TestTurnQueue_AnAutonomousStepYieldsToTheUser, `cmd/nerd/chat/self_diagnosis_verdict_test.go:263`), but a fix turn already running holds the turn slot until it ends or is stopped. | A running self-repair turn pauses at a safe boundary between tool calls when the user sends a turn, and resumes after it. | P2 | Open | Executor pause point (session) | A test in which a user turn submitted during a self-repair fix turn is answered before that fix turn completes, and the fix turn then finishes with its edits intact. |
| GAP-CLI-SELF-REPAIR-09 | A fix that is not installed is undone; its record is not a target | Observed live 2026-11-12: a failed fix turn left two modified files and a red test in the tree, and its own log lines became four new repair targets. `settleFixTurn` (`cmd/nerd/chat/self_repair_fix_turn.go:98`) now restores the fix's own writes from a write journal (`internal/session/write_journal.go:53`) and saves the attempt; the kernel rule at `internal/core/defaults/policy/self_repair.mg:186` holds problems logged only inside a fix window. Deficiency left: the journal and an open window do not survive a crash mid-fix. | Contract items 10, 11 | P1 | Implemented for a settled chain; crash recovery open | session write journal | TestSelfRepair_AFailedFixTurnUndoesOnlyItsOwnWritesAndTheRetryIsToldWhy, TestHandleTurnError_AVerificationVerdictIsNotAProblem (`cmd/nerd/chat/self_repair_fix_turn_test.go:100`, `:248`); TestSelfRepair_ProblemsLoggedInsideAFixTurnAreTheAttemptsNotDue (`internal/core/self_repair_exhaust_policy_test.go:21`). Exit for the open part: a fix chain killed mid-turn is undone by the next process. |
| GAP-CLI-SELF-REPAIR-10 | A fix chain converges or is cut | Timeline of the third live chain (2026-11-12, 19:06-20:02, optimize lane, `timing:kernel.Query`; session log of that process): 7 min lost to provider 503s with step 1 making no edit; 23 min and 1.55M input tokens on step 2 writing tests for the change step 1 never made; 7 min for the change itself once re-planned; 5 min for one repair round; 10 min for two Mangle specialist reviews of a Go-only change plus an uplift round that broke the tests and was reverted; 9+ min in the pinning gate and a pinning repair, unfinished when stopped. Built: the plan halts at a step that errored and wrote nothing (`internal/core/defaults/policy/turn_steps.mg:97`, `stepPlanHalt`, `internal/session/work_steps.go:508`), and the chain has a derived wall-clock budget (`internal/core/defaults/policy/self_repair.mg:286`). Also built (2026-11-12 sprint): the post-edit specialist review is dealt by what the change touches -- written files' extensions and the imports the diff adds, removes or names on a changed line (`internal/core/defaults/policy/shard_experts.mg`); a consult that failed on a provider error is not asked again in the same turn; an import change whose edges are already reachable through the package, or a new inert declaration, keeps the importer test selection (`internal/session/rts_imports.go`); a pin verdict is remembered for the tree it was measured on (`internal/session/pin_gate.go`); a failure the rerun cleared as flaky is not charged to the turn in gate scheduling or the uplift recheck (`internal/session/test_baseline.go:170`). Not built: pre-edit step consults are still dealt by the file's imports; for the live case the review still keeps the Mangle reviewers, because that change added a Mangle-library import and calls it on changed lines; the pinning gate and its repair have no share of the chain's budget, and each mutant is its own test build; a change whose own package is green cannot be parked as a candidate and finished later; a change that only alters performance cannot be pinned by a test, yet the gate still asks for one. | A correct, tested change in a central package is installed within the budget, or is stopped, undone and retried with what was learned. | P1 | Partial | session gates, shard dealing | TestRunPlannedSteps_AnErroredStepThatWroteNothingHaltsThePlan (`internal/session/work_steps_halt_test.go:38`); TestSelfRepair_AnOpenFixChainPastItsBudgetIsOverBudget (`internal/core/self_repair_budget_policy_test.go:15`); TestSelfRepair_AFixChainOverBudgetIsUndoneAndRecordedSo (`cmd/nerd/chat/self_repair_budget_test.go:53`). Exit for the open part: a live yolo session installs a fix to `internal/core` inside the default budget. |
| GAP-CLI-SELF-REPAIR-11 | The look off the Update thread | Observed live: the synchronous look took 238 ms and 523 ms on the Update thread, with a Jev network call inside it. Now three phases (`cmd/nerd/chat/self_repair_plan.go:117`, `:206`); boot completion, onboarding and session opening also moved their disk work into commands. | Contract item 13 | P2 | Implemented; focused tests passed; not yet observed in a live session | none | TestSelfRepairPlan_TheTriageNeverRunsOnTheUpdateThread, TestSelfRepairPlan_DroppedWhenTheUserBecameBusy, TestSelfRepairPlan_NeverTwoInFlight (`cmd/nerd/chat/self_repair_plan_test.go:92`, `:125`, `:157`); TestBootComplete_StaysUnder100msAndDefersSlowWork (`cmd/nerd/chat/boot_deferred_test.go:65`). |
| GAP-CLI-SELF-DIAGNOSE-ROUTE | Untargeted "debug yourself" | `verb_target_implied(/self_diagnose)` (`internal/core/defaults/policy/self_diagnosis.mg:18`) exempts the verb from target clarification; `/diagnose_self` is a `valid_action_type` (`internal/core/defaults/schema/intent_routing.mg:79`). | An untargeted self-diagnosis reaches the chain, not a deliberation. | P1 | Implemented; focused tests passed | routing | TestSelfDiagnosis_AnUntargetedTurnReachesTheChain, TestUnderstandingVocabulary_EveryOfferedActionIsKnownToTheKernel (`cmd/nerd/chat/self_diagnosis_route_test.go:21`, `:53`). |

**Built 2026-11-13 toward GAP-CLI-SELF-REPAIR-10:** a chain stopped by its budget with no failing test and its last own test run passing is parked, not failed: the change is undone, a candidate is kept under `.nerd/self_repair/parked/` (each path's after-content, the hash of the tree it applies to, which gates passed) and the ledger status is `/parked` (`parkCandidate`, `cmd/nerd/chat/self_repair_fix_turn.go:302`). `self_repair_finish_due` (`internal/core/defaults/policy/self_repair.mg:211`) precedes every new repair or optimize target: on a matching tree the change is re-applied and only the self-rebuild runs (`finishParkedCandidate`, `:342`); on a tree that moved the candidate is dropped as stale and named. The pinning gate has an attempt cap and a share of the turn, and retrieved knowledge a share of the prompt (session and prompt specs). A chain also has a token budget: the chat measures what the session has spent since the chain began (`self_repair_fix_spent`), and `self_repair_fix_budget_spent(Job, /time | /tokens)` (`internal/core/defaults/policy/self_repair.mg`) derives which budget it ran past against `logging.self_repair_fix_budget_tokens` (default 3,000,000, input plus output); the ledger note and the transcript name that budget (`fixBudgetPhrase`, `cmd/nerd/chat/self_repair_budget.go`). A session with no usage tracking asserts no spend and only the time budget applies (`TestSelfRepair_AnOpenFixChainPastItsTokenBudgetIsOverBudget`, `TestSelfRepair_ATokenBudgetVerdictIsRecordedAsTokens`). The default is a guess: the one chain measured spent 1.55M input tokens on a single wasted step. Not observed live: codeNERD was not run after these changes.

**Verified current:** `TestCampaignSlashPauseResumeControlsLiveOrchestrator` and `TestCampaignSlashPauseResumeDoNotClaimMissingTransitions` (`cmd/nerd/chat/campaign_control_test.go:70`, `:177`) pass. `TestEmbeddingReembedHonorsCommandCancellation` and `TestEmbeddingReembedTimeoutReachesStoreAndStopsLaterDatabaseWork` (`cmd/nerd/embedding_cmd_test.go:17`, `:50`) pass. Chat containment/forget regressions (`cmd/nerd/chat/chat_read_boundary_test.go:30`, `:69`, `:95`) and production articulation controls (`cmd/nerd/chat/process_control_roundtrip_test.go:114`, `:298`, `:349`) pass; `TestProcessTurnWithAppliedControlsRetainsMetadataWithoutReapplying` (`internal/context/compressor_applied_controls_test.go:12`) also passes. These are focused gates; aggregate CLI/corpus verification remains pending. GAP-CLI-01 stays partial because fresh production boot/shutdown timing and bounded-cleanup proof remain open; the inherited-idle-stdin refinement itself is implemented and focused-tested above.
