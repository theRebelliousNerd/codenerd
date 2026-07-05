## 2024-07-05 - VirtualStore Interactive Gate vs Dreamer Cache Collision
**Learning:** The Dreamer's cache implementation in `internal/core/dreamer.go` uses `string(req.Type) + ":" + req.Target` as the cache key. This completely ignores the `req.Payload`. Two concurrent interactive tool calls (e.g. `write_file`) modifying the same file with different content will collide, potentially allowing a malicious payload to bypass safety checks by reusing the cache entry of a benign payload.
**Action:** When testing the VirtualStore ↔ Dreamer boundary, always construct concurrent races that exploit cache key collisions (same type + target, different payload).

## 2024-07-05 - VirtualStore Interactive Gate vs Dreamer Cache Collision
**Learning:** The Dreamer's cache implementation in `internal/core/dreamer.go` uses `string(req.Type) + ":" + req.Target` as the cache key. This completely ignores the `req.Payload`. Two concurrent interactive tool calls (e.g. `write_file`) modifying the same file with different content will collide, potentially allowing a malicious payload to bypass safety checks by reusing the cache entry of a benign payload.
**Action:** When testing the VirtualStore ↔ Dreamer boundary, always construct concurrent races that exploit cache key collisions (same type + target, different payload).
