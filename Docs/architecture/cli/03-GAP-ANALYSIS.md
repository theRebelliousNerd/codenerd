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

**Verified current:** `TestCampaignSlashPauseResumeControlsLiveOrchestrator` and `TestCampaignSlashPauseResumeDoNotClaimMissingTransitions` (`cmd/nerd/chat/campaign_control_test.go:70`, `:177`) pass. `TestEmbeddingReembedHonorsCommandCancellation` and `TestEmbeddingReembedTimeoutReachesStoreAndStopsLaterDatabaseWork` (`cmd/nerd/embedding_cmd_test.go:17`, `:50`) pass. Chat containment/forget regressions (`cmd/nerd/chat/chat_read_boundary_test.go:30`, `:69`, `:95`) and production articulation controls (`cmd/nerd/chat/process_control_roundtrip_test.go:114`, `:298`, `:349`) pass; `TestProcessTurnWithAppliedControlsRetainsMetadataWithoutReapplying` (`internal/context/compressor_applied_controls_test.go:12`) also passes. These are focused gates; aggregate CLI/corpus verification remains pending. GAP-CLI-01 stays partial because fresh production boot/shutdown timing and bounded-cleanup proof remain open; the inherited-idle-stdin refinement itself is implemented and focused-tested above.
