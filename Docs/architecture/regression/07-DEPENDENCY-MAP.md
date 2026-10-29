# regression — Dependency Map

> Last verified against codebase: **2026-08-15**  
> Evidence: source imports in `battery.go`, `policy.go`, `seed.go`; reverse grep for `codenerd/internal/regression`

---

## 1. Upstream (what this package imports)

### 1.1 Standard library

| Package | Use |
|---------|-----|
| `context` | Cancellation and per-task timeouts |
| `fmt` | Error formatting / wrapping |
| `os` | `ReadFile` |
| `os/exec` | Subprocess execution |
| `path/filepath` | `DefaultBatteryPath` |
| `runtime` | `GOOS` shell selection |
| `strings` | Type/command normalize |
| `time` | Duration measurement; timeout durations |

### 1.2 Third-party

| Module | Version (go.mod) | Use |
|--------|------------------|-----|
| `gopkg.in/yaml.v3` | v3.0.1 | Unmarshal battery YAML |

### 1.3 Internal

| Package | Use |
|---------|-----|
| `internal/logging` | `CategoryRegression` run/task logging |
| `internal/types` | `types.Fact` for `PolicyFacts` constitutional projection |

Both are leaves (`internal/types` itself depends only on `internal/logging`), so
`internal/regression` stays cheap to import and cycle-free.

```
internal/regression
    ├── internal/logging
    ├── internal/types → internal/logging
    └── (stdlib + yaml.v3)
```

---

## 1.4 Runtime (external process) dependency

This package shells out. The interpreter is **not vendorable** and is the one
dependency an operator can be missing:

| Platform | Binary | Invocation | Notes |
|----------|--------|------------|-------|
| Linux / macOS | `bash` | `bash --noprofile --norc` | `bash -l` when `RunOptions.LoginShell` is set. `sh` is **not** a substitute: `--noprofile`/`--norc` are bash spellings, and the seeded battery uses bashisms. |
| Windows | `powershell` | `powershell -NoProfile -NonInteractive -Command -` | Windows PowerShell 5.x, present on every supported Windows. `pwsh` (PowerShell 7) is **not** looked up. |

`RequiredShell()` returns the binary name for the running platform and
`CheckShell()` reports whether it is on `PATH`. `RunBatteryWithOptions`
preflights with `CheckShell` and fails the **run** with an actionable error
rather than reporting every task as failed — a bare `exec.LookPath` failure
produced N identical "executable file not found in $PATH" task errors and never
stated the cause. Pinned by
`TestRunBattery_WhenTheShellIsMissing_ShouldFailTheRunNotEveryTask`.

Task commands may of course depend on `go`, `git`, a test runner and so on, but
that is the battery author's contract, not this package's.

---

## 2. Downstream (who imports this package)

### 2.1 Production Go

Search:

```text
codenerd/internal/regression
```

across `*.go` outside `internal/regression/`:

| Consumer class | Status |
|----------------|--------|
| `cmd/nerd` | **imports** — `cmd/nerd/cmd_regression.go` (`run` / `init` / `list`) |
| `cmd/nerd/chat` | does not import |
| `internal/campaign` | does not import |
| `internal/shards` / nemesis | does not import |
| `internal/core` / VirtualStore | does not import |
| `internal/testing` | does not import |

### 2.2 Tests

Only self-tests in `battery_test.go` (same package; no external import path needed).

### 2.3 Docs / metadata

Referenced in architecture index and audit tables as a clean package — documentation only.

---

## 3. Conceptual adjacency (not edges)

```
┌─────────────────┐     no import      ┌──────────────────┐
│ campaign assault│ ·················· │ internal/        │
│ / Nemesis       │                    │ regression       │
└─────────────────┘                    └────────┬─────────┘
                                                │ no import
                                       ┌────────▼─────────┐
                                       │ VirtualStore /   │
                                       │ kernel / CLI     │
                                       └──────────────────┘
```

These systems solve related “run checks / break patches” problems but share **no type or package edge** today.

---

## 4. Dependency risk assessment

| Risk | Level | Notes |
|------|-------|-------|
| Dependency churn | Low | Only yaml.v3 beyond stdlib |
| Circular deps | None | Leaf-ish library (actually **unused leaf**) |
| CGO | None | Pure Go |
| Platform tools | Medium | Requires `powershell` or `bash` on PATH — preflighted by `CheckShell`, documented in §1.4 |

---

## 5. Recommended future edges (not implemented)

| From | To | Purpose |
|------|----|---------|
| `internal/campaign` or assault host | `regression` | Stage type |
| `internal/init` | `regression.Seed` | Seed `battery.yaml` on `nerd init` — one call, see [08-WIRING-AND-INTEGRATION.md](08-WIRING-AND-INTEGRATION.md) §2.5 |
| VirtualStore action router | `regression` | **Deliberately not taken** — see [09-SAFETY-AND-INVARIANTS.md](09-SAFETY-AND-INVARIANTS.md) §4 |

Any edge must keep policy outside this package (P9). `PolicyFacts` honours that:
it projects the battery's tasks into EDB facts and asserts nothing about whether
they are allowed. The decision lives in
`internal/core/defaults/policy/regression_battery.mg`.

---

## 6. Verification commands

```powershell
rg "codenerd/internal/regression" -g "*.go"
go list -f "{{.ImportPath}} {{.Imports}}" ./internal/regression
```
