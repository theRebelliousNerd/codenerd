# build — Architecture Corpus (`internal/build`)

> Last verified against codebase: **2026-08-15**  
> Status: Living Reference Document  
> Language: Go (module `codenerd`)  
> Primary package: `internal/build/`  
> Scale: **1** non-test Go file (~611 lines); **4** test files (~1500 lines); **0** Mangle sources

## Scope

This corpus documents **`internal/build`**: the unified **Go toolchain environment builder**. It constructs filtered `[]string` environment slices for subprocess `go build` / `go test` / `go test -c` invocations so CGO headers (`sqlite_headers`), `GOCACHE`, whitelist-propagated vars, and optional cross-compile (`GOOS`/`GOARCH`) stay consistent.

It is **not**:

- A general build system (no Make/Ninja, no artifact graph)
- The tactile execution sandbox (`internal/tactile`)
- Shell tool env assembly (`internal/tools/shell`)
- The Mangle kernel or policy surface
- Product CLI “build” UX

## Why this package exists

Package comment in `internal/build/env.go` states the problem: multiple components historically spawned `exec.Command("go", …)` with raw or incomplete env, missing **`CGO_CFLAGS`** for `sqlite_headers` and related flags. `GetBuildEnv` is intended as the **single source of truth** for those subprocess environments.

**Today’s wiring reality:** three packages import it — **`internal/autopoiesis`**
(Ouroboros tool compiler, Thunderdome arena), **`internal/session`** (build /
test / coverage / gopls verification) and **`internal/core`** (VirtualStore
`run_tests` and `build` actions). Every other `exec.Command("go", …)` in the repo
carries a written exemption. Both facts are enforced by
`internal/build/go_invocation_inventory_test.go`, not by this paragraph.

Remaining honesty gap: all call sites except `session.verifyBuild` still pass
**`userCfg=nil`**, so config-driven `build` / `execution.allowed_env_vars` paths
are tested but under-exercised in production. See
[08-WIRING-AND-INTEGRATION.md](08-WIRING-AND-INTEGRATION.md).

## Document map

| Doc | Role |
|-----|------|
| [IMPLEMENTED_SPEC.md](IMPLEMENTED_SPEC.md) | Authoritative living architecture + inventory |
| [00-ALIGNMENT-VISION-REVIEW.md](00-ALIGNMENT-VISION-REVIEW.md) | North-star alignment scores |
| [01-VISION.md](01-VISION.md) | Target vision for build-env unification |
| [02-CURRENT-STATE.md](02-CURRENT-STATE.md) | On-disk inventory, line roles, hotspots |
| [03-GAP-ANALYSIS.md](03-GAP-ANALYSIS.md) | Spec vs reality, priorities, non-gaps |
| [04-ARCHITECTURAL-PRINCIPLES.md](04-ARCHITECTURAL-PRINCIPLES.md) | Binding principles for this package |
| [05-INTERNAL-ARCHITECTURE.md](05-INTERNAL-ARCHITECTURE.md) | Merge pipeline, helpers, data flow |
| [06-PUBLIC-API-AND-TYPES.md](06-PUBLIC-API-AND-TYPES.md) | Exported types and functions |
| [07-DEPENDENCY-MAP.md](07-DEPENDENCY-MAP.md) | Upstream/downstream with evidence |
| [08-WIRING-AND-INTEGRATION.md](08-WIRING-AND-INTEGRATION.md) | Who calls it; who still bypasses it |
| [09-SAFETY-AND-INVARIANTS.md](09-SAFETY-AND-INVARIANTS.md) | Env filtering, CGO, concurrency |
| [10-TESTING-ALIGNMENT.md](10-TESTING-ALIGNMENT.md) | Tests, coverage shape, commands |
| [11-OBSERVABILITY.md](11-OBSERVABILITY.md) | `logging.CategoryBuild` / `BuildDebug` |
| [12-FAILURE-MODES.md](12-FAILURE-MODES.md) | Concrete failures + mitigations |
| [TODO.md](TODO.md) / [OPEN-QUESTIONS.md](OPEN-QUESTIONS.md) / [_progress.md](_progress.md) | Governance |

### Superseded filenames (this rebuild)

Older auto-corpus names in this directory (`01-DOMAIN-MODEL.md`, `02-CURRENT-STATE-BUILD.md`, `03-GAP-ANALYSIS-BUILD.md`, `04-INVARIANTS-AND-GATES.md`, `05-CROSS-SYSTEM-WIRING.md`, `06-TESTING-STRATEGY.md`, `08-FAILURE-MODES.md` as a thin stub) are **replaced** by the map above. Prefer the links in this README.

## Verify commands

```powershell
# Unit tests for this package only
go test ./internal/build/...

# With race detector
go test -race ./internal/build/...

# Print the live repo-wide `go` invocation + importer inventory
go test -v ./internal/build/ -run 'TestGoInvocations|TestBuildImporters'

# Skip the tests that spawn the real toolchain
go test -short ./internal/build/...

# Manual: observe auto-detect when workspace has sqlite_headers/
# (codeNERD root includes sqlite_headers/sqlite3.h)
$env:CGO_CFLAGS = "-IC:/CodeProjects/codeNERD/sqlite_headers"
go build -o nerd.exe ./cmd/nerd
```

Default sample config embeds explicit build env under `UserConfig.Build` (see `internal/config/user_config.go` sample around the `Build: &BuildConfig{ EnvVars: CGO_CFLAGS… }` block).

## Quality bar

Modeled on `Docs/architecture/cli/`: real paths, control-flow diagrams, honest adoption gaps — **not** inventory stubs that swap the package name and still read true.

## Fact-flow placement

```
user_intent → kernel → next_action → VirtualStore → actuators
                                              │
                                              └─ autopoiesis compile paths
                                                   └─ internal/build.GetBuildEnv*
                                                        → exec.Command env for `go`
```

`internal/build` is **below** the executive loop: pure env construction for toolchain subprocesses. No Mangle predicates, no prompt atoms, no VirtualStore routes.
