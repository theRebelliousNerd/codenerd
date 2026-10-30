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
| `review-feedback-identity-and-current-verdicts` | `research-main-20261029-1018` | open |
| `revision-sentinel-build-exclusion-parity` | `research-main-20261029-1018` | open |
| `omitted-argument-admission-order` | `research-main-20261029-1018` | open |
| `system-execution-outcome-projection` | `research-main-20261029-1018` | open |
| `expert-lending-dependency-contract` | `research-main-20261029-2152` | open |
| `slash-command-specialist-target-continuity` | `research-main-20261029-2221` | open |
| `planned-step-expert-consultation-continuity` | `research-main-20261029-2241` | open |
| `advisory-repair-continuation` | `research-main-20261029-2258` | open |
| `expert-critique-domain-coverage` | `research-main-20261029-2258` | open |
| `plan-bound-expert-decks` (innovation) | `research-main-20261029-2258` | open |

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
| `knn-recall-limit-error-contract` | `research-main-20261029-1018` | open |
| `legacy-vector-repair-attempt-lifecycle` | `research-main-20261029-1018` | fixed `8ec6b638f` |
| `knowledge-index-repair-cache-freshness` | `research-main-20261029-1018` | fixed `44d8a7d95` |
| `embedding-stats-observation-boundary` | `research-main-20261029-1018` | fixed `27bc13b05` |
| `learned-example-literal-fidelity` | `research-main-20261029-0736` | fixed `8f8a2bb03` |
| `learning-lexical-candidate-window` | `research-main-20261029-1018` | open (Lane B) |
| `manual-learning-evidence-conservation` | `research-main-20261029-1018` | open (Lane B) |
| `project-knowledge-lexical-term-budget` | `research-main-20261029-1018` | open (Lane B) |
| `user-memory-archival-lifecycle` | `research-main-20261029-1950` | open (Lane B) |
| `document-extraction-coverage-resumption` | `research-main-20261029-1018` | open (Lane B) |
| `reflection-observation-publication` | `research-main-20261029-1018` | open (Lane B) |
| `queued-slash-command-kind-conservation` | `research-main-20261029-1018` | open |
| `slash-operation-slot-admission` | `research-main-20261029-1018` | open |
| `tui-legislator-dispatch-parity` | `research-main-20261029-1018` | open |
| `interaction-audit-surface-projection-parity` | `research-main-20261029-1018` | open |
| `taxonomy-initialization-completeness` | `research-main-20261029-0736` | open |
| `queued-history-projection` | `research-main-20261030-0017` | open |
