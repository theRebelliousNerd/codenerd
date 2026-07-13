# prompt — Open Questions

> Last verified: **2026-07-13**

## Q1 — Cache key completeness vs intentional stability

**Decision — 2026-07-13.** Correctness wins: every prompt-affecting field enters
versioned cache schema `compilation-context-v2`; set-like fields are canonicalized.
Call-site `ClearCache` is not a substitute for identity. Future context fields must
be explicitly classified and covered by hash/output tests.

## Q2 — Single ConfigAtom source of truth

**Decision — 2026-07-13.** Production boots use `NewDefaultConfigFactory()` and
canonical policy-set identifiers. **Open remainder:** generate its tool catalog
from the live registry or remove the dormant `SimpleRegistry` catalog.

## Q3 — Conflict resolution ownership

How much conflict/exclusive enforcement remains in Mangle vs Go resolver? If Mangle rules incomplete, Go may assemble conflicting guidance. Need explicit ownership matrix.

## Q4 — Evolved atoms in default boot

Is `EvolvedAtomManager` always attached to `JITPromptCompiler` in interactive chat, or only some paths? Incomplete attachment means SPL writes never reach Compile.

## Q5 — PredicateSelector product role

Is predicate JIT injection still a first-class path for legislator/mangle teaching prompts, or superseded by mangle atom YAML encyclopedias? Risk of dual incomplete systems.

## Q6 — Skeleton category set expansion

Should `hallucination` and/or `capability` enter skeleton (deterministic) categories? Today they are flesh/high-budget — failure modes differ.

## Q7 — CategorySystem validation vs assembly order

`CategorySystem` exists for atoms under `atoms/system/`. Confirm assembler `defaultCategoryOrder` includes system (if missing, system sections may append unordered at end — verify source).

## Q8 — Agents.md path

**Decision — 2026-07-13.** `internal/prompt/agents.md` now owns scoped authoring,
migration, and verification guidance; root routing remains correct.

## Q9 — Vector search over embedded-only mode

Is it acceptable that flesh semantic search is empty until corpus DB sync? Or should in-memory embeddings be supported for embedded atoms?

## Q10 — Manifest dropped completeness

Does every drop path (selector threshold, Mangle block, budget, structured filter) populate Manifest.Dropped with stable reason codes for UI?

---

When answering these in code, update this file with **Decision** + date rather than deleting history.
