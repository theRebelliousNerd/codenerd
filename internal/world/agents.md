# World identities

- Canonicalize the workspace once before scans, and resolve supplied paths
  against that identity before producing facts or cache keys. Fast, incremental
  and deep scans must give the same file the same label.
- Resolve all deep-scan inputs before launching workers; errors must not leave
  admitted work running after return. Test aliases and containment together.

- Filesystem and structure walks ask `internal/workspace.Membership` before
  descent and before file reads. Keep extension, secret-path and parsing filters
  local; never restore a private directory skip list or dot-directory allowlist.
- Default scanners read ignore patterns from the scanned workspace. Explicit
  scanner configurations use their supplied patterns; refresh on each scan tick.
