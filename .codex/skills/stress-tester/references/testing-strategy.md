# Stress testing strategy

## Pressure dimensions

Change one dimension per experiment so a failure has an attributable cause:

| Dimension | Examples | Primary oracle |
|---|---|---|
| Repetition | `--repeat 10` | stable exit/result across iterations |
| Concurrency | `go test -race` | no race report, deadlock, or lifecycle leak |
| Input shape | malformed Mangle, empty/huge inputs | explicit rejection without panic |
| Scale | package/file/fact count | bounded latency and memory trend |
| Duration | soak window | no monotonic resource growth |
| Composition | perception → kernel → action → articulation | end-to-end invariant and artifacts |
| Recovery | cancellation, panic restart, partial artifact | consistent state after interruption |

## Oracles

Prefer deterministic oracles in this order:

1. Go assertions and race-detector output.
2. Mangle policy derivations and explicit default-deny results.
3. Persisted campaign result/triage schemas.
4. Structured log events within the test window.
5. Human inspection of bounded artifacts.

An LLM-generated narrative is supporting evidence, never the sole pass oracle.

## Stopping rules

Stop immediately on data-corruption risk, uncontrolled mutation, repeated panic, disk exhaustion, runaway process creation, or evidence that the run escaped its named workspace. Stop a bounded profile on its first failed command unless investigation specifically requires downstream evidence.

Every registered profile also has a whole-run ceiling. Repetition consumes that shared budget; `--repeat 100` cannot multiply a two-hour profile into an unbounded campaign. Lower the ceiling for a constrained investigation instead of raising it ad hoc.

## Root-cause loop

1. Reproduce with the same receipt.
2. Reduce package, test, input, repetition, and concurrency independently.
3. Find the earliest violated invariant in the fact/action lifecycle.
4. Add a deterministic regression at the owning layer.
5. Repair the owner, not the most visible downstream symptom.
6. Rerun the minimized case, the owning profile, then the broader baseline.
