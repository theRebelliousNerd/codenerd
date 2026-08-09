# Authoritative uplift cards: mangle

> This file is the sole feature-card authority for the `mangle` corpus. Statuses
> describe decisions and evidence, not optimism. Dependency order is defined in
> [03-GAP-ANALYSIS.md](03-GAP-ANALYSIS.md).

<!-- NERD_FEATURE
id: mangle-diff-eval-option-parity-v1
owner: mangle
status: verified
kind: truth-gap
depends_on: []
affects: [core, mangle, safety]
-->

## Safe uplift: differential evaluator-option parity

**Value.** A user who enables differential evaluation should receive the same
resource ceiling and explicit failure semantics as full evaluation. This closes
a safety inconsistency before more workloads opt into the faster path.

**Pre-fix evidence.** `internal/core/kernel_eval.go#RealKernel.buildDiffEngineLocked`
copied `derivedFactLimit` into `mangle.Config`, but
`internal/mangle/differential.go#DifferentialEngine.ApplyAtomDelta`, `ApplyDelta`,
and the unified fast path called `EvalStratifiedProgramWithStats` without
`WithCreatedFactLimit`. The full Engine control path did forward it.

**Resolved gap.** Differential configuration carried a value that its evaluator
never consumed. `evalOptions` now consumes it at all three direct evaluation
sites, and the regression uses a deliberately small gas ceiling. The kernel also
now resolves its unset limit through one 500,000 default for both full and diff
configuration instead of allowing the diff path to inherit the Engine's 100,000
package default. External callbacks and provenance remain separate, explicit
full-path fallbacks.

**Desired behavior.** Every positive differential `DerivedFactsLimit` becomes an
upstream `WithCreatedFactLimit` option. Exceeding it returns the evaluator's limit
error to the caller. A zero/non-positive value retains the documented unlimited
configuration semantics.

**Non-goals.** Do not enable differential mode by default, implement true delta
propagation, add evaluator telemetry, or broaden external/provenance support in
this slice.

**Affected contracts.** `mangle.Config`, differential apply methods, core diff
eligibility comments, and safety/testing docs.

**Positive acceptance.** A finite rule with a very low created-fact limit returns
the same upstream error class on full Engine evaluation plus legacy atom, legacy
fact, and unified atom differential routes. A normal finite program retains
result parity.

**Negative acceptance.** No direct differential evaluator call can run with a
positive configured limit and omit the upstream option. The regression must use
a finite Cartesian fixture rather than an unbounded program or process OOM.

**Rollback.** Disable the diff feature or restore full-path fallback while
retaining the new regression. Policy, facts, and public query semantics must not
change during rollback.

**Verification receipt.** `internal/mangle/differential.go#DifferentialEngine.evalOptions`
forwards every positive configured limit into unified atom, legacy atom, and
legacy fact evaluator calls. `TestDifferentialEngine_DerivedFactsLimit` uses a
finite 10x10 derivation with limit 5: all three subtests failed before the fix and
return `fact size limit reached` after it.
`TestKernelEval_ZeroConfigDerivedFactLimitParity` verifies that zero-config full
and diff kernel paths both resolve to 500,000. Focused differential suites and
`internal/core/kernel_eval_test.go#TestKernelDifferentialEval` pass. External and
provenance options remain full-path fallbacks and are explicitly outside this
verified slice.

<!-- NERD_FEATURE
id: mangle-explainable-replay-v1
owner: mangle
status: proposed
kind: north-star
depends_on: [mangle-diff-eval-option-parity-v1]
affects: [core, mangle, observability, transparency, cli]
-->

## Bounded future: explainable no-effects replay

**Value.** When codeNERD derives or rejects an action, a developer should be able
to answer “which facts, policy, mode, and limits produced this?” and replay the
logic without repeating a filesystem, network, or process effect.

**Evidence.** Today `internal/mangle/engine.go#Engine.GetStats`,
`internal/mangle/proof_tree.go#DerivationTrace`, core provenance, and kernel logs
expose separate slices. Neither full nor differential evaluation emits one
bounded contract that ties them together.

**Observed gap.** Operators must correlate free-form logs and path-specific trace
objects. A fallback can change the evaluation route without producing a durable,
machine-checkable explanation of the effective options.

**Desired behavior.** Define an opt-in, versioned receipt containing redacted
program/schema/policy/EDB fingerprints, input-delta identity, full/diff mode,
effective options, fallback/error class, created-fact and strata counts, duration,
and proof/provenance correlation IDs. Replay runs in a no-effects sandbox and
compares derived outputs against the receipt.

**Non-goals.** Do not store raw prompts, secrets, complete source files, or
unbounded fact bodies. Do not replay tool calls. Do not create a second policy
engine or let a receipt grant `permitted/3`.

**Affected contracts.** planned: evaluation receipt schema, core evaluator
dispatch, proof/provenance correlation, observability retention/redaction, CLI
inspection and replay commands.

**Positive acceptance.** A capped fixture can record and replay both a successful
derivation and a resource-limit rejection with matching fingerprints and outputs.
Full and differential receipts use the same schema. Redaction and size caps are
unit-tested; replay performs zero registered external actions.

**Negative acceptance.** Receipt creation cannot change derivation order or
policy outcome, retain a marked secret, exceed configured bytes, or silently
accept a program/fact fingerprint mismatch. Replay cannot access VirtualStore
effect handlers.

**Rollback.** Turn off receipt emission and replay registration while preserving
ordinary evaluation. Existing receipts remain versioned data and never become a
required boot dependency.

## Supporting backlog, subordinate to the cards

- [x] Close direct `parse.Unit` production calls and prove mixed parser
  concurrency under `go test -race` (2026-08-09).
- Pin or remove the package-local intent-rule shadow after a live-runtime parity
  decision.
- Make unified-fast-path Query/Snapshot behavior explicit and tested.
- Feed analyzed `ProgramInfo` declarations into schema validation where possible.
- Migrate remaining model-authored-rule producers to a pinned synth protocol.
- Measure snapshot memory and SIMD value before redesign or wiring.
