# 07 — Dependency Map: `internal/build`

> Last verified: **2026-08-15**  
> Method: import inspection of `env.go` + reverse `rg "codenerd/internal/build"`

---

## 1. Upstream (what build imports)

```
internal/build
  ├── codenerd/internal/config     // UserConfig, GetExecution, Build block
  ├── codenerd/internal/logging    // BuildDebug / CategoryBuild
  └── stdlib
        maps
        os
        path/filepath
        slices
        strings
```

| Dependency | Why |
|------------|-----|
| `config` | Read `UserConfig.Build` and execution whitelist |
| `logging` | Debug traces of merge decisions |
| `maps` | `maps.Copy` for EnvVars |
| `slices` | `slices.Contains` for CGOPackages |
| `os` / `filepath` | Env + filesystem detection |

**Does not import:** core, mangle, session, shards, tactile, tools, prompt, store, campaign, cmd.

---

## 2. Downstream (who imports build)

### Production Go

| Importer | File | Usage |
|----------|------|-------|
| `internal/autopoiesis` | `tool_compiler.go` | `GetBuildEnvForCompile` + `MergeEnv` |
| `internal/autopoiesis` | `thunderdome.go` | `GetBuildEnv` + `MergeEnv` |

### Tests of other packages

None found that import `internal/build` solely for fixtures (as of verify date).

### Docs / audit references

| Location | Note |
|----------|------|
| `AUDIT.md` | Marks `internal/build` clean |
| `Docs/architecture/INDEX.md` | Tier-2 realized package |
| `Docs/architecture/DARK-FACTORY-JOURNAL.md` | Inventory scores |

### Verify command

```powershell
rg "codenerd/internal/build" -g "*.go" --glob "!*_test.go"
```

Expected (current): `internal/autopoiesis/{tool_compiler,thunderdome}.go`,
`internal/session/{build_verify,test_verify,coverage_profile,lsp_diagnostics}.go`,
`internal/core/virtual_store_actions.go`. `env.go` does not self-import.

The authoritative check is `TestBuildImporters_WhenNewConsumerAppears_ShouldBeDocumented`,
which fails when this list stops matching reality.

---

## 3. Sibling config types

```
internal/config/build.go     config.BuildConfig  ──json──► UserConfig.Build
        │
        │ type BuildConfig = config.BuildConfig   (alias, since 2026-08-15)
        ▼
internal/build/env.go        build.BuildConfig   ──► env slice
```

One type, two names. The field-wise copy and the drift risk it carried are gone.

---

## 4. Related but non-dependent packages

These construct process environments **without** importing `internal/build`:

| Package | Mechanism |
|---------|-----------|
| `internal/tools/shell` | `os.Environ()` based |
| `internal/tactile` | `buildEnvironment` methods |
| `internal/autopoiesis` runtime tool **execution** | separate `toolExecutionEnv()` (not compile) |

Compile path uses `internal/build`; **execution** path of tools may not.

---

## 5. Layering diagram

```
cmd/nerd  /  session  /  core  /  VirtualStore
              │
              ▼
        internal/autopoiesis  (actuator)
              │
              ▼
        internal/build        (env)
              │
              ├── internal/config
              └── internal/logging
              │
              ▼
        os/exec + go toolchain
```

---

## 6. Dependency risk notes

| Risk | Mitigation |
|------|------------|
| build → config grows heavy | Keep using only Build + Execution fields |
| logging init requirements | BuildDebug is fire-and-forget; safe if logger defaulted |
| Autopoiesis only consumer | Deleting package breaks tool forge/thunderdome — do not treat as dead |
