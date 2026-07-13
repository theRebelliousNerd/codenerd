# testing — Corpus rebuild progress

## 2026-07-13 — Full architecture corpus rebuild

- **Mode:** Docs only (no Go/Mangle/code changes).  
- **Source researched:** `internal/testing/` (anchor) + `internal/testing/context_harness/**` + CLI `cmd/nerd/cmd_test_context.go`.  
- **Procedure followed:** `Docs/architecture/_rebuild/SUBAGENT_INSTRUCTIONS.md` full document set.  
- **Quality bar:** Code-grounded narrative (cli-depth), honest partials, path citations, mermaid/ASCII flows.  
- **Replaced:** Earlier thin/auto inventory stubs under `Docs/architecture/testing/`.  
- **Flagship:** `IMPLEMENTED_SPEC.md` deep-dive with status tables, scenario catalog, engine sketches, wiring, gaps pointer.  
- **Notable findings captured:**  
  - Dual-mode `ContextEngine` is real and CLI-wired.  
  - Advanced checkpoint validators typed but not enforced.  
  - `--category` flag dead; `GetScenario` missing feedback-learning.  
  - Enrichment-ratio semantics vs stale package README compression claims.  
  - Parent package is export-empty by design.

### Produced files

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
