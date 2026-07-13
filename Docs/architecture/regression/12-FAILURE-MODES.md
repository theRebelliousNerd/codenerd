# regression — Failure Modes

> Last verified against codebase: **2026-07-13**  
> Concrete modes derived from `battery.go` control flow

---

## FM1 — Battery file missing

| | |
|--|--|
| **Trigger** | `LoadBattery` on non-existent path |
| **Symptom** | `error` from `os.ReadFile` |
| **Library behavior** | Hard error; no partial battery |
| **Mitigation** | Host checks `os.IsNotExist` and treats as “skip suite” or “fail config”; seed template on init |

---

## FM2 — Invalid YAML

| | |
|--|--|
| **Trigger** | Malformed YAML or wrong types |
| **Symptom** | `failed to parse battery YAML: …` |
| **Mitigation** | Validate in editor; keep schema simple; optional future strict schema check |

---

## FM3 — Empty / no-op battery

| | |
|--|--|
| **Trigger** | `tasks: []` or missing tasks |
| **Symptom** | `RunBattery` returns `(nil, nil)` |
| **Risk** | Host interprets as “all green” without running anything |
| **Mitigation** | Host policy: empty suite = error in CI; OK for optional path |

---

## FM4 — Unsupported task type

| | |
|--|--|
| **Trigger** | `type: http` (or anything ≠ shell/empty) |
| **Symptom** | `Result.Error` contains `unsupported task type:`; fail-fast |
| **Mitigation** | Stick to `shell`; extend switch when adding types |

---

## FM5 — Empty command

| | |
|--|--|
| **Trigger** | `command: ""` or whitespace |
| **Symptom** | `empty command` error on Result; fail-fast |
| **Mitigation** | Authoring validation |

---

## FM6 — Shell command non-zero exit

| | |
|--|--|
| **Trigger** | Script fails (`go test` fail, missing binary, etc.) |
| **Symptom** | `command failed (…):` wrapped exec error; Output may hold diagnostics |
| **Mitigation** | Read `Result.Output`; fix environment/command |

---

## FM7 — Per-task timeout

| | |
|--|--|
| **Trigger** | Command exceeds `timeout_sec` or default 5m |
| **Symptom** | `Result.Error` from `ctx.Err()` (typically deadline exceeded); possible partial Output |
| **Mitigation** | Raise `timeout_sec`; fix hung command; ensure process tree dies on cancel (OS-dependent) |

---

## FM8 — Parent context canceled

| | |
|--|--|
| **Trigger** | Host cancels suite ctx |
| **Symptom** | Current task fails; fail-fast stops suite |
| **Mitigation** | Expected for shutdown; log cancellation distinctly if host can detect |

---

## FM9 — Shell binary missing

| | |
|--|--|
| **Trigger** | No `powershell` / `bash` on PATH |
| **Symptom** | exec error on spawn |
| **Mitigation** | Document runtime requirements; container images include shell |

---

## FM10 — Fail-fast hides later failures

| | |
|--|--|
| **Trigger** | Early task fails |
| **Symptom** | Results length &lt; task count; later issues unknown |
| **Mitigation** | Intentional for latency; use report-all mode if added; run with fixed early tasks |

---

## FM11 — Non-deterministic Unix login environment

| | |
|--|--|
| **Trigger** | `bash -l` loads profiles with side effects / PATH differences |
| **Symptom** | Works on one machine, fails on CI |
| **Mitigation** | Prefer absolute tool paths in battery commands; consider future non-login bash flags |

---

## FM12 — PowerShell vs bash command dialect

| | |
|--|--|
| **Trigger** | Battery uses bashisms on Windows or PowerShell-only syntax on Unix |
| **Symptom** | Shell parse/runtime failure |
| **Mitigation** | Author portable commands carefully; or maintain OS-specific batteries / hosts filter tasks |

---

## FM13 — Large output memory

| | |
|--|--|
| **Trigger** | Verbose command floods stdout |
| **Symptom** | High memory; slow CombinedOutput |
| **Mitigation** | Keep battery commands quiet; future streaming/truncation |

---

## FM14 — Product dead-wiring (meta failure)

| | |
|--|--|
| **Trigger** | No importer ever calls the package |
| **Symptom** | Feature appears in tree/docs but never runs in product |
| **Mitigation** | Wire CLI/assault; update comments; periodic wiring audit |

---

## FM15 — Security: malicious battery

| | |
|--|--|
| **Trigger** | Untrusted YAML executed via `RunBattery` |
| **Symptom** | Arbitrary code execution as agent user |
| **Mitigation** | Trust workspace; agent path through `permitted`; do not fetch remote batteries without review |

---

## Summary table

| ID | Class | Fail-fast impact |
|----|-------|------------------|
| FM1–2 | Load | N/A (never runs) |
| FM3 | Vacuous | None run |
| FM4–9 | Task fail | Stops suite |
| FM10 | Semantics | By design |
| FM11–12 | Portability | Task fail likely |
| FM13 | Resource | May timeout/OOM |
| FM14 | Integration | Silent product gap |
| FM15 | Security | Catastrophic if abused |
