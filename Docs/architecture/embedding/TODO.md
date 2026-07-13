# TODO — embedding

> Last verified: 2026-07-13  
> Prioritized backlog. No time estimates. Order = dependency preference.

## P0 — Correctness

1. **Dimension contract**  
   - Probe first successful embed length or add config `dimensions`.  
   - Align `Dimensions()`, GenAI `OutputDimensionality`, and store schema validation.  
   - Coordinate with `internal/store` (do not change only one side).

2. **Provider/model change detection**  
   - Surface explicit “reembed required” when config engine identity ≠ stored vector metadata.  
   - Stats command already shows engine/dims; strengthen messaging on mismatch.

## P1 — Operational consistency

3. **Unify boot health policy**  
   - Decide: factory-strict vs chat-lenient.  
   - Document the choice in system + this corpus; implement one policy.

4. **Productize `EmbedBatchJob`**  
   - Poll helper, result extraction, timeout, cancel.  
   - Wire optional path in corpus_builder / large reembed.

5. **Ollama batch throughput** (only if profiling shows need)  
   - Bounded worker pool for EmbedBatch.  
   - Preserve order and ctx cancel; keep default conservative.

## P2 — Quality / polish

6. **GenAI HealthChecker**  
   - Cheap preflight so factory can treat GenAI like Ollama.

7. **SIMD default evaluation**  
   - Measure CosineSimilarity on large brute-force paths; consider default tags for release builds if stable.

8. **Optional embed cache**  
   - Hash(model, taskType, text) → vector with LRU; careful with memory.

9. **Metrics**  
   - Latency/error counters under future telemetry conventions.

10. **Live integration target**  
    - Optional test harness against local Ollama; never required for `go test ./...` green.

## Docs / hygiene

11. Remove or redirect obsolete thin stub filenames if still present (`01-DOMAIN-MODEL.md`, `02-CURRENT-STATE-EMBEDDING.md`, etc.) once consumers only link the rebuild set.  
12. Keep IMPLEMENTED_SPEC line counts honest after large source edits.

## Explicit non-TODO

- Do not add Mangle rules inside this package.  
- Do not merge chat LLM client into embedding.  
- Do not delete optional interfaces without multi-package migration.
