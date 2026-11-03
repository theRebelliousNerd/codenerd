---
title: "MCP stdio: outbound backpressure must not own cancellation"
doc_class: audit
subsystem: mcp
status: proposed
last_verified: 2026-11-02
source_revision: 7931fdfd0425d37c41955f57e47f44f77205cc29
method: static source inspection
supersedes: []
---

# MCP stdio: outbound backpressure must not own cancellation

## Finding and decision

A configured, successfully initialized stdio MCP server can retain its stdin pipe without continuing to read it. If a subsequently admitted request cannot fit in the available pipe buffering, CodeNERD's synchronous write can wait while holding the transport's state mutex. The request's context is checked only after that write returns. Disconnect and reader-error teardown require the same mutex before they can kill the child or close stdin. Thus cancellation can be requested without returning the operation or reaching the mechanism that would unblock it. [Request write and cancellation](https://github.com/theRebelliousNerd/codenerd/blob/7931fdfd0425d37c41955f57e47f44f77205cc29/internal/mcp/transport_stdio.go#L307-L363); [Disconnect and abort](https://github.com/theRebelliousNerd/codenerd/blob/7931fdfd0425d37c41955f57e47f44f77205cc29/internal/mcp/transport_stdio.go#L149-L207).

This is a conditional availability and resource-lifecycle gap, not an authorization bypass or a reproduced incident. The strong witness requires a request large enough relative to **available** buffering, a live non-draining reader, and an operation that actually passes the existing authority boundary. It does not require a malicious server. No fixed pipe capacity, production frequency, or measured latency is asserted.

**Decision proposed:** make cancellation and session teardown independent of a potentially blocked outbound write, while preserving serialized JSON-RPC framing, exact authorization, connection identity, and honest uncertain-effect outcomes. Returning promptly from a wrapper is not sufficient if an owned writer, child, or manager lock remains blocked.

This audit is static at the pinned revision. Existing source, guidance, tests, and prior audit status were inspected; no tests, model inference, application execution, or repository changes were performed.

## What is already repaired—and remains relevant

The [wire-fidelity audit's shipped section](https://github.com/theRebelliousNerd/codenerd/blob/7931fdfd0425d37c41955f57e47f44f77205cc29/Docs/audits/mcp-versioned-wire-fidelity.md#L174-L187) records real changes: conforming initialization and result decoding, bounded incoming messages, reader-exit teardown with a cause, and clean reconnect state. This finding does not reopen the original “reader exits without waking pending calls” defect. It identifies an outbound lock owner that can prevent the repaired reader from obtaining the lock needed to complete that wakeup.

The [capability-admission audit](https://github.com/theRebelliousNerd/codenerd/blob/7931fdfd0425d37c41955f57e47f44f77205cc29/Docs/audits/mcp-capability-admission.md#L129-L135) already notes that the authority mutex spans dispatch and warns against replacing consistency with a check/use race merely to reduce lock latency. The present proposal consumes the shipped authority path unchanged; it supplies a concrete transport condition that can extend that lock retention beyond cancellation.

Older QA also discusses generic cancellation and server-output pipe exhaustion. In particular, [the earlier stdio boundary analysis](https://github.com/theRebelliousNerd/codenerd/blob/7931fdfd0425d37c41955f57e47f44f77205cc29/.quality_assurance/mcp/2026-05-14_05-00-00-AM-EST_mcp_client_manager_boundary_analysis.md#L274-L283) discusses the server blocking while writing stdout. This audit is narrower: the **client's stdin writer** owns the lock required by its own cancellation/teardown paths. Pipe backpressure itself is not novel.

Follow current source over stale architecture claims. Root guidance explicitly warns that historical architecture prose may be outdated; system guidance distinguishes cooperative shutdown/join from elapsed cleanup timers. [Source-over-stale-docs guidance](https://github.com/theRebelliousNerd/codenerd/blob/7931fdfd0425d37c41955f57e47f44f77205cc29/agents.md#L60-L65); [Resource ownership guidance](https://github.com/theRebelliousNerd/codenerd/blob/7931fdfd0425d37c41955f57e47f44f77205cc29/internal/system/agents.md#L19-L23).

## Configured execution path, not a transport-only hypothetical

| Boundary | Current implementation and condition |
| --- | --- |
| Configuration | Enabled integrations can declare `protocol: stdio`, `endpoint`, and schema-bound reviews. Conversion now preserves endpoint and headers; older claims that endpoint is unreachable from config are stale. [Integration fields](https://github.com/theRebelliousNerd/codenerd/blob/7931fdfd0425d37c41955f57e47f44f77205cc29/internal/config/integrations.go#L27-L51); [conversion](https://github.com/theRebelliousNerd/codenerd/blob/7931fdfd0425d37c41955f57e47f44f77205cc29/internal/config/integrations.go#L168-L209). |
| Boot wiring | `initMCPBridge` constructs the bridge, binds the approval broker and VirtualStore adapters, installs the model-facing control plane, then starts connection in the background. [Actual factory path](https://github.com/theRebelliousNerd/codenerd/blob/7931fdfd0425d37c41955f57e47f44f77205cc29/internal/system/factory_mcp.go#L18-L54). |
| Connection | Enabled auto-connect entries reach manager `Connect`, which selects `NewStdioTransport(cfg.Endpoint)`. The transport launches a configured local command and performs initialization. Successful connection/discovery is a precondition of the primary witness. [Auto-connect](https://github.com/theRebelliousNerd/codenerd/blob/7931fdfd0425d37c41955f57e47f44f77205cc29/internal/mcp/client.go#L160-L177); [transport selection](https://github.com/theRebelliousNerd/codenerd/blob/7931fdfd0425d37c41955f57e47f44f77205cc29/internal/mcp/client.go#L260-L295); [child and handshake](https://github.com/theRebelliousNerd/codenerd/blob/7931fdfd0425d37c41955f57e47f44f77205cc29/internal/mcp/transport_stdio.go#L82-L146). |
| Model-facing call | Registered `mcp_call` executes `plane.Call(ctx,...)`. The plane resolves the tool, validates arguments, checks risk, and calls the manager. [Verb registration](https://github.com/theRebelliousNerd/codenerd/blob/7931fdfd0425d37c41955f57e47f44f77205cc29/internal/tools/mcpctl/register.go#L5-L28); [call tool](https://github.com/theRebelliousNerd/codenerd/blob/7931fdfd0425d37c41955f57e47f44f77205cc29/internal/tools/mcpctl/tools.go#L109-L157); [control-plane route](https://github.com/theRebelliousNerd/codenerd/blob/7931fdfd0425d37c41955f57e47f44f77205cc29/internal/mcp/controlplane_invoke.go#L86-L124). |
| Authority | The manager freezes arguments and routes the invocation through exact remote admission. Current reviews, schema/registration checks, policy permission, and any required human approval remain prerequisites. Configured reviews are reinstalled after connection rather than silently lost. [Guarded call](https://github.com/theRebelliousNerd/codenerd/blob/7931fdfd0425d37c41955f57e47f44f77205cc29/internal/mcp/client.go#L668-L698); [review preservation](https://github.com/theRebelliousNerd/codenerd/blob/7931fdfd0425d37c41955f57e47f44f77205cc29/internal/mcp/client.go#L350-L363); [final admission and dispatch](https://github.com/theRebelliousNerd/codenerd/blob/7931fdfd0425d37c41955f57e47f44f77205cc29/internal/mcp/remote_authority.go#L546-L579). |
| Transport | Stdio `CallTool` invokes `call(ctx,"tools/call",params)` synchronously. It does not detach the write or install a write deadline. [Tool invocation](https://github.com/theRebelliousNerd/codenerd/blob/7931fdfd0425d37c41955f57e47f44f77205cc29/internal/mcp/transport_stdio.go#L371-L391). |

The endpoint launches an operator-configured process; this is not a claim that arbitrary chat text becomes a local command. The operation must be admissible under the current policy. Refused calls never exercise the outbound witness.

### Timeout and size qualifications

The manager parses the server timeout, but supplies it only to HTTP and SSE constructors. Stdio receives the endpoint alone and relies on the caller context; its `call` does not create a timeout from configuration. The subprocess is constructed with `exec.Command`, not `CommandContext`. Therefore a configured `30s` is not evidence of an independently enforced stdio write deadline. [Constructor arguments](https://github.com/theRebelliousNerd/codenerd/blob/7931fdfd0425d37c41955f57e47f44f77205cc29/internal/mcp/client.go#L260-L285); [Process construction](https://github.com/theRebelliousNerd/codenerd/blob/7931fdfd0425d37c41955f57e47f44f77205cc29/internal/mcp/transport_stdio.go#L82-L118).

The incoming stdout scanner has the wire-message limit. That does not bound outgoing arguments or interrupt an outgoing write. The inspected MCP argument validator deliberately checks required presence and top-level types; a long, otherwise valid string is not rejected by a global outbound byte ceiling there. Canonical argument freezing and request marshaling also serialize the data without such a ceiling. [Incoming scanner](https://github.com/theRebelliousNerd/codenerd/blob/7931fdfd0425d37c41955f57e47f44f77205cc29/internal/mcp/transport_stdio.go#L221-L244); [Argument validation](https://github.com/theRebelliousNerd/codenerd/blob/7931fdfd0425d37c41955f57e47f44f77205cc29/internal/mcp/signature.go#L231-L285); [Argument snapshot](https://github.com/theRebelliousNerd/codenerd/blob/7931fdfd0425d37c41955f57e47f44f77205cc29/internal/mcp/remote_authority.go#L210-L223); [Outgoing serialization](https://github.com/theRebelliousNerd/codenerd/blob/7931fdfd0425d37c41955f57e47f44f77205cc29/internal/mcp/transport_stdio.go#L327-L340).

This is not a promise that every product entry accepts arbitrarily large inputs. The witness requires an admitted payload that reaches this path and exceeds available buffering. A request can be smaller than the incoming wire limit and still satisfy that condition.

## Lock and cancellation sequence

A sufficient source-derived sequence is:

1. An enabled stdio integration connects, completes the supported handshake, and publishes an admissible tool.
2. Its child stops draining stdin while retaining the pipe's read end and remaining alive.
3. A permitted tool invocation enters `StdioTransport.call`. It takes `t.mu`, assigns an ID, registers a buffered pending channel, marshals the request, and writes a newline-terminated frame.
4. The pipe cannot accept the entire frame without reader progress. The synchronous `Write` remains in progress with `t.mu` held.
5. The caller context expires or is canceled. The goroutine cannot enter the later select, so it does not remove the pending entry or return the context error.
6. A concurrent `Disconnect` waits for `t.mu`; it cannot yet reach process kill or stdin close. Reader `abort` likewise waits for that lock if it has a reason to end the stream. The server must resume reading, close the relevant pipe, exit, or be interrupted through another reachable mechanism for the blocked write to settle.

The one-second wait in `Disconnect` only bounds its wait for reader goroutines **after** it obtains the mutex and invokes teardown. It is not a one-second bound on the function from entry. [Teardown ordering](https://github.com/theRebelliousNerd/codenerd/blob/7931fdfd0425d37c41955f57e47f44f77205cc29/internal/mcp/transport_stdio.go#L149-L195).

The same state mutex is needed by response dispatch and connection-state reads. Even an already-written request's context branch reacquires it to remove its pending entry, so another blocked writer can delay that branch too if transport operations overlap. This secondary condition should not be confused with simultaneous admitted manager tool calls, which are themselves serialized by the authority lock. [Response dispatch](https://github.com/theRebelliousNerd/codenerd/blob/7931fdfd0425d37c41955f57e47f44f77205cc29/internal/mcp/transport_stdio.go#L259-L278); [Cancellation cleanup](https://github.com/theRebelliousNerd/codenerd/blob/7931fdfd0425d37c41955f57e47f44f77205cc29/internal/mcp/transport_stdio.go#L343-L362); [Status read](https://github.com/theRebelliousNerd/codenerd/blob/7931fdfd0425d37c41955f57e47f44f77205cc29/internal/mcp/transport_stdio.go#L445-L449).

`notify` also writes under the same mutex and takes no context. Its current initialized notification is small; simply pointing to that notification does not establish a full-pipe witness. The post-handshake large tool request is the stronger bounded case. [Initialization notification](https://github.com/theRebelliousNerd/codenerd/blob/7931fdfd0425d37c41955f57e47f44f77205cc29/internal/mcp/transport_stdio.go#L405-L436).

## Propagation: transport stall versus outer shutdown return

The admitted manager path holds `authorityMu` through invocation, holds the fact emitter's mutex, and retains a final manager read lock while dispatch runs. This prevents registration changes from invalidating the exact authorization during use, but also means the blocked transport retains more than its private lock. Other operations needing those locks—including disconnect—can wait behind it. [Authority lock](https://github.com/theRebelliousNerd/codenerd/blob/7931fdfd0425d37c41955f57e47f44f77205cc29/internal/mcp/remote_authority.go#L280-L290); [emitter lock](https://github.com/theRebelliousNerd/codenerd/blob/7931fdfd0425d37c41955f57e47f44f77205cc29/internal/mcp/remote_authority.go#L410-L420); [manager lock through invocation](https://github.com/theRebelliousNerd/codenerd/blob/7931fdfd0425d37c41955f57e47f44f77205cc29/internal/mcp/remote_authority.go#L546-L579).

Manager disconnect first acquires authority and manager locks, then eventually calls the transport's disconnect. Bridge close calls `DisconnectAll` synchronously before closing its store. Thus a production shutdown can stall before even reaching the transport's kill/close path. This is potential manager-wide contention, not proof that every unrelated subsystem is blocked. [Manager disconnect](https://github.com/theRebelliousNerd/codenerd/blob/7931fdfd0425d37c41955f57e47f44f77205cc29/internal/mcp/client.go#L409-L454); [Bridge close](https://github.com/theRebelliousNerd/codenerd/blob/7931fdfd0425d37c41955f57e47f44f77205cc29/internal/mcp/integration.go#L273-L279).

**Existing outer protection matters.** `Cortex.Close` cancels its MCP boot context and runs `MCPBridge.Close` through an eight-second close-step wrapper. It reports timeout and continues shutdown; a separate bounded wait exists for the boot connection worker. This audit does not claim that Cortex, the TUI, or the whole process necessarily waits forever. [Close-step bound](https://github.com/theRebelliousNerd/codenerd/blob/7931fdfd0425d37c41955f57e47f44f77205cc29/internal/system/cortex_close.go#L16-L20); [MCP shutdown steps](https://github.com/theRebelliousNerd/codenerd/blob/7931fdfd0425d37c41955f57e47f44f77205cc29/internal/system/cortex_close.go#L119-L138).

However, `runCloseStep` launches the closure in a goroutine and selects between its result and a timer. Its timeout branch does not cancel or join that closure. The outer wait can therefore finish with an explicit error while the underlying writer/close work remains unresolved. An eight-second return is evidence of bounded waiting, not evidence that the child, locks, or store have been released. [Wrapper semantics](https://github.com/theRebelliousNerd/codenerd/blob/7931fdfd0425d37c41955f57e47f44f77205cc29/internal/system/cortex_close.go#L364-L384).

## Controls and outcome truthfulness

- When a request fits the available pipe space, the write can finish and the existing response/context select works. A server that merely fails to answer a small request is not sufficient to demonstrate the outbound gap.
- Reader progress, process exit, or closing the read side can release a blocked writer. The new EOF/oversize abort behavior remains useful when it can obtain the mutex. Do not describe all server exits as hanging.
- The manager checks `ctx.Err()` before admission and again before final dispatch. The witness cancels after that check, during writing; it is not an already-canceled manager invocation being deliberately dispatched. [Initial check](https://github.com/theRebelliousNerd/codenerd/blob/7931fdfd0425d37c41955f57e47f44f77205cc29/internal/mcp/remote_authority.go#L280-L284); [final check](https://github.com/theRebelliousNerd/codenerd/blob/7931fdfd0425d37c41955f57e47f44f77205cc29/internal/mcp/remote_authority.go#L541-L549).
- Once a transport error returns, stdio `CallTool` produces `MCPCallResult{Success:false, Error:...}` with a nil Go error. While blocked, it returns neither a result nor an error. The finding is not a false success-counter claim. [Result boundary](https://github.com/theRebelliousNerd/codenerd/blob/7931fdfd0425d37c41955f57e47f44f77205cc29/internal/mcp/transport_stdio.go#L380-L391).
- A timed-out or interrupted write does not prove the server received no bytes or performed no effect. Some bytes may already have entered the pipe; completion may be uncertain. Preserve the existing no-automatic-replay posture and request-bound approval lifecycle. The approved path records dispatch before invocation and finishes its outcome only after invocation returns. [Approval dispatch receipt](https://github.com/theRebelliousNerd/codenerd/blob/7931fdfd0425d37c41955f57e47f44f77205cc29/internal/mcp/remote_authority.go#L566-L579); [Existing cancellation/replay limits](https://github.com/theRebelliousNerd/codenerd/blob/7931fdfd0425d37c41955f57e47f44f77205cc29/Docs/audits/mcp-versioned-wire-fidelity.md#L217-L221).

## Proposed contract and repair direction

**Cancellation must reach the owned blocking resource without first acquiring a lock held by that resource's blocked user.** Give the session an explicit lifecycle owner and a way to interrupt its outbound I/O independently of state publication and manager admission locks. Choose a documented policy for a timed-out write: usually the stream cannot safely be reused if a JSON frame may have been partially sent.

Separate short state transitions from potentially blocking I/O, but retain **one serialized writer per stream**. A dedicated writer with a byte-bounded queue, or another context-aware write owner, is a design option—not a complete fix by itself. Queue admission must be cancellable; queued requests must not be transmitted after cancellation; a blocked writer must be interruptible and joined rather than abandoned.

Preserve connection-generation identity when publishing pending responses and terminating a session. A late cancellation or reader from an older connection must not close a replacement transport. Canceling one request must not silently kill other calls sharing the child without an explicit session-failure policy and honest outcomes for those calls.

Do not simply release the manager's authority locks before transport invocation. They currently protect schema/review/registration consistency. Either keep those protections while making the invocation interruptible, or replace them with an explicitly owned admission/execution protocol whose invalidation and teardown semantics preserve the same authority. This proposal does not authorize weakening the Mangle gate or human approval.

Define separate observations for queued/not sent, writing/possibly sent, waiting for reply, and settled; avoid claiming remote completion from local cancellation. Apply any request-byte budget before expensive repeated serialization where practical. A byte limit reduces memory pressure but cannot establish cancellation: even a small frame can wait behind a full pipe.

Process ownership should cover stopping admission, interrupting blocked I/O, closing the session, and joining owned writer/reader/process cleanup. A goroutine plus a timer that returns while work continues repeats the current outer-wrapper limitation. Process-lifetime cancellation must not accidentally inherit a single request's lifetime for a shared long-lived server.

### Primary contracts

Linux documents finite pipe buffering, blocking writes when space is unavailable, and byte-stream rather than message-boundary semantics. Capacity depends on platform and configuration, not a universal 64 KiB constant. [Linux `pipe(7)`](https://man7.org/linux/man-pages/man7/pipe.7.html). Go's cancellation API signals abandonment without waiting for completion; `StdinPipe` exposes the child-input pipe, and `CommandContext` explicitly supplies process-interruption behavior absent from ordinary `Command`. These support the distinction between signaling, interruption, and joining—not a claim that replacing one constructor alone repairs this design. [Go context](https://pkg.go.dev/context#CancelFunc), [Go StdinPipe](https://pkg.go.dev/os/exec#Cmd.StdinPipe), [Go CommandContext](https://pkg.go.dev/os/exec#CommandContext).

Go's file-write deadline API can affect a currently blocked write, but deadline support depends on the file/platform and a timed-out write may report a positive byte count. The general writer contract likewise permits partial progress accompanied by an error. A repair must account for those bytes and retire or explicitly repair an interrupted frame's stream, not blindly resend the original request. [Go File.SetWriteDeadline](https://pkg.go.dev/os#File.SetWriteDeadline), [Go io.Writer](https://pkg.go.dev/io#Writer).

The implemented legacy MCP profile uses newline-delimited JSON-RPC over child stdio. Its cancellation notifications are optional, may race completion, and may be ignored for requests that cannot be canceled. Sending a cancellation frame over the same blocked writer would not solve local backpressure, nor prove absence of an effect. [MCP 2024-11-05 transports](https://modelcontextprotocol.io/specification/2024-11-05/basic/transports), [cancellation](https://modelcontextprotocol.io/specification/2024-11-05/basic/utilities/cancellation).

## Costs and tradeoffs

| Repair dimension | Cost and failure mode to evaluate |
| --- | --- |
| Dedicated writer/queue | Extra goroutine, synchronization and retained bytes; bounded queues reject or delay useful work. Count bytes, not only requests. Queue cancellation must remove ownership without losing a frame already in progress. |
| Session interruption on write timeout | Restores local liveness but disconnects other pending work and may require reconnect, discovery, analysis, and renewed admission. Remote side effects may remain uncertain. |
| Write deadlines/platform mechanisms | Deadline support and pipe-close behavior need platform validation; do not transplant socket assumptions blindly. A fallback that only abandons the writer leaks the core obligation. |
| Shorter state/authority critical sections | May improve contention but introduces generation, stale-registration and revocation races unless replaced by equivalent ownership. Measure at equal authorization strength. |
| Outbound limits | Reduce allocation/serialization pressure; reject legitimate large tool inputs and require a documented alternative. Limits alone do not fix blocked cancellation. |
| Process cleanup and join | Adds lifecycle state, failure reporting and potentially a bounded escalation policy. Killing a shared child is not equivalent to canceling only one remote action. |
| Diagnostics and receipts | Track wait stage, encoded byte count, cancellation-to-settle latency and incomplete cleanup without logging raw arguments or secrets. More observability has storage/locking cost. |
| Recovery | Reconnect and retry can cost model turns, CPU, I/O and user attention; automatically replaying an uncertain effect is not an acceptable efficiency optimization. |

No measured performance gain is claimed. The main objective is a provable stop/settlement contract. A lower timeout-return latency that leaves the same writer or child alive is not a win.

## Acceptance proposal — all evaluations unrun

Use a controllable subprocess that first completes the supported handshake and discovery, then deliberately stops reading stdin while keeping it open. Exercise the configured bridge and admitted model-facing route, not only a mock `io.Writer`. Use synthetic non-sensitive arguments. Choose encoded request sizes relative to the observed test environment's buffering; do not assume one OS capacity.

1. **Positive control:** a draining child receives complete newline-delimited requests and correct responses; ordinary small-call cancellation remains bounded.
2. **Primary backpressure case:** establish that the writer is pending, then cancel. Observe both caller completion and settlement of writer, pending entry, transport state, owned child cleanup, and relevant manager locks. Do not pass merely because a wrapper times out.
3. **Disconnect during write:** separately invoke direct transport disconnect, manager disconnect, and bridge/Cortex close. Check the eight-second wrapper's honest timeout baseline and whether the repaired underlying operation actually joins.
4. **Collateral work:** retain a second pending request and a second configured server. Measure which operations are delayed or failed, with accurate per-request outcomes; do not assume all calls are independent.
5. **Partial transmission and recovery:** interrupt at different frame-progress points, resume the peer, reconnect, and verify that frames do not interleave and old responses/cancellation cannot mutate a new session. Verify no automatic replay of uncertain effects.
6. **Existing repairs:** preserve handshake ordering, unsupported capability/version refusal, incoming oversize cause, server-exit wakeup, result `isError` handling, reconnect, review/schema admission and approval controls. Existing stdio tests cover several of these, but their helper normally drains stdin; they are not the primary blocked-writer witness. [Inspected transport fixtures](https://github.com/theRebelliousNerd/codenerd/blob/7931fdfd0425d37c41955f57e47f44f77205cc29/internal/mcp/wire_transports_test.go#L292-L440).
7. **Budget/failure controls:** queue saturation, canceled-before-enqueue, marshal failure, child exit, write failure, slow-but-progressing readers, and platform-specific interruption failure. Require explicit incomplete cleanup rather than fabricated completion.

Compare baseline and proposed behavior at identical authority, payload, peer behavior, concurrency, cancellation points and platform. Report cancellation-to-return **and** cancellation-to-settlement, lock-wait tails, retained goroutines/processes/bytes, throughput, legitimate-input refusals, reconnect work and uncertain-effect counts. Keep application/model costs separate from transport timings. A strict input cap or skipped authorization must not make an unfair comparison look faster.

**Acceptance boundary:** backpressure may delay delivery, but it must not prevent the owner from canceling and settling the delivery attempt—or reporting precisely which owned work remains unresolved—without surrendering the existing authority and wire-fidelity guarantees.

