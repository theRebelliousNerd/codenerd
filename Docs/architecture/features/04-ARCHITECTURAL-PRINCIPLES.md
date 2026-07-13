# 04 — Architectural Principles: features

> Last verified against codebase: **2026-07-13**  
> These principles are **binding** for changes under `internal/features/` and for new consumers.

## P1 — Leaf purity is non-negotiable

`internal/features` must not import any `codenerd/internal/*` package.  
Rationale: core/world/CLI must read flags without creating cycles through config/store.  
**Test:** `go list -f '{{.Imports}}' ./internal/features` stays stdlib-only.

## P2 — Three-way lockstep for new flags

Adding a flag requires all of:

1. Field on `FeaturesConfig` with JSON tag and doc comment (env name included).  
2. Public accessor implementing env → active → default.  
3. Decision in `DefaultFeaturesConfig` **and** `FullyEnabledFeaturesConfig` (or explicit comment why FullyEnabled differs).  
4. Prefer an immediate consumer or an explicit “registry-only until Track X” note in the field comment.

## P3 — Precedence is total and documented

For every toggle:

```
environment (recognized values) > active FeaturesConfig field > compile-time default
```

Unrecognized env values must **not** override. Never invent a fourth source without updating all accessors and tests.

## P4 — Pointer bools for tri-state config

Use `*bool` so “absent key” ≠ “false”. Integers may use zero as “unset → call-site default” when zero is never a valid override (workers, max bytes).

## P5 — Conservative compile defaults for semantic/cost paths

Paths that change evaluation semantics or allocate heavily (DiffEval, Provenance, PerShardFacts) default **OFF** in `DefaultFeaturesConfig`. Cheap observability (FlightRecorder) may default ON.  
**Do not** flip DefaultFeaturesConfig to “everything modern” — that is FullyEnabled’s job via disk seed.

## P6 — Snapshot on SetActive

`SetActive` copies the struct. Callers must not rely on mutating the original after install. Readers must not mutate `Active()` return values.

## P7 — Logging stays at the boundary

Features does not import logging. The loader (`LoadUserConfig`) emits Boot-level Summary after SetActive. New writers of SetActive must log or knowingly skip.

## P8 — Master switches ≠ granular disable lists

`IsSystemShardsEnabled` is process-wide. Per-shard disable lists (`NERD_DISABLE_SYSTEM_SHARDS`, CLI flags) live at call sites. Never overload one env to mean both.

## P9 — Incomplete subsystems stay opt-in

If enabling a flag can soft-brick without companion machinery (PerShardFacts / coordinator), keep FullyEnabled false and document readiness criteria in the field comment.

## P10 — Wiring audit before deletion

This repo has half-wired features. An accessor with zero importers may still be intentional registry surface (or a tool that greps env). Grep consumers, env names, JSON keys, and tests before removing.

## P11 — No second constitution

Feature flags enable machinery; they do **not** authorize actions. Policy remains `permitted(...)` / Dreamer / VirtualStore. Never add `if features.X { skip safety }`.

## P12 — Test the contract, not the struct fields alone

Prefer tests that call public `Is*` accessors after SetActive/env (as production does) over asserting only on raw `FeaturesConfig` fields.
