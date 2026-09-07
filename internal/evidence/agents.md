# Acceptance evidence

- This package supports a narrow Go source bug-fix contract with caller-owned,
  pre-existing tests. At least one exact named test must fail before edits;
  regression obligations must pass before edits. Compile errors and skipped or
  absent tests are not failing reproducers.
- Snapshot dirty and untracked files, names and modes. Reject non-regular files;
  exclude Git administration and `.nerd` runtime output while including project
  configuration and agent definitions.
- Pin contracts, test files, fixtures, module files, executable and environment.
  Changed verification inputs require a newly reviewed contract. Never let model
  prose supply a witness or weaken acceptance.
- Verify after the last edit, require stable before/after verification snapshots,
  and persist reports atomically under the contained `.nerd/evidence` directory.
- Tests must include broken behavior, stale evidence, absent/skipped tests and
  weakened acceptance controls. See `../../Docs/architecture/session/CHANGE_EVIDENCE.md`.
