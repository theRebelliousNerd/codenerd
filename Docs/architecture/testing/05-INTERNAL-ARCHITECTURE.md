# testing — Internal Architecture

> Last verified: 2026-07-13

## Component map

```
                    ┌──────────────────┐
                    │  Harness         │
                    │  scenarios map   │
                    │  reporter        │
                    │  optional tracers│
                    └────────┬─────────┘
                             │ RunScenario / RunAll
                             ▼
                    ┌──────────────────┐
                    │ SessionSimulator │
                    │ metrics collector│
                    │ lastUser* state  │
                    │ allFacts (live)  │
                    └────────┬─────────┘
           executeTurn       │
     ┌───────────────────────┼───────────────────────┐
     ▼                       ▼                       ▼
 ContextEngine         Observability            Checkpoint
 CompressTurn          tracers / inspector      validateCheckpoint
 RetrieveContext       compression viz          RetrieveContext +
 LoadFacts (inside)    piggyback / feedback     fuzzy P/R
```

## Control flow — single scenario

```
NewHarness / NewHarnessWithObservability
  │
  ├─ AllScenarios() → map[ScenarioID]*Scenario
  ├─ default MockContextEngine if not real
  └─ SetContextEngine(real|mock)
        │
        ▼
RunScenario(ctx, id)
  │
  ├─ NewSessionSimulator(kernel, config)
  ├─ SetObservability(...)
  ├─ SetContextEngine(...)
  └─ simulator.RunScenario(scenario)
        │
        for each turn:
          executeTurn
            ├─ (live + assistant) → executeLiveLLMTurn
            ├─ CompressTurn / fallback mock facts
            ├─ accumulate allFacts
            ├─ compressionViz / jitTracer / activationTracer / promptInspector
            ├─ (assistant) piggybackTracer + feedbackTracer
            └─ if IsQuestionReferringBack → RetrieveContext + RecordRetrieval
          if checkpoint.AfterTurn == i:
            validateCheckpoint → CheckpointResult
        Finalize metrics
        meetsExpectations → pass/fail
  │
  └─ reporter.Report(result)
```

## ContextEngine state machine

```
                 New*(kernel[, deps])
                         │
                         ▼
                   ┌───────────┐
           Reset() │  Ready    │◄──────────────┐
                   └─────┬─────┘               │
                         │ CompressTurn        │ Reset()
                         ▼                     │
                   ┌───────────┐               │
                   │  Facts++  │── Retrieve ──►│ scores / budget select
                   └───────────┘               │
                         │                     │
                         └─────────────────────┘
```

| Method | Mock | Real |
|--------|------|------|
| `CompressTurn` | Metadata → facts; optional `LoadFacts` | Same fact creation; `MarkNewFacts`; LoadFacts |
| `RetrieveContext` | Keyword/recency/back-ref score; threshold 100; budget | Back-ref context; `ScoreFacts`; `SelectWithinBudget` |
| `GetActivationBreakdown` | nil | map from last score |
| `SetCampaign/Issue/BackReferenceContext` | no-op | delegates to ActivationEngine |
| `Reset` | clear slices/counters | clear + clear activation contexts |
| `GetMode` | MockMode | RealMode |

## Checkpoint validation pipeline

```
validateCheckpoint(cp)
  │
  ├─ engine.RetrieveContext(cp.Query, tokenBudget)
  ├─ extractFactID each fact → []string
  ├─ if empty → fallback MustRetrieve (lenient mock path)
  ├─ fuzzyMatchFacts(retrieved, MustRetrieve) → MissingRequired
  ├─ fuzzyMatchFacts(retrieved, ShouldAvoid) → UnwantedNoise
  ├─ Precision / Recall / F1
  ├─ compare MinRecall / MinPrecision
  │
  └─ NOT YET: ValidateActivation / ValidateCompression / ValidateFeedback
```

### Fuzzy ID matching

`extractFactID` normalizes e.g. `turn_error_message(0, …)` → `turn_0_error_message`.  
`fuzzyFactMatch` allows substring and semantic families (`error` ↔ `error_message`, etc.).  
This bridges scenario authoring shortcuts and engine ID formats.

## Metrics pipeline

```
RecordCompression(orig, comp, latency)  ─┐
RecordRetrieval(prec, recall, latency)  ─┼─► Finalize() → Metrics
RecordTokenBudgetViolation()            ─┤
RecordMemory(mb)                        ─┘
```

`meetsExpectations`:

- Enrichment (`expected.CompressionRatio < 1`): actual ≤ expected × 1.5  
- Compression (`expected ≥ 1`): actual ≥ expected × 0.9  
- Recall / precision floors  
- Token budget violations ≤ expected max  

## Observability pipeline

```
CLI NewFileLogger(baseDir, console?)
  ├─ prompts.log          ← PromptInspector
  ├─ jit-compilation.log  ← JITTracer
  ├─ spreading-activation.log ← ActivationTracer
  ├─ compression.log      ← CompressionVisualizer
  ├─ piggyback-protocol.log ← PiggybackTracer
  ├─ context-feedback.log ← FeedbackTracer
  └─ summary.log          ← Reporter
Close() → footers + MANIFEST.txt
```

Default CLI flags enable all tracers (`true` by default).

## Live LLM path

```
config.UseLiveLLM && turn.Speaker == assistant && lastUserMessage != ""
  → RealIntegrationEngine.GenerateAssistantResponse
       system prompt demands JSON surface + control_packet.context_feedback
  → parseLiveLLMResponse
  → piggyback / feedback tracers (when wired)
```

Without a real engine + LLM client, live mode errors.

## Scenario data model

```
Scenario
  ScenarioID, Name, Description
  Turns[]
  Checkpoints[]
  ExpectedMetrics
  Mode, Category
  InitialFacts[]   // Mangle-like strings

Turn
  TurnID, Speaker, Message, Intent
  Metadata (files, symbols, errors, topics, back-ref flags)
  CampaignPhase, PhaseTransition

Checkpoint
  AfterTurn, Query
  MustRetrieve[], ShouldAvoid[]
  MinRecall, MinPrecision
  Description
  optional Validate* structs
```

## Fact predicates emitted by engines

| Predicate | Typical args |
|-----------|--------------|
| `conversation_turn` | turnID, speaker, message, intent |
| `turn_references_file` | turnID, path |
| `turn_references_symbol` | turnID, symbol |
| `turn_error_message` | turnID, message |
| `turn_topic` | turnID, topic |
| `turn_references_back` | turnID, referencedTurn |
| `turn_campaign_phase` | turnID, phase (real createFacts) |

Seeder predicates (integration setup): `current_campaign`, `campaign_phase`, `phase_objective`, `active_issue`, `issue_description`, `issue_mentioned_file`, `issue_error_type`, `symbol_graph`, `dependency_link`, `project_pattern`, `file_topology`.

## Concurrency model

- `MetricsCollector` uses `sync.Mutex`.  
- `RealIntegrationEngine` uses `sync.Mutex` around fact lists and scoring.  
- Tracers/reporters are single-writer from the sequential turn loop — not designed for concurrent `RunScenario` on one instance.  
- CLI runs one scenario at a time (or sequential `RunAll`).

## Package boundary

| Layer | Package |
|-------|---------|
| Anchor docs | `codenerd/internal/testing` |
| Implementation | `codenerd/internal/testing/context_harness` |
| Operator | `main` in `cmd/nerd` |
| SUT | `internal/context`, `internal/core`, … |
