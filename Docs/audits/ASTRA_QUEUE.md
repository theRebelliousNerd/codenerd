# Astra audit queue

Every audit an external researcher ("Astra") publishes on `origin/research-main-*` is merged into
`Docs/audits/` and addressed: a failing test that pins the defect, the root fix, and the commit
recorded here. Each owner works through its list in order. "Owner" is a codeNERD builder lane,
codeNERD's own ladder runs (`Docs/journeys/05-elite-harness-ladder.md`), or the root session.
An audit the code has already moved past is closed with the evidence.

Status: open, in progress, fixed `<commit>`, closed (`<reason>`).

## Root session: OpenAI and Responses client

| audit | branch | status |
|---|---|---|
| `openai-model-transition-effort-admission` | `research-main-20261029-2049` | fixed `f2ef38dd2` |
| `responses-local-hosted-tool-name-contract` | `research-main-20261029-2049` | fixed `f2ef38dd2` |
| `responses-refusal-wire-identity` | `research-main-20261029-1950` | fixed `cf55fc149` |
| `responses-assistant-phase-replay` | `research-main-20261029-1950` | fixed `89e4a65c2` |
| `exact-model-pin-tag-identity` | `research-main-20261029-1018` | fixed `e73911826` |
| `streaming-grounding-source-ownership` | `research-main-20261029-1018` | fixed `ad6c2c79d` |
| `ollama-shard-model-override` | `research-main-20261029-2152` | fixed `40d647187` |
| `exact-model-numeric-tag-residual` | `research-main-20261030-0017` | fixed `f12d823d0` |

## Lane A: experts, JIT, then session verdicts and test gates

| audit | branch | status |
|---|---|---|
| `expert-selection-temporary-fact-ownership` | `research-main-20261029-2049` | fixed `6dce40d90` (lane) |
| `specialist-roster-workspace-identity` | `research-main-20261029-1018` | fixed `53d7b8f80` (lane) |
| `knowledge-consultation-literal-identity` | `research-main-20261029-1018` | fixed `109761c4c` (lane) |
| `jit-co-use-observation-budget` | `research-main-20261029-1018` | fixed `3294c4682` |
| `jit-learning-semantic-identity-admission` | `research-main-20261029-1018` | fixed `e5944d38f` |
| `jit-relevance-carry-work-budget` | `research-main-20261029-1018` | fixed `1260b5f9f` |
| `retry-jit-callability-continuity` | `research-main-20261029-1018` | fixed `ee79110dc` |
| `baseline-exemption-tagged-test-obligations` | `research-main-20261029-1018` | fixed `cbcaf6630` |
| `explicit-package-selector-fidelity` | `research-main-20261029-0920` | fixed `d21bf9830` |
| `net-change-test-plan-applicability` | `research-main-20261029-0946` | fixed `5874f96c9` |
| `tagged-test-constraint-witness` | `research-main-20261029-1018` | fixed `782375c70` |
| `test-gate-memoization-identity` | `research-main-20261029-0850` | fixed `b1609df24` |
| `test-removal-subject-identity` | `research-main-20261029-0850` | fixed `99da1e661` |
| `tester-continuation-proposal-admission` | `research-main-20261029-1018` | fixed `d14597d56` |
| `review-feedback-identity-and-current-verdicts` | `research-main-20261029-1018` | fixed `d2f02c461` |
| `revision-sentinel-build-exclusion-parity` | `research-main-20261029-1018` | fixed `73215933f` |
| `omitted-argument-admission-order` | `research-main-20261029-1018` | fixed `41cc23f35` |
| `system-execution-outcome-projection` | `research-main-20261029-1018` | fixed `5be388e13` |
| `expert-lending-dependency-contract` | `research-main-20261029-2152` | fixed `a78f92ed0` |
| `slash-command-specialist-target-continuity` | `research-main-20261029-2221` | fixed `d1bd97fc9` |
| `planned-step-expert-consultation-continuity` | `research-main-20261029-2241` | fixed `ea0e0a6d3` |
| `advisory-repair-continuation` | `research-main-20261029-2258` | fixed `23e3a6237` |
| `expert-critique-domain-coverage` | `research-main-20261029-2258` | fixed `1e9e34bb6` |
| `plan-bound-expert-decks` (innovation) | `research-main-20261029-2258` | built: cards, dealing, hand-offs, routing ladder, mandates (`43b8c22a3`) |
| `knowledge-refresh-prune` | `research-main-20261030-0141` | fixed `32ad8379d` |
| `grounded-search-terminal-status` | `research-main-20261030-0141` | fixed `5fc51cc52` |
| `expert-card-scope-continuity` | `research-main-20261030-0211` | fixed `aae6b7eb1` |
| `expert-card-repair-continuity` | `research-main-20261030-0211` | fixed `e6218a8b7` |
| `knowledge-miss-retirement` | `research-main-20261030-0211` | fixed `dca4c1d4e` |
| `mangle-schema-rollback` | `research-main-20261030-0211` | fixed `e745223d9` |
| `effect-recovery-closure` | `research-main-20261030-0211` | fixed `86db8d747` |
| `regenerate-deletion-observation` | `research-main-20261030-0211` | fixed `0b1dab261` |
| `working-handle-allocation` | `research-main-20261030-0211` | open (Lane B, fixed on branch) |
| `expert-roster-domain-retirement` | `research-main-20261030-0211` | open (Lane A, fixed on branch) |

## Lane B: campaign and verification, then the Mangle kernel and world model

| audit | branch | status |
|---|---|---|
| `campaign-terminal-publication` | `research-main-20261029-2049` | fixed `df61b6786` (lane) |
| `campaign-ui-run-ownership` | `research-main-20261029-1950` | fixed `0abba9002` (lane) |
| `campaign-context-seed-scheduler-ownership` | `research-main-20261029-1018` | fixed `f58648041` (lane) |
| `recurse-compensation-write-ownership` | `research-main-20261029-1018` | fixed `3b6b6e1e0 + c7e090406` (lane) |
| `residual-tool-generation-restriction-admission` | `research-main-20261029-1018` | fixed `250dd7c79` |
| `verification-cancellation-after-deadline` | `research-main-20261029-0736` | fixed `f8bbdeb55` |
| `verification-process-cleanup-parity` | `research-main-20261029-0736` | fixed `ac6a2ae9d` |
| `mangle-content-fingerprint-typed-identity` | `research-main-20261029-1018` | fixed `3d95367ff` |
| `mangle-generic-merge-invalidation` | `research-main-20261029-0920` | fixed `2f1818f54` |
| `mangle-inclusion-projection-closure` | `research-main-20261029-0850` | fixed `c839c8549` |
| `mangle-overlay-raw-store-ownership` | `research-main-20261029-0946` | fixed `b72d18d79` |
| `projection-cache-allocation-acceptance` | `research-main-20261029-1018` | parked: the audited code exists only on an unmerged lane branch |
| `why-query-scope-preservation` | `research-main-20261029-1018` | fixed `b345d26b3` |
| `world-cache-rolling-format-admission` | `research-main-20261029-1018` | parked: the audited code exists only on an unmerged lane branch |
| `scanner-graph-source-retirement` | `research-main-20261029-1018` | fixed `4123cef79` |
| `symbol-focus-identity-continuity` | `research-main-20261029-1018` | fixed `f8c94b5b6` |
| `run_lint describes its build tags` (root finding, R1-27 rerun: the model believed run_lint takes no tags; it applies the workspace build env) | -- | fixed `c6659ef7d` |
| `gate-comparison-domain-admission` | `research-main-20261029-2152` | fixed `26649e371` |
| `gate-scoped-recurse-current-main-addendum` | `research-main-20261029-2221` | fixed `885fd6fde` |
| `run-lint-effective-build-tag-receipt` | `research-main-20261029-2241` | fixed `b2f5b2abe` |
| `scoped-gate-overlay-evidence-conservation` | `research-main-20261029-2241` | fixed `6cb53c9d4` |
| `compile-diagnostic-witness-admission` | `research-main-20261029-2258` | fixed `862bfe779` |
| `coverage-source-identity-witness` | `research-main-20261029-2258` | fixed `2065d2642` |
| `workspace-test-receipt-invocation-identity` | `research-main-20261030-0017` | fixed `077bd07e2` |

## codeNERD ladder runs: store, learning and memory, then chat and TUI

One audit per run, briefed as its symptom (the ladder's brief rule), in this order.

| audit | branch | status |
|---|---|---|
| `knn-recall-limit-error-contract` | `research-main-20261029-1018` | fixed `6c6a798ac` |
| `legacy-vector-repair-attempt-lifecycle` | `research-main-20261029-1018` | fixed `8ec6b638f` |
| `knowledge-index-repair-cache-freshness` | `research-main-20261029-1018` | fixed `44d8a7d95` |
| `embedding-stats-observation-boundary` | `research-main-20261029-1018` | fixed `27bc13b05` |
| `learned-example-literal-fidelity` | `research-main-20261029-0736` | fixed `8f8a2bb03` |
| `learning-lexical-candidate-window` | `research-main-20261029-1018` | fixed `89dbd36d1` |
| `manual-learning-evidence-conservation` | `research-main-20261029-1018` | fixed `4b7fe31fa` |
| `project-knowledge-lexical-term-budget` | `research-main-20261029-1018` | fixed `e178e465e` |
| `user-memory-archival-lifecycle` | `research-main-20261029-1950` | fixed `073bffe69` |
| `document-extraction-coverage-resumption` | `research-main-20261029-1018` | fixed `a285f0583` |
| `reflection-observation-publication` | `research-main-20261029-1018` | fixed `b68849adb` |
| `queued-slash-command-kind-conservation` | `research-main-20261029-1018` | fixed `40da23666` |
| `slash-operation-slot-admission` | `research-main-20261029-1018` | fixed `c3b5bef85` |
| `tui-legislator-dispatch-parity` | `research-main-20261029-1018` | fixed `9e564f857` |
| `interaction-audit-surface-projection-parity` | `research-main-20261029-1018` | fixed `b21fd586c` |
| `taxonomy-initialization-completeness` | `research-main-20261029-0736` | fixed `31123a4f4` |
| `queued-history-projection` | `research-main-20261030-0017` | fixed `2aa65dfda` |
| `git-sysfacts-porcelain` | `research-main-20261030-0141` | fixed `e2e4ddac6` |

## Codex lanes (2026-10-30)

One Codex lane per audit, each in its own worktree; the root runs the full suite and merges.

| audit | branch | status |
|---|---|---|
| `mangle-edb-alias-support` | `research-main-20261030-0442` | fixed `70a20472a` |
| `guardian-module-alignment-scope` | `research-main-20261030-0513` | fixed `135405dfc` |
| `tui-search-evidence-coverage` | `research-main-20261030-0513` | fixed `3c16ef3b5` |
| `world-refresh-publication-order` | `research-main-20261030-0513` | fixed `63a406d90` |
| `consultation-cache-snapshot` | earlier branch | fixed `39c4e9268` |
| `shard-db-registration-ownership` | earlier branch | fixed `6b745bba7` |
| innovation `mission-driven-forever-improvement` | `research-main-20261030-0442` | built `388829653` |

## Codex lanes (2026-10-30, second batch)

| audit | branch | status |
|---|---|---|
| `deliberation-terminal-budget` | `research-main-20261030-0642` | fixed `95477e1ce` |
| `prompt-evolution-screening-cohort` | `research-main-20261030-0544` | fixed `07e08faaf` |
| `verifier-change-evidence-coverage` | `research-main-20261030-0544` | fixed `f01bfee19` |
| `self-restart-failure-recovery` | `research-main-20261030-0544` | fixed `47994bfef` |
| `dev-policy-reload-adoption` | `research-main-20261030-0614` | fixed `47994bfef` |
| `jit-strategy-context-ownership` | `research-main-20261030-0614` | fixed `261585231` |
| `recurse-gate-contract-continuity` | `research-main-20261030-0614` | fixed `5efbc6782` |
| `recurse-gate-subsumption` | `research-main-20261030-0642` | fixed `5efbc6782` |
| `knowledge-synthesis-answer-identity` | `research-main-20261030-0642` | fixed `3d5699872` |
