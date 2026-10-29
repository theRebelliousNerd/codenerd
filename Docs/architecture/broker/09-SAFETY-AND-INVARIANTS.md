# Safety and invariants

## Invariants

| # | Invariant | Enforced by |
|---|---|---|
| I1 | No inference call reaches a provider without passing admission | `TestEveryClientConstructionPathIsBrokered`, `TestLegacyClientPathIsMetered` |
| I2 | A request whose size cannot be determined is refused | `TestFailsClosedWhenCounterErrors`, `TestLedgerFailsClosedOnUncountableRequest` |
| I3 | A refused request never reaches the provider and records no spend | `TestRefusedRequestNeverReachesTheProvider` |
| I4 | Spend is recorded exactly once per call | `TestNoDoubleCounting`, `TestRepeatedProviderReportsForOneCallAccumulate` |
| I5 | Only provider-reported numbers become spend | `settle` is the sole writer; `TestReceiptRecordsProviderActuals` |
| I6 | A count is never labelled stronger than its evidence | `TestAnthropicCounterDegradesOnServerError`, `TestAnthropicCounterRejectsNonsenseTokenCounts` |
| I7 | The wrapper's capability surface equals the underlying's | `TestWrapPreservesCapabilityMatrix` |
| I8 | Counting happens at the outbound boundary | `TestCountedAtOutboundBoundary` and the three segment tests |
| I9 | Decorator traversal terminates | `TestBaseTerminatesOnCyclicUnwrap` |
| I10 | Concurrent use is race-free | `TestLedgerIsRaceFree`, `TestCalibrationIsRaceFree`, and `-race` on the package |
| I11 | An abandoned stream leaks no goroutines | `TestAbandonedStreamDoesNotLeakGoroutines` |
| I12 | One learned ratio per model, clamped to a sane band | `TestCalibrationClampsIntoSaneBand` |

## Security properties

**The broker is not an authorization boundary.** It decides whether a request
fits a budget. It does not decide whether an action is permitted — `permitted/3`
in the Mangle kernel remains the sole authority, and nothing here can widen it.
A receipt explains a decision; it never becomes executive truth.

**Credentials.** `AnthropicCounter` holds an API key to reach the counting
endpoint. It is the same key the client already holds, is never logged, and
appears in no receipt. `LogSink` writes purpose, provider, method, counts, and
timing — never prompts, tool arguments, or user content.

**A CLI-backed engine is not its API provider.** `brokerCredsFor` maps
`engine=claude-cli` to the engine rather than to `anthropic`, even when an API
key is present for fallback. Pointing the counter at `api.anthropic.com` would
count a request that is not the one being made, and would send workspace content
to an endpoint the user chose a CLI specifically to avoid.

**Undercounting is the dangerous direction.** A tool schema that will not
marshal is charged `unmarshallableSchemaChars` rather than zero. A request
admitted on a count that omitted a schema entirely is rejected by the provider
*after* transmission, and billed.

**Untrusted content is data, never instruction.** Tool results and message text
reach this package only as things to measure. Nothing in a counted request is
interpreted, and no field of a provider response other than its numeric usage
influences any decision here.

## Failure posture

| Failure | Behaviour | Rationale |
|---|---|---|
| Counting endpoint down, slow, or unauthorized | degrade to the calibrated estimator, lower `Confidence` | an exact number that arrives after the user gave up is worse than a good estimate now |
| Counter returns an error | refuse the request | a budget that cannot be checked is not enforced |
| Window not configured yet | admit, report zero headroom on every receipt | "the limit is unknown" is not "the check passed", and refusing during boot would brick startup |
| Provider reports no usage | record the response's usage block if present, else nothing | never invent a number |
| Provider reports absurd usage | rejected by the ratio clamp | one bad sample must not poison the session |
| `Wrap` cannot meter | construction fails | returning a bare client silently restores un-metered inference |
| Consumer abandons a stream | forwarders drain and exit; the receipt still settles | otherwise the observer, the receipt, and the provider's goroutine live for the process lifetime |

## What this package deliberately does not protect against

- **Latency.** Duration is recorded, not priced. See P11 in
  [04-ARCHITECTURAL-PRINCIPLES.md](04-ARCHITECTURAL-PRINCIPLES.md); this is a
  hard precondition on any future routing work, not a follow-up.
- **Count fidelity drift.** `AnthropicCounter` builds a counting body that
  mirrors what the client sends, but the two are assembled by different code.
  `TestAnthropicCounterSendsSystemMessagesAndTools` makes divergence loud rather
  than impossible.
- **Cross-process spend.** The ledger is process-scoped. Two concurrent `nerd`
  processes in one workspace each keep their own; `internal/usage` remains the
  cross-process cost record.
