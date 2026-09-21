# cmd/nerd — the `nerd` CLI binary

Verified 2026-09-20 against `main` (working tree up to date with
`origin/main` at time of writing). `rootCmd` reports Cortex 1.5.0
(`cmd/nerd/main.go:100`). No HEAD hash was captured in this pass —
re-pin with `git rev-parse --short HEAD`.

## What the package is

`cmd/nerd` is `package main` spread over the root `*.go` files
(e.g. `cmd/nerd/apikey.go:1`, `cmd/nerd/main.go:54`). It contains no
business logic: it declares the cobra command tree, the global flags,
and the process boot sequence, then delegates. `cmd/nerd/chat/` is a
separate `package chat` (e.g. `cmd/nerd/chat/commands.go:26`) holding
the interactive TUI; `main.go:6-53` carries a header index mapping
files to commands. `package main` imports `chat` but not `ui`
(`cmd/nerd/main.go:56-71`).

Running `nerd` with no arguments starts the interactive chat:
`RunE` changes into `--workspace` and calls `chat.RunInteractiveChat`
(`cmd/nerd/main.go:148-175`). Every other invocation runs one
subcommand's `RunE` from its own `cmd_*.go` file.

## Command tree

Parents below are exactly the `AddCommand` blocks in `init()`
(`cmd/nerd/main.go:216-331`). Leaf `Use` strings were read from the
defining files.

- Core (`main.go:248-265`): `run` (`run [instruction]`,
  `cmd_instruction.go:33`), `chat` (`chat [turn...]`,
  `cmd_chat.go:23`), `define-agent` / `spawn` (`cmd_spawn.go:28,40`),
  `browser` (`cmd_browser.go:46`, subcommands `launch`…`honeypot`
  at `cmd_browser.go:58-113`), `query` / `status` / `why`
  (`cmd_query.go:23,37,44`), `init` / `scan`
  (`cmd_init_scan.go:36,110`), `campaign` (`cmd_campaign.go:73`;
  `start`/`status`/`pause`/`resume`/`list` at
  `cmd_campaign.go:93-132`), `check-mangle`
  (`cmd_mangle_check.go:25`), `mangle-lsp` (`cmd_mangle_lsp.go:75`),
  `auth` (plus `claude`/`codex`/`grok`/`status` at
  `cmd_auth.go:21-77`), `logs` (`cmd_logs.go:65`), `swebench`
  (plus `setup` at `cmd_swebench.go:22,33`).
- Direct-action verbs (`main.go:267-280`): `review`, `fix`, `test`,
  `push`, `commit`, `explain`, `create`, `refactor`, `perception`,
  `security`, `analyze` (`cmd_direct_actions.go:32-222`).
- Advanced (`main.go:282-293`): `dream`, `shadow`, `whatif`,
  `logic`, `agents`, `tool`, `jit` (`cmd_advanced.go:35-136`),
  plus `dom` and `embedding`, registered at `main.go:290-292` and
  defined in `dom_cmd.go` / `embedding_cmd.go`.
- `northstar` (`main.go:296-298`): subcommands `show`…`load` at
  `cmd_northstar.go:99-626` under the parent at
  `cmd_northstar.go:33`.
- Visibility (`main.go:300-314`): `mcp` (plus `list`/`tools`/`status`
  at `cmd_systems.go:26-123` and the autopoiesis/memory group at
  `cmd_systems.go:238-465`), `memory`, `usage` (`cmd_usage.go:37`),
  `regression` (plus `run`/`init`/`list` at
  `cmd_regression.go:24-116`), `features` (`cmd_features.go:19`),
  `snapshot` (plus `export`/`import`/`list` at
  `cmd_snapshot.go:29-70`), `context-stats`
  (`cmd_context_stats.go:27`), `meter` (plus `epochs`/`atoms` at
  `cmd_meter.go:95-444`), `audit` (plus `facts`/`playbook` at
  `cmd_audit.go:28-96`), `world` (plus `runbook`/`predicates` at
  `cmd_world.go:18-35`), `retrieve` (`cmd_retrieve.go:46`).
- `sessions` (`main.go:316-319`): plus `list`/`load` at
  `cmd_sessions.go:22-41`.
- `knowledge` (`main.go:321-324`): plus `list`/`search` at
  `cmd_knowledge.go:22-41`.
- Transparency (`main.go:326-331`): `glassbox`, `transparency`,
  `reflection` (`cmd_transparency.go:26-314`).

The following exist in the file listing but are attached nowhere in
`main.go:178-332`, so their attachment — if any — lives in their own
files, which this pass did not read: `journal`/`verify`/`replay`/
`report` (`cmd_campaign_journal.go:34-65`), `assault`
(`cmd_campaign_assault.go:64`), `recurse`
(`cmd_campaign_recurse.go:41`), `select`/`metrics`
(`cmd_mcp_select.go:35-47`), `test-context`
(`cmd_test_context.go:42`).

## Flags

Global (`main.go:180-198`): `--verbose`, `--yolo`, `--api-key`,
`--workspace`, `--timeout` (default `0` = none, `main.go:187-188`),
`--disable-system-shard`. Per-command: `define-agent` requires
`--name`/`--topic` (`main.go:191-195`); `fix` takes `--acceptance`
(`main.go:203`); six direct-action commands share `--interactive`
(`main.go:202-206`); `registerDebugFlags` covers the same six
(`main.go:210`); `init` takes `--force`/`--cleanup-backups`
(`main.go:213-214`); `campaign start` takes `--docs`/`--type` and
`campaign resume` takes `--retry-failed`/`--campaign`
(`main.go:224-229`, vars at `main.go:90-95`).

## What this corpus does not cover

- Registration and boot mechanics → `INTERNALS.md` (one question:
  how does a command get from a file to a running process?).
- What is reachable, what exists but is unwired here, and what the
  code assumes but does not do → `WIRING-AND-NOT-BUILT.md`.
- Per-command behaviour: each `RunE` lives in its own `cmd_*.go`
  and is not described here. Test files (`*_test.go`, e.g.
  `cmd_flags_test.go`, `parent_group_test.go`) sit beside the code
  in `package main`.
