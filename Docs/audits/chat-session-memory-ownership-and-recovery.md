---
doc-class: deep-dive
subsystem: chat-session-context-store
implementation-status: planned
last-verified: 2026-10-28
verified-against: c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c
verification-method: static-source-inspection
supersedes: []
---

# Chat session memory ownership and coherent recovery

## Decision

Make the selected conversation own its working memory and its saved checkpoint. A new transcript must not quietly inherit the previous conversation's compressor; a loaded transcript must not combine its memory with another session's live facts; and deferred saves must carry immutable, owner-checked state rather than reading whichever compressor happens to be active later.

This is a lifecycle repair, not another summary model or memory service. Preserve the existing bounded context, explicit session loading, asynchronous persistence, and workspace knowledge. Extend their ownership contract. Mangle should determine which session/work memories are applicable; Go should establish identities, snapshot and validate records, perform IO, and publish a coherent transition.

**Evidence status:** source inspection only. No tests, builds, model calls, benchmarks, failure injection, or repository writes were performed. The sequences below are proposed deterministic witnesses, not reproduced user incidents. A wrong-scope fact being available does not prove a model used it or an unauthorized effect occurred.

## 1. Source freeze and non-overlap

The inspected heads were main [`356f65e0e6bb639adaa90ab36212daa714114871`][main] and feature [`c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c`][feature], with feature 27 commits ahead and zero behind. Research was [`c4539f041ddee0ce15cbda02771af7422cdf053e`][research]. These were resolved at approximately 10:39 UTC. Main already contains the session/compressor persistence mechanisms examined here; the feature adds the conversation brief and held work, increasing the consequences of incomplete session switching. Source links below use the feature pin unless explicitly marked main.

Main and feature were re-resolved unchanged at approximately 10:49 UTC. Research had advanced to `62e90b2836b0afe15cee49b05950ff1fa97d20d3`; its compare lists only a verification-audit document change. That delta does not change the inspected implementation, and this note does not claim a fresh review of that document's contents.

The root [North Star][guidance] requires policy-managed memory, recoverable eviction and long-horizon context without prompt drift. The [architecture standard][standard] requires grounded current-state claims, distinct questions per document and checkable proposed exits.

This document asks **which conversation a restored or retained memory belongs to, and whether its checkpoint is coherent**. It does not repeat:

- [Conversation brief semantic fidelity and progressive disclosure](conversation-brief-semantic-fidelity-and-progressive-disclosure.md): item-level meaning, supersession, work applicability and clipping within a brief.
- [Side-turn ordering and cancellation ownership](side-turn-ordering-and-cancellation-ownership.md): concurrent message classification, brief publication and cancellation targets. Its in-memory ownership proposal explicitly does not establish restart durability.
- [Background outcome and continuation ownership](background-outcome-and-continuation-ownership.md): request-owned background results, cancellation settlement and continuation targeting. Here the primary witness is a sequential session command with no running background job.
- [Articulation admission and control fidelity](articulation-admission-and-control-fidelity.md): whether a requested memory/control operation really applied. A correctly applied note can still be exposed in the wrong session.
- [Perception restriction continuity](perception-restriction-continuity.md): permission and restriction transport. Restoring conversation state must not mint permissions or silently reauthorize work.

## 2. The current user journey, accurately bounded

| User action | What the inspected path does |
|---|---|
| Start the application | `hydrateNerdState` intentionally creates a fresh session, and offers `/sessions` when prior histories exist. It does **not** automatically resume the previous conversation. [`cmd/nerd/chat/session_boot_helpers.go:113–132`][fresh-boot] |
| Start `/new-session` | `handleCmdNewSession` clears visible history, replaces the model's session ID, resets its turn count and saves. It does not reset/rebind the compressor. [`cmd/nerd/chat/commands_handlers_files.go:121–136`][new-session] |
| Load a previous session | `loadSelectedSession` saves the outgoing view, loads the chosen JSON history, changes the model ID, estimates turns from message count and hydrates the compressor. [`cmd/nerd/chat/session_persistence.go:185–228`][select] |
| Hydrate compressed memory | `hydrateCompressorForSession` resets the compressor, sets the requested ID, loads the latest SQLite state and calls `LoadState`. It reports parse/load errors in logging, and raises the UI turn counter when the stored turn is ahead. [`cmd/nerd/chat/session_persistence.go:107–145`][hydrate] |
| Persist a normal UI update | `saveSessionStateCmd` copies the history slice and defers a save using a shallow model copy. The compressor remains a shared pointer. [`cmd/nerd/chat/model_update.go:1112–1125`][deferred] |

Several good mechanisms are already present and must not be described as missing. `GetState` and `LoadState` copy the compressor's internal slices; load recalculates its budget. Session history validates the session filename against traversal. Compression writes warn on failure rather than silently discarding it. These are valuable controls, but not a joined owner/checkpoint invariant. [`internal/context/compressor_metrics.go:161–245,247–324`][state-copy], [`internal/session/persistence.go:89–102`][history-path], [`internal/context/compressor_turns.go:221–235`][compressor-save]

The context wiring document already corrects its former claim that chat compression was uncalled. This audit credits the live implementation; it is not a proposal to wire a nonexistent compressor. [`Docs/architecture/context/WIRING-AND-NOT-BUILT.md`, “Wired and reachable”][context-wiring]

There is also a real transactional cleanup path: `HydrateSessionContext` replaces its whole `session_turn`, `similar_content` and `reasoning_trace` predicate set, including a fresh partial snapshot with a warning when a query fails. Chat calls it on the normal turn path. The findings below do **not** claim that all context accumulates forever or that no session cleanup exists. That hydrator does not own `session_note`, brief predicates, held work or the compressor's summary. [`internal/core/virtual_store_predicates.go:673–760`][dynamic-hydrate], [`cmd/nerd/chat/process.go:790–800`][dynamic-caller]

## 3. Finding M1: a new session can save the previous session's compressed identity

`handleCmdNewSession` assigns a fresh `m.sessionID` and zeroes `m.turnCount`, but leaves `m.compressor` unchanged. With both compressor and local DB wired, its immediate `saveSessionState` obtains the compressor's state and attempts to store it under **the model's** session ID. The embedded state still contains **the compressor's** session ID and old turns. The store accepts the outer ID and serialized payload independently. [`cmd/nerd/chat/commands_handlers_files.go:121–136`][new-session], [`cmd/nerd/chat/session_persistence.go:82–94`][save-compressed], [`internal/context/compressor_metrics.go:186–196`][state-identity], [`internal/store/local_session.go:182–206`][store]

This exists on main as well: the [main command implementation][main-new-session] has the same transition. No side turn, model error, timing race or crash is required.

Proposed sequential witness, with compressor/local DB wired and the relevant serialization and writes succeeding:

1. Session A has a compressor containing A's identity, recent turns and a rolling summary.
2. Invoke `/new-session`; the displayed conversation becomes B with a new-session notice.
3. The handler saves a compressed row keyed by B whose serialized `SessionID` and memories still belong to A.
4. Follow a B turn that reaches the articulation path while those old compressed segments remain. `IsCompressionActive` remains true, so a successful context build can supply A's compressed history to that articulation request. This is not a claim about every B route or every historical fact surviving rendering. [`internal/context/compressor_metrics.go:105–125`][active], [`cmd/nerd/chat/process.go:884–900`][chat-context]
5. Without a later save replacing or outranking that row, restart and explicitly load B. Hydration selects rows under B, but `LoadState` assigns the embedded state ID to the compressor without comparing it to the requested B. The outer UI identity and compressor identity can diverge again. [`hydrateCompressorForSession`][hydrate], [`LoadState`, lines 260–280][restore]

The UI says “Previous history saved,” but this handler does not first save the outgoing model before clearing its history. Previous asynchronous or completed saves may already contain it; the source does not establish that every prior message is lost. The precise missing guarantee is that this command has durably captured the outgoing conversation before promising it. [`handleCmdNewSession`][new-session]

**Required exit:** starting B produces an empty B-owned conversational working state while preserving intentional workspace-level knowledge; every persisted B record agrees on B's identity. No statement about a saved outgoing history should precede a known durable or explicitly qualified result.

## 4. Finding M2: loading a transcript does not replace its session-owned kernel memory

The load path does reset the compressor. That is an important distinction from `/new-session`. However, `Compressor.Reset` clears Go-side turns, summary, activation state, budget and ID; it does not retract kernel facts. `LoadState` then **adds** its `HotFacts` to the existing kernel. Neither operation establishes a selected-session fact partition. [`internal/context/compressor_metrics.go:260–344`][restore]

The existing short-term note representation is `session_note(Key, Value)`, with no session argument, and the policy makes it relevant without checking an active session. This is not merely a misleading name: the schema explicitly describes it as a note for the current session. [`internal/core/defaults/schemas_context.mg:40–46`][note-schema], [`internal/core/defaults/policy/context_compilation.mg:49–53`][note-policy]

Proposed sequential witness:

1. In A, record a note that A's current focus is the authentication module.
2. Load B, whose history concerns an unrelated parser and whose compressed snapshot is absent or contains no replacement note.
3. The compressor reset succeeds, but the A note remains in the shared kernel and remains eligible under the note-relevance rule.

If B's snapshot contains another value for the same note key, additive `HotFacts` restoration does not run the normal note replacement operation. Two differing values may coexist; exact-fact deduplication is not key-level supersession. This is a source-derived possibility, not a claim about every snapshot's contents. [`LoadState`, lines 282–311][restore], [`recordSessionNote`, `internal/context/memory_ops.go:133–175`][note-write]

### Feature-specific consequence: a loaded history can retain the old work brief

`currentBrief` reads all brief predicates without a session selector. `loadSelectedSession` does not clear those predicates or the model's held-work field. `/go` prefers `m.pendingWork`; otherwise it can use the current brief's goal. Consequently, a switch to B can retain A's proposed work or brief as the next go-ahead target. This establishes the selected input path, not guaranteed execution or a permission bypass: normal route/work-mode and effect gates are still consulted. For held work, the existing intent is reused without fresh perception; the source explicitly skips perception and Jev for that go-ahead. [`cmd/nerd/chat/work_hold.go:79–121`][go-brief], [`loadSelectedSession`][select], [`cmd/nerd/chat/process.go:168–201`][reuse-intent]

The same lifecycle omission is not a claim that every brief is necessarily durable after a cold restart. There is no explicit brief field in `CompressedState`; its generic `HotFacts` are selected using activation and a token allowance. A brief might be partially represented there or reconstructed later. A hot-context cache therefore cannot serve as proof that a complete accepted brief was durably recorded. [`internal/context/types.go:419–439`][state-type], [`buildStateLocked`, `internal/context/compressor_metrics.go:168–196`][state-identity]

**Required exit:** a session transition installs that session's conversational projection rather than unioning it with the previous one. Notes, brief and held proposal must either be scoped to their owner or explicitly detached. Keep workspace knowledge and current policy separate; clearing the entire kernel is not a safe substitute.

## 5. Finding M3: a copied history is not a coherent deferred checkpoint

`saveSessionStateCmd` correctly copies the history slice, preventing later append/mutation from changing that slice. It does not capture the compressor state at the same boundary. The deferred closure later calls `saveSessionState`, which reads the then-live compressor through the copied pointer. [`cmd/nerd/chat/model_update.go:1112–1125`][deferred], [`cmd/nerd/chat/session_persistence.go:82–94`][save-compressed]

Proposed controlled schedule, independent of M1, with compressor/local DB wired, successful relevant writes and no later replacement before inspection:

1. Under A, create a deferred persistence command, holding A's history/ID.
2. Before executing that command, successfully load B and rehydrate the shared compressor with B's state.
3. Execute the saved A command. It can write A's frozen history alongside B's current compressed payload indexed under A.
4. Choose B's stored turn greater than A's existing highest turn. Since `LoadLatestCompressedState` orders by turn number, a later explicit load of A selects the wrong-owner row if it still has the highest stored turn. [`internal/store/local_session.go:209–240`][load-latest]

This is a logical ownership failure even if all involved memory accesses are protected by locks. A race detector alone cannot establish snapshot consistency.

There is also no revision fence for older deferred history saves completing after newer saves of the same session. JSON persistence uses `os.WriteFile`, and session metadata, history and compressed state are separate writes. Consequently the source supplies no single committed generation linking them, nor atomic replacement of each JSON file. A crash or write failure can leave partial generations. This document does not claim a measured corruption rate, or that SQLite transactions themselves are broken. [`internal/session/persistence.go:74–86,105–133`][json-write], [`saveSessionState`, lines 45–104][save-all]

A delayed A save can also rewrite the persisted `.nerd/session.json` metadata after B was selected. It does not change the live model's `m.sessionID`, and fresh boot intentionally creates a new session rather than using that file to auto-select A. The finding is stale persisted metadata, not an automatic live switch or restart-resume claim. [`saveSessionState`, lines 45–61][save-all], [`hydrateNerdState`, lines 113–132][fresh-boot]

The compressed decoder accepts JSON into the current struct; hydration does not compare requested/outer/embedded session identity, schema compatibility, turn coverage or history digest. A version string exists in the state, but its presence is not validation. [`internal/context/serializer.go:610–616`][decode], [`hydrateCompressorForSession`][hydrate], [`CompressedState`][state-type]

**Required exit:** persisted projections must bind the same owner and source frontier, and stale callbacks must not regress the committed checkpoint or capture a successor's live state. Corrupt, incompatible, mismatched and incomplete snapshots must have explicit restore dispositions.

## 6. Smallest coherent recovery contract

The following is a proposed contract, not an existing API or schema.

### Separate source, projection and activation

- **Conversation source:** ordered immutable message/event identities and source content, retained under the conversation owner. UI notices are not user/assistant turn pairs. Accepted brief-item history can reuse the source-backed design in the brief study.
- **Derived projection:** compressor summary, accepted brief view and short-term notes, with schema/projection version and a source frontier identifying what they cover. A missing cache can be rebuilt; a missing accepted obligation cannot be silently invented.
- **Active projection:** policy-selected memory for one session/work context. Historical summaries are context evidence, not current permissions, current tool observations, or authorization to replay a pending action.

Preserve exact source identity independently of token-oriented `HotFacts` selection. Restoration should check owner, format, completeness and source coverage **before** mutating the active view. Replaying old control packets to rebuild a summary must not repeat their effects; the existing no-reapplication compressor route is a useful boundary, not a complete recovery protocol. [`ProcessTurnWithAppliedControls`, `internal/context/compressor_turns.go:33–43`][no-replay]

### Make selection one coordinated transition

Prepare the target session off the UI event loop. Validate it, establish the requested disposition for outgoing work and outstanding writes, then publish the selected identity and its active memory together. A failed preparation should leave the old selection coherent rather than half-switching. For missing/corrupt projections, distinguish “transcript loaded; memory rebuilding” from a fully restored session.

Do not automatically resume pending effects after restart. A previously held proposal is historical work that requires its current owner and normal approval/admission conditions. Unknown prior effects require reconciliation through existing effect evidence, not a fresh execution merely because a transcript says work was requested.

### Capture, order and publish checkpoints

Capture an immutable checkpoint payload with one session identity, source frontier, projection version and generation before dispatching its IO. Keep expensive serialization and storage off the UI thread, but do not defer the *choice of what state belongs to the save*. Serialize or revision-fence writes per session. An old save may remain a historical generation; it must not replace the latest committed view or regress persisted selected-session metadata after a newer transition.

A single transactional store with a small manifest is one option. Retaining JSON plus SQLite is also possible with immutable generation files and an atomically published manifest describing which components committed. Merely using atomic rename for each JSON file does not make the three-store checkpoint atomic. State the crash-consistency guarantee and the handling of partial commits rather than claiming generic exactly-once persistence.

Surface durable progress and degraded restore status at meaningful boundaries. Do not show an error for every optional cache miss, but do not call an incomplete memory restore complete. Keep diagnosis details out of every prompt; expose a compact policy-readable status and recoverable source references.

## 7. Verification program, not executed results

Existing inspected tests check file creation without `/init`, nonblocking command construction and the no-op completion message. `TestSaveSessionStateCmd_HistorySnapshotIsolated` appends to the source history and checks command completion; it does not read persisted bytes or assert an exact immutable checkpoint. The slice-isolation deduction here rests on the explicit copy in `saveSessionStateCmd`, not that test's name. These are useful positive controls, not evidence for owner consistency: the nonblocking fixture explicitly runs without a compressor or local DB. [`TestSaveSessionState_WritesWithoutInit`, `cmd/nerd/chat/session_persistence_test.go:10–31`][save-test], [`cmd/nerd/chat/model_update_persist_test.go:43–164`][async-tests], [`saveSessionStateCmd`][deferred]

Future deterministic tests should use the real command, store and hydration paths with a wired compressor and kernel:

| Journey | Required observation |
|---|---|
| A has compressed turns → `/new-session` B | UI/model/compressor/store all name B; no A summary, note or held work is active in B. |
| A → load B with no checkpoint | B transcript loads with explicit projection status; A session facts are not inherited. |
| A and B use the same note key | The selected projection has the selected session's value; no accidental union or cross-session replacement. |
| Feature A has brief/held proposal → load B → `/go` | B cannot silently approve A's target; any deliberate transfer is explicit. |
| Capture A save → select B → release A save | The save contains only A's captured state and cannot regress persisted selected-session metadata; live selection remains B. |
| Same-session newer save completes before older save | Committed frontier remains monotonic; both transcript and summary agree on the selected generation. |
| Failure at each metadata/history/compressed-state write | Recover the last coherent checkpoint or expose an incomplete state, never silently join unrelated pieces. |
| Wrong embedded ID, unsupported version, malformed JSON, missing source frontier | No partial activation; explicit rejected/degraded disposition. |
| Fresh boot then explicit resume | Boot stays fresh by design; selecting the previous conversation restores the accepted memory attributable to it. |
| Resume after source code or policy changed | Historical conversation survives; stale observations and grants are not promoted to current authority. |

Capture the **actual next perception, articulation and delegated-worker requests**, not only the viewport or kernel query. Pair these with semantic dialogues containing task switches, literal requirements, corrections, answered questions and deliberately absent memories. Test both retention and inappropriate resurrection. The existing brief semantic-fidelity evaluation supplies the language-quality oracle; this suite supplies the lifecycle transitions.

## 8. Cost, alternatives and checkable handoff

Do not restore UI stalls by doing all persistence synchronously. Compare immutable asynchronous snapshots, a per-session serialized writer, and a transactional event/projection store. Measure UI latency, checkpoint latency, copied bytes, write amplification, retained generations, recovery time, prompt/rebuild calls, memory contamination, lost accepted items and false claims of successful restore.

The current saver rewrites full history and iterates its message pairs on each save, so adding a new durable ledger should also account for existing work rather than merely adding another copy. Incremental source persistence may reduce write amplification, but it introduces frontier/index/migration complexity. No performance improvement is claimed from this static audit. [`saveSessionState`, `syncSessionToSQLite`, `cmd/nerd/chat/session_persistence.go:64–76,147–182`][save-all]

Candidate gap rows for the owning chat, session, context and store specs:

| Proposed capability | Checkable exit |
|---|---|
| Session transition ownership | `/new-session` and load fixtures show one consistent owner across visible history, compressed state and active conversational memory. |
| Coherent durable checkpoint | Controlled save reordering and component failure cannot regress or mix committed source/projection generations. |
| Validated recovery | Identity/version/frontier mismatches fail before activation; the user-visible restore status matches the available evidence. |
| Complete conversational continuity | Accepted meaning is recoverable after explicit resume without importing another session's notes, goals or approvals; final request captures prove the intended projection. |

Start with the sequential `/new-session` witness and an owner check at storage/hydration, then make selection detach/install the correct memory, then fence asynchronous saves. A narrow owner check alone detects an inconsistent record but does not reconstruct a lost conversation. The completion criterion is the person's experience: **starting fresh is fresh, loading B really loads B, and a saved memory can explain which conversation and source frontier it represents.**

[main]: https://github.com/theRebelliousNerd/codenerd/commit/356f65e0e6bb639adaa90ab36212daa714114871
[feature]: https://github.com/theRebelliousNerd/codenerd/commit/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c
[research]: https://github.com/theRebelliousNerd/codenerd/commit/c4539f041ddee0ce15cbda02771af7422cdf053e
[guidance]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/agents.md#north-star
[standard]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/Docs/journeys/09-architecture-doc-standard.md
[fresh-boot]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/session_boot_helpers.go#L113-L132
[new-session]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/commands_handlers_files.go#L121-L136
[main-new-session]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/cmd/nerd/chat/commands_handlers_files.go#L121-L136
[select]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/session_persistence.go#L185-L246
[hydrate]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/session_persistence.go#L107-L145
[deferred]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/model_update.go#L1112-L1125
[state-copy]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/context/compressor_metrics.go#L161-L324
[history-path]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/session/persistence.go#L89-L102
[compressor-save]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/context/compressor_turns.go#L221-L235
[context-wiring]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/Docs/architecture/context/WIRING-AND-NOT-BUILT.md#wired-and-reachable
[save-compressed]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/session_persistence.go#L82-L94
[state-identity]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/context/compressor_metrics.go#L168-L196
[store]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/store/local_session.go#L182-L206
[active]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/context/compressor_metrics.go#L105-L125
[chat-context]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/process.go#L884-L900
[restore]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/context/compressor_metrics.go#L260-L344
[note-schema]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/core/defaults/schemas_context.mg#L40-L46
[note-policy]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/core/defaults/policy/context_compilation.mg#L49-L53
[note-write]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/context/memory_ops.go#L133-L180
[go-brief]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/work_hold.go#L79-L121
[state-type]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/context/types.go#L419-L439
[load-latest]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/store/local_session.go#L209-L240
[json-write]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/session/persistence.go#L74-L133
[save-all]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/session_persistence.go#L45-L182
[decode]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/context/serializer.go#L610-L616
[no-replay]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/context/compressor_turns.go#L33-L43
[save-test]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/session_persistence_test.go#L10-L31
[async-tests]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/model_update_persist_test.go#L43-L164
[dynamic-hydrate]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/core/virtual_store_predicates.go#L673-L760
[dynamic-caller]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/process.go#L790-L800
[reuse-intent]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/process.go#L168-L201
