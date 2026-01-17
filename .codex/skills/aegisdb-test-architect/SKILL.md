---
name: aegisdb-test-architect
description: Design and implement Go test suites for AegisDB, covering unit, integration, fuzz, benchmark, race, and protocol conformance tests. Use when creating or improving AegisDB test coverage, defining test strategy, or reviewing tests for completeness, correctness, and LLM failure modes.
---

# AegisDB Test Architect

## Overview

Design production-grade Go tests for AegisDB with full coverage of success paths, error paths, edge cases, concurrency, and performance. The goal is a complete test system, not minimal test coverage.

## References

Read `references/go-testing-best-practices-and-llm-pitfalls.md` before drafting new tests or a test plan.

Read the specific guide for each test type:
- Unit: `references/unit-tests.md`
- Integration: `references/integration-tests.md`
- Fuzz: `references/fuzz-tests.md`
- Benchmark: `references/benchmark-tests.md`
- Race and concurrency: `references/race-and-concurrency-tests.md`
- Protocol conformance: `references/protocol-conformance-tests.md`

## Workflow

1. Scope the change and risks. Identify impacted subsystems, external boundaries, and failure modes.
2. Pattern search. Find existing tests and helpers, then match naming, structure, and setup/teardown.
3. Build a test portfolio. Select every applicable test type, do not skip a category.
4. Draft tests with strict assertions and deterministic setup. Never soften tests to pass.
5. Review for LLM pitfalls. No hardcoded outputs, no removed assertions, no time.Sleep hacks.
6. Deliver the plan and tests. Provide file paths, scenarios, and rationale.

## Test portfolio checklist (Go only)

- Unit tests: table-driven, subtests, validate error paths and edge cases, prefer black box for public APIs, use white box only when necessary.
- Integration tests: use build tags `//go:build integration`, keep external dependencies explicit, isolate state, use cleanup.
- Fuzz tests: add `FuzzXxx` for parsers and untrusted inputs, seed with realistic corpus data.
- Benchmarks: add `BenchmarkXxx` with sub-benchmarks, report allocations, vary input sizes.
- Race and concurrency tests: exercise goroutines, locks, and channels; enforce cleanup and context cancellation.
- Protocol conformance tests: validate REST, gRPC, MCP, and A2A handlers as Go tests using canonical request and response contracts.

## Quality gates and anti-cheat rules

- Tests must fail for known bad inputs and pass for valid inputs.
- Never change tests to make code pass unless the test is demonstrably wrong and the user explicitly approves.
- No hardcoded test-only logic in production code.
- Avoid fragile timing. Use deterministic clocks, contexts, and wait groups.
- Ensure every error path has an assertion and context-rich error expectations.

## Output expectations

When asked to add or improve tests, respond with:
- Proposed test files and target packages.
- Exact scenarios covered (happy path, error path, edge cases, concurrency).
- Required test data, fixtures, or helpers.
- Follow-up coverage gaps and risks.

## Repository notes

- Go only. Do not add tests in other languages.
- Do not run tests unless explicitly requested.
