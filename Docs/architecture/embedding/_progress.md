# Progress — embedding architecture corpus

> Last reconciled: 2026-07-13
>
> Source commit: `0f07a2905219109d249cd9e244ae3d22225e329b`
>
> Verified dirty-tree fingerprint: `0ece353e571098ed81453b922f51bcb05e748c10b1a341adffc6c891356aa152`

## Current disposition

| Area | Result | Evidence |
|---|---|---|
| Canonical corpus | **VERIFIED CURRENT** | 18 Markdown files; exact README routes; strict validator |
| Legacy redirects | **VERIFIED CURRENT — removed** | seven ledger-approved pure redirects; zero inbound compatibility links; strict legacy count 0 |
| Provider response trust | **VERIFIED CURRENT — repaired** | finite/non-empty single vectors; exact-cardinality uniform batches; focused regressions |
| Ollama lifecycle | **VERIFIED CURRENT — repaired** | locked model state, cancelled-bootstrap retry, cancellable backoff; race suite |
| Optional SIMD | **VERIFIED CURRENT — buildable experimental lane** | `GOEXPERIMENT=simd` plus `-tags simd` package test |
| Vector-space identity | **PROPOSED UPLIFT** | `embedding-vector-space-identity-v1` in [TODO.md](TODO.md) |
| Unified semantic health | **PROPOSED UPLIFT** | `embedding-semantic-health-receipt-v1` in [TODO.md](TODO.md) |

## Product repair ledger

| Defect | Causal change | Regression |
|---|---|---|
| Empty/non-finite/partial provider output could cross a success boundary | shared single/batch response validators wired into Ollama and GenAI | `TestValidateEmbeddingVectorRejectsInvalidProviderOutput`; `TestValidateEmbeddingBatchResponseEnforcesCardinalityAndShape`; malformed Ollama response test |
| Mutable Ollama model identity was read outside its lifecycle mutex | locked accessors/snapshots and locked invalidation | `TestOllamaEngineModelAccessIsConcurrentSafe` under `-race` |
| Short bootstrap pull deadline could permanently consume the only pull attempt | re-arm only when the caller context caused pull failure | `TestEnsureModel_BootstrapDeadlineDoesNotPoisonLaterPull` |
| Retry sleeps ignored cancellation and embed timer did not close on failure | context-selecting retry timer; deferred operation timer stop | `TestWaitForRetryHonorsCancellation` |
| Optional SIMD used a stale public-lane API | current opaque loads/stores plus scalar horizontal reduction | shared package tests under experiment + tag |

## Verification receipts

| Gate | Result |
|---|---|
| Focused provider/lifecycle regression command | **PASS** — five named repair tests |
| `go test -count=1 ./internal/embedding/...` | **PASS** |
| `go test -count=1 -race ./internal/embedding/...` | **PASS** |
| `go test -count=1 ./internal/store/... ./internal/prompt/...` | **PASS** — store, prompt, prompt/sync |
| `GOEXPERIMENT=simd go test -count=1 -tags simd ./internal/embedding/...` | **PASS** — package 3.939s |
| strict corpus `--check --strict --verify` | **PASS** — verify 7.712s; 18 Markdown; 4 cards; 0 legacy/broken links/unresolved refs/missing README sections |
| `git diff --check` scoped to embedding corpus and package | **PASS** |

The fixed-profile verifier executed
`go test -count=1 ./internal/embedding/...` against the commit and dirty-tree
fingerprint above. Direct focused/race/downstream jobs also passed; their wall
times were inflated by concurrent Go compilation and are not performance claims.

## Signed human-quality score

| Dimension | Score | Cited reason |
|---|---:|---|
| Human orientation | 3 | README explains the user-visible journey and failure behavior in plain language |
| North-star alignment | 3 | README/vision pin semantic substrate below the Mangle executive boundary |
| Evidence integrity | 3 | current/partial/uplift labels point to symbols, named tests, and receipts |
| Architecture clarity | 3 | boundaries, provider flow, state machines, ownership, and alternatives are explicit |
| Data and logic contract | 3 | vector and batch shape, task, dimensions, identity gap, and leaf boundary are pinned |
| Lifecycle completeness | 3 | construction, ensure/pull, retry, batching, cancellation, degradation, and recovery are covered |
| Deterministic safety | 3 | provider trust boundary, resource bounds, side effects, and default-deny non-ownership are explicit |
| JIT and agent behavior | 2 | concrete prompt consumers and atom retrieval are traced; package correctly owns no atom budget/selection policy |
| Ecosystem wiring | 3 | factory, chat, store, prompt, perception, MCP, campaign, init, CLI, and tools are traced |
| Operations | 3 | config, health, logs, failures, reembed, stats, privacy, and recovery are documented |
| Verification | 3 | focused, package, race, downstream, experimental, and strict fixed-profile gates passed |
| Uplift quality | 3 | verified truth repair leads to identity, health receipt, and bounded shadow lab |
| Navigation/governance | 3 | exact routes, single TODO authority, stable IDs, legacy deletion evidence, and scoped guidance |
| Consistency | 3 | all canonical surfaces distinguish observed response shape from unresolved persisted identity |

**Signed score: 41/42 — PASS.** The only intentionally non-exceptional lane is
JIT/agent behavior because embedding is a leaf provider and must not own prompt
selection, token budgets, or agent lifecycle.

## Bounded residuals

1. Provider/model/task/dimension/normalization identity is not yet one enforced
   persisted contract; reembed remains operator-coordinated.
2. `internal/system/factory.go` and interactive chat boot do not share one
   health/degradation decision.
3. GenAI live EmbedContent/rate-limit behavior and async batch-job completion are
   not exercised by the default package suite.
4. Ollama response-body reads have time bounds and post-decode validation but no
   package-specific byte ceiling.
5. Experimental SIMD is verified as an optional lane, not accepted as a release default.
