# 05 — Internal Architecture: session

> Last verified: 2026-08-09

## 1. Component diagram

```
┌────────────────────────────────────────────────────────────┐
│                        JITExecutor                         │
│  TaskRequest normalize → inline | subagent | async         │
└───────────────┬────────────────────────────┬───────────────┘
                │                            │
                │ CloneForTask               │ Spawn
                ▼                            ▼
         ┌────────────┐              ┌──────────────┐
         │  Executor  │◄─────────────│   Spawner    │
         │  (clean)   │  NewSubAgent │  registry    │
         └─────┬──────┘              └──────┬───────┘
               │                            │
               │                     ┌──────▼───────┐
               │                     │   SubAgent   │
               │                     │ hist+compress│
               │                     └──────┬───────┘
               │                            │ ProcessWithIntent
               ▼                            ▼
         ┌─────────────────────────────────────────┐
         │         Executor.ProcessWithIntent      │
         │  observe → JIT → runToolLoop → respond  │
         └─────────────────────────────────────────┘
```

## 2. Data flow — interactive turn

1. Caller invokes `Executor.Process(ctx, userText)`.  
2. Transducer produces `perception.Intent` with history context.  
3. Kernel asserts `user_intent(/current_intent, ...)`.  
4. Compilation context gathers dream mode + world counts.  
5. JIT emits system prompt; ConfigFactory emits allowed tools.  
6. LLM returns text and/or tool calls.  
7. For each tool: safety → preflight → execute → validate.  
8. Native tool results carry compact remaining-call/round guidance. The loop
   runs to the base MaxToolIterations and may receive bounded deterministic
   extensions only for intent-appropriate progress. Write turns need a durable
   write or focused post-write verification; repeated cycles and read-only
   stalls deny extension.
   The deadline’s reserved final-answer window remains independent. Piggyback
   shares batch accounting but remains single-round.
9. Native and Piggyback write turns pass the shared build/test/critic gate; Piggyback remains single-round and cannot auto-repair a hard-gate failure.
10. Piggyback control packet processed; surface text returned.
11. History + optional persist + taxonomy learning.

## 3. Data flow — delegated task

1. `JITExecutor.ExecuteWithContext(req, sessionCtx, priority)`.  
2. Verb normalized (coder → `/fix`, etc.).  
3. Priority injected into context for LLM scheduler.  
4. Branch: dream or complex intent → SubAgent path; else inline clone.  
5. Preset intent skips observation; task intent ID on kernel.  
6. Same tool loop; result string returned to orchestrator.

## 4. SubAgent state machine

```
        Run(task)
 IDLE ──────────► RUNNING
                    │
          ┌─────────┴─────────┐
          ▼                   ▼
      COMPLETED             FAILED
   (result, err=nil)    (err set)
```

Terminal states are cleaned from Spawner via `Cleanup()`.

## 5. Tool execution state (within Process)

```
generate → [optional no-tool retry] → batch tools
  → (native) append ToolResults → CompleteWithToolResults
  → until no tools | max iter | exploration cutoff | budget
  → at cutoff: remove exploration tools → capability-reduced final under parent deadline
```

## 6. Key internal types

| Type | File | Notes |
|------|------|-------|
| `Executor` | executor.go | Stateful conversation runner |
| `ToolCall` | executor_tools.go | Local name/args for safety |
| `ExecutionResult` | executor.go | Outbound result |
| `Spawner` | spawner.go | Map + pendingSpawns |
| `SubAgent` | subagent.go | Atomic state |
| `JITExecutor` | task_executor.go | results map |
| `TaskRequest` / `TaskResult` | task_executor.go | Public task DTOs |
| `SemanticCompressor` | semantic_compressor.go | Compressor impl |

## 7. Registry dual path

```
executeToolCall
  ├─ tools.Global().Has(name) → Execute
  └─ ouroborosRegistry.GetTool(name) → ExecuteRegisteredTool(JSON)
```

Piggyback catalog lists both; native tool defs currently built from modular registry only (`buildToolDefinitions` iterates AllowedTools against `tools.Global()`). Ouroboros tools are primarily Piggyback/registry execute path.

## 8. Session context sources

| Priority | Source | Use |
|----------|--------|-----|
| 1 | `types.GetSessionContext(ctx)` | Request-scoped, preferred |
| 2 | `Executor.sessionContext` field | Legacy stateful |

Dream mode flips `OperationalMode` to `/dream`.

## 9. Memory architecture (SubAgent)

```
history length > threshold
  → split older | recent
  → Compress(older)
  → [SUMMARY turn] + recent
  → on error: trim to threshold
```

Executor itself only caps at 50 turns; it does not semantic-compress.

## 10. Sequence: constitutional check

```
ToolCall
  → pending_action assert
  → query permitted
  → match action/target/payload ?
       yes → allow
       no  → safe_action(verb)? allow-warn : deny
  → retract pending_action (defer)
```
