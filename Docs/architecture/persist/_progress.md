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

## 2026-08-15 — First production caller (code + docs)

- **Mode:** code + docs. The P0 chain in TODO.md was executed rather than re-recommended.
- **Decision (OPEN-QUESTIONS Q1):** kernel debug export is the first caller,
  surfaced as `nerd snapshot export|import|list` (`cmd/nerd/cmd_snapshot.go`).
  Chosen over campaign/world because it boots its own kernel locally — no API
  key, no network, no shards — and nothing it writes is read back automatically.
- **New package:** `internal/persist/snapshot` owns `.nerd/snapshots/`, name
  sanitisation, bare-name resolution, listing and predicate summaries, so the
  CLI never invents paths. `internal/persist/doc.go` explains the split.
- **factsnap hardening:**
  - magic-byte sniffing (gzip `1f 8b`, zstd `28 b5 2f fd`); content beats a
    suffix that a rename made wrong (Q4 resolved)
  - `.sha256` sidecar in sha256sum(1) format, verified on read, `ErrIntegrity`
    on mismatch, stale sidecars cleared on `NoSidecar` writes
  - atomic write hardened: unique temp per call (a shared `<path>.tmp` let two
    writers interleave into one file and rename the mixture over a good
    snapshot), explicit `chmod 0644`, directory fsync, per-path write lock so
    data and digest always agree
  - `logging.CategoryStore` debug lines with codec, bytes, digest prefix and
    duration; warn on sidecar failure and codec disagreement
- **NameType divergence (Q5):** documented at `factsnap.baseTermToValue` with the
  reason unification is a core consumer migration, and pinned by
  `TestNameConstant_WhenRoundTripped_ShouldStayMangleAtom`. Left unresolved
  deliberately; `internal/core` was out of scope for this pass.
- **New finding recorded as Q8:** whole-valued floats (2.0, 0.0) come back as
  `int64` because mangle-go renders `Float64(2.0)` as the text `2`. Fractional
  values survive. Pinned by
  `TestFloat_WhenRoundTrippedTwice_ShouldBeStableAfterFirstHop`.
- **Tests added:** 11 in `factsnap_robustness_test.go`, 9 in
  `snapshot_test.go`, 2 in `kernel_roundtrip_test.go` (real kernel on both
  ends), 7 in `cmd/nerd/cmd_snapshot_test.go`.
- **Docs updated:** README, IMPLEMENTED_SPEC, 02, 03, 06, 07, 08, 09, 10, 11,
  TODO, OPEN-QUESTIONS, this journal.
