---
doc-class: governance
subsystem: workspace
implementation-status: partial
last-verified: 2026-09-29
verified-against: 4dded472+working-tree
supersedes: []
---

# Risks and decisions

| Risk | Consequence | Evidence or retirement obligation |
|---|---|---|
| New paths after snapshot | A just-created file disappears from discovery | Dynamic checks are covered by TestMembershipGit (`internal/workspace/membership_test.go:43`); Files remains snapshot-based until refresh (`internal/workspace/membership.go:335`). |
| Missing Git/non-worktree | Pattern fallback differs from Git membership | Fallback witness TestPatternFallbackOutsideGit (`internal/workspace/membership_test.go:321`); fallback is not separately reported in scan output. |
| Large tracked unwanted trees | Discovery/embedding costs | Patterns overlay Git (`Includes`, `internal/workspace/membership.go:254`); semantic treatment remains GAP-WS-05. |
| Private walker filters return | Multiple membership truths | Maintain the source census; briefing and transaction holdouts now use Admit (`internal/campaign/orchestrator_task_results.go:283`, `internal/campaign/orchestrator_task_transaction.go:395`). |
| Refresh overlaps dynamic checks | Stale answers repopulate a new cache | Serialized requests and concurrent regression (`internal/workspace/membership.go:477`, `internal/workspace/membership_regression_test.go:40`). |
| Alias/path corruption | Valid files are lost or external paths admitted | Root and whitespace regressions (`internal/workspace/membership_regression_test.go:12`, `internal/workspace/membership_regression_test.go:70`). |
| Integrated tree is not qualified | Authored callers may still fail | GAP-WS-06: enable compiler execution and pass caller/policy gates. |
| Ripgrep traversal differs | Extra-excluded trees may still be read before hits are filtered | Search/keepMemberHits (`internal/retrieval/backend.go:77`, `internal/retrieval/backend.go:148`); parity remains open. |

Git-backed authority, user overlays and universal .git/.nerd exclusions follow [ADR-001](adr/ADR-001-git-ls-files-and-check-ignore-as-membership-authority.md). C1 retained finite batched subprocesses from the original brief, kept default scanners root-config aware, and made campaign snapshot/cleanup share the root authority. No tunable or compatibility shim was introduced.