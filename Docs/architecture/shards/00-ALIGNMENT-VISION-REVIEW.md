# 00 — Alignment & Vision Review: shards

> Last verified against codebase: 2026-07-13  
> Status: Living Reference Document — code-grounded  
> Source: `internal/shards/`, `internal/shards/system/`

## 1. North-star statement

codeNERD’s north star: **LLM as creative center, Mangle kernel as executive**. Shards are the **long-running agents and factories** that realize the OODA loop around that kernel — they must **not** re-centralize executive control into opaque Go or LLM glue.

This package is healthy when:

1. User meaning becomes **facts** (`user_intent`) via perception, not free-form side effects.  
2. Actions only execute after **`permitted(...)`** / constitution gate (default deny).  
3. Domain coding personas are **JIT + session**, not hard-coded Go shards.  
4. System shards stay **logic-primary** where safety or routing is concerned.  
5. Registration is complete (no hollow shards missing kernel/LLM/VS).

## 2. Alignment dimensions

| Dimension | Score (0–5) | Evidence |
|-----------|-------------|----------|
| Creative/executive split | **5** | Executive/constitution/router logic-primary; perception/planner LLM (`system/executive.go`, `constitution.go`, `perception.go`) |
| Fact-flow fidelity | **5** | Explicit pipeline `pending_action` → `permitted_action` → tools (`executive.go`, `constitution.go`, `router.go`) |
| Constitutional safety | **5** | StrictMode, dangerous patterns, network allowlist, appeals (`constitution.go`) |
| JIT discipline | **4** | PromptAssembler on system shards; interrogator JIT-required; some legacy fallbacks remain |
| Lifecycle clarity | **4** | Auto vs OnDemand profiles (`registration.go`); dual boot paths risk drift |
| Domain shard retirement | **5** | Domain packages deleted; comments + JIT path in `session_boot.go` |
| Specialist orchestration | **3** | Matching/consultation/observers solid but pattern-only matching; partial chat/campaign use |
| Test grounding | **3** | Good unit coverage on base/matching/observers; hot paths (full OODA with real kernel) thinner |
| Observability | **4** | CategorySystemShards logging, heartbeats, GlassBox/ToolEventBus hooks |
| Registration completeness | **4** | RegistryContext DI; re-register overrides in factory for browser/campaign |

**Overall alignment: 4.2 / 5** — System OODA shards embody the north star; residual risk is dual-wiring drift, stale package README, and incomplete predicate-manifest consumption.

## 3. What “good” looks like (shards-specific)

| Good | Bad |
|------|-----|
| Constitution queries `permitted` then emits `permitted_action` | Router runs tools from raw `next_action` |
| Boot guard holds fire until first user turn | Rehydrated intents auto-execute campaigns |
| CostGuard wraps LLM on system shards | Unbounded perception loops burn tokens |
| New LLM behavior via atoms + assembler | Hard-coded multi-KB prompts inside shards |
| Domain work via `session.Executor` personas | Reintroducing `internal/shards/coder` |

## 4. Related corpora

- `Docs/architecture/core/` — Kernel, VirtualStore, ShardManager  
- `Docs/architecture/session/` — JIT clean executor  
- `Docs/architecture/perception/` / `articulation/` / `prompt/`  
- `Docs/architecture/campaign/` — long-horizon orchestration  
- `Docs/architecture/cli/` — boot surfaces that register these shards  
