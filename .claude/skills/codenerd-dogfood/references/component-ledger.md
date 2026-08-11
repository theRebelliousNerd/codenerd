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
