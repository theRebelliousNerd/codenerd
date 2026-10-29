---
doc-class: governance
subsystem: workspace
implementation-status: partial
last-verified: 2026-09-29
verified-against: 4dded472+working-tree
supersedes: []
---

# ADR-001: Git-backed membership authority

## Context and decision

The migration replaces per-walker directory lists with one leaf authority. Git supplies tracked and nonignored untracked content, configured patterns overlay it, and fallback mode evaluates those patterns. Only .git/.nerd are universal exclusions. Walkers prune rejected directories and retain purpose filters.

The original implementation brief specified For(root) and batched check-ignore. C1 preserves that API and uses finite sibling batches with mutex-protected caching; it does not introduce a persistent process lifecycle.

## Witness-derived status

**Implemented in the uncommitted working tree; integrated verification remains partial.** For and the Git driver resolve (`internal/workspace/membership.go:61`, `loadSnapshot`, `internal/workspace/git.go:98`, `checkIgnore`, `internal/workspace/git.go:247`). TestMembershipGit, pruning and boundary tests pass in native and CGO-disabled runs (`internal/workspace/membership_test.go:43`, `internal/workspace/membership_test.go:162`, `internal/workspace/membership_test.go:192`).

The query-side refresh/check synchronization and path fidelity have new witnesses (`internal/workspace/membership_regression_test.go:12`, `internal/workspace/membership_regression_test.go:40`, `internal/workspace/membership_regression_test.go:70`). This evidence does not establish a passing sqlite-vec build or all production caller behavior; GAP-WS-06 remains open.

## Consequences

Membership no longer depends on hidden-directory naming heuristics. Unknown paths can require a Git subprocess; explicit refresh pays snapshot cost and detects nested-ignore changes. Nested repositories remain closed boundaries unless an explicit child-workspace contract is added. Ripgrep's native traversal still has parity limits documented in the wiring census.