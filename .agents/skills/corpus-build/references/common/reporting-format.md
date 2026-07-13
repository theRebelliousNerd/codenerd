# Fleet Completion-Record Contract

Every corpus-build fleet agent (builder, critic, comms-plumber,
defense-auditor, consumables-keeper, doc-auditor, wiring-auditor,
jules-dispatcher) ends its work unit with a structured completion record in
this shape. The orchestrator and downstream agents (critic, doc-auditor)
consume these fields programmatically -- freeform prose in their place
breaks the pipeline.

## Required sections

1. **Files touched** -- every file created, edited, or deleted, as
   repo-relative paths. Group by kind (source, test, config, docs) if the
   list is long. This is the input to the write-scope-guard audit and to
   `corpus-doc-auditor`'s IMPLEMENTED_SPEC reconcile.
2. **Gate evidence** -- the actual command run and its verdict, not a claim.
   "go build ./internal/foo/... -- PASS" with the real output attached or
   referenced (`.corpus-build/results/<WU-id>.json`), not "should compile."
   Per `.claude/rules/agent-behavior.md`: "If you didn't build it, test it,
   and run it -- you didn't ship it." A work unit is DONE only with gate
   evidence recorded, never on a claim alone.
3. **Interface assumptions** -- every symbol, type, endpoint, or contract you
   used that came from a spec doc or a peer's completion record rather than
   a grep you personally ran (see `anti-hallucination-gate.md`). Flag
   anything you marked `// UNVERIFIED` here explicitly, with the reason.
4. **Skips, with reasons** -- every phase, surface, or checklist row you
   intentionally did NOT do, and why. Never a silent pass. Route through
   `scripts/record_skip.py` -> `.corpus-build/skips.jsonl` when that script
   is available in your work unit's context; otherwise state it plainly in
   this section so the orchestrator can decide whether to dispatch a
   follow-up.

## Style rules

- Record what you verified, not what you intended. "Wrote the handler" is
  not evidence; "wrote the handler, `go vet` clean, `go test
  ./internal/api/rest/handlers/... -run TestFoo` PASS" is.
- Never claim a capability is wired end-to-end unless you can point to the
  concrete file:line for each hop (route -> handler -> bind-struct ->
  OpenAPI contract, or predicate -> dispatch -> Mangle rule, etc.).
- Keep it terse and structured -- this artifact is consumed by other agents
  and by the orchestrator's serial gate, not primarily by a human reader.
- If you deviated from your assigned work-unit slice (e.g. touched a file
  outside your `.corpus-build/slices/<WU-id>.json` manifest), call that out
  explicitly with justification rather than letting it surface as a
  write-scope-guard violation downstream.
