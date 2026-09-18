# S15 — Nothing in the window is cut silently

## Status

last updated: 2026-09-18 (in progress — inventory)

seam: S15. branch: `worktree-agent-a0fe46d265c393352`, based on `dogfood/c2-closure` @ `9443b8a0`.

The rule: model- or user-bound content is never silently shortened. Where content must
leave the window it is either (a) **restated** — handed back to the model to be said again
within the limit (the broker's compression path), or (b) **elided** with the pipeline's own
marker (`[codenerd: truncated`, `internal/prompt/limits.go:32`) naming what was dropped —
count and kind — and, where the dropped content still exists, a handle that recovers it.
A count cap that drops content with no marker is the defect.

done:
- (pending)

open:
- (pending)

---

## Inventory of every cut

(pending — derived by grep, not by copying the study)

## Design

(pending)

## Changes

(pending)

## Tests

(pending)

## Full test run

(pending)

## Open

(pending)
