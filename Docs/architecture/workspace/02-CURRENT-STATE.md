---
doc-class: shipped
subsystem: workspace
implementation-status: partial
last-verified: 2026-09-29
verified-against: 4dded472+working-tree
supersedes: []
---

# Current implementation and verification

The L1 package exists in the uncommitted working tree. [IMPLEMENTED_SPEC.md](IMPLEMENTED_SPEC.md) owns its behavioral contract; [WIRING-AND-NOT-BUILT.md](WIRING-AND-NOT-BUILT.md) owns the walker census.

| Layer | Source-grounded state |
|---|---|
| API/cache | For/Open and Membership queries exist (`internal/workspace/membership.go:61`, `internal/workspace/membership.go:73`, `internal/workspace/membership.go:254`, `internal/workspace/membership.go:292`). |
| Git driver | Snapshot/boundaries, dynamic checks and freshness stamps exist (loadSnapshot/checkIgnore/stampCached, `internal/workspace/git.go:98`, `internal/workspace/git.go:247`, `internal/workspace/git.go:312`). |
| Patterns | Recursive globbing, directory patterns and ordered negation exist (compilePattern/blocksDir, `internal/workspace/patterns.go:39`, `internal/workspace/patterns.go:118`). |
| Traversal | Walk batches unknown children and never opens rejected directories (`internal/workspace/walk.go:18`, `internal/workspace/walk.go:46`). |
| Scanner config | Default scanners load scanned-root patterns; configured scanners use supplied patterns; defaults come from config (membership/DefaultScannerConfig, `internal/world/fs.go:41`, `internal/world/scanner_config.go:28`). |
| Resumed fixes | DOM's package import is aliased; stats/ingest use a file's parent as fallback root; init direct reads use initializer membership (collectReplaceFiles/computeStats/collectIngestFiles/detectEntryPointsForRoot, `cmd/nerd/dom_replace_cmd.go:234`, `cmd/nerd/stats.go:16`, `cmd/nerd/chat/ingest.go:202`, `internal/init/scanner.go:264`). |
| Campaigns | Briefing, snapshot and rollback walk the write-set subtree with root membership (`internal/campaign/orchestrator_task_results.go:217`, `internal/campaign/orchestrator_task_transaction.go:388`, `internal/campaign/orchestrator_task_transaction.go:487`). |
| Regressions | Git rules/dynamic files/pruning/boundaries/fallback plus whitespace/concurrent refresh/root aliases are covered (`internal/workspace/membership_test.go:43`, `internal/workspace/membership_test.go:162`, `internal/workspace/membership_test.go:192`, `internal/workspace/membership_test.go:321`, `internal/workspace/membership_regression_test.go:12`, `internal/workspace/membership_regression_test.go:40`, `internal/workspace/membership_regression_test.go:70`). |

Workspace, projectdoc, gates and Mangle tests passed in the native run and the supplemental CGO-disabled run. Required sqlite-vec compilation is blocked by compiler execution denial; CGO-dependent caller tests are unverified and policy guards have wiring failures and a chat literal-budget baseline reduction after skip-list removal. Source review and formatting do not replace those gates. GAP-WS-04 retains scanner entry-path verification, GAP-WS-05 semantic treatment, and GAP-WS-06 composed-tree verification.