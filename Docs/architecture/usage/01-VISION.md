# usage — Vision

> Last verified: **2026-07-13**  
> This is the **target** architecture for token/cost metering. Current implementation is a solid core subset; see [03-GAP-ANALYSIS.md](03-GAP-ANALYSIS.md).

## Product intent

Operators of codeNERD (humans and automation) need trustworthy answers to:

1. **How much** did this workspace spend (tokens in/out) across the project lifetime?
2. **Where** did spend go (provider, model, shard type, operation, session)?
3. **Did we miss metering** (nil tracker, wrong context, non-ZAI engines)?

The package answers (1) and (2) for wired paths; (3) remains an integration discipline problem.

## Target principles

### Ambient, never controlling

Usage attaches via context. Missing tracker ⇒ silent no-op. Metering must never fail a successful LLM call or block OODA.

### Workspace-scoped truth

One file per workspace: `.nerd/usage.json`. Aligns with config-is-boss / single project root rules used by chat workspace resolution.

### Attribution dimensions that match codeNERD

| Dimension | Why it matters here |
|-----------|---------------------|
| Provider / model | Multi-engine perception (ZAI, xAI, Claude CLI, …) |
| Shard type / name | Parallel specialists vs system vs ephemeral |
| Operation | `chat` vs `embedding` vs future `tool_gen` |
| Session | Long-horizon campaigns vs short CLI verbs |

### Separation from executive logic

Budget *policy* (if ever required) belongs in Mangle + VirtualStore as explicit facts and `permitted` rules — **not** inside `Tracker`. Tracker remains a sensor.

## Target shape (north of current code)

```
┌─────────────────────────────────────────────────────────┐
│  perception.LLMClient implementations                   │
│    on success: FromContext → Track(…, op)               │
│    on stream final: Track once with billed usage        │
└───────────────────────────┬─────────────────────────────┘
                            │
┌───────────────────────────▼─────────────────────────────┐
│  usage.Tracker                                          │
│    - aggregates (maps)                                  │
│    - optional event ring (bounded)                      │
│    - optional cost_est from static price table          │
│    - durable save (atomic write + dirty re-arm)         │
└───────────────────────────┬─────────────────────────────┘
                            │
          ┌─────────────────┼─────────────────┐
          ▼                 ▼                 ▼
     TUI usage page    CLI dump/export   future: store/
     (operator)        (scripts)         session reports
```

## Non-goals (vision boundary)

- Real-time cloud billing reconciliation with provider invoices  
- Cross-workspace fleet dashboards  
- Blocking execution when a soft budget is exceeded (that would be a **kernel** feature if productized)  
- Embedding model pricing as first-class without operation tags from embedding engines  

## Success criteria

| Criterion | Signal |
|-----------|--------|
| Complete metering | Every production `LLMClient` that reports usage tokens calls `Track` when tracker is present |
| Honest attribution | Shard and session keys rarely `"unknown"` in interactive multi-shard work |
| Durable aggregates | Crash within debounce window loses at most one dirty burst, not days of data |
| Operator UX | Usage page reflects same dimensions Stats exposes |
| Safety of absence | Nil tracker never panics; non-string context values degrade |

## Relationship to sibling systems

| System | Relationship |
|--------|----------------|
| `perception` | **Producer** of Track calls |
| `system` / Cortex | **Owner** of tracker lifetime at boot |
| `core/shards` | **Annotator** of context (shard name/type/session) |
| `cmd/nerd/ui` | **Consumer** of `Stats()` |
| `logging` / `observability` | Complementary — request logs vs durable project totals |
| `store` | Possible future durable sink; not required for MVP vision |
