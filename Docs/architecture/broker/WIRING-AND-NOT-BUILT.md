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
