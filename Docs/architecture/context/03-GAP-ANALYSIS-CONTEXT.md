# context — Gap Analysis

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/context/` (complete internal coverage)
> **Implementation: `internal/context/` — 9 non-test .go, 11 tests, 1 .mg**


## Spec vs reality

| Area | Status | Notes |
|------|--------|-------|
| Package on disk | Yes | `internal/context/` |
| Source files | 9 | non-test .go |
| Tests | 11 | `*_test.go` |
| Types sampled | 23 | export scan |
| Mangle local | 1 | package `.mg` |
| Full architecture corpus | Yes | this directory |

## Gaps (gates, not calendar)

1. Deep behavioral deep-dives beyond inventory when package is under active evolution.
2. Wiring proof for any new public entrypoints (registration, VirtualStore, CLI).
3. Test gaps if test count << source count (currently 11 vs 9).
4. Docs/Spec 18-file product templates remain a separate `spec-doc-sprint` track.

## Non-gaps

Implementation exists under `internal/context/`; do not treat as pre-implementation 0%.
