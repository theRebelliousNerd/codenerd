# Time as a dimension, not a column

Plain Datalog facts are true forever, so "is this still true?" becomes a
timestamp column plus a Go clock check. Mangle facts can carry a validity
interval, and rules can ask temporal questions directly. The engine owns
the arithmetic, interval merging and the reference "now".

## The vocabulary (see `readthedocs/temporal.md`)

- Facts: `lease(/shard_a, /file_x)@[2026-10-28T10:00:00, 2026-10-28T10:05:00].`
  `_` means unbounded, and `now` is the evaluation time.
- Declare it: `Decl lease(S, F) temporal bound [/name, /name].`
- Operators: `<-[0d, 7d] p(X)` (at some point in the last 7 days),
  `[-[0d, 7d] p(X)` (continuously for 7 days), and the forward versions
  `<+` and `[+`.
- Bind the interval: `p(X)@[S, E]` gives you the start and end timestamps.
- Interval comparisons: `:interval:before/after/meets/overlaps/during/contains/starts/finishes/equals`.
- Adjacent or overlapping intervals of the same fact coalesce
  automatically, so recursion doesn't multiply intervals.

## The Go side

- The program needs a `factstore.TemporalStore`,
  `engine.WithTemporalStore` and `engine.WithEvaluationTime(t)`.
  `nerd check-mangle` has none; the lab harness does, and `harness.At(t)`
  pins `now`.
- Go owns the clock, and that's a feature: pin `now` and an evaluation can
  be replayed exactly. Time-travel debugging becomes "evaluate with
  `At(then)`".
- The codeNERD kernel has no temporal store today. Promoting a temporal
  design means giving its program a temporal store and showing that
  boot cost and cone evaluation still hold.

## codeNERD seams to try

- **Leases and locks**: file and shard ownership with expiry. Expired
  leases stop deriving on their own; nothing has to sweep them.
- **Cooldowns and rate limits**: "allowed to retry" means no retry in the
  last 10 minutes. Project `recently_retried(T) :- <-[0d, 10m] retried(T).`
  and negate the projection. Whether `!` composes directly with `<-` is
  the first thing to probe.
- **Freshness**: "the test result is fresh if a passing run overlaps the
  last edit": `:interval:after(TestRun, LastEdit)`. This replaces mtime
  comparisons scattered through Go.
- **Session memory with decay**: a preference is active while its interval
  is, and continuous observation (`[-`) promotes a habit to a rule.
- **Campaign timelines**: phases as intervals, `:interval:meets` for
  handoffs, `:interval:overlaps` to detect conflicting work.
- **Incidents**: "error B within 10 minutes after event A" is the
  `temporal_sequence.mg` pattern, aimed at log facts.

## Traps

- Dates without a time mean midnight, and intervals are inclusive. To cover
  all of Jan 31, end at `2024-02-01`.
- `now`-relative rules give a snapshot. Re-evaluating later gives a
  different answer on the same data. Tests must pin time.
- Temporal facts and stratified negation together are untested ground
  here. Probe `!<-[...]` before you rely on it.
- Interval values in Go are nanosecond timestamps. Keep units in mind when
  you mix them with `fn:time:` and `fn:duration:` functions.
