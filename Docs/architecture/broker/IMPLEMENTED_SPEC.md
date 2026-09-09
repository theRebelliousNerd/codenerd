# Implemented specification

> Source boundary: `internal/broker` (see `corpus.toml`).
> Verified 2026-09-09. Every row below has a named test.

## Implementation status

| Capability | Status | Source | Evidence |
|---|---|---|---|
| Single admission/accounting boundary over every LLM call | **IMPLEMENTED** | `broker.go`, `wrap.go`, `internal/perception/client_factory.go` | `TestEveryClientConstructionPathIsBrokered` |
| Provider-backed exact counting (Anthropic) | **IMPLEMENTED** | `counter_anthropic.go` | `TestAnthropicCounterUsesProviderEndpoint` |
| Self-calibrating estimator for other providers | **IMPLEMENTED** | `calibration.go`, `counter.go` | `TestCalibrationConvergesOnObservedRatio` |
| Confidence label on every count | **IMPLEMENTED** | `types.go` | `TestConfidenceOrdering`, `TestAnthropicCounterDegradesOnServerError` |
| One ledger, segment-attributed | **IMPLEMENTED** | `ledger.go`, `measure.go` | `TestLedgerRecordsSpendPerPurposeAndInTotal`, the four segment tests |
| Hard window enforcement with output reserve | **IMPLEMENTED** | `ledger.go` | `TestLedgerRefusesWhenRequestExceedsWindowMinusReserve` |
| Cumulative per-purpose budgets | **IMPLEMENTED** | `ledger.go` | `TestLedgerEnforcesPerPurposeBudget` |
| Fail-closed on uncountable requests | **IMPLEMENTED** | `broker.go#admit` | `TestFailsClosedWhenCounterErrors` |
| `RequireExact` opt-in | **IMPLEMENTED** | `context.go`, `ledger.go` | `TestLedgerHonoursRequireExact` |
| Actual usage captured on every path incl. streaming | **IMPLEMENTED** | `internal/usage/observer.go`, `broker.go#settle` | `TestReceiptRecordsProviderActuals`, `TestStreamingSettlesWhenTheStreamEnds` |
| No double counting | **IMPLEMENTED** | `broker.go#settle` | `TestNoDoubleCounting` |
| Calibration fed by real responses | **IMPLEMENTED** | `broker.go#calibrate` | `TestCalibrationFeedsBackFromRealResponses` |
| Capability-preserving decorator (8 shapes) | **IMPLEMENTED** | `wrap.go`, `optional.go`, `passthrough.go` | `TestWrapPreservesCapabilityMatrix` |
| Late settlement for streams, no goroutine leak | **IMPLEMENTED** | `stream.go` | `TestAbandonedStreamDoesNotLeakGoroutines` |
| Receipts with estimate, actual, and error percentage | **IMPLEMENTED** | `receipt.go`, `types.go` | `TestEstimateErrorIsReportedOnTheReceipt` |
| Bounded receipt ring | **IMPLEMENTED** | `receipt.go` | `TestRingSinkKeepsTheMostRecentReceipts` |
| Heuristic counter removed from the tree | **IMPLEMENTED** | `internal/context/tokens.go` | `TestNoCompetingTokenCounters` |
| Orphan budget constants removed | **IMPLEMENTED** | session / init / articulation | `TestOrphanBudgetConstantsAreGone` |
| Prompt budgets derived from the ledger | **IMPLEMENTED** | `text.go#PromptBudget` | `TestPromptBudgetDerivesFromTheLedger` |
| Meter configured from workspace config at boot | **IMPLEMENTED** | `internal/system/broker_meter.go` | `TestBrokerIsInstalledAtBoot` |
| Latency recorded | **IMPLEMENTED** | `broker.go#settle` | `TestReceiptRecordsProviderActuals` |
| Latency priced into decisions | **NOT IMPLEMENTED** | — | see P11, and Phase 4 preconditions |
| Lossless native round-trip | **NOT IMPLEMENTED** | — | gap B11 |
| Typed task graph, obligation-driven selection | **NOT IMPLEMENTED** | — | Phase 2 |
| Economic layout transitions | **NOT IMPLEMENTED** | — | Phase 4, gated on Gate A |
| Selective lanes | **NOT IMPLEMENTED** | — | Phase 5, gated on Gate A |

## Verification receipts

```
go build ./...                       exit 0
go vet ./internal/broker/            exit 0
go test ./internal/broker/           ok       0.44s   coverage 76.5%
go test -race ./internal/broker/     ok       4.85s
go test ./...                        78 ok, 0 FAIL, 3 no-test-files
```

## Deliberately not built

Each of these was considered and rejected for this change set, with a reason:

- **Provider-native compaction, context editing, `clear_at` reminders.** Model-
  and beta-gated. Fine as optimizations; dangerous as load-bearing structure.
- **Non-prefix KV reuse (CacheBlend, KVFlow, KVCOMM).** Requires serving-engine
  control. Not a portable hosted-API capability, and not a dependency of anything
  here.
- **Per-request counting of every segment via separate API calls.** Four round
  trips per turn to replace a proportional split that is only used for
  attribution.
- **A cross-process ledger.** `internal/usage` already owns cross-process cost.
  Duplicating it here would be the exact disease this package treats.
