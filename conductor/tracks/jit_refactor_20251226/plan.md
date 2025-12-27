# Implementation Plan: JIT-Driven Universal Agent Refactor

## Phase 1: JIT Configuration Engine
- [x] Task: Define Configuration Schema & Types
    - [x] Sub-task: Create `internal/jit/config/types.go` defining `AgentConfig`, `ToolSet`, `PolicySet`.
    - [x] Sub-task: Implement strict validation logic for `AgentConfig`.
- [ ] Task: JIT Compiler Extension (Config Layer)
    - [ ] Sub-task: Refactor `Compiler` to support multi-modal output (Prompt String + Config Object).
    - [ ] Sub-task: Implement `ConfigAtom` registry for mapping intents to tool/policy sets.
    - [ ] Sub-task: Create logic for resolving conflicting config atoms (priority/merge rules).
    - [ ] Sub-task: Unit Test: Config generation for 'Coder', 'Tester', 'Reviewer' intents.
    - [ ] Sub-task: Unit Test: Config generation for complex/hybrid intents.
- [ ] Task: Conductor - User Manual Verification 'JIT Configuration Engine' (Protocol in workflow.md)

## Phase 2: Mangle-World Bridge (Data Access)
- [ ] Task: World Model Interface Exposure
    - [ ] Sub-task: Audit `internal/world` for public API surface area required by Logic.
    - [ ] Sub-task: Define `GraphQuery` interface for consistent access to AST, Dependency Graph, and File Topology.
- [ ] Task: Virtual Predicate Implementation
    - [ ] Sub-task: Implement `query_graph/3` predicate (QueryType, Params, Result).
    - [ ] Sub-task: Implement `query_symbol/3` predicate (SymbolName, Type, Location).
    - [ ] Sub-task: Implement `query_path/3` predicate (Source, Target, Path).
    - [ ] Sub-task: Integration Test: Verify Mangle queries return correct Go struct data from World.
- [ ] Task: Conductor - User Manual Verification 'Mangle-World Bridge' (Protocol in workflow.md)

## Phase 3: The Universal Executor (Core Loop)
- [ ] Task: Universal Executor Scaffold
    - [ ] Sub-task: Create `internal/executor/universal.go`.
    - [ ] Sub-task: Implement `NewExecutor(config)` factory.
    - [ ] Sub-task: Wire `VirtualStore` and `RealKernel` directly into Executor.
- [ ] Task: Dynamic Policy Loading
    - [ ] Sub-task: Implement logic to hot-load Mangle policies from Config.
    - [ ] Sub-task: Implement policy verification (ensure all required predicates exist).
- [ ] Task: The Execution Loop (OODA)
    - [ ] Sub-task: Implement the `Observe` step (Transducer integration).
    - [ ] Sub-task: Implement the `Orient` step (Spreading Activation via Mangle).
    - [ ] Sub-task: Implement the `Decide` step (Query `next_action`).
    - [ ] Sub-task: Implement the `Act` step (Route to `VirtualStore`).
- [ ] Task: Telemetry & Safety
    - [ ] Sub-task: Integrate `ConstitutionalGate` directly into the loop.
    - [ ] Sub-task: Implement detailed structured logging for the universal loop.
- [ ] Task: Conductor - User Manual Verification 'The Universal Executor' (Protocol in workflow.md)

## Phase 4: Autopoiesis & Learning Integration
- [ ] Task: Learning Interface Refactor
    - [ ] Sub-task: Create `internal/learning/bridge.go` to expose learning as a service (not a side-effect).
    - [ ] Sub-task: Implement `trigger_learning` virtual predicate (Mangle -> LearningStore).
    - [ ] Sub-task: Implement `feedback_loop` virtual predicate (Mangle -> Autopoiesis).
- [ ] Task: Mangle-Driven Learning
    - [ ] Sub-task: Create `learning_policy.mg` to define WHEN to trigger learning (e.g., "if action failed 3 times").
    - [ ] Sub-task: Update Universal Executor to respect learning predicates.
- [ ] Task: Conductor - User Manual Verification 'Autopoiesis & Learning Integration' (Protocol in workflow.md)

## Phase 5: Initialization & Persistent Shard Handling
- [ ] Task: Init Logic Refactor
    - [ ] Sub-task: Audit `internal/init/initializer.go` for legacy Shard dependencies.
    - [ ] Sub-task: Update `/init` to generate JIT Configs for Type 3 (Persistent) agents instead of Shard structs.
    - [ ] Sub-task: Ensure "Knowledge Base" paths are correctly passed in JIT Config.
- [ ] Task: Long-Running Agent Support
    - [ ] Sub-task: Update Universal Executor to support "Daemon Mode" (for persistent agents).
    - [ ] Sub-task: Verify `internal/init` can successfully spawn a Universal-backed "Researcher" agent.
- [ ] Task: Conductor - User Manual Verification 'Initialization & Persistent Shard Handling' (Protocol in workflow.md)

## Phase 6: Orchestration & The Big Bang
- [ ] Task: Orchestrator Refactor
    - [ ] Sub-task: Update `orchestrator_task_handlers.go` to instantiate `UniversalExecutor`.
    - [ ] Sub-task: Implement `Intent -> JIT Config -> Executor` pipeline in `Run()` loop.
- [ ] Task: System-Wide Verification
    - [ ] Sub-task: Run existing integration test suite (must pass 100%).
    - [ ] Sub-task: Perform manual "Smoke Tests" on key workflows (Refactor, Test, Review).
    - [ ] Sub-task: Benchmark: Startup time, Memory usage, Token usage.
- [ ] Task: The Purge (Cleanup)
    - [ ] Sub-task: Delete `internal/shards/coder`.
    - [ ] Sub-task: Delete `internal/shards/tester`.
    - [ ] Sub-task: Delete `internal/shards/reviewer`.
    - [ ] Sub-task: Remove `Shard` interface and legacy factories.
- [ ] Task: Conductor - User Manual Verification 'Orchestration & The Big Bang' (Protocol in workflow.md)
