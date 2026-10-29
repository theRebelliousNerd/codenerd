# internal/broker

Metered gate in front of every LLM inference call: count the request
before it is sent, refuse it when it exceeds limits, record what the
provider actually billed after it returns.

Verified 2026-09-20 against the working tree (commit hash unavailable:
no shell tool in this environment; re-pin with `git rev-parse --short HEAD`).

## The loop

One hot path, three steps, all in `internal/broker/broker.go`:

1. `core.admit` (broker.go:127) — builds a `Receipt` skeleton stamped
   with `Purpose` (`PurposeFromContext`), `Scope`
   (`usage.SessionIDFromContext`), `Method`, and `Prefix`
   (`prefixFingerprint`), counts via `cfg.Counter`, asks
   `cfg.Ledger` whether the call may proceed. Refusals still return
   the skeleton, so a refusal is recorded, not silent.
2. The underlying client runs with a `callObserver` installed on the
   context (`core.observed`, broker.go:244). Reports accumulate across
   retries and streaming chunks (broker.go:92-115).
3. `core.settle` (broker.go:165) — reconciles observer totals against
   any response-carried usage block, writes spend to the ledger,
   feeds the actual back into the counter, emits the receipt. Its doc
   comment states the invariant: it is the only place spend reaches
   the ledger (broker.go:162-164).

`Config` (broker.go:13-27) makes the dependencies explicit: `Counter`
and `Ledger` are required, `Sink` and `Reconciler` optional.

## Fail-closed by default

- A count error refuses with `DecisionCountUnavailable` plus an
  `AdmissionError` (broker.go:139-150). An unmeasurable request is
  not admitted on a guess.
- `Ledger.Admit` refuses zero/negative counts the same way
  (ledger.go:99-103).
- `Wrap` returns an error instead of a degraded client when there is
  no counter (wrap.go:29-38); `InstallBroker` turns that into a
  construction failure rather than silent un-metered inference
  (internal/perception/broker_install.go:31-38).

## File map

| File | Role |
|---|---|
| `broker.go` | `core`: admit / settle / calibrate / emit; model and capability forwarding |
| `wrap.go` | `Wrap`, capability-preserving wrapper shapes, `Base` / `Walk` / `IsBrokered` |
| `counter.go` | `Counter` / `CalibratingCounter` interfaces, `EstimatingCounter` |
| `counter_anthropic.go` | `AnthropicCounter`: exact provider-side counting |
| `default.go` | Process-wide default `Meter` (`Ledger()`, `Calibrator()`) |
| `calibration.go` | Per-model chars-per-token ratio learning |
| `measure.go` | Single definition of request size; proportional segment split |
| `ledger.go` | Window enforcement and purpose budgets; the only spend authority |
| `reconcile.go` | Advisory drift detection between estimates and bills |
| `receipt.go` | `ReceiptSink` contract; ring / fan-out / log sinks |
| `filesink.go` | Rotating JSONL receipt log; `ReadReceiptLog` |
| `epoch.go` | Epoch segmentation, histogram, provider break-even |
| `types.go` | `Request`, `Count`, `Spend`, `Decision`, `Purpose`, `Receipt` |

How counting works: COUNTING-AND-LIMITS.md. What is recorded and what
it proves: RECEIPTS-AND-EPOCHS.md. How the package is installed into
every client, and what is deliberately not built:
WIRING-AND-NOT-BUILT.md.
