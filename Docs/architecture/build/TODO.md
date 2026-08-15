# TODO — `internal/build`

> Prioritized backlog. No time estimates.  
> Last updated: **2026-08-15**

---

## P0 — Honesty and correctness

- [x] Align `env.go` package comment consumer list with real importers (or adopt the missing consumers).  
      Comment now names `internal/autopoiesis`, `internal/session`, `internal/core`;
      `TestBuildImporters_WhenNewConsumerAppears_ShouldBeDocumented` fails when that set changes.
- [ ] Thread `*config.UserConfig` into autopoiesis `ToolCompiler` / `Thunderdome` compile paths when available.  
      Call-site change inside `internal/autopoiesis`. `session.verifyBuild` already
      shows the shape (`GetBuildEnv(userCfg, workspace)`); the other session sites
      and both autopoiesis sites still pass `nil`.
- [x] Split **detection root** (workspace) from **module dir** (`cmd.Dir`) in call sites that need monorepo CGO.  
      `DetectionRootFor(moduleDir)` and `GetBuildEnvForModule` resolve the header
      root by walking up to the nearest `sqlite_headers`, bounded by the
      `.git` / `go.work` repo boundary. Adopting it in autopoiesis is a one-line
      change and remains open there.

## P1 — Adoption mandate

- [x] Inventory all `exec.Command("go", …)` sites; mark each `uses internal/build` or `exempt: reason`.  
      Encoded as `internal/build/go_invocation_inventory_test.go`: an AST scan of
      every non-test `.go` file in the repo. Each site must assign `cmd.Env` from
      a `build.*` call or appear in `goSpawnExemptions` with a reason. Stale
      exemptions fail too. `go test -v ./internal/build/ -run TestGoInvocations`
      prints the live inventory.
- [x] Document exemption for `tools/shell` and `tactile` env builders in those packages’ architecture docs.
- [x] If preflight / verification spawns `go test` for the monorepo, route env through `GetBuildEnv`.  
      There is no `internal/preflight`; `internal/session` verification is the real
      surface and all four sites already route through `GetBuildEnv`
      (see [08-WIRING-AND-INTEGRATION.md](08-WIRING-AND-INTEGRATION.md) §4a).
      Remaining: pass a real `*config.UserConfig` in three of the four.

## P2 — API hygiene

- [x] Implement real test specialization **or** delete `GetBuildEnvForTest`.  
      Implemented: `GOTRACEBACK=all`, `-count=1` folded into `GOFLAGS` (cached PASS
      results were reporting green for just-edited packages), and propagation of
      `CI` / `GORACE` / `GOMAXPROCS` / `GOTMPDIR`. Nothing overwrites an explicit
      caller value.
- [x] Either consume `GoFlags` (helper to extend argv) or remove field from build + config with migration note.  
      Consumed: `AppendGoFlags(userCfg, root, args)` inserts configured flags after
      the subcommand, skips flags already in argv, and refuses subcommands that
      take a sub-verb (`go mod tidy`).
- [x] Collapse dual `BuildConfig` types (`build` vs `config`) to one.  
      `build.BuildConfig` is now `= config.BuildConfig`. The persisted shape is the
      single definition; `DefaultBuildConfig` in `build` wraps the config one.
- [x] Normalize env keys with `setEnvKey` across all merge stages to prevent duplicates.  
      Whitelist, config env vars, CGO auto-detect and `getBaseGoEnv` all use
      `setEnvKey`; config keys are iterated in sorted order so the slice is
      deterministic. Covered by `TestGetBuildEnv_When*_ShouldNotDuplicateKey`.

## P3 — Hardening and ops

- [x] `BuildWarn` when GOCACHE cannot be derived.  
      Names the cause instead of letting the subprocess fail with an opaque
      "GOCACHE is not defined".
- [x] Optional keys-only `SummarizeEnv` for debug.
- [x] Integration test: construct env against real workspace with `sqlite_headers`, run `go env`.  
      `TestGetBuildEnv_WhenRealWorkspace_ShouldSurviveGoEnv` plus a companion that
      compiles and runs a throwaway test module with `GetBuildEnvForTest`.
- [x] Avoid logging secret-prone values at debug for config env (keys only, or redact).  
      `redactEnvValue` allowlists toolchain keys and redacts everything else;
      key-name fragments (TOKEN, SECRET, `_KEY`, AUTH, …) redact unconditionally.
      Redaction is a logging concern only — real values still reach the subprocess.

## P4 — Docs (this corpus)

- [x] Update [08-WIRING-AND-INTEGRATION.md](08-WIRING-AND-INTEGRATION.md) when new importers appear.
- [x] Refresh scores in [00-ALIGNMENT-VISION-REVIEW.md](00-ALIGNMENT-VISION-REVIEW.md) after adoption work.
- [x] Keep IMPLEMENTED_SPEC scale counts accurate after edits.

---

## Still open

- Thread real `*config.UserConfig` through autopoiesis and the three `nil`
  session call sites (P0 #2 above).
- Adopt `GetBuildEnvForModule` in autopoiesis so arena/tool compiles see monorepo
  headers when they live inside the workspace.
- `internal/tools/codedom/run_impacted_tests.go` should route through
  `GetBuildEnvForTest`; currently a recorded `pending adoption` exemption.

---

## Explicitly not TODO (non-goals)

- Turning this package into a full build system.  
- Adding Mangle rules for env construction.  
- Auto-installing Go toolchains or C compilers.  
- Replacing tactile sandbox policy.
