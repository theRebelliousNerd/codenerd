---
doc-class: governance
subsystem: orient
implementation-status: accepted-not-implemented
last-verified: 2026-09-29
verified-against: bb7bafac
supersedes: []
---

# ADR-008: One Unit Graph for Code and Spec

## Context

Prior to this decision, codeNERD maintained two completely disjoint dependency models:
1. The Code Graph: Generated at the package directory level by `DeriveWorkspaceDAG` (`internal/campaign/recurse_workspace.go:59-92`) and at the file/symbol level via `dependency_link` (`internal/world/dependency_links.go:12-51`) and `code_calls` (`internal/world/cartographer.go:273-279`).
2. The Specification Graph: Represented in North Star capabilities and requirements (`schemas_misc.mg:44-96`, `internal/northstar/types.go:27-38`) and architecture gap rows (`internal/docscheck/docscheck.go:50-54`).

These graphs had zero relational connections:
- No predicate or data structure joined a code symbol or file to a specification section.
- Campaign recursion (`cmd/nerd/cmd_campaign_recurse.go:42-80`, `internal/campaign/recurse_policy.go:66-106`) evaluated only compiler/linter gate findings, completely blind to missing or unbuilt specifications.
- The system could not evaluate whether a package was ahead of its spec, behind it, or missing architectural coverage entirely.

## Decision

1. **A Single Bipartite Unit Graph**:
   Construct a unified bipartite unit graph in the Mangle kernel comprising both code units and spec units.
2. **Granularity Alignment**:
   - Status derivation and execution readiness operate at the module/package grain (`recurse_workspace.go:94-105`).
   - Realization links operate at the file and symbol grain, mapping specific functions, types, and endpoints to specific specification requirements and capability clauses.
3. **Graph Edges**:
   The graph is formed by three relational families:
   - `code_depends(ConsumerUnit, ProviderUnit)`: Structural dependencies in source code.
   - `spec_depends(DependentSpec, PrerequisiteSpec)`: Hierarchical requirements in specifications.
   - `unit_realizes(CodeUnit, SpecUnit)`: Semantic realization links established through explicit code citations, spec path citations, and verified model transduction claims.

## Consequences

- **Positive**: Enables mathematical derivation of alignment, gaps, and readiness across the whole codebase; eliminates duplicate ad-hoc dependency traversals.
- **Negative**: Requires CodeDOM and document parsers to synchronize unit IDs; adds graph population overhead at system initialization.
- **Risks**: Massive monorepos could create large fact sets. Bounded by package-level status grain with lazy symbol-level link expansion.

## Witness

**Witness:** `test:TestSpecAlignment_OneUnitGraph_Derivation`

| Claim | Witness (Code Evidence as of 2026-09-29) |
|---|---|
| Disjoint graphs exist in codebase today | `DeriveWorkspaceDAG` (`internal/campaign/recurse_workspace.go:59-92`) and North Star schema (`schemas_misc.mg:44-96`). |
| Gap analysis disconnected from recurse | `recurse_policy.go:66-106` queries only compiler `gates.Finding`, ignoring `03-GAP-ANALYSIS.md`. |
| Unified unit graph target test | Planned test `TestSpecAlignment_OneUnitGraph_Derivation` in `internal/orient/spec_unit_test.go`. |

## Status (Derived, Not Asserted)

**`accepted-not-implemented`**. Derivation: The architectural decision is accepted, but the unified unit graph and `unit_realizes` relations have not yet landed in production source code as of commit `bb7bafac`. Status flips to `implemented` once `TestSpecAlignment_OneUnitGraph_Derivation` passes.
