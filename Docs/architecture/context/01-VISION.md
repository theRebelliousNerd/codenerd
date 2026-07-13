# 01 — Vision: Context Package

> Last verified against codebase: 2026-07-13  
> Package: `internal/context`  
> Status: Living vision document (target + realized trajectory)

## 1. Product problem

LLM coding agents fail long-horizon work for two opposite reasons:

1. **Context rot** — every turn appends surface text until the window is noise.  
2. **Context amnesia** — aggressive truncation drops the logical state that still matters.

codeNERD’s answer is **semantic compression with logic-directed activation**: keep the **Logical Twin** (kernel facts) dense, and treat natural language as a disposable interface.

## 2. Target architecture

```
┌─────────────────────────────────────────────────────────────┐
│  Durable truth: Mangle kernel (facts, permitted, goals)     │
└───────────────────────────┬─────────────────────────────────┘
                            │ Query / Assert
┌───────────────────────────▼─────────────────────────────────┐
│  internal/context                                           │
│  • ActivationEngine ranks facts for current intent          │
│  • Compressor manages window + rolling history              │
│  • Serializer emits Mangle context blocks                   │
│  • FeedbackStore learns predicate usefulness                │
└───────────────────────────┬─────────────────────────────────┘
                            │ GetContextString / scores
┌───────────────────────────▼─────────────────────────────────┐
│  Perception / Articulation / Prompt JIT                     │
│  LLM sees compressed logical state, not raw transcript      │
└─────────────────────────────────────────────────────────────┘
```

### Vision pillars

| Pillar | Target |
|--------|--------|
| **Atoms over text** | Surface responses never re-enter long-term context |
| **Spreading activation** | Intent energy selects related facts via graph + priorities |
| **Budgeted windows** | Explicit reserves; hard fail past total budget |
| **Kernel-derived inclusion** | Policy rules own relevance; Go executes budget math |
| **Observation masking** | Old turns keep reasoning atoms; drop verbose observations |
| **Learned usefulness** | Third feedback loop steers which predicates stay hot |
| **Campaign / issue awareness** | Long jobs keep phase/task/file energy without manual prompts |

## 3. Realized vs aspirational

| Pillar | Realized today | Aspiration |
|--------|----------------|------------|
| Atoms over text | `CompressedTurn` has no surface field used | Full parity: all chat paths never re-inject raw old text once compressed |
| Spreading activation | 9-component Go engine + graph rebuild | Primary scores from Mangle `context_score` / inclusion rules |
| Budgeted windows | TokenBudget + threshold compression | Provider-accurate tokenizers optional |
| Kernel inclusion | Wired with Go fallback | Fallback rare; rules cover edge cases |
| Observation masking | Age assert + simple summary path | Go always respects `should_mask_observation` |
| Feedback loop | SQLite store + score component | Continuous UI/manifest correlation |
| Multi-context | Campaign, issue, back-ref auto-refresh | Stronger SWE-bench / multi-issue isolation |

## 4. Non-goals

- Replacing vector retrieval for large codebases (`internal/retrieval` remains complementary).  
- Authorizing tool/file actions (kernel `permitted` only).  
- Storing full transcript forever inside compressor state.  
- Building client-app-specific context features into the core package.

## 5. Success metrics (operational)

| Metric | Signal |
|--------|--------|
| Compression ratio | `GetCompressionRatio` / rolling summary overall ratio |
| Budget utilization | `GetBudgetUtilization` / UI gauge |
| Hot fact quality | Activation logs + feedback helpful/noise rates |
| Inclusion source | Logs: kernel path vs Go fallback in `BuildContext` |
| Safety presence | Core section non-empty when kernel has permission facts |

## 6. North-star tie-in

- **LLM** invents summaries only when asked; preferred path is atom retention.  
- **Kernel** remains executive for inclusion predicates and permissions.  
- **JIT prompts** consume activation scores, not hardcoded context essays.
