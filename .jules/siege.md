## 2024-12-28 - VirtualStore Dreamer Cache Collision
**Learning:** The `DreamCache` key design uses only `ActionType + Target` (e.g., `write_file:config.json`), completely ignoring the `Payload`. This creates a catastrophic semantic bypass where a benign write caches a 'Safe' verdict, allowing a subsequent malicious write to the same target to bypass the kernel safety gate entirely.
**Action:** Always include a hash of the payload in the cache key for stateful safety gates, or explicitly separate caching mechanisms from semantic policy evaluations.

## 2024-12-28 - TOCTOU in ActionRequest Generation
**Learning:** The `SessionExecutor` passes its `args` map to the `VirtualStore` by reference. The `buildInteractiveActionRequest` function does a shallow copy. If the arguments are modified by another goroutine while the `Dreamer` is blocking on the ~277KB kernel clone, the safety check evaluates the old state but the executor uses the new state.
**Action:** Always perform deep copies or serialization bounds when passing mutable payloads across security boundaries where one side performs asynchronous or blocking evaluations.
