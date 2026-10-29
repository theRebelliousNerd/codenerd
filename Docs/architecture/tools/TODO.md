# tools — TODO

> Last verified: **2026-09-25**  
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
      09-SAFETY-AND-INVARIANTS.md. Session wiring **declined by the owner of
      the call site** (2026-09-25 re-check): per-turn `Global().SetAllowlist`
      makes one shard's envelope every concurrent shard's envelope; the
      reasoning and the test leak it caused are recorded at
      `internal/session/executor_tools.go` ("Why the registry allowlist is NOT
      set from here"). The envelope is installed by registry owners
      (`cmd/tools/change_benchmark/main.go`); `isToolAllowed` gates the session
      path.

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
      `CODENERD_WORKSPACE_ROOT` demoted to a fallback. Wired:
      `internal/system/factory.go` calls `tools.SetGlobalWorkspaceRoot(abs)`
      at boot (verified 2026-09-25).
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
      That is not available as written: internal/tools/registry.go declares `var globalRegistry = NewRegistry()` as a package-level singleton reached through `Global()` with no production setter for the registry itself (`SwapGlobal` is a test seam). Aliasing would require introducing one.
      Introducing one would be worse than the problem. Every VirtualStore in the process would then share one mutable registry, so tests that construct more than one VirtualStore would contaminate each other, and a tool registered by one workspace would be visible to another. The current duplication is bounded and deterministic, and TestCatalog_WhenHydratedTwice_ShouldProduceIdenticalRegistries already pins the property that makes it safe - that hydrating twice produces identical registries.
      Conclusion: the drift risk is mitigated by an existing test, and the proposed remedy trades it for a worse cross-contamination risk. Revisit only if the two registries ever diverge in practice, which that test would catch.

## P3 — Product depth

- [x] Optional disk-backed research cache under `.nerd/`.  
      `research/cache_disk.go`: JSON entries under
      `<workspace>/.nerd/cache/research`, hash-named so a caller-supplied key
      can never become a path segment, best-effort on every disk error.
      Wired: `internal/system/factory.go` calls `research.EnableDiskCache(abs)`
      at boot (verified 2026-09-25).
- [x] Assert `tool_execution` facts from Registry.Execute for learning.  
      `Registry.SetFactSink` fires once per completed execution, and never for a
      refused one. Wired: `VirtualStore.installToolFactSink`
      (`internal/core/virtual_store_tool_facts.go`) installs it on both the
      VirtualStore registry and `tools.Global()` (verified 2026-09-25).
- [x] Improve codedom EndLine via simple brace/indent block tracking.  
      Already implemented (`findBraceEndLine`, `findPythonEndLine`) and covered
      by `TestExtractCodeElements_BlockExtent*`. Audited; no change needed.
- [x] Metrics counters for tool success/duration.  
      `Registry.Metrics(name)` and `Registry.AllMetrics()`.

## Dead-code inventory — 2026-09-25 (lane B wave 3)

Every `internal/tools` entry of `scripts/testdata/deadcode-baseline.txt`
(codedom excluded, it is another lane's):

- `registry.go` package-level `Register`, `MustRegisterGlobal`, `Get`,
  `Execute`, `SetGlobalAllowlist`, `SetGlobalWriteGuard`,
  `SetGlobalFactSink`: **removed**. Each was a second spelling of a
  `Global()` method; production uses `Global().X` everywhere, tests now do
  too.
- `SwapGlobal`: **keep** — test seam (swap the process registry for one test;
  used by package and e2e tests, invisible to RTA from `main`).
- `research.SetBrowserManager`: **removed**, superseded by
  `SetBrowserRuntime(mgr, kernel)`, which keeps the browser manager and its
  reasoning kernel paired.
- `research.EnableDiskCacheFromContext`, `RegisterGroundedWebSearch` (alias of
  `RegisterGroundedWebSearchIfSupported`), `FormatSourcesMarkdown`,
  `SearchResultsToJSON`, `ResearchCache.Delete`: **removed**, no caller and no
  consumer the vision names.
- `research.GetDocURLsForTechs`: **wired** — the strategic knowledge pass
  (`internal/init/strategic_knowledge.go` `strategicDocURLs`) builds its
  URL-context set through it, so a technology named as both language and
  framework no longer sends its documentation twice
  (`TestStrategicDocURLs_ShouldSendEachDocumentationURLOnce`).

## Done / not TODO

- Modular registry + RegisterAll hydrate — **done**.  
- Workspace guard for core file ops — **done**.  
- shellquote for run_command — **done**.  
- Grounding/Thinking helpers — **done** as libraries.
