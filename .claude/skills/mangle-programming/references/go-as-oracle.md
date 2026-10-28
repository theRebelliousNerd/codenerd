# Go as an oracle: external predicates mid-derivation

Plain Datalog only sees facts loaded before evaluation starts. Mangle can
call Go *during* the fixpoint: a predicate declared `external()` is answered
by a Go callback, with modes saying which arguments Go receives and which
it fills. The kernel can ask the world a question exactly when a rule needs
the answer, and only for the bindings that rule reached.

## Mechanics (`engine.ExternalPredicateCallback`)

```mangle
Decl recall_similar(Query, TopK, Results)
  descr [external(), mode('+', '+', '-')] bound [/string, /number, /string].
```

- `ShouldQuery(inputs, filters, pushdown)` returns false to skip a call.
  Use it to cache and to remember empty answers.
- `ExecuteQuery(inputs, filters, pushdown, cb)` calls `cb(outputs)` once
  per row. The engine adds each row as a fact and keeps going.
- With `ShouldPushdown() == true`, Go also receives the rule's later
  subgoals on the output variables. It can apply them at the source, like a
  `WHERE` handed to SQL.
- An external must declare exactly one mode.
- codeNERD wires about a dozen of these in
  `internal/core/external_predicates.go` (`query_learned`, `query_session`,
  `recall_similar`, `query_knowledge_graph`, ...). The lab registers one
  with `harness.WithExternal`.

## Verified trap: negation skips the oracle

`engine/premise.go` `premiseNegAtom` answers `!ext(...)` from the fact store
only and never calls the callback (v0.7; Vectryx hit the same thing on
v0.6). `!recall_similar(Q, 5, R)` is true whenever nothing has been stored
for those inputs yet. Project positively first, then negate the projection:
`has_hit(Q) :- recall_similar(Q, 5, _).` then `!has_hit(Q)`. Test the
negated form against a positive control.

## Ideas

- **Jev as a predicate.** Ask TypeSafe a typed question per candidate, for
  example `jev_is(Text, /bug_report, P)`, and get a calibrated
  probability back as a fact. Rules then combine probabilities with hard
  facts. The LLM gives the signal and Mangle makes the decision; the
  "neuro-symbolic mediator" from the Vectryx skill is this pattern. Use
  `:float:gt` for thresholds (`>` is integers only), and derive the
  fallback decision from a separate predicate, never by negating the rule's
  own head.
- **Lazy measurement.** Stop pre-asserting filesystem or git facts for the
  whole workspace. A `file_stat(Path, Size, MTime)` external is asked only
  for the paths the rules reach, so the census cost drops to the cone of
  interest.
- **Top-k as a relation.** Vector search returns k rows. With a lattice or a
  `do` transform, Mangle picks among them by policy, not by raw score.
- **Pushdown into SQLite.** An external backed by the session database
  turns the rule's later subgoals into SQL predicates, so huge history
  never enters the kernel.
- **Oracle plus proofs.** The fact an external returned shows up as an EDB
  leaf in the proof, so the audit trail can show which outside answer a
  decision rested on.

## Discipline

- An oracle must be deterministic for one evaluation. Cache by inputs for
  the evaluation's lifetime, or the fixpoint can see two answers.
- A slow or networked oracle needs `ShouldQuery` gating and a timeout.
  A hang inside the fixpoint hangs the kernel.
- Errors from `ExecuteQuery` fail the whole evaluation. Return an error
  only when the decision can't be made safely without the answer.
- Arguments arrive as `ast.Constant`. Convert with the Decl's types:
  names stay names and strings stay strings.
