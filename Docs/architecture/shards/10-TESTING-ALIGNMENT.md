# 10 — Testing Alignment: shards

> Last verified against codebase: 2026-07-13

## 1. Commands

```powershell
# Full shards tree
go test ./internal/shards/...

# Root package only
go test ./internal/shards/ -count=1

# System subpackage
go test ./internal/shards/system/ -count=1

# Focused
go test ./internal/shards/ -run Matching -count=1
go test ./internal/shards/system/ -run Constitution -count=1
go test ./internal/shards/system/ -run Executive -count=1
go test ./internal/shards/system/ -run ActionPipeline -count=1
```

## 2. Existing test map

### Root package

| File | Focus |
|------|-------|
| `registration_test.go` | `RegisterAllShardFactories`, system profiles |
| `matching_test.go` | `MatchSpecialistsForTask` table, `GetExecutionMode` |
| `matching_classification_test.go` | execute/advisor predicates, patterns |
| `consultation_test.go` | request/response, advisors, format advice |
| `observer_manager_test.go` | start/stop, register, events, levels |
| `observer_manager_accessors_test.go` | callbacks, assessments |
| `observer_integration_test.go` | concurrency, Northstar handler, shutdown |
| `requirements_interrogator_test.go` | no-LLM fallback, setters, extract questions |

### system package

| File | Focus |
|------|-------|
| `base_coverage_test.go` | CostGuard, AutopoiesisLoop, BaseSystemShard DI/JIT/guarded LLM |
| `executive_coverage_test.go` / `executive_helpers_test.go` / `executive_ooda_test.go` | Executive paths |
| `constitution_coverage_test.go` | Gate coverage |
| `router_escalation_test.go` / `router_route_selection_test.go` | Routing |
| `perception_validation_test.go` / `perception_transient_test.go` | Perception |
| `planner_test.go` | Planner |
| `mangle_repair_test.go` / bench | Repair pipeline |
| `action_pipeline_test.go` | Pending → routing pipeline |
| `policy_action_routes_test.go` | Policy/route alignment |
| `learning_test.go` | Learning patterns |
| `system_helpers_test.go` | Shared helpers |

## 3. Coverage strengths

- CostGuard math and cooldown behavior well unit-tested  
- Matching classifications and verb modes table-tested  
- Observer concurrency/integration tests present  
- Payload encode/decode covered via base tests  
- Registration factory smoke tests  

## 4. Coverage gaps

| Area | Gap |
|------|-----|
| Full multi-shard OODA under CortexKernel | Mostly e2e outside package |
| Dual registration set equality (factory vs session_boot) | **Missing** |
| Predicate manifest consumption | **Missing** (feature partial) |
| Campaign runner restart backoff | Thin |
| Legislator sandbox ratification end-to-end | Thin |
| Perception classification client tiering | Limited |
| Router full route table vs VirtualStore handlers | action_linter tool helps; not complete |
| Long-running Execute loops with cancellation | Partial |

## 5. Recommended tests (backlog)

1. **Factory set lock:** collect `RegisterAllShardFactories` names vs names registered in a test double of session_boot list — fail on drift.  
2. **Boot guard:** executive with pending next_action produces zero `pending_action` until DisableBootGuard.  
3. **Strict deny:** constitution with empty permitted EDB blocks write_file pending_action.  
4. **Dangerous pattern:** pending shell target with `rm -rf` never yields permitted_action.  
5. **Repair interceptor:** invalid rule rejected; valid Decl-using rule accepted (uses mangle engine).  

## 6. Relation to e2e

`tests/e2e/*` exercises VirtualStore boot guard and kernel/VS integration. Treat those as complementary, not substitutes for package unit tests.
