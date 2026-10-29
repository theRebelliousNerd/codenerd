---
doc-class: governance
subsystem: broker
implementation-status: partial
last-verified: 2026-10-02
verified-against: db1d4b7e24d0f9440c8627b2561ca64a25876954
supersedes: []
---

# ADR-001: refuse exhausted known windows before inference

- **Status:** accepted, not implemented or runtime-verified.
- **Date:** 2026-10-02.
- **Requirement:** GAP-BROKER-01.
- **Authority:** root's broker corpus packet; [gap matrix](../03-GAP-ANALYSIS.md) and [counting contract](../COUNTING-AND-LIMITS.md).

## Context and choice

`internal/broker/ledger.go#Ledger.Admit` (line 93) currently checks fit only when available > 0. `internal/broker/ledger.go#NewLedger` (line 52) retains negative reserve. Available zero therefore conflates unknown-window compatibility with known exhausted capacity. No reproduction or test ran in this authoring packet.

Normalize negative constructor reserve to zero. For positive known window, compare reserve with window before subtracting; refuse a positive counted request with `DecisionWindowExceeded` if reserve equals or exceeds window. Otherwise enforce the safe positive difference and continue existing purpose-budget checks. Preserve nonpositive-window exception, count-unavailable precedence, normal fit and `SetWindow` update semantics.

## Witness seam and acceptance obligations

The current executable seam is `internal/broker/ledger.go#Ledger.Admit` (line 93), reached by `internal/broker/broker.go#core.admit` (line 127) before `internal/broker/compression.go#metered` (line 51) invokes inference. `internal/broker/broker.go#core.settleRefusal` (line 339) supplies the refusal receipt without recording spend. The seam exists; this decision's new boundary is not implemented yet.

Root's future packet may change only ledger implementation, ledger tests and broker tests. New equal/excess-reserve controls must fail before repair, pass after repair and fail when the old guard is restored. Independently assert zero inference calls, unchanged spend and the refusal receipt decision. Keep unknown/purpose, negative reserve, extreme integers, exact-fit/one-over and `SetWindow` controls. Root must attach exact revision, commands, race/component and normal-entry receipts before changing status.

## Consequences and limits

This is a deterministic arithmetic repair, not a concurrency reservation system or Mangle executive redesign. No caller reserve aggregation, downstream fallback budget, JIT retry text, stream lifecycle or sink/scope policy is changed. Those questions stay separate. Source inspection and corpus structure checks do not qualify runtime behavior.
