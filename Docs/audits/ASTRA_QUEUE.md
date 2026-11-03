# Astra audit queue

Every audit an external researcher ("Astra") publishes on `origin/research-main-*` is merged into
`Docs/audits/` and addressed: a failing test that pins the defect, the root fix, and the commit
recorded here. Each owner works through its list in order. "Owner" is a codeNERD builder lane,
codeNERD's own ladder runs (`Docs/journeys/05-elite-harness-ladder.md`), or the root session.
An audit the code has already moved past is closed with the evidence.

Status: open, in progress, fixed `<commit>`, closed (`<reason>`).

## Disposition vocabulary

- `open (untriaged)` — no disposition has been established.
- `confirmed-open` — a current finding has been confirmed and remains open.
- `verified-fixed <commit>` — the fix and its verification are recorded at the cited commit.
- `stale-closed (reason)` — the finding no longer applies; record why.
- `duplicate-of` — the finding is tracked by another identified item.
- `proposal-decision` — record the decision on a proposal or design.
- `deferred` — work is postponed; record why.
- `declined` — work will not be pursued; record why.
- `blocked-protected-path` — the necessary change is blocked by path protection.

Existing historical statuses are retained as recorded; they are not automatically verified by this vocabulary.

## Census manifest

- **Census HEAD revision:** `519ac934b6780f67b7401fd6d4f424a1f23e2d6f`.
- **Census document count:** 312, including the queue. The queue is excluded from the list below.
- **Short content hash for each document listed below:** `unknown`. The census did not calculate content hashes.

All names below have the path prefix `Docs/audits/`. This is the census list, not a verified recursive inventory; the census could not reliably determine whether subdirectories were present.

AUDIT.md, LEAK_FINDINGS.md, MANGLE_FACT_APIS_INVENTORY.md, UPLIFT_LEDGER.md, WRITE_MUTATION_INVENTORY.md, acceptance-fixture-census.md, advisory-repair-continuation.md, agent-native-architecture-grand-design.md, anthropic-native-context-admission.md, approval-expiry-journal-failure.md, approval-oversized-preview-metadata.md, archive-prefix-identity.md, articulation-admission-and-control-fidelity.md, articulation-operation-ownership.md, atom-duplicate-serving-domain-containment.md, background-outcome-and-continuation-ownership.md, baseline-exemption-tagged-test-obligations.md, brief-go-intent-handoff.md, browser-evidence-url-userinfo.md, campaign-acceptance-case-identity.md, campaign-analysis-artifact-role-continuity.md, campaign-capability-checkpoint-continuity.md, campaign-checkpoint-build-profile.md, campaign-checkpoint-report-freshness.md, campaign-checkpoint-scope-idempotence.md, campaign-completion-storage-hold.md, campaign-context-seed-scheduler-ownership.md, campaign-deletion-microcheckpoint.md, campaign-evidence-workspace-admission.md, campaign-finalization-artifact-scope.md, campaign-generation-admission-and-recovery.md, campaign-intake-pause-ownership.md, campaign-learning-receipt-fidelity.md, campaign-pending-phase-admission.md, campaign-recurse-scheduling-ownership.md, campaign-refinement-dependency-kind.md, campaign-refinement-scope-deduplication.md, campaign-reproduction-outcome-contract.md, campaign-resume-return-evidence.md, campaign-rollback-file-metadata.md, campaign-rollback-permission-mode.md, campaign-specialist-retrieval-contract.md, campaign-terminal-publication.md, campaign-ui-run-ownership.md, chat-expert-role-projection.md, chat-session-memory-ownership-and-recovery.md, classifier-fallback-scheduler-ownership.md, codedom-governed-code-replication.md, coder_token_cost_proposal.md, compile-diagnostic-witness-admission.md, compressed-control-outcome-fallback.md, compressed-observation-escape-fidelity.md, configuration-mutation-ownership.md, consultation-cache-snapshot.md, consultation-coordination-and-safe-reuse.md, consultation-knowledge-semantic-admission.md, context-calibration-model-binding.md, context-feedback-cache-publication.md, context-feedback-independent-observations.md, context-ledger-prefix-costs.md, context7-coverage-notice-continuity.md, context7-documentation-ref-continuity.md, continuation-obligation-generation.md, conversation-brief-semantic-fidelity-and-progressive-disclosure.md, conversation-history-coverage-handoff.md, correction-learning-answer-ownership.md, coverage-source-identity-witness.md, credential-pool-failure-scope-and-recovery.md, delegated-turn-failure-settlement.md, delegated-verdict-handoff.md, delegation-target-substring-scope.md, deliberation-read-invocation-identity.md, deliberation-terminal-budget-continuation.md, deliberation-terminal-budget.md, delivery-continuation-restriction.md, delivery-terminal-answer-ownership.md, detailed-usage-wiring-and-verified-efficiency.md, dev-policy-reload-adoption.md, document-extraction-coverage-resumption.md, documentation-evidence-contract.md, dream-confirmation-interception.md, dream-plan-foreground-slot.md, dream-plan-queue-conservation.md, edit-observation-precondition-continuity.md, effect-recovery-closure.md, embedding-representation-and-migration.md, embedding-stats-observation-boundary.md, evolved-atom-filename-identity-continuation.md, exact-model-numeric-tag-residual.md, exact-model-pin-tag-identity.md, expert-card-repair-continuity.md, expert-card-scope-continuity.md, expert-critique-domain-coverage.md, expert-lending-dependency-contract.md, expert-mandate-test-identity.md, expert-roster-domain-retirement.md, expert-selection-temporary-fact-ownership.md, explicit-deep-scan-admission.md, explicit-package-selector-fidelity.md, gate-comparison-domain-admission.md, gate-scoped-recurse-current-main-addendum.md, gemini-secondary-output-cap.md, generated-call-receipt-retention.md, generated-feedback-jev-accounting-idempotency.md, generated-jev-call-fact-ownership.md, generated-learning-aggregate-supersession.md, generated-learning-event-domain.md, generated-learning-multiwriter-publication.md, generated-test-authority-envelope.md, generated-tool-observation-fidelity.md, generated-tool-quality-profile-wiring.md, generated-tool-refinement-candidate-continuity.md, generated-tool-retirement-revision-identity.md, generated-tool-retirement-version-admission.md, git-sysfacts-porcelain.md, glassbox-sink-isolation-and-export-health.md, grounded-search-terminal-status.md, guardian-module-alignment-scope.md, interaction-audit-surface-projection-parity.md, interaction-trace-request-attribution.md, internal-world-risk-report.md, jit-co-use-observation-budget.md, jit-dependency-closed-budgeting.md, jit-generation-aware-optimization.md, jit-learning-semantic-identity-admission.md, jit-relevance-carry-work-budget.md, jit-strategy-context-ownership.md, jsonl-independent-writer-rotation.md, knn-recall-limit-error-contract.md, knowledge-acquisition-observed-version-receipts.md, knowledge-acquisition-subject-ownership.md, knowledge-consultation-literal-identity.md, knowledge-go-major-version-pin-continuity.md, knowledge-index-repair-cache-freshness.md, knowledge-miss-retirement.md, knowledge-refresh-prune.md, knowledge-refresh-publication-recovery.md, knowledge-repository-empty-snapshot-retirement.md, knowledge-synthesis-answer-identity.md, knowledge-synthesis-shard-grounding.md, learned-correction-integrity-lifecycle.md, learned-example-literal-fidelity.md, learned-rule-comment-admission.md, learning-index-zero-backlog-upgrade.md, learning-lexical-candidate-window.md, legacy-build-project-diagnostic-evidence.md, legacy-vector-repair-attempt-lifecycle.md, lsp-document-uri-identity-and-diagnostic-grounding.md, mangle-content-fingerprint-typed-identity.md, mangle-edb-alias-support.md, mangle-feature-adoption-contracts.md, mangle-generic-merge-invalidation.md, mangle-inclusion-projection-closure.md, mangle-kernel-performance.md, mangle-lsp-header-boundary.md, mangle-overlay-raw-store-ownership.md, mangle-schema-rollback.md, mangle-watcher-repair-publication-authority.md, manual-alignment-snapshot-and-coverage-continuity.md, manual-learning-evidence-conservation.md, marathon-init-resume-contract.md, mcp-capability-admission.md, mcp-retained-scalar-continuation.md, mcp-stdio-outbound-cancellation.md, mcp-versioned-wire-fidelity.md, memory-forget-literal-identity.md, memory-recall-and-forget-contract.md, mission-errorfree-witness-receipts.md, multistep-delivery-failure-evidence.md, multistep-fallback-condition.md, multistep-literal-case-continuity.md, native-keyword-context-allocation-and-byte-budgets.md, native-response-control-settlement.md, net-change-test-plan-applicability.md, northstar-lossless-strategic-editing.md, northstar-projection-ownership.md, northstar-sqlite-connection-policy.md, northstar-unchanged-projection-recovery.md, ollama-shard-model-override.md, omitted-argument-admission-order.md, openai-model-transition-effort-admission.md, orientation-history-rename-identity.md, orientation-treatment-disclosure-contract.md, pending_edit_inventory.md, perception-capability-recovery.md, perception-classification-outcomes-and-recovery.md, perception-classification-root-identity.md, perception-early-answer-evidence-admission.md, perception-query-persona-continuity.md, perception-restriction-continuity.md, planned-step-expert-consultation-continuity.md, planned-work-conservation.md, portable-recurse-deliberation-crucible.md, predicate-inventory-semantic-fidelity.md, project-knowledge-lexical-term-budget.md, projectdoc-incomplete-discovery-policy.md, projectdoc-logical-root-identity.md, projection-cache-allocation-acceptance.md, prompt-evolution-consumed-evidence-admission.md, prompt-evolution-lifecycle-and-evidence-accounting.md, prompt-evolution-screening-cohort.md, prompt-trial-exposure-admission.md, queued-history-projection.md, queued-slash-command-kind-conservation.md, read-continuation-position.md, recoverable-delete-index-assumptions.md, recurse-cargo-inherited-dependency-closure.md, recurse-collapsed-finding-invocation-identity.md, recurse-compensation-write-ownership.md, recurse-coverage-gate-identity.md, recurse-finding-census-completeness.md, recurse-gate-contract-continuity.md, recurse-gate-language-domain.md, recurse-gate-subsumption.md, recurse-go-test-dependency-closure.md, recurse-index-adoption-identity.md, recurse-no-tests-evidence-continuity.md, recurse-node-namespace-identity.md, recurse-prior-reason-budget.md, recurse-python-from-import-prerequisites.md, recurse-recovery-attempt-identity.md, recurse-root-crate-diagnostic-ownership.md, recurse-values-continuation-ownership.md, recurse-workspace-test-witness.md, reflection-observation-publication.md, regenerate-deletion-observation.md, regression-run-record-identity.md, request-bound-human-approval.md, request-visible-tool-contract-and-investigation-affordances.md, research-browser-ownership-and-evidence-retirement.md, research-cache-workspace-destination.md, research-observation-identity-and-admission.md, residual-tool-generation-restriction-admission.md, responses-assistant-phase-replay.md, responses-local-hosted-tool-name-contract.md, responses-refusal-wire-identity.md, retrieval-definition-scan-amplification.md, retrieval-delta-publication-fencing.md, retrieval-freshness-coverage-and-context-value.md, retrieval-go-module-directive-identity.md, retrieval-relevance-head-acquisition.md, retrieval-saturation-outcomes-and-producer-lifecycle.md, retry-jit-callability-continuity.md, review-feedback-identity-and-current-verdicts.md, review-finding-location-deduplication.md, review-fix-citation-scope.md, review-scope-remainder-continuity.md, review-severity-admission-continuation.md, revision-sentinel-build-exclusion-parity.md, run-lint-effective-build-tag-receipt.md, scanner-graph-source-retirement.md, scoped-gate-overlay-evidence-conservation.md, self-diagnosis-log-generation-coverage.md, self-restart-failure-recovery.md, semantic-corpus-observation-retirement.md, semantic-expansion-candidate-window.md, semantic-head-work-coalescing.md, session-learning-verb-persona-identity.md, sessionlog-durability-progress-responsiveness.md, shard-db-registration-ownership.md, shell-tail-effect-admission.md, side-turn-ordering-and-cancellation-ownership.md, slash-command-payload-integrity.md, slash-command-specialist-target-continuity.md, slash-operation-slot-admission.md, snapshot-roundtrip-semantics.md, specialist-roster-workspace-identity.md, strategy-refinement-claim-publication.md, strategy-refinement-scope-before-limit.md, stream-completion-contract.md, streaming-grounding-source-ownership.md, structure-coverage-and-refactor-preconditions.md, symbol-focus-identity-continuity.md, system-execution-outcome-projection.md, tactile-docker-execution-contract.md, tagged-test-constraint-witness.md, task-terminal-ownership.md, taxonomy-initialization-completeness.md, terminal-error-clarification-disposition.md, terminal-knowledge-synthesis-grounding.md, test-evidence-capability.md, test-gate-memoization-identity.md, test-removal-subject-identity.md, test-request-delegation-fidelity.md, test-verdict-diagnostic-coverage.md, tester-continuation-proposal-admission.md, tool-arg-coercion.md, tool-duplicate-substitute-retirement.md, tool-effect-receipts-and-validation-completeness.md, tool-value-channel-opportunity.md, tui-evidence-turn-ownership.md, tui-legislator-dispatch-parity.md, tui-search-evidence-coverage.md, usage-cross-process-retry-accounting.md, user-memory-archival-lifecycle.md, verification-applicability-and-progressive-evidence.md, verification-cancellation-after-deadline.md, verification-evidence-and-judge-contracts.md, verification-process-cleanup-parity.md, verified-delivery-change-evidence.md, verifier-change-evidence-coverage.md, why-query-scope-preservation.md, working-context-nul-recall.md, working-handle-allocation.md, working-observation-supersession-contract.md, workspace-membership-failure-and-incremental-census.md, workspace-search-navigation-fidelity.md, workspace-test-receipt-invocation-identity.md, world-cache-rolling-format-admission.md, world-holographic-continuation.md, world-refresh-publication-order.md


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

## Approved identity batch (2026-10-30)

The original reports below are pinned on the research refs linked here; this
batch does not merge those branches or their other proposals. Implementation,
owning specifications and regression/integration tests are in `00bf8968b` on
`codex/audit-identity-batch-20261030`, based on main `6cbe3962f`.

| audit | branch | status |
|---|---|---|
| [retrieval-go-module-directive-identity](https://github.com/theRebelliousNerd/codenerd/blob/921628ea7b1af635cb9c99d4055f6feed2a5efeb/Docs/audits/retrieval-go-module-directive-identity.md) | `research-main-20261030-1147` | fixed `00bf8968b` |
| [knowledge-go-major-version-pin-continuity](https://github.com/theRebelliousNerd/codenerd/blob/07bdcb50873498d22fce498f599073df00ac5750/Docs/audits/knowledge-go-major-version-pin-continuity.md) | `research-main-20261030-0850` | fixed `00bf8968b` |
| [context7-documentation-ref-continuity](https://github.com/theRebelliousNerd/codenerd/blob/b42816320d9164c824aba7da0c54bc27df2b4279/Docs/audits/context7-documentation-ref-continuity.md) | `research-main-20261030-1018` | fixed `00bf8968b` |

The new witnesses fail against the original implementations. Focused checks and
all six affected packages with `sqlite_vec` pass, including real session/Cortex
paths and local-browser acquisition through the production expert compiler.
Retrieval/session focused race checks and the tagged CLI build also pass. These
establish the named identity contracts, not live-model quality or cost effects.

## Census documents awaiting triage

These rows cover documents in the census above that did not already have a row. `--` means the branch is not recorded here. The census is not a verified recursive inventory; existing historical rows retain their recorded statuses without implying independent verification.

Remaining `open (untriaged)` rows are unverified: the cited code has not been read for those rows in the current pass. Earlier research remains evidence for later investigation, not a current-turn disposition.

Directory discrepancy: prior research reported `read_file` not-found for `new`, `sweeps`, `gemini`, `gpt`, `glm`, `kimi`, `mimo`, and `minimax`, but its recursive listing was refused. Their current presence is unverified; no subdirectory documents were dispositioned in this pass.

| audit | branch | status |
|---|---|---|
| `ASTRA_QUEUE` | -- | open (untriaged) |
| `AUDIT` | -- | open (untriaged) |
| `LEAK_FINDINGS` | -- | open (untriaged) |
| `MANGLE_FACT_APIS_INVENTORY` | -- | open (untriaged) |
| `UPLIFT_LEDGER` | -- | open (untriaged) |
| `WRITE_MUTATION_INVENTORY` | -- | open (untriaged) |
| `acceptance-fixture-census` | -- | confirmed-open: acceptance-fixture-census#1, defect (conditional, source only): the protected-input census and snapshot both filter by workspace membership, `internal/evidence/change.go:204-218,273-277,402-413`; working-tree read with HEAD `1e902faad7974b133aac17920c125c2f6187cf41`. Excluded-fixture execution witness and remaining subitems unverified. |
| `agent-native-architecture-grand-design` | -- | open (untriaged) |
| `anthropic-native-context-admission` | -- | open (untriaged) |
| `approval-expiry-journal-failure` | -- | open (untriaged) |
| `approval-oversized-preview-metadata` | -- | open (untriaged) |
| `archive-prefix-identity` | -- | open (untriaged) |
| `articulation-admission-and-control-fidelity` | -- | open (untriaged) |
| `articulation-operation-ownership` | -- | open (untriaged) |
| `atom-duplicate-serving-domain-containment` | -- | open (untriaged) |
| `background-outcome-and-continuation-ownership` | -- | open (untriaged) |
| `brief-go-intent-handoff` | -- | open (untriaged) |
| `browser-evidence-url-userinfo` | -- | open (untriaged) |
| `campaign-acceptance-case-identity` | -- | confirmed-open: campaign-acceptance-case-identity#1, defect (conditional, source only): case-insensitive deduplication can merge distinct paths on case-sensitive filesystems, `internal/campaign/orchestrator_acceptance.go:314-326,342-350`; the resulting scope is assigned to the acceptance remediation task at `internal/campaign/orchestrator_acceptance.go:251-255,283-292`. Working-tree read with HEAD `1e902faad7974b133aac17920c125c2f6187cf41` (element revs `fade04239e1d`, `b5a6bd5cf680`); execution witness and remaining subitems unverified. |
| `campaign-analysis-artifact-role-continuity` | -- | open (untriaged) |
| `campaign-capability-checkpoint-continuity` | -- | open (untriaged) |
| `campaign-checkpoint-build-profile` | -- | open (untriaged) |
| `campaign-checkpoint-report-freshness` | -- | confirmed-open: campaign-checkpoint-report-freshness#1, defect (conditional, source only): a fixed npm report may replace current counts without invocation-freshness checks, `internal/campaign/checkpoint.go:168-170,218-229`; working-tree read with HEAD `1e902faad7974b133aac17920c125c2f6187cf41`. Execution witness and #2 proposal unverified. |
| `campaign-checkpoint-scope-idempotence` | -- | confirmed-open: campaign-checkpoint-scope-idempotence#1, defect (conditional, source only): scope takes the parent of every task write-set entry, including prior remediation scopes, `internal/campaign/orchestrator_checkpoint_remediation.go:60,79-89,111-131`; working-tree read with HEAD `1e902faad7974b133aac17920c125c2f6187cf41`. Runtime consequence and #2 proposal unverified. |
| `campaign-completion-storage-hold` | -- | confirmed-open: campaign-completion-storage-hold#1, defect (source only): the task completion path ignores the snapshot save result before publishing success, `internal/campaign/orchestrator_tasks.go:467-480`; the persistence wrapper emits a failure event and storage hold, `internal/campaign/orchestrator_lifecycle.go:125-139`. Working-tree read with HEAD `1e902faad7974b133aac17920c125c2f6187cf41`; dependent dispatch and #2 remain unverified. |
| `campaign-deletion-microcheckpoint` | -- | confirmed-open: campaign-deletion-microcheckpoint#1, defect (conditional, source only): for a nonempty write set the gate accepts a surviving path via `os.Stat` and fails if none survives, `internal/campaign/micro_checkpoint.go:23-35,40-56`; working-tree read with HEAD `1e902faad7974b133aac17920c125c2f6187cf41`. Admitted deletion route, runtime witness and remaining subitems unverified. |
| `campaign-evidence-workspace-admission` | -- | confirmed-open: campaign-evidence-workspace-admission#1, defect (conditional, source only): brief path admission rejects lexical escapes but uses `os.Stat`, while the evidence renderer resolves the named path and reads it without a symlink containment check, `internal/campaign/task_preload.go:57-73`, `internal/campaign/task_evidence.go:91-98,578-591`. Working-tree read with HEAD `1e902faad7974b133aac17920c125c2f6187cf41` (element revs `36d4a335b862`, `2f5d7795a5f0`, `fba28993be13`); end-to-end exploit witness and remaining subitems unverified. |
| `campaign-finalization-artifact-scope` | -- | confirmed-open: campaign-finalization-artifact-scope#1, defect (conditional, source only): a nonempty write set prevents artifact fallback, while the root sweep declares only basenames from the resolved write set and moves new undeclared root files, `internal/campaign/orchestrator_tasks.go:580-588,873-885,900-910`; `Run` invokes the sweep after acceptance and before completion, `internal/campaign/orchestrator_execution.go:180-185,219-238`. Working-tree read with HEAD `1e902faad7974b133aac17920c125c2f6187cf41` (element revs `5f93c485bb26`, `60f016214731`, `a926fb79d24d`); runtime witness and remaining subitems unverified. |
| `campaign-generation-admission-and-recovery` | -- | open (untriaged) |
| `campaign-intake-pause-ownership` | -- | confirmed-open: campaign-intake-pause-ownership#1, defect (conditional, source only): requirement intake restores its captured pause state even if `Resume` runs in between, `internal/campaign/orchestrator_utils.go:335-357`, `internal/campaign/orchestrator_control.go:31-44`. Working-tree read with HEAD `1e902faad7974b133aac17920c125c2f6187cf41`; interleaving, panic witness and remaining subitems unverified. |
| `campaign-learning-receipt-fidelity` | -- | confirmed-open: campaign-learning-receipt-fidelity#1, defect (source only): the completion path invokes `applyLearnings`, which records the first promotion argument and task text but not the second promotion argument, `internal/campaign/orchestrator_tasks.go:467-474,702-746`; the two-argument declaration is `internal/core/defaults/schemas_analysis.mg:96-99`. Working-tree read with HEAD `1e902faad7974b133aac17920c125c2f6187cf41`; runtime and remaining subitems unverified. |
| `campaign-pending-phase-admission` | -- | confirmed-open: campaign-pending-phase-admission#1, defect (conditional, source only): policy derives a current phase from an eligible phase, `internal/core/defaults/policy/campaign_phases.mg:52-55`; `Run` starts a phase only when no current phase was selected, but otherwise executes it, `internal/campaign/orchestrator_execution.go:177-179,253-255,287-288`; phase-start status and observer handling live in `internal/campaign/orchestrator_phases.go:405-411,425-426,456-461`. Working-tree read with HEAD `1e902faad7974b133aac17920c125c2f6187cf41`; actual policy evaluation, execution witness and remaining subitems unverified. |
| `campaign-recurse-scheduling-ownership` | -- | open (untriaged) |
| `campaign-refinement-dependency-kind` | -- | confirmed-open: campaign-refinement-dependency-kind#1, defect (source only): add and unmatched-update paths merge `depends_on` into `ContextFrom` without assigning `DependsOn`, `internal/campaign/replan.go:536-548,649-663,730-744`; working-tree read with HEAD `1e902faad7974b133aac17920c125c2f6187cf41`. Scheduling consequence, tests and remaining subitems unverified. |
| `campaign-refinement-scope-deduplication` | -- | confirmed-open: campaign-refinement-scope-deduplication#1, defect (source only): the refinement guard strips qualifying path suffixes and drops a matching task before applying its targets, `internal/campaign/replan.go:592-599,642-664,1253-1263`. Working-tree read with HEAD `1e902faad7974b133aac17920c125c2f6187cf41`; execution witness and remaining subitems unverified. |
| `campaign-reproduction-outcome-contract` | -- | confirmed-open: campaign-reproduction-outcome-contract#1, defect (conditional, source only): `/repro_first` inserts a test-run diagnostic as a dependency, `internal/campaign/orchestrator_failure.go:91-101,221-245`; unsuccessful tests return `/tests_red`, `internal/campaign/orchestrator_task_handlers.go:975-983`, while incomplete dependencies block, `internal/core/defaults/policy/campaign_tasks.mg:20-24`. Working-tree read with HEAD `1e902faad7974b133aac17920c125c2f6187cf41` (element revs `78f171793625`, `cb4346d7b776`, `ab66a2014d43`, `78a8698af1cd`); execution witness, policy evaluation and remaining subitems unverified. |
| `campaign-resume-return-evidence` | -- | confirmed-open: campaign-resume-return-evidence#2, defect (source only): missing or empty task returns are silently omitted, `internal/campaign/orchestrator_task_results.go:205-218`; `taskContextSection` and `buildTaskInput` feed that context into shard input, `internal/campaign/task_evidence.go:431-450`, `internal/campaign/orchestrator_task_results.go:173-199`. Working-tree read with HEAD `1e902faad7974b133aac17920c125c2f6187cf41`; resume behavior and remaining subitems unverified. |
| `campaign-rollback-file-metadata` | -- | confirmed-open: campaign-rollback-file-metadata#1, defect (conditional, source only): the file snapshot retains path, existence and bytes but not mode, and rollback writes a restored file using `0644`, `internal/campaign/orchestrator_task_transaction.go:23-27,389-398,538-546`. Working-tree read with HEAD `1e902faad7974b133aac17920c125c2f6187cf41` (element revs `37148452b7fe`, `ed260b6ef997`, `6e04ac6227bc`); deletion reachability, execution witness and remaining subitems unverified. |
| `campaign-rollback-permission-mode` | -- | duplicate-of: campaign-rollback-permission-mode#1 tracks the same missing snapshot mode and `0644` restore as campaign-rollback-file-metadata#1, `internal/campaign/orchestrator_task_transaction.go:23-27,538-546`. Working-tree read with HEAD `1e902faad7974b133aac17920c125c2f6187cf41`; deletion reachability, execution witness and remaining subitems unverified. |
| `campaign-specialist-retrieval-contract` | -- | confirmed-open: campaign-specialist-retrieval-contract#1, defect (conditional, source only): the provider opens a new store and calls semantic search, then falls back to a task-derived concept, `internal/campaign/specialist_knowledge.go:73-83,97-100,124-142`; the constructor does not attach an embedding engine, `internal/store/local_core.go:114-153`, the semantic method returns no results without one, and fallback matches concept exactly, `internal/store/local_knowledge.go:197-199,434-440`. Explicit-shard execution calls the enrichment path, `internal/campaign/orchestrator_task_handlers.go:298-303`; runtime retrieval, downstream JIT compensation and remaining subitems unverified. Working-tree read with HEAD `1e902faad7974b133aac17920c125c2f6187cf41` (element revs `8172b17954dd`, `54416854fdff`, `11904593f5b3`, `77eadfae97ff`, `2939cb6d5999`). |
| `chat-expert-role-projection` | -- | open (untriaged) |
| `chat-session-memory-ownership-and-recovery` | -- | open (untriaged) |
| `classifier-fallback-scheduler-ownership` | -- | open (untriaged) |
| `codedom-governed-code-replication` | -- | open (untriaged) |
| `coder_token_cost_proposal` | -- | open (untriaged) |
| `compressed-control-outcome-fallback` | -- | open (untriaged) |
| `compressed-observation-escape-fidelity` | -- | open (untriaged) |
| `configuration-mutation-ownership` | -- | open (untriaged) |
| `consultation-coordination-and-safe-reuse` | -- | open (untriaged) |
| `consultation-knowledge-semantic-admission` | -- | open (untriaged) |
| `context-calibration-model-binding` | -- | open (untriaged) |
| `context-feedback-cache-publication` | -- | open (untriaged) |
| `context-feedback-independent-observations` | -- | open (untriaged) |
| `context-ledger-prefix-costs` | -- | open (untriaged) |
| `context7-coverage-notice-continuity` | -- | open (untriaged) |
| `continuation-obligation-generation` | -- | open (untriaged) |
| `conversation-brief-semantic-fidelity-and-progressive-disclosure` | -- | open (untriaged) |
| `conversation-history-coverage-handoff` | -- | open (untriaged) |
| `correction-learning-answer-ownership` | -- | open (untriaged) |
| `credential-pool-failure-scope-and-recovery` | -- | open (untriaged) |
| `delegated-turn-failure-settlement` | -- | open (untriaged) |
| `delegated-verdict-handoff` | -- | open (untriaged) |
| `delegation-target-substring-scope` | -- | open (untriaged) |
| `deliberation-read-invocation-identity` | -- | open (untriaged) |
| `deliberation-terminal-budget-continuation` | -- | open (untriaged) |
| `delivery-continuation-restriction` | -- | open (untriaged) |
| `delivery-terminal-answer-ownership` | -- | open (untriaged) |
| `detailed-usage-wiring-and-verified-efficiency` | -- | open (untriaged) |
| `documentation-evidence-contract` | -- | open (untriaged) |
| `dream-confirmation-interception` | -- | open (untriaged) |
| `dream-plan-foreground-slot` | -- | open (untriaged) |
| `dream-plan-queue-conservation` | -- | open (untriaged) |
| `edit-observation-precondition-continuity` | -- | open (untriaged) |
| `embedding-representation-and-migration` | -- | open (untriaged) |
| `evolved-atom-filename-identity-continuation` | -- | open (untriaged) |
| `expert-mandate-test-identity` | -- | open (untriaged) |
| `explicit-deep-scan-admission` | -- | open (untriaged) |
| `gemini-secondary-output-cap` | -- | open (untriaged) |
| `generated-call-receipt-retention` | -- | open (untriaged) |
| `generated-feedback-jev-accounting-idempotency` | -- | open (untriaged) |
| `generated-jev-call-fact-ownership` | -- | open (untriaged) |
| `generated-learning-aggregate-supersession` | -- | open (untriaged) |
| `generated-learning-event-domain` | -- | open (untriaged) |
| `generated-learning-multiwriter-publication` | -- | open (untriaged) |
| `generated-test-authority-envelope` | -- | open (untriaged) |
| `generated-tool-observation-fidelity` | -- | open (untriaged) |
| `generated-tool-quality-profile-wiring` | -- | open (untriaged) |
| `generated-tool-refinement-candidate-continuity` | -- | open (untriaged) |
| `generated-tool-retirement-revision-identity` | -- | open (untriaged) |
| `generated-tool-retirement-version-admission` | -- | open (untriaged) |
| `glassbox-sink-isolation-and-export-health` | -- | open (untriaged) |
| `interaction-trace-request-attribution` | -- | open (untriaged) |
| `internal-world-risk-report` | -- | open (untriaged) |
| `jit-dependency-closed-budgeting` | -- | open (untriaged) |
| `jit-generation-aware-optimization` | -- | open (untriaged) |
| `jsonl-independent-writer-rotation` | -- | open (untriaged) |
| `knowledge-acquisition-observed-version-receipts` | -- | open (untriaged) |
| `knowledge-acquisition-subject-ownership` | -- | open (untriaged) |
| `knowledge-refresh-publication-recovery` | -- | open (untriaged) |
| `knowledge-repository-empty-snapshot-retirement` | -- | open (untriaged) |
| `knowledge-synthesis-shard-grounding` | -- | open (untriaged) |
| `learned-correction-integrity-lifecycle` | -- | open (untriaged) |
| `learned-rule-comment-admission` | -- | open (untriaged) |
| `learning-index-zero-backlog-upgrade` | -- | open (untriaged) |
| `legacy-build-project-diagnostic-evidence` | -- | open (untriaged) |
| `lsp-document-uri-identity-and-diagnostic-grounding` | -- | open (untriaged) |
| `mangle-feature-adoption-contracts` | -- | open (untriaged) |
| `mangle-kernel-performance` | -- | open (untriaged) |
| `mangle-lsp-header-boundary` | -- | open (untriaged) |
| `mangle-watcher-repair-publication-authority` | -- | open (untriaged) |
| `manual-alignment-snapshot-and-coverage-continuity` | -- | open (untriaged) |
| `marathon-init-resume-contract` | -- | open (untriaged) |
| `mcp-capability-admission` | -- | open (untriaged) |
| `mcp-retained-scalar-continuation` | -- | open (untriaged) |
| `mcp-stdio-outbound-cancellation` | -- | open (untriaged) |
| `mcp-versioned-wire-fidelity` | -- | open (untriaged) |
| `memory-forget-literal-identity` | -- | open (untriaged) |
| `memory-recall-and-forget-contract` | -- | open (untriaged) |
| `mission-errorfree-witness-receipts` | -- | open (untriaged) |
| `multistep-delivery-failure-evidence` | -- | open (untriaged) |
| `multistep-fallback-condition` | -- | open (untriaged) |
| `multistep-literal-case-continuity` | -- | open (untriaged) |
| `native-keyword-context-allocation-and-byte-budgets` | -- | open (untriaged) |
| `native-response-control-settlement` | -- | open (untriaged) |
| `northstar-lossless-strategic-editing` | -- | open (untriaged) |
| `northstar-projection-ownership` | -- | open (untriaged) |
| `northstar-sqlite-connection-policy` | -- | open (untriaged) |
| `northstar-unchanged-projection-recovery` | -- | open (untriaged) |
| `orientation-history-rename-identity` | -- | open (untriaged) |
| `orientation-treatment-disclosure-contract` | -- | open (untriaged) |
| `pending_edit_inventory` | -- | open (untriaged) |
| `perception-capability-recovery` | -- | open (untriaged) |
| `perception-classification-outcomes-and-recovery` | -- | open (untriaged) |
| `perception-classification-root-identity` | -- | open (untriaged) |
| `perception-early-answer-evidence-admission` | -- | open (untriaged) |
| `perception-query-persona-continuity` | -- | open (untriaged) |
| `perception-restriction-continuity` | -- | open (untriaged) |
| `planned-work-conservation` | -- | open (untriaged) |
| `portable-recurse-deliberation-crucible` | -- | open (untriaged) |
| `predicate-inventory-semantic-fidelity` | -- | open (untriaged) |
| `projectdoc-incomplete-discovery-policy` | -- | open (untriaged) |
| `projectdoc-logical-root-identity` | -- | open (untriaged) |
| `prompt-evolution-consumed-evidence-admission` | -- | open (untriaged) |
| `prompt-evolution-lifecycle-and-evidence-accounting` | -- | open (untriaged) |
| `prompt-trial-exposure-admission` | -- | open (untriaged) |
| `read-continuation-position` | -- | open (untriaged) |
| `recoverable-delete-index-assumptions` | -- | open (untriaged) |
| `recurse-cargo-inherited-dependency-closure` | -- | open (untriaged) |
| `recurse-collapsed-finding-invocation-identity` | -- | open (untriaged) |
| `recurse-coverage-gate-identity` | -- | open (untriaged) |
| `recurse-finding-census-completeness` | -- | open (untriaged) |
| `recurse-gate-language-domain` | -- | open (untriaged) |
| `recurse-go-test-dependency-closure` | -- | open (untriaged) |
| `recurse-index-adoption-identity` | -- | open (untriaged) |
| `recurse-no-tests-evidence-continuity` | -- | open (untriaged) |
| `recurse-node-namespace-identity` | -- | open (untriaged) |
| `recurse-prior-reason-budget` | -- | open (untriaged) |
| `recurse-python-from-import-prerequisites` | -- | open (untriaged) |
| `recurse-recovery-attempt-identity` | -- | open (untriaged) |
| `recurse-root-crate-diagnostic-ownership` | -- | open (untriaged) |
| `recurse-values-continuation-ownership` | -- | open (untriaged) |
| `recurse-workspace-test-witness` | -- | open (untriaged) |
| `regression-run-record-identity` | -- | open (untriaged) |
| `request-bound-human-approval` | -- | open (untriaged) |
| `request-visible-tool-contract-and-investigation-affordances` | -- | open (untriaged) |
| `research-browser-ownership-and-evidence-retirement` | -- | open (untriaged) |
| `research-cache-workspace-destination` | -- | open (untriaged) |
| `research-observation-identity-and-admission` | -- | open (untriaged) |
| `retrieval-definition-scan-amplification` | -- | confirmed-open: retrieval-definition-scan-amplification#1, concern: the definition fallback invokes a scan for each keyword; #2, defect: it reads hits but does not propagate the result cap, `internal/retrieval/tiered_context.go:697-710`; semantic expansion calls it at `internal/retrieval/tiered_context.go:633-645`. Working-tree read with HEAD `1e902faad7974b133aac17920c125c2f6187cf41`; cost and runtime witnesses unverified. |
| `retrieval-delta-publication-fencing` | -- | confirmed-open: retrieval-delta-publication-fencing#1, defect (conditional, source only): delta captures keyword keys then applies to the current same-key entry without an epoch or domain check, `internal/retrieval/keyword_cache.go:152-165,179-208`; working-tree read with HEAD `1e902faad7974b133aac17920c125c2f6187cf41`. Interleaving and remaining subitems unverified. |
| `retrieval-freshness-coverage-and-context-value` | -- | open (untriaged) |
| `retrieval-relevance-head-acquisition` | -- | confirmed-open: retrieval-relevance-head-acquisition#1, concern (source only): the candidate reads the entire file before limiting the excerpt, `internal/retrieval/relevance.go:75-90`; Tier 2 judging calls it at `internal/retrieval/relevance.go:42-50`. Working-tree read with HEAD `1e902faad7974b133aac17920c125c2f6187cf41`; cost and runtime witnesses unverified. |
| `retrieval-saturation-outcomes-and-producer-lifecycle` | -- | open (untriaged) |
| `review-finding-location-deduplication` | -- | open (untriaged) |
| `review-fix-citation-scope` | -- | open (untriaged) |
| `review-scope-remainder-continuity` | -- | open (untriaged) |
| `review-severity-admission-continuation` | -- | open (untriaged) |
| `self-diagnosis-log-generation-coverage` | -- | open (untriaged) |
| `semantic-corpus-observation-retirement` | -- | open (untriaged) |
| `semantic-expansion-candidate-window` | -- | open (untriaged) |
| `semantic-head-work-coalescing` | -- | open (untriaged) |
| `session-learning-verb-persona-identity` | -- | open (untriaged) |
| `sessionlog-durability-progress-responsiveness` | -- | open (untriaged) |
| `shell-tail-effect-admission` | -- | open (untriaged) |
| `side-turn-ordering-and-cancellation-ownership` | -- | open (untriaged) |
| `slash-command-payload-integrity` | -- | open (untriaged) |
| `snapshot-roundtrip-semantics` | -- | open (untriaged) |
| `strategy-refinement-claim-publication` | -- | open (untriaged) |
| `strategy-refinement-scope-before-limit` | -- | open (untriaged) |
| `stream-completion-contract` | -- | open (untriaged) |
| `structure-coverage-and-refactor-preconditions` | -- | open (untriaged) |
| `tactile-docker-execution-contract` | -- | proposal-decision: implement now for tactile-docker-execution-contract#1, defect (source only): consecutive `--` arguments precede the image in both `internal/tactile/docker.go:443-450` and `internal/tactile/persistent_docker.go:384-392`. Proposed write set: those two files and `internal/tactile/docker_platform_test.go` plus `internal/tactile/persistent_docker_unix_test.go` for complete argv assertions; verify with `go test -count=1 ./internal/tactile/...` and the required build environment. Docker execution not reproduced; test plan not run. Working-tree read with HEAD `1e902faad7974b133aac17920c125c2f6187cf41` (element revs `851aa8509ecb`, `3ab607261f80`); remaining subitems unverified. |
| `task-terminal-ownership` | -- | open (untriaged) |
| `terminal-error-clarification-disposition` | -- | open (untriaged) |
| `terminal-knowledge-synthesis-grounding` | -- | open (untriaged) |
| `test-evidence-capability` | -- | open (untriaged) |
| `test-request-delegation-fidelity` | -- | open (untriaged) |
| `test-verdict-diagnostic-coverage` | -- | open (untriaged) |
| `tool-arg-coercion` | -- | open (untriaged) |
| `tool-duplicate-substitute-retirement` | -- | open (untriaged) |
| `tool-effect-receipts-and-validation-completeness` | -- | open (untriaged) |
| `tool-value-channel-opportunity` | -- | open (untriaged) |
| `tui-evidence-turn-ownership` | -- | open (untriaged) |
| `usage-cross-process-retry-accounting` | -- | open (untriaged) |
| `verification-applicability-and-progressive-evidence` | -- | open (untriaged) |
| `verification-evidence-and-judge-contracts` | -- | open (untriaged) |
| `verified-delivery-change-evidence` | -- | open (untriaged) |
| `working-context-nul-recall` | -- | open (untriaged) |
| `working-observation-supersession-contract` | -- | open (untriaged) |
| `workspace-membership-failure-and-incremental-census` | -- | open (untriaged) |
| `workspace-search-navigation-fidelity` | -- | open (untriaged) |
| `world-holographic-continuation` | -- | open (untriaged) |