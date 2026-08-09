# 12 — Failure Modes: session

> Last verified: 2026-08-09

## 1. Catalog

| ID | Failure | Detection | Mitigation in code | Residual risk |
|----|---------|-----------|--------------------|---------------|
| F1 | Empty user input | Guard in ProcessWithIntent | Error return | Caller must handle |
| F2 | Context cancelled before/during loop | ctx.Err checks | Pair remaining tool IDs with cancellation errors; abort | Earlier tools may have executed |
| F3 | Transducer failure | observe error | Process fails | No silent wrong intent |
| F4 | JIT compile failure | compile err | Baseline prompt | Weaker specialization |
| F5 | ConfigFactory failure | compileConfig | Empty config; tool allowlist fails closed | Prose-only response remains possible |
| F6 | LLM generation failure or nil response | generateResponse / follow-up guards | Fatal to Process | No response |
| F7 | Planning-only on mutation intent | no tool_calls + Mangle requires | Single nudge retry | Retry may still fail |
| F8 | Tool not allowed / missing | isToolAllowed / registries | Error tool result | Model may spin |
| F9 | Safety deny | checkSafety false | Block tool | Correct deny |
| F10 | Nil kernel + gate on | checkSafety | Fail closed | Correct |
| F11 | Payload >100KB | checkSafety | Deny | Large writes need chunking |
| F12 | Exact permitted miss on large write | safe_action query | Warn allow | Depends on safe_action facts |
| F13 | Executive gate preflight block | Preflight error | Block tool | Correct |
| F14 | Post-validate fail | Validate error | Error after side effect? | Side effect may already land; model retries |
| F15 | Tool timeout | context.WithTimeout | Error | Partial external effects |
| F16 | Max tool calls / iterations | counters | Pair budget errors; reduced-tool forced final | Incomplete task may be reported explicitly |
| F16a | Outer deadline approaches during exploration | exploration cutoff | Reserve 5m (or half of short remainder), force final under parent context | Final provider call can still exceed reserve |
| F16b | Piggyback write breaks build/tests | shared post-edit gate | Fail with compiler/test evidence; no native repair channel | Operator/model must repair in a later turn |
| F17 | Client lacks ToolResultsProvider | supportsLoop false | One-shot tools, warn | Model never sees results |
| F18 | Piggyback single-round only | code path | Execute tools, no feedback | Model may invent success |
| F19 | Max subagents | Spawn error | Caller backoff | Orchestrator must handle |
| F20 | Spawn TOCTOU | pendingSpawns | Reservation | Tested |
| F21 | Wait cancel without stop | old bug | Stop on cancel | Fixed |
| F22 | Shared executor history pollution | old bug | CloneForTask | Fixed if callers use JITExecutor |
| F23 | Concurrent /current_intent clobber | task intents | task_intent IDs | Interactive still single-owner |
| F24 | Specialist path traversal | name checks | Reject | Config path safe |
| F25 | Oversized specialist YAML | size check | Reject | DoS avoided |
| F26 | Compression LLM fail | Compress err | Hard trim fallback | Context loss |
| F27 | Persist failure | Store err | Log only | Continuity gap |
| F28 | Mangle update blocked | FilterMangleUpdates | Override envelope | Correct |
| F29 | Panic in tool | depends on registry | Process-level recovery tested for some state conflicts | Not universal recover |
| F30 | Campaign vs Cortex wire drift | dual constructors | Manual audit | Config inconsistency |
| F31 | nerd.md query unavailable during write | three write gates | Fail closed + durable safety audit | Writes pause until kernel recovery |
| F32 | Large pending edit body | byte cap | Store SHA-256 digest metadata above 16 KiB | Policies cannot inspect full oversized body |

## 2. Partial execution semantics

Once a tool executes successfully, a later failure (validate, next tool, LLM follow-up) does **not** automatically roll back filesystem changes. Session is not a transactional FS. Campaign write-set gating (outside session) may add higher-level recovery.

## 3. Operator playbooks

### Tools denied unexpectedly

1. Confirm EnableSafetyGate and kernel health.  
2. Check logs for payload size, empty name, permitted miss.  
3. Verify policy `safe_action` / `permitted` rules for the verb.  
4. Confirm args keys include a known target field.

### Subagent never completes

1. Check SubAgent state via metrics.  
2. Confirm timeout / context cancel.  
3. Check LLM client blocking (tests use blockyLLM).  
4. Ensure WaitForResult context not expired without observing Stop.

### Planning-only complete

1. Confirm policy has `intent_requires_tool_call` for verb.  
2. Look for no-tool-retry warn logs.  
3. Inspect AllowedTools in compiled config.

### Wrong tools available

1. Inspect EffectiveAgentRuntimeConfig.AllowedTools.  
2. Check ConfigFactory / intent verb.  
3. Empty list = unrestricted modular names still safety-gated.

## 4. Recovery expectations

| Layer | Recovery |
|-------|----------|
| Single Process | Error to caller; history may still append depending on path |
| Async task | FAILED state + GetResult error |
| Orchestrator | Retries / replan (campaign) |
| User | Re-issue turn; ClearHistory if contaminated (manual) |

## 5. Related docs

- [09-SAFETY-AND-INVARIANTS.md](09-SAFETY-AND-INVARIANTS.md)  
- [03-GAP-ANALYSIS.md](03-GAP-ANALYSIS.md)  
- [IMPLEMENTED_SPEC.md](IMPLEMENTED_SPEC.md)
