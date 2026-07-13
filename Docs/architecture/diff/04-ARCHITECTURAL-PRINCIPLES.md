# 04 — Architectural Principles: `internal/diff`

> Last verified against codebase: 2026-07-13  
> These principles are **binding** for changes to `internal/diff/`. Violations require explicit
> corpus + design rationale updates.

## P1 — Pure functions of content strings

The package **must not** open files, shells, network, or Mangle kernels. Callers own I/O.
Paths are labels, not open targets.

**Evidence:** `ComputeDiff` signature is strings-only (`diff.go`).

## P2 — Structured output over raw patch text

Primary product is `*FileDiff` with typed lines/hunks, not a `[]byte` unified-diff blob.
Rendering formats are consumer concerns.

## P3 — Bound pathological cost

Every public compute path must remain time-bounded. Today: `dmp.DiffTimeout = 5s`.
Do not remove the timeout without a replacement bound (chunking, size cap, or async cancel).

## P4 — Refuse binary through Myers

NUL (`0x00`) is the conventional binary sentinel. Prefer empty hunks + `IsBinary` over
garbage line ops. Do not silently strip NULs to “force text.”

## P5 — Clamp caller extremes

`contextLines` and similar knobs must be clamped (`[0, maxContextLines]`). Never trust
fuzzers or future option structs without bounds.

## P6 — Cache is an optimization, not truth

Cache keys may be approximate (hashes). Correctness must not depend on cache presence.
`ClearCache` must remain available. Prefer deep clones on hit over shared mutable slices
(current shallow copy is a known debt — do not spread the pattern to new caches).

## P7 — Prefer local engines for isolation

`DefaultEngine` is convenient for one-shot helpers; long-lived TUI components should prefer
`NewEngine()` (as `DiffApprovalView` does) so cache lifetime matches the component.

## P8 — Stay out of the executive path

Do not assert `permitted(...)`, do not route VirtualStore actions, do not embed policy.
Diff **describes**; kernel **decides**.

## P9 — Minimize third-party type leakage

Avoid expanding the public API surface that re-exports sergi types. New exports should use
codeNERD-owned structs where practical (`ComputeWordLevelDiff` is legacy leakage).

## P10 — Test edge cases next to the algorithm

Binary, empty/empty, concurrency, huge context, and cache rewrite belong in-package
(`diff_test.go`, `diff_comprehensive_test.go`), not only in UI tests.

## P11 — Single production file discipline (until scale forces split)

While the package remains ~400 lines, keep the pipeline colocated. Split only when a second
concern (e.g. patch parse/emit) arrives — not for cosmetic file counts.

## P12 — No sibling-platform / app-specific semantics

No foreign-product-surface, foreign-agent-kit, campaign-specific fields, or app-only predicates. General-use
text diff only.
