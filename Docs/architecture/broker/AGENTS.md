---
doc-class: governance
subsystem: broker
implementation-status: partial
last-verified: 2026-10-02
verified-against: db1d4b7e24d0f9440c8627b2561ca64a25876954
supersedes: []
---

# Broker corpus guidance

This directory specifies what `internal/broker` is intended to accomplish. Follow the parent guidance: current reality plus the implementable north-star design. The orchestrator owns documentation and shared registration; implementation agents are code-only unless the user explicitly assigns a documentation task. No previous lane grant authorizes another rewrite.

Read [README](README.md), [IMPLEMENTED_SPEC](IMPLEMENTED_SPEC.md), [gap matrix](03-GAP-ANALYSIS.md) and [counting contract](COUNTING-AND-LIMITS.md) before broker documentation changes. Keep all 18 canonical responsibilities; `00-INDEX.md`, constraints, risks and ADR supply the architecture-standard entry points. `TODO.md` is the only feature-card surface.

Current behavior needs fresh source and appropriately scoped behavioral evidence; inspection is not a passing test or production receipt. Distinguish component-qualified reserve admission from remaining factory/live integration obligations. Preserve unknown-window, purpose, integer and `SetWindow` contracts. Future implementation must receive explicit source ownership, not infer permission from a documentation assignment.

Preserve the full intended design for Mangle ownership, counting, concurrent reservation, global scope, streaming, receipts and caller budgeting even when it is not built. Mark actual gaps without shrinking those requirements. An archive is not a substitute for keeping intended design in its proper document. Current-state and implementation records need exact evidence; architecture prose defines current engineering intent. After large refactors update this guidance, and do not infer whole-feature completion from a focused gate.
