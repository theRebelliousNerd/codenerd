# config wiring — and what is NOT built

> **Status, 2026-09-25 (lane B wave 2).** Every entry below is resolved; the
> sections are kept as written at `b63e848` and this table is current.
>
> | Entry | Resolution | Evidence |
> |---|---|---|
> | §2 `MangleConfig` / `DefaultDerivedFactLimit` | deleted `adc30df` | zero references; its one live meaning is `core_limits.max_derived_facts_limit` (5,000,000). Wiring it would have added a second, contradicting gas knob (500,000) |
> | §2 `UIConfig` / `DefaultUIConfig` | closed `adc30df` | `ui.split_pane_ratio` is a UserConfig section (`GetUIConfig`, `Check`, in `DefaultUserConfig`); the chat builds its split pane from it (`newSplitPane` → `ui.NewSplitPaneViewWithRatio`). `LogicPaneWidth` removed: the split pane never supported it and no file could carry it. `TestSections_UIAndRetrievalLoadFromConfigJSON`, `TestNewSplitPane_UsesTheConfiguredRatio` |
> | §2 Integrations methods | traced | `ToMCPServerConfigs` has its caller (MCP boot); `GetServer` only feeds `IsServerEnabled`, which nothing calls -- redundant with the enabled filter `ToMCPServerConfigs` already applies. Declined: no consumer to wire |
> | §2 `GetActiveProvider`, `APIKeyForProvider`, `SetAPIKeyForProvider` | traced: wired | 10, 2 and 1 callers outside the package |
> | §2 timeout profiles, `baseProfile`, `applyDuration` | already wired (stale) | `llm_timeouts_config.go` resolves `llm_timeouts.profile` through `baseProfile` → `FastLLMTimeouts`/`AggressiveLLMTimeouts` |
> | §2 `IsCategoryEnabled`, `CLIModelLabel`, `AutoDetectContext7APIKey` | traced: wired | 2, 10 and 2 callers outside the package |
> | §2 `GetClaudeCLICommand` | stale | no longer exists |
> | §4 `internal/config/agents.md` tool-budget bullet | closed `adc30df` | the bullet describes `rejectRemovedKeys` now; `TestAgentsGuide_TeachesNoRemovedKey` fails if the guide names a removed key (it fails on the old text) |
> | §5 wall-clock runs, per-shard kernels | decided | removed deliberately (2026-09-18/19); §3 is the record |
> | §5 image providers beyond gemini, CLI model defaulting | decided | maintainer design choices (`GetImageLLMConfig` is Gemini-only by construction; `DefaultClaudeCLIConfig` names no model so the CLI's own default stands) |
> | New: `retrieval` section | added `216b818` | `retrieval.brief_min_relevance`, asserted as `config_param(/retrieval_brief_min_relevance, N)` for `retrieval_brief_file` |

Question answered here: what is wired and reachable, what exists but has no
caller traced, and what the design assumes that the code does not do?

## 1. Wired inside this package

Every row is a caller → callee edge verified in the code:

| Caller | Callee |
|---|---|
| `LoadUserConfig` (`user_config.go:525-593`) | `rejectRemovedKeys` (`removed_keys.go:69-98`), `decodeStrictJSON` (`persistence.go:17-33`), `validateExplicitCoreLimits` (`user_config.go:621-644`), `ValidateCoreLimits` (`limits.go:85-99`), `features.SetActive` (:561), `SetLLMTimeouts` (:584) |
| `validateExplicitCoreLimits` (:621-644) | `coreLimitsKeysPresent` (:600-612), `DefaultCoreLimits` (`limits.go:102-110`) |
| `GetWorkerLLMConfig` (:810-815), `GetPlannerLLMConfig` (:821-833) | `resolveSecondarySlot` (:837-852) → `GetOllamaLLMConfig` (:782-807) |
| `Save` (:647-658) | `writePrivateFileAtomically` (`persistence.go:36-38`) |
| `ApplyLoggingConfig` (:1462-1468) | `ToLoggingConfig` (`logging.go:22-35`) |
| `GetEffectiveAPISchedulerPolicy` (:1176-1226) | `EffectiveAPISchedulerPolicy` (`limits.go:67-74`), `subscriptionEngine` (:1165-1172) |

External readers attested by the code's own comments (not re-traced here):
the `GetLLMTimeouts` call sites and the leaf packages behind
`features.SetActive` (`user_config.go:557-577`); autopoiesis passing nil into
`GetBuildConfig` (:429-431); the autopoiesis orchestrator reading
`AllowToolExec` (`tool_generation.go:21-22`); the config wizard echoing shard
keys (`shard.go:27-31`); `nerd auth` persisting `DefaultUserConfig`
(`tool_generation.go:43-46`).

## 2. Exists, but no caller traced beyond this package

- **`MangleConfig` / `DefaultDerivedFactLimit`** (`mangle.go:4-13`). No
  `GetMangleConfig` accessor exists on `UserConfig`, unlike every wired
  subsystem — so nothing in this package resolves a mangle section onto the
  config root.
- **`UIConfig` / `DefaultUIConfig`** (`ux.go:4-19`, `ViewMode` :20-23).
  Same gap: no `GetUIConfig` accessor.
- **Integrations methods** (`integrations.go:47-112`: `ToMCPServerConfigs`,
  `GetServer`, `IsServerEnabled`) — defined and unit-consistent, callers
  outside the package untraced.
- **`GetActiveProvider`, `APIKeyForProvider`, `SetAPIKeyForProvider`**
  (`user_config.go:860-928, 940-1008`) — the selection and key machinery is
  fully implemented here; which clients call it was not re-traced.
- **Timeout profiles** `FastLLMTimeouts` / `AggressiveLLMTimeouts`
  (`llm_timeouts.go:80-110`), `baseProfile` / `applyDuration`
  (`llm_timeouts_config.go:64-94`), `IsCategoryEnabled`
  (`logging.go:40-52`), `CLIModelLabel` / `GetClaudeCLICommand`
  (`llm.go:238-243, 273-275`), `AutoDetectContext7APIKey`
  (`user_config.go:1556-1569`).

## 3. Refused at the door: removed keys

`rejectRemovedKeys` (`removed_keys.go:69-98`) fails load on these, with the
reason inline (`issueForKey` :101-112). Table order follows the four maps
(:23-61):

| Section | Rejected keys | Why |
|---|---|---|
| `features` (:23-26) | `diff_eval` | Removed 2026-09-18 (S23 differential path) |
| `core_limits` (:28-35) | `max_tool_calls`, `max_tool_iterations`, `adaptive_tool_budget`, `tool_iteration_extension_size`, `max_tool_iteration_extensions`, `tool_loop_repeat_threshold` | Tool loop stopped being count-bounded, 2026-09-18; only `max_concurrent_shards` still gates it |
| `core_limits` (:28-35) | `max_session_duration_min` | Sessions no longer wall-clock-bounded, 2026-09-19 |
| `llm_timeouts` (:36-44) | `shard_execution_timeout`, `ooda_loop_timeout`, `campaign_phase_timeout`, `document_processing_timeout`, `ouroboros_timeout` | Runs no longer wall-clock-bounded, 2026-09-19 |
| `shard_profiles.*`, `default_shard` (:46-54) | `max_execution_time_sec` | Runs no longer wall-clock-bounded, 2026-09-19 |

## 4. Stale paper the code itself contradicts

- `internal/config/agents.md:14-17` still tells maintainers to tune a
  tool-budget extension/repeat scheme whose keys were removed
  (`removed_keys.go:28-35`). The note is wrong; the code is right.
- `AllowToolExec` (`tool_generation.go:18-26`): the autopoiesis safety design
  documented an `os/exec` grant that was unreachable until this field was
  added — the docs described a knob that did not exist.
- Shard sampling fields were collected and persisted for months while no
  client read them; they went live 2026-09-11 (`shard.go:15-19`). The
  per-shard ceilings deleted 2026-09-10/11 were echoed by the config wizard
  while being read by nothing (`shard.go:23-31).
- The `windows/amd64` tool-generation default persisted itself into
  `config.json` via `nerd auth` and silently cross-compiled every generated
  tool; defaulting to the host fixed it (`tool_generation.go:30-49`).
- `GetBuildConfig` once panicked on a nil receiver that autopoiesis passes
  routinely; the guard is at `user_config.go:427-444`.

## 5. Assumed by the design, not done by the code

- Wall-clock-bounded runs: every removed timeout assumed a run could be
  bounded by time; no such bound exists anymore (§3).
- Per-shard kernels and per-shard context/token/fact ceilings: the design
  assumed a substrate per persona; there is none (`shard.go:23-31`).
- Image providers beyond gemini: `GetImageLLMConfig`
  (`user_config.go:760-779`) defaults the provider to gemini with no
  multi-provider resolution.
- CLI model defaulting: `GetClaudeCLIConfig` (`user_config.go:1049-1060`)
  keeps the CLI's own configured model when the file is silent — there is no
  codeNERD-side default to fall back to.

---
*Verified 2026-09-20 against `b63e848`. "Untraced" means no caller was found
inside `internal/config` and callers outside it were not re-traced for this
rewrite — not that none exist.*
