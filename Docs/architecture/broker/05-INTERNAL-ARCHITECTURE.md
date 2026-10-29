# Internal architecture

## Package layout

```
internal/broker/
├── doc.go              # why the package exists
├── types.go            # Confidence, Count, Segments, Request, Purpose, Spend,
│                       #   Decision, Receipt, AdmissionError
├── context.go          # Purpose and RequireExact on the request context
├── measure.go          # the one definition of "how big is this request"
├── calibration.go      # the learned chars-per-token ratio, per model
├── counter.go          # Counter interface + EstimatingCounter
├── counter_anthropic.go# provider count_tokens endpoint + LRU + degradation
├── text.go             # TextCounter and PromptBudget for non-request callers
├── ledger.go           # the single budget authority
├── receipt.go          # ReceiptSink, RingSink, MultiSink, LogSink
├── broker.go           # core: admit -> dispatch -> settle
├── optional.go         # the three capability-gating methods
├── passthrough.go      # unconditional forwarders, and why they are safe
├── stream.go           # late settlement for streamed turns
├── wrap.go             # the eight wrapper shapes, Base, IsBrokered
├── default.go          # the process meter
└── errors.go
```

## The call path

```text
caller
  │  ctx carries Purpose (and optionally RequireExact)
  ▼
broker.core.<Method>
  │
  ├─ build Request from the arguments as they will be sent
  │
  ├─ admit()
  │    ├─ Counter.Count(ctx, req)
  │    │     Anthropic  → POST /v1/messages/count_tokens  → Exact
  │    │     otherwise  → measure() / learned ratio       → Calibrated | Seeded
  │    │     on failure → estimator, with lower Confidence
  │    │
  │    └─ Ledger.Admit(purpose, count, requireExact)
  │          window check → purpose budget check → Decision
  │
  ├─ refused?  emit receipt, return AdmissionError, provider never called
  │
  ├─ install a callObserver on ctx (usage.WithObserver)
  ├─ dispatch to the underlying client
  │
  └─ settle()
       ├─ actual  ← observer (primary) or response usage (fallback)
       ├─ Ledger.Record(purpose, actual)
       ├─ Calibrator.Observe(chars, actual input)
       └─ Sink.Record(receipt)
```

Streamed turns settle on a goroutine when both channels drain, not when the call
returns. Settling at return time would record every streamed turn as free, which
is most of the traffic in chat mode.

## How actual usage is captured

The four `types.LLMClient` methods return no usage. Only `CompleteWithTools` and
`CompleteWithToolResults` carry a `UsageMetadata`, and streaming carries nothing.

Rather than change nine provider clients, the broker taps the plumbing they
already use. Every client calls `trackUsage`, which calls
`usage.TrackFromContext`. A fifteen-line hook in `internal/usage/observer.go`
makes that function also notify an observer installed on the context.

The broker installs a fresh observer per call, so it receives exactly the reports
that call produced:

```go
ctx, obs := c.observed(ctx)
out, err := c.underlying.Complete(ctx, prompt)
c.settle(receipt, req, obs, nil, err)
```

Reports accumulate rather than overwrite. A client that retries internally, or a
streaming turn that reports input at `message_start` and output at
`message_delta`, produces several reports for one logical call — and every one of
them was billed.

The observer is the primary source; a response-carried usage block is the
fallback for clients that do not report through the plumbing. They are never
added together. That is `TestNoDoubleCounting`.

## Calibration

`measure()` is the single definition of request size, in characters. Prediction
divides it by a learned ratio; calibration divides it by the provider's reported
token count. Because both go through `measure()`, any systematic bias in the
framing-overhead constants is absorbed into the ratio and cancels.

The weight is adaptive: `alpha = max(1/(n+1), 0.15)`.

- At `n = 0`, alpha is 1.0 and the first real response replaces the seed
  outright. A slow ramp from a wrong seed means the first several admissions of
  a session are made on a number nobody chose.
- The 0.15 floor keeps the ratio tracking a session whose content mix shifts —
  a prose turn, then a three-thousand-line diff — instead of freezing on early
  history.

Guards, in order of how much trouble they prevent:

| Guard | Prevents |
|---|---|
| ratio clamped to [1.0, 20.0] | one malformed observation poisoning the session |
| samples below 256 chars ignored | fixed framing noise dominating the signal |
| cached tokens added back before observing | a cache hit looking like ten-times-denser content, corrupting the ratio with the optimization that was working |
| non-positive token counts ignored | a partially-populated usage block |

## The eight wrapper shapes

Three optional interfaces select a control flow at their call sites:

| Interface | Chooses |
|---|---|
| `types.ToolResultsProvider` | native multi-turn tool loop vs. single-shot |
| `CompleteWithSchema` | structured output vs. free text |
| `CompleteWithStreamingAndThoughts` | thought streaming vs. plain streaming |

A single wrapper exposing all three would tell every probe "yes", and Gemini —
which deliberately uses a synthesized Piggyback envelope rather than native tool
results — would be routed into a path it cannot serve. The bug surfaces as tool
calls silently going nowhere, far from this package.

So `Wrap` type-switches on the three capabilities and returns one of eight
composite types. Each embeds `baseClient`, which embeds `*core`. That embedding
is safe precisely because `core`'s three gating methods are **unexported**:
`completeWithToolResults` does not satisfy `types.ToolResultsProvider`, so
embedding cannot accidentally advertise a capability the underlying lacks.

Everything else — `SetModel`, `GetModel`, `SetCachedContent`, `SchemaCapable`,
`DisableSemaphore`, the thinking accessors, the full `GroundingController` — is
forwarded unconditionally. The reasoning is in `passthrough.go`: every one of
those is an "enrich if available" read or a setter whose caller leaves its field
at the zero value when the probe fails, so forwarding to nothing is
indistinguishable from not being there.

## Base and the decorator chain

Production stacks decorators: `ScheduledLLMCall → TracingLLMClient → broker →
provider`. A call site asserting on a concrete provider type must reach through.
`Base` walks `Unwrap()` to the innermost client, bounded at 16 hops so a
malformed decorator that returns itself cannot spin forever inside prompt
assembly. `IsBrokered` walks the same chain looking for the unexported marker
interface, which is how the wiring audit proves no construction path escapes.

## The process meter

One `Meter` holds one `Ledger`, one `Calibrator`, and one receipt ring. It is
process-scoped because "one ledger" is the entire point, and codeNERD runs one
workspace per process.

Sharing the calibrator has a second benefit worth naming: every client talking to
a given model contributes observations to one ratio, so the perception tier's
cheap classification calls improve the session executor's admission accuracy for
free.

`Configure` is called once at boot from `internal/system/broker_meter.go`, which
reads `ContextWindow.MaxTokens` and folds `OutputReserve`, `ThinkingReserve`, and
`ToolUseBuffer` into one reserve. Thinking tokens are billed as output and occupy
the same reserve; a model configured for extended thinking with no allowance for
it produces a turn that is admitted and then truncated, which reads as a model
failure rather than a budgeting one.
