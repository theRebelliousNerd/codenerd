# 01 — Vision: Configuration Substrate

> Last verified: 2026-07-13  
> Package: `internal/config`

## 1. Product vision

Operators should configure codeNERD **once per workspace** with a single JSON file that is:

- **Human-editable** (pretty-printed by `Save`)
- **Wizard-writable** (chat config wizard, auth commands)
- **Default-safe** when absent (empty parse + Get* defaults)
- **Honest** about which cloud/local backend will be called

The vision is **not** “framework of plugins rewriting config every frame.” It is a **stable control surface** for engines, budgets, and UX.

## 2. Architectural vision

```
┌─────────────────────────────────────────────────────────┐
│  Operator surface (CLI auth, /config wizard, init)       │
└───────────────────────────┬─────────────────────────────┘
                            │ Save / Load
                            ▼
┌─────────────────────────────────────────────────────────┐
│  UserConfig  (.nerd/config.json)                         │
│  engines · keys · limits · JIT · embed · world · UX      │
└───────┬─────────────┬──────────────┬────────────────────┘
        │             │              │
        ▼             ▼              ▼
   perception     core/scheduler   prompt/world/tactile
   clients        ceilings         budgets & allowlists
```

### Target properties

1. **Single aggregate** for runtime (UserConfig) — YAML `Config` either thin adapter or retired.
2. **Symmetric env override policy** documented and applied once.
3. **Validated engines/providers** on load (fail early, not mid-OODA).
4. **No product-specific sibling-platform/foreign-product-surface knobs** in this package — stay general-purpose for codeNERD.
5. **Leaf packages** continue to use `internal/features` active pointer rather than importing config.

## 3. Relationship to north star

| North-star idea | Config’s contribution |
|-----------------|----------------------|
| LLM = creative center | Chooses provider/engine/model/temperature profiles |
| Kernel = executive | Supplies fact limits, derived limits, session ceilings |
| Constitutional safety | Supplies execution allowlists and concurrency bounds |
| JIT atoms | Supplies JIT enablement, budget, semantic top-k |
| Wiring before deletion | Dual path means “unused” YAML may still boot — audit first |

## 4. Non-goals

- Hot-reload every keystroke into a live kernel without explicit reload.
- Fuzzy natural-language “config intent” inside this package.
- Embedding large prompt prose here (belongs in prompt atoms).
- Mangle rule authoring (belongs in policy corpus).
