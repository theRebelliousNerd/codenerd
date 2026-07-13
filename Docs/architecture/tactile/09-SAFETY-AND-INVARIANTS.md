# tactile — Safety and Invariants

> Last verified: **2026-07-13**

## Layered safety model

```
┌─────────────────────────────────────┐
│  Mangle permitted(...) default-deny │  ← constitutional
├─────────────────────────────────────┤
│  VirtualStore action routing        │  ← policy gate
├─────────────────────────────────────┤
│  tactile sandbox / limits / env     │  ← containment
├─────────────────────────────────────┤
│  OS / Docker / Job / cgroup         │  ← kernel enforcement
└─────────────────────────────────────┘
```

Tactile owns only the **bottom two** rings. It must not pretend to own constitutional policy.

## Invariants

### I1 — No full environment by default

`DirectExecutor.buildEnvironment` only includes keys in `AllowedEnvironment` plus explicit command env.  
**Invariant:** accidental leakage of `AWS_*`, tokens, etc. from ambient shell should not occur unless allowlisted or passed in.

### I2 — Output and time are bounded

Defaults: 30s timeout, 10MB output, 10m max timeout cap via Merge.  
**Invariant:** callers can override but Merge caps timeout at MaxTimeout.

### I3 — Docker network default none

`buildDockerArgs` chooses `none` unless NetworkMode set or NetworkAllowed true → bridge.  
**Invariant:** hermetic default for untrusted commands.

### I4 — Success semantics

`Success=false` only for infrastructure failure (or explicit error path). Non-zero exit remains Success=true.  
**Invariant:** policy/tests must not treat Success as “tests passed”.

### I5 — Sandbox mode validation per executor

Direct rejects non-none sandbox; Docker requires docker mode; etc.  
**Tension:** Composite may fall back to default Direct if mode unregistered — **violates ideal fail-closed** (known gap).

### I6 — Cycle-free Fact type

Tactile Fact is independent of core.Fact. Conversion only at VS boundary.

### I7 — Concurrent callback safety

Audit/file callbacks copied under RLock before invoke; metrics use locks.  
**Invariant:** SetCallback during execute should not race map iteration (composite propagates under lock).

### I8 — Process tree kill best-effort

Unix: kill process group. Windows: taskkill /T or Job terminate.  
**Invariant:** timeout path attempts children cleanup; not guaranteed for breakaway processes without job object assignment.

### I9 — File path resolution

Relative paths join WorkingDir. Absolute paths used as-is.  
**No** chroot in FileEditor — host FS trust boundary is caller/workspace policy.

### I10 — Privileged isolation features degrade

cgroup write requires permissions; namespaces may need root/userns; firejail must exist.  
**Invariant:** detection fails soft to weaker executor rather than panic (good availability, weaker security — document at call site).

## Constitutional relationship

| Concern | Owner |
|---------|-------|
| May agent run `rm -rf`? | Mangle + VS |
| How is command sandboxed if allowed? | tactile SandboxConfig |
| Did it finish / exit code? | ExecutionResult + facts |
| Did file change hash? | FileEditor facts |

## Concurrency hazards

| Hazard | Mitigation / residual risk |
|--------|----------------------------|
| Shared Composite map | RWMutex |
| PersistentDocker health loop | stopChan; Stop closes once |
| RetryExecutor busy delay | residual CPU spin (gap) |
| Concurrent FileEditor same path | No file lock — last writer wins |

## Mangle Decl invariants

Any new `ToFacts` predicate **must** get `Decl` in `internal/core/defaults` before relying on Assert in production.

Known related decls:

- `execution_started`, `execution_completed` — `schemas_shards.mg`  
- `file_read`, `file_written` — `schemas_codedom.mg`  

Audit full ToFacts catalog against Decl when changing audit.go.

## What tactile will not enforce

- Binary allow/deny lists as constitution  
- Path allowlists for FileEditor (unless caller configures sandbox mounts for Docker)  
- Rate limiting of agent intent  
- Secret scanning of stdout  

Those belong higher (policy, tools, perception).
