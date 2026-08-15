# 03 — Gap Analysis: `internal/build`

> Last verified: **2026-08-15**  
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
| All go-toolchain spawners adopt package | **Met (enforced)** | autopoiesis + session + core; every other site carries a reasoned exemption checked by `go_invocation_inventory_test.go` |
| Non-nil UserConfig in production | **Gap** | Only `session.verifyBuild` passes one |
| Workspace root (not temp dir) for detect | **Partial** | `DetectionRootFor` / `GetBuildEnvForModule` exist; autopoiesis has not adopted them |
| Real GetBuildEnvForTest behavior | **Met** | `GOTRACEBACK=all`, `-count=1`, CI/GORACE/GOMAXPROCS/GOTMPDIR |
| GoFlags applied | **Met** | `AppendGoFlags` |
| CGOPackages runtime consumer | **Gap (by design)** | Metadata / append only |
| Env key normalization | **Met** | `setEnvKey` at every stage; sorted config keys for determinism |
| Dual BuildConfig eliminated | **Met** | `build.BuildConfig = config.BuildConfig` |
| Package comment accuracy | **Met (enforced)** | `TestBuildImporters_WhenNewConsumerAppears_ShouldBeDocumented` |
| Integration test with real `go` | **Met** | `go env` and a compiled-and-run throwaway test module |
| Observability of final env (redacted) | **Met** | `SummarizeEnv` keys-only + `redactEnvValue`; still no audit trail |

---

## 2. Priority backlog (architecture, not schedule)

### P0 — Correctness / honesty

| ID | Gap | Why it matters | Direction |
|----|-----|----------------|-----------|
| G-01 | ~~Package comment overclaims consumers~~ | Misleads wiring audits | **Closed** — comment lists real importers; a test fails when the set changes |
| G-02 | Autopoiesis `nil` config | Sample Build config never reaches tool compile | **Open** — thread `*UserConfig` when available |
| G-03 | Wrong root for detection on tool builds | Auto CGO never sees monorepo headers (often OK with CGO=0) | **Half closed** — `DetectionRootFor` / `GetBuildEnvForModule` ship; autopoiesis call sites still pass `tmpDir` |

### P1 — Mandate completion

| ID | Gap | Why it matters | Direction |
|----|-----|----------------|-----------|
| G-10 | ~~Incomplete adoption~~ | Drift in env conventions | **Closed** — the grep became `go_invocation_inventory_test.go`: an AST audit that fails on any new unmarked `go` spawn and on stale exemptions |
| G-11 | ~~Shell/tactile separate env builders~~ | Dual physics for “what go sees” | **Closed** — permanent exemptions written into `Docs/architecture/tools/09-SAFETY-AND-INVARIANTS.md` and `Docs/architecture/tactile/09-SAFETY-AND-INVARIANTS.md` |
| G-12 | `internal/tools/codedom/run_impacted_tests.go` runs `go test` with no `cmd.Env` | A CGO project reports a compile error as a test failure | **Open** — recorded as `pending adoption`; should use `GetBuildEnvForTest` + `AppendGoFlags` |

### P2 — API completeness

| ID | Gap | Why it matters | Direction |
|----|-----|----------------|-----------|
| G-20 | ~~`GetBuildEnvForTest` no-op~~ | Dead API surface | **Closed** — implemented, not deleted |
| G-21 | ~~`GoFlags` unused~~ | Config lies | **Closed** — `AppendGoFlags(userCfg, root, args)` |
| G-22 | ~~Dual `BuildConfig`~~ | Drift risk | **Closed** — `type BuildConfig = config.BuildConfig` |

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
