---
doc-class: shipped
subsystem: workspace
implementation-status: shipped
last-verified: 2026-09-29
verified-against: e056692c
supersedes: []
---

# IMPLEMENTED_SPEC — Shipped Workspace Truth

> **Authoritative Baseline Record**. Verified 2026-09-29 against commit `e056692c`. If any other document in this directory makes claims about current shipped functionality that contradict this file, **this file wins**.

---

## 1. Absolute Truth: Nothing of `internal/workspace` Has Shipped

As of commit `e056692c`:
- **`internal/workspace` does not exist on disk**. There is no unified workspace membership package, no `git ls-files` snapshotting system, and no `git check-ignore` batched subprocess in the repository tree.
- **`.gitignore` is completely unparsed and unhonored** by all scanners, indexers, and tools.
- **Filesystem traversal remains fragmented** across roughly 35 independent walkers maintaining roughly 25 private ignore lists.
- **Lane `L1`** is actively authoring `internal/workspace` and executing the universal repointing migration.

---

## 2. Shipped Baseline of Walker Behavior Today

The actual behavior of the codebase today is verified across these specific source locations:

1. **World Scanners**:
   - `internal/world/fs.go:226-238`: Hardcodes `ignoredDirs` map (`node_modules`, `vendor`, `dist`, `build`, `.git`, `.nerd`).
   - `internal/world/fs.go:241-263`: Hardcodes dot-directory allowlist allowing only `.github`, `.vscode`, `.circleci`, `.config`.
   - `internal/world/incremental_scan.go:137-153`: Enforces dot-directory allowlist but omits `node_modules` map.
2. **Duplicate Default Lists**:
   - `internal/config/world.go:33-47` and `internal/world/scanner_config.go:42-56` duplicate the 13-name default ignore list.
3. **Factory Configuration Gap**:
   - `internal/system/factory.go:2239-2245` is the only call site that copies `world.ignore_patterns`.
   - `internal/init/initializer.go:360` and `cmd/nerd/cmd_init_scan.go:416` instantiate scanners with compiled defaults, silently ignoring user configurations.
4. **Git Tracking Reference**:
   - `internal/docscheck/docscheck.go:723-740` (`gitTrackedFiles`) executes `git -C <abs> ls-files -z` for architectural documentation validation only.
