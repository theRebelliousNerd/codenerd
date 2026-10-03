## 2026-10-03 - Autopoiesis Ouroboros Risk
**Learning:** The Autopoiesis subsystem is intended to synthesize rules from completed turns. If a synthesized rule introduces an infinite loop or contradicts base schemas, and RuleCourt fails to validate or enforce timeout, it can corrupt `learned.mg`, causing the Kernel to fail on the next instantiation. This is a critical cross-boundary failure path (Executor -> Autopoiesis -> Kernel -> System Boot).
**Action:** In integration tests, inject adversarial actions that trigger the synthesis of dangerous rules, and assert that RuleCourt strictly intercepts them, preserving the validity of the kernel instance and the system on disk.

## 2026-10-03 - VirtualStore Assertion Failure Cascades
**Learning:** The Session Executor expects VirtualStore to write to disk *and* assert a success fact into the Kernel. If VirtualStore executes the action but panics or fails before the Kernel assertion, the system state is inconsistent (side-effects happened, but no evidence was recorded). The session may retry or hang.
**Action:** Introduce partial-failure mocks or context cancellations exactly between the execution and the assertion steps in VirtualStore to test the Executor's ability to handle this inconsistency.
