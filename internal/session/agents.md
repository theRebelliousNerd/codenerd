# Session contributor guidance

- Preserve the control split: the model proposes; the effective capability
  envelope narrows availability; exact Mangle `permitted(Action,Target,Payload)`
  authorizes; VirtualStore validates and executes.
- Nil or empty runtime config/AllowedTools means no tools. Tool registration,
  Ouroboros generation, prompt text, and `safe_action/1` never grant capability or
  permission.
- Canonicalize and bound the exact payload before asserting `pending_action/3`.
  Reject empty names, oversize payloads, wrong arity, mismatched targets/payloads,
  stale facts, and missing kernel/gates.
- Specialist YAML must remain path-contained, size-bounded, strictly decoded, and
  validated before registration or spawn. Failed config leaves no agent behind.
- Keep native and Piggyback tool paths on one capability, permission, timeout,
  cancellation, idempotency, and result-bound contract.
- New LLM-facing behavior is a prompt atom first. `AvailableTools` describes the
  effective envelope; it is not authority.
- Maintain explicit ownership for executor history, Spawner reservations, SubAgent
  state, shared kernel use, persistence, and teardown. Add race tests for changes.
- Run session/JIT config tests, focused `-race`, relevant integration tests, and
  reconcile `Docs/architecture/session/` when contracts or wiring change.
