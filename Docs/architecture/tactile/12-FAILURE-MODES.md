# tactile — Failure Modes

> Last verified: **2026-07-13**

## Classification key

| Class | Meaning |
|-------|---------|
| Infra | Executor could not run the process/container correctly |
| Command | Process ran; non-zero exit or logical fail |
| Safety | Isolation weaker than intended |
| Resource | Limits/timeouts/output caps hit |
| Integration | Wiring/kernel/fact path issues |

---

## FM1 — Binary not found / not executable

| | |
|--|--|
| **Class** | Infra |
| **Symptom** | `Success=false`, Error message from os/exec; audit `execution_error` |
| **Cause** | PATH incomplete due to env allowlist; wrong Binary |
| **Mitigation** | Ensure PATH in AllowedEnvironment; use absolute binary paths for tools |

## FM2 — Timeout kill

| | |
|--|--|
| **Class** | Resource |
| **Symptom** | `Killed=true`, KillReason timeout, Success=true |
| **Cause** | TimeoutMs / DefaultTimeout exceeded |
| **Mitigation** | Raise limits for long tests; campaign configs; distinguish from crash in policy |

## FM3 — Output truncation

| | |
|--|--|
| **Class** | Resource |
| **Symptom** | `Truncated=true`, TruncatedBytes > 0; partial stdout |
| **Cause** | MaxOutputBytes (default 10MB) |
| **Mitigation** | Increase limit; write heavy output to files and read via FileEditor |

## FM4 — Docker unavailable

| | |
|--|--|
| **Class** | Infra |
| **Symptom** | Docker Validate error; factory CreateDocker error; Composite without docker mode |
| **Cause** | No binary, daemon down, version command fail |
| **Mitigation** | detectDocker logs warn; fall back to direct only if mode none; don’t request docker mode |

## FM5 — Requested sandbox silently not applied

| | |
|--|--|
| **Class** | Safety |
| **Symptom** | Command asks namespace/firejail/docker; runs under Direct default |
| **Cause** | Composite selectExecutor falls back to defaultExecutor when mode missing |
| **Mitigation** | Gap G-P1-1/2 — prefer fail closed; RegisterExecutor for available modes; check SandboxUsed |

## FM6 — Non-zero exit misread as infra failure

| | |
|--|--|
| **Class** | Integration |
| **Symptom** | Caller aborts pipeline on `Success` only, missing exit code |
| **Cause** | Misunderstanding Success semantics |
| **Mitigation** | Use `ExitCode`, `IsNonZeroExit`, facts `execution_nonzero` |

## FM7 — cgroup setup failure

| | |
|--|--|
| **Class** | Safety / Infra |
| **Symptom** | LimitedExecutorLinux falls back to Direct without cgroup limits |
| **Cause** | Permission denied on cgroupfs |
| **Mitigation** | Run with privileges, use Docker limits, or accept host limits-only timeout |

## FM8 — Job object assign failure (Windows)

| | |
|--|--|
| **Class** | Resource |
| **Symptom** | Limited path may fail or degrade when AssignProcess fails |
| **Cause** | Access rights, process already in another job (Windows nested job rules) |
| **Mitigation** | Tests cover happy path; production may use Direct without jobs when limits nil |

## FM9 — Persistent container orphan

| | |
|--|--|
| **Class** | Infra |
| **Symptom** | docker ps shows `codenerd.managed` containers after crash |
| **Cause** | Process exit without Cleanup/Remove; health loop stopped |
| **Mitigation** | Always defer Cleanup; label-based janitor script; idle reaper (config exists, behavior partial) |

## FM10 — Fact inject failure

| | |
|--|--|
| **Class** | Integration |
| **Symptom** | VirtualStore logs “Failed to inject tactile fact” |
| **Cause** | Missing Decl, wrong arity, kernel not set |
| **Mitigation** | Align ToFacts with schemas_*.mg; ensure SetKernel before execute |

## FM11 — Env secrets not available to tool

| | |
|--|--|
| **Class** | Command / Infra |
| **Symptom** | Tool fails missing API keys |
| **Cause** | Not in AllowedEnvironment and not passed in Command.Environment |
| **Mitigation** | Explicitly pass required env on Command after policy approval |

## FM12 — RetryExecutor thrash

| | |
|--|--|
| **Class** | Infra |
| **Symptom** | High CPU during retries; no real wait |
| **Cause** | Delay loop not using wall clock sleep |
| **Mitigation** | Fix retry delay; avoid wrapping kill/timeout failures (already non-retry) |

## FM13 — File line index mistakes

| | |
|--|--|
| **Class** | Command |
| **Symptom** | Wrong code edited; empty range |
| **Cause** | 1-based inclusive math; startLine>endLine |
| **Mitigation** | Unit tests; CodeDOM should supply correct ranges |

## FM14 — Network isolation blocks pip/git

| | |
|--|--|
| **Class** | Command |
| **Symptom** | python Setup fails installs |
| **Cause** | NetworkEnabled false or docker network none |
| **Mitigation** | EnvironmentConfig.NetworkEnabled true for setup; re-disable after if desired |

## FM15 — Namespace clone fails

| | |
|--|--|
| **Class** | Infra |
| **Symptom** | Execute error under SandboxNamespace |
| **Cause** | Kernel disallows unprivileged userns; missing caps |
| **Mitigation** | GetPlatformExecutor detection; use Docker; root |

---

## Response playbook (operators)

1. Read `ExecutionResult.Error`, `Killed`, `ExitCode`, `SandboxUsed`.  
2. Check tactile category logs around timestamp.  
3. Confirm whether path used Direct vs Docker vs modern VS executor.  
4. For fact absence, confirm AuditLogger callback + kernel.  
5. For isolation surprises, print Composite Capabilities SupportedSandboxModes.
