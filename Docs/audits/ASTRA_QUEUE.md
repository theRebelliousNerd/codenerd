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
| `exact-model-pin-tag-identity` | `research-main-20261029-1018` | open |
| `streaming-grounding-source-ownership` | `research-main-20261029-1018` | open |
| `ollama-shard-model-override` | `research-main-20261029-2152` | open |

## Lane A: experts, JIT, then session verdicts and test gates

| audit | branch | status |
|---|---|---|
| `expert-selection-temporary-fact-ownership` | `research-main-20261029-2049` | open |
| `specialist-roster-workspace-identity` | `research-main-20261029-1018` | open |
| `knowledge-consultation-literal-identity` | `research-main-20261029-1018` | open |
| `jit-co-use-observation-budget` | `research-main-20261029-1018` | open |
| `jit-learning-semantic-identity-admission` | `research-main-20261029-1018` | open |
| `jit-relevance-carry-work-budget` | `research-main-20261029-1018` | open |
| `retry-jit-callability-continuity` | `research-main-20261029-1018` | open |
| `baseline-exemption-tagged-test-obligations` | `research-main-20261029-1018` | open |
| `explicit-package-selector-fidelity` | `research-main-20261029-0920` | open |
| `net-change-test-plan-applicability` | `research-main-20261029-0946` | open |
| `tagged-test-constraint-witness` | `research-main-20261029-1018` | open |
| `test-gate-memoization-identity` | `research-main-20261029-0850` | open |
| `test-removal-subject-identity` | `research-main-20261029-0850` | open |
| `tester-continuation-proposal-admission` | `research-main-20261029-1018` | open |
| `review-feedback-identity-and-current-verdicts` | `research-main-20261029-1018` | open |
| `revision-sentinel-build-exclusion-parity` | `research-main-20261029-1018` | open |
| `omitted-argument-admission-order` | `research-main-20261029-1018` | open |
| `system-execution-outcome-projection` | `research-main-20261029-1018` | open |
| `expert-lending-dependency-contract` | `research-main-20261029-2152` | open |
| `slash-command-specialist-target-continuity` | `research-main-20261029-2221` | open |

## Lane B: campaign and verification, then the Mangle kernel and world model

| audit | branch | status |
|---|---|---|
| `campaign-terminal-publication` | `research-main-20261029-2049` | open |
| `campaign-ui-run-ownership` | `research-main-20261029-1950` | open |
| `campaign-context-seed-scheduler-ownership` | `research-main-20261029-1018` | open |
| `recurse-compensation-write-ownership` | `research-main-20261029-1018` | open |
| `residual-tool-generation-restriction-admission` | `research-main-20261029-1018` | open |
| `verification-cancellation-after-deadline` | `research-main-20261029-0736` | open |
| `verification-process-cleanup-parity` | `research-main-20261029-0736` | open |
| `mangle-content-fingerprint-typed-identity` | `research-main-20261029-1018` | open |
| `mangle-generic-merge-invalidation` | `research-main-20261029-0920` | open |
| `mangle-inclusion-projection-closure` | `research-main-20261029-0850` | open |
| `mangle-overlay-raw-store-ownership` | `research-main-20261029-0946` | open |
| `projection-cache-allocation-acceptance` | `research-main-20261029-1018` | open |
| `why-query-scope-preservation` | `research-main-20261029-1018` | open |
| `world-cache-rolling-format-admission` | `research-main-20261029-1018` | open |
| `scanner-graph-source-retirement` | `research-main-20261029-1018` | open |
| `symbol-focus-identity-continuity` | `research-main-20261029-1018` | open |
| `run_lint describes its build tags` (root finding, R1-27 rerun: the model believed run_lint takes no tags; it applies the workspace build env) | -- | open |
| `gate-comparison-domain-admission` | `research-main-20261029-2152` | open |
| `gate-scoped-recurse-current-main-addendum` | `research-main-20261029-2221` | open |

## codeNERD ladder runs: store, learning and memory, then chat and TUI

One audit per run, briefed as its symptom (the ladder's brief rule), in this order.

| audit | branch | status |
|---|---|---|
| `knn-recall-limit-error-contract` | `research-main-20261029-1018` | open |
| `legacy-vector-repair-attempt-lifecycle` | `research-main-20261029-1018` | open |
| `knowledge-index-repair-cache-freshness` | `research-main-20261029-1018` | open |
| `embedding-stats-observation-boundary` | `research-main-20261029-1018` | open |
| `learned-example-literal-fidelity` | `research-main-20261029-0736` | open |
| `learning-lexical-candidate-window` | `research-main-20261029-1018` | open |
| `manual-learning-evidence-conservation` | `research-main-20261029-1018` | open |
| `project-knowledge-lexical-term-budget` | `research-main-20261029-1018` | open |
| `user-memory-archival-lifecycle` | `research-main-20261029-1950` | open |
| `document-extraction-coverage-resumption` | `research-main-20261029-1018` | open |
| `reflection-observation-publication` | `research-main-20261029-1018` | open |
| `queued-slash-command-kind-conservation` | `research-main-20261029-1018` | open |
| `slash-operation-slot-admission` | `research-main-20261029-1018` | open |
| `tui-legislator-dispatch-parity` | `research-main-20261029-1018` | open |
| `interaction-audit-surface-projection-parity` | `research-main-20261029-1018` | open |
| `taxonomy-initialization-completeness` | `research-main-20261029-0736` | open |
