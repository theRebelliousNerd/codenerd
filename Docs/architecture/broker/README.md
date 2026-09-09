# broker — the single admission and accounting boundary for every inference call

> **VERIFIED CURRENT** on 2026-09-09 against `internal/broker` and its factory,
> context, session, and prompt consumers. `corpus.toml` owns the source boundary;
> [_progress.md](_progress.md) owns test and review receipts.

## In one minute

Before this package existed, codeNERD had 145 LLM call sites across 76 files, and
at least a dozen independent opinions about how many tokens a request costs. The
context compressor budgeted 200,000 tokens. The JIT prompt compiler budgeted
200,000 tokens. Neither subtracted the other. The session executor ignored both
and used a 6-message ring buffer. `internal/session/semantic_compressor.go` had
its own `maxTokens = 64000` that nothing else knew about. Every one of those
numbers was measured with the same ruler: `charsPerToken = 4.0`.

You cannot optimize what you cannot measure, and you cannot measure with a dozen
rulers that disagree.

`broker` is one chokepoint that every inference call passes through. It owns:

- **One token counter**, backed by the provider's own API rather than a divide-by-four
  heuristic.
- **One ledger**, so the prompt, the context block, the history, and the tool
  schemas are charged against the same account.
- **One admission decision**, made before the request leaves the process.
- **One receipt** per call, recording what was actually spent versus what was
  estimated.

The visible outcome is that "how many tokens did that cost" has exactly one
answer, and that answer comes from the provider.

## Its place in codeNERD

The model remains the creative center. The Mangle kernel remains the executive.
`broker` is neither — it is the **meter**. It does not decide what the agent
should think about or what it is permitted to do. It decides whether a request
fits, records what it cost, and refuses to let a caller spend budget it does not
have.

```text
caller (perception / articulation / session / campaign / critic / subagent)
        |
        v
  types.LLMClient  <-- every caller already depends on this interface
        |
        v
  broker.Client    <-- decorator installed at the factory; nothing opts out
        |
   admission -> count -> dispatch -> record -> receipt
        |
        v
  provider client (anthropic / openai / gemini / ollama / zai / xai / ...)
```

`broker` does not own prompt atom selection, context compression, policy
evaluation, tool execution, or provider wire formats. Those live in `prompt`,
`context`, `core`/`mangle`, `tools`, and `perception` respectively.

## The two-clock token counting model

The central design decision is that **token counting has two jobs with two
different accuracy requirements**, and conflating them is why the old heuristic
survived so long.

| Job | When | Source of truth | Accuracy |
|---|---|---|---|
| **Accounting** — what did this actually cost? | After the response | The provider's own `usage` block | Exact |
| **Admission** — will this fit before I send it? | Before the request | Provider count endpoint where one exists; a calibrated estimator elsewhere | Exact for Anthropic; self-correcting elsewhere |

Accounting is easy and was already available: every provider client in
`internal/perception` already reports real usage through `trackUsage`. The broker
captures it.

Admission is the hard half. Anthropic exposes `POST /v1/messages/count_tokens`,
which is exact and model-specific. Most other providers do not offer a free
pre-flight count. Rather than fall back to a fixed constant forever, the broker
runs a **calibrating estimator**: it makes a prediction, then compares that
prediction against the provider's reported actual once the response returns, and
adjusts the model's chars-per-token ratio toward observed truth.

This means the estimator's error shrinks as a session runs. It starts at a
seeded ratio and converges on the real tokenizer behaviour for the specific
model, in the specific language, on the specific kind of content this workspace
actually sends. A fixed 4.0 could never do that, and the old code had no path to
learn because no component ever compared its estimate to reality.

Confidence is reported, never hidden. A count is labelled `exact`, `calibrated`,
or `seeded`, and callers that need a hard guarantee can require `exact`.

## A representative journey

A user asks the session executor to fix a failing test.

1. The JIT compiler assembles a system prompt; the context compressor assembles a
   context block. Both ask the broker how many tokens their candidate content is,
   through the same counter.
2. The executor calls `CompleteWithToolResults` on what it believes is an ordinary
   `types.LLMClient`. It is actually `broker.Client`.
3. The broker counts the outbound request at the real boundary — system prompt,
   every history message, every tool schema — not at whatever internal stage last
   guessed.
4. Admission runs against the ledger for this purpose. Over budget fails closed
   with a typed error naming the account, the request, and the overage.
5. The request dispatches to the underlying provider client.
6. The response's real `usage` is recorded: input, output, thinking, cached. The
   estimator is calibrated against the delta between prediction and actual.
7. A `Receipt` is emitted with both numbers, the confidence label, the admission
   decision, and the account balances.

Nothing about that path is optional and no caller can skip it, because the only
way to obtain an `LLMClient` in this codebase is through the factory, and the
factory wraps.

## What exists today

| Claim | Status | Evidence |
|---|---|---|
| Every `types.LLMClient` construction path returns a brokered client | **VERIFIED CURRENT** | `internal/perception/client_factory.go`; `internal/broker/wiring_test.go#TestEveryClientConstructionPathIsBrokered` |
| Anthropic pre-flight counts come from the provider's `count_tokens` endpoint | **VERIFIED CURRENT** | `internal/broker/counter_anthropic.go`; `internal/broker/counter_anthropic_test.go` |
| Non-Anthropic estimates calibrate against reported actuals and converge | **VERIFIED CURRENT** | `internal/broker/calibration.go`; `internal/broker/calibration_test.go#TestCalibrationConvergesOnObservedRatio` |
| One ledger charges prompt, context, history and tool schemas against one account | **VERIFIED CURRENT** | `internal/broker/ledger.go`; `internal/broker/ledger_test.go` |
| Admission fails closed when the counter cannot produce a count | **VERIFIED CURRENT** | `internal/broker/broker.go#admit`; `internal/broker/broker_failclosed_test.go` |
| Counting happens at the real outbound boundary, after all assembly | **VERIFIED CURRENT** | `internal/broker/broker.go`; `internal/broker/integrity_test.go#TestCountedAtOutboundBoundary` |
| The heuristic `charsPerToken` counter is gone from `internal/context` | **VERIFIED CURRENT** | `internal/context/tokens.go`; `internal/broker/wiring_test.go#TestNoCompetingTokenCounters` |
| Receipts record estimate, actual, confidence, and admission outcome | **VERIFIED CURRENT** | `internal/broker/receipt.go`; `internal/broker/receipt_test.go` |
| Usage is recorded exactly once per call (no double counting) | **VERIFIED CURRENT** | `internal/broker/broker.go`; `internal/broker/integrity_test.go#TestNoDoubleCounting` |
| Lane routing, epoch rebasing, evidence packets | **PROPOSED** | [13-ROADMAP-AND-GATES.md](13-ROADMAP-AND-GATES.md); gated behind measurement that does not exist yet |

## Reading order

New to this corpus: [01-VISION.md](01-VISION.md) for the whole idea set, then
[05-INTERNAL-ARCHITECTURE.md](05-INTERNAL-ARCHITECTURE.md) for how the built part
works, then [13-ROADMAP-AND-GATES.md](13-ROADMAP-AND-GATES.md) for what is
deliberately not built yet and what evidence would unlock it.
