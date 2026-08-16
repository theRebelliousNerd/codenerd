# tools — TODO

> Last verified: **2026-08-16**  
> Prioritized backlog for `internal/tools` and tightly coupled contracts.

## P0 — Safety

- [x] Apply `resolveWorkspacePath` (or equivalent) to `glob`, `grep`, `search_code` base_path/path.  
      `core.searchBase` resolves `base_path`/`path`; the `**` pattern prefix is
      resolved too (it was a second, unchecked path argument); both walks refuse
      to follow symlinks. An omitted argument now means the workspace root, not
      the process working directory.
- [x] Apply workspace containment to codedom path args (elements + line tools).  
      `get_elements` / `get_element` route `path` through
      `tools.ResolveWorkspacePath`; the line tools already did.
- [x] Contain shell/git `working_dir` to workspace root.  
      `shell.resolveWorkingDir` guards `run_command`, `bash`, `run_build`,
      `run_tests` and all three git tools; `git_diff` / `git_log` also contain
      their pathspec.
- [x] Contract with session: empty `AllowedTools` must not mean "all tools" when the safety gate is on.  
      `Registry.SetAllowlist(*Allowlist)`: `Enforced` is a separate field from
      the name list, so an enforced-and-empty envelope denies everything. See
      09-SAFETY-AND-INVARIANTS.md. **Wiring pending**: `internal/session` must
      call `SetAllowlist` whenever the effective config changes.

## P1 — Catalog & correctness

- [x] Add `git_diff`, `git_log`, `git_operation` to `modular_tool_allowed` in `intent_routing.mg`.  
      Also added `apply_edits`, plus a `/git` verb category for the mutating one.
- [x] Add `research_cache_clear`, `research_cache_stats` to Mangle routing.  
      `_stats` is read-only and follows the cache wherever it is reachable;
      `_clear` stays confined to `/research`, because the cache is a
      process-wide singleton and clearing it discards other agents' work.
- [x] Use `coerceInt` for search tool integer args (`max_results`, `context_lines`).  
      All four private copies collapsed into `tools.CoerceInt` / `tools.ArgInt`.
- [x] Thread workspace root through tool context; reduce env-only coupling.  
      `Registry.SetWorkspaceRoot` → context → `tools.WorkspaceRoot(ctx)`;
      `CODENERD_WORKSPACE_ROOT` demoted to a fallback. **Wiring pending**:
      `internal/system/factory.go` should call `tools.SetGlobalWorkspaceRoot`
      beside its existing `os.Setenv`.
- [x] Golden test: RegisterAll names ⊆ Mangle modular_tool_allowed ∪ intentional_exceptions.  
      `internal/tools/catalog_golden_test.go`, enforced in both directions.

## P2 — Hygiene

- [x] Rewrite `codedom/doc.go` to match registered tools.  
      `codedom/doc.go` was already accurate; `core/doc.go` and `shell/doc.go`
      were not. A golden test now pins every doc list to `RegisterAll`.
- [x] Decide CategoryReview / CategoryAttack: implement tools or stop mapping intents to empty categories.  
      Decision: implement. `Tool.AltCategories` lets one tool serve several
      intent families, and the read / inspect / exec tools now declare
      `/review`, `/attack` and `/general`. A test asserts that no intent in
      `intentToCategory` resolves to an empty toolbox.
- [x] Prefer `logging.Tools*` for file/shell completions instead of the VirtualStore channel.
- Decided NO - Register tools only once into Global from the VS pointer to eliminate dual-map drift risk.
      The duplication is real: internal/core/virtual_store_tools.go HydrateModularTools calls RegisterAll twice for each tool family, once into the VirtualStore's own modularTools registry and once into tools.Global(), and installs the write guard and fact sink on both.
      The proposed fix is to register once "from the VS pointer", which means making the VirtualStore registry and the global registry the same object.
      That is not available as written: internal/tools/registry.go:514 declares `var globalRegistry = NewRegistry()` as a package-level singleton with accessors (Global, SetGlobalWriteGuard, SetGlobalAllowlist, SetGlobalWorkspaceRoot, SetGlobalFactSink) but no setter for the registry itself. Aliasing would require introducing one.
      Introducing one would be worse than the problem. Every VirtualStore in the process would then share one mutable registry, so tests that construct more than one VirtualStore would contaminate each other, and a tool registered by one workspace would be visible to another. The current duplication is bounded and deterministic, and TestCatalog_WhenHydratedTwice_ShouldProduceIdenticalRegistries already pins the property that makes it safe - that hydrating twice produces identical registries.
      Conclusion: the drift risk is mitigated by an existing test, and the proposed remedy trades it for a worse cross-contamination risk. Revisit only if the two registries ever diverge in practice, which that test would catch.

## P3 — Product depth

- [x] Optional disk-backed research cache under `.nerd/`.  
      `research/cache_disk.go`: JSON entries under
      `<workspace>/.nerd/cache/research`, hash-named so a caller-supplied key
      can never become a path segment, best-effort on every disk error.
      **Wiring pending**: call `research.EnableDiskCache(workspaceRoot)` at boot.
- [x] Assert `tool_execution` facts from Registry.Execute for learning.  
      `Registry.SetFactSink` fires once per completed execution, and never for a
      refused one. **Wiring pending**: `internal/core` must install a sink that
      asserts `tool_execution(ToolName, Success, Timestamp)`.
- [x] Improve codedom EndLine via simple brace/indent block tracking.  
      Already implemented (`findBraceEndLine`, `findPythonEndLine`) and covered
      by `TestExtractCodeElements_BlockExtent*`. Audited; no change needed.
- [x] Metrics counters for tool success/duration.  
      `Registry.Metrics(name)` and `Registry.AllMetrics()`.

## Done / not TODO

- Modular registry + RegisterAll hydrate — **done**.  
- Workspace guard for core file ops — **done**.  
- shellquote for run_command — **done**.  
- Grounding/Thinking helpers — **done** as libraries.
