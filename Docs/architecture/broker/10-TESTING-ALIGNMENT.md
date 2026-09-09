# Testing alignment

`go test ./internal/broker/` — 0.4s. `go test -race ./internal/broker/` — 4.9s.
Statement coverage 76.5%.

## Tiers

| Tier | Files | Proves |
|---|---|---|
| Unit | `calibration_test.go`, `ledger_test.go`, `measure_test.go` | the arithmetic, its bounds, and its degenerate inputs |
| Contract | `wrap_test.go` | the wrapper's capability surface equals the underlying's, in all eight shapes |
| Integration | `broker_test.go`, `stream_test.go` | admit → dispatch → settle end to end, including late settlement |
| Integrity | `integrity_test.go` | what is charged, and that segments sum |
| Provider | `counter_anthropic_test.go` | the real endpoint contract, via `httptest` |
| Static audit | `wiring_test.go` | properties of the *rest of the codebase* that this package depends on |
| Race | all of the above under `-race` | concurrent admission, recording, and calibration |

## The tests that matter most

**`TestWrapPreservesCapabilityMatrix`** is the load-bearing one. It checks both
directions — gained and lost — for three interfaces across five client shapes. A
wrapper that gains `ToolResultsProvider` routes Gemini into a native tool loop it
does not implement, and the symptom appears as tool calls silently going nowhere,
nowhere near this package.

**`TestEveryClientConstructionPathIsBrokered`** parses
`client_factory.go` and requires every exported constructor returning an
`LLMClient` to route through metering. It found three that did not on its first
run, one of which — `NewImageClientFromUserConfig` — was genuinely spending off
the books. It also asserts that the in-file helper it permits delegation to is
itself metered, so the allowance is not a hole.

**`TestNoDoubleCounting`** covers the specific mistake this design invites: the
observer and the response's usage block describe the same call, and adding them
would double every tool-call turn in the system.

**`TestAbandonedStreamDoesNotLeakGoroutines`** starts twenty streams, reads one
chunk from each, cancels, and watches the goroutine count. Without the drain in
`stream.go` the forwarders block forever on a send nobody will receive.

## Testing conventions

Tests build isolated meters via `testMeter` rather than touching
`broker.Default()`. The process meter is shared by design; tests that trained a
shared calibrator key would leak into one another. Where a test must use the
process calibrator (the `internal/context` counter tests), it uses a model name
unique to that test.

Streaming settles on a goroutine, so streaming assertions poll through `waitFor`
rather than racing the thing they assert.

Comment-stripping in the static audits matters: several removed identifiers are
deliberately named in comments explaining why they are gone, and a textual scan
that could not tell the difference would force the explanation out of the
codebase.

## Regression surface outside the package

Replacing the counter and wrapping the factory touched seven packages. All were
run: `internal/usage`, `internal/context`, `internal/session`,
`internal/perception`, `internal/system`, `internal/articulation`, `internal/init`.

`internal/perception`'s factory tests asserted on concrete client types and
failed — correctly. They were updated to use `broker.Base`, which is what the one
production site doing the same thing now does. Relaxing them to interface checks
would have been the wrong fix: they are the reason the production break was found.

Full suite: 78 packages ok, 0 failures.

## Not covered

- A live provider. `counter_anthropic_test.go` uses `httptest`; it pins the
  contract as understood, not the contract as served.
- Cross-process ledger behaviour. Out of scope by design.
- Long-horizon calibration drift over a multi-hour session. The unit tests cover
  convergence and outlier resistance; they do not cover a content-mix shift after
  ten thousand observations.
