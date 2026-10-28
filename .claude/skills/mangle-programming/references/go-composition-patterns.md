# Composing Mangle with Go

Mangle is a library, not a server. Programs, stores, evaluation options,
externals and recorders are Go values you can build, combine and throw
away freely. Most of the leverage is in how Go arranges them.

## The split

Go measures and asserts, Mangle judges, and Go executes and renders.
Policy branches in Go "just in case" are a bug (`codenerd-integration.md`).
The patterns below are ways to keep that split while getting more from
it.

## Arrangements

- **Several small engines**: one program per concern (action kernel,
  orientation, routing experiments), each with its own store. Hand facts
  between them in Go. A slow concern no longer stalls the action loop.
  codeNERD already runs orientation separately.
- **Layered stores**: `factstore.NewMergedStore(readStores, writeStore)`
  evaluates over a shared read-only base plus a private write layer.
  That's a cheap what-if: same base EDB, a speculative fact added,
  derivations diffed, layer thrown away. Use it for the Dreamer and Precog
  safety checks.
- **Teeing stores**: `NewTeeingStore(base)` watches every fact added during
  evaluation. Use it to stream derived facts to telemetry or to the UI
  while the fixpoint runs.
- **Pinned evaluation**: `WithEvaluationTime` plus `WithDeterministicOrder`
  make an evaluation a pure function of (program, EDB, time). Snapshot all
  three on a bad decision and replay it exactly in a test.
- **Speculation**: evaluate "if I take action A" by asserting A's expected
  effects into a layered store, and derive `violates(...)`. Only actions
  whose speculative world is clean get `permitted`.
- **Differential checking**: run the cone evaluation and the full fixpoint
  on the same input and require identical results. Run the old policy and
  the new one and diff the decisions (`programs-as-data.md`).
- **Invariants as relations**: write `violation(Kind, Subject)` rules over
  facts Go extracts from its own state, and fail a test or alert when any
  row derives. Mangle becomes the oracle for Go's property tests: Go
  generates random states, Mangle says which are illegal.
- **Answer-shaped results**: end pipelines with
  `collect`/`collect_to_map`/structs so Go reads one fact and gets a
  render-ready value (`transforms-and-aggregates.md`).
- **Lazy world**: externals answer questions on demand
  (`go-as-oracle.md`), so the kernel holds decisions, not a copy of the
  world.

## Boundary hygiene

- Build values from the Decl at the producer: names versus strings,
  int64 versus float64. A string in a `/name` slot is the most common
  reason a rule derives nothing.
- Convert once. Keep `ast` constants on the Go side across evaluations of
  the same program generation, and drop them when the program changes.
- Batch assertions, then evaluate once. Never evaluate per fact.
- Query for existence with a bound atom, not by exporting every row.
- Log phases separately: parse, analyze, assert, evaluate per stratum,
  query.

## Experiments to push

1. A what-if gate for one destructive action, using a merged store and a
   `violates` relation, compared against today's check.
2. A teeing store that streams `next_action` derivations to the TUI as
   they happen.
3. A property test where Go generates random tool-call histories and
   Mangle invariants catch illegal ones.
