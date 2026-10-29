# Dependency map

## What `internal/broker` imports

| Package | Why |
|---|---|
| `internal/types` | `LLMClient`, `Message`, `ToolDefinition`, `UsageMetadata` |
| `internal/usage` | the observer hook that captures provider actuals |
| `internal/logging` | `CategoryAPI` |
| stdlib | `net/http`, `container/list`, `crypto/sha256`, `encoding/json`, `sync`, `time`, `go/*` (tests) |

Deliberately absent: `internal/perception`, `internal/prompt`, `internal/context`,
`internal/core`, `internal/session`, `internal/config`. The broker is imported
*by* those; importing any of them back would create a cycle and would also mean
the meter had opinions about what it was metering.

The three capability-gating interfaces are declared **structurally** inside
`broker` rather than imported from `core`. Go interfaces are structural, so a
shape satisfying the local declaration also satisfies `core.SchemaCapableLLMClient`
— and the package stays free of a dependency it does not need.

## What imports `internal/broker`

| Package | Uses |
|---|---|
| `internal/perception` | `Wrap`, `Default().ConfigFor`, `IsBrokered` — installs metering at the factory |
| `internal/system` | `Configure` — points the meter at the workspace window at boot |
| `internal/context` | `TextCounter` — the compressor's token counting |
| `internal/session` | `Default().Ledger()`, `PromptBudget` — summarization allowance, prompt budget |
| `internal/init` | `PromptBudget` — init prompt sizing |
| `internal/articulation` | `PromptBudget` — the focused-shard clamp |
| `cmd/nerd/chat` | `Base` — reaching through the decorator for the engine check |

## The one change outside the package

`internal/usage/usage_tracker.go#TrackFromContext` gained four lines that notify
an observer alongside the tracker. `internal/usage/observer.go` is new.

This is the whole mechanism by which the broker learns actual usage on code paths
whose return values carry none. Fifteen lines in the plumbing every provider
already uses, instead of five methods across nine provider clients — which is the
difference between one accounting boundary and a tenth parallel system.

## Decorator ordering in production

```text
core.ScheduledLLMCall
  └── perception.TracingLLMClient
        └── broker wrapper shape
              └── provider client (anthropic / openai / gemini / …)
```

The broker sits **inside** tracing rather than outside, for two reasons. Tracing
probes its underlying for a long list of accessors, and the broker forwards all
of them. And `cmd/nerd/chat/commands.go` asserts on `*perception.TracingLLMClient`,
which keeps working while the outermost layer is unchanged.
