---
doc-class: shipped-with-future
subsystem: cli
implementation-status: partial
last-verified: 2026-10-02
verified-against: db1d4b7e24d0f9440c8627b2561ca64a25876954
supersedes: []
---

# Prompt seed maintenance

The implementation belongs to the `tools` corpus, whose `corpus.toml` owns `cmd/tools`; this is adjacent operator documentation, not a second source-root claim. GAP-TOOLS-PROMPT-SEED-01 in [tools gap analysis](../tools/03-GAP-ANALYSIS.md) owns the implementation contract. Root ownership validation rejected an attempted overlapping CLI root before accepting further source work.

**VERIFIED CURRENT:** `cmd/tools/prompt_builder/main.go#createDatabase` (`cmd/tools/prompt_builder/main.go:543`) removes its output before ordinary generation; a `-skip-embeddings` rebuild would discard existing vectors. `internal/prompt/reconciler.go#ReconcilePromptCorpus` (`internal/prompt/reconciler.go:43`) provides transactional canonical-input retention. The historical full prompt gate rejected stale perception atom seed content in `artifact:.corpus-build/runs/all-features-20261002/round3-perception.stdout.log`; the bounded correction receipt below supersedes that stale-seed result.

**PROPOSED UPLIFT — GAP-CLI-PROMPT-SEED-01:** explicit `-reconcile-embedded` mode opens only an existing valid corpus, reconciles compiled canonical atoms through the production prompt API and reports actual upsert/delete/retained/cleared counts. It never removes/truncates the target, contacts an embedding provider, fabricates vectors or silently ignores a conflicting custom input. Ordinary generation remains unchanged. Unchanged embedding input retains exact vector/task bytes; changed input invalidates them. Nonblank description is the embedding input, otherwise content: a content-only change with unchanged description retains its valid vector.

## Exit evidence

Real SQLite regressions cover exact unchanged-vector/task retention, changed-input invalidation, content-only description retention, canonical content/tags and project ownership, missing/invalid target refusal without replacement, cancellation/failure and credential-free mode. Root builds the command with sqlite_vec, runs it against an owned candidate seed copy, checks integrity/content parity and per-atom vector/task equality before publishing the generated asset. Freshness and normal first-boot/JIT consumption are separate required witnesses. Active workspace databases, schema/policy, global configuration and protected files are outside this packet.

## Vector qualification boundary

**VERIFIED CURRENT:** the shipped seed inspected on 2026-10-02 has 711 nonempty vectors: 419 with 768 dimensions and 292 with 3072 dimensions. Those rows lack stored descriptions and per-atom model provenance; its legacy `vec_index` has 768 dimensions and `vec_prompt_atoms` is absent. Reconciliation may legitimately invalidate many vectors when canonical descriptions change their effective input. Retained bytes prove input-based retention only, not compatible model namespaces, task eligibility or live ANN retrieval. Report actual retained/cleared counts and inspect legacy-index consumers before generated-asset publication; never promise preservation of all 711 vectors or fabricate their provenance.

**PARTIAL — root gate:** `artifact:.corpus-build/runs/all-features-20261002/round4-seed-mode.receipt.json` records exit 1. Invalid-target refusal controls pass, but valid-corpus controls stop during fixture creation because `seed ?# ü.db` is not a legal Windows filename. Keep spaces, URI-sensitive legal characters and Unicode coverage with platform-valid paths, and retain any POSIX-only filename controls separately. Shared reconciler task/ownership obligations are pinned by GAP-PROMPT-RECONCILE-PRESERVATION; this failure does not close them or authorize generated-seed publication.

**VERIFIED CURRENT — bounded correction:** `round5-reconciler-builder.receipt.json` passes the real shared reconciler and builder controls after the platform-path and preservation repairs. The actual sqlite_vec command passes against an owned copy in `round5-seed-candidate.receipt.json`; `round5-seed-retention.json` independently checks all 923 rows, exact vector/task retention for 682 rows, legitimate changed-input invalidation for 29 rows and SQLite integrity. `round5-seed-publication.json` records the generated seed update and preserved pre-update backup, not a Git push. Only perception_understanding changes content. Retained vectors remain unstamped; this does not qualify semantic namespaces or legacy ANN indexes. Full dependent suites and normal first-boot/JIT consumption remain required.
