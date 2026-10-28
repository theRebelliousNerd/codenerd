# Relational thinking and proof-shaped debugging

## Translate intent, not procedure

Don't turn an imperative procedure into clauses line by line. Turn it into
relations and invariants:

| Imperative instinct | Mangle formulation |
| --- | --- |
| loop over rows | body atoms joined on shared variables |
| `if x then y` | a rule with head `y` whose body proves `x` |
| mutable counter | a `do` transform over a closed relation |
| "the best one" | a merge lattice, or `fn:max` joined back on equality |
| early return | a more specific rule, often in a later stratum |
| absence check | stratified negation of a fully bound projection |
| object payload | a struct or tagged union, or an ID plus narrow relations |
| "is it still valid" | a temporal fact, not a timestamp plus a Go clock check |
| call an API mid-decision | an external predicate |
| "explain why" | the recorded proof, not prose |

Before writing a clause, answer four questions:
1. What ground facts will Go measure and assert, and in what exact shapes?
2. What derived relation states the judgment?
3. Which Decl makes every argument's type unambiguous?
4. Who queries the result, and which test sees a true case and a near miss?

A missing answer means you have a fragment, not a feature.

## Names are identity, strings are content

`/review` is a name. `"review"` and `"/review"` are strings. `7`, `7.0`
and `"7"` are three different values. At the Go boundary, use
`MangleAtom` for names and plain strings for paths and prose. Never
normalize by trimming slashes or comparing rendered forms.

## Rules are proofs, not branches

A rule fires for every substitution that satisfies its whole body. Body
order isn't control flow. The engine postpones `=`, `!=` and negation
until their variables are bound, but `<` and `:float:lt` need their
variables bound earlier.

Negation means "not derivable from a closed lower stratum", not "false
in the world". If a fact can arrive later in the same evaluation, the
negated consumer belongs in a later stratum or a later kernel transaction.
Never paper over an unstratifiable cycle by evaluating twice in Go;
redesign the relations.

## Aggregate late, keep witnesses

Aggregating early destroys the rows downstream rules need. Keep a witness
relation when explanations or later joins need the members of a group.

## Debugging order

1. Reduce the problem to its Decls, rules and a hand-written EDB with one
   expected derivation and one near miss. A lab is the right place.
2. Check value types at the Go boundary: name versus string, int versus
   float.
3. Name the phase that failed: parse, analyze (bounds, modes, safety),
   stratify or evaluate.
4. Query each body relation on its own. Don't stare at the head.
5. If the head is missing, find the first premise or built-in with no
   match under the intended substitution. `r.Why` on a neighbor helps.
6. If the head is wrong, read its proof. Every wrong fact has a derivation,
   and the bad premise is in it.
7. Only then look at engine internals.

A passing parse proves almost nothing. The useful claim is: this exact
EDB, under this exact analyzed program, derives these rows and not the
near miss.
