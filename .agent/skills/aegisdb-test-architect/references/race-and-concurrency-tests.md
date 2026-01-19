# Race and Concurrency Tests

## Purpose
Expose data races, deadlocks, and incorrect concurrent behavior.

## Techniques
- Run tests with -race.
- Use WaitGroup, context cancellation, and channels with timeouts.
- Use t.Parallel carefully; avoid shared mutable globals.
- Exercise concurrent calls with goroutines.

## Patterns
- Start N goroutines calling the same API.
- Use timeouts to prevent hangs.
- Use atomic counters for invariants.

## AegisDB focus areas
- Cache L1 and L2 concurrency
- Wormhole engine scoring and cache updates
- Graph traversal with concurrent queries
- Protocol telemetry broadcast loops

## Pitfalls
- Avoid time.Sleep as synchronization.
- Ensure goroutines exit; use t.Cleanup and context cancel.
