# cli wiring — and what is NOT built here

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
