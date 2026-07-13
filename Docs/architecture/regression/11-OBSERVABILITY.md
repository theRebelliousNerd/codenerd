# regression — Observability

> Last verified against codebase: **2026-07-13**  
> Source: full read of `internal/regression/` — **no logging or metrics APIs**

---

## 1. Current state

| Signal | Present? |
|--------|----------|
| Structured logger | **No** |
| Log category under `.nerd/logs/` | **No** |
| Metrics / counters | **No** |
| Tracing spans | **No** |
| Debug dump file | **No** |
| Result persistence helper | **No** |
| stdout from library | **No** (output only in `Result.Output`) |

The only built-in observability is the **`Result` struct** returned to the caller:

- `TaskID`
- `Success`
- `Output` (combined process output)
- `Error`
- `DurationMs`

---

## 2. Implications

1. Production hosts that call `RunBattery` without logging will produce **silent** suite runs aside from their own printing.  
2. Fail-fast means later tasks never appear in results — hosts should log “stopped after task X.”  
3. Large `Output` strings can be expensive to log wholesale; hosts should truncate.

---

## 3. Recommended host logging (not implemented)

When wiring CLI or campaign:

```text
category: regression
events:
  - battery_load path=… version=… task_count=…
  - task_start id=… type=… timeout_sec=…
  - task_end id=… success=… duration_ms=… error=…
  - battery_done passed=… failed=… stopped_early=…
```

Align with existing `.nerd/logs/*` category files if using `internal/logging` patterns from other packages.

---

## 4. Recommended artifacts (not implemented)

| Artifact | Purpose |
|----------|---------|
| `.nerd/regression/runs/<ts>.json` | Durable `[]Result` |
| Campaign assault folder entry | Correlate with long-horizon runs |

---

## 5. Glass-box / transparency

No integration with glass-box or transparency packages. Battery runs would need host-side journaling to appear in operator UI.

---

## 6. Debug tips (developer)

1. Unit test with `-v` to see failures.  
2. Construct a minimal battery and print `Result` in a scratch test.  
3. On timeout issues, check nested contexts and default 5m.  
4. On Windows, confirm `powershell` on PATH; on Unix, `bash`.  
5. Remember fail-fast when “later tasks never ran.”

---

## 7. Metrics candidates (future)

| Metric | Type |
|--------|------|
| `regression_tasks_total` | counter (success/fail labels) |
| `regression_task_duration_ms` | histogram |
| `regression_suite_fail_fast_total` | counter |

Not present in code today.
