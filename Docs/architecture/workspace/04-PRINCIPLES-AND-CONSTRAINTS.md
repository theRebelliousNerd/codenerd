---
doc-class: governance
subsystem: workspace
implementation-status: target-state
last-verified: 2026-09-29
verified-against: e056692c
supersedes: []
---

# 04 — Principles and Constraints — Workspace Invariants

Every modification, extension, or walker repointing associated with `internal/workspace` must strictly observe these six architectural principles.

---

### Principle 1: One Authority for the Entire Workspace
*Tied to: Workspace Brief (`L1_workspace_membership.txt:1-2`), Ruling on Single Membership Authority*

Exactly one package in codeNERD decides whether a file or directory belongs to the workspace: `internal/workspace`.
- **Invariant**: No tool, scanner, indexer, or chat command may maintain a private list of ignored directory names.
- **Invariant**: The duplicated 13-name default ignore list in `internal/world/scanner_config.go:42-56` is eradicated. The default name list lives in exactly one place: `internal/config/world.go:33-47` (`DefaultWorldConfig`).

---

### Principle 2: Git Is the Ground Truth Floor
*Tied to: Architectural Ruling (`ADR-001`), Ruling on Version Control Reality*

In any git work tree, git's index and ignore rules define the absolute baseline of workspace membership.
- **Invariant**: If git tracks a file (`git ls-files`), that file is a member of the workspace.
- **Invariant**: If git ignores a directory via `.gitignore`, codeNERD treats that directory as excluded, regardless of where the `.gitignore` file is situated in the directory hierarchy.

---

### Principle 3: Prune at the Highest Boundary
*Tied to: Performance Invariant, Ruling on Zero-Cost Exclusion*

Walkers must never stat, read, or descend into directories that are excluded from the workspace.
- **Invariant**: Walkers must invoke `Membership.IncludesDir(rel)` before descending into any directory. If the check returns `false`, the walker must immediately return `filepath.SkipDir`.
- **Invariant**: Returning `nil` on an ignored directory (which causes `filepath.Walk` to continue statting every entry inside it) is strictly forbidden.

---

### Principle 4: Zero Hidden-Directory Allowlist Special Cases
*Tied to: General-Purpose Invariance, Ruling on Tool Agnosticism*

The legacy practice of checking `strings.HasPrefix(name, ".")` against a hardcoded allowlist (`.github`, `.vscode`, `.circleci`, `.config`) is abolished.
- **Invariant**: Hidden directories are evaluated identically to normal directories: if git tracks them or does not ignore them, they are members. Preexisting coding-agent directories (`.claude/`, `.codex/`, `.agents/`, `.gemini/`) are admitted naturally whenever git tracks them.
- **Invariant**: The only directories universally excluded regardless of git tracking are `.git` (the repository database) and `.nerd` (codeNERD internal state).

---

### Principle 5: User Configuration Overlays Git
*Tied to: Configuration Invariance, Ruling on User Sovereignty*

The user's `world.ignore_patterns` configuration is applied on top of git truth as an extra exclusion layer.
- **Invariant**: Setting `world.ignore_patterns` allows operators to exclude tracked directories (e.g. large vendored assets or benchmark datasets).
- **Invariant**: Pattern evaluation must support full globbing semantics, including recursive `**` wildcards, directory prefix matching (`pattern/`), and `!` negation overrides.

---

### Principle 6: Concurrency-Safe and Fast
*Tied to: Subsystem Runtime SLA, Ruling on Zero Search Latency*

Workspace membership checks occur on every tool execution, code search, and file modification.
- **Invariant**: Membership checks must be non-blocking and concurrency-safe across multiple parallel agent goroutines.
- **Invariant**: Ground truth snapshots must be cached in memory, refreshing only upon explicit notification, scan ticks, or detected changes to `.git/index` or `HEAD`.
