# system — Architectural Principles

> Last verified: **2026-07-13**  
> These principles are **binding** for changes under `internal/system/`.

## P1 — Motherboard, not actor

`system` constructs the object graph. It must not implement OODA steps, tool semantics, or domain policy rules. If a change wants “when intent X, do Y,” it belongs in kernel policy, VirtualStore handlers, session, or a shard — not here.

## P2 — One complete identity for Cortex

The live cache currently hashes:

```
workspace + provider + apiKey + model
+ normalized(disabledSystemShards)
```

The disabled set is trimmed, emptied, deduplicated, sorted, and shared by hash
and boot behavior. Do not treat the current tuple as complete: separately
configured engine/provider mode must participate in identity or force fresh
boot. Cache keys may hash secret material, but receipts and logs must never
expose it. Do not reintroduce a process-wide unkeyed singleton.

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

Every owned resource opened during boot that holds OS handles or goroutines must
be releasable via `Cortex.Close` and reverse-order rollback when later boot stages
fail. Caller-owned overrides need explicit ownership. New acquisitions require a
matching idempotent close path and a failure-path regression.

## P10 — Maintenance is GetOrBoot-only by design (today)

Background archival runs only when Cortex enters the cache. Direct boot callers own their lifecycle. Do not silently start maintenance from `BootCortexWithConfig` without considering double-tickers on multi-boot.

## P11 — Kernel domains are explicit at boot

`shards.DefaultShardPredicateManifests` is the canonical source for domains and
owned predicates; system converts it into `KernelShardConfig`. Expanding
ownership requires a manifest change plus uniqueness and exact-routing tests —
do not restore a second hard-coded table or invent domain names casually.

## P12 — JIT is non-optional for a complete Cortex

A successful production boot includes JIT compiler + PromptAssembler wiring. Tests may stub Kernel/LLM; they should still accept that JIT init can hard-fail if corpus load fails.

## P13 — Authorization envelopes stay together

`pending_action`, `permitted_action`, `permission_check_result`, and `permitted`
must share the policy shard so Mangle can join one exact action, target, and
payload. `safe_action/1` is classification, not authority. Routers preserve the
executive-issued action ID; adapters must not mint a replacement.

## P14 — Prompt selection never mutates the live executive

Production prompt compilation must create a private RealKernel clone per
compilation. Selector assertions, cleanup, failure, cancellation, and concurrent
compiles must remain inside that scope. Never attach transient prompt facts to
the live Cortex.
