---
doc-class: governance
subsystem: workspace
implementation-status: not-applicable
last-verified: 2026-09-29
verified-against: 4dded472+working-tree
supersedes: []
---

# Membership constraints

1. **One authority.** Directory membership lists belong only to configuration; walkers call membership and retain purpose filters. DefaultScannerConfig copies the single default rather than defining another (`internal/world/scanner_config.go:28`, `DefaultWorldConfig`, `internal/config/world.go:41`).
2. **Git floor plus user exclusions.** Tracked and nonignored paths establish the baseline, then patterns can drop unwanted tracked content. User negation cannot restore Git-ignored paths or internal state (`Includes`, `internal/workspace/membership.go:254`).
3. **Prune before opening children.** Return SkipDir for rejected directories; skip rejected files. Membership Walk does not read a rejected directory (`Admit`, `walk`, `internal/workspace/membership.go:415`, `internal/workspace/walk.go:46`).
4. **No hidden-name membership policy.** Only .git/.nerd are universal exclusions. Display, evidence and secret-path filters remain explicit purpose contracts (`alwaysExcluded`, `internal/workspace/membership.go:540`; evidence Snapshot exception, `internal/evidence/change.go:135`).
5. **Preserve path identity.** NUL-delimited Git paths retain whitespace. Canonical roots and aliased traversal paths agree; symlink file entries are not resolved into foreign content (`acceptGitPath`, `canonicalRoot`, `relOf`, `internal/workspace/git.go:182`, `internal/workspace/membership.go:118`, `internal/workspace/membership.go:429`).
6. **Coordinate refresh and dynamic checks.** Cached hits avoid Git calls; misses may synchronously query Git. Refresh cannot publish while a batched answer is in flight (`askBatch`, `internal/workspace/membership.go:477`). Do not add a run-level deadline or an undocumented tunable.
7. **Keep semantic decisions separate.** Orientation treatment is GAP-WS-05 and must consume derived evidence rather than a new Go heuristic.