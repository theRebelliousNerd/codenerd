# codeNERD — Unfinished Feature Backlog

> Generated 306 actionable open items from `Docs/architecture/*/TODO.md`.
> Priorities are the ones already declared in each corpus TODO (P0 = honesty/wiring, P3+ = nice-to-have).

## Counts by corpus

| Corpus | Open |
|---|---|
| mcp | 20 |
| usage | 20 |
| retrieval | 19 |
| campaign | 18 |
| regression | 18 |
| build | 17 |
| tools | 17 |
| northstar | 16 |
| transparency | 16 |
| autopoiesis | 15 |
| persist | 15 |
| context | 14 |
| sqlpragmas | 14 |
| world | 14 |
| browser | 13 |
| diff | 13 |
| init | 13 |
| logging | 13 |
| features | 11 |
| types | 10 |

| **total** | **306** |

## Items by priority

### P0 (56)

- **[autopoiesis]** Route all production tool creation through `ExecuteOuroborosLoop` (chat `generate_tool`, `ExecuteAction`).
- **[autopoiesis]** Fail closed when `go_safety.mg` fails to load (no empty policy).
- **[autopoiesis]** Audit default `AllowExec: true` — document or tighten for untrusted workspaces.
- **[browser]** On-demand construct + inject into `TactileRouterShard` / chat model on first browser action
- **[browser]** Document operator risk: CLI engines do not feed chat kernel (already true — keep explicit in UX)
- **[build]** Align `env.go` package comment consumer list with real importers (or adopt the missing consumers).
- **[build]** Thread `*config.UserConfig` into autopoiesis `ToolCompiler` / `Thunderdome` compile paths when available.
- **[build]** Split **detection root** (workspace) from **module dir** (`cmd.Dir`) in call sites that need monorepo CGO.
- **[campaign]** Audit every production `NewOrchestrator` call site sets non-nil `TaskExecutor`
- **[campaign]** Surface risk preflight blocks in CLI/chat UX (not only CategoryCampaign logs)
- **[campaign]** Golden tests for `ToFacts` predicate/arity stability
- **[campaign]** Confirm checkpoint-fail never completes phase (regression test)
- **[context]** Audit every chat path that injects history for `IsCompressionActive` parity (perception + articulation + session context).
- **[context]** Keep race coverage green: `go test -race ./internal/context/...` on activation changes.
- **[context]** Preserve issue weight clamp + score caps when editing `activation_scoring.go`.
- **[diff]** Deep-copy `Hunks`/`Lines` on cache hit (or store immutable snapshots)
- **[diff]** Bound cache size (LRU / max entries / max total bytes)
- **[diff]** Optional content verification on cache hit (lengths + secondary hash)
- **[init]** Document operator embedding prerequisites next to CLI `nerd init` help text (code change — track here).
- **[init]** Confirm force-reinit never deletes `preferences.json` without explicit wipe path.
- **[mcp]** Include `policy_mcp.mg` in kernel policy load (or relocate under `internal/core/defaults/policy/`)
- **[mcp]** Emit EDB on discover/save: `mcp_server_*`, `mcp_tool_registered`, capability, category, domain, affinity, condensed, analyzed
- **[mcp]** Retract/update facts on disconnect / re-analyze
- **[mcp]** Golden tests for `mcp_tool_selected` given fixture EDB + vector scores
- **[northstar]** **Single vision authority**: define and implement bridge between `.nerd/northstar.json` and `Store.SaveVision` (or reverse: export JSON from Store for CLI).
- **[northstar]** **Kernel wire parity**: call `SetParentKernel` in `session_boot.go` the same way as `session_shared_boot.go`.
- **[northstar]** **Document operator path**: after wizard, how facts get into Guardian DB (runbook in CLI corpus if needed).
- **[persist]** Choose first production caller (campaign fact bag **or** world code-index freeze **or** kernel debug export)
- **[persist]** Implement export/import at that site using `factsnap.Write` / `Read`
- **[persist]** Add integration test: domain → facts → snap → facts → domain/kernel equalish
- **[persist]** Update [08-WIRING-AND-INTEGRATION.md](08-WIRING-AND-INTEGRATION.md) with real call sites when done
- **[regression]** **Wire one real consumer** — prefer `nerd regression run` or a campaign assault optional stage that calls `LoadBattery`/`RunBattery`.
- **[regression]** **Reconcile package comment** — until wired, change “can be run as part of Nemesis gauntlets” to “intended for” or implement the hook.
- **[regression]** **Decide empty-suite policy** for any host (vacuous pass vs config error).
- **[retrieval]** Call `Model.Retriever.FindRelevantFiles` or `TieredContextBuilder.BuildContext` from `seedIssueFacts` (or session observe phase) under timeout.
- **[retrieval]** Assert `candidate_file` / `keyword_hit` / multi-tier `tiered_context_file` / `issue_context` into kernel EDB.
- **[retrieval]** Resolve paths before asserting `file_mentioned` / tier facts (reuse `findFile` logic).
- **[retrieval]** Update glass-box or context logs when sparse search runs (prove liveness).
- **[sqlpragmas]** When changing `pragmasFor`, update unit + integration tests in the same PR.
- **[sqlpragmas]** When adding a profile, update this corpus (`IMPLEMENTED_SPEC`, API, failure modes).
- **[sqlpragmas]** Never add mid-layer imports to this package.
- **[tools]** Apply `resolveWorkspacePath` (or equivalent) to `glob`, `grep`, `search_code` base_path/path.
- **[tools]** Apply workspace containment to codedom path args (elements + line tools).
- **[tools]** Contain shell/git `working_dir` to workspace root.
- **[tools]** Contract with session: empty `AllowedTools` should not mean “all tools” when safety gate is on (document + implement).
- **[transparency]** **Unify or dual-feed shard visibility:** call `TransparencyManager.StartShard` / `UpdateShardPhase` / `EndShard` from ShardManager lifecycle **or** stop advertising Active Operations from unfed Observer.
- **[transparency]** **Type `SetTransparencyManager`:** replace `any` with a small interface in `internal/types` (Enable/phase methods) **or** remove dead storage.
- **[transparency]** **Status honesty:** mark `StreamReasoning` / `JITExplain` / `OperationSummaries` as experimental in `GetStatus` until wired, or implement wiring.
- **[types]** Audit hot assert paths for remaining bare `Args[i].(T)` and `%v` dumps; migrate to `Extract*` / typed construction
- **[types]** Ensure all production kernels used with multi-op updates implement `KernelTransactor` (mocks too)
- **[usage]** Call `usage.FromContext` + `Track` from **every** production `perception.LLMClient` that receives usage metadata (not only ZAI).
- **[usage]** Confirm streaming completion paths attach the same context and Track **once** with final billed tokens.
- **[usage]** Standardize provider string ids (match config engine names).
- **[world]** Unify path canonicalization across full scan, incremental scan, and deep cache keys.
- **[world]** Expand or document `WorldPredicates` vs all emitters (`entry_point`, CodeDOM, git, scope).
- **[world]** Property/integration test: full vs incremental produce identical Path identities.

### P1 (70)

- **[autopoiesis]** Parity check post-boot: registry tool count vs `tool_registered` facts.
- **[autopoiesis]** Confirm `StartKernelListener` started on all interactive boot paths; document poll interval.
- **[autopoiesis]** Expand e2e: scripted multi-stage Ouroboros (safety fail → regen → thunderdome survive).
- **[autopoiesis]** Campaign pregen always uses same safety depth as chat Ouroboros helpers.
- **[browser]** Implement Go `honeypot_suspicious_url` assertion or drop from reason table/policy
- **[browser]** Align Go reason checklist with `browser_honeypot.mg` (clip/overflow/no_keyboard coverage)
- **[browser]** Optional gate: Click/Type refuse when `is_honeypot` for resolved element (caller-level or manager-level flag)
- **[browser]** Cobra: `list`, `screenshot`, `click`, `type`, `fork`, `honeypot`
- **[build]** Inventory all `exec.Command("go", …)` sites; mark each `uses internal/build` or `exempt: reason`.
- **[build]** Document exemption for `tools/shell` and `tactile` env builders in those packages’ architecture docs.
- **[build]** If preflight / verification spawns `go test` for the monorepo, route env through `GetBuildEnv`.
- **[campaign]** Default-wire IntelligenceGatherer when Cortex boot has world/git/MCP
- **[campaign]** Define hard vs soft advisory blocking contract; implement hard path
- **[campaign]** Document or implement Cobra assault configuration parity with chat
- **[campaign]** Nested `campaign_ref` e2e for propagate/absorb/transform policies
- **[context]** Measure frequency of Go fallback vs kernel inclusion in production logs; reduce dual-path drift.
- **[context]** Expand tests with loaded `context_compilation.mg` so `should_include_context` path is first-class.
- **[context]** Finish C3: consume `should_mask_observation` in Go when building summaries (assert path already present).
- **[features]** Wire `cmd/tools/verify_taxonomy` to `features.IsTaxonomyFastEnabled()` (and ensure SetActive/env path consistent with resolveBool, not only `== "1"`).
- **[features]** Align comments: remove “hard short-circuit” language for PerShardFacts where accessor is normal resolveBool; update `kernel_eval.go` DiffEval default claim; fix SystemShards field env comment.
- **[init]** Wire `InteractiveAgentSelection` when `InitConfig.Interactive` and TTY available.
- **[init]** Wire `--define-agent` / Type U into CLI `runInit` merge path.
- **[init]** Attach `ProgressChan` from chat `/init` if slash init exists.
- **[init]** Ingest `populateProjectAtoms` into `prompts/corpus.db` for JIT visibility.
- **[logging]** Align config schema: `json_format` bool vs `config.LoggingConfig.Format` string — pick one and document/load both
- **[logging]** Document (and optionally implement) loading from the same file the rest of the app treats as source of truth
- **[logging]** LLM I/O redaction hooks for common secret patterns
- **[mcp]** Retain `MCPIntegrationBridge` on boot/system context for compile access
- **[mcp]** Readiness signal after ConnectAll + initial discover
- **[mcp]** Wire `CompileToolsForShard` (or equivalent) into shard/articulation JIT prompt path if product requires MCP tools in LLM context
- **[mcp]** Fix `cmd_mangle_check` path: `internal/core/defaults/schemas_mcp.mg` (not missing `internal/mcp/schemas_mcp.mg`)
- **[mcp]** Align package README structure section with on-disk files
- **[northstar]** Persist wizard completion via `Guardian.UpdateVision` so `/alignment` and campaigns see the same vision.
- **[northstar]** CLI: `nerd northstar history|drift|state` over SQLite.
- **[northstar]** Emit or drop unused relational facts (`northstar_serves`, `supports`, `addresses`).
- **[northstar]** Encode mitigation free text (or hash) instead of constant `/mitigation`.
- **[persist]** Optional CLI: export/import fact snapshots under `.nerd/snapshots/`
- **[persist]** Document canonical workspace paths once chosen
- **[regression]** Example `battery.yaml` for codeNERD workspace (build + `go test ./internal/regression/...` smoke).
- **[regression]** Optional seed from `nerd init` under `.nerd/regression/`.
- **[regression]** Print-friendly summary helper or CLI table (pass/fail/duration).
- **[regression]** Persist last run under `.nerd/regression/runs/` (host-side OK).
- **[retrieval]** Remove dead `FindRelevantFiles(ctx, "", …)` call in `searchKeywordFiles`.
- **[retrieval]** Fix T4 definition search to not treat regex anchors as literals.
- **[retrieval]** Inject optional embedding query for real semantic T4 with heuristic fallback.
- **[retrieval]** Add Go import expander for T3.
- **[retrieval]** Max file size + binary skip in `searchSingleKeyword`.
- **[retrieval]** Cap max hits per keyword before ranking.
- **[sqlpragmas]** Periodic audit: product `sql.Open` sites without `ApplyDefaultPragmas` (or explicit exception comment).
- **[sqlpragmas]** Prefer `sqlpragmas` import in new mid-layer packages that must not touch `store`.
- **[tools]** Add `git_diff`, `git_log`, `git_operation` to `modular_tool_allowed` in `intent_routing.mg` (or explicitly document intentional omit).
- **[tools]** Add `research_cache_clear`, `research_cache_stats` to Mangle routing if they should be agent-callable.
- **[tools]** Use `coerceInt` for search tool integer args (`max_results`, `context_lines`).
- **[tools]** Thread workspace root through tool context; reduce env-only coupling (TODO already in `workspace_guard.go`).
- **[tools]** Golden test: RegisterAll names ⊆ Mangle modular_tool_allowed ∪ intentional_exceptions.
- **[transparency]** Auto `ReportSafetyViolation` on constitutional / `permitted` deny with rule + action + target.
- **[transparency]** Emit `CategoryJIT` events from prompt compiler when JIT explain is on.
- **[transparency]** Align config comments (`GlassBoxCategories`) with `CategoryRouting`.
- **[transparency]** Wire `OperationSummaries` to post-turn summary using `FormatOperationSummary`.
- **[transparency]** Add drop counters to `GlassBoxBusStats` and ToolEventBus.
- **[types]** Plan deprecation path: `KernelInterface` / `KernelFact` → full `Kernel` + adapters only at edges
- **[types]** Decide: typed context keys for spawn priority / model capability (match session key pattern)
- **[types]** Add container (`map`/`slice`) ToAtom table tests
- **[usage]** Atomic save: write temp file then rename onto `usage.json`.
- **[usage]** Fix dirty re-arm: under one critical section, Save then if mutations occurred while saving, keep dirty and re-arm timer.
- **[usage]** Flush on Cortex close / chat shutdown (`Save` if dirty).
- **[usage]** Use or remove `autoSaveTimer` field; prefer cancelable timer.
- **[world]** Multi-lang Cartographer `MapFile` (or dedicated deep API) for py/ts/js/rs `code_defines`/`code_calls`.
- **[world]** Implement real `dependency_link` emission **or** remove from replace-set and marketing claims.
- **[world]** Coordinate dual writers: chat incremental vs `WorldModelIngestorShard` (ownership matrix).

### P2 (76)

- **[autopoiesis]** Unify Yaegi vs binary execution policy (config switch + docs).
- **[autopoiesis]** Human-in-the-loop default for SPL auto-promote.
- **[autopoiesis]** Agent spec → runtime scheduler ownership decision (shards vs autopoiesis).
- **[autopoiesis]** Export optional metrics (generation latency, reject rates).
- **[autopoiesis]** Golden suite per `ViolationType`.
- **[browser]** TUI browser status / session list slash command
- **[browser]** VS `handleBrowse` thin delegate to shared manager (if design accepts)
- **[browser]** Contract tests: SnapshotDOM predicates ⊆ Decl in schemas_browser.mg
- **[build]** Implement real test specialization **or** delete `GetBuildEnvForTest`.
- **[build]** Either consume `GoFlags` (helper to extend argv) or remove field from build + config with migration note.
- **[build]** Collapse dual `BuildConfig` types (`build` vs `config`) to one.
- **[build]** Normalize env keys with `setEnvKey` across all merge stages to prevent duplicates.
- **[campaign]** Migrate high-traffic roles from `prompts.go` into `internal/prompt/atoms/`
- **[campaign]** Keep `StaticPromptProvider` as thin fallback only
- **[campaign]** Refresh `internal/campaign/README.md` modular map + date (code package doc)
- **[context]** Validate target compression ratio on real multi-hour sessions (campaign assault artifacts).
- **[context]** Optional provider-aligned tokenizer adapter behind `TokenCounter`.
- **[context]** Ensure `LoadState` + `RefreshBudget` always paired on session rehydrate.
- **[diff]** `DiffOptions{ContextLines, DisableCache, ...}` with zero-value defaults
- **[diff]** Word-level spans as codeNERD types (stop leaking `diffmatchpatch.Diff` in public API)
- **[diff]** Document or deprecate unused `LineHeader` production gap
- **[diff]** Align `CreateDiffFromStrings` with view-local engine (avoid dual-cache surprise)
- **[features]** Improve `Summary()` to print resolved booleans (dereference `*bool` or log `Is*` snapshot) so Boot logs are human-readable.
- **[features]** Optional CLI: `nerd features` or status subsection listing resolved flags (env vs active vs default source).
- **[features]** Optional chat slash `/features` mirroring Summary.
- **[init]** Improve framework detection (populate `ProjectProfile.Framework` from deps).
- **[init]** Monorepo multi-root profiles (beyond 2-level globs).
- **[init]** Hermetic tests for strategic knowledge JSON parsing with fake LLM.
- **[logging]** Make `CloseAll` close audit + LLM I/O (or document and wire CLI/chat to call all three)
- **[logging]** Resolve `sync.Once` + `--workspace` first-init race (boot order or rebind design)
- **[logging]** Add enabled-path tests for `trace_llm_io` writing expected markers
- **[mcp]** Feed usage stats into selection (success rate / latency boost or penalty)
- **[mcp]** Expose Info log when path is mangle vs fallback
- **[mcp]** Revisit skeleton counter naming vs policy skeleton tools
- **[mcp]** Optional re-analyze invalidation policy (schema hash change)
- **[northstar]** Atomize `buildAlignmentSystemPrompt` / user prompt under `internal/prompt/atoms/northstar/`.
- **[northstar]** Use or remove `GuardianConfig.AlignmentModel`.
- **[northstar]** Implement or remove `ingested_docs` + embedding relevance path.
- **[northstar]** Integration test: boot with vision → `northstar_defined` query true.
- **[northstar]** Chat adapter unit tests for `northstarHandlerAdapter`.
- **[persist]** Optional content sniff when suffix missing (gzip `1f 8b`, zstd magic)
- **[persist]** Cross-link comments: `core.baseTermToValue` vs `factsnap.baseTermToValue` NameType divergence
- **[persist]** Explicit tests for empty slice, bool, float multi-hop
- **[persist]** Consider shared conversion helper under `internal/types` if drift becomes painful
- **[regression]** Unit: missing file, bad YAML, timeout, empty command, workdir, multi-task success.
- **[regression]** Document runtime dependency on `powershell` / `bash`.
- **[regression]** Consider `bash --noprofile --norc` (or equivalent) for more deterministic Unix runs — **behavior change**, needs decision.
- **[retrieval]** Shared worker pool across keywords (avoid P×P goroutines).
- **[retrieval]** Invalidate cache on workspace file writes / session hooks.
- **[retrieval]** Either implement real `rg` backend behind interface **or** delete/rename `parseRipgrepOutput` + update comments/tests (`RealRg` → `NativeScan`).
- **[retrieval]** Structured metrics (latency, cache hit rate, files walked).
- **[retrieval]** Use or remove unused `SparseRetriever.mu`.
- **[retrieval]** Expand `filePathPattern` extensions as needed (`.tsx`, `.vue`, `.kt`, …).
- **[sqlpragmas]** Optional modernc.org/sqlite integration test (build tag) to catch reject sets early.
- **[sqlpragmas]** Document multi-conn pool guidance next to major store openers (or shared helper).
- **[sqlpragmas]** Consider Debug log including profile name on failure only.
- **[tools]** Rewrite `codedom/doc.go` to match registered tools.
- **[tools]** Decide CategoryReview / CategoryAttack: implement tools or stop mapping intents to empty categories.
- **[tools]** Prefer `logging.Tools*` for file/shell completions instead of VirtualStore channel (optional consistency).
- **[tools]** Consider registering tools only once into Global from VS pointer to eliminate dual-map drift risk.
- **[transparency]** Mutex (or single-owner docs + race tests) for `SafetyReporter`.
- **[transparency]** Stress test: multi-goroutine Emit + Subscribe drain under `-race`.
- **[transparency]** Expand `explainRule` map from real policy rule names used in `.mg` corpus.
- **[transparency]** Structured error types at VirtualStore boundaries to reduce ClassifyError ambiguity.
- **[types]** Consider nested sub-structs if `SessionContext` gains more sections (keep field groups navigable)
- **[types]** Optional test helper: `MockKernel` implementing `Kernel` + `KernelTransactor` for shared unit tests (only if it does not create cycles — may belong in `internal/testing`)
- **[types]** Document VirtualStore expansion policy in code comment when next method is added
- **[usage]** Either implement bounded `Events` ring **or** document reserved + stop implying raw event log.
- **[usage]** Cost estimation: static price table keyed by model → fill `TokenCounts.Cost`.
- **[usage]** UI: render `BySession`; optional cost column (`cmd/nerd/ui/usage_page.go` TODOs align).
- **[usage]** Log Load/Save failures through `internal/logging`.
- **[world]** gopls (or generic LSP client) under `lsp.Manager` as sketched in `lsp/README.md`.
- **[world]** Narrow holographic kernel dependency from `*core.RealKernel` to a small query interface.
- **[world]** Optional JIT prompt atoms for stable holographic sections.
- **[world]** Structured observability: cache hit rate metrics for FileCache (not only DataFlowCache).
- **[world]** Ensure incremental path also refreshes `project_language` / `entry_point` when majority shifts.

### P3 (71)

- **[autopoiesis]** Refresh package `internal/autopoiesis/README.md` date/architecture version to match 2026 corpus.
- **[autopoiesis]** Remove or redirect legacy architecture filenames if still present beside this corpus.
- **[autopoiesis]** Reduce dual templates vs JIT prompt residual prose over time.
- **[browser]** Complete BPAR-5 contract audit, repo trace, Docker correlation, and final live parity gate
- **[browser]** Fact GC / epoch for long event streams
- **[browser]** Header ingestion default policy for research vs operator modes
- **[browser]** CI job: integration tag with headless Chrome
- **[build]** `BuildWarn` when GOCACHE cannot be derived.
- **[build]** Optional keys-only `SummarizeEnv` for debug.
- **[build]** Integration test: construct env against real workspace with `sqlite_headers`, run `go env`.
- **[build]** Avoid logging secret-prone values at debug for config env (keys only, or redact).
- **[campaign]** Journal verify/replay operator command
- **[campaign]** Chaos test: kill during snapshot rename
- **[campaign]** Assault summary export (aggregate results → single report file)
- **[context]** Wire audit: confirm prompt JIT actually calls `GetActivationScores` each turn when expected.
- **[context]** Surface feedback store stats in glass-box / transparency UI.
- **[context]** Document operator workflow for inspecting helpful vs noise predicates.
- **[diff]** `Engine.Stats()` counters (hits, misses, binary, computes)
- **[diff]** Test: shallow-cache mutation fail-closed after deep-copy fix
- **[diff]** Test: ClearCache concurrent with ComputeDiff under `-race`
- **[diff]** Test: assert DiffTimeout behavior on synthetic pathological input
- **[diff]** Test: trailing-newline-only change representation precision
- **[diff]** Benchmark CI smoke (optional)
- **[features]** Env prefix migration plan (`NERD_*` → `CODENERD_*` dual-read then deprecate).
- **[features]** Document JSON schema snippet for `features` block in user-facing config docs (outside this package if preferred).
- **[init]** Split `Initialize` into phase methods without behavior change.
- **[init]** Relocate session persistence types to `internal/session` (breaking API care).
- **[init]** Remove accidental `debug_program_ERROR.mg` from package tree / ignore dumps.
- **[init]** Complete Ouroboros tool generation call site or delete dead `determineRequiredTools` UI noise.
- **[logging]** ContextLogger / RequestLogger respect `json_format` via structured entries
- **[logging]** Operator playbook snippet in root AGENTS or help command pointing at this corpus
- **[logging]** Optional CLI offline: audit JSONL → `.mg` facts file
- **[logging]** Size/time-based log rotation beyond daily name
- **[mcp]** Fake MCP server tests for HTTP list/call
- **[mcp]** `-race` CI for manager+store
- **[mcp]** Document/configure stdio sandbox expectations
- **[mcp]** Secret redaction strategy for tool outputs in logs
- **[northstar]** Metrics: checks total, blocked rate, mean score.
- **[northstar]** Structured log fields (subject, trigger, score).
- **[northstar]** Validate threshold ordering on `NewGuardian`.
- **[northstar]** Singleton Guardian per session (avoid dual DB handles for `/alignment`).
- **[persist]** Logging (size, duration, codec) on write/read
- **[persist]** Optional integrity hash sidecar
- **[persist]** Package-level doc file or root re-export if more subpackages appear
- **[persist]** Streaming writer only if multi-million fact dumps appear in practice
- **[regression]** VirtualStore action `run_regression_battery`.
- **[regression]** Mangle `Decl` + `permitted(...)` rules.
- **[regression]** Never expose unrestricted shell battery run without policy.
- **[retrieval]** Cross-package test: seed fact arity vs `schemas_knowledge.mg`.
- **[retrieval]** SIMD-tagged CI job optional.
- **[retrieval]** Keep this corpus updated when wire lands (date stamp).
- **[sqlpragmas]** `EnableForeignKeys(db *sql.DB)` helper for schemas ready to enforce FKs.
- **[sqlpragmas]** Idempotency tests for BulkBuild / Query / ReadOnly.
- **[sqlpragmas]** Named Go constants for cache/mmap sizes (readability) without changing values.
- **[tools]** Optional disk-backed research cache under `.nerd/`.
- **[tools]** Assert `tool_execution` facts from Registry.Execute for learning.
- **[tools]** Improve codedom EndLine via simple brace/indent block tracking.
- **[tools]** Metrics counters for tool success/duration.
- **[transparency]** Optional JSON/NDJSON event sink for headless campaign runs.
- **[transparency]** OTel bridge (optional) mapping categories → span events.
- **[transparency]** Per-turn Glass Box export attached to campaign assault artifacts.
- **[transparency]** Machine-checkable invariant tests that ToolEvent still flows when Glass Box disabled.
- **[types]** Optional package-level godoc examples for `ToAtom` and `NewKernelTx`
- **[types]** When dual Kernel APIs collapse, delete obsolete aliases after one release cycle
- **[usage]** Aggregate by shard **name** (or composite name+type) if operators need specialist-level spend.
- **[usage]** Optional CLI: `nerd usage` / dump JSON to stdout for scripts.
- **[usage]** Cap or prune `BySession` for long-lived workspaces.
- **[usage]** Reject negative token inputs in `Track`.
- **[world]** Remove or relocate `debug_program_ERROR.mg` artifact from package tree if accidental.
- **[world]** Align `symbol_graph` arg typing (string vs `/name` atoms) with Decl bounds.
- **[world]** Document operator runbook in CLI help for `nerd scan` / chat rescan.

### P4 (27)

- **[build]** Update [08-WIRING-AND-INTEGRATION.md](08-WIRING-AND-INTEGRATION.md) when new importers appear.
- **[build]** Refresh scores in [00-ALIGNMENT-VISION-REVIEW.md](00-ALIGNMENT-VISION-REVIEW.md) after adoption work.
- **[build]** Keep IMPLEMENTED_SPEC scale counts accurate after edits.
- **[campaign]** Closed enum or constants file for OrchestratorEvent.Type strings
- **[campaign]** Optional metrics hooks (task duration histograms) without coupling to one backend
- **[context]** Align `internal/context/README.md` defaults (200k, current date, file list including feedback_store).
- **[context]** Remove or relocate crash-dump `debug_program_ERROR.mg` from package tree if not intentional.
- **[features]** Table-driven precedence matrix for all eight boolean accessors.
- **[features]** Summary format test once Summary is fixed.
- **[features]** Optional `-race` concurrent SetActive stress.
- **[logging]** Northstar convenience wrappers (Info/Debug/Warn/Error)
- **[logging]** Optional `runtime.Caller` population of StructuredLogEntry file/line
- **[logging]** Expand call-site audit for `SafetyCheck` next to real `permitted` checks
- **[mcp]** MCP resources/prompts beyond tools capability flags
- **[mcp]** Auth headers / token injection for HTTP transports
- **[mcp]** Metrics exporter for call latency/error rates
- **[regression]** `RunOptions{FailFast bool}` default true.
- **[regression]** Optional `expect_contains` / `expect_exit` on `Task`.
- **[regression]** Honor or validate `Version`.
- **[regression]** Structured logging category `regression`.
- **[regression]** Result JSON tags for easy serialization.
- **[sqlpragmas]** Config/env overrides for host class (laptop vs workstation).
- **[sqlpragmas]** `database/sql` connector hook helper for per-connection apply.
- **[sqlpragmas]** Metrics counter for pragma failures (behind observability flag).
- **[usage]** Unify chat session tracker with Cortex tracker (single owner per process).
- **[usage]** Consider typed context keys for shard metadata (breaking; needs coordinated shards change).
- **[usage]** Integration test: boot → NewContext → mock client Track → Save → reload.

### P5 (1)

- **[features]** When ShardFactRouter auto-wiring is production-ready, flip FullyEnabled PerShardFacts (or document continued opt-in) and expand integration tests.

### P? (5)

- **[campaign]** Re-verify line counts after large refactors
- **[campaign]** Cross-link from CLI corpus assault section when CLI docs change
- **[persist]** Refresh reverse-deps after first real importer lands
- **[usage]** When Track producers expand, update [08-WIRING-AND-INTEGRATION.md](08-WIRING-AND-INTEGRATION.md) producer table.
- **[usage]** Keep IMPLEMENTED_SPEC status table in sync after code changes.

