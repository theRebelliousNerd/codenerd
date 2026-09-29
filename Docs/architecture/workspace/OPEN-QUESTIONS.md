---
doc-class: governance
subsystem: workspace
implementation-status: not-applicable
last-verified: 2026-09-29
verified-against: e056692c
supersedes: []
---

# OPEN-QUESTIONS — Workspace Invariants and Open Edges

This document catalogues the standing invariants and unresolved architectural questions for `internal/workspace`.

---

## 1. Standing Architectural Tripwires

A future author must never breach these three invariants:

### Tripwire 1: Zero Private Ignore Lists in Walkers
- **Constraint**: No walker may maintain an ad-hoc slice or map of directory names to skip (`node_modules`, `vendor`, `build`). All exclusion decisions must query `Membership.IncludesDir`.
- **Verification**: Code linters must fail if any walker file defines a local directory exclusion map.

### Tripwire 2: Zero Descent into Ignored Trees
- **Constraint**: Walkers must never return `nil` on excluded directories. If `IncludesDir(rel)` is false, the walker must immediately return `filepath.SkipDir`.
- **Verification**: Unit tests wrapping directory traversals must verify that `ReadDir` is never called on excluded directories.

### Tripwire 3: Zero Hidden Directory Special-Casing
- **Constraint**: Code must never check `strings.HasPrefix(name, ".")` against a hardcoded list of approved tool names. Git tracking alone decides whether a hidden file or directory belongs to the workspace.

---

## 2. Open Architectural Questions

### Question 1: Git Submodules and Nested Worktrees
- **Problem**: In a repository containing git submodules, `git ls-files` at the top level lists the submodule directory as a single gitlink entry (mode `160000`). It does not list the files inside the submodule.
- **Current Position**: `internal/workspace` treats submodules as member directories of the parent worktree.
- **Open Edge**: Should `Membership` recursively spawn a child `Membership` instance inside each submodule, or should submodules be treated as external dependencies?

### Question 2: Symlinks and Windows Directory Junctions
- **Problem**: `filepath.Walk` and `WalkDir` report symlinks via `Lstat` (`IsDir() == false`). They do not descend into symlinked directories. On Windows, directory junctions exhibit complex stat behaviors.
- **Current Position**: `internal/workspace` adheres strictly to git's standard behavior: symlinks are treated as pointer files and are not followed as directories.
- **Open Edge**: Does codeNERD require a configuration option to follow internal directory symlinks within the workspace boundary?

### Question 3: Dynamic `git check-ignore` Subprocess Lifecycle
- **Problem**: When an agent performs thousands of file edits during a multi-step campaign, issuing dynamic `git check-ignore` checks could cause process spawn overhead.
- **Current Position**: Batching queries through a long-lived `git check-ignore -z --stdin` pipe with mutex locking and caching in a `sync.Map`.
- **Open Edge**: If git locks the repository index during an external commit, the pipe might block. Should the pipe enforce a strict 250ms deadline with a non-git fallback?
