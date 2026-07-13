# 10 — Testing Alignment: Northstar

> Last verified against codebase: 2026-07-13  
> Package: `internal/northstar`

## 1. Commands

```powershell
go test ./internal/northstar/...
go test -race ./internal/northstar/...
go test -count=1 ./internal/northstar/... -run TestGuardian
```

Campaign-side integration (consumer tests, not in package):

```powershell
go test ./internal/campaign/... -run Northstar
```

## 2. Coverage by file

| Test file | ≈Lines | What it grounds |
|-----------|-------:|-----------------|
| `guardian_test.go` | 1103 | Construction, init, vision copy, all alignment branches, observe, ShouldCheckNow, parse, concurrency, path match |
| `store_test.go` | 710 | NewStore, vision round-trip timestamps, observations, checks, drift resolve, state counters |
| `types_test.go` | 623 | JSON enums and config defaults |
| `observer_test.go` | 514 | Campaign/task/background happy paths and assessment mapping |
| `types_facts_test.go` | 114 | Priority/impact maps, `ToFacts` shape |
| `guardian_warn_test.go` | 71 | No-vision Warn logging behavior |

**Strength:** library is heavily unit-tested (~3.1k test lines vs ~2.2k product lines).

## 3. Notable test scenarios (selection)

| Scenario | Test name pattern |
|----------|-------------------|
| Nil store does not panic on NewGuardian | `TestNewGuardian_NilStore` |
| Concurrent Initialize | `TestGuardian_Initialize_Concurrent` |
| GetVision returns defensive copy | `TestGuardian_GetVision_ReturnsCopy` |
| No vision skip | `TestGuardian_CheckAlignment_NoVision` |
| No LLM soft pass + history | `TestGuardian_CheckAlignment_NoLLM*` |
| LLM parse SCORE/RESULT | `TestGuardian_ParseAlignmentResponse*` |
| Explicit RESULT wins over score class | `TestGuardian_ParseAlignmentResponse_ExplicitResultWins` |
| Drift updates state | `TestGuardian_CheckAlignment_DriftRefreshesGuardianState` |
| High-impact nested wildcard | `TestGuardian_CalculatePathRelevance_WildcardMatchesNestedFile` |
| SaveVision preserves created_at | `TestStore_SaveVision_PreservesCreatedAtOnUpdate` |
| Background handler records observation | `TestBackgroundEventHandler_HandleEvent_RecordsObservation` |

## 4. Gaps

| Gap | Severity |
|-----|----------|
| No integration test that chat boot wires kernel + guardian facts end-to-end | Medium |
| No test that wizard JSON and SQLite stay consistent (because they don’t) | High product gap |
| Mitigation fact always `/mitigation` — no regression forcing richer encoding | Low |
| `ingested_docs` untested (no API) | n/a |
| Live LLM network tests absent (correct; mocks only) | Expected |
| Race tests on CampaignObserver under parallel campaign hooks | Low |

## 5. Recommended additions (doc backlog only)

1. Boot smoke: shared boot asserts `northstar_defined` when vision in store.
2. Adapter round-trip test in `cmd/nerd/chat` for `northstarHandlerAdapter`.
3. Golden `ToFacts` snapshot if policy arity changes.

## 6. Quality assessment

| Dimension | Rating |
|-----------|--------|
| Unit density | **Excellent** |
| Concurrency | **Good** (guardian) |
| Cross-package wiring | **Weak** |
| Property/fuzz | **None** |

Package testing is a **strength** relative to many internal packages; residual risk is integration, not unit logic.
