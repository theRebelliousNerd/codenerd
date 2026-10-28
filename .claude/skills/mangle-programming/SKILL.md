---
name: mangle-programming
description: >
  Designs, writes, debugs and experiments with Mangle (codeberg.org/TauCeti/mangle-go
  v0.7) for codeNERD's kernel, leading with what Mangle has beyond plain Datalog:
  structured values and tagged unions, merge lattices, temporal facts, proofs as
  data, Go-backed external predicates, checkable types and transforms, composed
  with Go. Use for any .mg file, Decl, policy rule, aggregation, kernel derivation,
  check-mangle error, or a rule that compiles but derives nothing, and when
  deciding how Go and Mangle should split a job.
paths:
  - "**/*.mg"
metadata:
  version: 2.0.0
---

# Mangle programming: make Mangle the star

Treat Mangle as a reasoning engine that Go composes, not as a lookup
table or a Datalog dialect to keep small. A good design here uses
something only Mangle has, where it does the job better than Go would.
Hold every design to that. **TauCeti v0.7 is canon**: when this skill
and the engine disagree, the engine wins and this skill gets fixed.

## First question for any task

*What can Mangle do here that Go code and flat Datalog can't do as
clearly?* Go through the list below before writing a clause:

| If the problem has... | Reach for | Read |
| --- | --- | --- |
| records that travel together (events, tool calls, payloads) | structs, lists, maps, tagged unions | [structured-values](references/structured-values.md) |
| "keep the best / freshest / cheapest per key" | `fundep` plus `merge` lattices | [lattices-and-merge](references/lattices-and-merge.md) |
| validity windows, expiry, cooldowns, "in the last N days" | temporal facts and operators | [time-as-a-dimension](references/time-as-a-dimension.md) |
| "why did it decide that?", audit, "why not?" | proofs as Mangle facts | [proofs-as-data](references/proofs-as-data.md) |
| a question only Go, the disk, SQLite or Jev can answer | external predicates | [go-as-oracle](references/go-as-oracle.md) |
| a closed vocabulary or variants that must not drift | union, singleton and tagged types with strict bounds | [types-as-contracts](references/types-as-contracts.md) |
| counting, budgets, rankings, answer-shaped output | `let` and `do` transforms | [transforms-and-aggregates](references/transforms-and-aggregates.md) |
| reachability, inheritance, components, shortest paths | recursion carrying values, pruned by lattices | [graphs-and-closure](references/graphs-and-closure.md) |
| rules to generate, gate, diff or mutate | programs as data | [programs-as-data](references/programs-as-data.md) |
| facts about code: wiring, taint, cycles, dead Decls | Mangle over code | [mangle-over-code](references/mangle-over-code.md) |
| several engines, what-if worlds, replay, invariants | Go composition patterns | [go-composition-patterns](references/go-composition-patterns.md) |

If nothing on the list fits, plain Datalog with stratified negation is
fine. Say so, and keep it small.

## Think differently

- **A relation is a judgment, not a table.** Name the EDB facts Go will
  measure, the IDB judgment Mangle derives, and the consumer that obeys
  it ([relational-thinking](references/relational-thinking.md)).
- **One fact can hold a whole thing.** Stop spreading an event across five
  predicates. Destructure it only where a rule must join on a part.
- **"Best" is a lattice, not a sort.** Keep the winner inside the
  fixpoint instead of computing everything and picking afterwards in Go.
- **Time belongs to the fact.** A lease that expires stops deriving. Go
  owns the clock and pins it, so every decision can be replayed.
- **The proof is the explanation.** Proofs are data, so policy can reason
  about reasoning, and the UI can show the real derivation.
- **Go is the oracle, Mangle is the judge.** Externals bring the world in
  on demand, only for the bindings the rules reach.
- **Types are contracts you can switch on.** The kernel doesn't check
  bounds today. Strict analysis in labs and tests makes Decls mean
  something.
- **Programs are values.** Rules can be generated, gated, diffed between
  versions and mutation-tested like any other Go value.
- **Cost is rows, not lines.** Estimate intermediate substitutions per
  rule ([scale-and-performance](references/scale-and-performance.md)).

## Experiment first, then build

Every mission starts in a disposable lab that tries a Mangle-only feature
on the real problem. Then the verdict decides the production design.

```bash
.claude/skills/mangle-programming/scripts/new-lab.sh my_idea
go test -v ./_mangle_lab/my_idea/
```

- [scripts/new-lab.sh](scripts/new-lab.sh) scaffolds `_mangle_lab/<slug>/`.
  It is gitignored and invisible to `./...`.
- [scripts/lab/harness.go](scripts/lab/harness.go) runs a program on the
  pinned engine with strict bounds, a temporal store, the proof recorder,
  externals and a pinned `now`.
- [scripts/lab/template_test.go.txt](scripts/lab/template_test.go.txt) is
  the experiment file: hypothesis, program, verdict.
- [lab-protocol](references/lab-protocol.md) covers the loop, what makes
  a good experiment, how to promote one, and what to remember.
- [pushing-mangle-forward](references/pushing-mangle-forward.md) lists
  feature combinations and a ranked backlog of experiments, so there is
  always a next push.

Quick probes without time features: `./nerd check-mangle --standalone --eval <pred> file.mg`.

## Ground truth (each item checked on v0.7)

- Reducers are lowercase (`fn:count()`). There is no `fn:pair:fst/snd`;
  use `:match_pair`.
- A `:match_*` output slot must be a free variable. Bind it, then compare.
- Negation is `!`, and it must be fully bound: `!r(X, _)` is an analysis
  error. Negate a projection.
- `!ext(...)` on an external predicate reads only the store and never
  calls Go. Project positively first.
- The kernel does **not** check Decl bounds (`AnalyzeOneUnit`). Only
  generated rules are checked strictly.
- Termination is **not** guaranteed: `fn:plus` recursion runs to the fact
  limit.
- Temporal programs need a temporal store. `check-mangle` has none; the
  lab harness does.

Details and the rest: [verified-traps](references/verified-traps.md).
Add every new trap you prove.

## codeNERD rules of the road

- Go measures, asserts and executes. Mangle decides permission, routing,
  freshness, budgets and completion. No policy `if` in Go "just in case".
- Schemas then policy then learned rules form one program, rebuilt on
  every change. A parse error anywhere is a boot failure everywhere. Keep
  one Decl per predicate, in one schemas file.
- No prose, taxonomies or fuzzy matching in `.mg`. Similarity belongs to
  embeddings or Jev behind an external.
- Rebuild `nerd` after any `.mg` change: the programs are embedded.
- Wiring, admission traps, layers and test commands:
  [codenerd-integration](references/codenerd-integration.md).

## Showcase

codeNERD is the premier Mangle showcase, and Mangle's author has forked
it. Good examples of what Mangle can really do don't exist anywhere else
yet, so codeNERD's production features are meant to be those examples.
When a Mangle-only feature ships, index it as `showcase/<slug>.md` in the
format in [showcase/README](showcase/README.md), and link it here.

Entries: none yet.

## Where truth lives

[canonical-sources](references/canonical-sources.md) gives the authority
order, which upstream file answers which question, and mines worth
digging (the engine's `examples/` and the Vectryx skill, which is good for
ideas but stale for code). Upstream prose has bugs. The engine's source,
then a lab run, settle every question.
