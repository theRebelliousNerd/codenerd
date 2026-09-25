# build — wiring and what is NOT built

Verified 2026-09-20 against commit `456e521` (`main`); the runner wiring
re-verified 2026-09-25 on the lane B wave 3 branch. Measured by grepping
`codenerd/internal/build` and each exported helper name across the repo this
turn; both greps returned complete (untruncated) sets.

## Wired and reachable

Seven packages import this package in production (the two
`internal/tools` runners are described below):

- `internal/session` — nine files: `build_verify.go`, `change_gates.go`,
  `importer_packages.go`, `lsp_diagnostics.go`, `pin_existing.go`,
  `pin_gate.go`, `tag_gated_packages.go`, `test_baseline.go`,
  `test_verify.go`. All call `GetBuildEnv`; none calls the test/compile
  specializations.
- `internal/autopoiesis` — `thunderdome.go` and `tool_compiler.go` call
  `GetBuildEnv`/`GetBuildEnvForCompile` (plus
  `build_env_threading_test.go` in tests).
- `internal/core` — `virtual_store_actions.go` calls `GetBuildEnv`.
- `internal/system` — `factory_execution.go` calls `GetBuildEnv`.
- `internal/campaign` — `checkpoint.go` and
  `orchestrator_task_handlers.go`. These are the only two production
  callers of `TestTagsForWorkspace` (`checkpoint.go:124`,
  `orchestrator_task_handlers.go:877`).

`DefaultBuildConfig` (:46-49) and `SummarizeEnv` (:355-368) look public
but are package-internal working helpers: their only production callers
are `loadBuildConfig` (`internal/build/env.go:488`) and the package's own
debug logging (`env.go:163`, `env.go:498`).

## Model-facing runners (added 2026-09-25)

`GoInvocation` (`internal/build/invocation.go`) is the entry point for a `go`
command codeNERD runs on a workspace's behalf. It loads
`<workspace>/.nerd/config.json` (`WorkspaceUserConfig`), bounds the header
walk (`DetectionRootFor`, `isRepoBoundary`) at the workspace root, picks
`GetBuildEnvForTest` for `go test` and `GetBuildEnv` otherwise, and injects
the configured `build.go_flags` (`AppendGoFlags`). Its callers:

- `internal/tools/shell/verification.go` — the typed `run_build` /
  `run_tests` tools. They spawn through a variable binary (`argv[0]`), which
  is why the inventory test never saw them spawning `go` with the process
  environment.
- `internal/tools/codedom/run_impacted_tests.go` — `runGoTests`.

`GetBuildEnvForModule` was deleted: `GoInvocation` computes the same
detection root and, unlike it, never adopts headers above the workspace.

## Exempt by reason, not by wiring

`go_invocation_inventory_test.go` exempts four files from routing `go`
spawns through this package:

- `cmd/nerd/dom_cmd.go`, `dom_apply_cmd.go`, `dom_replace_cmd.go` —
  operator-invoked CLI verification inherits the operator's ambient shell
  environment; narrowing to the filtered build env would drop vars the
  operator deliberately set.
- `internal/autopoiesis/tool_compiler.go` — only for its `go mod tidy`
  step, which needs the ambient module-resolution credentials the build
  env filter deliberately drops (GOPROXY auth, .netrc, GOPRIVATE). The
  compile and test steps in the same file do use this package.

The scanner follows one assignment: `env, argv := build.GoInvocation(...)`
then `cmd.Env = env` counts as the build env.

## Assumed by design, not enforced by code

- Comments say every `go` spawn "should use GetBuildEnv"
  (`env.go:7-8`), but the only enforcement is the inventory test, which
  covers non-test files (`go_invocation_inventory_test.go:26-28`) — test
  fixtures spawning `go` are outside the mandate.
- A stale exemption fails the inventory test just like a missing one
  (`go_invocation_inventory_test.go:129-133`), so the exemption list
  cannot rot silently.
- `MergeEnv` (`env.go:601-613`) silently drops malformed entries without
  a `=` separator (`env.go:605-610`); there is no writer-side validation
  and no log when an entry is dropped.
