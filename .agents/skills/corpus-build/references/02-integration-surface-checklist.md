# Integration Surface Checklist (codeNERD)

Human-readable companion to machine registry `surfaces.yaml`.
When they disagree, fix **surfaces.yaml** first, then reconcile this file.

## A — Core engine

| ID | Surface | Typical paths | Fix owner |
|----|---------|---------------|-----------|
| A1 | Kernel / fact load | internal/core/ | corpus-builder |
| A2 | Schemas Decl | internal/core/defaults/schemas.mg | corpus-builder + mangle |
| A3 | Policy corpus | internal/core/defaults/policy/ | corpus-builder + mangle |
| A4 | VirtualStore routes | internal/core/virtual_store*.go | corpus-builder |
| A5 | Shard manager | internal/core/shards/ | corpus-builder |
| A6 | Shard registration | internal/shards/registration.go | intent → wiring-auditor |
| A7 | Session executor | internal/session/ | corpus-builder |
| A8 | Perception | internal/perception/ | corpus-builder |
| A9 | Articulation / Piggyback | internal/articulation/ | corpus-builder |
| A10 | Prompt compiler / atoms | internal/prompt/ | corpus-builder + prompt-jit |

## B — Config & CLI

| ID | Surface | Paths | Fix owner |
|----|---------|-------|-----------|
| B1 | Config schema | internal/config/, .nerd/config.json | corpus-builder |
| B2 | CLI commands | cmd/nerd/ | corpus-builder |
| B3 | Chat handlers | cmd/nerd/chat/ | corpus-builder |

## C — Memory & storage

| ID | Surface | Paths | Fix owner |
|----|---------|-------|-----------|
| C1 | Store tiers | internal/store/ | corpus-builder |
| C2 | Embedding | internal/embedding/ | corpus-builder |
| C3 | World model | internal/world/ | corpus-builder |

## D — Observability & safety

| ID | Surface | Paths | Fix owner |
|----|---------|-------|-----------|
| D1 | Logging categories | internal/logging/ | corpus-builder |
| D2 | Dreamer / safety | internal/core/dreamer.go | corpus-builder |
| D3 | Transparency | internal/transparency/ | corpus-builder |

## E — Campaign & autopoiesis

| ID | Surface | Paths | Fix owner |
|----|---------|-------|-----------|
| E1 | Campaign | internal/campaign/ | corpus-builder |
| E2 | Autopoiesis | internal/autopoiesis/ | corpus-builder |

## F — Tools & protocols

| ID | Surface | Paths | Fix owner |
|----|---------|-------|-----------|
| F1 | Tools | internal/tools/ | corpus-builder |
| F2 | MCP | internal/mcp/ | corpus-builder |

## G — Tests & docs

| ID | Surface | Paths | Fix owner |
|----|---------|-------|-----------|
| G1 | Package tests | `*_test.go` near code | corpus-builder |
| G2 | Architecture status | Docs/architecture/ | corpus-doc-auditor |
| G3 | Product Spec | Docs/Spec/ | corpus-doc-auditor (note only) / spec-doc-sprint |

## Verdict model

- **PASS** — detection greps hit for applicable surface
- **FAIL** — applicable, zero hits → fix_owner
- **N-A** — applicability false
- **AMBIGUOUS** — judgment-required or mixed hits → corpus-wiring-auditor
