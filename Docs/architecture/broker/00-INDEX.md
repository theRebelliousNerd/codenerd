---
doc-class: governance
subsystem: broker
implementation-status: partial
last-verified: 2026-10-02
verified-against: db1d4b7e24d0f9440c8627b2561ca64a25876954
supersedes: []
---

# Broker corpus index

[README](README.md) is the product-facing front door. [IMPLEMENTED_SPEC](IMPLEMENTED_SPEC.md) owns source-observed behavior; [01-VISION](01-VISION.md) owns target behavior. A source witness is not an executed behavioral receipt.

| Responsibility | Document |
|---|---|
| Entry and representative journey | [README](README.md) |
| Current authoritative contract | [IMPLEMENTED_SPEC](IMPLEMENTED_SPEC.md) |
| Alignment and applicability | [00-ALIGNMENT-VISION-REVIEW](00-ALIGNMENT-VISION-REVIEW.md) |
| North star | [01-VISION](01-VISION.md) |
| Current package inventory | [02-CURRENT-STATE](02-CURRENT-STATE.md) |
| Accepted gap with checkable exit | [03-GAP-ANALYSIS](03-GAP-ANALYSIS.md) |
| Design principles | [04-ARCHITECTURAL-PRINCIPLES](04-ARCHITECTURAL-PRINCIPLES.md) |
| Runtime internals | [05-INTERNAL-ARCHITECTURE](05-INTERNAL-ARCHITECTURE.md) |
| APIs and types | [06-PUBLIC-API-AND-TYPES](06-PUBLIC-API-AND-TYPES.md) |
| Dependencies and downstream consumers | [07-DEPENDENCY-MAP](07-DEPENDENCY-MAP.md) |
| Production entry, state and teardown | [08-WIRING-AND-INTEGRATION](08-WIRING-AND-INTEGRATION.md) |
| Safety and invariants | [09-SAFETY-AND-INVARIANTS](09-SAFETY-AND-INVARIANTS.md) |
| Tests and root acceptance obligations | [10-TESTING-ALIGNMENT](10-TESTING-ALIGNMENT.md) |
| Receipts and operational signals | [11-OBSERVABILITY](11-OBSERVABILITY.md) |
| Failure behavior | [12-FAILURE-MODES](12-FAILURE-MODES.md) |
| Sole feature-card queue | [TODO](TODO.md) |
| Separate undecided contracts | [OPEN-QUESTIONS](OPEN-QUESTIONS.md) |
| Current implementation progress and remaining obligations | [_progress](_progress.md) |

The accepted [counting contract](COUNTING-AND-LIMITS.md) and [ADR](adr/ADR-001-known-window-reserve-refusal.md) pin reserve admission requirements. [Receipt interpretation](RECEIPTS-AND-EPOCHS.md), [reachable/unproved inventory](WIRING-AND-NOT-BUILT.md), [constraints](04-PRINCIPLES-AND-CONSTRAINTS.md) and [risk register](RISK-REGISTER-AND-DECISION-LOG.md) provide narrow detail. [INTERNALS](INTERNALS.md) explains broker responsibilities and mechanisms. Intended capabilities remain in their design documents; source and behavioral evidence establish implementation status, not authority to discard that design. [AGENTS](AGENTS.md) keeps this distinction and ownership boundary explicit.
