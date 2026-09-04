# Campaign: make codeNERD compose the way it was designed to

## The idea, stated once

codeNERD is a partnership with a strict division of labour: perception turns
language into facts; the kernel derives what is true and what is allowed
(`next_action`, `permitted/3`, persona, routing, completion) from facts and
stratified policy; the VirtualStore is the only door to the world; the session
executor is one universal loop; context is the Logical Twin (facts dense,
language disposable); prompts are compiled from atoms; articulation carries a
surface stream for the human and a control stream of atoms for the kernel;
memory persists facts and is read back on the live path.

Every place where a Go heuristic, a prose string, or the model itself does one
of the kernel's jobs is a composition break: tokens are spent re-deriving what
the harness already knows, and two subsystems can disagree about one decision.
**Objective: restore the composition — one source of truth per decision, owned
by the subsystem the design gives it to — so fewer tokens buy more verified
work with higher accuracy and reasoning.** Latency is a consequence. Never
remove a check to save tokens; replace it with a deterministic one.

## The map is already done — use it, verify it, do not redo it

`.claude/skills/codenerd-dogfood/references/composition-map-2026-09-03.md`
holds the as-built composition, the contract/orphan table, nine composition
breaks (a–i), what the harness re-derives, the memory story, and twelve ranked
seams S1–S12 with owners, inputs and measures. Read it first. Each phase below
names its seams; each task verifies the cited lines before changing anything
and records what it found if the map was wrong.

## Already landed on main before this campaign (verify, do not redo)

These were built and measured live on 2026-09-04; each phase below that names
one of them starts by confirming the cited code exists and then skips it.

- `turn_cost/6` (Phase 0.1): `Decl` in `internal/core/defaults/schemas_execution.mg`;
  written by `assertTurnCost` in `internal/session/executor_memory.go` with
  per-session token counts from `usage.WithSessionID`/`Tracker.SessionTokens`
  (commits 5aaaf96c, 37ee811b). Measured: `prompt=26497 completion=3631 tools=1`
  on a tool-using turn.
- Turn evidence and the single verdict (Phase 2, second half): the executor
  asserts `turn_evidence/6` for every non-dream turn and policy in
  `internal/core/defaults/policy/coder_safety.mg` derives `hollow_success/1`
  and `turn_done/1`; read-only turns are verified, not failed (30094744).
- Memory read-back on the live path (Phase 3, memory half):
  `session.MemoryHydrator` is implemented by the factory adapter and called in
  `ProcessWithIntent` — `HydrateLearnings` once, `HydrateSessionContext` per
  turn (5aaaf96c, 37ee811b); `atomsJSON` comes from `compilationAtomsJSON`.
- Dreamer gate on the executor path (16e830d8, 8319abef): modifications hit
  `/critical_path_hit` only under `.git`.
- Campaign orchestrator: start and resume boot through the Cortex
  (6b6eefa3, 00ce937f); dependent tasks receive upstream artifacts, transitively
  over phase dependencies, and verify tasks fail a findings-free report
  (54ba9912, 11faf065); blocked campaigns re-arm via `PrepareResume`
  (d5dd27da, 9e77ffd3).
- Reviewer verdicts as facts (S7, first half): `checkpoint_verdict/4` now
  crosses every hop — prompt example with the trailing period, articulation
  validator, executor allowlist, kernel, checkpoint parser — with stale
  verdicts retracted before the reviewer spawns (c202a520, 43f9e043); the
  articulation validator logs every dropped update with its reason (66d4a79a).
  Still open in S7/S11: `phase_complete/1` / `campaign_complete/1` consumers.
- Metering on every LLM call: `sessionLLMAdapter` attaches the usage tracker
  when the context lacks one (0e0905d3), so `turn_cost` is non-zero for
  campaign-spawned shards. The ten hand-tagging sites in `cmd/nerd` are now
  redundant and are a deletion task, not a design question.

Still open from the map: S1, S2 (break (a) unverified), S3–S6 (the harness
detectors and incremental scan), S7/S11 (reviewer verdicts as facts), S4/S10,
S8/S9/S12.

## Phases, in order

### Phase 0 — The denominator, and a verified map
1. Add the `turn_cost(SessionID, TurnNum, PromptTokens, CompletionTokens,
   ToolCalls, VerifiedOutcome)` fact. Its `Decl` goes into the EXISTING
   canonical schema file `internal/core/defaults/schemas.mg` (a modify, not a
   new file — do not invent new schema directories); it is written by
   `persistTurn` in `internal/session/executor.go` and persisted through the
   existing LocalDB session-turn path, so tokens per verified unit of work is
   measurable from the kernel and from `.nerd/logs`. Wire the usage numbers
   the clients already track (`internal/usage`). Test it. When a task creates
   a file, type it `/file_create`; when it edits one, `/file_modify` — the
   orchestrator enforces that contract transactionally.
2. Verify break (a): reproduce or refute that the executor's
   `pending_action/5` can be consumed by `ConstitutionGateShard` /
   `TactileRouterShard` during the executor's own `permitted/3` query, and
   whether a tool can run twice. Durable report with evidence either way;
   this decides Phase 1's shape.
3. Record a baseline: run the three hard-engineering asks in the ledger
   (session A2) through `nerd chat` and capture LLM calls, tool calls and
   tokens per turn. This is the before-measurement for every later phase.

### Phase 1 — One executive: S2 then S1
S2: namespace the interactive gate's fact (`interactive_pending_action/5`) or
scope the two shards to their own producers so nothing re-decides or re-runs
the executor's action; make the second pipeline's scope explicit. S1: a single
`tool_capability` predicate in `internal/core/defaults/policy` from which
AllowedTools, `safe_action`, the router table and the Dreamer map are projected
or validated; `apply_edits` gets its `safe_action` and gate mapping through
that table, nowhere else. Tests: every registered tool appears in exactly one
capability row; a table test proves the projections agree.

### Phase 2 — Decisions as facts: S7, S11, plus completion
Reviewer verdicts arrive as structured control-packet output and become
`checkpoint_verdict/4`; `phase_complete/1` and `campaign_complete/1` get their
Go consumers; substring parsing of PASS/FAIL is removed. The executor asserts
`turn_evidence(...)` (write counts, written paths, test executions, build/test
verdicts, validator outcomes) and policy derives `hollow_success/1` and a
single `turn_done/1`; the TUI continuation protocol
(`cmd/nerd/chat/process_continuation.go:169-211`), the learning trace and the
audit event all read that one predicate. Tests: a write-oriented turn with no
write cannot derive done; a failed build cannot derive done.

### Phase 3 — The harness tells the model what it knows: S3, S5, S6
`test_framework/1` and `build_command/2` derived once and consumed by
checkpoint, build/test verification and prompt facts; the four Go detectors
go. Incremental scan on every Cortex boot; holographic file section for every
file a turn touches and after every write. `HydrateLearnings` and
`HydrateSessionContext` called in `ProcessWithIntent`; `atomsJSON` persisted
from the Piggyback control packet. Tests: a fixture turn whose files the
kernel describes issues fewer reads and the same answer; a memory op written
in turn N is present in turn N+1.

### Phase 4 — Budgets and denials as structure: S4, S10
Shrink `toolDefs` monotonically as the budget drains (search/read dropped at
"tight", write and verify kept at "critical") using the mechanism
`forceFinalAnswer` already proves; on a safety denial drop the tool for the
rest of the turn and assert `denied_this_turn/2`. Prose nudges become one atom
rendered from budget facts. Tests: no repeat denial of the same (action,
target) in a turn; tokens after the tight threshold fall.

### Phase 5 — Prompts and intent: S8, S9, S12, perception atoms
One `CompilationContext` constructor in `internal/prompt` used by the executor
and the PromptAssembler; the perception atom contract fixed so the JIT prompt
is what perception uses (`internal/perception/understanding_adapter.go`) and
the embedded string retired; preset intents extended to unambiguous chat
surfaces (slash commands, `/consult/<agent>`, single-verb imperatives with a
resolved path); repair and critic prompts moved to atoms selected on
`world_state`. Tests: perception manifest lists atoms; classification calls
per session fall on the fixed ask set.

### Phase 6 — Measure and report
Re-run the Phase 0 baseline; report tokens per verified unit of work, LLM
calls per turn by purpose, tool calls per turn, and denial/hollow rates
before and after, from `turn_cost` facts and the logs. One regression scenario
per phase in the existing regression harness.

## Rules for every task

- One seam slice per task; smallest correct diff; a unit test pinning the
  deterministic behaviour; `go build ./...` and the touched packages' tests
  green; the tree buildable at the end of every task.
- Mangle: `Decl` before use, never declare a predicate twice at the same
  arity, bound variables before negation, aggregation via `|> do ... let`,
  premise order aware of the intermediate fact limit
  (`internal/mangle/agents.md`).
- New model-facing text is a prompt atom under `internal/prompt/atoms/`.
- Do not touch what `nerd.md` forbids (`constitution.mg`, `coder_quality.mg`,
  `modularity.go`, `write_guards.go`, the tripwire test, `.nerd/config.json`,
  `.nerd/mangle/learned`). Where S1 needs `safe_action` to follow the
  capability table, add the projection beside the constitution and file the
  constitution edit as a finding for the human. Never widen a permission;
  `delete_file` stays a human decision.
- Cross-package refactors are filed as findings with the exact seam, not
  started mid-task. Prefer targeted `go test ./<pkg>/`; the full suite once
  per phase.
- Persist every finding and measurement as a durable report so the checkpoint
  reviewer can read it.
