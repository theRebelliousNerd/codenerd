# testing — Alignment / Vision Review

> Last verified: 2026-07-13  
> Package: `internal/testing` (+ `context_harness`)  
> Scoring: 0–5 per dimension (5 = fully realizes north star in this package’s domain)

## Summary

| Dimension | Score | One-line evidence |
|-----------|------:|-------------------|
| Inversion of control (logic executive) | **4** | Checkpoints validate fact retrieval from kernel-backed stores; engines load `core.Fact`s |
| Creative center preserved | **3** | Optional live LLM; default path is synthetic turns (correct for CI) |
| Constitutional safety adjacency | **4** | No tool execution / no `permitted` bypass — harness is observation + retrieval only |
| JIT-first LLM surface | **2** | JITTracer + PromptInspector exist; simulator mostly emits **mock** atom snapshots |
| Long-horizon context | **5** | Core mission: 40–100 turn scenarios with recall checkpoints |
| Deterministic CI path | **4** | MockContextEngine + unit tests; real mode needs Cortex/API |
| Wiring honesty | **3** | CLI wired; several flags/types partially unused (category filter, advanced checkpoint validators) |
| Observability / glass box | **5** | FileLogger + six tracer channels + console/JSON reporter |
| Isolation / hygiene | **2** | TestKernelFactory / FactSeeder.Clear incomplete; CLI shares booted Cortex kernel |
| Documentation / corpus | **5** | This rebuild |

**Overall alignment: ~3.7 / 5** — a real, production-adjacent stress harness with clear dual-mode design, strong observability, and known integration partials.

---

## Dimension detail

### 1. Inversion of control — score 4

**Claim:** Facts and structured IDs, not free-form logs, are the validation substrate.

**Evidence:**

- `MockContextEngine.CompressTurn` / `RealIntegrationEngine.createFactsFromTurn` emit predicates: `conversation_turn`, `turn_references_file`, `turn_error_message`, `turn_topic`, `turn_references_back`, …
- Checkpoints declare `MustRetrieve` / `ShouldAvoid` fact ID strings; `validateCheckpoint` scores precision/recall against them (`simulator.go`).
- Integration scenarios seed Mangle-shaped strings via `InitialFacts` + `parseMangleFact`.

**Gap:** Real mode still builds facts from turn metadata rather than driving the full perception → compressor LLM pipeline for every turn (compressor is constructed but not the primary compress path in `CompressTurn`).

### 2. Creative center preserved — score 3

**Claim:** The LLM is not forced to be the test oracle for CI.

**Evidence:**

- Default `--mode=mock` is keyword/scoring simulation.
- `--live` + real engine calls `GenerateAssistantResponse` for piggyback-style JSON with `context_feedback`.

**Gap:** Mock path generates synthetic piggyback / JIT / prompt data; it documents *shape* more than production truth.

### 3. Constitutional safety adjacency — score 4

**Claim:** Harness must not open a backdoor around policy.

**Evidence:**

- No VirtualStore action routing, no tool calls from scenarios.
- CLI boots Cortex via `coresys.GetOrBootCortex` only to obtain kernel / store / LLM client.
- Fact loading uses `kernel.LoadFacts` — setup, not agent-initiated writes through policy gates.

**Gap:** Not a safety *test* harness for `permitted(...)`; adjacent, not covering that surface.

### 4. JIT-first LLM surface — score 2

**Evidence:** `JITTracer`, `PromptInspector`, mock atom generation in `generateMockAtoms`.

**Gap:** Snapshots are largely simulated (`TotalAtomsAvailable: 150`, fixed latencies). No wire into `internal/prompt` compiler for real selection in the default path.

### 5. Long-horizon context — score 5

**Evidence:** Scenarios up to ~100 turns (`refactoring-campaign`), multi-phase campaigns, back-reference questions, integration scenarios for phase transition / token budget / dependency spreading / feedback learning.

This is the package’s reason to exist.

### 6. Deterministic CI path — score 4

**Evidence:** `go test ./internal/testing/...` covers metrics, reporter, tracers, seeder, fuzzy matching helpers. Mock engine is pure Go scoring.

**Gap:** End-to-end scenario pass/fail is CLI-driven and boots Cortex even for mock mode (heavy for CI). Category flag exists but is not applied in `runTestContext`.

### 7. Wiring honesty — score 3

**Wired:**

- `cmd/nerd/cmd_test_context.go` → `context_harness` only consumer of the package from `cmd/`.
- `NewHarnessWithObservability` + engine injection.

**Partial / dormant:**

- `testContextCategory` flag declared, never filters scenarios.
- `Checkpoint.ValidateActivation` / `ValidateCompression` / `ValidateFeedback` types exist; `validateCheckpoint` does not enforce them.
- `GetScenario` map omits `context-feedback-learning` while `IntegrationScenarios()` includes it.
- Parent `internal/testing` is an empty anchor (`doc.go`, `harness_subsystem.go`).

### 8. Observability — score 5

**Evidence:** `FileLogger` multi-writers to `prompts.log`, `jit-compilation.log`, `spreading-activation.log`, `compression.log`, `piggyback-protocol.log`, `context-feedback.log`, `summary.log` + `MANIFEST.txt`. Console/JSON `Reporter`.

### 9. Isolation — score 2

**Evidence of intent:** `TestKernelFactory.CreateIsolatedKernel`, `FactSeeder.Clear`, engine `Reset()`.

**Reality:** CLI uses shared Cortex kernel; `Clear()` is a no-op; `randomID()` returns 0; minimal schema load is stubbed (`_ = schemaFile`).

### 10. Documentation — score 5

Package-local `context_harness/README.md` is rich; this architecture corpus rebuild closes the Docs/architecture gap.

---

## Alignment with fact-flow

```
Production:  user → perception → user_intent → kernel → next_action → VirtualStore → articulation
Harness:     scenario turns → ContextEngine.CompressTurn → LoadFacts →
             (optional RetrieveContext) → checkpoint precision/recall → Reporter
```

The harness **replays context encoding + retrieval**, not the full OODA executive loop. That is intentional and aligned: it measures the memory substrate the executive depends on.
