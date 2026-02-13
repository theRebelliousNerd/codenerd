# internal/build - Unified Build Environment

This package provides unified build environment configuration for all components that run go build/test commands.

**Related Packages:**
- [internal/config](../config/CLAUDE.md) - UserConfig with execution settings
- [internal/shards/nemesis](../shards/nemesis/CLAUDE.md) - Attack compilation using build env
- [internal/autopoiesis](../autopoiesis/CLAUDE.md) - Ouroboros tool compilation

## Architecture

Addresses the wiring issue where multiple components (preflight, thunderdome, ouroboros, attack_runner, tester) bypass the tactile layer and use raw `exec.Command` without proper environment variables like `CGO_CFLAGS`.

**All components that run go build/test should use GetBuildEnv()** to ensure consistent environment configuration.

## File Index

| File | Description |
|------|-------------|
| `env.go` | Unified build environment configuration merging process env, whitelisted vars, and project config. Exports `BuildConfig` (EnvVars/GoFlags/CGOPackages), `GetBuildEnv()` returning properly configured []string environment, and `detectCGOFlags()` for auto-detection of sqlite_headers. |
| `env_test.go` | Unit tests for GetBuildEnv environment merging. Tests whitelisted env var filtering and CGO auto-detection. |

## Key Types

### BuildConfig
```go
type BuildConfig struct {
    EnvVars     map[string]string // CGO_CFLAGS, CGO_LDFLAGS, etc.
    GoFlags     []string          // Additional go build flags
    CGOPackages []string          // Packages requiring CGO
}
```

## Environment Merge Order

1. Current process environment (filtered)
2. Whitelisted env vars from config (AllowedEnvVars)
3. Project-specific build config from .nerd/config.json
4. Auto-detected CGO flags (sqlite_headers detection)

## Usage

```go
env := build.GetBuildEnv(userCfg, workspaceRoot)
cmd := exec.Command("go", "build", "./...")
cmd.Env = env
```

## Dependencies

- `internal/config` - UserConfig for execution settings
- `internal/logging` - Build category logging

## Testing

```bash
go test ./internal/build/...
```

---

**Remember: Push to GitHub regularly!**


> *[Archived & Reviewed by The Librarian on 2026-01-25]*