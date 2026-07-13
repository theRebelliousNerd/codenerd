# core — Architectural Principles

> Last verified: **2026-07-13**  
> Binding for work in `internal/core/`. Package-specific; not generic “write clean code.”

## P1 — Logic determines reality

Mangle IDB is the authority for *what the system believes is next*. Go handlers execute envelopes; they do not redefine policy.

**Implications:** Prefer new `Decl` + rules over `if` chains that invent permissions. When Go must hard-block (constitution), keep rules simple, enumerable, and dual-logged as `security_violation` facts.

## P2 — Default deny

Effects require positive permission. Absence of `permitted` is deny.

**Implications:** Never “allow if kernel nil” for paths that can mutate. Current
RouteAction permission checks deny nil kernels; boot must attach the kernel before
routing.

## P3 — Fail closed on safety machinery

Dreamer, constitution check, and parse/analyze failures must not fail open.

**Evidence:** `dreamer.go` treats missing kernel, eval error, missing `panic_state` Decl as unsafe. `NewRealKernel` returns error if constitution will not compile.

## P4 — Stratified trust

Load order is a security boundary:

1. Schemas (Decl physics)  
2. Policy (constitution + executive)  
3. Learned (autopoiesis)

**Implications:** Never prepend learned rules before constitution. Sandbox-validate (`HotLoadRule`) before mutating live learned text.

## P5 — Quiescent boot

Chat process start and session rehydrate must not execute stale `next_action`
chains. Command-oriented `BootCortex` is a separate mode entered after an explicit
user command and may release its guard during construction.

**Evidence:** ephemeral boot fact filter; `bootGuardActive` until first user interaction.

## P6 — Effects through VirtualStore

Side effects that change the world (files, shell, network tools, MCP) belong behind VS (or a documented, equally gated delegate).

**Implications:** New tool = ActionType + handler + policy surface + tests. Avoid new package-local `os/exec` in core outside tactile.

## P7 — Defense in depth, not single chokepoint

Layers: boot guard → Dreamer → Go constitution → Mangle `permitted` → binary/env allowlists → post validators → result facts.

**Implications:** Removing any layer requires a written threat-model exception.

## P8 — Import-cycle discipline

`Fact` / `Kernel` / `LLMClient` live in `internal/types` with core aliases. Session is wired via interfaces defined in core (`TaskDelegator`), not core importing session.

**Implications:** Do not reintroduce `core → articulation → core` cycles; use types + interfaces.

## P9 — EDB mutability is explicit

Facts are assert/retract, not silent in-place edit. Retracts invalidate differential engines.

**Implications:** Prefer exact retract batches over “overwrite by reasserting different args” without retracting old.

## P10 — Feature flags preserve legacy semantics

Cortex per-shard facts and diff-eval must default to behavior that does not surprise existing sessions when flags are off.

**Evidence:** comments in `cortex_kernel.go` and `kernel_eval.go`.

## P11 — Wiring before deletion

Partial integrations (ShadowMode, RuleCourt, Cortex, APIScheduler) have e2e and CLI surfaces. Audit reverse deps before labeling “unused.”

## P12 — Mangle is not an NLP engine

No large fuzzy string banks in policy. Use retrieval/embeddings externally; assert structured facts; reason with Decl rules.

**Implications:** Intent classification corpora may ship as data, but matching strategy stays outside pure Datalog when fuzzy.

---

## Principle conflicts (resolve explicitly)

| Conflict | Resolution bias |
|----------|-----------------|
| Perf (skip dreamer) vs safety | Safety wins for destructive |
| Fast Exec vs full RouteAction | Document Exec as limited; prefer RouteAction for policy-derived actions |
| Full re-eval simplicity vs diff-eval speed | Correctness first; flag experimental |
| Monolithic boot corpus vs selective load | Prefer correct full boot for interactive agent; optimize CLI verbs later |

## Checklist for new core features

1. Does it change what is *allowed*? → policy/schema  
2. Does it change what is *executed*? → VS handler + ActionType  
3. Does it change what is *true*? → Fact assert path  
4. Can it break boot? → embed load + golden/policy test  
5. Can LLM abuse it? → permitted + dreamer + constitution  
