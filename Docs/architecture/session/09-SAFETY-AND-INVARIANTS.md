# 09 — Safety and Invariants: session

> Last verified: 2026-07-13

## 1. Constitutional model

Session enforces constitutional safety at tool-call time, not at LLM sampling time.

```
LLM proposes ToolCall
  → allow-list (config / ouroboros)
  → checkSafety → pending_action → permitted match
  → PreflightDestructiveToolCall (optional gate)
  → execute
  → ValidateInteractiveToolResult (optional gate)
```

North star: **default deny** when the gate is enabled and permission is not proven.

## 2. Invariants

### I1 — Fail closed with gate on

If `EnableSafetyGate && kernel == nil` → deny tool.

Tests: `TestExecutor_SafetyGateFailClosed`, constitutional suite in `executor_test.go`.

### I2 — Empty tool name never becomes `/`

Categorical reject before assert.

### I3 — Args nil marshals as `{}`

Matches policy’s empty payload convention; `null` would desync `permitted`.

### I4 — Oversized payloads denied

`maxPayloadBytes = 100KB`. Truncation would break exact match and hide size.

### I5 — pending_action always retracted

`defer RetractFact` after assert (even on deny path after query).

### I6 — Task intents do not clobber interactive routing

Preset runs use `/task_intent_<n>` and retract on exit.

### I7 — Delegated inline tasks do not pollute session history

`CloneForTask` drops history, session context, persister, injected agent config.

### I8 — Dream mode is isolated

`JITExecutor` forces subagent path when `sessionCtx.DreamMode`.

### I9 — Async cancel stops workers

`WaitForResult` cancellation calls `spawner.Stop(taskID)` to prevent zombie LLM spend.

### I10 — Spawn capacity accounts for in-flight construction

`pendingSpawns` reservation + recheck after config generation.

### I11 — Specialist path traversal blocked

Names with `..` or `/` `\` rejected before filesystem read.

### I12 — Piggyback mangle_updates are filtered

Only allowlisted predicates; blocked atoms get constitutional override on envelope.

## 3. Safety algorithms (reference)

Full narrative: [IMPLEMENTED_SPEC.md §6](IMPLEMENTED_SPEC.md).

`safe_action` fallback: only after pending_action was asserted and exact permitted match failed; still deny unknown verbs.

## 4. Concurrency hazards

| Hazard | Mitigation |
|--------|------------|
| Concurrent Spawn vs max | pendingSpawns + double-check |
| Shared kernel facts | Task-scoped intent IDs; retract pending_action |
| SetSessionContext race on shared executor | CloneForTask for inline tasks |
| CompressMemory holds lock during LLM | Unlock around Compress |
| Concurrent Ouroboros set | Executor mutex |

Kernel-level serialization of concurrent `Assert`/`Query` is a core package concern; session assumes thread-safe Kernel implementation.

## 5. What session does **not** guarantee alone

| Concern | Owner |
|---------|-------|
| Policy Decl and rules | `internal/core/defaults/policy/` |
| Workspace sandbox FS | VirtualStore / tools |
| Network egress policy | tools + config |
| User approval UX | CLI chat |
| Dreamer simulation fidelity | core VirtualStore |

## 6. Security notes

- Specialist configs load from workspace `.nerd/agents/` — treat as trusted workspace content; size-capped.  
- Tool results truncated only when fed back to the model (16KB); full result still returned from modular execute to caller path.  
- Persist goroutine is best-effort; do not store secrets in free-form response persistence without higher-layer scrubbing.

## 7. Testing anchors

- Real kernel: `check_safety_real_kernel_test.go`, `check_safety_write_large_test.go`  
- Boundary: `executor_boundary_test.go`  
- Process safety: `TestExecutor_Process_SafetyGate`, `TestExecutor_SafetyGateFailClosed`  
