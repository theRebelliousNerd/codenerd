# TODO — articulation

> Last verified: 2026-07-13  
> Prioritized backlog. No time/cost estimates.

## P1 — Safety / correctness

1. **Shared mangle assert gate** — Ensure every consumer of `Control.MangleUpdates` runs through the same filters as session (`applyCaps` + constitutional/block list), not ad-hoc assert.  
2. **Surface claim vs dropped atoms** — Consider warning users or logging high-visibility events when model surface asserts work that control filters removed.

## P2 — Completeness / wiring

3. **memory_operations end-to-end** — Either wire durable handlers for all op types or mark ops as advisory-only in protocol docs.  
4. **context_feedback loop** — Connect usefulness/noise signals to spreading-activation / retrieval scoring.  
5. **knowledge_requests coverage** — Audit all LLM entry points for specialist re-entry parity with chat main path.  
6. **Constitutional override on chat path** — Align chat mangle filtering with session’s override helper where missing.

## P3 — Hardening

7. **Reduce fallback rate** — Prefer schema-capable clients for Piggyback-required shards; measure via stats/logs.  
8. **Atomize legacy templates** — Move coder/tester/reviewer/researcher hard-coded strings into prompt atoms; keep thin emergency stubs.  
9. **StreamParser concurrency docs/tests** — Resolve test TODO or document single-owner invariant as permanent.  
10. **ProcessorStats aggregation** — Optional session-level counters / glass-box events for fallback ratio.  
11. **Schema↔struct CI check** — Lightweight test that required schema keys match Go json tags.

## P4 — Docs / hygiene

12. **Refresh package README** — Include `json_scanner.go`, `stream_parser.go`, adapter, full test list (this corpus is authoritative).  
13. **Retire or redirect obsolete architecture filenames** if still present beside the rebuild set (e.g. older DOMAIN-MODEL titles).

## Done (do not re-open as missing)

- Dual-channel types + schema  
- Multi-stage parse + salvage  
- StreamParser  
- JIT-aware PromptAssembler  
- Decoy last-match-wins  
- applyCaps mangle filters  
- Extensive unit/boundary/fuzz suite  
