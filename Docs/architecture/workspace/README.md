---
doc-class: governance
subsystem: workspace
implementation-status: not-applicable
last-verified: 2026-09-29
verified-against: 4dded472+working-tree
supersedes: []
---

# Workspace membership

This corpus governs the single membership authority and its walker consumers. Start with [00-INDEX.md](00-INDEX.md). The L1/C1 implementation is present in the uncommitted working tree; integrated verification is still open.

For/Open cache membership for a resolved root and pattern list (`internal/workspace/membership.go:61`, `internal/workspace/membership.go:73`). Git supplies tracked and nonignored untracked paths; user patterns add exclusions; fallback mode uses those patterns (`loadSnapshot`, `newMembership`, `internal/workspace/git.go:98`, `internal/workspace/membership.go:138`). Walkers prune rejected directories through Admit (`internal/workspace/membership.go:415`).

Read [IMPLEMENTED_SPEC.md](IMPLEMENTED_SPEC.md) for the authoritative contract, [WIRING-AND-NOT-BUILT.md](WIRING-AND-NOT-BUILT.md) for source-level reachability, and [03-GAP-ANALYSIS.md](03-GAP-ANALYSIS.md) for verification and semantic-treatment obligations. Leaf tests passed in native and CGO-disabled runs; that does not qualify the sqlite-vec build.