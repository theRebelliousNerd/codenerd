# 03 — Gap Analysis: `internal/build`

> Last verified: **2026-07-13**  
> Vision baseline: [01-VISION.md](01-VISION.md)  
> Reality baseline: [02-CURRENT-STATE.md](02-CURRENT-STATE.md)

---

## 1. Spec vs reality matrix

| Vision item | Status | Notes |
|-------------|--------|-------|
| Env factory for Go subprocesses | **Met** | `GetBuildEnv*` + helpers |
| GOCACHE always defined when possible | **Met** | `deriveGOCACHE` |
| sqlite_headers → CGO_CFLAGS | **Met** (when root correct) | `loadBuildConfig` |
| Default sqlite_vec tag | **Met** (conditional) | Skipped if GOFLAGS already set |
| Cross-compile GOOS/GOARCH | **Met** | `GetBuildEnvForCompile` |
| Merge overlay helper | **Met** | `MergeEnv` |
| All go-toolchain spawners adopt package | **Gap** | Only autopoiesis |
| Non-nil UserConfig in production | **Gap** | Callers pass `nil` |
| Workspace root (not temp dir) for detect | **Gap** | Callers pass tmp/arena |
| Real GetBuildEnvForTest behavior | **Gap** | Identity wrapper |
| GoFlags applied | **Gap** | Stored, never used for argv |
| CGOPackages runtime consumer | **Gap** | Metadata / append only |
| Env key normalization | **Gap** | Duplicates possible |
| Dual BuildConfig eliminated | **Gap** | build + config packages |
| Package comment accuracy | **Gap** | Lists non-importers |
| Integration test with real `go` | **Gap** | Unit only |
| Observability of final env (redacted) | **Partial** | Debug count + keys, no audit trail |

---

## 2. Priority backlog (architecture, not schedule)

### P0 — Correctness / honesty

| ID | Gap | Why it matters | Direction |
|----|-----|----------------|-----------|
| G-01 | Package comment overclaims consumers | Misleads wiring audits | Align comment with importers **or** adopt package in claimed sites |
| G-02 | Autopoiesis `nil` config | Sample Build config never reaches tool compile | Thread `*UserConfig` when available |
| G-03 | Wrong root for detection on tool builds | Auto CGO never sees monorepo headers (often OK with CGO=0) | Split `workspaceRoot` vs `cmd.Dir` |

### P1 — Mandate completion

| ID | Gap | Why it matters | Direction |
|----|-----|----------------|-----------|
| G-10 | Incomplete adoption | Drift in env conventions | Grep all `exec.Command("go"`; adopt or document exemption |
| G-11 | Shell/tactile separate env builders | Dual physics for “what go sees” | Document boundary; only project-aware go builds must use `internal/build` |

### P2 — API completeness

| ID | Gap | Why it matters | Direction |
|----|-----|----------------|-----------|
| G-20 | `GetBuildEnvForTest` no-op | Dead API surface | Implement or delete |
| G-21 | `GoFlags` unused | Config lies | Apply via helper `ApplyGoFlags(args, cfg)` or remove |
| G-22 | Dual `BuildConfig` | Drift risk | Single type in config; build imports it only |

### P3 — Hardening

| ID | Gap | Why it matters | Direction |
|----|-----|----------------|-----------|
| G-30 | Duplicate env keys | Subtle override bugs | Normalize with `setEnvKey` throughout merge |
| G-31 | No integration test | Regressions only unit-level | `go env GOCACHE` under constructed env |
| G-32 | No redacted dump API | Hard ops debug | Optional `FormatEnvSummary(env) string` |

---

## 3. Non-gaps (do not “fix”)

| Observation | Why not a gap |
|-------------|----------------|
| Package is small (1 file) | Intentional; do not invent subsystems |
| No Mangle rules | Correct layer — env is not policy |
| No prompt atoms | Correct — no LLM surface |
| Callers force `CGO_ENABLED=0` | Sandbox isolation for generated tools is intentional |
| Race detector not auto-enabled | Explicit design comment; speed/CI concerns |
| Does not use full `os.Environ()` | Safety feature, not omission |
| Not on OODA hot path | Correct placement under actuators |

---

## 4. Interaction with other packages’ gaps

| External gap | Coupling |
|--------------|----------|
| Autopoiesis tool compile failures on missing go/toolchain | Orthogonal; build cannot install go |
| Tactile env filtering differs from build filtering | Boundary; document in both corpuses |
| Config sample hardcodes `C:/CodeProjects/codeNERD/...` | Portable fix is auto-detect + relative workspace |

---

## 5. Suggested acceptance tests for closing major gaps

```text
G-02 closed when:
  tool_compiler or thunderdome receives non-nil UserConfig in at least one production boot path
  and a unit/integration test asserts Build.EnvVars appear in cmd.Env

G-03 closed when:
  GetBuildEnv is called with workspace root while cmd.Dir remains temp module
  and a test with sqlite_headers in workspace shows CGO_CFLAGS when CGO enabled

G-10 closed when:
  architecture doc lists every go-spawn site as "uses build" or "exempt: reason"
```

---

## 6. Residual risk if gaps stay open

- Future features re-copy CGO flag recipes into new packages.  
- Config UI for `build.env_vars` appears to work but never affects Ouroboros compiles.  
- Auditors delete “unused” build helpers after grepping only cmd/, missing autopoiesis.  
- Operators trust package comment and assume attack_runner already has correct env.
