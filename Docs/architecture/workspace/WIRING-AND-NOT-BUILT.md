---
doc-class: shipped
subsystem: workspace
implementation-status: shipped
last-verified: 2026-09-29
verified-against: e056692c
supersedes: []
---

# WIRING-AND-NOT-BUILT — The Universal Walker Census & Repointing Checklist

> Verified 2026-09-29 against `e056692c` (`main`).

This document provides the authoritative census of every filesystem walker in codeNERD. It serves as the engineering repointing checklist for Lane `L1`: every workspace-content walker must be migrated to query `internal/workspace`, completely replacing private skip lists.

---

## 1. Workspace Content Walkers (The Migration Checklist)

These walkers traverse the user's repository and determine which files codeNERD discovers, indexes, and searches. Every row in this table must be repointed.

| File & Line | Function / Caller | What It Feeds | Exclusion Logic Today (To Be Deleted) | Target Repointing Action |
|---|---|---|---|---|
| `internal/world/fs.go:195` | `(*Scanner).ScanDirectory` | Topology & language facts | Hardcoded `ignoredDirs` (`226-238`), dot allowlist (`241-262`), `isIgnoredRel` (`265, 285`). | Call `Membership.IncludesDir(rel)` on dirs (returning `filepath.SkipDir` if false); call `Membership.Includes(rel)` on files. |
| `internal/world/incremental_scan.go:128` | `(*Scanner).ScanWorkspaceIncremental` | Delta topology facts | Dot allowlist (`137-153`), `isIgnoredRel` (`154, 165`). No `node_modules` map. | Repoint to `Membership.IncludesDir` / `Includes`. |
| `internal/shards/system/world_model.go:354` | `(*WorldModelIngestorShard).performFullScan` | Shard knowledge facts | Hardcoded `ExcludePatterns` (`96-100`); `RootPath "."` (`89`). | Set `RootPath` to canonical root; repoint to `Membership.IncludesDir`. |
| `internal/shards/system/world_model.go:429` | `performIncrementalScan` | Steady-state topology | Exclude patterns checked on files only; dirs return `nil` without `SkipDir` (`430-431`). | Add `Membership.IncludesDir` with immediate `filepath.SkipDir`. |
| `internal/world/structure_index.go:166` | `(*StructureIndex).refreshLocked` | CodeDOM structural index | `codemodel.SkipDir` (`model.go:168-176`), `tools.IsSecretPath` (`187`). | Repoint directory skip to `Membership.IncludesDir`. |
| `internal/world/scope.go:683` | `(*FileScope).findInboundDeps` | Inbound Go imports | Skips hidden and `vendor` (`687-690`). `node_modules` not skipped. | Repoint to `Membership.IncludesDir`. |
| `internal/world/dataflow.go:607` | `ExtractDataFlowForDirectory` | Data-flow facts | Skips hidden, `vendor`, `node_modules` (`619`), caps at 10,000 files (`614`). Dormant. | Repoint to `Membership.IncludesDir` or retire if obsolete. |
| `internal/retrieval/sparse.go:602` | `(*SparseRetriever).searchSingleKeyword` | Native keyword retrieval | `DefaultSparseRetrieverConfig` 13-name skip list (`76-89`). | Replace private skip list with `Membership.IncludesDir` / `Includes`. |
| `internal/retrieval/backend.go:77` | `(*RipgrepBackend).Search` | Ripgrep keyword search | Passes `--glob !pattern` (`94-98`). | Pass `--ignore-file` pointing to active gitignore; rely on rg native gitignore. |
| `internal/retrieval/semantic.go:146` | `(*EmbeddingSemanticSearcher).corpus` | Embedding search corpus | Skips `.git`, `node_modules`, `__pycache__`, `venv`, `vendor` (`155-156`). | Repoint to `Membership.IncludesDir` / `Includes`. |
| `internal/retrieval/tiered_context.go:313` | `(*TieredContextBuilder).locateFile` | Partial path resolution | Skips `.git`, `node_modules`, `__pycache__`, `venv`, `vendor` (`323-325`). | Repoint to `Membership.IncludesDir`. |
| `internal/tools/core/search.go:128` | `executeGlob` | `glob` tool | Zero directory skips; `skipUncontained` on symlinks (`137-141`). | Call `Membership.IncludesDir` on directories before descent. |
| `internal/tools/core/search.go:361` | `contentSearch.run` (`grep`, `search_code`) | Tool grep results | Skips hidden dirs and `node_modules`, `vendor` (`373-382`). | Repoint to `Membership.IncludesDir` / `Includes`. |
| `internal/tools/core/file_ops.go:681` | `executeListFiles` (recursive) | `list_files` tool | Skips hidden only (`687-691`). Does not skip `node_modules`. | Call `Membership.IncludesDir` with `filepath.SkipDir`. |
| `internal/tools/shell/builtins.go:525` | `builtinGrep` | Shell grep command | Skips `.git`, `node_modules`, `vendor` (`532-533`). | Repoint to `Membership.IncludesDir` / `Includes`. |
| `internal/core/virtual_store_file_actions.go:464` | `(*VirtualStore).handleSearchCode` | Kernel search actions | No `SkipDir`; skips `.git` and `.nerd` substrings (`469`). Reads every file. | Repoint to `Membership.IncludesDir` with immediate `SkipDir`. |
| `internal/init/scanner_dependencies.go:55` | `findManifestFiles` | Manifest detection | Private `manifestSkipDirs` 20-name list (`17-23`). | Repoint to `Membership.IncludesDir`. |
| `internal/init/scanner.go:284, 954` | `detectEntryPointsForRoot`, `hasMainFunction` | Entry point detection | Zero directory skips; reads every `.go` under root. | Repoint to `Membership.IncludesDir`. |
| `cmd/nerd/chat/helpers_scan.go:388` | `runDirScan` | User directory scan | Skips hidden dirs only (`394-397`). No `node_modules` skip. | Repoint to `Membership.IncludesDir`. |
| `cmd/nerd/chat/helpers_files.go:168` | `handleStatsIntent` | `/stats` line counts | Skips dot names, `node_modules`, `vendor`, `bin`, `build`, `tmp` (`178-182`). | Repoint to `Membership.IncludesDir`. |
| `cmd/nerd/chat/helpers_files.go:240` | `searchInFiles` | Chat in-file search | Skips hidden only (`245-247`). No `node_modules` skip. | Repoint to `Membership.IncludesDir`. |
| `cmd/nerd/stats.go:66` | `computeStats` | CLI stats command | Duplicates chat stats skip list (`76-80`). | Repoint to `Membership.IncludesDir`. |
| `cmd/nerd/chat/delegation_multistep.go:246` | `discoverFiles` | Multi-step file list | Returns `nil` on dirs; filters paths with substring checks (`252-260`). | Add `Membership.IncludesDir` with `filepath.SkipDir`. |
| `cmd/nerd/chat/review_aggregator.go:1000` | `resolveReviewTarget` | Review file target list | Returns `nil` on dirs; filters files by substring (`1005-1007`). | Add `Membership.IncludesDir` with `filepath.SkipDir`. |
| `cmd/nerd/chat/ingest.go:210` | `collectIngestFiles` | Knowledge ingest | Returns `nil` on directories without `SkipDir` (`214-215`). | Add `Membership.IncludesDir` with `filepath.SkipDir`. |
| `cmd/nerd/dom_replace_cmd.go:268` | `collectReplaceFiles` | DOM replace targets | Skips `.git`, `.nerd`, `vendor`, `node_modules` (`275-276`). | Repoint to `Membership.IncludesDir`. |
| `internal/evidence/change.go:143` | `Snapshot` | Acceptance hash | Skips `.git`, enters `.nerd/agents` only (`155-163`). Non-regular fails. | Repoint to `Membership.IncludesDir`. |
| `internal/evidence/change.go:323` | `verificationInputs` | Verification hash | Skips `.git`, `.nerd` (`331-332`). Filters Go files. | Repoint to `Membership.IncludesDir`. |
| `internal/gates/metrics.go:137` | `walkSources` | Metric line counts | `gates.SkipDir` 12-name list (`32-36`). | Repoint to `Membership.IncludesDir`. |
| `internal/campaign/recurse_workspace.go:260` | `collectSourceFiles` | Recurse DAG sources | `gates.SkipDir` (`273`). | Repoint to `Membership.IncludesDir`. |
| `internal/campaign/decomposer_documents.go:164` | `readDocumentsFromDir` | Campaign doc ingest | Zero directory skips; reads matching extensions inside `node_modules`. | Repoint to `Membership.IncludesDir`. |
| `internal/campaign/orchestrator_task_results.go:275` | `writeSetBriefing` | Task file modify list | Skips `.git`, `.nerd`, `vendor`, `testdata` (`229-234`). | Repoint to `Membership.IncludesDir`. |
| `internal/campaign/orchestrator_task_transaction.go:379` | `snapshotDirectoryFiles` | Task rollback snapshots | Skips only `.git` and `.nerd` (`384-385`). Reads every file. | Repoint to `Membership.IncludesDir`. |
| `internal/projectdoc/nerdmd.go:282` | `LoadAll` | `nerd.md` loader | Skips hidden dirs, `node_modules`, `vendor` (`288-293`). | Repoint to `Membership.IncludesDir`. |
| `internal/mangle/lsp.go:602` | `IndexWorkspace` | Mangle LSP indexer | Skips `node_modules`, `.git`, `vendor`, `.nerd/cache` (`615-619`). | Repoint to `Membership.IncludesDir`. |
| `internal/browser/repo_trace.go:263` | `runTraceScan` | Browser repo trace | `isSkippedRepoDir` 7-name list (`143-149`). | Repoint to `Membership.IncludesDir`. |

---

## 2. Fixed-Directory and Developer-Gate Walks (Do Not Repoint)

These walkers inspect codeNERD's own internal directories or developer validation gates. They do **not** decide workspace membership for the user's repository and must **not** be repointed to `internal/workspace`:

- `internal/core/mangle_watcher.go:51, 408`: Watches `.nerd/mangle` and `internal/mangle` for Mangle language development.
- `internal/prompt/atom_schema.go:269`: Parses prompt atoms within configured atom directories.
- `internal/prompt/loader.go:518, 562`: Reads `.nerd/prompts` and `.nerd/agents` for prompt synchronization.
- `internal/store/reembed_all.go:143`: Re-embeds databases under `.nerd/`.
- `internal/docscheck/docscheck.go:158`: Validates architecture markdown files under `Docs/architecture/`.
- `internal/campaign/intelligence_gathering_methods.go:461`: Inspects `.nerd/campaigns`.
- `cmd/tools/audit_*`: Developer quality gates inspecting codeNERD's own repository structure.
