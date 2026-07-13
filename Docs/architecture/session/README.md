# Session: the universal execution loop

> Corpus: `session` | Live owner: `internal/session` | Verified: 2026-07-13

## In one minute

Session is where codeNERD turns one structured request into a bounded interaction
with a model, tools, specialists, and the kernel. It builds the turn-specific JIT
prompt and capability config, lets the model propose tool calls, checks each call
against both the effective allowlist and exact Mangle permission, executes through
VirtualStore, feeds results back, articulates the answer, and optionally persists
the turn.

The user-visible outcome is a coding agent that can read, edit, test, and delegate
without treating model text or tool availability as authority.

`VERIFIED CURRENT` — the primary route is
`internal/session/executor.go#Executor.ProcessWithIntent`; the native tool loop is
`internal/session/executor_tools.go#Executor.runToolLoop`; specialist construction
is `internal/session/spawner.go#Spawner.SpawnSpecialist`.

## Its place in codeNERD

The LLM is the creative center: it interprets the compiled prompt, proposes prose,
tool calls, and Piggyback control. Session is the deterministic harness around
that creativity. Mangle is still the executive: session asserts the exact
`pending_action(Action, Target, Payload)` and accepts only an exactly matching
`permitted(Action, Target, Payload)` decision before effect.

```text
perception Intent / delegated task
  -> assert user_intent
  -> JIT prompt + EffectiveAgentRuntimeConfig
  -> resolve only AllowedTools
  -> LLM proposes tool call
  -> effective capability check
  -> pending_action exact envelope -> Mangle permitted/3
  -> VirtualStore preflight/execute/post-validate
  -> tool result -> model -> articulation -> user
```

Session does not own provider transport, prompt atom policy, tool implementation,
or the constitutional rule corpus. It owns their turn lifecycle and refuses to
turn missing configuration into ambient capability.

## A representative journey

Suppose the user asks, “Fix the nil dereference in the provider factory and run
the focused test.”

1. Perception supplies an intent; `ProcessWithIntent` asserts it and constructs a
   `prompt.CompilationContext` from target, mode, diagnostics, session state, and
   runtime capabilities.
2. JIT returns a prompt; `ConfigFactory` returns identity, allowed tools, policies,
   loop bounds, and safety settings. Nil or empty allowed tools means no tools.
3. The model proposes `read_file`, then `edit_file`, then `run_tests`. Session
   resolves only tools present in the effective capability envelope. An Ouroboros
   registry entry does not grant capability by itself.
4. Before each call, `checkSafety` canonicalizes JSON payload, bounds it, asserts
   the exact pending action, evaluates the kernel, and requires the matching
   `permitted/3`. `safe_action/1` classification alone cannot authorize it.
5. VirtualStore performs its own interactive preflight/post-validation around the
   exact effect. Results return to the model until loop/call bounds or completion.
6. Hollow-success detection can request one JIT no-tool nudge for a mutation that
   produced only narrative. Articulation separates surface and control; the turn
   may be persisted.

`VERIFIED CURRENT` — exact permission is covered by
`internal/session/executor_test.go#TestExecutor_CheckSafety_SafeActionWithoutPermittedDenies`;
capability failure is covered by
`internal/session/executor_capability_test.go#TestExecutorToolCapabilityEnvelopeFailsClosed`.

## What exists today

| Applicability lane | Evidence-backed answer |
|---|---|
| Mangle | `VERIFIED CURRENT` — session asserts `user_intent` and exact `pending_action/3`, evaluates the kernel, queries `permitted/3`, and filters for action/target/canonical payload equality. `safe_action/1` is classification only. Mangle declarations/policy live in core defaults, not this package. |
| Permission and safety | `VERIFIED CURRENT` — effective allowlist first, payload/name/size checks, exact constitutional permission, Dreamer handling, and VirtualStore interactive gate compose. Nil kernel, nil/empty config, unknown tool, malformed payload, wrong arity, and nonmatching permission fail closed. |
| Fact flow | `VERIFIED CURRENT` — Intent -> `user_intent` -> JIT/model -> tool proposal -> `pending_action` -> `permitted` -> effect -> tool result -> articulated surface. Piggyback control is parsed and selectively applied by the executor. |
| JIT and agents | `VERIFIED CURRENT` — compiler/config factory drive identity and capabilities; specialist YAML is size/path bounded and now runs `EffectiveAgentRuntimeConfig.Validate`. Spawner/SubAgent/JITExecutor provide bounded dynamic workers. `PARTIAL` — default policy references still need the typed live registry repair documented in the JIT corpus. |
| Wiring | `VERIFIED CURRENT` — system factory constructs the normal Executor/Spawner/JITExecutor and connects Cortex; campaigns also build a stack. `PARTIAL` — duplicated assembly can drift, and optional persistence/Ouroboros wiring varies by caller. |
| State and concurrency | `VERIFIED CURRENT` — executor history is locked/bounded, Spawner reserves capacity atomically under lock, SubAgent state is atomic, task clones isolate intent/history, and shutdown/spawn race tests exist. `PARTIAL` — workers share the executive kernel by design and require exact task/turn identity discipline. |
| Recovery | `VERIFIED CURRENT` — JIT/config fallback, tool timeouts, panic capture, max iterations/calls, context cancellation, hollow-success nudge, task completion/error state, compression, and shutdown paths exist. `PARTIAL` — Piggyback tool feedback is single-round and durable state restoration is incomplete. |
| Observability | `VERIFIED CURRENT` — category logs trace compilation, tools, gate outcomes, loops, spawn state, persistence errors, and completion. `PARTIAL` — no durable correlated receipt joins compilation, capability, permission, effect, control processing, persistence, and response. |
| Testing | `VERIFIED CURRENT` — package tests cover exact permissions, real-kernel allow/deny, payload bounds, capability failure, Ouroboros non-grant, config validation, process/tool failure, timeouts, races, spawn lifecycle, and hollow success. Integration-tagged empty/failing config tests now assert no ambient tools. |

The full state machine is in [Implemented Spec](IMPLEMENTED_SPEC.md); detailed
boundaries are in [Safety](09-SAFETY-AND-INVARIANTS.md) and [Wiring](08-WIRING-AND-INTEGRATION.md).

## North star

Every turn should be reproducible from a redacted execution receipt: perception
intent, JIT context/manifest, immutable capability envelope, exact Mangle decision,
VirtualStore validation/effect, tool-result loop, Piggyback controls, persistence,
and user response. All creation routes should use one stack builder and one
lifecycle contract; recovery should resume only idempotent, proven state.

Non-goals:

- Capability lists and prompt text never replace constitutional permission.
- Session does not implement fuzzy routing in Mangle or provider-specific clients.
- A fallback prompt/config cannot grant ambient tools.
- Dynamic specialists do not reintroduce hardcoded domain shard executors.
- Receipts do not store secrets, raw prompts, hidden reasoning, or full payloads.

## Improvement frontier

1. `VERIFIED CURRENT` — nil/empty capability envelopes deny all tools; Ouroboros
   registry membership cannot bypass the allowlist; specialist config runs strict
   validation; exact `permitted/3` is the only kernel authorization.
2. `PROPOSED UPLIFT` — complete multi-turn Piggyback tool-result feedback under
   the same bounds, capability, permission, and cancellation contract as native
   function calling.
3. `PROPOSED UPLIFT` — replace duplicated Cortex/campaign construction with a
   versioned session stack builder and explicit ownership/teardown manifest.
4. `PROPOSED UPLIFT` — persist compiled atom IDs and compressed subagent/session
   state with schema versions, idempotency, and restart tests.
5. `PROPOSED UPLIFT` — emit a redacted turn execution receipt and operator “why
   blocked/why executed/why stopped?” view.
6. `DEFERRED` — replay a receipt in a deny-all/dry-run sandbox to compare candidate
   prompts, policies, and loop strategies without external effects.

Machine-readable acceptance and rollback live in [TODO](TODO.md).

## Choose a reading route

| Time | Route |
|---|---|
| 90 seconds | This README, then [Current State](02-CURRENT-STATE.md) and [Gap Analysis](03-GAP-ANALYSIS.md). |
| 10 minutes | [Internal Architecture](05-INTERNAL-ARCHITECTURE.md), [Wiring](08-WIRING-AND-INTEGRATION.md), [Safety](09-SAFETY-AND-INVARIANTS.md), and [Failure Modes](12-FAILURE-MODES.md). |
| Deep implementation | [Implemented Spec](IMPLEMENTED_SPEC.md), [Public API](06-PUBLIC-API-AND-TYPES.md), [Dependencies](07-DEPENDENCY-MAP.md), and [Testing](10-TESTING-ALIGNMENT.md). |
| Build or review an uplift | [Vision](01-VISION.md), [Gap Analysis](03-GAP-ANALYSIS.md), [TODO](TODO.md), [Open Questions](OPEN-QUESTIONS.md), then [_progress](_progress.md). |
