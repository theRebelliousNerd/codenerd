# transparency — Testing Alignment

> Last verified: 2026-07-13

## 1. Commands

```powershell
# Package unit tests
go test ./internal/transparency/...

# Race detector (recommended when touching buses / Manager)
go test -race ./internal/transparency/...

# Verbose single test
go test ./internal/transparency/ -run TestGlassBoxEventBusEmitImmediate -v
```

Consumer spot checks (not package-owned):

```powershell
go test ./cmd/nerd/chat/ -count=1 -run "GlassBox|ToolEvent|Transparency"
go test ./internal/core/shards/ -count=1 -run "Transparency|GlassBox"
```

## 2. Test file map

| File | Approx focus |
|------|----------------|
| `error_classifier_test.go` | Safety/timeout classify; recovery guide unknown |
| `event_bus_test.go` | Immediate emit, category filter, flush, unsubscribe, clear turn |
| `explainer_test.go` | Trace/fact/decision, QuickExplain, operation summary |
| `glass_box_events_test.go` | Event string formatting, ValidCategory, ToolEventBus |
| `glass_box_helpers_test.go` | Category string/prefix, HasDetails, verbose bus, explainer setters |
| `safety_reporter_test.go` | Destructive/secret classification, ExplainSafetyAction |
| `shard_observer_test.go` | Lifecycle with observer, phase string, durations |
| `transparency_test.go` | Manager enable/toggle, status, format error |
| `transparency_comprehensive_test.go` | Table-driven sweep: categories, classify, observer matrix, safety matrix, manager gates |

## 3. Coverage strengths

| Area | Strength |
|------|----------|
| Error category Prefix/String | Comprehensive + out-of-range |
| ClassifyError patterns | Multi-case table in comprehensive |
| ShardObserver enable/disable notify | Explicit tests |
| SafetyReporter classify branches | Destructive, secret, protected, resource, policy, disabled |
| Manager gate no-ops when disabled | Start/Update/End/Report |
| Event bus filter + flush + unsubscribe | Present |
| ToolEventBus basic emit/subscribe | Present |

## 4. Coverage gaps

| Gap | Risk | Suggested test |
|-----|------|----------------|
| Concurrent Emit under load / drop path | Silent loss; races | goroutine storm + Stats (once drops added) |
| SafetyReporter concurrent ReportViolation | Data race | `-race` with parallel Report |
| End-to-end VS → bus → subscriber | Wiring regressions | core or chat integration |
| Explainer with deep trees / maxDepth | Truncation UX | fixture trace depth > max |
| ClearTurn interaction with flush timer | Subtle batch bugs | timer + ClearTurn race |
| Real kernel deny → SafetyReporter | Product gap | integration once auto-feed exists |
| Config flag StreamReasoning/JITExplain | Documented non-effect | status-only assertion until wired |

## 5. Alignment with principles

| Principle | Tested? |
|-----------|---------|
| Disabled bus no-op | Yes (enable tests) |
| Category filter | Yes |
| Manager master gate | Yes |
| Non-blocking (drop) | Indirect (full channel hard to assert without timing) |
| Always-on ToolEvent | Unit only; chat e2e separate |
| Explain nil-safety | Yes (nil trace messages) |

## 6. Naming conventions observed

Comprehensive tests use `When_Should` style:

`TestShardObserver_StartExecution_WhenEnabled_ShouldTrackAndNotify`

Older files use shorter names (`TestClassifyErrorSafety`). Prefer When_Should for new tests.

## 7. Fixtures

Explainer tests construct `mangle.DerivationTrace` / nodes in-process (see `explainer_test.go`). No external golden files required.

## 8. CI expectation

`go test ./internal/transparency/...` should be green on every PR touching:

- event shapes  
- Manager gates  
- classifier heuristics  
- bus batching  

If changing `GlassBoxEvent` fields, also run chat Glass Box tests.

## 9. Manual verification (operator)

1. Boot chat → confirm tool lines appear without `/glassbox`.  
2. `/glassbox` / verbose → subsystem lines stream.  
3. `/transparency on` → status table.  
4. `/why next_action` (with kernel state) → Explainer markdown.  
5. Force a blocked destructive action → observe Tool/Routing failure line; check whether SafetyViolation appears (gap awareness).
