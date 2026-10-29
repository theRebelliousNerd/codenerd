# retrieval — Failure Modes

> Last verified: **2026-07-13**

Concrete failure modes grounded in current code paths.

## FM-01 — Idle retriever (integration)

**Symptom:** Boot logs “Initializing sparse retriever” but issue workflows only get extract-time facts; many relevant files never appear in activation.  
**Cause:** `Model.Retriever` never invoked; seed uses `ExtractKeywords` only.  
**Detection:** No `candidate_file` / T2+ `tiered_context_file` in EDB; no “searching N keywords” logs during chat.  
**Mitigation:** Wire `FindRelevantFiles`/`BuildContext` into seed/session (P0).

## FM-02 — Search timeout partial results

**Symptom:** Slow trees; `searchSingleKeyword` returns hits + `search timeout for keyword %q`.  
**Cause:** `SearchTimeout` exceeded; workers exit on ctx.  
**Effect:** `SearchKeywords` logs error but still merges partial hits; ranking under-complete.  
**Mitigation:** Raise timeout carefully; reduce parallelism thrash; exclude more dirs; future index.

## FM-03 — Context cancel hang (historical risk)

**Symptom:** Canceled search never returns.  
**Cause:** Workers not observing cancel (regression).  
**Detection:** `TestSparseRetriever_ContextCancellation` (integration) fails if hang > ~2s.  
**Mitigation:** Keep cancel checks in worker loop; close `files` channel after walk.

## FM-04 — False positive keywords

**Symptom:** Stopwords/common tokens still pollute Secondary/Tertiary (or miss domain terms).  
**Cause:** Heuristic extract + incomplete stopword list; PascalCase false hits.  
**Effect:** Noisy `issue_keyword` EDB → weak or wrong activation.  
**Mitigation:** Tune `isCommonWord`; optional LLM re-rank outside package; prefer Primary/MentionedFiles.

## FM-05 — Missed mentioned files

**Symptom:** Issue cites `foo/bar.py` but T1 empty.  
**Cause:** `filePathPattern` extension set limited; path not on disk; `findFile` walk fails; seed asserts unvalidated path without resolve.  
**Mitigation:** Expand extension list; resolve-then-assert in seed; fuzzy basename search.

## FM-06 — Import expand misses (T3)

**Symptom:** Critical dependency file absent from context.  
**Cause:** Python-only import regex; relative/absolute resolve incomplete; non-Python codebases.  
**Mitigation:** Language expanders; package-index integration.

## FM-07 — Tier 4 symbol false confidence

**Symptom:** “May define symbol” files are wrong; patterns treated as literals not regex.  
**Cause:** `findSymbolDefinitions` passes `^class X` into literal keyword scanner.  
**Mitigation:** Dedicated definition search API; or embedding T4; fix pattern strategy.

## FM-08 — Memory pressure from large files / hit floods

**Symptom:** High RSS during search.  
**Cause:** Full-file `ReadFile`; many KeywordHits retained; huge `parseRipgrepOutput` test shows unbounded parse.  
**Mitigation:** Max file size; cap hits per keyword; stream scan.

## FM-09 — Cache stale after local edits

**Symptom:** Second search misses just-written symbols within TTL (5m default).  
**Cause:** KeywordHitCache keys only by keyword string, not mtime.  
**Mitigation:** `Clear()` on workspace write events; shorter TTL; mtime-aware keying.

## FM-10 — Exclusion false negatives

**Symptom:** Search skips needed code under `build/` or `vendor/` (defaults exclude).  
**Cause:** Broad default patterns.  
**Mitigation:** Config overrides per project; document defaults.

## FM-11 — Soft failure hides broken search

**Symptom:** `BuildContext` returns nil error with empty T2 after walk permission errors.  
**Cause:** Tier2 error logged only; walk returns nil on many FS errors (`return nil` in WalkDir callback).  
**Mitigation:** Surface degraded flag on `TieredContext`; metrics.

## FM-12 — Windows path / colon parse

**Symptom:** `parseRipgrepOutput` mis-splits `C:\repo\file.go:1:2:text`.  
**Cause:** SplitN on `:` with drive letter.  
**Mitigation:** Avoid path for live search (native paths); if re-enabling rg, use null-delimited or JSON output.

## FM-13 — Parallelism explosion

**Symptom:** High load with many keywords.  
**Cause:** P concurrent keywords × P file workers.  
**Mitigation:** Shared global worker pool; lower defaults for huge monorepos.

## FM-14 — Comment/ops confusion (ripgrep)

**Symptom:** Operators install/debug `rg` believing it is required; it is not used.  
**Cause:** Stale comments and test names.  
**Mitigation:** Doc/comment fix (this corpus + code comments when allowed).

## Recovery playbook (operator)

1. Confirm extract facts with kernel query.  
2. Manually run package tests on a fixture to validate search binary path.  
3. If agent context poor: check verb gating (`/fix` etc.) — other verbs skip seed entirely.  
4. Clear caches by restarting chat session (new `SparseRetriever`).  
5. Narrow workDir / exclude noise for scale.
