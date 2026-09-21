# build — wiring and what is NOT built

Verified 2026-09-20 against commit `456e521` (`main`). Measured by grepping
`codenerd/internal/build` and each exported helper name across the repo this
turn; both greps returned complete (untruncated) sets.

## Wired and reachable

Five packages import this package in production:

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

## Exists but nothing calls

Three exported helpers have no production callers. Grepping each bare
name across the repo returns only the definition in
`internal/build/env.go` and the package's own tests
(`internal/build/env_features_test.go`,
`internal/build/env_gaps_test.go`):

- `GetBuildEnvForTest(userCfg, workspaceRoot)` (`env.go:241-261`) — the
  test-env specialization. Its own doc comment (`env.go:222-240`) states
  the three reasons it exists: GOTRACEBACK=all, `-count=1` folded into
  GOFLAGS, and CI/GORACE/GOMAXPROCS/GOTMPDIR propagation. The one site
  that should use it (`internal/tools/codedom/run_impacted_tests.go`) is
  still on the exemption list below instead.
- `AppendGoFlags(userCfg, workspaceRoot, args)` (`env.go:300-338`) —
  called only from `env_features_test.go` (:239-282, :450).
- `GetBuildEnvForModule(userCfg, moduleDir)` (`env.go:175-177`) — called
  only from `env_features_test.go` (:104). Its helper
  `DetectionRootFor(moduleDir)` (`env.go:187-209`) runs in production only
  via `GetBuildEnvForModule` itself (`env.go:176`); every other caller is
  a test (`env_features_test.go:98-132`).

## Exempt by reason, not by wiring

`go_invocation_inventory_test.go:32-52` exempts four files from routing
`go` spawns through this package:

- `cmd/nerd/dom_cmd.go`, `dom_apply_cmd.go`, `dom_replace_cmd.go` —
  operator-invoked CLI verification inherits the operator's ambient shell
  environment; narrowing to the filtered build env would drop vars the
  operator deliberately set.
- `internal/autopoiesis/tool_compiler.go` — only for its `go mod tidy`
  step, which needs the ambient module-resolution credentials the build
  env filter deliberately drops (GOPROXY auth, .netrc, GOPRIVATE). The
  compile and test steps in the same file do use this package.
- `internal/tools/codedom/run_impacted_tests.go` — pending adoption: it
  should take the session's UserConfig and route through
  `GetBuildEnvForTest`. Until then the most purpose-built helper in the
  package has no production callers (see above).

## Assumed by design, not enforced by code

- Comments say every `go` spawn "should use GetBuildEnv"
  (`env.go:7-8`), but the only enforcement is the inventory test, which
  covers non-test files (`go_invocation_inventory_test.go:26-28`) — test
  fixtures spawning `go` are outside the mandate.
- A stale exemption fails the inventory test just like a missing one
  (`go_invocation_inventory_test.go:129-133`), so the exemption list
  cannot rot silently; but nothing forces the pending
  `run_impacted_tests.go` adoption to ever happen.
- `MergeEnv` (`env.go:601-613`) silently drops malformed entries without
  a `=` separator (`env.go:605-610`); there is no writer-side validation
  and no log when an entry is dropped.
