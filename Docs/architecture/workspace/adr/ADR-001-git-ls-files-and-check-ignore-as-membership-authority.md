---
doc-class: governance
subsystem: workspace
implementation-status: accepted-not-implemented
last-verified: 2026-09-29
verified-against: e056692c
supersedes: []
---

# ADR-001: `git ls-files` and Batched `check-ignore` as the Single Membership Authority

## Context

Across the codeNERD repository, filesystem traversal is currently managed by approximately 35 independent walkers that collectively maintain roughly 25 private, hardcoded ignore lists. Key systems duplicate ignore logic:
- `internal/world/fs.go:226-263` hardcodes a 6-directory skip map and an allowlist of 4 dot-directories (`.github`, `.vscode`, `.circleci`, `.config`).
- `internal/world/incremental_scan.go:137-153` duplicates the dot allowlist but drops `node_modules`.
- `internal/config/world.go:33-47` and `internal/world/scanner_config.go:42-56` duplicate a 13-name default ignore list.
- Walkers in `internal/tools/core/search.go:128` (`executeGlob`) and `internal/core/virtual_store_file_actions.go:464` (`handleSearchCode`) perform zero directory skips.

Crucially, **nothing in codeNERD currently parses or obeys `.gitignore`**. On a foreign repository where git tracks ~20,000 files, codeNERD traverses over 100,000 files, statting virtual environments, python bytecode caches, temporary test directories, and build artifacts. This causes severe search degradation, excessive CPU consumption, and token budget exhaustion.

## Decision

1. **Establish `internal/workspace` as the Sole Authority**:
   Create a leaf package `internal/workspace` that owns the canonical `Membership` type. All ~35 walkers across scanners, tools, CodeDOM, campaigns, and chat handlers will be repointed to query `Membership.IncludesDir` and `Membership.Includes`.
2. **Git Ground Truth Floor**:
   In any git work tree, baseline membership is established via a single streaming invocation of `git ls-files -z -co --exclude-standard` executed with `GIT_OPTIONAL_LOCKS=0`.
3. **Batched Dynamic Ignore Checking**:
   Newly created or unstaged files that do not exist in the cached snapshot are evaluated via a persistent background `git check-ignore -z --stdin` process and cached in memory.
4. **User Extra Exclusions on Top**:
   The user's `world.ignore_patterns` configuration is applied on top of git truth in both git and non-git modes, supporting true recursive `**` wildcards, directory prefix matching, and `!` negation overrides.
5. **Universal Pruning Contract**:
   Walkers must call `Membership.IncludesDir(rel)` on directory entries and return `filepath.SkipDir` immediately if `false`.

## Consequences

- **Positive**: codeNERD automatically obeys all `.gitignore` rules across all languages; walkers immediately prune ignored trees at the top directory boundary; eliminates ~25 duplicate ignore lists from the codebase.
- **Negative**: Adds a dependency on the `git` binary for repository workspaces (mitigated by a non-git fallback using `world.ignore_patterns`).
- **Risks**: High file churn in massive repositories requires robust cache invalidation without causing process spawn overhead.

## Witness

**Witness:** `test:TestMembership_GitignoreRules` and `symbol:internal/workspace.For`

| Claim | Witness (Code Evidence as of 2026-09-29) |
|---|---|
| Hardcoded skip lists exist today | `ignoredDirs` in `internal/world/fs.go:226-238` and `manifestSkipDirs` in `internal/init/scanner_dependencies.go:17-23`. |
| Git index enumeration pattern exists | `internal/docscheck/docscheck.go:723-740` (`gitTrackedFiles`). |
| Target constructor symbol | `For(root string, cfg *config.WorldConfig) (*Membership, error)` in `internal/workspace/membership.go`. |
| Proving regression test | `TestMembership_GitignoreRules` verifying that nested `.gitignore` files exclude target directories and files while admitting unignored files. |

## Status (Derived, Not Asserted)

**`accepted-not-implemented`**. Derivation: The decision is approved, but `internal/workspace` has not yet landed in commit `e056692c`. Status flips to `implemented` once the package is created and passes `TestMembership_GitignoreRules`.
