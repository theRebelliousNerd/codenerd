# corpus-realize outer loop

`corpus-realize` is the architecture-to-plan half of corpus-build. It determines what current codeNERD should build; the worker fleet determines how.

## Loop

```text
vision anchor
  → current-tree/concurrency audit
  → corpus requirement extraction
  → live code/wiring/test audit
  → BUILD / EVOLVE / PIVOT judgment
  → pinned contracts and DAG
  → corpus-build fleet
  → verification and corpus reconciliation
```

## Judgment

- **BUILD** — the corpus contract is still valid and implementation is missing.
- **EVOLVE** — implementation exists but needs bounded structural change toward the contract.
- **PIVOT** — the proposed mechanism conflicts with live invariants or a better existing path satisfies the intent.
- **DOC-ONLY** — code is ahead of prose; route only to the doc-auditor.
- **BLOCKED** — evidence or authority needed to choose safely is unavailable.

Percentage-complete estimates are optional and must derive from explicit requirement rows, never narrative impression.

## Preflight against concurrent work

For every candidate write path, inspect current status, recent commits, symbols, and newly-created files. Treat handoffs and corpus status as advisory until verified against the live tree. Shared-checkout workers use exact disjoint paths; contested registries are serialized through intent files.

## Relationship to other skills

- `arch-propose` authors a new pre-implementation corpus upstream.
- `corpus-build` realizes an accepted current corpus.
- `integration-auditor` traces behavior that exists but may not run.
- `nerd-evolve` optimizes a working measured system after realization; it is not a substitute for missing implementation.
- `spec-doc-sprint` documents an existing subsystem rather than proposing one.

No historical model prices, duration guesses, Vectryx service surfaces, or nonexistent remediation runtime are part of this loop.
