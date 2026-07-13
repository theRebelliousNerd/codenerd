# regression — Dependency Map

> Last verified against codebase: **2026-07-13**  
> Evidence: source imports in `battery.go`; reverse grep for `codenerd/internal/regression`

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

**None.** `internal/regression` does not import any other `codenerd/internal/…` package.

```
internal/regression
    └── (stdlib + yaml.v3 only)
```

---

## 2. Downstream (who imports this package)

### 2.1 Production Go

Search:

```text
codenerd/internal/regression
```

across `*.go` outside `internal/regression/`: **no matches**.

| Consumer class | Status |
|----------------|--------|
| `cmd/nerd` | does not import |
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
| Platform tools | Medium | Requires `powershell` or `bash` on PATH |

---

## 5. Recommended future edges (not implemented)

| From | To | Purpose |
|------|----|---------|
| `cmd/nerd` | `regression` | `nerd regression run` |
| `internal/campaign` or assault host | `regression` | Stage type |
| VirtualStore action router | `regression` | Agent-permitted run |
| `internal/init` or templates | files only | Seed `battery.yaml` |

Any edge must keep policy outside this package (P9).

---

## 6. Verification commands

```powershell
rg "codenerd/internal/regression" -g "*.go"
go list -f "{{.ImportPath}} {{.Imports}}" ./internal/regression
```
