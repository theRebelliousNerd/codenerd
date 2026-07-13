# system — Architectural Principles

> Last verified: **2026-07-13**  
> These principles are **binding** for changes under `internal/system/`.

## P1 — Motherboard, not actor

`system` constructs the object graph. It must not implement OODA steps, tool semantics, or domain policy rules. If a change wants “when intent X, do Y,” it belongs in kernel policy, VirtualStore handlers, session, or a shard — not here.

## P2 — One identity tuple for Cortex

Identity dimensions are exactly:

```
workspace + provider + apiKey + model
```

Cache keys hash that tuple (SHA-256 over NUL-joined fields). Do not reintroduce a process-wide unkeyed singleton. Do not put workspace alone in the key.

## P3 — Never cache failures

If `BootCortex` returns an error, `GetOrBootCortex` must not insert into `cortexCache`. Transient env/config/embedding failures must remain retryable.

## P4 — Prefer GetOrBootCortex at the edge

All long-lived CLI handlers should call `GetOrBootCortex`, not bare `BootCortex`. Direct `BootCortexWithConfig` is reserved for:

- tests with DI overrides  
- TUI (current exception — migrate carefully)  
- one-shot tools that intentionally avoid cache (document why)

## P5 — Soft periphery, hard core

| Hard fail | Soft warn |
|-----------|-----------|
| Kernel shard create/register/Evaluate | Embedding health |
| JIT compiler construction | MCP bridge init / ConnectAll |
| System shard StartSystemShards | Taxonomy defaults / hydrate |
| Embedded corpus load for JIT | Hybrid prompt ingest; agent sync; modular tools hydrate |

Do not promote soft-fail subsystems to hard-fail without an explicit product decision.

## P6 — LLM clients are scheduled and role-split

- Main agent: `core.NewScheduledLLMCall("main", …)`  
- Shards / spawn / create: worker client when configured, else main  
- Image generation: dedicated client; never the Ollama worker  

API concurrency is global via `core.ConfigureGlobalAPIScheduler` from user config policy.

## P7 — Adapters only at package boundaries

Adapters exist to prevent import cycles and satisfy foreign interfaces. Prefer thin forwarding. When parsing Mangle fact strings (MCP / KernelAdapter), keep constant-type handling consistent with `core.MangleAtom` for name constants.

## P8 — Wiring before deletion

Before removing a factory step, adapter, or field on `Cortex`:

1. Grep CLI/TUI for field use  
2. Grep VirtualStore setters  
3. Grep shard registration  
4. Confirm no dormant call site expects the wire  

This package *is* where “unused” often means “wired but feature-flagged.”

## P9 — Lifecycle is part of the contract

Every resource opened during boot that holds OS handles (SQLite, spawn queues, shards) must be releasable via `Cortex.Close`. New boot resources need a matching Close path.

## P10 — Maintenance is GetOrBoot-only by design (today)

Background archival runs only when Cortex enters the cache. Direct boot callers own their lifecycle. Do not silently start maintenance from `BootCortexWithConfig` without considering double-tickers on multi-boot.

## P11 — Kernel domains are explicit at boot

The motherboard declares which domains own which predicates (routing, world, tools, policy, campaign, prompts, cortex). Expanding ownership requires coordinated changes with `core` kernel sharding — do not invent domain names casually.

## P12 — JIT is non-optional for a complete Cortex

A successful production boot includes JIT compiler + PromptAssembler wiring. Tests may stub Kernel/LLM; they should still accept that JIT init can hard-fail if corpus load fails.
