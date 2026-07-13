# 09 — Safety and Invariants: shards

> Last verified against codebase: 2026-07-13  
> Constitutional safety, concurrency, Mangle surface used by system shards

## 1. Constitutional core

### Invariant S1 — No effect without permission

Router consumes **`permitted_action`**, not raw `pending_action` or `next_action`. Constitution is the only producer of the permitted stream (plus appeal overrides internal to the gate).

### Invariant S2 — Default deny

`StrictMode=true` by default. Failed `permitted` query → deny. Unmapped tool routes denied.

### Invariant S3 — Deterministic danger filters

Regex dangerous patterns and domain allowlists run **before / alongside** Mangle and do not require LLM.

Default dangerous patterns include: `rm -rf`, `mkfs`, `dd if=`, `chmod 777`, `curl|sh`, `wget|sh`, writes into `/etc`, `sudo rm`.

Default allowed domains include github.com, golang.org, pkg.go.dev, docs.anthropic.com, developer.mozilla.org.

### Invariant S4 — Appeals are explicit

Blocked actions can emit `appeal_available`. Overrides require `SubmitAppeal` + `HandleAppeal`. Temporary overrides expire.

## 2. Boot and rehydration safety

### Invariant B1 — Boot guard

`ExecutivePolicyShard.bootGuardActive` starts **true**. Derived actions suppressed until `DisableBootGuard()`.

### Invariant B2 — Stale fact retract

On executive Execute start, retracts:

- `user_intent`  
- `processed_intent`  
- `executive_processed_intent`  
- `pending_action`  

Prevents infinite OODA loops from persisted kernel state.

### Invariant B3 — Campaign not auto-start

`campaign_runner` profile is **OnDemand** specifically to avoid automatic campaign execution on boot.

## 3. Cost and loop safety

### Invariant C1 — CostGuard on LLM

Base defaults: 10/min, 100/session, exponential cooldown on errors, validation budget 20.

### Invariant C2 — Action storm cap

Executive `MaxActionsPerTick` default 5.

### Invariant C3 — Heartbeat budget

Executive/constitution use multi-second ticks and 15s heartbeats to avoid saturating kernel evaluate under large EDBs.

### Invariant C4 — Autopoiesis wait

Executive waits `autopoiesisWg` before Completed so proposal goroutines do not race replacements.

## 4. Concurrency invariants

| Area | Mechanism |
|------|-----------|
| BaseSystemShard fields | `sync.RWMutex` |
| Observer EventsReceived | `atomic` int64 |
| Consultation batch | WaitGroup + channels |
| Router rate limiters | per-tool mutex |
| CostGuard | mutex |

Event channels must be unsubscribed on Stop to avoid bus leaks.

## 5. Mangle surface (shard-relevant)

Shards **consume and assert** facts; Decl lives in core schemas/policy.

### Asserted / processed by system shards (representative)

| Predicate | Producer | Consumer |
|-----------|----------|----------|
| `user_intent` | perception (and chat) | executive, policy |
| `next_action` | policy / strategies | executive |
| `pending_action` | executive | constitution |
| `permitted_action` | constitution | router |
| `permission_check_result` | constitution | observability |
| `security_violation` | constitution | audit / UI |
| `appeal_available` | constitution | appeal UX |
| `exec_request` | router | VirtualStore |
| `routing_result` | constitution (failure) | waiters |
| `executive_blocked` / `executive_error` | executive | ops |
| `ooda_timeout` | executive | stall detection |
| `strategy_activated` | executive | metrics |
| `campaign_runner_heartbeat` | campaign_runner | health |
| heartbeats (`EmitHeartbeat`) | system shards | health rules |

### Queried

| Predicate | Querier |
|-----------|---------|
| `permitted` | constitution |
| `next_action` (+ variants) | executive |
| `pending_action` | constitution |
| `permitted_action` | router |
| `pending_intent` | executive OODA timeout |
| barriers / strategies | executive |

### Learned rule gate

`MangleRepairShard` validation pipeline: parse → safety (incl. unbound vars / unsafe negation) → corpus schema → stratification. Max 3 LLM repairs then reject. Kernel interceptor prevents invalid persistence.

Legislator synthesizes with `SynthModeRequire` single-clause options; schema JSON when provider supports it.

## 6. Permission profiles (least privilege)

| Shard | Permissions (profile intent) |
|-------|------------------------------|
| constitution_gate | AskUser only |
| executive_policy | Read + CodeGraph + AskUser |
| perception_firewall | Read + AskUser |
| tactile_router | Exec + Network + Browser |
| campaign_runner | Read + Write + Exec |

Do not grant WriteFile to constitution or executive profiles.

## 7. Testing safety properties

- Action pipeline test asserts pending → routing path (`system/action_pipeline_test.go`)  
- Constitution / router unit tests cover permit/deny and escalation edges  
- Boot guard behavior covered in VS/e2e tests outside this package  

When changing check order in `checkPermitted`, add regression tests for dangerous patterns and StrictMode query failure.
