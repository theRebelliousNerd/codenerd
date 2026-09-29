---
doc-class: north-star
subsystem: workspace
implementation-status: target-state
last-verified: 2026-09-29
verified-against: 4dded472+working-tree
supersedes: []
---

# 05 — Membership Spec — The Architecture of `internal/workspace`

This capability specification details the design, algorithms, API contracts, and concurrency models for the single workspace membership authority package, `internal/workspace`, owned by Lane `L1`.

---

## 1. Package Purpose & API Contract

Package `internal/workspace` is a low-level leaf package positioned immediately above the operating system and git CLI. It imports no business logic packages (`internal/world`, `internal/init`, or `internal/campaign`), ensuring zero cyclic dependencies.

### Primary Public Interface

```go
package workspace

import (
	"context"
	"io/fs"
)

// Membership represents the concurrency-safe membership authority for a root.
type Membership struct {
	// unexported fields
}

// For returns or constructs the canonical Membership instance for a workspace root.
func For(root string) (*Membership, error)

// Open supplies an explicit extra-exclusion list from an already loaded configuration.
func Open(root string, patterns []string) (*Membership, error)

// Includes reports whether a relative file path is a member of the workspace.
func (m *Membership) Includes(rel string) bool

// IncludesDir reports whether a directory should be traversed.
// If false, walkers MUST return filepath.SkipDir immediately.
func (m *Membership) IncludesDir(rel string) bool

// Walk executes a fast, non-statting traversal over all member files.
func (m *Membership) Walk(ctx context.Context, fn func(rel string, entry fs.DirEntry) error) error

// Files returns a copy of all current member file paths in repo-relative form.
func (m *Membership) Files() []string

// Treatment returns the semantic operational treatment derived by orientation.
// Returns "/index_and_parse", "/index_names_only", or "/exclude".
func (m *Membership) Treatment(rel string) string

// Refresh forces a re-snapshot of `git ls-files -z -co --exclude-standard` and re-evaluation of ignore rules.
func (m *Membership) Refresh() error
```

---

## 2. In a Git Repository: Git Truth & Batched Dynamic Checking

When `root` resides within a git work tree:

### Baseline Snapshot Construction
1. On construction (`For`) or refresh (`Refresh`), `internal/workspace` executes:
   ```bash
   git -C <absRoot> ls-files -z -co --exclude-standard
   ```
2. The command runs with environment variable `GIT_OPTIONAL_LOCKS=0`.
3. The zero-delimited (`-z`) byte stream is parsed without allocation into:
   - `files`: a hash map of repo-relative paths (`map[string]struct{}`).
   - `dirs`: a hash map of all ancestor directory paths for every file in `files`.
4. Any path in `dirs` immediately returns `IncludesDir(rel) == true`.
5. Any directory not in `dirs` (e.g. `node_modules`, `venv`, `build`) immediately returns `IncludesDir(rel) == false`, prompting the walker to return `filepath.SkipDir`.

### Dynamic Evaluation for Newly Created Files
When an agent or tool creates an unstaged, untracked file during an active turn:
1. `Includes(rel)` checks the cached `files` map. If found, returns `true`.
2. If absent from `files`, the path is queued for batched dynamic evaluation.
3. A batched request executes `git check-ignore -z --stdin`; `Walk` groups unknown siblings before descending:
   - If git check-ignore returns the path, the file is ignored $\rightarrow$ cached as non-member (`false`).
   - If git check-ignore does not return the path, the file is untracked but not ignored $\rightarrow$ cached as member (`true`).
4. Results are stored in a mutex-protected delta cache. Results from an older snapshot cannot overwrite a refreshed cache.

### Automatic Cache Invalidation
The snapshot records control-file timestamps. Membership queries refresh when they change. Explicit `Refresh` always reloads the snapshot, including nested ignore changes and newly created files that do not move the index.

---

## 3. Non-Git Fallback & User Extra Exclusions

### Non-Git Mode
If `root` is not a git repository (e.g. an unpacked source archive):
1. `Membership` falls back to evaluating `world.ignore_patterns`.
2. The default ignore patterns come exclusively from `config.DefaultWorldConfig` (`internal/config/world.go:41`).
3. Pattern matching is implemented with full globbing semantics:
   - Recursive `**` wildcards (e.g. `**/target/**`).
   - Directory prefix rules (`pattern/` matches directories and all descendants).
   - Negation rules (`!pattern` overrides preceding exclusions).

### User Extra Exclusions Overlay
In **both git and non-git modes**, user-configured `world.ignore_patterns` are applied on top of git truth as extra exclusions.
- If a repository tracks large fixture files that an operator wishes to hide from codeNERD, adding those paths to `.nerd/config.json` under `world.ignore_patterns` causes `Includes` and `IncludesDir` to return `false`, pruning them from the workspace.

---

## 4. Semantic Discernment Overlay Integration

`internal/workspace` integrates with the orientation discernment subsystem:
1. It monitors `.nerd/orientation/membership.json` (generated by `internal/orient/trees.go`).
2. When present, `Membership` loads the directory treatment map.
3. `Treatment(rel string)` resolves the treatment for any path:
   - `/index_and_parse`: Normal application source code. Full CodeDOM and AST indexing.
   - `/index_names_only`: Seed datasets, golden answer keys, large fixtures. File paths are preserved for reference, but file bodies are skipped by CodeDOM indexers and vector embedders.
   - `/exclude`: Directory is skipped entirely.

---

## 5. Universal Walker Migration Plan

Every walker in codeNERD is modified to follow a standard two-line pattern:

```go
if info.IsDir() {
    if !membership.IncludesDir(relPath) {
        return filepath.SkipDir
    }
    return nil
}
if !membership.Includes(relPath) {
    return nil
}
```

All private `ignoredDirs` maps, dot-prefix allowlists, and ad-hoc string comparisons are completely excised.

Directory write-set briefings, transaction snapshots, and rollback cleanup must use the campaign workspace root's authority, even when traversal starts below that root. Cleanup preserves excluded trees. Init entry-point and manifest discovery must also gate direct reads and subtree walks against the initializer's workspace membership; purpose filters remain local.

---

## 6. Verification Seams & Tests

1. `TestMembership_GitignoreRules`: Constructs a temporary git repository with root and nested `.gitignore` files (`dir/`, `*.log`, `!important.log`, `**/tmp/**`), asserting exact exclusion of ignored files and admission of unignored files.
2. `TestMembership_DynamicCheckIgnore`: Creates a file during the test that is ignored by `.gitignore` but not yet committed; verifies that `Includes` returns `false` via the batched `check-ignore` path.
3. `TestMembership_PruningPreventsDescent`: Constructs a deep directory tree inside an ignored directory (`ignored/deep/nested/file.txt`); wraps `filepath.Walk` with a directory read counter and asserts that `ReadDir` is never called on the ignored directory.
4. `TestMembership_UserExtraExcludes`: Asserts that a git-tracked file is successfully excluded when added to `world.ignore_patterns`.
