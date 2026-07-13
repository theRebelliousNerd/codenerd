# persist — Corpus Rebuild Progress

## 2026-07-13 — Full rebuild (new document set)

- **Mode:** docs only; no Go/Mangle/test changes  
- **Source read:** `internal/persist/factsnap/factsnap.go` (+ all 4 test files)  
- **Reverse deps:** none in production `*.go`  
- **Replaced:** thin auto-inventory stubs with code-grounded narrative corpus  
- **Document set (new layout per SUBAGENT_INSTRUCTIONS):**

  - README.md  
  - IMPLEMENTED_SPEC.md (flagship)  
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

- **Key findings recorded:**
  1. Only subpackage is `factsnap` (no root `package persist`)  
  2. SimpleColumn + gzip/zstd + atomic rename fully implemented  
  3. Strong unit tests including 10k codec parity  
  4. Zero production importers (dormant wiring)  
  5. Intentional `atomToFact` fork vs `internal/core` (MangleAtom names)  

- **Obsolete thin filenames removed/superseded:**  
  `01-DOMAIN-MODEL.md`, `02-CURRENT-STATE-PERSIST.md`, `03-GAP-ANALYSIS-PERSIST.md`,  
  `04-INVARIANTS-AND-GATES.md`, `05-CROSS-SYSTEM-WIRING.md`, `06-TESTING-STRATEGY.md`,  
  `08-FAILURE-MODES.md` (content moved to numbered set above)
