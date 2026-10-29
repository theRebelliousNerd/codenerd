---
doc-class: governance
subsystem: orient
implementation-status: accepted-not-implemented
last-verified: 2026-09-29
verified-against: e056692c
supersedes: []
---

# ADR-003: Orientation Executive Decisions Are Mangle-Derived in a Dedicated Engine

## Context

A central founding principle of codeNERD (`CLAUDE.md:49-54`) is "Clean fixpoint, not clean loop: the executive decisions — what a turn is, whether it is done, what is delegated and with what task, what enters the window, what the verdict is — are the fixpoint of the kernel over the facts. The drift to hunt is decisions computed in Go instead of derived."

In the codebase today, orientation and onboarding decisions have drifted into imperative Go logic:
- `internal/init/agents.go:373-626` decides specialist agents via an imperative Go `switch` statement on language and framework strings.
- `internal/init/strategic_knowledge.go:246-260` decides documentation importance via hardcoded integer maps.
- `internal/init/initializer.go:796-804` leaves North Star synthesis to disconnected heuristics.

## Decision

1. **Dedicated Orientation Engine**:
   Construct an independent Mangle Datalog engine (`internal/orient.NewEngine`) loading embedded policies (`internal/orient/*.mg`). It operates in complete isolation from the main session kernel to avoid fact pollution during initialization.
2. **Strict Go / Mangle Division of Responsibilities**:
   - **Go as Sensor**: Go code performs streaming I/O, regex-free format parsing, AST inspection, hash generation, and token budgeting, asserting ground EDB facts.
   - **Mangle as Executive**: All executive decisions — defining development eras (`repo_era`), establishing document lineages (`doc_evolved_into`), picking winning skills (`agent_source_winner`), deriving specialist agents (`orient_agent`), classifying tree treatments (`tree_treatment`), and generating clarification questions (`orient_question`) — are derived as Datalog fixpoints.
3. **No Shims on Migration**:
   When Mangle takes ownership of an executive decision, the corresponding imperative Go switch or heuristic is completely deleted, repointing callers directly to engine queries.

## Consequences

- **Positive**: Every orientation decision becomes inspectable, traceable, and explainable through standard Datalog derivation trees; business rules move from compiled Go code to declarative `.mg` files.
- **Negative**: Requires maintaining embedded Mangle policy files and ensuring strict adherence to stratification and negation safety.
- **Risks**: Engine evaluation time must be bounded by structuring rules to avoid non-terminating recursive joins.

## Witness

**Witness:** `symbol:internal/orient.NewEngine` and `test:TestOrientEngine_EvaluatesFixpoint`

| Claim | Witness (Code Evidence as of 2026-09-29) |
|---|---|
| Imperative Go decision switch lives today | `determineRequiredAgents` in `internal/init/agents.go:373-626`. |
| Target Go API symbol | `NewEngine(cfg *config.OrientConfig) (*Engine, error)` in `internal/orient/engine.go`. |
| Proving regression test | `TestOrientEngine_EvaluatesFixpoint` verifying complete derivation of `orient_agent` and `tree_treatment` over synthetic fact sets. |

## Status (Derived, Not Asserted)

**`accepted-not-implemented`**. Derivation: The decision is approved, but `internal/orient/engine.go` has not yet landed in commit `e056692c`. Status flips to `implemented` once the engine compiles and passes fixpoint evaluation tests.
