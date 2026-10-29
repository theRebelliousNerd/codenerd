# verification — Internal Architecture

> Last verified: **2026-07-13**

## Components

```
┌─────────────────────────────────────────────────────────────────┐
│                         TaskVerifier                            │
│  deps: LLMClient, LocalStore, ShardManager, TaskExecutor,       │
│        Autopoiesis Orchestrator, sessionID/turnCount            │
├─────────────────────────────────────────────────────────────────┤
│  VerifyWithRetry ──► spawnTask ──► verifyTask                   │
│        │                  │              │                      │
│        │                  │              ├─ isReviewTask        │
│        │                  │              ├─ CompleteWithSystem  │
│        │                  │              ├─ parseVerification…  │
│        │                  │              └─ basicQualityCheck   │
│        │                  │                                     │
│        │             normalizeIntentVerb                        │
│        │             TaskExecutor | ShardManager.Spawn          │
│        │                                                        │
│        ├─ storeVerification                                     │
│        ├─ selectBestShard ──► parse | heuristicShardSelection   │
│        ├─ applyCorrectiveAction ──► findMatchingSpecialist      │
│        │                         └─ GenerateTool / decompose    │
│        └─ enrichTaskWithContext                                 │
└─────────────────────────────────────────────────────────────────┘
```

## Key types (data model)

### `VerificationResult`

| Field | Meaning |
|-------|---------|
| `Success` | Judge believes task completed properly |
| `Confidence` | 0.0–1.0 self-score |
| `Reason` | Human-readable explanation |
| `Suggestions` | Improvement hints |
| `Evidence` | Concrete snippets/lines |
| `QualityViolations` | Enum list |
| `CorrectiveAction` | Optional next-step prescription |

Loop success requires `Success && len(QualityViolations)==0`.

### `CorrectiveAction`

| Field | Meaning |
|-------|---------|
| `Type` | research \| docs \| tool \| decompose |
| `Query` | What to look up / tool name / decomposition focus |
| `Reason` | Why this helps |
| `ShardHint` | Preferred specialist name |

### `ShardSelectionResult`

| Field | Meaning |
|-------|---------|
| `ShardType` | Next intent/shard string (often becomes spawn intent) |
| `ShardName` | Mirrors selected name when parsed from LLM |
| `Reason` / `Confidence` / `Alternatives` | Explainability |

Note: `parseShardSelection` maps JSON `selected_shard` into **both** `ShardType` and `ShardName`. Category field `shard_type` from the LLM is **parsed but not stored** on the result struct.

## State machine (per VerifyWithRetry call)

```
                     ┌──────────┐
                     │  START   │
                     └────┬─────┘
                          ▼
                   ┌──────────────┐
          ┌───────►│   EXECUTE    │◄──────────────────┐
          │        └──────┬───────┘                   │
          │               │ spawn err → FAIL_HARD     │
          │               ▼                           │
          │        ┌──────────────┐                   │
          │        │   VERIFY     │                   │
          │        └──────┬───────┘                   │
          │               │                           │
          │     ┌─────────┴──────────┐                │
          │     ▼                    ▼                │
          │  PASS                 FAIL                │
          │     │                    │                │
          │     ▼                    ▼                │
          │  STORE ok          STORE fail             │
          │     │                    │                │
          │     ▼              last attempt?          │
          │   SUCCESS ──yes──► ESCALATE               │
          │                    │ no                   │
          │                    ▼                      │
          │              SELECT SHARD                 │
          │                    │                      │
          │                    ▼                      │
          │              APPLY CORRECTIVE             │
          │                    │                      │
          │                    ▼                      │
          │              ENRICH TASK ─────────────────┘
          │
          └──── (attempt++)
```

States are logical — not a formal state machine type in code.

## Data flow — one successful mutation

1. Chat builds `task` string (`formatShardTaskWithContext`)  
2. `SetSessionContext(sessionID, turn)`  
3. Spawn produces `result` string  
4. Judge returns `VerificationResult{Success:true, Violations:[]}`  
5. `storeVerification(..., true)`  
6. Return `(result, verification, nil)`  
7. Chat: ResultToFacts → kernel; formatVerifiedResponse  

## Data flow — fail then recover

1. Spawn result contains `// TODO`  
2. Judge or basic check → Success=false, violations include `placeholder`  
3. Store failure; select may switch to specialist or `/research`  
4. Corrective may inject specialist markdown  
5. Enriched task includes “Previous Attempt Failed” + IMPORTANT anti-mock banner  
6. Second spawn succeeds; store success; return  

## Truncation policy

| Helper | Limit | Used for |
|--------|------:|----------|
| `truncateForVerification` | 8000 | Result fed to judge LLM |
| `truncateContext` | caller (often 2000) | Specialist/corrective context injection |

Suffix: `"\n... [truncated]"`.

## Specialist matching algorithm

1. Require `shardMgr != nil`  
2. `ListAvailableShards()`  
3. If hint equals a specialist name (case-insensitive) → return  
4. Else for each specialist: name substring of query/hint, or tech keyword map hit  
5. Else empty string  

Tech map keys are **shard name lower** keys (`rod`, `golang`, …) matched against keyword lists — not free-form embedding search.

## Prompt strategy (current)

Two large system prompts in `verifyTask` (review vs impl) and one in `selectBestShard`. All demand JSON-only responses. This is the main JIT-atom extraction candidate.

## Error taxonomy

| Condition | Return shape |
|-----------|--------------|
| No executor/manager | `("", nil, err)` hard |
| Spawn failure | `("", nil, fmt.Errorf("shard execution failed: %w", err))` |
| Verify LLM error | Loop continues with synthetic soft success 0.3 |
| Parse error | `basicQualityCheck` result |
| Exhausted retries | `(lastResult, lastVerification, ErrMaxRetriesExceeded)` |
| Store failure | Logged; loop continues |
