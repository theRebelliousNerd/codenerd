# articulation — Implemented Spec (Deep-Dive)

> Last verified against codebase: **2026-07-13**  
> Status: Living Reference Document  
> Language: Go  
> Module: `codenerd`  
> Primary sources: `internal/articulation/`  
> Scale: **8** non-test Go files ≈ **3,200** lines; **7** test files; **0** local `.mg`  

## 1. Overview

The articulation package is codeNERD’s **output transduction and prompt assembly boundary**. It sits opposite perception in the OODA fact-flow:

```
user input → perception → user_intent → kernel next_action
  → VirtualStore / shards / tools → LLM
  → articulation → surface (user) + control (kernel/tools/memory)
```

### What it does

1. **Piggyback Protocol** — Dual-channel steganographic envelope: human `surface_response` + machine `control_packet`.  
2. **Robust parsing** — Multi-stage recovery from messy LLM text (JSON, markdown fences, embedded objects, plain/salvage).  
3. **Streaming surface extraction** — `StreamParser` for TUI token streaming without control leaks.  
4. **Caps and filters** — Size limits and mangle-atom hygiene before trust.  
5. **Prompt assembly** — `PromptAssembler` bridges kernel facts + session context + optional JIT (`internal/prompt`) into system prompts that mandate Piggyback.  
6. **Schema** — JSON Schema strings for schema-capable LLM clients.  

### What it is not

- Not the JIT atom compiler (prompt package).  
- Not NL intent parsing (perception).  
- Not policy/`permitted` evaluation (core/policy).  
- Not the chat TUI (though chat is the largest consumer).  

### Architecture slogan (package)

> Corpus Callosum between surface text and logic atoms.

### High-level control flow

```
                    ┌─────────────────────┐
                    │  PromptAssembler    │◄── KernelQuerier / Session / Intent
                    │  (+ JIT optional)   │
                    └──────────┬──────────┘
                               │ system prompt
                               ▼
                            ┌─────┐
                            │ LLM │
                            └──┬──┘
                               │ raw text / chunks
              ┌────────────────┼────────────────┐
              ▼                                 ▼
       StreamParser                      ResponseProcessor
       (surface deltas)                  Process / applyCaps
              │                                 │
              ▼                                 ▼
         TUI stream                    ArticulationResult
                                       Surface + ControlPacket
                                               │
                         ┌─────────────────────┼─────────────────────┐
                         ▼                     ▼                     ▼
                      display            mangle/tools            memory/knowledge
                                         (callers)               feedback (callers)
```

---

## 2. Implementation status

| Component | Status | Notes |
|-----------|--------|-------|
| `PiggybackEnvelope` / control types | **Implemented** | `protocol_types.go` with tolerant unmarshals |
| JSON Schema constants | **Implemented** | `schema.go` full + simple |
| `ResponseProcessor` multi-stage parse | **Implemented** | `emitter.go` |
| Partial-envelope salvage | **Implemented** | looksLike / salvage / truncated message |
| `applyCaps` + mangle filters | **Implemented** | Security-relevant |
| `StreamParser` | **Implemented** | Surface-only progressive parse |
| `findJSONCandidates` scanner | **Implemented** | Depth/size caps |
| `ProcessLLMResponse*` helpers | **Implemented** | Shard/session convenience |
| `Emitter` stdout helpers | **Implemented** | CLI instruction path |
| `ApplyConstitutionalOverride` | **Implemented** | Thin surface rewrite + atom filter |
| Reasoning directives | **Implemented** | Full + short constants |
| `PromptAssembler` JIT bridge | **Implemented** | Default JIT on unless env false |
| Legacy / baseline / kernel templates | **Implemented** | Multi-level fallback |
| `PromptAssemblerAdapter` | **Implemented** | Perception import-cycle break |
| Kernel context sections | **Implemented** | `kernel_context.go` |
| Local Mangle corpus | **N/A** | Queries only |
| End-to-end memory_ops application | **Partial** | Types yes; durable apply outside |
| context_feedback learning loop | **Partial** | Types yes; scoring outside |
| Stats → glass-box metrics | **Partial** | In-memory only |

**Overall:** living production package — **not** pre-implementation. Heuristic completeness of package-local design: **~90%**; system-wide “every control field always effects world state”: **lower**, depending on callers.

---

## 3. Source inventory

### 3.1 Package layout

```
internal/articulation/
  README.md                      # package overview (may lag this corpus)
  protocol_types.go              # envelope + control types
  schema.go                      # JSON Schema strings
  emitter.go                     # processor, emitter, override, ProcessLLM*
  json_scanner.go                # findJSONCandidates
  stream_parser.go               # progressive surface parse
  prompt_assembler.go            # system prompt assembly
  prompt_assembler_adapter.go    # adapter + map DI
  kernel_context.go              # injectable/specialist sections
  *_test.go                      # 7 test files
```

### 3.2 Non-test sources (size / role)

| Path | ≈ Lines | Purpose |
|------|--------:|---------|
| `prompt_assembler.go` | 1165 | JIT/legacy assembly, session/intent formatting, Piggyback suffix, fallback templates |
| `emitter.go` | 1103 | ResponseProcessor, caps, salvage, Emitter, constitutional override, convenience process API |
| `schema.go` | 240 | `PiggybackEnvelopeSchema`, `SimpleEnvelopeSchema` |
| `protocol_types.go` | 239 | Types + UnmarshalJSON coercion |
| `kernel_context.go` | 128 | GetKernelContext / BuildContextSection |
| `stream_parser.go` | 109 | StreamParser |
| `prompt_assembler_adapter.go` | 108 | Adapter + mapToPromptContext |
| `json_scanner.go` | 105 | JSON candidate state machine |

### 3.3 Test sources

| Path | Focus |
|------|--------|
| `emitter_test.go` | Core parse matrix + fuzz |
| `emitter_boundary_test.go` | Adversarial + applyCaps |
| `emitter_extra_test.go` | Override + ProcessLLMResponse |
| `emitter_helpers_test.go` | Emitter helpers + truncation |
| `json_scanner_test.go` | Scanner limits + benchmarks |
| `prompt_assembler_test.go` | Assembly + JIT + races |
| `stream_parser_test.go` | Streaming surface |

---

## 4. Piggyback Protocol (deep dive)

### 4.1 Envelope shape

Canonical JSON (control **first** — Bug #14 thought-first / premature articulation fix):

```json
{
  "control_packet": {
    "intent_classification": {
      "category": "/mutation|/query|/instruction",
      "verb": "/action_verb",
      "target": "...",
      "constraint": "...",
      "confidence": 0.0
    },
    "mangle_updates": ["predicate(arg).", "..."],
    "memory_operations": [{"op": "note", "key": "...", "value": "..."}],
    "self_correction": {"triggered": false, "hypothesis": "..."},
    "knowledge_requests": [],
    "reasoning_trace": "...",
    "context_feedback": {
      "overall_usefulness": 0.8,
      "helpful_facts": [],
      "noise_facts": [],
      "missing_context": ""
    },
    "tool_requests": []
  },
  "surface_response": "Human-readable response"
}
```

Go mirrors this in `PiggybackEnvelope` / `ControlPacket` (`protocol_types.go`).

### 4.2 Control field purposes

| Field | Consumer intent |
|-------|-----------------|
| `intent_classification` | Kernel / routing signal about what the model believes it is doing |
| `mangle_updates` | Propose facts for kernel assert (after filters + permission) |
| `memory_operations` | Cold storage directives (`promote_to_long_term`, `forget`, `store_vector`, `note`) |
| `self_correction` | Autopoiesis / retry hypothesis signal |
| `reasoning_trace` | Debug / learning; capped at 50KB |
| `knowledge_requests` | Specialist consultation or research re-entry |
| `context_feedback` | Training signal for context selection quality |
| `tool_requests` | Portable tool calls without monopolizing native FC |

### 4.3 Why structured tools in-band

Comments in `ControlPacket` document design goals:

- Coexist with Gemini grounding tools  
- Dynamic discovery including Ouroboros-generated tools  
- Unified multi-provider interface  
- Full debuggability of tool lists as data  

Session executor’s Piggyback++ path is the primary runtime consumer of `tool_requests`.

### 4.4 Schema enforcement levels

| Level | Mechanism |
|-------|-----------|
| Provider-side | `PiggybackEnvelopeSchema` via perception schema clients |
| Strict processor | `RequireValidJSON` + `DisallowUnknownFields` + recursive `schemaAllowedKeys` |
| Default processor | Tolerant decode + required non-empty surface |
| Soft enums | Custom Unmarshal for confidence strings, required flags, single-string mangle_updates |

---

## 5. ResponseProcessor pipeline (deep dive)

### 5.1 Entry

```go
rp := articulation.NewResponseProcessor()
// defaults: RequireValidJSON=false, AllowMarkdownWrapped=true,
// MaxSurfaceLength=50000, LogFallbackAsError=true
result, err := rp.Process(raw)
```

### 5.2 Stage ladder

| Stage | Method | Confidence | Notes |
|------:|--------|------------|-------|
| 1 | `parseJSON` | 1.0 | Trim; optional unknown-field check; streaming decoder from first `{` |
| 2 | `parseMarkdownWrappedJSON` | 0.95 | Strip ```json / ``` |
| 3 | `extractEmbeddedJSON` | 0.85 | Candidates + last-match-wins rich pass |
| 4 | fallback | 0.5 | If `!RequireValidJSON` |
| 5 | error | — | Strict only |

Validation on structured success:

- Always require non-empty `surface_response`.  
- Strict also requires intent `category` + `verb`; null `mangle_updates` → empty slice.  
- Self-correction trigger increments stats + warning.

### 5.3 Fallback salvage logic

When stage 4 runs:

1. If text **looks like** partial envelope (`{` or fence + control field names):  
   - Try extract `surface_response` with escape-aware scan.  
   - Else friendly truncation message, optionally quoting salvaged `reasoning_trace` (≤600 chars).  
2. Else use raw trimmed text as surface with warning.  
3. Empty control packet.  
4. Diagnostic Error log with joined parse errors + preview (unless quiet settings).

This is the primary defense against the historical UX bug where users saw bare `control_packet` JSON.

### 5.4 applyCaps (post-process)

Runs on every successful `Process` return:

| Cap | Limit |
|-----|------:|
| Surface | `MaxSurfaceLength` (default 50k) + `\n\n[TRUNCATED]` |
| Mangle updates count | 2000 |
| Single mangle update | 1000 chars + syntax/metachar filters |
| Memory ops | 500 |
| Reasoning trace | 50_000 |
| Tool requests | 20 |
| Knowledge requests | 20 |

Mangle syntax filter:

- must contain `(`  
- must end with `.`  
- reject `` `$;| ``  
- skip empties  

Warnings accumulate for each skip/truncation.

### 5.5 Convenience wrappers

| API | Use case |
|-----|----------|
| `ProcessLLMResponse` | General shards/session; error log on fallback; Control only if not fallback |
| `ProcessLLMResponseAllowPlain` | Intentional plain text ok (no error log) |
| `MustExtractSurface` / `ExtractSurfaceOnly` | Display-only |

**Contract:** callers that need tools/mangle must check `Control != nil` / `ParseMethod != "fallback"`.

### 5.6 Embedded extraction algorithm

```
candidates = findJSONCandidates(s)
// pass 1: reverse order, require both surface_response and control_packet keys
// pass 2: reverse order, other candidates
// first successful parseJSON wins
```

Scanner properties (`json_scanner.go`):

- Byte state machine for `{` `}` and string escapes  
- Depth circuit breaker at 200  
- Candidate max 5MB  
- Prefer retaining surface-bearing candidates; cap noise objects at 50  

---

## 6. StreamParser (deep dive)

### 6.1 Role

Chat streaming (`helpers_articulation.go`) feeds tokens into `StreamParser.ProcessChunk` and only forwards returned deltas to the UI. Full buffer is re-parsed with `ResponseProcessor` at end for control.

### 6.2 Behavior

1. Buffer all chunks.  
2. Search `"surface_response"` → `:` → opening `"`.  
3. Emit unescaped body until closing `"`.  
4. Escapes: `\n` `\r` `\t` `\"` `\\`.  
5. `GetFullBuffer()` for final parse.

### 6.3 Limits

- Not concurrent-safe.  
- Does not stream control fields.  
- If surface key never appears, deltas stay empty until final parse.

---

## 7. PromptAssembler (deep dive)

### 7.1 Construction

| Constructor | Behavior |
|-------------|----------|
| `NewPromptAssembler(kernel)` | Requires non-nil querier; `useJIT` from `USE_JIT_PROMPTS != "false"` |
| `NewPromptAssemblerWithJIT(kernel, jit)` | Enables JIT iff compiler non-nil |
| Factory path | `NewPromptAssembler` + `SetJITCompiler` + `SetJITBudgets` + `EnableJIT` |

### 7.2 AssembleSystemPrompt

Accepts:

- `*PromptContext`  
- `map[string]any` (via `mapToPromptContext` — autopoiesis/DI)  

Flow:

```
if useJIT && compiler:
  Compile(toCompilationContext(pc))
  if ok:
    append PiggybackProtocolSuffix if required keys missing
    return
  else:
    AssertFacts jit_fallback(...); warn; fall through

legacy:
  query shard_prompt_base OR AssembleEmbeddedBaselinePrompt OR getFallbackTemplate
  + injectable_context atoms (sorted)
  + session section + intent section
  + Piggyback suffix if usedLegacyTemplate && not structured-only
```

### 7.3 CompilationContext bridging

Notable mappings (see `toCompilationContext`):

- Stable shard ID stripping for `coder-123` → `coder`  
- Dream / TDD repair / active modes  
- Campaign phase/name, large refactor, high churn, security flags  
- ExtraContext tags for build/init/northstar/ouroboros/frameworks/language  
- Semantic query from intent target or override  
- Force `/mangle` language for autopoiesis/legislator/mangle_* shards  
- Tighter budgets for legislator / mangle_repair  

### 7.4 Kernel queries

| Query | Meaning |
|-------|---------|
| `shard_prompt_base` | Match type → template string; multi-match → alphabetical first |
| `injectable_context(id)` + `*` + `/_all` | Context atoms, deduped, sorted |
| `specialist_knowledge` | Used by `BuildContextSection` / specialist formatting |

### 7.5 Session section content (legacy path)

When JIT does not run, `buildSessionContext` injects capped lists (typically max 20) for:

- Dream mode banner  
- Diagnostics, failing tests, findings, reflection hits  
- Impacted files, git context, campaign, prior shard outputs  
- Recent actions, knowledge atoms, specialist hints  
- Available tools, blocked actions, safety warnings  
- Compressed history if &lt; 1500 chars  

### 7.6 PiggybackProtocolSuffix

Large mandatory instruction block defining full JSON shape, thought-first rule, mangle format examples, knowledge request guidance, and “emit empty values for unused fields.”

Skipped when `prompt.IsStructuredOutputOnly` or shard type is perception/perception_firewall.

### 7.7 Adapter

`PromptAssemblerAdapter` exposes:

```go
AssembleSystemPrompt(ctx, shardID, shardType string) (string, error)
JITReady() bool
```

Pulls `types.GetSessionContext(ctx)` when present. Used by perception transducer and some shard injections to avoid import cycles.

---

## 8. Constitutional override

```go
ApplyConstitutionalOverride(envelope *PiggybackEnvelope, blocked []string, reason string) *ConstitutionalOverride
```

- No-op if no blocked list and empty reason.  
- If reason set: prepend `[SAFETY NOTICE: ...]` to surface; mutate envelope.  
- If blocked set: filter exact-match mangle update strings.  
- Returns audit struct with original surface, modified surface, reason, blocked atoms.  

**Session** uses this when mangle updates are blocked. Not a full policy engine.

---

## 9. Integration map (runtime)

### 9.1 Boot

`internal/system/factory.go`:

- Creates assembler, attaches JIT + budgets, injects into transducer (adapter), poiesis, system/campaign shards.  
- Cortex struct field `PromptAssembler *articulation.PromptAssembler`.

Chat `session_boot.go` mirrors for interactive sessions.

### 9.2 Chat

`helpers_articulation.go` is the richest consumer: streaming, thoughts, conversation context, full control extraction into `ArticulationOutput`.

### 9.3 Session executor

- Piggyback++ tools via `tool_requests`  
- Mangle updates with constitutional override  
- Self-correction / memory op handling in `processPiggybackControlPacket`  

### 9.4 Perception

- Schema constant loaded for structured completion  
- Type alias for envelope  
- Adapter for prompt assembly  

### 9.5 Shards / campaigns

- Planner, perception shard, legislator, mangle_repair, requirements interrogator  
- Campaign JIT providers assemble phase prompts  

### 9.6 CLI instruction

`cmd_instruction.go` uses `Emitter` for dual-channel output formatting.

---

## 10. Observability summary

- Category: `articulation`  
- Timers on Process/parse/assemble/emit  
- Fallback Error logs with parse error chain  
- `ProcessorStats` per processor instance  
- JIT fallback facts best-effort  

See `11-OBSERVABILITY.md`.

---

## 11. Testing summary

Strong unit/adversarial coverage inside the package; fuzz on `Process`; scanner benchmarks; assembler race tests. StreamParser concurrency remains a documented test gap.

Commands:

```powershell
go test ./internal/articulation/...
go test ./internal/articulation/ -fuzz=FuzzResponseProcessor_Process -fuzztime=10s
```

See `10-TESTING-ALIGNMENT.md`.

---

## 12. Safety summary

| Layer | Package responsibility |
|-------|------------------------|
| Display isolation | Surface vs control; salvage |
| Resource limits | Depth, size, list caps |
| Mangle hygiene | Syntax/metachar/length |
| Constitutional helper | Surface notice + atom drop |
| Authorization | **Not here** — core/session |

See `09-SAFETY-AND-INVARIANTS.md`.

---

## 13. Gaps pointer

Honest incompleteness lives in:

- `03-GAP-ANALYSIS.md` — matrix  
- `TODO.md` — backlog  
- `OPEN-QUESTIONS.md` — design questions  
- `12-FAILURE-MODES.md` — operational modes  

Do **not** interpret partial caller wiring as “articulation package is unimplemented.”

---

## 14. Related documents in this corpus

| Doc | Content |
|-----|---------|
| `README.md` | Map + verify commands |
| `00-ALIGNMENT-VISION-REVIEW.md` | North-star scores |
| `01-VISION.md` | Target vision |
| `02-CURRENT-STATE.md` | Inventory |
| `04-ARCHITECTURAL-PRINCIPLES.md` | Binding principles |
| `05-INTERNAL-ARCHITECTURE.md` | Diagrams / state machines |
| `06-PUBLIC-API-AND-TYPES.md` | Export reference |
| `07-DEPENDENCY-MAP.md` | Importers |
| `08-WIRING-AND-INTEGRATION.md` | Boot/call sites |

---

## 15. Change discipline

When editing articulation:

1. Prefer additive control fields with caps + schema + suffix + tests.  
2. Never break ParseMethod string semantics without multi-package update.  
3. Audit reverse importers (`rg codenerd/internal/articulation`).  
4. Do not grow hard-coded templates when atoms can express the behavior.  
5. Run `go test ./internal/articulation/...` before handoff.  

---

## 16. Fact-flow reminder

```
user_intent → kernel → next_action → VirtualStore
  → (tools/shards) → LLM
  → articulation.Process* → surface to human
                         → control to executive layers
```

Logic determines reality; articulation ensures the model’s **description** is separated from its **proposed** logic and tool effects until deterministic layers accept them.
