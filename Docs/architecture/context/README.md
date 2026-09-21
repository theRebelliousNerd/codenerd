---
doc-class: governance
subsystem: context
implementation-status: not-applicable
last-verified: 2026-09-21
verified-against: ea90cc63
supersedes: []
---

# context

> Verified 2026-09-20 against `456e521` (`main`).

This directory is the governed architecture corpus for `internal/context` — relevance, retention, eviction, retrieval, and ordering of the agent working context. It holds the north-star vision, shipped truth, gap matrix, and capability specs for that scope. Start with [00-INDEX.md](00-INDEX.md) for read order and the grounded-vs-hypothesized map.

Package `codenerd/internal/context`: two bounded-context loops that keep an
unbounded agent session inside a finite LLM window.

1. **Turn loop** — `Compressor.ProcessTurn`
   (`internal/context/compressor_turns.go:30`) extracts control-packet atoms
   (`ExtractAtomsFromControlPacket`, `internal/context/serializer.go:321`),
   commits them to the kernel, drops the surface text, and keeps a compressed
   turn plus token accounts (`TokenCounter`, `TokenBudget`,
   `internal/context/tokens.go:38,184`).
2. **Selection loop** — `WorkingSet.Select`
   (`internal/context/working_set.go:297`) picks which stored observations
   fill the next system prompt within a character budget, walking at most two
   dependency hops and 64 entities
   (`internal/context/working_set.go:314-343`).

Scoring is a nine-scorer Go engine: `ActivationEngine.computeScore` sums
base, recency, relevance, dependency, campaign, session, issue, feedback,
and back-reference components
(`internal/context/activation_scoring.go:39-559`); kernel scores can
override per fact (`ScoreFactsWithKernelOverride`,
`internal/context/activation_scoring.go:653`).

Surfaced to operators: `BuildContext` / `GetContextString`
(`internal/context/compressor.go:645,774`), metrics (`GetMetrics`,
`GetSelectionStats`, `internal/context/compressor_metrics.go:22,145`),
persistence (`GetState` / `LoadState`,
`internal/context/compressor_metrics.go:242,254`), and per-predicate
usefulness history (`ContextFeedbackStore`,
`internal/context/feedback_store.go:25`).

What runs today is in [02-CURRENT-STATE.md](02-CURRENT-STATE.md) and
[IMPLEMENTED_SPEC.md](IMPLEMENTED_SPEC.md); what runs, what is idle,
and what the design assumes without enforcing is in
[WIRING-AND-NOT-BUILT.md](WIRING-AND-NOT-BUILT.md). `corpus.toml` in this
