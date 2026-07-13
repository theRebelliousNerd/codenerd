<!-- SYNCED from corpus-build/references/common/anti-hallucination-gate.md sha256:68e3f9435b17 -- DO NOT EDIT -->

# Anti-Hallucination Gate

Treat architecture prose and prior agent reports as hypotheses until the live
tree confirms them.

Before using a symbol:

1. Verify Go types, functions, methods, and registrations with `rg` and read
   the owning implementation.
2. Verify Mangle predicate declarations, arity, and type bounds under
   `internal/core/defaults/schemas*.mg` and the relevant policy/rule files.
3. Verify action routes through the kernel, VirtualStore, shard/session, MCP, or
   CLI implementation that actually owns them.
4. Verify another worker's claimed addition in the diff before depending on it.
5. Verify a named algorithm by reading its computation, not its comment.

If evidence is unavailable, mark the claim `UNVERIFIED` in the completion
record. Do not ship placeholder code merely to make an assumption compile.
