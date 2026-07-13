# regression — Public API and Types

> Last verified against codebase: **2026-07-13**  
> Package: `codenerd/internal/regression`  
> Source: `internal/regression/battery.go`

---

## 1. Import

```go
import "codenerd/internal/regression"
```

No init side effects.

---

## 2. Types

### 2.1 `Battery`

| Field | Type | YAML | Notes |
|-------|------|------|-------|
| `Version` | `int` | `version` | Loaded; not enforced by runner |
| `Tasks` | `[]Task` | `tasks` | Order = execution order |

Location: `internal/regression/battery.go` (struct `Battery`).

### 2.2 `Task`

| Field | Type | YAML | Notes |
|-------|------|------|-------|
| `ID` | `string` | `id` | Copied to `Result.TaskID` |
| `Type` | `string` | `type` | `"shell"`; empty ⇒ shell |
| `Command` | `string` | `command` | Shell script body |
| `TimeoutSec` | `int` | `timeout_sec` | omitempty; ≤0 ⇒ 300s |

Location: `internal/regression/battery.go` (struct `Task`).

### 2.3 `Result`

| Field | Type | Notes |
|-------|------|-------|
| `TaskID` | `string` | From task |
| `Success` | `bool` | true only on clean shell success |
| `Output` | `string` | Combined stdout+stderr (may be partial on fail) |
| `Error` | `string` | Empty on success |
| `DurationMs` | `int64` | Wall time including setup |

No YAML tags. Location: `internal/regression/battery.go` (struct `Result`).

---

## 3. Functions

### 3.1 `LoadBattery`

```go
func LoadBattery(path string) (*Battery, error)
```

| Aspect | Detail |
|--------|--------|
| Input | Filesystem path to YAML |
| Success | `*Battery` |
| Failure | Raw IO error or `failed to parse battery YAML: %w` |
| Validation | None beyond YAML decode |

### 3.2 `RunBattery`

```go
func RunBattery(ctx context.Context, b *Battery, workdir string) ([]Result, error)
```

| Aspect | Detail |
|--------|--------|
| `ctx` | Parent cancel/deadline; nested per task |
| `b` | nil or empty Tasks ⇒ `(nil, nil)` |
| `workdir` | `""` = process default; else `cmd.Dir` |
| Return error | Currently always `nil` after early checks; inspect Results |
| Order | Task order; stops at first `!Success` |

### 3.3 `DefaultBatteryPath`

```go
func DefaultBatteryPath(workspace string) string
```

Returns `filepath.Join(workspace, ".nerd", "regression", "battery.yaml")`.  
Does not touch the filesystem.

---

## 4. Unexported (for implementers)

### `runShell`

```go
func runShell(ctx context.Context, command string, workdir string) (string, error)
```

| OS | Command |
|----|---------|
| windows | `powershell -NoProfile -Command -` + stdin |
| else | `bash -l` + stdin |

Not part of the stable public API; hosts should not rely on it via tricks.

---

## 5. Typical host snippet

```go
path := regression.DefaultBatteryPath(workspace)
b, err := regression.LoadBattery(path)
if err != nil {
    return err
}
results, err := regression.RunBattery(ctx, b, workspace)
if err != nil {
    return err // rare today
}
for _, r := range results {
    if !r.Success {
        return fmt.Errorf("task %s failed: %s", r.TaskID, r.Error)
    }
}
```

Vacuous case: missing battery file is an error from `LoadBattery`; empty battery file yields nil results (host must decide if that is OK).

---

## 6. Stability notes

| Surface | Stability |
|---------|-----------|
| Type fields | Additive YAML fields should use `omitempty` and version bumps if semantic |
| Fail-fast | Behavioral contract — treat as stable default |
| Default timeout 5m | Behavioral default — document if changed |
| Error return of `RunBattery` always nil | **Fragile** — hosts should not depend on “err means failure”; prefer Results. A future improvement might return a sentinel error without breaking Result inspection. |
