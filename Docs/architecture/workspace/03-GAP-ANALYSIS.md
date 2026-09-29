---
doc-class: shipped-with-future
subsystem: workspace
implementation-status: partial
last-verified: 2026-09-29
verified-against: 4dded472+working-tree
supersedes: []
---

# Membership gaps

Closed rows identify the uncommitted working tree, not a release commit. Integrated verification stays open.

| Gap ID | Capability | Current state | Target state | Severity | Phase | Blocking dependencies | Exit criteria |
|---|---|---|---|---|---|---|---|
| GAP-WS-01 | Single authority | Closed in working tree: For/Open/Walk exist (`internal/workspace/membership.go:61`, `internal/workspace/walk.go:18`); leaf tests passed in the native and CGO-disabled runs. | [05-MEMBERSHIP-SPEC.md](05-MEMBERSHIP-SPEC.md), API. | Critical | 1 / closed | Integration tracked separately | `go test -count=1 ./internal/workspace/...` with the declared environment. |
| GAP-WS-02 | Gitignore and dynamic membership | Closed in working tree: Git driver and batched checks exist; Git/fallback/pruning tests pass (`internal/workspace/git.go:98`, `internal/workspace/git.go:247`, `internal/workspace/membership_test.go:43`, `internal/workspace/membership_test.go:162`). | [05-MEMBERSHIP-SPEC.md](05-MEMBERSHIP-SPEC.md), Git/pattern semantics. | Critical | 1 / closed | Integration tracked separately | `go test -count=1 ./internal/workspace/... -run 'MembershipGit|NestedGitignoreRefresh|UserPatternsOnTopOfGit|PatternFallbackOutsideGit|WalkNeverEntersBigIgnoredDir'`. |
| GAP-WS-03 | Walker consolidation | Closed for C1's owned census: no private directory membership lists remain; campaign holdouts now gate paths (`internal/campaign/orchestrator_task_results.go:283`, `internal/campaign/orchestrator_task_transaction.go:395`). | [WIRING-AND-NOT-BUILT.md](WIRING-AND-NOT-BUILT.md), census. | Critical | 2 / closed for owned census | Strategic knowledge, orient, browser sessions and parsing remain other-lane ownership | Audit census files for private directory sets/old SkipDir helpers/dot allowlists: zero membership filters; scanner/tool behavior checks remain GAP-WS-06. |
| GAP-WS-04 | Config entry paths | Authored: default scanners load root patterns; factory supplies config; init subtrees use initializer root (`internal/world/fs.go:41`, `internal/system/factory.go:2239`, `internal/init/scanner.go:264`). Caller tests are blocked. | [05-MEMBERSHIP-SPEC.md](05-MEMBERSHIP-SPEC.md), uniform config. | High | 2 / open verification | CGO and concurrent dependencies | `go test -count=1 ./internal/world/... ./internal/init/... ./internal/shards/system/... ./internal/system/...`, including TestScanDirectoryHonorsGitignore and TestEntryPointsUseWorkspaceMembershipForSubmodule. |
| GAP-WS-05 | Semantic treatment | Open: query surface currently exposes membership, not orientation treatment (`internal/workspace/membership.go:254`, `internal/workspace/membership.go:292`). | [05-MEMBERSHIP-SPEC.md](05-MEMBERSHIP-SPEC.md), Treatment. | High | 3 | Orientation owner | Real artifact-loading test proves names-only treatment, exclusion and inheritance from Mangle-derived orientation evidence. |
| GAP-WS-06 | Integrated native verification | Open: sqlite-vec build cannot execute compiler; caller tests and policy guards are not green. Regressions exist (`internal/workspace/membership_regression_test.go:40`, `internal/campaign/workspace_membership_test.go:10`). | [IMPLEMENTED_SPEC.md](IMPLEMENTED_SPEC.md), evidence boundary. | Critical | Integration | Compiler access; other lanes' parser/policy/orientation wiring | `go build -tags sqlite_vec ./...` and `go test -count=1` for every census package plus `./internal/core/defaults/...`: all pass on composed tree. |

## Test environment

GOTMPDIR/GOCACHE and TEMP/TMP stay inside the repository; CGO_CFLAGS points at its SQLite headers. GIT_CEILING_DIRECTORIES=GOTMPDIR prevents fallback fixtures from discovering codeNERD's enclosing repository. Passing leaf evidence used CGO_ENABLED=0; it does not qualify sqlite-vec or CGO-dependent callers.