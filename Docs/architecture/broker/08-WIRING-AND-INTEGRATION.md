# Wiring and integration

> Verified against the codebase on 2026-09-09.

## 1. Boot

`internal/system/factory.go` calls `configureBrokerMeter(bctx.appCfg)` before any
client is constructed. `internal/system/broker_meter.go` reads
`UserConfig.GetContextWindowConfig()` and folds three reserves into one:

```go
reserve := ctxCfg.OutputReserve + ctxCfg.ThinkingReserve + ctxCfg.ToolUseBuffer
broker.Configure(broker.MeterConfig{Window: ctxCfg.MaxTokens, OutputReserve: reserve})
```

Thinking tokens are billed as output and occupy the same reserve. Tool cycles
append results into the same window after admission.

## 2. Client construction

Three exported constructors in `internal/perception/client_factory.go` are the
only way to obtain an `LLMClient`:

| Constructor | Raw builder | Metered by |
|---|---|---|
| `NewClientFromConfig` | `newRawClientFromConfig` | `InstallBroker(client, config)` |
| `NewClassificationClientFromConfig` | `newRawClassificationClientFromConfig` | `InstallBroker` with `ClassificationModel` substituted |
| `NewClientFromEnv` | — | delegates to `NewClientFromConfig` |

Splitting raw construction from the exported wrapper means a **new provider case
added to the switch is metered automatically**, rather than depending on whoever
adds it remembering to wrap.

The classification tier substitutes its own model before metering. Attributing
its tokens to the main model would misfile the spend and — worse — feed the
shared calibrator observations from one tokenizer labelled as another's.

Two further exported constructors delegate to `newSecondarySlotClient`, which
itself routes through `NewClientFromConfig`:

- `NewWorkerClientFromUserConfig`
- `NewPlannerClientFromUserConfig`

`NewImageClientFromUserConfig` builds a Gemini client directly and calls
`InstallBroker` explicitly. It did not, until the wiring audit caught it; image
generation is inference and was spending entirely off the books.

## 3. The one construction outside the factory

`internal/system/factory.go` builds a ZAI client from a legacy raw `--api-key`
argument. It calls `perception.InstallBrokerForProvider` rather than leaving a
hole the audit has to special-case.

## 4. Idempotency

`InstallBroker` returns a client that is already brokered untouched, so a nested
construction path cannot charge one request to the ledger twice.

## 5. Call sites that changed

| Site | Change | Why |
|---|---|---|
| `cmd/nerd/chat/model_session_context.go` | `broker.Base(m.client).(*perception.CodexCLIClient)` | a bare assertion never matches a wrapped client, and the `codex_cli` framework tag would silently vanish from JIT atom selection |
| `internal/context/tokens.go` | `broker.TextCounter` replaces `charsPerToken = 4.0` | one ruler |
| `internal/session/semantic_compressor.go` | `summarizationCharAllowance()` replaces `const maxTokens = 64000` | derived from the ledger |
| `internal/session/executor.go` | `DefaultTokenBudget()` replaces the const | derived from the ledger |
| `internal/init/jit_integration.go` | `PromptBudget(0.75, 120000)` replaces the literal | scales with the window |
| `internal/articulation/prompt_assembler.go` | `PromptBudget(0.3, 60000)` replaces the clamp | a deliberate narrowing, expressed as a share |

Provider clients and the 145 completion call sites are **unchanged**. They take a
`types.LLMClient` and always did.

## 6. Registration hooks

| Hook | Present? |
|---|---|
| Mangle `Decl` owned by this package | No — the broker meters, it does not derive |
| VirtualStore action route | No |
| Shard registration | No |
| CLI subcommand | No (receipts are read through `Meter.Receipts()`) |

## 7. Audit entry points

| Property | Test |
|---|---|
| every exported constructor is metered | `TestEveryClientConstructionPathIsBrokered` |
| the delegate helper is itself metered | same test, second half |
| the heuristic counter is gone from the tree | `TestNoCompetingTokenCounters` |
| the orphan constants are gone | `TestOrphanBudgetConstantsAreGone` |
| the usage observer hook is present | `TestUsageObserverHookIsWired` |
| boot configures the meter | `TestBrokerIsInstalledAtBoot` |
| the legacy path is metered | `TestLegacyClientPathIsMetered` |
| concrete assertions unwrap | `TestConcreteTypeAssertionsReachThroughTheDecorator` |

These scan code with comments stripped, so an explanation that names a removed
identifier does not read as its return.
