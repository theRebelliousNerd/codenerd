---
doc-class: shipped
subsystem: workspace
implementation-status: partial
last-verified: 2026-09-29
verified-against: 4dded472+working-tree
supersedes: []
---

# Walker census and repoint status

This is the source-reviewed L1/C1 checklist for the uncommitted working tree. Every owned row now queries membership. Caller runtime verification remains GAP-WS-06; a source row is not a passing execution claim.

| File & line | Function / caller | Membership witness | Repoint status / retained purpose |
|---|---|---|---|
| `internal/world/fs.go:158` | ScanDirectory | `internal/world/fs.go:258` | Shared scanner membership; purpose filters retained. |
| `internal/world/incremental_scan.go:69` | ScanWorkspaceIncremental | `internal/world/incremental_scan.go:136` | Directory pruning and file admission. |
| `internal/shards/system/world_model.go:381` | performFullScan | `internal/shards/system/world_model.go:405` | scanMembership uses loaded exclude patterns; parsed-language inclusion stays local. |
| `internal/shards/system/world_model.go:460` | performIncrementalScan | `internal/shards/system/world_model.go:474` | Rejected directories return SkipDir before checking files. |
| `internal/world/structure_index.go:170` | refreshLocked | `internal/world/structure_index.go:192` | Membership replaces the removed codemodel directory helper. |
| `internal/world/scope.go:666` | findInboundDeps | `internal/world/scope.go:696` | Membership replaces hidden/vendor skips. |
| `internal/world/dataflow.go:600` | ExtractDataFlowForDirectory | `internal/world/dataflow.go:625` | Repointed but dormant: production search finds only its definition; retained. |
| `internal/retrieval/sparse.go:462` | searchSingleKeyword native path | `internal/retrieval/sparse.go:634` | Membership determines traversal; bytecode/minified-file filters are purpose filters. |
| `internal/retrieval/backend.go:77` | RipgrepBackend.Search | `internal/retrieval/backend.go:161` | rg performs native ignore pruning; member-hit filtering adds configured exclusions. Absolute root and --hidden preserve relative-root and hidden-member results. |
| `internal/retrieval/semantic.go:144` | corpus | `internal/retrieval/semantic.go:157` | Membership before embedding corpus admission. |
| `internal/retrieval/tiered_context.go:304` | locateFile | `internal/retrieval/tiered_context.go:331` | Direct targets and recursive resolution are gated. |
| `internal/tools/core/search.go:101` | executeGlob | `internal/tools/core/search.go:171` | Membership plus existing containment and glob semantics. |
| `internal/tools/core/search.go:392` | contentSearch.run | `internal/tools/core/search.go:421` | Membership before content reads. |
| `internal/tools/core/file_ops.go:651` | executeListFiles | `internal/tools/core/file_ops.go:691` | Membership in recursive and direct listing; include_hidden remains a display filter. |
| `internal/tools/shell/builtins.go:418` | builtinGrep | `internal/tools/shell/builtins.go:542` | Recursive and direct targets use membership. |
| `internal/core/virtual_store_file_actions.go:444` | handleSearchCode | `internal/core/virtual_store_file_actions.go:476` | Membership replaces substring exclusions. |
| `internal/init/scanner_dependencies.go:36` | findManifestFiles | `internal/init/scanner_dependencies.go:66` | Private manifest directory list removed; depth/count bounds remain unchanged. |
| `internal/init/scanner.go:264` | detectEntryPointsForRoot | `internal/init/scanner.go:307` | Initializer-root membership also gates direct candidate reads. |
| `internal/init/scanner.go:970` | hasMainFunction | `internal/init/scanner.go:997` | Directory and direct-file admission. |
| `cmd/nerd/chat/helpers_scan.go:376` | runDirScan | `cmd/nerd/chat/helpers_scan.go:401` | Membership before scan descent. |
| `cmd/nerd/chat/helpers_files.go:139` | handleStatsIntent | `cmd/nerd/chat/helpers_files.go:201` | Membership before file counting. |
| `cmd/nerd/chat/helpers_files.go:265` | searchInFiles | `cmd/nerd/chat/helpers_files.go:275` | Membership before search reads. |
| `cmd/nerd/stats.go:16` | computeStats | `cmd/nerd/stats.go:95` | File targets without a workspace use their parent as authority root. |
| `cmd/nerd/chat/delegation_multistep.go:222` | discoverFiles | `cmd/nerd/chat/delegation_multistep.go:255` | Membership replaces path-substring filtering. |
| `cmd/nerd/chat/review_aggregator.go:979` | resolveReviewTarget | `cmd/nerd/chat/review_aggregator.go:1014` | Recursive and direct target admission. |
| `cmd/nerd/chat/ingest.go:202` | collectIngestFiles | `cmd/nerd/chat/ingest.go:233` | Membership plus supported-extension filtering. |
| `cmd/nerd/dom_replace_cmd.go:234` | collectReplaceFiles | `cmd/nerd/dom_replace_cmd.go:280` | Membership; import alias preserves the package-level workspace flag. |
| `internal/evidence/change.go:135` | Snapshot | `internal/evidence/change.go:170` | Membership with the existing evidence-purpose exception for project config/agent definitions under .nerd. |
| `internal/evidence/change.go:334` | verificationInputs | `internal/evidence/change.go:356` | Membership plus verification-input filtering. |
| `internal/gates/metrics.go:122` | walkSources | `internal/gates/metrics.go:145` | Membership; the private gates directory helper is removed. |
| `internal/campaign/recurse_workspace.go:258` | collectSourceFiles | `internal/campaign/recurse_workspace.go:278` | Direct membership instead of gates.SkipDir. |
| `internal/campaign/decomposer_documents.go:161` | readDocumentsFromDir | `internal/campaign/decomposer_documents.go:179` | Membership plus documentation extensions. |
| `internal/campaign/orchestrator_task_results.go:217` | writeSetBriefing | `internal/campaign/orchestrator_task_results.go:283` | Campaign-root membership replaces the last briefing skip map. |
| `internal/campaign/orchestrator_task_transaction.go:388` | snapshotDirectoryFiles | `internal/campaign/orchestrator_task_transaction.go:395` | Captured campaign-root authority; rollbackTaskExecutionSnapshot also gates cleanup (same file:487). |
| `internal/projectdoc/nerdmd.go:260` | LoadAll | `internal/projectdoc/nerdmd.go:296` | Membership; the inherited wscope import already fixes parameter shadowing. |
| `internal/mangle/lsp.go:603` | IndexWorkspace | `internal/mangle/lsp.go:622` | Membership before Mangle indexing. |
| `internal/browser/repo_trace.go:244` | runTraceScan | `internal/browser/repo_trace.go:311` | handleDir/handleFile query membership (same file:332); containment/purpose limits remain local. |

## Configuration entry paths

Default scanners query For against the scanned root, so init/scan/campaign constructors inherit target-root ignore configuration without caller shims (NewScanner/membership, `internal/world/fs.go:33`, `internal/world/fs.go:41`). The factory already supplies its loaded settings (initFinalExecutors, `internal/system/factory.go:2239`). World-model configuration copies loaded patterns and resolves the kernel workspace before scanning (WorldModelConfigFor/resolvedRoot/scanMembership, `internal/shards/system/world_model.go:133`, `internal/shards/system/world_model.go:340`, `internal/shards/system/world_model.go:358`).

## Deliberately outside this census

Strategic knowledge, orientation, browser session walkers and CodeDOM parsing belong to other lanes and were not modified by C1. Fixed-directory prompt, Mangle-development, runtime-database and developer-quality walks are not workspace-content membership decisions and remain outside repointing.

## What remains unverified or unbuilt

- CGO-dependent production caller tests and the tagged integrated build are blocked; policy guards also report concurrent wiring failures (GAP-WS-06).
- Semantic Treatment and orientation membership-artifact loading remain planned (GAP-WS-05).
- Ripgrep still has its own native ignore traversal and applies user overlays to returned hits; this does not prove traversal equivalence for extra exclusions or force-tracked ignored files (Search/keepMemberHits, `internal/retrieval/backend.go:77`, `internal/retrieval/backend.go:148`).
- Directory-read pruning is directly proved for Membership.Walk, not measured separately for every caller (TestWalkNeverEntersBigIgnoredDir, `internal/workspace/membership_test.go:162`).