---
doc-class: governance
subsystem: orient
implementation-status: accepted-not-implemented
last-verified: 2026-09-29
verified-against: e056692c
supersedes: []
---

# ADR-002: Classify Document Functional Roles Rather Than Ranking by Recency or Filename

## Context

In the current codebase, documentation is gathered and prioritized via hardcoded filename tables in `internal/init/strategic_knowledge.go:246-260`, where `CLAUDE.md` is hardcoded as Priority 0, and `README.md`, `ARCHITECTURE.md`, `DESIGN.md`, and `VISION.md` are hardcoded as Priority 1. Files not matching these specific strings receive low priority, while `AGENTS.md` and `GEMINI.md` are relegated to Priority 3.

Furthermore, standard agent systems frequently equate recency with importance. However, on software projects, a comprehensive architectural specification committed in a single burst on one day eighteen months ago is often the foundational source of truth, while files touched yesterday are merely operational notes or minor dependency adjustments.

## Decision

1. **Eliminate Filename Priority Tables**:
   Remove all hardcoded filename scoring maps. A document's importance is never inferred from its filename string.
2. **Lossless Functional Role Transduction**:
   The LLM reads candidates in full via sequential paging, classifying functional roles (`doc_role_claim(Path, Role, ConfidencePct)`) with values including `/vision`, `/north_star_draft`, `/origin_design`, `/spec`, `/plan`, `/report`, `/standard`, `/guide`.
3. **Deductive Weighting via Temporal Lineage**:
   Mangle policy (`lineage.mg`) weights vision sources by evaluating role claims against structural evidence: generation vintage (`doc_generation`), commit bursts (`doc_burst`), and active liveness (`doc_live`). A living specification or an active burst specification outranks an ephemeral changelog regardless of calendar age.

## Consequences

- **Positive**: Eliminates tool-specific filename biases; accurately captures foundational architecture written in single bursts; distinguishes living designs from superseded predecessors.
- **Negative**: Requires multi-stage evaluation: git history parsing must precede document attention selection, which must precede role classification.
- **Risks**: Repositories with zero formal documentation must fall back to commit message and code comment synthesis.

## Witness

**Witness:** `predicate:vision_source/3` and `test:TestLineage_BurstOriginOutranksRecentChangelog`

| Claim | Witness (Code Evidence as of 2026-09-29) |
|---|---|
| Hardcoded filename map lives today | `priorityFiles` map in `internal/init/strategic_knowledge.go:246-260`. |
| Target Mangle derivation defined | `vision_source(Path, WeightPct, Why)` in `internal/orient/lineage.mg`. |
| Proving regression test | `TestLineage_BurstOriginOutranksRecentChangelog` asserting that an older `/origin_design` burst specification outranks a recently touched operational log. |

## Status (Derived, Not Asserted)

**`accepted-not-implemented`**. Derivation: The decision is approved, but `internal/orient/lineage.mg` is not yet present on disk in commit `e056692c`. Status flips to `implemented` once the witness test passes.
