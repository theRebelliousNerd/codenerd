# Wiring and what is not built

How metering gets installed into every client, what enforces that it
stays installed, and what the design assumes or leaves unbuilt.

Verified 2026-09-20 against the working tree (commit hash unavailable:
no shell tool in this environment; re-pin with `git rev-parse --short HEAD`).

## Installation: one choke point in perception

`Wrap` lives in the broker package (wrap.go:32) but nobody calls it
directly except through `InstallBroker`
(internal/perception/broker_install.go:21), which every exported
perception constructor routes through: factory call sites at
`internal/perception/client_factory.go:731,750,773`, plus the legacy
direct-ZAI path in `internal/system` (`internal/system/factory.go:928`)
via `InstallBrokerForProvider` (broker_install.go:79-86).

Three properties make "no un-metered inference" structural:

- Idempotency: `IsBrokered` (wrap.go:124) walks the decorator chain,
  so a nested construction returns the client untouched instead of
  double-charging one request (broker_install.go:18-27). The
  decorator-chain test pins this from the outside
  (`internal/system/decorator_chain_test.go:69-92`).
- Fail-closed install: a `Wrap` error (nil client, missing counter)
  surfaces as construction failure, never as a bare client
  (broker_install.go:31-38).
- Per-provider config from the process default `Meter`:
  `broker.Default().ConfigFor` mints the `Config`, `CounterFor`
  selects the counter (broker_install.go:30,41).

Credential scoping has one subtle rule: a CLI-backed engine is not
the API provider even with a key present for fallback — the CLI
shells out and never reaches the API endpoint, so the counter must
not count the request that is not being made
(broker_install.go:60-67).

## Capability preservation: eight shapes, not one

`Wrap` composes one of eight wrappers from the three gating
capabilities (tool-results, schema, streaming-thoughts), probed on
the underlying client (wrap.go:54-72). A single always-capable
wrapper would lie to call-site probes — notably routing Gemini,
which uses a synthesized Piggyback envelope instead of native tool
results, into a multi-turn loop it cannot serve (wrap.go:9-27).

Traversal is centralized: `Walk` (wrap.go:103) with a 16-deep cycle
bound (wrap.go:116-119), `Base` reaching the innermost client for
concrete-type behavioural switches (wrap.go:74-88), `IsBrokered`
marking via the unexported `brokered` interface so outsiders cannot
claim metering without it (wrap.go:121-135). `baseClient`
demonstrably satisfies `types.GroundingController` (wrap.go:214-222).

One control-flow answer rides the forwarders rather than the eight shapes:
`ShouldUsePiggybackTools` (`passthrough.go`). It selects the session's tool
channel, but it is an answer, not a method whose presence is the capability,
so forwarding it with a false default reproduces the unwrapped probe exactly.
Until 2026-09-25 the broker did not forward it, and since every client is
metered, no client in the process was ever treated as Piggyback: the CLI
engines took the native tool path and failed their first continuation
(`TestWrapAnswersThePiggybackQuestionForTheUnderlyingClient`; the whole
session chain is held by
`TestSessionAdapterReportsAnEnvelopeOnlyEngineThroughTheProductionChain` in
`internal/system`).

Forwarding is deliberately dumb: `SetModel` keeps the broker's model
in step for receipts while passing through (broker.go:51-63),
`SetCachedContent` passes through (broker.go:65-70), and
`SchemaCapable` defers to the underlying client so a wrapper can
never re-enable structured output the client switched off
(broker.go:72-81).

## Enforcement is tests, not comments

`internal/broker/wiring_test.go` audits instead of asserting:

- `TestEveryClientConstructionPathIsBrokered` (wiring_test.go:52)
  parses `internal/perception/client_factory.go` and requires every
  exported `LLMClient` constructor to route through metering, with
  exactly three named raw constructors allowed (wiring_test.go:64-68),
  a minimum of 3 audited constructors or the test fails for having
  drifted (wiring_test.go:95-98), and the `newSecondarySlotClient`
  delegation helper independently verified so worker/planner tiers
  cannot pass through a hole (wiring_test.go:100-118).
- `TestNoCompetingTokenCounters` (wiring_test.go:169),
  `TestOrphanBudgetConstantsAreGone` (wiring_test.go:229),
  `TestUsageObserverHookIsWired` (wiring_test.go:304),
  `TestBrokerIsInstalledAtBoot` (wiring_test.go:318),
  `TestMeasurementLogsAreInstalledAtBoot` (wiring_test.go:341),
  `TestLegacyClientPathIsMetered` (wiring_test.go:370), and
  `TestConcreteTypeAssertionsReachThroughTheDecorator`
  (wiring_test.go:384) pin the rest of the invariant surface.

## Not built, assumed, or advisory

- **No cache-rebuild controller.** The epoch machinery measures
  whether prefix caching would pay; nothing in the package acts on
  the answer. See RECEIPTS-AND-EPOCHS.md.
- **Exact counting is single-provider.** Every non-Anthropic backend
  spends on a self-calibrating estimate; cross-provider error
  comparability is not established anywhere in the package.
- **Reconciliation never blocks.** A model can drift past 10% and
  keep spending; the output is one log line per model lifetime.
- **Retention is lossy by design.** Ring (256 default) and rotating
  JSONL both age old receipts out; long-horizon analysis must
  consume the log before rotation.
- **Window enforcement assumes configuration.** Unknown window
  admits with zero headroom rather than refusing.
- **`Segment`/`Histogram` have no in-package caller on the request
  path.** They are pure analysis functions over receipt slices;
  whatever reads them lives outside `internal/broker`.

## Wave 2 reconciliation (2026-09-25, verified against the code)

| Item above | Classification | Evidence |
|---|---|---|
| No cache-rebuild controller | declined (gated) | Phase 4 in `TODO.md`: preconditions are session data (Gate A) and a prompt-ordering decision that wants an eval. |
| Exact counting is single-provider | open, by construction | Only Anthropic exposes a count endpoint the counter uses (`counter_anthropic.go`); other providers estimate and self-calibrate. Nothing to wire without a provider API. |
| Reconciliation never blocks | declined | Drift past 10% warns once per model (`reconcile.go`). Blocking on estimator error would stop work on a measurement defect, not on the task's state. |
| Retention is lossy by design | declined | Bounded ring and rotating JSONL are the design; long-horizon readers consume the log (`nerd meter`). |
| Window enforcement assumes configuration | declined, documented | An unconfigured window admits with zero headroom and every receipt says so (`ledger.go`, `default.go`); refusing would stop every run whose config names no window. Boot configures the window from `GetContextWindowConfig().MaxTokens` (`internal/system/broker_meter.go`) and configures none, silently, when that is zero. |
| `Segment`/`Histogram` have no in-package caller | already wired | `cmd/nerd/cmd_meter.go` (`nerd meter epochs`) calls `broker.Histogram(broker.Segment(receipts))`. |
