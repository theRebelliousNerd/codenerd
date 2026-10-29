# `internal/build` — TODO

Verified 2026-09-25 against the lane B wave 3 branch. Items are ordered by
priority. Each item cites the code it refers to; a closed item is removed,
not struck through, so this file never accumulates history.

## P3 — Keep the tactile containment rule visible

Tactile deliberately does not inherit the monorepo `CGO_CFLAGS`
(`Docs/architecture/tactile/09-SAFETY-AND-INVARIANTS.md:135-137`). Any new
tactile-spawned `go` command that needs cgo must pass the flags explicitly
through the tactile request. If `internal/build` ever gains a
tactile-facing entry point, it must preserve that caller-passes-env shape
instead of unioning `os.Environ()` from below.

This stays prose: it constrains a design that does not exist yet (a
tactile-facing entry point), and there is no code shape to test until one
does.

## Closed 2026-09-25

- **P1, route the impacted-test runner through the build env.**
  `internal/tools/codedom/run_impacted_tests.go` `runGoTests` now takes its
  env and argv from `build.GoInvocation` (`internal/build/invocation.go`), and
  its entry is gone from `goSpawnExemptions` in
  `internal/build/go_invocation_inventory_test.go`. The typed `run_build` /
  `run_tests` tools (`internal/tools/shell/verification.go`), which spawned
  `go` through a variable binary the inventory could not see, take the same
  path. Proof: `TestTypedVerification_GoTestRunsUnderTheBuildEnv` fails
  without the change (the parent's secret reached the test binary; the
  configured `go_flags` were not applied).
- **P2, the uncalled helpers.** `GetBuildEnvForTest`, `AppendGoFlags`,
  `DetectionRootFor` and `isRepoBoundary` are called by `GoInvocation`.
  `GetBuildEnvForModule` was deleted: `GoInvocation` computes the same
  detection root, bounded by the workspace root, which the helper was not
  (`TestGoInvocation_WhenHeadersSitAboveTheWorkspace_ShouldNotAdoptThem`).
  `GetBuildEnvForCompile` was already live (autopoiesis).
