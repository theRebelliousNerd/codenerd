# jit — architecture corpus progress

## 2026-07-13 — Postfix boundary and policy-registry reconciliation

- **Source snapshot:** `e5d8bfe4365e8973f0bdb01d026fb0b970ebbe63`.
- **Dirty-tree fingerprint:** `c3b78fa8c4fc58558ba2e6f9a2bff9adb6c00d42799005bfe6b3fac5d1adffb7`.
- **Owned source boundary:** `internal/jit` from `corpus.toml`; this audit also
  inspected repaired consumers and registry producers in `internal/session`,
  `internal/core`, and `internal/prompt` without changing product code.
- **Direct wiring audited:** specialist loader, executor capability gate, modular
  and Ouroboros registries, Piggyback catalog, core policy inventory, both prompt
  config providers, kernel root-module inventory, and the package validator.
- **Audit artifact:** `artifact:.corpus-build/audits/jit-postfix-audit.md`.

### Finding disposition

| Finding | Current disposition |
|---|---|
| `artifact:.corpus-build/findings/jit-specialist-config-validation-bypass.md` | **RESOLVED:** bounded/path-contained specialist YAML now calls `Validate`; blank identity, missing policies, traversal, and size have named regressions. |
| `artifact:.corpus-build/findings/jit-empty-config-capability-bypass.md` | **RESOLVED:** nil/empty/unlisted capabilities deny modular and Ouroboros execution; Piggyback catalogs are filtered by the same grant list. |
| `artifact:.corpus-build/findings/jit-policy-reference-drift.md` | **RESOLVED WITH RESIDUAL:** stable default sets resolve only to canonical embedded boot-corpus members and JIT validation rejects invalid references; session still lacks selective per-agent loading and set identity/version. |

The truth-gap feature card remains `in_progress`: uniform factory/fallback
validation and a typed degradation reason remain open. The policy/schema parity
card is now `in_progress`: its canonical registry/provider-parity slice is
verified, while general field-consumer parity and set-to-turn semantics remain.

### Verification receipts

- `go test -count=1 -timeout=120s ./internal/jit/...` exited 0:
  `ok codenerd/internal/jit/config`, 0.025s.
- Focused specialist/capability/Ouroboros/Piggyback session tests exited 0:
  `ok codenerd/internal/session`, 0.987s.
- The same focused session selection with `go test -race` exited 0:
  `ok codenerd/internal/session`, 7.202s.
- Strict structural validation exited 0 with 18 Markdown documents, four feature
  cards, zero legacy documents, zero broken links, zero unresolved source
  references, and all seven README sections. The post-registry strict rerun
  retained those exact counts.
- Focused canonical-policy command across core, prompt, and JIT exited 0:
  `ok codenerd/internal/core` 0.160s, `ok codenerd/internal/prompt` 0.288s, and
  `ok codenerd/internal/jit/config` 0.276s.
- Bounded validator verification ran `go test -count=1 ./internal/jit/...` and
  exited 0 in 4.089s (`ok codenerd/internal/jit/config`, 0.067s) against the
  source snapshot and dirty-tree fingerprint above.
- `git diff --check -- Docs/architecture/jit` exited 0; Git emitted only the
  repository's expected LF-to-CRLF conversion notices.

### Applicability review

All nine lanes remain resolved in [README.md](README.md) and
[02-CURRENT-STATE.md](02-CURRENT-STATE.md). The safety and testing lanes now cite
the repaired negative gates; the Mangle lane now separates verified canonical
global-corpus membership from dormant selective-loading and set-identity
contracts.

### Four-step uplift ladder

1. Preserve the verified specialist validation and fail-closed capability gates;
   finish generated/fallback validation and typed degradation.
2. Preserve verified policy registry/provider parity; finish schema-consumer
   parity and set identity/version semantics.
3. Emit a bounded effective-capability receipt.
4. Defer no-effect shadow comparison until steps 1–3 are verified.

### Fourteen-dimension self-score

Signed by `jit_postfix_auditor` and `jit_policy_reconciler` after source, diff,
registry, and regression review.

| Dimension | Score | Evidence |
|---|---:|---|
| Human orientation | 3 | README explains a review specialist from intent through effect and failure |
| North-star alignment | 3 | creative identity and deterministic executive grants remain separate |
| Evidence integrity | 3 | repaired, partial, proposed, and open claims are separated with named tests and findings |
| Architecture clarity | 3 | schema ownership and prompt/session/core boundaries are explicit |
| Data and logic contract | 3 | fields, canonical policy validation, boot/provider parity, allowlist semantics, and selective-loading residual are mapped |
| Lifecycle completeness | 3 | YAML/factory production, injection, catalog, execution, response, and fallback are covered |
| Deterministic safety | 3 | nil/empty/unlisted modular and Ouroboros paths deny before handlers; `permitted/3` remains downstream |
| JIT and agent behavior | 3 | producer, specialist injection, budgets, and config/prompt split are traced |
| Ecosystem wiring | 3 | loader, executor, both registries, Piggyback catalog, and boot consumers are cited |
| Operations | 2 | warnings and deny-all fallback exist; typed degradation and correlated receipts do not |
| Verification | 3 | package, focused boundary, and race gates prove the repaired slices |
| Uplift quality | 3 | verified truth repair and residual work form an explicit dependency ladder |
| Navigation/governance | 3 | seven routes, sole TODO authority, manifest, and audit artifact are present |
| Consistency | 3 | no current page describes empty/Ouroboros capability as permissive or canonical membership as selective policy activation |

**Total: 41/42 — PASS.** Operations remains 2 until a correlated effective-
capability receipt and typed degradation/recovery contract exist.

## Prior rebuild history

The earlier 2026-07-13 Superstar packet established the 18-document authority
set, removed seven ledger-approved redirect stubs, and recorded the three product
findings. The first postfix audit reconciled specialist and capability repairs;
this second reconciliation closes canonical reference drift without overstating
the still-open selective-loading, set-identity, and generated-fallback contracts.
