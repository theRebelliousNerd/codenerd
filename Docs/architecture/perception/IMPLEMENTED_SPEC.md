# perception — Implemented Spec (Deep-Dive)

> Last verified against codebase: **2026-07-13**  
> Status: Living Reference Document  
> Language: Go  
> Primary sources: `internal/perception/`, `internal/perception/xaioauth/`  
> Scale (approx): **~50** non-test Go files in root package + **13** xaioauth sources; **~48** package tests + xaioauth tests; **0** local `.mg` (taxonomy consumes embedded core schemas)

---

## 1. Overview

`internal/perception` is the **ingress sensory system** for codeNERD. It converts natural language into:

1. **Legacy-compatible `Intent`** (`Category`, `Verb`, `Target`, `Constraint`, …) asserted as `user_intent(...)` Mangle facts.
2. **Rich `Understanding`** (semantic/action/domain/scope/signals/routing) for harness-side routing derivation.
3. **Provider-agnostic `LLMClient`** implementations used by chat, shards, campaigns, and tools.

It is **not** the executive. The Mangle kernel still decides `next_action` / `permitted(...)`. Perception supplies structured description plus optional routing facts; the harness may override LLM suggestions.

### Key characteristics

| Property | Value |
|----------|-------|
| Canonical NL path | `UnderstandingTransducer` → `LLMTransducer.Understand` |
| Fallback / dual path | Regex candidates + `TaxonomyEngine` + `SemanticClassifier` (verb corpus) |
| Fact emission | `Intent.ToFact()` → `user_intent(/current_intent, Cat, Verb, Target, Constraint)` |
| Semantic path | Embed query → dual corpus search → `semantic_match` kernel facts |
| Client factory | Config-first (`DetectProvider` / `NewClientFromConfig`) + env fallback |
| Engines | `api`, `claude-cli`, `codex-cli`, `xai-oauth` |
| API providers | zai, anthropic, openai, gemini, xai, openrouter, ollama |
| Classification tier | `NewClassificationClientFromConfig` (fast models; never inherits main model by default) |
| Philosophy | `LLM describes → Harness determines` (`transducer_llm.go`, `understanding.go`) |
| Logging category | `logging.CategoryPerception` |

### High-level control flow

```
User NL
   │
   ├─ (chat / session) UnderstandingTransducer.ParseIntentWithContext
   │       │
   │       ├─ SharedSemanticClassifier.Classify ──► semantic_match facts + exemplars
   │       ├─ LLMTransducer.Understand (classification prompt + CompleteWithSystem)
   │       │       ├─ parse UnderstandingEnvelope / Understanding JSON
   │       │       └─ deriveRouting via RoutingKernel (Mangle affinities)
   │       └─ understandingToIntent ──► Intent{Verb, Category, Target, …}
   │
   ├─ (legacy / corpus path) matchVerbFromCorpus
   │       ├─ getRegexCandidates (max 2000 chars)
   │       ├─ SemanticClassifier.Classify
   │       └─ TaxonomyEngine.ClassifyInput
   │
   └─ Intent.ToFact() → kernel user_intent → next_action / VirtualStore
```

Fact-flow (always):

```
user input → perception → user_intent → kernel next_action
  → VirtualStore / shards / tools → articulation → TUI/stdout
```

---

## 2. Implementation status

| Component | Status | Notes |
|-----------|--------|-------|
| `UnderstandingTransducer` / `LLMTransducer` | **Implemented** | Primary interactive path |
| `GeminiThinkingTransducer` | **Implemented** | Schema-capable thinking path |
| `SemanticClassifier` + dual corpus stores | **Implemented** | Graceful degradation if embed engine fails |
| `TaxonomyEngine` + verb corpus | **Implemented** | SharedTaxonomy init; learned_taxonomy.mg |
| Multi-provider LLM clients | **Implemented** | ZAI, Anthropic, OpenAI, Gemini, xAI, OpenRouter, Ollama |
| CLI engines (Claude / Codex) | **Implemented** | Subprocess backends |
| `xaioauth` SuperGrok OAuth | **Implemented** | Independent of API-key xAI client |
| Classification model tiering | **Implemented** | Haiku / flash-lite / gpt-4o-mini defaults |
| Worker secondary client | **Implemented** | `NewWorkerClientFromUserConfig` |
| TracingLLMClient + ConsolidationWorker | **Implemented** | Async learning from traces |
| Piggyback schema builders | **Implemented** | Canonical schema from articulation |
| Transient LLM failure signalling | **Implemented** | `ErrLLMUnavailable` + `Intent.TransientFailure` |
| Stability-bypass (reuse prior intent) | **Removed** | Documented as deliberate removal in adapter |
| `validate()` field checks on understand path | **Dead / skipped** | Comments: always discarded; deriveRouting uses suggestions + Mangle |
| Local `.mg` sources in package | **N/A** | Uses `core.GetDefaultContent` schemas + policy taxonomy files |
| Full CLI↔provider feature parity | **Partial** | Tools/streaming/schema differ by provider |
| JIT classification prompt | **Partial** | Falls back to embedded if contract invalid |

**Overall:** living production perception stack — **not** pre-implementation.

---

## 3. Source inventory

### 3.1 Package layout

```
internal/perception/
  transducer.go              # Intent, Transducer interface, verb corpus path
  transducer_llm.go          # LLMTransducer, JSON extract, routing derivation
  transducer_gemini.go       # GeminiThinkingTransducer + understanding schema
  understanding.go           # Understanding / Scope / Signals / Routing types
  understanding_adapter.go   # UnderstandingTransducer (canonical Transducer)
  semantic_classifier.go     # Vector classify + stores + SharedSemanticClassifier
  taxonomy.go                # TaxonomyEngine, SharedTaxonomy, ClassifyInput
  taxonomy_persistence.go    # TaxonomyStore DB hydration
  learning.go                # Critic / learned_exemplar extraction
  consolidation.go           # ConsolidationWorker async sleep-cycle
  client.go                  # Modularization marker
  client_types.go            # Provider configs, wire types, LLMClient alias
  client_factory.go          # DetectProvider, NewClientFrom*, tiering, worker
  client_schema.go           # Piggyback structured-output builders
  client_tool_helpers.go     # Shared tool-call helpers
  client_{zai,anthropic,openai,gemini,xai,openrouter,ollama}.go
  client_gemini_{files,streaming,tools}.go
  client_zai_{retry,streaming}.go
  claude_cli_client.go       # Claude Code CLI subprocess
  codex_cli_client.go        # Codex CLI
  codex_cli_probe.go         # Auth/health probe
  codex_exec_client.go       # Exec client alias
  tracing_client.go          # ReasoningTrace capture wrapper
  metrics.go                 # Process-wide LLMMetrics
  transport.go               # Shared HTTP transport (campaign parallelism)
  scanner_pool.go            # SSE scanner buffer pool
  utils.go / debug.go
  xaioauth/                  # SuperGrok OAuth LLM client subsystem
```

### 3.2 Top non-test sources (line counts ≈ from prior inventory)

| Path | Lines | Purpose |
|------|------:|---------|
| `semantic_classifier.go` | ~1254 | Dual-store vector classify, hydrate, fact inject |
| `client_zai.go` | ~1041 | Z.AI chat + structured + tools |
| `transducer_llm.go` | ~949 | LLM-first understand + routing |
| `tracing_client.go` | ~946 | Trace wrapper + tool result paths |
| `client_gemini.go` | ~944 | Gemini complete/schema/thinking |
| `taxonomy.go` | ~799 | Mangle taxonomy engine |
| `understanding_adapter.go` | ~725 | Canonical Transducer impl |
| `client_anthropic.go` | ~686 | Anthropic messages + cache |
| `client_types.go` | ~624 | Configs + request/response DTOs |
| `transducer.go` | ~616 | Intent, corpus matching, interfaces |
| `codex_cli_client.go` | ~596 | Codex subprocess |
| `claude_cli_client.go` | ~576 | Claude CLI subprocess |
| `client_openai.go` | ~505 | OpenAI-compatible chat |
| `client_openrouter.go` | ~441 | OpenRouter multi-model |
| `client_factory.go` | ~425 | Factory + classification tier |
| `client_gemini_files.go` | ~403 | Files API / cache content |
| `client_gemini_tools.go` | ~400 | Function calling |
| `learning.go` | ~400 | Meta-cognitive pattern extract |

### 3.3 Subpackage `xaioauth/`

Independent SuperGrok / X Premium+ OAuth backend (not the API-key `XAIClient`):

| File | Role |
|------|------|
| `doc.go` | Protocol notes (issuer, device code, scopes, model) |
| `config.go` | Defaults / engine constants |
| `errors.go` | Typed auth/rate/tier errors |
| `store.go` | `~/.nerd/xai_oauth.json` persistence |
| `grok_auth_import.go` | Import from `~/.grok/auth.json` |
| `auth_device.go` | OAuth 2.0 device-code login |
| `token.go` | EnsureValid + refresh |
| `transport.go` | Bearer HTTP |
| `client.go` | Client constructors |
| `chat.go` / `streaming.go` / `tools.go` | Complete surfaces |
| `probe.go` | `nerd auth` health probe |

---

## 4. Transducer deep dive

### 4.1 Interfaces

Defined in `transducer.go`:

```text
Transducer
  ParseIntent(ctx, input) (Intent, error)
  ParseIntentWithContext(ctx, input, history) (Intent, error)
  ParseIntentWithGCD(ctx, input, history, maxRetries) (Intent, []string, error)
  ResolveFocus(ctx, reference, candidates) (FocusResolution, error)
  SetPromptAssembler(pa)
  SetStrategicContext(context)

TransducerWithKernel extends Transducer with kernel integration for routing.
```

Canonical constructor: `NewUnderstandingTransducer(client LLMClient) Transducer`.  
If client implements `ThinkingProvider` and thinking is enabled → returns `*GeminiThinkingTransducer`.

### 4.2 `Intent` (kernel-facing)

| Field | Meaning |
|-------|---------|
| `Category` | `/query`, `/mutation`, `/instruction` |
| `Verb` | `/review`, `/fix`, `/explain`, `/assault`, … |
| `Target` / `Constraint` | Scope strings (sanitized for facts) |
| `Confidence` | 0.0–1.0 |
| `Response` | Surface NL (Piggyback-era / Understanding surface) |
| `MemoryOperations` | promote/forget ops |
| `TransientFailure` | True when LLM outage, not user ambiguity |
| `IsQuestion` | From Understanding signals / interrogative semantic types |

`Intent.ToFact()` builds:

```text
user_intent(/current_intent, Category, Verb, sanitize(Target), sanitize(Constraint))
```

`sanitizeFactArg` strips Mangle-significant characters and caps length (injection hardening; exercised in `break_test.go`).

### 4.3 `Understanding` (LLM-facing)

From `understanding.go`:

- **Core:** `PrimaryIntent`, `SemanticType`, `ActionType`, `Domain`, `Scope`, constraints, confidence  
- **Signals:** question / hypothetical / multi-step / negated / confirmation / urgency  
- **SuggestedApproach:** mode, primary/supporting shards, tools, context needs  
- **Routing:** harness-filled after `deriveRouting` (may override suggestions)  
- **SurfaceResponse:** user-visible text  

Envelope: `UnderstandingEnvelope{understanding, surface_response}`.

### 4.4 `ParseIntentWithContext` pipeline (`understanding_adapter.go`)

1. Lazy `initialize`: load JIT or embedded understanding prompt; construct `LLMTransducer`.  
2. Empty input → safe `/explain` intent.  
3. Truncate input > 50_000 chars.  
4. **No stability-bypass** — every turn classifies fresh (prior short-circuit removed; misclassified “thanks” as `/fix`).  
5. `SharedSemanticClassifier.Classify` (optional; failures non-fatal).  
6. `llmTransducer.Understand(...)` with history, matches, session ambient, strategic context.  
7. On error: return degraded `/explain` with **nil error** (contract); set `TransientFailure` if `errors.Is(err, ErrLLMUnavailable)`.  
8. Cache last understanding; `understandingToIntent` maps action→verb, semantic→category, carries `IsQuestion`.

### 4.5 `LLMTransducer.Understand` (`transducer_llm.go`)

1. `BuildPrompt`: ambient workspace, strategic context, high-sim semantic exemplars (>0.8), last ≤5 history turns (incl. thought summaries), current request.  
2. `client.CompleteWithSystem(ctx, systemPrompt, fullPrompt)`.  
3. `parseResponse` via `ExtractCleanJSON` (last valid balanced JSON object; cap 1000 candidates).  
4. Accept envelope **or** bare Understanding; `normalizeLLMFields` lowercases vocab fields.  
5. `deriveRouting` (not discarded validation).  
6. Dense timing logs: prompt / LLM / parse / route bottleneck labels.

### 4.6 Routing derivation

With `RoutingKernel`:

| Query predicate | Purpose |
|-----------------|---------|
| `mode_from_semantic` / `mode_from_action` / `mode_from_domain` | Mode selection |
| `shard_affinity_action` / `shard_affinity_domain` | Primary + supporting shards (score ≥50 supports) |
| `context_affinity_*` | Context priorities |
| tool affinity queries | Tool priorities + blocked tools from constraints |

Signal override: `IsHypothetical` → mode `dream`.

If kernel implements `KernelAsserter`, assert per turn:

- `current_understanding(Semantic, Action, Domain, ScopeLevel)`  
- `llm_suggested_mode`, `derived_mode`, `derived_primary_shard`  
- `derived_context_priority`, `derived_tool_priority`  

Comments document consumption by `perception_routing.mg`, context compilation, JIT selection; facts retracted at turn start by chat `process.go`.

Without kernel: routing copies LLM `SuggestedApproach`.

### 4.7 Action → verb mapping (selected)

| action_type | verb |
|-------------|------|
| implement | `/create` |
| modify | `/fix` |
| refactor | `/refactor` |
| verify | `/test` |
| attack | `/assault` |
| review (+ security domain) | `/security` else `/review` |
| investigate (+ testing) | `/debug` else `/analyze` |
| chat | `/converse` |
| unknown | `/explain` (warn) |

Category: instruction semantic → `/instruction`; mutating actions → `/mutation`; else `/query`.

### 4.8 Gemini thinking path

`transducer_gemini.go` embeds `UnderstandingTransducer`, re-implements `ParseIntentWithContext` for thinking-mode clients, uses flattened `understandingSchema` (Gemini max schema depth 6) with `CompleteWithSchema` when available.

### 4.9 Verb-corpus dual path (`transducer.go`)

`matchVerbFromCorpus`:

1. Regex candidates from `GetVerbCorpus()` (populated from SharedTaxonomy at init).  
2. Semantic classify → kernel facts (or seed fallback semantic facts).  
3. `SharedTaxonomy.ClassifyInput`.  
4. Default `/explain` @ 0.3 confidence.

Input to regex capped at 2000 chars (`maxRegexInputLen`).

---

## 5. Semantic classifier deep dive

### 5.1 Architecture

```
input
  → Embed (RETRIEVAL_QUERY / task-aware)
  → parallel Search: EmbeddedCorpusStore + LearnedCorpusStore
  → merge (learned boost +0.1, cap 1.0) + dedupe + MinSimilarity filter
  → inject semantic_match facts into core.Kernel
  → return []SemanticMatch for prompt exemplars / logs
```

### 5.2 Types & config

| Type | Role |
|------|------|
| `SemanticMatch` | text, verb, target, constraint, similarity, rank, source |
| `CorpusEntry` | corpus row |
| `EmbeddedCorpusStore` | baked intent definitions; optional SQLite cache |
| `LearnedCorpusStore` | dynamic patterns via store backend + autopoiesis |
| `SemanticConfig` | TopK=5, MinSimilarity=0.5, LearnedBoost=0.1, parallel=true |

### 5.3 Boot / hydrate

`NewSemanticClassifierFromConfig`:

- Builds `embedding.NewEngine` with `TaskType: RETRIEVAL_QUERY`.  
- On engine failure: returns classifier with nil stores/engine (**graceful degrade**).  
- Cache path: `.nerd/intent_embeddings.db` when taxonomy has workspace.  
- Hydrate from kernel intent definitions with **60s** timeout (`intentHydrateTimeout`), chunk size **32** (durable progress).  
- `InitSharedSemanticClassifier` / `CloseSharedSemanticClassifier` for process globals.

### 5.4 Classify limits

- Empty input → nil.  
- ClassifyWithoutInjection truncates at 32 KiB.  
- Embedding failures → empty matches (regex-only fallback), not hard error.  
- Parallel search uses `errgroup`; search errors warn only.

### 5.5 Learning loop

`learning.go` + `ConsolidationWorker`:

- Meta-cognitive critic prompt extracts `learned_exemplar(...)` facts from reasoning traces.  
- Async queue capacity 100; drops when full (never blocks chat).  
- Successful facts may call `SharedSemanticClassifier.AddLearnedPattern`.  
- Taxonomy persists learned rules to `.nerd/mangle/learned_taxonomy.mg` (not kernel `learned.mg`).

---

## 6. LLM clients deep dive

### 6.1 Interface surface

`LLMClient` is an alias of `types.LLMClient` (base: `Complete`, `CompleteWithSystem`).  
Concrete clients extend with optional capabilities:

| Capability | Typical methods | Providers (partial) |
|------------|-----------------|---------------------|
| Tools | `CompleteWithTools`, `CompleteWithToolResults` | most API clients |
| Streaming | `CompleteWithStreaming` / SSE callbacks | Gemini, ZAI, Claude CLI, xaioauth |
| Schema | `CompleteWithSchema`, `SchemaCapable` | Gemini, Claude CLI |
| Thinking | `ThinkingProvider` | Gemini |
| Semaphore | `DisableSemaphore` | ZAI (scheduler integration) |

Tool DTOs: package-local `ToolResult`; aliases for `ToolDefinition`, `ToolCall`, `LLMToolResponse`.

### 6.2 Factory & config precedence

```
.nerd/config.json (config is boss)
  engine: claude-cli | codex-cli | xai-oauth | api
  provider + api keys + model + classification_model + gemini/ollama/worker blocks
else env (order): ANTHROPIC → OPENAI → GEMINI → XAI → ZAI → OPENROUTER
```

Explicit provider without key → **error, no silent fallback** (`LoadConfigJSON`).

`NewClientFromConfig` engine switch before provider switch.  
`NewClassificationClientFromConfig`:

| Provider | Default classification model |
|----------|------------------------------|
| Anthropic | `claude-haiku-4-5` + system caching |
| Gemini | `gemini-3.1-flash-lite` |
| OpenAI | `gpt-4o-mini` |
| zai/xai/openrouter | only if `ClassificationModel` set; else nil → use main client |
| CLI engines | nil (no tiering) |

**Critical:** main `Model` is **not** applied to classification — historical bug put large models on every-turn critical path.

`NewWorkerClientFromUserConfig`: secondary LLM for shards/spawn (ollama/xai/openai/gemini).  
`NewImageClientFromUserConfig`: Gemini image models (not ollama worker).

### 6.3 Provider notes

| Provider | File(s) | Distinguishing behavior |
|----------|---------|-------------------------|
| **Z.AI** | `client_zai.go`, retry, streaming | Semaphore, structured json_object, thinking budget types |
| **Anthropic** | `client_anthropic.go` | System prompt caching (`EnableSystemCaching`), tool_use blocks |
| **OpenAI** | `client_openai.go` | Chat completions + tools + stream options |
| **Gemini** | gemini + files/tools/streaming | Thinking levels, Google Search, URL context, files API, token count, **ErrLLMUnavailable** on durable 5xx |
| **xAI** | `client_xai.go` | API-key path |
| **OpenRouter** | `client_openrouter.go` | Multi-model proxy + site headers |
| **Ollama** | `client_ollama.go` | Local OpenAI-compat `/v1` |
| **Claude CLI** | `claude_cli_client.go` | Subprocess, rate-limit typed errors, max turns |
| **Codex CLI** | `codex_cli_client.go`, probe, exec | Health probes map login/skill/rate failures |
| **xAI OAuth** | `xaioauth/*` | Device code, refresh, separate credential store |

### 6.4 Structured output / Piggyback schemas

`client_schema.go` parses **canonical** `articulation.PiggybackEnvelopeSchema` once (avoids schema drift).

| Builder | Enforcement style |
|---------|-------------------|
| `BuildZAIPiggybackEnvelopeSchema` | `json_object` only (prompt must carry schema) |
| `BuildOpenAIPiggybackEnvelopeSchema` | strict `json_schema` |
| `BuildGeminiPiggybackEnvelopeSchema` | raw schema map for responseJsonSchema |
| `BuildOpenRouterPiggybackEnvelopeSchema` | strict json_schema |

**Note:** classification uses **Understanding** contract, not Piggyback. JIT prompt validation **rejects** prompts containing `"control_packet"` or legacy `category/verb/target/constraint` fields.

### 6.5 Transport & streaming hygiene

- `transport.go`: process-wide `http.Transport` — MaxIdleConnsPerHost 64, MaxConnsPerHost 128, HTTP/2 preferred (campaign parallel LLM).  
- `scanner_pool.go`: 64 KiB pooled buffers for SSE scanners.  
- `utils.requiresJSONOutput`: prompt markers force JSON MIME/mode.

### 6.6 Tracing

`TracingLLMClient` wraps any `LLMClient`, sets shard context, stores `ReasoningTrace`, records `RecordLLMCall` metrics. Used so shard LLM traffic is attributable for learning/consolidation.

---

## 7. Taxonomy engine

`TaxonomyEngine` owns a dedicated `mangle.Engine` (not the Cortex kernel):

**Schema load order** (`taxonomySchemaFiles`):

1. `schemas_intent.mg`  
2. `core.DefaultIntentSchemaFiles()`  
3. `schemas_learning.mg` (fallback Decl if missing)  
4. `policy/taxonomy_qualifiers.mg`  
5. `policy/taxonomy_inference.mg`  

Schemas loaded **once** per engine (bug #18: prior per-call Reset+reload was hot-path cost).

Default verb data asserted as `verb_def` / `verb_synonym` / `verb_pattern`.  
Learned path: workspace `.nerd/mangle/learned_taxonomy.mg` only.  
`SetWorkspace` reloads learned file.  
`HydrateFromDB` via `TaxonomyStore`.  
`QueueForLearning` → ConsolidationWorker.

`SharedTaxonomy` init at package load; failures print CRITICAL but process continues with degraded corpus.

---

## 8. Integration map

| Consumer | How it uses perception |
|----------|------------------------|
| `cmd/nerd/chat` | Boots client + transducer; every user turn `ParseIntentWithContext` |
| `cmd/nerd` direct actions | May construct intents / clients for one-shot verbs |
| `cmd/nerd/cmd_auth.go` | Claude/Codex/Grok auth; xaioauth probe |
| `cmd/nerd/cmd_campaign.go` | LLM clients for campaign workers |
| `internal/session` | Executor holds Transducer / clients |
| `internal/core` / shards | Routing facts; TracingLLMClient for shard calls |
| `internal/articulation` | Piggyback types aliased; schema source of truth |
| `internal/embedding` | Query embeddings for SemanticClassifier |
| `internal/store` | Learned corpus backend |
| `internal/config` | Provider/engine/model keys |
| `tests/e2e/*perception*` | Contract, adversarial, stateful tests |
| `cmd/tools/verify_taxonomy` | Taxonomy verification tool |

### Downstream of Intent

```
Intent.ToFact → user_intent
  → policy derives next_action / permitted
  → VirtualStore executes
  → articulation emits surface
```

`IsQuestion` and routing facts influence whether work is delegated to shards vs answered directly (chat/session arbitration outside this package).

---

## 9. Safety surface (summary)

| Mechanism | Location |
|-----------|----------|
| Fact arg sanitization | `sanitizeFactArg` |
| Input length caps | 50k parse, 32k classify, 2k regex |
| Config is boss | no silent key fallback when provider set |
| Transient outage vs ambiguity | `TransientFailure` / `ErrLLMUnavailable` |
| Queue drop not block | ConsolidationWorker |
| Double-stop safe | ConsolidationWorker `stopOnce` |
| Verb corpus concurrency | `verbCorpusMu` RWMutex |
| Mangle injection tests | `break_test.go` |

Constitutional `permitted(...)` is **kernel policy**, not enforced inside perception clients (clients will call whatever the harness requests).

---

## 10. Observability (summary)

- `logging.Perception` / `PerceptionDebug` / `PerceptionError` with stage timers.  
- Bottleneck labels: `LLM_API`, `PROMPT_BUILD`, `PARSE_ROUTE`.  
- `LLMMetrics` process map keyed `shardCategory:shardType`.  
- Reasoning traces for post-hoc learning.  
- Auth probes for CLI/OAuth engines.

See [11-OBSERVABILITY.md](11-OBSERVABILITY.md).

---

## 11. Gaps pointer

Honest partials and non-gaps: [03-GAP-ANALYSIS.md](03-GAP-ANALYSIS.md).  
Prioritized backlog: [TODO.md](TODO.md).  
Open design questions: [OPEN-QUESTIONS.md](OPEN-QUESTIONS.md).

---

## 12. Verify commands

```powershell
go test ./internal/perception/...
go test ./internal/perception/xaioauth/...
go test ./tests/e2e/ -run 'Perception|Intent' -count=1
```

---

## 13. File role quick map (non-test)

| Area | Files |
|------|-------|
| Transduction | `transducer*.go`, `understanding*.go` |
| Semantic | `semantic_classifier.go` |
| Taxonomy / learn | `taxonomy*.go`, `learning.go`, `consolidation.go` |
| Clients | `client_*.go`, `claude_cli_*.go`, `codex_*.go` |
| OAuth | `xaioauth/*` |
| Cross-cut | `tracing_client.go`, `metrics.go`, `transport.go`, `scanner_pool.go`, `debug.go`, `utils.go` |

---

**End of IMPLEMENTED_SPEC** — living document; update when public contracts or factory behavior change.
