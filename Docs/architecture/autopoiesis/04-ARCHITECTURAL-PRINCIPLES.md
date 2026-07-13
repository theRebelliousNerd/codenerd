# 04 — Architectural Principles: Autopoiesis

> Last verified against codebase: **2026-07-13**  
> Binding for changes under `internal/autopoiesis/`

These principles are **package-specific**. Root north star still applies: LLM creative, Mangle executive, JIT for prompt behavior.

---

## P1 — Propose / dispose, never auto-commit unsafe code

Generated source is a **proposal** until SafetyChecker (and preferably Thunderdome + simulation) accept it. Regeneration with structured violation feedback is the recovery path; silent downgrade of safety is forbidden.

**Evidence:** `OuroborosLoop.ExecuteWithConfig` audit/retry loop; `FormatViolationsForFeedback`.

---

## P2 — Parent kernel is the discoverability plane

A tool that exists only as a binary without `tool_registered` / path / capability facts is invisible to logic routing. On success and on restore, assert batch facts. Prefer `AssertFactBatch` for sync.

**Evidence:** `assertToolRegistered`, `syncExistingToolsToKernel`.

---

## P3 — Prefer kernel-mediated work for cross-system generation

Campaigns and policy should trigger tools via `delegate_task(/tool_generator, Cap, /pending)` rather than importing Orchestrator deep into unrelated packages.

**Evidence:** `ProcessKernelDelegations`, e2e bridge tests, instruction CLI hook.

---

## P4 — Bound self-modification (gas)

Always enforce at least one of: session tool cap, confidence threshold, loop MaxIters/retries, learning fact max, tool source size, compile/execute timeout, Thunderdome timeout/memory.

**Evidence:** `Config`, `ExecuteConfig`, `OuroborosConfig`, `ThunderdomeConfig`.

---

## P5 — Dual-path generation is a liability

If a light path (`GenerateTool` + write) coexists with Ouroboros, it must be labeled diagnostic or temporary. New features should not add a third highway.

**Evidence:** gap analysis dual paths in chat vs `ExecuteOuroborosLoop`.

---

## P6 — Learn into the next prompt, not only into logs

Execution outcomes must update LearningStore and `SetLearningsContext` / `RefreshLearningsContext` so generators see anti-patterns.

**Evidence:** `RecordExecution`, `AggregateLearningsForPrompt`, Ouroboros `SetLearningsContext`.

---

## P7 — Adversarial testing is part of governance, not a side demo

Thunderdome/PanicMaker belong in the default Ouroboros path when enabled. Skipping is allowed only on generator failure with logged warn — not as a silent product default off without config.

**Evidence:** `EnableThunderdome: true` in orchestrator construction.

---

## P8 — Separate internal state engine from session kernel

Ouroboros may use a private `mangle.Engine` for halt/stability. Do not dump all intermediate facts into the session kernel; publish durable outcomes only.

**Evidence:** `NewOuroborosLoop` engine load of `schemas_state.mg`.

---

## P9 — Absolute paths and JSON I/O for tool binaries

Runtime tools execute only with absolute binary paths; contract is JSON stdin/stdout. Relative path = refuse.

**Evidence:** `RuntimeTool.Execute`.

---

## P10 — JIT-first for LLM-facing autopoiesis prompts

New generation/refinement/judge prompts should attach via `PromptAssembler` / prompt atoms (and SPL atom generator), not grow permanent free-text blocks in tool templates unless transitional.

**Evidence:** `SetPromptAssembler`, `prompt_evolution` package.

---

## P11 — Fail soft on missing optional kernel; fail loud on essential engines

No kernel → skip asserts / delegations cleanly. Failure to create Ouroboros Mangle engine currently **panics** at construction — treated as essential. Document that dependency.

**Evidence:** nil kernel returns in delegation; engine init panic in `NewOuroborosLoop`.

---

## P12 — Own analysis, not campaign execution

Autopoiesis may recommend campaigns and construct payloads; it must not re-implement campaign orchestration.

**Evidence:** `ExecuteAction` `ActionStartCampaign` returns nil.

---

## Anti-patterns

| Anti-pattern | Why |
|--------------|-----|
| `go build` without safety stage | Bypasses constitutional intent |
| Network allow by default | Expands attack surface of generated tools |
| Unbounded learning fact asserts | Kernel bloat / eval cost |
| Deleting “unused” registry restore | Breaks session continuity |
| Importing `articulation` concretely into autopoiesis | Cycle risk; use `PromptAssembler` interface |
