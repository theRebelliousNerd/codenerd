# Verified traps on the pinned engine (v0.7.0)

Each line below was checked on the engine in this repo, not recalled from
memory. Re-run a lab before trusting anything not listed here, and add
every new trap you find.

## Names and functions

- Reducers are lowercase: `fn:count()`, `fn:sum(V)`, `fn:max(V)`.
  `fn:Count()` is rejected as an unknown function.
- There is no `fn:pair:fst` / `fn:pair:snd`. Destructure with
  `:match_pair(P, First, Second)`.
- Pattern built-ins: `:match_cons`, `:match_nil`, `:match_pair`,
  `:match_entry`, `:match_field`, `:match_prefix`. List and map functions:
  `fn:list:get/len/append/contains/cons`, `fn:map:get`, `fn:struct:get`.
- The output slot of a `:match_*` premise must be a free variable.
  `:match_field(E, /kind, /login)` is rejected. Bind the variable, then
  compare: `:match_field(E, /kind, K), K = /login`.

## Negation

- Negation is `!p(X)`. There is no `not`, and `!` is not a Prolog cut.
- A negated literal must be fully bound. `!r(X, _)` is an analysis error
  ("variable X0 is not bound"). Project first:
  `has_r(X) :- r(X, Y).` then `!has_r(X)`.

## Types and termination

- Decl bounds are **not checked** by `analysis.AnalyzeOneUnit`, which is
  the entry point the codeNERD kernel and `internal/mangle.Engine` use. A
  rule can put `/medium` into a `/number` column and analysis says nothing.
  Only `AnalyzeAndCheckBounds(..., ErrorForBoundsMismatch)` checks bounds.
  In codeNERD that runs only for generated candidate rules
  (`internal/core/kernel_rule_bounds.go`). The lab harness turns it on.
- Bounds don't stop bad facts in source either: `sev(/b, /medium)` loads
  under a union-of-singletons bound.
- Termination is **not** guaranteed by analysis. `n(Y) :- n(X), Y = fn:plus(X, 1).`
  analyzes fine and runs until the fact limit
  ("fact size limit reached 100001 > 100000"). Once a program has
  functions, keeping it finite is your job.

## Time and tooling

- Temporal programs analyze fine but need a temporal store at evaluation
  time. `nerd check-mangle` has none ("temporal literal encountered but no
  temporal store configured"). Use the lab harness.
- `nerd check-mangle --standalone --eval pred file.mg` is the fast probe
  for programs without time. Pass `--eval` predicate names without arity.

## Aggregation

- A `do` head variable must be a group key or an aggregate. Anything else
  is rejected ("neither part of group_by nor aggregated").
- `let` without `do` is a per-row transform: `|> let P2 = fn:list:append(P, Z).`
- The upstream aggregation tutorial has bugs: `fn:sum(TotalHours)` where it
  means `Hours`, and a stray `.` mid-rule. Upstream prose is evidence, not
  ground truth. Probe it.
