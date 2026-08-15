# Progress — `Docs/architecture/diff`

| Date | Change |
|------|--------|
| 2026-08-15 | Backlog closeout. Cache key widened to two hashes + length per side, with opt-in exact content verification (`Options.VerifyCacheContent`) and a `Stats.Collisions` counter. Word-level API returns codeNERD `WordSpan` instead of `diffmatchpatch.Diff`, and `cmd/nerd/ui` now actually paints the highlights (segment plan + scroll-aware windowing) instead of ignoring an `any`. `LineHeader` decided as a UI-owned enum member with a test enforcing the engine never emits it. `CreateDiffFromStrings` and every `DiffApprovalView` moved onto one `ui.uiDiffEngine`, ending the dual-cache surprise. Benchmarks added plus a CI smoke test that runs them all in the normal test pass. TODO.md fully closed. |
| 2026-07-13 | Full architecture corpus **rebuilt** to cli quality bar (SUBAGENT_INSTRUCTIONS). Replaced thin auto-inventory stubs with code-grounded docs for `internal/diff/` (1 src ≈379 LOC, 2 tests ≈949 LOC). Flagship `IMPLEMENTED_SPEC.md`; full 00–12 set; reverse-dep evidence via `cmd/nerd/ui/diffview.go`. |

## Corpus files (required set)

- README.md
- IMPLEMENTED_SPEC.md
- 00-ALIGNMENT-VISION-REVIEW.md
- 01-VISION.md
- 02-CURRENT-STATE.md
- 03-GAP-ANALYSIS.md
- 04-ARCHITECTURAL-PRINCIPLES.md
- 05-INTERNAL-ARCHITECTURE.md
- 06-PUBLIC-API-AND-TYPES.md
- 07-DEPENDENCY-MAP.md
- 08-WIRING-AND-INTEGRATION.md
- 09-SAFETY-AND-INVARIANTS.md
- 10-TESTING-ALIGNMENT.md
- 11-OBSERVABILITY.md
- 12-FAILURE-MODES.md
- TODO.md
- OPEN-QUESTIONS.md
- _progress.md

## Superseded stub names (removed if present)

Prior thin files used different numbering (`01-DOMAIN-MODEL`, `02-CURRENT-STATE-DIFF`,
`04-INVARIANTS-AND-GATES`, etc.). Rebuild targets the contract names above.
