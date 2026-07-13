# testing — Gap Analysis

> Last verified: 2026-07-13  
> Against vision in `01-VISION.md` and package-local README claims

## Legend

| Status | Meaning |
|--------|---------|
| **Done** | Implemented and exercised |
| **Partial** | Code exists; incomplete wiring or fidelity |
| **Missing** | Vision / docs claim without implementation |
| **Non-gap** | Intentionally out of scope |

---

## Spec vs reality matrix

| Capability | Status | Evidence | Priority |
|------------|--------|----------|----------|
| Dual-mode ContextEngine | **Done** | `MockContextEngine`, `RealIntegrationEngine`, interface in `engine_interface.go` | — |
| Scenario registry (mock + integration) | **Done** | `scenarios.go`, `scenarios_integration.go` | — |
| Turn simulation loop | **Done** | `SessionSimulator.RunScenario` | — |
| Checkpoint precision/recall | **Done** | `validateCheckpoint` + fuzzy match | — |
| Metrics + reporter | **Done** | `metrics.go`, `reporter.go` | — |
| CLI `nerd test-context` | **Done** | `cmd/nerd/cmd_test_context.go` | — |
| File-based glass-box logs | **Done** | `FileLogger` + tracers | — |
| Live LLM piggyback feedback | **Partial** | `GenerateAssistantResponse` + `executeLiveLLMTurn`; requires real mode + API | P2 |
| Real ActivationEngine scoring | **Done** (retrieve) | `RealIntegrationEngine.RetrieveContext` → `ScoreFacts` | — |
| Real Compressor / LLM summary compression | **Partial** | Compressor field set; `CompressTurn` uses metadata facts + token estimate | P1 |
| `ValidateActivation` at checkpoints | **Partial** | Type + method on validation struct; **not called** by simulator | P1 |
| `ValidateCompression` at checkpoints | **Partial** | Type only; not enforced | P1 |
| `ValidateFeedback` at checkpoints | **Partial** | Type only; not enforced | P1 |
| Category filter CLI | **Partial** | Flag + `ScenariosByCategory`; CLI never filters | P2 |
| `GetScenario("context-feedback-learning")` | **Missing** | In `IntegrationScenarios`, absent from `GetScenario` map | P2 |
| Isolated per-scenario kernel | **Partial** | Factory exists; CLI uses shared Cortex kernel | P1 |
| `FactSeeder.Clear` | **Partial** | No-op stub | P3 |
| Minimal schema load in factory | **Partial** | Schema names listed, not loaded | P3 |
| Adversarial scenarios | **Missing** | README “future” | P3 |
| Replay from `.nerd/logs` | **Missing** | README “future” | P3 |
| Real JIT compiler traces | **Partial** | Tracer + mock atoms only | P2 |
| CI job running harness | **Missing** as package-owned | README suggests YAML; not verified in-repo here | P2 |
| Full OODA / tool execution tests | **Non-gap** | Session/campaign domain | — |
| Constitutional `permitted` tests | **Non-gap** | Core policy domain | — |
| Production import of harness | **Non-gap** | Must stay test-side | — |

---

## Priority narratives

### P1 — Enforce advanced checkpoint validators

Scenarios already attach `ValidateActivation`, `ValidateCompression`, and `ValidateFeedback` on integration checkpoints. The simulator only scores `MustRetrieve` / `ShouldAvoid`. That means integration scenarios can “pass” while campaign boosts, budget compression triggers, and feedback learning never run their assertions.

**Fix shape:** extend `validateCheckpoint` to call existing validators when fields are non-nil; fail `CheckpointResult` with component-specific reasons.

### P1 — Real compression fidelity

`NewRealIntegrationEngine` builds `internalcontext.NewCompressor(...)` but `CompressTurn` does not call compressor LLM paths; it mirrors mock-style fact extraction. Real mode still improves **retrieval** fidelity; compression metrics remain enrichment estimates (~20 tokens/fact).

### P1 — Scenario isolation

`RunAll` reuses one Cortex kernel + one engine without guaranteed `Reset` between scenarios (engine has `Reset()`, harness does not always call it between runs). Cross-scenario fact contamination is a real risk.

### P2 — CLI category filter + GetScenario completeness

Operators following Long help (`--category=integration`) get no filtering. Feedback-learning scenario is invisible to name lookup via `GetScenario`.

### P2 — Mock JIT/prompt honesty

Tracer logs look production-like but are simulator-generated. Document as **synthetic** unless wired to `internal/prompt`.

### P3 — Factory / seeder polish

`randomID`, schema loading, `Clear()` — quality-of-life for package-level tests, not CLI path.

---

## Non-gaps (do not “fix”)

1. **Empty parent package** — intentional anchor; imports target `context_harness`.  
2. **No Mangle files in package** — harness asserts facts into kernel; rules live in core defaults.  
3. **Not a general eval platform** — scoped to context memory quality.  
4. **Enrichment ratios &lt; 1.0** — by design for short simulated messages; do not “fix” by demanding 5:1 without changing scenario text length.

---

## Documentation drift

| Claim | Reality |
|-------|---------|
| Package README architecture tree lists `integration.go` | File is `scenarios_integration.go` |
| README sample metrics show 5.43x compression | Code expects enrichment ratios &lt; 1.0 for mock scenarios |
| README lists 4 mock scenarios | Code has **8** mock scenarios |
| README lists 6 integration scenarios | Code has **7** (includes feedback learning) |
| CLI Long lists 6 integration scenarios | Omits `context-feedback-learning` |

Architecture corpus should track **code**, not the older README sample report.
