# Transforms: computing over relations, not rows

Plain Datalog can't count. Mangle adds two transforms after the rule
body: `let` (per row: compute a new column) and `do` (per group: reduce a
whole closed relation). The second is a relation boundary. It runs only
once its inputs are complete, so the engine puts it in a later layer,
just as it does for negation.

## Shapes

```mangle
# per row: extend a path
step(X, Z, P2) :- path(X, Y, P), edge(Y, Z) |> let P2 = fn:list:append(P, Z).
# per group: one fact per key
load(Shard, N, Tokens) :- turn(Shard, _, T)
  |> do fn:group_by(Shard), let N = fn:count(), let Tokens = fn:sum(T).
# whole relation into one payload for Go
risk_map(M) :- file_risk(F, R) |> do fn:group_by(), let M = fn:collect_to_map(F, R).
```

Reducers: `count`, `sum`, `min`, `max`, `avg`, `collect`,
`collect_distinct`, `collect_to_map`, `pick_any`, plus float, time and
duration variants. There is no arg-max, percentile or mode. A `do` head may
only hold group keys and aggregates.

## Patterns that use what's distinctive

- **Answer-shaped output**: end a pipeline with `collect_to_map` or
  `collect` so Go gets one fact holding the finished structure. That beats
  N queries and assembly in Go.
- **Arg-max two ways**: (a) `fn:max` per group in one layer, joined back on
  equality in the next. Ties surface as several rows, which is honest.
  (b) A merge lattice (`lattices-and-merge.md`) that keeps the winner
  during the fixpoint. Measure both.
- **Witness relations**: aggregating early destroys members. Keep
  `member_of_group(G, X)` beside `group_size(G, N)` when explanations or
  later joins need the members.
- **Counting distinct**: project to the distinct columns first (facts are
  a set), then `fn:count()`. `fn:count_distinct` is rejected at analysis.
- **Budgets and thresholds** belong in rules: `config_param` facts plus
  `do` sums give "over budget" as a derived fact Go obeys, not a Go `if`.
- **Proofs of aggregates**: in recorder mode, provenance explains a `do`
  result with its group key and transform text (`proofs-as-data.md`).

## Traps

- A list literal as a reducer argument inside `group_by` is a type error
  in v0.7. Reducing a list-valued column also fails to type-check. Unnest
  first with `:match_cons` recursion or a narrow relation.
- Float reducers are typed `/float64`. Compare their results with
  `:float:` predicates, not `<`.
- An aggregate that feeds its own input is a cycle through a relation
  boundary. Expect stratification to reject it (probe), and redesign as
  two relations or a lattice. If a count looks low, check which layer its
  inputs were finished in.
- An engine that admits transforms but never evaluates them leaves them
  decorative. Vectryx's facade had that bug. Always pin an aggregate with a
  test that queries the aggregated head.
