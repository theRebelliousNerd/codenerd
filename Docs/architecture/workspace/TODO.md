---
doc-class: governance
subsystem: workspace
implementation-status: target-state
last-verified: 2026-09-29
verified-against: e056692c
supersedes: []
---

# TODO — Workspace Build Queue

This document specifies the leaf implementation tasks for `internal/workspace` and universal walker repointing. Every task traces directly to an identifier in [03-GAP-ANALYSIS.md](03-GAP-ANALYSIS.md).

---

## Phase 1: Leaf Package Implementation (`internal/workspace`)

- [ ] `TODO-WS-01` (`GAP-WS-01`, Lane `L1`): Author `internal/workspace/membership.go` defining the `Membership` struct and public API (`For`, `Includes`, `IncludesDir`, `Walk`, `Files`, `Treatment`, `Refresh`).
- [ ] `TODO-WS-02` (`GAP-WS-02`, Lane `L1`): Implement streaming `git ls-files -z -co --exclude-standard` snapshotting and directory ancestor map construction in `internal/workspace`.
- [ ] `TODO-WS-03` (`GAP-WS-02`, Lane `L1`): Implement batched `git check-ignore -z --stdin` subprocess evaluator with `sync.Map` caching for newly created or unstaged files.
- [ ] `TODO-WS-04` (`GAP-WS-02`, Lane `L1`): Implement non-git fallback pattern matching with full `**`, directory prefix, and `!` negation glob semantics.
- [ ] `TODO-WS-05` (`GAP-WS-01`, Lane `L1`): Implement unit test suite in `internal/workspace/membership_test.go` covering git, non-git, and dynamic ignore cases.

---

## Phase 2: Configuration Unification & Factory Wiring

- [ ] `TODO-WS-06` (`GAP-WS-04`, Lane `L1`): Delete duplicate 13-name ignore list in `internal/world/scanner_config.go:42-56`, centralizing defaults in `internal/config/world.go:33-47` (`DefaultWorldConfig`).
- [ ] `TODO-WS-07` (`GAP-WS-04`, Lane `L1`): Update scanner constructors to accept `config.WorldConfig` across `internal/init/initializer.go:360`, `cmd/nerd/cmd_init_scan.go:416`, and `internal/campaign/orchestrator_init.go:385`.
- [ ] `TODO-WS-08` (`GAP-WS-04`, Lane `L1`): Update `internal/shards/system/world_model.go` to set `RootPath` to the canonical workspace root and pass user ignore patterns.

---

## Phase 3: Universal Walker Repointing

- [ ] `TODO-WS-09` (`GAP-WS-03`, Lane `L1`): Repoint `internal/world/fs.go:195` (`ScanDirectory`) and `internal/world/incremental_scan.go:128` (`ScanWorkspaceIncremental`) to query `Membership.IncludesDir`.
- [ ] `TODO-WS-10` (`GAP-WS-03`, Lane `L1`): Repoint `internal/world/structure_index.go:166` and `internal/world/codemodel/model.go:168` (`SkipDir`) to query `Membership.IncludesDir`.
- [ ] `TODO-WS-11` (`GAP-WS-03`, Lane `L1`): Repoint `internal/world/scope.go:683` (`findInboundDeps`) and `internal/world/dataflow.go:607` to query `Membership.IncludesDir`.
- [ ] `TODO-WS-12` (`GAP-WS-03`, Lane `L1`): Repoint retrieval walkers in `internal/retrieval/sparse.go:602`, `semantic.go:146`, and `tiered_context.go:313` to query `Membership.IncludesDir`.
- [ ] `TODO-WS-13` (`GAP-WS-03`, Lane `L1`): Repoint search tools in `internal/tools/core/search.go:128, 361`, `file_ops.go:681`, and `shell/builtins.go:525`.
- [ ] `TODO-WS-14` (`GAP-WS-03`, Lane `L1`): Repoint VirtualStore in `internal/core/virtual_store_file_actions.go:464` (`handleSearchCode`).
- [ ] `TODO-WS-15` (`GAP-WS-03`, Lane `L1`): Repoint campaign walkers in `internal/campaign/recurse_workspace.go:260`, `decomposer_documents.go:164`, and `orchestrator_task_transaction.go:379`.
- [ ] `TODO-WS-16` (`GAP-WS-03`, Lane `L1`): Repoint chat walkers in `cmd/nerd/chat/helpers_scan.go`, `helpers_files.go`, `stats.go`, and `delegation_multistep.go`.

---

## Phase 4: Semantic Discernment Overlay Integration

- [ ] `TODO-WS-17` (`GAP-WS-05`, Lane `O1`): Implement `Treatment(rel string)` in `internal/workspace/membership.go` reading `.nerd/orientation/membership.json`.
- [ ] `TODO-WS-18` (`GAP-WS-05`, Lane `O1`): Update CodeDOM indexers and vector embedders to consult `Treatment(rel)` and skip body parsing on `/index_names_only`.
