---
doc-class: governance
subsystem: broker
implementation-status: partial
last-verified: 2026-10-02
verified-against: db1d4b7e24d0f9440c8627b2561ca64a25876954
supersedes: []
---

# Alignment review without invented scores

| Applicability lane | Evidence and unresolved obligation |
|---|---|
| Mangle | **OPEN QUESTION:** `internal/broker/ledger.go#Ledger.Admit` (line 93) decides in Go, not through a derived budget predicate. |
| Permission/safety | **PARTIAL:** resource admission is not constitutional tool permission. The local known-exhaustion repair is component-qualified; factory and live enforcement remain open. |
| Fact flow | **PARTIAL:** factory wrapper to Count/Decision/Receipt is traced; `internal/broker/broker.go#core.admit` (line 127) issues no kernel completion verdict. |
| JIT/agents | **PARTIAL:** `internal/broker/wrap.go#Wrap` (line 32) preserves gating interfaces. Inline retry instruction in `internal/broker/compression.go#compressionInstruction` (line 22) is a separate JIT frontier. |
| Wiring | **PARTIAL:** boot/factory/session/readout paths are traced in [wiring](08-WIRING-AND-INTEGRATION.md); construction audits need root execution. |
| State/concurrency | **PARTIAL:** ledger operations lock; Admit/Record are separate, not an in-flight reservation (`internal/broker/ledger.go#Ledger.Admit`, line 93). |
| Recovery | **PARTIAL:** streaming forwards cancellation and drains; producer closure and final receipt barriers remain unestablished (`internal/broker/stream.go#core.proxyStream`, line 39). |
| Observability | **PARTIAL:** sink append failures are exposed (`internal/broker/filesink.go#FileSink.Failures`, line 44); durable receipt completeness requires lifecycle evidence. |
| Testing | **PARTIAL:** reserve admission and source scanning have named root package/race/mutation receipts in [current state](02-CURRENT-STATE.md). These do not establish full coverage, live acceptance, or a rubric score. |

GAP-BROKER-01 is the bounded accepted repair. Executive authority, JIT restatements, scope isolation and concurrent spend remain explicit questions. No applicability lane is silently treated as complete or generically N-A.
