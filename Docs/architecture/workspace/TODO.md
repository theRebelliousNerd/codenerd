---
doc-class: governance
subsystem: workspace
implementation-status: partial
last-verified: 2026-09-29
verified-against: 4dded472+working-tree
supersedes: []
---

# Remaining build queue

- [x] GAP-WS-01 / GAP-WS-02: leaf API, Git snapshot/check-ignore, fallback/pattern semantics and regressions are authored; leaf tests pass in native and CGO-disabled runs (`internal/workspace/membership_test.go:43`, `internal/workspace/membership_regression_test.go:40`).
- [x] GAP-WS-03: repoint every C1-owned census walker, remove private directory lists and old SkipDir helpers; preserve other-lane exclusions. See [WIRING-AND-NOT-BUILT.md](WIRING-AND-NOT-BUILT.md).
- [ ] GAP-WS-04: run scanner/ingestor/factory/init tests and exercise init/scan/campaign configuration entry paths when CGO dependencies compile (authored init regression, `internal/init/scanner_membership_test.go:10`).
- [ ] GAP-WS-06: enable the configured native compiler, resolve other-lane guard failures, run the full sqlite-vec build and all census package tests. Campaign and retrieval regressions await this gate (`internal/campaign/workspace_membership_test.go:10`, `internal/retrieval/backend_membership_test.go:11`).
- [ ] GAP-WS-05: orientation owner implements artifact-driven Treatment and proves names-only/exclude inheritance before parsing/embedding consumers use it.