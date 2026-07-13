# 01 — Vision: Build Environment Unification

> Last verified: **2026-07-13**  
> Package: `internal/build`  
> Audience: maintainers of autopoiesis, verification, campaign, tactile, and any future “self-build” actuators

---

## 1. Product / architecture vision

codeNERD frequently **compiles and tests code as an effect**: Ouroboros-generated tools, Thunderdome attack arenas, future preflight gates, campaign assault artifacts, operator-facing `go test` orchestration.

Those effects must not invent private conventions for:

- Where C headers live (`sqlite_headers`)
- Whether sqlite-vec tags are on
- Whether `GOCACHE` exists in stripped subprocess envs
- How cross-compile targets are expressed
- Which process secrets leak into sandbox builds

**Vision:** `internal/build` is the **only** package that answers “what environment should this Go toolchain subprocess inherit?” for project-aware builds. Execution *policy* (allowlists, timeouts, containers) stays in `tactile` / tools / kernel; **env composition** for Go stays here.

---

## 2. Target behaviors

### 2.1 Single call pattern

```go
env := build.GetBuildEnv(userCfg, workspaceRoot)
// or ForTest / ForCompile variants
cmd.Env = build.MergeEnv(env, "CGO_ENABLED=0") // when sandbox requires
```

Every internal `go build` / `go test` / `go test -c` that cares about project CGO or tags uses this pattern.

### 2.2 Correct roots

Callers pass the **workspace root** (repo root containing `.nerd/` and optionally `sqlite_headers/`), not a throwaway module dir, when they need project CGO detection. Temporary module dirs remain `cmd.Dir`; env detection uses workspace.

### 2.3 Config as first-class

`.nerd` / user config `build.env_vars`, `build.go_flags`, and execution `allowed_env_vars` are honored when Cortex has loaded config. Autopoiesis and other actuators receive `*config.UserConfig` rather than hardcoding `nil`.

### 2.4 Explicit non-goals

| Non-goal | Owner instead |
|----------|----------------|
| Permission to run compile | Mangle `permitted`, VirtualStore policy |
| Process sandbox / firejail / docker | `internal/tactile` |
| Arbitrary shell command env | `internal/tools/shell` + tactile |
| Ninja/Make graphs | External tooling |
| LLM-facing “how to build” prose | Prompt atoms under `internal/prompt` if ever needed |
| Editing source to make it compile | Perception / coder shards |

### 2.5 Test specialization becomes real

`GetBuildEnvForTest` either:

- Applies documented test defaults (`-count=1` via `GOFLAGS` merge policy, timeout env, etc.), or  
- Is deleted in favor of `GetBuildEnv` + caller argv flags.

No empty dual API.

### 2.6 Field honesty

`GoFlags` and `CGOPackages` either:

- Drive something observable (argv helpers, diagnostics), or  
- Move entirely to config docs as metadata and drop from the build package struct.

---

## 3. Success criteria

| Criterion | Observable |
|-----------|------------|
| Adoption | `rg "codenerd/internal/build"` shows all Go-toolchain spawners that need project CGO |
| Config | At least one production path passes non-nil `UserConfig` with Build block |
| Headers | Building workspace packages with headers present auto-injects `CGO_CFLAGS` without manual shell |
| Sandbox | Tool compile paths still force `CGO_ENABLED=0` via `MergeEnv` where required |
| Docs | This corpus stays accurate; package comment matches importers |
| Tests | Unit suite green; optional one integration test that runs `go env` under constructed env |

---

## 4. Relationship to north star

Logic remains executive. Build env is **infrastructure fidelity**: when the executive permits a compile, the compile must not fail for “forgot CGO_CFLAGS” or “GOCACHE undefined” reasons that the platform already knows how to fix.

That reduces false negatives in autopoiesis / verification loops and keeps human operators and agents on the same build physics documented in root `Agents.md`.

---

## 5. Vision anti-patterns

- Re-implementing header detection inside thunderdome / campaign / preflight.  
- Passing full `os.Environ()` into sandboxed tool compiles “to make go work.”  
- Growing `env.go` into a generic process launcher.  
- Coupling env construction to Mangle rule evaluation.  
- Documenting 100% adoption while only autopoiesis imports the package.
