# 06 — Public API and Types: `internal/build`

> Last verified: **2026-07-13**  
> Import: `"codenerd/internal/build"`

---

## 1. Exported type

### `BuildConfig`

**File:** `internal/build/env.go`  
**Kind:** struct (JSON tags for documentation parity with config)

| Field | Type | JSON | Meaning |
|-------|------|------|---------|
| `EnvVars` | `map[string]string` | `env_vars` | Extra KEY→value for build env |
| `GoFlags` | `[]string` | `go_flags` | Intended go argv flags (**not applied** by this package today) |
| `CGOPackages` | `[]string` | `cgo_packages` | Packages needing CGO (metadata; auto may append `sqlite-vec`) |

**Note:** Persistence for users is via **`config.BuildConfig`** on `UserConfig.Build` (`internal/config/build.go`). The build package type is the in-memory mirror used by `loadBuildConfig`.

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

Documented as test-oriented. **Implementation:** returns `GetBuildEnv(...)` unchanged. Placeholder comment about race detector intentionally not forcing `-race`.

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
| `GetBuildEnvForTest` | Unstable semantics until specialization exists |
| `BuildConfig.GoFlags` | Unstable / possibly removed or moved |
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

### Preferred workspace-aware build (recommended pattern)

```go
cmd := exec.Command("go", "test", "./...")
cmd.Dir = workspaceRoot
cmd.Env = build.GetBuildEnv(userCfg, workspaceRoot)
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
