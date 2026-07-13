---
name: corpus-build
description: >
  Spec-driven implementation engine for codeNERD architecture corpora under
  feature directories under Docs/architecture/. Audits the live Go and Mangle implementation,
  classifies the gap, packetizes work into a dependency DAG, dispatches a
  governed Codex specialist fleet, verifies wiring and constitutional safety,
  and reconciles the corpus from test evidence. Use for "corpus-build",
  "build from spec", "realize this architecture", "make the corpus real", or
  "implement the subsystem docs". Do not use for routine fixes, architecture
  ideation (use arch-propose), or documentation-only drift review.
metadata:
  version: 3.0.0
  author: codeNERD
  last-verified: 2026-07-13
---

# Corpus Build

Turn an accepted codeNERD architecture corpus into verified production behavior.

The LLM fleet is the creative and implementation center. The Mangle kernel,
repository contracts, test gates, and explicit run ledger are the executive.
A file existing is not completion; the behavior must be wired into the live
fact flow and derive through the expected policy surface.

## Codex orchestration contract

This skill explicitly requires a subagent workflow for non-trivial runs.

- Keep the root agent responsible for scope, plan state, dependency ordering,
  serial integration gates, user-visible checkpoints, and final evidence.
- Delegate independent read-heavy work in parallel.
- Delegate writes only through bounded packets with disjoint ownership.
- Use the registered agent names from `.codex/config.toml`.
- Keep `agents.max_depth = 1`; fleet agents do not recursively delegate.
- Wait for all agents in a wave, reconcile their outputs, then open the next
  dependency level.
- Preserve unrelated dirty-tree changes and never use broad staging.

## Required inputs

A full run requires:

1. `Docs/architecture/<subsystem>/IMPLEMENTED_SPEC.md`
2. The adjacent corpus documents and their explicit invariants
3. Live source paths discovered from the repository, not guessed from the
   subsystem name
4. The closest `AGENTS.md` or scoped guidance for every touched subtree

If the corpus is incomplete or contradictory, route back to `arch-propose
--expand` or record the contradiction before implementation.

## Fleet

### Discovery and judgment

| Agent | Registry key | Mission | Sandbox |
|---|---|---|---|
| Corpus reader | `corpus_reader` | Extract requirements, invariants, source-path claims, and acceptance gates | read-only |
| Requirements interrogator | `requirements_interrogator` | Attack ambiguity and pin interface contracts | read-only |
| Corpus judge | `corpus_judge` | Classify each gap as build, evolve, pivot, already-real, or blocked | read-only |
| Corpus packetizer | `corpus_packetizer` | Convert accepted gaps into bounded work packets and a dependency DAG | workspace-write |

### Implementation lanes

| Agent | Registry key | Mission |
|---|---|---|
| Foundation worker | `corpus_foundation_worker` | Types, config, schemas, deterministic local scaffolding |
| Mangle specialist | `mangle_specialist` | Declarations, policies, recursion, stratification, and logic-derived behavior |
| Prompt architect | `prompt_architect` | JIT prompt atoms, compiler selection, piggyback/control-packet behavior |
| Wiring worker | `corpus_wiring_worker` | Registrations and already-specified cross-package seams |
| Integration worker | `corpus_integration_worker` | Ambiguous multi-package runtime integration |
| Surface worker | `corpus_surface_worker` | CLI, MCP, A2A, TUI, and external tool exposure |
| Corpus builder | `corpus_builder` | Bounded general implementation fallback |

### Verification and closeout

| Agent | Registry key | Mission | Sandbox |
|---|---|---|---|
| Corpus critic | `corpus_critic` | Review code, stubs, regressions, and missing tests | read-only |
| Wiring auditor | `corpus_wiring_auditor` | Prove execution-path and registration completeness | read-only |
| Defense auditor | `corpus_defense_auditor` | Verify `permitted(...)`, trust boundaries, and observability | read-only |
| Comms plumber | `corpus_comms_plumber` | Repair CLI/MCP/A2A/tool routes | workspace-write |
| Consumables keeper | `corpus_consumables_keeper` | Keep prompts, schemas, embeds, and generated artifacts synchronized | workspace-write |
| Doc auditor | `corpus_doc_auditor` | Reconcile architecture status from evidence | workspace-write |
| Governance reconciler | `corpus_governance_reconciler` | Close TODO, open-question, index, and run-ledger state | workspace-write |
| Jules dispatcher | `corpus_jules_dispatcher` | Package exhausted failures for the repo remediation workflow | workspace-write |

## Run artifacts

Keep active run state under `.corpus-build/`:

```text
.corpus-build/
  contracts/
  intents/
  ledger/
  manifests/
  matrices/
  packets/
  plans/
  results/
  reviews/
  slices/current/
```

Create `.corpus-build/ledger/<session_id>.active` with the full session ID and:

```json
{"run_id":"corpusbuild_<subsystem>_<epoch>","phase":"init","skill":"corpus-build"}
```

Update `phase` at each transition. The Codex hooks use this exact file to
separate governed fleet telemetry from unrelated subagent work.

## Pipeline

### Phase 0: Initialize and deconflict

1. Read the corpus and closest guidance.
2. Record `git status --short`, recent path history, and current unpushed state.
3. Resolve the real source paths. A documentation directory does not imply a
   matching `internal/<name>/` package.
4. Create the active ledger and run directories.
5. Verify every required registry key and attached skill path exists.
6. Detect concurrent or already-landed work before assigning packets.

Gate: all inputs, paths, fleet roles, and dirty-tree boundaries are explicit.

### Phase 1: Ingest and audit

Dispatch `corpus_reader` and `integration_auditor` in parallel.

The reader emits a requirement manifest. The auditor traces the live path:

```text
user input -> perception -> user_intent -> kernel next_action
-> VirtualStore/tool/shard execution -> articulation
```

Inspect these codeNERD control surfaces when relevant:

- `internal/core/defaults/schemas.mg`
- `internal/core/defaults/policy/`
- `internal/core/kernel_*.go`
- `internal/core/virtual_store.go`
- `internal/prompt/compiler.go` and `internal/prompt/atoms/`
- `internal/articulation/prompt_assembler.go`
- `internal/session/executor.go`
- `internal/core/shards/manager.go`
- `internal/shards/registration.go`
- `cmd/nerd/`

Gate: each claimed gap has file/symbol evidence or is marked unverified.

### Phase 2: Interrogate and judge

Dispatch `requirements_interrogator` with the manifest and audit.

Pin:

- data and predicate contracts
- ownership of state and side effects
- `permitted(...)` and default-deny behavior
- JIT prompt-atom impact
- lifecycle registration points
- failure, cancellation, and recovery semantics
- tests that can falsify the design

Then dispatch `corpus_judge`. It must classify every row:

- `ALREADY_REAL`
- `BUILD`
- `EVOLVE`
- `PIVOT`
- `BLOCKED_BY_SPEC`

Gate: no implementation begins until every row has an evidence-backed verdict.

### Phase 3: Packetize the DAG

Dispatch `corpus_packetizer`.

Each packet must declare:

- stable packet ID and requirement IDs
- owned files and symbols
- dependencies
- lane/agent
- forbidden files
- acceptance commands
- rollback boundary
- registration intents for shared files

Route packets by work type:

- declarations/types/local config -> `corpus_foundation_worker`
- Mangle schemas/policy/rules -> `mangle_specialist`
- prompt atoms/JIT assembly -> `prompt_architect`
- specified registrations -> `corpus_wiring_worker`
- ambiguous runtime seams -> `corpus_integration_worker`
- CLI/MCP/A2A/TUI exposure -> `corpus_surface_worker`
- mixed bounded fallback -> `corpus_builder`

Gate: the DAG is acyclic and concurrently scheduled packets have disjoint writes.

### Phase 4: Build in dependency waves

Spawn one agent per ready packet, bounded by `agents.max_threads`.

After each wave:

1. Wait for all workers.
2. Inspect the diff and packet acceptance results.
3. Apply shared-file registration intents serially.
4. Run `gofmt` on touched Go files.
5. Run targeted tests before opening the next wave.
6. Record actual failures and usage when supplied; never estimate token totals.

Do not allow builders to edit `Docs/architecture/`. Documentation reconciliation
belongs to the doc-auditor lane after runtime evidence exists.

### Phase 5: Critic, wiring, and defense gates

Run these independently in parallel:

- `corpus_critic`: correctness, stubs, regression risk, missing tests
- `corpus_wiring_auditor`: activation, registration, persistence, and end-to-end reachability
- `corpus_defense_auditor`: constitutional permission path, trust boundaries, telemetry, and failure visibility

Use `scripts/verify_surfaces.py` with
`references/surfaces.yaml` for machine-checkable surface claims. Treat
`AMBIGUOUS` as unresolved until the wiring auditor adjudicates it.

Gate: no critical finding remains and every applicable surface is PASS or has an
explicit accepted exception.

### Phase 6: Verification ladder

Run the narrowest decisive commands first:

1. targeted package tests
2. touched-subsystem integration tests
3. Mangle validation for changed `.mg` files
4. `go test ./...`
5. `go build ./cmd/nerd` with the repository sqlite-vec headers
6. focused campaign/stress validation when the change affects orchestration,
   scheduling, memory, recovery, or long-horizon execution

A passing build does not override a failed behavioral or wiring gate.

### Phase 7: Reconcile and report

Dispatch `corpus_doc_auditor` and `corpus_governance_reconciler` only after
the runtime gates pass.

Update:

- `IMPLEMENTED_SPEC.md` from measured evidence
- relevant TODO/open-question rows
- architecture index or progress surfaces
- the corpus-build ledger and journal
- scoped `AGENTS.md` guidance after large structural refactors

Report the before/after gap matrix, packet outcomes, validation commands, wiring
verdict, residual risks, and any remediation packet. Keep the user-facing report
compact.

## Hook and safety contract

Codex hook ownership lives in `.codex/hooks.json`; handlers live under
`.codex/hooks/corpus-build/`.

- `SubagentStart` and `SubagentStop` record governed fleet activity.
- Global `SubagentStart` injects bounded shared agent-memory context when present.
- Exact token usage is recorded only when the stop payload exposes usage fields;
  otherwise telemetry says `unavailable`.
- No PreToolUse write-scope or compile guard is enabled: the imported source hook
  assumptions conflicted with codeNERD's live build/test contract.
- Hooks are guardrails, not complete security boundaries.
- The sandbox, command policy, constitutional Mangle policy, review gates, and
  tests remain authoritative.
- Never auto-merge, delete passing tests, fabricate evidence, or stage unrelated
  files.
- Never exceed three write-heavy workers concurrently.
- Never guess at token or cost totals.

## Package references

- `references/01-work-unit-types.md`
- `references/02-integration-surface-checklist.md`
- `references/03-dag-ordering-rules.md`
- `references/surfaces.yaml`
- `references/common/`
- `references/agent-skill-quality-rubric.md`
- `references/plans/`

## Completion standard

A corpus-build run is complete only when:

1. every accepted requirement maps to code, an explicit non-code artifact, or a
   justified residual;
2. the live execution path is wired;
3. Mangle declarations and policy are valid;
4. JIT prompt behavior uses atoms rather than ad-hoc shard prose;
5. targeted and repo-level verification pass;
6. the architecture corpus is reconciled from evidence;
7. the ledger contains no unexplained skip or active packet.
