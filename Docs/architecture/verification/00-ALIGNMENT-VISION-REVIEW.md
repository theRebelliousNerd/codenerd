# verification — Alignment / Vision Review

> Last verified: **2026-07-13**  
> Package: `internal/verification/`  
> Scores: 1–5 (5 = strongly aligned with codeNERD north star)

## North star reminder

- **LLM = creative center**; **logic / deterministic control = executive**  
- Constitutional safety: `permitted(...)`, default deny (kernel)  
- JIT prompt atoms for new LLM-facing behavior  
- Wiring audit before declaring anything unused  

---

## Scored dimensions

| # | Dimension | Score | Evidence |
|---|-----------|------:|----------|
| 1 | Inversion of control (LLM creative / control executive) | **4** | Loop is Go-deterministic; LLM only *judges* and *suggests* shards/correctives. Executive decides retries, enrichment, escalation. |
| 2 | Separation from constitutional safety | **5** | Package never claims to be `permitted(...)`. Orthogonal: legality vs quality. |
| 3 | Quality enforcement vs corner-cutting | **4** | Explicit violation taxonomy + retry enrichment; package purpose matches slogan. |
| 4 | Deterministic fail-safe behavior | **2** | Fail-open on verify LLM errors and nil client (Success=true). Correct for availability, weak for “no shortcuts.” |
| 5 | JIT / prompt-atom discipline | **1** | Large inline system prompts in `verifyTask` and `selectBestShard` — not atoms under `internal/prompt/atoms/`. |
| 6 | Fact-flow / kernel integration | **3** | Caller injects ResultToFacts after verification; package does not assert facts itself. Persistence is store-side, not Mangle. |
| 7 | Shard / TaskExecutor modern path | **4** | Prefers `session.TaskExecutor`; normalizes intent verbs for JIT executor contract. |
| 8 | Closed-loop learning | **2** | Writes `task_verifications`; does not read stats/history to bias selection. |
| 9 | Corrective action depth | **3** | Decompose + tool generation real; research/docs mostly specialist fallback; web path removed. |
| 10 | Observability / escalation UX | **4** | Escalation formatting in chat; store persistence; warn/error logs on store/marshal. |
| 11 | Test grounding | **4** | Dense pure-function tests; constructor/edge paths; less live multi-attempt LLM IT. |
| 12 | Scope honesty / non-overreach | **5** | Single-purpose package; no product Vectryx terms; no fake Mangle surface. |

**Mean ≈ 3.4 / 5** — load-bearing production gate with clear architectural identity; weak on JIT prompts and fail-open quality, strong on purpose and wiring.

---

## What is aligned

1. **Post-act quality loop** sits after shard execution, not inside the LLM’s free narrative.  
2. **Escalation to human** (`ErrMaxRetriesExceeded`) respects default-deny of *accepting bad work*.  
3. **Mutation-only gating** in chat avoids latency tax on pure queries — pragmatic executive policy.  
4. **Review-aware prompts** prevent punishing reviewers for reporting broken code.  
5. **Intent normalization** is a real integration fix (documented failure mode of bare shard names).  

## What is misaligned or partial

1. **Fail-open verification** can accept mock-laden output when the judge LLM is down.  
2. **Inline prompts** violate repo contract “new LLM-facing behavior → prompt atoms.”  
3. **Learning store is a write sink** — north-star long-horizon memory is incomplete without read-back.  
4. **CorrectiveResearch/Docs** comments promise Context7/web; implementation prefers specialists only.  

## Verdict

Treat verification as a **realized executive control loop with known quality-vs-availability tradeoffs**. Do not document it as “planned.” Prioritize fail-mode policy and JIT-prompt extraction over greenfield rewrites.
