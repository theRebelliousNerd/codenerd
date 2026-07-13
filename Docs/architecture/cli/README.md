# cli — the human control surface

> `VERIFIED CURRENT` against `cmd/nerd` on 2026-07-13. The CLI owns presentation,
> command validation, workspace selection, cancellation, and boot orchestration;
> it does not own constitutional policy.

## In one minute

The `nerd` binary is how a person enters codeNERD. It offers single-shot Cobra
commands, an interactive Bubble Tea chat, campaigns, logic inspection, auth,
browser and CodeDOM utilities, and operator-facing status surfaces. Its outcome
is not merely text: it boots the same logic-first runtime, sends intent through
the kernel and VirtualStore, and renders the resulting evidence.

`cmd/nerd/main.go#rootCmd` is the command-tree entry. Bare `nerd` calls
`cmd/nerd/chat/model_lifecycle.go#RunInteractiveChat`; `nerd run` reaches
`cmd/nerd/cmd_instruction.go#runInstruction`.

## Its place in codeNERD

```text
human input / flags
  -> CLI validates workspace, timeout, and command shape
  -> interactive or one-shot boot constructs Cortex
  -> perception turns language into typed intent
  -> Mangle derives the permitted next action
  -> VirtualStore/tactile perform bounded effects
  -> articulation + transparency become terminal output
```

The CLI is an adapter around the architecture, not an alternate executive.
Creative interpretation stays in the LLM-facing perception/session layers;
planning and permission stay in Mangle and the kernel. A CLI flag may select a
workspace or request a system-shard configuration, but it must never invent a
parallel permission bypass.

The owned surface is `cmd/nerd`, including `chat/` and `ui/`. Internal config,
kernel, shard, campaign, prompt, store, tactile, and transparency packages remain
separate authorities; [08-DEPENDENCY-MAP.md](08-DEPENDENCY-MAP.md) records those
edges.

## A representative journey

For `nerd --workspace C:\work run "review auth"`:

1. `cmd/nerd/main.go#rootCmd` resolves CLI flags, initializes bounded logging,
   and loads workspace configuration without printing credentials.
2. `cmd/nerd/cmd_instruction.go#runInstruction` creates the operation deadline
   and calls the shared Cortex boot surface with the selected workspace and
   disabled-shard set.
3. The runtime perceives the instruction, asserts intent, derives an exact
   action, checks permission, executes through VirtualStore, and returns a
   result. The CLI prints the result; it does not reinterpret permission.
4. Cancellation propagates through the command context. Boot/action failures
   return errors rather than a hollow success.

The interactive route replaces step 2 with
`cmd/nerd/chat/session_shared_boot.go#performSystemBootShared`, then
`cmd/nerd/chat/process.go#Model.processInput` performs each OODA turn. That async
closure recovers a panic into a user-visible `errorMsg`, proven by
`cmd/nerd/chat/chat_loop_contract_e2e_test.go#TestE2E_ChatLoop_PerceptionPanic_RecoveredAsErrorAndIdle`.

## What exists today

- `VERIFIED CURRENT`: Cobra registers one-shot, direct-action, campaign, auth,
  Mangle, browser, advanced, and diagnostic commands in
  `cmd/nerd/main.go#rootCmd`. [05-COMMAND-ARCHITECTURE.md](05-COMMAND-ARCHITECTURE.md)
  is the full command map.
- `VERIFIED CURRENT`: interactive input has readiness guards, bounded turn
  context, shutdown cancellation, and panic recovery in
  `cmd/nerd/chat/process.go#Model.processInput`.
- `VERIFIED CURRENT`: chat boot prefers the shared production path in
  `cmd/nerd/chat/session_shared_boot.go#performSystemBoot`; the large legacy boot
  remains as a compatibility seam and is not safe to delete from prose alone.
- `PARTIAL`: Cobra and slash commands overlap substantially but have no generated
  parity authority. Some differences are intentional; others can drift.
- `PARTIAL`: boot is broad and well exercised at package level, but failure
  injection and resource-unwind receipts remain weaker than the success path.
- `PARTIAL`: JIT status is visible, while a turn-correlated explanation of atom
  selection, permission, tool execution, and final rendering is still spread
  across several surfaces.

Current implementation truth is in [IMPLEMENTED_SPEC.md](IMPLEMENTED_SPEC.md)
and [02-CURRENT-STATE-CLI.md](02-CURRENT-STATE-CLI.md). Safety and tests are in
[09-CONSTITUTIONAL-SAFETY.md](09-CONSTITUTIONAL-SAFETY.md) and
[10-TESTING-ALIGNMENT.md](10-TESTING-ALIGNMENT.md).

## North star

Every command and interactive turn should be a thin, consistent window into one
logic-first runtime: explicit workspace, deterministic boot identity, exact
permission, cancellable effects, honest partial failure, and a concise receipt
of what happened. A feature should not exist as a differently behaving Cobra,
slash, MCP, and campaign implementation when one typed application service can
serve all four.

Non-goals: the CLI does not become the policy engine, store secrets in command
history, swallow boot failures for a prettier screen, or expose a “skip safety”
mode. Visual polish cannot outrank action identity, cancellation, or recovery.

## Improvement frontier

The immediate repair is a generated command-surface manifest that classifies
Cobra/slash parity and pins intentional asymmetry. The next leverage step is a
transactional boot receipt: resolved workspace/config identity, acquired
resources, enabled system shards, rollback outcome, and a stable error class.

The bounded north-star option is one turn receipt joining input, selected JIT
context, derived permission, effect IDs, cancellation, and rendered result with
redaction and retention limits. It explains the runtime without storing raw
prompts, keys, or unbounded tool output. Authoritative cards are in
[TODO.md](TODO.md).

## Choose a reading route

- **90 seconds:** this README, then [03-GAP-ANALYSIS-CLI.md](03-GAP-ANALYSIS-CLI.md).
- **10 minutes:** add [05-COMMAND-ARCHITECTURE.md](05-COMMAND-ARCHITECTURE.md),
  [06-TUI-CHAT-SURFACE.md](06-TUI-CHAT-SURFACE.md), and
  [11-CROSS-SYSTEM-WIRING-JOURNAL.md](11-CROSS-SYSTEM-WIRING-JOURNAL.md).
- **Deep implementation:** read [IMPLEMENTED_SPEC.md](IMPLEMENTED_SPEC.md),
  [09-CONSTITUTIONAL-SAFETY.md](09-CONSTITUTIONAL-SAFETY.md),
  [10-TESTING-ALIGNMENT.md](10-TESTING-ALIGNMENT.md), then begin at
  `cmd/nerd/main.go#rootCmd` and `cmd/nerd/chat/model_lifecycle.go#RunInteractiveChat`.

Open design choices remain in [OPEN-QUESTIONS.md](OPEN-QUESTIONS.md); signed
evidence and freshness live in [_progress.md](_progress.md).
