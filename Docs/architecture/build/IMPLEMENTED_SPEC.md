# internal/build — Implemented Spec (Deep-Dive)

> Last verified against codebase: **2026-08-15**  
> Status: Living Reference Document  
> Language: Go  
> Primary source: `internal/build/env.go`  
> Tests: `internal/build/env_test.go`, `internal/build/env_gaps_test.go`, `internal/build/env_features_test.go`, `internal/build/go_invocation_inventory_test.go`  
> Scale: **1** non-test Go file ≈ **611** lines; **4** test files ≈ **1500** lines; **0** `.mg`

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
| `GetBuildEnvForTest` differentiation | **Implemented** | `GOTRACEBACK=all`, `-count=1` folded into `GOFLAGS`, `CI`/`GORACE`/`GOMAXPROCS`/`GOTMPDIR` propagation |
| Apply `BuildConfig.GoFlags` to argv | **Implemented** | `AppendGoFlags` inserts after the subcommand; explicit argv wins; sub-verb commands (`go mod tidy`) untouched |
| Detection root separate from module dir | **Implemented** | `DetectionRootFor` / `GetBuildEnvForModule`, bounded by `.git` / `go.work` |
| Duplicate-key prevention across merge stages | **Implemented** | every stage uses `setEnvKey`; config keys iterated sorted for determinism |
| Secret redaction in debug logs | **Implemented** | `redactEnvValue` + keys-only `SummarizeEnv`; values still reach the subprocess |
| `BuildWarn` when GOCACHE underivable | **Implemented** | `buildWarn` seam in `getBaseGoEnv` |
| Repo-wide `go` invocation inventory | **Implemented as a test** | `go_invocation_inventory_test.go` AST scan; unmarked or stale entries fail |
| Use of `CGOPackages` at runtime | **Doc-only (by design)** | list mutated for detection; no consumer reads it from build API |
| Pass real `UserConfig` from callers | **Partial** | only `session.verifyBuild`; autopoiesis and three session sites pass `nil` |
| Adoption by preflight / shell / tactile / attack paths | **Resolved** | no `preflight` package; session verification routed; shell/tactile exempt with documented reasons |
| Dual `BuildConfig` types (`build` vs `config`) | **Collapsed** | `build.BuildConfig = config.BuildConfig` (alias) |

**Overall:** production-ready **env factory**, and the “single source of truth”
mandate is now **enforced by test** rather than asserted in a comment. The
remaining gap is call-site quality: `nil` user configs and temp-dir detection
roots in `internal/autopoiesis`.

---

## 3. Source inventory

### 3.1 Package layout

```
internal/build/
  env.go                            # all production code (~611 lines)
  env_test.go                       # core unit tests (~211 lines)
  env_gaps_test.go                  # gap / edge coverage (~427 lines)
  env_features_test.go              # dedup/determinism, test specialization, GoFlags,
                                    #   detection root, redaction, toolchain integration (~460 lines)
  go_invocation_inventory_test.go   # repo-wide `go` invocation + importer audit (~402 lines)
```

No `README.md`, no `agents.md`, no YAML, no Mangle under this package root.

### 3.2 File roles

| Path | Lines (approx) | Role |
|------|---------------:|------|
| `internal/build/env.go` | 611 | Entire production surface |
| `internal/build/env_test.go` | 211 | `deriveGOCACHE`, key helpers, `detectCGOFlags`, `loadBuildConfig` sqlite cases |
| `internal/build/env_gaps_test.go` | 427 | Public API paths, nil config, whitelist, cross-compile, MergeEnv edges |
| `internal/build/env_features_test.go` | 460 | Key normalization, ordering determinism, `GetBuildEnvForTest` specialization, `AppendGoFlags`, `DetectionRootFor`, redaction, `go env` / `go test` integration |
| `internal/build/go_invocation_inventory_test.go` | 402 | AST audit of every non-test `exec.Command("go", …)` and of the importer set |

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
| `internal/session/build_verify.go` | `GetBuildEnv(userCfg, workspace)` — the one production site passing real config |
| `internal/session/test_verify.go` | `GetBuildEnv(nil, workspace)` for `go test` |
| `internal/session/coverage_profile.go` | `GetBuildEnv(nil, workspace)` for `go test -coverprofile` |
| `internal/session/lsp_diagnostics.go` | `GetBuildEnv(nil, workspace)` for `gopls check` |
| `internal/core/virtual_store_actions.go` | `buildToolEnv` unions `GetBuildEnv` with the execution allowlist |

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

### 5.1 One type named `BuildConfig` (collapsed 2026-08-15)

```go
// internal/build/env.go
type BuildConfig = config.BuildConfig
```

There used to be two structs with identical fields — one per package — and
`loadBuildConfig` copied field-by-field between them. They drifted (only the
config one ever carried yaml tags) and every schema change had to be made twice.
The persisted shape in `internal/config/build.go` is now the single definition;
`internal/build` aliases it, so `build.BuildConfig` and `config.BuildConfig` are
the same type and cannot diverge. `build.DefaultBuildConfig()` wraps
`config.DefaultBuildConfig()` and returns a pointer, preserving its old signature.

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
| `BuildConfig` | type alias | `= config.BuildConfig`, the persisted shape |
| `DefaultBuildConfig` | func | Empty maps/slices, non-nil |
| `GetBuildEnv` | func | Primary env factory |
| `GetBuildEnvForTest` | func | Build env + `GOTRACEBACK=all`, `-count=1`, `CI`/`GORACE`/`GOMAXPROCS`/`GOTMPDIR` |
| `GetBuildEnvForCompile` | func | Build + GOOS/GOARCH |
| `GetBuildEnvForModule` | func | Build env for a command whose `cmd.Dir` is a nested module |
| `DetectionRootFor` | func | Resolve the header-detection root from a module dir |
| `AppendGoFlags` | func | Inject configured `build.go_flags` into a `go` argv |
| `SummarizeEnv` | func | Sorted, keys-only rendering for logs and diffs |
| `MergeEnv` | func | Overlay KEY=value strings |

Unexported but central: `getBaseGoEnv`, `deriveGOCACHE`, `loadBuildConfig`, `detectCGOFlags`, `hasEnvKey`, `setEnvKey`, `envValue`, `redactEnvValue`, `isRepoBoundary`, `withCountOne`.

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
| Integration with real `go` toolchain + CGO | **Strong** — `TestGetBuildEnv_WhenRealWorkspace_ShouldSurviveGoEnv` runs `go env` against the constructed env; `TestGetBuildEnvForTest_WhenRealWorkspace_ShouldCompileAndRunATest` compiles and runs a throwaway module |
| Duplicate keys / ordering determinism | **Strong** — `env_features_test.go` |
| Repo-wide adoption of the env factory | **Strong** — `go_invocation_inventory_test.go` (AST, fails on new unmarked sites and on stale exemptions) |
| Caller contract (autopoiesis nil config) | **Absent** — still not asserted anywhere |

Commands:

```
go test ./internal/build/...
go test -v ./internal/build/ -run 'TestGoInvocations|TestBuildImporters'   # prints the inventory
go test -short ./internal/build/...                                        # skips toolchain integration
```

---

## 11. Gaps pointer

Authoritative gap matrix: [03-GAP-ANALYSIS.md](03-GAP-ANALYSIS.md).

Highest-signal gaps:

1. **Production callers pass `nil` config** (all but `session.verifyBuild`) → sample `UserConfig.Build` unused on those paths.  
2. **`workspaceRoot` is arena/tmp** in autopoiesis → monorepo `sqlite_headers` auto-detect skipped (mitigated by `CGO_ENABLED=0`). `GetBuildEnvForModule` exists; adoption is a call-site change.  
3. **`CGOPackages` not consumed** at runtime — descriptive only, by design.  
4. `internal/tools/codedom/run_impacted_tests.go` still spawns `go test` with no `cmd.Env` (recorded `pending adoption`).

Closed since 2026-07-13: adoption inventory (now a test), `GoFlags` (now
`AppendGoFlags`), `GetBuildEnvForTest` specialization, duplicate env keys, dual
`BuildConfig` types, secret-prone debug logging, GOCACHE warning.

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
| No duplicate keys in any returned env | `setEnvKey` at every stage + `TestGetBuildEnv_When*_ShouldNotDuplicateKey` |
| Env slice order is deterministic | sorted config keys + `TestGetBuildEnv_WhenCalledTwice_ShouldBeDeterministic` |
| Debug logs never print secret-prone values | `redactEnvValue` / `SummarizeEnv` + `TestRedactEnvValue_WhenSecretProneKey_ShouldRedact` |
| Redaction never alters the subprocess env | `TestGetBuildEnv_WhenSecretInConfigEnvVars_ShouldStillBePassedToSubprocess` |
| Detection-root walk never escapes the repo | `.git` / `go.work` boundary + `TestDetectionRootFor_*` |
| `GetBuildEnvForTest` never overwrites an explicit caller value | `TestGetBuildEnvForTest_WhenCallerPinnedCount_ShouldNotOverride` |
| Every non-test `go` spawn is routed or exempted | `TestGoInvocations_WhenSpawningGo_ShouldUseBuildEnvOrBeExempt` |
| Package-comment importer list matches reality | `TestBuildImporters_WhenNewConsumerAppears_ShouldBeDocumented` |

---

## 13. What “done” looks like for this package

A finished “unified build env” mandate would mean:

1. ~~Every internal `exec.Command("go", …)` that needs project CGO uses `GetBuildEnv*`~~ — **done for the routing half**, enforced by test. The **real workspace root** and **non-nil user config** half is still open at the autopoiesis call sites.  
2. ~~Documented non-adopters (shell, tactile) explicitly out of scope~~ — **done**; see `Docs/architecture/tools/09-SAFETY-AND-INVARIANTS.md` and `Docs/architecture/tactile/09-SAFETY-AND-INVARIANTS.md`.  
3. ~~Either apply `GoFlags` at a higher layer or drop the dead field~~ — **done** (`AppendGoFlags`).  
4. ~~Collapse the dual `BuildConfig` types~~ — **done** (alias).  
5. ~~Normalize env keys before return~~ — **done** (`setEnvKey` at every stage).

`internal/build` is now a **correct, well-tested library with an enforced
integration surface**. The residual work is caller-side quality, tracked in
[TODO.md](TODO.md) under “Still open”.

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
