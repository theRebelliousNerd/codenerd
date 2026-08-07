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
### ✅ Perception / intent classification — `internal/perception`, `internal/system/factory.go`
- **Exercise:** `nerd run "review internal/features/features.go for correctness"` on the two-tier stack (run 15).
- **Upgrade:** **F-CLASS-1** (b097deff) — classification runs on **every** interactive turn before anything else, and was still served by the main client (`factory.go` built the transducer with `bctx.llmClient`). One `nerd run` spent **91 seconds** in classification alone. `NewClassificationClientFromConfig` had existed for exactly this since the P2 model-tiering work — its own doc comment calls main-model classification "a bug" — but was only wired into *shard registration*, never into the transducer. A wiring gap, not a missing feature. Now resolves worker tier → main fast tier → main-with-warning. Measured 91s → 47s, and **more accurate**: the same input classified as `/explain` with no target on the reasoning model and `/review` with the correct target on the cheap no-thinking client.
- **Verification of the whole two-tier design**, one `/review` turn in `_llm_io.log`:
  `classification → muse-spark-1.2`, `planner-tools → qwen3.8-max`, `planner-tool-results → qwen3.8-max` (×8, whole tool loop on one model).
- **Open:** 47s is still slow for an interactive turn. Next lever is a genuinely small classification model (`classification_model`), which needs a verified cheap Qwen/Meta tier name.

- **Open (F-STATUS-1, not fixed):** `nerd status` prints "✓ Z.AI API key configured" purely from the ambient `ZAI_API_KEY`, regardless of the configured provider — the same env-over-config confusion as F-INIT-1, in the one command whose entire job is telling you what's configured.
- **Open (F-INIT-2, not fixed):** init's summary never aggregates LLM failures. The "quality" score is an atom-count proxy (`agents_knowledge.go:132-142`) computed from Context7 research, which succeeds independently of the LLM — so a run where every LLM call fails still reports Good/80%. The two warnings it did print named neither the failure count nor the provider.

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
  - **Residual, not swept:** virtual/external predicates built by `appendAtom` (`virtual_store_predicates.go:901`) bypass `k.facts` and therefore the new coercion — it is a free function with no kernel/Decl access. Most callers echo already-bound query args, and none is known to emit a float, but `query_traces`' `duration` slot is unverified. Also found while sweeping (separate defect classes, not fatal): `strategic_knowledge` is asserted with **no `Decl` anywhere**, and `internal/tactile/audit.go:715` asserts `test_coverage` with 2 args against `Decl test_coverage(FilePath)` (arity 1).

### ⬜ Not yet exercised / fully swept
- Kernel core — `internal/core` (fact flow, derivation). Exercised every run; routing layer well-swept (F-ROUTE-2/-3, F-TOOL-1/-3), numeric typing now swept (F-NUM-1). Deeper derivation-chain sweep (rule liveness, virtual-predicate perf) remains open-ended.

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
