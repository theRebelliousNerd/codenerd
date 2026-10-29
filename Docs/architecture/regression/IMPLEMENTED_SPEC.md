# regression — Implemented Spec (Deep-Dive)

> Last verified against codebase: **2026-07-13**  
> Status: Living Reference Document  
> Language: Go  
> Package: `codenerd/internal/regression`  
> Primary sources: `internal/regression/battery.go`, `internal/regression/battery_test.go`  
> Scale: **1** non-test Go file ≈ **138** lines; **1** test file ≈ **102** lines; **0** Mangle

---

## 1. Overview

`internal/regression` implements a **YAML-defined regression battery**: an ordered list of shell tasks that an operator (or a future gauntlet stage) can run against a workspace to continuously evaluate agent-adjacent environment health.

Package comment (`battery.go`):

> Package regression provides a lightweight, optional regression battery harness.  
> Batteries are YAML-defined task suites that can be run as part of Nemesis gauntlets or manually to continuously evaluate agent behavior.

That comment is **aspirational on the integration side**. As of 2026-07-13:

| Claim in comment | Code reality |
|------------------|--------------|
| YAML task suites | **True** — `LoadBattery` + `Battery`/`Task` YAML tags |
| Manual run | **True as a library** — callers can invoke `RunBattery` |
| Part of Nemesis gauntlets | **Not wired** — zero Go importers of `codenerd/internal/regression` outside this package |

### Key characteristics

| Property | Value |
|----------|-------|
| Style | Pure library; no init hooks, no registration |
| Config format | YAML (`gopkg.in/yaml.v3`) |
| Task types | `shell` only (empty type defaults to `shell`) |
| Execution order | Sequential, document order |
| Failure policy | **Fail-fast** — stop after first unsuccessful task |
| Default per-task timeout | **5 minutes** if `timeout_sec` ≤ 0 |
| Shell (Windows) | `powershell -NoProfile -Command -` (command on stdin) |
| Shell (Unix) | `bash -l` (command on stdin) |
| Output capture | `cmd.CombinedOutput()` into `Result.Output` |
| Canonical path | `{workspace}/.nerd/regression/battery.yaml` |
| Kernel / Mangle | None |
| Prompt / JIT | None |
| Concurrency | None (single-threaded loop) |

### High-level control flow

```
workspace path
    │
    ▼
DefaultBatteryPath(workspace)
    │  →  {ws}/.nerd/regression/battery.yaml
    ▼
LoadBattery(path)
    │  →  os.ReadFile + yaml.Unmarshal → *Battery
    ▼
RunBattery(ctx, battery, workdir)
    │
    ├─ nil / empty Tasks  →  (nil, nil)
    │
    └─ for each Task (in order):
           normalize type ("" → "shell")
           switch type
             shell → runShell(timeout ctx, command, workdir)
             other → Result{Success:false, Error:"unsupported…"}
           append Result
           if !Success → break   // fail-fast
    │
    ▼
[]Result  (error from RunBattery is always nil today; failures are in Result)
```

### Fact-flow placement

codeNERD’s primary OODA fact-flow:

```
user input → perception → user_intent → kernel next_action
  → VirtualStore / shards / tools → articulation
```

`internal/regression` **sits outside** that loop. It does not:

- emit `user_intent` or any Mangle `Decl`
- route through VirtualStore
- participate in `permitted(...)` derivation
- assemble prompts

It is an **optional side harness** for environment/agent regression gates. Once wired (e.g. campaign assault stage or Nemesis post-patch check), a host would load a battery, run it, and *then* assert structured facts into the kernel if desired. That host glue is not present.

---

## 2. Implementation status

| Component | Status | Notes |
|-----------|--------|-------|
| `Battery` / `Task` YAML model | **Implemented** | `battery.go` |
| `Result` outcome model | **Implemented** | In-memory only; no serialization helper |
| `LoadBattery` | **Implemented** | File + YAML; wraps parse errors |
| `RunBattery` | **Implemented** | Sequential + fail-fast + timeouts |
| `runShell` | **Implemented** (unexported) | OS-specific shell, stdin script |
| `DefaultBatteryPath` | **Implemented** | Joins workspace + fixed relative path |
| Task type: `shell` | **Implemented** | Only supported type |
| Task type: others | **Rejected** | Explicit error string; fail-fast |
| Unit tests | **Implemented** | Load, success, unsupported, empty, path |
| CLI verb (`nerd regression …`) | **Absent** | No `cmd/nerd` registration |
| Nemesis / gauntlet integration | **Absent** | Comment-only intent |
| Campaign assault integration | **Absent** | Assault uses its own stage runner |
| Expected-output assertions | **Absent** | Success = process exit 0 only |
| Continue-on-error / report-all | **Absent** | Hard fail-fast |
| Result persistence | **Absent** | No write-to-disk API |
| Mangle surface | **N/A** | No `.mg` in package |
| Structured logging | **Absent** | No log category |

**Overall:** living library code with solid unit coverage of the public API; **integration completeness is low** (~library 95%, product wiring ~0%). Heuristic package completeness for “usable harness if called”: **~70%** (core works; no host, no richer task model).

---

## 3. Source inventory

### 3.1 Package layout

```
internal/regression/
  battery.go        # types, LoadBattery, RunBattery, runShell, DefaultBatteryPath
  battery_test.go   # five unit tests
```

No subpackages. No `agents.md`. No package `README.md` under source.

### 3.2 File roles

| Path | Lines (approx) | Role |
|------|---------------:|------|
| `internal/regression/battery.go` | 138 | Entire implementation |
| `internal/regression/battery_test.go` | 102 | Unit tests |

### 3.3 Symbols

| Kind | Name | Export | Location |
|------|------|--------|----------|
| type | `Battery` | yes | `battery.go:20` |
| type | `Task` | yes | `battery.go:27` |
| type | `Result` | yes | `battery.go:35` |
| func | `LoadBattery` | yes | `battery.go:44` |
| func | `RunBattery` | yes | `battery.go:58` |
| func | `runShell` | no | `battery.go:106` |
| func | `DefaultBatteryPath` | yes | `battery.go:136` |

---

## 4. Deep dive — data model

### 4.1 `Battery`

```go
type Battery struct {
    Version int    `yaml:"version"`
    Tasks   []Task `yaml:"tasks"`
}
```

- `Version` is loaded but **never validated or branched on**. Callers may use it for forward compatibility; the runner ignores it.
- `Tasks` is the only operational field. Empty or nil → `RunBattery` returns `(nil, nil)`.

### 4.2 `Task`

```go
type Task struct {
    ID         string `yaml:"id"`
    Type       string `yaml:"type"` // "shell"
    Command    string `yaml:"command"`
    TimeoutSec int    `yaml:"timeout_sec,omitempty"`
}
```

| Field | Semantics |
|-------|-----------|
| `ID` | Copied to `Result.TaskID`; not uniqueness-checked |
| `Type` | Case-insensitive after trim; empty → `"shell"`; unknown → failure |
| `Command` | Shell script body (stdin to powershell/bash); empty after trim → shell error |
| `TimeoutSec` | Seconds; ≤0 → default **300s** (5 minutes) |

There is **no** YAML for: expected exit code, expected stdout/stderr match, env vars, required tools, tags/severity, skip conditions, or workdir override per task (workdir is suite-level via `RunBattery` argument).

### 4.3 `Result`

```go
type Result struct {
    TaskID     string
    Success    bool
    Output     string
    Error      string
    DurationMs int64
}
```

- No YAML/JSON tags (in-memory / caller serialization).
- `Success` is boolean; no multi-state enum (skipped, timeout-vs-fail).
- On timeout, `Error` is typically `context.DeadlineExceeded` string form (from `ctx.Err()`).
- On unsupported type, `Error` is `unsupported task type: …`.
- `Output` may be partial on failure (CombinedOutput still returns bytes).

### 4.4 Example battery YAML (inferred from tags + tests)

```yaml
version: 1
tasks:
  - id: smoke
    type: shell
    command: echo ok
    timeout_sec: 5
  - id: unit
    type: shell
    command: go test ./internal/regression/...
    timeout_sec: 120
```

---

## 5. Deep dive — `LoadBattery`

**Path:** `internal/regression/battery.go` (`LoadBattery`)

1. `os.ReadFile(path)` — any IO error returned raw.
2. `yaml.Unmarshal` into `Battery`.
3. Parse failure wrapped: `fmt.Errorf("failed to parse battery YAML: %w", err)`.
4. No schema validation (version, required fields, duplicate IDs, empty commands).

**Implications:**

- Missing file → caller sees OS error (e.g. `os.ErrNotExist`).
- Empty file / `{}` → empty `Battery`; `RunBattery` no-ops successfully with nil results.
- Malformed YAML → wrapped error; no partial tasks.

---

## 6. Deep dive — `RunBattery`

**Path:** `internal/regression/battery.go` (`RunBattery`)

### 6.1 Preconditions

| Input | Behavior |
|-------|----------|
| `b == nil` | `(nil, nil)` |
| `len(b.Tasks) == 0` | `(nil, nil)` |
| otherwise | allocate `results` with capacity `len(Tasks)` |

Note: **function error return is unused for task failures**. Task-level failures are encoded in `Result`. As of current code, `RunBattery` always returns `err == nil` after the nil/empty checks (including partial fail-fast runs). Callers must inspect `Result.Success`.

### 6.2 Per-task algorithm

1. Record `start := time.Now()`.
2. Normalize type: `strings.ToLower(strings.TrimSpace(task.Type))`; empty → `"shell"`.
3. Seed `Result{TaskID: task.ID}`.
4. Switch:
   - **`shell`**: derive timeout; `context.WithTimeout(ctx, timeout)`; `runShell`; set Output/Success/Error; cancel child context.
   - **default**: `Success=false`, `Error=fmt.Sprintf("unsupported task type: %s", task.Type)` (uses original `task.Type` in message, not normalized).
5. `DurationMs = time.Since(start).Milliseconds()`.
6. Append result.
7. If `!res.Success` → **break** (fail-fast).

### 6.3 Fail-fast rationale

Comment in source:

> Fail-fast on first hard failure to keep gauntlet latency bounded.

This is intentional for adversarial/gauntlet latency budgets. Trade-off: later tasks never run; report is partial. There is no flag to collect all failures.

### 6.4 Timeout composition

- Parent `ctx` cancellation aborts the shell subprocess via `exec.CommandContext`.
- Child timeout is min(parent remaining, task timeout) in practice (Go nested contexts).
- Default task timeout 5 minutes can dominate if parent has no deadline.

---

## 7. Deep dive — `runShell`

**Path:** `internal/regression/battery.go` (`runShell`)

### 7.1 Command construction

| OS | Executable | Args | Command delivery |
|----|------------|------|------------------|
| Windows (`runtime.GOOS == "windows"`) | `powershell` | `-NoProfile`, `-Command`, `-` | stdin |
| Else | `bash` | `-l` | stdin |

Notes:

- **Login shell on Unix** (`bash -l`): picks up profile/env; slower; behavior depends on operator shell config.
- **PowerShell `-NoProfile`**: avoids profile side effects; good for determinism; may miss tools only on PATH via profile.
- Command is **not** passed as argv array; entire string is stdin script. Multi-line commands are valid if the shell accepts them.
- `workdir` non-empty → `cmd.Dir = workdir`.

### 7.2 Result mapping

```
out, err := cmd.CombinedOutput()
if ctx.Err() != nil  → return string(out), ctx.Err()   // timeout/cancel preferred
if err != nil        → return string(out), fmt.Errorf("command failed (%s): %w", command, err)
else                 → return string(out), nil
```

Empty command after trim → `fmt.Errorf("empty command")` before spawn.

### 7.3 Security posture (summary)

This is an intentional **local shell executor**. Any process that can write `battery.yaml` and trigger `RunBattery` can run arbitrary shell as the agent user. There is no allowlist, sandbox, or `permitted(...)` gate inside this package. Safety must be enforced by **callers** (workspace trust, constitutional action gating before invoking the harness). See [09-SAFETY-AND-INVARIANTS.md](09-SAFETY-AND-INVARIANTS.md).

---

## 8. Deep dive — `DefaultBatteryPath`

```go
func DefaultBatteryPath(workspace string) string {
    return filepath.Join(workspace, ".nerd", "regression", "battery.yaml")
}
```

- Pure path join; does not create directories or check existence.
- Aligns with codeNERD workspace convention of durable state under `.nerd/`.
- As of verification, **this repo’s own `.nerd/` tree has no `regression/` directory** — no checked-in default battery for the codeNERD workspace itself.

---

## 9. Integration map

### 9.1 Upstream (imports)

| Dependency | Use |
|------------|-----|
| `context` | Parent + per-task timeout |
| `fmt` | Error wrapping |
| `os` | ReadFile |
| `os/exec` | Shell subprocess |
| `path/filepath` | Default path |
| `runtime` | GOOS shell selection |
| `strings` | Type normalize, trim |
| `time` | Duration + timeouts |
| `gopkg.in/yaml.v3` | Battery parse (`go.mod` → v3.0.1) |

**No** imports of other `codenerd/internal/*` packages.

### 9.2 Downstream (importers)

Grep for `codenerd/internal/regression` across `*.go` (excluding this package): **zero matches**.

| Expected consumer (docs/comments) | Actual import |
|-----------------------------------|---------------|
| Nemesis gauntlet | none |
| Campaign assault | none (`cmd/nerd/chat/campaign_assault.go` is independent) |
| CLI | none |
| VirtualStore action | none |
| Shard | none |

This is a textbook **wiring gap**: implementation exists, product path does not call it. Per repo contract, prefer wiring over deletion.

### 9.3 Adjacent systems (conceptual, not code-linked)

| System | Relationship |
|--------|--------------|
| `internal/campaign` assault | Parallel idea: staged shell/`go test`/`go vet` gates. Assault has its own runner; does not call regression. |
| Nemesis / Thunderdome | Package comment names them; no shared types. |
| `internal/testing` | Different concern (test helpers/harnesses), no import either way. |
| `.nerd/` workspace | Canonical battery path lives under `.nerd/regression/`. |
| Constitutional policy | Shell execution would need `permitted(...)` at the **action** layer if exposed as an agent verb — not implemented here. |

---

## 10. Testing surface

| Test | Proves |
|------|--------|
| `TestLoadBattery` | YAML round-trip for version + one shell task |
| `TestRunBatterySuccess` | Shell task succeeds; output contains `ok` |
| `TestRunBatteryUnsupportedTask` | Unknown type fails; second task not run (fail-fast) |
| `TestRunBatteryEmpty` | Empty battery → nil results, nil error |
| `TestDefaultBatteryPath` | Path contains `.nerd` and `battery.yaml` |

Gaps: timeout behavior, empty command, workdir, Windows vs Unix matrix, cancel parent ctx, LoadBattery missing file / bad YAML, multi-task all-success, version ignored, concurrent call safety (N/A but undocumented).

Commands:

```powershell
go test ./internal/regression/...
go test -race ./internal/regression/...
go test -v ./internal/regression/ -count=1
```

---

## 11. Observability

**None inside package.** No:

- structured logger / `internal/logging` category
- metrics counters
- debug dump path
- result file writer

Callers must log `[]Result` themselves. Duration is available per task in `DurationMs`.

---

## 12. Gaps pointer

See [03-GAP-ANALYSIS.md](03-GAP-ANALYSIS.md) for the full matrix. Headline gaps:

1. **No production importer** (integration dead package).
2. **Shell-only** task model (no `go test` typed task, no HTTP probe, no Mangle check).
3. **Exit-code-only** success (no golden stdout).
4. **Fail-fast only** (no full-suite report mode).
5. **No CLI / VirtualStore / Mangle bridge**.
6. **No observability**.
7. **Security delegated entirely to callers**.

Non-gaps (do not “fix”):

- Being outside OODA fact-flow is correct for a side harness.
- Small surface area is intentional (“lightweight, optional”).
- Ignoring `Version` is acceptable until a v2 schema ships.

---

## 13. North-star checklist (package-local)

| Principle | Assessment |
|-----------|------------|
| LLM creative / logic executive | Package is pure deterministic executive; no LLM. **Aligned.** |
| Constitutional safety `permitted(...)` | Not applied here; shell is unrestricted if called. **Gap if exposed as agent action.** |
| JIT prompt atoms | N/A (no LLM surface). |
| Wiring audit before “unused” | Code is unused from product paths — **wire or document**, do not silent-delete. |

---

## 14. Non-goals of this corpus

- Implementing wiring (docs-only rebuild).
- Designing full Nemesis armory schema.
- Replacing campaign assault.
- Inventing CLI flags not in source.

---

## 15. Maintenance notes

When changing this package:

1. Keep fail-fast semantics or introduce an explicit option (do not silently change latency properties relied on by future gauntlets).
2. Any new task type needs a case in `RunBattery` + tests.
3. If adding agent-callable execution, route through VirtualStore + `permitted(...)`.
4. Re-run `go test ./internal/regression/...` and update this corpus’s last-verified date.
