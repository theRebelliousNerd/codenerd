---
doc-class: deep-dive
subsystem: chat-session-history
implementation-status: partially-implemented
last-verified: 2026-10-30
verified-against: 4cb0b3650de09085351014d522beb440c7e97ae3
verification-method: static-source-inspection
---

# Queued chat messages lose their answer ownership in SQLite history

## Finding and evidence boundary

At inspected main `4cb0b3650de09085351014d522beb440c7e97ae3`, the TUI can preserve a complete visible/JSON conversation while its SQLite session-history projection omits the running request and records a queue acknowledgement as the queued request's answer. The current projection pairs by message position, not execution ownership. Completing both requests and saving again does not correct that selection.

This is a source-derived residual of the previous background-history repair, not an executed reproduction, measured production incident, or claim that every downstream model receives incorrect context. Inspection used public repository sources only. No builds, tests, application runs, model calls, or repository changes were performed.

## Relationship to existing audits

The [background ownership audit](https://github.com/theRebelliousNerd/codenerd/blob/4cb0b3650de09085351014d522beb440c7e97ae3/Docs/audits/background-outcome-and-continuation-ownership.md#L139-L145) identified the earlier even-offset pairing implementation: an unsolicited assistant message shifted subsequent alternating turns. Its [as-built ledger](https://github.com/theRebelliousNerd/codenerd/blob/4cb0b3650de09085351014d522beb440c7e97ae3/Docs/audits/background-outcome-and-continuation-ownership.md#L249-L273) marks that defect fixed by `sessionTurns`. Credit that repair: an extra assistant result between otherwise complete exchanges no longer offsets every later pair.

The narrower residual here is **two user submissions before the first execution finishes**, together with the real busy-queue notice. The repaired algorithm still cannot identify which assistant messages are terminal answers to which requests. Its existing [regression fixture](https://github.com/theRebelliousNerd/codenerd/blob/4cb0b3650de09085351014d522beb440c7e97ae3/cmd/nerd/chat/background_ownership_test.go#L151-L164) checks `[user A, acknowledgement A, background result A, user B, answer B]`; it does not check the queued sequence below.

This is not the separate [queued slash-command dispatch mismatch](https://github.com/theRebelliousNerd/codenerd/blob/4cb0b3650de09085351014d522beb440c7e97ae3/Docs/audits/queued-slash-command-kind-conservation.md), cross-session save generation problem, or compressor lane-coverage problem. The witness uses natural-language requests, one session, successful ordered saves, and no compression requirement. It can be tracked as a concrete follow-on to the background audit rather than a competing history architecture.

## Production trace

1. While foreground request A is admitted but not yet marked as working, normal busy submission takes `submitWhileBusy`. It immediately appends user B, enqueues B, and appends an assistant message saying `_Queued: I'll take this up as soon as the current turn finishes._` The stopping/wind-down branch has an analogous notice. [Busy admission](https://github.com/theRebelliousNerd/codenerd/blob/4cb0b3650de09085351014d522beb440c7e97ae3/cmd/nerd/chat/side_turn.go#L276-L294).
2. A normal terminal `assistantMsg` or `responseMsg` appends its answer and schedules persistence. The queue drain retains the foreground-slot checks; no overlapping foreground execution is necessary. [Terminal handlers](https://github.com/theRebelliousNerd/codenerd/blob/4cb0b3650de09085351014d522beb440c7e97ae3/cmd/nerd/chat/model_update.go#L138-L201), [queue drain](https://github.com/theRebelliousNerd/codenerd/blob/4cb0b3650de09085351014d522beb440c7e97ae3/cmd/nerd/chat/side_turn.go#L455-L467).
3. Persistence snapshots the history slice before deferred IO. `syncSessionToSQLite` passes that history through `sessionTurns`. For each user message, this function scans only until the next user and chooses the first assistant encountered. User-message numbering advances even when no pair is emitted. [Snapshot](https://github.com/theRebelliousNerd/codenerd/blob/4cb0b3650de09085351014d522beb440c7e97ae3/cmd/nerd/chat/model_update.go#L1239-L1252), [projection and writer](https://github.com/theRebelliousNerd/codenerd/blob/4cb0b3650de09085351014d522beb440c7e97ae3/cmd/nerd/chat/session_persistence.go#L173-L215).
4. `StoreSessionTurn` upserts by `(session_id, turn_number)`. Its nonempty response replaces the existing response without a terminal/acknowledgement distinction. It cannot repair a wrong pairing supplied by the caller. [Store semantics](https://github.com/theRebelliousNerd/codenerd/blob/4cb0b3650de09085351014d522beb440c7e97ae3/internal/store/local_session.go#L80-L126).
5. This is a consumed projection, not merely a dead table. `GetSessionHistory` reads those rows; `VirtualStore.QuerySession` exposes their user and response as `session_turn(SessionID, TurnNumber, UserInput, Response)`. Hydration collects those facts, and later chat articulation reaches that hydration path. [Store reader](https://github.com/theRebelliousNerd/codenerd/blob/4cb0b3650de09085351014d522beb440c7e97ae3/internal/store/local_session.go#L129-L171), [fact projection](https://github.com/theRebelliousNerd/codenerd/blob/4cb0b3650de09085351014d522beb440c7e97ae3/internal/core/virtual_store_predicates.go#L347-L376), [hydration](https://github.com/theRebelliousNerd/codenerd/blob/4cb0b3650de09085351014d522beb440c7e97ae3/internal/core/virtual_store_predicates.go#L750-L763), [chat caller](https://github.com/theRebelliousNerd/codenerd/blob/4cb0b3650de09085351014d522beb440c7e97ae3/cmd/nerd/chat/process.go#L844-L853).

The inspected continuity rule derives `context_atom(UserInput)` from positive-numbered session turns; it does not read the response column. Therefore this trace establishes incorrect queryable history and potentially missing continuity inputs, **not** that a queue notice becomes an instruction or necessarily reaches a final provider prompt. [Continuity rule](https://github.com/theRebelliousNerd/codenerd/blob/4cb0b3650de09085351014d522beb440c7e97ae3/internal/core/defaults/policy/knowledge.mg#L76-L80).

## Deterministic source witness

Use two ordinary nondelegating requests in the same session with local SQLite wired and successful writes. Let A be in prework when B is submitted. Allow A to finish, then allow the existing queue to run B and let B finish. No session switch or reordered save is needed.

| Visible history suffix | Meaning |
|---|---|
| user A | First request |
| user B | Request entered before A answered |
| assistant queue notice B | Admission acknowledgement, not B's answer |
| assistant answer A | A's completed response |
| assistant answer B | B's completed response |

For A, the next message is another user, so `sessionTurns` emits no A pair. For B, the queue notice is the first assistant, so it emits B paired with that notice and ignores both actual answers. Every later full-history projection repeats these choices. If there were prior complete turns, their numbering remains intact; this is not the old permanent parity offset. Choose an A after the initial turn if evaluating the rule's `TurnNum > 0` condition.

Scope matters. An independent executor can also persist turns; do not generalize this witness to every delegated SQLite outcome. The clean witness excludes such execution. The chat projection itself remains wrong even if another producer happens to preserve additional evidence. Raw history, JSON serialization, and perception's raw-history path can still retain the actual messages. Loading a JSON transcript does not prove that SQLite recall has the same coverage. No data-loss rate, wrong model answer, permission bypass, or actual unauthorized action is asserted.

## Smallest coherent contract

Preserve the existing transcript and storage owners. Give accepted submissions stable session/request identities and attach terminal messages to those identities. Distinguish acknowledgement, progress, side reply, terminal answer and unsolicited result. Persist the completed exchange by its owner, not by whichever assistant appears next in display order. A queued request is pending until its own terminal disposition arrives; an acknowledgement must not satisfy that disposition.

Do not simply choose the last assistant before the next user: that can steal another request's answer or unrelated background result. Nor should skipping strings beginning with `_Queued` become the protocol; wording and localization are presentation, not ownership. Keep source messages independently recoverable when an exchange is canceled, incomplete, has multiple answer parts, or receives a late result.

Go should carry stable identities, snapshot the records, and perform IO. Any retention or recall-applicability choice belongs in the existing policy layer; no extra language-model pass is needed to guess ownership. Compatibility rows with unknown ownership should remain explicitly legacy/unattributed rather than being silently upgraded to a verified completed exchange.

## Failure handling and validation not executed

Define behavior for a save failure after terminal delivery, duplicate terminal notifications, a canceled queued request, shutdown before execution, partial/streamed answers, and navigation during outstanding work. Preserve current foreground settlement, queue FIFO and session ownership checks. Updating an SQLite projection must never replay an external action. A failed projection can be retried from retained owned events without rerunning the task.

A future test should drive actual Enter admission, A settlement, queue drain, B settlement and the production save/read/hydrate boundaries using fake nondelegating endpoints. Assert that A's and B's final responses are associated with their original requests, notices are retained only as notices, and `QuerySession` exposes the same ownership. Inspect stored rows, not merely that a persistence command returned.

Include controls for ordinary alternating exchanges, the existing unsolicited-background fixture, multiple queued requests, a side conversational reply, failed/canceled A, duplicate completions, missing dependencies, and no local database. Run same-session and cross-session variants separately so an existing ownership defect is not misattributed. Check raw JSON preservation as a separate property. A later provider-request capture or model-quality evaluation is needed before claiming an improvement to recall or answer accuracy.

## Costs and acceptance

The proposed bookkeeping does not require additional provider calls. Measure added identity/storage bytes, snapshot-copy and save latency, full-history rewrite cost, SQLite upserts and hydration size. Compare against the current implementation on identical message schedules; do not count dropping substantive answers as an efficiency win. An incremental owned-event projection may reduce rewrites but adds migration and delivery-state complexity, so it should not be assumed superior without measurement.

The mechanical exit is narrow: after two accepted requests finish, their durable queryable conversation records retain their own completed outcomes regardless of queue notices or display interleaving. Existing queue safety and full transcript recovery must remain intact. No performance or model-quality gain has been measured here.

## Primary reference

SQLite documents [UPSERT conflict handling](https://www.sqlite.org/lang_upsert.html). It resolves a conflicting row according to the specified update expressions; it does not infer conversational ownership or whether a response is terminal.
