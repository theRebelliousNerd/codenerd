# internal/build — Implemented Spec (Deep-Dive)

> Last verified against codebase: **2026-07-13**  
> Status: Living Reference Document  
> Language: Go  
> Primary source: `internal/build/env.go`  
> Tests: `internal/build/env_test.go`, `internal/build/env_gaps_test.go`  
> Scale: **1** non-test Go file ≈ **312** lines; **2** test files ≈ **638** lines; **0** `.mg`

---

## 1. Overview

`internal/build` is a **small, intentional single-purpose package**: assemble the environment for Go toolchain subprocesses that codeNERD itself launches.

It exists because codeNERD is not a pure-Go binary in the abstract. Production features (sqlite-vec embeddings path, CGO-linked sqlite) require:

| Requirement | Why it matters |
|-------------|----------------|
| `CGO_CFLAGS=-I…/sqlite_headers` | Headers live at workspace `sqlite_headers/` (see root `Agents.md` build recipe) |
| `GOFLAGS=-tags=sqlite_vec` (when headers present) | Enable sqlite-vec build tag for internal builds |
| `GOCACHE` always defined | Subprocess `go` fails hard if cache path is missing |
| Filtered env (not full `os.Environ()`) | Avoid leaking secrets / unbounded process env into sandboxed compiles |
| Optional `GOOS` / `GOARCH` | Ouroboros tool binaries may target another OS/arch |
| Whitelist pass-through | `UserConfig.Execution.AllowedEnvVars` can admit extra process vars |

### Key characteristics

| Property | Value |
|----------|-------|
| Package name | `build` |
| Import path | `codenerd/internal/build` |
| Entry API | `GetBuildEnv`, `GetBuildEnvForTest`, `GetBuildEnvForCompile`, `MergeEnv` |
| Config input | `*config.UserConfig` (nullable) + `workspaceRoot string` |
| Output shape | `[]string` of `KEY=value` entries (ready for `exec.Cmd.Env`) |
| Side effects | Filesystem `Stat` for header dirs; `logging.BuildDebug` |
| Mangle / shards | None |
| Live importers | `internal/autopoiesis` only (2 call sites) |

### Design slogan (from package comment)

> All components that run `go build`/`go test` should use `GetBuildEnv()` to ensure consistent environment configuration across the codebase.

That is the **intent**. Section 8 documents **adoption reality**.

---

## 2. Implementation status

| Component | Status | Evidence |
|-----------|--------|----------|
| Base Go env (`PATH`, `GOPATH`, `GOROOT`, caches, home, temp) | **Implemented** | `getBaseGoEnv` |
| `GOCACHE` derivation when unset | **Implemented** | `deriveGOCACHE` precedence chain |
| Config `Build.EnvVars` merge | **Implemented** | `loadBuildConfig` + `GetBuildEnv` |
| Execution whitelist env | **Implemented** | `userCfg.GetExecution().AllowedEnvVars` |
| `sqlite_headers` auto CGO | **Implemented** | `loadBuildConfig` + `detectCGOFlags` fallback |
| Default `-tags=sqlite_vec` when headers present | **Implemented** | `loadBuildConfig` sets `GOFLAGS` if unset |
| Cross-compile `GOOS`/`GOARCH` | **Implemented** | `GetBuildEnvForCompile` + `setEnvKey` |
| `MergeEnv` override helper | **Implemented** | used by autopoiesis to force `CGO_ENABLED=0` |
| `GetBuildEnvForTest` differentiation | **Stub / no-op extra** | race/CI branch is empty; returns same as build env |
| Apply `BuildConfig.GoFlags` to argv | **Not implemented** | flags stored, never appended to commands (package only builds env) |
| Use of `CGOPackages` at runtime | **Doc-only** | list mutated for detection; no consumer reads it from build API |
| Pass real `UserConfig` from callers | **Partial / unused in prod** | autopoiesis passes `nil` |
| Adoption by preflight / shell / tactile / attack paths | **Not adopted** | package comment aspirational; only autopoiesis imports |
| Dual `BuildConfig` types (`build` vs `config`) | **Living duplication** | parallel structs, config is the persisted shape |

**Overall:** production-ready **env factory** with solid unit tests; **incomplete “single source of truth” mandate** relative to the package comment and config comments.

---

## 3. Source inventory

### 3.1 Package layout

```
internal/build/
  env.go              # all production code (~312 lines)
  env_test.go         # core unit tests (~211 lines)
  env_gaps_test.go    # gap / edge coverage (~427 lines)
```

No `README.md`, no `agents.md`, no YAML, no Mangle under this package root.

### 3.2 File roles

| Path | Lines (approx) | Role |
|------|---------------:|------|
| `internal/build/env.go` | 312 | Entire production surface |
| `internal/build/env_test.go` | 211 | `deriveGOCACHE`, key helpers, `detectCGOFlags`, `loadBuildConfig` sqlite cases |
| `internal/build/env_gaps_test.go` | 427 | Public API paths, nil config, whitelist, cross-compile, MergeEnv edges |

### 3.3 Related files outside the package (not owned, but load-bearing)

| Path | Relationship |
|------|----------------|
| `internal/config/build.go` | Persisted `config.BuildConfig` (JSON/YAML under `.nerd` user config) |
| `internal/config/user_config.go` | `UserConfig.Build *BuildConfig`; sample defaults with `CGO_CFLAGS` |
| `internal/config/execution.go` | `AllowedEnvVars` whitelist consumed by `GetBuildEnv` |
| `internal/logging/logger.go` | `CategoryBuild = "build"` |
| `internal/logging/logger_convenience.go` | `BuildDebug` / `Build` / `BuildWarn` / `BuildError` |
| `sqlite_headers/sqlite3.h` | On-disk header tree auto-detected from workspace root |
| `internal/autopoiesis/tool_compiler.go` | `GetBuildEnvForCompile` + `MergeEnv(..., "CGO_ENABLED=0")` |
| `internal/autopoiesis/thunderdome.go` | `GetBuildEnv` + `MergeEnv(..., "CGO_ENABLED=0")` |

---

## 4. Deep dive: environment merge pipeline

### 4.1 `GetBuildEnv` control flow

```
GetBuildEnv(userCfg, workspaceRoot)
  │
  ├─1─ getBaseGoEnv()
  │      PATH (if set)
  │      essentialVars if set: GOPATH GOROOT GOCACHE GOMODCACHE GOFLAGS
  │                           HOME USERPROFILE LOCALAPPDATA TEMP TMP TMPDIR
  │      if GOCACHE still missing → deriveGOCACHE()
  │
  ├─2─ if userCfg != nil
  │      for key in userCfg.GetExecution().AllowedEnvVars
  │         if os.Getenv(key) != "" → append KEY=val
  │      (does not de-dupe against base; later setEnvKey only used elsewhere)
  │
  ├─3─ loadBuildConfig(userCfg, workspaceRoot)
  │      DefaultBuildConfig()
  │      copy userCfg.Build.EnvVars / GoFlags / CGOPackages if present
  │      if <workspace>/sqlite_headers exists:
  │         set CGO_CFLAGS=-I<abs> unless already in cfg.EnvVars
  │         set GOFLAGS=-tags=sqlite_vec unless cfg or process GOFLAGS set
  │         append "sqlite-vec" to CGOPackages if missing
  │      for each cfg.EnvVars → append to env slice
  │
  ├─4─ if env still lacks CGO_CFLAGS
  │      detectCGOFlags(workspaceRoot) scans:
  │         sqlite_headers, include, vendor/include, third_party/include
  │      append CGO_CFLAGS=joined -I paths
  │
  └─5─ return []string
```

Important ordering notes:

1. **Base env first**, then whitelist, then project build config. Later appends can **duplicate keys** if the same key appears in base and config (e.g. process `GOFLAGS` in base and also config `GOFLAGS`). Go’s `os/exec` behavior uses the **last** occurrence of a key in the env slice — but this package does not normalize duplicates in `GetBuildEnv` (only `setEnvKey` / `MergeEnv` do).
2. **`detectCGOFlags` is a second chance** for `CGO_CFLAGS` only; it does not re-apply sqlite-vec `GOFLAGS` if `loadBuildConfig` skipped it.
3. **`workspaceRoot` is path-sensitive.** Auto-detect uses `filepath.Abs` when relative. Callers that pass a temp compile dir (Ouroboros `tmpDir`, Thunderdome `arenaDir`) will **not** see the monorepo’s `sqlite_headers` unless they pass the real workspace root — and current autopoiesis callers pass the **arena/tmp dir**, so auto-detect usually sees **no headers**. They force `CGO_ENABLED=0`, so that is intentional for sandboxed tool builds.

### 4.2 `GetBuildEnvForTest`

```go
func GetBuildEnvForTest(userCfg *config.UserConfig, workspaceRoot string) []string {
	env := GetBuildEnv(userCfg, workspaceRoot)
	// Empty race-detector / CI branch (comment only)
	return env
}
```

Documented intent: test-specific settings. **Implemented behavior:** identity wrapper over `GetBuildEnv`. Tests assert length ≥ build env (currently equal).

### 4.3 `GetBuildEnvForCompile`

```
GetBuildEnvForCompile(userCfg, workspaceRoot, targetOS, targetArch)
  → GetBuildEnv(...)
  → setEnvKey(GOOS) if targetOS != ""
  → setEnvKey(GOARCH) if targetArch != ""
```

Used by Ouroboros `ToolCompiler.Compile` with targets from `OuroborosConfig` / tool generation config. Empty strings leave host GOOS/GOARCH unset (inherit absence → host defaults via Go toolchain).

### 4.4 `MergeEnv`

Pure helper: copy base, then for each `KEY=value` additional entry, `setEnvKey` (update or append). Malformed entries without `=` are skipped. Does not mutate the base slice (tests assert this).

Production use:

```go
// tool_compiler.go
cmd.Env = build.MergeEnv(
  build.GetBuildEnvForCompile(nil, tmpDir, tc.config.TargetOS, tc.config.TargetArch),
  "CGO_ENABLED=0",
)

// thunderdome.go
cmd.Env = build.MergeEnv(build.GetBuildEnv(nil, arenaDir), "CGO_ENABLED=0")
```

---

## 5. Deep dive: `BuildConfig` and sqlite-vec detection

### 5.1 Two types named `BuildConfig`

| Type | Package | Persistence | Used by |
|------|---------|-------------|---------|
| `build.BuildConfig` | `internal/build` | In-memory only | Returned by `DefaultBuildConfig` / `loadBuildConfig` |
| `config.BuildConfig` | `internal/config` | JSON `UserConfig.Build` | User/workspace config load |

Fields are isomorphic: `EnvVars`, `GoFlags`, `CGOPackages`. `loadBuildConfig` **copies** from `config.BuildConfig` into `build.BuildConfig`.

### 5.2 `sqlite_headers` special case

When `filepath.Join(absRoot, "sqlite_headers")` exists:

1. `CGO_CFLAGS=-I` + absolute headers path (if not already set in cfg.EnvVars)
2. `GOFLAGS=-tags=sqlite_vec` if neither cfg.EnvVars nor process `GOFLAGS` set
3. Ensure `"sqlite-vec"` ∈ `CGOPackages`

This matches root agent guidance:

```powershell
$env:CGO_CFLAGS = "-IC:/CodeProjects/codeNERD/sqlite_headers"
go build -o nerd.exe ./cmd/nerd
```

…but **automates** it for subprocesses that receive a workspace root containing that directory.

### 5.3 Header directory scan (`detectCGOFlags`)

Ordered list (all matching dirs become space-joined `-I` flags):

1. `sqlite_headers`
2. `include`
3. `vendor/include`
4. `third_party/include`

Only directories (not files) count. Empty → `""`.

---

## 6. Deep dive: `GOCACHE` derivation

`deriveGOCACHE` preference order (first non-empty base wins):

| Priority | Env var | Derived path |
|---------:|---------|--------------|
| 1 | `LOCALAPPDATA` | `$LOCALAPPDATA/go-build` |
| 2 | `USERPROFILE` | `$USERPROFILE/.cache/go-build` |
| 3 | `HOME` | `$HOME/.cache/go-build` |
| 4 | `TEMP` | `$TEMP/go-build` |
| 5 | `TMP` | `$TMP/go-build` |
| 6 | `TMPDIR` | `$TMPDIR/go-build` |
| 7 | (none) | `""` — leave unset; Go may error |

Unit tests in `env_test.go` pin this order with `t.Setenv` isolation.

---

## 7. Public API summary

| Symbol | Kind | Purpose |
|--------|------|---------|
| `BuildConfig` | type | In-package config struct |
| `DefaultBuildConfig` | func | Empty maps/slices, non-nil |
| `GetBuildEnv` | func | Primary env factory |
| `GetBuildEnvForTest` | func | Test env (currently = build) |
| `GetBuildEnvForCompile` | func | Build + GOOS/GOARCH |
| `MergeEnv` | func | Overlay KEY=value strings |

Unexported but central: `getBaseGoEnv`, `deriveGOCACHE`, `loadBuildConfig`, `detectCGOFlags`, `hasEnvKey`, `setEnvKey`.

Full signatures and file anchors: [06-PUBLIC-API-AND-TYPES.md](06-PUBLIC-API-AND-TYPES.md).

---

## 8. Integration map

### 8.1 Live reverse dependencies

```
codenerd/internal/build
        ▲
        │ import
        │
internal/autopoiesis
  ├─ tool_compiler.go  → GetBuildEnvForCompile(nil, tmpDir, GOOS, GOARCH)
  │                      MergeEnv(..., "CGO_ENABLED=0")
  └─ thunderdome.go    → GetBuildEnv(nil, arenaDir)
                         MergeEnv(..., "CGO_ENABLED=0")
```

No imports from `cmd/nerd`, `internal/core`, `internal/session`, `internal/tactile`, `internal/tools`, `internal/testing` (if any), etc.

### 8.2 Upstream dependencies

```
internal/build
  → codenerd/internal/config   (UserConfig, Execution whitelist, Build block)
  → codenerd/internal/logging  (BuildDebug)
  → stdlib: maps, os, path/filepath, slices, strings
```

### 8.3 What still bypasses this package

| Area | Env strategy | Notes |
|------|--------------|-------|
| `internal/tools/shell/execute.go` | `os.Environ()` + filters | Agent shell runs; not Go-build specialized |
| `internal/tactile/*` | `buildEnvironment` helpers | Sandbox policy, not CGO detection |
| Manual operator builds | Shell `CGO_CFLAGS` | Documented in root `Agents.md` |
| Package-comment “preflight / attack_runner / tester” | N/A | Names are aspirational; no matching importers |

### 8.4 Relation to constitutional / OODA fact flow

`internal/build` does **not** participate in:

- Perception → `user_intent`
- Kernel `next_action` / `permitted(...)`
- Prompt atom JIT
- Shard registration

It is **effect plumbing** for autopoiesis actuators that already decided to compile. Safety for *whether* to compile belongs upstream; safety for *what env the compiler sees* is this package’s narrow job.

---

## 9. Observability

All instrumentation goes through `logging.BuildDebug` (`CategoryBuild`). Messages include:

- Workspace root being built for
- Whitelisted keys added
- Build-config keys added
- Auto-detected `CGO_CFLAGS`
- Final var count
- Derived `GOCACHE`
- Loaded user BuildConfig
- Detected `sqlite_headers` path

No metrics counters, no structured trace spans, no glass-box events. See [11-OBSERVABILITY.md](11-OBSERVABILITY.md).

---

## 10. Testing posture

| Area | Coverage quality |
|------|------------------|
| `deriveGOCACHE` precedence | Strong (temp dirs + clear env) |
| `hasEnvKey` / `setEnvKey` / `MergeEnv` | Strong including partial-prefix false positives |
| `detectCGOFlags` multi-dir order | Strong |
| `loadBuildConfig` sqlite + overrides | Strong |
| `GetBuildEnv` nil config / whitelist / headers | Strong |
| `GetBuildEnvForCompile` GOOS/GOARCH | Strong |
| Integration with real `go build` + CGO | **Absent** (unit only; no toolchain integration test) |
| Caller contract (autopoiesis nil config) | **Absent** |

Commands: `go test ./internal/build/...`

---

## 11. Gaps pointer

Authoritative gap matrix: [03-GAP-ANALYSIS.md](03-GAP-ANALYSIS.md).

Highest-signal gaps:

1. **Adoption incomplete** vs package comment mandate.  
2. **Production callers pass `nil` config** → sample `UserConfig.Build` unused on these paths.  
3. **`workspaceRoot` is arena/tmp** in autopoiesis → monorepo `sqlite_headers` auto-detect usually skipped (mitigated by `CGO_ENABLED=0`).  
4. **`GoFlags` / `CGOPackages` not consumed** by env builders for argv or package selection.  
5. **`GetBuildEnvForTest` is a no-op specialization**.  
6. **Duplicate env keys possible** in `GetBuildEnv` merge stages (no normalize).  
7. **Dual `BuildConfig` types** risk field drift.

---

## 12. Invariants (code-enforced or tested)

| Invariant | How held |
|-----------|----------|
| `DefaultBuildConfig` never nil maps | Constructor + tests |
| Malformed MergeEnv entries skipped | `SplitN` length check + test |
| MergeEnv does not mutate base | copy + test |
| `hasEnvKey` requires `KEY=` prefix | avoids `FOO` matching `FOOBAR` |
| User CGO_CFLAGS overrides auto | empty-check before sqlite inject |
| Process GOFLAGS blocks default sqlite_vec tag | `os.Getenv("GOFLAGS")` check |
| Cross-compile empty targets leave GOOS/GOARCH unset | tests |

---

## 13. What “done” looks like for this package

A finished “unified build env” mandate would mean:

1. Every internal `exec.Command("go", "build"| "test"…)` that needs project CGO uses `GetBuildEnv*` with **real workspace root** and **non-nil user config when available**.  
2. Documented non-adopters (shell, tactile) explicitly out of scope.  
3. Either apply `GoFlags` at a higher layer or drop the dead field.  
4. Collapse or generate-from-one the dual `BuildConfig` types.  
5. Optional: normalize env keys (last-wins rewrite) before return.

Until then, treat `internal/build` as a **correct, well-tested library** whose **integration surface is narrow**.

---

## 14. Quick reference for implementers

```go
import (
  "codenerd/internal/build"
  "codenerd/internal/config"
)

// Preferred for building codeNERD itself or workspace packages:
env := build.GetBuildEnv(userCfg, workspaceRoot)
cmd := exec.Command("go", "build", "-o", out, "./cmd/nerd")
cmd.Env = env
cmd.Dir = workspaceRoot

// Portable tool binary (Ouroboros style):
env = build.MergeEnv(
  build.GetBuildEnvForCompile(userCfg, workspaceRoot, "linux", "amd64"),
  "CGO_ENABLED=0",
)
```

Do **not** invent VirtualStore action names or Mangle predicates for this package — it has none.

---

## 15. Document maintenance

| When | Update |
|------|--------|
| New public API on `env.go` | IMPLEMENTED_SPEC §7, 06-PUBLIC-API, tests doc |
| New importer | §8, 07-DEPENDENCY-MAP, 08-WIRING |
| Adoption of real UserConfig in autopoiesis | close gap #2 in 03-GAP-ANALYSIS |
| New header auto-detect dirs | §5.3 + tests |

Last full rebuild: **2026-07-13**.
