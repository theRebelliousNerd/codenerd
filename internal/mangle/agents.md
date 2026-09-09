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

## A quoted string starting with `/` does not match a stored string

Measured against a booted kernel, all four rules below run against one stored
fact whose third argument is the Go string `"/etc/passwd"`:

```
projected_fact(A, /modified, "/etc/passwd")  ->  0 rows
projected_fact(A, /modified, "etc/passwd")   ->  1 row
projected_fact(A, /modified, "passwd")       ->  1 row
projected_fact(A, /modified, _)              ->  1 row
```

The slot is declared `/string`. The leading slash makes the literal read as
something other than that string, and the join goes quietly empty — no parse
error, no type error, no warning. It is the Decl-contract trap above wearing a
different hat: both halves internally consistent, nothing to fail.

Absolute paths are everywhere in this system — targets, workspace roots,
protected paths — so this is easy to hit and hard to see. It cost an entire
e2e file: a Dreamer policy rule that was supposed to block writes to
`/etc/passwd` matched nothing, and the tests around it reported "expected error,
got nil" as though the safety gate were broken.

**Write the join on a variable, and put the path in a fact.**

```
Decl protected_path(Path) bound [/string].

panic_state(A, "writes_protected_path") :-
    projected_fact(A, /modified, Path),
    protected_path(Path).
```

A variable binds whatever is stored, so the comparison is between values rather
than between a value and however a literal happens to lex. Seed
`protected_path("/etc/passwd")` as a fact from Go, where it is an ordinary
string.

## `HotLoadRule` validates; it does not load

`RealKernel.HotLoadRule` compiles a candidate against a sandbox kernel and
returns whether it is acceptable. It does not install it, and until 2026-09 its
doc comment said the opposite ("dynamically loads a single Mangle rule at
runtime... used by Autopoiesis to add new rules without restarting" — that is
`HotLoadLearnedRule`, immediately below it).

Both production callers are correct and use it as a validator:
`feedback/loop.go` as "Phase 3: Sandbox compilation",
`shards/system/mangle_repair.go` as "Phase 1: Syntax check via kernel".

- To make the kernel evaluate a rule now: `AppendPolicy`.
- To make it evaluate the rule and survive a restart: `HotLoadLearnedRule`,
  which persists to `learned.mg`.
- To ask only whether a rule compiles: `HotLoadRule`.

A test that hot-loaded a rule, got `nil`, and asserted on the derivation it
expected is how this surfaced. Every signal the caller had said the rule was in.
