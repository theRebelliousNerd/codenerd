# verification — Vision

> Last verified: **2026-07-13**  
> Status: Target architecture (aspirational where noted; grounded in what the package already is)

## Product role

When codeNERD **mutates** a codebase (or performs other high-stakes delegated work), the system must **refuse to silently ship mock, incomplete, or hallucinated results**. The human should either receive:

- work that passed a quality gate, or  
- an **escalation** with violations, evidence, and suggestions — never a polished lie.

`TaskVerifier` is that gate: a **retrying quality executive** around creative shard execution.

## Target architecture

```
                    ┌──────────────────────────────────────┐
                    │         Executive quality layer       │
                    │  (deterministic loop, fixed policy)  │
                    └───────────────┬──────────────────────┘
           spawn / re-spawn         │         judge / select
                    ▼               │               ▼
           TaskExecutor / Shards    │        LLM (creative judge)
           + optional Autopoiesis   │        or heuristic fallback
                    │               │               │
                    └──────► result text ◄──────────┘
                              │
                    persist attempts (store)
                    optional: assert structured facts
                    escalate to user when exhausted
```

### Vision principles

1. **Quality is executive, not decorative** — success is not “the model said LGTM in prose”; it is a structured `VerificationResult` that the loop interprets.  
2. **Mutations pay for rigor; queries stay light** — verification cost is acceptable when code was written.  
3. **Correctives feed the next attempt**, not a parallel chat: specialist knowledge, tool generation, decomposition, re-shard.  
4. **Escalation is a first-class outcome** — max retries is not a crash; it is user handoff.  
5. **History compounds** — stored verifications should eventually bias shard selection and confidence (learning loop).  
6. **Judge prompts are policy artifacts** — same as other LLM-facing text: versionable, atom-selectable, testable.  
7. **Orthogonal to constitution** — `permitted(write_file, …)` can hold while quality fails; both gates matter.  

## Target outcomes (user-visible)

| Outcome | Meaning |
|---------|---------|
| Verified success | UI shows passed confidence; facts loaded for blackboard |
| Soft skip | Only when deliberately configured (offline/dev) — not accidental |
| Escalation | Clear violations + evidence + next steps |
| Never | Silent acceptance of `// TODO implement` as finished work under normal online boot |

## Non-goals

- Replacing unit/integration test runners  
- Replacing Mangle policy / permission  
- Becoming a general “eval framework” product  
- Verifying every `/query` or `/review` by default (latency)  
- Embedding Vectryx-specific product surfaces  

## Path from current → vision

| Now | Target |
|-----|--------|
| Inline system prompts | Prompt atoms + optional JIT selection |
| Fail-open on LLM judge failure | Configurable: fail-open (dev) vs fail-closed (strict) |
| Write-only `task_verifications` | Read-back into `selectBestShard` / confidence |
| Research corrective = specialists only | Wire JIT research path intentionally |
| String-equal error check in chat | `errors.Is` + stable sentinel |
| Heuristic covers 3 of 8 violations | Align fallback with full taxonomy or document intentional subset |

## Success metrics (qualitative)

- Mutation delegations that ship placeholder code without escalation trend toward zero under normal boot  
- Retry loops change shard or enrich task more often than pure blind re-run  
- Operators can query verification history per session for forensics  
- No import cycle with chat; persona mapping stays shared-by-convention or extracted to a neutral package  
