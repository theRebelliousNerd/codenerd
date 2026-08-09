# mangle corpus rebuild — progress

| Date | Action |
|------|--------|
| 2026-07-13 | Full corpus rebuild from `internal/mangle/` source (docs-only). Flagship `IMPLEMENTED_SPEC.md` rewritten with dense engine / differential / feedback deep dives. Canonical doc set per `Docs/architecture/_rebuild/SUBAGENT_INSTRUCTIONS.md`. Quality bar: `Docs/architecture/cli/`. |

## 2026-08-09 parser concurrency boundary

- Routed sanitizer, synth compiler, and both system fact adapters through the
  process-wide `ParseUnit` chokepoint.
- Added a whole-module AST source guard that rejects raw parser selectors outside
  `parse_lock.go`, including test calls and function references, plus a mixed
  ParseUnit/ParseAtom/sanitizer/synth race test.
- Corrected `TestCortexKernel_WhenConcurrentTransactions_ShouldNotDeadlock`: it
  previously started `Wait` before any `Add`, allowing an immediate false pass
  and leaked workers. Five focused race repetitions and the full concurrency
  slice now pass.
- Verification: affected packages pass serially; mixed caller race passes five
  repetitions; core concurrency race passes in 75.357s; vet passes. The full
  799-test core race package reached its 10-minute package timeout without a
  race and is not claimed green.

## Files produced (canonical set)

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

## Research basis

- Read: `engine.go`, `differential.go`, `parse_lock.go`, `schema_validator.go`, `grammar.go`, `proof_tree.go`, `lsp.go`, `feedback/*`, `synth/*`, `transpiler/sanitizer.go`
- Kernel wiring: `internal/core/kernel_eval.go`, `kernel_init.go`, `kernel_policy.go`, `parse_serial.go`
- Downstream: `internal/shards/system/legislator.go`, `internal/shards/system/executive.go`, `internal/shards/system/constitution.go`, `internal/shards/system/mangle_repair.go`, `cmd/nerd/cmd_mangle_check.go`, `cmd/nerd/cmd_mangle_lsp.go`
- Reverse imports grepped for `codenerd/internal/mangle`

## 2026-07-13 superstar uplift review

### Source snapshot and dirty boundary

- Git commit inspected: `c8f21b46ec4b28529953094e0c18dac4dfd0c8eb`.
- Pre-edit `git status --short` fingerprint: SHA-256
  `c48f18e317759fc609c610b25fffea2793f82e9835e58878eb14b53ce06017a0`
  across 138 status lines. The worktree already contained unrelated Codex, Grok,
  stress-tester, core, prompt, CLI, and portfolio changes; none were reverted.
- Realized source inventory: 21 non-test Go files, 40 Go test files, one `.mg`.
- Scoped guidance used: root `AGENTS.md`, `Docs/architecture/AGENTS.md`,
  `_rebuild/SUPERSTAR_CORPUS_STANDARD.md`, corpus-build, and Mangle guardrails.
  The package-scoped Mangle guide referenced by root guidance was absent, so no
  additional subtree instructions were available.

### Review work

- Re-read engine, differential, parse, schema, grammar, feedback, synth,
  sanitizer, proof, LSP, package tests, core evaluator integration, intent boot,
  and model-rule-producing system shards.
- Replaced README with the seven-section human front door, a user-visible
  journey, all nine applicability lanes, claim statuses, and three reading routes.
- Reconciled vision/current/gap and replaced free-form backlog authority with two
  complete `NERD_FEATURE` cards.
- Recorded probable product findings for differential gas-option loss and the
  unified fast-path read contract under `.corpus-build/findings/` without editing
  product code.
- Removed seven ledger-approved redirect-only files after SHA-256 prefix, target,
  and inbound full-path checks. No substantive claim required harvest.

### Verification

- `go test ./internal/mangle/... -count=1` — PASS for root, feedback, synth, and
  transpiler packages.
- Bounded unified-mode reproduction — `Query` returned 0 bindings, a snapshot
  query returned 0, while `CopyAllFactsTo` returned 1 fact; the temporary
  reproduction source was removed after the run.
- Strict corpus subset validator — PASS after documentation, redirect, and P0
  product changes; the exact current command and receipt are recorded below.
- `git diff --check` — PASS for the owned Mangle/core/doc paths (line-ending
  conversion warnings only).

### Re-signed human rubric after P0 closure: 39/42

| Dimension | Score | Review evidence |
|---|---:|---|
| Human orientation | 3 | README minute view, mental model, failing-test journey |
| North-star alignment | 3 | creative-center/executive boundary and explicit non-goals |
| Evidence integrity | 3 | claim labels, symbol/test evidence, dirty fingerprint |
| Architecture clarity | 3 | ownership table, journey, full/diff split |
| Data and logic contract | 3 | Decl, atoms/strings, negation, recursion, aggregation |
| Lifecycle completeness | 3 | ingress through failure, fallback, response, recovery |
| Deterministic safety | 3 | protected heads, default-deny ownership, full/diff gas enforcement and regressions |
| JIT and agent behavior | 2 | caller protocol matrix; not every producer is synth-first |
| Ecosystem wiring | 3 | core, system-shard, CLI, browser, and intent boot paths |
| Operations | 2 | limits/signals/fallback documented; unified receipt absent |
| Verification | 3 | package, focused race, kernel parity, strict+verify, and diff-check receipts |
| Uplift quality | 3 | fail-closed product repair, default parity, and bounded read-contract reproduction |
| Navigation/governance | 3 | exact README routes, sole TODO cards, legacy cleanup |
| Consistency | 2 | reviewed core surfaces aligned; older deep docs need later global pass |

Reviewer: Codex Wave-2 `mangle_corpus_writer`. Re-signed after the P0 product and
default-parity fixes. The score passes the 34/42 corpus threshold and every
mandatory dimension is at least 2. The unified fast-path read-contract finding
remains open and prevents a stronger operations/consistency score.

## 2026-07-13 P0 differential gas follow-up

- Reproduced the finding before implementation with
  `TestDifferentialEngine_DerivedFactsLimit`: legacy atom delta, legacy fact
  delta, and unified atom delta all exceeded a configured limit of 5 without
  returning an error.
- Added `DifferentialEngine.evalOptions` and passed its
  `WithCreatedFactLimit` option to every direct differential evaluator call.
- After the fix the same finite 10x10 Cartesian fixture returns the upstream
  `fact size limit reached` error on all three routes.
- Updated the kernel eligibility comment to distinguish verified gas enforcement
  from the still-unsupported external-predicate option. No Dreamer, VirtualStore,
  or RuleCourt file was touched.
- Closed the follow-on zero-config mismatch: full kernel evaluation previously
  defaulted to 500,000 while the diff builder inherited the reusable Engine's
  100,000 default. Both now consume
  `internal/core/kernel_init.go#RealKernel.effectiveDerivedFactLimitLocked`, with
  `TestKernelEval_ZeroConfigDerivedFactLimitParity` guarding configuration parity.
- Current verification receipts:
  - `go test ./internal/mangle/... -count=1` — PASS across mangle, feedback,
    synth, and transpiler.
  - `go test -race ./internal/mangle -run '^(TestDifferentialEngine_(DerivedFactsLimit|Incremental)|TestSnapshotIsolation)$' -count=1`
    — PASS.
  - `go test ./internal/core -run '^(TestKernelEval_ZeroConfigDerivedFactLimitParity|TestKernelDifferentialEval)$' -count=1`
    — PASS.
  - `python .codex/skills/corpus-build/scripts/validate_architecture_corpora.py --corpus Docs/architecture/mangle --strict --verify --verify-timeout-seconds 180`
    — PASS: `corpora=1 features=2 legacy=0 broken_links=0 unresolved_refs=0`;
    fixed Mangle verification profile PASS.
  - Scoped `git diff --check` — PASS (line-ending conversion warnings only).
- Final review boundary: Git commit
  `c8f21b46ec4b28529953094e0c18dac4dfd0c8eb`; post-fix
  `git status --short` SHA-256 fingerprint
  `74f6d58f545be522d765a8ee3c4bed0a51703e179e0fdf35b5276d2b7548f979`
  across 185 status lines. The dirty tree includes concurrent, unrelated work;
  it was preserved.
