# 07 — Dependency Map: JIT config

> Last verified against codebase: **2026-07-13**  
> Package: `internal/jit/config`

## 1. Upstream (what this package imports)

| Dependency | Used for | Evidence |
|------------|----------|----------|
| `fmt` | error wrapping in `Validate` | `types.go` |
| `strings` | `TrimSpace` on identity | `types.go` |

**No** project-internal imports. This is a leaf schema package.

```
stdlib (fmt, strings)
        ▲
        │
internal/jit/config
```

## 2. Downstream (who imports `codenerd/internal/jit/config`)

### 2.1 Production Go

| Package | Files (representative) | How used |
|---------|------------------------|----------|
| `internal/prompt` | `config_factory.go`, `compiler.go` | Construct & optionally attach config |
| `internal/session` | `executor.go`, `executor_tools.go`, `spawner.go`, `subagent.go` | Hold, inject, allowlist tools |

### 2.2 Tests

| Package / tree | Role |
|----------------|------|
| `internal/jit/config` | Unit Validate tests |
| `internal/prompt` | ConfigFactory output shape + Validate |
| `internal/session` | Mocks, spawner YAML, executor boundaries |
| `tests/e2e` | Cross-boundary, campaign, orchestrator, specialist, piggyback, race |

### 2.3 Docs / skills (non-compile)

| Path | Role |
|------|------|
| `.claude/skills/codenerd-builder/references/jit-execution-model.md` | Architecture narrative |
| `internal/prompt/README.md` | Points at types.go |
| `Docs/architecture/prompt/`, `session/`, `cli/` | Cross-links |

## 3. Import direction (must preserve)

```
internal/jit/config  ◄──  internal/prompt
internal/jit/config  ◄──  internal/session
internal/prompt      ◄──  internal/session  (CompilationResult, compiler interfaces)
internal/session     ◄──  cmd/nerd, tests
```

**Forbidden:** `internal/jit` importing `prompt`, `session`, `core`. Would create cycles or pull orchestration into schema.

## 4. Runtime collaboration graph

```
cmd/nerd/chat/session_boot.go
    NewDefaultConfigFactory()  ──► prompt.ConfigFactory
    wires into session.Executor / Spawner

perception.Intent.Verb ──► ConfigFactory.Generate ──► EffectiveAgentRuntimeConfig
                                                      │
                                                      ▼
                                              session.Executor
                                                      │
                              AllowedTools ──► tools.Global() / VirtualStore
```

## 5. Soft dependencies (names only)

| Logical dependency | Mechanism |
|--------------------|-----------|
| Policy files (`base.mg`, `coder.mg`, …) | Strings in `Policies`; content owned by Mangle defaults / policy corpus |
| Tool names (`read_file`, …) | Strings in `AllowedTools`; implementations in `internal/tools` |
| Specialist path `.nerd/agents/<name>/config.yaml` | Consumer convention in Spawner |

## 6. Verify reverse deps

```powershell
rg "codenerd/internal/jit" -g "*.go"
```

Expect: prompt, session, jit tests, e2e tests only (plus any future consumers).

## 7. Dependency risk register

| Risk | Severity | Notes |
|------|----------|-------|
| Name collision with `internal/config` | Low | Import path disambiguates |
| Expanding package into factory | High | Would duplicate prompt and couple leaf |
| Session depending on more fields without tests | Medium | Partial wiring already present |
