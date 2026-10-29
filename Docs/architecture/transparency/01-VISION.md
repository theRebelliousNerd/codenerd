# transparency — Vision

> Last verified: 2026-07-13  
> Scope: target architecture for `internal/transparency/` and its consumers

## 1. Product intent

codeNERD is a **logic-first** coding agent: the LLM is creative; the Mangle kernel is executive. That split only earns user trust if the system can **show its work** when asked:

1. **Why** an action was chosen (`next_action` derivation narrative).  
2. **Why** an action was blocked (`permitted` / constitutional gate explanation).  
3. **What** is running right now (shard phases, tool calls, routing).  
4. **How** to recover when something fails (typed errors + remediation).  

The transparency package is the **shared vocabulary and plumbing** for those answers. The TUI, slash commands, and core execution sites emit into it or format through it; they should not each invent private telemetry types.

## 2. Dual visibility model

Vision cleanly separates two channels that already exist in code:

| Channel | Default | Purpose |
|---------|---------|---------|
| **Glass Box** (`GlassBoxEventBus`) | Bus enabled at boot; TUI may gate display | Subsystem telemetry: perception, kernel, JIT, shard, control, routing |
| **Tool events** (`ToolEventBus`) | Always active | Tool/action execution must never be invisible “magic” |
| **Transparency manager** | Master `Enabled=false` by default | Operator-depth features: phase tracking, safety report history, verbose errors, status tables |

Rationale: everyday users see tools without configuring anything. Power users enable deeper glass / transparency when debugging the executive loop.

## 3. Target capabilities (north star for this package)

### 3.1 Explain “why” from logic, not slogans

- Derivation traces from Mangle (`DerivationTrace` / proof trees) render as human narratives (`Explainer`).  
- Decision explanations specifically surface `next_action` roots when present.  
- Safety blocks map to rule/action/target and suggest safe alternatives (`/shadow`, `/why`, `/query permitted`).

### 3.2 Live, ordered, non-blocking telemetry

- Sequence numbers preserve order across async producers.  
- Batching reduces UI churn in normal mode; **verbose/full-stream** bypasses batch delay for live chat.  
- Subscriber channels are large (512) and **drop on full** rather than blocking producers (execution path stays non-intrusive).

### 3.3 Config-driven feature flags

`config.TransparencyConfig` is the single schema for:

- Master enable  
- Shard phases, stream reasoning, safety explanations, JIT explain, operation summaries, verbose errors  
- Glass Box enable/disable, categories, verbose details  

Vision: every flag either drives real behavior or is marked deferred in gap analysis—no silent “status table only” forever.

### 3.4 Non-intrusive executive path

Transparency never:

- Asserts facts that change `next_action`  
- Softens constitutional default-deny  
- Blocks VirtualStore on a full event channel  

It may only **observe**, **record**, and **format**.

### 3.5 One vocabulary for TUI and headless

Types live here so:

- `cmd/nerd/chat` can subscribe and render  
- `internal/core` (VirtualStore, ShardManager) can emit without importing chat  
- System shards can attach buses via setters without import cycles into UI  

## 4. Placement in the OODA / fact flow

```
Observe   → perception may emit CategoryPerception events
Orient    → kernel assertions/derivations may emit CategoryKernel
Decide    → next_action / permitted remain kernel facts;
            Explainer / SafetyReporter explain after the fact
Act       → VirtualStore + router emit ToolEvent + CategoryRouting;
            ShardManager emits CategoryShard spawn/complete
Articulate→ chat formats Glass Box lines, tool lines, /why narratives
```

Transparency is **side-channel observability** parallel to fact-flow, not a new control plane.

## 5. Non-goals

- Not a general-purpose logging framework (use `internal/logging`).  
- Not a metrics backend or OpenTelemetry exporter (may emit *into* one later; not required here).  
- Not a replacement for Mangle provenance / `/explain` proof trees (complementary: heuristic `/why` + explainer formatting vs recorded provenance).  
- Not product-specific sibling-platform/foreign-product-surface UI.  
- Not a policy engine: does not decide `permitted(...)`.

## 6. Success criteria

| Criterion | Observable |
|-----------|------------|
| Tools never silent | ToolEvent lines appear with Glass Box off |
| Glass Box useful under load | Multi-shard tool storms do not deadlock; drops only when buffers full |
| Safety explainable | Blocked actions can produce formatted `SafetyViolation` text |
| Logic explainable | `/why` path can render `ExplainTrace` markdown |
| Operator toggle | `/transparency on|off` reflects in `GetStatus()` |
| Tests hold the contract | `go test ./internal/transparency/...` green; bus filter/batch/unsubscribe covered |

## 7. Evolution path (vision, not commitment)

1. **Close wiring gaps**: `TransparencyManager` phase APIs actually called from shard lifecycle (today Glass Box events are the live path; `ShardObserver` is underused).  
2. **Honor remaining config flags**: `JITExplain`, `StreamReasoning`, `OperationSummaries` either wire fully or drop from status.  
3. **Structured safety feed**: constitutional denies → `ReportSafetyViolation` automatically with rule IDs from kernel, not only string heuristics.  
4. **Richer category filter**: config documents `routing`; keep `AllCategories()` and config docs aligned.  
5. **Headless/JSON mode**: optional machine-readable event export for CI campaigns without Bubble Tea.
