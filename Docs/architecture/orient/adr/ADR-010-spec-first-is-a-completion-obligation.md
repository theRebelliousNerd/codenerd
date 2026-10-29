---
doc-class: governance
subsystem: orient
implementation-status: accepted-not-implemented
last-verified: 2026-09-29
verified-against: bb7bafac
supersedes: []
---

# ADR-010: Spec-First Is a Completion Obligation

## Context

In the current codeNERD runtime, session turn completion is governed by `turn_done(Turn) :- turn_executed(Turn), turn_verified(Turn)` in `internal/core/defaults/policy/coder_safety.mg:632-633` and consumed by `consumeTurnDoneSignal` in `internal/session/executor.go:2879-2915`.

In lines 674-676 of `coder_safety.mg`, `turn_verified` evaluates strictly mechanical toolchain gates:
- Clean build (`!has_red_gate(Turn)`)
- Passing tests (`!has_unmet_gate(Turn)`)
- Test coverage and execution debt (`!turn_has_untested`, `!turn_has_uncovered`)
- Clean linter and vet reports (`!turn_vet_red`)

Zero obligations exist for maintaining specifications. An agent can modify public APIs, introduce new features, or alter component contracts, and the turn derives `turn_done` as long as the compiler passes. Consequently, documentation and specifications continuously drift behind implementation reality, violating codeNERD's core mandate: "spec and code never drift apart: a change that leaves them misaligned is not done."

## Decision

1. **Spec-First Completion Gate**:
   Introduce a mandatory spec-first verification arm into `turn_verified(Turn)`. A turn that changes the public surface or behavior of a spec-covered unit, or creates a new code unit, owes an accompanying specification update in the same change.
2. **Granular Obligation Thresholds**:
   - Public Surface / New Feature: Mandates updating the corresponding capability or requirement section in the specification unit.
   - Private Helper Modification: A private modification inside an already covered unit owes only a current-state touch when the specification explicitly cites the changed symbol.
3. **Mangle Derivation and Missing Evidence**:
   - The session executor asserts `turn_spec_updated(Turn, Unit)` when spec files are included in the turn's write set.
   - If an edit touches code without satisfying the spec update requirement, Mangle derives `turn_has_spec_misalignment(Turn)`.
   - Line 675 of `coder_safety.mg` is conjoined with `!turn_has_spec_misalignment(Turn)`.
   - When violated, `turn_missing_evidence(Turn, /spec_misaligned)` derives, signaling the executor to refuse turn completion.

## Consequences

- **Positive**: Guarantees that specifications and code remain continuously synchronized; prevents documentation decay; eliminates un-specced feature creep.
- **Negative**: Adds friction to coding turns by preventing hasty code-only edits; requires agents to understand the repository's spec format.
- **Risks**: False alarms on trivial refactors or comment fixes could frustrate developers. Mitigated by restricting the strict obligation to public surfaces and behavioral modifications.

## Witness

**Witness:** `test:TestSpecFirst_CompletionObligationEnforced`

| Claim | Witness (Code Evidence as of 2026-09-29) |
|---|---|
| Existing turn_done and turn_verified gates | `turn_done` and `turn_verified` in `internal/core/defaults/policy/coder_safety.mg:632-676`. |
| Missing evidence consumption in Go driver | `consumeTurnDoneSignal` in `internal/session/executor.go:2879-2915`. |
| Spec-first turn gate target test | Planned test `TestSpecFirst_CompletionObligationEnforced` in `internal/session/spec_gate_test.go`. |

## Status (Derived, Not Asserted)

**`accepted-not-implemented`**. Derivation: The architectural decision is accepted, but `turn_has_spec_misalignment` and `/spec_misaligned` missing evidence have not yet landed in production source code as of commit `bb7bafac`. Status flips to `implemented` once `TestSpecFirst_CompletionObligationEnforced` passes.
