# regression — Testing Alignment

> Last verified against codebase: **2026-07-13**  
> Tests: `internal/regression/battery_test.go`

---

## 1. Existing tests

| Test | Intent | Assertions |
|------|--------|------------|
| `TestLoadBattery` | YAML load | version=1; one task id `smoke` |
| `TestRunBatterySuccess` | Happy path shell | success; output contains `ok`; 10s parent timeout; task timeout 5s |
| `TestRunBatteryUnsupportedTask` | Fail-fast + type reject | one result; error contains `unsupported task type`; second task not run |
| `TestRunBatteryEmpty` | Vacuous suite | nil results; nil error |
| `TestDefaultBatteryPath` | Path convention | contains `.nerd` and `battery.yaml` |

All tests are package-local unit tests (same package `regression`).

---

## 2. Coverage vs code paths

| Code path | Covered? |
|-----------|----------|
| Load happy path | Yes |
| Load missing file | **No** |
| Load bad YAML | **No** |
| Run shell success | Yes |
| Run unsupported + fail-fast | Yes |
| Run empty/nil | Empty yes; **nil battery not explicit** (same branch as empty) |
| Empty command | **No** |
| Timeout fires | **No** |
| Parent cancel | **No** |
| workdir non-empty | **No** |
| Multi-task all success | **No** |
| Default timeout when TimeoutSec=0 | **No** (tests use explicit 5s) |
| Windows vs Unix shell selection | Indirect only (runs on host OS) |

---

## 3. Commands

```powershell
go test ./internal/regression/...
go test -race ./internal/regression/...
go test -v ./internal/regression/ -count=1
go test ./internal/regression/ -cover
```

No CGO requirement for this package. No external services.

Platform note: `TestRunBatterySuccess` requires `powershell` (Windows) or `bash` (Unix) available.

---

## 4. Recommended additional tests (backlog)

| Priority | Test idea |
|----------|-----------|
| P1 | `LoadBattery` on missing path → error |
| P1 | `LoadBattery` invalid YAML → wrapped error |
| P1 | Timeout: `timeout_sec: 1` + sleep command → failure, fail-fast |
| P2 | workdir: write file relative to temp workdir |
| P2 | Multi-task success chain (2+ shell echos) |
| P2 | Empty command task → failure |
| P3 | Nil `*Battery` explicit |
| P3 | Table-driven OS-specific command (`echo` vs `Write-Output`) if needed |

Avoid tests that depend on network or full `go test ./…` of the monorepo inside unit tests.

---

## 5. Integration tests (absent)

When wiring lands, add:

| Host | Test |
|------|------|
| CLI | temp workspace + battery.yaml + `nerd regression run` exit codes |
| Campaign | assault config enabling battery stage |
| VS | permitted vs denied action (policy) |

---

## 6. Alignment with repo verification bar

Repo guidance: run targeted `go test` for touched packages; prefer `go test ./…` when feasible.

For this package alone:

```powershell
go test ./internal/regression/...
```

is sufficient and fast. It does **not** substitute monorepo CI.

---

## 7. What “regression” tests elsewhere mean

Other packages’ tests named “regression guard” are **historical bug locks**, unrelated to this YAML harness. Do not move them here.
