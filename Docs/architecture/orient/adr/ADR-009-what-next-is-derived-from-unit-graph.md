---
doc-class: governance
subsystem: orient
implementation-status: accepted-not-implemented
last-verified: 2026-09-29
verified-against: bb7bafac
supersedes: []
---

# ADR-009: What-Next Is Derived from the Unit Graph, Not Suggested by the Model

## Context

Coding assistants routinely guess what a developer should do next by inspecting the last prompt, grepping for "TODO" comments, or asking an ungrounded LLM to suggest tasks. In `cmd/nerd/chat/commands.go:55-265`, no `/next` or `/todo` command exists; `next_action` in `internal/core/defaults/policy/capabilities.mg:20-51` is the internal agent loop dispatcher, not an architectural recommendation to the developer.

This approach fails:
1. It suggests tasks whose architectural foundations do not exist.
2. It suffers from context drift and recency bias.
3. It cannot justify why a task was recommended or explain its dependency chain.

## Decision

1. **Derived, Not Suggested**:
   "What next" recommendations are computed as a deductive fixpoint in Mangle, never generated free-form by an LLM prompt.
2. **Foundations-Up Kahn Ordering**:
   Extend the topological ordering algorithm from `internal/core/defaults/policy/recurse.mg:377-386` across the unified unit graph. A unit is derived as ready when it is not `/aligned` and every prerequisite unit it depends upon is `/aligned` (or explicitly classified to leave).
3. **Pluggable Starting Directions**:
   Allow the developer to choose an architectural layer to seed from (`start_from(Kind)` with `/types`, `/ingress`, `/persistence`, `/api`, `/tooling`), defaulting to all non-aligned units when omitted.
4. **Parametric Ranking and Verbal Justification**:
   Rank ready units using external weights loaded via `config_param(Key, Value)`. Emits `what_next(Unit, Rank, Why)` where `Why` is a deterministic derivation string explaining the exact dependency and capability justification.
5. **Universal Consumption Surfaces**:
   Expose recommendations consistently across the `/next` chat command, natural language intent routing (`/what_next`), the `nerd next` CLI binary with `--json` output, and campaign recursion (`nerd campaign recurse`).

## Consequences

- **Positive**: Guarantees that recommended work is unblocked and building from foundations up; eliminates hallucinated task lists; provides transparent, audit-grade explanations.
- **Negative**: Requires strict maintenance of dependency links in the unit graph; missing links could cause premature readiness derivations.
- **Risks**: Circular dependencies in messy foreign code could block readiness. Mitigated by Tarjan SCC cycle collapsing in `DeriveWorkspaceDAG`.

## Witness

**Witness:** `test:TestWhatNext_DerivedFromUnitGraph`

| Claim | Witness (Code Evidence as of 2026-09-29) |
|---|---|
| Existing Kahn ordering rule in Mangle | `recurse_node_ready` in `internal/core/defaults/policy/recurse.mg:381-386`. |
| Absence of `/next` command in chat | Command switch in `cmd/nerd/chat/commands.go:55-265` contains no `/next` case. |
| What-Next derivation target test | Planned test `TestWhatNext_DerivedFromUnitGraph` in `internal/orient/what_next_test.go`. |

## Status (Derived, Not Asserted)

**`accepted-not-implemented`**. Derivation: The architectural decision is accepted, but the unified unit graph Kahn ordering and `what_next` relation have not yet landed in production source code as of commit `bb7bafac`. Status flips to `implemented` once `TestWhatNext_DerivedFromUnitGraph` passes.
