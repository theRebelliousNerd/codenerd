# transparency — Open Questions

> Last verified: 2026-07-13  
> Real design questions—not rhetorical.

## Q1 — Is ShardObserver still a product surface?

Glass Box `CategoryShard` already drives live TUI. Observer offers structured phase enums and active lists.

**Options**

1. Feed Observer from the same lifecycle points as Glass Box (keep both).  
2. Demote Observer to library-only / tests; remove from Manager status.  
3. Replace Glass Box shard lines with Observer→adapter bridge.

**Who decides:** UX + shards owners.

## Q2 — Should TransparencyManager own the buses?

Today Manager and buses are siblings. Unifying would simplify mental model but couples opt-in master toggle to always-on tools (must carefully preserve ToolEvent ungated).

**Default lean:** keep sibling ownership; document Track A/B forever.

## Q3 — Session vs durable config for `/transparency`

Does `/transparency on` persist to `.nerd/config.json`?

**Evidence lean:** runtime Manager mutation only—confirm or add explicit save path.

## Q4 — What is the source of truth for “why”?

| Path | Fidelity |
|------|----------|
| Heuristic `/why` + Explainer | Fast, may miss complex rules |
| Provenance `/explain` | Higher fidelity when enabled |

Should Explainer eventually consume only provenance trees? Should `/why` be an alias?

## Q5 — Safety classification: kernel-authored vs heuristic?

Should policy emits structured `SafetyViolationType` atoms, or remain string heuristics in Go?

**Tradeoff:** Mangle purity vs UX accuracy.

## Q6 — Drop policy: silent vs visible?

Silent drop protects latency. Visible “N events dropped” protects trust.

**Proposal:** Stats counter + occasional control-category notice under verbose only.

## Q7 — Category set frozen?

Is six categories enough? Need `category:memory`, `category:mcp`, `category:campaign`?

**Cost:** filters, docs, TUI icons.

## Q8 — Headless nerd run

Single-shot `nerd run` and campaign assault: which transparency channel is mandatory?

**Lean:** ToolEvent-equivalent logging always; Glass Box optional env flag.

## Q9 — Import of mangle from transparency

Explainer couples package to mangle types. Alternative: accept a narrow interface / DTO so mangle can evolve.

Worth an interface?

## Q10 — Concurrent SafetyReporter

Is multi-goroutine Report a real production path? If yes, mutex is mandatory; if no, document “chat UI thread only”.

---

Resolved answers should move into IMPLEMENTED_SPEC / principles and be deleted or struck here.
