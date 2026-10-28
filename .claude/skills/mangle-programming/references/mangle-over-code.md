# Mangle over code: static analysis as a few rules

Program analysis is a fixpoint over a graph, which is what Mangle is for.
The engine ships live-variable analysis (`examples/dataflow_liveness.mg`)
and a borrow-style region checker over struct-encoded instructions
(`examples/flow_checking.mg`). codeNERD is a Go codebase that is mostly
wiring, so pointing Mangle at its own source is the obvious next step.

## The seam

Go extracts facts with `go/packages` and `go/ast` (or the existing
`internal/world` scanners). Mangle derives the judgments, and Go reports
them. Facts are narrow and typed:

```mangle
func_decl(Fn, Pkg).               calls(Caller, Callee).
implements(Type, Iface).          registers(Fn, /virtual_predicate, Name).
mg_decl(Pred, Arity, File).       mg_uses(Pred, Arity, File).
go_asserts(Pred, Arity, Fn).      go_queries(Pred, Arity, Fn).
```

## Analyses worth building

- **"Exists but doesn't run"**: reachability from `main` and the test
  roots over `calls` plus registration edges. A function with real code
  but no path from a root is unwired. This is the codeNERD failure mode
  in one rule.
- **Decl to producer to consumer**: a predicate declared in `.mg` that no
  Go code asserts and no rule derives is dead. One asserted but never
  queried is telemetry nobody reads. Join `mg_decl`, `go_asserts`,
  `go_queries` and `mg_uses`, then negate the projections.
- **Taint ("poisoned river" in the Vectryx skill)**: user or LLM text
  flowing to a shell, path or SQL sink without passing a sanitizer. That's
  transitive closure over data-flow edges, cut at sanitizer nodes with
  stratified negation.
- **Lock order / deadlock**: a lock-acquired-while-holding graph. A cycle
  means a potential deadlock. Cycle detection is
  `reach(X, X) :- ...` over the wait-for relation.
- **Layer rules**: "internal/core must not import cmd/..." as a rule over
  `imports` facts. A violation is a derived fact with its proof as the
  import chain.
- **Mangle over Mangle**: parse codeNERD's own `.mg` corpus with the
  engine's `parse` package, assert `rule_head` / `rule_body` /
  `negates` facts, and derive dead rules, unreachable strata or the
  predicates a change touches (its cone). The analyzer and the corpus
  share one language.

## Making it fast

- One fact per edge, scalar IDs, and join on IDs. Don't put whole AST
  nodes in hot relations. Keep a `node_source(Id, Text)` side table for
  reporting.
- Extract once per git HEAD and cache by content hash (see
  `scale-and-performance.md`). The census is the expensive part, not the
  rules.
- An external `callers_of(Fn, Caller)` backed by `go/callgraph` lets the
  rules pull only the cone they need (`go-as-oracle.md`).

## Experiment ideas, in order of payoff

1. The Decl/producer/consumer audit over the real repo. Compare against
   the undeclared-assert budget tests already in `internal/core`.
2. An unwired-function report for one package, checked against what
   reviewers have flagged by hand.
3. Policy diff by proof (`programs-as-data.md`) for the next `.mg` change.
