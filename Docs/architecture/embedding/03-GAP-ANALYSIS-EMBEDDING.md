# embedding — Gap Analysis

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/embedding/` (complete internal coverage)
> **Implementation: `internal/embedding/` — 6 non-test .go, 7 tests, 0 .mg**


## Spec vs reality

| Area | Status | Notes |
|------|--------|-------|
| Package on disk | Yes | `internal/embedding/` |
| Source files | 6 | non-test .go |
| Tests | 7 | `*_test.go` |
| Types sampled | 9 | export scan |
| Mangle local | 0 | package `.mg` |
| Full architecture corpus | Yes | this directory |

## Gaps (gates, not calendar)

1. Deep behavioral deep-dives beyond inventory when package is under active evolution.
2. Wiring proof for any new public entrypoints (registration, VirtualStore, CLI).
3. Test gaps if test count << source count (currently 7 vs 6).
4. Docs/Spec 18-file product templates remain a separate `spec-doc-sprint` track.

## Non-gaps

Implementation exists under `internal/embedding/`; do not treat as pre-implementation 0%.
