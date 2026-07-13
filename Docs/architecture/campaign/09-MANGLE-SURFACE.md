# 09 — Mangle Surface: campaign (extra deep-dive)

> Last verified: **2026-07-13**  
> Campaign Go package asserts/queries facts; rules live mainly under `internal/core/defaults/`.

## Layering (from `campaign_rules.mg` header)

```
policy.mg (Section 19)  → base campaign state machine, task selection
build_topology.mg       → architectural layer enforcement via phase_category
campaign_rules.mg       → advanced scheduling, quality, learning
Go types.ToFacts()      → ground facts from durable Campaign JSON
```

## Ground facts emitted by Go (`types.go`)

| Predicate (arity as used) | Source |
|---------------------------|--------|
| `campaign/5` | Campaign ID, type, title, source, status |
| `campaign_metadata/4` | ID, created unix, phase count, confidence×100 |
| `campaign_goal/2` | ID, goal text |
| `campaign_progress/5` | phases/tasks completed totals |
| `source_document/...` | ingested docs |
| `campaign_phase/6` | phase identity + status + profile |
| `phase_category/2` | topology category |
| `phase_objective/4` | objectives + verification method |
| `phase_dependency/3` | hard/soft/artifact deps |
| `phase_estimate/3` | tasks + complexity |
| `campaign_task/5` | task identity + status + type |
| `task_priority/2` | priority |
| `task_order/2` | deterministic order |
| `task_dependency/2` | hard deps |
| `task_soft_dependency/2` | soft deps |
| `requires_resource/2` | resource semaphore names |
| `task_sub_campaign/2` | nested campaign |
| `task_artifact/...` | outputs |
| `task_inference/...` | provenance |
| `task_attempt/...` | try history |
| `context_compression/...` | phase summaries |

## Runtime asserts (orchestrator)

| Predicate | When |
|-----------|------|
| `campaign_task` retract/assert | status transitions |
| `campaign_phase` retract/assert | phase start/complete |
| `task_error` | failures / max retries |
| `replan_trigger` | checkpoint fail / failure paths |
| `campaign_heartbeat` | heartbeat loop |
| config/budget related asserts | `assertCampaignConfigFacts` (utils) |
| `phase_context_atom` / `activation` | ContextPager |
| intelligence-related seeds | decomposer `seedIntelligenceFacts` / `seedDocFacts` |

## Runtime queries (orchestrator)

| Predicate | Purpose |
|-----------|---------|
| `current_phase` | active phase id |
| `eligible_task` | runnable tasks |
| `next_campaign_task` | preferred next task |
| `phase_eligible` | next phase to start |
| `campaign_blocked` | hard stop reason |
| `phase_context_atom` | compress/filter |
| replan triggers (via helper) | replanner context |

## Derived intelligence (`campaign_rules.mg` samples)

Illustrative rule families (not exhaustive — read the `.mg` file):

| Family | Examples |
|--------|----------|
| Goal complexity | `goal_requires_campaign`, `simple_goal`, `recommend_downgrade` |
| Phase count heuristics | `campaign_too_ambitious`, `campaign_too_trivial`, `decomposition_warning` |
| Confidence | `plan_needs_review`, `plan_can_autostart`, `next_action(/campaign_clarify)` |
| Scheduling quality | parallel interference, eligible selection helpers |
| Learning | `campaign_task_error`, error counts, failure patterns |
| Shard reliability | `shard_campaign_success/failure/reliable` |
| Milestones | phase completion milestone rules |

## Topology coupling

`phase_category` facts feed `build_topology.mg` so campaigns respect architectural layering (e.g., do not schedule UI-only phases that violate substrate rules). Categories normalized in `Phase.ToFacts`.

## Decl / variable discipline

When adding predicates:

1. Add `Decl` in schemas/policy corpus.  
2. Use `/lowercase` atoms for statuses/types (already encoded in Go string constants).  
3. Variables Uppercase in rules.  
4. Negation only after positive binding.  
5. Aggregations use `|> do ... let ...` pipeline syntax.

## Failure dump

If the kernel crashes, `debug_program_ERROR.mg` may appear (repo root Agents.md). Package-local copy can exist after failed runs for debugging combined sources — treat as diagnostic artifact, not source of truth.

## Testing Mangle interaction

Unit tests typically mock `core.Kernel` (`mocks_test.go`, thread-safe variants). Policy correctness needs kernel program load tests or `nerd mangle-check` style tooling at repo level.

## Gap honesty

Exact arity/Decl tables for every predicate live in the defaults corpus and may drift. When Go asserts a new fact, **grep** `internal/core/defaults` for Decl before shipping. This document lists **observed** campaign usage, not a formal schema dump of the entire program.
