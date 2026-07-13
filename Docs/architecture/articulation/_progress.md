# articulation corpus rebuild — progress

| Date | Action |
|------|--------|
| **2026-07-13 (Superstar uplift)** | Reconciled against commit `c8f21b46` and dirty-tree fingerprint `94e0bc360ed90b56f52328c1e4fb14827978ac27f7f3aeab1fd36cf5dbabb3b5`. Added the seven-section human entry, all nine applicability lanes, six acceptance/rollback feature cards, and a signed score. Removed seven ledger-approved redirects after exact SHA-256 and zero-inbound verification. Fixed and race-tested shared processor statistics and UTF-8-safe caps. |
| **2026-07-13** | Full architecture corpus rebuilt under `Docs/architecture/articulation/` against `internal/articulation/`. Research: all 8 non-test Go sources + package README; reverse importers (`cmd/nerd/chat`, `internal/session`, `internal/system/factory.go`, `internal/shards`, `internal/perception`). Flagship `IMPLEMENTED_SPEC.md` densified to CLI depth. Document set aligns with the rebuild naming contract (01-VISION, 05-INTERNAL-ARCHITECTURE, 06-PUBLIC-API, 08-WIRING, etc.). No Go/Mangle/test code modified. |

## Deliverables checklist

- [x] README.md  
- [x] IMPLEMENTED_SPEC.md  
- [x] 00-ALIGNMENT-VISION-REVIEW.md  
- [x] 01-VISION.md  
- [x] 02-CURRENT-STATE.md  
- [x] 03-GAP-ANALYSIS.md  
- [x] 04-ARCHITECTURAL-PRINCIPLES.md  
- [x] 05-INTERNAL-ARCHITECTURE.md  
- [x] 06-PUBLIC-API-AND-TYPES.md  
- [x] 07-DEPENDENCY-MAP.md  
- [x] 08-WIRING-AND-INTEGRATION.md  
- [x] 09-SAFETY-AND-INVARIANTS.md  
- [x] 10-TESTING-ALIGNMENT.md  
- [x] 11-OBSERVABILITY.md  
- [x] 12-FAILURE-MODES.md  
- [x] TODO.md  
- [x] OPEN-QUESTIONS.md  
- [x] _progress.md  

## Source snapshot at rebuild

| Metric | Value |
|--------|------:|
| Non-test `.go` | 8 |
| Test `.go` | 7 |
| `.mg` | 0 |
| Largest sources | `prompt_assembler.go` (~1165), `emitter.go` (~1103) |

## Signed Superstar score

**40/42 — PASS.** Human orientation 3; north-star alignment 3; evidence integrity
3; architecture clarity 3; data/logic contract 3; lifecycle completeness 3;
deterministic safety 3; JIT/agents 3; ecosystem wiring 3; operations 3;
verification 3; uplift quality 3; navigation/governance 2; consistency 2.

The deductions preserve honest residuals: protocol knowledge is duplicated across
types/schema/prompts/consumers, and cross-entrypoint application parity is not yet
proven. The nine-lane README table, Implemented Spec state machine, feature-card
negative tests, and focused race receipt are the signed evidence.
