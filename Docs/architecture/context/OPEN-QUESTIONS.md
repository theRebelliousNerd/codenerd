# context — Open Questions

1. Which cross-package callers are load-bearing for `internal/context/` after the next major refactor?
2. Are there dormant integration points that look unused but are registration-driven?
3. Should any logic here move into Mangle policy vs stay in Go?
4. What observability category should own this package's critical path logs?
