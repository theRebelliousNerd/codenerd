# regression — Safety and Invariants

> Last verified against codebase: **2026-07-13**  
> Source: `internal/regression/battery.go`

---

## 1. Threat model (package-local)

| Threat | Severity | Notes |
|--------|----------|-------|
| Malicious `battery.yaml` content | **High** | Arbitrary shell as the running user |
| Untrusted workspace path | High | workdir + scripts can touch repo and home (via shell) |
| Agent-triggered run without policy | High | Not currently possible via product path (unwired) |
| Resource exhaustion (long loops) | Medium | Mitigated by timeouts + fail-fast; 5m default still heavy |
| Output memory growth | Medium | Entire CombinedOutput buffered in string |
| Dependency on shell availability | Low | Clear failure if powershell/bash missing |

Trust assumption: **caller + battery author are trusted** at the same level as “run a terminal command in this workspace.”

---

## 2. Invariants

### I1 — Sequential execution

Tasks never run concurrently. No shared mutable package globals.

### I2 — Fail-fast

After the first `Result` with `Success == false`, no further tasks execute.

### I3 — Context-bound subprocesses

Shell tasks use `exec.CommandContext`. Parent cancel/timeout can interrupt the wait.

### I4 — Unknown task types fail

Unsupported types produce a failed `Result`, not a silent skip-success.

### I5 — Empty command fails

`runShell` rejects empty/whitespace-only commands before spawn.

### I6 — Empty suite is a no-op success path

`nil` battery or zero tasks → `(nil, nil)` from `RunBattery` (vacuous pass at library level).

### I7 — Load errors are hard errors

IO/parse failures return from `LoadBattery`; they do not become empty successful batteries.

### I8 — No privilege elevation

Package does not attempt elevation; inherits process identity.

---

## 3. Non-invariants (do not assume)

| Assumption | Reality |
|------------|---------|
| `RunBattery` err ≠ nil on task failure | **False** today — check Results |
| stdout matches expectation | Not checked |
| Task IDs unique | Not enforced |
| Version == 1 | Not checked |
| Sandboxed filesystem | **False** |
| Network disabled | **False** (shell can network) |
| Deterministic Unix env | `bash -l` loads login profile |

---

## 4. Constitutional safety relationship

codeNERD default-deny: actions should derive `permitted(...)`.

| Path | Gate |
|------|------|
| Direct library call from trusted Go | Outside constitutional system (like any `os/exec` helper) |
| Future VS action | **Must** add policy rules before enabling |
| Future CLI | Operator-initiated; still respect workspace trust |

This package will **not** implement Mangle policy itself (keep library pure). Policy belongs in `internal/core/defaults/policy/` and VirtualStore.

---

## 5. Concurrency

- API is safe for concurrent use on **disjoint** batteries/results (no package state).  
- Concurrent `RunBattery` on the same machine may contend for CPU/IO; not synchronized.  
- No mutexes; none needed for current design.

Race tests are low priority (`go test -race` still green as a hygiene check).

---

## 6. Mangle Decl surface

**None.** No `.mg` files in package. No predicates emitted.

If a host wants logic memory:

```text
# illustrative only — not in tree
Decl regression_battery_loaded(Path).
Decl regression_task_result(TaskID, Success, DurationMs).
```

---

## 7. Verify gates

```powershell
go test ./internal/regression/...
go test -race ./internal/regression/...
```

Manual safety review checklist for new task types:

1. Is `ctx` honored?  
2. Is output bounded or streamable?  
3. Can untrusted input reach argv/shell?  
4. Does fail-fast still hold?  
5. Are errors visible on `Result`?
