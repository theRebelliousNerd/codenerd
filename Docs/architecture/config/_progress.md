# config corpus progress

## 2026-07-13 — strict Superstar uplift

- Audited all 20 non-test config sources, six package test files, primary
  system/chat/campaign consumers, logging/redaction, Mangle boundary, and
  persistence paths.
- Reconciled realized truth, added all nine applicability lanes, and promoted
  five acceptance-complete uplift cards in `TODO.md`.
- Product defects discovered during the audit were handed to the root owner and
  recorded at `artifact:.corpus-build/findings/config-audit-defects.md`; this
  corpus worker did not edit product code.
- Concurrent root repair added strict unknown/trailing rejection, fail-closed
  shared boot and explicit-provider routing, same-directory synced private
  persistence, wizard merge, shared execution containment/projection, forced
  Codex CLI isolation, trace-off defaults, and private raw-trace mode.
- Focused regressions now prove strict decode, pre-rename preservation, Unix
  `0600` plus round-trip, wizard preservation, shared execution positive/hostile
  cases, invalid-file ambient-rescue denial, Codex hostile-config filtering and
  trace-off defaults. The corpus marks those slices verified and keeps semantic,
  campaign/dormant parity, writer-conflict/backup/Windows ACL, secret/redaction,
  and process-lifecycle residuals open.
- Harvested seven ledger-approved redirect stubs after each SHA-256 prefix
  matched `Docs/architecture/_rebuild/LEGACY_MIGRATION_LEDGER.md` and config-local
  inbound references were removed and rechecked at zero.

## Verification receipt

| Gate | Result |
|---|---|
| Source commits inspected | `cfc537e96495e1fbccd7efff8bb8e4001c93ca9c` at audit start; config repair `6cdef4b3`; final tree `0f07a2905219109d249cd9e244ae3d22225e329b` |
| Target tree before edits | only `Docs/architecture/config/corpus.toml` was untracked in owned config scope |
| Package gate | `go test -count=1 -timeout=240s ./internal/config/...` — PASS |
| Race gate | `go test -race -count=1 -timeout=240s ./internal/config/...` — PASS |
| Shared system repair gate | `go test -count=1 -timeout=240s ./internal/system -run 'TestExecutionLayerConfigs\|TestInitCoreComponentsRejectsPresentInvalidConfig\|TestBootCortexWithConfig_NoLLMConfigured'` — PASS |
| Codex boundary gate | `go test -count=1 -timeout=240s ./internal/perception -run 'TestCodexCLIClient_buildCLIArgs_(DisableShellToolConfig\|FiltersEffectOverrides)'` — PASS |
| Wizard gate | `go test -count=1 -timeout=240s ./cmd/nerd/chat -run TestSaveConfigWizardPreservesUnownedSettings` — PASS |
| Strict structure | PASS — 18 Markdown, 5 cards, 0 legacy, 0 broken links, 0 unresolved source refs, 0 missing README sections |
| Fixed corpus verification | PASS — validator ran `go test -count=1 ./internal/config/...` in 5.524s |

Legacy SHA-256 prefixes: `ce01b8649279`, `06ebc9d82572`,
`132fe40a3133`, `d7b546365b8e`, `e636dfb79494`, `7a2dd51d1952`,
`0254b1f5186a`.

## Signed human score

**40/42 — PASS.** Human orientation 3, north-star alignment 3, evidence
integrity 3, architecture clarity 3, data/logic contract 3, lifecycle 3,
deterministic safety 3, JIT/agents 2, ecosystem wiring 3, operations 3,
verification 3, uplift 3, navigation/governance 2, consistency 3.

Mandatory dimensions are all at least 2. Signed: **Codex config corpus auditor,
2026-07-13**. The evidence-by-dimension review is in
[00-ALIGNMENT-VISION-REVIEW.md](00-ALIGNMENT-VISION-REVIEW.md).

## Final tree identity

- Git commit at fixed verification: `0f07a2905219109d249cd9e244ae3d22225e329b`.
- Validator dirty-tree fingerprint:
  `0ece353e571098ed81453b922f51bcb05e748c10b1a341adffc6c891356aa152`.
- Receipt reconciles the final settled config/product repair tree inspected by
  this corpus worker.
