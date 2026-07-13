# 00 — Alignment & Vision Review (perception)

> Last verified: **2026-07-13**  
> Scored against codeNERD north star with **code evidence**.

Scoring: **5** = fully aligned in code · **3** = partial / mixed · **1** = weak or aspirational only.

---

## Dimensions

| Dimension | Score | Evidence |
|-----------|------:|----------|
| **LLM as creative center** | 5 | `LLMTransducer.Understand` owns semantic interpretation; action/domain/scope from model JSON (`transducer_llm.go`). |
| **Logic as executive** | 5 | `deriveRouting` queries Mangle affinities; may override `SuggestedApproach`; `Intent.ToFact` feeds kernel; philosophy comments “Harness determines”. |
| **Constitutional safety** | 3 | Fact sanitization + degraded intent contracts exist; `permitted(...)` not local—kernel owns deny-by-default. Clients do not gate tool calls. |
| **JIT prompt atoms** | 4 | `PromptAssembler` + `getUnderstandingPrompt` with contract validation; embedded fallback when JIT fails/invalid. |
| **Transduction interface** | 5 | NL → `Understanding` → `Intent` → `user_intent` atoms is the package’s primary job. |
| **Neuro-symbolic grounding** | 4 | SemanticClassifier injects `semantic_match`; taxonomy Mangle inference; graceful offline when embed fails. |
| **Multi-provider abstraction** | 5 | Unified factory + engines; `LLMClient` alias; Tracing wrapper. |
| **Long-horizon / learning** | 4 | ConsolidationWorker + learned corpus + critic exemplar facts; queue may drop under load. |
| **Observability** | 4 | Dense Perception logs, metrics map, traces; not full distributed tracing. |
| **Wiring completeness** | 4 | Wired through chat/session/auth/campaign; some optional paths (SharedSemanticClassifier nil) degrade silently. |
| **Test / adversarial hardness** | 4 | Unit + break tests + e2e perception contracts; live provider tests gated. |
| **Scope discipline** | 5 | General multi-provider perception only; no app-specific product surface. |

**Mean (heuristic): ~4.3 / 5** — strong north-star fit; residual risk in safety-at-edge (client trust) and silent degradation paths.

---

## Strengths

1. Clear **describe vs decide** split in types and routing.  
2. Classification **model tiering** fixes critical-path latency bug.  
3. **TransientFailure** prevents laundering 503s as user ambiguity.  
4. **Injection** and JSON extraction hardness covered by break tests.  
5. **xaioauth** cleanly separated from API-key xAI.

## Risks to watch

1. Dual paths (LLM-first vs corpus/taxonomy) can diverge if not both exercised in boot.  
2. JIT prompt contract is string-snippet fragile.  
3. Provider capability surface is uneven (tools/schema/stream).  
4. Shared globals (`SharedTaxonomy`, `SharedSemanticClassifier`) complicate tests and multi-workspace.

## Alignment verdict

**Production-aligned perception stack.** Continue tightening wiring audits when classifier is nil at boot and when classification client is nil (falls back to main model).
