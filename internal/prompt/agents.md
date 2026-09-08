# Prompt subsystem guidance

- JIT atoms are the source of truth for new model-facing behavior; do not add a parallel hardcoded prompt path.
- Compiler shutdown must stop admission, cancel active compilation, join it, and
  close owned databases. Do not race WaitGroup admission against shutdown or
  replace joined cleanup with a timer that abandons live database handles.
- `atom_schema.go#AtomDefinition` and `ParsePromptAtomYAML` own the YAML contract for filesystem loading, embedding, synchronization, and validation.
- Canonical agent selectors use `shard_types`. `agent_types`, legacy metadata, and nested `selectors` exist only as bounded, observable compatibility migrations scheduled for removal on 2027-01-01.
- Built-in atoms under `atoms/` must parse without migrations. Unknown fields and invalid records fail the complete document; never log-and-skip a bad atom.
- Closed selector vocabulary comes from live typed context definitions (`AllContextDimensions`), not a validator-local list.
- Keep runtime selector values normalized without `/`; YAML authoring uses `/` for non-world-state selectors.
- After atom/schema changes run:
  - `go run ./cmd/tools/validate_prompt_atoms -root internal/prompt/atoms -fail-on-warn`
  - `go test -count=1 ./internal/prompt ./internal/prompt/sync ./cmd/tools/validate_prompt_atoms`
  - `go test -race -count=1 ./internal/prompt ./internal/prompt/sync`
- Browser observe/act guidance lives in
  `atoms/capability/browser_progressive.yaml`; keep it ref-first, bounded, and
  free of raw selector or arbitrary-JavaScript instructions.
- Browser wait/reason/read-only-Mangle guidance lives in
  `atoms/capability/browser_reasoning.yaml`; keep waits fresh by default and do
  not teach rule/fact mutation through the browser surface.
- Browser flight-recorder guidance lives in
  `atoms/capability/browser_evidence.yaml`; preserve provenance, truncation,
  redaction, owner-only persistence, and writable-root confinement.
- Browser spec guidance lives in `atoms/capability/browser_specs.yaml`; keep
  delivery workspace-confined and invariant checks read-only against the live
  Cortex browser allowlist.
- Browser declarative-test guidance lives in
  `atoms/capability/browser_tests.yaml`; keep fixtures semantic-targeted,
  credential-free, bounded, and limited to live-kernel single-atom assertions.
- Grounded web search guidance lives at
  `atoms/capability/grounded_web_search.yaml`; select it only when
  `grounded_web_search` is in the effective JIT catalog, never for ordinary
  `/test`, `/benchmark`, or `/profile` intents.

- `ReloadAllPrompts` aggregates discovered-agent failures for its caller; a partial atom count must not hide a missing or invalid expert database.
- Knowledge retrieval combines project knowledge with only the selected expert's
  registered database. Keep queries bounded and cancellable, preserve source and
  confidence in compiled context, and never close a borrowed handle. Test both
  missing-source controls and actual compiled inclusion; a stored row alone is
  not evidence that an expert consumed it.
- Ephemeral lookup results emit `retrieved_context` for Mangle selection. Keep
  that witness distinct from vector similarity and mandatory atoms; context,
  conflict, dependency, and token-budget rules still apply.
