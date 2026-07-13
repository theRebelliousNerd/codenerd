# 00 — Alignment & Vision Review: Autopoiesis

> Last verified against codebase: **2026-07-13**  
> Status: Living Reference Document — code-grounded  
> Source: `internal/autopoiesis/` (+ `prompt_evolution/`)

## 1. North-star statement

codeNERD separates **creative** work (LLM) from **executive** control (Mangle kernel + policy). Autopoiesis is the subsystem that lets the agent **grow new capabilities** without the LLM becoming an unconstrained code executor:

- The model drafts tool source, attack vectors, refinements, and prompt atoms.  
- Safety policy, AST facts, Thunderdome, and Mangle state rules decide what may commit.  
- Parent-kernel facts make new tools **legible** to OODA routing (`tool_registered`, `missing_tool_for`, learnings).

Autopoiesis is **not** “the model rewrites the agent binary.” It is **scoped self-extension**: tools under `.nerd/tools`, agent specs under `.nerd/agents`, and JIT atoms via prompt evolution.

## 2. Alignment dimensions

| Dimension | Score (0–5) | Evidence |
|-----------|-------------|----------|
| Creative / executive split | **5** | LLM generates; `SafetyChecker` + Ouroboros Mangle engine + parent `KernelInterface` gate commit (`ouroboros.go`, `checker.go`, `autopoiesis_kernel.go`) |
| Fact-flow fidelity | **4** | Registration/learning facts asserted; `delegate_task` path implemented; chat also has direct `generate_tool` that can bypass full Ouroboros (`process.go`) |
| Constitutional safety | **4** | Embedded `go_safety.mg`, AST facts, default no networking, binary path absolute check; `AllowExec` true by default needs operator awareness |
| JIT / atom discipline | **4** | `PromptAssembler` injection; `prompt_evolution` produces atoms; tool gen prompts still partially template-driven |
| Wiring honesty | **4** | Boot wires Orchestrator + VirtualStore tool gen/exec (`factory.go`); UI/chat surfaces real; campaign pregen uses package |
| Observability | **4** | Dedicated log category, stage banners, traces, Ouroboros/Thunderdome stats, UI dashboard |
| Test grounding | **4** | Large unit suite + e2e kernel contracts; fewer full-loop LLM integration tests |
| Gas / non-divergence | **4** | Session tool caps, learning fact bound, MaxIters/retries, Thunderdome timeouts |
| Scope discipline | **5** | Campaign *execution* and `permitted` policy stay outside package |

**Overall alignment: 4.2 / 5** — mature self-extension layer with residual risk around dual generation paths and operator-facing power of compiled tools.

## 3. What “good” looks like (autopoiesis-specific)

| Good | Bad |
|------|-----|
| Kernel learns `tool_registered` after commit | Silent binary drop with no facts |
| Safety fail → regenerate with violation feedback | Compile unsafe code “for later” |
| `delegate_task(/tool_generator,…)` from policy | Only chat-side side effects |
| Learnings refresh into next generation prompts | Fresh session ignores past failures |
| Thunderdome kill → bounded panic retries | Infinite regenerate-on-panic |
| QuickAnalyze for hot path; full Analyze offline | LLM complexity on every keystroke |

## 4. Tension with north star (honest)

1. **Compile-and-run tools** are powerful effectful programs. Safety is strong but not a full OS sandbox; Thunderdome and policy are mitigations, not proof.  
2. **`ExecuteAction` tool path** can write/register without full Ouroboros stages — lighter, less governed.  
3. **Prompt evolution auto-promote** can change agent behavior without a human review gate if enabled.

## 5. Related corpora

- `Docs/architecture/core/` — kernel, bridge, VirtualStore  
- `Docs/architecture/campaign/` — consumers of complexity / tools  
- `Docs/architecture/prompt/` — atom integration target  
- `Docs/architecture/cli/` — operator surfaces  
