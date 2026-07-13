# TODO — mangle package

> Prioritized backlog. No calendar estimates — dependency order only.  
> Last updated: 2026-07-13

## P0 — Correctness

- [ ] Route `transpiler.Sanitizer` parse through `mangle.ParseUnit` (not raw `parse.Unit`).
- [ ] Route `synth.Compile` / any remaining parse sites through `ParseUnit`.
- [ ] Add `-race` regression test covering concurrent Engine schema load + Sanitize.

## P1 — Differential eval completeness

- [ ] Extend DifferentialEngine eval APIs to accept / forward `engine.EvalOption` (externals, created-fact limit, derivation recorder).
- [ ] Kernel: re-enable more cases on diff path once options land; keep fallbacks.
- [ ] Document + test unified vs legacy path parity for Snapshot/Query when unified enabled.
- [ ] Investigate true delta propagation (engine-level) after option forwarding.

## P2 — Generation stack

- [ ] Default autopoiesis/legislator/mangle_repair to `SynthModePrefer` or `Require`.
- [ ] Optional VirtualStore tool `mangle_synth_tool` per `synth/README.md`.
- [ ] SchemaValidator: prefer ProgramInfo Decl map when available over regex.

## P3 — Observability & corpus

- [ ] Unify ProofTreeTracer with provenance recorder for one glass-box story.
- [ ] Export structured metric: eval_path=diff|full, derived_count, feedback_attempts.
- [ ] Wiring audit: `intent_routing.mg` load into kernel program.
- [ ] Wiring audit: `IntersectSIMD` production call sites.

## P4 — Docs / hygiene

- [ ] Remove or redirect legacy differently-named architecture stubs if still present beside this set (`01-DOMAIN-MODEL.md`, `02-CURRENT-STATE-MANGLE.md`, etc.) after consumers migrate.
- [ ] Keep IMPLEMENTED_SPEC updated when eval contracts change.

## Done (context)

- Process-wide parse lock + core delegation.
- Forbidden learned heads map.
- Unified fast path for kernel deltas.
- FeedbackLoop budgets and progressive prompts.
- Large validation suite against real policy corpus.
