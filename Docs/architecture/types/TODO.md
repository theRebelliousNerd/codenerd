# TODO — `internal/types`

> Last verified: **2026-07-13**  
> Docs-only corpus rebuild; items below are **package evolution backlog**, not incomplete documentation.

## P0 — Safety / correctness

- [ ] Audit hot assert paths for remaining bare `Args[i].(T)` and `%v` dumps; migrate to `Extract*` / typed construction
- [ ] Ensure all production kernels used with multi-op updates implement `KernelTransactor` (mocks too)

## P1 — API consolidation

- [ ] Plan deprecation path: `KernelInterface` / `KernelFact` → full `Kernel` + adapters only at edges
- [ ] Decide: typed context keys for spawn priority / model capability (match session key pattern)
- [ ] Add container (`map`/`slice`) ToAtom table tests

## P2 — Hygiene

- [ ] Consider nested sub-structs if `SessionContext` gains more sections (keep field groups navigable)
- [ ] Optional test helper: `MockKernel` implementing `Kernel` + `KernelTransactor` for shared unit tests (only if it does not create cycles — may belong in `internal/testing`)
- [ ] Document VirtualStore expansion policy in code comment when next method is added

## P3 — DX

- [ ] Optional package-level godoc examples for `ToAtom` and `NewKernelTx`
- [ ] When dual Kernel APIs collapse, delete obsolete aliases after one release cycle

## Done (recent, evidenced in code)

- [x] Remove silent ToAtom stringification of unknown/pointer types
- [x] Remove non-atomic NewKernelTx fallback (panic)
- [x] Extract* helpers for fact args
- [x] Optional LLM capability interfaces (grounding, thinking, piggyback, …)
- [x] GraphQuery / VirtualStore cycle-break markers
