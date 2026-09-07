# Subprocess ownership

- Use `NonInteractive` when a child has no interactive stdin. Windows null-device
  console probing can stall heavily concurrent test runs.
- Use `Cancellable` on `exec.CommandContext` before starting verification work.
  It bounds pipe waits and kills the process tree on cancellation. Keep cleanup
  bounded as well; do not spawn an unbounded taskkill helper.
