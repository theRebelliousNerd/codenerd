# OPEN QUESTIONS — Context

> Last verified against codebase: 2026-07-13  
> Real open questions (not rhetorical)

## Q1 — Dual scoring systems: how long?

Kernel `should_include_context` and Go 9-component scoring both live.  
What is the retirement criterion for the Go heuristic as primary path?  
Who owns score parity tests when policy rules change?

## Q2 — Token accounting accuracy

Is the 4-char/token heuristic acceptable forever for hard enforcement, or do we need model-specific counters before claiming window safety against provider limits?

## Q3 — Async ProcessTurn consistency

Chat fires `ProcessTurn` in a goroutine. Should critical paths await completion before the next user turn is accepted, or is eventual consistency intentional?

## Q4 — Observation masking completeness

**Resolved (2026-08-15).** `maskedObservationTurns` queries it on every compression and `generateObservationMaskedSummary` obeys it. The predicate had never fired at all: `assertTurnAgeCategories` appended a clause terminator that `ParseFactString` adds itself, so every assertion failed to parse and the error was discarded.

## Q5 — Feedback cold start

With `minSamples=10`, early sessions get zero feedback signal. Is that correct, or should bootstrap priors exist per intent verb?

## Q6 — Session dual IDs

Compressor and ActivationEngine mint independent session IDs. When is `SetSessionID` required for rehydrate correctness, and is activation session ID ever persisted?

## Q7 — Relationship to retrieval

When should tiered retrieval (`internal/retrieval`) inject facts for activation vs when should activation alone suffice? Is there a documented precedence?

## Q8 — Package README vs architecture corpus

Should package README become a thin pointer to `Docs/architecture/context/`, or remain a standalone operator guide?

## Q9 — Load-bearing callers after refactor

Which cross-package callers (`process.go`, harness, JIT) are load-bearing if compressor construction moves into `internal/system`?

## Q10 — Crash dump files

Should `debug_program_ERROR.mg` under `internal/context/` be gitignored / cleaned, or kept as forensic samples?
