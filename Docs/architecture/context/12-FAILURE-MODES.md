# 12 — Failure Modes: Context

> Last verified against codebase: 2026-07-13  
> Package: `internal/context`  
> Status: Living Reference Document

## 1. Failure mode catalog

| ID | Failure | Symptom | Likely cause | Mitigation in code | Operator action |
|----|---------|---------|--------------|--------------------|-----------------|
| F1 | Context window exceeded | `BuildContext` / allocate errors | Total usage > budget | `ErrContextWindowExceeded`; refuse build | Lower history, force compress, raise MaxTokens |
| F2 | Empty high-activation set | LLM lacks file/goal context | Threshold too high / cold intent | Logs above_threshold counts | Check user_intent; lower ActivationThreshold carefully |
| F3 | Go fallback always | Never sees kernel selection | `should_include_context` empty | Fallback path | Ensure policy `context_compilation.mg` loaded |
| F4 | Core safety missing | No constitutional section | Kernel Query fail | Warn per predicate | Inspect kernel program / Decl |
| F5 | Compression never fires | History unbounded growth | Budget undercount / low utilization | ShouldCompress on util | Check TokenCounter estimates; force segments |
| F6 | Compression thrash | Frequent compress with thin segments | Threshold low + large turns | Window keep RecentTurnWindow | Tune threshold / window |
| F7 | Async ProcessTurn lag | Next turn misses last atoms | goroutine delay / timeout | 2m timeout in chat | Ensure session waits if needed |
| F8 | Persist fail silent | Restart loses compressed state | store nil or marshal error | Best-effort `_ = store...` | Check store logs / disk |
| F9 | Feedback DB fail | No learning scores | Path/permissions | Boot warn, continue | Create `.nerd/`, fix perms |
| F10 | Adversarial issue boost | Noise facts dominate | Unclamped weights (historical) | Clamp [0,1] + issue cap 100 | Update issue_keyword producers |
| F11 | Concurrent map panic | Process crash | Races on maps (historical) | Engine mutex + race test | Run `-race` on regressions |
| F12 | Turn age all /recent | Mask rules dead | age used slice length (historical) | age = turnNumber − turn.TurnNumber | Verify assertTurnAgeCategories |
| F13 | Token under-estimate | Provider 400/context error | Heuristic vs real tokenizer | Soft only | Increase reserve slack |
| F14 | LoadState duplicate assert | Noisy kernel | Re-assert hot facts | Skip existing fact strings | Clear kernel if corrupt |
| F15 | Malformed mangle_update | Missing atoms | Bad control packet strings | Skip parse errors | Fix articulation packet |

## 2. Cascading failure: long session without compression

```
ProcessTurn undercounts tokens
  → IsCompressionActive false forever
  → chat injects full history
  → provider rejects / quality collapses
```

**Detect:** budget UI flat; no COMPRESSION TRIGGERED logs; history length huge.  
**Fix:** recalcBudget correctness; lower threshold; inspect OriginalTokens.

## 3. Cascading failure: over-aggressive threshold

```
ActivationThreshold too high
  → almost no atoms pass
  → LLM only has core + empty active context
  → poor tool choices (still constrained by permitted)
```

**Detect:** above_threshold near 0 in ScoreFacts debug logs.  
**Fix:** Tune threshold; ensure intent and focus_resolution present.

## 4. Recovery playbook

1. Enable verbose / CategoryContext debug.  
2. Confirm compressor non-nil on Model.  
3. Check `GetMetrics` / budget UI.  
4. Query kernel for `user_intent`, `should_include_context`.  
5. Inspect `.nerd/context_feedback.db` only if learning suspected.  
6. If kernel dump `debug_program_ERROR.mg` present, fix program load first.

## 5. Non-failures

| Observation | Why OK |
|-------------|--------|
| Feedback score 0 early | minSamples=10 |
| LLM generateSummary unused | C3 simple path by design |
| Default 200k not 128k | Config override at boot expected |
