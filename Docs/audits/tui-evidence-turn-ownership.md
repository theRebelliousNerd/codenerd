---
doc-class: deep-dive
subsystem: cli
implementation-status: partial
last-verified: 2026-10-28
verified-against: 356f65e0e6bb639adaa90ab36212daa714114871
verification-method: static-source-inspection; as-built 2026-10-28 verified against the branch feat/conversation-brief by the tests named below
supersedes: []
---

# TUI evidence turns: owning the read is not the same as delivering the answer

**Decision:** give an interactive evidence request one turn-scoped owner from routing through observation delivery. Preserve the current working executor and constitutional gates. Repair the residual chat path's result correlation and read-delivery contract before adding another planner or making the model more insistent.

**Evidence status:** this is source-inspected diagnosis and proposed design, not a reproduced incident or implemented fix. Main resolved to the revision above at approximately 07:39 UTC on 2026-10-28 and was unchanged when rechecked at approximately 07:55 UTC. No codeNERD runtime, tests, model inference, training, benchmark, failure injection, or user-computer task was run. The only local output is this research document. The user's report of fragile preliminary investigation motivates the scope; source establishes mechanisms that can explain fragility, not which one occurred in that user's run.

## 1. The user journey and the actual split

The requested capability is ordinary: in the main TUI, ask a question about a file or investigate something before deciding what work to do; obtain a grounded answer without knowing which internal lane can read files. A successful read that never reaches the answer model does not satisfy that capability. Neither does choosing a shard prove that the shard received the original question or sufficient evidence.

The repository's [North Star](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/agents.md#north-star) places executive decisions in Mangle, uses tools to reduce turns and offload deterministic work, and requires the harness to deliver relevant knowledge. This proposal follows that split: Go measures and transports observations; Mangle derives routing, evidence obligations and continuation; the model investigates and explains within the resulting authority.

The main TUI is not simply a wrapper around `session.Executor.Process`:

| Stage | Pinned source | Actual contract |
|---|---|---|
| Submit | [model_handlers.go:105–220](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/cmd/nerd/chat/model_handlers.go#L105-L220) | Slash commands leave through `handleCommand`. Normal input enters history, then `processInput` as a Bubble Tea command. Do not equate `/read path` with a natural-language read request. |
| Understand | [process.go:129–197](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/cmd/nerd/chat/process.go#L129-L197) | Prefer `PerceptionFirewall.Perceive`, otherwise direct transducer parsing. History goes to perception. |
| Publish and arbitrate | [process.go:219–295](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/cmd/nerd/chat/process.go#L219-L295), [delegation_routing.go:94–245](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/cmd/nerd/chat/delegation_routing.go#L94-L245) | Seed current intent, refresh routing inputs, read one derived lane. `RouteNone` means articulation, not a generic tool loop. |
| Explicit work | [process.go:399–649](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/cmd/nerd/chat/process.go#L399-L649) | Multi-step and explicit delegate lanes run their own work and return before the later information-gathering/system-result branch. Delegated mutations require a verification obligation and configured verifier. |
| Delegated executor | [task_executor.go:315–409](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/task_executor.go#L315-L409), [executor.go:1169–1373](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/executor.go#L1169-L1373) | Structured task intent, isolated executor/agent ownership, retrieval, tool envelope, JIT, working loop, error/evidence closure and observed return. This is a real execution path, not an unwired feature. |
| Planning within execution | [executor_tools.go:57–67](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/executor_tools.go#L57-L67), [work_steps.go:166–230](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/work_steps.go#L166-L230) | Edit-site planning is conditional on a write-oriented intent, suitable tools/channel and a brief needing a plan. Failure or fewer than two steps uses one pass. Read-only investigation is not universally forced through edit planning. |
| Residual information gathering | [process.go:703–790](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/cmd/nerd/chat/process.go#L703-L790), [process_helpers.go:83–158](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/cmd/nerd/chat/process_helpers.go#L83-L158) | Remaining turns query `next_action`; a local helper performs selected reads/searches and loads returned facts. The separate system-shard pipeline may also publish action outcomes. |
| Residual result handling | [process_dream_delegation.go:149–274](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/cmd/nerd/chat/process_dream_delegation.go#L149-L274) | With kernel and shard manager, inspect new execution rows, conditionally poll, interpret a result or fall back to a pending delegation. If neither exists, return nil and continue to articulation. `RouteRespondDirectly` skips this handler. |
| Articulation | [process.go:793–976](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/cmd/nerd/chat/process.go#L793-L976), [helpers_articulation.go:129–404](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/cmd/nerd/chat/helpers_articulation.go#L129-L404) | JIT chat system prompt plus history, shard summaries, compressed context, warnings and normalized intent; a streaming/system completion, not the session executor's native tool loop. Knowledge requests can cause specialist consultation afterward. |

This scope matters. The defects below do **not** mean every explicit reviewer/coder delegation loses its result. The result-count defect directly affects the residual system-result path; summaries also use it, but only when system-action display or debug logging enables that summary. The explicit delegate branch has a separate return path and structured observations.

## 2. What already improved, and what is not a new finding

On current main, [routing_arbitration.mg:96–123](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/defaults/policy/routing_arbitration.mg#L96-L123) explicitly recognizes workspace evidence. File/line scope or suggested tools prevents `wants_direct_answer`. The comment records the October 27 problems-log question that previously received a tool-less answer. [The corresponding test definition](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/cmd/nerd/chat/routing_arbitration_roundtrip_test.go#L111-L147) covers a confident `/analyze` query with a reviewer candidate. It is incorrect to report that this guard is still absent. The test was inspected, not run here.

Other prior work should remain intact:

- [CLI read-containment spec](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/Docs/architecture/cli/03-GAP-ANALYSIS.md#L45-L51) covers canonical workspace containment and correctly reflects the current resolver. It does not establish complete observations or prompt delivery.
- [S17 chat JIT design](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/Docs/journeys/impl/S17-chat-jit-prompt.md#L170-L205) already addresses the persona-only prompt and nonexistent `context_to_inject` bridge. The live [persona.go:489–541](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/cmd/nerd/chat/persona.go#L489-L541) compiles a JIT skeleton and combines it with the architect persona; unavailable or refused compilation falls back to the persona. This investigation is about evidence not reaching that skeleton, not restoring the dead query.
- [The journey map](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/Docs/journeys/00-journey-map.md#L130-L147) already identifies task rewriting from `(verb,target,constraint)` rather than preserving the original sentence. Current [delegation.go:460–545](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/cmd/nerd/chat/delegation.go#L460-L545) still maps `/analyze` to `complexity file:...`; constraints are appended. Treat this as an existing upstream fidelity concern, not a newly discovered audit item.
- [Stream completion](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/Docs/audits/stream-completion-contract.md#as-built-2026-10-27) now requires provider terminal evidence. A complete model generation can still lack the file evidence needed for its answer. These are separate contracts.
- [Task terminal ownership](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/Docs/audits/task-terminal-ownership.md#as-built-2026-10-28) now settles workers independently of polling. That repair does not identify which global `execution_result` rows belong to a TUI request.

## 3. Finding E1: result counts cross a destructive turn boundary

At [process.go:103–112](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/cmd/nerd/chat/process.go#L103-L112), `baseRoutingCount` and `baseExecCount` capture the current number of routing/execution rows. Perception then runs. At line 221 the caller invokes `clearStaleKernelFacts`, whose [lines 24–25](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/cmd/nerd/chat/process_helpers.go#L13-L37) retract both entire predicates.

Later, [diffFacts:588–593](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/cmd/nerd/chat/process_dream.go#L587-L594) assumes append-only indexing:

```text
query the current rows
if len(rows) <= old_count: return no rows
otherwise return rows[old_count:]
```

**Source-derived witness:** the previous turn leaves one execution row; the next turn records baseline 1, clears the predicate and obtains one new execution row. The handler sees no new execution. If the new turn produces two rows, it drops one. This arithmetic does not depend on the model's ability to read code. A particular real turn still requires the residual path to be taken and system execution to publish results.

The boundary is worse than a harmless display omission. [handleSystemDelegations](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/cmd/nerd/chat/process_dream_delegation.go#L149-L169) uses the filtered rows to choose between interpreting an existing result, performing a pending-delegation fallback, and proceeding without a system result. When an existing result is hidden, those choices are based on absence the caller manufactured. Duplicate fallback execution is a **conditional risk** requiring a remaining pending delegation; it is not established for every task or every read.

Even moving the baseline below the clear would only repair this immediate count mismatch. A queried fact set is not an append-only ordered event log; unrelated work can publish rows and pruning can remove them. The real contract is membership by request/action identity, not position in a fresh query result.

### Polling is a second boundary, not the whole diagnosis

[waitForSystemResults:562–584](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/cmd/nerd/chat/process_dream.go#L561-L584) returns when **either** routing or execution rows appear, or after the caller's timeout. The handler discards the routing slice and retains only execution rows; it uses a 1.2-second polling timeout on this path. The timer does not bound synchronous kernel-query work.

Do not claim that ordinary routing success always precedes execution. [router.go:404–447](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/shards/system/router.go#L404-L447) invokes `VirtualStore.RouteAction` synchronously before emitting routing success. However, [no-handler and rate-limit branches:333–373](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/shards/system/router.go#L333-L373) publish routing failure without a successful execution. Those are actionable outcomes, not evidence that no work was requested. A timeout is likewise “not observed yet,” not completion or proof that retry is safe.

## 4. Finding E2: local read success is neither complete nor delivered

The residual helper's `/fs_read` branch [process_helpers.go:96–112](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/cmd/nerd/chat/process_helpers.go#L96-L112) calls `readFileContent(...,8000)`, emits `file_content(target,content)`, and appends a warning saying the file was read and how many bytes the returned string contains.

The helper [helpers_files.go:90–106](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/cmd/nerd/chat/helpers_files.go#L90-L106) first performs `os.ReadFile`, then slices the bytes to `maxBytes`, returning only `(string,error)`. It supplies no whole-file size, missing-tail status, span, revision, continuation or recall handle. The cap does not bound disk IO or the initial allocation. The resulting warning can say “8000 bytes” for a much larger file without distinguishing those meanings.

The next boundary loses more than the tail:

1. [process.go:774–779](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/cmd/nerd/chat/process.go#L774-L779) loads the returned facts into the kernel.
2. [helpers_articulation.go:129–277](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/cmd/nerd/chat/helpers_articulation.go#L129-L277) renders history, structured prior shard information, compressed context, warnings and intent. It does not receive the local `executionResults` body.
3. The current JIT kernel-context route is `injectable_context`/specialist knowledge, not an automatic dump of arbitrary facts. [prompt_context.mg](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/defaults/policy/prompt_context.mg) has no `file_content` bridge. The inspected default-policy occurrences of `file_content` are its schema, trace metadata and coder-workflow presence check, not a content-to-chat injection rule.
4. `payloadForArticulation` carries action names/updates, not file bytes ([helpers_articulation.go:67–80](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/cmd/nerd/chat/helpers_articulation.go#L67-L80)); its [fallback merge:397–404](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/cmd/nerd/chat/helpers_articulation.go#L397-L404) happens after generation.

**Bounded conclusion:** when the residual local read is the only available observation and the independent system-execution path does not deliver an outcome, the answer model can receive “read succeeded” metadata without receiving the file. It cannot reliably answer a question about those unseen bytes. Custom policy could add an injection rule, and independent system-result interpretation or prior conversation could provide the content; neither is a guarantee supplied by this local helper. Do not generalize this finding to all file-reading tools.

There are good existing implementations to extend rather than duplicate. [VirtualStore.handleReadFile:24–135](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/virtual_store_file_actions.go#L24-L135) retains the bytes it actually read and uses the observation codec to project the model view, with whole-file refusal when a configured read ceiling is exceeded. The slash-command helper [readFileRange:109–172](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/cmd/nerd/chat/helpers_files.go#L109-L172) exposes span/total-line/truncation information. Its comment explicitly records the old problems-log tail-loss bug. Repairing that slash command did not remove the natural-language path's separate 8 KB helper.

## 5. Finding E3: evidence need vetoes one lane but does not guarantee an evidence outcome

The new `needs_workspace_evidence` guard is necessary but narrower than its motivating capability. It excludes `wants_direct_answer`; it does not itself derive a new execution lane, bind a source, or require observed evidence before final prose.

[should_delegate](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/defaults/policy/delegation.mg#L328-L342) still needs a non-`/none` candidate at or above the configured confidence threshold. [Routing clarification](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/defaults/policy/routing_arbitration.mg#L255-L297) covers particular uncertainty/target/clarification facts; low-confidence query work is not universally covered by the mutation-specific low-confidence rule. The caller explicitly treats no derived lane as articulation ([delegation_routing.go:191–206](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/cmd/nerd/chat/delegation_routing.go#L191-L206)).

**Proposed witness, not a classification-frequency claim:** an evidence-bearing query has concrete file scope but no candidate shard, no other early/clarification lane, no pending-delegation fallback and no successful system outcome. The direct-answer veto holds, yet the turn can still reach the same tool-less articulation call through `RouteNone`. Below-threshold queries also need explicit coverage: the separate system-delegation path may rescue them rather than simply no-op, but that outcome is not guaranteed by the evidence guard. The current high-confidence `/analyze` route test does not cover these boundaries.

The no-candidate shape is concrete, not an invented invalid intent: [taxonomy.go:719–723](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/perception/taxonomy.go#L719-L723) defines `/read` as `/query` with `/none` shard. [resolveShardTypeForIntent:259–274](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/cmd/nerd/chat/delegation_routing.go#L259-L274) returns no candidate unless an additional perception shard hint supplies one. With a concrete target and no other clarifier, this can use the residual information/action path rather than explicit delegation. That does not mean every natural-language read is classified identically or that every residual read fails.

This does not justify bypassing the confidence gate or assigning broad tools in Go. The missing invariant is: an unresolved workspace-evidence request ends in authorized investigation, explicit clarification, an actionable blocked result, or a qualified partial answer—not silently in an ordinary answer solely because no route was selected.

## 6. Evaluated hypotheses and deliberately limited claims

| Hypothesis | Assessment at the pin |
|---|---|
| The TUI has no working file/tool executor | Refuted: delegated session execution, VirtualStore reads and slash reads are live. |
| The October 27 evidence-routing guard is missing | Refuted: present; its test covers a confident reviewer case. |
| Every shard result is filtered by the broken count baseline | Refuted: explicit delegation returns earlier. Scope this to residual results and enabled summaries. |
| A successful local file read automatically reaches the chat prompt | Unsupported by the inspected default bridges; direct payload/body delivery is absent. An independent route may rescue it. |
| The cap saves a full-file disk read | Refuted: truncation follows `os.ReadFile`. |
| The planner is necessarily why preliminary investigation fails | Not established: edit-site planning has explicit write/brief/tool gates. No performance run was performed. |
| The main chat necessarily erases freshly produced perception-routing facts on every normal turn | Too broad. **If** the selected transducer publishes into the same kernel, source order is publish at [transducer_llm.go:631–636,671–725](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/perception/transducer_llm.go#L631-L725), then cleanup at [process_helpers.go:39–50](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/cmd/nerd/chat/process_helpers.go#L39-L50). Firewall transducer construction must be checked separately for kernel attachment; do not infer it from the direct transducer path. Also current `context_compilation.mg`/`jit_selection.mg` lack the priority consumers claimed by old comments. This is a conditional fact-lifetime inconsistency, not proven lost JIT capability. |
| Task-template loss is new | Refuted: already documented in the journey corpus. Preserve the original brief in a future repair but do not inflate this audit's novelty. |

The fact-lifetime issue reinforces a transactional ownership design, but E1–E3 are the primary implementation queue. No claim about failure incidence, elapsed-time improvement or token savings has been established.

## 7. Proposed contract: one owner, one evidence disposition

The following are design names, not current APIs or registered predicates.

```text
EvidenceTurn:
  turn_id, session_id, request_digest, original_request
  normalized_intent, evidence_need, scope_reference
  selected_lane, execution_owner, admitted_action_ids
  authority_envelope_reference
  status: admitted | investigating | ready | partial | blocked | canceled

EvidenceObservation:
  turn_id, action_id, source_identity, revision_or_freshness
  outcome: succeeded | failed | canceled | unknown
  coverage: whole | span | projected | unavailable
  requested_span, returned_span, source_extent, omitted_extent
  body_or_authorized_recall_handle, diagnostic

EvidenceDelivery:
  turn_id, observation_ids, consumer_request_id
  delivered_body_or_handle, projection_status, budget_disposition
```

Keep acquisition, coverage and delivery distinct. A source can be read successfully but partially projected. A handle can be delivered without its body; that is not evidence the model redeemed it. A previously obtained observation can satisfy the request if source identity, freshness, scope and current authorization still hold; do not mandate another tool call merely to improve a count.

### 7.1 Admission and fact ownership

1. Allocate a unique turn identity before perception or action publication. Carry the original request alongside normalized intent; do not make a template the only surviving task objective.
2. Retire the prior turn's ephemeral facts before publishing the new frame, or use scoped facts and retire only that scope. No cleanup phase may delete evidence just published for the current turn. Preserve required long-lived observations and other active tasks.
3. Publish routing signals as a coherent turn frame before executive dispatch. Mangle derives one execution owner/lane or an explicit unresolved disposition. This is not permission; existing capability and `permitted(Action,Target,Payload)` checks remain mandatory.
4. Register action IDs under that owner before dispatch. Consume terminal outcomes by those IDs, not by global counts, query order or timestamps alone. Existing router correlation IDs should be carried through rather than replaced.

### 7.2 Evidence routing and read execution

5. Extend current routing arbitration with a total disposition for evidence-bearing requests. Reuse a suitable existing read-only session executor or the governed action path. If the source is ambiguous, clarify; if unavailable or denied, report the blocker. A tool-less final call is valid only with sufficient admitted evidence or an explicitly qualified disposition.
6. Retire or adapt the ad hoc local read helper to the shared read/observation contract. Retain workspace containment, secret-path policy and exact permission checks; never treat the new evidence record as a capability grant. A larger observation budget does not expand readable scope.
7. Return explicit source/span/revision/coverage metadata and an authorized continuation or recall route. For a count over an entire log, a prefix is not complete coverage. Prefer a deterministic count/structured extraction when it answers the actual question; source reading need not become a full-file prompt dump.
8. Bound read IO, memory and cancellation at the operation boundary. Preserve source-change detection between pages. Do not join pages from different revisions and label them one whole-file observation.

### 7.3 Waiting and delivery

9. Wait for the owned action's terminal outcome. An event or joinable handle is useful if the owner already exposes one; correctly correlated polling is also acceptable. The necessary fix is identity and terminal semantics, not replacing every timer.
10. A routing failure is a typed blocked/failed outcome. A waiting deadline leaves pending/unknown state and ownership intact; it must not automatically authorize a second dispatch. Only the owner can decide a safe retry using the action's effect class and known outcome.
11. Put the selected observation body or usable handle into the actual answering request under the same context-budget policy as other evidence. Record which observation/projection was delivered. Avoid full source duplicated in facts, warning text, history and system prompt.
12. Final prose is constrained by evidence disposition: answer, qualified partial answer, targeted question, or actionable blocker. The user's ability to proceed is the criterion—not a generic “read succeeded” line or a model-generated claim that it inspected everything.

Go owns filesystem/process mechanics, synchronization, observations and provider delivery. Mangle owns executive selection and whether the current obligations are satisfied or unresolved. JIT atoms explain the chosen evidence protocol; they do not become a second dispatch implementation. Reuse existing observation stores, receipt/usage infrastructure and terminal-task ownership before adding a parallel persistence subsystem.

## 8. Failures, cancellation, retention and recovery

- **Two chat turns and late work:** a late outcome remains attached to its original turn. It cannot satisfy the newer turn merely because the same path or `/current_intent` appears again.
- **Canceled UI wait:** distinguish canceling work from detaching a view according to the existing API contract. Preserve actual outcome and partial observations; neither cancellation nor timeout proves no effect.
- **Read failure or policy denial:** retain a typed diagnostic and do not surface it as an empty successful source. Denied access is not something to route around through another helper.
- **Missing payload publication:** report “executed, evidence unavailable” or equivalent, preserve recoverable metadata, and do not replay a potentially effectful action to repair delivery.
- **Large or changing sources:** expose projection/coverage and current source revision; do not silently call an 8 KB prefix a file. Bound handle lifetime and disclose expired/stale handles.
- **Concurrent owners:** cleanup and retention must be per owner/scope. Global predicate retraction is not an acceptable way to make the UI look idle while another task owns work.
- **Privacy:** evidence receipts should default to IDs, paths/digests and coverage metadata, not raw file bodies in an additional log. Follow existing access and deletion rules for retained observations.

Persistence beyond the existing session/observation store is not required to fix E1. If recovery across restart becomes a requirement, specify durable action ownership and unknown-effect reconciliation separately; an in-memory turn ID alone is not crash recovery.

## 9. Full cost accounting and fair alternatives

The current local cap has an especially misleading cost shape: whole-file IO/allocation, an 8 KB retained string, and potentially no useful answer-model evidence. The alternate system path can perform its own read and interpretation. Source shows these possibilities, not their frequency or a measured double-read rate.

Report a resource vector rather than adding incompatible units:

- Perception, planner, JIT, answering, specialist, verification and retry model calls; input/output/cache usage with the existing accounting semantics.
- Query/derivation CPU, filesystem bytes read, extraction CPU, allocations and unique retained bytes.
- Evidence projection and recall calls, serialization/storage IO, receipt bytes and retention cost.
- Time to first useful evidence, time to an actionable answer, p50/p95/p99 end-to-end latency, cancellation settlement and human recovery time.
- Grounded-answer correctness, source/coverage accuracy, successful follow-on work and unauthorized-effect rate.

Only convert this vector into a scalar monetary cost with disclosed prices and an explicit human-time valuation. Count failed attempts and follow-up turns, not just the successful final call. No price or improvement is asserted here.

Compare matched alternatives:

1. Current main, including its already-shipped evidence-routing guard.
2. Minimal count-boundary correction as a diagnostic control, without claiming it solves identity or delivery.
3. Correlated residual action outcomes plus shared complete-observation delivery, retaining the current executor topology.
4. A single read-only working-executor lane for evidence requests, with the same capabilities and context ceiling.

The fourth option may remove fragile one-shot handoffs but can add an LLM round, tool-catalog tokens and task-setup cost for a trivial read. The third may be cheaper but needs a real continuation mechanism for multi-file investigation. Prefer the smallest option that meets the journey's evidence and follow-on-work contract, not the one with the fewest visible tools. Do not broaden permissions, increase the model budget, weaken acceptance checks or switch to a stronger model and then credit the harness for the gain.

## 10. Proposed validation and build queue — not executed

| Fixture / journey | Required observation |
|---|---|
| Prior turn has 1/3 execution rows; new turn has fewer/equal/more after cleanup | Every result belonging to the new turn is visible; none selected by count position. |
| Same file requested across two turns with a late first result | Only the appropriate revision and turn's observation can satisfy the second request. |
| Confident reviewer evidence query | Preserve the October 27 guard's intended routing and successful delegated path. |
| File-scoped query with no shard candidate; below-threshold query | Explicit evidence-capable or unresolved disposition, not accidental plain articulation. |
| Residual read with system shards absent/unavailable/delayed | The answering request receives the local observation or an honest blocker; a “read succeeded” warning is not the assertion. |
| File longer than 8 KB with answer only in its tail | Correct answer or explicit partial coverage and continuation, never false whole-file confidence. |
| Slash `/read` versus natural-language read | Both retain their intended UX and consistent completeness/containment guarantees. |
| No-handler, rate-limit, permission failure, canceled read | Correct typed terminal/blocked state; no automatic duplicate fallback or unauthorized retry. |
| Multiple actions and unrelated background results | Correlate all required action outcomes; do not choose an arbitrary latest row. |
| Existing evidence is fresh and sufficient | Reuse without an unnecessary reread; receipt proves delivery rather than an inflated tool count. |
| First source read, follow-up explanation, then authorized work | Original objective and relevant evidence survive the handoff; no reliance on a lossy template alone. |
| Prompt inspection with a unique sentinel in a fixture file | Sentinel or authorized recall handle reaches the actual answer request; kernel presence alone does not pass. |

Use controlled barriers for lifecycle ordering rather than timing sleeps. Keep deterministic source/correlation fixtures separate from model-quality evaluation. For the latter, use matched multi-turn tasks covering source questions, preliminary investigations and follow-on work; independently score cited evidence and answer correctness. Retain simple conversational controls so an evidence guarantee does not turn every greeting into repository exploration. Test definitions are proposed here, not results.

| Research ID | Narrow deliverable | Checkable exit |
|---|---|---|
| TUI-EVID-01 | Per-turn action/result ownership | New-turn outcomes survive old-turn cleanup and cannot be hidden by prior counts or accepted from unrelated work. |
| TUI-EVID-02 | Complete read observations and actual prompt delivery | Tail-only and prompt-sentinel fixtures demonstrate explicit coverage and usable evidence through the real residual path. |
| TUI-EVID-03 | Total disposition for workspace-evidence needs | No-candidate/uncertain/denied cases terminate explicitly or investigate under unchanged authority. |
| TUI-EVID-04 | Preserve request objective through work handoff | Existing task-rewrite concern reconciled with the owning journey/spec; original question and constraints remain available. |
| TUI-EVID-05 | End-to-end value | Fewer failed/repeated investigation turns at unchanged answer quality, authority and full accounted cost. |

Before implementation, reconcile the CLI, perception, session and relevant system-shard specs with the selected contract. Do not rewrite their protected guidance or claim this proposal is built. The first milestone is a concrete user-visible guarantee: **if the TUI says it read a source to answer a question, the answering path has the right source evidence—or clearly tells the user what is missing.**

## As built (2026-10-28, branch `feat/conversation-brief`)

The claims were re-read against this branch, which has since added a conversation brief, work-mode admission (`routing_arbitration.mg` `work_admitted`), background read-only shards and Jev judgments. None of those touch the residual result path; E1, E2 and E3 all stood.

### Verification of the claims

| Claim | Branch state |
|---|---|
| October 27 evidence guard present (section 2) | Still true, unchanged: `needs_workspace_evidence` at `routing_arbitration.mg:111-114`, `wants_direct_answer` vetoes at `:120-132`; `TestDecideRoute_FileScopeQuestionDelegates` (`routing_arbitration_roundtrip_test.go`) covers it. Not rebuilt. |
| E1 count baselines cross a destructive clear | Real. `process.go` took `len(routing_result)` / `len(execution_result)` before perception and `clearStaleKernelFacts` (`process_helpers.go:22-23`, called at `process.go:239`) retracted both predicates afterwards; `diffFacts` returned `rows[old:]`. Fixed (below). |
| Polling returns on either predicate and the handler discards routing | Real: `handleSystemDelegations` kept `_, execFacts = wait(...)`. Fixed: routing failures are now typed (below). |
| E2 residual read caps at 8000 after `os.ReadFile` and the body never reaches articulation | Real. `process_helpers.go` called `readFileContent(...,8000)` (whole `os.ReadFile`, then a slice), asserted the prefix as `file_content`, and `articulateWithConversation` rendered no `executionResults`; no default policy bridges `file_content` to the prompt. Fixed (below). |
| E3 an evidence request with no candidate shard ends in tool-less articulation | Real: `route_decision` derived nothing (`decideRoute`, `RouteNone`). Fixed (below). |
| E4 `formatShardTask` rewrites the request | Real, unchanged; deliberately not built (see left open). |
| Section 6 row "main chat erases perception-routing facts on every normal turn" | Still the conditional the doc says: the retraction at `process_helpers.go:44-51` runs after perception. It belongs to the perception-restriction audit's ordering work, not to this one. |

### What was built

* **TUI-EVID-01, per-turn result ownership by identity.** `cmd/nerd/chat/turn_results.go`: `systemResultBaseline` is the set of `routing_result` / `execution_result` rows standing when the wait began, keyed by predicate and arguments (the action id is arg 0); `snapshotSystemResults` and `newSystemResults` replace the integer counts and `diffFacts`. `processInput` snapshots at turn start and again immediately after `clearStaleKernelFacts` (`process.go:106`, `:240`), so the clear cannot make the turn's own first result look old. All consumers (`collectSystemSummary`, `waitForSystemResults`, `handleSystemDelegations`, `buildResponseFromExecutions`, `executeDelegateTaskFallback`) take the baseline. A routing failure is a typed outcome: `unroutedActions` names each action whose routing failed (no handler, rate limit, error) and produced no execution row; `handleSystemDelegations` returns those as blockers and `processInput` adds them to the articulation warnings, so the answer is not written as though the action ran. A wait that returns empty is "not observed yet" (comment on `waitForSystemResults`).
* **TUI-EVID-02, complete read observations and actual prompt delivery.** `cmd/nerd/chat/turn_evidence.go`: `readEvidenceFile` opens the file, takes the size from the handle and reads at most `observation.max_region_bytes` (`config.ResolvedObservationLimits`, default 24 KiB; no new constant) through `io.LimitReader`, so a large file is not slurped to be sliced. The result is a typed `evidenceObservation` with coverage `whole`, `span` (prefix: returned bytes, source bytes, returned lines) or `unavailable` (diagnostic; containment is the shared `tools.ResolveWorkspacePath`, so the earlier containment guarantees are unchanged), trimmed to a rune boundary. `executeInfoGatheringActions` now returns the observations and asserts `file_content` only for a whole file: a prefix stored as the file made every later reader of the fact a prefix. `ConversationContext.Evidence` is rendered by `articulateWithConversation` (`helpers_articulation.go`, `renderEvidence`) into the answering request itself: body in a fence the file cannot close, a PARTIAL banner naming `N of M bytes` and that the rest was not read, or an UNAVAILABLE line telling the model not to claim it read the file. `/edit` used the same silent 16000-byte cut and offered a prefix to `/write`; it now uses `readEvidenceFile` and refuses to offer a prefix (`commands_handlers_analysis.go`). `readFileContent` is deleted.
* **TUI-EVID-03, a total disposition for workspace-evidence turns.** `routing_arbitration.mg`: `delegate_lane_derived()` (nullary, so it can be negated) and `evidence_lane()` derive `route_decision(/gather_evidence, /none)` for a read-only (`/query`) turn that `needs_workspace_evidence()` and that no early, direct-answer, clarify or delegate lane took. It is the lowest-precedence lane, so the one-lane-at-most invariant holds. `RouteGatherEvidence` (`delegation_routing.go`) makes `processInput` call `gatherWorkspaceEvidence`: it reads the file perception named (scope file, else a path-like target) under the same authority, or, when the turn names no file, produces an `unavailable` observation saying so (with the tools perception suggested). The turn therefore ends in investigation with evidence, or an answer that is told it has none; it can no longer be a tool-less answer written as though the file had been seen. A file already read by the residual path is not read twice.

### Tests (all in `cmd/nerd/chat`)

* `turn_results_test.go`: `TestSystemResults_ClearThenNewRowsAreAllVisible` (the 1-row/clear/1-or-2-new witness of section 3), `TestSystemResults_StandingRowsAreExcludedByIdentity` (pruning and reorder), `TestWaitForSystemResults_ReturnsNewRowsAndTimesOutEmpty`, `TestUnroutedActions_NamesFailuresWithoutExecution`.
* `turn_evidence_test.go`: `TestReadEvidenceFile_WholeAndPrefixAreTold` (1 MiB log, tail sentinel, bounded body), `TestReadEvidenceFile_CutNeverSplitsARune`, `TestReadEvidenceFile_UnavailableIsTyped` (outside path, missing, directory), `TestResidualRead_PrefixIsNotFileContent`, `TestArticulation_PromptCarriesEvidenceAndItsCoverage` (prompt-sentinel; whole, partial, unavailable, none), `TestFencedBody_FileCannotCloseItsFence`, `TestDecideRoute_EvidenceWithNoShardGathersEvidence` (no candidate; candidate below the gate; exactly one `route_decision` row), `TestDecideRoute_ToolsOnlyEvidenceGathers`, `TestDecideRoute_GatherEvidenceDoesNotTakeOtherLanes` (confident reviewer still delegates, mutation does not gather, prose question still answers), `TestGatherWorkspaceEvidence_*`, and `TestProcessInput_EvidenceTurnDeliversTheFileToTheAnsweringCall`, which drives `processInput` with a scripted perception and a fixture file larger than the budget and asserts the head sentinel is in the prompt the mock model received and that the prompt says the file was partly read. `chat_read_boundary_test.go` was updated for the new return value; its containment tests still pass unchanged in meaning.
* Mangle decls added in `schemas_shards.mg`: `delegate_lane_derived()`, `evidence_lane()`; `route_decision`'s documented lane set gains `/gather_evidence`.

### Deliberately left open

* **Late work from a previous turn.** The baseline is membership by row identity, not ownership by action id registered under the turn. A result from a prior turn that completes after the clear is indistinguishable from this turn's. Ownership needs the dispatcher to carry a turn id on `routing_result` / `execution_result` (`internal/shards/system/router.go`, `internal/core/virtual_store_routing.go`), a schema change across the action pipeline that this audit's chat-side repair does not need.
* **Timeout leaves ownership intact (item 10).** `executeDelegateTaskFallback` still runs a pending `delegate_task` after the 1.2 s wait elapses with no observed outcome; a slow synchronous `RouteAction` could in principle overlap it. Closing it needs an in-flight registry exposed by the router; no such fact exists and the router's `pendingCalls` is not queryable from the chat.
* **TUI-EVID-04, original request preserved through `formatShardTask`.** The task string (`complexity file:...`, `fix file:... with constraint:`) is a protocol parsed by several shards and shown to the user; appending the sentence is a typed task envelope change across them, not a suffix. It stays with the journey corpus item the audit cites.
* **Multi-page continuation and shared observation codec.** The residual read delivers one bounded prefix and names `/read` with a line range for the rest; it does not reuse `VirtualStore.handleReadFile`'s observation codec, handles or revision checks, and does not detect a source change between pages. A deterministic count for "how many lines" over a prefix is not computed.
* **TUI-EVID-05, end-to-end value and cost comparison** (paired model trials, resource vector) needs live sessions and was not run; no improvement is claimed.
