# verification — Gap Analysis

> Last verified: **2026-07-13**

## Spec vs reality matrix

| Capability (vision / implied) | Reality | Gap severity | Notes |
|-------------------------------|---------|--------------|-------|
| Quality gate on mutations | **Present** | — | Chat scopes via `/mutation` |
| LLM structured judge | **Present** | Low | JSON + fence strip |
| Fail-closed when judge unavailable | **Fail-open** | **High** | nil client / LLM err → Success |
| Full violation taxonomy in fallback | **Partial** | Medium | `basicQualityCheck` misses many kinds |
| Corrective research (web) | **Absent** | Medium | Specialists only; comment admits removal |
| Corrective docs (Context7) | **Absent** | Medium | Same |
| Corrective tool gen | **Present** | Low | Needs autopoiesis wire |
| Corrective decompose | **Present** | Low | Hint only (no auto multi-step) |
| Intelligent shard switch | **Present** | Low | LLM + heuristic |
| Persist attempts | **Present** | Low | store write |
| Learn from history | **Absent** | **High** | no read-back in package |
| Prompt atoms / JIT | **Absent** | **High** for repo contract | Inline strings |
| Mangle quality predicates | **Absent** | Low (optional) | May stay Go-side forever |
| Stable escalation error handling | **Partial** | Medium | chat uses string compare |
| Concurrent multi-task verify | **Unspecified** | Low | single-thread assumption |
| Glass-box events for verify steps | **Absent** | Medium | spawn path has glass box; verify loop less so |
| Configurable maxRetries | **API yes, chat hardcodes 3** | Low | `VerifyWithRetry(..., 3)` |
| Hash of *result* for dedup | **Hashes task** | Low | column name misleading |
| Interface for mocking in chat tests | **Concrete type** | Low | tests often nil verifier |

## Priority backlog (from gaps)

### P0 — correctness / safety of the gate

1. **Decide fail-open vs fail-closed policy** and document + implement config (e.g. strict mode).  
2. **Use `errors.Is` for `ErrMaxRetriesExceeded`** in `cmd/nerd/chat/process.go`.  

### P1 — executive effectiveness

3. Wire research/docs correctives to the live JIT/research path or delete dead Corrective types.  
4. Read `GetQualityViolationStats` / history into heuristic selection.  
5. Expand `basicQualityCheck` or explicitly mark remaining types as “LLM-only.”  

### P2 — platform contract

6. Extract verify/select system prompts into `internal/prompt/atoms/…` with selection.  
7. Emit glass-box / observability events for attempt N, violations, shard switch.  
8. Rename or dual-write `result_hash` to reflect task hash (or hash both).  

### P3 — ergonomics

9. Configurable retry count from config, not magic 3.  
10. Optional confidence floor (e.g. Success but Confidence < 0.5 still retry).  

## Explicit non-gaps

| Topic | Why not a gap |
|-------|----------------|
| No local `.mg` | Quality judgment is fuzzy/LLM-shaped; Mangle not required |
| Not verifying all queries | Intentional latency policy |
| Single source file | Acceptable for ~800 LOC cohesive loop |
| No VirtualStore action type | Verifier is a chat/session *wrapper*, not an effect verb |
| Review-aware prompts | Already implemented |

## Dependency gaps (wiring, not missing files)

| Dependency | Required for | If nil |
|------------|--------------|--------|
| LLM client | Real verification | Soft success 0.5 |
| TaskExecutor *or* ShardManager | Execution | Hard error |
| LocalStore | Learning persistence | Silent no-op store |
| Autopoiesis | Tool corrective | Empty context for tool type |
| ShardManager | Specialist list / fallback spawn | Weaker selection; spawn may still work via executor |

## Regression risks

| Change | Risk |
|--------|------|
| Making fail-closed default | Offline/dev UX breakage |
| Removing normalizeIntentVerb | Retry path dies on bare names again |
| Tightening isReviewTask | Implementation tasks with “review” in prose misclassified |
| Expanding basicQualityCheck “mock” matching | False positives on legitimate mock test doubles |
