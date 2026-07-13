# world — TODO

> Last verified: **2026-07-13**  
> Docs-only backlog derived from code gaps. Not a commitment schedule.

## P0

- [ ] Unify path canonicalization across full scan, incremental scan, and deep cache keys.
- [ ] Expand or document `WorldPredicates` vs all emitters (`entry_point`, CodeDOM, git, scope).
- [ ] Property/integration test: full vs incremental produce identical Path identities.

## P1

- [ ] Multi-lang Cartographer `MapFile` (or dedicated deep API) for py/ts/js/rs `code_defines`/`code_calls`.
- [ ] Implement real `dependency_link` emission **or** remove from replace-set and marketing claims.
- [ ] Coordinate dual writers: chat incremental vs `WorldModelIngestorShard` (ownership matrix).

## P2

- [ ] gopls (or generic LSP client) under `lsp.Manager` as sketched in `lsp/README.md`.
- [ ] Narrow holographic kernel dependency from `*core.RealKernel` to a small query interface.
- [ ] Optional JIT prompt atoms for stable holographic sections.
- [ ] Structured observability: cache hit rate metrics for FileCache (not only DataFlowCache).
- [ ] Ensure incremental path also refreshes `project_language` / `entry_point` when majority shifts.

## P3 / polish

- [ ] Remove or relocate `debug_program_ERROR.mg` artifact from package tree if accidental.
- [ ] Align `symbol_graph` arg typing (string vs `/name` atoms) with Decl bounds.
- [ ] Document operator runbook in CLI help for `nerd scan` / chat rescan.

## Done (observed in code — keep)

- [x] Nano mtime cache invalidation
- [x] Semaphore-before-spawn walk
- [x] Hidden dir allowlist (blind spot fix)
- [x] HolographicCodeScope deep wiring
- [x] Multi-lang CodeDOM factory
- [x] Multi-lang dataflow extractors
