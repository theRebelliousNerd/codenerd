# Proofs as data: the kernel can explain itself, to itself

Plain Datalog gives answers. Mangle can also give the derivation: which
rule fired, with what bindings, resting on which base facts, including
"this was absent" for negation. Those proofs come back as ordinary Mangle
facts, so a second program can reason about why the first one decided.

## What exists (from `readthedocs/provenance.md` and `provenance/`)

- `provenance.Explain(info, store, goal, opts)` rebuilds a proof after the
  fact. It covers ordinary rules, recursion, equality and negation.
- Record while evaluating, with `engine.WithDerivationRecorder(provenance.NewMemoryRecorder())`
  and then `BuildFromRecording`. This also covers `let` and `do`
  transforms, so aggregates get proofs too.
- `provenance.EmitFacts` writes proofs as facts: `proves(Id, Fact)`,
  `uses_rule`, `premise(Id, Index, Sub)`, `edb_leaf`, `absence_leaf`,
  `binding(Id, Var, Value)` and `rule_source`. A fact appears as a list, so
  `edge(1, 2)` becomes `[/edge, 1, 2]`.
- Proof IDs are content hashes, and shared sub-proofs share an ID. A proof
  is a graph with stable identities across runs (v0.7 sorts alternatives).
- codeNERD already uses `Explain` in `internal/core/kernel_explanation.go`.
  In the lab, `r.Why(goal)` and `r.ProofFacts(goal)` do both.

## Ideas that put proofs to work

- **Meta-policy**: load a decision's proof facts into a second program and
  ask policy questions about the reasoning itself, for example "was this
  `permitted` derived without any rule from `learned`?" or "does this route
  rest on an EDB fact older than the last edit?" That makes trust in a
  decision something you can query.
- **Blast radius of a fact**: invert `premise` to answer "if this EDB fact
  is retracted, which decisions lose their only proof?" Mangle computes
  that by reachability over proof facts.
- **Why not**: build the near miss as a goal, look for the first premise
  without proof, and report it. This is the debugging step of
  `relational-thinking.md`, automated.
- **Proof IDs as cache keys**: a decision with the same proof ID was made
  for the same reasons. Skip re-explaining it to the user, and dedupe audit
  log entries.
- **Learning from proofs**: tally which rules fire in successful versus
  failed turns (`uses_rule` aggregated with `do`). Dead rules and
  overloaded rules both show up.
- **Glass-box UI**: render the proof graph instead of having an LLM
  paraphrase "why". The proof is the explanation.

## Costs and limits

- A recorder on a hot path allocates per derived fact. Measure, record per
  evaluation only when asked, and reset between evaluations.
- `Explain` marks proofs `Partial` for transforms; use recorder mode
  there.
- Temporal annotations fall outside simple provenance. Proofs of temporal
  facts are an open experiment.
- An `absence_leaf` holds for a closed lower stratum. It means "not
  derivable here", not "false in the world".
