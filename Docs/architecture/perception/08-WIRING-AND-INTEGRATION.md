# 08 — Wiring and Integration (perception)

## Compatible and Responses transport uncertainty

The compatible Chat, shared non-streaming Responses and grounded-search HTTP
error branches notify the existing `usage.IncompleteObserver` chain before
retry/cancellation. Broker and enclosing mission captures retain that signal
alongside the later successful report; no synthetic usage is added. The normal
single-key factory paths for Chat, search-enabled Meta completion and explicit
grounded search are controlled in
`internal/campaign/recurse_mission_cross_loss_test.go:23`. Shared OpenAI and
key-pool wrappers retain their existing routing and error behavior.

## Anthropic lost-response uncertainty

`sendMessages` (`internal/perception/anthropic_transport.go:147`) notifies the
existing incomplete-observer chain after `httpClient.Do` fails. The signal
reaches enclosing broker and mission captures even if that inner retry loop
later returns a decoded or streamed success. Counts continue through their
existing decode/stream report sites; no synthetic usage report is added. The
single-key production factory control is
`internal/campaign/recurse_mission_lost_response_test.go:21`; key-pool failure
and retry behavior retains its existing transport/router path.

## Non-streaming Anthropic accounting

`completeText` and `completeTools` pass their existing operation category through
`exchange` and `postMessages` to `postMessagesOnce`
(`internal/perception/anthropic_transport.go:257`). Each decoded attempt reports
before validation to the tracker and nested observers, including the broker
and mission meter. Missing metadata and send/read/decode failure notify those
same captures. Exchange still merges accepted continuation content and usage;
its callers no longer observe that aggregate. Streaming uses its separate
`sendMessages`/stream-reader/terminal-report path unchanged. The real-kernel
mission control is `internal/campaign/recurse_mission_anthropic_usage_test.go:20`.

> Last verified: **2026-07-13**  
> How perception is **registered and called** at runtime.

## Boot sequence (typical interactive)

From CLI chat boot (`cmd/nerd/chat/session_boot.go` pattern — see CLI corpus for exact steps):

1. Load `UserConfig` / workspace `.nerd/`.  
2. `perception.DetectProvider` or config-driven `ProviderConfig`.  
3. `NewClientFromConfig` → main `LLMClient`.  
4. Optional `NewClassificationClientFromConfig` → dedicated classifier client.  
5. Optional `NewWorkerClientFromUserConfig`.  
6. Wrap shard-facing client with `NewTracingLLMClient(store)`.  
7. `NewUnderstandingTransducer(classificationOrMainClient)`.  
8. `SetKernel` / `SetPromptAssembler` / `SetStrategicContext` when available.  
9. `InitSharedSemanticClassifier(kernel, cfg)` if embeddings enabled.  
10. `SharedTaxonomy.SetWorkspace(root)` for learned taxonomy paths.  

If step 9 fails, interactive still runs **LLM-only** classification.

## Per-turn wiring (chat)

```
chat process input
  → transducer.ParseIntentWithContext(ctx, text, history)
  → Intent asserted / handed to session executor
  → kernel derives next_action
  → VirtualStore / shards use TracingLLMClient for model calls
  → articulation surfaces response
```

Routing facts asserted during Understand (when KernelAsserter present) are **retracted at turn start** by chat process (documented in `transducer_llm.go` comments).

## Auth command wiring

`cmd/nerd/cmd_auth.go` imports:

- `codenerd/internal/perception` — Claude/Codex client helpers  

Engines `claude-cli`, `codex-cli` are first-class in `NewClientFromConfig`.

## Campaign wiring

Campaigns construct/use perception clients for parallel specialist LLM traffic. Shared transport limits exist specifically so campaign concurrency is not serialized.

Assault verbs covered in taxonomy/corpus tests (`assault_verb_test.go`) and chat campaign assault path.

## Session executor wiring

Session executor holds a `Transducer` (interface). E2E tests install mocks implementing the full interface. Production installs `UnderstandingTransducer`.

## Classification client wiring contract

```
classClient, err := NewClassificationClientFromConfig(cfg)
if classClient == nil {
  // use main client for Understand
} else {
  // UnderstandingTransducer(classClient) preferred for hot path
}
```

Subscription engines always return nil classification client (no tiering).

For Meta API mode, `UserConfig.ReasoningEffort` is copied through
`ProviderConfig` into both main and classification clients. Classification still
sets `EnableThinking=false`, but a nonempty explicit Meta override takes
precedence. Worker and planner slots carry their own overrides and preserve
their endpoint/base-URL settings.

## Semantic classifier wiring contract

```
InitSharedSemanticClassifier(kernel, userCfg)
// later
CloseSharedSemanticClassifier() // shutdown
```

`matchVerbFromCorpus` and `ParseIntentWithContext` both consult `SharedSemanticClassifier` when non-nil.

## Piggyback / articulation boundary

- **Perception classification** emits Understanding / Intent.  
- **Articulation** owns PiggybackEnvelope for model **responses** during act/emit phases.  
- Clients may still enforce Piggyback schema when completing tool/agent turns (schema builders), but transducer JIT path **rejects** Piggyback-shaped classification prompts.

## Integration audit checklist

Before declaring a perception feature “unused”:

1. Grep `New*` constructors and interface implementers.  
2. Check chat boot + session executor fields.  
3. Check Mangle predicates asserted (`semantic_match`, `current_understanding`, `derived_*`).  
4. Check auth/campaign/cmd_direct_actions.  
5. Check e2e packages under `tests/e2e/perception_*`.  

## Partial / dormant wires to watch

| Wire | Risk |
|------|------|
| SharedSemanticClassifier nil | Silent loss of embedding path |
| Classification client nil on API providers without fast tier | Uses main model (cost/latency) |
| RoutingKernel nil | Routing copies LLM suggestions only |
| PromptAssembler not set | Embedded understanding prompt only |
| TraceStore nil | Tracing client may no-op store |

See also CLI [11-CROSS-SYSTEM-WIRING-JOURNAL.md](../cli/11-CROSS-SYSTEM-WIRING-JOURNAL.md) for binary-level boot narrative.


## Decoded retry uncertainty wiring

The decoded adapter's `reportUsage` calls `usage.NoteUnknownUsage` when token
metadata is absent. Nested captures reach both the broker's
`usage.IncompleteObserver` and the enclosing mission meter. The broker preserves
reported lower bounds as partial; the mission's existing kernel admission
then derives `/unknown_spend`. A successful answer does not erase missing usage.
This bounded path is locally verified in
`a6aaaf0f1cbc429dac34f57b3d92bba2486c8fca`; see GAP-PERCEPTION-RETRY-USAGE and
broker B12. Existing permission, provider selection and executive policy are
unchanged. Publication and live qualification remain separate.


## Accepted response read-error wiring

The compatible chat transport calls `usage.NoteUnknownUsage` on HTTP 200
body-read failure before retry or cancellation returns. Nested captures reach
both the existing broker incomplete observer and the mission meter. A later
successful report retains its counts but cannot clear uncertainty; kernel
admission stops the next paid phase. This path is locally verified in
`567d7926565626a24082a7416d11e4144fb03abe` (GAP-PERCEPTION-ACCEPTED-READ).
Existing retry, close, cancellation and executive policy statements are unchanged.
