# Progress and verification: shards

## 2026-07-13 Superstar reconciliation

Rebuilt the corpus against 18 production and 25 test files under
`internal/shards`. Traced registration through Cortex/chat/campaign/init boot,
profile and Mangle activation, exact action dispatch, VirtualStore execution,
specialist consultation, background observation, JIT prompt selection, and
teardown.

This corpus writer made no product edits. The owning implementation lanes fixed
all four verified defects during reconciliation; the packets are retained as
resolved incident evidence:

- `artifact:.corpus-build/findings/shards-predicate-manifest-drift.md`
- `artifact:.corpus-build/findings/shards-consultation-error-loss.md`
- `artifact:.corpus-build/findings/shards-observer-restart.md`
- `artifact:.corpus-build/findings/shards-router-unmapped-replay.md`

Seven ledger-approved redirect stubs were removed only after their SHA-256
prefixes matched `928929ce513e`, `0ad7b9dea738`, `880fbb3f95bf`,
`48d9908fb0af`, `64ab69488dc9`, `e4581de923c8`, and `083cb50d70fc`, and no exact
inbound link to a shards-relative legacy path remained. Unique responsibilities
were already represented in the canonical destinations.

## Verification receipts

| Gate | Result | Evidence |
|---|---|---|
| focused package suite | **PASS** | independent run: root 1.860s, system 12.458s; strict receipt: root 1.087s, system 10.586s |
| race suite | **PASS** | `go test -race -count=1 -timeout=240s ./internal/shards/...`: root 6.925s, system 59.572s |
| exact Cortex ownership | **PASS** | `TestDefaultCortexPermissionEnvelopeRoutesToPolicyShard`, focused internal/system run 7.503s |
| strict structural validation | **PASS** | 21 Markdown files, 4 cards, 0 legacy docs, 0 broken links, 0 unresolved source refs, all README sections |
| strict configured verification | **PASS** | focused package suite, 15.523s validator duration |
| semantic audit | **PASS WITH RESIDUALS** | four defects closed with focused regressions; JIT consistency, full factory/profile parity, boot readiness, drop/cache/result telemetry remain PARTIAL |

Reviewed HEAD: `cfc537e96495e1fbccd7efff8bb8e4001c93ca9c`.
Latest completed strict-validator dirty-tree receipt:
`f149ddb4c8cd76629b05509a1423bb6e472ebf37bcb730b71b7748d054c56251`.
The receipt precedes this transcription line; subsequent concurrent workspace
edits require a fresh semantic review, not a date-only refresh.

## Signed 42-point score

| Dimension | Score | Corpus evidence |
|---|---:|---|
| Human orientation | 3 | README gives the user problem and one read-file journey |
| North-star alignment | 3 | creative roles are separated from Mangle/VirtualStore authority |
| Evidence integrity | 3 | claim labels, exact symbols/tests, dirty-tree and residual findings |
| Architecture clarity | 3 | owner table, action sequence, registry/override topology, rejected directions |
| Data and logic contract | 3 | arities, producers/consumers, exact authorization envelope |
| Lifecycle completeness | 3 | boot, queue, spawn, events, cancellation, restarted generations, teardown, readiness residual |
| Deterministic safety | 3 | exact permission/default deny, envelope co-location, negative controls |
| JIT and agent behavior | 3 | atom families, budgets/fallback modes, inline residuals |
| Ecosystem wiring | 3 | Cortex/chat/campaign/init/session consumers, canonical ownership, compatibility paths |
| Operations | 2 | rich signals and recovery order; boot generation/drop telemetry absent |
| Verification | 3 | package + race receipts and risk-selected missing falsifiers |
| Uplift quality | 3 | safe registry/terminal repair through bounded activation generations |
| Navigation/governance | 3 | exact seven-section README, sole card authority, clear reading routes |
| Consistency | 3 | action ID, facts, statuses, defects, and ownership agree across corpus |
| **Total** | **41/42** | Passing threshold 34; every mandatory dimension is at least 2 |

Signed by: `shards_corpus_writer` on 2026-07-13. This score evaluates the corpus,
not product completeness.
