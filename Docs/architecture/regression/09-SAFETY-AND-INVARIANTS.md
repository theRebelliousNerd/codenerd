# regression — Safety and Invariants

> Last verified against codebase: **2026-08-15**  
> Source: `internal/regression/battery.go`, `internal/regression/policy.go`,
> `internal/core/defaults/policy/regression_battery.mg`,
> `internal/core/defaults/schemas_safety.mg` (SECTION 24)

---

## 1. Threat model (package-local)

| Threat | Severity | Notes |
|--------|----------|-------|
| Malicious `battery.yaml` content | **High** | Arbitrary shell as the running user |
| Untrusted workspace path | High | workdir + scripts can touch repo and home (via shell) |
| Agent-triggered run without policy | **High** | Not possible: no VirtualStore handler routes `/run_regression_battery`, and the action is registered `requires_permission` → `dangerous_action` (default deny). See §4. |
| Battery used to launder a blocked command | **High** | The reason the action is not allowlisted. See §4.1. |
| Resource exhaustion (long loops) | Medium | Mitigated by timeouts + fail-fast; 5m default still heavy |
| Output memory growth | Medium | Entire CombinedOutput buffered in string |
| Dependency on shell availability | Low | Preflighted by `CheckShell`; fails the run with an actionable error naming the missing interpreter |

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

### I6 — Empty suite is a configuration error

`Battery.Validate` rejects a zero-task battery, so `LoadBattery` never returns
one: a suite that reports green because it contains nothing is the worst
possible regression signal. `RunBattery` still returns `(nil, nil)` for a
programmatically-constructed empty battery — the error is raised where a human
authored the file. The Mangle gate agrees:
`regression_battery_permitted` requires `regression_battery_has_task`, so an
empty battery derives `regression_battery_refused(Path, "battery declares no
tasks")` rather than a vacuous permit.

### I7 — Load errors are hard errors

IO/parse failures return from `LoadBattery`; they do not become empty successful batteries.

### I8 — No privilege elevation

Package does not attempt elevation; inherits process identity.

---

## 3. Non-invariants (do not assume)

| Assumption | Reality |
|------------|---------|
| `RunBattery` err ≠ nil on task failure | **False** — check `Results` / `Summary.OK()`. A non-nil error means the harness could not run (bad YAML, missing shell), not that a task failed. |
| stdout matches expectation | Checked **only** when the task declares `expect_contains` / `expect_not_contains` |
| Task IDs unique | Enforced by `Battery.Validate` |
| Version == 1 | Enforced by `Battery.Validate` (0 means "unversioned", still accepted) |
| Sandboxed filesystem | **False** |
| Network disabled | **False** (shell can network) |
| Deterministic Unix env | Default is `bash --noprofile --norc`; `RunOptions.LoginShell` opts back into `bash -l` and its dotfiles |
| A permitted battery is a safe battery | **False.** `regression_battery_permitted` only proves no task matches a `blocked_pattern`. It is a necessary condition, never a sufficient one. |

---

## 4. Constitutional safety relationship

codeNERD default-deny: actions must positively derive `permitted(...)`.

| Path | Gate | Status |
|------|------|--------|
| Direct library call from trusted Go | Outside the constitutional system (like any `os/exec` helper) | in use |
| `nerd regression run` (CLI) | Operator-initiated; workspace trust, no kernel involved | **wired** |
| VirtualStore agent action | `dangerous_action` + battery-content gate | **deliberately not wired — see §4.1** |

### 4.1 Decision: `/run_regression_battery` is not agent-callable

**Decided 2026-08-15. Recorded at the decision point in
`internal/core/defaults/policy/regression_battery.mg`.**

Registering the battery as `safe_action(/run_regression_battery)` would have
been a privilege escalation, not a convenience:

1. `dangerous_content/2` — the constitution's entire content gate, thirty-plus
   rules covering `rm -rf`, `sudo`, `git push --force`, `| bash`, `nc -e` — only
   inspects a `pending_action`'s **Target** and **Payload**.
2. A battery action's Target is a **path**. The commands live inside the file.
3. `safe_action(/write_file)` is already permitted.

So an agent holding an allowlisted battery action could write
`.nerd/regression/battery.yaml` containing any blocked pattern and run it. The
battery would be strictly **more** powerful than `/exec_cmd`, which is the thing
those thirty rules exist to constrain. An action that grants more than the
primitive it wraps does not belong on the allowlist.

What is implemented instead:

- `requires_permission(/run_regression_battery).` — the constitution's
  `dangerous_action(A) :- requires_permission(A).` wiring turns this into
  default deny. `permitted/3` derives only through `signed_approval` **and**
  `admin_override`, and `permission_denied` derives otherwise.
- A **content gate** for any future host, because an override authorizes running
  *a* battery, not laundering a blocked command through one. The host projects
  the battery with `regression.PolicyFacts` — one
  `regression_battery_task(Path, TaskID, Command)` fact per task — and must
  query `regression_battery_permitted(Path)` **in addition to** `permitted/3`.
  Both must hold.
- A host that skips the projection derives nothing and is denied. The failure
  direction is closed.

Ordering is part of the contract: the kernel evaluates incrementally and never
retracts a derived fact, so `PolicyFacts` emits the task facts **before** the
declaration. Asserting the declaration first derives
`regression_battery_refused(Path, "battery declares no tasks")` in that instant
and the later task facts cannot take it back.

This package still implements **no** Mangle policy itself (P9). `PolicyFacts`
projects facts; `internal/core/defaults/policy/regression_battery.mg` decides.

Enforced by `internal/regression/policy_test.go`, which loads the real corpus
files from disk. `TestBatteryPolicy_RunBatteryAction_ShouldBeDangerousAndNotAllowlisted`
fails if anyone adds the action to the `safe_action` list.

---

## 5. Concurrency

- API is safe for concurrent use on **disjoint** batteries/results (no package state).  
- Concurrent `RunBattery` on the same machine may contend for CPU/IO; not synchronized.  
- No mutexes; none needed for current design.

Race tests are low priority (`go test -race` still green as a hygiene check).

---

## 6. Mangle Decl surface

The package still contains no `.mg` files. Its predicates are declared in the
shared constitutional schema, `internal/core/defaults/schemas_safety.mg`
SECTION 24, and every argument is `/string` — a `MangleAtom` in any of these
slots would land as a `NameType` constant and never unify:

```text
Decl regression_battery_declared(BatteryPath) bound [/string].
Decl regression_battery_task(BatteryPath, TaskID, Command) bound [/string, /string, /string].
Decl regression_task_forbidden(TaskID, Pattern) bound [/string, /string].
Decl regression_battery_has_task(BatteryPath) bound [/string].
Decl regression_battery_has_forbidden_task(BatteryPath) bound [/string].
Decl regression_battery_permitted(BatteryPath) bound [/string].
Decl regression_battery_refused(BatteryPath, Reason) bound [/string, /string].
```

The two `has_*` predicates are bound-negation helpers (SECTION 11C): a negated
literal containing an anonymous wildcard excludes nothing in this Mangle build,
so the wildcards are projected away before the negation sees them.

Result facts (`regression_task_result`) are still **not** emitted; runs are
persisted as JSON under `.nerd/regression/runs/` instead.

---

## 7. Verify gates

```powershell
go test ./internal/regression/...
go test -race ./internal/regression/...
```

`policy_test.go` is the constitutional gate's regression suite. It boots a real
Mangle engine over `schemas_safety.mg`, `schemas_shards.mg`,
`schemas_execution.mg`, `policy/constitution.mg` and
`policy/regression_battery.mg`, then proves that a clean battery is permitted,
that one containing `rm -rf /`, `git push --force`, `curl … | bash` or `sudo` is
refused, that an empty or undeclared battery derives nothing, and that the
seeded template passes its own policy.

Manual safety review checklist for new task types:

1. Is `ctx` honored?  
2. Is output bounded or streamable?  
3. Can untrusted input reach argv/shell?  
4. Does fail-fast still hold?  
5. Are errors visible on `Result`?
