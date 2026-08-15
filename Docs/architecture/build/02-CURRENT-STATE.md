# 02 — Current State: `internal/build`

> Last verified: **2026-08-15**  
> Method: list dir, full read of package sources, reverse-import grep

---

## 1. Scale snapshot

| Kind | Count | Paths |
|------|------:|-------|
| Non-test Go | **1** | `internal/build/env.go` |
| Test Go | **2** | `env_test.go`, `env_gaps_test.go` |
| Mangle (`.mg`) | **0** | — |
| Package docs in tree | **0** | no README/agents.md under package |
| Approximate production LOC | **~312** | `env.go` |
| Approximate test LOC | **~638** | tests combined |

---

## 2. File inventory

### 2.1 Production

| File | Role | Hotspots |
|------|------|----------|
| `internal/build/env.go` | Entire package | `GetBuildEnv` (merge pipeline), `loadBuildConfig` (sqlite_headers), `deriveGOCACHE`, `detectCGOFlags`, `MergeEnv` |

Package comment (paraphrased): unifies build env so preflight, thunderdome, ouroboros, attack_runner, tester do not bypass tactile / miss `CGO_CFLAGS`.

### 2.2 Tests

| File | Focus |
|------|--------|
| `env_test.go` | `deriveGOCACHE` precedence; `hasEnvKey`/`setEnvKey`/`MergeEnv`; multi-header `detectCGOFlags`; `loadBuildConfig` sqlite + user override |
| `env_gaps_test.go` | `DefaultBuildConfig`; `getBaseGoEnv`; full `GetBuildEnv*` matrix; MergeEnv mutation/malformed; TMP/TMPDIR GOCACHE |

### 2.3 Symbol inventory (production)

**Exported types**

- `BuildConfig` — alias for `config.BuildConfig` (`EnvVars`, `GoFlags`, `CGOPackages`)

**Exported functions**

- `DefaultBuildConfig() *BuildConfig`
- `GetBuildEnv(userCfg *config.UserConfig, workspaceRoot string) []string`
- `GetBuildEnvForTest(userCfg *config.UserConfig, workspaceRoot string) []string`
- `GetBuildEnvForCompile(userCfg *config.UserConfig, workspaceRoot string, targetOS, targetArch string) []string`
- `MergeEnv(base []string, additional ...string) []string`

**Unexported functions**

- `getBaseGoEnv`, `deriveGOCACHE`, `loadBuildConfig`, `detectCGOFlags`, `hasEnvKey`, `setEnvKey`, `envValue`, `redactEnvValue`, `isRepoBoundary`, `withCountOne`, `goFlagName`

No interfaces, no constructors beyond `DefaultBuildConfig`, no init hooks, no registration.

---

## 3. Behavioral inventory (what code actually does)

### 3.1 Always / usually present in env output

If present in the **parent process**:

- `PATH`
- `GOPATH`, `GOROOT`, `GOCACHE`, `GOMODCACHE`, `GOFLAGS`
- `HOME`, `USERPROFILE`, `LOCALAPPDATA`
- `TEMP`, `TMP`, `TMPDIR`

If `GOCACHE` absent after copy → derived path.

### 3.2 Conditional injections

| Condition | Injection |
|-----------|-----------|
| `userCfg != nil` + whitelist keys non-empty in process | Those keys |
| `userCfg.Build.EnvVars` set | Those KEY=value |
| `workspace/sqlite_headers` dir exists | `CGO_CFLAGS=-I…`, maybe `GOFLAGS=-tags=sqlite_vec`, `CGOPackages+=sqlite-vec` |
| Still no `CGO_CFLAGS` | Scan common include dirs |
| Compile API with OS/arch | `GOOS`, `GOARCH` |

### 3.3 What it does **not** do (current state)

- Does not invoke `go` itself  
- Does not write files  
- Does not read `go.mod`  
- Does not apply `GoFlags` to any argv  
- Does not enable race detector (comment explicitly declines)  
- Does not integrate with tactile allowlists beyond env shape  
- Does not register with kernel/shards  

---

## 4. Call-site inventory (current wiring)

| Caller | API | userCfg | workspaceRoot | Extra |
|--------|-----|---------|---------------|-------|
| `internal/autopoiesis/tool_compiler.go` | `GetBuildEnvForCompile` + `MergeEnv` | `nil` | `tmpDir` (temp module) | `CGO_ENABLED=0` |
| `internal/autopoiesis/thunderdome.go` | `GetBuildEnv` + `MergeEnv` | `nil` | `arenaDir` | `CGO_ENABLED=0` |

No other production importers as of 2026-08-15 (`rg "codenerd/internal/build" -g "*.go"`).

---

## 5. Config surface (external but coupled)

| Location | Content |
|----------|---------|
| `internal/config/build.go` | `config.BuildConfig` + `DefaultBuildConfig()` value type — the **single** definition; `build.BuildConfig` aliases it |
| `UserConfig.Build` | Optional JSON/YAML block |
| Sample defaults in `user_config.go` | Hardcoded example `CGO_CFLAGS=-IC:/CodeProjects/codeNERD/sqlite_headers` |
| `ExecutionConfig.AllowedEnvVars` | Whitelist for step 2 of merge |

Note: sample path is machine-specific; auto-detect is the portable path when workspace root is correct.

---

## 6. Hotspots / maintenance risk

1. **Duplicate key emission** across base + whitelist + config (last-wins depends on Go runtime env rules).  
2. ~~**Two BuildConfig types** can drift.~~ Collapsed to an alias 2026-08-15.  
3. **Package comment list of consumers** is aspirational and will mislead auditors.  
4. **`GetBuildEnvForTest`** looks like a real specialization but is identity.  
5. **Autopoiesis root choice** makes monorepo detection inert on those paths.

---

## 7. Comparison to package comment claims

| Claim | Reality 2026-08-15 |
|-------|--------------------|
| Addresses preflight / thunderdome / ouroboros / attack_runner / tester bypass | thunderdome + ouroboros (tool_compiler) **yes**; others **no importers** |
| Single source of truth; all components should use GetBuildEnv | **Library ready**; **mandate not enforced** |
| Bypass of tactile with raw exec | Callers still use raw `exec.Command`; build only supplies env |

---

## 8. Status label

**Realized living package — narrow adoption.** Not pre-implementation. Not a stub. Corpus must not claim 0% implementation.
