# 01 — Vision: shards

> Last verified against codebase: 2026-07-13  
> Target state for `internal/shards` + `internal/shards/system`

## 1. Product role

Shards are the **typed agents of the cortex**:

- **Type 1 (system)** — permanent or on-demand OODA services that speak only in facts and tools.  
- **Type 2 (ephemeral)** — short-lived helpers (e.g. requirements interrogator).  
- **Persona execution** — *not* Go types here; JIT personas spawned through session/ShardManager for user work.

The vision is a **thin, durable system spine** plus **declarative personas**, not a zoo of bespoke coding shards.

## 2. Target capabilities

### 2.1 Single registration story

One registration API (`RegisterAllShardFactories` + profile defs) consumed by **all** boots (CLI Cortex factory, interactive chat, campaign CLI, init scanner). Chat-only DI (GlassBox, ToolStore) should attach via:

- post-spawn hooks on ShardManager, or  
- optional fields on `RegistryContext`,  

not a parallel hand-rolled factory list that diverges.

### 2.2 Authoritative predicate routing

`DefaultShardPredicateManifests` becomes the **single source of truth** for per-domain fact ownership when multi-kernel / per-shard facts are enabled. Asserts to the wrong domain re-route or fail loud.

### 2.3 Event-driven OODA by default

Every continuous system shard prefers kernel event-bus subscription over polling. Fallback tickers remain for tests and degraded kernels only.

### 2.4 Logic-primary safety

Constitution gate and executive barriers remain free of LLM decision authority. LLM may **propose** rules (autopoiesis/legislator) but **never** grant `permitted` by fiat.

### 2.5 Specialist mesh (JIT-native)

Matching, consultation, and observers orchestrate **registered agents** (`.nerd/agents`) and JIT personas without resurrecting domain Go packages. Matching may later add embedding retrieval; classifications stay structured facts.

### 2.6 Honest package docs

`internal/shards/README.md` should describe **what is live**, not only the 2024 migration.

## 3. Non-goals

- Rebuilding hard-coded coder/tester/reviewer Go shards  
- Moving policy corpus into this package  
- Replacing VirtualStore tool implementations  
- Becoming a second session executor  

## 4. Success metrics

| Metric | Target |
|--------|--------|
| Dual registration drift | Zero intentional differences; tests lock factory sets |
| Boot auto-execution of stale actions | Zero (boot guard + intent retract) |
| Unpermitted tool runs via router | Zero (requires `permitted_action`) |
| Domain Go packages under `internal/shards/*` | Remain absent |
| System shard disable via config/env | Works for both boots |
