# World identities

- Canonicalize the workspace once before scans, and resolve supplied paths
  against that identity before producing facts or cache keys. Fast, incremental
  and deep scans must give the same file the same label.
- Resolve all deep-scan inputs before launching workers; errors must not leave
  admitted work running after return. Test aliases and containment together.
