# verification — Observability

> Last verified: **2026-07-13**

## Logging

Package uses `codenerd/internal/logging`:

| Call site | Category helper | When |
|-----------|-----------------|------|
| Marshal quality violations fail | `SystemShardsWarn` | `storeVerification` |
| Marshal evidence fail | `SystemShardsWarn` | same |
| Marshal corrective fail | `SystemShardsWarn` | same |
| `StoreVerification` error | `StoreError` | after LocalStore returns err |

There is **no** info/debug log for:

- attempt number start/end  
- chosen shard after selection  
- violations detected  
- corrective type applied  
- soft verification skip  

Operators mostly infer state from:

1. User-visible escalation / “Verification: ✅ Passed” formatting  
2. SQLite `task_verifications` rows  
3. Downstream store debug logs inside `LocalStore.StoreVerification`  

### Store-side logs (sibling)

`internal/store/local_verification.go`:

- `StoreDebug` on store + retrieve  
- `CategoryStore` Error on SQL failure  
- Timers via `logging.StartTimer(..., "StoreVerification")` etc.

## Metrics

No Prometheus/OTel counters in this package. No attempt latency histograms at package boundary.

Useful metrics if added later:

- `verification_attempts_total{outcome=success|fail|escalate|spawn_error}`  
- `verification_violations_total{type=...}`  
- `verification_retry_shard_switch_total`  
- `verification_llm_judge_errors_total`  
- `verification_soft_skip_total`  

## Persistence as observability

Table `task_verifications` columns (from store DDL / API):

| Column | Observability use |
|--------|-------------------|
| session_id, turn_number | Join to chat turn |
| task | What was attempted |
| shard_type | Which shard/intent |
| attempt_number | Retry index |
| success | Outcome |
| confidence | Judge confidence |
| reason | Narrative |
| quality_violations | JSON list |
| corrective_action | JSON object |
| evidence | JSON list |
| result_hash | Task content hash (naming caveat) |
| created_at | Ordering |

APIs:

- `GetVerificationHistory(sessionID, limit)`  
- `GetQualityViolationStats()`  

These are **forensic** surfaces; not yet wired to TUI glass box.

## Glass box / transparency

Verified mutation path in `process.go` does **not** emit the intermediate “Spawning:” glass-box events that the non-verify branch emits. After completion, user still sees formatted assistant surface with verification confidence.

Gap: attempt-level glass-box would help long retries feel less “stuck.”

## Debug hooks

| Hook | How |
|------|-----|
| Force nil verifier | Boot without wiring (not normal) → no loop |
| Force soft skip | nil LLM client on verifier |
| Inspect DB | Query `.nerd` local store `task_verifications` |
| Unit-level | Call unexported helpers only from package tests |

## User-visible signals

| Signal | Source |
|--------|--------|
| `**Verification**: ✅ Passed (confidence: N%)` | `formatVerifiedResponse` |
| `## ⚠️ Task Escalation Required` | `formatVerificationEscalation` |
| Synthesized reason when empty | helpers.go Gemini quirk handling |

## Privacy / retention

Task text and evidence may contain proprietary code. Persistence is local SQLite under workspace `.nerd` (via LocalStore path chosen at boot). Treat verification history with same sensitivity as reasoning traces.
