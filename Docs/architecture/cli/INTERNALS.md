# cli internals — from a file to a running process

Verified 2026-09-20 against `main` (working tree up to date with
`origin/main`). Pin notes as in `README.md`. All symbols below are in
`cmd/nerd/main.go` unless stated.

## Registration: `init()` builds the whole tree

`init()` (`main.go:178-332`) does three things in order: persistent
global flags (`main.go:180-198`), per-command flags and subgroups
(`browserCmd`, `campaignCmd`, `authCmd` at `main.go:216-245`;
`define-agent`, `fix`, direct-action, `init`, `campaign` flags at
`main.go:190-229`), then five `rootCmd.AddCommand` blocks
(`main.go:247-331`). Nothing registers itself anywhere else as far as
this file shows — there is no other `AddCommand` callee in
`main.go`, and the header index (`main.go:6-53`) describes
`cmd_*.go` files as command definitions consumed by this hub.

## Pre-run: logger, then file logging, except for bare `nerd`

`PersistentPreRunE` (`main.go:109-140`) sets `SilenceUsage = true`,
then returns early for bare interactive invocation
(`main.go:112-114`). Otherwise it builds the zap logger
(`--verbose` selects debug level, `main.go:117-125`) and calls
`logging.Initialize` on the workspace, warning — not failing — if
file logging cannot start (`main.go:129-137`).
`PersistentPostRun` syncs the logger and closes file logging
(`main.go:141-147`).

## Run: change directory, then chat

`RunE` (`main.go:148-175`) resolves `--workspace` (default: cwd),
`os.Chdir`s into it with distinct permission-denied / not-exist
errors (`main.go:157-167`), and calls
`chat.RunInteractiveChat(chat.Config{DisableSystemShards: ...})`
(`main.go:171-174`). The comment at `main.go:149-150` states why the
chdir exists: the chat UI uses `os.Getwd()` as its workspace root.

## Boot before cobra: `main()`

`main()` (`main.go:353-400`) runs, in order: idempotent
`logging.Initialize` (`main.go:355-361`), eager
`config.GlobalConfig()` load tolerated on error
(`main.go:363-370`), `observability.LogStartupMetrics()`
(`main.go:373`), and conditional `StartFlightRecorder` with a
panic-dump hook (`main.go:387-400`). Campaign invocations skip the
recorder: `isCampaignInvocation` (`main.go:343-351`) returns true when
the first positional arg is `campaign`, and its comment
(`main.go:334-342`) records the reason — tracer stack-table OOM under
campaign goroutine churn (F-TRACE-1, observed 2026-07-13).
