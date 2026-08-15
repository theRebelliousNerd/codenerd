# usage — Wiring and Integration

> Last verified: **2026-08-15**

## Registration model

There is **no** central plugin registry, Mangle Decl set, or VirtualStore route for usage. Integration is **manual context plumbing**.

## Boot wiring (Cortex)

**File:** `internal/system/factory.go`

1. `initCoreComponents` calls `usage.Shared(bctx.workspace)`.  
2. On error: stderr warning; tracker may be nil.  
3. Tracker stored on `bootContext.tracker`.  
4. Assembled into `Cortex.UsageTracker` when Cortex is returned (~line 1086 region).

```
BootCortex / GetOrBootCortex
  → initCoreComponents
      → usage.NewTracker(workspace)
  → Cortex{ UsageTracker: tracker, ... }
```

## CLI verb wiring

Each long-running / LLM-using command that boots Cortex attaches the tracker:

| File | Pattern |
|------|---------|
| `cmd/nerd/cmd_advanced.go` | `if cortex.UsageTracker != nil { ctx = usage.NewContext(ctx, …) }` |
| `cmd/nerd/cmd_interactive.go` | same |
| `cmd/nerd/cmd_instruction.go` | same |
| `cmd/nerd/cmd_direct_actions.go` | same (+ tracer note) |

If a new Cobra verb boots Cortex and calls the LLM **without** this attach, metering silently skips.

## Interactive chat wiring

| Step | File | Action |
|------|------|--------|
| Construct | `cmd/nerd/chat/session.go` | `usage.Shared(workspace)` into model — the **same** tracker Cortex gets |
| Store field | `cmd/nerd/chat/model_types.go` | `usageTracker *usage.Tracker` |
| Per-turn / stream | `cmd/nerd/chat/process.go` | `usage.NewContext` on process and stream contexts |
| Campaign phases | `cmd/nerd/chat/campaign.go`, `campaign_assault.go` | `NewContext` when tracker non-nil |

## Shard attribution wiring

**File:** `internal/core/shards/manager_spawn.go`

Before goroutine execution:

```go
ctx = usage.WithShardContext(ctx, config.Name, string(config.Type), sessionID)
```

`sessionID` falls back to `"current-session"` if manager session id empty.

Downstream LLM calls inside the shard inherit attribution **only if** they use this ctx (or a child).

## Perception producer wiring

Every producer goes through one helper, `internal/perception/usage_track.go`:

```go
trackUsage(ctx, model, provider, input, output, operation)
  → usage.TrackFromContext(ctx, model, usageProviderID(provider), …)
```

`usageProviderID` maps a `perception.Provider` onto the id config uses for that
engine, and prefixes anything unrecognized with `unregistered:` rather than
silently merging it into a real provider's row.

### Producer table

| Client | File(s) | Track point | Operation | Provider id |
|--------|---------|-------------|-----------|-------------|
| ZAI | `client_zai.go` | after `ZAIResponse` parse (chat, structured, tools) | `chat` / `tool_gen` | `zai` |
| ZAI stream | `client_zai_streaming.go` | once after stream ends, from final usage chunk | `chat` | `zai` |
| Anthropic | `client_anthropic.go` | after `AnthropicResponse` parse (chat, tools, tool-results) | `chat` / `tool_gen` | `anthropic` |
| Anthropic stream | `client_anthropic.go` | once after stream ends, from `message_start` + `message_delta` | `chat` | `anthropic` |
| OpenAI | `client_openai.go` | `CompleteWithSystem` + `completeNonStreaming` funnel | `chat` / `tool_gen` | `openai` |
| OpenAI stream | `client_openai.go` | once after stream ends, from `include_usage` chunk | `chat` | `openai` |
| Ollama | `client_ollama.go` | OpenAI transport with `provider = ollama`; own tool-results path | `chat` / `tool_gen` | `ollama` |
| xAI | `client_xai.go` | chat, tools, tool-results (streaming delegates — no second Track) | `chat` / `tool_gen` | `xai` |
| OpenRouter | `client_openrouter.go` | chat, tools, and stream-end | `chat` / `tool_gen` | `openrouter` |
| OpenAI-compatible | `client_openai_compat.go` | `executeChat` funnel + `consumeStream` end | `chat` / `tool_gen` | `dashscope`, `meta`, `moonshot` |
| Meta Responses | `client_meta_responses.go` | after reply decode | `tool_gen` | `meta` |
| Meta grounded search | `client_openai_compat_grounding.go` | after result assembly | `grounded_search` | `meta` |
| Gemini | `client_gemini.go`, `client_gemini_tools.go` | after `GeminiResponse` parse | `chat` / `tool_gen` | `gemini` |
| Gemini stream | `client_gemini_streaming.go` | once after stream ends, from last `usageMetadata` | `chat` | `gemini` |

### Rules producers must keep

- **Track once per billed request.** A retry that reached the vendor is a second
  billed request and gets its own Track; a streamed delta is not.
- **Track at the funnel where one exists** (`executeChat`, `completeNonStreaming`),
  so a new caller cannot forget.
- **Gemini output = candidates + thoughts** (`geminiOutputTokens`); thinking
  tokens are billed as output.
- **Provider id comes from the `Provider` constant**, never a literal.

### Not metered

| Client | Why |
|--------|-----|
| `claude_cli_client.go` | `claudeCLIResponse` decodes no token counts |
| `codex_cli_client.go`, `codex_exec_client.go` | CLI event stream carries no usage |

These are wiring-ready: give the decoder token counts and one `trackUsage` call
completes them.

## UI consumer wiring

**File:** `cmd/nerd/ui/usage_page.go`

- `NewUsagePageModel(tracker, styles)`  
- `UpdateContent` → `tracker.Stats()`  
- Tables: Provider, Model, Shard Type, Operation  
- Missing: Session table, Cost column, Events  

Nil tracker message: `"Usage tracking not available."`

## Fact-flow relationship (non-interference)

```
user input
  → perception (intent / LLM)
       │
       ├─ (metering side channel) Track if context has tracker
       │
  → user_intent facts
  → kernel next_action
  → VirtualStore / shards
  → articulation
```

Usage never injects `user_intent`, never queries the kernel, never routes actions.

## Wiring checklist for new features

| Change | Must also |
|--------|-----------|
| New LLM client | Call `FromContext` + `Track` on success with real token counts |
| New CLI command using LLM | `NewContext` after Cortex boot |
| New shard spawn path | Preserve or re-apply `WithShardContext` |
| New embedding path | Track with operation `"embedding"` if tokens known |
| Move workspace root | Ensure tracker recreated for new path (no global) |

## Dormant / partial hooks (do not delete casually)

| Hook | Status |
|------|--------|
| `UsageEvent` / Events slice | Live bounded ring, opt-in via `WithEventLog` |
| `Cost` field | Populated from `pricing.go`; `UnpricedTokens` covers table misses |
| `autoSaveTimer` | Live, cancelable, re-arms after a failed flush |
| Chat vs Cortex tracker | Unified: both call `usage.Shared`, refcounted, last `Close` wins |
| `LineHeader`-style reserved values | none remaining in this package |
