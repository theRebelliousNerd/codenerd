# OPEN QUESTIONS — Autopoiesis

> Last verified against codebase: **2026-07-13**  
> Only questions that remain real after reading the code

## Q1 — Should light generation paths remain at all?

`ExecuteAction` and chat `generate_tool` can create tools without full Ouroboros. Is the light path intentional for latency/diagnostics, or accidental dual maintenance?

**Impact:** safety depth, operator mental model.

## Q2 — What is the long-term sandbox?

Binary + policy + Thunderdome vs Yaegi-only vs OS containers/seccomp? Code currently mixes binary primary and Yaegi alternate without a single product policy.

## Q3 — How much internal Ouroboros Mangle state should surface?

Parent kernel gets durable outcomes. Should halt reasons, battle_hardened, or iteration counts be queryable session-wide for glass-box UX?

## Q4 — Who schedules persistent agents?

Autopoiesis writes `.nerd/agents` specs and memory. Is the executive owner shards, user-agent JIT, a future scheduler, or external ops?

## Q5 — SPL promotion authority?

AutoPromote at confidence 0.7 — is that constitutional enough for a logic-first agent, or must policy derive `permitted(promote_atom, …)`?

## Q6 — AllowExec default

Orchestrator builds Ouroboros with `AllowExec: true` while networking is false. Is exec required for useful tools (e.g. wrapping CLIs), and how does that interact with go_safety.mg?

## Q7 — Campaign coupling

Complexity analysis recommends campaigns but does not start them. Should autopoiesis assert a kernel fact (`needs_campaign`) instead of returning Go-only actions for chat to interpret?

## Q8 — Tool identity and capability naming

`normalizeCapabilityName(tool.Name)` equates name and capability. Will multi-capability tools need richer `tool_capability` fan-out as first-class design?

## Q9 — Offline / air-gapped compile

`go mod tidy` during compile assumes module access. What is the supported offline story for generated tools with non-stdlib imports?

## Q10 — Cross-repo (Vectryx) memory

Should successful tool schemas or learnings eventually consolidate into Vectryx, or remain workspace-local by north-star scope discipline?

---

When a question is decided, move the decision into IMPLEMENTED_SPEC / principles and delete the question.
