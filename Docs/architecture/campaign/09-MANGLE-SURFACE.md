# 09 — Mangle Surface: campaign (extra deep-dive)

> Last verified: **2026-08-15**  
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
| `task_soft_dependency/2` | scheduling preference (Decl added 2026-08-15) |
| `requires_resource/2` | resource semaphores (Decl added 2026-08-15) |
| `task_sub_campaign/2` | nested campaign (Decl added 2026-08-15) |
| `task_artifact/...` | outputs |
| `task_inference/...` | provenance |
| `task_attempt/...` | try history |
| `context_compression/...` | phase summaries |

## Risk preflight contract (Section 13 of `campaign_rules.mg`)

Added 2026-08-15. Go measures the preflight, the kernel decides what stops the
campaign, Go enforces the derivation.

**EDB asserted by `assertRiskContractFacts` (risk_gate_contract.go):**

| Predicate | Meaning |
|-----------|---------|
| `campaign_risk_gate_outcome/3` | `(CampaignID, /northstar\|/edge\|/advisory\|/override, /passed\|/blocked\|/skipped)` |
| `campaign_risk_concern/3` | `(CampaignID, Gate, /blocking\|/requires_changes\|/unapproved)` |
| `campaign_protected_surface/2` | campaign targets a protected root |
| `campaign_risk_posture/4` | `(CampaignID, Score, Threshold, /true\|/false)` |
| `campaign_risk_signal/3` | `(CampaignID, /safety_warnings\|/blocked_actions\|/gathering_errors\|/tool_gaps, Count)` |
| `campaign_risk_override/2` | `/force_block` or `/force_allow` |

**IDB derived by Section 13:**

| Predicate | Meaning |
|-----------|---------|
| `campaign_risk_classification_ready/1` | readiness canary — proves to Go that the rules loaded |
| `campaign_risk_critical_signal/1` | safety warnings or blocked actions present |
| `campaign_risk_block/3` | **HARD** stop, with a reason atom |
| `campaign_risk_blocked_gate/2` | fully-bound helper for safe negation |
| `campaign_risk_warning/3` | **SOFT** advisory |
| `campaign_risk_preflight_blocked/1` | any hard block exists |

Reason atoms: `/protected_surface`, `/vision_alignment`,
`/critical_advisor_rejection`, `/force_block`, `/gated_with_critical_signals`
(hard); `/advisory_only`, `/requires_changes`, `/unapproved` (soft).

**Negation gotcha, recorded because it cost a debugging cycle:**
`!campaign_risk_block(C, G, _)` did **not** exclude blocked gates — the wildcard
slot leaves the literal unbound rather than existentially quantified, so the soft
rules fired alongside every hard one. Section 13 negates the fully-bound
`campaign_risk_blocked_gate/2` instead. Other `.mg` files use the wildcard form
(e.g. `!campaign_task_shard_override(TaskID, _)`) and may be over-deriving.

**Fail-safe:** when the canary does not derive, Go uses
`mirrorRiskClassification` — a Go copy of the same contract — rather than
reading "no blocks" out of "no rules".
`TestRiskClassification_KernelAndMirror_ShouldAgree` pins the two together.

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

Exact arity/Decl tables for every predicate live in the defaults corpus and may drift.

Since 2026-08-15 this is **enforced rather than remembered**:
`internal/campaign/types_tofacts_golden_test.go` walks `internal/core/defaults`
for `Decl` lines and fails when a predicate `ToFacts` emits has no Decl, or a
Decl whose arity disagrees. It found three long-standing offenders on its first
run (`task_soft_dependency`, `requires_resource`, `task_sub_campaign`), all now
declared. `testdata/tofacts_predicates.golden` additionally pins argument kinds,
so a slot changing from atom to string — which silently breaks `bound [/name]`
matching — fails the build instead of quietly killing a rule.

This document still lists **observed** campaign usage, not a formal schema dump
of the entire program.
