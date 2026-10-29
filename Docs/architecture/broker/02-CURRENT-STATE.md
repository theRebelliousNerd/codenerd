---
doc-class: shipped
subsystem: broker
implementation-status: partial
last-verified: 2026-10-02
verified-against: db1d4b7e24d0f9440c8627b2561ca64a25876954
supersedes: []
---

# Current state and evidence limits

## Independently gated reserve and scanner slice (2026-10-02)

**VERIFIED CURRENT — bounded checkout evidence:** `ledger.go#NewLedger` normalizes negative reserve; `#availableLocked` compares before subtraction, and `#Admit` (line106) enforces positive known windows even with zero availability. Actual wrapper/refusal/spend controls and all broker tests pass in `artifact:.corpus-build/runs/all-features-20261002/round6-broker-auditor.receipt.json`; citation-auditor and strict prompt-validator packages also pass. Full broker race passes in `round6-broker-race.receipt.json`.

`wiring_test.go#scanTokenCounterSource` now parses identifiers instead of banning prose/string fixtures, includes test/broker source and propagates read/parse errors. The full-package scanner and executable-identifier/negative-fixture controls pass. Restoring the old zero-availability bypass makes actual wrapper refusal controls fail in `round6-broker-mutation.receipt.json`; source was restored byte-identically to SHA256 D0FFA23FFB51A8861E5B8A3D7AD2ED4A3DE379DB80125109FA9CC01A62526779. These receipts supersede the unrun author statements only for the named contracts. Factory configuration, fresh binary/provider behavior, concurrent spend reservations and whole-kernel obligations remain open.

**PARTIAL:** the component gates above establish their named contracts. Production caller integration and live acceptance require separate evidence.

| Area | Source witness | Observed boundary |
|---|---|---|
| State | `internal/broker/default.go#Default` (line 78); `internal/broker/ledger.go#NewLedger` (line 52) | Process singleton; copied positive caps; negative constructor reserve normalized to zero. |
| Client shape | `internal/broker/wrap.go#Wrap` (line 32) | Eight combinations of native results/schema/thought streaming; other accessors forward. |
| Count | `internal/broker/counter.go#EstimatingCounter.Count` (line 45); `internal/broker/counter_anthropic.go#AnthropicCounter.Count` (line 88) | Learned estimate or counting API, with labelled fallback. |
| Measure | `internal/broker/measure.go#measure` (line 40) | System/user/history/tools and ordered message blocks contribute. |
| Limits | `internal/broker/ledger.go#Ledger.Admit` (line 93) | Nonpositive count refuses; positive known windows enforce admission even when availability is zero. Unknown windows retain their separate contract. |
| Usage | `internal/broker/broker.go#core.settle` (line 166) | Observer first; response usage fallback; cache/thinking metadata do not independently inflate total spend. |
| Streaming | `internal/broker/stream.go#core.proxyStream` (line 39) | Settlement follows asynchronous forwarding/drain completion. |
| Text fitting | `internal/broker/text.go#Meter.PromptBudget` (line 77) | Zero availability falls back, including known exhaustion. That upstream behavior is outside the accepted fix. |
| Analytics | `internal/broker/epoch.go#prefixFingerprint` (line 47); `internal/broker/reconcile.go#Reconciler.Observe` (line 73) | Prefix/drift observations are not correctness or routing verdicts. |

Testing requirements and assertion witnesses are in [testing](10-TESTING-ALIGNMENT.md). The B-row capabilities and roadmap remain active design obligations in their existing documents. Their presence does not establish current implementation, coverage, or live acceptance; use the scoped evidence above for those claims.
