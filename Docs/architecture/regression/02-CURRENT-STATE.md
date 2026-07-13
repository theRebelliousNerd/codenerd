# regression — Current State

> Last verified against codebase: **2026-07-13**  
> Package: `internal/regression/`  
> Method: full read of both package files; reverse-import grep

---

## 1. Inventory snapshot

| Metric | Value |
|--------|------:|
| Non-test `.go` files | 1 |
| Test `.go` files | 1 |
| Mangle `.mg` | 0 |
| Subpackages | 0 |
| Approx. source lines | 138 |
| Approx. test lines | 102 |
| Exported types | 3 |
| Exported funcs | 3 |
| Unexported funcs | 1 (`runShell`) |
| External module deps | `gopkg.in/yaml.v3` |
| Internal package deps | 0 |
| Known Go importers (other packages) | **0** |

---

## 2. File map

```
internal/regression/
├── battery.go         # package docs, types, load, run, shell, path
└── battery_test.go    # 5 tests
```

| Path | Role | Hotspot? |
|------|------|----------|
| `internal/regression/battery.go` | Entire product surface | **Yes** — only implementation file |
| `internal/regression/battery_test.go` | Unit coverage of public API | Primary test entry |

---

## 3. Behavioral inventory (what works today)

| Behavior | Status | Location |
|----------|--------|----------|
| Parse YAML battery from disk | Works | `LoadBattery` |
| Default workspace path convention | Works (path only) | `DefaultBatteryPath` |
| Run shell tasks in order | Works | `RunBattery` + `runShell` |
| Empty type → shell | Works | type normalize |
| Per-task timeout (default 5m) | Works | `WithTimeout` |
| Parent context cancel/timeout | Works | `CommandContext` + `ctx.Err()` preference |
| Fail-fast on first failure | Works | break after `!Success` |
| Unsupported type error | Works | default switch branch |
| Nil/empty battery no-op | Works | early return `(nil, nil)` |
| Combined stdout/stderr capture | Works | `CombinedOutput` |
| Workdir for subprocess | Works when non-empty | `cmd.Dir` |
| Version field enforcement | **Not implemented** | field loaded only |
| Expected output matching | **Not implemented** | — |
| Multi-type tasks | **Not implemented** | only `shell` |
| Result JSON/YAML export | **Not implemented** | — |
| CLI / campaign / Nemesis host | **Not implemented** | no importers |
| Create `.nerd/regression/` on init | **Not implemented** | no init template found |
| Logging | **Not implemented** | — |

---

## 4. Workspace state

Under this repository’s `.nerd/` (as listed 2026-07-13): **no** `regression/` directory and **no** `battery.yaml`. The path convention exists only in code (`DefaultBatteryPath`).

---

## 5. API completeness matrix

| Symbol | Purpose | Tested? |
|--------|---------|---------|
| `Battery` | Suite container | via Load/Run tests |
| `Task` | Single task | via Load/Run tests |
| `Result` | Outcome | via Run tests |
| `LoadBattery` | Disk → struct | `TestLoadBattery` |
| `RunBattery` | Execute suite | success / unsupported / empty |
| `DefaultBatteryPath` | Path helper | `TestDefaultBatteryPath` |
| `runShell` | OS shell | indirect via success test |

---

## 6. Complexity / risk hotspots

Despite small size, the **real risk** concentrates in `runShell`:

1. Arbitrary shell execution.  
2. OS divergence (PowerShell vs bash login).  
3. Timeout vs process cleanup (OS-dependent child process trees).  
4. Fail-fast hiding later task failures.  
5. Silent product deadness (no importer → bit-rot risk).

---

## 7. Comparison to package comment claims

| Claim | Current state |
|-------|---------------|
| Lightweight harness | **True** |
| Optional | **True** (and currently always optional because unwired) |
| YAML-defined suites | **True** |
| Nemesis gauntlet integration | **False** (no code path) |
| Manual use | **True** as library; **False** as first-class CLI |

---

## 8. Relationship to “regression” elsewhere

Many tests in other packages use the English word “regression” in comments (guards against reintroduced bugs). Those are **not** this package. This package exclusively means **YAML battery harness**.
