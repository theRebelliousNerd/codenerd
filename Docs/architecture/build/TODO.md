# TODO — `internal/build`

> Prioritized backlog. No time estimates.  
> Last updated: **2026-07-13**

---

## P0 — Honesty and correctness

- [ ] Align `env.go` package comment consumer list with real importers (or adopt the missing consumers).  
- [ ] Thread `*config.UserConfig` into autopoiesis `ToolCompiler` / `Thunderdome` compile paths when available.  
- [ ] Split **detection root** (workspace) from **module dir** (`cmd.Dir`) in call sites that need monorepo CGO.

## P1 — Adoption mandate

- [ ] Inventory all `exec.Command("go", …)` sites; mark each `uses internal/build` or `exempt: reason`.  
- [ ] Document exemption for `tools/shell` and `tactile` env builders in those packages’ architecture docs.  
- [ ] If preflight / verification spawns `go test` for the monorepo, route env through `GetBuildEnv`.

## P2 — API hygiene

- [ ] Implement real test specialization **or** delete `GetBuildEnvForTest`.  
- [ ] Either consume `GoFlags` (helper to extend argv) or remove field from build + config with migration note.  
- [ ] Collapse dual `BuildConfig` types (`build` vs `config`) to one.  
- [ ] Normalize env keys with `setEnvKey` across all merge stages to prevent duplicates.

## P3 — Hardening and ops

- [ ] `BuildWarn` when GOCACHE cannot be derived.  
- [ ] Optional keys-only `SummarizeEnv` for debug.  
- [ ] Integration test: construct env against real workspace with `sqlite_headers`, run `go env`.  
- [ ] Avoid logging secret-prone values at debug for config env (keys only, or redact).

## P4 — Docs (this corpus)

- [ ] Update [08-WIRING-AND-INTEGRATION.md](08-WIRING-AND-INTEGRATION.md) when new importers appear.  
- [ ] Refresh scores in [00-ALIGNMENT-VISION-REVIEW.md](00-ALIGNMENT-VISION-REVIEW.md) after adoption work.  
- [ ] Keep IMPLEMENTED_SPEC scale counts accurate after edits.

---

## Explicitly not TODO (non-goals)

- Turning this package into a full build system.  
- Adding Mangle rules for env construction.  
- Auto-installing Go toolchains or C compilers.  
- Replacing tactile sandbox policy.
