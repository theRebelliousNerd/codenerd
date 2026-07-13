# 04 — Architectural Principles: session

> Last verified: 2026-07-13  
> These principles are **binding** for changes in `internal/session`.

## P1 — Logic is executive; model is creative

The LLM may propose any tool call. Execution requires `checkSafety` (when enabled) to observe a matching `permitted` fact (or the documented `safe_action` fallback). Session never invents alternate permission math.

## P2 — JIT is the specialization mechanism

Persona, tools, and policies come from `JITCompiler` + `ConfigFactory` / injected `EffectiveAgentRuntimeConfig`. Do not grow domain-specific `if verb == "/review" { ... 200 lines }` inside the loop.

## P3 — Fail closed on safety

If `EnableSafetyGate` is true and the kernel is missing, or payload cannot be represented, or name is empty: **deny**. Do not “allow for convenience” in production paths.

## P4 — Isolate delegated work

- Interactive history must not absorb task turns → `CloneForTask`.  
- Concurrent tasks must not clobber `/current_intent` → `/task_intent_*` + retract.  
- Dream mode → always SubAgent.

## P5 — Prefer preset intents for machine tasks

Routing/orchestration already classified the verb. Re-perception of synthetic task text is wasteful and can mis-route.

## P6 — Tool budgets are hard stops

`MaxToolCalls`, `MaxToolIterations`, and `ToolTimeout` exist to bound cost and hang risk. Raising them requires product reason, not silent default creep.

## P7 — Dual tool protocol, single safety path

Native function-calling and Piggyback structured tools may differ in how calls are obtained, but both must funnel through `executeToolCall` (allow-list → safety → executive gate → registry).

## P8 — Graceful degradation over hard crash

Missing JIT → baseline prompt. Failed config → empty config. Missing InteractiveExecutiveGate → skip gate. These degrade **capability**, not **constitutional fail-closed** (which remains strict).

## P9 — Wiring before deletion

Partial integrations (SessionPersister, memory ops, Ouroboros missing_tool_for) are hooks for other packages. Grep reverse deps and factory wiring before removing “unused” fields.

## P10 — Neuro-symbolic fixes for model failure modes

Planning-only false completes are fixed by **Mangle + prompt atoms** (`intent_requires_tool_call`, tool_nudge atom), not by hardcoding “you must call write_file” strings in Go.

## P11 — Capacity is a first-class resource

Spawner max active + pending reservation prevents TOCTOU over-subscription. New spawn entry points must honor the same accounting.

## P12 — Observability via CategorySession

Use `logging.Session` / `SessionDebug` / CategorySession warn-error. Do not invent ad-hoc print paths for production diagnostics.
