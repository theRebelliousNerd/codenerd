# OPEN QUESTIONS — Northstar

> Last verified against codebase: 2026-07-13

## Q1 — Which artifact is the long-term source of truth?

**Options:** SQLite (`northstar_knowledge.db`), JSON (wizard ergonomics), or generated dual-write.  
**Why it matters:** CLI show vs Guardian checks currently disagree by design.  
**Evidence:** `cmd_northstar.go` vs `store.go` / `model_helpers.go`.

## Q2 — Should general OODA / VirtualStore actions ever hard-block on Northstar?

Today only campaign observer (+ risk package) hard-fails. Background Level `block` may or may not stop work depending on shards manager policy.  
**Need:** product decision on advisory vs constitutional coupling.

## Q3 — Why does primary boot omit `SetParentKernel`?

Shared boot sets it; primary `session_boot.go` does not. Intentional (avoid fact churn) or wiring gap?  
**Need:** owner confirmation; default assumption in this corpus is **gap**.

## Q4 — Should `ToFacts` expand to relational edges?

Decls for `northstar_serves` / `supports` / `addresses` exist in policy dumps. Wizard may capture data that could populate them, but package types lack those edges today.  
**Need:** schema ownership between wizard state and `Vision` type.

## Q5 — Is TaskObserver load-bearing?

Few non-test production greps. Campaign + background may subsume it. Keep for API stability or fold?

## Q6 — Alignment model selection

`AlignmentModel` on config is unused. Should Guardian open a model by name, or remain pure DI via `SetLLMClient`?

## Q7 — How should multi-workspace / multi-session Guardians share DB?

`/alignment` opens a second connection. Acceptable? Prefer session-scoped singleton on `SystemComponents`?

## Q8 — Prompt atom vs inline Guardian prompts

Wizard atoms are rich; Guardian judge prompt is inline. Is atomization required for consistency with JIT mandate, or is library-local prompt acceptable as a permanent exception?
