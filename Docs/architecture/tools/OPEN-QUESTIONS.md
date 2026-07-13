# tools — Open Questions

> Last verified: **2026-07-13**

## Q1 — Single registry or dual forever?

Session uses `tools.Global()`; VirtualStore uses `modularTools`. Hydrate fills both.  
**Options:** (a) session takes VS registry pointer only; (b) Global remains canonical; (c) keep dual with strict hydrate.  
**Impact:** tests, multi-session purity, desync risk.

## Q2 — Who is the source of truth for allowlists?

Mangle `modular_tool_allowed`, ConfigFactory `AllowedTools`, and soft `FilterByIntent` overlap.  
Should session query kernel per call, or only trust ConfigFactory materialization?

## Q3 — Empty AllowedTools semantics

Fail open (current) vs fail closed vs “default safe subset” (read-only tools).  
E2E currently documents fail-open — is that intentional product policy?

## Q4 — Should search tools escape workspace for monorepos?

Some agents want to read sibling repos. Prefer explicit `allowed_roots[]` config over unrestricted paths?

## Q5 — bash tool threat model

Is multi-line bash acceptable for all `/code` intents, or should it require elevated permission atoms?

## Q6 — Browser lifecycle ownership

tools/research owns a process-global SessionManager; `internal/browser` is also used by CLI.  
Should research tools use injected manager from VS instead of package Once?

## Q7 — Ouroboros name collisions

If a compiled tool shares a Name with a modular tool, modular wins (checked first).  
Is that permanent policy? Should dual registration error?

## Q8 — CategoryReview / CategoryAttack

Keep empty categories for future Nemesis/review tools, or remove until implemented?

## Q9 — Research cache multi-session

In-memory cache shared process-wide — OK for single interactive nerd, wrong for multi-tenant.  
Scope: process, session, or workspace?

## Q10 — Impact tools vs run_tests

When should agents prefer `run_impacted_tests` over `run_tests`? Prompt atoms / Mangle priority only partially guide this.
