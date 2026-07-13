# 09 — Safety and Invariants: Context

> Last verified against codebase: 2026-07-13  
> Package: `internal/context`  
> Status: Living Reference Document

## 1. Scope of safety ownership

| Concern | Owner |
|---------|-------|
| Action authorization (`permitted`) | Kernel / policy — **not** context |
| Dangerous action classification | Kernel |
| What the LLM **sees** | Context package |
| Preventing score domination of window | Context package |
| Hard token limit | Context package (`TokenBudget`) |
| Concurrent map corruption | Context package (mutexes) |

Context is a **presentation and memory** layer. It must not become a backdoor that omits constitutional facts or floods the window with adversarial issue keywords.

## 2. Invariants

### I1 — Core safety facts are always attempted

`getCoreFacts` always queries:

- `permitted`  
- `dangerous_action`  
- `admin_override`  
- `security_violation`  
- `block_commit`  

On Query error: **log Warn**, continue (does not silently pretend success without signal).

### I2 — No surface text in compressed turns

`CompressedTurn` stores atoms and mangle updates, not `SurfaceResponse` body.

### I3 — Threshold before budget packing

`SelectWithinBudget` always filters by activation threshold first.

### I4 — Issue keyword weights clamped

Weights outside [0,1] are clamped so adversarial `issue_keyword` facts cannot produce thousands of activation points.

### I5 — Component score caps

| Component | Cap |
|-----------|-----|
| Dependency | 40 |
| Campaign | 60 |
| Issue | 100 |
| Back-reference | 70 |
| Feedback | effectively ±20 |

### I6 — Hard total budget

If total usage already exceeds budget, `BuildContext` returns `ErrContextWindowExceeded`.

### I7 — Observation masking never drops reasoning intent

C3 design: `should_preserve_reasoning` for all aged turns; only observations masked for old/ancient. Go summary path still retains intent atoms in simple summary.

### I8 — Map access under lock

Activation graph maps mutated only under `ActivationEngine.mu`.

### I9 — Feedback cannot empty core

Feedback only adjusts activation scores; core reserve path is independent.

## 3. Concurrency

| Resource | Protection |
|----------|------------|
| Compressor fields | `Compressor.mu` |
| Activation maps/state | `ActivationEngine.mu` (RW) |
| Feedback DB writes | `ContextFeedbackStore.mu` |
| Feedback cache | `cacheMu` |

Known historical bug: concurrent `ScoreFacts` / `GetHighActivationFacts` map race — fixed with mutex + race test.

`RefreshBudget` unlocks before `recalcBudget` to avoid deadlock.

## 4. Mangle Decl relevance

Declarations in `schemas_context.mg` (core package):

- `context_relevant` / `should_include_context`  
- `context_reachable` / `context_file_priority`  
- `turn_age_category` / `should_mask_observation` / `should_preserve_reasoning`  

Context asserts `turn_age_category` as EDB; derives inclusion via kernel query.

## 5. Failure containment

| Failure | Containment |
|---------|-------------|
| AssertBatch fails | Per-atom Assert fallback |
| Compression error | Logged; turn still returns |
| Feedback DB open fails | Boot continues without store |
| QueryAll for refresh fails | Warn; skip context refresh |
| Malformed mangle_update | Skip atom, continue parse |

## 6. What context must never do

- Assert `permitted(...)` on its own authority.  
- Execute tools/files.  
- Drop safety predicates from core selection intentionally.  
- Treat LLM free text as durable truth without atomization.  
