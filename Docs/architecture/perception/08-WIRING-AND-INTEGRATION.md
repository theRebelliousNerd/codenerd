# 08 — Wiring and Integration (perception)

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
- `codenerd/internal/perception/xaioauth` — Grok OAuth status / login / probe  

Engines `claude-cli`, `codex-cli`, `xai-oauth` are first-class in `NewClientFromConfig`.

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
