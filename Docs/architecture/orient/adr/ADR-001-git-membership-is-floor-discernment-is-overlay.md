---
doc-class: governance
subsystem: orient
implementation-status: accepted-not-implemented
last-verified: 2026-09-29
verified-against: e056692c
supersedes: []
---

# ADR-001: Git Membership Is the Floor; Discernment Is an Overlay

## Context

Today, repository directory traversal across codeNERD relies on fragmented, hardcoded directory ignore maps. In `internal/world/fs.go:226-263`, `ScanDirectory` uses a 6-directory hardcoded list and a rigid hidden-directory allowlist (`.github`, `.vscode`, `.circleci`, `.config`), silently dropping `.claude`, `.codex`, `.agents`, `.gemini`, `.grok`, `.jules`, and `.cursor`. Furthermore, any unignored directory is treated as application source code, resulting in megabytes of test fixtures, seed datasets, and golden answer keys being parsed into CodeDOM and embedded into vector stores.

This produces two severe defects:
1. Complete blindness to preexisting developer tool configurations and skills.
2. Saturation of agent context and memory with massive non-code data files.

## Decision

1. **Git Membership as the Absolute Floor**:
   Repository membership is anchored strictly in git version control truth via `internal/workspace`. Any file tracked by git (`git ls-files -z -co --exclude-standard`) is a member of the workspace. There are zero special-case hidden directory allowlists.
2. **Discernment as a Semantic Overlay**:
   On top of git membership truth, `internal/orient` derives a semantic discernment overlay. Mangle policy (`trees.mg`) evaluates directory metrics and role claims, assigning operational treatments (`tree_treatment(Dir, T)` with values `/index_and_parse`, `/index_names_only`, `/exclude`).
3. **Overlay Consumption**:
   Treatments are exported to `.nerd/orientation/membership.json`. `internal/workspace` consumes this overlay, exposing a `Treatment(rel string) string` interface. Indexing and code search tools ask `Treatment` before parsing or embedding files.

## Consequences

- **Positive**: codeNERD instantly recognizes all git-tracked agent skills and rules; massive seed data and answer keys remain searchable by path but never pollute CodeDOM AST models or token budgets.
- **Negative**: Requires synchronization between `internal/orient` fact derivation and `internal/workspace` runtime queries.
- **Risks**: Repositories without git must fall back to heuristic directory walk patterns.

## Witness

**Witness:** `test:TestTreeDiscernment_OverlayAppliesOverGitFloor`

| Claim | Witness (Code Evidence as of 2026-09-29) |
|---|---|
| Hardcoded skip map and hidden allowlist live today | `ignoredDirs` map (`internal/world/fs.go:226-238`) and dot-prefix allowlist (`internal/world/fs.go:241-263`). |
| Git tracked files enumerated safely in codebase | `internal/docscheck/docscheck.go:723-740` (`gitTrackedFiles` via `exec.Command("git", "-C", abs, "ls-files", "-z")`). |
| Overlay interface defined | Target symbol `Treatment(rel string) string` in `internal/workspace/membership.go`. |

## Status (Derived, Not Asserted)

**`accepted-not-implemented`**. Derivation: The architectural decision is accepted, but `internal/workspace` and `internal/orient/trees.go` have not yet landed on disk in the verified commit (`e056692c`). Status flips to `implemented` once `TestTreeDiscernment_OverlayAppliesOverGitFloor` passes.
