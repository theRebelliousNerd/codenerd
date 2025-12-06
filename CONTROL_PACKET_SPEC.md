# Control Packet Specification v1.2.0

**Date:** 2025-12-06
**Status:** Production
**Addresses:** Critical feedback on comprehensive state tracking

---

## Overview

The **ControlPacket** is the hidden "truth channel" of the Piggyback Protocol—what the agent actually believes vs. what it says to the user. This specification defines a robust, comprehensive state tracking system that prevents cognitive dissonance and ensures the agent's internal logic always reflects reality.

---

## Design Principles

1. **Truth Over Polish** - The control packet must reflect the ACTUAL state, even if the surface response is polite
2. **Complete Observability** - Every state transition must be traceable through control packets
3. **Enforcement Ready** - All fields must be machine-readable and enforceable by the kernel
4. **Schema Evolution** - Forward-compatible design allows extending without breaking changes

---

## Full Schema (v1.2.0)

```json
{
  "control_packet": {
    "intent_classification": {
      "category": "/query | /mutation | /instruction",
      "verb": "/explain | /fix | /refactor | /review | ... (see verb taxonomy)",
      "target": "file path, function name, or 'codebase'",
      "constraint": "security only | without tests | none",
      "confidence": 0.95
    },

    "mangle_updates": [
      "user_intent(/fix, \"auth.go\")",
      "task_status(/auth_fix, /in_progress)",
      "file_state(\"auth.go\", /read)",
      "diagnostic(/error, \"auth.go\", 42, \"E001\", \"fixed\")"
    ],

    "memory_operations": [
      {
        "op": "promote_to_long_term",
        "key": "preference:code_style",
        "value": "concise"
      },
      {
        "op": "store_vector",
        "key": "pattern:auth_fix_20251206",
        "value": "User prefers Bearer token validation over session cookies"
      },
      {
        "op": "forget",
        "key": "temp:old_session_id",
        "value": ""
      }
    ],

    "self_correction": {
      "triggered": true,
      "hypothesis": "Previous edit failed due to syntax error at line 45",
      "recovery_action": "/reparse_file",
      "confidence": 0.85
    },

    "reasoning_trace": "User requested auth fix. Analyzed auth.go lines 40-50. Identified missing Bearer token check. Proposed patch at line 42.",

    "execution_metadata": {
      "shard_type": "coder",
      "execution_time_ms": 2340,
      "tokens_used": 1523,
      "retry_count": 0,
      "blocked_by_constitution": false
    },

    "state_transitions": [
      {
        "from": "task_status(/auth_fix, /pending)",
        "to": "task_status(/auth_fix, /in_progress)",
        "timestamp": "2025-12-06T10:15:23Z",
        "reason": "Shard spawned"
      },
      {
        "from": "file_state(\"auth.go\", /clean)",
        "to": "file_state(\"auth.go\", /modified)",
        "timestamp": "2025-12-06T10:15:45Z",
        "reason": "Applied patch at line 42"
      }
    ],

    "impact_analysis": {
      "files_affected": ["auth.go", "auth_test.go"],
      "dependencies_broken": [],
      "tests_impacted": ["TestAuthValidation", "TestBearerToken"],
      "estimated_scope": "minor"
    },

    "safety_gates": {
      "requires_user_approval": false,
      "chesterton_fence_warning": null,
      "dangerous_action_blocked": null,
      "constitutional_override": null
    },

    "learning_signals": {
      "pattern_id": "auth_fix_bearer_token",
      "success": true,
      "user_accepted": null,
      "rejection_reason": null,
      "should_promote_to_learned": true
    }
  },

  "surface_response": "I've fixed the authentication bug in auth.go by adding Bearer token validation at line 42."
}
```

---

## Field-by-Field Specification

### 1. `intent_classification` (REQUIRED)

Maps natural language to structured intent.

- **category**: `/query`, `/mutation`, `/instruction`
- **verb**: One of 50+ verbs from the taxonomy (see [internal/perception/transducer.go](internal/perception/transducer.go#L52-L628))
- **target**: Resolved file path, function name, or `codebase` for broad requests
- **constraint**: Filters like `security only`, `go files`, `without tests`, or `none`
- **confidence**: 0.0-1.0 confidence score

**Purpose:** Enables logic-driven task routing and shard delegation.

---

### 2. `mangle_updates` (REQUIRED)

List of Mangle atoms representing state changes.

**Common Patterns:**
```mangle
user_intent(/verb, "target")
task_status(TaskID, /pending | /in_progress | /complete | /failed)
file_state("path", /clean | /read | /modified | /deleted)
diagnostic(/error | /warning, "file", line, "code", "message")
shard_state(ShardID, /spawned | /executing | /completed | /failed)
```

**Purpose:** The "state diff" that updates the Mangle kernel.

---

### 3. `memory_operations` (REQUIRED)

Directives to the 4-tier memory system.

**Operation Types:**
- `promote_to_long_term`: Move fact from RAM to SQLite (persistent preferences, learned patterns)
- `store_vector`: Add to vector DB for semantic search
- `forget`: Remove outdated/incorrect fact
- `note`: Temporary session-scoped annotation

**Schema:**
```json
{
  "op": "promote_to_long_term",
  "key": "preference:naming_style",
  "value": "camelCase for variables, PascalCase for types"
}
```

**Purpose:** Enables learning without retraining.

---

### 4. `self_correction` (OPTIONAL)

Triggered when the agent detects its own error.

**Fields:**
- `triggered`: boolean - Was self-correction activated?
- `hypothesis`: string - What went wrong?
- `recovery_action`: string - What action should fix it?
- `confidence`: 0.0-1.0

**Example:**
```json
{
  "triggered": true,
  "hypothesis": "Test failed because I used wrong import path",
  "recovery_action": "/fix_imports",
  "confidence": 0.9
}
```

**Purpose:** Abductive reasoning and automatic error recovery.

---

### 5. `reasoning_trace` (OPTIONAL)

Internal "scratchpad" for the agent's thought process.

**Example:**
```
"Analyzed 3 approaches: (1) refactor function, (2) extract method, (3) add caching. Chose (2) because it reduces complexity without changing API surface."
```

**Purpose:** Debugging, transparency, explainability.

---

### 6. `execution_metadata` (RECOMMENDED)

Runtime statistics and execution context.

**Fields:**
- `shard_type`: "coder" | "tester" | "reviewer" | "researcher" | "main"
- `execution_time_ms`: Actual execution time
- `tokens_used`: LLM tokens consumed
- `retry_count`: How many retries occurred
- `blocked_by_constitution`: Was any action blocked by safety rules?

**Purpose:** Performance monitoring, cost tracking, safety auditing.

---

### 7. `state_transitions` (RECOMMENDED)

Explicit before/after state changes.

**Schema:**
```json
{
  "from": "task_status(/task123, /pending)",
  "to": "task_status(/task123, /in_progress)",
  "timestamp": "2025-12-06T10:15:23Z",
  "reason": "CoderShard spawned"
}
```

**Purpose:** Audit trail, rollback capability, debugging.

---

### 8. `impact_analysis` (RECOMMENDED)

What will this action affect?

**Fields:**
- `files_affected`: List of file paths
- `dependencies_broken`: List of dependent files that may break
- `tests_impacted`: List of test functions/files to run
- `estimated_scope`: "trivial" | "minor" | "major" | "critical"

**Purpose:** Risk assessment, test selection, change impact visualization.

---

### 9. `safety_gates` (CRITICAL)

Constitutional logic enforcement points.

**Fields:**
- `requires_user_approval`: boolean - Must user approve this action?
- `chesterton_fence_warning`: string | null - Warning about modifying recently-changed code
- `dangerous_action_blocked`: string | null - What dangerous action was prevented?
- `constitutional_override`: string | null - Why was default behavior overridden?

**Example:**
```json
{
  "requires_user_approval": true,
  "chesterton_fence_warning": "This file was modified by another developer 2 hours ago",
  "dangerous_action_blocked": null,
  "constitutional_override": null
}
```

**Purpose:** Safety enforcement, prevents catastrophic actions.

---

### 10. `learning_signals` (OPTIONAL - Autopoiesis)

Feedback for the learning system.

**Fields:**
- `pattern_id`: Unique ID for this pattern
- `success`: boolean - Did the action succeed?
- `user_accepted`: boolean | null - Did user accept/reject the change?
- `rejection_reason`: string | null
- `should_promote_to_learned`: boolean - Should this become a learned rule?

**Purpose:** Enable runtime learning and skill acquisition.

---

## Enforcement Strategy

### How Config Limits Are Enforced

1. **Kernel Level** ([internal/core/kernel.go](internal/core/kernel.go))
   - `CoreLimits.MaxFactsInKernel` → Enforced in `LoadFacts()`
   - `CoreLimits.MaxDerivedFactsLimit` → Passed to `engine.WithCreatedFactLimit()`

2. **Shard Level** ([internal/core/shard_manager.go](internal/core/shard_manager.go))
   - `ShardProfile.MaxContextTokens` → Enforced when building shard prompts
   - `ShardProfile.MaxExecutionTimeSec` → Enforced via `context.WithTimeout()`
   - `ShardProfile.MaxFactsInShardKernel` → Passed to shard's kernel instance

3. **Context Level** ([internal/context/compressor.go](internal/context/compressor.go))
   - `ContextWindow.MaxTokens` → Budget enforcement in token allocator
   - Reserve percentages → Enforced in `TokenBudget.Allocate()`

### Example Enforcement Code

```go
// In shard_manager.go
func (sm *ShardManager) Spawn(ctx context.Context, shardType string, task string) (string, error) {
    profile := sm.config.GetShardProfile(shardType)

    // Enforce execution timeout
    shardCtx, cancel := context.WithTimeout(ctx,
        time.Duration(profile.MaxExecutionTimeSec)*time.Second)
    defer cancel()

    // Enforce context limits
    if len(task) > profile.MaxContextTokens*4 { // ~4 chars/token
        return "", fmt.Errorf("task exceeds shard context limit")
    }

    // Create shard with enforced fact limit
    shard := NewCoderShard(profile)
    shard.kernel.SetFactLimit(profile.MaxFactsInShardKernel)

    return shard.Execute(shardCtx, task)
}
```

---

## Validation & Error Handling

### Required Field Validation

The articulation layer MUST validate:
1. `intent_classification` is present
2. `mangle_updates` contains valid Mangle syntax (via [internal/mangle/schema_validator.go](internal/mangle/schema_validator.go))
3. All referenced predicates are declared in schemas.gl

### Graceful Degradation

If validation fails:
1. Parse what's parseable
2. Emit warnings in `ArticulationResult.Warnings`
3. Continue with partial control packet
4. Log failure for debugging

**Never block the user** - degrade gracefully.

---

## Configuration Management

### Setting Per-Shard Models

**Via config.json:**
```json
{
  "shard_profiles": {
    "reviewer": {
      "model": "claude-opus-4-20250514",
      "temperature": 0.3,
      "max_context_tokens": 40000
    }
  }
}
```

**Programmatically:**
```go
cfg := config.Load(".nerd/config.json")
cfg.SetShardProfile("reviewer", config.ShardProfile{
    Model: "claude-opus-4-20250514",
    Temperature: 0.3,
    MaxContextTokens: 40000,
})
cfg.Save(".nerd/config.json")
```

### Setting Core Limits

```json
{
  "core_limits": {
    "max_total_memory_mb": 4096,
    "max_concurrent_shards": 8,
    "max_facts_in_kernel": 200000,
    "max_derived_facts_limit": 100000
  }
}
```

---

## Future Extensions (v1.3.0+)

**Planned additions:**
- `dependencies_added`: List of new imports/packages added
- `security_review`: Security impact assessment
- `performance_profile`: Runtime/memory profiling data
- `test_coverage_delta`: How coverage changed
- `git_metadata`: Commit hashes, branch info
- `collaboration_hints`: Multi-user conflict detection

---

## Summary

This comprehensive Control Packet design ensures:

✅ **Complete Observability** - Every state change is tracked
✅ **Safety Enforcement** - Constitutional logic prevents catastrophic actions
✅ **Learning Capability** - Runtime skill acquisition without retraining
✅ **Audit Trail** - Full history for debugging and rollback
✅ **Resource Control** - Config-driven limits are enforced
✅ **Cognitive Integrity** - No dissonance between thought and speech

**Status:** Production-ready, addresses all critical feedback.

---

**Implemented by:** Claude Sonnet 4.5
**Date:** 2025-12-06
**Version:** 1.2.0 (Thought-First + Comprehensive State Tracking)
