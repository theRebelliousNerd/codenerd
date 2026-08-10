# Session contributor guidance

- Preserve the control split: the model proposes; the effective capability
  envelope narrows availability; exact Mangle `permitted(Action,Target,Payload)`
  authorizes; VirtualStore validates and executes.
- Nil or empty runtime config/AllowedTools means no tools. Tool registration,
  Ouroboros generation, prompt text, and `safe_action/1` never grant capability or
  permission.
- Canonicalize and bound the exact payload before asserting `pending_action/5`.
  Reject empty names, oversize payloads, wrong arity, mismatched targets/payloads,
  stale facts, and missing kernel/gates.
- Treat `nerd.md` write protection as fail closed: recognized write tools need a
  target and a live kernel authority before any executor, VirtualStore, or
  registry path may mutate.
- Specialist YAML must remain path-contained, size-bounded, strictly decoded, and
  validated before registration or spawn. Failed config leaves no agent behind.
- Keep native and Piggyback tool paths on one capability, permission, timeout,
  cancellation, accounting, post-edit build/test/critic, idempotency, and
  result-bound contract. Piggyback has no native repair round.
- Preserve the deadline conclusion window and tool-use/result pairing; never
  replay a side effect while transitioning from exploration to forced final.
  Every terminal path must still reach the shared post-edit proof gate.
- Adaptive tool-budget extensions must remain deterministic and bounded by the
  hard tool-call/time ceilings. Grant only on novel successful tool evidence;
  repeated full call+argument+result cycles deny extension. Keep the live
  calls/rounds nudge short and attach it to an already-paired tool result.
- New LLM-facing behavior is a prompt atom first. `AvailableTools` describes the
  effective envelope; it is not authority.
- Maintain explicit ownership for executor history, Spawner reservations, SubAgent
  state, coherent config snapshots, shared kernel use, persistence, and teardown.
  Add race tests for changes.
- Run session/JIT config tests, focused `-race`, relevant integration tests, and
  reconcile `Docs/architecture/session/` when contracts or wiring change.
