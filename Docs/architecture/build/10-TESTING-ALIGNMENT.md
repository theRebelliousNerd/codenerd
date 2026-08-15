# 10 — Testing Alignment: `internal/build`

> Last verified: **2026-08-15**

---

## 1. Test inventory

| File | Approx lines | Role |
|------|-------------:|------|
| `internal/build/env_test.go` | 211 | Core helpers + loadBuildConfig sqlite matrix |
| `internal/build/env_gaps_test.go` | 427 | Public API + edge cases (“gaps” coverage) |
| `internal/build/env_features_test.go` | 460 | Key normalization + ordering determinism, `GetBuildEnvForTest` specialization, `AppendGoFlags`, `DetectionRootFor`, GOCACHE warning, redaction, real-toolchain integration |
| `internal/build/go_invocation_inventory_test.go` | 402 | Repo-wide AST audit: every non-test `exec.Command("go", …)`, and the importer set |

**Total tests:** package tests only; no external suite owns this package.

---

## 2. Coverage map (behavior → tests)

| Behavior | Tests (representative) |
|----------|------------------------|
| `deriveGOCACHE` none / LOCALAPPDATA / USERPROFILE / HOME / TEMP | `TestDeriveGOCACHE_Precedence` |
| `deriveGOCACHE` TMP / TMPDIR | `TestDeriveGOCACHE_TMP`, `_TMPDIR` |
| `hasEnvKey` / `setEnvKey` / `MergeEnv` basics | `TestEnvKeyHelpers` |
| Partial key false positive | `TestHasEnvKey_WhenPartialMatch` |
| Merge override / malformed / no mutate | `TestMergeEnv_*` |
| Multi-dir `detectCGOFlags` order | `TestDetectCGOFlags` |
| Empty / single header dir | `TestDetectCGOFlags_WhenNoHeaderDirs`, `_WhenSingleHeaderDir` |
| sqlite default + GOFLAGS respect + user override | `TestLoadBuildConfig_SqliteHeaders` |
| No sqlite → no CGO | `TestLoadBuildConfig_WhenNoSqliteHeaders` |
| User GoFlags / CGOPackages merge | `TestLoadBuildConfig_WhenUserConfigGoFlags`, `_CGOPackages` |
| DefaultBuildConfig empty | `TestDefaultBuildConfig_ShouldHaveEmptyFields` |
| getBaseGoEnv PATH/GOPATH | `TestGetBaseGoEnv_*` |
| GetBuildEnv nil / headers / whitelist / config | `TestGetBuildEnv_*` |
| GetBuildEnvForTest length | `TestGetBuildEnvForTest_ShouldIncludeBuildEnv` |
| Cross-compile set / empty | `TestGetBuildEnvForCompile_*` |
| No duplicate env keys across merge stages | `TestGetBuildEnv_WhenWhitelistRepeatsEssentialVar_ShouldNotDuplicateKey`, `_WhenConfigOverridesEssentialVar_` |
| Env slice ordering determinism | `TestGetBuildEnv_WhenCalledTwice_ShouldBeDeterministic` |
| Test specialization (GOTRACEBACK, -count=1, CI/GORACE) | `TestGetBuildEnvForTest_When*` |
| Explicit caller values survive specialization | `TestGetBuildEnvForTest_WhenCallerPinnedCount_ShouldNotOverride` |
| `GoFlags` → argv | `TestAppendGoFlags_*` |
| Detection root vs module dir | `TestDetectionRootFor_*` |
| GOCACHE warning fires / does not fire | `TestGetBaseGoEnv_WhenGOCACHEUnderivable_ShouldWarn`, `_WhenGOCACHEDerivable_ShouldNotWarn` |
| Secret redaction in logs, values still passed through | `TestRedactEnvValue_WhenSecretProneKey_ShouldRedact`, `TestGetBuildEnv_WhenSecretInConfigEnvVars_ShouldStillBePassedToSubprocess` |
| Keys-only summary | `TestSummarizeEnv_ShouldReturnSortedKeysOnly` |
| Real toolchain accepts the env | `TestGetBuildEnv_WhenRealWorkspace_ShouldSurviveGoEnv`, `TestGetBuildEnvForTest_WhenRealWorkspace_ShouldCompileAndRunATest` |
| Repo-wide adoption of the env factory | `TestGoInvocations_WhenSpawningGo_ShouldUseBuildEnvOrBeExempt` |
| Package-comment importer list accuracy | `TestBuildImporters_WhenNewConsumerAppears_ShouldBeDocumented` |

---

## 3. Commands

```powershell
go test ./internal/build/...
go test -race ./internal/build/...
go test ./internal/build/... -count=1 -v
go test -short ./internal/build/...                                      # skip toolchain integration
go test -v ./internal/build/ -run 'TestGoInvocations|TestBuildImporters' # print the inventory
```

No CGO required for these unit tests (they do not invoke the Go toolchain with CGO compile).

---

## 4. Testing strengths

1. **Env isolation** via `t.Setenv` and `clearEnvVars` helpers.  
2. **Filesystem fixtures** via `t.TempDir` for headers.  
3. **Deterministic multi-dir flag order** asserted exactly.  
4. **Both happy path and negative path** for sqlite detection.  
5. High ratio of test LOC to production LOC for a utility package.

---

## 5. Testing gaps

| Gap | Risk | Suggested test |
|-----|------|----------------|
| No real `go build`/`go env` under constructed env | Miss GOCACHE/path bugs on OS | Integration test calling `go env GOCACHE` with `cmd.Env` |
| Callers always `nil` config | Regression if someone threads config wrong | Autopoiesis-level test with mock UserConfig (belongs there) |
| Duplicate key last-wins not asserted | Subtle env bugs | Assert single FOO after multi-stage merge (if normalized) |
| GetBuildEnvForTest equals GetBuildEnv only | Future drift | Keep assertion or delete API |
| Windows vs Unix path separators | Low (filepath.Join used) | Already use filepath in expected values |
| Concurrent GetBuildEnv | Low | Optional `-race` is enough given no shared state |

---

## 6. Alignment with package principles

| Principle | Test support |
|-----------|--------------|
| Filtered env | Nil config still returns env; whitelist tests |
| sqlite honesty | Header tempdir tests |
| Merge purity | No-mutate test |
| Cross-compile | GOOS/GOARCH tests |

---

## 7. CI notes

Package is pure unit-testable; should run in any CI that has Go. Does not require `sqlite_headers` present in the real repo for tests (creates its own). Does not require ROCm/CUDA or large disk.

---

## 8. Definition of test-done for future changes

When changing merge order or detection:

1. Unit tests in this package green.  
2. If changing public semantics, update this doc’s coverage map.  
3. If changing autopoiesis wiring, add/adjust tests under `internal/autopoiesis`.  
