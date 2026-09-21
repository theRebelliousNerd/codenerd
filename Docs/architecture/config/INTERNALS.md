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

---
*Verified 2026-09-20 against `b63e848`. All line ranges are 1-indexed file
ranges in `internal/config`.*
