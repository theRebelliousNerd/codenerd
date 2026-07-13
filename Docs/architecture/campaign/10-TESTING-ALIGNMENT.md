# 10 — Testing Alignment: campaign

> Last verified: **2026-07-13**

## How to run

```powershell
$env:CGO_CFLAGS = "-IC:/CodeProjects/codeNERD/sqlite_headers"
go test ./internal/campaign/...

# Focused
go test ./internal/campaign/ -run Orchestrator -count=1
go test ./internal/campaign/ -run Assault -count=1
go test ./internal/campaign/ -run Risk -count=1
go test ./internal/campaign/ -run ContextPager -count=1

# Cross-package
go test ./tests/e2e/ -run Campaign -count=1
```

## Test inventory (by area)

| File pattern | Focus |
|--------------|-------|
| `orchestrator_behavior_test.go` | Endish behavior of orchestrator loops with mocks |
| `orchestrator_di_test.go` | Constructor/setters DI |
| `orchestrator_init_validation_test.go` | Config validation edges |
| `orchestrator_phases_test.go` | Phase transitions / queries |
| `orchestrator_failure_test.go` | Retry/backoff/escalation |
| `orchestrator_journal_test.go` | Journal durability |
| `orchestrator_task_handlers_test.go` | Handler routing |
| `orchestrator_task_transaction_test.go` | Transactional task updates |
| `orchestrator_write_set_gating_test.go` | Lock gating under parallel schedule |
| `decomposer_test.go` / helpers | Plan pipeline pieces |
| `context_pager_test.go` | Budget, activate, compress (incl. ThreadSafeMockKernel) |
| `checkpoint_integration_test.go` / parsers | Verification + output parse |
| `replan_test.go` | Replan application |
| `risk_scoring_test.go` | Score/gates |
| `assault_helpers_test.go` / `assault_tasks_test.go` | Assault utilities/handlers |
| `intelligence_*_test.go` | Gatherer + gaps |
| `edge_case_*_test.go` | Detector + gaps |
| `tool_pregenerator_test.go` | Pregen |
| `shard_advisory_board_test.go` | Votes/synthesis |
| `write_set_lock_manager_test.go` | Lock manager unit |
| `types_test.go` | Fact emission / model |
| `checkpoint_parsers_test.go` | go test JSON / jest parse |
| `mocks_test.go` / `main_test.go` | Shared fakes |

## What is well covered

- Config validation and defaults  
- Write-set lock timeouts and exclusivity  
- Journal append/checksum patterns (unit)  
- Risk score math and gate toggles  
- Context pager budget math and activation with mock kernel  
- Failure retry bookkeeping  

## What is thinly covered / gaps

| Gap | Why it matters |
|-----|----------------|
| Full multi-phase Run with real Mangle program | Eligibility rules live outside package |
| Nested `campaign_ref` policies | Complex failure inheritance |
| Assault on non-Go monorepos | Discover paths differ |
| Intelligence gatherer with all 12 systems live | Heavy integration |
| Advisory hard-block end-to-end | Contract incomplete |
| Chaos: kill mid-write recovery | Journal designed for it; needs stress harness |
| PromptProvider JIT path | Mostly CLI-level |

## Testing principles for this package

1. Prefer mock kernel for unit speed; reserve real kernel for e2e.  
2. When testing eligibility, assert facts the real rules expect — or inject derived facts directly if mocking Query.  
3. Always clean temp `.nerd/campaigns` directories.  
4. Parallel tests must not share write-set workspace paths.  
5. Do not network-call real LLMs in unit tests — inject `MockLLMClient`.

## Alignment with vision

Vision J1–J3 need at least one e2e each:

| Journey | Current support |
|---------|-----------------|
| Spec feature campaign | package + e2e partial |
| Knowledge-base greenfield | knowledge ingest unit/soft |
| Assault | unit heavy; operator e2e via chat/campaign |

## Recommended additions (no estimates)

1. Golden-file tests for `ToFacts` predicate sets.  
2. Journal crash-mid-rename simulation.  
3. Phase checkpoint fail → replan_trigger presence assertion.  
4. Assault discover idempotency (already coded — lock in test).  
5. Risk force_block prevents Run start integration test.
