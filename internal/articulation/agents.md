# Articulation contributor guidance

- Preserve the dual channel: surface text is for the user; control data is typed
  input to downstream validation. Neither a parsed update nor a tool request is
  authorization.
- Every control consumer must validate the exact action, target, and payload
  through the constitutional gate before assertion or execution. Default deny on
  unknown fields, operations, declarations, arity, provenance, or targets.
- Keep parsing and streaming bounded. Depth, candidate, surface, reasoning, list,
  and per-item ceilings must retain hostile-input tests and valid UTF-8 output.
- Configure `ResponseProcessor` before concurrent use. Statistics are race-safe;
  mutable parser options are not a live reconfiguration API.
- `StreamParser` is stateful and currently single-owner. Do not share one instance
  between goroutines without first adding an explicit synchronized/bounded contract.
- New model-facing protocol instructions are prompt atoms first. Keep Go types,
  schema, prompt fragments, provider adapters, and consumer tests in parity.
- On tolerant fallback, return surface only. Never salvage or execute control from
  malformed text without the full typed validation path.
- Run package tests, focused `-race`, and relevant session/chat consumer gates;
  reconcile `Docs/architecture/articulation/` after contract or wiring changes.
