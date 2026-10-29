# Campaign: the right context, at the right time, for the right thing

## The idea, stated once (the architect's words, 2026-09-04)

codeNERD is an agentic coding harness whose features must compose fluidly so
that on any coding task — from brainstorming and ideation, through design,
planning, implementation, and verification, to hardening and bug fixing — the
system gives the agent exactly the context it needs, at the moment it needs
it, for the thing it is about to do, and nothing else. The external yardstick
for the bug-fixing slice is SWE-bench. The internal measure, per lifecycle
stage, is tasks completed correctly per token, and context precision (what the
model rated helpful, noise, or missing).

The harness never declares its own completion: instrument the lifecycle, let
the feedback loop drive context selection, and let the numbers report what
they report.

## What already exists — verify, do not redo

Landed 2026-09-04 (commits on main; each phase confirms before touching):

- `turn_cost/6` per turn with real token counts on every path, including
  campaign-spawned shards (5aaaf96c, 37ee811b, 0e0905d3, 811a7ca6).
- `turn_evidence/6` → `hollow_success/1` → `turn_done/1` for every turn
  (30094744); read-only turns verified, not failed.
- Memory read-back on the executor path (`session.MemoryHydrator`,
  `HydrateLearnings` once, `HydrateSessionContext` per turn).
- Campaign orchestrator: Cortex boot for start and resume; transitive upstream
  artifacts into dependent tasks; verify tasks fail hollow reports; structured
  `checkpoint_verdict/4` reaching the kernel; tactile children carrying the
  project build environment; re-plan output defended like plan output; block
  reasons persisted; one-shot re-plan at the attempt cap.
- Composition seams S1–S12 mapped in
  `composition-map-2026-09-03.md`; a paused campaign (aa202a5c) holds
  phase-0 verification artifacts under `.nerd/campaigns/aa202a5c/artifacts/`
  and a git stash holds its half-finished S1/S2 edits. Those seams are
  plumbing this campaign pulls in only where a stage measurement shows the
  seam is the bottleneck.
- The audit report `.nerd/campaigns/5a2f4c8d/artifacts/retrieval_risk_report.md`
  lists eleven line-anchored defects in `internal/retrieval` — the issue-to-
  context pipeline the bug-fixing slice runs on.

## Phase 0 — Close the context feedback loop (the learning mechanism)

Every Piggyback response carries `control_packet.context_feedback`:
`overall_usefulness`, `helpful_facts`, `noise_facts`, `missing_context`
(`internal/articulation/protocol_types.go`). A `ContextFeedbackStore`
(`internal/context/feedback_store.go`) learns per-predicate usefulness per
intent and feeds spreading-activation scoring (`internal/context/activation.go`).
Today it is wired only on the TUI path (`cmd/nerd/chat/process.go:903`).
The session executor — the universal loop behind `nerd chat`, every campaign
shard, every fix run — logs the block at
`internal/session/executor.go:1830-1838` and drops it. The model tells the
harness every turn what it was missing, and the harness does not learn.

1. Wire the feedback store onto the executor path the way hydration was wired
   (a capability on the factory adapter, set by the Cortex, called from
   `processPiggybackControlPacket`). Assert the same signal as facts so the
   kernel can reason over it: `context_feedback(SessionID, TurnNum, Usefulness)`,
   `context_fact_helpful(SessionID, TurnNum, Predicate)`,
   `context_fact_noise(SessionID, TurnNum, Predicate)`,
   `context_missing(SessionID, TurnNum, Description)` — `Decl` in the existing
   `internal/core/defaults/schemas_execution.mg`.
2. Make `missing_context` actionable: when the model names files or symbols it
   lacked, the next turn's context assembly resolves them (holographic file
   section, symbol facts) before the model asks again. Measure: repeated
   `missing_context` for the same target across consecutive turns → 0.
3. Record the injected inventory per turn (fact families, atom ids, byte
   sizes) as a durable fact or log line so "injected vs needed" is comparable.
4. Confirm the JIT selection the executor uses reads the feedback store's
   usefulness scores (activation scoring), not only the TUI's.

## Phase 1 — Lifecycle stage as a kernel fact, with stage-shaped context policy

1. Derive `task_stage/1` in policy from intent and world state: `/ideate`,
   `/design`, `/plan`, `/implement`, `/verify`, `/harden`, `/debug`,
   `/refactor`, `/review`. Intent → stage is a Mangle table, not a Go switch.
2. Per-stage context policy in Mangle: which fact families and atom
   categories are required, optional, and forbidden at each stage, feeding
   the existing `injectable_context` derivation. Budgets are structure: the
   tool list the model sees is shaped by stage and remaining budget.
3. First-class `/brainstorm` and `/design` intents with their own atoms and
   context — north star, architecture map, capabilities inventory, standing
   constraints (`nerd.md`, CLAUDE rules), prior decisions from memory — so
   ideation builds on what exists instead of re-deriving it.

## Phase 2 — Golden tasks per stage, on this repository, and the baseline

Three real tasks per stage, drawn from this codebase (open items in
`subsystem-pass-2026-09-04.md` are a ready source: e.g. items 39, 43, 44).
Run each through `nerd chat` via `scripts/chat_driver.py`. Capture per turn:
tokens (`turn_cost`), tool calls, injected inventory, `context_feedback`,
verdict (`turn_done` / `hollow_success`), wall time. Durable baseline report
per stage: tasks completed correctly per token, context precision
(helpful / noise / missing), repeated-miss count.

## Phase 3 — The bug-fixing slice: retrieval hardening and SWE-bench wiring

1. Fix the eleven findings in the retrieval risk report in fix-priority order
   (P1 `GetTopFiles` negative-n panic; P2+P3 path containment in `locateFile`
   and bounded `LoadContent`; P4+P5 issue-text and keyword caps with
   context-aware cancellation; …). One finding per task; tests pin each.
2. Wire a SWE-bench Lite run: the `nerd-evolve` skill's evaluation cascade
   names an L5 SWE-bench tier (`.claude/skills/nerd-evolve/references/02-evaluation-cascade.md`);
   find or build the runner that feeds an instance's issue text through
   `nerd run`/`nerd chat`, applies the patch in an isolated worktree, and
   runs the instance's tests. Run a small fixed subset; record pass rate,
   tokens per instance, context precision. This is the before-number.

## Phase 4 — Implementation and verification slices

Holographic file section for every file a turn touches and after every
write; `test_framework/1` and `build_command/2` as facts consumed by
verification and prompts (seam S3); the coder sees callers, covering tests,
and the build environment without reading for them. Measure on the phase-2
implement/verify golden tasks: reads per task fall, tokens per correct task
fall, verdict rate unchanged or better.

## Phase 5 — Hardening, review, and refactor slices

Reviewer and nemesis verdicts already reach the kernel; make `review_finding`
facts drive the next turn's context for the fix; refactors get the symbol
graph and write-set gating as facts; `denied_this_turn/2` removes a denied
tool for the rest of the turn (seam S10). Measure on the harden/review/refactor
golden tasks.

## Phase 6 — Re-measure and report

Re-run phases 2 and 3's measurements. Report per stage: tasks completed
correctly per token, context precision, repeated-miss count, SWE-bench subset
pass rate, before versus after. One regression scenario per stage in the
existing regression harness. Then pull in whichever composition seam the
numbers say is the next bottleneck.

## Rules for every task

- One concern per task; smallest correct diff; a unit test pinning the
  deterministic behaviour; `go build ./...` and the touched packages' tests
  green; tree buildable at the end of every task.
- New model-facing text is a prompt atom under `internal/prompt/atoms/`;
  decisions are Mangle facts and rules, not Go switches or prose.
- Mangle: `Decl` before use, never the same predicate at the same arity
  twice, bound variables before negation, aggregation via `|> do ... let`.
- Do not touch what `nerd.md` forbids (`constitution.mg`, `coder_quality.mg`,
  `modularity.go`, `write_guards.go`, the tripwire test, `.nerd/config.json`,
  `.nerd/mangle/learned`). Never widen a permission; `delete_file` stays a
  human decision; file constitution edits as findings for the human.
- Durable reports go under the campaign's own artifacts directory, never the
  repository root.
- Cross-package refactors are filed as findings with the exact seam, not
  started mid-task. Targeted `go test ./<pkg>/`; the full suite once per phase.
- When a task creates a file, type it `/file_create`; when it edits one,
  `/file_modify`; the orchestrator enforces that contract.
