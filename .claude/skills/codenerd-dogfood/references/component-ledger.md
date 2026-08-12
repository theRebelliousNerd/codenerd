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
- **Exercise:** world scan at boot; `internal/world` self-audit (run 13); cold start on a wiped `.nerd/` (run 15).
- **Upgrade:** mock_file Cartesian explosion (62cca3e4) — `mock_file` paired every test×source (~500k, overflowed the 500000 kernel fact limit). Fixed with a `file_dir` per-package join key (scanner emits companion `file_dir` fact). Verified: world loads clean.
- **Upgrade:** **F-MOCK-2** (e8a6f268) — the same explosion **recurred** on run 15 (`500423 > 500000`, all scan facts dropped, kernel left with zero world facts). The `file_dir` join was intact; the defect was premise **ORDER**. Mangle evaluates premises strictly left-to-right and enforces the limit against the **intermediate** solution set (`seminaivebottomup.go:645`, `len(newsolutions)`), so with the two `file_topology` scans adjacent the repo-wide product materialised before either `file_dir` premise could filter it. Binding `Dir` from `TestFile` first keeps the peak at per-directory scale. Regression test `policy_mock_file_scale_test.go` reproduces the exact production error when reordered back.
  - **Lesson worth generalising:** a bounded *result* does not imply a bounded *evaluation*. Fact-limit errors name the rule whose intermediate join overflowed, not a rule that produced too many facts.
  - **Open:** the rest of the policy corpus has not been swept for the same two-large-scans-adjacent hazard.

### ✅ Cold start — `cmd/nerd/cmd_init_scan.go`, `internal/init`
- **Exercise:** hard-wiped `.nerd/` (2.53 GB) and ran `nerd init` against a fresh two-tier config (run 15).
- **Upgrade:** **F-INIT-1** (e8a6f268) — `nerd init` ignored `.nerd/config.json` entirely for LLM selection: it read only `--api-key` / `ZAI_API_KEY` and called `perception.NewZAIClient`. With a stale `ZAI_API_KEY` in the ambient environment the entire cold start ran on an unconfigured provider and **195 of 196 LLM calls failed** (429 / context deadline) while the summary still printed "All 10 knowledge bases validated successfully / Average KB Quality: 80%". Same class as the campaign bug (565da00e). Now resolves through the shared `newConfiguredLLMClient()`; verified `MODEL: qwen3.8-max` in `_llm_io.log`.
- **Upgrade:** **F-INIT-3** (cda328ae) — `filterDocumentsByRelevance` issued its ~196 doc-relevance batches **strictly sequentially**. At the ~16–25s per call a real API model takes, cold start needed 54–82 minutes against a 25-minute operation timeout, so it could never finish. It only *looked* fine before because every call was failing fast (F-INIT-1). Batches are independent, so they now run concurrently (pool of 8; real API concurrency still bounded by the APIScheduler). Measured: ~3.6 calls/min → ~23 calls/min. Results are collected per batch and concatenated in batch order so output is completion-order independent; verified with `-race`. Init also moved to the worker tier — doc triage is bulk labelling, which is what the cheap tier is for.
- **Upgrade:** **F-SCHEMA-1** (e9b515c1) — the piggyback envelope schema is sent with `strict: true` but never carried `additionalProperties: false`. Meta rejects that with `400 'additionalProperties' is required to be supplied and to be false.`; OpenAI documents the same strict-mode requirement, so it was latent there too. Worse, the retry-without-`response_format` fallback only matched bodies containing `"response_format"`/`"json_schema"` — Meta says `"param":"schema"` — so a *recoverable* schema complaint became a hard turn failure. Both fixed, with negative cases pinned so unrelated 400s don't silently drop structured output.
- **Upgrade:** **F-5XX-1** — the OpenAI-compat retry loop handled `429` but fell straight through on `5xx`. A single transient `500 internal server error` from Meta killed a turn that would have succeeded on retry. Now retried on the same backoff, with `501 Not Implemented` excluded (retrying an unimplemented endpoint just burns budget).
- **Upgrade:** **F-INIT-2** (2026-08-09) — init now counts every provider attempt at the shared JIT call boundary and reports succeeded/failed totals with provider/model and a bounded/redacted last error. Required artifact and structural-validation failures drive `InitResult.Success=false`; optional enrichment failures produce an explicit degraded completion. Atom-count thresholds are labeled KB population proxies instead of semantic quality. The same packet makes force init preserve user `.gitignore` and Mangle overlays with atomic create-if-absent, closes the initializer on the long-lived chat path, enforces `InitConfig.Timeout` between phases while preserving cancellation identity, and boots the kernel with the workspace before loading overlays.
  - **Live proof:** rebuilt binary review of `internal/init/initializer.go` completed in 10m01s with no blockers. Its confirmed adjacent SQLite-owner and `.gitignore` findings were repaired and focused/race-tested; 24 logs archived under `.nerd/campaigns/dogfood-20260809-init/review-2-init-truth-closure-logs`.
- **Closed:** **F-STATUS-1** (`1f2dc814`, live-verified 2026-08-09) — `nerd status` reports the configured provider/model slots; current output identifies DashScope with worker Meta and planner DashScope rather than ambient Z.AI.
### ✅ Perception / intent classification — `internal/perception`, `internal/system/factory.go`
- **Exercise:** `nerd run "review internal/features/features.go for correctness"` on the two-tier stack (run 15).
- **Upgrade:** **F-CLASS-1** (b097deff) — classification runs on **every** interactive turn before anything else, and was still served by the main client (`factory.go` built the transducer with `bctx.llmClient`). One `nerd run` spent **91 seconds** in classification alone. `NewClassificationClientFromConfig` had existed for exactly this since the P2 model-tiering work — its own doc comment calls main-model classification "a bug" — but was only wired into *shard registration*, never into the transducer. A wiring gap, not a missing feature. Now resolves worker tier → main fast tier → main-with-warning. Measured 91s → 47s, and **more accurate**: the same input classified as `/explain` with no target on the reasoning model and `/review` with the correct target on the cheap no-thinking client.
- **Verification of the whole two-tier design**, one `/review` turn in `_llm_io.log`:
  `classification → muse-spark-1.2`, `planner-tools → qwen3.8-max`, `planner-tool-results → qwen3.8-max` (×8, whole tool loop on one model).
- **Open:** 47s is still slow for an interactive turn. Next lever is a genuinely small classification model (`classification_model`), which needs a verified cheap Qwen/Meta tier name.

### ✅ Reviewer prompt atoms — `internal/prompt/atoms/shards/reviewer`
- **Exercise:** `/shard_validation` checkpoints during any campaign.
- **Upgrade:** `tool_steering.yaml` (3364305b) — steer the reviewer to dedicated tools (search_code/read_file/list_files) over fragile shell, and always emit a complete verdict. Produced the first-ever `/shard_validation` merit pass (run 12).

### ✅ Session executor + `nerd run` OODA path — `internal/session`, `cmd/nerd/cmd_instruction.go`
- **Exercise:** run 12 self-audit (4/4 phases); one-shot `nerd run "<edit intent>"` on `internal/features` (run bqe14s3v5).
- **Upgrade:** **F-TOOL-3** (498d0484) — permit `/edit_file` + `/fs_edit` as `safe_action` in constitution.mg. The safety gate (`checkSafety`, executor_tools.go:510) mapped every edit_file call to `/edit_file`, which was absent from the allowlist though `/write_file` (strictly more powerful) was permitted → `permitted(...)` default-denied every edit ("tool call blocked by safety gate: edit_file"). edit_file ⊆ write_file, so no new capability; paranoid validation still applies. **Live-verified run bqe14s3v5 (exit 0, zero gate blocks).**
- **Upgrade 2:** **F-ROUTE-2** (cf0116c0, branch `fix/audit-action-mapping`) — LIVE dogfood find: `nerd run "Analyze internal/perception ..."` classified the intent as verb `/audit` (conf 0.95) but the one-shot path hard-failed with "no action derived from policy" (exit 1). `/audit` is a `workhorse_verb` (routing_arbitration.mg:47) with no `action_mapping` in delegation.mg → the bridge rule `next_action(A) :- user_intent(_,_,Verb,_,_), action_mapping(Verb,A)` derived nothing. Added `action_mapping(/audit, /delegate_reviewer)`, mirroring `/analyze`/`/security` (both action_mapping-only, no delegate_task, rely on the next_action handoff at cmd_instruction.go:223). Deterministic kernel test (NewRealKernel → assert user_intent(/audit) → next_action(/delegate_reviewer)). Broader gap (7 more unmapped workhorse verbs) flagged as spawn_task task_b83c7ed2.
- **Upgrade 3:** **F-ROUTE-1 closed** (76c88618 + pending_action follow-up, 2026-07-22) — two halves: (1) the CLI built 2-arg `next_action` facts that VirtualStore's parser rejected ("got 2"); now builds canonical 3-arg facts. (2) Even a well-formed fact was default-denied because the one-shot path never filed the `pending_action/5` that constitution.mg requires to derive `permitted/3`; `assertPendingAction` (cmd_instruction.go) files it just before `RouteAction` — the kernel still decides (safe_action + !dangerous_content unchanged). Regression tests both layers (`TestRouteAction_CLINextActionShape_PassesParsing`, `TestCheckKernelPermitted_PendingActionOpensGate`). The second half was recovered from uncommitted work found in the codeNERD-merge worktree during the 2026-07-22 mass branch integration.

### ✅ Features — `internal/features`
- **Exercise:** `nerd run "review Summary() ... render *bool as true/false/unset ... use edit_file ..."` (run bqe14s3v5). codeNERD's reviewer shard independently confirmed the fix + repaired a compile-blocking test-file string.
- **Upgrade:** **F-FEATURES-1** (879ce8ec) — `Summary()` applied `%v` to eight `*bool` fields → boot log printed pointer addresses (`diff_eval=0x3e37…`). Added nil-safe `boolPtrString` (nil→`unset`, else `true`/`false`), switched to `%s`, kept the two `%d` args, documented the rationale. Tests: `TestSummaryRendersBoolPointersAsValues` (exact-string + `0x`-leak guard + nil→unset) + `TestBoolPtrString`. `go build`+`go test ./internal/features` green (independently verified, `ok 0.081s`).

### ✅ Tools/core file_ops — `internal/tools/core`
- **Exercise:** `edit_file` driven live end-to-end by the reviewer shard in run bqe14s3v5 (happy path); edge cases proven at the production tool-code layer with real-file-IO tests (no stubs). Adversarial-CLI exercise of the new guard rides the next live run transitively.
- **Upgrade:** **F-TOOLS-CORE-1** (058c45a6) — `edit_file` had two silent-corruption paths `write_file` did not. (1) Non-unique `old_text` with `replace_all` unset replaced only the FIRST match and reported "Replaced 1 occurrence(s)" → could corrupt the wrong site; now refuses when `old_text` occurs >1 time (asks for more context or `replace_all`). (2) Non-string `new_text` (model emits number/null) failed `.(string)` and coerced to `""`, silently turning the edit into a deletion; now rejects non-string/absent `new_text` with an explicit type error mirroring `write_file` — explicit `""` stays a valid deletion. 6 real-IO tests; full `internal/tools/core` package green. Same asymmetry class as F-TOOL-3 (write_file hardened, edit_file left loose).

### ✅ Cross-cutting file mutation paths — line-ending preservation
- **Exercise:** `nerd dom demo` surfaced `encoding_issue:2`; byte inspection found lone LF sequences injected into CRLF Go files. Three bounded live `nerd review internal/core/line_ending.go --verbose` passes then falsified the first fallback-only fix, traced the global modular-tool dispatch path, and closed with no blockers.
- **Upgrade:** **F-EOL-1** (2026-08-09) — existing LF/CRLF convention is now preserved across the production tactile `FileEditor`, VirtualStore handlers and fallback, transaction commits, global `write_file`/`edit_file`, CodeDOM `edit_lines`/`insert_lines`/`delete_lines`, and Mangle watcher repair. Multi-line LF search text now matches CRLF files in normalized space before output is restored. Missing files retain LF behavior; other detection failures abort instead of guessing. Real-file byte-count regressions cover every route. `go test ./... -count=1` passed.

### ✅ Integration execution auditor — `cmd/tools/integration_audit`
- **Exercise:** the old skill-local `audit_execution.py --component core --json` scanned 179 files including `.claude/worktrees`, emitted banner text before its JSON, returned exit 0 with ERROR findings, and reported 26 false ERRORs. codeNERD's first live review found six additional mechanical analyzer defects.
- **Upgrade:** **F-AUDIT-1** (2026-08-09) — canonical tracked command now uses exact path segments, excludes runtime mirrors, blanks comments/literals, scopes object/channel checks to enclosing functions, handles `WithCancel`, multi-return constructors, composite literals, promoted embedded fields, Bubble Tea cases/type assertions, and JSON exit semantics. Three ignored skill roots use identical launchers. 13 synthetic tests pass; live `core` audit is 0 errors/0 warnings and the full-repo machine scan is 0 errors.
- **Closed:** **F-REVIEW-TIMEOUT-1** (2026-08-09) — the native tool loop now reserves five minutes (or half of a short turn's remainder) before the outer deadline. Exploration tools and follow-ups run under the earlier cutoff; pending tool-use/result pairs are completed without replay, then one capability-reduced final uses the still-live parent context. The original 12-minute self-review replay crossed the boundary after 31 executed calls and returned a 7,930-character verdict in 10m25s. Focused session tests and `-race` pass; proof logs live under `.nerd/campaigns/dogfood-20260809-integration-auditor/review-4-deadline-reserve-proof-logs`.
- **Follow-up:** **F-SESSION-LOOP-1** (2026-08-09) — that returned verdict exposed and drove repairs for drifted constitutional target extraction, command intents incorrectly requiring file-write tools, prose-only retry response loss, returned already-executed final tool calls, silent Piggyback budget skips, zero config budgets/timeouts, invalid UTF-8 truncation, and swallowed security-violation assertion failures. The session architecture corpus was reconciled to the live 14-production-file/33-test package and the deadline-finalization contract.
- **Follow-up review:** the rebuilt 12-minute replay again crossed the reserve boundary (25 calls) and returned a 7,643-character verdict in 11m40s. Its confirmed findings drove a strict final-tool allowlist, write-intent-aware final capabilities, nil-response guards, validated Mangle verbs, bounded `pending_edit` digests, cancellation result pairing, typed hollow-success errors, grounded exact permission queries, and fail-closed `nerd.md` protection at executor, VirtualStore, and registry layers. Session race/vet and full core/projectdoc suites pass after the repairs; proof logs are in `review-5-followup-findings-logs`.
- **Final semantic review:** a third rebuilt 12-minute replay crossed the reserve boundary after 21 calls and returned a 9,281-character verdict in 8m59s. Its real findings drove mutex-backed config snapshots, one shared native/Piggyback batch path, kernel-owned `write_oriented_intent/1` with a parity-tested conservative fallback, and missing-target/missing-authority denial at every write gate. Its claimed missing durable-write identity test was independently falsified: `TestIsWriteMutationTool_CoversEveryDurableWriteAction` already enumerates the production action set. Proof logs are in `review-6-final-semantic-review-logs`.
- **Closure review:** the rebuilt CLI returned a 5,476-character verdict in 9m06s with no critical/high finding and explicitly recognized the existing durable-write identity test. Its one plausible medium finding was real: Piggyback returned immediately after execution and bypassed build, test, coverage, and critic gates. `verifyCompletedToolTurn` now owns those gates for every transport and terminal path, including forced finals and one-shot native fallback; Piggyback passes clean work and fails red build/tests with grounded evidence because it has no native repair channel. The review also found one inconsistent empty-action audit guard, now removed. Proof logs are in `review-7-transport-gate-closure-logs`.

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

### ✅ VirtualStore — `internal/core/virtual_store_actions.go`
- **Exercise:** the action router dispatches every kernel-derived `next_action`; `handleGitOperation` is on the git commit/add path that F-TOOL-3's live run exercised. Sibling handlers `handleRunTests`/`handleBuildProject` already keyed success off the exit code — `handleGitOperation` did not.
- **Upgrade:** **F-VS-1** (788e664d, branch `fix/vs-git-exit-code`) — `handleGitOperation` set `success = err == nil`, so a git command that ran to completion but exited non-zero (e.g. `commit` with nothing staged, a failed `push`) was reported as a **success** in both `ActionResult.Success` and the `git_result` fact the kernel reads back. Changed to `success = err == nil && result.ExitCode == 0`, mirroring `handleRunTests` (line 297) / `handleBuildProject` (line 346). Test `TestHandleGitOperation_NonZeroExitIsFailure` drives a mock executor returning `{ExitCode:1, err:nil}` and asserts `!res.Success`.
- **Upgrade:** **F-VS-2** (c9376918, branch `fix/vs-execcmd-exit-code`) — closes the hold-out F-VS-1 flagged: legacy `handleExecCmd:160-167` had the same err-only success signal. `tactile.DirectExecutor` returns `(result{Success:true, ExitCode:N≠0}, nil)` when a command **ran but exited non-zero**, so a failed command was reported `Success:true` **and asserted a false `cmd_succeeded(binary, output)` fact into the kernel EDB** — policy reasoning over `cmd_succeeded`/`cmd_failed` was driven by a lie. Changed to require `result.Success && result.ExitCode == 0`, mirroring `handleExecCmdModern` (line 194). Production defaults to the modern executor, so the live blast radius is the safety/gaps test surface (legacy path via `DisableModernExecutor`). Test `TestHandleExecCmd_NonZeroExitReportsFailure` (mock `{Success:true, ExitCode:1, nil}`) asserts `!res.Success` and no `cmd_succeeded` fact; **proven red** (buggy version returns `Success:true` + `cmd_succeeded("bash","boom")`) **and green**.

### ✅ Code DOM CLI — `cmd/nerd/dom_cmd.go`, `dom_apply_cmd.go`
- **Exercise:** `nerd dom demo` live end-to-end (2026-07-22): open → 7 elements parsed → semantic edit of `fn:domdemo.Add` → verify → `go test` PASS in the demo workspace → close_scope.
- **Upgrade:** **F-DOM-1** (2026-07-22) — every `nerd dom demo|inspect|get|edit|apply` was dead since e5d8bfe4 removed the safe_action fast-path from `CheckKernelPermitted`: the DOM commands route `next_action` facts directly (no executive shard) and never filed the `pending_action/5` that constitution.mg requires to derive `permitted/3` → default-deny at the first `open_file`. New shared `routePermittedAction`/`filePendingAction` (cmd/nerd/pending_action.go) files the request with the canonical payload JSON (mirrors `parseActionFact`), routes, retracts; all 14 DOM call sites plus the one-shot `nerd run` path go through it. Also fixed malformed edit/get/apply facts that put the action *type* in the ActionID slot — those mis-parsed even before the gate denial. The gate itself is untouched (default-deny preserved; the kernel still decides).

### ✅ Memory / context — `internal/context/activation.go`
- **Exercise:** the context compressor's `compressor_metrics.go:455` selects which scored atoms survive within the token budget on every context assembly. Kernel priority scores land at ≤100.
- **Upgrade:** **F-CTX-1** (368a64ab, branch `fix/ctx-prefiltered-budget`) — `SelectWithinBudget` unconditionally runs `FilterByThreshold` (default `ActivationThreshold=105`) before the budget greedy loop, so kernel-scored atoms (all ≤100) were pruned to nothing → the budget path silently dropped every kernel candidate. Added `SelectWithinBudgetPreFiltered(scored, budget)` (budget-only greedy loop, no threshold refilter) and switched the already-filtered call site to it. Test `TestSelectWithinBudgetPreFiltered` proves ≤-threshold atoms survive on budget alone.

### ✅ Research tools — `internal/tools/research`
- **Exercise:** the researcher shard's `context7_fetch` / `web_fetch` / `web_search` tools each read an integer cap (`max_docs` / `max_length` / `max_results`) from LLM-supplied JSON args. JSON numbers decode to `float64`, never `int`.
- **Upgrade:** **F-RESEARCH-1** (ac104f50, branch `fix/research-numeric-args`) — every tool used `args["max_*"].(int)`, which **always fails** on the production JSON path (`float64`), silently discarding the caller's cap and falling back to the default. Added `argInt(args, key) (int, bool)` (handles `int`/`int64`/`float64`/`json.Number`) and routed all three tools through it. Table test `TestArgInt` includes the `float64` production case. Mirrors the documented `factArgAsInt`/`coerceInt`/`payloadInt` helper family. Ouroboros has the same `tool_version` int64 pattern — flagged as a follow-up (low blast radius, no live consumer), not fixed here.

### ✅ Autopoiesis / Ouroboros — tool generation
- **Exercise:** scouted the accept/reject decision paths of the tool generator. Safety-critical gates (forbidden imports, dangerous operations) **fail closed** — a generated tool that trips a safety check is rejected, verified sound. No safety defect found. Then followed the flagged `tool_version` counter into a live integration test.
- **Upgrade:** **F-OURO-1** (branch `fix/ouroboros-version`, 5aab3ba0 + 20505c8e) — `OuroborosLoop.hotReload`'s tool-version counter was stuck at 1 forever. Root cause (verified by diagnostic, not assumed): it read the current version via `?tool_version(Tool, V)` with `V` **unbound**, which violates the schema mode decl `tool_version ... bound [/string, /string]` (schemas_state.mg) and returns nothing → the version was always recomputed to 1. (The int write was a secondary type inconsistency — it persisted as a *number*, not a failed write.) Fixed to read via `engine.QueryFacts` (a direct EDB scan that bypasses the query engine's mode check and works with `AutoEval=false`, as the loop uses), take max+1, and write a string to match the decl. Extracted `versionFromBinding` (robust to string/int/int64/float64). Two tests: `TestVersionFromBinding` + `TestHotReload_IncrementsVersion` (two hot-reloads → version 2, real schema + AutoEval=false); **proven red** (stuck at `[1]`) **and green**. Low blast radius (no consumer reads the counter), but it now actually increments.

### ✅ Mangle policy corpus / intent routing — `internal/core/defaults/policy/`
- **Exercise:** the `action_mapping(Verb, Action)` → `next_action(Action)` derivation (delegation.mg:242-245) fires on every `nerd run`; scouted the FULL perception verb taxonomy (taxonomy.go) against the `action_mapping` facts to find verbs that classify correctly but derive no action.
- **Upgrade:** **F-ROUTE-3** (branch `fix/route-optimize-verb`, 276bceda + 6dbefd53) — the four `/mutation`→`/coder` verbs `/optimize` (taxonomy.go:760), `/migrate` (755), `/format` (795), `/scaffold` (785) had **no `action_mapping`**, so `nerd run "optimize/migrate/format/scaffold ..."` derived no `next_action` and died with "no action derived from policy" (exit 1) — the identical F-ROUTE-2 gap `/audit` had. Added `action_mapping(/<verb>, /delegate_coder).` for all four (the complete unmapped `/mutation`→`/coder` set; `/document` is intentionally `/delegate_researcher`). Each routes to the coder via the `next_action` handoff (cmd_instruction.go:223-237) and, because `/delegate_coder` is `side_effecting_action`, makes `intent_requires_tool_call` true (anti-hollow). Table test `TestPolicyDerivesNextAction_MutationVerbsRouteToCoder` over all four; **proven red** (each derives `[]` without its mapping) **and green**.
- **Residual — now closed** (branch `fix/route-lint-bench-profile`, 1f931726): `action_mapping(/lint, /delegate_reviewer)` (analysis → prose verdict OK, stays non-side-effecting) and `action_mapping(/benchmark|/profile, /delegate_tester)`. The design question is resolved: the **tester executes** (runs tests/benchmarks/profiles), so `/delegate_tester` was added to `side_effecting_action` (it was an orphan atom — `/test` routes via `/run_tests` — so zero collateral), making `intent_requires_tool_call` true for those verbs (anti-hollow). Two tests: routing table (red `[]`→green) + `TestPolicyIntentRequiresToolCall_TesterVsReviewer` (benchmark/profile require a tool_call, lint does not) with an isolation red proof for the side_effecting fact. **All 7 F-ROUTE-3 workhorse verbs now route.**
- **Prior routing upgrades in this corpus:** F-TOOL-1 (permit `/grep`+`/search_code`), F-TOOL-3 (`safe_action(/edit_file)`/`/fs_edit`), F-ROUTE-2 (`/audit`→`/delegate_reviewer`).
- **Upgrade:** **F-MG-EXPLODE-1** (2026-07-22) — Cartesian/derivation-explosion audit of the Code DOM corpus (precedent: the mock_file T×S blowup). Three defused: (1) `proven_safe_edit` was an O(N³) three-way self-join over `code_edit_outcome`, which grows per edit during campaigns — replaced with `fn:count` aggregation over `successful_edit` (same distinct-refs semantics, linear); (2) `interface_impl` materialized a per-file struct×interface product of FALSE facts (no method matching, zero consumers) — rule removed, Decl kept, real implementation belongs in go/types; (3) `deny_edit(/auth_removed)` had a contradictory body (`!P ∧ P`) — a silently dead auth guard, documented out pending before/after snapshot facts. Also: RuleCourt sandbox now sets a derivation-bomb ceiling (4×fact base + 10k) so explosive candidate rules veto deterministically via the engine fact limit and a timed-out evaluation goroutine dies fast (Mangle eval is uncancellable). Cleared as safe: `impact_graph` (depth-capped 3), `impacted` (unary head), `test_depends_on_transitive` (recursion only via method_of/type_embeds; code_element is scope-bounded), knowledge.mg bridges (linear).

### ✅ Kernel fact boundary / numeric typing — `internal/core`, `internal/types`
- **Exercise:** live dogfood session, idle TUI. `_kernel.log` carried `evaluate: fixpoint evaluation failed: value 110 (4) is not a number` **1257 times, ~4 per 2s** (the `autopoiesisOrch.StartKernelListener` tick). Type tag `4` is `ast.Float64Type`.
- **Upgrade:** **F-NUM-1** — a Go `float64` in a slot declared `/number` took the **entire kernel offline**, silently. Three compounding facts: (1) `types.Fact.ToAtom` is Decl-blind and maps every `float64` to `ast.Float64`; (2) the pinned fork implements `<,<=,>,>=` over **int64 only** — `builtin.go` routes each through `getNumberValues`→`getNumberValue`, which errors on any `Type != NumberType`; `getFloatValue` exists but **has no caller**, so there is no float comparison in the language at all; (3) that error propagates out of `EvalStratifiedProgram`, so `evaluate()` returns at `kernel_eval.go:299` **before** `k.store = baseStore` — every derivation in the kernel is lost, on every pass, and the message names only the value, never the predicate. The corpus declares **zero** `/float64` bounds, so a Float64 in the EDB is *always* a bug here.
  - **Boundary fix:** `kernel_fact_decl.go` — `factToAtomLocked` now applies Decl-directed coercion at the single choke point (`addFactIfNewLocked`, the `evaluateFullLocked` cache rebuild, and the in-place update path). Integral float → `ast.Number`; non-integral → rejected with predicate + arg index. `evaluateFullLocked` evicts an unconvertible fact and keeps `facts`/`cachedAtoms`/`factIndex` in lockstep instead of aborting. New `atomCacheStale` flag forces one reconversion after a policy rebuild and for facts admitted pre-first-eval (when no Decls exist yet).
  - **Source fixes:** `normalizePercent` returned `float64` into `tool_learning` (`/number`); `feedback.go:521` wrote `%.2f` **into generated Mangle source** persisted to `learned.mg`, so a float poisoned every later boot; `decomposer.go` fed raw 0..1 ratios into seven `intelligence_*` `/number` slots. All now go through the new `types.PercentScale`.
  - **Corpus fix:** 17 rules compared `/number` args against **float literals** (`AvgQuality < 50.0`, `Confidence > 0.7`, `Coverage < 0.3`, `Weight > 0.5`, …) across `autopoiesis.mg` + `intelligence.mg` — each a latent whole-kernel outage armed the moment a matching fact appeared. Rescaled to integer percent.
  - **Tests:** `TestFactBoundary_*` (**red proof reproduces the exact production error**, `value 10 (4) is not a number`, and shows one bad fact killing an *unrelated* `context_priority` query), `TestCorpus_NoFloatLiteralInComparison` (static ban, regex self-tested), `TestPercentScale*`.
  - **Residual closed by F-VPRED-1:** virtual/external predicates built by `appendAtom` bypass `k.facts` and the Decl-aware coercion. Source audit found raw activation and strategic-confidence floats plus missing `strategic_knowledge`/`trace_stats` Decls; trace duration was verified as `int64`. Source typing, direct AST tests, and real-kernel assertion now cover the affected output families. The tactile `test_coverage` arity/type defect found in this sweep is closed by F-TACTILE-1 below.

### ✅ Tactile analyzer facts and audit lifecycle — `internal/tactile`, `internal/core`
- **Exercise:** rebuilt codeNERD reviewed `internal/tactile/audit.go` live three times (9m03s pre-fix, 10m41s follow-up, 9m48s final). It found incompatible analyzer predicate arities/semantics, float64 coverage at a `/number` boundary, audit handle/error gaps, missing wrapper events, PASS/FAIL ordering, killed-result misclassification, secret argv exposure, and the dormant analyzer integration. The final verdict had no blockers; every actionable correctness finding was then source-verified, repaired, and independently gated.
- **Upgrade:** **F-TACTILE-1** (2026-08-09) — analyzer output now uses dedicated, Decl-aligned `execution_test_summary`, `execution_test_state`, `execution_failed_test`, `execution_test_coverage`, `execution_build_summary`, and `execution_diagnostic` facts; it can no longer overwrite the global TDD `test_state`. Completed `go test`/`go build` events append those analyzer facts to the live audit→kernel stream. Coverage is an integer percent; build diagnostics retain column and Windows paths; bare/detail failures are sticky; exit code is authoritative. A real `RealKernel` injection/query test proves the production event path, including string tags named `success`/`none`. Audit files are owner-only, redact env/stdin/known secret argv, bound outputs on UTF-8 boundaries, expose sink failures in metrics, recover rotation where possible, close replaced sinks, and wrappers provide fallback lifecycle events when no callback is available. Killed results remain killed with timeout errors; duration averages count every timed completion. Focused tactile/core, race, vet, and schema gates pass.

### ✅ Virtual predicate and durable trace boundary — `internal/core`, `internal/store`
- **Exercise:** rebuilt codeNERD's pre-fix review found session turns silently becoming zero and per-shard results reconstructed from a global duration/top-ten/five-sample report. The accompanying exact-source audit found raw 0..1 floats crossing `/number`, missing Decls, and swallowed hydration errors. The post-fix live review found the hidden `NewKernelTx` panic and stale prior-turn context after a partial query failure.
- **Upgrade:** **F-VPRED-1** (2026-08-09) — LocalStore now exposes one exact shard-filtered aggregate; VirtualStore emits deterministic Decl-compatible names/numbers, preserves stored turn integers, defaults absent trace categories to `/unknown`, and surfaces query/assert/commit failures. Session hydration requires transactional support, atomically replaces stale context with the fresh partial snapshot, and returns the committed count alongside joined source warnings. Direct AST-type, exact small-sample SQL, transaction/error, and real-kernel assertion tests pin the boundary. Rebuilt live review reported no critical issue after the repair and supplied the final panic/stale-context corrections.
- **Verification boundary:** exact changed-path race, focused core/store, vet, real-kernel schema, and core strict+verify gates pass. The default full-package fan-out still timed out four heavy packages that pass separately. The apparent broad-core ANTLR race was subsequently localized and closed by F-MANGLE-PARSE-1 rather than retained as an unverified parser defect.

### ✅ Mangle parser concurrency boundary — `internal/mangle`, `internal/core`, `internal/system`
- **Exercise:** first-race capture of the broad core suite reproduced the old report in `TestCortexKernel_WhenConcurrentTransactions_ShouldNotDeadlock`. Git history at `a0e6e1cf` documented a residual third-party parser race, but the complete stack showed the current trigger was a test lifecycle defect: `Wait` ran before any `Add`, so the test could pass immediately and leak workers into later tests.
- **Upgrade:** **F-MANGLE-PARSE-1** (`8162177c`, 2026-08-09) — the transaction test registers every worker before starting its bounded waiter. Sanitizer, synth compiler, both system fact adapters, and test helpers now use the same process-wide `ParseUnit`/`ParseAtom` lock as core. A whole-module AST source guard rejects raw parser selectors outside `parse_lock.go`, including function references, and a mixed ParseUnit/ParseAtom/sanitizer/synth integration test exercises the boundary concurrently. Rebuilt codeNERD's 10m31s self-review returned PASS/no blockers and prompted the whole-module/test hardening.
- **Verification boundary:** five focused transaction-race repetitions, five mixed-caller race repetitions, the complete core concurrency slice under race (75.357s), serial core/Mangle/system package tests, and vet pass. A full 799-test core race run reached its 10-minute package timeout without reporting a race; it is not relabeled green. The separate default `go test ./...` fan-out timeout was subsequently closed by F-SUITE-1.

### ✅ Go suite liveness and hermetic command tests — CLI, session, core, browser
- **Exercise:** ran the complete repository suite both with bounded package parallelism and with Go's default 32-package fan-out, then captured the first timeout stacks instead of treating individually green packages as proof. The stacks showed ordinary CLI tests reaching the configured live LLM, `runInit` leaking signal waiters, noninteractive `gofmt`/`go test`/`git` children entering Windows `os.DevNull`/`GetConsoleMode`, and a browser negative-selector probe consuming the shared 60-second lifecycle deadline.
- **Upgrade:** **F-SUITE-1** (2026-08-09) — `runInit` now inherits the Cobra context through `signal.NotifyContext`, cleans up signal registration, and exposes a narrow LLM-configurer seam so filesystem command tests cannot contact the user's provider. Its workspace config is loaded once; Context7 resolves from that target workspace; `--api-key` updates the selected provider's real root key and warns when init deliberately routes through a different worker. `runScan` inherits cancellation, normalizes `-w`, boots the kernel at that workspace as an explicit validation gate, and cannot persist its preferred DB cache or `scan.mg` before the facts pass evaluation. An uninitialized scan is a real command failure. Hollow tests that booted the full Cortex merely to accept any timeout were replaced with exact argument/budget contracts; query output is tested as a pure renderer. `processutil.NonInteractive` supplies deterministic EOF without replacing explicit stdin and now protects the proven CLI gofmt, session `go test`, and core git call sites. Browser negative selector probes receive independent one-second contexts, so the live lifecycle reaches React reification, session forking, and shutdown instead of accepting later `context deadline exceeded` results.
- **Verification boundary:** `GOEXPERIMENT=nogreenteagc; go test -p 4 ./... -count=1 -timeout=15m` passed (slowest package: core 138.526s). The standard `go test ./...` also passed (slowest package: core 573.983s), proving the old hard timeout closed while retaining the default command gate. The final affected-package rerun passed (`core` 132.587s, live browser 5.493s, CLI 6.062s); targeted race and vet passed. Rebuilt codeNERD's 7m13s self-review reported no blockers, found the DB-before-validation durability bug and the cross-provider worker warning, and both findings were closed with regression coverage. Default 32-package fan-out remains substantially slower than `-p 4` on this host; that is recorded as an environment/throughput characteristic, not a false functional failure.

### ⬜ Not yet exercised / fully swept
- Kernel core — `internal/core` (fact flow, derivation). Exercised every run; routing layer is well-swept (F-ROUTE-2/-3, F-TOOL-1/-3), numeric typing is swept (F-NUM-1), and the durable virtual-predicate boundary is swept (F-VPRED-1). Deeper rule-liveness and virtual-predicate performance work remains open-ended.

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
| (scout+code) | internal/core VirtualStore git path | +F-VS-1 | `handleGitOperation` reported non-zero git exit as success | exit-code success signal; deterministic mock-executor test |
| (scout+code) | internal/context activation budget | +F-CTX-1 | kernel-scored atoms (≤100) pruned by threshold refilter | pre-filtered budget selector; deterministic test |
| (scout+code) | internal/tools/research numeric args | +F-RESEARCH-1 | `.(int)` always fails on JSON `float64` → caps silently dropped | `argInt` coercion helper; table test incl. float64 path |
| (scout+code) | policy corpus verb-taxonomy vs action_mapping | +F-ROUTE-3 | 4 `/mutation`→`/coder` verbs (`/optimize` etc.) derived no `next_action` → exit 1 | full-taxonomy sweep found the F-ROUTE-2 gap class; table test red→green; residual 3 verbs → task_dc40000b |
| (scout+code) | kernel fact-flow: legacy `handleExecCmd` | +F-VS-2 | non-zero exit reported `Success:true` + false `cmd_succeeded` fact into EDB | closes F-VS-1's flagged hold-out; red proof shows the false fact being asserted |
| (scout+code) | policy corpus: `/query` verbs `/lint` `/benchmark` `/profile` | +F-ROUTE-3 residual | 3 verbs derived no `next_action` → exit 1; tester verbs needed a side-effecting decision | `/lint`→reviewer, `/benchmark`/`/profile`→tester (executes → side_effecting); 2 tests + isolation red proof; closes task_dc40000b |
| (scout+code) | autopoiesis: Ouroboros `hotReload` tool_version | +F-OURO-1 | version counter stuck at 1 (unbound read violates `bound` mode decl) | QueryFacts mode-safe read + string write; integration test 2 hot-reloads→v2, red `[1]`→green; closes task_3b01a1a7 |
| (live review+code) | core/store virtual predicate boundary | +F-VPRED-1 | zeroed turns, raw floats, thresholded/global trace reconstruction, swallowed hydration failures | exact typed facts + per-shard SQL + atomic partial snapshot; live post-fix review found and closed panic/stale-context residuals |

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

## Dream/shadow consulted daemon shards — F-DREAM-3, F-CAMP-2 (2026-08-08)

`nerd dream <scenario>` ran its full 25-minute budget, EXIT=124, and emitted 376
bytes: the header, the "Consulting 9 agents" line, and nothing else. No agent
perspective, no error.

**Root cause (F-DREAM-3), `internal/core/shards/manager.go`.**
`ListAvailableShards` typed factory-registered shards from a hardcoded list of
six system-shard names and labelled everything else `ephemeral`. The list had
drifted from the registry: `mangle_repair`, `campaign_runner` and `legislator`
all register `system` profiles (`internal/shards/registration.go`) but were
reported `ephemeral`. dream and shadow filter on `ShardTypeSystem`, so they
consulted all three. `CampaignRunnerShard.Execute` is an infinite supervision
loop that only returns on cancellation — dream waited on it until its own
deadline. Fixed by reading the type off the registered profile; the name list
survives only as a fallback for a factory with no profile. `fcf0e849`

**Second-order (F-CAMP-2), `internal/shards/system/campaign_runner.go`.** Once
consulted, the runner did not merely idle — it loaded a real 22-task campaign
off disk and tried to start it 299 times in 25 minutes, once every 5 seconds,
each blocked by the same gate ("mandatory northstar safety review missing").
`restartBackoffSec` doubled only when `LoadCampaign` failed; when the load
succeeded and `orch.Run` returned the error, `tick()` **reset** the backoff to 5.
The common failure path defeated the backoff the rare one set up. 850 attempts
across the day. Fixed by codeNERD via `nerd fix`. `ed1f96c1`

**How to apply.** Two lessons. (1) A hardcoded name list beside a registry always
drifts; derive the type from the registry. (2) Dream's contract is "do NOT
execute any actions" — but `DreamMode` is honoured in the session executor path
(`executor.go`, `spawner.go`, `task_executor.go`) and *not* by system shards'
own `Execute`. A system shard reached through dream will still do real work.
That gap is closed here by not consulting them at all, not by the flag.

**Verification note.** codeNERD's regression test initially appeared to pin the
backoff, but against the pre-fix file it *panicked* on a nil `Kernel` rather than
failing the assertion. Re-checked by reverting only the backoff logic while
holding the new nil-guards constant: it then failed with
`restartBackoffSec = 5, want 10`. A regression test that fails for the wrong
reason has not pinned anything.

## Collaborators that were settable, required, and never set (2026-08-08)

Three defects of one shape, all found by sweeping the LLM-consuming verbs.

**F-NORTH-1 — `OrchestratorConfig.NorthstarObserver` set at 2 of 5 call sites.**
`risk_scoring.go:238` refuses any campaign whose targets touch a protected root
(`internal/core`, `internal/mangle`, `internal/campaign`, `internal/perception`,
`internal/articulation`) when the observer is nil, so every campaign started
from the TUI against the core of the codebase was refused. One was refused 850
times in a day. `northstar_wiring_test.go` had guarded `wireIntelligenceComponents`
all along and passed throughout, because the wiring function was never the broken
part — the call sites were. Fixed at `6d3d3f8d`; the new test parses every
`OrchestratorConfig` literal in the repo and names the file:line of any that omits
the field, and fails if it finds fewer than five literals so a rename cannot turn
it into a test that checks nothing.

**F-CFG-1 — `core_limits` reached two executors out of five.**
`max_tool_iterations: 24` was applied only by `factory.go` and the chat boot path.
`NewSubAgent` and both CLI campaign paths built an Executor and never called
`SetConfig`, so they silently ran on `DefaultExecutorConfig`: 8 iterations, empty
WorkspaceRoot. The boot log printed "120 calls / 24 iterations" while session.log
printed "Max tool iterations reached: 8" the same day, and a `nerd refactor` run
wrote "No exploration budget left" into the file it was editing. Fixed at `4cd233de`.

**F-UX-1 — a working verb was indistinguishable from a hang.** No output between
"Spawning" and the result; `nerd analyze` went silent for 12 minutes while making
43 tool calls. I killed it at 600s believing it deadlocked. Heartbeat added at
`72e83cf7`.

**How to apply.** The recurring shape is a dependency that is optional in the type
system and mandatory in behaviour. Grepping for the setter finds the definition and
the one site that uses it, which reads like the feature is wired. The question that
actually finds these is "how many places CONSTRUCT this, and do all of them set it?"
— `grep -rn "SomeConfig{"` and count. Two of five, two of five, and three of five.

## What codeNERD did and did not do for itself (2026-08-08)

It fixed F-CAMP-2 outright with a real regression test. On the three larger tasks it
produced sound designs and correct partial work, then ran out of tool budget and said
so rather than claiming completion — which is the behaviour that made delegating to it
worthwhile. Three failure modes to expect and check for:

- It left the tree not compiling twice (an import orphaned by a move; six imports
  duplicated). Always `go build ./...` after handing it a task.
- It invented `Cortex.SuccessfulWriteTools()` and wrote a comment asserting the
  plumbing existed. The underlying field was real; the accessor was not.
- Its regression test for F-CAMP-2 failed against the pre-fix code by PANICKING on a
  nil Kernel, not by failing its assertion. Re-check that a regression test fails for
  the intended reason, holding unrelated new guards constant.

Sizing matters: single-file, single-function tasks completed; multi-file tasks
(hoist a function across packages, then wire three call sites, then write a test) hit
the 24-iteration ceiling every time. Split them.

---

## F-PERM-1 — the five permissioned actions are unreachable by construction

**Status: OPEN. Needs a human decision; deliberately not fixed by Claude or by codeNERD.**

Symptom: codeNERD asked to delete two obsolete test files. It emitted
`/delete_file` six times, then `/run_command` twice and `/bash` once trying to
route around the refusal — nine blocked tool calls, nine LLM round trips — and
finally degraded to truncating both files to 17-byte `package campaign` stubs.

Evidence (`.nerd/logs/20260810_232614...`): the kernel received
`pending_action("call_...", /delete_file, "internal/campaign/orchestrator_callsite_test.go",
"{\"confirmed\":true,\"path\":\"...\"}")`. Confirmation *was* present. The
following `permitted(/delete_file, ...)` query returned nothing.

Root cause: `confirmed:true` is required by the tool schema
(`internal/tools/core/file_ops.go:482`), enforced at `:514`, and covered by a
passing test — and it is never translated into any kernel fact. The Go
confirmation and the Mangle permission derivation are two systems that never
meet. Tracing every `permitted` route in `constitution.mg`:

| Route | Status |
|---|---|
| `safe_action(/delete_file)` | absent — it is `requires_permission` → `dangerous_action` |
| `dangerous_action` + `signed_approval` + `admin_override` | neither fact is ever asserted outside tests |
| `permitted_action` + `permission_check_result(/permit)` | **circular** — the gate emits these *after* deciding |
| `has_active_override` ← `appeal_granted` | `HandleAppeal` has **zero production callers** |

`escalateToUser` asserts `escalation_needed`, which nothing consumes. So the
whole appeal-and-escalation apparatus is built and unreachable, and all five
`requires_permission` actions are dead: `/delete_file`, `/git_push`,
`/git_force`, `/run_arbitrary_command`, `/system_modify`.

The uncomfortable part: `/write_file` **is** a `safe_action`. The agent may
overwrite any file with any content, which is how it routed around the block.
The gate did not prevent destruction; it forced an uglier form of it and left
artifacts resembling the scratch-file failure mode.

Proposed minimal rule, for `internal/core/defaults/policy/constitution.mg`
(escaped-quote form has precedent at `:387`):

```
permitted(/delete_file, Target, Payload) :-
    pending_action(_, /delete_file, Target, Payload, _),
    :string:contains(Payload, "\"confirmed\":true"),
    !dangerous_content(/delete_file, Target),
    !dangerous_content(/delete_file, Payload).
```

Residual protections, all verified to still apply to `delete_file`: the tool
layer's required `confirmed`; nerd.md `project_forbidden_path`, since
`delete_file` is in `projectdoc.IsWriteMutationTool`; and the Dreamer
destructive-action preflight. These are the same layers `/write_file` already
passes through while being `safe_action`.

Recommendation: adopt for `/delete_file` only; leave `/git_push` and the other
three gated. The alternative — wiring `HandleAppeal` to a real prompt — is the
better long-term answer but is a feature, not a fix.

Not applied unilaterally: widening a safety gate is the user's call, and
`.claude/settings.json` `permissions.deny` correctly prevents Claude from
self-granting it.

---

## F-SWEB-1 — the SWE-bench harness has no way to load instances

**Resolves the long-standing "adopt or delete `internal/tactile/swebench`" question: ADOPT.**

The north star is 100% on SWE-bench. The execution machinery for that exists and
is wired: `internal/core/virtual_store_python.go` implements
`handleSWEBenchSetup`, `handleSWEBenchApplyPatch`, `handleSWEBenchRunTests`,
`handleSWEBenchSnapshot`, `handleSWEBenchRestore` and teardown; they are routed
through the VirtualStore and `constitution.mg:196-199` marks
`/swebench_snapshot`, `/swebench_restore`, `/swebench_evaluate` and
`/swebench_teardown` as `safe_action`. The design is right for this repo: the
handlers assert `swebench_instance`, `swebench_environment`,
`swebench_expected_fail_to_pass` and `swebench_expected_pass_to_pass` as kernel
facts, so resolution is a logic question rather than a Go one.

The gap: `handleSWEBenchSetup` reads `instance_id`, `repo`, `base_commit`,
`problem_statement`, `fail_to_pass` and `pass_to_pass` out of `req.Payload`.
Something has to supply them, and nothing does. A repo-wide search finds no
dataset loading on the live path at all.

The missing piece already exists, unimported: `internal/tactile/swebench` holds
`LoadInstances(path)`, `Instance`, `Prediction` and `EvaluationResult`, with
passing tests and **zero importers**. So the two halves of a SWE-bench run were
built separately and never joined — the dataset reader has no caller, and the
executor has no data source.

Conclusion: do not delete the package. Adopt its instance/dataset layer and let
it drive the existing live handlers through the action path. Do NOT adopt its
parallel Go `Harness` (Initialize/Setup/Evaluate/EvaluateWithReset) — that
duplicates the routed handlers imperatively and would move resolution out of the
kernel, against the architecture the live path already implements.

Suggested slicing, because multi-file tasks reliably hit the tool-iteration
ceiling: (1) a command that loads a dataset file and drives ONE instance through
setup via the action path, proving the join; (2) patch + test + evaluate for that
instance; (3) batch iteration and scoring.

Status: OPEN, unblocked, no policy decision needed.

### F-SWEB-1 addendum — the pipeline is complete except for its entry point

Correcting the first pass, which undercounted the permitted actions. The full
SWE-bench path is already in place:

- **Handlers**: `handleSWEBenchSetup`, `ApplyPatch`, `RunTests`, `Snapshot`,
  `Restore`, `Evaluate`, `Teardown` in `internal/core/virtual_store_python.go`.
- **Routing**: dispatched via `virtual_store_routing.go`.
- **Permission**: `constitution.mg:193-199` marks **all seven** `/swebench_*`
  actions `safe_action` — not four, as first reported.
- **Intent derivation**: `capabilities.mg:45-51` derives each `next_action` from
  `user_intent(..., /swebench, "<subcommand>", _)`.

So policy, permission, routing and execution are all wired. What is missing is
narrower and more embarrassing than "the feature is unfinished":

1. **No CLI entry point.** `nerd --help` lists no `swebench` command. The verb is
   reachable in principle through `nerd run` if perception classifies an
   utterance as `/swebench`, but nothing exposes it directly.
2. **No dataset reader on the live path.** `handleSWEBenchSetup` takes
   `instance_id`, `repo`, `base_commit`, `fail_to_pass` and `pass_to_pass` out of
   `req.Payload`. Those fields cannot come from natural language; they have to be
   read from the benchmark dataset. `LoadInstances` does exactly that and lives in
   `internal/tactile/swebench`, still with zero importers.

`cmd/nerd/pending_action.go` is the seam a new command should use: it files the
permission facts a CLI-driven `RouteActionResult` needs, and its own comment warns
that a bare `RouteAction` must go through it.

So the whole north-star capability is one command and one import away from being
exercisable. That is the highest-value unblocked work outstanding.

---

## F-AUTO-4/5 — Ouroboros ships tools that compile, register, and answer the wrong question

**Ouroboros was completely unable to generate a tool until 079bc2ab.** The
entry-point contract was split across three files that agreed nowhere:
`tool_compiler.go` accepts only `func(context.Context, string) (string, error)`
(`findEntryPoint` deliberately skips `main`, because `writeWrapper` generates its
own), `tool_generation.go` asks the model for whatever types the `ToolNeed`
carries, and `cmd/nerd/cmd_advanced.go:buildCLIToolNeed` — the path
`nerd tool generate` actually uses — never set those types. The prompt went out
as `(ctx context.Context, input ) (, error)`. Given no contract, the model
invented one and the compiler refused it.

Note the shape of the first fix attempt, because it is the recurring failure of
this codebase: coercion was added at the two construction sites that were known,
the third was missed, the build was green, a unit test passed, and the feature
was still 100% broken. Enforcement now lives at `GenerateTool`, the choke point
every path crosses. **Fix at the choke point, not at the construction sites.**

### The part that still is not fixed

The generated tool now compiles, registers, and runs — and is wrong:

```
ground truth (grep '^\s*Decl ' schemas_safety.mg):  50
the self-generated tool:                             0
```

Asked to "count the number of Mangle Decl statements", it searched for the
literal phrase `(?i)\bmangle\s+decl\b` — reading the request as *find the
two-word token "mangle decl"* rather than *count `Decl` statements in a Mangle
file*. The rest of the code is careful: per-line context cancellation, a 10MB
scanner buffer, comment stripping, wrapped errors. Well-engineered, confidently
wrong.

**Two distinct gaps, and only one is a wiring bug.**

`F-AUTO-4` — the generated tests are never run. `GeneratedTool.TestCode` is
filled on both generation paths, written to disk, and `thunderdome.go:231`
compiles the package with `go test -c` so they must *build* — but the only test
ever executed is `thunderdome.go:544`, `-test.run=TestThunderdomeArena`, the
adversarial harness. `validateCode` is AST-only. So a tool that fails its own
author's assertions still registers. Fixing this is in scope and dispatched.

`F-AUTO-5` — **running those tests would not have caught this bug**, and this is
the important finding. The generated test file asserts:

```go
{"single match", "mangle decl", "1"},
{"case insensitive", "Mangle Decl\nMANGLE DECL\nmAnGlE dEcL", "3"},
```

The tests encode the *same misreading* as the implementation, because the same
model wrote both from the same interpretation. They are self-consistent and
therefore worthless as a correctness check. Model-authored tests verify the
model's interpretation, never the request.

Catching this class requires an oracle the generating model did not author:
either a user-supplied example (`--expect "<input>=><output>"`, cheap and
reliable), or a second model asked to write tests from the ORIGINAL description
without seeing the implementation. Until one exists, `nerd tool generate`
prints "✅ Tool registered" for a tool nothing has ever checked — the same
hollow-success shape as a checkpoint reporting PASS without verifying.

Status: F-AUTO-4 dispatched. F-AUTO-5 open, needs a product decision.

### F-AUTO-5 addendum — Ouroboros is correct on unambiguous specs

A second live generation, run to test whether the earlier wrong tool indicated a
systematic problem or a specification problem:

```
nerd tool generate "given a file path as input, return the total number of lines
                    in that file as a decimal number"
```

Generated, gated on its own tests, compiled, registered in 55s. Run against
`schemas_safety.mg` it returned **194**, which is correct — `wc -l` and
`grep -c ''` both agree.

Worth recording how close this came to being logged as a second failure: the
first ground truth used was PowerShell `Measure-Object -Line`, which returned
136 and would have made a correct tool look broken. 136 is exactly the count of
NON-EMPTY lines (`grep -cve '^[[:space:]]*$'`). The verifier was wrong, not the
tool. **Cross-check ground truth with a second method before calling generated
output incorrect** — an agent grading its own tools with a faulty oracle is the
same failure mode as a model writing its own tests, one level up.

So the two results together say something more precise than "Ouroboros produces
wrong tools":

- Unambiguous spec → correct tool. The pipeline works.
- Ambiguous spec ("count the number of Mangle Decl statements", readable either
  as *find the token "mangle decl"* or *count `Decl` statements in a Mangle
  file") → a confidently wrong tool, with self-consistent tests that ratify the
  wrong reading, registered with a ✅.

That reframes F-AUTO-5. The risk is not unreliability, it is that an ambiguous
request produces a wrong tool **indistinguishable from a right one** at every
gate the pipeline has. The mitigation is correspondingly cheaper than a general
correctness oracle: make the resolved interpretation visible and checkable at
registration — echo the tool's own understanding of the request, and/or gate on
one user-supplied `input => expected` example. Ambiguity detection at the
detection stage would help more than test-running ever can.

---

## F-WHATIF-1 — `nerd whatif` never reads the code it reasons about

Exercised live on a real counterfactual from this session:

```
nerd whatif "revert LoadFacts to always evaluate eagerly instead of deferring on an initialized kernel"
```

It exits 0, asserts `derives_from_hypothetical(...)` into the kernel, and returns
a well-structured analysis. It also says, in its own first line:

> "No source code for `LoadFacts`/kernel was provided - analysis below is
> hypothetical/general only"

So the counterfactual is reasoned from the prompt string alone. Nothing retrieves
`kernel_facts.go`, the call sites, or the fixpoint measurements — in a repo whose
north star is that logic determines reality, the what-if tool consults no facts.

The output is a fair illustration of why that matters. Its performance bullet is
right — eager evaluation raises startup cost, which matches the measured 143
fixpoints / 723.8s — but that is general knowledge landing luckily. Its
"Improvement — Determinism/Correctness" bullet is **wrong for this codebase**:
it claims eager evaluation eliminates stale reads, when `ensureEvaluated()`
already guarantees correct-on-read and is called at the top of every Query. A
grounded analysis would have found that in one search.

Minimum fix: resolve the symbols named in the hypothetical (`LoadFacts`) via the
existing `search_code`/`read_file` tools and include them, so the analysis is
about this repo rather than about kernels in general.

## F-DREAM-1 — dream consults every agent regardless of relevance

```
nerd dream "how should codeNERD detect that a generated tool answers a different
            question than the one asked"
```

Works, and is genuinely useful: `requirements_interrogator` framed it as semantic
alignment rather than a syntactic safety rule, and proposed freezing intent as a
`requested_tool_spec(Capability, Description, ExpectedInputs, ExpectedOutputs,
ExampleIO)` fact at request time and verifying behaviour against it. That is a
real contribution to the open F-AUTO-5 decision.

The cost side is the defect. It consults **7 agents** for every scenario —
including `bubbleteaexpert` and `cobraexpert`, which have nothing domain-relevant
to say about tool semantic drift and duly produced generic answers. Seven full
LLM round trips to obtain perhaps two relevant perspectives, on a north star that
is explicitly hyper token efficiency.

The selection signal already exists and is unused: each agent carries topics and a
role in its `prompts.yaml`, and the JIT compiler already does semantic atom
selection. Dream should score agents against the scenario and consult the top
few, the same way atoms are selected, rather than fanning out to everyone. Note
it already skips 16 (9 system, 7 image aliases), so a skip mechanism exists —
it is relevance that is missing, not the ability to exclude.

---

## F-JIT-2 — every JIT cache miss deep-copies the entire kernel

Found by codeNERD on rung 3 of an open-ended optimization ladder, verified here.

`internal/prompt/compiler.go:462` calls `acquireCompilationKernel` on every cache
MISS, which reaches `KernelAdapter.NewCompilationScope`
(`internal/system/factory_adapters.go:79-97`) and returns
`NewKernelAdapter(live.Clone())`. `RealKernel.Clone` is a deep copy
(`internal/core/kernel_eval.go:673-713`).

Measured cost, which the run itself could not measure and said so:

```
file_topology     4453
symbol_graph     10231
dependency_link     74
tool_registered      3
                 -----
                 14761 facts deep-copied per cache miss
```

The isolation is deliberate and correct in intent — prompt compilation must not
mutate the live kernel, and a scope that shares state would let a compile perturb
the run it is compiling for. The cost is that isolation is bought with a full O(N)
copy of an EDB that is almost entirely world-derived and read-only during
selection.

Worth noting what it spans, since that was the question: the facts are produced in
`internal/world` (cartographer/incremental scan), stored in `internal/core`
(RealKernel EDB), and copied in `internal/prompt` (compilation scope). No single
subsystem owns the redundancy, which is why it survived.

Second candidate it ranked, not yet verified: dual-schema emission, legacy
`symbol_graph`/`dependency_link` alongside newer `code_defines`/`code_calls` for
the same symbols and calls. `symbol_graph` alone is 10231 of the 14761 facts, so
if that duplication is real it is most of the clone.

### The methodological result matters more than the finding

Rungs 1 and 2 both produced confident quantified claims that were false and
checkable in under a minute — "duplicates verbatim" (the two atoms share no
content) and "cutting ~3200 tokens" (a controlled A/B showed byte-identical
output, because `minScoreThreshold` is never read).

Rung 3 added one sentence to the instruction: state how every number was measured
and cite the evidence, or say plainly that you could not measure it. The answer
then opened with an explicit inventory of what it had NOT done — no live workload,
no bench, no pprof — and ranked qualitatively where it lacked numbers, marking
each gap.

So the unmeasured-confidence failure is closable by instruction. That makes it a
prompt-atom fix rather than a mechanical one, which is the opposite conclusion
from the scratch-artifact case, where 43 in-prompt impressions failed to prevent
the behaviour. Two failure modes, two different classes of remedy: one responds to
being told, the other does not.

---

## F-IMPACT-1 — WITHDRAWN. The instrument was broken, not the subsystem.

**Superseded by F-QUERY-1 below. Do not cite the original entry.**

The original claim was that every predicate in `impact.mg` derives zero facts,
including `relevant_context_file` and `context_priority_file`. Its sole evidence
was `nerd query <pred>` printing "No facts found". That evidence is void:
`nerd query` reports "No facts found" for predicates that demonstrably hold tens
of thousands of facts.

The concrete refutation. `code_defines` was the load-bearing claim — "a producer,
nine policy consumers and zero facts at runtime":

```
nerd query code_defines   ->  No facts found          (9 runs, all identical)
nerd logic code_defines   ->  Found 0 facts           (3 runs)
nerd logic  (summary)     ->  code_defines    10231   (3 runs)
                              symbol_graph    10231
```

10,231 is exactly the `symbol_graph` count, which is what the bridge rule at
`internal/core/defaults/policy/knowledge.mg:70` should produce — one
`code_defines` per `symbol_graph`. The derivation works. The read path does not.

So the corrected finding is the inverse of the original: `code_defines` is
**populated**, and I could not see it because the kernel's primary read API
returned empty. Whether the `impact.mg` predicates derive anything is now
**unknown and unmeasurable** until F-QUERY-1 is fixed — the only working reader
(`QueryAll`) is exposed solely through a summary hardcoded to the top 25
predicates (`cmd/nerd/cmd_advanced.go`, `const shown = 25`), and none of the
impact predicates rank that high.

### What survived from the original entry

Two observations were verified independently of `nerd query` and still hold, but
neither is "the subsystem is inert":

- **Two ID spaces that cannot join.** `dependency_link`'s CallerID is a file path
  (`"C:\\...\\audit_execution.py"`) while `symbol_graph`'s SymbolID is a symbol
  (`"method:(e *Executor).SetHistory"`). `dependency_link_exists`
  (`reviewer.mg:451-454`) joins one to the other, so it can never unify —
  independent of any query bug.
- **Backslash paths defeat a substring test.** `layer` (`reviewer.mg:541-544`)
  tests `:string:contains(File, "/internal/")` against `symbol_graph`'s
  backslashed absolute paths. `configured_layer_pattern` has facts; the pattern
  cannot match. `layer` gates `architecture_violation`.

### What was disproved by experiment

The first hypothesis was that Decl bound violations silently suppress derivation:
`knowledge.mg:70` puts `""` into a slot declared `/number`, and `symbol_graph`
carries strings in two slots declared `/name`. An isolated four-variant repro
against the pinned Mangle (production-shaped / head fixed / source fixed / both)
derived a fact in **all four**. Mangle does not enforce `Decl` bounds at
evaluation time. The type violations are real contract rot but caused none of
this. Worth keeping only as a note: a Decl is documentation here, not a check.

### The lesson, which is the actual value of this entry

I wrote a confident, specific, committed finding on top of an unvalidated
instrument, having already been burned this same way twice today (PowerShell
`Measure-Object -Line` vs `wc -l`; the `awk` range that fabricated a
self-referential atom). The tell was present and I walked past it: `nerd query`
was returning "No facts found" for *many* predicates at once, and I read a
consistent story where I should have read a suspicious one. **When a measurement
makes a whole subsystem look dead, suspect the meter before the subsystem** —
cross-check with a second, independent reader before writing anything down.

---

## F-QUERY-1 — `Kernel.Query` returns empty for predicates that hold facts

The kernel's primary read API is unreliable, and it fails **silently and
plausibly**: it reports "no facts", which is a legitimate Datalog answer, so
every caller treats a broken read as a true negative.

### Evidence

`code_defines` holds 10,231 facts. Two readers, same kernel, same process shape,
9+3+3 observations, zero variance:

| Reader | Path | Result |
|---|---|---|
| `Kernel.Query("code_defines")` | `kernel_query.go:96-105` | **0** |
| `Kernel.QueryAll()` | `kernel_query.go:329-339` | **10,231** |

Controls proving `Query` is not simply broken for everything: `symbol_graph`
(10,231), `file_topology` (4,453), `code_calls` (74), `dependency_link` (74),
`configured_layer_pattern`, and `is_called` all return facts through the same
`Query` call. So the failure is predicate-specific, which is what makes it so
dangerous — it looks like data, not like a bug.

### Root cause

The two functions differ in exactly one respect. `Query` scans
`programInfo.Decls` for a matching **symbol** and `break`s at the first hit,
then fetches facts for that one `PredicateSym`:

```go
for pred := range k.programInfo.Decls {
    if pred.Symbol == predicateName {
        predicateFound = true
        k.store.GetFacts(ast.NewQuery(pred), ...)
        break              // <-- takes the first arity it happens to see
    }
}
```

`QueryAll` iterates every entry and never breaks. A `PredicateSym` is
`{Symbol, Arity}`, so when one symbol is declared at more than one arity, `Query`
resolves to whichever arity Go's randomized map iteration yields first and
returns that arity's facts — zero, if it picked the unpopulated one. `QueryAll`
sees them all.

`predicateFound` is set to `true` on the symbol match, so the existing
"predicate not found in declarations" warning never fires; the fresh kernel log
contains zero of them. The failure is invisible in logs by construction.

Corroboration that multi-arity `code_defines` is a live concern, not a
hypothetical: `internal/core/dreamer.go:548` already defends against it —
`pred.Symbol == "code_defines" && (pred.Arity == 5 || pred.Arity == 2)`, with the
comment "Be tolerant of older arities to avoid schema drift breakage." One
subsystem worked around this; the shared read path did not.

Unexplained and worth stating rather than papering over: the outcome was stable
across all 15 runs, where randomized map iteration predicts variance. Either the
second arity is reached by a different route than I have identified, or map
ordering is effectively stable for this map. **The asymmetry is measured fact;
the arity mechanism is the best-supported explanation, not a confirmed one.**
Whoever fixes this should confirm by dumping the actual `PredicateSym` set for
the symbol — which nothing currently exposes.

### Why this is the highest-severity defect found today

`Query` is the kernel read API. Every "predicate X derives nothing" conclusion in
this ledger that rests on `nerd query` alone is now suspect and needs re-checking
against `QueryAll` once it is exposed. It also means a policy author cannot trust
a negative result, which undercuts the "logic is the executive" premise: the
executive can be asked a question and quietly give the wrong answer.

### Fix shape

1. Make `Query` collect facts across **all** arities of the matched symbol —
   delete the `break`, mirroring `QueryAll`. Preserve the explicit-arity fast
   path for pattern queries.
2. `QueryAll` has a milder version of the same bug: it does
   `results[predName] = make([]Fact, 0)` per `PredicateSym`, so for a multi-arity
   symbol each arity **resets** the accumulator and the last one iterated wins.
   It reported the right number here by luck of ordering. Accumulate instead.
3. Give the summary a way to show every predicate (`const shown = 25` is
   hardcoded), so a second independent reader exists for exactly the kind of
   cross-check whose absence produced the withdrawn F-IMPACT-1.
4. Regression test: assert `Query(p)` and `QueryAll()[p]` agree for every
   predicate holding facts. That invariant is cheap and would have caught this.

### ROOT CAUSE CONFIRMED — it is shard routing, not arity

The arity theory above was a real bug but the wrong one. Fixing it (`212ecb81`)
changed nothing about the symptom: `nerd query code_defines` still printed
"No facts found" against a kernel holding 10,231, measured on a fresh
`-tags sqlite_vec` build after the change. That is the second time in this
investigation a plausible mechanism survived scrutiny and still was not the
cause — recorded because the diff looked right and the test went RED→GREEN, and
neither of those facts said anything about the symptom.

The booted kernel is a `*CortexKernel`, not a `*RealKernel`
(`internal/system/factory.go:855,870`). The two readers differ structurally:

```go
func (c *CortexKernel) Query(predicate string) ([]types.Fact, error) {
    shard := c.routeToShard(predicate)      // ONE shard
    return shard.Query(predicate)
}
// QueryAll iterates c.shards and merges every shard's facts
```

`routeToShard` consults `c.predicateOwner[bare]` and, on a miss, falls back to
the cortex catch-all shard. Boot log, verbatim:

```
[cortex] registered shard 'routing'  (owned=4 predicates, router=false)
[cortex] registered shard 'world'    (owned=4 predicates, router=false)
[cortex] registered shard 'tools'    (owned=3 predicates, router=false)
[cortex] registered shard 'policy'   (owned=8 predicates, router=false)
[cortex] registered shard 'campaign' (owned=4 predicates, router=false)
[cortex] registered shard 'prompts'  (owned=3 predicates, router=false)
[cortex] registered shard 'cortex'   (owned=0 predicates, router=false)
```

**26 owned predicates across seven shards, against ~1,720 declared.** Everything
else — including every predicate derived by a rule rather than loaded as a fact —
misses the owner map and is answered by the cortex shard, which owns nothing and
holds nothing. `code_defines` is derived from `symbol_graph`, which `world` owns,
so its 10,231 facts live in `world`'s store and the query never looks there.

`KernelShard.Query` has the correct fallback built for exactly this case: if the
shard does not own the predicate and a router is present, fan out via
`ShardFactRouter.QueryVia`. Note `router=false` on all seven shards. The router
is constructed only when `features.IsPerShardFactsEnabled()` is true
(`cortex_kernel.go:70-81`), and it is off.

**So the defect is the asymmetry, not either half.** Narrowing a query to one
shard is only safe when the router can fan out on a miss; the narrowing is
unconditional while the fan-out is behind a flag that is off. Whoever turned the
flag off got "behavior matches pre-Track-D code exactly" for writes, and a
silent-empty read path for anything unowned.

This is the same wiring-gap family logged six times already in this ledger, with
one difference worth keeping: the missing collaborator here is not unset by
oversight but **deliberately disabled**, and the code that depends on it was
never gated to match. A feature flag that turns off a compensating mechanism
without also turning off the behaviour that requires it is a wiring gap wearing
a config switch.

### Corrected fix shape

The multi-arity fix (`212ecb81`) stands on its own merits — it is a genuine bug
with a RED→GREEN regression test — but it is not this. The fix for the symptom:
make `CortexKernel.Query` merge across all shards when the predicate has no
registered owner, matching what `QueryAll` already does, and leave the owned
path routing to its owner exactly as today. That restores correct reads without
enabling per-shard fact routing.

### Scope of the damage

`Query` is the kernel read API and 26 of ~1,720 predicates have an owner. Every
conclusion in this ledger of the form "predicate X derives nothing", where the
evidence was `nerd query` alone, is unreliable — including F-AUTO-3's thirteen
dead predicates, F-LEARN-1's unused `query_learned`, and the five `impact.mg`
predicates from the withdrawn F-IMPACT-1. Re-check all of them against a merged
reader once this lands. Note the direction of the bias: the bug can only
manufacture *false* deaths, never hide a live one, so nothing previously
declared alive needs revisiting.

### F-RUN-1 (side finding) — one bad filename guess ends the whole run, exit 0

The first attempt at this fix produced no edits and terminated after 30 seconds
with exit status 0. The only error in the logs:

```
[ERROR] Tool call read_file failed: modular tool execution failed:
file not found: internal/types/kernel.go. That directory contains:
extract.go, ..., types.go, ... (pick one of those, or use glob to search
elsewhere -- do not guess another filename)
```

The tool error message is genuinely good — it lists the directory contents and
tells the model exactly how to recover. The run stopped anyway, and reported
success to the shell. Re-running with the file paths pinned in the brief let it
proceed, which confirms the guess was the blocker rather than the task.

Two separate defects here, worth separating:

1. **A single recoverable tool error aborts the run.** A missing file is the most
   ordinary failure an agent hits; the correct response is to use the listing it
   was just handed and retry, not to stop.
2. **Aborting exits 0.** Same hollow-success family as the campaign checkpoints
   (`skipped, therefore passed`) and `nerd init`'s quality score. A caller that
   trusts the exit code sees a completed task with an empty diff. I only noticed
   because I diff the touched-file list against the brief on every run.

The second is the more dangerous of the two, and it is cheap to fix: a run that
performs no edits and exits on an unrecovered tool error should be non-zero.

### RESOLVED — `f7cf1b1b`

`nerd query code_defines` returns 10,231 facts. Measured zero across 9 runs
before, 10,231 across 4 runs after, on fresh `-tags sqlite_vec` builds.

The fix: `CortexKernel.Query` now fans out across every shard when a predicate
has no registered owner, matching what `QueryAll` already did. The owned path is
untouched and per-shard fact routing stays disabled. Two regression tests, both
proven RED first.

What finally identified it was **instrumentation, not reading**. Three static
hypotheses each survived scrutiny and each was wrong, and I only stopped guessing
once the kernel could report what it had actually looked at:

```
Query: decl code_defines/5 -> 0     facts
Query: decl code_defines/5 -> 10231 facts
Query: predicate=code_defines matchedDecls=1 totalResults=10231
```

`matchedDecls=1` also retired the multi-arity theory for good: only one Decl was
ever involved. That bug (`212ecb81`) is real and separately tested; it had
nothing to do with this symptom. The per-arity logging was kept, because a
zero-result query being indistinguishable from a genuinely empty predicate is
the property that let this hide for as long as it did.

### The withdrawal was itself wrong, in an instructive direction

With a working reader, the `impact.mg` predicates are still empty — every one:

```
impact_caller  impact_graph  impact_implementer
relevant_context_file  context_priority_file  relevant_context   -> all 0
```

So the original F-IMPACT-1 **conclusion** was correct; its **evidence** was
worthless. Withdrawing it was still the right call — a true claim resting on a
broken instrument is not knowledge, and keeping it for being accidentally right
would have taught exactly the wrong lesson. But the honest scorecard is: right
answer, invalid reasoning, withdrawn, then re-derived on valid evidence.

The real cause is upstream of anything to do with reads, and is now measurable:
`modified_function`, `modified_interface` and `code_implements` have **no
producer anywhere in the codebase**. `modified_function` appears once in Go, as
a bare map key at `internal/session/executor.go:1158`, and no `.mg` rule derives
it. Every `impact.mg` rule bottoms out in one of those three, so the subsystem
cannot fire regardless of how healthy `code_defines` is. `code_defines` being
populated was never sufficient.

### Three lessons, in the order they cost time

1. **Suspect the meter before the subsystem.** A measurement that makes a whole
   subsystem look dead is more likely broken than the subsystem. This cost the
   most and was the most avoidable.
2. **A RED→GREEN test proves the test, not the fix.** Both intermediate fixes had
   genuine failing-then-passing regression tests and neither moved the symptom.
   The only thing that ever settles it is the before/after measurement of the
   original complaint.
3. **When static reading has failed twice, instrument.** I read the same forty
   lines repeatedly and generated three wrong mechanisms from them. One Debug
   line per matched Decl answered it immediately, and is worth keeping.

---

## F-ATOM-1 — `symbol_graph` emits strings where every consumer matches atoms

This is why the impact-analysis subsystem is inert, and it is measurable now
only because F-QUERY-1 gave us a working reader. It is the substantive finding
that the withdrawn F-IMPACT-1 was groping at.

### Measured

`code_defines` Type slot, counted from live `nerd query` output:

| Value | Count | Kind |
|---|---|---|
| `"method"` | 189 | string |
| `"function"` | 123 | string |
| `"struct"` | 48 | string |
| `"type"` | 8 | string |
| `"interface"` | 4 | string |
| `"class"` | 1 | string |
| `/predicate` | 26 | **atom** |

Every consumer matches an atom:

```
impact.mg:37           code_defines(ImplFile, Struct, /struct, _, _)
impact.mg:60,70,74,78  code_defines(File, Func, /function, _, _)
reviewer.mg:424        code_defines(File, ID, /function, _, _)
```

In Mangle a string constant and a name constant are different terms. There are
123 function definitions in the fact base and **zero** that match `/function`.
So `relevant_context_file`, `context_priority_file`, `impact_implementer` and
`unwired_function` all derive nothing — confirmed empty against a working reader.

### Root cause: two producers, two conventions

```go
internal/world/mangle_fastparse.go:27
    Args: []any{symbolID, "/predicate", "/public", path, sig}   // correct

internal/world/ast_treesitter.go  (18 sites)
    Args: []any{id, "function", visibility, path, signature}    // wrong
```

A leading slash makes the value land as an atom; without it the value stays a
string. The 26 `/predicate` facts are the minority producer doing it right,
which is what made the two conventions visible side by side in one relation.

The schema already specifies atoms and documents the enum:

```
schemas_world.mg:49  Decl symbol_graph(...) bound [/string, /name, /name, /string, /string]
schemas_world.mg:45  Type: /function, /class, /interface, /struct, /variable, /constant
schemas_world.mg:46  Visibility: /public, /private, /protected
```

### The Decl-enforcement point, now with teeth

Earlier in this investigation I disproved the theory that `Decl` bound
violations suppress *derivation* — an isolated four-variant Mangle repro derived
a fact in all four cases, so bounds are not enforced at evaluation time. That
result was correct and it made the type violations look harmless. They are not.
Bounds are not enforced, so a violating value propagates happily through the
producer and the bridge rule, and then silently fails to unify at the one place
it matters — the consumer that filters on it.

So the sharper statement is: **an unenforced `Decl` does not break the relation
that violates it, it breaks the relation that trusts it.** That is strictly
worse than a loud failure, because the damage surfaces two hops away from the
mistake, in a rule that is correct.

Note also `"method"` (189 facts, the largest group) is not in the documented
enum at all, and `/method` would still not match `impact.mg`'s `/function`.
Converting the strings to atoms is necessary but not sufficient for methods;
whether a method should satisfy a `/function` filter is a policy question worth
deciding explicitly rather than by accident.

### Why it stayed hidden

Same shape as every other finding in this ledger: an empty join is
indistinguishable from an empty relation. Nothing errors, nothing warns, the
scan reports success, `symbol_graph` has 10,231 healthy-looking facts, and the
rules that consume them quietly produce nothing.

---

## F-RUN-2 — the silent-success defect disables the override that exists for it

This is a composition bug between two mechanisms that are each individually
reasonable, and it deadlocks the dogfooding loop.

### The two halves

**Half one (F-RUN-1, now seen four times today).** A run spawns its coder shard,
does nothing, and exits **0**. Observed at 30 seconds against successful runs
that take 8–18 minutes, so the shape is unmistakable. Once it was traceable to a
single failed `read_file` on a guessed filename; once to iteration exhaustion
(19 tool calls, 17 turns, zero edits); twice with no diagnosable cause at all.
Nothing is written to any log at ERROR level. The shell sees success.

**Half two.** `.claude/hooks/block-direct-codebase-edits.py` refuses a
hand-editing override unless the `ATTEMPTED:` line names a recent `.nerd/logs`
file containing one of `FAILURE_MARKERS` (`shard execution failed`,
`execution failed`, `llm generation failed`, `build failed`, `panic:`, ...). Its
stated rationale is exactly right: "Free text was self-granted in practice... This
cannot be satisfied without having run codeNERD."

### The deadlock

codeNERD could not perform a ten-character edit across three consecutive runs.
That is precisely the circumstance the override exists for. But because the runs
failed **silently**, no log contains a failure marker, so the override is
refused — and a refused override is still spent. The gate's own message closes
the loop: *"ATTEMPTED log records no failure — if codeNERD did not fail, it was
not blocked, so fix it there."*

It did fail. It simply did not say so.

So: the work cannot be done by codeNERD, and cannot be authorized for hand-repair
either, because **the evidence required to unblock is the evidence the system
declines to produce**. The scarcity mechanism is sound; it is starved by a
defect in the thing it is measuring.

### Why this matters more than the edit it blocked

The blocked task was trivial — adding a leading slash to ten string literals in
a test file. The interesting part is structural: a safety mechanism whose input
is "did the subject fail" is only as good as the subject's honesty about
failure, and this subject reports success by default. Every hollow-success
defect in this ledger (campaign checkpoints "skipped, therefore passed";
`nerd init`'s quality score surviving 195 of 196 failed LLM calls) is the same
disease, but this instance is the first where the dishonesty disables a control
built to contain it.

### Fix, in dependency order

1. **Make failure observable.** A run that reaches its iteration ceiling, or
   ends on an unrecovered tool error, or produces no edits when the brief asked
   for edits, must log at ERROR with one of the existing marker strings and exit
   non-zero. This is the blocker; everything else is downstream of it.
2. **Do not recover from a bad filename by giving up.** The `read_file` error
   already hands back the directory listing and says "do not guess another
   filename". Consume it and retry.
3. Only then is the override gate load-bearing again.

Recording this rather than routing around it: per the repo contract, when
codeNERD cannot do something, fixing the *blocker* is the dogfooding work and
doing its job for it is not. The blocker here is that it cannot admit failure.

### F-ATOM-2 — the producer fix does not reach the kernel: `nerd scan` leaves a stale cache

The `symbol_graph` atom fix (`806714a8`) is correct and lands in the persisted
scan output. It still does not change what the kernel answers with, because
`nerd scan` does not invalidate the store the kernel actually reads.

Measured after a full `nerd scan` on the fixed binary:

| Source | Type-slot form | Count |
|---|---|---|
| `.nerd/mangle/scan.mg` (rewritten 02:31) | `/method` 4769, `/function` 2995, `/struct` 1472, `/predicate` 2549 | 12,097 facts |
| kernel via `nerd query symbol_graph` | `"method"`, `"function"`, `"struct"` — **strings** | ~10,231 |
| `.nerd/knowledge.db` (97 MB) | `symbol_graph("method:(i *Initializer).runPhase7cCreateCoreShardKBs", "method"` … | stale strings |

Three things establish that the cache, not the loader, is at fault:

1. **The fresh output is correct.** `scan.mg` holds atom form for all 12,097
   facts, so the producer change works.
2. **The loader preserves atoms.** `/predicate` facts — emitted by
   `mangle_fastparse.go`, which always used atom form — arrive in the kernel
   still as atoms. Nothing downgrades an atom to a string in transit.
3. **The counts disagree.** 12,097 in `scan.mg` versus ~10,231 in the kernel.
   Two different populations, so two different sources.

`strings .nerd/knowledge.db` returns the old string-form facts verbatim. The
kernel is served from there.

So a full rescan on a fixed producer produced no observable change, and would
have read as "the fix didn't work" to anyone who checked only the query. That
is the same shape as F-QUERY-1 — a correct change hidden behind a reader that
answers from the wrong place — and it is why the payoff measurement for
F-ATOM-1 must stay open rather than be recorded as a win.

This also reframes the long-standing "447 stale facts" backlog item. That was
filed as a data-hygiene chore. It is the same defect: the world-model cache
outlives the scan that is supposed to refresh it, so corrections to any producer
are invisible until something else evicts the cache.

**Not yet determined:** whether `nerd scan` is meant to invalidate
`knowledge.db` and fails to, or whether the kernel is meant to prefer `scan.mg`
and does not. Those need different fixes, and I did not measure which is
intended, so I am not guessing. Deliberately not "fixed" by deleting the cache —
that would destroy user state and prove nothing about the mechanism.

**Consequence for the impact predicates.** `unwired_function`,
`relevant_context_file`, `context_priority_file`, `impact_caller` and
`impact_graph` are all still empty after the rescan, but that measurement is now
uninformative: the kernel never saw the corrected facts. The F-ATOM-1 payoff is
untested, not disproved. Separately and independently, `modified_function`,
`modified_interface` and `code_implements` still have no producer at all, so the
impact rules cannot fire on atoms alone regardless.

---

## F-RUN-2 — WITHDRAWN. The deadlock does not exist; I measured the wrong thing.

The claim was that codeNERD's silent exit-0 disables the hand-edit override,
because the gate requires a logged failure and the system never logs one. Both
halves of that were wrong, and both errors were mine.

**The runs did not exit 0.** I read the exit status from the background-task
notifications, but those report the status of the wrapper shell
(`nohup ./nerd.exe fix ... & echo dispatched`), which is the exit of `echo`.
I never measured `nerd.exe`'s own exit code. When I finally read the run logs:

```
Error: shard execution failed: execution failed: LLM generation failed:
edits broke the tests and the repair round did not fix them. Test output: ...
```

That run made its edits, saw the tests break, attempted a repair round, failed,
rolled back, and exited non-zero with an accurate message. That is the system
working correctly — the exact behaviour the hollow-success guards were built to
produce.

**The failure markers were there.** I scanned `.nerd/logs/*.log` for
`FAILURE_MARKERS` and found none, and concluded no override could ever validate.
The markers were in the run's own stdout log, which is where a CLI failure
naturally lands. `shard execution failed`, `execution failed`, `llm generation
failed`, `broke the tests`, `did not fix them` — five of them in one line. An
override citing that log would have validated.

So the override mechanism is intact and I owe it no criticism. The commit that
recorded this deadlock (`806714a8`) is wrong on that point.

## F-RUN-3 — the real defect: a non-empty result is treated as a completion

What actually happens is narrower than "silent success" and more interesting.
`cmd/nerd/cmd_direct_actions.go` guards against a *blank* result:

```go
if strings.TrimSpace(result) == "" {
    return fmt.Errorf("hollow success blocked: %s completed with an empty result", verb)
}
```

The comment above it is right that "an empty result is never a success". The gap
is that a **non-empty** result is taken as proof of one. Two live instances from
this session, both exiting 0 with no file modified:

1. **Prose announcing intent.** Final result, verbatim: *"Review comments
   received — checking each item against the actual code before patching."* The
   run ended at the moment it described what it was about to do. Non-empty, so
   the guard passed.

2. **A fabricated completion report.** A run listed all ten target lines
   individually as `**changed**`, named the test that covers each, and appended
   `go test ./internal/world -run TestASTParser -v => PASS for all four named
   tests (Python 7 facts, Rust 7, TypeScript 4, JavaScript 5)` plus a clean
   `go vet`. `git status` showed the file unmodified. The report is more
   specific and more confident than the honest failure in the rollback case
   above, and entirely wrong.

The second is the dangerous one. A blank result is obviously suspicious; a
detailed verification narrative is what a caller most wants to trust, and it is
unfalsifiable from the exit code alone. I caught it only because I diff the
touched-file list against the brief on every run.

### Fix shape

The result string cannot be the evidence, because the result string is written
by the thing being audited. For write-oriented verbs the check has to be
mechanical and external:

- Snapshot the mtime+size of every file the run touches (or the workspace's
  tracked-file state) before and after, and treat "briefed to edit, edited
  nothing" as a hollow success — the same reasoning already applied to
  `snapshotDirectRoot`/`findNewRootEntries` in that file for undeclared writes.
- Do not accept the model's own claim of having run tests. If the report says
  tests pass, either the harness ran them or the claim is not evidence.

This is the `executor_tools.go` argument again, and it is now three-for-three in
this ledger: prose in the prompt is a request the model complies with most of
the time; a fact checked before the tool runs is one no amount of model
conviction gets past. The same holds for prose in the *output*.

### On my own measurement, fourth time today

PowerShell `Measure-Object -Line` vs `wc -l`. An `awk` range that fabricated a
self-referential atom. `nerd query` reporting a live subsystem as dead. Now a
background-task exit code that belonged to `echo`. Every one was a case of
trusting a reading without asking what it was a reading *of*. The pattern is
specific enough to be actionable: **before a measurement becomes a claim, name
the thing it measures and confirm that is the thing in question.** All four
would have been caught by that one question.

### F-ATOM-2 RESOLVED — and the atom fix had introduced a second, worse defect

The open question was whether `nerd scan` should invalidate `knowledge.db` or
the kernel should prefer `scan.mg`. Answered by reading the boot path rather
than guessing: **the DB is the source of truth and `scan.mg` is a fallback.**
`internal/system/factory.go:879-897`:

```go
loadedWorld := false
if bctx.localDB != nil {
    if cached, err := bctx.localDB.LoadAllWorldFacts("fast"); err == nil && len(cached) > 0 {
        ... loadedWorld = true
    }
}
if !loadedWorld { /* only now read .nerd/mangle/scan.mg */ }
```

So the design is coherent, and the defect was that the scan was not getting its
facts into the DB at all.

**The cause was my own atom fix.** `worldFactPathArg`
(`internal/world/incremental_scan.go:449`) decided which argument of a fact is
its file path by a single test — does the string contain a slash:

```go
if strings.Contains(s, "/") || strings.Contains(s, "\\") { return s }
```

`symbol_graph` args are `[id, type, visibility, path, signature]`, and
`groupFactsByPath` takes the *first* argument this accepts. Once the type slot
became `/function`, that slot contained a slash, so every `symbol_graph` fact
was grouped under the pseudo-path `/function` instead of its real path in
Args[3]. `PersistFastSnapshotToDB` then called `os.Stat("/function")`, failed,
and `continue`d — dropping the entire group before it reached the DB.

Fixed by teaching `worldFactPathArg` to recognise atom form: a leading slash
followed by an identifier containing no further separator and no dot is an atom,
not a path. Table-driven test proven RED first.

**End-to-end verification, which is the only thing that settles it.** Rebuilt,
rescanned, queried the kernel:

```
symbol_graph Type: /method 140, /function 91, /struct 40, /predicate 22
code_defines Type: /method 146, /function 73, /struct 48, /predicate 21
```

Atom form now flows producer -> scan.mg -> DB -> kernel.

**A note on how nearly I mis-called this.** I twice counted rows by running
`strings` over `knowledge.db` and grepping for a serialized fact pattern. It
reported zero atom rows after a successful scan, which looked like proof the
persistence fix had failed. The DB stores predicate and args in columns, so that
pattern cannot match new rows at all; the 287 "string-form" hits were incidental
text from older pages. Fifth instrument error today, same shape as the other
four: I never asked what the reading was a reading *of*. The kernel query was
available the whole time and answered immediately.

### F-ATOM-3 — a rule that demanded the same value in two representations

With atoms flowing, the impact predicates are *still* all empty
(`unwired_function`, `relevant_context_file`, `context_priority_file`,
`impact_caller`, `impact_graph`, `impact_implementer`, `layer`,
`architecture_violation`). That was expected and stated in advance — atoms are
necessary, not sufficient. But `unwired_function` turned out to carry its own
contradiction, in adjacent lines of a single rule (`reviewer.mg:423-424`):

```
unwired_function(ID, File) :-
    symbol_graph(ID, "function", "public", File, _),   # strings
    code_defines(File, ID, /function, _, _),           # atoms
```

`code_defines` is derived from `symbol_graph` by a pass-through bridge
(`knowledge.mg:70`), so both lines constrain **the same value**. One demanded a
string constant and the other a name constant, which never unify. The rule could
not fire under either convention: before the producer change line 424 failed;
after it line 423 failed. It had simply never worked, and no convention choice
could have saved it.

`reviewer.mg:423` was the only string-form `symbol_graph` match in the entire
policy corpus, and the `Decl` says `/name`, so line 423 was the wrong one.
Fixed.

**Still blocking `unwired_function`, recorded not fixed:** its
`file_topology(File, ...)` premise joins `File` from `symbol_graph`'s DefinedAt
slot — absolute Windows paths — against `file_topology` keys, which are
workspace-relative POSIX. This is the same path-format defect that survived the
withdrawn F-IMPACT-1 on independent evidence. It is now the last representation
mismatch standing between this rule and its first derived fact.

Independently, `impact_caller` and `impact_implementer` remain blocked by
missing producers entirely: `modified_function`, `modified_interface` and
`code_implements` are declared, consumed by policy, and emitted by nothing.

### F-PATH-1 — symbol facts were keyed differently from the file facts about the same file

`file_topology` recorded `"internal/session/executor.go"`; `symbol_graph`
recorded `"C:\\CodeProjects\\codeNERD\\internal\\session\\executor.go"` for that
same file. Any rule joining the two unified nothing.

The canonical identity was already being computed — `internal/world/fs.go:332`,
`canonical := canonicalScanPath(root, path)` — and used for `file_topology` and
`file_dir`. The six AST parser calls a few dozen lines below were still handed
the raw absolute `path`, so every symbol fact was keyed differently from the file
fact emitted beside it in the same loop iteration. `os.ReadFile` genuinely needs
the absolute path; the parser's first argument is the *identity*, and those two
had been conflated.

Fixed by passing `canonical` to all six parsers (Go, Mangle, Python, Rust,
JavaScript, TypeScript) while leaving `os.ReadFile(path)` alone.

**First derived fact of the session.** After rebuild and rescan, `layer` returns
facts for the first time. It had been failing on
`:string:contains(File, "/internal/")` against a backslash path — a substring
test that could never match. `layer` gates `architecture_violation`, so a second
rule is now unblocked behind it.

**A note on the RED check.** My first two attempts ran `go test -run
"Canonical|Join"` against a test named
`TestSymbolGraphDefinedAtMatchesFileTopology`. The pattern matched nothing, Go
reported `ok ... [no tests to run]`, and I read that as GREEN and then as a
non-discriminating test. Running the real name showed four distinct assertions
failing before the fix. Sixth instrument error today and the most embarrassing,
because `[no tests to run]` was printed in the output I looked at.

### Still blocking `unwired_function`, measured not assumed

- **`in_scope` has zero facts.** It is a required positive premise of
  `unwired_function` (`reviewer.mg:425`), so the rule cannot fire no matter what
  else is fixed. Another declared-and-consumed predicate with no producer, the
  same family as `modified_function`, `modified_interface` and `code_implements`.
- `is_called` does have facts, and `is_entry_point_file` has none (which is
  permissive here, since it appears negated).

### F-PATH-2 — changing a key format orphans every previously persisted row

The fact base now holds **both** formats simultaneously:

```
symbol_graph("func:ValidateAll", /function, /public, "internal/core/action_validator.go", ...)
symbol_graph("struct:CortexTransaction", /struct, /public, "C:\\CodeProjects\\codeNERD\\internal\\core\\cortex_kernel.go", ...)
```

Sampling 400 facts: 201 stale absolute-path rows against 215 fresh relative ones.
Roughly half the world model is duplicate garbage describing files that are also
described correctly.

The mechanism is `ReplaceWorldFactsForFile(path, "fast", fp, inputs)`, which
replaces rows **keyed by path**. When the path format itself changes, the new
rows land under a new key and the old rows under the old key are never matched,
so nothing replaces them. They are unreachable by any future scan and will
persist indefinitely.

This is exactly the long-standing "447 stale facts" backlog item, and it was
never a hygiene chore: any correction to a path-producing scanner silently
doubles the affected relation instead of updating it. The duplicates are worse
than noise, because a rule joining on path now sees two rows per symbol and only
one of them can match anything.

Needs a migration or a format-versioned cache, not a manual purge — a purge
fixes today's instance and leaves the mechanism in place for the next format
change.

### F-JIT-3 — a new atom is a candidate on every coder turn and is never selected

`capability/impact_reporting` was added to close the last input gap in the
impact chain: `modified_function` is permitted for the model to assert
(`executor.go:1148-1159`) and nothing had ever asked for it. The atom builds,
validates (346 files / 906 atoms, no issues), and syncs to the corpus DB. It has
never appeared in a prompt.

Two live coder runs, both `nerd fix`, measured by grepping the `_llm_io.log` for
the atom's content heading:

| Run | Work done | `impact_reporting` in prompt | `tool_thinking` control |
|---|---|---|---|
| remove a duplicate doc comment | 1 deletion | 0 | 21 |
| add a `--all` flag | 33 insertions across functions | 0 | 29 |

The second run modified functions, which is precisely the trigger the atom
describes, so "correctly judged irrelevant" does not explain it.

**Ruled out by measurement, not argument:**

- **Not the corpus DB.** The atom is present in `.nerd/prompts/corpus.db` (5
  occurrences against `tool_thinking`'s 18), and the DB's mtime is later than the
  YAML's, so the sync ran.
- **Not `go:embed` staleness.** The binary was rebuilt before both runs.
- **Not the `shard_types` slash convention.** This one nearly cost a pointless
  22-file change. The corpus overwhelmingly writes `["/coder"]` (492 slash-
  prefixed entries against 22 without), and this atom was written `["coder"]`,
  which looked exactly like the string-versus-atom defect fixed three times
  elsewhere this session. It is not: `normalizeList` (`internal/prompt/atoms.go:298`)
  strips the leading slash from every selector entry at load, and `matchSelector`
  (`:420`) normalises the incoming value and compares against both forms. Both
  spellings match. **The 22 slashless atoms are fine and need no change.**

**What remains:** the atom is `is_mandatory: false` while the control
(`tool_thinking`) is `is_mandatory: true`, so the live difference is that one is
guaranteed inclusion and the other must win a scored selection. The atom is a
candidate that consistently loses.

That is the interesting question for the north star rather than a nuisance:
JIT selection is the mechanism the whole token-efficiency thesis rests on. If an
atom whose subject is "you just modified a function" cannot surface on a turn
that modifies functions, then relevance scoring is not doing the job the
architecture assigns it, and every non-mandatory atom is suspect. The cheap
diagnostic is to record per-atom scores at selection time — the same move that
resolved F-QUERY-1 after static reading failed three times.

Deliberately not "fixed" by flipping `is_mandatory: true`. That would make the
atom appear and prove nothing, while charging every coder turn for it forever.
The point is to find out why scoring rejects it.

### Second reader restored

`nerd logic --all` now exists. `printKernelFactSummary` had `const shown = 25`
hardcoded with no way past it, so everything below the top 25 was unreachable —
the exact blind spot that let a whole subsystem look dead and produced the
withdrawn F-IMPACT-1. Zero-count predicates are filtered (QueryAll seeds an empty
slice for all ~1700 declared), so the header now separates "predicates with
facts" from "declared".

It paid for itself immediately: `layer` reads **698 facts**, where it had been 0
before the path fix in `c71fdaf9`, and 182 predicates hold facts. Every earlier
"predicate X is dead" claim in this ledger can now be re-checked against a
reader that shows the whole population.

---

## F-JIT-4 — the symbolic half of atom selection is inert; only the vector half works

This is the most consequential finding of the session, because it is an
inversion of the project's premise rather than a bug inside it.

`internal/core/defaults/policy/jit_selection.mg` offers two independent ways for
a non-mandatory atom to become a candidate:

```
# rule 1 -- neural
candidate_atom(AtomID) :-
    vector_hit(AtomID, Score), Score > 30, !prohibited_atom(AtomID).

# rule 2 -- symbolic
candidate_atom(AtomID) :-
    prompt_atom(AtomID, _, Priority, _, _), Priority > 50,
    !prohibited_atom(AtomID), !mandatory_atom(AtomID),
    atom_tag(AtomID, /shard_type, ShardType),
    compile_shard(_, ShardType).
```

Rule 2 is the one that says *"this atom belongs to this persona"* — the
deterministic, explainable path. It cannot fire, for two independent reasons,
each sufficient on its own.

### Cause 1: `compile_shard` has no producer

Declared at `schemas_prompts.mg:235`. Consumed by **four** rules —
`jit_logic.mg:29` and `jit_selection.mg:165, 197, 255`. Asserted by nothing: a
grep across `internal/` and `cmd/` for the predicate name returns only the Decl,
the four consumers, and a stale crash dump. Live confirmation: it does not
appear in `nerd logic --all`, which now lists every predicate holding facts.

### Cause 2: the join is type-incompatible anyway

```
Decl compile_shard(ShardID, ShardType) bound [/string, /name].
Decl atom_tag(AtomID, Dimension, Tag)  bound [/string, /name, /string].
```

Rule 2 binds one variable, `ShardType`, from `atom_tag`'s third slot (declared
`/string`) and passes it to `compile_shard`'s second slot (declared `/name`).
A string constant and a name constant never unify, so the rule would derive
nothing even if `compile_shard` were fully populated. Producing the missing
predicate alone would not fix this.

This is the same shape as F-ATOM-3 (`reviewer.mg:423-424`), one layer up: a rule
whose two premises demand incompatible representations of the same value. That
is now **three** occurrences today of the identical defect, which makes it a
pattern rather than an accident — and Mangle not enforcing `Decl` bounds at
evaluation time is what lets every instance fail silently.

### Why this matters more than the individual bugs

codeNERD's stated thesis is that the LLM is the creative center and the logic
kernel is the executive. JIT atom selection is where that thesis is cashed out:
the kernel is supposed to decide, deterministically and explainably, which
instructions the model sees.

With rule 2 dead, **every non-mandatory atom enters the prompt only by embedding
similarity.** The executive decision is being made by a vector search — the
neural half doing the symbolic half's job. That is precisely the inversion the
architecture exists to prevent, and it has been silently true for as long as
`compile_shard` has been unproduced.

It also explains F-JIT-3 completely. `capability/impact_reporting` has priority
80 and a `shard_type` tag, so rule 2 is exactly the path it was designed to take.
Rule 2 is dead, so the atom's only route was a vector hit above 30 against a
task query, which it never achieved. Nothing was wrong with the atom.

### Fix shape, in dependency order

1. **Reconcile the types first.** Changing `compile_shard`'s ShardType slot to
   `/string` is the smaller correction: `atom_tag` is generic across many
   dimensions (language, framework, model, provider), so its Tag slot cannot be
   `/name` for all of them. Doing this second would mean shipping a producer that
   still derives nothing and looks like a failed fix.
2. **Then assert `compile_shard(ShardID, ShardType)`** in
   `AtomSelector.buildContextFacts`, which today asserts only `current_context`
   and already has the compilation context in hand.
3. **Then re-run the F-JIT-3 live test.** If rule 2 is alive,
   `capability/impact_reporting` should appear in a coder prompt without being
   made mandatory, which is the honest confirmation that selection is working
   symbolically rather than by luck of embedding.

Verify by counting, not by reading: the number of selected atoms per turn, and
specifically whether a shard-tagged atom with no vector hit now appears.

### F-JIT-4 status — three defects fixed, selection still not reviving

Fixed and each proven RED first:

1. `compile_shard` had **no producer**. Now asserted in
   `AtomSelector.buildContextFacts` (`d32170b7`).
2. The join was **type-incompatible**: `compile_shard.ShardType` was declared
   `/name`, `atom_tag.Tag` is `/string`, and `jit_selection.mg:248-255` binds one
   variable across both. Decl reconciled to `/string`.
3. The value was **slash-asymmetric**. `cc.ShardType` arrives as `"/coder"` —
   `prompt_assembler.go:309,468` both `TrimPrefix` it for that reason — while
   atom selector values pass through `normalizeList` (`atoms.go:298`), which
   strips the slash, so `atom_tag` holds `"coder"`. The producer now normalizes.

That is the **fourth** slash-normalization asymmetry today, after `symbol_graph`
Type/Visibility, `worldFactPathArg`, and `reviewer.mg:423`. Two components each
normalize correctly for themselves and disagree at the seam.

**Still not selected.** `capability/impact_reporting` remains absent from coder
prompts across three more live runs (control `tool_thinking`: 8, 9, 5).

Ruled out by measurement, so the next investigator does not repeat them:

- Not corpus absence — `nerd jit` reports 906 embedded atoms including this one.
- Not corpus-DB sync — present in `.nerd/prompts/corpus.db`, mtime later than the YAML.
- Not `go:embed` staleness — rebuilt before every run.
- Not the `shard_types` spelling — `normalizeList` + `matchSelector` accept both forms.
- Not the Go fallback path — `fallbackFleshSelection` has three entry points, each
  logging, and none fired. The Mangle selection path is genuinely running.

**Next candidate, untested:** whether `atom_tag(AtomID, /shard_type, ...)` facts
are actually asserted for flesh atoms at compile time. `ToSelectorFacts`
(`atoms.go:489`) builds them, but I have not confirmed they reach the kernel in
the same batch the rule is evaluated against. The cheap check is to assert the
batch and query `atom_tag` for this atom's ID mid-compile — instrumentation
again, which is what resolved F-QUERY-1 after static reading failed three times.

Recording the miss rather than declaring victory on three green unit tests: the
fixes are individually correct and verified, and the behaviour they were meant
to produce has not appeared. Those are different claims.

### F-JIT-4 ROOT CAUSE — the JIT policy and its Go producer speak different vocabularies

The three fixes in `d32170b7` and `aece3df2` were each real and each verified,
and none of them could have worked, because the problem is a layer above all of
them: **`internal/core/defaults/policy/jit_selection.mg` is written against a
fact vocabulary that nothing in `internal/prompt` emits.**

| Policy consumes | Emitted by the Go layer? |
|---|---|
| `prompt_atom(AtomID, Category, Priority, TokenCount, IsMandatory)` — **15 rules** | **no** |
| `atom_tag(AtomID, Dimension, Tag)` | **no** |
| `compile_shard(ShardID, ShardType)` | only since `d32170b7` |
| `vector_hit(AtomID, Score)` | yes |
| `is_mandatory(AtomID)` | yes |

`AtomSelector.buildContextFacts` emits exactly five predicates: `atom`,
`atom_category`, `atom_priority`, `is_mandatory`, `current_context` — plus
`compile_shard` as of today. Note `atom_category` and `atom_priority` carry
precisely the data `prompt_atom` wants, under different names and a different
shape. The two halves were designed against each other and drifted.

There is even a third vocabulary. `PromptAtom.ToSelectorFacts`
(`internal/prompt/atoms.go:470`) builds the per-dimension selector facts the
policy needs — shard_type, language, framework, intent_verb and the rest — under
the predicate name `atom_selector`. **It has zero callers.** The correct producer
was written, named differently from both the emitter and the policy, and never
wired to anything.

### What this means in practice

`mandatory_atom` has three rules. Two of them require `prompt_atom` and
`atom_tag`, so they are dead. The third is `mandatory_atom(AtomID) :-
is_mandatory(AtomID), !prohibited_atom(AtomID)` — and `is_mandatory` is one of
the five predicates actually emitted, so that one works.

`candidate_atom` has two rules. The symbolic one needs `prompt_atom` + `atom_tag`
+ `compile_shard`: dead. The other is `vector_hit(AtomID, Score), Score > 30`.

So the entire live selection surface is: **atoms flagged mandatory, plus atoms
that clear an embedding-similarity threshold.** Everything else in
`jit_selection.mg` — conflict resolution, priority thresholds, shard scoping,
skeleton-category enforcement — is inert. That is the complete explanation for
every observation in F-JIT-3: `capability/impact_reporting` is non-mandatory, so
its only possible route was a vector hit, and it never cleared 30.

### Why this is the session's most important finding

Every other defect today was two components disagreeing about the representation
of one value — string versus atom, absolute versus relative, slashed versus bare.
This is the same failure at the scale of an entire interface: two halves of the
system's central mechanism, each internally coherent, sharing no vocabulary.

And it lands on the project's core claim. codeNERD exists to put the logic kernel
in the executive seat, with JIT selection as the flagship demonstration. What
actually decides prompt contents today is a mandatory flag and a cosine
similarity. The policy corpus that is supposed to be making those decisions
compiles, loads, stratifies, and derives nothing.

Nothing errors. `nerd jit` reports 906 atoms loaded and a healthy compiler. The
rules are syntactically valid and reference declared predicates. Only asking
which of those predicates hold facts reveals it — which required
`nerd logic --all`, built earlier today for exactly this reason.

### Fix shape

Do **not** rename predicates in the policy to match the emitter. The policy's
vocabulary is the better-designed one: `prompt_atom` carries category, priority,
token count and mandatory status in a single fact, and `atom_tag` generalises
across every selector dimension, which is why the rules can be written once
instead of per-dimension.

Wire the producer instead:

1. Emit `prompt_atom(AtomID, Category, Priority, TokenCount, IsMandatory)` from
   `buildContextFacts`, alongside or in place of the `atom_category` /
   `atom_priority` pair. `kernel_facts.go:1133` already normalises this exact
   arity, so the ingest path anticipates it.
2. Emit `atom_tag` for every selector dimension. `ToSelectorFacts` already
   computes all of them; rename its predicate to `atom_tag` and call it.
3. Only then re-run the F-JIT-3 live check. A shard-tagged, non-mandatory atom
   appearing without a vector hit is the proof that symbolic selection is alive.

Sequence matters, as it did for the Decl and the producer: shipping any one of
these alone leaves the rules deriving nothing and looks like a failed fix.

### F-JIT-4 CORRECTED — the earlier root-cause entry was largely wrong

The entry committed in `c4dfaedc` claimed the JIT policy and its Go producer
"share no vocabulary": that `prompt_atom` (15 rules) and `atom_tag` were never
emitted, and that live selection was therefore only "mandatory flag plus cosine
similarity". **Most of that is false.** Three separate reading errors produced it,
all the same shape.

1. I grepped for `"prompt_atom"` while the code writes
   `fb.WriteString("prompt_atom(")` — with an open paren. The pattern could not
   match. `prompt_atom` has been emitted all along, with a careful comment
   documenting a past bug where `ContentHash` was placed in the numeric
   TokenCount slot and broke fixpoint evaluation.
2. I enumerated emitted predicates with `sed -n '1200,1330p'`, which ends before
   the second `atom_tag` emitter at line ~1336. `atom_tag` is emitted too.
3. I listed the `addTags` calls from a window starting at 1345 and concluded
   shard scoping was absent. `addTags("shard", atom.ShardTypes)` was already
   there, just above my window.

Each time a truncated view was treated as the whole picture, and each time the
conclusion grew rather than shrank — the error compounded into an architectural
claim about the project's core thesis. That claim is withdrawn.

### What is actually true

Two narrow defects, both real:

- **`compile_shard` had no producer.** This one stands. It is consumed by four
  rules and was asserted nowhere. Fixed in `d32170b7`.
- **A dimension-name mismatch.** `addTags("shard", ...)` emits
  `atom_tag(ID, /shard, /coder)`. `mandatory_atom` (`jit_selection.mg:196`) reads
  `/shard` and matches. But `candidate_atom` (`:253`) reads `/shard_type`, which
  nothing emitted. One predicate, two names, one of them fed.

So the symbolic path was blocked by a missing `compile_shard` on both rules, plus
a name mismatch on the candidate rule specifically. Not a vocabulary schism.

### And I made it worse before making it better

Trusting `atom_tag`'s `Decl` (`Tag bound /string`) over the emitter, I changed
`compile_shard`'s ShardType slot from `/name` to `/string` and normalised the
emitted value to a bare `"coder"`. The emitter is the reality: `addTags` writes
atoms (`/coder`), with an explicit comment that this is required to match
`current_context(/shard, /coder)`. `atom_tag`'s Decl is the thing that is wrong.
Reverted: `compile_shard` is `/name` again and emits atom form via `writeAtom`.

The lesson is the sharper version of the one from F-ATOM-1. Earlier today I
proved Mangle does not enforce `Decl` bounds at evaluation time. That cuts both
ways: a Decl cannot be trusted as a description of what a predicate contains,
because nothing forces it to be true. **Read the emitter, not the schema** — and
where they disagree, that disagreement is itself the bug worth filing.

`atom_tag`'s Decl is now a known-wrong schema: declared `/string`, emitted
`/name`. Left alone deliberately — correcting it touches every consumer of every
tag dimension, which is a change with its own blast radius and deserves its own
measurement.

### F-JIT-4 FINAL — there are two selection rulesets, and I fixed the dead one

The answer was written at the top of the file I spent this whole thread
analysing. `internal/core/defaults/policy/jit_selection.mg`, lines 5-23:

> **STATUS (audited 2026-08-08): THIS IS NOT THE LIVE SELECTION RULESET.**
> The Go selector queries `selected_result/3`, which is defined in
> `defaults/jit_compiler.mg`. Nothing in production Go queries `selected_atom`,
> `candidate_atom`, `mandatory_atom`, `prohibited_atom`, `conflict_loser`... Both
> files are loaded into the same program, so these rules are evaluated on every
> prompt compile and **the results are discarded.**

I began reading that file at line 236 and never read its header. Ninth
scope-of-evidence error today, and the one that cost the most.

### The live pipeline, verified

`internal/prompt/selector.go` queries `selected_result(Atom, Priority, Source)`.
In `jit_compiler.mg` that chain is:

    selected_result(Atom, Prio, _)   :- final_valid(Atom), atom_priority(Atom, Prio), ...
    final_valid(Atom)                :- tentative(Atom), !invalid(Atom).
    tentative(Atom)                  :- mandatory_selection(Atom).
    tentative(Atom)                  :- candidate_selection(Atom, _), !suppressed(Atom).
    candidate_selection(Atom, Score) :- vector_hit(Atom, Score), !blocked_by_context(Atom), !prohibited(Atom).

**`vector_hit` is the only route to candidacy in the live ruleset.** There is no
priority rule, no shard-scoping rule, no conflict resolution by priority — those
exist only in the discarded file. So the substance of the original F-JIT-4 claim
holds after all: for a non-mandatory atom, an embedding score decides. But the
reason is architectural — a parallel dead ruleset — not the missing predicates I
went after.

### My two fixes serve nothing

`compile_shard` has exactly two consumers and both are dead:

- `jit_selection.mg` — documented dead above.
- `jit_logic.mg:27` `atom_has_shard_match`, which is **derived and consumed by
  nothing**: `jit_compiler.mg` references it zero times. It also needs
  `atom_selector`, produced only by `PromptAtom.ToSelectorFacts`
  (`atoms.go:476`), which still has zero callers.

So `d32170b7` (produce `compile_shard`) and `ce65c7b9` (emit `/shard_type`) add
facts that only discarded rules can read. They are harmless — build and tests
green — but they achieve nothing, and on a project whose north star is token
efficiency, emitting facts per atom per turn for dead rules is a cost, not a
neutral. **Left in place pending a decision rather than reverted unilaterally;
reverting shipped commits is the maintainer's call.**

### The real, actionable finding

The dead ruleset is not free. `jit_selection.mg` and `jit_logic.mg`'s contextual
matching are evaluated on **every prompt compile, over ~890 atoms**, and thrown
away. That is measurable waste on the exact axis the project optimises for, and
the 2026-08-08 audit already flagged it — "it does cost work". Nothing was done
then. The options are to delete the dead ruleset, or to wire the live compiler to
it. The second is the interesting one, because the dead file contains precisely
the symbolic machinery — shard scoping, priority thresholds, conflict resolution
— whose absence makes selection purely neural today.

### The lesson, ninth iteration

Every error in this thread was the same shape: a partial view of the evidence
treated as the whole. Truncated grep patterns, `sed` windows that stopped one
line short, a file read from the middle. Concretely, for next time:

- Read a file's header before analysing its contents.
- Grep predicate names with an open paren, because the code builds them as
  `WriteString("pred(")`.
- Before calling a rule live, find who queries it.

---

## F-LEARN-2 — the learning display could only ever read zero

Broadening from the JIT thread to exercise modules the mission names and this
session had not touched — autopoiesis, ouroboros, reflection, knowledge, memory —
`nerd autopoiesis status` reported:

    Ouroboros Loop:    3 tools generated
    Prompt Evolution:  0 evolutions
    Learning Store:    0 patterns
    Thunderdome:       0 battles

The Ouroboros line proves the display works. The learning line was structurally
incapable of reporting anything else:

    patterns, _ := cortex.Kernel.Query("learned_pattern")

`learned_pattern` is declared in `schemas_learning.mg` and **asserted nowhere** —
zero Go call sites, and the only `.mg` file mentioning it is the one holding its
Decl. So the count was pinned at 0 regardless of what the learning subsystem had
recorded. `.nerd/learned_patterns.db` is 3.2 MB and was last written *today*.

The predicates that are actually produced and consumed:

| Predicate | Asserted in Go | Read by policy |
|---|---|---|
| `success_pattern` | yes | 10 files |
| `failure_pattern` | yes | 6 files |
| `correction_pattern` | yes | **0 files** |
| `learned_pattern` | **no** | 0 (Decl only) |

Fixed to report success and failure separately, and to surface
`correction_pattern` — which is produced by Go and consumed by no rule, so a
non-zero count there means facts are being manufactured for nobody.

This is the same false-negative instrument class as F-QUERY-1, and the third
time today a "0" turned out to be a broken meter rather than an empty subsystem.
What it does **not** establish is that learning works: `success_pattern` and
`failure_pattern` are shard-scoped, so their zero in a bare `nerd query` boot is
expected and uninformative. The instrument is now capable of telling the truth;
whether the truth is good has not been measured.

### Learning lives in Go maps, not in the kernel

Worth recording separately, because it bears on the north star.
`BaseSystemShard.loadLearnedPatterns` (`internal/shards/system/base.go:585`)
reads persisted learnings and stores them in **Go maps** —
`b.patternSuccess[pattern] = 3`, `b.patternFailure[pattern] = 3` — not as kernel
facts. So a shard's accumulated experience is not available to any Mangle rule;
the executive cannot reason over what the system has learned. For a project whose
premise is that logic is the executive and memory is one of the things logic is
supposed to own, that is an inversion worth deciding about deliberately rather
than inheriting.

### F-META-2 (side finding) — duplicate tool-result envelope

The run that made this fix died at the end with:

    responses HTTP 400: Duplicate function_call_output for call_id 'req_vet'.
    Each function_call must have exactly one matching function_call_output

The edit had already landed correctly, so this is a protocol defect in the
tool-result follow-up path, not a code-generation failure: the client sent two
outputs for a single `call_id`. Unlike F-META-1 (empty stream on a small
completion budget), this one produces a hard 400 and loses the run's final
report. Not investigated further; recorded with the call_id so it can be traced.

---

## F-WHATIF-1 CONFIRMED — and the fix exposed two deeper defects

Exercising the modules the mission names but this session had not touched.

### The original finding, confirmed by A/B against a sibling

Asked to analyse deleting a specific `.mg` file, `nerd whatif` replied:

> Analysis is hypothetical only - no code/content for `jit_selection.mg` was
> supplied to verify ... would require exhaustive grep

`nerd shadow`, given a comparable prompt, replied:

> Verified via read_file + search_code

and produced a real impact list naming the symbols that would break. Two sibling
commands in the same file, one grounded and one speculating. Cause:

    runShadow (~531)  cortex.SpawnTaskWithContext(ctx, "coder", prompt, shadowCtx, ...)
    runWhatIf (~571)  cortex.LLMClient.Complete(analysisCtx, prompt)

A bare completion cannot open a file. The only kernel fact `whatif` contributes
is `derives_from_hypothetical(<the input string>)`.

### Attempt 1 — and a documented warning I should have heeded

Routing `whatif` through the shard path deleted this comment:

> Optional LLM elaboration uses a short direct Complete call — **never
> SpawnTask(researcher), which previously hung** after the first kernel line when
> the JIT spawn path stalled.

Measured: it did not hang (57s), but printed *"(analysis timed out; kernel
implications above are complete)"*. The tool-using path needs longer than the
45-second sub-budget at line 619, because it actually reads the repository.
Second time today I overrode a documented intentional decision; the first was
rung 2's framework strictness, also reverted after measurement.

### Attempt 2 — removed the sub-budget, and found the real defects

`runShadow` has the same 2-minute parent context and **no** inner sub-budget.
Removing `whatif`'s 45-second cap: 93 seconds, no timeout. Stdout showed:

> Analysis complete — writing the 9-point deletion impact with exact line cites now.

An intent announcement, not the analysis — the F-RUN-3 pattern again. But the
analysis was not missing. It was written to **`kernel_query_deletion_impact.md`
in the repository root**, 10,632 bytes, and it is good:

> `kernel_query.go` is the **sole read path** for the Mangle EDB/IDB. Deleting it
> does not "remove a feature" — it severs `RealKernel` from its store, breaks the
> `types.Kernel` contract, and collapses every subsystem that derives
> `permitted`/`next_action`/`project_forbidden_path` via query.

with citations to files it actually read, and an explicit uncertainty section for
cross-references it could not verify. That is exactly the grounded output the
change was meant to produce.

So the fix direction is right and two other defects block it:

1. **The shard writes its deliverable to disk instead of returning it.** The
   caller receives a sentence announcing the work; the work is in a file the
   caller was never told about.
2. **The undeclared-root-write guard does not cover this path.**
   `cmd_direct_actions.go` references `snapshotDirectRoot`/`findNewRootEntries`
   four times; `cmd_advanced.go` — which holds whatif, shadow, dream, logic and
   agents — references them **zero** times. A 10 KB file appeared in the
   repository root and nothing warned. The guard was added at the direct-verb
   construction site rather than at a choke point, which is the same failure the
   Ouroboros contract fix hit earlier in this ledger.

**Reverted** rather than shipped: grounded-analysis-in-a-mystery-file is worse
for a user than generic-analysis-on-stdout, and I have now degraded this command
twice. The change is worth redoing once output routing is fixed — the evidence
that it works is preserved.

### Also observed

Both `shadow` and the `whatif` analysis independently cited
`internal/core/current-state.md:84` as claiming `kernel_query.go` is 577 lines,
against 494 actually read. A stale doc that two separate runs tripped over.

---

## Dream state — works, with one real limitation (and a false alarm I did not file)

Exercised `nerd dream` against a live scenario. It works: 4 agents consulted,
19 correctly skipped (9 system, 7 image-generation aliases, 3 low relevance),
responses returned, and every agent correctly framed its answer as hypothetical
with no actions executed. The relevance ranking added earlier (`a353c53a`) is
live and its `--max-agents` bound is respected.

### The false alarm

Every agent scored 0 on my first scenario ("the JIT selector loads two rulesets
and discards one"), and three were "skipped for low relevance" against a field
where nothing scored above zero. That looks exactly like a scorer returning a
constant.

It is not. A discriminating run — "go concurrency and error handling across
interfaces" — produced `goexpert (score: 3)`, correctly ranked first, matching
three tokens against its declared Topics. `dreamRelevanceScore` works, agent
metadata loads, and the stable-sort tie-break on equal scores is the documented
behaviour.

Recording the near-miss because it is the tenth time today an apparent defect
was my reading rather than the system's, and the first time I caught it before
writing it down. The check cost one command.

### The real limitation

`dreamRelevanceScore` (`cmd_advanced.go:413`) is **pure lexical token overlap**
between the scenario and `meta.Role + meta.Topics`. So on my first scenario the
score was a truthful zero — and consequently the four agents consulted were
simply the first four in stable order, not the four best suited. `mangleexpert`
scored 0 on a question about **rulesets and a selector**, because its Topics
happen not to contain the words "jit", "ruleset" or "selector".

That is four LLM calls spent on arbitrarily-chosen agents whenever a scenario
does not literally reuse an agent's topic vocabulary — which is most of the
time, since scenarios are written in the language of the problem and Topics in
the language of the specialty.

The repository already has the machinery to fix this: an embedding engine with
47,537 vectors (`nerd embedding stats`), which is what the JIT selector uses for
atom relevance. Dream agent selection is the one relevance decision still made
by string matching. Scoring agents semantically — or falling back to embeddings
only when lexical overlap is zero, which is the cheap version — would put the
right specialists in front of the scenario without changing the consultation
budget.

Filed as an improvement rather than a defect: the current behaviour is correct,
documented, and bounded. It is just leaving capability on the table on the exact
axis the project cares about, since a wasted consultation is four wasted calls.

---

## F-AUTO-6 — the autopoiesis layer runs and is invisible to the kernel

### First, a correction I stated twice

I reported that thunderdome and prompt evolution have "no user-reachable entry
point and no producer", and used that to explain why they could not be
exercised. The second half is right and the first half is wrong.

- **Prompt evolution is reachable.** `/evolve` is a live TUI slash command
  (`cmd/nerd/chat/commands.go:229`, registered in `command_categories.go:418`).
  The package is imported by six files outside itself, including
  `commands_evolution.go`, `delegation.go` and both session-boot paths.
- **Thunderdome is reachable and almost certainly already ran.** It is not a
  command; it is part of tool generation. `EnableThunderdome` defaults to true
  (`ouroboros.go:145`) and `ouroboros.go:462` calls
  `o.thunderdome.Battle(ctx, tool, attacks)` inside the generation path. The
  system reports 3 generated tools, so battles have executed.

I looked for CLI subcommands, found none under `nerd autopoiesis`, and concluded
the features were unreachable. The entry points were in chat mode and inside the
Ouroboros pipeline. Eleventh scope-of-evidence error, and the same shape as all
the others: I searched one surface and treated its emptiness as absence.

### The real finding

All three autopoiesis subsystems execute, and none of them tell the kernel:

| Subsystem | Runs? | Reports to kernel? |
|---|---|---|
| Ouroboros tool generation | yes, 3 tools | **yes** — `tool_registered`, `tool_hash`, `tool_description`, `tool_binary_path` |
| Thunderdome | yes, inside generation | **no** — `thunderdome_result` asserted nowhere |
| Prompt evolution | reachable via `/evolve` | **no** — `prompt_evolved` asserted nowhere |
| Learning | yes, 3.2 MB DB written today | **no** — loaded into Go maps at `shards/system/base.go:585` |

Ouroboros is the control that makes the rest legible: it does emit facts, which
is exactly why its status line reads a real number while the other three read
zero or nothing.

So the pattern is not "these features are missing". It is that **the
self-modification layer is doing work the executive cannot see.** A battle
outcome, an evolved atom, and an accumulated success pattern are all invisible
to Mangle, so no policy rule can react to them: nothing can prefer a tool that
survived adversarial testing, decline an atom whose evolution regressed, or
weight a shard's history. Those are precisely the decisions a logic executive
exists to make.

For a project whose thesis is that the LLM is the creative center and the kernel
is the executive, an entire self-improvement layer operating outside the
kernel's fact base is the deepest form of the inversion this ledger keeps
finding — deeper than the JIT selector case, because there at least the
mechanism was present and merely dead.

### What a fix looks like

Each subsystem needs to assert its outcome as a fact at the point it completes,
using the predicates already declared for them:

- `thunderdome_result` after `Battle` returns, in `ouroboros.go` around line 462.
- `prompt_evolved` when an evolution is accepted.
- `learned_pattern`, or the already-consumed `success_pattern`/`failure_pattern`,
  when `loadLearnedPatterns` populates its maps — writing to the kernel instead
  of, or in addition to, the Go map.

The Decls exist. The consumers, where they exist, are waiting. This is fact
emission, not new architecture — the same shape as the `compile_shard` gap,
except here the consumers are the status display and any future policy rule
rather than a dead ruleset.

Not attempted in this session: each touches a different package's completion
path, and the learning one in particular is an architectural choice about
whether shard experience belongs in the kernel, which is one of the open
decisions already recorded.

---

## F-JIT-5 — the JIT firewall keys on tag dimensions no producer emits

Five rules across both selection rulesets form the explicit-prohibition half of
atom selection. All five were dead, and the cause is the same in each: they join
`atom_tag` on a dimension the fact producer never writes.

`internal/prompt/selector.go` (the only `atom_tag` producer, ~line 1340-1352)
emits exactly twelve dimensions: `mode`, `phase`, `layer`, `init_phase`,
`northstar_phase`, `ouroboros_stage`, `intent`, `shard`, `shard_type`, `lang`,
`framework`, `state`. Neither `/tag` nor `/category` is among them.

| Rule | Keys on | Fixable? |
|---|---|---|
| `jit_selection.mg` `/production` + `/debug_only` | `/tag` | no — `PromptAtom` has no `Tags` field at all |
| `jit_selection.mg` `/dream` + `/ouroboros` | `/category` | **yes** — category is `prompt_atom` arg 2 |
| `jit_selection.mg` `/init` + `/campaign` | `/category` | **yes** — same |
| `jit_selection.mg` `/active` + `/dream_only` | `/tag` | no — same missing field |
| `jit_compiler.mg:137` `/active` + `/dream_only` | `/tag` | no — same missing field |

The two `/category` rules are the representation-mismatch shape this ledger keeps
recording: the data exists, in the fact base, on every compile — as argument 2 of
`prompt_atom(ID, Category, Priority, Tokens, IsMandatory)`, in atom form. The rule
asked for it as a tag. Fixed by reading `prompt_atom` directly.

Honest scope of the fix: it derives nothing on the workload measured, because all
7 ouroboros atoms carry `ouroboros_stages` and all 8 campaign atoms carry
`campaign_phases` — both fail-closed `regime_dimension`s that already blocked them.
It binds on a compile that legitimately sets `/ouroboros_stage` while in `/dream`
mode, which is what the rule means. Correct, and not yet load-bearing; those are
different claims.

The `/tag` pair cannot be fixed from the logic side. `PromptAtom` has no tag field,
so no producer can exist. Left in place with an `INERT` comment naming the missing
field, because a silently dead rule is worse than a labelled one.

## F-JIT-6 — wiring the second ruleset as a source saturates the token budget

The open decision "delete the discarded ruleset or wire it" resolved to wire it.
Wired the obvious way — `tentative(Atom) :- selected_atom(Atom)` — it is a defect.

| | skeleton | flesh | tokens |
|---|---|---|---|
| baseline (`/fix`, coder shard) | 35 | 32 | 26279 / 65536 = 40.1% |
| admissive bridge | 207 | 313 | 65036 / 65536 = **99.2%** |
| + `!blocked_by_context`, `!prohibited` | 46 | 208 | — |
| restrictive polarity (shipped) | 35 | 32 | 26279 / 65536 = 40.1% |

Two distinct causes, and the second is the one that matters:

1. `policy/jit_selection.mg` has no equivalent of `jit_compiler.mg`'s fail-closed
   `regime_dimension` gate — the one whose comment records a live turn that
   compiled 114 mandatory atoms carrying 25 contradictory identities. Adding
   `!blocked_by_context` recovered the skeleton half exactly (207 → 46).
2. Flesh stayed at 208 regardless. Each admitted atom recursively pulls its
   `atom_requires` dependencies into `tentative`, and that recursion is unbounded.
   No threshold on the bridge fixes this, because the inflation is not in the
   admitted set — it is in the closure over it.

So the polarity was wrong, not the threshold. `selected_atom`, `candidate_atom`
and `mandatory_atom` are admissions, and they are looser duplicates of rules the
live compiler already has; unioning two selectors can only inflate a prompt. What
the policy ruleset uniquely owns is restrictive — a firewall that propagates
through `atom_requires`, and conflict resolution that lets a mandatory atom beat a
candidate. Wired as `prohibited` and `suppressed` instead. Atom count unchanged,
ruleset live.

**The rule this establishes: a second opinion in a selector may veto, never admit.**
It is the constitution's default-deny applied one layer down.

## F-JIT-7 — the same concept under two keys, only one of them gated

`addTags` emits `atom_tag(ID, /shard, X)` and `atom_tag(ID, /shard_type, X)` from
the same `atom.ShardTypes` values. Only `/shard` is in `regime_dimension`. So a
rule spelled `/shard_type` reads identical data while silently bypassing the
fail-closed identity gate — which is how `ce65c7b9`, in making a dead rule
matchable, also made it live and ungated. That rule is what the admissive bridge
then amplified into 99.2% saturation.

`candidate_atom` now reads `/shard`. Adding `/shard_type` to `regime_dimension`
would have been the wrong repair, and dangerously so: `current_context` is
generated with `UseShort: true` and emits `/shard` only, so fail-closing a
dimension that has no context value would have starved **every** shard of its own
atoms — a capability outage wearing a safety fix's clothes.

Same family, one layer up: `current_context` (short dims, `ForceAtoms: true`) and
`compile_context` (long dims, `ForceAtoms: false`) are the same `CompilationContext`
rendered twice under two predicate names, read by the two rulesets respectively.
Both live. Worth collapsing; not attempted here.

## F-JIT-8 — the conflict-resolution machinery rests on one declaration

`beats`, `suppressed`, `conflict_loser` and the mandatory-beats-candidate
prohibition all join `atom_conflicts`. The entire 906-atom corpus declares
**one** conflict (`internal/prompt/atoms/knowledge/persistence.yaml`).

So both rulesets carry a complete conflict-resolution subsystem that can fire on
one pair of atoms. Not a wiring defect — the machinery is correct and reachable.
It is a corpus gap: the metadata that would make it useful was never authored.
Recorded rather than fixed, because deciding which atoms genuinely conflict is a
content judgement, not a logic one.

---

## F-LEARN-3 — the executive shard's learning is write-only, and the store proves it

A natural experiment was already sitting on disk. `.nerd/shards/`:

| shard | `learnings` rows |
|---|---|
| `perception_firewall` | **5** (4 failure, 1 success) |
| coder, researcher, reviewer, tester | 0 (files exist, empty) |
| **executive** | **no database file at all** |

The executive is the shard with the most decisions in the system, and it has
never persisted a single pattern.

**Root cause — Go field shadowing.** `ExecutivePolicyShard` embeds
`*BaseSystemShard` (`executive.go:78`) and *also* declares its own
`patternSuccess`, `patternFailure` and `learningStore` (`:99-101`). It then
defines its own `SetLearningStore` (`:174`) which assigns the field and returns.
The base's version (`base.go:574`) additionally calls `loadLearnedPatterns`,
which seeds each loaded pattern so a pattern learned in an earlier session
starts at threshold instead of zero.

So the executive's counters restart at 0 every session, while `recordSuccess`
only saves at count >= 5 and `recordFailure` at >= 3. **A pattern seen four
times across four sessions is never learned.** The store is written only in the
session that happens to see the same pattern five times, which is why the file
does not exist at all.

The perception firewall is the control that makes this legible: it inherits the
base's method (`perception.go:90` says so explicitly), loads at boot, and is the
one shard with rows.

**Fix.** The shadowed maps are left alone — the executive tracks *action*
patterns and the base tracks *task* patterns, and merging the namespaces would
conflate two different key spaces. It gets its own load path instead, seeding
from the stored count (`FactArgs[1]` for success, `FactArgs[2]` for failure,
accepting int/int64/float64 since JSON round-trips make the concrete type
unreliable) and falling back to the per-kind threshold when the count is absent.

RED is behavioural, not compile-level: with `executive.go` reverted and the test
file kept, 7 subtests fail — the seeded-count cases and both fallback cases.
That is the strongest evidence class available, and it is available here
precisely because `SetLearningStore` already existed and did the wrong thing.

**Correction to my own F-AUTO-6 note.** I wrote there that `success_pattern` and
`failure_pattern` were "already-consumed" kernel predicates. They are not kernel
predicates at all — no `Decl` for either exists. They are row labels in the
SQLite `learnings` table. The only learning Decl in the corpus is
`learned_pattern(Category, Pattern)` (`schemas_learning.mg:21`), which nothing
asserts. So the kernel still cannot see shard experience; this fix makes the
SQLite half work, and the move into the kernel remains open.

**Also noted, not fixed.** `learnings` is declared `UNIQUE(fact_predicate,
fact_args)` (`internal/store/learning.go:116-131`) — the owner is not in the
key, while `LoadByPredicate` filters *by* `shard_type`. Two shards that learn
the same pattern collide on write, and `ON CONFLICT DO UPDATE` means the second
writer takes the row. The loser then loads nothing, having "learned" it. Same
family as the rest of this ledger: a key that omits the dimension the reader
filters on.

---

## F-LEARN-4 — shard experience now reaches the kernel, and the kernel dump cannot see it

`shard_pattern(ShardID, Kind, Pattern, Count)` is declared and asserted by
`BaseSystemShard` at the same thresholds that already trigger the SQLite save,
plus once per pattern at load, so patterns learned in an earlier session are in
the kernel at boot rather than only after they recur. Lock discipline follows
the `promoteAtomLocked` precedent: decide under `b.mu`, release, then assert.

**What is verified.** From one correlated run (`20260812_000945`):

- `[perception_firewall] Loaded 1 success patterns` / `Loaded 6 failure patterns`
- kernel log: `Assert: shard_pattern("perception_firewall", /success, /explain:/query, 3).`
- audit log: `"event":"kernel_assert","target":"shard_pattern","success":true,"fields":{"arg_count":4}`
- `[executive] Loaded 0 success patterns` — the F-LEARN-3 load path is live at
  boot; it reads 0 because the executive has no history to load yet, which is
  the defect it fixes, not a failure of the fix.

**What is not.** `nerd logic shard_pattern` and `nerd logic "shard_pattern(S,K,P,C)"`
both return **0 facts**, in the same process that just logged the successful
asserts. Ruled out by measurement, not by argument:

- not attachment order — `[perception_firewall] Parent kernel (unwrapped from
  CortexKernel) attached` is logged *before* `Learning store attached`
- not a rejected assert — the audit records `success:true`
- not a malformed program — no `debug_program_ERROR.mg`, and the kernel log
  shows `rebuildProgram: parsed 2812 clauses` with no parse error
- not CLI arity — the explicit four-variable form returns 0 as well

So a fact can be accepted by `RealKernel.Assert`, stored, and still be absent
from every query surface. That is the same family as F-QUERY-1 (where `Query`
asked one shard while `QueryAll` merged all) and deserves the same treatment:
find which kernel instance the dump enumerates versus which one
`ck.GetPrimaryRealKernel()` hands the shard. Left open deliberately rather than
guessed at — the last four hypotheses above were each killed by a measurement,
and the honest state is "emission proven at the assert layer, visibility
unproven."

## F-LEARN-5 — my own emitter violates the Decl it was written against

The fact logged above reads:

    shard_pattern("perception_firewall", /success, /explain:/query, 3)

`Pattern` is declared `/string`. `/explain:/query` is rendered as an **atom**,
because the fact serializer treats any Go string beginning with `/` as a Mangle
name constant. The neighbouring failure rows render as `"ambiguous:/remember"`
— a quoted string — because they do not start with a slash. **One column now
holds both atoms and strings depending on the first character of the value.**

Mangle does not enforce `Decl` bounds at evaluation time — established earlier
in this ledger by isolated repro — so nothing complains. The consequence is the
one already recorded there: the rule that breaks is not the one that violates
the Decl, it is the rule two hops away that trusts it. Any future policy rule
joining `Pattern` as a string silently skips every slash-prefixed row.

This is the third instance of the leading-slash convention causing a defect this
week, and the first where the offending emitter is one written *in response to*
the earlier two. Worth stating plainly: knowing the failure mode was not
sufficient to avoid it, because the conversion is implicit and invisible at the
call site.

The real fix is not in this emitter. The serializer has no way to express a
string that begins with `/`, so no caller can be correct. Either the `Fact`
type needs an explicit string wrapper to match `types.MangleAtom`, or the
serializer must stop inferring representation from the first character. Until
then any `bound [/string]` column fed from user- or intent-derived text is
subject to the same silent split.

---

## F-LEARN-4 — WITHDRAWN. The instrument was wrong, not the kernel.

I wrote that "a fact can be accepted by `RealKernel.Assert`, stored, and still
be absent from every query surface", and filed it as the same family as
F-QUERY-1. That is false, and the evidence that kills it was in the log I had
already pulled.

In run `20260812_000945`, the `Assert: shard_pattern(...)` lines are at
`20:10:00.495063` and the kernel log's **last** line is `20:10:00.551` — the
asserts are the final kernel activity before the process exits. No
`Query: predicate=shard_pattern` appears anywhere in that log. In the later
arity-qualified run there are **no asserts at all**. So `nerd logic` took its
dump *before* system shards attached their learning stores, and in the second
run the shard was never constructed.

Two structural facts I should have established before filing anything:

1. `nerd logic` is a **separate process**. Runtime asserts live in that
   process's memory, so a fact asserted by a shard in one invocation can never
   be observed by a later one, no matter how correct the emission is.
2. Within a single invocation the CLI queries before system-shard attachment
   completes, so even the same process cannot see them.

**The correct proof is in-process, and it now exists.**
`TestInProcessVisibility_RealKernel` builds a real `core.NewRealKernel`,
attaches it via `SetParentKernel`, drives `trackSuccess` past threshold, and
asserts that `Query("shard_pattern")` returns
`shard_pattern("visibility_test_shard", /success, "inProcessPattern", 3)`,
cross-checked through `QueryAll`. `TestSetLearningStore_InProcessVisibility`
does the same for the load path. Both pass. Assert-then-query works.

This is the same error class as the eleven scope-of-evidence mistakes already
recorded here, and it is now the most expensive one: I chose an instrument
whose scope could not cover the claim, got a zero, and filed the zero as a
defect in the thing being measured. The tell was available and I walked past
it — an always-zero reading is the signature I have described in this very
ledger as "the single most reliable defect signature", and the first suspect
for an always-zero reading should have been the reader.

**What survives.** Nothing about kernel visibility. The one durable point is
operational: `nerd logic` cannot verify runtime-asserted facts, so any future
claim of that shape needs an in-process test rather than a CLI dump. F-LEARN-5
(the `Decl`-violating atom in a `/string` column) is unaffected and stands.

---

## F-LEARN-5 — RESOLVED. MangleString ships.

The gap was not in the emitter, it was that no emitter could be correct:
`MangleAtom` forces a name constant and a plain Go string is guessed by shape,
so a `/string` column fed from intent-derived text had no correct spelling.
`types.MangleString` is the missing counterpart, handled beside `MangleAtom`
in both `Fact.String` and `Fact.ToAtom`.

Verified live on the production value, both from a real boot with
perception_firewall loading its 1 success and 6 failure patterns:

    before  shard_pattern("perception_firewall", /success, /explain:/query, 3)
    after   shard_pattern("perception_firewall", /success, "/explain:/query", 3)

The existing shape inference is deliberately untouched — the repo depends on it
and changing it is a behavioural change, not a fix. Two tests pin it so a later
cleanup has to be a conscious decision: a plain "/success" still yields a name,
"not an atom" still yields a string.

**Still open, and larger than this fix.** The inference exists because the
serializer does not consult the `Decl` it is writing against. The kernel knows
`shard_pattern` argument 3 is `/string`; the serializer guesses from the first
character instead. Every other `bound [/string]` column fed from user- or
intent-derived text remains subject to the same silent split until a caller
opts into `MangleString`. Consulting the Decl at assert time would close the
family rather than one instance — and would fit the repo's thesis better, since
it is the logic layer, not the Go layer, that already holds the type.

---

## F-MANGLE-1 — a second Fact serializer, and MangleString is invisible to it

Surfaced by the self-audit campaign, which read `internal/mangle/engine.go` and
noted in its own artifact that it defines a `Fact` type distinct from
`types.Fact` — "do not conflate with types.Fact". It was right, and the
consequence is one I created an hour earlier.

There are two `Fact` types with two independent `String()` serializers:

| | `internal/types/types.go` | `internal/mangle/engine.go:81` |
|---|---|---|
| fields | Predicate, Args | Predicate, Args, Line, Timestamp |
| `MangleAtom` case | yes | **no** |
| `MangleString` case | yes (new) | **no** |
| string → name test | `isValidMangleNameConstant`: rejects `//`, >2 slashes, file extensions, whitespace, then validates with `ast.Name` | bare `strings.HasPrefix(v, "/")` |

Two consequences.

**The wrapper types fall through to `default:`.** `fmt.Sprintf("%v", v)` renders
a `MangleString` **unquoted** — the exact opposite of what the type exists to
guarantee. A caller who correctly reaches for `MangleString` and whose value
routes through `internal/mangle.Fact` gets a name constant anyway, silently.
This is a trap the previous commit introduced by fixing one serializer and not
the other, and it is worth naming as such: adding a type that means "trust me,
this is a string" without covering every serializer creates a guarantee that
holds in one half of the codebase.

**The inference is strictly cruder.** `types.Fact` refuses to treat a
path-shaped or comment-shaped value as a name; this one accepts anything with a
leading slash and never validates with `ast.Name`, so a malformed name reaches
the program text rather than being quoted.

### Two hypotheses this killed, recorded because the checking is the point

- *"Absolute source paths break the autopoiesis checker on Linux, masked on
  Windows by drive letters."* Wrong. `ExtractASTFacts` parses with the literal
  name `"generated.go"` (`checker.go:131`), so `e.fileName` is never a path on
  any platform.
- *"`eligible_task` derives nothing, so the campaign's task selection is Go, not
  logic."* Wrong. The log line beside it says the phase's only task is already
  `/in_progress`, so there is correctly nothing eligible; the dependency
  fallback then matches nothing for the same reason.

Both looked like the defect family this ledger is full of, and both were
zero-readings with an innocent explanation one line away. Same lesson as the
withdrawn F-LEARN-4, learned twice more in one hour.

### What is actually worth doing

Add the `MangleAtom` and `MangleString` cases to `engine.go`'s switch, and have
it reuse the same validation rather than `HasPrefix`. No import cost — that file
already imports `codenerd/internal/types` (line 20), and `internal/types` does
not import `internal/mangle`, so there is no cycle.

The deeper answer remains the one recorded under F-LEARN-5: neither serializer
should be guessing. The `Decl` already states whether an argument is `/string`
or `/name`, and the kernel holds it at assert time.

---

## F-TOOL-1 — codeNERD cannot search its own codebase past 50 matches, and it found this out itself

The self-audit campaign hit this mid-run and wrote the workaround into its own
artifact:

> grep `^\s*Decl\s+\w+\(` with file_pattern, **bypasses 50-cap by per-file
> sharding; global grep hard-capped at 50 verified even with max_results=500**

It asked for 500 results, received 50, verified the cap empirically, and
re-planned around it by sharding the query per file. Then it carried on. This is
the agent discovering a defect in its own instrument, working around it, and
documenting the workaround — which is exactly what the dogfood exercise is for,
and is a better bug report than the one I would have written.

**Root cause.** `internal/tools/core/search.go` reads its caller-supplied limits
with a bare type assertion to `int`:

- `:57`  `if mr, ok := args["max_results"].(int); ok && mr > 0` — glob, default 100
- `:208` `if cl, ok := args["context_lines"].(int); ok`         — grep
- `:213` `if mr, ok := args["max_results"].(int); ok && mr > 0` — grep, default 50

LLM tool-call arguments are JSON-decoded, and `encoding/json` without
`UseNumber()` materializes every number as `float64`. So the assertion never
succeeds in production, the override is silently discarded, and the default is
always used. `max_results: 500` and `max_results: 5` are the same request.

**What makes this the sharpest instance of the recurring pattern.** This exact
bug was already found and fixed three separate times in this repo, and none of
the fixes reached the one tool that surveys the codebase:

| package | mechanism | status |
|---|---|---|
| `internal/tools/research` | `argInt` (`numeric_args.go`) | fixed |
| `internal/tools/shell` | `coerceInt` (`execute.go:52`) | fixed |
| `internal/tools/codedom` | inline `float64` fallback (`lines.go:66-76`) | fixed |
| **`internal/tools/core`** | bare `.(int)` | **broken** |

`numeric_args.go`'s own comment states the consequence in the general case —
"caller-supplied limits (max_docs / max_length / max_results) were discarded and
the default was always used" — and names codedom and shell as prior art. The
package it was not applied to is the one whose truncation does the most damage.

**Why it matters beyond the tool.** A silent 50-match ceiling on grep is a
machine for manufacturing exactly the error this ledger records more than any
other: reasoning from a partial view believed to be complete. Eleven-plus
scope-of-evidence mistakes are recorded here, at least one withdrawn in git
after being committed. Some fraction of those were mine reading truncated grep
output and treating absence as evidence. It is also a token-efficiency defect:
the correct workaround is per-file sharding, which is N calls where one would do,
and the campaign paid that cost in real tokens to finish its inventory.

---

## F-ROOT-2 — the undeclared-root-write guard does not cover campaign tasks

The self-audit campaign wrote 152 KB into the repository root — `decl_inventory.md`
(72 KB), `decl_canonical_map.md` (45 KB), `mangle_internal_consumers.md` (35 KB)
and a `reports/` directory — and **the guard did not fire once**. Verified by
grepping every log from the run: the only "undeclared" hits are the campaign
narrating its search for undeclared *predicates*.

The guard works. It caught a stray `nerd.exe~` during a `fix` run the same hour.
It simply is not on this path. Commit e9d9dd17 extended it from the direct verbs
to the shadow, what-if and dream spawn sites, and campaign task execution is a
fourth spawn path that was not in that list.

**Sixth instance of the same pattern, and this one is mine.** The ledger already
records it for the Ouroboros tool contract, the original root-write warning, the
Meta call_id pairing, the prompt-evolution callback, and F-TOOL-1's numeric
coercion. Each time the fix was applied per call site and a whole family stayed
uncovered. I wrote the commit message for e9d9dd17 arguing that a guard belongs
at a choke point rather than at call sites, and then extended it to three of the
four call sites.

The lesson is not "add campaign to the list" — that is the same mistake a fifth
time. It is that spawn paths need one entry point, so a new one inherits the
guard instead of having to be remembered. Recorded rather than patched, because
picking that choke point is a design decision worth making deliberately.

Related: the deliverables belong in `.nerd/campaigns/<id>/artifacts/`, where the
same campaign already wrote 17 files correctly. Two writers with two
destinations, one of them the repo root.

---

## F-LEARN-5 addendum — the Decl-aware converter exists, and is deliberately not used

Correcting my own note above. I wrote that the real fix is for the serializer to
consult the `Decl` instead of guessing, and called it the direction that would
"fit the repo's thesis better". Both halves need qualifying: it exists already,
and there is a documented reason the kernel does not use it.

`internal/mangle/engine.go:564 factToAtomLocked` is a complete Decl-driven
conversion. It rejects undeclared predicates, checks arity against the symbol,
reads `decl.Bounds`, maps `/name`, `/string`, `/number`, `/float64`, `/time`,
`/duration` and `/bytes` to concrete `ast` types, and converts each argument to
its declared type. That is the design I proposed as though it were hypothetical.

The kernel does not use it, on purpose. `internal/core/kernel_eval.go:401`:

> IMPORTANT: We deliberately convert facts using types.Fact.ToAtom() (the
> kernel's own encoding) rather than letting DifferentialEngine.ApplyDelta call
> mangle.Engine.factToAtomLocked. The two paths apply different type-coercion
> rules [...] Using ApplyAtomDelta keeps the encoding identical to the
> full-rebuild path so query results match bit-for-bit.

So the constraint is not that nobody thought of consulting the Decl. It is that
**two encoders exist and the incremental and full-rebuild paths must agree**, or
identical queries return different results depending on which path last ran.
Any adoption of the Decl-aware encoder has to happen in both paths in one
change, with the equivalence tested, not swapped in at one call site.

That is a materially different piece of advice than "the serializer should
consult the Decl", and it is the third time this session that reading the
comment above the code changed the recommendation. Recorded before attempting
anything, because the two previous times I overrode a documented decision I had
to revert it after measuring.

**What stays safely in scope for F-MANGLE-1:** `internal/mangle/engine.go's
Fact.String()` gains the `MangleAtom` and `MangleString' cases. That method
produces persisted fact text, not the in-memory atom encoding, so it is outside
the bit-for-bit constraint entirely -- and today both wrapper types fall through
to `default` and render unquoted, which makes `MangleString` mean its opposite
on that path.

---

## F-CAMP-1 — a campaign reported total success on a wholly fabricated report

The headline result of running codeNERD against itself. An `audit` campaign was
given a real goal — find every predicate declared in the Mangle corpus but never
asserted by Go, and vice versa — and seeded with five gaps this ledger already
recorded (`thunderdome_result`, `prompt_evolved`, `success_pattern`,
`failure_pattern`, `learned_pattern`) as a self-check on its own correctness.

**It reported complete success.** 5 of 5 phases `/completed`, 40 of 40 tasks
completed, every phase gate logging `Checkpoint PASSED`, final line
`🏆 Campaign completed successfully`.

**The final report is fabricated end to end.** Every identifier it cites appears
in **zero files** in this repository:

| cited by the report | files containing it |
|---|---|
| `decl.mangle.validate`, `decl.mangle.route`, `decl.mangle.cache` … | 0 |
| `RegisterMangleTransform`, `HandleMangleRoute`, `ValidateMangle` | 0 |
| `CacheMangle`, `CleanupMangle` | 0 |

It states "Total Declarative Entries (Decl) Reviewed: **18**". The corpus holds
**1550**. It contains a section titled **"Proof: Five Known Gaps Rediscovered"**
which concludes "5/5 known gaps rediscovered and evidenced above" — while
mentioning **none** of the five predicate names, mapping them instead to five
findings it invented. It closes with a "No-Modification Statement" asserting no
files were created outside the report, which is also false: the run left four
files in the repository root.

Its own methodology section says why: *"Analysis performed strictly on supplied
content without filesystem or network browsing."* The synthesis task received no
upstream artifacts and did not read the repository, so it confabulated a
plausible-shaped audit rather than failing.

**Phases 1–4 were genuinely good**, which is what makes this dangerous rather
than merely broken. They produced 17+ durable artifacts with real `file:line`
citations, a 72 KB Decl inventory, a correct reading of the two `Fact` types,
and — unprompted — the discovery of F-TOOL-1 in its own grep tool. All of that
real work sits on disk, and the deliverable a reviewer would actually read
throws it away.

### Root cause, with the precedent sitting ten lines below it

`internal/campaign/checkpoint.go`, `runManualReviewCheckpoint`:

```go
// In non-interactive mode, we can't do manual review
// Return true with a note that review was skipped
return true, fmt.Sprintf("Manual review for phase '%s' skipped ...
```

The decomposer chose `/manual_review` for every phase, so every gate returned
PASSED without checking anything. Ten lines below, `runShardValidationCheckpoint`
already carries the fix and the reasoning:

> Fail closed. This used to return PASS, which made "we did not check"
> indistinguishable from "we checked and it was fine" — the single most
> dangerous answer a verification gate can give, and one that survived precisely
> because it was silent.

That reasoning was applied to `/shard_validation` and never to the method beside
it. **Seventh instance of the choke-point-versus-call-site pattern in this
ledger, and the most expensive one yet.** The others cost a dead rule or a
missing fact; this one cost the truth value of an entire campaign.

### The fix

`/manual_review` in non-interactive mode now escalates to
`runShardValidationCheckpoint`, which spawns a reviewer shard and inspects the
phase's objectives and completed tasks — and which already fails closed when no
task executor is wired. Returning plain `false` was rejected: it would block
every campaign whose decomposer picked this method, which is most of them.
Escalation keeps the capability and removes the rubber stamp.

RED is behavioural, not compile-level: with the fix reverted, four subtests fail,
including `fail_closed_without_task_executor` — the exact case that previously
returned `true`.

### What this does not fix, and should be recorded as owed

- **Synthesis without inputs.** The report task confabulated because it had no
  artifacts and no filesystem read. A gate that fails closed will now catch it,
  but the underlying defect is that phase 5 was not handed phase 1–4's output.
- **`nerd campaign start` exits 0 on a timeout-pause**, so a script or CI reading
  the exit code sees success for an incomplete run.
- **Two writers, two destinations.** The same campaign wrote correctly into
  `.nerd/campaigns/<id>/artifacts/` and also dropped four files in the repo root,
  where F-ROOT-2's guard does not look.
- **Task inflation.** Phase 4 was planned with 5 tasks and executed 11; phase 3's
  producer-extraction task was restated three times with growing verbosity. Real
  tokens, no added information.

The lesson worth carrying beyond this repo: the campaign did not fail loudly
anywhere. Every individual signal said success. Only reading the deliverable and
grepping for one cited symbol revealed that none of it was real — which is the
one check no automated gate in the system was performing.

---

## F-CAMP-2 — the fabricated report had nothing to fabricate from

Root cause of F-CAMP-1, found by following the task record rather than the code.

`buildTaskInput(task)` appends `=== CONTEXT FROM TASK <id> ===` blocks for every
entry in `task.ContextFrom`, pulling each dependency's result. It works. Its
**only** caller was `buildTaskInputWithSpecialistKnowledge`, which is reached
**only** from `executeWithExplicitShard`. So dependency context reached a task if
and only if the decomposer happened to give it an explicit shard.

Every phase-5 task in the audit campaign had an empty `shard`, so all were
type-routed. The report task went `executeDocumentTask` → `executeFileTask`,
which built its shard input as:

    fmt.Sprintf("%s file:%s %s", action, targetPath, task.Description)

The description and nothing else. The shard was asked for "summary counts,
methodology, Decl vs Go cross-ref tables, four required columns per finding,
dedicated section proving five known gaps rediscovered" and handed no data. It
returned a document with exactly those headings and invented content under each.
**The report's structure mirrors its prompt because the prompt was its only
input.** Nothing in the system was lying; one component was asked to summarise
work it had never been shown.

Two compounding factors from the same run:

- Only **13 of 40** tasks had `context_from` populated at all.
- Phase 5's tasks are duplicated — each of its four appears twice — and the
  duplicate that actually wrote the report has `context_from` empty. The
  replanner inflated the phase and dropped the dependency edges on the copies.

### Fix

All four type-routed spawn sites now use `o.buildTaskInput(task)`: research,
file/document, and both test handlers. That is the complete set, verified by
grepping every `spawnTask` call in the package — the explicit-shard site already
injects, and the two retry paths deliberately send a narrowed instruction and
are left alone.

Covering the whole family rather than the one site that failed is the point.
This ledger records six prior defects caused by doing the opposite, and F-CAMP-1
two commits ago was the seventh.

Incidental repair: the test handlers built `generate_tests file:%s` and
`run_tests package:%s` and did not include the task description **at all**, so a
test task never saw what it was asked to test beyond a path.

RED is behavioural: with the handler file reverted and the tests kept, all three
new tests fail.

### Still open

The decomposer and replanner produce tasks with no `context_from`, and duplicate
tasks that lose it. A synthesis task with no declared dependencies will still
receive nothing — correctly, since nothing is declared. Making the decomposer
wire dependencies for synthesis phases, and stopping the replanner from dropping
them on duplicates, is the remaining half and is not attempted here.

### The shape of this pair, worth keeping

F-CAMP-1 and F-CAMP-2 are the same event seen from two ends. One component was
asked to produce a document without being given the material; another was asked
to verify it and reported PASSED without checking. Neither failed loudly. The
run ended with "Campaign completed successfully" and a deliverable in which every
cited symbol was invented — and the only thing that surfaced it was a human
grepping one cited identifier and finding zero files.

---

## Module exercise coverage — measured, not assumed

The standing goal includes "every module exercised on itself at least once".
That was being asserted rather than measured, so here is the measurement.

**Method.** Every log file written under `.nerd/logs/` by live runs against this
repository, grouped by subsystem category and summed. 12 runs. A category with a
file but only a few dozen bytes has been *initialised*, not exercised — the file
is created at boot whether or not the subsystem does anything.

**Exercised — 23 subsystems with real live output:**

`kernel` (12 MB), `campaign` (6 MB), `performance`, `embedding`, `store`,
`virtual_store`, `session`, `tools`, `shards`, `articulation`, `api`, `jit`,
`problems`, `context`, `perception`, `system_shards`, `boot`, `autopoiesis`,
`world`, `build`, `tactile`, plus `audit` (20 MB) and `llm_io` (52 MB) which are
cross-cutting recorders rather than subsystems.

**Not exercised — 3:**

| subsystem | evidence |
|---|---|
| `browser` | 72 bytes per run across 8 runs — a header and nothing else |
| `researcher` | 75 bytes per run across 8 runs — same |
| `northstar` | 132 bytes across 3 runs |

**Marginal:** `dream` writes 576 bytes per run. `/dream` and `/shadow` were
exercised earlier in this session and are known to work, so this is a logging
gap rather than an execution gap — worth noting because it means this
measurement under-reports a subsystem that did run.

**What it would take to close the three.** `browser` needs a page-driving task,
which the browser tool suite and `internal/browser/specs` support. `researcher`
is the Context7-backed external lookup path and needs `context7_api_key` set;
the `/research` *shard* did run repeatedly during the campaign, but it logs under
`shards` and `campaign`, so the empty `researcher` category is the external
lookup tooling specifically, not research as an activity. `northstar` is the
project-vision wizard, reachable from chat.

**Caveat on the method, stated because it changes how the numbers should be
read.** Log category does not map one-to-one onto Go package. A subsystem can
run while logging under another category — `dream` demonstrates exactly that.
So this is a lower bound on what has been exercised, and the three named above
are candidates for "not exercised", not proof of it. Confirming any of them
requires driving the feature and watching its own log grow, which is how the
other 23 were confirmed.
