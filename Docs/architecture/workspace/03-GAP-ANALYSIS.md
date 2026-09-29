---
doc-class: shipped-with-future
subsystem: workspace
implementation-status: planned
last-verified: 2026-09-29
verified-against: e056692c
supersedes: []
---

# 03 — Gap Analysis — Workspace Membership & Ignore Consolidation

This document defines the gap matrix for `internal/workspace` against the target state specified in [01-VISION.md](01-VISION.md). Current state rows cite verified source lines from [02-CURRENT-STATE.md](02-CURRENT-STATE.md).

## 1. The Gap Matrix

| Gap ID | Capability | Current State (Shipped) | Target State (Spec) | Severity | Phase | Blocking Dependencies | Exit Criteria |
|---|---|---|---|---|---|---|---|
| `GAP-WS-01` | Single Membership Authority Package | Non-existent; membership logic is fragmented across 35 walkers in `internal/world`, `tools`, `retrieval`, `campaign`. | [05-MEMBERSHIP-SPEC.md](05-MEMBERSHIP-SPEC.md): `internal/workspace` package exposes `For`, `Includes`, `IncludesDir`, `Walk`. | Critical | Phase 1 | Lane `L1` | `go test ./internal/workspace` passes with unit tests covering git-tracked, untracked, and ignored files. |
| `GAP-WS-02` | Universal `.gitignore` Compliance | Zero `.gitignore` support; `internal/world/fs.go:226-263` uses static name maps; `go.mod` has no gitignore parser. | [05-MEMBERSHIP-SPEC.md](05-MEMBERSHIP-SPEC.md): `git ls-files -z` snapshot plus batched `git check-ignore -z --stdin` for new files. | Critical | Phase 1 | Lane `L1` | Test in temp git repository with nested `.gitignore` (wildcards, `**`, `!` negation) verifies ignored files are excluded and unignored files are admitted. |
| `GAP-WS-03` | Universal Walker Repointing | ~35 independent production walkers maintain ~25 private skip lists (`fs.go`, `incremental_scan.go`, `sparse.go:76`, `search.go:373`). | [WIRING-AND-NOT-BUILT.md](WIRING-AND-NOT-BUILT.md): Repoint all walkers to query `Membership.Includes` or `Membership.IncludesDir`. | Critical | Phase 2 | Lane `L1` | Scanner census audit verifies zero hardcoded skip lists remain; `grep` for `node_modules` skip maps returns 0 matches in walker files. |
| `GAP-WS-04` | Universal Factory & Config Wiring | Only `internal/system/factory.go:2239` copies `world.ignore_patterns`; `init` (`initializer.go:360`) and `scan` (`cmd_init_scan.go:416`) discard user config. | [05-MEMBERSHIP-SPEC.md](05-MEMBERSHIP-SPEC.md): All scanner and workspace constructors require user configuration. | High | Phase 2 | Lane `L1` | `TestScanner_ReceivesUserIgnorePatterns` passes for `nerd init`, `nerd scan`, and campaign orchestrators. |
| `GAP-WS-05` | Semantic Discernment Overlay | All unignored files are treated as application code; seed data, golden answer keys, and stubs are fully parsed. | [05-MEMBERSHIP-SPEC.md](05-MEMBERSHIP-SPEC.md): `Membership` ingests `.nerd/orientation/membership.json`, exposing `Treatment(rel)`. | High | Phase 3 | Lane `O1` (`internal/orient`) | Test loading `membership.json` verifies a seed-data directory returns `Treatment() == "/index_names_only"`. |

---

## 2. Phase Execution Order

1. **Phase 1: Leaf Package Implementation** (`GAP-WS-01`, `GAP-WS-02`):
   Construct `internal/workspace` with fast `git ls-files` snapshotting, batched `git check-ignore` dynamic checking, and non-git fallback matching.
2. **Phase 2: Universal Repointing & Config Unification** (`GAP-WS-03`, `GAP-WS-04`):
   Migrate all ~35 production walkers to ask `Membership.IncludesDir` before descending, eliminating all private ignore lists and wiring user configuration through all factory constructors.
3. **Phase 3: Semantic Discernment Overlay Integration** (`GAP-WS-05`):
   Wire the orientation discernment artifact (`.nerd/orientation/membership.json`) into `internal/workspace`, enabling search and index tools to distinguish code from non-code datasets.
