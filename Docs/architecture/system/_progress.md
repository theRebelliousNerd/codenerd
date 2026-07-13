# Corpus progress: system

## 2026-07-13 Superstar realization

The system corpus was reconciled against the live five-file production package,
19 test files, cross-package VirtualStore safety path, CLI/chat consumers, and
current product repairs. The tree was dirty with concurrent work and no
unrelated file was reset or staged.

### Review boundary

- Git commit: `c8f21b46ec4b28529953094e0c18dac4dfd0c8eb`
- `git status --short`: 412 lines
- SHA-256 of UTF-8 status text joined with LF plus final LF:
  `65c9157a78610e96e7bb99ae2cc8fc300edc84f4cdddc33e31feec167cadae31`
- Realized source root: `internal/system`
- Inventory: 5 non-test Go files, 19 test files, 61 named tests, one crash
  artifact that is not package-owned Mangle policy

### Realized and re-signed

- README now has the exact seven human-entry sections, an end-to-end journey,
  nine applicability lanes, current/residual truth, and three reading routes.
- Current/spec/wiring/safety/testing/failure docs now describe complete policy-
  shard authorization ownership, primary-RealKernel Dreamer wiring, preserved
  executive action IDs, private JIT compilation scopes, and maintenance Close
  ordering.
- Four authoritative feature cards cover the verified executive-envelope repair,
  cache identity plus boot rollback, the session file-policy adapter, and a typed
  resource registry/redacted boot receipt.
- Two system findings were packetized and are now resolved for their verified
  slices: disabled-shard cache identity and deterministic late-boot cleanup.
- `09-CONSTITUTIONAL-SAFETY.md` and `09-MANGLE-SURFACE.md` were converted from
  stubs into optional deep dives rather than deleted.

### Legacy migration

The seven ledger-approved redirect files matched their recorded SHA-256 prefixes
and had zero exact inbound references before removal:

| Removed redirect | Verified prefix |
|---|---|
| `01-DOMAIN-MODEL.md` | `8eced542fbfe` |
| `02-CURRENT-STATE-SYSTEM.md` | `75ed1bcc9e7e` |
| `03-GAP-ANALYSIS-SYSTEM.md` | `e3fff33f07b8` |
| `04-INVARIANTS-AND-GATES.md` | `5649d243a107` |
| `05-CROSS-SYSTEM-WIRING.md` | `85d7fd995dd9` |
| `06-TESTING-STRATEGY.md` | `d80f013ddc02` |
| `08-FAILURE-MODES.md` | `814712956641` |

### Verification receipts

- `go test ./internal/system/... -count=1` — PASS in 78.718s.
- Focused race gate over exact policy ownership, concurrent prompt scopes,
  maintenance no-immediate-run, and Close ordering — PASS in 20.430s.
- Strict structural validator before verification — PASS:
  `corpora=1 features=4 legacy=0 broken_links=0 unresolved_refs=0`.
- Strict validator with fixed system verification profile — PASS in 80.623s.
- Scoped `git diff --check` — PASS after whitespace repair; line-ending
  conversion warnings only.

### Signed Superstar rubric: 40/42

| Dimension | Score | Review evidence |
|---|---:|---|
| Human orientation | 3 | one-minute model, representative query journey, reading routes |
| North-star alignment | 3 | LLM creative center, Mangle executive, motherboard non-goals |
| Evidence integrity | 3 | claim labels, symbols/tests, commit and dirty fingerprint |
| Architecture clarity | 3 | boot stages, identity/state diagrams, component boundaries |
| Data and logic contract | 3 | shard predicate ownership, exact envelope and correlation |
| Lifecycle completeness | 2 | normal maintenance/Close proven; late-boot unwind open |
| Deterministic safety | 3 | default deny, exact target/payload, Dreamer fail closed |
| JIT and agent behavior | 3 | embedded/project/user-agent wiring and private compile scopes |
| Ecosystem wiring | 3 | CLI/chat, kernel, VirtualStore, shards, session, adapters |
| Operations | 2 | degradation/signals/triage documented; typed receipt absent |
| Verification | 3 | full package, focused race, strict fixed-profile verification |
| Uplift quality | 3 | verified repair plus dependency-ordered safe and north-star cards |
| Navigation/governance | 3 | sole TODO authority, exact routes, ledger-safe cleanup |
| Consistency | 3 | stale maintenance/policy/prompt claims reconciled across corpus |

Reviewer: Codex `system_corpus_writer`, reconciled by `corpus-doc-auditor`. The
40/42 score passes the 34/42 threshold and every mandatory dimension is at least
2. The score intentionally preserves the engine-identity, typed acquisition-
registry, reset, adapter-policy, and boot-receipt residuals.

## 2026-07-13 post-repair reconciliation

Product evidence reviewed after the initial corpus realization:

- `normalizeDisableSystemShards`, length-delimited `cortexKey`, and
  `getOrBootCortex` now share the normalized disabled-shard set.
- `defaultBootSteps`, `bootCortexWithSteps`, `rollbackBootContext`, and
  `cortexFromBootContext` provide one named failure boundary and aggregate
  cleanup path.
- `Cortex.Close` is mutex-idempotent and owns shard admission/workers, MCP,
  browser, closable embeddings, JIT/DBs, perception, maintenance, and cache
  eviction with bounded steps.
- `defaultKernelShardConfigs` consumes the canonical
  `DefaultShardPredicateManifests`; unique predicate ownership and the complete
  exact permission envelope are regression-protected.
- New system tests raise the live inventory to 19 test files and 61 named tests.

Focused evidence:

- cache/rollback/idempotence regression slice — PASS;
- same lifecycle slice under `-race` — PASS in 9.091s;
- `go vet ./internal/system` — PASS;
- fixed-profile `go test -count=1 ./internal/system/...` — PASS in 79.191s.

Honesty boundary: the disabled-shard identity defect and enumerated late-failure
leak are closed. Engine/provider mode remains outside identity. Cleanup is not a
typed exact-reverse-order registry, caller-owned override semantics are unpinned,
and queue/MCP/browser cleanup-failure injection remains open.

### Current-tree verification closure

After the concurrent config/runtime tree settled, the fixed corpus profile
passed. `python .agents/skills/corpus-build/scripts/validate_architecture_corpora.py
--check --strict --verify --verify-timeout-seconds 180 --corpus
Docs/architecture/system --json` reported:

- structural validity: PASS;
- 20 Markdown files and 4 feature cards;
- zero legacy documents, broken links, unresolved source references, or missing
  README sections;
- `go test -count=1 ./internal/system/...`: PASS, package time 79.191s and
  verifier wall time 85.557s;
- commit `8eda80b00c9e769d709aa2f58f10ab98624d65e2`;
- dirty fingerprint
  `85c64071e14a15f5eb80e926ea8f342259e147da400f932018a3efff8addc4fc`.

The package pass includes the restored missing-LLM soft-boot contract without
ambient fallback. The transient failed receipt is superseded by this current-
tree evidence; product residual statuses above are unchanged.
