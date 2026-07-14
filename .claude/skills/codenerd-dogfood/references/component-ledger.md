# codeNERD Component Ledger

Living status matrix for the dogfood sweep: exercise each component through the
CLI, then upgrade it. Update after every loop. `EXERCISED` = driven live through
the CLI on the target path. `UPGRADED` = a real defect it surfaced was fixed +
tested + committed.

Legend: ✅ done · 🔄 in progress · ⬜ not yet exercised

## Components

### ✅🔄 Campaign orchestrator — `internal/campaign`
- **Exercise:** `nerd campaign start "Audit internal/<pkg> ..." --type audit --timeout 30m`
- **Status:** heavily exercised (runs 1–13). Many upgrades landed; run 14 verifies the latest three.
- **Upgrades:**
  - F-TOOL-1 — permit `/grep` + `/search_code` in constitution.mg.
  - F-SCHED-2 — scoped task snapshot + swap-resilient scheduler (fixed completion→restart loop).
  - F-CKPT-2 — bound phase-checkpoint failure retries (no deadlock on repeated FAIL).
  - F-STALL-1 / F-TASK-1 — failed-task deadlock + pathless document tasks.
  - F-DOC-1 / F-DOC-2 — document-task fallback + degenerate-generation guard (anti-repetition retry + honest placeholder).
  - **F-DURABLE-1** (c18cbc6d) — persist research/audit task output as a durable `/doc` artifact; the reviewer had nothing to verify before this.
  - **F-HOLLOW-1** (84228fc1) — retry empty `/research` responses (empty EOF was marked "completed").
  - **F-VERIFY-1** (84228fc1) — route analytical `/verify` tasks to research+persist instead of a bare `go build`.
- **Open:** run 14 (fixed binary) must show phases 0/1/2 `/shard_validation` pass on merit. Contract #4 was the last non-A+ contract.

### ✅ FlightRecorder / tracer — `runtime/trace`, `cmd/nerd/main.go`
- **Exercise:** any `nerd campaign` run (tracer starts in main.go).
- **Upgrade:** F-TRACE-1 (bf7c4dd5) — `isCampaignInvocation()` gates the tracer off for campaigns; it OOM-crashed (`fatal error: traceRegion: out of memory`) under campaign goroutine churn. Verified live run 10.

### ✅ Shell tools — `internal/tools/shell`
- **Exercise:** reviewer/coder shards issuing `run_command`; run 11 audit.
- **Upgrade:** F-CMD-2 (bf7c4dd5, 996689d1) — cross-platform read-only builtins (ls/cat/wc/head/tail/grep/rg) + route not-found PowerShell cmdlets (`Get-ChildItem`, `Select-String`) through PowerShell on Windows. Verified run 11.

### ✅ World model — `internal/world`, `internal/core/defaults/policy/codedom_core.mg`
- **Exercise:** world scan at boot; `internal/world` self-audit (run 13).
- **Upgrade:** mock_file Cartesian explosion (62cca3e4) — `mock_file` paired every test×source (~500k, overflowed the 500000 kernel fact limit). Fixed with a `file_dir` per-package join key (scanner emits companion `file_dir` fact). Verified: world loads clean.

### ✅ Reviewer prompt atoms — `internal/prompt/atoms/shards/reviewer`
- **Exercise:** `/shard_validation` checkpoints during any campaign.
- **Upgrade:** `tool_steering.yaml` (3364305b) — steer the reviewer to dedicated tools (search_code/read_file/list_files) over fragile shell, and always emit a complete verdict. Produced the first-ever `/shard_validation` merit pass (run 12).

### ✅ Session executor + `nerd run` OODA path — `internal/session`, `cmd/nerd/cmd_instruction.go`
- **Exercise:** run 12 self-audit (4/4 phases); one-shot `nerd run "<edit intent>"` on `internal/features` (run bqe14s3v5).
- **Upgrade:** **F-TOOL-3** (498d0484) — permit `/edit_file` + `/fs_edit` as `safe_action` in constitution.mg. The safety gate (`checkSafety`, executor_tools.go:510) mapped every edit_file call to `/edit_file`, which was absent from the allowlist though `/write_file` (strictly more powerful) was permitted → `permitted(...)` default-denied every edit ("tool call blocked by safety gate: edit_file"). edit_file ⊆ write_file, so no new capability; paranoid validation still applies. **Live-verified run bqe14s3v5 (exit 0, zero gate blocks).**
- **Upgrade 2:** **F-ROUTE-2** (cf0116c0, branch `fix/audit-action-mapping`) — LIVE dogfood find: `nerd run "Analyze internal/perception ..."` classified the intent as verb `/audit` (conf 0.95) but the one-shot path hard-failed with "no action derived from policy" (exit 1). `/audit` is a `workhorse_verb` (routing_arbitration.mg:47) with no `action_mapping` in delegation.mg → the bridge rule `next_action(A) :- user_intent(_,_,Verb,_,_), action_mapping(Verb,A)` derived nothing. Added `action_mapping(/audit, /delegate_reviewer)`, mirroring `/analyze`/`/security` (both action_mapping-only, no delegate_task, rely on the next_action handoff at cmd_instruction.go:223). Deterministic kernel test (NewRealKernel → assert user_intent(/audit) → next_action(/delegate_reviewer)). Broader gap (7 more unmapped workhorse verbs) flagged as spawn_task task_b83c7ed2.
- **Flagged (not yet fixed):** F-ROUTE-1 (`/research` invalid action fact "got 2").

### ✅ Features — `internal/features`
- **Exercise:** `nerd run "review Summary() ... render *bool as true/false/unset ... use edit_file ..."` (run bqe14s3v5). codeNERD's reviewer shard independently confirmed the fix + repaired a compile-blocking test-file string.
- **Upgrade:** **F-FEATURES-1** (879ce8ec) — `Summary()` applied `%v` to eight `*bool` fields → boot log printed pointer addresses (`diff_eval=0x3e37…`). Added nil-safe `boolPtrString` (nil→`unset`, else `true`/`false`), switched to `%s`, kept the two `%d` args, documented the rationale. Tests: `TestSummaryRendersBoolPointersAsValues` (exact-string + `0x`-leak guard + nil→unset) + `TestBoolPtrString`. `go build`+`go test ./internal/features` green (independently verified, `ok 0.081s`).

### ✅ Tools/core file_ops — `internal/tools/core`
- **Exercise:** `edit_file` driven live end-to-end by the reviewer shard in run bqe14s3v5 (happy path); edge cases proven at the production tool-code layer with real-file-IO tests (no stubs). Adversarial-CLI exercise of the new guard rides the next live run transitively.
- **Upgrade:** **F-TOOLS-CORE-1** (058c45a6) — `edit_file` had two silent-corruption paths `write_file` did not. (1) Non-unique `old_text` with `replace_all` unset replaced only the FIRST match and reported "Replaced 1 occurrence(s)" → could corrupt the wrong site; now refuses when `old_text` occurs >1 time (asks for more context or `replace_all`). (2) Non-string `new_text` (model emits number/null) failed `.(string)` and coerced to `""`, silently turning the edit into a deletion; now rejects non-string/absent `new_text` with an explicit type error mirroring `write_file` — explicit `""` stays a valid deletion. 6 real-IO tests; full `internal/tools/core` package green. Same asymmetry class as F-TOOL-3 (write_file hardened, edit_file left loose).

### ✅ Config — `internal/config`
- **Exercise:** config loads on every boot (`GetReflectionConfig` runs during chat setup). The min_score:0 regime isn't in the live config, so the real-struct/real-UnmarshalJSON deterministic tests are the proof (documented limitation, not simulated by mutating `.nerd/config.json`).
- **Upgrade:** **F-CONFIG-1** (70f3eb12) — `GetReflectionConfig` treated an explicit `min_score:0` (= "no similarity floor, recall everything") as "unset" and clobbered it to the 0.70 default; the consumer (chat/reflection.go:279,298) uses it as a hard filter with no downstream defense → silently dropped most System-2 recalls. Mirrored the existing `enabledSet` zero-value-ambiguity pattern with `minScoreSet` in `UnmarshalJSON`. 5 tests; full `internal/config` package green.

### ✅ Prompt compiler / JIT — `internal/prompt`
- **Exercise:** the JIT compiler runs on every prompt build (every live run/campaign exercises it). Budget-degradation defect proven at the assembler/stats layer with deterministic tests.
- **Upgrade:** **F-JIT-1** (7c1ad5ad) — `Fit` degrades over-budget atoms to their `concise`/`min` variant, records `OrderedAtom.RenderMode`, and charges the SMALLER token count, but the assembler emitted the full standard `Content` regardless and the stats/manifest summed the standard `TokenCount`. Net: silent budget overflow, dead concise/min variants, misreporting manifest. Extracted `contentForMode`/`tokenCountForMode` (empty→standard fallback), used in assembler (emit) + compiler stats/manifest (count); `Fit`'s closure delegates to the shared helper so charge/emit/report agree. `render_mode_test.go`; full `internal/prompt` package green.

### ✅ Articulation — `internal/articulation`
- **Exercise:** the StreamParser runs on every streamed chat turn; the re-emit defect proven at the parser layer with deterministic chunk-boundary tests (no stubs).
- **Upgrade:** **F-ART-1** (215eb810, branch `fix/stream-parser-terminal-state`) — `StreamParser.ProcessChunk` had no terminal state. After the closing `"` of `surface_response`, a trailing chunk (the `}` arriving split from the quote — a common token boundary) re-entered the key-detection block, re-found `surface_response` in the retained buffer, reset `lastEmittedIndex` to the opening quote, and re-streamed the whole surface → duplicated TUI output (`HiHi`). Added a `completed` terminal flag (set at the closing quote; short-circuits after buffering so `GetFullBuffer` stays complete). 2 regression tests; full `internal/articulation` package green.
- **Note:** the empty-response symptom (`Fallback parse: empty response ... EOF`) is a separate upstream (empty subagent) issue — not yet exercised.

### 🔄 Perception — `internal/perception` (EXERCISED, no perception-layer defect)
- **Exercise:** `nerd run "Analyze internal/perception ..."` (run b6k65xprn) classified correctly — verb `/audit`, target `internal/perception`, confidence 0.95. Perception worked; the failure was downstream in policy routing → surfaced F-ROUTE-2. No perception-layer bug found in this exercise.

### ⬜ Not yet exercised / fully swept
- Kernel core — `internal/core` (fact flow, derivation). Routing layer upgraded via F-ROUTE-2 (policy) + F-TOOL-3 (constitution).
- Mangle policy corpus — `internal/mangle`, `internal/core/defaults/policy/` (F-TOOL-1/F-TOOL-3/F-ROUTE-2 landed; corpus not exhaustively swept).
- VirtualStore — `internal/core/virtual_store.go` (router test green; no defect swept yet).
- Research tools — `internal/tools/research` (context7; needs network).
- Autopoiesis / Ouroboros — tool generation (needs LLM).
- Memory / context — `internal/context` (reflection config done via F-CONFIG-1; paging/compression not swept).

## Run journal (self-audit campaigns)

| Run | Target | Binary | Result | What it taught |
|-----|--------|--------|--------|----------------|
| 10 | (tracer verify) | +F-TRACE-1 | clean, tracer live | F-TRACE-1 confirmed |
| 11 | shell/core tooling | +F-CMD-2 | failed (reviewer `Get-ChildItem` infra) | reviewer needs tool steering |
| 12 | internal/session | +reviewer steering | 4/4 clean; **first `/shard_validation` merit pass** (phase 2) | phases 0/1 failed: no durable outputs |
| 13 | internal/world | +F-DURABLE-1 (pre-hollow/verify) | phases 0/1/2 fail, phase 3 passes | reviewer now reads artifacts; exposed F-HOLLOW-1 + F-VERIFY-1 |
| 14 | internal/world | +DURABLE-1/HOLLOW-1/VERIFY-1 | **phase 0 PASSED (first ever)**; phase 1 fail; **paused** at phase 2 | exposed F-HOLLOW-2 (empty explicit-shard) + F-GREP-1 (grep hard-fail → pause) |
| 15 | internal/world | +all five fixes | **completed 5/5, 23/23, no pause; phases 0+1 PASS on merit** | all 10 contracts A+; exposed F-STUB-1 (intent-stubs on deep phases 2/3/4) |
| 16 | internal/world | +F-STUB-1 (58bea9e1) | verified: F-STUB-1 caught 172B/146B intent-stubs live | intent-stub retry works under contention (only 2/18 recovered — LLM backend degraded) |
| bqe14s3v5 | internal/features (`nerd run`, edit intent) | +F-TOOL-3 | exit 0, **zero safety-gate blocks**; reviewer confirmed fix + repaired test file | F-TOOL-3 verified: edit_file passes the constitutional gate; component #2 (nerd-run OODA path) + Features done |
| b6k65xprn | internal/perception (`nerd run`, audit intent) | +F-TOOL-3/F-FEATURES-1 | **exit 1 "no action derived from policy"** — LIVE dogfood find | perception classified `/audit` correctly (0.95); `/audit` had no action_mapping → **F-ROUTE-2** |

## Campaign orchestrator: A+ reached (run 15)

All 10 contracts satisfied. Fixes F-DURABLE-1, F-HOLLOW-1, F-VERIFY-1, F-HOLLOW-2,
F-GREP-1 on `main`; F-STUB-1 pushed on `fix/campaign-intent-stub-guard-wt`. The
checkpoint now verifies at high fidelity: passes good work on merit (phases 0-1),
fails hollow work on merit (2-4 intent-stubs), advances gracefully (no deadlock,
no pause). F-STUB-1 retries intent-stubs; its live verification (run 16) is the
next step.

## Operational hazard — concurrent working-tree churn (2026-07-14)

A concurrent automation (jules-* / thunderdome QA agents + a merge bot) actively
`git checkout`s branches, merges QA work, and resets the shell cwd on the SHARED
`C:\CodeProjects\codeNERD` working tree every few minutes. It silently wiped an
in-progress (uncommitted) fix mid-edit by switching the tree to `main`. **How to
apply:** commit + push early and often; for any multi-edit fix, work in an
isolated `git worktree add ../codeNERD-<slug>` (immune to the main tree's branch
switches) and push the branch. Note: a fresh worktree lacks the gitignored
`.nerd/config.json`, so live `nerd` runs must use the main tree (once stable) or a
copied-in config — never Write the original config.
