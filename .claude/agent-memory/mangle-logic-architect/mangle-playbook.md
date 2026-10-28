---
name: mangle-playbook
description: How to do remarkable things with Mangle in codeNERD; the stance, the starting backlog, and findings verified on v0.7 (2026-10-28)
metadata:
  type: project
---

**Stance:** Mangle is the star. Every mission tries a Mangle-only feature in a lab first: lattices, temporal facts, proofs as data, externals, structured and tagged types, strict bounds, programs as data. Then the evidence picks the production design.

**Why:** codeNERD is the premier Mangle showcase. Burak, Mangle's author and the user's friend, has forked it. Nobody has good public examples of what Mangle can really do: not the upstream `examples/`, not the Vectryx skill. codeNERD's production features are meant to be those examples.

**How to apply:**
- Lab: `.claude/skills/mangle-programming/scripts/new-lab.sh <slug>`, then `go test -v ./_mangle_lab/<slug>/`.
- Start ideas: [[backlog]] in `references/pushing-mangle-forward.md`.
- A striking result goes to the skill's `showcase/` and gets linked from SKILL.md.

Findings verified on v0.7 (2026-10-28):
- The kernel analyzes with `AnalyzeOneUnit`, so Decl bounds are unchecked. Only generated rules get `ErrorForBoundsMismatch` (`internal/core/kernel_rule_bounds.go`).
- `!ext(...)` reads only the store and never calls the Go callback (`engine/premise.go`).
- `n(Y) :- n(X), Y = fn:plus(X,1).` analyzes fine and runs to the 100k fact limit.
- Temporal programs need a temporal store; `nerd check-mangle` has none.
- A merge lattice replaces a fact only if the store implements `factstore.FactStoreWithRemove`. Check this before any lattice design.
- The Vectryx skill calls weighted shortest paths a Mangle limit. Merge lattices may refute that; it's backlog item 2.
