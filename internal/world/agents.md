# World identities

- Canonicalize the workspace once before scans, and resolve supplied paths
  against that identity before producing facts or cache keys. Fast, incremental
  and deep scans must give the same file the same label.
- Resolve all deep-scan inputs before launching workers; errors must not leave
  admitted work running after return. Test aliases and containment together.

- Stage a complete filesystem generation before retiring database rows or
  publishing cache entries. The full fallback used for empty, missing, invalid
  or null manifests has the same contract as an incremental refresh.
- Propagate walk, admitted-file read/hash and cancellation failures only after
  joining admitted workers. Failed scans publish neither partial results nor
  a replacement manifest; syntax-parser best effort is a separate contract.
- Regression gates must seed old database/cache generations, inject failure
  during the second census as well as the first, and retain successful deletion
  and canonical migration controls. A successful enumeration alone is not a
  successful full scan.

- Filesystem and structure walks ask `internal/workspace.Membership` before
  descent and before file reads. Keep extension, secret-path and parsing filters
  local; never restore a private directory skip list or dot-directory allowlist.
- Default scanners read ignore patterns from the scanned workspace. Explicit
  scanner configurations use their supplied patterns; refresh on each scan tick.
