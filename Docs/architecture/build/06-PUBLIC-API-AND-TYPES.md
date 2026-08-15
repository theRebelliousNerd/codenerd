# 06 — Public API and Types: `internal/build`

> Last verified: **2026-08-15**  
> Import: `"codenerd/internal/build"`

---

## 1. Exported type

### `BuildConfig`

**File:** `internal/build/env.go`  
**Kind:** type alias — `type BuildConfig = config.BuildConfig`

| Field | Type | JSON | Meaning |
|-------|------|------|---------|
| `EnvVars` | `map[string]string` | `env_vars` | Extra KEY→value for build env |
| `GoFlags` | `[]string` | `go_flags` | Go argv flags, applied by `AppendGoFlags` |
| `CGOPackages` | `[]string` | `cgo_packages` | Packages needing CGO (metadata; auto may append `sqlite-vec`) |

**Note:** the struct is declared once, in `internal/config/build.go`, and persisted
on `UserConfig.Build`. `build.BuildConfig` is an alias for it, not a mirror — the
two cannot drift.

---

## 2. Exported functions

### `DefaultBuildConfig() *BuildConfig`

Returns non-nil config with empty (non-nil) `EnvVars` map and empty slices.

### `GetBuildEnv(userCfg *config.UserConfig, workspaceRoot string) []string`

**Primary API.** Builds filtered env for `go build` / general toolchain use.

| Param | Nullable | Role |
|-------|----------|------|
| `userCfg` | yes | Whitelist + `Build` block |
| `workspaceRoot` | path string | Header detection root |

**Returns:** `[]string` suitable for `exec.Cmd.Env`. Never nil empty-only in normal cases (may still be short if process env is barren).

### `GetBuildEnvForTest(userCfg *config.UserConfig, workspaceRoot string) []string`

`GetBuildEnv` plus test-only specialization, none of which overwrites a value the
caller or config already set:

| Addition | Why |
|----------|-----|
| `GOTRACEBACK=all` | A panic in a background goroutine otherwise prints only that goroutine, and the verification parsers cannot attribute the failure |
| `-count=1` folded into `GOFLAGS` | The toolchain replays a cached PASS for a package whose source was just rewritten — green for code that never ran |
| `CI`, `GORACE`, `GOMAXPROCS`, `GOTMPDIR` propagated | Suites branch on `CI`; `GORACE` configures the race detector; the base build filter drops all four |

`-count` is left alone when the caller already chose one (e.g. `-count=5` for a
benchmark sweep). Unknown flags in `GOFLAGS` are ignored by subcommands that do
not define them, so the slice stays safe if reused for a non-test command.

### `GetBuildEnvForModule(userCfg *config.UserConfig, moduleDir string) []string`

`GetBuildEnv(userCfg, DetectionRootFor(moduleDir))`. Use when `cmd.Dir` is a
nested module and the headers live at the repo root.

### `DetectionRootFor(moduleDir string) string`

Walks up from `moduleDir` to the first directory containing `sqlite_headers`, or
to the repository boundary (`.git` / `go.work`), whichever comes first. Never
escapes above the repo, so a stray `/include` cannot become a detection root.
Returns `moduleDir` when no marker is found anywhere up the chain.

### `AppendGoFlags(userCfg *config.UserConfig, workspaceRoot string, args []string) []string`

Injects configured `build.go_flags` into a `go` argv (`args` is everything after
the `go` binary). Flags go immediately after the subcommand, because package
patterns must stay last. A flag whose name already appears in `args` is skipped,
so an explicit argv always beats config. Subcommands that take a sub-verb
(`go mod tidy`, `go tool …`) are returned untouched.

### `SummarizeEnv(env []string) string`

Sorted, comma-joined, **keys only**. Values are never rendered: build envs carry
whatever the operator whitelisted, which in practice includes API keys.

### `GetBuildEnvForCompile(userCfg *config.UserConfig, workspaceRoot string, targetOS, targetArch string) []string`

Calls `GetBuildEnv`, then:

- if `targetOS != ""` → set `GOOS`
- if `targetArch != ""` → set `GOARCH`

Uses `setEnvKey` (overwrite-safe for those keys).

### `MergeEnv(base []string, additional ...string) []string`

| Behavior | Detail |
|----------|--------|
| Copy | Does not mutate `base` |
| Parse | `SplitN(add, "=", 2)`; skip if not 2 parts |
| Override | Later additional wins via `setEnvKey` |

---

## 3. Unexported helpers (maintainers)

| Func | Role |
|------|------|
| `getBaseGoEnv()` | Essential Go/OS vars |
| `deriveGOCACHE()` | Platform cache path |
| `loadBuildConfig(userCfg, root)` | Config + sqlite_headers |
| `detectCGOFlags(root)` | Multi include-dir scan |
| `hasEnvKey(env, key)` | Prefix check |
| `setEnvKey(env, key, val)` | Update or append |

These are covered extensively by tests even though unexported (same package tests).

---

## 4. Dependencies in signatures

```go
import "codenerd/internal/config"
```

Callers that already hold `*config.UserConfig` pass it through. Callers without config pass `nil`.

---

## 5. Stability expectations

| API | Stability |
|-----|-----------|
| `GetBuildEnv` | Stable; extend carefully |
| `GetBuildEnvForCompile` | Stable |
| `MergeEnv` | Stable |
| `GetBuildEnvForTest` | Stable; specialization implemented 2026-08-15 |
| `GetBuildEnvForModule` / `DetectionRootFor` | Stable |
| `AppendGoFlags` / `SummarizeEnv` | Stable |
| `BuildConfig.GoFlags` | Stable; consumed by `AppendGoFlags` |
| Unexported helpers | Free to refactor if tests updated |

---

## 6. Usage examples (accurate to code)

### Sandboxed tool compile (matches Ouroboros)

```go
cmd := exec.CommandContext(ctx, "go", "build", "-ldflags", ldflags, "-o", out, ".")
cmd.Dir = tmpDir
cmd.Env = build.MergeEnv(
    build.GetBuildEnvForCompile(nil, tmpDir, targetOS, targetArch),
    "CGO_ENABLED=0",
)
```

### Preferred workspace-aware test run (recommended pattern)

```go
args := build.AppendGoFlags(userCfg, workspaceRoot, []string{"test", "-count=1", "./..."})
cmd := exec.CommandContext(ctx, "go", args...)
cmd.Dir = workspaceRoot
cmd.Env = build.GetBuildEnvForTest(userCfg, workspaceRoot)
```

### Compiling inside a nested module of a monorepo

```go
cmd := exec.CommandContext(ctx, "go", "build", "./...")
cmd.Dir = moduleDir                                  // where the build runs
cmd.Env = build.GetBuildEnvForModule(userCfg, moduleDir)  // headers resolved from the repo root
```

### Overlay test flags without forking API

```go
env := build.GetBuildEnvForTest(userCfg, workspaceRoot)
env = build.MergeEnv(env, "GOFLAGS=-count=1")
```

---

## 7. Non-APIs (do not invent)

- No `RegisterBuildProvider`
- No VirtualStore action `build_env`
- No Mangle `Decl` surface
- No CLI subcommand owned by this package
