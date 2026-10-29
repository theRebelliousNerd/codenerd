# testing — Open Questions

> Last verified: 2026-07-13

## Q1 — Should mock mode boot Cortex at all?

**Context:** Even `--mode=mock` calls `GetOrBootCortex`, which is heavy for CI.  
**Trade-off:** Shared real kernel catches LoadFacts schema issues early vs. slow, flaky boot.  
**Options:** (a) keep Cortex for all modes; (b) light kernel path for mock; (c) unit-test-only mock without CLI.  
**Status:** Open — current code always boots.

## Q2 — What is the source of truth for compression metrics?

**Context:** Engines estimate compressed tokens as `facts*20` while types document enrichment vs compression carefully. Real Compressor is constructed but unused in CompressTurn.  
**Question:** Should scenario expected ratios target (1) metadata enrichment forever, (2) true LLM compression, or (3) split metrics?  
**Status:** Open — code leans (1); older README leaned (2).

## Q3 — How strict should checkpoint fallback be?

**Context:** Empty retrieval falls back to MustRetrieve, guaranteeing recall on empty engines in some paths.  
**Question:** Fail hard always? Fail hard only in real mode? Metric-only warning?  
**Status:** Open — currently lenient.

## Q4 — Does the parent package `internal/testing` have a future API?

**Context:** Only comments. All imports use `context_harness`.  
**Question:** Grow shared testkits here (temp workspaces, kernel fixtures), or leave as namespace for context harness only?  
**Status:** Open — do not invent API without a second consumer.

## Q5 — Should integration scenarios refuse mock mode?

**Context:** Scenarios set `Mode: RealMode` but harness does not reject running them under mock engine.  
**Question:** Hard error, warning, or allow degraded runs?  
**Status:** Open.

## Q6 — Component count documentation (7 vs 9)

**Context:** Comments and CLI strings still say “7-component” while `ActivationBreakdown` has nine fields (incl. feedback + back-reference).  
**Question:** Canonical count for docs/CLI?  
**Status:** Prefer **9** as implemented on the breakdown struct; clean comments when code next touched.

## Q7 — Ownership of campaign paging verification

**Context:** Integration scenarios mention phase transitions; production paging may live in campaign/context packages.  
**Question:** Who owns end-to-end paging correctness tests — context harness, campaign tests, or both with shared fixtures?  
**Status:** Open — harness currently narrative/activation oriented.

## Q8 — Live mode cost / determinism policy

**Context:** `--live` calls real LLM; non-deterministic JSON and cost.  
**Question:** Nightly only? Record/replay fixtures?  
**Status:** Open — manual today.

## Q9 — Import of production feedback store in tests

**Context:** `feedback_test.go` integrates with store-side feedback learning.  
**Question:** How much production learning stack should package tests own vs mock?  
**Status:** Monitor for brittle cross-package tests.

## Q10 — Should scenario definitions leave Go?

**Context:** Large Go builders with message templates.  
**Question:** External YAML/JSON scenarios for non-dev authors?  
**Status:** Open — Go is fine until volume hurts reviewability.
