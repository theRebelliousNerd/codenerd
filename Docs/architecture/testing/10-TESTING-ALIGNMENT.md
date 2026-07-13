# testing — Testing Alignment

> Last verified: 2026-07-13  
> “Testing the testing package”

## What exists

### Unit tests (`go test ./internal/testing/...`)

| File | Coverage focus |
|------|----------------|
| `metrics_test.go` | Finalize averages, F1, ratio |
| `reporter_test.go` | JSON encode shape; console contains status markers |
| `simulator_test.go` | Fact score/reason, convert activations, retrieval metrics, floors, checkpoint fallback, meetsExpectations, contains/sort helpers |
| `file_logger_test.go` | Creates session files + MANIFEST |
| `seeder_logger_test.go` | FactSeeder issue/graphs; FileLogger writers non-nil |
| `feedback_test.go` | FeedbackTracer output; PiggybackTracer feedback; context feedback store integration; score impact |
| `helpers_extra_test.go` | estimateFactTokens, formatFloat, ActivationValidationError message |
| `tracer_helpers_test.go` | formatNumber, truncate, sumTokens, groupByCategory, percent, FactSeeder campaign |

These tests **do not** run full 50–100 turn scenarios end-to-end.

### CLI / integration (manual or external CI)

```powershell
go test ./internal/testing/...

$env:CGO_CFLAGS = "-IC:/CodeProjects/codeNERD/sqlite_headers"
go build -o nerd.exe ./cmd/nerd
.\nerd.exe test-context --scenario debugging-marathon --mode=mock
.\nerd.exe test-context --all --mode=mock --console=false
.\nerd.exe test-context --scenario campaign-phase-transition --mode=real
.\nerd.exe test-context --scenario debugging-marathon --mode=real --live
```

## Alignment of scenarios to package tests

| Scenario class | Unit-tested? | CLI-exercised? |
|----------------|--------------|----------------|
| Mock multi-turn | Indirect (helpers only) | Yes (primary) |
| Integration validators | Types tested lightly | Real mode manual |
| Feedback learning | Tracer/store tests | Live/real manual |
| File logging | Yes | Yes |

## Gaps in self-test

1. **No automated full-scenario golden test** in `go test` (would need lighter kernel or heavy boot).  
2. **Advanced checkpoint validators untested** because unenforced.  
3. **GetScenario map completeness** not tested (feedback-learning omission would not fail unit tests).  
4. **CLI category flag** has no test.  
5. **RealIntegrationEngine** largely untested under `go test` (needs deps).  
6. **Cross-scenario Reset isolation** not tested.  
7. **README vs code thresholds** not linted.

## Recommended verification bar for harness changes

| Change type | Minimum bar |
|-------------|-------------|
| Helper / metrics / reporter | `go test ./internal/testing/...` |
| Mock scoring / fuzzy match | unit + one mock scenario CLI |
| Real activation mapping | real-mode scenario CLI |
| Live piggyback | `--live` single short scenario |
| New scenario | unit for any new helper + CLI run + registry completeness check |
| FileLogger channels | unit + inspect session dir |

## Suggested future tests (backlog-aligned)

```go
// Pseudocode intent — not implemented in this docs-only task
func TestGetScenarioRegistryComplete(t *testing.T) {
  for _, s := range AllScenarios() {
    if GetScenario(s.ScenarioID) == nil { t.Fatalf(s.ScenarioID) }
  }
}

func TestValidateCheckpointHonorsActivation(t *testing.T) { ... }
func TestHarnessRunAllResetsEngine(t *testing.T) { ... }
```

## Relation to repo-wide testing

| Surface | Owner |
|---------|-------|
| Package unit tests | each `internal/*` |
| Context long-horizon | **this package** + `nerd test-context` |
| Campaign assault | campaign subsystem / chat `/campaign assault` |
| Mangle check | `nerd mangle-check` / mangle tools |
| Full `go test ./...` | pre-handoff bar from Agents.md |

This package’s unit tests should always pass under plain `go test` without GPU or API keys. Real/live remain optional gates.
