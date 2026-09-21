# build

Verified 2026-09-20 against commit `456e521` (`main`).

`internal/build` answers one question for every Go-toolchain subprocess in this
repo: what environment — and, for tests, what extra flags — should the `go`
command inherit? Two production files, six test files, three docs (this one
included).

## Files

- `internal/build/env.go` — environment builders and merge helpers. Exported:
  `DefaultBuildConfig` (:46-49), `GetBuildEnv` (:126-165),
  `GetBuildEnvForModule` (:175-177), `DetectionRootFor` (:187-209),
  `GetBuildEnvForTest` (:241-261), `GetBuildEnvForCompile` (:277-289),
  `AppendGoFlags` (:300-338), `SummarizeEnv` (:355-368), `MergeEnv` (:601-613).
- `internal/build/tags.go` — `TestTagsForWorkspace` (:18-23) returns
  `-tags sqlite_vec` for codeNERD workspaces and nil elsewhere; detection lives
  in `isCodeNERDWorkspace` (:25-42).
- Tests in `internal/build/`: `env_test.go`, `env_features_test.go`,
  `env_gaps_test.go`, `go_invocation_inventory_test.go`, `gofmt_tree_test.go`,
  `tags_test.go`.

## Docs

- `INTERNALS.md` — what the builders and helpers do, and what verifies them.
- `WIRING-AND-NOT-BUILT.md` — who imports the package, which `go` spawns are
  exempt from using it, and what the design assumes that the code does not do.

## Consumers

Five packages import `codenerd/internal/build` (verified by grep this turn):
`internal/autopoiesis`, `internal/session`, `internal/core`,
`internal/system`, `internal/campaign`. Full file list and the three exempt
`go` spawners are in `WIRING-AND-NOT-BUILT.md`.
