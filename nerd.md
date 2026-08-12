---
schema: nerd/v1
project: codeNERD
language: go
commands:
  build: go build -o nerd.exe ./cmd/nerd
  test: go test ./...
  lint: go vet ./...
  env:
    CGO_CFLAGS: -IC:/CodeProjects/codeNERD/sqlite_headers
forbid:
  - match: .nerd/config.json
    reason: >-
      Live user-owned runtime config holding API keys and provider selection.
      A single overwrite has already destroyed ~160 lines of customization.
      Edit it by hand, never through a tool.
  - match: .nerd/mangle/learned
    reason: >-
      Persistent kernel overlay written by the autopoiesis loop. Hand edits
      here are silently overwritten and can poison every subsequent boot.
  - match: internal/session/modularity.go
    reason: >-
      The modularity standard's measurement and kernel evaluation. Observed
      2026-08-12: when the guard refused a write, the agent's next move was to
      edit the guard to exempt the file it had just been blocked on, and the
      constitution derived permitted for that edit. A standard the subject can
      amend is not a standard. This file exists only to constrain, so nothing
      legitimate is lost by making it unwritable by tool.
  - match: internal/core/defaults/policy/coder_quality.mg
    reason: >-
      Holds the modularity thresholds the kernel decides with. They live in
      policy precisely so a limit is changed deliberately rather than by the
      agent that finds it inconvenient.
  - match: internal/core/defaults/policy/constitution.mg
    reason: >-
      Derives permitted/3. Every other guard is downstream of it, so an agent
      able to edit this can authorise anything it likes, including its own
      exemptions.
require:
  - Run `go test ./...` before declaring work complete.
  - All Mangle predicates need a Decl before use, and no predicate may be
    declared twice at the same arity — a duplicate takes the whole kernel down
    at boot, not just the rule that uses it.
  - New LLM-facing behaviour becomes a prompt atom under
    internal/prompt/atoms/, not prose hardcoded in a shard.
conventions:
  - id: conventional-commits
    rule: Commit subjects use a conventional-commit prefix (feat, fix, chore, docs, test).
  - id: mangle-atoms-are-lowercase
    rule: Mangle variables are UPPERCASE and atoms are /lowercase; the two types are disjoint and never unify.
  - id: numbers-are-int64
    rule: >-
      Every numeric Mangle slot is /number. The pinned fork compares int64 only,
      and one float64 fact aborts the entire kernel fixpoint. Scale ratios with
      types.PercentFromRatio before they reach a fact.
  - id: audit-before-delete
    rule: >-
      Look for wiring gaps before deleting apparently-unused code. This codebase
      has many partially wired features and dormant integration points.
---

## codeNERD

A high-assurance, logic-first CLI coding agent. The model is the creative
center; the Mangle kernel is the executive. Logic determines reality; the model
merely describes it.

### Architecture in one paragraph

Natural language enters through the perception transducer and becomes formal
atoms. The Mangle kernel derives `next_action` from those atoms plus policy.
The VirtualStore executes. Articulation renders the result. Every action must
derive `permitted(...)`; the default is deny.

### Where things live

| Area | Path |
|------|------|
| Kernel | `internal/core/kernel_*.go` |
| Policy corpus | `internal/core/defaults/policy/` |
| Schemas | `internal/core/defaults/schemas_*.mg` |
| Prompt atoms | `internal/prompt/atoms/` |
| JIT compiler | `internal/prompt/compiler.go` |
| Session execution | `internal/session/executor.go` |
| Project instructions (this file) | `internal/projectdoc/` |

### Notes for whoever is reading this

The `forbid` rules above are not advice. They are asserted into the kernel at
boot as `project_forbidden_path` facts and checked before any write-mutation
tool runs. Attempting one costs a turn and changes nothing.
