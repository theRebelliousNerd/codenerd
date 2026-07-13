# testing — Wiring and Integration

> Last verified: 2026-07-13

## Registration

### Cobra command

| Item | Location |
|------|----------|
| Command var | `testContextCmd` in `cmd/nerd/cmd_test_context.go` |
| Registration | `rootCmd.AddCommand(testContextCmd)` in `init()` |
| Use | `test-context` |
| RunE | `runTestContext` |

No other binary or shard registers the harness.

### Package anchor

`internal/testing/harness_subsystem.go` comments that the file is an “integration anchor point” — it does **not** register with the shard manager or VirtualStore. Treat as documentation residue, not a boot hook.

## Boot sequence (`runTestContext`)

```
1. context.WithTimeout(30m)
2. Resolve API key (flag or ZAI_API_KEY)
3. NewFileLogger(log-dir, console?)
4. Construct optional tracers from flags (defaults: on)
5. Print observability banner
6. coresys.GetOrBootCortex(ctx, workspace, key, nil)
7. Mode: mock | real (live forces real)
8. Type-assert cortex.Kernel → *core.RealKernel (fail if not)
9. Engine:
     real → NewRealIntegrationEngine(kernel, LocalDB, LLMClient, DefaultConfig())
     mock → NewMockContextEngine(kernel)
10. NewHarnessWithObservability(..., summaryWriter, format, tracers..., engine)
11. If no --scenario and not --all → ListScenarios + usage help
12. RunAll or RunScenario
13. PromptInspector.Summary() if present
14. Non-zero exit via returned error if failures
15. defer cortex.Close(); fileLogger.Close() → session path printed
```

## Wiring that works

| Wire | Status |
|------|--------|
| CLI → FileLogger → tracer writers | **Live** |
| CLI → Cortex kernel/store/LLM | **Live** |
| CLI → Mock/Real engine → Harness | **Live** |
| Harness → Simulator → engine Compress/Retrieve | **Live** |
| Simulator → Reporter via Harness | **Live** |
| Real RetrieveContext → ActivationEngine.ScoreFacts | **Live** |
| Live GenerateAssistantResponse → LLMClient | **Live** when `--live` |

## Wiring gaps / partials

| Intended wire | Status | Notes |
|---------------|--------|-------|
| `--category` → filtered run set | **Gap** | Flag stored in `testContextCategory`; never read in `runTestContext` |
| Integration scenario `context-feedback-learning` → `GetScenario` | **Gap** | Present in `AllScenarios` / Integration list only |
| Checkpoint `ValidateActivation` → simulator | **Gap** | Not invoked |
| Checkpoint `ValidateCompression` → simulator | **Gap** | Not invoked |
| Checkpoint `ValidateFeedback` → simulator | **Gap** | Not invoked |
| `InitialFacts` auto-seed on harness run | **Partial** | Seeder exists; CLI/harness path does not clearly call `SeedScenario` before turns (integration scenarios rely on engine LoadFacts from turns + optional factory) |
| Engine.Reset between RunAll scenarios | **Partial** | Not shown in Harness.RunAll |
| TestKernelFactory in CLI | **Unused** | CLI uses Cortex kernel |
| Real Compressor.Compress* in CompressTurn | **Partial** | Compressor constructed; compress path is metadata facts |
| JIT real compiler | **Synthetic** | Mock atoms in simulator |
| Parent package exports | **None** | Import subpackage |

## Fact-flow position

The harness **does not** run:

```
user_intent → next_action → VirtualStore → tools
```

It runs a **context subsystem loop**:

```
scripted Turn → CompressTurn → LoadFacts → (RetrieveContext) → metrics/checkpoints
```

Perception/articulation are only involved when:

- Cortex boot pulls full system  
- `--live` calls LLM for assistant JSON  

## Shard / policy involvement

None directly. Scenarios may *mention* shards or Mangle policy in narrative turns (`shard-collaboration`, `mangle-policy-debug`) but do not activate the shard manager.

## How to extend wiring safely

1. Add scenario builder + register in `GetScenario`, `AllScenarios`, and category helper.  
2. If integration-only, set `Mode: RealMode`, `Category: CategoryIntegration`.  
3. If new observability: FileLogger channel + CLI flag default + MANIFEST text.  
4. If new production SUT API: prefer extending `ContextEngine` or calling into `internal/context` from `RealIntegrationEngine` only.  
5. Update this doc’s gap table when closing a wire.

## Integration test surface (package)

Package tests do not boot Cortex. They exercise pure helpers, loggers, reporters, and some feedback/store interactions in `feedback_test.go`. End-to-end multi-turn validation is CLI-level.
