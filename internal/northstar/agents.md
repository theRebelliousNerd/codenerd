# North-star derivation

- `derive*.go` owns non-interactive classification, vision drafting and requirement generation; the TUI only adapts its inputs and messages.
- Derivation prompts come from `northstar/derive/*` JIT atoms. Complete user requests include all framing and carry-forward summary bytes under `orient.derive_request_bytes`.
- Phase adapters and their private scopes expose the owned kernel's complete
  QueryAll snapshot for cache identity. Detach result containers and refuse nil
  or closed scopes; do not bypass snapshot requirements to compile a phase.
- Validate evidence against the full document and provenance against supplied paths. Preserve draft markers and report rejected claims, omissions and pruned links.
- Drafting is pure. Installation preserves existing authority and writes a separate derived candidate; init/report/policy wiring belongs to the orientation lane.
