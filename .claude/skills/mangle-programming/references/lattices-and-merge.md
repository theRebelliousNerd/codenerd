# Lattices: relations that keep only the best fact

In plain Datalog a relation is a set, and every derivation adds a row.
Mangle can instead declare that a relation keeps one winning value per key,
chosen by a merge predicate you write. That's a custom lattice: "best so
far" computed inside the fixpoint, not afterwards in Go.

## The shape (from the engine's shortest-path example)

```mangle
Decl shortest_path(Source, Target, Path)
  descr [fundep([Source, Target], [Path]), merge([Path], "shorter")].
Decl shorter(P1, P2, Best) descr [mode('+', '+', '-'), deferred()].

shorter(P1, P2, P1) :- fn:list:len(P1) < fn:list:len(P2).
shorter(P1, P2, P2) :- fn:list:len(P2) <= fn:list:len(P1).
```

The rules for `shortest_path` look exactly like an all-paths closure. The
Decl makes the store replace a longer path with a shorter one as it goes,
so the recursion explores far less than the full path set.

## How the engine applies it (read from `engine/seminaivebottomup.go`)

- When a new fact arrives with the same key, the engine evaluates the merge
  predicate top-down on (old value, new value) and keeps the result.
- If the merge predicate gives no answer, the two values are incomparable
  and **both are kept**. A partial order is allowed.
- Exactly one `fundep` and exactly one target column per predicate.
  Anything else is an error ("not implemented").
- The existing fact is matched on the **leading** argument positions, one
  per source key. Put the key columns first. (Inferred from the code;
  confirm in a lab.)
- The old fact is removed only when the store implements
  `factstore.FactStoreWithRemove`; otherwise it stays. **Check this first in
  any lab**: the store the kernel and the harness use decides whether the
  lattice really replaces facts.

## Why this beats the usual workaround

There is no arg-max reducer. The usual pattern computes `fn:max` in one
stratum and joins it back in the next. That costs two strata and needs the
full relation first. A lattice keeps the winner continuously, works inside
recursion, and can rank with any rule-defined comparison, not just numbers.

## codeNERD seams to try

- **Route or model choice**: per task, keep the cheapest route that
  satisfies the constraints, with cost as a struct of
  (latency, tokens, risk) and a merge rule that encodes the trade-off.
- **Freshest evidence**: per (file, claim), keep the observation with the
  latest timestamp. This replaces "sort by time and take the first" in Go.
- **Highest-confidence classification**: per item, keep the label with the
  best Jev probability (see `go-as-oracle.md`). A tie is incomparable, so
  both labels are kept and the tie is visible.
- **Minimal blast radius**: per change, keep the smallest affected set
  (`fn:list:len`), the way shortest paths does.
- **Budget closure**: propagate a remaining budget down a plan graph and
  keep the max remaining per node.

## Open questions for experiments

- Does merging stay monotone enough that semi-naive evaluation still gives
  the same answer as a naive run? Compare the two on the same EDB.
- How do provenance proofs look for a fact that replaced another? Is the
  loser's derivation still visible?
- What's the speedup over the max-then-join-back pattern at 100k+ rows?
