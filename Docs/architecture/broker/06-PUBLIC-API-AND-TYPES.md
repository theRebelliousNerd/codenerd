# Public API and types

## Installing metering

```go
// The normal path. Callers never touch broker directly; the factory wraps.
client, err := perception.NewClientFromConfig(providerCfg)

// The escape hatch, for a client built outside the factory.
client, err := perception.InstallBrokerForProvider(raw, perception.ProviderZAI, model, apiKey)
```

`broker.Wrap(underlying, cfg)` is the primitive. It returns an error rather than
a degraded client when it cannot meter, because a broker that cannot count
enforces nothing.

## Tagging a call

```go
ctx = broker.WithPurpose(ctx, broker.PurposeSession)   // attribute the spend
ctx = broker.WithRequireExact(ctx)                     // refuse rather than estimate
```

`PurposeFromContext` falls back to `PurposeUnattributed`, which is a real
account rather than a discard — unattributed spend is a wiring bug and should be
a growing number somebody notices.

```go
ctx = broker.WithPhase(ctx, broker.PhaseRepair)        // which round of the purpose's work
```

A phase names a round within a purpose: `repair`, `uplift`, `step_plan`,
`forced_final`, `no_tool_retry`, `admission_audit`, `survivors`. `PhaseFromContext` returns
`""` for an ordinary round. The session purpose covers the whole tool loop, so
the phase is what lets a change aimed at one of those rounds be measured on its
own. `nerd meter` lists the named rounds under "Named rounds within a purpose".

## Counting

| Type | Role |
|---|---|
| `Counter` | `Count(ctx, *Request) (Count, error)` |
| `CalibratingCounter` | a `Counter` that also takes `Observe(Observation)` |
| `EstimatingCounter` | measure + learned ratio; used for every provider without a counting endpoint |
| `AnthropicCounter` | `POST /v1/messages/count_tokens`, LRU-cached, degrades to the estimator |
| `Calibrator` | the learned chars-per-token ratio, per model |
| `TextCounter` | bare-string counting for callers that are not sizing a request |

```go
type Count struct {
	Tokens     int
	Segments   Segments   // System / History / User / Tools
	Confidence Confidence // Exact | Calibrated | Seeded
	Source     string     // "anthropic.count_tokens", "estimator:calibrated(3.71)"
	Model      string
}
```

`Segments` is proportional attribution, not four separate measurements: provider
endpoints return one number for the whole request. The total is authoritative.

## Budgeting

```go
type LedgerConfig struct {
	Window        int                // hard provider limit
	OutputReserve int                // held back for the response
	Budgets       map[Purpose]int64  // optional cumulative caps
}

func (l *Ledger) Admit(p Purpose, c Count, requireExact bool) Decision
func (l *Ledger) Record(p Purpose, actual Spend)
func (l *Ledger) Available() int
func (l *Ledger) Account(p Purpose) Spend
func (l *Ledger) Accounts() map[Purpose]Spend  // a copy
func (l *Ledger) Total() Spend
```

`Decision.Code` is one of `admitted`, `window_exceeded`, `budget_exhausted`,
`count_unavailable`, `confidence_too_low`. A refusal always carries a `Reason`
an operator can act on.

## The process meter

```go
broker.Configure(broker.MeterConfig{
	Window:        ctxCfg.MaxTokens,
	OutputReserve: reserve,
	PrimaryModel:  model,
})

m := broker.Default()
m.Ledger()          // the single budget authority
m.Calibrator()      // the shared learned ratios
m.Receipts()        // the recent receipt ring
m.TextCounter("")   // counting for the primary model
m.PromptBudget(0.5, fallback)
```

`NewMeter` builds an independent meter; tests use it to avoid sharing process
state.

## Receipts

```go
type Receipt struct {
	Purpose  Purpose
	Phase    Phase    // "" for an ordinary round
	Provider, Model, Method string
	Started  time.Time
	Duration time.Duration

	Estimated        Count    // what admission believed
	Actual           Spend    // what the provider reported
	EstimateErrorPct float64  // whether the meter can be trusted

	Decision Decision
	Err      string
}
```

`Spend` counts input, output, calls, and two sub-counts of input: `CachedTokens`
(read from a prompt cache) and `CacheWriteTokens` (written to one, billed at a
premium). Input always includes both. Anthropic reports `input_tokens` as the
uncached remainder only, so its API and CLI clients add the cache read and
write back in (`anthropicUsage.promptTokens`).

Sinks: `RingSink` (bounded, in-memory), `LogSink` (debug line per call, warn on
refusal), `MultiSink`, and `ReceiptFunc` for an inline closure.

## Decorator helpers

```go
broker.Base(client)        // reach the innermost client through any decorator chain
broker.IsBrokered(client)  // is metering installed anywhere in the chain
```

`Base` is required at any call site asserting on a concrete provider type.
Production stacks `ScheduledLLMCall → TracingLLMClient → broker → provider`, so a
bare assertion matches nothing.

## Errors

```go
if ae, ok := broker.IsAdmissionError(err); ok {
	// ae.Decision.Code, ae.Decision.Reason, ae.Decision.Headroom
}
```

`ErrNoCounter` and `ErrNoUnderlying` are construction-time failures.
