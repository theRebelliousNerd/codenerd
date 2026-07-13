# Shard maintenance guidance

- Treat `DefaultShardPredicateManifests` as the single production ownership
  source; `internal/system/factory.go` must consume it rather than duplicate it.
- Batch consultation returns successful responses in requested order and joins
  every specialist failure. Never turn partial or total failure into `nil` error.
- Observer managers are restartable generations. Each `Start` owns a fresh
  context; `Stop` waits first for event producers and then admitted tasks, drains
  stale events, and remains idempotent.
- Every terminal router outcome consumes the `permitted_action` and its unary
  marker by action ID. Autopoiesis recording is not successful execution and
  must not leave an action to amplify on later ticks.
- Keep constitutional permission in the policy shard and execution in tactile.
  Run package tests plus `-race` after lifecycle, consultation, or routing edits.
