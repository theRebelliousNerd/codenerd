# 05 — Internal Architecture: `internal/build`

> Last verified: **2026-08-15**  
> Source of truth: `internal/build/env.go`

---

## 1. Component diagram

```
┌─────────────────────────────────────────────────────────────┐
│                     Callers (external)                       │
│  autopoiesis.ToolCompiler / Thunderdome  (+ future adopters) │
└────────────────────────────┬────────────────────────────────┘
                             │ GetBuildEnv* / MergeEnv
                             ▼
┌─────────────────────────────────────────────────────────────┐
│                    Public API surface                        │
│  GetBuildEnv | GetBuildEnvForTest | GetBuildEnvForCompile    │
│  GetBuildEnvForModule | DetectionRootFor | AppendGoFlags     │
│  SummarizeEnv | MergeEnv | DefaultBuildConfig | BuildConfig  │
└────────────────────────────┬────────────────────────────────┘
                             │
          ┌──────────────────┼──────────────────┐
          ▼                  ▼                  ▼
   getBaseGoEnv      loadBuildConfig     detectCGOFlags
          │                  │                  │
          ▼                  │                  │
   deriveGOCACHE             │                  │
          │                  │                  │
          └──────────┬───────┘──────────────────┘
                     ▼
              hasEnvKey / setEnvKey
                     │
                     ▼
              []string KEY=value
                     │
                     ▼
              logging.BuildDebug (CategoryBuild)
```

There is **no state machine**, **no long-lived object**, and **no package-level mutable cache**. Each call constructs a new slice.

---

## 2. Data structures

### 2.1 `BuildConfig` (alias for `config.BuildConfig`)

```text
BuildConfig = config.BuildConfig
  EnvVars     map[string]string   // injected as KEY=value (sorted, via setEnvKey)
  GoFlags     []string            // injected into argv by AppendGoFlags
  CGOPackages []string            // documentation / detection side list
```

Produced by `DefaultBuildConfig` / `loadBuildConfig`. Not returned from
`GetBuildEnv*` (only influences the env slice). The struct itself is declared in
`internal/config/build.go`; this package aliases it so the persisted and
in-memory shapes cannot drift.

### 2.2 Environment representation

Canonical form: `[]string` where each element is `KEY=value`.

Operations:

| Op | Semantics |
|----|-----------|
| `hasEnvKey(env, key)` | true if any entry has prefix `key+"="` |
| `setEnvKey(env, key, val)` | update first match or append |
| `MergeEnv(base, add...)` | copy base; setEnvKey each well-formed add |

---

## 3. Pipeline stages (detailed)

### Stage A — Base (`getBaseGoEnv`)

**Inputs:** process environment  
**Outputs:** minimal slice  

Always try `PATH`. Then essential list. Then ensure `GOCACHE`.

```
deriveGOCACHE priority:
  LOCALAPPDATA → …/go-build
  USERPROFILE  → …/.cache/go-build
  HOME         → …/.cache/go-build
  TEMP | TMP | TMPDIR → …/go-build
  else empty
```

### Stage B — Whitelist (`GetBuildEnv` when userCfg ≠ nil)

For each key in `userCfg.GetExecution().AllowedEnvVars`:

- If `os.Getenv(key) != ""`, **append** (does not use `setEnvKey` — duplicate risk if key already in base).

Defaults when Execution nil (via `GetExecution`): `PATH`, `HOME`, `GOPATH`, `GOROOT` (may re-append values already present from base).

### Stage C — Project build config (`loadBuildConfig`)

1. Start `DefaultBuildConfig()`.  
2. If `userCfg.Build != nil`, `maps.Copy` EnvVars; append GoFlags & CGOPackages.  
3. Absolutize workspace root.  
4. If `sqlite_headers` dir exists → inject CGO + maybe GOFLAGS + package name.  
5. Caller appends all `cfg.EnvVars` onto env (again via append, not setEnvKey).

### Stage D — Fallback CGO (`detectCGOFlags`)

Only if stage C/B left `CGO_CFLAGS` unset. Scans fixed relative dirs under workspace.

### Stage E — Specialization

| API | Extra stage |
|-----|-------------|
| `GetBuildEnvForTest` | none (today) |
| `GetBuildEnvForCompile` | `setEnvKey` GOOS/GOARCH when non-empty |
| Caller `MergeEnv` | arbitrary overlays |

---

## 4. Control flow ASCII (call site pattern)

```
Cortex / autopoiesis decides to compile tool
        │
        ▼
  write sources into tmp/arena (cmd.Dir)
        │
        ▼
  env := GetBuildEnv*(nil|cfg, root)
        │
        ▼
  env = MergeEnv(env, "CGO_ENABLED=0")   // sandbox tools
        │
        ▼
  exec.CommandContext(ctx, "go", "build"|"test", ...)
  cmd.Dir = tmp/arena
  cmd.Env = env
        │
        ▼
  CombinedOutput / Run → success or compiler stderr
```

---

## 5. Concurrency model

- Functions are **stateless** and safe for concurrent use as long as callers do not mutate returned slices concurrently.  
- `MergeEnv` copies the base; `setEnvKey` may mutate the slice it is given (in-place update of an element).  
- No mutexes. No goroutines.  
- Relies on process env reads (`os.Getenv`) which are concurrent-safe in Go.

---

## 6. Error model

**No error returns** from public API. Failure modes are silent degradations:

| Situation | Behavior |
|-----------|----------|
| Missing GOCACHE bases | empty GOCACHE; Go may fail later |
| No header dirs | no CGO_CFLAGS |
| nil userCfg | skip whitelist + user Build; still auto-detect headers |
| Relative workspace that Abs fails | use original string for joins |

Errors surface later in the **caller’s** `go` invocation stderr.

---

## 7. Extension points (intentional)

1. Add essential vars in `getBaseGoEnv` list.  
2. Add header search roots in `detectCGOFlags`.  
3. New specialization wrapper (e.g. `GetBuildEnvForRace`) using `MergeEnv`.  
4. Future: return structured `EnvBuildResult{Env, Config, Warnings}` without breaking slice API via new function.

Avoid extension via global variables or init-time filesystem scans.
