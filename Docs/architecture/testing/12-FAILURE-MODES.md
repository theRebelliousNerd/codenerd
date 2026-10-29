# testing — Failure Modes

> Last verified: 2026-07-13

## FM1 — Unknown scenario ID

**Symptom:** `unknown scenario: …` from `Harness.RunScenario`.  
**Cause:** Typo, using display Name instead of ScenarioID, or ID only in IntegrationScenarios but missing from harness map… (harness loads `AllScenarios`, so missing GetScenario alone is OK for RunAll; wrong ID still fails).  
**Mitigation:** `nerd test-context` with no flags lists IDs; keep registry complete.

## FM2 — Cortex boot failure

**Symptom:** `failed to boot cortex: …`  
**Cause:** Bad workspace, missing sqlite/CGO build, API/config issues, locked DB.  
**Mitigation:** Build with CGO flags from Agents.md; run from initialized workspace; check `GetOrBootCortex` errors.

## FM3 — Kernel type assertion failure

**Symptom:** `test-context: cortex.Kernel is %T, expected *core.RealKernel`  
**Cause:** Alternate kernel implementation injected into Cortex.  
**Mitigation:** Fail loud (current behavior); do not restore nil fallback.

## FM4 — Compression / LoadFacts errors

**Symptom:** `turn N failed: compression failed: …` or load facts errors.  
**Cause:** Kernel rejects facts; kernel nil; concurrent misuse.  
**Mitigation:** Align predicates with schemas; ensure RealKernel; isolate scenarios.

## FM5 — Retrieval errors in real mode

**Symptom:** turn fails with `retrieval failed`.  
**Cause:** Activation engine / context errors under RetrieveContext.  
**Mitigation:** Inspect activation log; check fact set non-empty; review back-ref assembly.

## FM6 — Checkpoint soft-pass via fallback

**Symptom:** Checkpoints pass but retrieval is empty in reality.  
**Cause:** `validateCheckpoint` sets `retrievedFactIDs = checkpoint.MustRetrieve` when engine returns nothing.  
**Impact:** False confidence in mock/broken engines.  
**Mitigation:** Treat empty retrieval as fail in real mode (recommended code change); today document as softness.

## FM7 — Metrics expectation miss

**Symptom:** Scenario FAILED with “Metrics did not meet expectations”.  
**Cause:** Enrichment ratio outside tolerance; recall/precision floors; token violations.  
**Mitigation:** Read EXPECTED vs ACTUAL section; note enrichment vs compression branch; adjust scenario thresholds only with intent.

## FM8 — Cross-scenario contamination

**Symptom:** Second scenario fails after first succeeds under `--all`.  
**Cause:** Shared kernel facts; engine not Reset.  
**Mitigation:** Call `contextEngine.Reset()` between scenarios; prefer isolated kernels.

## FM9 — Live LLM parse failure

**Symptom:** live turn errors; JSON parse failures.  
**Cause:** Model returns non-JSON or wrong shape; no LLM client.  
**Mitigation:** Strict system prompt already requests JSON; `extractJSON` helper; require `--mode=real --live` and valid key.

## FM10 — Disk / log-dir failures

**Symptom:** `failed to create file logger` / create log file errors.  
**Cause:** Path permissions, full disk, invalid baseDir.  
**Mitigation:** Writable `--log-dir`; monitor disk free (machine disk-guard).

## FM11 — CLI category filter noop

**Symptom:** `--category=integration` still lists/runs everything when combined poorly; flag ignored.  
**Cause:** `testContextCategory` never applied.  
**Mitigation:** Use explicit scenario IDs until wired; fix wiring (TODO).

## FM12 — GetScenario miss for feedback-learning

**Symptom:** Lookup helper returns nil for `context-feedback-learning` while AllScenarios includes it.  
**Cause:** Map incompleteness in `GetScenario`.  
**Mitigation:** Add map entry; prefer iterating AllScenarios as source of truth.

## FM13 — Mock scoring false negatives on back-ref

**Symptom:** “What was the original error?” checkpoints fail.  
**Cause:** Threshold/back-ref boost insufficient; keyword match fails on normalized topics.  
**Mitigation:** Ensure `turn_references_back` facts exist; check containsKeyword normalization; real mode for production scoring.

## FM14 — Real mode still “enrichment only”

**Symptom:** Expect true 50:1 compression; get ~0.4 ratios.  
**Cause:** CompressTurn does not drive LLM compressor summaries.  
**Mitigation:** Align expectations with types.go; implement real compress path before raising ratio floors.

## FM15 — Timeout

**Symptom:** 30m context cancel mid-suite.  
**Cause:** Many long scenarios + real/live LLM latency.  
**Mitigation:** Run single scenarios; raise timeout carefully; mock for CI.

## FM16 — Observability noise

**Symptom:** Console flooded; hard to see pass/fail.  
**Cause:** All tracers default true + console multi-write.  
**Mitigation:** `--console=false`, disable unused tracers, start with `summary.log`.

## Failure → mitigation quick table

| Mode | First action |
|------|----------------|
| Boot | Cortex/workspace/CGO |
| Unknown scenario | List IDs |
| Checkpoint | summary + activation + compression logs |
| Metrics | enrichment vs compression expectations |
| Live | API key + JSON response shape |
| Flaky --all | isolation / Reset |
