# world — TODO

> Last verified: **2026-08-16**  
> Docs-only backlog derived from code gaps. Not a commitment schedule.

## P0

- [x] Unify path canonicalization across full scan, incremental scan, and deep cache keys.
      `internal/world/canonical_path.go` is now the single definition (`CanonicalPath` /
      `ResolveWorkspacePath`). The incremental scanner was handing absolute walk paths to the
      AST parsers and keying LocalStore rows by absolute path while the full scan keyed them
      canonically, so symbol facts named a file no `file_topology` row mentioned and the
      retraction lookup missed every row.
- [x] Expand or document `WorldPredicates` vs all emitters (`entry_point`, CodeDOM, git, scope).
      `internal/world/world_predicates.go` now states an ownership matrix
      (scanner / deep / LSP / session scope / git) and `ApplyIncrementalResult` replaces only
      the scanner-owned set.
- [x] Property/integration test: full vs incremental produce identical Path identities.
      `internal/world/canonical_path_test.go` (randomized workspaces, plus deep-scan identity).

## P1

- [x] Multi-lang Cartographer `MapFile` (or dedicated deep API) for py/ts/js/rs `code_defines`/`code_calls`.
      `internal/world/cartographer_multilang.go`; `MapFileAs(fsPath, factPath)` keeps the
      readable path and the fact identity separate.
- [x] Implement real `dependency_link` emission **or** remove from replace-set and marketing claims.
      Implemented: `internal/world/dependency_links.go` resolves in-workspace imports to
      file→file edges (Go package granularity, exact file for py/ts/js/rs). External imports keep
      their token form. Also fixed: the Go walker never saw grouped `import ( ... )` blocks.
- [x] Coordinate dual writers: chat incremental vs `WorldModelIngestorShard` (ownership matrix).
      Decision recorded in `world_predicates.go` and enforced at `ApplyIncrementalResult`;
      see OPEN-QUESTIONS Q1.

## P2

- [x] gopls (or generic LSP client) under `lsp.Manager` as sketched in `lsp/README.md`.
      `internal/world/lsp/client.go` — generic JSON-RPC/LSP client over any transport,
      `Manager.StartLanguageServer`/`AddLanguageServer`, diagnostics projected into
      `code_diagnostic` with canonical paths. Verified against an in-process fake server;
      no live `gopls` run in this environment.
- [x] Narrow holographic kernel dependency from `*core.RealKernel` to a small query interface.
      `world.FactQuerier` (single `Query` method), with typed-nil flattening so the existing
      graceful-degradation path cannot become a panic.
- Decided NO - Optional JIT prompt atoms for stable holographic sections. Requires atom files under `internal/prompt/atoms/`; not attempted from the world package. Investigated 2026-08-16:
  - The "stable sections" in question live in HolographicContext.FormatForPrompt (internal/world/holographic_formatting.go:99). Measured, the genuinely stable prose is about five lines: the "## Package Context" header, the "### Functions Available in Package Scope" header and its one explanatory sentence "These are defined in sibling files and can be called without import:", and the "### Types Defined in Package" header. Everything else in that function is per-file data.
  - Extracting those into JIT atoms would separate each label from the data it labels. Atoms are budgeted and selectable, so a tight budget could drop the header while the renderer still emits the signature or type block beneath it, producing an unlabelled wall of code in the prompt. The headers are structurally coupled to their payload and must travel with it.
  - The budget saving is negligible - roughly twenty tokens - so the change trades a real new failure mode for no measurable gain.
  - Note the revisit condition: this becomes worth doing only if the holographic renderer grows substantial instructional prose that is genuinely independent of the data it accompanies. It has not.
- [x] Structured observability: cache hit rate metrics for FileCache (not only DataFlowCache).
      `FileCache.Stats()/LogStats()`; both scanners log a hit-rate line. The manifest is now
      written atomically (unique temp + fsync + rename).
- [x] Ensure incremental path also refreshes `project_language` / `entry_point` when majority shifts.
      Delta scans recompute both from the current file set;
      `ApplyIncrementalResult` retracts the snapshot-global predicates before reloading.

## P3 / polish

- [x] Remove or relocate `debug_program_ERROR.mg` artifact from package tree if accidental.
      It was an untracked crash dump written by a `go test ./internal/core/...` run
      (`.nerd/debug/` is relative to the process cwd). Removed; the invariant that keeps it
      harmless — `.nerd` skipped at any depth — is now pinned by
      `internal/world/scan_nerd_artifacts_test.go`.
- [x] Align `symbol_graph` arg typing (string vs `/name` atoms) with Decl bounds.
      The JS/TS extractors wrote a bare `public`/`private` into a slot the Decl bounds `/name`.
      `internal/world/decl_conformance_test.go` now checks every emitted fact against the
      shipped Decl bounds. See "Known Decl drift" below.
- [x] Document operator runbook in CLI help for `nerd scan` / chat rescan.
      `world.ScanRunbook` (owned by the package that implements the behaviour), surfaced by
      `nerd world runbook` and `nerd world predicates`.

## Known Decl drift (found by the conformance test, owned elsewhere)

`schemas_reviewer.mg` bounds these slots `/string` while every emitter writes `/name` atoms,
and the consuming rules in `policy/data_flow.mg` were written against the emitters:

| predicate | slots | declared | emitted |
|-----------|-------|----------|---------|
| `uses` | Func, Var | `/string` | `/name` |
| `assigns` | Var | `/string` | `/name` |
| `guards_return`, `guards_block`, `safe_access` | Var | `/string` | `/name` |
| `function_scope`, `guard_dominates` | Func | `/string` | `/name` |
| `call_arg` | CallSite, VarRef | `/string` | `/name` |

Bounds are not enforced at load today, so the rules work; a rule written with a literal of the
declared type would silently match nothing. Correcting the Decls belongs to the owner of
`internal/core/defaults`. Pinned in `knownDeclDrift` in `decl_conformance_test.go`.
