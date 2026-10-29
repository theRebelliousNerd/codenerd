---
doc-class: governance
subsystem: workspace
implementation-status: not-applicable
last-verified: 2026-09-29
verified-against: 4dded472+working-tree
supersedes: []
---

# Read order and authority

| Document | Responsibility |
|---|---|
| [01-VISION.md](01-VISION.md) | Target behavior and connection to the repository north star. |
| [IMPLEMENTED_SPEC.md](IMPLEMENTED_SPEC.md) | Authoritative working-tree implementation contract and verification boundary. |
| [02-CURRENT-STATE.md](02-CURRENT-STATE.md) | Current package layers and resumed fixes. |
| [WIRING-AND-NOT-BUILT.md](WIRING-AND-NOT-BUILT.md) | Complete source-reviewed walker census, exceptions and excluded ownership. |
| [03-GAP-ANALYSIS.md](03-GAP-ANALYSIS.md) | GAP-WS-01 through GAP-WS-06, including unresolved integration gates. |
| [05-MEMBERSHIP-SPEC.md](05-MEMBERSHIP-SPEC.md) | Membership API, patterns, traversal and future treatment. |
| [04-PRINCIPLES-AND-CONSTRAINTS.md](04-PRINCIPLES-AND-CONSTRAINTS.md) | Constraints on future changes. |
| [adr/ADR-001-git-ls-files-and-check-ignore-as-membership-authority.md](adr/ADR-001-git-ls-files-and-check-ignore-as-membership-authority.md) | Git-backed authority decision and resolved local witnesses. |
| [OPEN-QUESTIONS.md](OPEN-QUESTIONS.md) | Boundary and semantic-treatment questions. |
| [RISK-REGISTER-AND-DECISION-LOG.md](RISK-REGISTER-AND-DECISION-LOG.md) | Remaining risks and mitigation evidence. |
| [TODO.md](TODO.md) | Leaf obligations tied to gap IDs. |

## Grounded and planned

IMPLEMENTED_SPEC, CURRENT-STATE and WIRING describe the uncommitted implementation with source citations; their verification limits are explicit. VISION and the treatment section of MEMBERSHIP-SPEC describe target behavior. Closed leaf gaps do not close the integrated-build gap.