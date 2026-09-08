# Campaign execution contracts

- Keep declared task effects through decomposition and rolling-wave refinement. Missing file targets require refinement; never convert an edit or test obligation into research to make a plan executable.
- `/test_run` uses VirtualStore host execution even with an explicit shard. Only successful execution against an unchanged workspace produces a snapshot-bound witness. Model prose is not test evidence.
- Verification tasks after edits depend on preceding edits. Preserve intentional reproducer-before-fix order.
- Cross-process pause uses a separate `.pause` control file. The owner cancels and joins workers and heartbeat before persisting a reusable paused checkpoint. Explicit resume clears the request; control clients never overwrite the owner snapshot.
- Run campaign package tests and the live pause/resume path when changing scheduling or lifecycle ownership.
- Tests that execute stages, complete phases, or sweep writes must place logs
  and checkpoint directories under `t.TempDir()`. Package-directory artifacts
  change the very workspace snapshot that campaign host checks must certify.
- Pathless campaign risk intelligence must scan the configured workspace, not
  the process working directory. Exercise this with different caller and target
  directories so a scan of the wrong project cannot pass unnoticed.
- Coverage and architecture-hint gathering consume the scanner's FileTopology;
  join that producer before reading its map. Keep independent collectors parallel.
