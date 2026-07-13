# transparency — Corpus Rebuild Progress

> Date: **2026-07-13**  
> Mode: DOCS ONLY — no Go/Mangle/code changes  
> Source: `internal/transparency/`  
> Instructions: `Docs/architecture/_rebuild/SUBAGENT_INSTRUCTIONS.md`

## What was rebuilt

Full architecture corpus rewritten to cli-quality depth (not thin inventory stubs):

| File | Action |
|------|--------|
| `README.md` | Replaced — scope, map, verify, fact-flow |
| `IMPLEMENTED_SPEC.md` | Replaced — flagship deep dive |
| `00-ALIGNMENT-VISION-REVIEW.md` | Replaced — scored dimensions |
| `01-VISION.md` | **New** (replaces role of old domain stub naming) |
| `02-CURRENT-STATE.md` | **New** inventory |
| `03-GAP-ANALYSIS.md` | **New** matrix |
| `04-ARCHITECTURAL-PRINCIPLES.md` | **New** 12 principles |
| `05-INTERNAL-ARCHITECTURE.md` | **New** components/flows |
| `06-PUBLIC-API-AND-TYPES.md` | **New** API reference |
| `07-DEPENDENCY-MAP.md` | Replaced / rewritten |
| `08-WIRING-AND-INTEGRATION.md` | **New** boot/wiring |
| `09-SAFETY-AND-INVARIANTS.md` | **New** |
| `10-TESTING-ALIGNMENT.md` | **New** |
| `11-OBSERVABILITY.md` | **New** |
| `12-FAILURE-MODES.md` | **New** (replaces thin 08-FAILURE-MODES role) |
| `TODO.md` | Replaced prioritized backlog |
| `OPEN-QUESTIONS.md` | Replaced real questions |
| `_progress.md` | This file |

## Research performed

1. Listed all 8 non-test + 9 test files under `internal/transparency/`.  
2. Read full sources: `doc.go`, `transparency.go`, `event_bus.go`, `glass_box_events.go`, `shard_observer.go`, `safety_reporter.go`, `error_classifier.go`, `explainer.go`.  
3. Grepped reverse deps: chat boot, VirtualStore, ShardManager, system router/base.  
4. Read `TransparencyConfig` in `internal/config/ux.go`.  
5. Sampled test function list and comprehensive coverage areas.  
6. Skimmed prior thin corpus only to preserve counts; content replaced.

## Quality checklist

- [x] No files outside `Docs/architecture/transparency/` modified for this task  
- [x] Paths cited exist under `internal/transparency/` and known consumers  
- [x] IMPLEMENTED_SPEC is dense living spec  
- [x] No pre-impl “no code exists” banners  
- [x] README links full doc set  
- [x] Honest gaps (Observer vs Glass Box, status-only flags, SafetyReporter mutex)  
- [x] North star: LLM creative / logic executive / default deny referenced  

## Legacy filenames

Older stubs may still exist beside the new set (`01-DOMAIN-MODEL.md`, `02-CURRENT-STATE-TRANSPARENCY.md`, etc.). Canonical set is the numbered map in `README.md`. Prefer the new names; treat unmatched old files as superseded.

## Next (not done here)

Implement P0 wiring items from `TODO.md` in a code-change task; re-verify scores in `00-ALIGNMENT-VISION-REVIEW.md` after.
