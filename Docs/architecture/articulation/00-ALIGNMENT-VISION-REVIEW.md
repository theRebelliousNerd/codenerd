# 00 — Alignment / Vision Review: articulation

> Last verified: 2026-07-13  
> Scores are evidence-based against **code as it exists**, not aspirational decks.

## Scoring rubric

| Score | Meaning |
|------:|---------|
| 5 | Fully aligned; production path matches north star |
| 4 | Strong; minor drift or optional paths |
| 3 | Partial; main path works, gaps documented |
| 2 | Weak; code exists but poorly wired or contradictory |
| 1 | Misaligned or mostly aspirational |

## Dimension scores

| Dimension | Score | Evidence |
|-----------|------:|----------|
| **LLM creative / logic executive split** | **5** | Control packet carries `mangle_updates`, tool/knowledge requests, intent classification; surface is display-only. Kernel still owns permission and assert (`ApplyConstitutionalOverride` + session executor filtering). |
| **Piggyback dual-channel fidelity** | **5** | `PiggybackEnvelope` / `ControlPacket` in `protocol_types.go`; thought-first ordering mandated in `PiggybackProtocolSuffix` and struct field order (Bug #14). |
| **Robust transduction of messy LLM output** | **5** | Fallback chain: direct JSON → markdown fence → embedded candidates → plain/salvage (`emitter.go` `Process`). Decoy last-match-wins; depth/size caps in `json_scanner.go`. |
| **JIT-first prompt generation** | **4** | `PromptAssembler` prefers JIT (`AssembleSystemPrompt`); falls back to kernel template / embedded baseline / hard-coded templates. Env `USE_JIT_PROMPTS=false` disables. Legacy templates remain for emergency. |
| **Constitutional safety surface** | **4** | `applyCaps` strips bad mangle atoms (syntax, length, shell metacharacters). `ApplyConstitutionalOverride` rewrites surface and filters blocked atoms. Full `permitted(...)` is **outside** this package (session/core). |
| **Streaming UX without control leak** | **4** | `StreamParser` emits only `surface_response` string content. Full control parse happens after stream completes in chat helpers. Truncation salvage avoids dumping raw control JSON. |
| **Structured tool / knowledge channel** | **4** | `ToolRequest`, `KnowledgeRequest`, schemas in `schema.go`; session executor and chat knowledge paths consume them. Caps at 20 each. |
| **Observability** | **4** | Dedicated `logging.CategoryArticulation`; timers on Process/parse paths; `ProcessorStats`. Fallback logged as Error when unexpected. |
| **Test / adversarial coverage** | **4** | Rich unit, boundary, decoy, DOS, fuzz tests. StreamParser concurrency and end-to-end control→kernel assert coverage live mostly outside this package. |
| **Wiring completeness** | **5** | Booted in `internal/system/factory.go` and chat `session_boot.go`; used by session executor, system shards, perception schema/transducer, campaign JIT providers. |
| **Mangle-local logic** | **n/a** | No `.mg` in package; queries `shard_prompt_base`, `injectable_context`, `specialist_knowledge` via `KernelQuerier`. |
| **Docs / discoverability** | **4** | Package README slightly stale (structure list incomplete); this corpus is the living map. |

**Weighted overall: ~4.5 / 5** — mature production package, not a stub.

## North-star fit narrative

codeNERD’s inversion of control requires a **hard boundary** where natural language stops being executive. Articulation is that boundary on the **output** side (perception is the **input** side):

1. Models may **describe** actions in `surface_response`.  
2. Models may **propose** logic and tools only inside `control_packet`.  
3. Deterministic code validates, caps, and hands proposals to kernel / VirtualStore.  
4. System prompts that demand this protocol are assembled from JIT atoms + kernel facts, not ad-hoc chat prose growth (preferred path).

## Misalignment risks (do not over-score)

| Risk | Reality |
|------|---------|
| Fallback plain text | When Piggyback fails, surface becomes raw text and control is empty — creative path continues without executive atoms until next turn. |
| Legacy templates | Hard-coded coder/tester/reviewer/researcher strings still exist if JIT + baseline + kernel templates all fail. |
| Constitutional override thin | Override is string rewrite + atom filter; it does not re-run full policy derivation inside this package. |
| Schema strictness optional | Default processors are tolerant; strict unknown-field checks only when `RequireValidJSON`. |

## Verdict

Articulation is **aligned and load-bearing**. Treat changes as high-blast-radius: chat streaming, session tool loops, and kernel fact injection all depend on stable Piggyback shapes and parse confidence semantics.
