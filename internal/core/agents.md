# Core Runtime Guidance

- Read `README.md`, `../mangle/agents.md`, and `../session/README.md` before changing kernel, schema, VirtualStore, or execution behavior.
- Every emitted or asserted fact must match its `Decl` at the source boundary. Mangle `/number` values are `int64`; use `types.PercentFromRatio` for 0..1 ratios and `types.PercentClamp` for existing 0..100 values. Use `MangleAtom` for `/name` slots.
- Virtual-predicate atoms bypass the kernel's Decl-directed fact coercion. Test their AST constant types directly and, for hydrated facts, assert them into a real `RealKernel`.
- Per-shard trace statistics must come from one exact shard-filtered aggregate. Do not reconstruct them from global averages, top-N maps, success rates, or minimum-sample reports.
- `HydrateSessionContext` requires `types.KernelTransactor`. Query failures commit the fresh partial snapshot to clear stale context and return a warning with the committed count; commit failures return count zero.
- Keep dynamic context replacement atomic and fail closed at constitutional action boundaries.

Focused verification:

```powershell
go test ./internal/store ./internal/core -count=1
go test -race ./internal/store ./internal/core -count=1
go vet ./internal/store ./internal/core
```
