# Mangle Subsystem Guidance

- Read `Docs/architecture/mangle/README.md` and the Mangle skill references before changing parser, evaluator, schema, or `.mg` behavior.
- All production parsing must enter through `ParseUnit` or `ParseAtom`. Never call `mangle-go/parse.Unit` or `parse.Atom` directly outside `parse_lock.go`; the upstream ANTLR prediction state is not concurrency-safe.
- Preserve declaration-first semantics, atom/string distinctions, positive binding before negation, stratification, and bounded derivation.
- Differential evaluation must not silently drop gas, external-predicate, provenance, query, or snapshot semantics. Use a verified fallback until a mode has parity.
- Treat model-authored Mangle as untrusted: structured synth, sanitizer, analyzer, schema/protected-head validation, and constitutional ownership remain separate gates.

Focused verification:

```powershell
go test ./internal/mangle/... ./internal/core -count=1
go test -race ./internal/mangle -run '^Test(CodeUsesSerializedMangleParser|ProductionParserCallersShareSerializedEntryPoint)$' -count=5
go test -race ./internal/core -run 'Concurrent|DreamerSingleton|DreamRouterSingleton' -count=1
go vet ./internal/mangle/... ./internal/core
```
