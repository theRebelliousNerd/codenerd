# `internal/build` — TODO

Verified 2026-09-20 against commit `456e521` (`main`). Items are ordered by
priority. Each item cites the code it refers to; a closed item is removed,
not struck through, so this file never accumulates history.

## P1 — Route the impacted-test runner through the build env

`internal/tools/codedom/run_impacted_tests.go` spawns `go test` in the
project root with no `cmd.Env`, so a project needing CGO headers fails there
with a compile error reported as a test failure. It should call
`GetBuildEnvForTest` (`internal/build/env.go:241-261`) and build its argv
with `AppendGoFlags` (`internal/build/env.go:300-338`).

Tracked by the pending-adoption entry in
`internal/build/go_invocation_inventory_test.go:50-51` and by
`Docs/architecture/tools/09-SAFETY-AND-INVARIANTS.md:137-142`. See
`WIRING-AND-NOT-BUILT.md` ("exists but nothing calls") for the measurement.

## P2 — Decide the fate of the uncalled helpers

`GetBuildEnvForModule` (`internal/build/env.go:175-177`),
`DetectionRootFor` / `isRepoBoundary` (`internal/build/env.go:187-220`),
and `GetBuildEnvForCompile` (`internal/build/env.go:277-289`) have no
callers (see `WIRING-AND-NOT-BUILT.md`). Either wire a call site to each or
delete it. Leaving them is the status quo, not a plan.

## P3 — Keep the tactile containment rule visible

Tactile deliberately does not inherit the monorepo `CGO_CFLAGS`
(`Docs/architecture/tactile/09-SAFETY-AND-INVARIANTS.md:135-137`). Any new
tactile-spawned `go` command that needs cgo must pass the flags explicitly
through the tactile request. If `internal/build` ever gains a
tactile-facing entry point, it must preserve that caller-passes-env shape
instead of unioning `os.Environ()` from below.

## P4 — Done 2026-09-20

The 18-file corpus was replaced by `README.md`, `INTERNALS.md`,
`WIRING-AND-NOT-BUILT.md`, and this file. Verified claims were folded into
those three; unverifiable claims were dropped, not carried forward. Details
in the rewrite report, not here.
