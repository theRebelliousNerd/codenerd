# 02 — Current State: shards

> Last verified against codebase: 2026-07-13  
> Inventory of `internal/shards/` as it exists on disk

## 1. Package stats (approx)

| Kind | Count | Notes |
|------|------:|-------|
| Non-test Go (root) | 5 | registration, matching, consultation, observer_manager, requirements_interrogator |
| Non-test Go (`system/`) | 13 | base, perception, executive(+helpers), constitution, router, world_model, planner, campaign_runner, legislator, mangle_repair, payloads |
| Test Go (root) | 8 | matching, consultation, observer, registration, requirements |
| Test Go (`system/`) | 16 | base, executive, constitution paths, router, perception, planner, repair, action pipeline, etc. |
| Local `.mg` | 0 | Policy lives in core defaults; debug dump may appear as `debug_program_ERROR.mg` under system/ on crash |
| Package README | 1 | **Stale** migration narrative (Dec 2024) |
| Learnings DBs in tree | 3 | `*_learnings.db` artifacts (coder/researcher/reviewer/tester names — historical) |

## 2. File roles

### Root package `codenerd/internal/shards`

| File | Role |
|------|------|
| `registration.go` | Factories, profiles, `RegistryContext`, predicate manifests |
| `matching.go` | Technology patterns, verb execution modes, classifications |
| `consultation.go` | Cross-specialist consultation manager |
| `observer_manager.go` | Background observer events and assessments |
| `requirements_interrogator.go` | Ephemeral Socratic shard |

### Subpackage `codenerd/internal/shards/system`

| File | Role |
|------|------|
| `base.go` | `BaseSystemShard`, `CostGuard`, `AutopoiesisLoop` |
| `perception.go` | Perception firewall |
| `executive.go` | OODA executive loop |
| `executive_intent.go` | Intent snapshots, next_action hydration |
| `executive_autopoiesis.go` | Strategy gap proposals |
| `constitution.go` | Safety gate + appeals |
| `router.go` | Tactile routes + dispatch |
| `world_model.go` | Workspace fact ingestion |
| `planner.go` | Session agenda planner |
| `campaign_runner.go` | Campaign supervisor |
| `legislator.go` | Learned rule legislator |
| `mangle_repair.go` | Learned rule interceptor/repair |
| `payloads.go` | Action payload encode/decode |

## 3. Registered shard names (runtime)

From `registration.go` factories + profiles:

| Name | Class |
|------|-------|
| `perception_firewall` | System Auto |
| `world_model_ingestor` | System OnDemand |
| `executive_policy` | System Auto |
| `constitution_gate` | System Auto |
| `legislator` | System OnDemand |
| `mangle_repair` | System Auto |
| `tactile_router` | System OnDemand |
| `campaign_runner` | System OnDemand |
| `session_planner` | System OnDemand |
| `requirements_interrogator` | Ephemeral |

## 4. Hotspots (complexity / risk)

1. **Dual registration** — `session_boot.go` vs `RegisterAllShardFactories` + factory re-register.  
2. **Executive + constitution + router** — largest behavioral surface; event vs poll fallback.  
3. **Router route table** — long static map; unmapped actions denied by default.  
4. **Mangle repair + legislator** — LLM near policy layer (mitigated by validation pipeline).  
5. **Matching** — string heuristics only; false specialist picks.

## 5. Explicit absences

| Expected historically | Status |
|-----------------------|--------|
| `internal/shards/coder` | **Deleted** |
| `internal/shards/tester` | **Deleted** |
| `internal/shards/reviewer` | **Deleted** |
| `internal/shards/researcher` | **Deleted** |
| `internal/shards/nemesis` | **Deleted** |
| `internal/shards/tool_generator` | **Deleted** (ouroboros via VirtualStore) |

Chat sources still contain commented import stubs documenting the removal.

## 6. Sibling ownership

| Concern | Owner package |
|---------|---------------|
| Spawn/start/stop/profile storage | `internal/core/shards` |
| Task/persona execution | `internal/session` |
| Cortex assembly | `internal/system` |
| Policy rules | `internal/core/defaults/policy` |

## 7. Test entry points

```powershell
go test ./internal/shards/ -count=1
go test ./internal/shards/system/ -count=1
```
