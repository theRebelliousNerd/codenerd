# testing — Implemented Spec (Deep-Dive)

> Last verified against codebase: 2026-07-13  
> Status: Living Reference Document  
> Language: Go  
> Module: `codenerd`  
> Primary sources: `internal/testing/`, `internal/testing/context_harness/`  
> Operator entry: `cmd/nerd/cmd_test_context.go` (`nerd test-context`)  
> Scale: **21** non-test Go files in `context_harness` + **8** test files + **2** parent-package anchors; **0** local `.mg`

---

## 1. Overview

The **Context Test Harness** is codeNERD’s specialized long-horizon memory stress system. It does not replace `go test` for ordinary packages. It answers a narrower, harder question:

> After dozens of coding-session turns, can the context substrate still retrieve the facts a competent agent needs?

It implements:

1. **Scripted multi-turn scenarios** (debug marathons, campaigns, TDD loops, SWE-bench-style issues, …).  
2. A **dual-mode context engine** seam (`ContextEngine`):  
   - **Mock** — fast fact enrichment + simplified activation (CI-friendly).  
   - **Real** — production `ActivationEngine` scoring with optional live LLM piggyback feedback.  
3. **Checkpoint validation** (precision / recall / F1 against must-retrieve / should-avoid fact IDs).  
4. **Glass-box observability** (seven log channels under `.nerd/context-tests/session-*`).  
5. **CLI orchestration** that boots Cortex, injects engines, and reports pass/fail.

### 1.1 Key characteristics

| Property | Value |
|----------|-------|
| Implementation package | `codenerd/internal/testing/context_harness` |
| Parent package | `codenerd/internal/testing` (doc anchor only) |
| Production importers | **None** (CLI only) |
| Default engine mode | `mock` |
| Default token budget | 8000 (`--token-budget`) |
| CLI timeout | 30 minutes |
| Default log dir | `.nerd/context-tests` |
| Scenario count | 8 mock + 7 integration (see registry notes) |
| Mangle ownership | None local; emits/loads `core.Fact`s |
| North-star role | Measures memory substrate; does not run OODA executive |

### 1.2 High-level control flow

```
nerd test-context [--scenario ID | --all] [--mode mock|real] [--live]
        │
        ▼
  FileLogger + tracers
        │
        ▼
  GetOrBootCortex → *core.RealKernel (+ LocalDB, LLMClient)
        │
        ▼
  ContextEngine (Mock | RealIntegration)
        │
        ▼
  Harness → SessionSimulator.RunScenario
        │
        ├─ per turn: CompressTurn → LoadFacts → (tracers)
        ├─ back-ref turns: RetrieveContext → retrieval metrics
        ├─ checkpoints: RetrieveContext → fuzzy P/R gates
        └─ Finalize metrics → meetsExpectations
        │
        ▼
  Reporter (console | json) → summary.log (+ stdout)
```

### 1.3 Relation to production fact-flow

```
Production OODA:
  user → perception → user_intent → kernel next_action
    → VirtualStore / shards / tools → articulation

Harness loop (intentionally narrower):
  scripted Turn → ContextEngine.CompressTurn → kernel.LoadFacts
    → ContextEngine.RetrieveContext → checkpoint metrics
```

The harness validates the **memory and retrieval half** of long sessions. It does not authorize tools, derive `permitted(...)`, or drive VirtualStore.

---

## 2. Implementation status

| Component | Status | Notes |
|-----------|--------|-------|
| Parent package anchor | **Implemented** | Comments only; no API |
| Types / scenario model | **Implemented** | Dual-mode, categories, advanced checkpoint types |
| Harness orchestrator | **Implemented** | Run one / all / list |
| SessionSimulator | **Implemented** | Turns, live branch, fuzzy checkpoints |
| MockContextEngine | **Implemented** | Thresholded scoring, back-ref boost |
| RealIntegrationEngine retrieve | **Implemented** | 9-component ScoreFacts path |
| RealIntegrationEngine compress | **Partial** | Metadata facts; Compressor constructed but underused |
| Live LLM assistant | **Implemented** | `GenerateAssistantResponse` + JSON parse |
| Scenario library (mock) | **Implemented** | 8 builders |
| Scenario library (integration) | **Implemented** | 7 builders |
| FactSeeder | **Implemented** | Clear is no-op |
| TestKernelFactory | **Partial** | Schema load stub; randomID=0 |
| FileLogger + 6 tracers + inspector | **Implemented** | CLI defaults on |
| Metrics + Reporter | **Implemented** | console + json |
| CLI command | **Implemented** | Full flag surface |
| `--category` filter | **Partial** | Flag only |
| Advanced checkpoint validators | **Partial** | Types exist; simulator does not enforce |
| GetScenario completeness | **Partial** | Missing `context-feedback-learning` map entry |
| Package unit tests | **Implemented** | 8 test files; no full E2E scenarios |
| CI-native zero-boot suite | **Partial** | unit tests OK; CLI always boots Cortex |

**Overall:** living, production-adjacent harness — **not** pre-implementation. Heuristic completeness **~75%** of the dual-mode vision (strong mock + real retrieve + observability; partial isolation, compress fidelity, advanced validators).

---

## 3. Source inventory

### 3.1 Tree

```
internal/testing/
  doc.go
  harness_subsystem.go
  context_harness/
    activation_tracer.go
    compression_viz.go
    engine_interface.go
    fact_seeder.go
    feedback_tracer.go
    file_logger.go
    harness.go
    inspector.go
    jit_tracer.go
    metrics.go
    mock_engine.go
    piggyback_tracer.go
    real_engine.go
    reporter.go
    scenarios.go
    scenarios_integration.go
    simulator.go
    test_kernel_factory.go
    types.go
    README.md
    *_test.go (8)
```

### 3.2 Role summary (non-test)

| File | Responsibility |
|------|----------------|
| `types.go` | Domain structs, modes, metrics semantics |
| `engine_interface.go` | Dual-mode contract + activation validation types |
| `harness.go` | Scenario registry map, run APIs, observability injection |
| `simulator.go` | Turn execution, checkpoints, live mode, fuzzy IDs |
| `mock_engine.go` | CI engine |
| `real_engine.go` | ActivationEngine + live LLM |
| `scenarios.go` | Mock scenarios + registry helpers |
| `scenarios_integration.go` | Integration scenarios |
| `metrics.go` | Thread-safe aggregation |
| `reporter.go` | Human + JSON reports |
| `fact_seeder.go` | Initial world facts for integration |
| `test_kernel_factory.go` | Isolated kernel attempts + parseMangleFact |
| `file_logger.go` | Session artifacts |
| `inspector.go` | Prompt/response glass box |
| `jit_tracer.go` | JIT compilation traces |
| `activation_tracer.go` | Spreading activation traces |
| `compression_viz.go` | Compression before/after |
| `piggyback_tracer.go` | Control packet traces |
| `feedback_tracer.go` | Context usefulness learning traces |

### 3.3 Hotspots

1. `simulator.go` — behavioral center of gravity  
2. `mock_engine.go` / `real_engine.go` — SUT adapters  
3. `scenarios*.go` — product of the harness (what we measure)  
4. `cmd/nerd/cmd_test_context.go` — only external wire  

---

## 4. Domain model deep dive

### 4.1 Scenario

A `Scenario` is a complete test case:

- **Identity:** `ScenarioID` (kebab-case, map key), `Name`, `Description`  
- **Script:** ordered `Turns`  
- **Oracles:** `Checkpoints`, `ExpectedMetrics`  
- **Engine requirements:** `Mode` (`mock`/`real`), `Category`  
- **World setup:** `InitialFacts` (Mangle-like strings)

### 4.2 Turn

Each turn models one message in a coding session:

- Speaker `user` | `assistant`  
- Natural language `Message` + coarse `Intent`  
- `TurnMetadata`: files, symbols, errors, topics, back-reference flags  
- Campaign: `CampaignPhase`, `PhaseTransition`  

Back-reference questions set `IsQuestionReferringBack` and `ReferencesBackToTurn` to trigger retrieval metrics during simulation.

### 4.3 Checkpoint

After a turn index, the harness runs a retrieval query and scores:

| Field | Role |
|-------|------|
| `MustRetrieve` | Required fact IDs (high signal) |
| `ShouldAvoid` | Noise IDs |
| `MinRecall` / `MinPrecision` | Hard floors |
| `ValidateActivation` | **Declared; not enforced in simulator** |
| `ValidateCompression` | **Declared; not enforced** |
| `ValidateFeedback` | **Declared; not enforced** |

### 4.4 Metrics semantics (critical)

From `types.go` comments and collector math:

```
CompressionRatio = totalOriginalTokens / totalCompressedTokens
```

| Ratio | Interpretation |
|------:|----------------|
| &lt; 1.0 | Semantic **enrichment** (short text → structured facts) |
| &gt; 1.0 | True **compression** |

Mock scenarios commonly expect ~0.3–0.5 enrichment. Older README samples showing 5× compression are **stale** relative to code.

`meetsExpectations` allows:

- Enrichment: actual ≤ expected × 1.5  
- Compression: actual ≥ expected × 0.9  
- Recall/precision ≥ expected  
- Token violations ≤ expected max  

---

## 5. Dual-mode engines

### 5.1 Interface

`ContextEngine` (`engine_interface.go`) is the single seam:

- Compress turn → facts + compressed token estimate  
- Retrieve within budget  
- Optional multi-context setters (campaign / issue / back-reference)  
- Activation breakdown inspection  
- Reset / mode  

### 5.2 MockContextEngine

**Purpose:** deterministic CI path without depending on production activation weights.

**CompressTurn:**

- Estimates original tokens as `len(message)/4`  
- Emits structured facts from metadata  
- Optionally `kernel.LoadFacts`  
- Compressed tokens ≈ `len(facts)*20`  

**RetrieveContext:**

- Base 50 + recency up to 40 + back-ref +50 + keyword up to 30 + predicate priorities  
- Filter score ≥ **100**  
- Pack at ~20 tokens/fact until budget  

**Breakdown:** always nil (validation skips).

### 5.3 RealIntegrationEngine

**Purpose:** exercise real `internal/context.ActivationEngine` selection.

**Construction:**

```go
NewRealIntegrationEngine(kernel, localStore, llmClient, config)
// builds ActivationEngine + Compressor
```

**CompressTurn (current fidelity):**

- Builds the same family of metadata facts as mock  
- `LoadFacts` + `activation.MarkNewFacts`  
- Does **not** call LLM compressor for summary compression  
- Token estimate still ~20/fact  

**RetrieveContext:**

- Builds `BackReferenceActivationContext` from `turn_references_back` and related metadata  
- Synthesizes a query `user_intent` fact  
- `ScoreFacts` → store 9-component `ActivationBreakdown`  
- `SelectWithinBudget`  

**Live:**

- `GenerateAssistantResponse` asks the LLM for JSON with surface response + `context_feedback`  
- Used when simulator `UseLiveLLM` and assistant turn  

---

## 6. Simulator deep dive

### 6.1 RunScenario

1. For each turn: `executeTurn`  
2. After matching turn index: `validateCheckpoint` for each matching checkpoint  
3. `metrics.Finalize`  
4. `meetsExpectations` against scenario expected metrics  
5. Aggregate pass/fail + failure reasons  

### 6.2 executeTurn phases

| Phase | Behavior |
|-------|----------|
| Live assistant | Optional short-circuit to LLM path |
| Compress | Engine or fallback mock facts |
| Accumulate | `allFacts` for live context |
| Compression viz | Always if tracer set |
| JIT trace | Mock atoms from intent/metadata |
| Activation trace | Snapshot of compressed/retrieved facts |
| Prompt inspect | Mock system prompt + atoms/facts |
| Piggyback | Assistant turns: synthetic or live control packet |
| Feedback | If piggyback feedback present |
| Retrieval metrics | Only if `IsQuestionReferringBack` |

### 6.3 validateCheckpoint

1. `RetrieveContext(query, budget)`  
2. Map facts → IDs via `extractFactID`  
3. Empty → **fallback to MustRetrieve** (lenient)  
4. Fuzzy match required / avoided  
5. Precision = matchedRequired / totalRetrieved  
6. Recall = matchedRequired / mustRetrieve count  
7. Fail if below Min*  

**Not done:** component activation floors, compression checkpoints, feedback sample floors.

### 6.4 Fuzzy identity

`extractFactID` normalizes `turn_error_message(0,…)` → `turn_0_error_message`.  
`fuzzyFactMatch` allows semantic families (`error` ↔ `error_message`, etc.).  
This is deliberate resilience for scenario authors; it can also hide ID drift bugs.

---

## 7. Scenario catalog

### 7.1 Mock scenarios (`CategoryMock` / default)

| ScenarioID | Intent of test | Approx scale |
|------------|----------------|--------------|
| `debugging-marathon` | Long retention of original error / failed fixes | 50 turns |
| `feature-implementation` | Multi-phase plan→implement→test paging | 75 turns |
| `refactoring-campaign` | Long stability + cross-file tracking | 100 turns |
| `research-and-build` | Cross-phase research retrieval | 80 turns |
| `tdd-loop` | Test-fix cycle recall | ~40 turns |
| `campaign-execution` | Multi-phase campaign narrative | 60 turns |
| `shard-collaboration` | Multi-shard style collaboration narrative | scenario-defined |
| `mangle-policy-debug` | Policy/debug narrative stress | scenario-defined |

### 7.2 Integration scenarios (`CategoryIntegration`, intended for `--mode=real`)

| ScenarioID | Intent |
|------------|--------|
| `campaign-phase-transition` | Phase reset / campaign boost / cross-phase recall |
| `swebench-issue-resolution` | Issue context + tiered file boosting |
| `token-budget-overflow` | Compression / budget pressure |
| `dependency-spreading` | Symbol graph depth decay |
| `verb-specific-boosting` | Intent verb boost behavior |
| `ephemeral-filtering` | Boot guard / ephemeral category filtering |
| `context-feedback-learning` | Usefulness learning over samples |

### 7.3 Registry APIs

| API | Behavior |
|-----|----------|
| `AllScenarios()` | Mock list + `IntegrationScenarios()` |
| `MockScenarios()` | 8 mock only |
| `IntegrationScenarios()` | 7 integration |
| `ScenariosByCategory` | Filter |
| `GetScenario` | Map lookup — **missing** `context-feedback-learning` entry |

`NewHarness` loads **AllScenarios** into its map by ScenarioID, so RunAll includes feedback-learning even when GetScenario does not.

---

## 8. Observability system

### 8.1 FileLogger

Creates timestamped session directory; opens seven logs; multi-writes with console; writes MANIFEST on close.

### 8.2 Tracers (optional, CLI default on)

| Component | Log |
|-----------|-----|
| PromptInspector | prompts.log |
| JITTracer | jit-compilation.log |
| ActivationTracer | spreading-activation.log |
| CompressionVisualizer | compression.log |
| PiggybackTracer | piggyback-protocol.log |
| FeedbackTracer | context-feedback.log |
| Reporter | summary.log |

### 8.3 Synthetic vs real signals

| Channel | Mock path | Real/live path |
|---------|-----------|----------------|
| Compression facts | Metadata enrichment | Same metadata path today |
| Activation scores | Heuristic / viz | Real ScoreFacts on retrieve |
| JIT atoms | Generated | Still generated unless wired |
| Piggyback | Synthetic feedback | Live JSON feedback with `--live` |

---

## 9. Seeding & isolation

### 9.1 FactSeeder

Helpers to seed campaign, issue, symbol graph, dependency links, project patterns, file topology. Used by tests and available for integration setup. `Clear()` is intentionally stubbed (“use fresh kernels”).

### 9.2 TestKernelFactory

- `CreateKernel` — `NewRealKernel` + parse InitialFacts  
- Minimal schema list **not loaded** (`_ = schemaFile`)  
- `CreateIsolatedKernel` — workspace path with `randomID()` always **0**  

CLI does not use the factory; it uses the booted Cortex kernel.

---

## 10. CLI integration map

| Concern | Implementation |
|---------|----------------|
| Command | `test-context` |
| Flags | scenario, all, format, max-turns, token-budget, paging, mode, category, live, verbose, six tracer flags, log-dir, console |
| Boot | `coresys.GetOrBootCortex` |
| Kernel | Assert `*core.RealKernel` |
| Engine select | real vs mock constructors |
| Observability | New*Tracer → FileLogger writers |
| Harness | `NewHarnessWithObservability` |
| Exit | Error if any scenario fails |

### 10.1 Known CLI partials

1. `--category` not applied  
2. Always boots Cortex even for pure mock  
3. Help text scenario lists lag code (4 mock / 6 integration in Long vs 8 / 7 in code)  

---

## 11. Integration map (packages)

```
cmd/nerd
  └─ context_harness
       ├─ internal/core          (kernel, Fact)
       ├─ internal/context       (ActivationEngine, Compressor, configs, activation contexts)
       ├─ internal/perception    (LLMClient)
       └─ internal/store         (LocalStore)
```

No reverse imports. No shard registration. No VirtualStore routes.

---

## 12. Public surface (operator + library)

**Operator:**

```text
nerd test-context
nerd test-context --scenario debugging-marathon
nerd test-context --all --format json
nerd test-context --scenario campaign-phase-transition --mode=real
nerd test-context --mode=real --live --scenario debugging-marathon
```

**Library (tests / future tooling):**

- `NewHarness`, `NewHarnessWithObservability`  
- `NewMockContextEngine`, `NewRealIntegrationEngine`  
- Scenario builders + registry  
- `NewFileLogger`, tracers, `NewReporter`  
- `NewFactSeeder`, `NewTestKernelFactory`  

See `06-PUBLIC-API-AND-TYPES.md` for exhaustive tables.

---

## 13. Concurrency & resource model

- Single-threaded turn loop per simulator  
- Mutexes in metrics + real engine  
- File handles held open for session lifetime  
- Cortex resources closed by CLI defer  
- No parallel scenario runner  

---

## 14. Safety summary

- No tool execution from scenarios  
- No policy bypass for agent actions  
- Logs may contain prompts and API-adjacent content — treat as sensitive  
- Live mode network dependency explicit  
- Mock mode avoids LLM calls (Cortex boot may still initialize broader system)

---

## 15. Gaps pointer

Authoritative gap matrix: [`03-GAP-ANALYSIS.md`](03-GAP-ANALYSIS.md).

Highest-impact incompletes:

1. Advanced checkpoint validators unenforced  
2. Real compression path underuses Compressor  
3. Scenario isolation / Reset between runs  
4. CLI category filter dead  
5. GetScenario map incomplete  
6. Synthetic JIT/prompt traces  

---

## 16. Verification commands

```powershell
go test ./internal/testing/...

$env:CGO_CFLAGS = "-IC:/CodeProjects/codeNERD/sqlite_headers"
go build -o nerd.exe ./cmd/nerd
.\nerd.exe test-context --scenario debugging-marathon --mode=mock
.\nerd.exe test-context --all --mode=mock --console=false
```

---

## 17. Design principles (binding)

See [`04-ARCHITECTURAL-PRINCIPLES.md`](04-ARCHITECTURAL-PRINCIPLES.md). Short form:

1. Dual-mode seam = `ContextEngine`  
2. Facts are ground truth  
3. Mock stays offline-fast  
4. Enrichment ratios are first-class  
5. File-first observability  
6. Never import harness from production runtime  
7. Isolation over ambient state  
8. Typed validators must mean something  
9. Keep registries synchronized  
10. Measure memory, not whole agent  

---

## 18. Failure modes pointer

See [`12-FAILURE-MODES.md`](12-FAILURE-MODES.md) for FM1–FM16 (unknown scenario, boot, soft-pass fallback, contamination, live parse, …).

---

## 19. Evolution notes (historical honesty)

- Package-local `context_harness/README.md` predates enrichment-ratio semantics and undercounts scenarios.  
- Parent `internal/testing` was left as an anchor for “subsystem” discovery without forcing import of heavy harness into unrelated tests.  
- Advanced checkpoint and feedback types show the intended integration depth; simulator enforcement is the next coherence step.  
- Scoring component counts in comments have drifted (7 vs 8 vs 9); `ActivationBreakdown` currently models **nine** components including feedback and back-reference.

---

## 20. Document index

| Doc | Role |
|-----|------|
| [README.md](README.md) | Entry + map |
| [00-ALIGNMENT-VISION-REVIEW.md](00-ALIGNMENT-VISION-REVIEW.md) | Scores |
| [01-VISION.md](01-VISION.md) | Target vision |
| [02-CURRENT-STATE.md](02-CURRENT-STATE.md) | Inventory |
| [03-GAP-ANALYSIS.md](03-GAP-ANALYSIS.md) | Gaps |
| [04-ARCHITECTURAL-PRINCIPLES.md](04-ARCHITECTURAL-PRINCIPLES.md) | Principles |
| [05-INTERNAL-ARCHITECTURE.md](05-INTERNAL-ARCHITECTURE.md) | Internals |
| [06-PUBLIC-API-AND-TYPES.md](06-PUBLIC-API-AND-TYPES.md) | API |
| [07-DEPENDENCY-MAP.md](07-DEPENDENCY-MAP.md) | Deps |
| [08-WIRING-AND-INTEGRATION.md](08-WIRING-AND-INTEGRATION.md) | Wiring |
| [09-SAFETY-AND-INVARIANTS.md](09-SAFETY-AND-INVARIANTS.md) | Safety |
| [10-TESTING-ALIGNMENT.md](10-TESTING-ALIGNMENT.md) | Self-tests |
| [11-OBSERVABILITY.md](11-OBSERVABILITY.md) | Logs/metrics |
| [12-FAILURE-MODES.md](12-FAILURE-MODES.md) | Failures |
| [TODO.md](TODO.md) | Backlog |
| [OPEN-QUESTIONS.md](OPEN-QUESTIONS.md) | Questions |
| [_progress.md](_progress.md) | Rebuild log |

---

## 21. Appendix — example checkpoint (code shape)

```go
Checkpoint{
    AfterTurn:    45,
    Query:        "What was the original error?",
    MustRetrieve: []string{"turn_0_error_message"},
    ShouldAvoid:  []string{"turn_30_unrelated"},
    MinRecall:    0.9,
    MinPrecision: 0.1,
    Description:  "Recall original error late in session",
    // Optional — types exist; simulator does not yet enforce:
    // ValidateActivation: &ActivationValidation{ MinRecencyBoost: 0, MinTotalScore: 100 },
}
```

## 22. Appendix — mock scoring sketch

```
score = 50
score += recency * 40          // turn / maxTurn
if turn in referencedTurns: score += 50
if keyword match: score += 30
score += predicate priority    // errors +25, topics +20, ...
keep if score >= 100
select until tokenBudget
```

## 23. Appendix — real retrieve sketch

```
collect turn_references_back → BackReferenceActivationContext
intentFact = user_intent("query","retrieve", query, "")
scored = activation.ScoreFacts(allFacts, intentFact)
store ActivationBreakdown per factID
return SelectWithinBudget(scored, tokenBudget)
```

## 24. Appendix — CLI engine selection

```go
if engineMode == RealMode {
  contextEngine = NewRealIntegrationEngine(realKernel, cortex.LocalDB, cortex.LLMClient, DefaultConfig())
} else {
  contextEngine = NewMockContextEngine(realKernel)
}
```

## 25. Closing

This package is the **regression spine for infinite context**. Treat scenario failures as product failures of the memory substrate until proven otherwise. Expand fidelity along the gap priorities before inventing new scenario categories.
