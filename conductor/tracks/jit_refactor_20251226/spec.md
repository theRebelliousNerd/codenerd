# Specification: JIT-Driven Universal Agent Refactor

## Overview
This track aims to fundamentally refactor the codeNERD architecture by replacing the rigid, hard-coded `Shard` agents with a JIT-driven **Universal Executor**. This shift will move agent behavior from Go code to Mangle logic and JIT prompt/config atoms, resulting in a cleaner, more adaptable, and lower-boilerplate codebase.

## Goals
- **Clean Slate:** Eliminate the `Shard` interface and hard-coded agent structs.
- **Dynamic Instantiation:** Create agents on-the-fly based on JIT-compiled configurations.
- **Boilerplate Reduction:** Centralize execution logic in a single `UniversalExecutor` powered by `VirtualStore`.
- **Logic-First Behavior:** Define agent specialized knowledge and workflows purely in Mangle (`.mg` files).

## Functional Requirements
- **Universal Executor:** Implement a core execution loop that consumes a `JITAgentConfig` and interacts directly with `VirtualStore` for tool/action execution.
- **Structured JIT Configuration:** Extend the JIT Prompt Compiler to output a structured schema (JSON/YAML) containing identity prompts, allowed toolsets, and required Mangle policies.
- **Virtual Predicate Bridge:** Implement virtual predicates (e.g., `query_graph`) to allow Mangle policies to query the World Model (dependency graph, code elements) on demand.
- **Dynamic Orchestration:** Update the `CampaignOrchestrator` to spawn these dynamic agents based on intent rather than pre-registered names.

## Non-Functional Requirements
- **Performance:** Measure and achieve a noticeable performance gain (reduced memory footprint/execution overhead).
- **Stability:** Maintain a 100% pass rate on existing integration tests post-refactor.
- **Idiomatic Go:** Ensure the new architecture follows Go best practices and minimizes "magic" outside of the logic engine.

## Acceptance Criteria
- [ ] All `Shard` interfaces are removed from `internal/core/`.
- [ ] `internal/shards/coder`, `tester`, and `reviewer` directories are successfully purged.
- [ ] JIT compiler produces valid `JITAgentConfig` objects for at least 3 agent types.
- [ ] Performance benchmarks demonstrate reduced boilerplate and memory usage.
- [ ] Full TDD repair loop and Campaign execution are verified as functional under the new model.

## Out of Scope
- Rewriting the Mangle logic engine itself.
- Changes to the frontend/CLI UI components (unless necessary for debugging the refactor).
