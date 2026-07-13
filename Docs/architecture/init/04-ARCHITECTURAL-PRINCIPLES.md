# init — Architectural Principles

> Last verified: 2026-07-13  
> These principles are **binding for changes** in `internal/init/`.

## 1. Cold-start is substrate, not cognition

Init materializes directories, DBs, profiles, and registries. It does **not** replace the session OODA loop. Prefer finishing a usable `.nerd/` over perfect LLM analysis.

## 2. Deterministic detection before creative enrichment

Language, dependencies, entry points, and agent **candidates** come from file/lock heuristics. LLM may **describe** (strategic knowledge, doc filtering, research atoms) but must not be the sole source of project identity.

## 3. Prefer warnings over total failure for enrichment

Missing research, strategic knowledge, or tool generation should append `InitResult.Warnings` and continue. Hard-fail only when the workspace substrate cannot be created (directory tree, required embedding engine for sqlite-vec KBs).

## 4. Upgrade, do not blindly clobber

`--force` / existing KBs use upgrade mode: hash-based atom append, schema migration via `store.MigrateAllAgentDBs`, registry load. Do not delete user preferences unless the operator explicitly destroys `.nerd/`.

## 5. Shared knowledge first, specialists second

Create `core_concepts.db` before per-agent KBs; new agents **inherit** shared atoms. Avoid N× duplicated base concepts.

## 6. JIT-first for LLM-facing init behavior

New init-phase prompt guidance goes through prompt atoms / `jit_integration` / corpus, not a resurrected domain Researcher shard. Session-time deep research stays on JIT clean loop + `session.Executor`.

## 7. Batch kernel facts; never O(N²) Assert loops

World scan facts must use `kernel.LoadFacts` (or equivalent batch APIs). The code comments document the 10K-file cost of per-file Assert — do not regress.

## 8. Mangle hygiene for generated facts

- Escape strings in `profile.mg`.
- Sanitize name constants with `sanitizeForMangle`.
- Ship empty `extensions.mg` / `policy_overrides.mg` templates, not silent coupling to core policy files.

## 9. Progress is first-class UX

Phases have names shared with `DefaultPhaseDurations` / ETA tracker. Progress channel must remain non-blocking (`select` + `default`). Do not block init on a full progress buffer.

## 10. Specialist agents are profiles, not free processes

`registerAgentsWithShardManager` defines **profiles** (`DefineProfile`) with permissions and tools metadata. Init does not own long-lived agent runtimes.

## 11. Validation closes the loop

After success, run `ValidateAllAgentDBs` (or equivalent) and surface backup cleanup guidance. Leaving corrupt KBs “green” without warning is a principle violation.

## 12. Wiring audit before deletion

Partial surfaces (interactive, Type U, tool gen stub, strategic docs) may look unused. Grep CLI/chat before removing; prefer completing wiring over deleting half-integrated features.
