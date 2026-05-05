1. **Understand `internal/core/dream_plan.go`**: Analyze the module and its current tests (`internal/core/dream_plan_test.go`).
2. **Identify Test Gaps based on Vectors**:
   * Null/Undefined/Empty:
     * `GetNextPendingSubtask` with dependencies pointing to out-of-bounds indices.
     * `GetNextPendingSubtask` with self-referencing dependencies.
     * Methods receiving empty or non-existent `id` string (e.g. `MarkSubtaskRunning("")`).
   * Type Coercion: N/A, Go handles type safety.
   * User Request Extremes:
     * `Progress` method with massive amount of subtasks.
     * `DependsOn` containing thousands of indices causing potential performance bottleneck in `GetNextPendingSubtask`.
     * Very long `Hypothetical` strings or task descriptions during json serialization (although this file doesn't handle serialization, just holding the struct).
   * State Conflicts/Race Conditions:
     * All methods like `AddSubtask`, `MarkSubtask*`, `GetNextPendingSubtask`, `IsComplete`, `Progress` are currently NOT thread-safe (no mutex in `DreamPlan`). If multiple shards/goroutines update the plan concurrently, it will race and potentially panic (slice append).
3. **Write QA Journal**: Document findings in `.quality_assurance/2026-03-20_04-25-UTC_dream_plan_boundary_analysis.md` (minimum 400 lines).
4. **Insert `// TODO: TEST_GAP:` comments**: Add specific comments directly into `internal/core/dream_plan_test.go`.
5. **Request Review**: Once the QA file is written and comments are added, submit for review.
