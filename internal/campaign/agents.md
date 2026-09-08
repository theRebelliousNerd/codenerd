# Campaign execution contracts

- Keep declared task effects through decomposition and rolling-wave refinement. Missing file targets require refinement; never convert an edit or test obligation into research to make a plan executable.
- `/test_run` uses VirtualStore host execution even with an explicit shard. Only successful execution against an unchanged workspace produces a snapshot-bound witness. Model prose is not test evidence.
- Verification tasks after edits depend on preceding edits. Preserve intentional reproducer-before-fix order.
- Cross-process pause uses a separate `.pause` control file. The owner cancels and joins workers and heartbeat before persisting a reusable paused checkpoint. Explicit resume clears the request; control clients never overwrite the owner snapshot.
- Run campaign package tests and the live pause/resume path when changing scheduling or lifecycle ownership.
