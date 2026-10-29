# Subprocess ownership

- Use `NonInteractive` when a child has no interactive stdin. Windows null-device
  console probing can stall heavily concurrent test runs.
- Run `exec.CommandContext` commands through `Run` or `CombinedOutput`, never `cmd.Run`/`cmd.CombinedOutput`: they own Start so the child can join a kill scope (a job object on Windows, a process group elsewhere), bound pipe waits, and kill the whole tree on cancellation. Keep cleanup bounded as well; do not spawn an unbounded taskkill helper.
