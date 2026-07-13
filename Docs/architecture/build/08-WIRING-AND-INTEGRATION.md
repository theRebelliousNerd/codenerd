# 08 — Wiring and Integration: `internal/build`

> Last verified: **2026-07-13**

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
| preflight | No import |
| attack_runner | No import |
| tester (shard type) | No import; prompt/shard concept only |
| shell execute | Uses `os.Environ()` |
| tactile direct/docker/platform | Own env builders |
| campaign assault go tests | Operator/docs set CGO manually |
| cmd/nerd build helper | Human uses shell recipe |

---

## 8. How to wire a new consumer (checklist)

1. Import `codenerd/internal/build`.  
2. Prefer `GetBuildEnv(userCfg, workspaceRoot)` with real workspace root.  
3. Set `cmd.Dir` separately (module / package path).  
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
