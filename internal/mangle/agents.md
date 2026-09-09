# Mangle Subsystem Guidance

- Read `Docs/architecture/mangle/README.md` and the Mangle skill references before changing parser, evaluator, schema, or `.mg` behavior.
- All production parsing must enter through `ParseUnit` or `ParseAtom`. Never call `mangle-go/parse.Unit` or `parse.Atom` directly outside `parse_lock.go`; the upstream ANTLR prediction state is not concurrency-safe.
- Preserve declaration-first semantics, atom/string distinctions, positive binding before negation, stratification, and bounded derivation.
- Differential evaluation must not silently drop gas, external-predicate, provenance, query, or snapshot semantics. Use a verified fallback until a mode has parity.
- Treat model-authored Mangle as untrusted: structured synth, sanitizer, analyzer, schema/protected-head validation, and constitutional ownership remain separate gates.

## A Decl is a contract, and breaking it fails silently

The most expensive defect class in this repo is a producer whose facts do not
match the shape its consumers join on. Both sides are internally consistent, so
nothing errors, no test goes red, and the rules simply derive nothing — forever.
Three instances were found in one audit (2026-09):

| Predicate | Consumers wanted | Producer wrote | Cost |
|---|---|---|---|
| `modified_function` | `<pkg>.<Name>`, joined to `code_calls` | *nothing at all* | the whole caller-impact chain, and the impact-ranked holographic context |
| `modified_function` (after wiring) | `<pkg>.<Name>` | bare `Name` | same chain, still empty |
| `plan_edit` | a `code_element` ref | `FileEdit.FilePath` | five rules in `test_impact.mg`; both impacted-test tools returning "none" on every call |

Before adding or changing a producer:

1. **Read every consumer.** `grep` the predicate across `*.mg`. What binds its
   variables? If a rule does `p(X), code_element(X, _, _, _, _)`, then `X` is a
   ref — `fn:<pkg>.<Name>` — not a path and not a bare name.
2. **Match the existing identifier shape exactly.** `world.Cartographer` keys
   `code_calls` and `code_defines` by `<pkg>.<Name>`, and `<pkg>.<Recv>.<Name>`
   for a method. `world.GoCodeParser.buildRef` emits the same string with a kind
   prefix. Getting this wrong is invisible: every fact is well-formed and the
   join is empty.
3. **If the producer does not know the shape the Decl asks for, do not guess.**
   Emit into a predicate whose contract it can meet, or write a producer where
   the identity is actually known. `TransactionManager` only has file paths, so
   it emits `modified_file`; `plan_edit` belongs where the element ref is known.
4. **Prove it end to end.** A unit test on either side passes while the join is
   empty, because each side is self-consistent. Assert the *derived* predicate
   from a real kernel — see `TestImpactChain_EndToEndThroughVirtualStore`.

`TestStarvedPredicateBudget` (`internal/core/defaults`) catches the "no producer
at all" case and fails when the count moves in either direction. It cannot catch
a producer of the wrong shape. That one is on the reader.

Focused verification:

```powershell
go test ./internal/mangle/... ./internal/core -count=1
go test -race ./internal/mangle -run '^Test(CodeUsesSerializedMangleParser|ProductionParserCallersShareSerializedEntryPoint)$' -count=5
go test -race ./internal/core -run 'Concurrent|DreamerSingleton|DreamRouterSingleton' -count=1
go vet ./internal/mangle/... ./internal/core
```
