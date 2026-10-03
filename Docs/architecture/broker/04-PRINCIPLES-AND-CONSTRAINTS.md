---
doc-class: governance
subsystem: broker
implementation-status: partial
last-verified: 2026-10-02
verified-against: db1d4b7e24d0f9440c8627b2561ca64a25876954
supersedes: []
---

# Broker principles and constraints

The runtime principles live in [04-ARCHITECTURAL-PRINCIPLES](04-ARCHITECTURAL-PRINCIPLES.md). This page records the current authoring and implementation boundaries.

1. Implementation lanes author code and tests only. The orchestrator owns specification updates, shared registration, Git publication, and acceptance gates; documentation ownership does not grant runtime ownership.
2. Source-observed behavior and accepted targets remain distinct. Inspection date and manifest `verified_on` do not claim executed tests.
3. Reserve admission requirements are established before implementation. Ledger and wrapper tests establish the local repair; factory configuration and live provider integration remain separate acceptance obligations.
4. Preserve unknown-window compatibility, purpose caps, count-unavailable precedence, normal fit and `SetWindow` semantics while repairing exhausted-known-window arithmetic.
5. No fabricated alignment score, dimension, coverage percentage, passing count or completed card. Executed evidence requires exact root revision and receipts.
6. Every large rewrite preserves substantive target behavior, contracts, rationale, and acceptance obligations in their proper documents, and updates scoped guidance. Archiving intended design is not preservation of the active specification.
7. New model-facing behavior must use JIT atoms and control-packet-aware assembly. Existing inline compression prose is an open question, not a template for new prompt text.
8. Kernel-derived executive decisions and typed default-deny tool permissions remain the north star. Broker arithmetic must never be described as constitutional permission.

[The accepted ADR](adr/ADR-001-known-window-reserve-refusal.md) records the local arithmetic choice and its witness seam. [TODO](TODO.md) is the sole feature-card authority. Root owns portfolio/index registration, causal source grant, test/build/live gates and status reconciliation.
