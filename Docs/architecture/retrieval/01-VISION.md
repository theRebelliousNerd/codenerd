# retrieval — Vision

> Last verified: **2026-07-13**  
> Status: Target architecture (aspirational vs code; see gap doc for delta)

## Product purpose

When a user (or SWE-bench-style harness) describes a bug, feature, or review target, the agent must **discover the right files quickly** without:

- Dumping the whole repo into the LLM context
- Asking the model to invent paths without evidence
- Paying vector search cost before cheap lexical filters

`internal/retrieval` is the **cheap first stage** of that funnel: lexical + structural signals that become **Mangle facts**, which the executive kernel and context activation layer use to budget attention.

## Target experience

```
User: "ValueError in serialize_user() when email is empty — see accounts/models.py"
     │
     ▼
[retrieval] extract keywords + mentioned paths
[retrieval] sparse scan ranks candidates in <N seconds
[retrieval] tiered pack: mentioned → keyword → importers → semantic peers
     │
     ▼
EDB: issue_text, issue_keyword, file_mentioned,
     keyword_hit, candidate_file, tiered_context_file, issue_context
     │
     ▼
context activation boosts those files
JIT prompt atoms select issue-aware behavior
VirtualStore actions stay under permitted(...)
```

## Architectural vision

### Stages (funnel)

| Stage | Mechanism | Cost | Authority |
|-------|-----------|------|-----------|
| 0 | Mentioned paths | Free | Highest (Tier 1) |
| 1 | Weighted keyword sparse scan | Low | High (Tier 2) |
| 2 | Language-aware import/graph expand | Medium | Medium (Tier 3) |
| 3 | Embedding / corpus similarity | Higher | Supportive (Tier 4) |
| 4 | Kernel policy + activation | Deterministic | Executive |

Stages 0–3 are **retrieval**; stage 4 is **core/context**. Retrieval proposes; logic disposes.

### Non-goals

- Becoming a full code search product UI
- Replacing LSP / CodeDOM for exact navigation
- Embedding model training or corpus build (lives in embedding/store)
- Authoring prompt prose (JIT atoms live under `internal/prompt/atoms/`)

### Multi-language target

| Language family | Extract | T3 graph | Notes |
|-----------------|---------|----------|-------|
| Python | Strong today | Implemented | SWE-bench heritage |
| Go | Path + symbols partial | Target | `import` / package dir |
| TS/JS | Path partial | Target | module resolvers |
| Rust/Java | Path only | Later | optional |

### Observability vision

- Structured log events: keyword count, hit count, tier counts, elapsed, cancel reason
- Optional glass-box events for “why this file was selected”
- Metrics: p50/p95 search latency, cache hit rate, files walked

### Safety vision

- Read-only filesystem contract
- Explicit exclude globs + max file size + max total bytes scanned
- Respect workspace root (no path escape outside `workDir` when resolving imports)
- Never emit write/actions; only facts and data structures

## Success criteria

1. **Issue verbs always seed** extract + sparse candidate facts into the kernel.
2. **`BuildContext` (or equivalent) is the single assembly path** used by chat/session, not only tests.
3. **T4 calls embedding** when engine available; degrades to symbol heuristic otherwise.
4. **Comments match implementation** (native scan or real `rg` — pick one).
5. **Large-repo p95** under configured timeout with partial ranked results, not hang.

## Relationship to north star

- **Creative center (LLM):** may refine issue understanding; does not decide FS authority alone.
- **Executive (Mangle):** ranks policy, permits tools, selects next actions using retrieval-derived facts.
- **Transduction:** this package’s primary permanent job.
