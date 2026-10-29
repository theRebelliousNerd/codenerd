# _progress — perception architecture corpus

| Date | Action |
|------|--------|
| **2026-07-13 (Superstar uplift)** | Reconciled the corpus against commit `c8f21b46` and dirty-tree fingerprint `687d1098c52d77160c71cab2f447c786cd8033c3`. Replaced the front door with the seven-section human contract and all nine applicability lanes; added five feature cards with acceptance and rollback; removed seven ledger-approved redirect stubs after exact SHA-256 and zero-inbound verification. Fixed the live nil-provider panic boundary and added its regression. Full perception tests passed. |
| **2026-07-13** | Full rebuild to SUBAGENT_INSTRUCTIONS + cli quality bar. Research: listed `internal/perception/` (~50 non-test Go + ~48 tests + xaioauth), read transducer/understanding/semantic/factory/clients/taxonomy/learning/tracing/transport sources, grepped exports and reverse deps. Replaced thin inventory stubs with dense narrative corpus. Flagship: `IMPLEMENTED_SPEC.md` (transducer, semantic classifier, LLM clients). |
| 2026-07-13 (earlier) | Thin auto-inventory stubs (domain-model naming) — **superseded**. |

## Canonical file set (this rebuild)

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

The seven ledger-approved legacy redirects were removed only after their content
hashes matched the migration ledger and no inbound links targeted them.

## Signed Superstar score

**40/42 — PASS.** Human orientation 3; north-star alignment 3; evidence integrity
3; architecture clarity 3; data/logic contract 3; lifecycle completeness 3;
deterministic safety 3; JIT/agents 2; ecosystem wiring 3; operations 3;
verification 3; uplift quality 3; navigation/governance 3; consistency 2.

The consistency deduction records two intentional residuals rather than hiding
them: provider failure typing is not uniform, and process-global taxonomy/
semantic ownership is not a multi-workspace isolation contract. Evidence examples
are the nine-lane table in `README.md`, the control flow in `IMPLEMENTED_SPEC.md`,
and the tested nil-config repair in `client_factory_test.go`.

## Scope discipline

- Corpus edits stayed under `Docs/architecture/perception/`.
- The separately verified product repair touched only the perception client
  factory and its focused test; no Mangle/config formats changed.
