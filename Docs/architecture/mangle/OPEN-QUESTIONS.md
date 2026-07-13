# OPEN QUESTIONS — mangle

> Last verified: 2026-07-13  
> Real unknowns — not rhetorical.

## Q1 — Is `intent_routing.mg` on the runtime program path?

The file defines intent→action/persona rules with local Decls. Is it merged into kernel schemas/policy at boot, loaded only in tests, or currently dormant?

**Why it matters:** Claiming declarative intent routing as live depends on the answer. Wiring audit required.

## Q2 — Should DifferentialEngine always use fine-grained Stratify inside unified path only?

Today library stratification for per-stratum stores is 2-bucket; unified path uses full Stratify for a single call. Is there a future where strataStores go away entirely if kernel is the only production consumer?

## Q3 — Auto-atomizer vs strict schema types

`convertValueToTypedTerm` promotes identifier-like strings to names when not StringType. Does this still surprise callers outside kernel? Should Auto-Atomizer be opt-in?

## Q4 — Snapshot semantics name

Comments say “Copy-On-Write” but implementation deep-copies facts. Rename API docs to “isolated snapshot copy” or implement real COW?

## Q5 — Sanitizer and feedback ValidationError type dualism

`mangle.ValidationError` (grammar) vs `feedback.ValidationError` are different types. Is that intentional long-term or should they converge?

## Q6 — Proof trees vs provenance

When should operators trust `ProofTreeTracer` vs kernel `provenance.DerivationRecorder`? Which is the glass-box source of truth?

## Q7 — Session budget defaults

Are MaxRetries=3 and SessionBudget=20 correct for long campaigns that learn many rules, or should campaign mode raise budgets explicitly?

## Q8 — SIMD intersect ownership

Does join performance work belong in `internal/mangle` or closer to mangle-go / a dedicated index package?

## Q9 — External tools for synth

Should `mangle_synth_tool` be a VirtualStore tool (agent-callable) or stay an internal library only?

## Q10 — Policy on unlocked parse in tests

Tests that call `parse.Unit` directly — allowed if single-threaded, or should test helpers also use ParseUnit for consistency?
