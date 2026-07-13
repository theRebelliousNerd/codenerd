# campaign — Gap Analysis

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/campaign/` (complete internal coverage)
> **Implementation: `internal/campaign/` — 44 non-test .go, 29 tests, 1 .mg**


## Spec vs reality

| Area | Status | Notes |
|------|--------|-------|
| Package on disk | Yes | `internal/campaign/` |
| Source files | 44 | non-test .go |
| Tests | 29 | `*_test.go` |
| Types sampled | 40 | export scan |
| Mangle local | 1 | package `.mg` |
| Full architecture corpus | Yes | this directory |

## Gaps (gates, not calendar)

1. Deep behavioral deep-dives beyond inventory when package is under active evolution.
2. Wiring proof for any new public entrypoints (registration, VirtualStore, CLI).
3. Test gaps if test count << source count (currently 29 vs 44).
4. Docs/Spec 18-file product templates remain a separate `spec-doc-sprint` track.

## Non-gaps

Implementation exists under `internal/campaign/`; do not treat as pre-implementation 0%.
