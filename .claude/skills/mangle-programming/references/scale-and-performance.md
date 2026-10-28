# Scale: 100k to millions of facts

## Measure the phase before optimizing

Separate these costs, because each has a different fix: source assembly,
parse, analysis and stratification, Go-to-atom conversion, store
insertion and indexing, fixpoint per stratum, and query or export.

In codeNERD the action program parses and analyzes in hundreds of
milliseconds, while a workspace kernel can spend a minute in
`loadMangleFiles`. That's orientation refresh: a ~55 MB snapshot, 100k+
facts asserted into a second engine, and a multi-megabyte projection.
Name the phase before calling the engine slow. `r.Stats` in a lab gives
per-stratum durations.

## The cardinality law

Cost comes from intermediate substitutions, not source lines. For every
rule, estimate rows per body predicate, how much each shared key
restricts the join, recursive growth per iteration, distinct head rows,
and whether a later stratum repeats the same join.

One accidental Cartesian join of two 10k-row relations is 100M
candidates. **Variable continuity** (from the Vectryx skill): every body
atom after the first should share a variable with an atom before it.
An atom that shares none is a cross product.

## Rules of shape

- Join on stable, selective, scalar, consistently typed keys.
- Split a huge rule into named intermediate relations when a selective
  restriction can then be materialized once. Don't split mechanically:
  every materialized relation costs memory.
- Aggregate late, unless the aggregate is itself the restriction.
- Avoid building structured terms inside hot recursion. New terms defeat
  deduplication and can grow without bound.
- Avoid string or regex built-ins over census-sized relations. Normalize
  once at measurement time.
- Close graphs only in the direction the question needs.
- A lattice can turn "all paths, then pick" into "best path as you go".
  That's often the biggest win available (`lattices-and-merge.md`).
- A fact limit is a circuit breaker, not a design. Hitting it must
  surface as an error.

## EDB discipline

- Assert each ground fact once. Batch, then evaluate once.
- Reuse converted atoms only while the program generation is unchanged.
  A stale atom cache is a correctness bug.
- Bulk history belongs behind an external predicate or a store
  (`go-as-oracle.md`), not in every action kernel.
- Prefer projection facts that answer downstream questions over raw
  documents.

## Whole fixpoint versus cone

- A fresh bottom-up store is sound after retractions. Never reuse a store
  that holds old derived facts for a "small" re-evaluation: bottom-up only
  adds, so retracted premises and aggregates go stale.
- Cone evaluation must stay observationally identical to the full
  fixpoint. Keep a differential test.
- Parallelize independent programs before parallelizing one fixpoint.

## Caching

- Cache parse and analysis by exact program bytes, and only if the
  analysis object is safe to share. Prove it with race tests.
- Key every refresh memo on all of its inputs (HEAD, snapshot, config). A
  bare `sync.Once` around a workspace refresh is a stale-data bug.
- Invalidate derived-fact snapshots by program hash **and** EDB identity.

## Benchmark protocol

For a claimed speedup, record before and after on the same workspace:
program bytes, EDB and derived counts, per-phase times, fixpoint per
stratum, queries with row counts, and wall time and peak memory. A
benchmark over 1,000 toy facts doesn't validate a million-fact shape;
generate production-sized EDB in the lab.

## Red flags

Several kernels each rescanning the repo. A JSON snapshot parsed just to
learn whether anything changed. Full history asserted before a yes/no
staleness check. Materializing every row to test existence. Evaluating
after each fact in a batch. `QueryAll` inside a per-fact loop. Logs that
show only total time.
