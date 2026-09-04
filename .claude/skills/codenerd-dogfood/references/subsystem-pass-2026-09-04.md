# Subsystem first pass — 2026-09-04

Five thorough read-only sweeps (kernel/policy/core; the turn loop; long-horizon
subsystems; effect and memory subsystems; the control surface), each verified
at the cited lines and each running its packages' targeted tests. This is the
punch list; items are marked as they land. Ledger context:
`component-ledger.md` (2026-09-03 section) and `composition-map-2026-09-03.md`.

## Landed the same day

| # | Finding | Commit |
|---|---|---|
| 1 | Checkpoint verdict read from the kernel (`checkpoint_verdict/4` declared); previously parsed the stripped surface text and failed every checkpoint | 16e830d8 |
| 2 | New-source hollow rule scoped to this turn; existence check restored in `recordGoFileCreations`; `build_state` asserted from build verification | 16e830d8 |
| 3 | Production VirtualStore adapters implement the interactive gate (Dreamer + validators now run outside the TUI) | 16e830d8 |
| 4 | ConstitutionGateShard owns only executive-prefixed `pending_action`; executor uses `exec-`; `permitted_action` gated on a running router and pruned | 16e830d8 |
| 5 | Meta reasoning cache keyed by conversation and bounded; `SetStrategicContext` locked | 16e830d8 |
| 6 | Six granted tools had no `safe_action` (web_search, web_fetch, context7_fetch, get_impacted_tests, run_impacted_tests, apply_edits); `apply_edits` mapped in the gate | 72990135 |
| 7 | Decomposer retypes file tasks against the filesystem | d21bc4ce |
| 8 | Campaign phase 0–1 output (turn evidence, safe-action projection, one project-shape source) | 755c5732 |
| 9 | `nerd sessions load` printed a flag that does not exist | 5479bdcf |
| 10 | Dreamer projected `/critical_path_hit` for *edits* under `internal/core`, `internal/mangle`, `cmd/nerd`, `.nerd`, `.git`, so `panic_state(_, "critical_path_missing")` blocked every write there. Surfaced the moment item 3 put the Dreamer on the executor path (run 23: 38 tool calls, every edit refused, hollow-success guard caught it). The schema defines critical paths as "never removed recursively"; modification now hits only for `.git`, and the prefix match is segment-aware (`internal/corex` no longer matches) | (this batch) |
| 11 | `TestAddFactsContext_HonorsDeadline` waited a fixed 2 ms for a 1 ns timer; on Windows the timer fires at ~15 ms resolution. Now waits on `ctx.Done()` | (this batch) |
| 12 | Registry↔policy parity test (`internal/tools/catalog_policy_parity_test.go`, built by codeNERD, run 22). It immediately found four more granted-but-ungated tools: `research_cache_get/set/stats/clear`. get/set/stats gained `safe_action`; `clear` stays denied on purpose and is the test's one listed exception | (this batch) |
| 13 | Campaign file tasks: target from `Artifacts[0]`, else the first exact write-set entry (relativized), else the description; empty target is an error before any shard call; a directory or missing file never counts as written; pathless mutating tasks retyped to `/research` at plan time (codeNERD, run 24) | (this batch) |

## In flight (briefs written)

- `-i` mode (`cmd/nerd`): run 23 was refused every write by item 10; re-dispatch after the rebuild.
- Campaign resume: accept failed/blocked campaigns, reset tasks with a cap, resume boots the same system as start (`cmd/nerd`, `internal/campaign`).
- Factory wiring: `SetSessionID` + `SetOuroborosRegistry` (`internal/system`, `internal/session`).
- ConfigFactory: `Validate()` on every generated config, inert `ToolLoop` and unread fields removed (`internal/prompt`, `internal/jit/config`).
- TUI state: value-receiver writes, continuation context parented on shutdown (`cmd/nerd/chat`).
- Retrieval semantic tier wired through `nerd retrieve` and the TUI seed (`internal/retrieval`).

## Open, ranked (one concern each)

**Turn loop**
- `SetSessionID` never called in the factory: every non-`nerd chat` turn persists under `"default"` (`internal/system/factory.go` ~1610).
- `SetOuroborosRegistry` wired only in the TUI; `executor.go:513` panics on nil (`factory.go` ~1616).
- `ConfigFactory.Generate` never calls `Validate()`; its `ToolLoop` 5/50 budget is inert and contradicts `DefaultExecutorConfig` (`internal/prompt/config_factory.go:148-157`).
- Memory is write-only on the executor path: `HydrateLearnings`/`HydrateSessionContext` chat-only; `atomsJSON` hardcoded empty; `memory_operation/3` has no reader; `context_feedback` and `intent_classification` from the Piggyback packet are log-only. `turn_cost/6` still unimplemented.
- `internal/context` (compressor, activation) not wired outside the TUI.
- `next_action/1` never consulted by the executor (deliberate; documented).

**Kernel / shards**
- `tactile_router` never spawned in production; the second pipeline is dormant by construction — decide: spawn it or delete the emission.
- `ExecutivePolicyShard` retracts `user_intent`/`pending_action` on every start (destructive on mid-session restart).
- ~~`diff_eval` incremental eval dead; evals cost 14–24 s~~ — wrong for production. `.nerd/config.json` sets `diff_eval: true`; the 04:23 kernel log shows 105 evaluations via the differential path, 0 full, ~60–70 ms wall each (912 strata). The 14–24 s figure is the test-time full path (compile-time default off). Only the stale comment in `kernel_eval.go:26-38` was real; corrected. `per_shard_facts` is still off by config (deliberate, pinned by test), so the 7-shard manifest is inert data.
- Shadowed `tactile_router`/`campaign_runner` factories in `registration.go:302-319`.

**Campaign / autopoiesis**
- `nerd campaign start` and `resume` each hand-assemble their own kernel, shard manager and VirtualStore (`cmd_campaign.go` ~242 and ~830) instead of booting the Cortex like every other command — a fourth and fifth boot path. This is *why* `ToolPregenerator` is nil (~650 lines dead), the intelligence gatherer gets no tool generator or MCP store, and every factory fix has to be mirrored by hand. Brief written: `brief_campaign_boot.txt`.
- `TaskVerifier` fails open: a verifier error is recorded as success at confidence 0.3 and a nil client as 0.5 (`internal/verification/verifier.go` ~224-232, ~283). Brief written: `brief_verifier_failopen.txt`. The TUI compares the max-retries error by string (`process.go:515`).
- `ExecutivePolicyShard` retract of `user_intent`/`pending_action` happens in Start (`executive.go:427-434`) and is correct at boot; it is destructive only if the shard restarts mid-session. Leave until a restart path exists.
- MCP `JITToolCompiler`/`CompileToolsForShard` orphan is inert here: `.nerd/config.json` has `integrations.servers: {}`. Low priority.
- `next_action(/campaign_*)` derived, never consumed (orchestrator dispatches through Go).
- Verifier (`VerifyWithRetry`) reachable only from the TUI.
- `prompt_evolved`/`thunderdome_result` have no producer; `correction_pattern` has no consumer.
- Windows hot-reload holds the tool binary (`TestOuroborosLoop_HotReload_LockedBinary`).

**Effects / memory**
- Retrieval semantic tier (`NewEmbeddingSemanticSearcher`) has no production caller.
- No workspace rescan on `nerd run/fix/create/review`; campaign builds a second default scanner.
- Embedding degradation announced once at boot, then invisible.
- MCP `JITToolCompiler`/`CompileToolsForShard` orphan: MCP tools are callable but never described to the model.
- `review_findings` write-only; `task_verifications` readers uncalled.
- `Registry.SetAllowlist` never set (by design; direct `tools.Global().Execute` is gated only by the write guard).

**Control surface**
- `nerd fix X` ≠ TUI "fix X" (no perception/routing/verification); `nerd chat` ≠ TUI (`SystemComponents.SessionExecutor` never read by the Model); `nerd run` is a third loop.
- `--dry-run`/`--dump-kernel`/`--trace-api` registered, unread; `--verbose` bound twice with different meanings; `--disable-system-shard` only on `run` but read everywhere.
- Continuation steps use `context.Background()` (Ctrl+X cannot cancel); `launchClarifyAnswers` and `awaitingKnowledge` written on value receivers.
- `main.go` loads `config.yaml`, not `config.json`, so `GetExecutionTimeout` never applies.
- `performSystemBootLegacy` (~1050 lines) dead.
- Config inert: `guidance.*`, onboarding tour fields, `logging.file`; six `llm_timeouts` fields Z.AI-only (Gemini/Anthropic/OpenAI/compat hardcode 10 min).
- TransparencyManager TUI-only; `sqlpragmas` host-class and metrics, `internal/build` helpers, `ux.GetDisclosureLevel`, `transparency.QuickExplain` unused.
