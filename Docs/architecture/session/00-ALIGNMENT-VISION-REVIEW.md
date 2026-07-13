# 00 — Alignment & Vision Review: session (`internal/session`)

> Last verified against codebase: 2026-07-13  
> Status: Living Reference Document — code-grounded  
> Source: `internal/session/` (6 non-test Go ≈ 3.1k lines; 14 test files)

## 1. North-star statement

codeNERD separates **LLM creativity** from **logic executive control**. The session package is the **runtime membrane** of that split:

- The model proposes text and tool calls (creative center).  
- The Mangle kernel decides permission via `permitted(...)` (executive).  
- JIT prompt atoms + config factories specialize behavior without hardcoding domain Go.  
- VirtualStore executive layers (Dreamer preflight, post-validators) must still fire on the interactive path even though tools no longer all go through `RouteAction`.

Session succeeds when every interactive side effect is **policy-gated**, every delegated task is **context-isolated**, and specialization is **JIT-driven**.

## 2. Alignment dimensions

| Dimension | Score (0–5) | Evidence |
|-----------|-------------|----------|
| Creative/executive split | **5** | LLM generates; `checkSafety` queries `permitted`; empty name / nil kernel fail-closed (`executor_tools.go`) |
| Fact-flow fidelity | **5** | `user_intent` assert → compilation → tools → articulation (`ProcessWithIntent`) |
| JIT / atom discipline | **5** | JITCompiler + ConfigFactory; no-tool retry recompiles atoms with `PreviousAttemptNoToolCall` |
| Isolation of delegated work | **5** | `CloneForTask`, task-scoped `/task_intent_*`, SubAgent own history |
| Constitutional safety completeness | **4** | Strong gate + InteractiveExecutiveGate seam; Piggyback single-round + safe_action fallback are nuanced |
| Concurrency / lifecycle | **4** | pendingSpawns, WaitForResult Stop-on-cancel; Wait still polls |
| Persistence / memory | **3** | SessionPersister optional; memory ops logged/asserted; Cold Storage partial |
| Test grounding | **4** | Dense unit tests + real-kernel safety + broad e2e; Piggyback multi-turn under-specified |
| Wiring honesty | **4** | Booted in `system.factory` initFinalExecutors; campaign re-constructs; dual paths documented |
| Observability | **3** | `CategorySession` helpers; no structured metrics/spans beyond logs |

**Overall alignment: 4.2 / 5** — session is a mature north-star implementation of the clean loop; residual risk is Piggyback loop completeness, persistence depth, and dual boot paths (Cortex vs campaign-local assembly).

## 3. What “good” looks like (session-specific)

| Good | Bad |
|------|-----|
| Tool blocked when `permitted` missing | Tool runs because kernel is nil |
| Delegated task uses preset intent | Re-perceive synthetic task string |
| CloneForTask / SubAgent isolation | Contaminate interactive history |
| JIT baseline fallback on compile fail | Hard crash mid-turn |
| Max tool iterations / budgets | Infinite tool loops |
| Executive gate on interactive modular tools | Only RouteAction path protected |
| Intent requires tool_call → nudge atom | Planning-only “done” with no side effects |
| Pending-spawn capacity reservation | TOCTOU over-spawn |

## 4. Related corpora

- `Docs/architecture/core/` — kernel, VirtualStore gate, ToolRegistry  
- `Docs/architecture/prompt/` — compiler + atoms  
- `Docs/architecture/perception/` / `articulation/` — observe + Piggyback  
- `Docs/architecture/campaign/` / `cli/` — primary consumers  
- `Docs/architecture/tools/` — modular registry
