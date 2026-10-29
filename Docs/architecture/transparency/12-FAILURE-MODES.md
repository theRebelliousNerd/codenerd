# transparency — Failure Modes

> Last verified: 2026-07-13

## FM1 — Event drop under load

**Symptom:** Missing Glass Box or tool lines during multi-shard tool storms.  
**Cause:** Subscriber channel full; `select default` drops. Tool buffer 50; Glass Box sub buffer 512.  
**Impact:** Incomplete live picture; executive path continues.  
**Mitigation:** Drain faster in chat (`maxGlassBoxDrain`); increase buffers carefully; prefer batching when not verbose; add drop counters.  
**Detection:** Hard today without Stats.Drops; user reports “skipped lines”.

## FM2 — Split-brain visibility (Observer vs Glass Box)

**Symptom:** `/transparency` active ops empty while Glass Box shows spawns.  
**Cause:** ShardManager emits Glass Box; does not feed `StartShard` on Manager.  
**Impact:** Operator distrust of status table.  
**Mitigation:** Wire phase APIs or remove active-ops section until wired.  
**Detection:** Manual; gap analysis.

## FM3 — Safety block without SafetyViolation record

**Symptom:** Tool fails / action denied; no `[SAFETY]` formatted violation in reporter history.  
**Cause:** Deny path emits routing/tool failure but never `ReportSafetyViolation`.  
**Impact:** Weaker remediation UX; only raw tool error remains.  
**Mitigation:** Hook constitutional deny → reporter with rule id.  
**Detection:** Compare kernel deny logs vs `GetRecentViolations`.

## FM4 — Misclassified errors

**Symptom:** Config issue labeled `[FS]` or timeout as `[NET]`.  
**Cause:** Substring heuristics; overlapping phrases.  
**Impact:** Wrong recovery steps.  
**Mitigation:** Typed errors at source; tighten order; tests for ambiguous strings.  
**Detection:** User reports; golden classify tests.

## FM5 — Misclassified safety violations

**Symptom:** Policy deny shown as Unknown or wrong type.  
**Cause:** `classifyViolation` depends on action/target/rule string shapes.  
**Impact:** Misleading explanation section.  
**Mitigation:** Pass structured violation type from kernel policy layer.

## FM6 — Explainer empty or shallow narrative

**Symptom:** “No derivation found” or generic decision text.  
**Cause:** Nil/empty trace; missing `next_action` root; provenance off; heuristic tracer limits.  
**Impact:** `/why` unhelpful.  
**Mitigation:** Enable provenance / `/explain`; ensure tracer runs after evaluate; deepen glossary.  
**Detection:** Chat UX; explainer tests with empty fixtures.

## FM7 — Double-close or use-after-close on bus

**Symptom:** Panic on send to closed channel (if producer races Close), or listen loop exits.  
**Cause:** `Close` closes all subscriber chans; Unsubscribe also closes.  
**Impact:** Session teardown glitches.  
**Mitigation:** Disable before Close; stop producers; single owner for Close.  
**Detection:** Lifecycle tests; stress boot/quit.

## FM8 — Flush timer races

**Symptom:** Rare lost batch or double-flush concerns.  
**Cause:** `AfterFunc` + `bufferMu` design is careful but timer + Disable/Flush interleaving is subtle.  
**Impact:** Usually self-heals (Flush on Disable).  
**Mitigation:** Keep flushLocked under bufferMu; tests for Flush/ClearTurn.  
**Detection:** event_bus tests; race detector.

## FM9 — SafetyReporter data race

**Symptom:** `-race` failures or corrupt history under concurrent reports.  
**Cause:** No mutex on `violations` slice.  
**Impact:** Unstable history; rare crash.  
**Mitigation:** Add mutex or document single-threaded ownership + enforce.  
**Detection:** `go test -race` with parallel ReportViolation.

## FM10 — Config flag false confidence

**Symptom:** User enables `jit_explain` / `stream_reasoning` in JSON; no behavior change.  
**Cause:** Status-only flags.  
**Impact:** Wasted config; support burden.  
**Mitigation:** Wire or remove from GetStatus / schema docs.  
**Detection:** Gap matrix; config audit.

## FM11 — Reflect unsubscribe mismatch

**Symptom:** Unsubscribe fails to remove; leak of subscriber; Close later double-close risk.  
**Cause:** Channel pointer identity must match.  
**Impact:** Memory growth; extra sends.  
**Mitigation:** Always Unsubscribe with exact Subscribe return value; tests cover happy path.  
**Detection:** `TestGlassBoxEventBusUnsubscribe`.

## FM12 — Verbose mode UI flood

**Symptom:** Chat unusable; lag; history bloat.  
**Cause:** Verbose → EmitImmediate + every event to scrollback.  
**Impact:** UX degradation, not correctness.  
**Mitigation:** Category filters; toggle verbose off; cap rings (500).  
**Detection:** Manual full-stream sessions.

## Summary table

| ID | Severity | Package-local fix? |
|----|----------|--------------------|
| FM1 | Medium | Partial (stats, buffers) |
| FM2 | Medium | Needs shard wiring |
| FM3 | Medium | Needs deny-path wiring |
| FM4 | Low–Med | Heuristic/tests |
| FM5 | Low–Med | Structured types |
| FM6 | Medium | Mostly producers |
| FM7 | Low | Lifecycle discipline |
| FM8 | Low | Careful bus edits |
| FM9 | Medium | Mutex |
| FM10 | Low | Honesty |
| FM11 | Low | API discipline |
| FM12 | Low | Consumer UX |
