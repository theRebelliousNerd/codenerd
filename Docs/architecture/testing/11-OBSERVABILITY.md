# testing — Observability

> Last verified: 2026-07-13

## Design

The harness is a **glass-box** stress tool: when a checkpoint fails at turn 45, operators need more than “Recall 0.40 &lt; 0.90”. FileLogger + tracers produce a multi-file session that reconstructs what was compressed, activated, and (in live mode) what the model said about usefulness.

## Session layout

Default base: `.nerd/context-tests/` (flag `--log-dir`).

```
.nerd/context-tests/session-YYYYMMDD-HHMMSS/
  MANIFEST.txt
  summary.log
  prompts.log
  jit-compilation.log
  spreading-activation.log
  compression.log
  piggyback-protocol.log
  context-feedback.log
```

`--console=false` writes file-only; default multi-writes console + file.

## Channels

| Log file | Producer | Content |
|----------|----------|---------|
| `summary.log` | `Reporter` | Pass/fail, metrics, expected vs actual, checkpoints |
| `prompts.log` | `PromptInspector` | Token budget, selected atoms/facts, system/user text |
| `jit-compilation.log` | `JITTracer` | Atom selection, categories, budget split, latency |
| `spreading-activation.log` | `ActivationTracer` | Query, scores, selected facts, optional campaign/issue/session graphs |
| `compression.log` | `CompressionVisualizer` | Original text vs facts, ratio, metadata |
| `piggyback-protocol.log` | `PiggybackTracer` | Surface text, control packet (intent, mangle updates, context feedback) |
| `context-feedback.log` | `FeedbackTracer` | Helpful/noise predicates, learned scores, score impacts |
| `MANIFEST.txt` | `FileLogger.Close` | File guide + viewing order |

## CLI flags (defaults)

| Flag | Default | Effect |
|------|---------|--------|
| `--inspect-prompts` | true | PromptInspector |
| `--trace-jit` | true | JITTracer |
| `--trace-activation` | true | ActivationTracer |
| `--vis-compression` | true | CompressionVisualizer |
| `--trace-piggyback` | true | PiggybackTracer |
| `--trace-feedback` | true | FeedbackTracer |
| `--verbose` / `-v` | false | Deeper per-channel detail |
| `--format` | console | console \| json for reporter |
| `--console` | true | MultiWriter vs file-only |

## Metrics surface

### Collector fields → `Metrics`

| Metric | Meaning |
|--------|---------|
| `CompressionRatio` | original/compressed (enrichment if &lt; 1) |
| `AvgRetrievalPrec` | mean precision on back-ref retrievals |
| `AvgRetrievalRecall` | mean recall |
| `AvgF1Score` | harmonic mean of avgs |
| `TokenBudgetViolations` | count |
| `AvgCompressionLatency` | mean |
| `AvgRetrievalLatency` | mean |
| `PeakMemoryMB` | max recorded (only if `RecordMemory` called) |
| `QualityDegradation` | field exists; not heavily populated by current loop |

### Reporter formats

- **console**: human boxes with ✓/✗  
- **json**: full `TestResult` tree via `encoding/json`

## Honesty about mock traces

In the default simulator path (non-live):

- JIT atom lists are **generated** (`generateMockAtoms`) — not from `internal/prompt` compiler.  
- Prompt system text is labeled `simulator-mock`.  
- Piggyback control packets are **synthetic** from turn intent (`generateMockContextFeedback`).  
- Activation traces on non-retrieve turns often re-visualize compressed facts as “activated.”

Real mode improves **RetrieveContext** fidelity (true ActivationEngine scores). Live mode improves **assistant feedback** fidelity.

When debugging production regressions, prefer:

1. `--mode=real`  
2. Inspect `spreading-activation.log` + `summary.log`  
3. Use `--live` only when feedback learning is under test  

## Correlation tips

| Symptom | First logs |
|---------|------------|
| Checkpoint miss | summary → spreading-activation → compression (was fact ever created?) |
| Budget blowups | compression + prompts budget utilization |
| Wrong intent class | piggyback-protocol (live) |
| Learning not moving scores | context-feedback |
| “Silent” session | confirm FileLogger created; check permissions on log-dir |

## Relation to production observability

| Production | Harness analogue |
|------------|------------------|
| Glass box / transparency pages | FileLogger multi-file session |
| Prompt assembly debug | PromptInspector |
| Activation engine internals | ActivationTracer + real breakdown map |
| Campaign metrics | scenario expected metrics only |

Harness does not write to production telemetry backends; artifacts are local filesystem only.
