# 01 — Vision: articulation

> Last verified: 2026-07-13  
> This is the **target architecture** for the package — grounded in what already ships, not a greenfield fantasy.

## Product role

Articulation is the **exit transduction layer** of the agent:

- Convert **LLM free text** into a **dual-channel envelope**: user prose + machine control.  
- Convert **kernel/session/world state** into **system prompts** that force that envelope (via JIT atoms when possible).  
- Keep **control data out of user eyes** and **untrusted mangle strings out of the kernel** until filtered.

Slogan (from package commentary): the **Corpus Callosum** between surface language and logic atoms.

## Target properties

### 1. Thought-first dual channel (non-negotiable)

```json
{
  "control_packet": { "...": "..." },
  "surface_response": "What the user sees"
}
```

- Control is fully formed **before** surface (Bug #14 — premature articulation).  
- If generation truncates mid-envelope, users see salvage text or a friendly cut-off notice — **never** raw `control_packet` JSON as chat.

### 2. Graceful degradation, never silent corruption

Parse pipeline prefers perfect JSON, then markdown fences, then embedded objects, then plain text / salvage.  
Confidence and `ParseMethod` are first-class so callers can decide trust:

| Method | Typical confidence |
|--------|-------------------:|
| `json` | 1.0 |
| `json_markdown` | 0.95 |
| `json_extracted` | 0.85 |
| `fallback` | 0.5 |

### 3. JIT-native system prompts

Preferred path:

```
PromptContext → CompilationContext → JITPromptCompiler.Compile
  → atoms under token budget → append Piggyback suffix if missing
```

Legacy assembly remains a **safety net**, not the design center.

### 4. Structured side effects instead of native function calling

`tool_requests` and `knowledge_requests` live in the control packet so:

- Grounding tools (e.g. Gemini search) coexist with codeNERD tools.  
- Tool IDs correlate results.  
- Debugging sees the full request list as data, not opaque provider tool protocol.

### 5. Feedback loop into context selection

`context_feedback` (helpfulness / noise / missing context) is part of the protocol so spreading-activation / retrieval scoring can learn. Downstream application of feedback is a consumer responsibility (session/store), not hidden inside the parser.

### 6. Safety is layered

| Layer | Owner |
|-------|--------|
| Size / spam caps | articulation `applyCaps` |
| Mangle syntax & shell-metachar filters | articulation `applyCaps` |
| Surface rewrite / blocked atoms | `ApplyConstitutionalOverride` |
| `permitted(...)` / VirtualStore | core / session / policy |

## Non-goals (for this package)

- **Not** a full TUI or streaming transport (chat owns channels).  
- **Not** the JIT atom library or selection scoring (`internal/prompt`).  
- **Not** NL intent parsing (perception).  
- **Not** durable memory store (store / learning paths consume `memory_operations`).  
- **Not** Mangle policy corpus (no local `.mg`).

## Success criteria

1. Every user-facing LLM path processes responses through `ResponseProcessor` or `ProcessLLMResponse*`.  
2. Every system prompt for Piggyback-required shards includes protocol instructions (JIT or suffix).  
3. Truncated / partial envelopes never dump control JSON to operators.  
4. Adversarial decoy envelopes lose to last-match-wins extraction.  
5. Stats and category logs make fallback rate observable in production sessions.

## Evolution direction

- Reduce reliance on hard-coded fallback templates.  
- Tighten wiring so memory ops and context feedback always hit durable paths when present.  
- Keep schema and Go types in lockstep (`schema.go` ↔ `protocol_types.go`).  
- Prefer expanding **tests at the package boundary** when adding control fields rather than only e2e TUI tests.
