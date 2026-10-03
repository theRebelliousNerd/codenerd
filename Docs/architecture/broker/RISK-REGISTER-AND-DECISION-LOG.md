---
doc-class: governance
subsystem: broker
implementation-status: partial
last-verified: 2026-10-02
verified-against: db1d4b7e24d0f9440c8627b2561ca64a25876954
supersedes: []
---

# Risk register and decision log

Qualitative risks below reflect inspected source and contract uncertainty. No numerical probability, impact score or measured occurrence is claimed.

| Risk | Evidence | Impact | Disposition |
|---|---|---|---|
| Known exhausted window permits inference | `internal/broker/ledger.go#Ledger.Admit` (line 93) | High: inference can start with no configured input capacity | Local regression and wrapper controls pass; factory and live integration remain open |
| Constructor reserve increases capacity when negative | `internal/broker/ledger.go#NewLedger` (line 52) | High: reserve arithmetic violates intended headroom | Constructor normalizes to zero; preserve regression controls |
| Concurrent requests overshoot purpose limits | `internal/broker/ledger.go#Ledger.Record` (line 139) follows admission | Contract-dependent: settled versus reserved caps | OQ-BROKER-01; no reservation change authorized |
| Global reconfiguration mixes scope/sink identities | `internal/broker/default.go#Meter.ConfigFor` (line 238) | Isolation/lifecycle uncertainty | OQ-BROKER-02 |
| Stuck stream delays settlement | `internal/broker/stream.go#core.proxyStream` (line 39) | Pending spend/resource retention | OQ-BROKER-03 |
| Partial receipt evidence misread as zero inference | `internal/broker/filesink.go#ReadReceiptLog` (line 70) | Acceptance false positive | Couple fake call count, ledger and refusal receipt |
| Target design confused with shipped behavior | [Current state](02-CURRENT-STATE.md) and [vision](01-VISION.md) | Either fabricated implementation claims or loss of intended capabilities | Keep the full target design; qualify current behavior with scoped evidence |

## Decisions

2026-10-02: accept [ADR-001](adr/ADR-001-known-window-reserve-refusal.md) for GAP-BROKER-01 before source implementation. Compare positive known window against normalized reserve before subtraction. Keep unknown-window exception and existing update semantics. Require causal before/after/mutation controls and provider/spend/receipt evidence.

The target design remains in its proper documents. Requirement retirement needs an explicit design decision, not a source-only rewrite or archive migration. Root retains shared registration and runtime acceptance.
