---
schema: nerd/v1
project: codeNERD
language: go
commands:
  build: go build ./...
  test: go test -count=1 ./...
  lint: go vet -tags sqlite_vec ./...
  env:
    CGO_CFLAGS: -IC:/CodeProjects/codeNERD/sqlite_headers
generators:
  # Checked-in files built from sources. The regenerate tool runs one by id;
  # its outputs are the call's writes. The freshness checks that fail when
  # one is stale name the generator.
  - id: atom-importance
    run: go run ./cmd/tools/prompt_builder -score-importance
    outputs: [internal/prompt/atom_importance.json]
    when: after adding or editing an atom YAML under internal/prompt/atoms (scores new or changed atoms through the configured Jev judge)
  - id: prompt-corpus
    run: go run ./cmd/tools/prompt_builder -reconcile-embedded
    outputs: [internal/core/defaults/prompt_corpus.db]
    when: after adding, editing or deleting an atom YAML under internal/prompt/atoms (run atom-importance first)
  - id: predicate-corpus
    run: go run ./cmd/tools/predicate_corpus_builder
    outputs: [internal/core/defaults/predicate_corpus.db]
    when: after adding, changing or removing a Decl in any .mg under internal/core/defaults
  - id: dark-fields-baseline
    run: go run ./cmd/tools/audit_dark_fields -update
    outputs: [scripts/testdata/dark-fields-baseline.txt]
    when: after wiring a previously dark struct field so it is now written in production, or intentionally recording a new dark field with its reason
  - id: deadcode-baseline
    run: bash ./scripts/deadcode-budget.sh --update
    outputs: [scripts/testdata/deadcode-baseline.txt]
    when: after wiring or removing a previously unreachable function so the dead-code baseline shrinks
  - id: json-errors-baseline
    run: go run ./cmd/tools/audit_json_errors -update
    outputs: [scripts/testdata/json-errors-baseline.txt]
    when: after handling a previously discarded JSON error so the baseline shrinks, or recording a genuinely safe struct-of-strings/ints Marshal
  - id: agent-docs
    run: go run ./cmd/tools/agent_docs
    outputs:
      - CLAUDE.md
      - Docs/architecture/CLAUDE.md
      - Docs/architecture/broker/CLAUDE.md
      - Docs/architecture/broker/nerd.md
      - Docs/architecture/nerd.md
      - Docs/live-validation/polystack/CLAUDE.md
      - Docs/live-validation/polystack/nerd.md
      - cmd/nerd/chat/CLAUDE.md
      - cmd/nerd/chat/nerd.md
      - internal/articulation/CLAUDE.md
      - internal/articulation/nerd.md
      - internal/autopoiesis/CLAUDE.md
      - internal/autopoiesis/nerd.md
      - internal/browser/CLAUDE.md
      - internal/browser/nerd.md
      - internal/campaign/CLAUDE.md
      - internal/campaign/nerd.md
      - internal/config/CLAUDE.md
      - internal/config/nerd.md
      - internal/core/CLAUDE.md
      - internal/core/nerd.md
      - internal/embedding/CLAUDE.md
      - internal/embedding/nerd.md
      - internal/evidence/CLAUDE.md
      - internal/evidence/nerd.md
      - internal/init/CLAUDE.md
      - internal/init/nerd.md
      - internal/logging/CLAUDE.md
      - internal/logging/nerd.md
      - internal/mangle/CLAUDE.md
      - internal/mangle/nerd.md
      - internal/northstar/CLAUDE.md
      - internal/northstar/nerd.md
      - internal/orient/CLAUDE.md
      - internal/orient/nerd.md
      - internal/perception/CLAUDE.md
      - internal/perception/nerd.md
      - internal/processutil/CLAUDE.md
      - internal/processutil/nerd.md
      - internal/prompt/CLAUDE.md
      - internal/prompt/atoms/northstar/CLAUDE.md
      - internal/prompt/atoms/northstar/nerd.md
      - internal/prompt/nerd.md
      - internal/session/CLAUDE.md
      - internal/session/nerd.md
      - internal/shards/CLAUDE.md
      - internal/shards/nerd.md
      - internal/store/CLAUDE.md
      - internal/store/nerd.md
      - internal/system/CLAUDE.md
      - internal/system/nerd.md
      - internal/tactile/CLAUDE.md
      - internal/tactile/nerd.md
      - internal/tools/CLAUDE.md
      - internal/tools/nerd.md
      - internal/tools/research/CLAUDE.md
      - internal/tools/research/nerd.md
      - internal/transparency/CLAUDE.md
      - internal/transparency/nerd.md
      - internal/workspace/CLAUDE.md
      - internal/workspace/nerd.md
      - internal/world/CLAUDE.md
      - internal/world/nerd.md
      - nerd.md
      - tests/e2e/CLAUDE.md
      - tests/e2e/nerd.md
    when: after adding or editing any agents.md or AGENTS.md (each directory's CLAUDE.md is a byte copy and its nerd.md body is the agents.md text; nerd.md frontmatter is kept)
gates:
  # Per-node gates: recurse measures each node of the derived DAG with these,
  # so a visit re-runs the node's own vet and tests, not the whole suite.
  # commands.test (go test -count=1 ./...) stays the workspace-wide gate every
  # pass closes on. commands.build/lint/test are the elite ladder's G1-G3
  # (Docs/journeys/05-elite-harness-ladder.md, "the gates").
  - id: go-vet
    kind: lint
    run: go vet -tags sqlite_vec {pkg}
    scope: node
  - id: go-test
    kind: test
    run: go test -count=1 -cover {pkg}
    scope: node
  # The audits CI enforces (.github/workflows/ci.yml), so the loop holds
  # every change to the bar a PR is held to.
  - id: test-bodies
    kind: audit
    run: go run ./cmd/tools/audit_test_bodies .
  - id: committed-binaries
    kind: audit
    run: go run ./cmd/tools/audit_committed_binaries
  - id: json-errors
    kind: audit
    run: go run ./cmd/tools/audit_json_errors
  - id: dark-fields
    kind: audit
    run: go run ./cmd/tools/audit_dark_fields
  - id: parallel-globals
    kind: audit
    run: go run ./cmd/tools/audit_parallel_globals
  - id: tiebreak
    kind: audit
    run: go run ./cmd/tools/audit_tiebreak
  - id: action-drift
    kind: audit
    run: go run ./cmd/tools/action_linter -mg-root internal/core/defaults -virtual-store internal/core/virtual_store_types.go
  - id: predicate-corpus
    kind: audit
    run: go run ./cmd/tools/predicate_corpus_builder -check
  # Ladder R6: every path, line and symbol Docs/architecture cites exists
  # (1a39a3c1). Doc findings name no Go package, so recurse reports them as
  # unattributed rather than sweeping them.
  - id: doc-citations
    kind: audit
    run: go run ./cmd/tools/audit_doc_citations
  - id: deadcode-budget
    kind: audit
    run: bash ./scripts/deadcode-budget.sh
docs:
  # The paths whose files are documentation: a write under one that is not
  # source code (the corpus.toml beside a docs corpus) owes what a document
  # owes -- no build, no test run. Source code keeps its class anywhere.
  - Docs
critical:
  # The paths whose loss or breakage is catastrophic for codeNERD: the risk
  # gate protects them, the Dreamer treats deleting them as catastrophic, and
  # the Northstar Guardian checks alignment on any change to them. These were
  # three lists of codeNERD's packages written into Go; they are declared here
  # so every other workspace declares its own.
  - internal/core
  - internal/mangle
  - internal/campaign
  - internal/perception
  - internal/articulation
  - internal/session
  - cmd/nerd
  - "*.mg"
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
  - match: internal/session/write_guards.go
    reason: >-
      The single entry point every pre-write guard runs through. It exists so
      the guard SET is one protected unit: a new guard is added here and
      inherits this protection, instead of becoming another deletable block in
      executor_tools.go. That file holds the whole tool-execution path and
      cannot be frozen, so the call into this one is the seam that must be.
  - match: internal/session/modularity_guard_verify_test.go
    reason: >-
      Contains the tripwire. TestVerifyModularityGuard_NewFileBlocked drives a
      violating write through executeToolCall and expects a refusal, so deleting
      the single runWriteGuards call turns the suite red. Protecting the test is
      what stops that failure from being edited away instead of fixed.
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
  - match: internal/usage/attempt_scope.go
    reason: The mission uncertainty interlock cannot be changed by its own attempt.
  - match: internal/perception/usage_track.go
    reason: >-
      Observes provider usage and preserves missing-attempt uncertainty.
      The model must not alter the accounting that constrains its own work.
  - match: internal/broker/broker.go
    reason: >-
      Owns inference admission, usage capture and settlement. Changes belong
      to the owner, outside the model's tool execution path.
  - match: internal/system/knowledge_refresh_scope.go
    reason: >-
      Suspends and joins knowledge refresh before mission admission. The
      model must not amend the boundary that isolates its bounded mission.
  - match: internal/system/knowledge_refresh_boot.go
    reason: >-
      Owns mission boot suppression and background refresh identity. These
      restrictions must remain outside the model's write authority.
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

# codeNERD

codeNERD is a high-assurance, logic-first CLI coding agent. The model is the creative center; the Mangle kernel is the executive. Logic determines reality; the model merely describes it.

## North Star

The current generation of coding agents makes a category error: it asks LLMs to do both creativity and executive control. codeNERD separates those roles. The LLM handles problem solving, synthesis, and insight; the deterministic Mangle layer handles planning, memory, orchestration, and safety.

This repo exists to make that split real in production: creative power with deterministic safety, long-horizon context without prompt drift, and parallel specialists whose behavior is grounded by logic rather than luck.

### Inversion of Control

- **LLM as creative center**: problem solving, synthesis, goal shaping, and insight.
- **Logic as executive**: planning, memory, orchestration, safety, and policy.
- **Transduction interface**: natural language and code are converted into formal atoms that the kernel can reason over.

### How This Shapes Runtime Changes

- The world model, durable facts, and CodeDOM are the model's primary codebase context. Use targeted, revision-aware source views when those representations need detail or refresh.
- Mangle manages the active working context throughout execution: relevance, retention, eviction, retrieval, and ordering. A growing tool transcript with occasional summarization does not fulfill this design. Evicted context must remain recoverable; stale evidence must not survive a source change as current truth.
- Models use typed, policy-mediated operations. Do not give codeNERD models free-form CLI/shell access by default or use it to bypass missing tool wiring. Build/test tools must constrain their inputs and expose structured results. Development agents may use their own shell to build and verify codeNERD.
- Bound each model request's context while allowing a task to continue as long as it makes progress within the user's constraints. Arbitrary tool-call counts are not task-completion criteria; detect stalls and repeated failures explicitly.
- Validate this architecture through the normal production entry paths with CodeDOM and world-model context enabled. Restricted-tool benchmarks are supplementary evidence.

## The Vision (Steve, 2026-09-18) — read before any architectural change

- This is an experimental project and a different paradigm. Mangle — a deductive database
  programming language, not Datalog — plus JIT prompt compilation is meant to be the
  replacement for subagents and skills: the harness decides, not the model's discretion.
- The harness forces. Test coverage is not optional: the agent cannot stop until the tests
  that actually harden the system exist. Domain knowledge is not at the LLM's discretion: it
  is pushed into the context window when the harness decides it is needed — by user request,
  task type, and the history of what has been built. The agent keeps working not until the
  task is done, but until the behavior the north star of that system envisions holds.
- It knows the codebase better than any other coding agent because it never has to grep
  around: the kernel has the math to hand the LLM what it needs — a blend of domain knowledge
  from SQLite, vector search, CodeDOM and history — injected at the right time, for the right
  reason, to the right agent, so the job finishes in as few turns as possible without wasting
  tool calls or thinking tokens double-checking.
- Quality and capability first. Tokens are the constraint, not the goal. Lost-in-the-middle
  is eliminated by context compression, pruning and ordering — where a fact sits in the
  window is a decision.
- Tools exist for exactly three things: condense the search space, reduce the turns to
  complete the task, and offload cognition to deterministic code (a change with a blast
  radius is carried out by the tool, not by the LLM hand-editing). A tool that does none of
  these is cruft.
- "Clean fixpoint," not "clean loop." Go is fine and may be as large as the work needs — it
  is the FFI, the drivers, the tools. The executive decisions — what a turn is, whether it is
  done, what is delegated and with what task, what enters the window, what the verdict is —
  are the fixpoint of the kernel over the facts. The drift to hunt is decisions computed in
  Go instead of derived.
- Complexity is not a defect. This may be a highly complex system; what is scored is whether
  a decision is derived and whether an obligation is forced.
- It hunts its own bugs and intelligently extends its own tooling, in Mangle and in Go.
- The north star is a final state, not a goal to be hit. It describes the behavior the
  finished system exhibits; the harness continuously derives the distance between what the
  evidence shows and that state, and applies that distance as pressure on every agent it
  runs. Nothing achieves the north star; every turn moves toward it, and the pressure never
  lets go.
- The original bet lives in `.codex/skills/codenerd-builder/references/` — an input to be
  judged, not a request to restore. `Docs/architecture/` is codeNERD's spec (Steve,
  2026-09-29), being rebuilt package by package to `Docs/journeys/09-architecture-doc-standard.md`.
  Its July-2026 files that have not been rewritten to that standard were generated one package
  at a time by a weak model and their claims have been found stale or never true: never cite
  those as evidence of what the code does. Read the code.

## Use codeNERD Where It Can Do the Job (a suggestion, not a mandate — for now)

codeNERD is meant to fix codeNERD, and every change routed through it is a measurement of
how close it is to that. So *prefer* aiming it at work it can plausibly land, and hand-edit
freely where it can't yet — this is a dogfood signal to collect, not a gate to pass.

```powershell
.\nerd.exe fix "<symptom, with the evidence: log lines, file:line, what was observed>"
```

What tends to land: one file, one named symptom, the evidence quoted, no diagnosis handed
over (a brief that carries the answer produces a stenographer). What tends not to: multi-file
causes, anything upstream of the file the symptom names, and tests — as of 2026-09-18 the
coder shard does not write them unless told, and its verdict can contradict its own evidence.

When you try it, record the outcome in the dogfood ledger
(`.claude/skills/codenerd-dogfood/references/component-ledger.md`): brief, minutes, tool
calls, what landed, what it missed. When you hand-edit instead, say so in the commit; if
codeNERD *couldn't* do it, say why — that is the finding, and fixing that blocker is the
highest-value dogfood there is.

Two things stay off-limits for codeNERD regardless: safety-gate and permission logic (the
model should not widen the rule that constrains it), and a broken build that stops it from
running at all.

## Repo Contract

- JIT is the standard for all new LLM-facing behavior.
- New prompt behavior becomes prompt atoms first, not ad-hoc shard prompt text.
- Internal prompt atoms live under `internal/prompt/atoms/<category>/`.
- Project-specific or user-agent prompt atoms live under `.nerd/agents/`.
- `Docs/architecture/` is the spec, and work is spec-first (Steve, 2026-09-29): a change starts by
  changing the spec of each package it touches (vision / capability spec / the `GAP-<PKG>-NN` row
  with a checkable exit, to `Docs/journeys/09-architecture-doc-standard.md`), then the code is
  driven to the spec, then current-state / `IMPLEMENTED_SPEC.md` / wiring rows are updated with
  cited `path:line` and the gap row is closed. Spec and code never drift apart: a change that
  leaves them misaligned is not done. A new package gets a corpus. (Much of the corpus is being
  written backwards from existing code right now; new work goes spec first.)
- codeNERD holds foreign repositories to the same loop: it discerns whether a repo's specs are
  ahead of the code, behind it, or missing; builds or repairs them (on init and as it works);
  changes the spec before the code; and drives code to spec and spec toward the north star.
- This repository is public: nothing from the user's private projects goes into it. See `.claude/rules/public-repo.md`.
- Always look for wiring gaps before deleting "unused" code. This codebase frequently has partially wired features and dormant integration points.
- Keep root-level agent guidance concise. Put subsystem detail in scoped `agents.md` files or skill references.
- Push to GitHub regularly and use conventional commits.

## Maintenance Rule For This File

- Keep this file focused on repo-wide instructions, not subsystem encyclopedias.
- Preserve the north star, hard requirements, live command snippets, and a current file map.
- Move deep reference material into scoped docs such as `internal/mangle/agents.md`, `internal/prompt/agents.md`, and `internal/core/agents.md`.
- When editing this file, verify that every path and command still works.
- Every `agents.md` (and `AGENTS.md`) outside the harness mirrors is the source for two generated twins in the same directory: a byte-identical `CLAUDE.md` (gitignored, for Claude Code) and the body of `nerd.md` (tracked, for codeNERD; an existing `nerd.md` keeps its frontmatter). Edit `agents.md`, then run `go run ./cmd/tools/agent_docs`; `TestAgentDocTwinsAreCurrent` fails while a twin is stale.

## Quick Commands

### Build With sqlite-vec Support (PowerShell)

```powershell
if (Test-Path .\nerd.exe) {
    Remove-Item .\nerd.exe -ErrorAction SilentlyContinue
}
$env:CGO_CFLAGS = "-IC:/CodeProjects/codeNERD/sqlite_headers"
go build -tags sqlite_vec -o nerd.exe ./cmd/nerd
```

**`-tags sqlite_vec` is required, not optional.** `internal/store/vec_support_enabled.go`
is behind `//go:build sqlite_vec && cgo`; without the tag the binary takes
`vec_support_disabled.go`, `defaultRequireVec` is false, and every boot logs
`sqlite-vec not available; falling back from ANN to lexical search` four times.
Nothing fails — the JIT selector's vector half just quietly degrades to lexical
matching. Setting `CGO_CFLAGS` alone does not enable it. Measured 2026-08-10:
four warnings per boot without the tag, zero with it.

### Build With sqlite-vec Support (bash)

```bash
rm -f ./nerd.exe
CGO_CFLAGS="-IC:/CodeProjects/codeNERD/sqlite_headers" go build -tags sqlite_vec -o nerd.exe ./cmd/nerd
```

SQLite headers live at `sqlite_headers/sqlite3.h`.

### Test

```bash
go test ./...
```

### Live Model Calls

- Live tests and probes against `muse-spark-1.3-contributor` and Jev (TypeSafe) are fine and encouraged; no per-run approval needed.
- Dogfooding (`nerd` run against its own source) may use every provider `.nerd/config.json` configures.
- Use the endpoints and keys in `.nerd/config.json`; never print or commit a key.

### Stress / Long-Horizon Validation

- In chat mode, run `/campaign assault ...`.
- Artifacts persist under `.nerd/campaigns/<campaign>/assault/`.

If you see `debug_program_ERROR.mg`, the system crashed and dumped combined `.mg` sources into that file for debugging.

## Quick Reference

- **OODA loop**: Observe -> Orient -> Decide -> Act.
- **Fact flow**: user input -> perception -> `user_intent` -> kernel derives `next_action` -> VirtualStore executes -> articulation responds.
- **Constitutional safety**: every action must derive `permitted(...)`; default deny.
- **nerd.md frontmatter is enforced, not advice**: `forbid` paths become `project_forbidden_path` facts at boot and are checked before any write-mutation tool runs.

## Working Map

| Area | Live location | Notes |
|------|---------------|-------|
| Kernel | `internal/core/kernel.go` | Package marker; implementation is split across `kernel_*.go`. |
| Policy | `internal/core/defaults/policy/` | Modular Mangle policy corpus. |
| Schemas | `internal/core/defaults/schemas.mg` | Core schema declarations. |
| Prompt compiler | `internal/prompt/compiler.go` | JIT prompt compilation and atom selection. |
| Prompt assembly | `internal/articulation/prompt_assembler.go` | Runtime prompt assembly bridge. |
| Prompt atoms | `internal/prompt/atoms/` | Canonical atom library. |
| Session execution | `internal/session/executor.go` | Clean execution loop. |
| Shard lifecycle | `internal/core/shards/manager.go` | Shard manager and lifecycle plumbing. |
| Shard registration | `internal/shards/registration.go` | Registers domain and system shards. |
| VirtualStore | `internal/core/virtual_store.go` | Action routing and external integration. |
| Research tools | `internal/tools/research/context7.go` | Context7-backed research tooling. |

## When Working In Specific Areas

- Prompt or shard-behavior changes: read `internal/prompt/agents.md` first.
- Mangle logic changes: read `internal/mangle/agents.md` before editing `.mg` files.
- Core runtime or execution changes: read `internal/core/agents.md` and `internal/session/README.md`.
- Integration or "code exists but doesn't run" issues: audit wiring before deleting code or declaring a system unused.
- Specialized references live under `.claude/skills/` and `.codex/skills/`.

## Mangle Guardrails

- All predicates need `Decl` before use.
- Variables are uppercase; atoms use `/lowercase`.
- Negation only works when variables are already bound by positive atoms.
- Aggregation must use `|> do ... let ...` pipeline syntax.
- Do not use Mangle for fuzzy matching or large natural-language pattern banks. Use embeddings or other retrieval first, then assert structured facts into the kernel.
- For deeper syntax, failure modes, and examples, use `internal/mangle/agents.md` and the Mangle skill references instead of expanding this root file.

## Development Guidelines

- Run `go test ./...` before handoff.
- Keep new LLM systems JIT-first.
- Prefer adding prompt atoms and selection logic over hardcoding prose in shards.
- When in doubt, preserve the architectural north star and trim encyclopedic detail.

## Grok Harness

Grok Build project wiring lives under `.grok/` (rules, agents, personas, roles, skills). Domain skills remain under `.agents/skills/`. Orient with `/codenerd-session` or `grok inspect`.

## Deep References

- `.claude/skills/codenerd-builder/references/`
- `.claude/skills/mangle-programming/references/`
- `.codex/skills/` mirrors these skills for Codex-oriented workflows.
