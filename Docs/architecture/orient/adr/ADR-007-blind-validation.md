---
doc-class: governance
subsystem: orient
implementation-status: accepted-not-implemented
last-verified: 2026-09-29
verified-against: e056692c
supersedes: []
---

# ADR-007: The Blind-Orientation Acceptance Principle

## Context

A recurring failure mode in AI agent development is **overfitting to the development target**. When engineers and models develop an orientation system while testing against a single target codebase (such as an internal test repository or a motivating foreign benchmark), they inevitably introduce subtle, insidious shortcuts:
1. Special-casing specific directory names (e.g. adding hardcoded exclusions for a specific repository's data directory).
2. Authoring prompt templates that mention target tools, libraries, or architectures by name.
3. Adding keyword matchers tailored to the specific headings of the test repository's design files.

An orientation engine that passes tests by memorizing the target is a failure. It will degrade or crash when pointed at an arbitrary foreign repository in production.

## Decision

1. **Adoption of the Blind Acceptance Law**:
   The orientation subsystem must achieve acceptance on an unseen, genuine foreign repository whose specific directory names, file paths, documentation titles, and architectural findings are **never written into implementation briefs, prompt templates, or production Go code**.
2. **Prohibition of Name-Specific Logic**:
   Automated linters and regression checks must verify that no proprietary or target-specific repository identifiers exist in `internal/orient/` or `internal/init/`.
3. **Property-Based Verification**:
   Orientation acceptance criteria must assert abstract architectural properties, not specific string matches:
   - Does the orientation derived during `nerd init` (and refreshed by the OODA loop) hold at least one `/wave` era and zero negative timestamps?
   - Does the number of read candidates remain bounded within the configured budget?
   - Are seed datasets classified as `/seed_data` and treated as `/index_names_only`?
   - Are private dependencies detected without executing external HTTP lookups?
   - Is a non-empty, relationally consistent `WizardDocument` generated and stored?

## Consequences

- **Positive**: Guarantees true general-purpose autonomy across arbitrary foreign software projects; eliminates stenographic agent behavior and brittle heuristics.
- **Negative**: Requires rigorous, property-based testing and careful design of general statistical and structural rules.
- **Risks**: Ensuring general rules perform well across vastly different languages (e.g. C++ monorepos vs. Python microservices vs. TypeScript web apps) requires comprehensive evaluation across diverse synthetic fixtures.

## Witness

**Witness:** `test:TestBlindOrientation_AcceptanceSuite`

| Claim | Witness (Code Evidence as of 2026-09-29) |
|---|---|
| Repository guidance mandates generalization | `CLAUDE.md:35-41` ("It knows the codebase better than any other coding agent..."). |
| Hardcoded name exclusions present today | `internal/world/fs.go:226-238` (hardcoded 6-directory name list). |
| Proving regression test | `TestBlindOrientation_AcceptanceSuite` running `nerd init` against a multi-language, multi-era synthetic repository with zero hardcoded name matching. |

## Status (Derived, Not Asserted)

**`accepted-not-implemented`**. Derivation: The decision is approved, but the complete acceptance test suite (`TestBlindOrientation_AcceptanceSuite`) has not yet been authored in commit `e056692c`. Status flips to `implemented` once the acceptance suite passes.
