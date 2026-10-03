# Perception contributor guidance

- Bootstrap uses the WithContext constructors and strict LoadFromKernelWithContext
  path. Hydration caps inherit the parent; only an alive parent permits optional
  degradation. The legacy LoadFromKernel retains its partial-cache flush contract.
- Constructors own engines and stores until successful publication. On parent
  cancellation close those handles and retain cleanup errors; shared admission
  observes cancellation and must not publish a failed classifier.
- Learned-corpus initialization uses the context-aware store constructor itself,
  not just cancellation checks around an uncancelable database constructor.

- Preserve the boundary: models propose `Understanding`; Go normalizes it; Mangle
  and session policy remain the authority for `next_action` and `permitted/3`.
- Treat `Intent.ToFact` and routing-fact writers as hostile-input boundaries.
  Keep argument sanitization, length limits, and declaration/arity parity tested.
- `NewClientFromConfig` requires a non-nil config and must never silently switch
  providers after explicit configuration. Optional classification/worker clients
  may still return `(nil, nil)` where their API documents fallback.
- Meta `reasoning_effort` is an explicit provider contract. A configured value
  must survive root, classification, worker, and planner factories, override
  every capability hint (including unhinted calls), and never leak into
  non-Meta request JSON. Preserve per-slot endpoint overrides while wiring it.
- Preserve typed cancellation, authentication, rate-limit, and transient outage
  identity through retry wrapping. Never log secrets or raw credential payloads.
- Process globals (`SharedTaxonomy`, `SharedSemanticClassifier`) are lifecycle and
  isolation risks. New state should have explicit workspace owner, teardown, and
  race tests rather than adding more globals.
- Prompt changes are JIT atom changes first. Perception capability descriptions
  inform prompt selection but never authorize tool execution.
- Before handoff run focused tests, `go test ./internal/perception/...`, and race
  tests for shared corpus/taxonomy/tracing changes.
- Reconcile `Docs/architecture/perception/` after changing public contracts,
  fact flow, provider behavior, lifecycle, or feature-card status.
- Provider failures have one contract (`provider_failure.go`): classify with
  `ClassifyProviderFailure`, surface with `SafeProviderFailureMessage`, never the
  raw provider body. A new adapter joins `TestProviderFailureConformance`, and
  its retry loop reads `llmMaxRetries`/`llmRetryBackoff`, not a constant.
- The routing vocabulary check is the kernel's (`understanding_vocab_miss` in
  `perception_routing.mg`); do not reintroduce a Go validator.
- Meta grounding (2026-09-29): with `meta.enable_web_search` on, every Meta request goes to
  `/responses` with `web_search` (Chat Completions cannot carry it). The answer is the
  non-commentary message items only (`metaAnswerText`); citations and retrieved pages are both
  kept. Grounding control is vendor-neutral (`SetEnableWebSearch`). Spec:
  `Docs/architecture/perception/13-META-SEARCH-GROUNDING.md`.
