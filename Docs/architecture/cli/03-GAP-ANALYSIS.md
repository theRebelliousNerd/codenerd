---
doc-class: shipped-with-future
subsystem: cli
implementation-status: partial
last-verified: 2026-10-02
verified-against: db1d4b7e24d0f9440c8627b2561ca64a25876954
supersedes: []
---

# CLI gap analysis

This matrix records accepted, source-grounded implementation obligations. It is an incremental census; the remaining CLI portfolio is still being audited and is not declared complete.

| Gap ID | Capability | Current state | Target state | Severity | Phase | Blocking dependencies | Exit criteria |
|---|---|---|---|---|---|---|---|
| GAP-CLI-01 | Explicit headless-chat command deadline | **PARTIAL:** `cmd/nerd/cmd_chat.go#runChatWith` (`cmd/nerd/cmd_chat.go:168`) now derives commandContext before boot and shares cancellation/deadline through turns and idle input. Root filtered behavioral CLI gate passes: `artifact:.corpus-build/runs/all-features-20261002/round2-cli.log`. Baseline dogfood-stop.json records the former 616-second overrun. Fresh production boot/shutdown exit and bounded cleanup remain unqualified. | **PROPOSED UPLIFT:** [headless chat constraints](06-HEADLESS-CHAT-CONSTRAINTS.md) applies an explicit caller deadline to boot, turns, input waiting, and cancellation-aware shutdown while preserving unlimited duration when no deadline is selected. | High | Partial; lifecycle tests pass, production exit/cleanup open | CLI owner; shared shutdown or boot changes require separate ownership if necessary | Root-run behavioral regression proves explicit deadline and inherited cancellation reach boot/turn processing and prevent later turns; zero or negative timeout adds no deadline. Production `nerd chat --timeout` exits under the chosen bound plus documented bounded cleanup and independently preserves completed edits. |

The generic command helper already defines the intended positive/non-positive semantics: `cmd/nerd/operation_context.go#operationContext` (`cmd/nerd/operation_context.go:19`) and `#commandContext` (`cmd/nerd/operation_context.go:32`). Their existence does not prove headless chat uses them.

Close a gap only after its exit evidence exists. Preserve the row and cite the closing revision and measured commands; source edits or a passing helper test alone cannot close the production deadline obligation.
