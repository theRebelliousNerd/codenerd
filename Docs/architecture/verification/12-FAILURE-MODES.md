# verification — Failure Modes

> Last verified: **2026-07-13**

## Catalog

### F1 — No executor available

| | |
|--|--|
| **Trigger** | `taskExecutor == nil` and `shardMgr == nil` |
| **Symptom** | `VerifyWithRetry` returns error `"no executor available..."` |
| **Mitigation** | Boot always `SetTaskExecutor`; keep shardMgr as backup |
| **Tests** | `TestSpawnTask_WhenNoExecutor`, `TestVerifyWithRetry_WhenNoExecutor` |

### F2 — Shard / task execution failure

| | |
|--|--|
| **Trigger** | TaskExecutor.Execute or Spawn returns error |
| **Symptom** | Immediate return `shard execution failed: ...`; no quality retry |
| **Mitigation** | Fix routing/intent; normalize verbs; executor health |
| **Note** | Distinct from quality failure |

### F3 — Judge LLM unavailable (fail-open)

| | |
|--|--|
| **Trigger** | `client == nil` or `CompleteWithSystem` error |
| **Symptom** | Soft Success (0.5 or 0.3); may accept bad code |
| **Mitigation** | Ensure LLM client at boot; consider strict mode (gap) |
| **Tests** | nil client path unit-tested; transport error soft path less so |

### F4 — Judge returns non-JSON

| | |
|--|--|
| **Trigger** | Model prose instead of JSON |
| **Symptom** | Falls back to `basicQualityCheck` |
| **Mitigation** | Fence stripping; prompt “JSON only”; heuristic catches TODOs/mocks |
| **Risk** | Hallucinated APIs / empty functions may pass heuristics |

### F5 — Quality failures exhaust retries

| | |
|--|--|
| **Trigger** | Repeated Success=false or residual violations |
| **Symptom** | `ErrMaxRetriesExceeded`; chat escalation UI |
| **Mitigation** | Enrichment + shard switch; user re-specifies task |
| **Chat risk** | String compare on error text instead of `errors.Is` |

### F6 — Bare shard name on retry (historical)

| | |
|--|--|
| **Trigger** | Selection LLM returns `world_model_ingestor` without `/` |
| **Symptom** | TaskExecutor rejects intent (before normalize fix) |
| **Mitigation** | `normalizeIntentVerb` → `/consult/<name>` or persona map |
| **Tests** | `TestNormalizeIntentVerb` |

### F7 — Corrective research/docs no-op

| | |
|--|--|
| **Trigger** | Corrective type research/docs, no matching specialist |
| **Symptom** | Empty additional context; weak retry |
| **Mitigation** | Prefer ShardHint specialists; re-wire JIT research (gap) |

### F8 — Tool generation fails

| | |
|--|--|
| **Trigger** | Autopoiesis nil or `GenerateTool` error |
| **Symptom** | Empty corrective context for tool type |
| **Mitigation** | Ensure autopoiesis orch at boot (chat does) |

### F9 — Persistence failure

| | |
|--|--|
| **Trigger** | LocalStore closed, SQL error, marshal error |
| **Symptom** | Warn/error logs; verification loop continues |
| **Mitigation** | Store health; disk free space |

### F10 — False positive quality flags

| | |
|--|--|
| **Trigger** | Legitimate code contains “mock”, “TODO in comment about prior art”, “stub package” |
| **Symptom** | Unnecessary retries or escalation |
| **Mitigation** | LLM judge context; heuristic is blunt; review mode for analyses |

### F11 — False negative quality flags

| | |
|--|--|
| **Trigger** | Subtle incomplete logic without keyword markers; fail-open |
| **Symptom** | Bad work accepted with ✅ Passed |
| **Mitigation** | Better judge prompts; strict mode; secondary static checks |

### F12 — Review misclassification

| | |
|--|--|
| **Trigger** | Implementation task text contains “review” substring rules |
| **Symptom** | Implementation prompt skipped; incomplete code may be “ok” if framed as review |
| **Mitigation** | Keyword list care; prefer intent category over text (open) |

### F13 — Truncation hides violations

| | |
|--|--|
| **Trigger** | Result > 8000 chars with violations only in tail |
| **Symptom** | Judge never sees the bad tail |
| **Mitigation** | Truncate smarter (head+tail); raise limit carefully |

### F14 — Session context not injected on verify spawns

| | |
|--|--|
| **Trigger** | `spawnTask` uses `Execute` without SessionContext |
| **Symptom** | Retry shards miss blackboard/session injection available on non-verify path |
| **Mitigation** | Use `ExecuteWithContext` (gap) |

### F15 — Concurrent VerifyWithRetry on shared instance

| | |
|--|--|
| **Trigger** | Parallel mutations sharing one verifier |
| **Symptom** | Data races on loop locals / session fields |
| **Mitigation** | One call at a time per instance; or add locking |

## Severity matrix

| ID | Severity | Likelihood (prod chat) |
|----|----------|------------------------|
| F1 | High if boot broken | Low |
| F2 | High | Medium |
| F3 | **High** (silent quality hole) | Medium |
| F4 | Medium | Medium |
| F5 | Medium (user visible) | Medium |
| F6 | High historically | Low now |
| F7 | Medium | Medium |
| F8 | Low–Medium | Low |
| F9 | Low | Low |
| F10 | Medium | Medium |
| F11 | High impact | Medium |
| F12 | Medium | Low–Medium |
| F13 | Medium | Low |
| F14 | Medium | High if session context matters |
| F15 | High if concurrent | Low today |
