# Types as contracts the engine can check

Plain Datalog has no types. Mangle Decls carry type expressions: names,
numbers, strings, structs with optional fields, unions, single-value types,
tagged unions and type variables. The checker can prove a rule respects
them, but in codeNERD that checker mostly isn't running.

## The state of things (verified)

- The kernel analyzes with `AnalyzeOneUnit`, which defaults to
  `NoBoundsChecking`. Bounds are parsed and then ignored. A rule put
  `/medium` into a `/number` column without complaint.
- Generated candidate rules get `ErrorForBoundsMismatch` in
  `internal/core/kernel_rule_bounds.go`. The comment there explains why the
  whole corpus doesn't: the analyzer stops at the first mismatch, and the
  corpus has disagreements of its own (`activation/2` is declared over
  strings and derived over tool names).
- The lab harness checks bounds strictly by default; `LooseBounds()` matches
  the kernel.

## Moving the ball

- **Corpus census**: one lab that analyzes the full embedded program
  (schemas + policy) with `ErrorForBoundsMismatch` and fixes mismatches one
  at a time until it passes. Each mismatch is either a wrong Decl or a
  silent bug. Ending with the kernel checking strictly is the prize.
- **Enums that mean it**: `fn:Union(fn:Singleton(/low), fn:Singleton(/high))`
  in place of `/name` for closed vocabularies (severity, phase, verdict).
  With checking on, a typo'd `/hihg` is a compile error instead of a rule
  that silently never fires.
- **Tagged unions for variants**: tool calls, shard results and
  verification outcomes as one Decl with variants (see
  `structured-values.md`). The type shows which fields exist per variant.
- **Optional fields**: `opt /reason : /string` models "maybe present"
  without a second predicate or a sentinel value.
- **Type variables** for generic helper relations, such as a reusable
  `member_of(X, L)` over lists of any element type. Probe how far the
  checker follows them.

## Contracts at the Go boundary

- Go producers build values from the Decl. `MangleAtom` is for names; plain
  strings are for paths and prose. A typed Decl makes the wrong choice
  visible, but only when checking is on.
- Generate Go types from Decls, or Decls from Go types, so the two can't
  drift. `json2struct` and `proto2struct` show the engine side of that
  mapping. That's an experiment candidate.
- `descr [doc(...), arg(X, "...")]` puts the meaning next to the type, and
  tools can read both.

## Limits to respect

- Bounds are upper bounds. They don't make facts in source illegal (a
  `/medium` fact under a singleton union loaded fine), and they don't prove
  that a predicate is non-empty.
- Multiple `bound [...]` lists are alternatives, not intersections.
- `/any` is a wildcard the checker admits everywhere. Use it only where the
  column really is heterogeneous.
