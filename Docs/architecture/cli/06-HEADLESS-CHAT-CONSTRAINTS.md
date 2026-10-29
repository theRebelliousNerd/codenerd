---
doc-class: north-star
subsystem: cli
implementation-status: partial
last-verified: 2026-10-02
verified-against: db1d4b7e24d0f9440c8627b2561ca64a25876954
supersedes: []
---

# Headless chat caller constraints

## Purpose and evidence

**PROPOSED UPLIFT:** headless chat should continue while it makes progress within the user's constraints. An explicit wall-clock bound is one such constraint. Its absence should permit long-running work; no tool-count limit or new default task clock should substitute for kernel-derived progress and stall obligations.

The global CLI flag promises a whole-command bound in `cmd/nerd/main.go#init` (`cmd/nerd/main.go:199`). On baseline `db1d4b7e`, headless chat contexts carried cancellation but did not apply the selected timeout. The live counterexample and independently measured artifact repair are recorded under `artifact:.corpus-build/runs/all-features-20261002/dogfood-stop.json`, `dogfood-chat.log`, and `dogfood-after.log`.

**PARTIAL — authored lifecycle integration:** `cmd/nerd/cmd_chat.go#runChatWith` (`cmd/nerd/cmd_chat.go:168`) now derives the command context before boot and uses it for turn admission and idle input cancellation. Root's filtered CLI gate passes; receipt: `artifact:.corpus-build/runs/all-features-20261002/round2-cli.log`. Behavioral witnesses include boot (`cmd/nerd/cmd_chat_lifecycle_test.go:49`), shared turn deadline/artifact retention (`cmd/nerd/cmd_chat_lifecycle_test.go:71`) and inherited production stdin cancellation (`cmd/nerd/cmd_chat_lifecycle_test.go:320`). Cortex cleanup remains synchronously joined rather than abandoned. Context-unaware boot-lock/shutdown dependencies and a fresh normal-entry measured process exit are still unqualified; GAP-CLI-01 remains open, including the bounded-cleanup contract.

## Accepted contract

**PARTIAL — fresh production counterexample:** `artifact:.corpus-build/runs/all-features-20261002/round5-firstboot-idle.receipt.json` records the fresh sqlite_vec binary with a new isolated fixture, stdin held open and no submitted turns. `--timeout 20s` returns exit 1 with `chat: context deadline exceeded`, but only after 58.4495 seconds; no external termination occurred and output drained. Source/phase analysis in `02-execution/handoff-5.json` identifies contextless cold classifier hydration: about 47.922 seconds of hydration, followed by further boot work after cancellation. Cleanup was subsecond in this probe, not the measured overrun's cause; its worst-case bound remains unproved. The later canceled embedding health check is downstream evidence, not the root cause. Accepted system/perception/embedding context-propagation gaps now own the repair. First-boot prompt rows are current; model-turn/world/CodeDOM behavior remains unqualified. Preserve this discriminator; never replace joined cleanup with abandonment or claim the filtered helper green closes GAP-CLI-01.

1. A positive `--timeout` should establish one command deadline before Cortex boot. Boot, all turns, and waiting for further input should share it. Processing a later turn should not restart the command budget.
2. With zero or negative timeout, chat should add no deadline. Parent command cancellation and operating-system signals should still propagate. Reuse `cmd/nerd/operation_context.go#commandContext` (`cmd/nerd/operation_context.go:32`) rather than inventing a second interpretation.
3. Expiration or parent cancellation should stop further turn admission, cancel active request/tool work, and produce an explicit unsuccessful command result. Preserve the executor's failure semantics and typed, permission-mediated operations.
4. Input waiting should observe cancellation without requiring another input line. Avoid leaving an admitted reader or signal-wait goroutine behind after the command ends.
5. Shutdown should retain resource cleanup and use a documented, measured bound. A cleanup allowance should not become an unlimited command extension. If boot or shutdown seams cannot satisfy this, register a separate cross-package packet and preserve this gap as open.
6. Completed source edits should survive cancellation. Their existence or green independent tests should not be presented as successful completion of an interrupted command.

## Verification obligations

**VERIFIED CURRENT — cold normal-entry discriminator only:** after the context-propagation repair, fresh sqlite_vec binary SHA25661820661540A43937CEB1AD04969A1BC5496B05FB94F56D703D0DE1DB77DC140 exits a new cold fixture's20s command in21.6602s with typed boot/kernel/classifier deadline failure, no external termination and output drained (`artifact:.corpus-build/runs/all-features-20261002/round6-firstboot-idle.receipt.json`). The prior58.4495s counterexample remains as before-repair evidence. Context-aware default-bootstrap and focused race controls pass;448 cache rows survive cancellation and no later prompt corpus is created. This meets this probe's20s-plus5s allowance only. Actual command-context creation is not separately timestamped; elapsed overhead is not all attributed to cleanup. Complete affected suites, successful turns/world/CodeDOM, warm/inherited cancellation and universal bounded shutdown remain separately unqualified.

**PROPOSED UPLIFT:** add behavioral regressions in the CLI test suite for a blocked turn receiving the deadline, inherited cancellation, prevention of later turn admission, cancellation while input is idle, and the absence of an added deadline for non-positive timeout. Test collaborators may isolate external model latency; assertions must concern the actual command lifecycle rather than merely duplicate the context helper.

The root should run the affected CLI tests, relevant boot/session cancellation tests, a sqlite_vec build, and a normal production `nerd chat` probe with an explicit deadline and an available embedding engine. Capture process exit, elapsed time, cancellation result, completed source artifacts, and original fixture test results. Any absent seam or failing assertion keeps GAP-CLI-01 open.

## Ownership and rollback

The initial packet may own `cmd/nerd/cmd_chat.go` and adjacent tests. Changes to `internal/system`, `internal/session`, or shared CLI helpers require an explicit ownership expansion and corresponding package specs first. No changes to prompt policy, permission derivation, or protected concurrent files are implied. Reverting the packet's exact changes should restore the prior lifecycle without touching user work.
