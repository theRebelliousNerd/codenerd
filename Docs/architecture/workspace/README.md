---
doc-class: governance
subsystem: workspace
implementation-status: not-applicable
last-verified: 2026-09-29
verified-against: e056692c
supersedes: []
---

# workspace

> Verified 2026-09-29 against `e056692c` (`main`).

This directory is the governed architecture corpus for `internal/workspace` — the single, authoritative workspace membership system for codeNERD. It establishes the north-star vision, current shipped baseline, gap matrix, and capability specifications for workspace membership and `.gitignore` compliance. Start with [00-INDEX.md](00-INDEX.md) for the read order and governance map.

Package `codenerd/internal/workspace`: a dedicated leaf package providing a single, concurrency-safe authority (`Membership`) that answers whether any file or directory belongs to the workspace.

1. **The Membership Crisis** — In real-world foreign repositories, git typically tracks ~20,000 files, yet codeNERD's walkers visit upwards of 100,000 files (traversing virtual environments, compilation artifacts, package caches, pytest temporary directories, and seed datasets). This occurs because codeNERD currently lacks `.gitignore` support, relying instead on approximately 35 independent filesystem walkers maintaining roughly 25 fragmented, hardcoded ignore lists.
2. **Git Version Control Truth** — In a git repository, `internal/workspace` establishes membership via a single, fast `git ls-files -z -co --exclude-standard` invocation with `GIT_OPTIONAL_LOCKS=0`. Member directories are defined as the ancestors of member files. Newly created or unstaged files are resolved via batched `git check-ignore -z --stdin` calls and cached.
3. **User-Configured Extra Exclusions** — The user's `world.ignore_patterns` configuration is applied on top of git truth as an extra exclusion layer with full `**`, directory prefix, and `!` negation glob semantics.
4. **Semantic Discernment Overlay** — Ingests `.nerd/orientation/membership.json` from `internal/orient`, providing operational treatments (`/index_and_parse`, `/index_names_only`, `/exclude`) so that seed datasets and golden answer keys are recognized by name without being parsed as code.
5. **Universal Repointing** — Every filesystem walker in codeNERD (scanners, search tools, CodeDOM indexers, campaign decomposers, and TUI handlers) is repointed to query `Membership.Includes` or `Membership.Walk`, eliminating all private skip lists across the codebase.

What runs today across existing walkers is documented in [02-CURRENT-STATE.md](02-CURRENT-STATE.md) and [IMPLEMENTED_SPEC.md](IMPLEMENTED_SPEC.md); the full census of ~35 production walkers is catalogued in [WIRING-AND-NOT-BUILT.md](WIRING-AND-NOT-BUILT.md).
