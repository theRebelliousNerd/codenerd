# Architecture corpus rebuild — subagent contract

**DO NOT modify any Go, Mangle, tests, configs, or code under `internal/`, `cmd/`, or elsewhere.**  
**ONLY write markdown under `Docs/architecture/<PACKAGE>/` (and nothing else).**

## Package

You are assigned exactly one package:

- Source: `internal/<PACKAGE>/`
- Output: `Docs/architecture/<PACKAGE>/`

## Quality bar (mandatory)

Your corpus must match the **depth and seriousness** of `Docs/architecture/cli/` (the reference rewrite), not the earlier thin auto-inventory stubs.

Anti-patterns (automatic fail):

- Generic filler (“this package provides functionality…”)
- Tables of files with no behavioral narrative
- Claiming pre-implementation 0% when code exists
- Inventing APIs, routes, or wires not in source
- sibling-platform product terms (foreign-product-surface, foreign-agent-kit, foreign-codegen, attention-channel-as-product)

Required qualities:

- Real path citations that exist on disk (`internal/<PACKAGE>/...`)
- Control-flow / architecture diagrams in mermaid or ASCII where useful
- Honest gaps and partials
- Package-specific content (if you could swap the package name and the doc still reads true, rewrite)

## Research procedure (do this first)

1. List all files under `internal/<PACKAGE>/` (go, mg, md, yaml, etc.). Count src vs tests vs mg.
2. Read package docs if any (`README.md`, `agents.md`, package comments).
3. Read the **largest and most important** non-test `.go` files (aim for top ~10–15 by size or centrality).
4. Grep for exported types (`^type [A-Z]`), key constructors (`New…`), interfaces, registration hooks.
5. Grep reverse deps: who imports `codenerd/internal/<PACKAGE>` (from `cmd/`, other `internal/`).
6. Note how it sits in fact-flow:  
   `user_intent → kernel → next_action → VirtualStore → articulation`  
   and Mangle/prompt/shard involvement if any.
7. Skim existing `Docs/architecture/<PACKAGE>/` only to avoid losing unique truths; **replace thin stubs wholesale**.

## Documents to produce (full set)

Write **all** of these under `Docs/architecture/<PACKAGE>/` (overwrite thin files):

| File | Minimum substance |
|------|-------------------|
| `README.md` | Scope, doc map, verify commands, links |
| `IMPLEMENTED_SPEC.md` | **Flagship** — overview, status table, inventory, deep dives on main flows, integration map, gaps pointer. Target **≥400 lines or ≥15KB** for large packages (core, mangle, perception, prompt, shards, campaign, store, world, autopoiesis, session, tools, config, context, mcp, tactile, testing); smaller packages still need **real narrative**, not stubs (aim ≥150 lines). |
| `00-ALIGNMENT-VISION-REVIEW.md` | Scored dimensions vs codeNERD north star with evidence |
| `01-VISION.md` | Target product/architecture vision for this package |
| `02-CURRENT-STATE.md` | Precise inventory: files, lines, roles, hotspots |
| `03-GAP-ANALYSIS.md` | Spec vs reality matrix, priorities, non-gaps |
| `04-ARCHITECTURAL-PRINCIPLES.md` | 6–12 binding principles specific to this package |
| `05-INTERNAL-ARCHITECTURE.md` | Components, data flow, key types, state machines |
| `06-PUBLIC-API-AND-TYPES.md` | Exported types/funcs that matter, with file refs |
| `07-DEPENDENCY-MAP.md` | Upstream/downstream packages with evidence |
| `08-WIRING-AND-INTEGRATION.md` | How this is registered/called (boot, CLI, kernel, shards) |
| `09-SAFETY-AND-INVARIANTS.md` | Safety, concurrency, Mangle Decl if relevant |
| `10-TESTING-ALIGNMENT.md` | Existing tests, gaps, commands |
| `11-OBSERVABILITY.md` | Logging categories, metrics, debug hooks |
| `12-FAILURE-MODES.md` | Concrete failure modes + mitigations |
| `TODO.md` | Prioritized backlog |
| `OPEN-QUESTIONS.md` | Real open questions |
| `_progress.md` | Date + what was rebuilt |

Add extra deep-dives only if the package warrants (e.g. `09-MANGLE-SURFACE.md` for mangle-heavy packages).

## North star (inject where relevant)

- LLM = creative center; Mangle kernel = executive
- Constitutional safety: `permitted(...)`, default deny
- JIT prompt atoms for new LLM-facing behavior
- Wiring audit before calling anything unused

## Completion checklist

- [ ] No files outside `Docs/architecture/<PACKAGE>/` modified
- [ ] Every cited path exists (`Test-Path` / read)
- [ ] IMPLEMENTED_SPEC is the dense living spec
- [ ] No pre-impl “no code exists” banners
- [ ] README links to every produced doc
- [ ] Write a one-line status to stdout when done: `DONE <PACKAGE> files=N bytes=B`

## Date

Use last-verified date **2026-07-13** (or today’s date if later).
