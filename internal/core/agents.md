# Core Runtime Guidance

- Read `README.md`, `../mangle/agents.md`, and `../session/README.md` before changing kernel, schema, VirtualStore, or execution behavior.
- Every emitted or asserted fact must match its `Decl` at the source boundary. Mangle `/number` values are `int64`; use `types.PercentFromRatio` for 0..1 ratios and `types.PercentClamp` for existing 0..100 values. Use `MangleAtom` for `/name` slots.
- Virtual-predicate atoms bypass the kernel's Decl-directed fact coercion. Test their AST constant types directly and, for hydrated facts, assert them into a real `RealKernel`.
- Per-shard trace statistics must come from one exact shard-filtered aggregate. Do not reconstruct them from global averages, top-N maps, success rates, or minimum-sample reports.
- `HydrateSessionContext` requires `types.KernelTransactor`. Query failures commit the fresh partial snapshot to clear stale context and return a warning with the committed count; commit failures return count zero.
- Keep dynamic context replacement atomic and fail closed at constitutional action boundaries.
- Dreamer evaluates each exact request against current state. Do not restore authorization caching without a complete policy, fact, payload, and external-state identity; cancellation precedes evaluation.
- Every executable tool declares an effect in `../tools/effects.go` or its registered `Tool`. Unknown effects and absent mandatory executive gates fail closed, including registry entry paths.
- `turn_executed` means an action passed mechanical hollow-success guards. Only host-issued, current `turn_acceptance` evidence permits `turn_done`; control packets cannot assert either conclusion or its witnesses.
- Keep large-world delta routing bounded before evaluation (`differentialFactCeiling`), expose `LastEvaluation`, and check semantic equivalence against full fixpoint evaluation.

Focused verification:

```powershell
go test ./internal/store ./internal/core -count=1
go test -race ./internal/store ./internal/core -count=1
go vet ./internal/store ./internal/core
```

- Build/test action handlers must preserve the host OS and toolchain. Windows `bash` can launch WSL; use the native verification shell and assert host identity plus nonzero exit propagation in live tests.

- Large full-fixpoint evaluations use argument-indexed fact storage. Preserve complete production-corpus closure and retraction equivalence to the scan store; report lookup speed together with allocation and real persisted-state profiles. Indexing must not alter policy, provenance, external callbacks, or derivation limits.
