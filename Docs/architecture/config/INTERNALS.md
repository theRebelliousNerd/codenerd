# config internals — how it works

Question answered here: how does `.nerd/config.json` become values other
packages use? (What is wired vs dormant lives in WIRING-AND-NOT-BUILT.md.)

## 1. The load pipeline

`LoadUserConfig` (`internal/config/user_config.go:525-593`) runs six stages
in order:

1. Read the file; missing file means empty config (:528-534).
2. `rejectRemovedKeys` (:541-543) — renamed/retired keys fail here with the
   reason for the removal, because the strict decoder in the next step would
   otherwise report them as generic unknown fields. The comment at :536-540
   states exactly that.
3. `decodeStrictJSON` (`persistence.go:17-33`) — unknown fields are an error.
4. `validateExplicitCoreLimits` (:549-551) — see §2.
5. `ValidateCoreLimits` on the resolved limits (:552-555).
6. Two process-wide singleton installs: `features.SetActive(cfg.Features)`
   (:561) so leaf packages that cannot import this package still see the
   flags (:557-560), and `SetLLMTimeouts` (:584) for the timeout profile the
   `GetLLMTimeouts` call sites read (:574-577). Legacy `NERD_*` overrides are
   logged as deprecations (:570-572), and a malformed timeout profile is a
   hard error, not a silent default (:578-583).

Saving is `Save` (`user_config.go:647-658`): indented JSON plus newline via
`writePrivateFileAtomically` (`persistence.go:36-38`).

## 2. Explicit zero vs omitted key

`GetCoreLimits` fills unset fields with defaults, so a resolved struct cannot
tell "user wrote 0" from "key omitted". `coreLimitsKeysPresent`
(`user_config.go:600-612`) recovers that distinction by re-reading the raw
JSON envelope, and `validateExplicitCoreLimits` (:621-644) merges defaults
only for absent keys before running `ValidateCoreLimits`
(`limits.go:85-99`, defaults at `limits.go:102-110`). Only values the user
actually wrote can fail.

## 3. Browser options use pointer semantics

`BrowserAutomationConfig` (`user_config.go:1314-1337`) keeps `MultiTabDefault`
(:1321) and `EvidenceEnabled` (:1332) as pointers so "unset" survives merging;
`boolConfigPointer` (:1356) is the helper. `GetBrowserConfig` (:1359-1401)
normalizes the whole section: invalid tab/browser limits fall back to
defaults, and the evidence-file count/byte ceilings clamp to their maxima.
Reachability of this section outside the package is untraced — see
WIRING-AND-NOT-BUILT.md §2.

## 4. Secondary LLM slots resolve to nil, never to half-config

Worker, planner, and Ollama slots share `resolveSecondarySlot`
(`user_config.go:837-852`): a slot with no provider resolves to nil, and the
Ollama slot inherits endpoint and model from the worker slot when its own are
empty (`GetOllamaLLMConfig` :782-807). A planner that duplicates the worker
resolves to nil rather than a redundant client (`GetPlannerLLMConfig`
:821-833). API keys resolve through `APIKeyForProvider` (:860-888), which
treats the literal `"ollama"` as a local-mode sentinel, not a secret
(:858-859, :883-884). Image slots are separate: `GetImageLLMConfig`
(:760-779) normalizes model aliases and defaults the provider to gemini, but
sets no model default of its own.

## 5. Engine selection: config is the boss, except where safety overrules

`GetActiveProvider` (`user_config.go:940-1008`) resolves provider in priority
order: explicit override, per-slot provider, engine default, global default
(:930-939). `SetEngine` (:1033-1045) accepts exactly `claude`, `codex`, and
`gemini`. `HasExplicitLLMSelection` (:1021-1030) reports whether the user
chose anything at all. One exception to config supremacy: `GetCodexCLIConfig`
(:1064-1103) forces the sandbox controls regardless of what the file says,
because the Codex CLI is a completion backend, never an effect executor;
likewise `GetClaudeCLIConfig` (:1049-1060) never invents a model default.

## 6. Tool-generation targets default to the host

`DefaultToolGenerationConfig` (`tool_generation.go:50-55`) returns
`runtime.GOOS`/`runtime.GOARCH`, because Ouroboros executes the binary it
compiles and a foreign default would die with "exec format error"
(:30-49). `AllowToolExec` (:27) is the opt-in for `os/exec` in generated
tools; its comment (:10-27) records that the grant was documented but
unreachable until this field existed.

## 7. Per-slot input cap (`max_input_tokens`) — as built 2026-10-27

`SecondaryLLMConfig.MaxInputTokens` (`user_config.go:832`, worker/planner):
0 is no cap; `checkSlotInputCap` (`check.go:487`) refuses negative or
`< MinPromptTokens` (`working.go:90`, also the broker's prompt floor) and
warns when the slot names no provider. Units are the session's ruler,
`prompt.EstimateRequestTokens` (`internal/prompt/request_tokens.go:19`).
Layer 1: the turn's resolved client is threaded into `beginWorkingLoop`, whose
`inputCap` is `broker.InputWindow` of it (`internal/session/working_context.go:244`);
`inputBudgetFor` (:661) is min(TokenBudget-or-default, cap) for every working
request, the compile budget (`executor.go:1379`), the holographic render and a
spawned subagent's compile (`spawner.go:685`); `inputBudgetExceeded` (:683)
names the slot cap when it binds. Layer 2: Anthropic and OpenAI-compatible
clients report the cap (`types.InputWindowReporter`) and refuse a larger
request before any send (`internal/perception/input_window.go:26`, same
measure, so an executor-fit request is never refused). A slot whose client
cannot enforce it is refused at construction (`client_factory.go:843`). The
measure, like the executor's, does not count replayed thinking or server-tool
blocks.

## 8. Reasoning effort — as built 2026-10-27

Every request carries one (`reasoning_effort.go`): main `reasoning_effort`
(default medium), `worker`/`planner.reasoning_effort` (medium/high, filled by
`resolveSecondarySlot`), `classification_reasoning_effort` (default the
classifying provider's lowest: Meta minimal, Anthropic low), and
`shard_profiles.*.reasoning_effort` (rides the context; unset inherits the
serving slot, and a profile route inherits the routing slot via
`SecondaryLLMConfig.RouteTo`, one router per slot in `internal/system`).
`Check` validates each against its provider and model
(`ValidateReasoningEffort`, `check.go:126`, profiles `:445`). Clients send it
where the model accepts one (`config.AnthropicSupportsEffort`; Meta always);
the capability-hint mappings are gone. `nerd config full` writes the resolved
values (`withResolvedDefaults`, `reasoning_effort.go:195`).

## 9. Anthropic server web tools — as built 2026-10-27

`anthropic.web_search`/`web_fetch` (off), `web_search_max_uses`/
`web_fetch_max_uses` (1-20, default 5 while on), per slot through
`worker|planner.anthropic`. `AnthropicWebToolVersions` (`anthropic.go:171`)
picks the dynamic (20260209) or basic versions by model. The client
(`internal/perception/anthropic_web_tools.go`) declares them on non-streaming
requests, resumes `pause_turn` (bounded, :18), keeps server blocks verbatim
(`types.BlockServerTool`, never a tool call), and records result and citation
URLs for `GetLastGroundingSources`, the one grounding channel production reads
(scheduler, tracing and broker forward it to chat articulation, delegation and
campaign planning). The tools follow the config alone: no run-time grounding
control. Search counts are logged, not priced: the token meter has no per-use
charge. Boot-path proof: `internal/system/factory_slot_policy_boot_test.go`.

---
*Verified 2026-09-20 against `b63e848`; §7-9 against the 2026-10-27 working
tree. All line ranges are 1-indexed file ranges in `internal/config` unless a
path is given.*
