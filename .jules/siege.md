## 2024-05-24 - Dreamer Clone Boundary Isolation
**Learning:** The boundary between the `Dreamer` and `RealKernel` during `SimulateAction` heavily depends on the assumption that `kernel.Clone()` provides absolute isolation. If `Clone()` does a shallow copy of fact argument slices, mutations during sandbox evaluation can silently corrupt the parent kernel's state. Furthermore, context cancellation is not natively supported by Mangle's `Evaluate()` loop, leading to potential fixpoint deadlocks.
**Action:** Always ensure that boundary tests explicitly verify state mutations in the parent kernel after a sandbox failure, and always include context cancellation tests to expose uninterruptible goroutines at FFI boundaries.

## 2024-05-26 - Orchestrator and Executor Implicit Contracts
**Learning:** The `Campaign Orchestrator` delegating tasks to the `Session Executor` heavily relies on the assumption that tasks are completely synchronous and error boundaries cleanly delineate task failure vs system failure. If the Executor swallows panic-level events or times out silently, the orchestrator hangs indefinitely on `WaitForResult` or repeats the same phase indefinitely.
**Action:** Implement tests that explicitly force the Executor to panic or timeout mid-task, and verify the Orchestrator safely marks the Phase as failed or triggers replanning instead of hanging.

## 2024-05-26 - VirtualStore FFI and Mangle Type Mismatches
**Learning:** When the `Session Executor` requests tool execution via the `VirtualStore`, the `VirtualStore` must translate tool arguments into Mangle Atoms to assert facts in the kernel. If a tool argument is passed as a generic string instead of a strict Mangle `ast.String` or `ast.Name`, the kernel join silently fails (returns 0 results) rather than erroring out, leading the Executor to believe an action is permitted when it isn't (or vice versa).
**Action:** Write boundary tests that explicitly inject Mangle type confusion (e.g., passing `/string` instead of `"string"`) at the VirtualStore FFI layer to ensure the type-checker catches the error before policy evaluation.
