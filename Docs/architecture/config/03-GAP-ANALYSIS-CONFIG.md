# config — Gap Analysis

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/config/` (complete internal coverage)
> **Implementation: `internal/config/` — 17 non-test .go, 5 tests, 0 .mg**


## Spec vs reality

| Area | Status | Notes |
|------|--------|-------|
| Package on disk | Yes | `internal/config/` |
| Source files | 17 | non-test .go |
| Tests | 5 | `*_test.go` |
| Types sampled | 33 | export scan |
| Mangle local | 0 | package `.mg` |
| Full architecture corpus | Yes | this directory |

## Gaps (gates, not calendar)

1. Deep behavioral deep-dives beyond inventory when package is under active evolution.
2. Wiring proof for any new public entrypoints (registration, VirtualStore, CLI).
3. Test gaps if test count << source count (currently 5 vs 17).
4. Docs/Spec 18-file product templates remain a separate `spec-doc-sprint` track.

## Non-gaps

Implementation exists under `internal/config/`; do not treat as pre-implementation 0%.
