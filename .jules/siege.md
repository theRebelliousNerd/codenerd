## 2026-10-03 - Autopoiesis Ouroboros Risk
**Learning:** The Autopoiesis subsystem is intended to synthesize rules from completed turns. If a synthesized rule introduces an infinite loop or contradicts base schemas, and RuleCourt fails to validate or enforce timeout, it can corrupt `learned.mg`, causing the Kernel to fail on the next instantiation. This is a critical cross-boundary failure path (Executor -> Autopoiesis -> Kernel -> System Boot).
**Action:** In integration tests, inject adversarial actions that trigger the synthesis of dangerous rules, and assert that RuleCourt strictly intercepts them, preserving the validity of the kernel instance and the system on disk.

## 2026-10-03 - VirtualStore Assertion Failure Cascades
**Learning:** The Session Executor expects VirtualStore to write to disk *and* assert a success fact into the Kernel. If VirtualStore executes the action but panics or fails before the Kernel assertion, the system state is inconsistent (side-effects happened, but no evidence was recorded). The session may retry or hang.
**Action:** Introduce partial-failure mocks or context cancellations exactly between the execution and the assertion steps in VirtualStore to test the Executor's ability to handle this inconsistency.

## 2026-11-03 - Perception Transducer Semantic Type Mismatch
**Learning:** The Perception Transducer translates natural language into Mangle atoms. However, if adversarial input causes it to generate valid Mangle syntax that uses incorrect types (e.g. stringly typed instead of ast.Name), the kernel silently produces empty results instead of an error, breaking the intent pipeline without triggering fallback.
**Action:** When testing Perception to Kernel integration, assert that malformed types cause proper rejections rather than silent empty result sets.

## 2024-12-01 - ShardManager/JITExecutor Boundary Implicit Fallback
**Learning:** The ShardManager implicitly relies on the JITExecutor (via TaskDelegator) for any shard type it lacks a factory for. When testing this boundary, passing execution through an intermediary requires precise error translation and state management. The failure of JIT compilation or the generation of empty strings (the "BaseShardAgent" bug) must be explicitly managed by both components.
**Action:** In E2E tests, verify that `ShardStateCompleted` is only set when the delegator produces tangible work, and that temporal failures (context timeouts) are respected across the boundary.
