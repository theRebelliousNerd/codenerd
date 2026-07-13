# core — Vision

> Last verified: **2026-07-13**  
> This is the *target* architecture for `internal/core`. Reality is in [02-CURRENT-STATE.md](02-CURRENT-STATE.md) and [IMPLEMENTED_SPEC.md](IMPLEMENTED_SPEC.md).

## Product thesis

codeNERD must not let an LLM be both visionary and executive. **Core is the executive:**

- Natural language and model proposals become **facts**.
- Mangle derives **what is true and what is allowed**.
- VirtualStore is the only privileged way those truths become **effects**.
- Dreamer is the speculative conscience: *would this effect panic the system?*

## Target responsibilities

| Responsibility | In scope for core | Out of scope |
|----------------|-------------------|--------------|
| Compile & evaluate constitution | Yes | — |
| Hold session EDB | Yes | Long-term vector memory product |
| Permit / deny actions | Yes | UX copy for denials (articulation) |
| Route effects | Yes | Browser engine internals |
| Speculative safety | Yes | Full OS sandboxing product |
| Shard spawn plumbing | Yes (minimal) | Domain agent personas (prompt atoms / session) |
| LLM creative planning | No | Perception, articulation, prompt JIT content |

## Target control loop

```
                    ┌────────────────────┐
   perception ─────►│  Kernel EDB facts  │
                    └─────────┬──────────┘
                              │ evaluate (stratified)
                    ┌─────────▼──────────┐
                    │  IDB: next_action  │
                    │       permitted    │
                    │       persona …    │
                    └─────────┬──────────┘
                              │
              ┌───────────────▼────────────────┐
              │ VirtualStore.RouteAction       │
              │  boot → dream → const → permit │
              └───────────────┬────────────────┘
                              │ effects + result facts
                    ┌─────────▼──────────┐
   articulation ◄───│  updated EDB/IDB   │
                    └────────────────────┘
```

## Non-negotiable invariants (vision)

1. **Default deny:** no effect without derived or layered permission.  
2. **Fail closed on safety machinery failure:** Dreamer/eval errors block destructive actions.  
3. **Stratified trust:** learned rules never outrank constitution (load order).  
4. **Quiescent boot:** rehydration must not replay effects without a user turn.  
5. **Logic-visible critical paths:** catastrophic deletes are expressible as `panic_state` facts/rules.  
6. **One kernel truth per process (default):** multi-domain Cortex is federation, not N independent constitutions without bridging.  
7. **JIT for new LLM prose:** new model-facing instructions become atoms elsewhere; core gains **Decls/rules**, not free-text shard prompts.

## Evolution targets (not claims of done)

| Target | Motivation |
|--------|------------|
| Single preferred orchestration entry | Reduce session vs ShardManager dual mental model |
| Diff-eval production-ready or clearly off | Boot/eval cost at campaign scale |
| Stronger projection model for Dreamer | Catch more classes of harm without OS sandbox |
| Stable public Kernel/VS interfaces | Easier e2e and external tools |
| Smaller boot program options | Selective schema load for lightweight CLI verbs |

## Anti-vision (do not build into core)

- App-specific product features for showcase demos  
- Fuzzy NLP pattern banks as Mangle rules  
- Silent default-allow on missing policy  
- Embedding vector search *inside* the kernel engine  
- Growing ad-hoc prompt strings in VirtualStore handlers  

## Success criteria

Core is successful when:

1. A hostile or confused LLM cannot execute `rm -rf`-class outcomes without multi-layer deny.  
2. Adding a new safe tool is: Decl + `safe_action` + VS handler + tests — not a special case in chat.  
3. Policy authors can reason about `permitted` / `next_action` without reading Go.  
4. Boot either loads a coherent constitution or fails loudly (`debug_program_ERROR.mg`).
