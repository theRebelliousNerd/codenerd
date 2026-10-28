---
doc-class: deep-dive
subsystem: cli
implementation-status: planned
last-verified: 2026-10-28
verified-against: 356f65e0e6bb639adaa90ab36212daa714114871
verification-method: static-source-inspection
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

## Branch qualification — background evidence and Jev (2026-10-28)

The findings above describe main at [`356f65e0e6bb639adaa90ab36212daa714114871`](https://github.com/theRebelliousNerd/codenerd/commit/356f65e0e6bb639adaa90ab36212daa714114871). The supplementary comparison includes `feat/conversation-brief` at `58463864e3e78efdf72b0706a21f495ffead23c3`, then subsequent commits through [`9755db4f5851c28771dc1dd6d720856be1d5813a`](https://github.com/theRebelliousNerd/codenerd/commit/9755db4f5851c28771dc1dd6d720856be1d5813a) (committed 08:59:40 UTC; branch checked 2026-10-28 at 09:00 UTC). Credit the feature's new evidence-delivery paths without treating them as a repair of every residual path. This is source inspection only: no tests, live probes or performance measurements were run, and no merge into main is asserted.

### Added capabilities to retain

- **Job-owned background delivery.** A delegated `/query` can derive `route_background`; the chat returns a `backgroundStartMsg`, starts the job through Update, and handles its `backgroundDoneMsg`. The completion carries its job, result, facts and error directly. Completion attempts to load returned facts; a `LoadFacts` failure is logged rather than propagated, so successful fact publication is not guaranteed. On job success it stores the shard result and adds its surface to chat; job failure/cancellation gets a visible disposition. This is a real independent delivery path, not the old `execution_result` count-difference mechanism. Do not apply E1 indiscriminately to it. [Routing](https://github.com/theRebelliousNerd/codenerd/blob/9755db4f5851c28771dc1dd6d720856be1d5813a/internal/core/defaults/policy/routing_arbitration.mg#L274-L286), [dispatch](https://github.com/theRebelliousNerd/codenerd/blob/9755db4f5851c28771dc1dd6d720856be1d5813a/cmd/nerd/chat/process.go#L487-L504), [Update](https://github.com/theRebelliousNerd/codenerd/blob/9755db4f5851c28771dc1dd6d720856be1d5813a/cmd/nerd/chat/model_update.go#L168-L178), [completion](https://github.com/theRebelliousNerd/codenerd/blob/9755db4f5851c28771dc1dd6d720856be1d5813a/cmd/nerd/chat/background.go#L134-L197).
- **Foreground side turns and cancellation.** Once foreground work is marked active, a busy submission can take the new side-turn path instead of starting another full turn; an earlier submission is queued. Its explicit bridge uses separate intent/judgment predicates and updates the brief. Ctrl+X and the derived side-turn stop now call the registered turn cancel. This is useful ownership/cancellation plumbing, but cancel invocation is not proof of terminal settlement, and a side-turn reply is not a workspace observation. It does not replace the residual full-turn count logic below. [Busy submission and side turn](https://github.com/theRebelliousNerd/codenerd/blob/9755db4f5851c28771dc1dd6d720856be1d5813a/cmd/nerd/chat/side_turn.go#L139-L222), [turn registration](https://github.com/theRebelliousNerd/codenerd/blob/9755db4f5851c28771dc1dd6d720856be1d5813a/cmd/nerd/chat/process.go#L78-L83), [Ctrl+X](https://github.com/theRebelliousNerd/codenerd/blob/9755db4f5851c28771dc1dd6d720856be1d5813a/cmd/nerd/chat/model_key_handler.go#L60-L70).
- **Review citation checks.** The intervening `78103e6` commit adds Jev checks of usable file/line citations, flags refuted findings through `review_suspect` and injectable verification warnings, and invokes the check for foreground and background reviews. It does not discard a finding merely because Jev disputes it. This strengthens review evidence, not general read completeness or request ownership. [Checker](https://github.com/theRebelliousNerd/codenerd/blob/9755db4f5851c28771dc1dd6d720856be1d5813a/internal/turnjudge/findings.go#L34-L105), [policy](https://github.com/theRebelliousNerd/codenerd/blob/9755db4f5851c28771dc1dd6d720856be1d5813a/internal/core/defaults/policy/jev_evidence.mg#L78-L95).
- **Self-diagnosis gets actual problem observations.** The self-diagnosis path optionally triages kernel-selected hot problems, orders likely codeNERD defects first and lists surrounding failures separately. It retains the other group, leaves uncertain judgments with defects and preserves log order if triage fails. The task includes each selected problem's full sample. This is a concrete log-to-task evidence path; it is not proof that an arbitrary natural-language file read reaches articulation. [Task assembly](https://github.com/theRebelliousNerd/codenerd/blob/9755db4f5851c28771dc1dd6d720856be1d5813a/cmd/nerd/chat/self_diagnosis.go#L57-L117), [split semantics](https://github.com/theRebelliousNerd/codenerd/blob/9755db4f5851c28771dc1dd6d720856be1d5813a/internal/turnjudge/triage.go#L51-L64).

### Residual findings remain narrower than those capabilities

| Finding | Source at the feature freeze |
|---|---|
| E1: old counts survive destructive cleanup | `processInput` still captures counts at [107–115](https://github.com/theRebelliousNerd/codenerd/blob/9755db4f5851c28771dc1dd6d720856be1d5813a/cmd/nerd/chat/process.go#L107-L115), clears later at [248](https://github.com/theRebelliousNerd/codenerd/blob/9755db4f5851c28771dc1dd6d720856be1d5813a/cmd/nerd/chat/process.go#L246-L249), and the residual handler still consumes [`diffFacts`](https://github.com/theRebelliousNerd/codenerd/blob/9755db4f5851c28771dc1dd6d720856be1d5813a/cmd/nerd/chat/process_dream_delegation.go#L149-L169), whose [baseline slicing](https://github.com/theRebelliousNerd/codenerd/blob/9755db4f5851c28771dc1dd6d720856be1d5813a/cmd/nerd/chat/process_dream.go#L587-L594) is unchanged. |
| E2: residual local read is still a separate contract | The helper still emits `file_content` plus a byte-count warning after an [8,000-byte read projection](https://github.com/theRebelliousNerd/codenerd/blob/9755db4f5851c28771dc1dd6d720856be1d5813a/cmd/nerd/chat/process_helpers.go#L96-L112); [`readFileContent`](https://github.com/theRebelliousNerd/codenerd/blob/9755db4f5851c28771dc1dd6d720856be1d5813a/cmd/nerd/chat/helpers_files.go#L90-L106) still reads the file before slicing. The new background message does not carry this helper's result. |
| E3: background eligibility is not total evidence disposition | `route_background` requires a derived delegate lane; it does not create a shard candidate or waive the confidence gate. [No derived lane still returns `RouteNone`](https://github.com/theRebelliousNerd/codenerd/blob/9755db4f5851c28771dc1dd6d720856be1d5813a/cmd/nerd/chat/delegation_routing.go#L218-L229), so the no-candidate/uncertain residual cases still require explicit coverage. |

The implementation queue should therefore **reuse and verify the background result path**, not duplicate it, while repairing residual count ownership and establishing coverage-aware delivery for local reads. Test two active jobs, a newer chat turn, side-turn/queued submissions and late completion; verify the actual answering request receives the appropriate source observation. Separately test the old residual path with a nonzero prior baseline and a tail-only answer. These remain proposed validations, not passed results.

## Authority-parity qualification — host evidence acquisition (2026-10-28)

**Decision:** retain the feature branch's new evidence acquisition and delivery, but do not qualify it as operating under the session's existing authority contract until the host read passes the same applicable access checks. Workspace containment and read-only classification do not establish permission to disclose an in-workspace protected file to a model.

This is a focused continuation of this audit, not a new independent discovery of the need for safe reads. Section 7.2, step 6 already requires the shared read/observation contract to retain secret-path policy and exact permission. The new contribution is a source-level verification that the feature's implementation does not yet satisfy that requirement. The feature audit's as-built TUI-EVID-03 account says the host reads under the same authority; the source call chain does not establish that parity. Correct that qualification without withdrawing credit for actual evidence delivery. [Feature audit, “What was built,” TUI-EVID-03](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/Docs/audits/tui-evidence-turn-ownership.md#L248-L252)

### Freeze and evidence status

Main is pinned to [`356f65e0e6bb639adaa90ab36212daa714114871`](https://github.com/theRebelliousNerd/codenerd/commit/356f65e0e6bb639adaa90ab36212daa714114871); `feat/conversation-brief` is pinned to [`c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c`](https://github.com/theRebelliousNerd/codenerd/commit/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c). The prior audit was read on `codenerd-research-docs` at [`0de7ff76968955ad5185d8dd13b85bc05775aa98`](https://github.com/theRebelliousNerd/codenerd/commit/0de7ff76968955ad5185d8dd13b85bc05775aa98), whose TUI evidence document blob was `c39bdb6ed9eee2c083ec15737f14c3a71fa3ebff`; the feature's separate as-built appendix was read at the feature pin, document blob `97b9bbdaba67773c9b99c46be60855a94c030720`. Inspection occurred approximately 13:38–13:50 UTC on October 28, 2026. No merge into main is implied.

**Static inspection only.** No runtime, tests, inference, training, benchmark, exploit, or real secret-file access was performed. Examples below use proposed synthetic fixtures, not observed leaks. The supported finding is an admission-contract discrepancy with a complete source path to prompt construction; exposure frequency and actual user impact remain unmeasured.

The root [North Star and runtime contract](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/agents.md#north-star) require typed policy-mediated operations and logic-owned safety, while tools reduce turns and deterministic work. [Session contributor guidance, capability/permission split](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/agents.md#L3-L17) makes capability, exact permission and execution separate stages. This appendix follows the [architecture-document standard, “What the corpus is for” and citation rules](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/Docs/journeys/09-architecture-doc-standard.md#what-the-corpus-is-for): present facts, proposed repair and future validation remain separate.

### 1. Credit the feature's real improvements

The research branch's older feature qualification stops at `9755db4`; it is not a current description of every mechanism at `c2d39e5`. The feature branch has its own [as-built account](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/Docs/audits/tui-evidence-turn-ownership.md#L232-L266), which credits new result identity, coverage and delivery while expressly leaving late-work ownership, timeout duplication, original-request handoff and multi-page revision continuity open. This appendix qualifies authority parity rather than re-auditing all of those items.

At the newer feature pin:

- `readEvidenceFile` bounds acquired bytes using `io.LimitReader`, reports whole/span/unavailable coverage, and retains source size and returned size. It no longer slurps a whole file just to retain an 8 KB prefix. [`cmd/nerd/chat/turn_evidence.go:47–108`, `evidenceReadBudget`, `readEvidenceFile`](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/turn_evidence.go#L47-L108)
- The residual `/fs_read` helper now returns observations and asserts `file_content` only for whole-file coverage. [`cmd/nerd/chat/process_helpers.go:95–130`, `executeInfoGatheringActions`](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/process_helpers.go#L95-L130)
- A new derived gather-evidence lane handles a workspace-evidence query when no earlier or delegated lane takes it. The branch no longer simply relies on a candidate always being available. [`internal/core/defaults/policy/routing_arbitration.mg:289–310`, `evidence_lane`, `route_decision`](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/core/defaults/policy/routing_arbitration.mg#L289-L310)
- Observations are explicitly carried into `ConversationContext` and rendered into the answering prompt. There is a production-entry test definition whose sentinel assertion checks that prompt, not merely kernel fact presence. [`cmd/nerd/chat/process.go:842–904`, `processInput`](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/process.go#L842-L904); [`cmd/nerd/chat/helpers_articulation.go:251–256`, evidence rendering](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/helpers_articulation.go#L251-L256); [`cmd/nerd/chat/turn_evidence_test.go:232–264`, `TestProcessInput_EvidenceTurnDeliversTheFileToTheAnsweringCall`](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/turn_evidence_test.go#L232-L264)

These are substantive E2/E3 advances to preserve. They are not proof of full revision/freshness coverage, complete multi-file investigation, or policy parity. The newly effective delivery makes the admission boundary more important, not less: “the model finally received the body” is a success criterion only for an authorized body.

### 2. The exact discrepancy

The project already has a specific protection contract, not merely a generic preference to avoid secrets. `ExecutionConfig.SecretPaths` says protected file contents do not reach a model; its defaults include `.env` and `.nerd/config.json`. An explicitly configured empty list is intentionally different from an absent configuration. [`internal/config/execution.go:33–73`, `SecretPaths`, `DefaultSecretPaths`, `ResolvedSecretPaths`](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/config/execution.go#L33-L73)

The governed session route measures the target and argument paths with `tools.IsSecretPath`, asserts `touches_secret_path`, then queries exact `permitted(Action, Target, Payload)`. Assertion failure fails closed. The constitution joins the measurement to the pending action as dangerous content. These are working pieces, not proposed replacements. [`internal/session/secret_paths_gate.go:31–66`, `secretPathFact`, `secretTarget`](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/session/secret_paths_gate.go#L31-L66); [`internal/session/executor_tools.go:3005–3046`, `admitToolCall`](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/session/executor_tools.go#L3005-L3046); [`internal/core/defaults/policy/constitution.mg:379–387`, `dangerous_content`](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/core/defaults/policy/constitution.mg#L379-L387)

The feature's host-read route instead does this:

| Boundary | Inspected source | What is established |
|---|---|---|
| Evidence need | `evidence_lane`, policy lines 301–307 above | Requires a query and evidence need, excludes earlier/clarify/delegate lanes. It does not test a concrete file's secret policy or exact permission. |
| Target selection | [`turn_evidence.go:114–121`, `workspaceEvidenceTargets`](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/turn_evidence.go#L114-L121) | Uses perception's nonempty `ScopeFile`, otherwise a path-like `Target`. This is an inferred source selection, not a trusted consent event. |
| Host acquisition | [`turn_evidence.go:56–108`, `readEvidenceFile`](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/turn_evidence.go#L56-L108) | Resolves workspace containment, opens the file, reads a bounded body. No call to the secret matcher, session admission, capability resolution or exact permission occurs in this function. |
| Caller | [`turn_evidence.go:133–154`, `gatherWorkspaceEvidence`](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/turn_evidence.go#L133-L154) | Calls that helper directly for selected targets. Duplicate-path avoidance is not an authorization check. |
| Resolver | [`internal/tools/workspace_guard.go:143–232`, `ResolveWorkspacePath`](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/tools/workspace_guard.go#L143-L232) | Canonical/symlink-resolved containment, not secret classification or permission. The file is identical on both pins. A protected file genuinely inside the root passes containment. |
| Delivery | `processInput`, prompt rendering references above; [`turn_evidence.go:160–185`, `renderEvidence`](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/turn_evidence.go#L160-L185) | Whole and partial bodies are placed in the answering prompt. Fencing preserves content presentation; it does not withhold protected content from the provider. |

The conclusion is narrower than “CodeNERD has no safety.” This is an alternate host acquisition path not using an existing protection measurement and exact-admission path. A Mangle-derived decision that evidence is needed does not establish permission for every concrete target later selected by perception. Conversely, fixing it does not require returning executive routing decisions to Go.

### 3. Source-derived witness and limits

A proposed deterministic witness uses a temporary workspace and a harmless sentinel in a file whose name matches the configured secret policy. No real credentials are needed.

The conditions are:

1. The protection pattern is active and matches the target; it was not intentionally disabled by an explicit empty configured list.
2. The file is inside the canonical workspace, exists, and is readable by the host process.
3. Perception supplies that file as `ScopeFile` or a suitable path-like target; the query reaches the gather-evidence lane because no higher-priority lane takes it.
4. The evidence budget is positive and includes the sentinel. The file can be smaller than the budget, so truncation is not needed for the witness.
5. The turn continues to articulation rather than returning through an intervening system-result branch, and the answering request is admitted by its provider/broker path.

Under those conditions the inspected implementation reads the bytes and puts the body into the answering prompt without the existing secret-path check. The existing real-kernel, scripted-transducer test provides a useful entry-path scaffold, but it uses an ordinary `app.log`, not a protected-name fixture. This is a source deduction; it is not a test result or a claim that an attacker can force any arbitrary perception outcome.

The risk is provider-bound input disclosure relative to the repository's own contract. The model need not echo the sentinel for that boundary to be crossed. This study does not establish provider retention, training use, onward disclosure, an external adversary, or an incident involving the user's data. A local provider changes the exposure destination but not the stated path-protection discrepancy.

Important counterexamples:

- An outside absolute path, traversal escape, or symlink resolving outside the workspace remains rejected by the shared resolver. Do not refile the already repaired containment gap.
- A delegated query can use the governed session executor; this finding is not applied to every reviewer/researcher run.
- Missing files, directories and zero-byte budgets yield unavailable observations. The defect is not that every target succeeds.
- An explicitly empty secret-pattern configuration deliberately changes what is protected; the default-policy witness must not silently assume otherwise.
- A protected-file denial should remain unavailable evidence, not a reason for the model to try another alias, slash command, generated helper or copy operation.

### 4. Main and explicit `/read` are different cases

On main, `executeInfoGatheringActions` calls the older containment-only `readFileContent`, then emits `file_content`; the helper does not call the secret matcher. However, main's evidence-delivery gap remains relevant: this alone is not proof that this local fact body reaches the answering provider on every residual turn. [`cmd/nerd/chat/process_helpers.go:83–112`, `executeInfoGatheringActions`](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/cmd/nerd/chat/process_helpers.go#L83-L112); [`cmd/nerd/chat/helpers_files.go:90–106`, `readFileContent`](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/cmd/nerd/chat/helpers_files.go#L90-L106)

Both pins also retain the explicit `/read` path. `handleCmdRead` reads via `readFileRange`, writes the body as a **user** message, adds a question, and starts `processInput`. `readFileRange` performs containment and opens the file without the secret matcher. [`cmd/nerd/chat/commands_handlers_analysis.go:254–316`, `handleCmdRead`](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/commands_handlers_analysis.go#L254-L316); [`cmd/nerd/chat/helpers_files.go:96–109`, `readFileRange`](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/helpers_files.go#L96-L109)

That is not the same authority case as inferred evidence acquisition. A person explicitly naming a file may intend to share its contents. The product must decide whether `/read` is subject to the documented unconditional protected-file rule or is a distinct user-upload operation with a visible sharing contract. The current code's conversion into user-history content cannot by itself settle that policy question. Do not describe an explicit operator action as an autonomous exploit, and do not infer from it that a natural-language routing fallback may make the same exception.

This is also not a local-only preview: `addMessage` appends the supplied message without content filtering, and in the uncompressed case `processInput` copies history message content into perception history; the answering context likewise receives history. Provider exposure can therefore occur before a final answer. [`cmd/nerd/chat/model_handlers.go:28–43`, `addMessage`](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/model_handlers.go#L28-L43); [`cmd/nerd/chat/process.go:141–156,189–200`, `processInput`](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/process.go#L141-L200); [answering history context, lines 883–904](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/process.go#L883-L904)

### 5. Reconcile prior coverage rather than duplicate it

- **This audit's §7.2 step 6** already specifies secret-path and exact-permission preservation. The new source check refines TUI-EVID-02/03 with an explicit authority-parity exit. It must not be marketed as a newly invented safety principle.
- **GAP-CLI-CHAT-READ-CONTAINMENT** explicitly requires outside/traversal/symlink rejection and in-root success. Its implementation claim is accurate within that scope; it does not prove every in-root file is permitted. [`Docs/architecture/cli/03-GAP-ANALYSIS.md:45–51`, “Chat information read containment”](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/Docs/architecture/cli/03-GAP-ANALYSIS.md#L45-L51)
- **Existing secret-path work** provides the matcher, configuration semantics and session measurement. Reuse those owners. Copying the pattern list into chat would create another drift-prone protection policy.
- **Tool effect receipts** concern retaining actual effects/validation after dispatch. This finding is before disclosure admission; a receipt recording that a protected body was sent is not prevention.
- **Request-visible tool contracts** concern the model's usable catalog. Describing a tool correctly does not authorize a host helper that bypasses its execution path; granting more tools is not this repair.
- The feature tests for whole/prefix/rune handling, unavailable evidence and actual prompt sentinels are valuable. `TestReadEvidenceFile_UnavailableIsTyped` uses an outside file named `evidence-outside.txt` with a harmless body string, not an in-workspace secret-policy target. That is containment coverage, not secret-path coverage. [`cmd/nerd/chat/turn_evidence_test.go:14–72,100–128,232–264`](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/cmd/nerd/chat/turn_evidence_test.go#L14-L72)

### 6. Proposed repair contract — not implemented here

Treat **need for evidence**, **authority to acquire**, **what was acquired**, and **authority to deliver/reuse** as separate facts. The existing observation shape should carry the disposition rather than become a grant.

1. Resolve the concrete source under the existing canonical workspace identity. Apply existing configured secret-path semantics to the requested and resolved source identity as appropriate; define and test alias handling explicitly. Do not weaken containment or redefine the pattern configuration.
2. Route host acquisitions through an existing governed read service or factor a reusable admission boundary from current execution. Preserve applicable capability and exact `permitted(Action, Target, Payload)` checks. A host API must name its principal/call/turn ownership; a comment saying “host read” is not authority.
3. Make admission precede opening/reading protected contents, fact publication and prompt/history insertion. A failed or unavailable policy check yields a typed unavailable/denied observation with no body, not a fallback raw read.
4. Keep Mangle responsible for permission and continuation. Go supplies canonical identities, measurements and bounded IO. The model chooses neither an exemption nor a weaker backend after refusal.
5. Preserve the feature's bounded observation and actual prompt delivery for permitted ordinary files. Do not solve the issue by reverting to a tool-less final answer or silently withholding all evidence.
6. Bind retained observations to source and access-policy identity where reuse is supported; recheck authorization before later delivery if policy can change. Do not require a redundant reread merely to prove delivery. No new durable ledger is needed for the immediate pre-read parity repair.
7. Decide explicit `/read` semantics separately. If it is policy-bound, deny before the body enters user history or any perception call. If a future distinct user-sharing operation is approved, its explicit consent, scope, visible destination and retention contract must not be reusable by inferred evidence acquisition. No such exception is granted by this proposal.
8. Keep denial diagnostics useful without including the protected body. Do not log raw evidence in an additional receipt store. If blocked evidence is required for the answer, report the missing scope and ask for a sanitized alternative rather than claiming investigation succeeded.

A minimal patch that invokes the existing secret matcher before these reads is a reasonable containment of the demonstrated omission, but it does not on its own establish general exact-permission parity. Compare it with the reusable governed-read boundary; do not conflate their completion claims. Prompt instructions, output-only redaction, and a larger context budget do not prevent protected input from reaching the provider.

### 7. Principles and literature

This is an application of complete mediation and fail-safe defaults, not an LLM-specific security invention. Saltzer and Schroeder require checking authority across access paths and caution about remembered authorization when authority changes. Applied here, a new evidence-delivery adapter must not omit a protection check that the normal executor performs. [Saltzer and Schroeder, “The Protection of Information in Computer Systems,” §I.A.3](https://web.mit.edu/Saltzer/www/publications/protection/Basic.html)

OWASP's LLM02:2025 guidance calls for least-privilege access and restricting model data sources, and warns that prompt restrictions alone are not dependable protection. Applied here, controlling source admission is more relevant than telling the answering model not to repeat protected material after supplying it. These references motivate design; they neither demonstrate an exploit in CodeNERD nor establish provider training or retention behavior. [OWASP, “LLM02:2025 Sensitive Information Disclosure”](https://genai.owasp.org/llmrisk/llm022025-sensitive-information-disclosure/)

### 8. Cost and efficiency accounting

The goal is successful grounded investigation within the same authority boundary, not fewer reads at any price. Measure separately:

- Host canonicalization, pattern checks and exact-policy evaluation CPU/latency; bytes read, allocations and observation serialization.
- Provider calls and input/output/cache tokens across perception, answering, retries and follow-on work, including denied cases.
- Correct authorized answers, correctly blocked protected targets, false refusals, missed protected disclosures and useful sanitized alternatives.
- End-to-end time, human repair turns and policy/diagnostic maintenance burden.

For P configured patterns, the direct matcher performs bounded path-pattern checks per source; its actual cost depends on path and pattern lengths. Reusing exact admission adds policy work but need not add an LLM turn. Delegating every trivial read to a full agent may introduce extra setup, model and catalog cost. No latency, token savings or risk reduction percentage is asserted here.

Compare: **A**, current feature; **B**, immediate shared secret-matcher checks plus retained evidence delivery; **C**, unified governed read admission with the same delivery. Keep workspace, policy, model/version, budgets and tasks fixed. A protected-target refusal is not a failed task if the policy forbids the requested data; scoring should reward an honest blocker and safe alternative, not penalize a system for declining unauthorized disclosure. B is cheaper to integrate but has a narrower guarantee; C is preferable only if its wider contract and ownership justify its cost.

### 9. Proposed validation and acceptance — not run

Use only temporary synthetic fixtures and capture-only provider adapters. Never use real credentials or invoke a remote model to test whether protected bytes escape.

| Fixture/journey | Required observation |
|---|---|
| In-root default protected filename with synthetic sentinel | No body acquisition/publication/delivery; typed policy denial. Ordinary neighbor file still answers correctly. |
| In-root custom protected pattern | Same behavior under configured patterns, not a hardcoded default-only list. |
| Absent versus explicitly empty protection list | Preserve documented default versus explicit-empty semantics; do not invent protection where policy intentionally disables it. |
| Natural-language evidence query, no candidate and below-threshold candidate | Real gather-evidence entry path reaches the same admission result as a session read; no bypass from routing confidence. |
| Residual `/fs_read` path | Same denial; no `file_content`, observation body or provider sentinel. |
| Exact permission denied for a nonsecret ordinary file | No read. This distinguishes full admission parity from a secret-pattern-only patch. |
| Missing kernel/gate or failed admission measurement | Explicit unavailable/denied outcome, no raw-read fallback. |
| Allowed ordinary whole, partial and unavailable reads | Keep existing coverage labels, bounded IO and answering-request sentinel assertions. |
| Absolute/traversal/outside symlink plus inside alias to protected target | Preserve containment; separately specify and verify canonical secret-target semantics. |
| Explicit `/read` | Validate the chosen product contract before both perception-history and answer delivery, not only final output. |
| Policy change before retained-observation reuse | Current denial prevents disclosure; stale authorization is not reused. |
| Denied initial read followed by suggestion to use another backend | The denial cannot be laundered into permission through `/read`, copy, generated tool or a raw helper. |

Assertions should observe the **actual provider request**, not the model's output. They should also instrument the acquisition boundary to distinguish “not read” from “read then hidden,” and inspect only synthetic sentinel membership in facts/history/retained observations. Keep the existing production-entry sentinel test and add denied-source variants; helper-only coverage is insufficient.

Refine the existing queue rather than creating a competing project:

| Existing research ID | Added exit |
|---|---|
| TUI-EVID-02 | An ordinary permitted body reaches the answering request; a protected/denied body does not enter acquisition, facts, user history or provider input through any supported evidence adapter. |
| TUI-EVID-03 | Gather-evidence is a total **authority-aware** disposition: permitted observation, explicit partial, unavailable or denied; the no-candidate fallback never grants broader read scope than the governed executor. |
| TUI-EVID-05 | Grounded authorized answers and safe follow-on work improve without a policy regression, measured at full end-to-end cost. |

Before implementation, reconcile the CLI/read adapter and session safety specifications and settle `/read` semantics. Do not change protected safety guidance or widen rules merely to make an evidence test pass. The user-visible guarantee is: **the main chat can investigate and use the evidence it is allowed to see, and a routing fallback cannot quietly turn a protected file into model input.**
