# cli wiring — and what is NOT built here

> **Status, 2026-09-25 (lane B wave 2).** Every entry below is resolved; the
> sections are kept as written on 2026-09-20 and this table is current.
>
> | Entry | Resolution | Evidence |
> |---|---|---|
> | `fix --acceptance` consumer | already wired | `validateFixArgs` and `runDirectAction` load the contract and put it on the context (`cmd/nerd/cmd_direct_actions.go`); `acceptance_args_test.go` |
> | `--yolo` and `--api-key` read outside `main.go` | closed `eac600f` (bare `nerd`) | the root RunE built `chat.Config` from `--disable-system-shard` alone, so `nerd --yolo` opened a chat that asked every question and `nerd --api-key K` dropped K. `chatLaunchConfig` carries both; `--yolo` is session autonomy (`yoloEnabled`, `syncYoloFact`, never persisted, ended by `/yolo off`); `--api-key` reaches `BootConfig.APIKey` (`sharedBootConfig`). `TestChatLaunchConfig_CarriesRootFlags`, `TestInitChat_YoloFlagIsSessionAutonomy`, `TestSharedBootConfig_CarriesLaunchFlags`. The one-shot verbs already read `--api-key` (`resolveAPIKey`) and have no clarification path for `--yolo` to silence |
> | `--yolo` "never overrides safety denials" | standing rule `c92cd25` | `TestYoloMode_OnlySilencesQuestions`: every corpus clause reading `yolo_mode` reads it negated and concludes a clarification, never a verdict; `TestYoloMode_CheckerCatchesAWideningRule` is its negative control |
> | `--timeout` enforcement | already wired | `operationContext` (`cmd/nerd/operation_context.go`, `operation_context_test.go`) |
> | `timeout`, `campaignRetryFailed`, `campaignResumeID` | traced: wired | read by the one-shot verbs through `operationContext` and by `campaign resume` |
> | `domCmd` / `embeddingCmd` naming | documented | defined in `dom_cmd.go` / `embedding_cmd.go`; `embedding reembed` is the store's dim-change procedure |
> | journal / assault / recurse / mcp select / test-context "no AddCommand" | already wired (stale) | each registers itself in its own `init()`: `campaignCmd.AddCommand(campaignJournalCmd, campaignReportCmd)`, `campaignCmd.AddCommand(campaignAssaultCmd)`, `campaignRecurseCmd` in `main.go`, `mcpCmd.AddCommand(mcpSelectCmd, mcpMetricsCmd)`, `rootCmd.AddCommand(testContextCmd)` |
> | Flight recorder, `SilenceUsage` | documented | contracts of `internal/observability` and of each command's error text; nothing to wire in `main.go` |
> | `nerd memory` printed every table's sum as "Vector (Embeddings)" | closed `2acad15` | `renderStoreStats` prints the vectors count there, every table by name and the store's gauges |

Verified 2026-09-20 against `main` (working tree up to date with
`origin/main`). Pin notes as in `README.md`. Every path below was
read this pass (`cmd/nerd/main.go` lines 1-400; `Use` lines and
package clauses via repository search); files marked unread were not.

## Wired and reachable from this package

- Bare `nerd` → `RunE` → workspace chdir → `chat.RunInteractiveChat`
  (`main.go:148-175`); the only subpackage import is `chat`
  (`main.go:56-71`). `--disable-system-shard` flows into
  `chat.Config{DisableSystemShards}` (`main.go:171-173,198`).
- Pre/post-run logging: zap build, `logging.Initialize` with warn-
  and-continue fallback, `logger.Sync()` + `logging.CloseAll()`
  (`main.go:109-147`).
- Pre-cobra boot: idempotent logging init, best-effort config load,
  startup metrics, flight recorder unless a campaign invocation
  (`main.go:353-400`, `isCampaignInvocation` at `main.go:343-351`).
- The full top-level tree plus the `browser` / `campaign` / `auth`
  subgroups, all attached in `init()` (`main.go:216-331`).

## Exists but is not wired here

- `fix --acceptance` is declared (`main.go:203`) with a contract
  description in its help text, but no acceptance handling appears in
  `main.go` — the consumer, if any, is in `cmd_direct_actions.go`,
  which this pass did not read.
- `yoloMode`, `apiKey`, `timeout`, `campaignRetryFailed`,
  `campaignResumeID` are declared (`main.go:73-95`); only
  `--workspace`, `--verbose`, and `--disable-system-shard` are
  consumed in `main.go` itself. The rest are read by code outside
  this file.
- `domCmd` / `embeddingCmd` are attached (`main.go:290-292`) but
  defined in `dom_cmd.go` / `embedding_cmd.go`, outside the
  `cmd_*.go` naming of every other command — their behaviour is not
  described anywhere in this corpus.
- `cmd_campaign_journal.go`, `cmd_campaign_assault.go`,
  `cmd_campaign_recurse.go`, `cmd_mcp_select.go`,
  `cmd_test_context.go` define `Use` strings (see `README.md`) with
  no `AddCommand` in `main.go:178-332`. If they are reachable, the
  wiring is in their own files, unread here.

## Assumed by this code, not done by it

- `--timeout` defaults to no limit (`main.go:187-188`); enforcement
  is pointed at `operation_context.go` by the header index
  (`main.go:50-51`), which was not read — no timeout logic exists in
  `main.go`.
- `--yolo` help text promises it "never overrides safety denials"
  (`main.go:181`); no denial logic exists in this package, so the
  guarantee is implemented — if at all — downstream.
- The flight-recorder comments (`main.go:375-386`: 64 MiB / 30 s
  window, memory watchdog, exit-time stop) describe the
  `observability` package's contract; `main.go` only passes the
  arguments.
- `SilenceUsage = true` (`main.go:110`) assumes command errors are
  already human-readable without usage text; nothing here verifies
  that per command.
