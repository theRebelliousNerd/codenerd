# store — Gap Analysis

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/store/` (complete internal coverage)
> **Implementation: `internal/store/` — 39 non-test .go, 44 tests, 0 .mg**


## Spec vs reality

| Area | Status | Notes |
|------|--------|-------|
| Package on disk | Yes | `internal/store/` |
| Source files | 39 | non-test .go |
| Tests | 44 | `*_test.go` |
| Types sampled | 40 | export scan |
| Mangle local | 0 | package `.mg` |
| Full architecture corpus | Yes | this directory |

## Gaps (gates, not calendar)

1. Deep behavioral deep-dives beyond inventory when package is under active evolution.
2. Wiring proof for any new public entrypoints (registration, VirtualStore, CLI).
3. Test gaps if test count << source count (currently 44 vs 39).
4. Docs/Spec 18-file product templates remain a separate `spec-doc-sprint` track.

## Non-gaps

Implementation exists under `internal/store/`; do not treat as pre-implementation 0%.
