# 04 — Architectural Principles: shards

> Last verified against codebase: 2026-07-13  
> Binding principles for work in `internal/shards` and `internal/shards/system`

## P1 — System shards speak facts, not side effects

System shards assert/query kernel facts. Effectful work goes through **constitution-cleared** routes and VirtualStore tools. Never call OS/network APIs from executive or constitution for “convenience.”

## P2 — Default deny

`ConstitutionGateShard` StrictMode: if `permitted` cannot be derived, **block**. Unmapped router actions are also denied (`AllowUnmappedActions=false`).

## P3 — Logic-primary where authority lives

| Shard | Primary mode |
|-------|--------------|
| executive_policy | Mangle evaluation |
| constitution_gate | Mangle + deterministic patterns |
| tactile_router | Static routes + rate limits |
| perception_firewall | LLM (creative transduction) |
| session_planner | LLM (creative decomposition) |

Do not flip executive/constitution to “LLM decides.”

## P4 — Boot guard until human presence

Executive (and VirtualStore) boot guards prevent rehydration storms. Call `DisableBootGuard` only on genuine user/CLI initiation.

## P5 — CostGuard all LLM egress from system shards

Use `GuardedLLMCall` / adapters that check CostGuard. Session and minute caps + error backoff are mandatory for continuous loops.

## P6 — JIT-first for new LLM-facing behavior

New prompts = prompt atoms + assembler selection. Do not grow multi-page string constants in system shards when an atom category exists.

## P7 — No hollow factories

Factories registered without kernel/LLM/VS when those deps are required recreate the hollow-shard bug. Use `RegistryContext` fields; fail loud or no-op safely.

## P8 — Auto vs OnDemand is a safety decision

Auto shards are the OODA spine. OnDemand shards (campaign, planner, router idle) avoid accidental long work. Do not flip `campaign_runner` to Auto without an explicit product decision.

## P9 — Domain personas stay out of this tree

Do not reintroduce `internal/shards/coder` et al. Extend `session` + prompt atoms + Mangle routing instead.

## P10 — Dual registration is temporary debt

Any new system shard must be registered in **both** paths until unified — or added only via `RegisterAllShardFactories` if session_boot is updated to call it exclusively.

## P11 — Repair before persist

Learned Mangle rules pass `MangleRepairShard` / kernel interceptor. Legislator sandboxes synthesis. Invalid rules never become durable policy silently.

## P12 — Wiring audit before deletion

Before removing “unused” registration or manager helpers, grep:

- `RegisterShard("`  
- `DefineProfile("`  
- chat process lookups by config name  
- campaign/CLI factory calls  
- Mangle predicates the shard alone asserts  

Prefer fixing wiring to deleting half-integrated features.
