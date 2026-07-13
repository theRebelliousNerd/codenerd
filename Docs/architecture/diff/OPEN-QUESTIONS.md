# OPEN QUESTIONS — `internal/diff`

> Last verified against codebase: 2026-07-13

## Q1 — Should cache hits deep-copy always, or only when requested?

Deep copy protects correctness; shallow copy is faster for pure render.  
**Trade-off:** approve/reject UI rarely mutates hunks, so shallow may be fine if documented
as read-only. Mutation-by-accident remains a footgun.

## Q2 — Should `DefaultEngine` exist at all?

Global cache is convenient for `ComputeDiff` and `CreateDiffFromStrings`, but couples
unrelated call sites. Alternatives: remove package-level function, or make it
cache-disabled.

## Q3 — Expose timeout outcomes?

If sergi returns partial results on timeout, should `FileDiff` grow `TimedOut bool`?
Callers could surface a warning in DiffApprovalView.

## Q4 — Expand binary detection beyond NUL?

Options: MIME sniff, printable ratio, size caps. Risk of false positives on UTF-16 source
or intentional binary-as-string workflows.

## Q5 — First-class multi-file patch type?

`PendingMutation` list already exists in UI. A package-level `MultiFileDiff` might help
CLI non-interactive reports — or might be premature until a second consumer appears.

## Q6 — Assert structured diff facts into Mangle?

Could enable logic queries over “files changed,” hunk counts, etc.  
**Counter:** fuzzy/large NL pattern banks are anti-pattern in Mangle; only structured
summaries after compute. Not required for current TUI path.

## Q7 — Who owns whitespace-ignore semantics?

Today: UI `filterHunkLines`. Could move into engine option for consistency across
future consumers. Prefer keeping presentation filters in UI unless a second consumer needs
identical behavior.

## Q8 — Is `LineHeader` forever unused?

Either engine starts emitting headers (unlikely) or const remains UI-only alias ballast.
Decide in a future API cleanup.
