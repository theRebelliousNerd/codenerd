---
doc-class: shipped
subsystem: workspace
implementation-status: shipped
last-verified: 2026-09-29
verified-against: e056692c
supersedes: []
---

# 02 — Current State — The Fragmented Walker Landscape Today

> Verified 2026-09-29 against `e056692c` (`main`). Every citation reflects source code inspected directly in the repository.

This document records the current state of filesystem traversal and workspace membership in codeNERD.

As of commit `e056692c`, package `internal/workspace` has **not shipped**. There is no unified workspace membership authority. Instead, repository traversal is fragmented across **approximately 35 independent production walkers** maintaining **roughly 25 separate, hardcoded directory ignore lists**, none of which honor `.gitignore`.

---

## 1. The Core World Scanners

The two primary world model scanners maintain separate, diverging directory exclusion logic:

1. **Full Directory Scanner** (`internal/world/fs.go:195-285`, `ScanDirectory`):
   - Enforces three exclusion tiers:
     - Tier 1 (`internal/world/fs.go:226-238`): Hardcoded map `ignoredDirs` containing `node_modules`, `vendor`, `dist`, `build`, `.git`, `.nerd` returning `filepath.SkipDir`.
     - Tier 2 (`internal/world/fs.go:241-263`): Dot-directory allowlist allowing only `.github`, `.vscode`, `.circleci`, `.config`; every other directory starting with `.` is skipped via `filepath.SkipDir`.
     - Tier 3 (`internal/world/fs.go:265, 285`): Calls `isIgnoredRel(rel, name, s.config.IgnorePatterns)` (`scanner_config.go:68-99`), which supports only exact name equality, prefix matching, or shallow `path.Match` (no `**` wildcard traversal).
2. **Incremental Workspace Scanner** (`internal/world/incremental_scan.go:128-182`, `ScanWorkspaceIncremental`):
   - Duplicates the Tier 2 dot-directory allowlist (`incremental_scan.go:137-153`).
   - **Critical Divergence**: **Omits the Tier 1 `ignoredDirs` map entirely**. If an operator configures an `ignore_patterns` list that excludes `vendor` but omits `node_modules`, the full scan skips `node_modules` while the incremental scan walks the entirety of `node_modules`.

---

## 2. The Duplicate Default Name Lists & The Factory Wiring Deficit

1. **Verbatim Code Duplication**:
   The default 13-name ignore list (`node_modules`, `vendor`, `.git`, `.nerd`, `dist`, `build`, `target`, `out`, `__pycache__`, `.venv`, `.next`, `.nuxt`, `coverage`) is duplicated verbatim in two files:
   - `internal/config/world.go:33-47` (`DefaultWorldConfig`)
   - `internal/world/scanner_config.go:42-56` (`DefaultScannerConfig`)
2. **The Factory Wiring Deficit**:
   `internal/system/factory.go:2239-2245` is the **only production site in the entire codebase** that copies `world.ignore_patterns` from user configuration onto a scanner instance.
   - `nerd init` (`internal/init/initializer.go:360`) instantiates `world.NewScanner()` with compiled defaults.
   - `nerd scan` (`cmd/nerd/cmd_init_scan.go:416`) instantiates `world.NewScanner()` with compiled defaults.
   - Campaign initialization (`internal/campaign/orchestrator_init.go:385`, `campaign_runner.go:251`, `cmd_campaign.go:178`) instantiates `world.NewScanner()` with compiled defaults.
   - **Result**: Neither `nerd init`, `nerd scan`, nor campaigns ever see the user's configured `world.ignore_patterns`.

---

## 3. World Model Ingestor Shard Blindness

In `internal/shards/system/world_model.go`:
- `performFullScan` (line 354) and `performIncrementalScan` (line 429) walk the filesystem to assert `file_topology` facts.
- **Root Path Disconnection**: Line 89 sets `RootPath: "."`, which resolves to the process current working directory rather than the canonical workspace root.
- **Missing SkipDir on Incremental**: Lines 430-431 check exclusions only on files; directory entries return `nil` without returning `filepath.SkipDir`. Consequently, the incremental walk stats every directory inside `node_modules` on every cycle.

---

## 4. CodeDOM, Structural, and Scope Walkers

1. **Structure Index** (`internal/world/structure_index.go:166`, `refreshLocked`):
   - Relies on `codemodel.SkipDir` (`internal/world/codemodel/model.go:168-176`), which skips `vendor`, `node_modules`, `testdata`, and any name starting with `.` or `_`. It is hardcoded to Go and Mangle files (`model.go:158-166`).
2. **File Scope Import Walker** (`internal/world/scope.go:683`, `findInboundDeps`):
   - Skips hidden directories and `vendor` (lines 687-690). **`node_modules` is not skipped**.
3. **Dataflow Extractor** (`internal/world/dataflow.go:607`, `ExtractDataFlowForDirectory`):
   - Skips hidden directories, `vendor`, and `node_modules` (line 619). Halts when `fileCount >= 10000` (line 614). Has zero production callers outside `dataflow_test.go`.

---

## 5. Retrieval and Search Walkers

1. **Native Sparse Retriever** (`internal/retrieval/sparse.go:602`, `searchSingleKeyword`):
   - Matches base directory names against `DefaultSparseRetrieverConfig` (`sparse.go:76-89`), maintaining its own private skip list (`*.pyc`, `__pycache__`, `.git`, `node_modules`, `*.egg-info`, `.tox`, `.pytest_cache`, `*.min.js`, `vendor`, `dist`, `build`, `.venv`, `venv`).
2. **Semantic Searcher** (`internal/retrieval/semantic.go:146`, `corpus`):
   - Skips `.git`, `node_modules`, `__pycache__`, `.venv`, `venv`, `vendor`, `dist`, `build` (lines 155-156). Caps files at 256.
3. **Tiered Context Builder** (`internal/retrieval/tiered_context.go:313`, `locateFile`):
   - Skips `.git`, `node_modules`, `__pycache__`, `.venv`, `venv`, `vendor` (lines 323-325).

---

## 6. Core Tools and VirtualStore Walkers

1. **Glob Tool** (`internal/tools/core/search.go:128`, `executeGlob`):
   - Contains **zero directory skips**. Descends every directory until `max_results` is hit.
2. **Grep and SearchCode Tools** (`internal/tools/core/search.go:361`, `contentSearch.run`):
   - Skips hidden directories and `node_modules`, `vendor` (lines 373-382).
3. **List Files Tool** (`internal/tools/core/file_ops.go:681`, `executeListFiles`):
   - Skips hidden directories only (lines 687-691). Does not skip `node_modules`.
4. **Shell Builtin Grep** (`internal/tools/shell/builtins.go:525`, `builtinGrep`):
   - Skips `.git`, `node_modules`, `vendor` (lines 532-533).
5. **VirtualStore Search Action** (`internal/core/virtual_store_file_actions.go:464`, `handleSearchCode`):
   - **Zero `filepath.SkipDir` logic**. Directories return `nil` (lines 465-466). Skips files containing `.git` or `.nerd` (line 469). Reads every other file under the byte limit in the entire tree.

---

## 7. Campaign, Evidence, and Chat Walkers

1. **Manifest Scanner** (`internal/init/scanner_dependencies.go:55`, `findManifestFiles`):
   - Defines a private 20-name skip list in `manifestSkipDirs` (`scanner_dependencies.go:17-23`).
2. **Strategic Knowledge Ingest** (`internal/init/strategic_knowledge.go:294`, `GatherProjectDocumentation`):
   - Defines a private 11-name skip list in `skipDirs` (`strategic_knowledge.go:286-291`) and skips hidden dirs except `.github` and `.claude` (lines 305-308).
3. **Multi-Step Delegation Discovery** (`cmd/nerd/chat/delegation_multistep.go:246`, `discoverFiles`):
   - Returns `nil` on directories without `SkipDir` (lines 247-248). Stats every entry in `node_modules`.
4. **Review Aggregator** (`cmd/nerd/chat/review_aggregator.go:1000`, `resolveReviewTarget`):
   - Returns `nil` on directories without `SkipDir` (line 1005). Stats the entire tree.
5. **Campaign Document Ingest** (`internal/campaign/decomposer_documents.go:164`, `readDocumentsFromDir`):
   - Contains **zero directory skips**. A directory path argument walks all matching extensions everywhere, including inside `node_modules`.
6. **Task Transaction Snapshots** (`internal/campaign/orchestrator_task_transaction.go:379`, `snapshotDirectoryFiles`):
   - Skips only `.git` and `.nerd` (lines 384-385). Reads every regular file in directory write-sets.

---

## 8. Summary: Zero `.gitignore` Support

Across the entire codebase:
- `go.mod` vendors zero `.gitignore` libraries.
- No scanner or walker parses `.gitignore` files.
- The string `".gitignore"` appears in `internal/world` only once: as a static test fixture string in `internal/world/dataflow_multilang_test.go:37`.
- The sole enumerator of the git index in production is `gitTrackedFiles` (`internal/docscheck/docscheck.go:723-740`), which is used solely for architectural documentation checks, never for world modeling.
