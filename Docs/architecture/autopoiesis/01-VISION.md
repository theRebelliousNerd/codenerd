# 01 — Vision: Autopoiesis

> Last verified against codebase: **2026-07-13**  
> Status: Living Reference Document  
> Package: `internal/autopoiesis`

## 1. Product intent

codeNERD should not stall when the fixed tool set is incomplete. Autopoiesis is the **closed loop** that:

1. Notices a **capability gap** (or campaign / persistence need).  
2. Proposes a **minimal new tool or agent** to close the gap.  
3. Subjects the proposal to **logic + adversarial** gates.  
4. Commits only if gates pass, then **publishes facts** so the executive kernel can route to the new capability.  
5. **Learns** from execution quality so the next generation is better.

The long-horizon vision is an agent that improves its *toolbox* and *prompt atoms* without rewriting its constitutional core.

## 2. Target architecture (aspirational + partially real)

```
                    ┌─────────────────────────────┐
                    │     Parent Mangle Kernel    │
                    │  permitted / next_action    │
                    │  tool_registered / learnings│
                    └──────────▲────────┬─────────┘
                               │ facts  │ delegate_task
                    ┌──────────┴────────▼─────────┐
                    │      Orchestrator           │
                    │ detect · throttle · learn   │
                    └──────────▲────────┬─────────┘
                               │        │
              ┌────────────────┘        └────────────────┐
              ▼                                          ▼
     Complexity / Persistence                   OuroborosLoop
     (campaign/agent advice)              (proposal→audit→battle
                                           →sim→commit→execute)
                                                      │
                                                      ▼
                                              RuntimeRegistry
                                              .nerd/tools/.compiled
```

### Target properties

| Property | Vision |
|----------|--------|
| **Single generation highway** | All tool commits go through full Ouroboros + parent-kernel facts |
| **Logic-first triggers** | Policy derives `delegate_task` / refine signals; Go only executes |
| **Bounded self-modification** | Gas meters, size limits, session caps always on |
| **Battle-hardened tools** | Thunderdome is default, not optional experiment |
| **SPL integration** | Failure categories become JIT atoms with review thresholds |
| **Restorable menagerie** | Restart restores tools + kernel facts without re-generation |
| **Operator transparency** | Dashboard + glass box show why a tool was born or killed |

## 3. Non-goals

- Rewriting `nerd` itself or mutating core policy from untrusted tool code.  
- Replacing `internal/campaign` as multi-phase planner.  
- Becoming a general plugin marketplace (no remote tool download protocol in this package).  
- Vectryx-style durable semantic memory for tools (local `.nerd/` is the persistence model).  
- Unrestricted network/exec “super tools” by default.

## 4. User-visible outcomes

| Actor | Experience |
|-------|------------|
| Interactive user | System proposes campaigns for epic work; can `/generate` tools; Alt+A dashboard shows patterns/learnings |
| Automation | `ProcessKernelDelegations` after instruction turns; systems status shows autopoiesis |
| Campaign | Pregenerates or requests tools when intelligence detects gaps |
| Operator | Inspect `.nerd/tools`, safety rejections in logs, Thunderdome kill feedback |

## 5. Success metrics (engineering)

- Fraction of generated tools that pass safety without exceeding retry budget.  
- Thunderdome survival rate; fatal attack categories.  
- Kernel discoverability: registered tools queryable as facts post-boot restore.  
- Learning usefulness: reduced recurrence of same `IssueType` after refine.  
- No unbounded loops (halt oracle fires under stagnation tests).

## 6. Relationship to inversion of control

```
LLM:  invent tool shape, code, attacks, refinements, atoms
Logic: decide if need is real, if code is safe, if state may advance, if tool is known
Go:   compile, isolate execute, persist artifacts, adapt interfaces
```

Vision fails if any path lets “LLM said ship it” skip safety or kernel publication.
