# CLI feature cards

> Sole authoritative `NERD_FEATURE` surface for the CLI corpus.

## P0: Keep async turn failure inside the TUI contract

<!-- NERD_FEATURE
id: cli-async-panic-recovery-v1
owner: cli
status: verified
kind: truth-gap
depends_on: []
affects: [cli, observability]
-->

**Value.** A panic in an asynchronous turn becomes a visible error and releases
the loading state instead of killing or wedging the process.

**Evidence and observed gap.** Bubble Tea executes the closure returned by
`cmd/nerd/chat/process.go#Model.processInput` asynchronously. It now uses a named
return and deferred recovery to emit `errorMsg`; the contract regression lives at
`cmd/nerd/chat/chat_loop_contract_e2e_test.go#TestE2E_ChatLoop_PerceptionPanic_RecoveredAsErrorAndIdle`.

**Desired behavior.** Every fire-and-forget CLI/TUI boundary either proves it
cannot panic or converts panic, cancellation, and error into a terminal message
and correlated diagnostic signal.

**Non-goals.** Recovery does not authorize retry, hide the stack from protected
logs, or turn a partial effect into success.

**Affected contracts.** Bubble Tea commands, loading state, shutdown context,
operator diagnostics.

**Positive acceptance.** The named regression proves a recovered panic returns
an error message and model update records the error; race tests cover the chat
package.

**Negative acceptance.** No raw credential or full prompt is shown; a recovered
panic cannot report success or silently repeat an effect.

**Rollback.** Replace only with a broader async boundary that preserves the same
terminal/error contract and regression.

## P0: Make boot transactional and explainable

<!-- NERD_FEATURE
id: cli-transactional-boot-receipt-v1
owner: cli
status: proposed
kind: truth-gap
depends_on: []
affects: [cli, system, config, store, shards, observability]
-->

**Value.** Failed boot leaves no leaked database, goroutine, cache entry, or
partially active shard, and tells the operator which stage failed.

**Evidence and observed gap.** `cmd/nerd/chat/session_shared_boot.go#performSystemBootShared`
delegates to the shared system factory, while the compatibility implementation
in `cmd/nerd/chat/session_boot.go#performSystemBootLegacy` acquires many resources
inline. Success wiring is visible; complete stage-by-stage rollback evidence is
not one stable CLI contract.

**Desired behavior.** Use one staged boot transaction with resolved identity,
resource ownership, reverse-order unwind, cache admission only after success,
and a redacted receipt shared by interactive and one-shot commands.

**Non-goals.** Do not duplicate system-factory lifecycle logic in the CLI or
continue after a required kernel/policy failure.

**Affected contracts.** Workspace/config identity, logging, stores, kernel,
shards, watchers, maintenance, TUI error rendering.

**Positive acceptance.** Failure injection at every stage proves acquired
resources close once, no failed instance is cached, later boot succeeds, and the
CLI presents a stable stage/recovery class.

**Negative acceptance.** Keys and raw config are redacted; optional subsystem
degradation is distinguishable from required boot failure.

**Rollback.** Keep the shared boot adapter and old error renderer behind a
temporary compatibility switch; never roll back deterministic resource unwind.

## P1: Generate the command-surface parity contract

<!-- NERD_FEATURE
id: cli-command-surface-manifest-v1
owner: cli
status: proposed
kind: leverage
depends_on: []
affects: [cli, mcp, campaign, testing]
-->

**Value.** Users know which operation exists in Cobra, slash, MCP, or campaign
form, and adding a surface cannot silently diverge validation or permission.

**Evidence and observed gap.** `cmd/nerd/main.go#rootCmd` registers Cobra commands
while chat command routing is maintained separately. The current corpus maps both
but no machine-readable authority proves required parity or intentional TUI-only
behavior.

**Desired behavior.** Define one typed operation manifest with aliases, surfaces,
arguments, workspace semantics, permission path, output mode, and intentional
asymmetry. Generate/check help and dispatch registration from it.

**Non-goals.** Do not force visual-only TUI controls into Cobra, or let a manifest
grant constitutional permission.

**Affected contracts.** Cobra help, slash routing, MCP/A2A exposure, campaigns,
documentation, conformance tests.

**Positive acceptance.** CI enumerates each live registry, rejects missing or
duplicate mappings, and runs equivalent positive/negative fixtures for every
operation marked portable.

**Negative acceptance.** An undocumented command or argument drift fails; aliases
cannot bypass validation; intentional asymmetry requires a reason and owner.

**Rollback.** Retain existing registries as generated consumers until parity is
proven; rollback generation, not the conformance inventory.

## P2: Expose one redacted turn receipt

<!-- NERD_FEATURE
id: cli-turn-explanation-receipt-v1
owner: cli
status: proposed
kind: north-star
depends_on: [cli-transactional-boot-receipt-v1, cli-command-surface-manifest-v1]
affects: [cli, prompt, core, tactile, transparency, articulation]
-->

**Value.** A user can answer what codeNERD understood, why logic allowed or
blocked it, what effect ran, and why that text appeared—without entering debug
logs.

**Evidence and observed gap.** Glass-box, transparency, JIT, campaign, and normal
chat surfaces expose slices of a turn, but there is no one bounded schema joining
prompt-selection, permission, effect, and articulation IDs.

**Desired behavior.** Render a versioned receipt with correlation IDs, selected
atom summary, intent/action/permission class, tool and truncation status,
cancellation/recovery state, and final response method. Store digests and bounded
previews under explicit retention/redaction policy.

**Non-goals.** Do not reveal chain-of-thought, credentials, raw prompts, or full
tool output; the receipt observes rather than authorizes.

**Affected contracts.** JIT compiler, kernel decisions, VirtualStore/tactile,
transparency, articulation, TUI pages and one-shot output.

**Positive acceptance.** Success, denial, no-route, timeout, cancellation,
truncation, and partial-observability fixtures render deterministically with the
same turn ID across surfaces.

**Negative acceptance.** Missing optional telemetry is labeled rather than
fabricated; secret fixtures never appear; receipt failure cannot alter execution.

**Rollback.** Preserve existing glass-box and normal output surfaces; disable the
joined renderer without removing producer correlation IDs.
