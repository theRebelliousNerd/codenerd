# prompt — Open Questions

> Last verified: **2026-07-13**

## Q1 — Cache key completeness vs intentional stability

Which dynamic fields **must** bust the prompt cache?  
`AvailableTools` and `PreviousAttemptNoToolCall` clearly affect templates/selection, but hashing them increases miss rate. Prefer full correctness or selective ClearCache at call sites?

## Q2 — Single ConfigAtom source of truth

Should session always use `NewDefaultConfigFactory()` (in-package provider) or `SimpleRegistry` + `RegisterDefaultConfigAtoms`? Tool name sets differ today. Which is canonical for production boots?

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

Root Agents.md Working Map cites `internal/prompt/agents.md`. File may be absent; package README is dense. Should agents.md be created or root map retargeted?

## Q9 — Vector search over embedded-only mode

Is it acceptable that flesh semantic search is empty until corpus DB sync? Or should in-memory embeddings be supported for embedded atoms?

## Q10 — Manifest dropped completeness

Does every drop path (selector threshold, Mangle block, budget, structured filter) populate Manifest.Dropped with stable reason codes for UI?

---

When answering these in code, update this file with **Decision** + date rather than deleting history.
