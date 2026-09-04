# 08 — Wiring and Integration: `internal/build`

> Last verified: **2026-08-15**

---

## 0. The importer list is now a test, not a claim

`internal/build/go_invocation_inventory_test.go` owns this document's factual
core and fails when it drifts:

| Test | Invariant |
|------|-----------|
| `TestBuildImporters_WhenNewConsumerAppears_ShouldBeDocumented` | The set of packages importing `codenerd/internal/build` equals the set documented in `env.go`'s package comment and in §3–§5 below. Adding or dropping an importer fails until both are updated. |
| `TestGoInvocations_WhenSpawningGo_ShouldUseBuildEnvOrBeExempt` | Every non-test `exec.Command("go", …)` in the repo either assigns `cmd.Env` from a `build.*` call or is listed in `goSpawnExemptions` with a reason. Stale exemptions also fail. |

Run `go test -v ./internal/build/ -run 'TestGoInvocations|TestBuildImporters'`
to print the current inventory. Do not transcribe it here by hand.

---

## 1. Registration story

**None.** This package has:

- No `init()`
- No shard registration
- No VirtualStore route
- No CLI `AddCommand`
- No MCP tool registration
- No prompt atom

Wiring is **import + direct function call**.

---

## 2. Boot path relevance

Cortex boot (`cmd/nerd/chat/session_boot.go` and related) does **not** initialize `internal/build`. Config loading may populate `UserConfig.Build`, but nothing invokes `GetBuildEnv` at boot.

First use is demand-driven when autopoiesis compiles a tool or arena binary.

---

## 3. Live integration: Ouroboros tool compiler

**File:** `internal/autopoiesis/tool_compiler.go`

| Step | Detail |
|------|--------|
| When | `ToolCompiler.Compile` after writing sources to temp dir |
| Command | `go build -ldflags … -o <output> .` |
| Env | `MergeEnv(GetBuildEnvForCompile(nil, tmpDir, TargetOS, TargetArch), "CGO_ENABLED=0")` |
| Why CGO off | Portable tool binaries; avoid linking host CGO deps into generated tools |
| Config | **`nil`** — ignores user Build block |
| Detection root | **tmpDir** — no monorepo `sqlite_headers` |

---

## 4. Live integration: Thunderdome arena

**File:** `internal/autopoiesis/thunderdome.go`

| Step | Detail |
|------|--------|
| When | Compiling harness with `go test -c -o arena.test` |
| Env | `MergeEnv(GetBuildEnv(nil, arenaDir), "CGO_ENABLED=0")` |
| Config | **`nil`** |
| Detection root | **arenaDir** — isolated module, not workspace |

Thunderdome’s **tool execution** uses `toolExecutionEnv()` (separate path), not `GetBuildEnv`.

---

## 4a. Live integration: session verification

**Files:** `internal/session/build_verify.go`, `test_verify.go`, `coverage_profile.go`, `lsp_diagnostics.go`

| Site | Command | Env |
|------|---------|-----|
| `verifyBuild` | `go build ./...` | `GetBuildEnv(userCfg, workspace)` — the only production call passing a real `*config.UserConfig` |
| `verifyTests` | `go test …` | `GetBuildEnv(nil, workspace)` |
| `uncoveredWrittenCode` | `go test -coverprofile …` | `GetBuildEnv(nil, workspace)` |
| `goplsDiagnostics` | `gopls check …` | `GetBuildEnv(nil, workspace)` — not a `go` binary, but wants the same CGO flags |

This is the P1 “route preflight/verification through `GetBuildEnv`” item: session
verification is the surviving preflight surface (there is no `internal/preflight`
package) and it is already routed. The remaining gap is `nil` user config in
three of the four sites.

## 4b. Live integration: VirtualStore actions

**File:** `internal/core/virtual_store_actions.go` (`buildToolEnv`)

Unions `GetBuildEnv(nil, v.workingDir)` with the execution allowlist so the
`run_tests` / `build` actions can compile codeNERD itself. Before this, the
default `go test ./...` action failed with `fatal error: 'sqlite3.h' file not
found` and reported it as a test failure.

## 4c. Live integration: tactile executor base environment

**File:** `internal/system/factory_execution.go` (`executionLayerConfigs`)

Sets `tactile.ExecutorConfig.BaseEnvironment = GetBuildEnv(nil, workingDir)` at
boot, so every command the tactile `DirectExecutor` runs (campaign
checkpoints, shard build/test verification) sees `CGO_CFLAGS`/`GOFLAGS`/
`GOCACHE` regardless of the parent shell. Before this the executor passed
only allowlisted parent variables and the `/tests_pass` checkpoint reported
37 package build failures on `sqlite3.h` (campaign 5a2f4c8d, 2026-09-04).

---

## 5. Config wiring (latent)

```
.nerd / user config JSON
  └── "build": { "env_vars": {...}, "go_flags": [...], "cgo_packages": [...] }
  └── "execution": { "allowed_env_vars": [...] }

UserConfig loaded by config subsystem
  │
  ▼
GetBuildEnv(userCfg, root)   // only if callers pass userCfg
```

Sample defaults in `internal/config/user_config.go` include an explicit `CGO_CFLAGS` for the codeNERD monorepo path. That sample is **latent** for autopoiesis until callers stop passing `nil`.

---

## 6. Logging wiring

```
logging.CategoryBuild ("build")
  └── logging.BuildDebug used throughout env.go
```

Category defined in `internal/logging/logger.go`. Convenience wrappers: `Build`, `BuildDebug`, `BuildWarn`, `BuildError` in `logger_convenience.go`. Package uses **Debug** only.

---

## 7. Integration journal — what is *not* wired

| Claimed / expected consumer | Status |
|-----------------------------|--------|
| preflight | Package does not exist; session verification is the real surface (§4a) — **routed** |
| attack_runner | No import; spawns no `go` binary |
| tester (shard type) | No import; prompt/shard concept only |
| shell execute (`internal/tools/shell`) | Uses `os.Environ()` — **exempt**, see [../tools/](../tools/) |
| tactile direct/docker/platform | Own env builders — **exempt**, see [../tactile/](../tactile/) |
| `internal/tools/codedom/run_impacted_tests.go` | **Pending adoption** — spawns `go test` in the user project root with no `cmd.Env`; recorded in `goSpawnExemptions` |
| `cmd/nerd/dom_*.go` | **Exempt** — operator-invoked verification inherits the operator's shell environment |
| `internal/autopoiesis/tool_compiler.go` `go mod tidy` | **Exempt** — module resolution needs the ambient credentials the build filter drops |

The reason strings above live in `goSpawnExemptions`
(`internal/build/go_invocation_inventory_test.go`); this table mirrors them.

---

## 8. How to wire a new consumer (checklist)

1. Import `codenerd/internal/build`.  
2. Prefer `GetBuildEnv(userCfg, workspaceRoot)` with real workspace root.  
   If all you hold is the module directory, use `GetBuildEnvForModule(userCfg, moduleDir)`,
   which resolves the detection root via `DetectionRootFor` (walks up to the
   nearest `sqlite_headers`, bounded by the `.git` / `go.work` repo boundary).  
3. Set `cmd.Dir` separately (module / package path).  
3a. For `go test`, use `GetBuildEnvForTest` (adds `GOTRACEBACK=all`, folds
   `-count=1` into `GOFLAGS`, propagates `CI`/`GORACE`/`GOMAXPROCS`/`GOTMPDIR`).  
3b. Build argv with `AppendGoFlags(userCfg, root, args)` so configured
   `build.go_flags` reach the command.  
4. For sandboxed generated code, `MergeEnv(env, "CGO_ENABLED=0")`.  
5. For host project builds needing sqlite-vec, do **not** force CGO off; ensure headers present or config sets flags.  
6. Update this journal and [07-DEPENDENCY-MAP.md](07-DEPENDENCY-MAP.md).  
7. Do not reimplement header detection.

---

## 9. Fact-flow placement (again)

```
user_intent → … → next_action → VirtualStore
                    │
                    └─ (if action is autopoiesis compile)
                         ToolCompiler / Thunderdome
                              └─ build.GetBuildEnv*
                                   └─ go toolchain
```

No Mangle `Decl` required for env construction.
