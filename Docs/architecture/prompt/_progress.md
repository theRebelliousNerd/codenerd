# Prompt corpus progress

## Post-repair reconciliation: 2026-07-13

Scope was documentation under `Docs/architecture/prompt`, resolution status in
the two prompt finding records, and read-only source/test audit. No product Go,
Mangle, YAML, shared portfolio, or unrelated dirty-tree file was edited by this
postfix pass.

### Source snapshot

- Commit: `cfc537e96495e1fbccd7efff8bb8e4001c93ca9c`.
- Pre-final-record dirty fingerprint from the strict verifier:
  `79e94536827bc46b728ceff0e07f3f90733c99bae574b8b197a605693993f20f`.
- Fingerprint method: corpus validator SHA-256 over the dirty-tree snapshot; the
  final progress-record edit is intentionally subsequent to that receipt.
- Worktree condition: heavily dirty with concurrent user/Grok/agent changes;
  claims were verified from live named symbols and bounded gates.
- Scoped guidance: `Docs/architecture/AGENTS.md`, the Superstar standard, and
  `internal/prompt/agents.md` were followed.

### Reconciled closures

| Feature card | Status | Evidence |
|---|---|---|
| `prompt-no-tool-retry-cache-key-v1` | `verified` | `CompilationContext.Hash` schema v2; hash regression; production retry output regression |
| `prompt-compile-fact-scope-v1` | `verified` | `KernelScopeProvider`; cloned production kernel; success/error/cancel/panic and concurrent race gates |
| `prompt-atom-contract-gate-v1` | `verified` | shared strict parser; bounded migrations; fail-closed sync; 333-file/888-ID ordered parity |

`artifact:.corpus-build/findings/prompt-jit-context-isolation.md` and
`artifact:.corpus-build/findings/prompt-atom-contract-drift.md` are now marked
resolved while retaining their original causal histories.

### Corpus surfaces updated

| Surface | Reconciliation |
|---|---|
| `README.md` | Representative retry journey, production scope isolation, strict parity, honest external-adapter/test-race residuals, reordered frontier |
| `01-VISION.md`, `02-CURRENT-STATE.md`, `03-GAP-ANALYSIS.md` | G1-G3 closed with source/test evidence; G4-G8 residuals separated |
| `04`, `05`, `06`, `08`, `09` | Scope ownership, strict parser, public interfaces, boot/sync wiring, safety invariants |
| `10-TESTING-ALIGNMENT.md`, `12-FAILURE-MODES.md`, `13-PROMPT-JIT-DEEP-DIVE.md` | Current gates, guarded failure modes, v2 identity and scope lifecycle |
| `IMPLEMENTED_SPEC.md` | Exact scale, APIs, pipeline, lifecycle, schema, and status rows |
| `TODO.md`, `OPEN-QUESTIONS.md` | Three verified cards; cache/scoped-guidance decisions; genuine decision-receipt and adapter work preserved |

### Receipts

| Gate | Command | Result |
|---|---|---|
| Strict atom tree | `go run ./cmd/tools/validate_prompt_atoms -root internal/prompt/atoms -fail-on-warn` | PASS: 333 YAML, 888 atoms, 0 issues |
| Package/schema/sync | `go test -count=1 -timeout=240s ./internal/prompt ./internal/prompt/sync ./cmd/tools/validate_prompt_atoms` | PASS: 2.563s, 0.182s, 0.722s |
| Production scope/cache race | `go test -race -count=1 -timeout=240s ./internal/system -run 'TestKernelAdapter_(CompilationScopesIsolateConcurrentPrompts|CompilationScopeDoesNotLeakOnBudgetError|CompilationScopeDoesNotLeakOnCancellation|RetryContextBypassesPreRetryCache)$'` | PASS: 18.070s |
| Full prompt/sync race | `go test -race -count=1 -timeout=240s ./internal/prompt ./internal/prompt/sync` | PASS: prompt 25.909s; sync 1.427s |
| Corpus structure/runtime | `python .agents/skills/corpus-build/scripts/validate_architecture_corpora.py --check --strict --verify --verify-timeout-seconds 240 --corpus Docs/architecture/prompt --json` | PASS: 21 Markdown, 5 cards, 0 legacy, 0 broken links, 0 unresolved refs; package verify 11.105s |

The earlier test-fixture race was repaired with mutex-protected mock kernel access;
both the whole-package and focused production race receipts are now green.

### Signed semantic score

| Dimension | Score | Evidence |
|---|---:|---|
| Human orientation | 3 | README explains the user-visible prompt path and retry outcome |
| North-star alignment | 3 | Creative model remains downstream of deterministic selection and permission |
| Evidence integrity | 3 | Closed and residual claims cite live symbols and both passing/failing gates |
| Architecture clarity | 3 | Cache, compilation scope, schema, persistence, and consumer boundaries are explicit |
| Data and logic contract | 3 | v2 identity, strict atom schema, typed vocabulary, and Mangle facts documented |
| Lifecycle completeness | 3 | Boot, cache miss, scope close, sync, retry, cancellation, and panic covered |
| Deterministic safety | 3 | Production facts isolated; prompt/config cannot replace `permitted/3` |
| JIT and agent behavior | 3 | Selection, migrations, synchronization, budgets, and visible retry behavior |
| Ecosystem wiring | 3 | Factory, session, articulation, shards, validator, and sync routes traced |
| Operations | 2 | Current logs/manifests exist; durable correlated decision receipt remains open |
| Verification | 2 | Strong unit/parity/full-race/production gates; fuzz and external-adapter conformance remain open |
| Uplift quality | 3 | Three truth repairs verified; adapter, receipt, shadow, and replay sequence bounded |
| Navigation/governance | 2 | Exact cards/routes/statuses; two approved supersession stubs remain |
| Consistency | 3 | Cache, scope, schema, counts, and finding statuses agree across canonical docs |

**Total: 39/42 — PASS.** Mandatory dimensions are at least 2. This is a corpus
quality verdict, not a whole-product completion claim.

### Residuals

1. Require scope support or fail closed for third-party kernel adapters.
2. Add fuzz pressure for parser/context/resolver boundaries.
3. Generate or remove the dormant ConfigAtom tool catalog and make cache TTL/result ownership honest.
4. Design the durable redacted prompt decision receipt before shadow/replay evolution.
5. Retain the two ledger-approved supersession files until shared governance authorizes removal.
