# _progress — config architecture corpus

| Date | Event |
|------|--------|
| 2026-07-13 | **Full rebuild** to cli/ quality bar per `Docs/architecture/_rebuild/SUBAGENT_INSTRUCTIONS.md`. Package `internal/config` inventoried (17 non-test Go, 5 tests, 0 mg). Replaced thin stubs with full document set: README, IMPLEMENTED_SPEC, 00–12, TODO, OPEN-QUESTIONS, progress. Covered UserConfig, engines, limits, load paths, dual YAML/JSON model, wiring, safety, testing, observability, failure modes. **No code changes.** |

## Canonical files (post-rebuild)

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

## Legacy names

Older thin stubs used alternate titles (e.g. `01-DOMAIN-MODEL.md`). Prefer canonical names above; legacy files may contain redirects.
