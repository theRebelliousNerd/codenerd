# core — Architecture Corpus Progress

## 2026-07-13 — Executive-envelope repair reconciliation

- **Source commit:** `c8f21b46ec4b28529953094e0c18dac4dfd0c8eb`
- **Dirty-tree fingerprint captured by strict+verify:**
  `3cabf450784db2c8c1629f92250e25d448b39314befb41ec66645d3afd288575`
  (`SHA-256(git status --porcelain=v1)`; the shared worktree contained concurrent
  changes outside this corpus).
- **Handoff dirty-tree fingerprint:**
  `ffc768f008794c104e4fc327be8dd845b96fa36a4a24ad3d524e8a55a87c9bcd`.
- **Scope:** documentation reconciliation only in `Docs/architecture/core`; no
  product or shared-surface edit by the corpus-doc-auditor.
- **Semantic verdict:** **PASS**, with bounded residuals listed below.

### Current product receipts

- `go test -count=1 -timeout=240s ./internal/core` — **PASS**,
  `ok codenerd/internal/core`, 168.105s.
- Focused 14-test core safety/lifecycle command covering sandbox isolation,
  checked projection rejection, full/diff limit parity, lazy predicate corpus,
  Unicode-blank RuleCourt rejection, nil-kernel/Dreamer routes, Cortex Dreamer,
  failure fact shapes, prune timestamp, validation propagation, and
  classification-only permission cache — **PASS**,
  `ok codenerd/internal/core`, 7.396s.
- `go test -count=1 -timeout 120s ./internal/system -run
  '^TestDefaultCortexPermissionEnvelopeRoutesToPolicyShard$'` — **PASS**,
  7.374s.
- `go test -count=1 -timeout 120s ./internal/shards/system -run
  '^TestPendingActionPipelineProducesRoutingResult$'` — **PASS**, 7.281s.
- `go test -count=1 -timeout 120s ./internal/core -run
  '^(TestTDDLoop_RunTests_Success|TestVirtualStoreCodeDOM_EditElement_Detailed)$'`
  — **PASS**, 5.630s.

The full core root gate is intentionally bounded: composite tactile-executor
construction probes Docker availability, so host Docker state can add latency or
produce an environmental failure. The recorded run passed within its 240-second
budget.

### Signed 14-dimension score

| Dimension | Score | Evidence |
|---|---:|---|
| Human orientation | 3 | README one-minute explanation and edit journey |
| North-star alignment | 3 | creative/model vs deterministic executive boundary and non-goals |
| Evidence integrity | 3 | current/proposed labels, named negative tests, commit and receipts |
| Architecture clarity | 3 | kernel, RouteAction, Dreamer, Cortex ownership flows |
| Data and logic contract | 3 | exact `permitted/3` and declared failure/result arities |
| Lifecycle completeness | 2 | ingress-to-result, dual boot modes, recovery; mid-eval cancel remains partial |
| Deterministic safety | 3 | nil dependency, mismatch, staging, and validation failures deny |
| JIT and agent behavior | 2 | correct N-A boundary and prompt ownership; not a core prompt corpus |
| Ecosystem wiring | 3 | policy-shard join, executive action ID, session/system paths |
| Operations | 2 | signals/recovery documented; full gate has Docker probe dependency |
| Verification | 3 | focused negatives, cross-package seams, and bounded full-core PASS |
| Uplift quality | 3 | two verified truth repairs plus staged registry/receipt/shadow cards |
| Navigation/governance | 3 | authority routes, feature IDs, owners, and deduplicated cards |
| Consistency | 3 | prior permission, Dreamer, schema, validation, and boot contradictions reconciled |
| **Total** | **39/42** | Pass threshold 34; every mandatory floor is at least 2 |

**Signed:** `corpus-doc-auditor`, 2026-07-13.

### Honest residuals

1. Dreamer cache identity is `type:target`; canonical payload and a unified
   kernel/policy mutation epoch are not part of the freshness contract.
2. Direct `VirtualStore.Exec` remains a limited peer path without the full
   Dreamer/Mangle `RouteAction` envelope.
3. Context is checked around Dreamer evaluation, but a running Mangle evaluation
   is not itself context-cancellable.
4. Action enum, handler, destructive classification, and Mangle policy still lack
   one generated parity registry.
5. CodeDOM concrete-file metadata is implemented and the handler/full suite pass;
   a dedicated negative semantic-validation seam test is still desirable.
6. The successful full-core receipt is bounded rather than proof of every
   repository consumer; Docker probing makes it host-sensitive.

### Corpus gates

- `python .agents/skills/corpus-build/scripts/validate_architecture_corpora.py
  --strict --verify --verify-timeout-seconds 300 --corpus
  Docs/architecture/core --json` — **PASS** in 152.655s: valid true, zero
  errors/warnings/broken links/unresolved source references; 23 Markdown files and
  5 feature cards. Its verification command
  `go test -count=1 ./internal/core/...` passed: root 148.942s, defaults 0.028s,
  policy 0.312s, shards 1.944s.
- `git diff --check -- Docs/architecture/core` — **PASS** (exit 0; only the
  repository's LF-to-CRLF conversion warnings were emitted).

## 2026-07-13 — Verified Dreamer state-isolation repair

- **Source snapshot:** `63ed8e460ea012665dd775fc40d812c45fa1c64b`
- **Dirty-tree fingerprint before packet:** `38ad0ca465e5520b048989c1e3629244686b0fe8`
- **Packet-owned product files:** `internal/core/dreamer.go`,
  `internal/core/dreamer_test.go`
- **Discriminator:** `TestDreamer_SimulateAction_DoesNotMutateLiveHypotheticals`
  asserts that the live `hypothetical/1` fact count does not grow and that the
  sandbox projection still receives the fact.
- **Receipt:**
  `go test -count=1 ./internal/core -run 'TestDreamer_SimulateAction_(DoesNotMutateLiveHypotheticals|Safe|Unsafe)$'`
  exited 0 (`ok codenerd/internal/core`, 1.564s).
- **Rollback boundary:** the Dreamer projection and focused regression only; no
  Mangle schema or external API changed.
- **Concurrent state:** the wider worktree was dirty; no pre-existing core corpus
  document or Dreamer source change was overwritten.

## 2026-07-13 — Full rebuild (docs-only)

**Agent task:** Rebuild `Docs/architecture/core/` to CLI-depth quality.  
**Source:** `internal/core/`, `internal/core/defaults/`, `internal/core/shards/`.  
**Constraint:** No Go/Mangle/test/code modifications.

### Produced / overwritten (authority set)

| File | Status |
|------|--------|
| `README.md` | Rebuilt |
| `IMPLEMENTED_SPEC.md` | Dense flagship rebuilt |
| `00-ALIGNMENT-VISION-REVIEW.md` | Rebuilt |
| `01-VISION.md` | New authority name |
| `02-CURRENT-STATE.md` | New authority name |
| `03-GAP-ANALYSIS.md` | New authority name |
| `04-ARCHITECTURAL-PRINCIPLES.md` | New authority name |
| `05-INTERNAL-ARCHITECTURE.md` | New authority name |
| `06-PUBLIC-API-AND-TYPES.md` | New authority name |
| `07-DEPENDENCY-MAP.md` | Rebuilt |
| `08-WIRING-AND-INTEGRATION.md` | New authority name |
| `09-SAFETY-AND-INVARIANTS.md` | New authority name |
| `10-TESTING-ALIGNMENT.md` | New authority name |
| `11-OBSERVABILITY.md` | New authority name |
| `12-FAILURE-MODES.md` | Rebuilt as failure catalog |
| `13-MANGLE-SURFACE.md` | Extra deep-dive (critical package) |
| `TODO.md` | Rebuilt |
| `OPEN-QUESTIONS.md` | Rebuilt |
| `_progress.md` | This file |

### Research notes used

- Read `kernel_types.go`, `kernel_init.go` (loadMangleFiles), `kernel_eval.go`, `kernel_facts.go` (API surface), `kernel_policy.go`
- Read `virtual_store.go`, `virtual_store_routing.go`, `virtual_store_constitution.go`, `virtual_store_types.go`, `virtual_store_actions.go`
- Read `dreamer.go`, `cortex_kernel.go`, `shards/manager.go`
- Read `defaults/schemas.mg`, `schemas_safety.mg`, `policy/constitution.mg`, `policy/dreamer.mg`, `policy/system_core.mg`
- Grepped constructors, types, reverse imports
- Compared quality bar to `Docs/architecture/cli/`

### Explicit non-actions

- Did not modify any files outside `Docs/architecture/core/`
- Did not update `internal/core/README.md` (code tree) — noted staleness in current-state doc instead

### Legacy thin filenames

Redirect stubs written for: `01-DOMAIN-MODEL.md`, `02-CURRENT-STATE-CORE.md`, `03-GAP-ANALYSIS-CORE.md`, `04-INVARIANTS-AND-GATES.md`, `05-CROSS-SYSTEM-WIRING.md`, `06-CONSTITUTIONAL-SAFETY.md`, `06-MANGLE-SURFACE.md`, `06-TESTING-STRATEGY.md`, `08-FAILURE-MODES.md` (legacy path), `09-CONSTITUTIONAL-SAFETY.md`, `09-MANGLE-SURFACE.md` → point at rebuild authority names.

The seven filenames covered by the portfolio legacy pattern were removed on
2026-07-13 after their ledger hashes matched, all canonical destinations existed,
and the full-path Markdown consumer scan found no inbound references. Other named
deep-dive compatibility files are outside that pattern and remain subject to
separate semantic review.

### Follow-ups

- Engineering backlog remains in `TODO.md` / `OPEN-QUESTIONS.md`
- Optional later: delete redirect stubs once no external links remain
