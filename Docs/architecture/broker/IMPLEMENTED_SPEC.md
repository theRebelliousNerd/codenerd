---
doc-class: shipped
subsystem: broker
implementation-status: partial
last-verified: 2026-10-02
verified-against: db1d4b7e24d0f9440c8627b2561ca64a25876954
supersedes: []
---

# Source-observed implementation record

## Root-qualified reserve and source-scanner contracts

**VERIFIED CURRENT:** known positive windows enforce exhausted reserve; negative reserves normalize to zero and subtraction cannot wrap. `internal/broker/ledger.go#Ledger.Admit` at line106 and `#availableLocked` are witnessed by all wrapper/refusal/spend controls in `artifact:.corpus-build/runs/all-features-20261002/round6-broker-auditor.receipt.json`, full broker race in `round6-broker-race.receipt.json`, and a causally reverted bypass that fails those controls in `round6-broker-mutation.receipt.json`. Restored ledger SHA256 is D0FFA23FFB51A8861E5B8A3D7AD2ED4A3DE379DB80125109FA9CC01A62526779.

**VERIFIED CURRENT:** `internal/broker/wiring_test.go#scanTokenCounterSource` distinguishes executable banned identifiers from literals/comments, scans tests and fails on malformed/unreadable source. Its regression fixtures, complete broker, citation-auditor and prompt-validator packages pass in the same root receipt. This is a test-boundary repair, not a production token-counter change. GAP-BROKER-01 remains component-qualified with factory/live integration open; GAP-BROKER-02 is verified at its specified scanner/package boundary. Earlier source-only rows are not expanded into universal provider or portfolio guarantees.

This record separates implementation evidence from the target design. **PARTIAL:** the named root gates establish the reserve and scanner contracts. Source inspection of other mechanisms does not establish live integration or universal provider guarantees.

| Contract | Source evidence and practical limit |
|---|---|
| Construction | `internal/broker/wrap.go#Wrap` (line 32) refuses nil client/counter and supplies an unknown-window ledger when omitted. Capability combinations depend on the underlying client. |
| Counting | `internal/broker/default.go#Meter.CounterFor` (line 223) selects Anthropic counting with provider/key/base URL, otherwise a calibrating estimator. Exact means exact for the submitted count body, subject to transport parity. |
| Admission | `internal/broker/ledger.go#Ledger.Admit` (line 93) refuses nonpositive counts, checks positive purpose caps, and enforces positive known windows even when reserve exhausts availability. |
| Settlement | `internal/broker/broker.go#core.settle` (line 166) uses observed usage first, response usage second, records spend, calibrates and emits. Missing reported usage is not proof of free inference. |
| Refusal | `internal/broker/broker.go#core.settleRefusal` (line 339) emits without recording spend. An omitted sink means no durable refusal record. |
| Live state | `internal/broker/default.go#Configure` (line 86) changes the captured ledger in place; `internal/broker/default.go#DetachExtraSink` (line 120) detaches by installed identity. Old wrappers' captured sinks are not globally rebound by this contract. |
| Streaming | `internal/broker/stream.go#core.proxyStream` (line 39) settles after forwarders finish. Cancellation drains depend on upstream closure. |
| Readout | `internal/broker/filesink.go#ReadReceiptLog` (line 70) reads rotating generations; `internal/broker/epoch.go#Segment` (line 208) excludes refusals. |

Reserve admission is component-qualified by the named root gates; factory and live integration remain open. Source-scanner behavior is verified at its package boundary. Intended broker capabilities remain in the vision, gap analysis, and design documents, independent of implementation status. No component receipt establishes whole-feature or whole-system completion.

## Enclosing mission accounting (authored, execution pending)

`core.settle` in `internal/broker/broker.go` notifies an optional enclosing
`usage.IncompleteObserver` when a call has no provider reports, incomplete usage
or conflicting counts. This preserves unknown spend when neighboring calls in
the same mission phase reported valid usage. Response-only fallback remains
valid for the broker receipt; it cannot silently fill a separate provider-price
ledger. The notification creates no extra billed report and does not alter
reserve/window admission or ledger settlement.

`internal/broker/enclosing_usage_test.go` defines the production-wrapper
regression fixture, and the mission cycle fixture uses the real broker path.
Both are unrun here. Full mission admission and estimate limitations remain
specified in `Docs/architecture/campaign/MISSION-IMPROVEMENT.md`.


## Incomplete retry capture: bounded local candidate

**LOCALLY VERIFIED:** candidate `a6aaaf0f1cbc429dac34f57b3d92bba2486c8fca`
closes B12 for adapter-notified incomplete attempts. The mutex-protected
`internal/broker/broker.go:145` retains uncertainty before, after or concurrently
with reports. `internal/broker/broker.go:151` returns reported counts as partial
lower bounds without adding a synthetic report; existing settlement excludes
partial totals from calibration. No reports remains unreported.
`internal/broker/incomplete_attempt_test.go:13` covers ordering, unchanged ledger
counts and calibration exclusion; concurrent reporting is at
`internal/broker/incomplete_attempt_test.go:47`.

These controls and the complete affected-package race suites pass locally, as
do the uncached full offline sqlite_vec suite, tagged build and vet. The provider
retry fixture and mission admission control cross the real broker boundary.
Publication requires the unavailable private-terms scan. This bounded result
does not establish universal adapter, malformed-reply or transport-failure
accounting, nor exact billing totals.
