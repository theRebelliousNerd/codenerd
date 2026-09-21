# Counting and limits

How `internal/broker` answers "how big is this request" and "may it go".

Verified 2026-09-20 against the working tree (commit hash unavailable:
no shell tool in this environment; re-pin with `git rev-parse --short HEAD`).

## Measuring: one function, in runes

`measure` (measure.go:40) is the single definition of request size.
It counts runes, not bytes, because token-per-rune ratios are stable
across languages where token-per-byte ratios are not (measure.go:33-35).

The per-element overhead constants (measure.go:22-31) do not need to
be exactly right: the calibrator learns its ratio over `measure`'s
output and predicts from the same output, so systematic bias cancels.
They only need to scale with request shape (measure.go:11-21).

Two details that have bitten before:

- History is counted from each message's ordered content blocks, not
  its flat text fields, so a replayed thinking block is billed. A
  redacted block's encrypted signature is ~13 chars per reasoning
  token and would read a 22,853-token think as ~193,735 tokens and
  refuse it; instead its replay cost is bounded by the token count
  the provider reported when it produced the reasoning, expressed
  back in characters (measure.go:64-82).
- A value that will not JSON-marshal is charged a deliberately large
  4096 chars: fail toward refusing, never toward silently
  undercounting (measure.go:148-151).

The per-segment split of an authoritative total is proportional
attribution for observability only, never a measurement
(measure.go:153-159). Segment remainders go to Tools so parts always
sum to the total (measure.go:168-181).

## Learning: the ratio, not a constant

`Calibrator` (calibration.go:60) learns chars-per-token per model.
Seed is 3.5, set below 4.0 because `measure` includes framing
overhead and code tokenizes densely (calibration.go:11-21). The first
observation replaces the seed outright (`alpha = 1/(n+1)`, floored at
0.15 so tracking never freezes), and every sample is clamped to
[1.0, 20.0] with sub-256-char samples ignored
(calibration.go:115-151).

Confidence is therefore ever only `seeded` or `calibrated`
(calibration.go:86-95). `EstimatingCounter.Count`
(counter.go:45-60) says it is a guess in its own type. Exact counts
exist from exactly one place: the Anthropic counting endpoint, and
only Anthropic gets API credentials for that purpose
(internal/perception/broker_install.go:45-49).

Calibration integrity is guarded at settle time: cached tokens are
added back to input before observing, because a provider that
excludes cache reads from `input_tokens` would otherwise make a
cache hit look like impossibly dense content and poison the ratio
with the optimization working as intended (broker.go:215-234).

## Enforcing: hard window, policy budget

`Ledger` (ledger.go:24) enforces two different limits, and the
distinction is load-bearing (ledger.go:8-21):

- The context window is a hard provider limit. A request that
  exceeds it is rejected by the API after transmission, and billed.
  Refusing locally is strictly cheaper.
- A purpose budget is local cumulative policy, not a protocol error.

`Ledger.Admit` (ledger.go:93) checks the window first (minus output
reserve, ledger.go:105-114), then the purpose cap against recorded
spend (ledger.go:116-127). `Ledger.Record` (ledger.go:139) folds in
only provider-reported actuals — estimates are never booked as
money (ledger.go:134-138).

Three operational notes:

- `SetBudgets` replaces caps in place, keeping balances: live cores
  hold the ledger pointer from `Wrap` time, so swapping the object
  would fork enforcement and race settlements (ledger.go:198-216).
- `Reset` clears spend but keeps limits for session restart
  (ledger.go:218-224).
- A non-positive window disables window enforcement and every
  admission reports zero headroom — visible in receipts, not
  mistaken for a pass (ledger.go:36-46). Enforcement assumes the
  window is configured.
