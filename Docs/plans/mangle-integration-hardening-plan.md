# Mangle Integration Hardening Plan (Wiring Gaps + Architectural Repair)

**Status:** Living plan (actively being implemented)  
**Owner:** codeNERD core team  
**Last updated:** 2025-12-13  
**Scope:** Mangle usage in codeNERD (kernel, policy, shards, routing, JIT, autopoiesis)  
**Primary goal:** Make “Logic determines Reality” *true at runtime* by eliminating drift between:  
- Mangle schemas/policy (`internal/core/defaults/*.mg`)  
- Go orchestration (system shards + shard manager + virtual store)  
- Prompt/JIT control-plane (prompt atoms, control packets, GCD)

---

## 0) What is Mangle? (quick but correct)

**Mangle** (from Google: `github.com/google/mangle`) is a Datalog-like logic language + engine:

- **Facts (EDB)**: ground truths you assert, e.g. `user_intent(/current_intent, /mutation, /fix, "internal/core/kernel.go", "").`
- **Rules (IDB)**: implications that derive new facts, e.g. `next_action(/delegate_coder) :- user_intent(/current_intent, /mutation, /fix, _, _).`
- **Fixpoint evaluation**: the engine repeatedly applies rules until no new facts can be derived.

Why it matters for codeNERD:

- It’s the deterministic **Executive**: it decides `next_action/1`, safety gates, and context selection.
- The LLM is the **Creative center**: it proposes text/patches, but it should *not* be responsible for orchestration correctness.

---

## 1) Current architecture (how codeNERD uses Mangle)

### 1.1 The kernel “brainstem”

Core runtime:

- `internal/core/kernel.go`: loads schemas + policy, ingests facts, evaluates to fixpoint, answers queries.
- `internal/core/defaults/schemas.mg`: predicate declarations (“schema-first”).
- `internal/core/defaults/policy.mg`: executive logic rules (IDB).
- Autopoiesis learned rules: layered on top (learned rules must never violate constitutional rules).

### 1.2 The OODA loop (system shards + facts)

Today’s runtime OODA loop is implemented as system shards that exchange facts:

- **Observe**: `perception_firewall` emits `user_intent/5` + grounding facts.
- **Decide**: `executive_policy` queries Mangle-derived `next_action/1` and emits `pending_action/5`.
- **Act (safety)**: `constitution_gate` checks `pending_action/5` and emits `permitted_action/5` (or denial facts).
- **Act (routing/execution)**: `tactile_router` maps actions to tools/VirtualStore calls and emits `routing_result/4`.

Key wiring files:

- `internal/shards/system/perception.go`
- `internal/shards/system/executive.go`
- `internal/shards/system/constitution.go`
- `internal/shards/system/router.go`
- `internal/core/virtual_store.go`

### 1.3 JIT prompt compilation (logic-driven context assembly)

The “prompt system” is itself part of the logic-first control plane:

- Prompt atoms live under `internal/prompt/atoms/**`
- JIT selects atoms based on Mangle predicates like `selected_result/3`, `mandatory_selection/1`, etc.

This must remain consistent with:

- Control packets emitted by the LLM (Piggyback protocol)
- The GCD (grammar constrained decoding) validator

---

## 2) Hard requirement: eliminate drift (the root problem)

codeNERD’s biggest failure mode is *drift*:

- **Policy** says one thing
- **Go orchestration** does another
- **VirtualStore** supports a third vocabulary

Drift causes the classic “code exists but doesn’t run” symptom.

This plan is a drift-reduction roadmap.

---

## 3) “Done now” (critical fixes already implemented in working tree)

These are high-leverage fixes that unblock correct executive behavior.

### 3.1 Canonical `user_intent` scoping (`/current_intent`)

**Problem:** Policy rules are scoped to `user_intent(/current_intent, ...)`, but PerceptionFirewall previously emitted `/intent_<ts>` IDs. Chat would *not* re-assert `/current_intent` when PerceptionFirewall was running, so policy couldn’t “see” the current intent.

**Fix:** Use a stable intent ID and retract stale facts each turn.

- `internal/shards/system/perception.go`: emits `user_intent` with `intentID := "/current_intent"` and retracts prior `user_intent(/current_intent, ...)` + `processed_intent(/current_intent)`.
- `internal/shards/system/executive.go`: `latestUserIntent()` prefers `/current_intent` if present, so action hydration works even without timestamp IDs.

### 3.2 Campaign queries must use current intent

**Problem:** Campaign rules used `user_intent(_, /query, ...)`, allowing *non-user* synthetic intents to trigger campaign query actions.

**Fix:** Scope campaign query triggers to the canonical intent.

- `internal/core/defaults/campaign_rules.mg`: status/progress queries now use `user_intent(/current_intent, /query, ...)`.

### 3.3 Tool-routing context must not pollute campaign detection

**Problem:** Tool relevance derivation in `ShardManager` asserted a synthetic `user_intent(..., /mutation, ...)`. During active campaigns, the campaign rules could mistake this for a real user mutation intent.

**Fix:** Use a dedicated ID and category for tool-routing context and avoid `current_time` accumulation.

- `internal/core/shard_manager.go`:
  - `intentID := "/tool_routing_context"`
  - category set to `"/routing"` (not `/mutation`)
  - retract stale `user_intent(/tool_routing_context, ...)`
  - retract `current_time` before asserting new timestamp

### 3.4 `action_permitted/1` must use ActionID (not ActionType)

**Problem:** Schema declares `action_permitted(ActionID)`, but ConstitutionGate used the action type (e.g. `/read_file`) as the argument, and Router retracted by action type.

**Fix:** Use the actual `ActionID` envelope consistently:

- `internal/shards/system/constitution.go`: assert `action_permitted(actionID)`
- `internal/shards/system/router.go`: retract `action_permitted(actionID)`

### 3.5 Docs plan tracking (don’t gitignore plans)

**Problem:** `Docs/*` + `*.md` ignore rules prevent `Docs/plans/**.md` from being tracked, defeating “plan as artifact”.

**Fix:** Unignore `Docs/plans/**`:

- `.gitignore`: allow `Docs/plans/**` and `Docs/plans/**/*.md`

---

## 4) Still broken / still drifting (must fix next)

This is the core of “do all fixes”.

### 4.1 Policy vs system-shard coordination mismatch

Symptoms:

- Policy describes a rich action pipeline using predicates like `pending_permission_check/1` and `permission_check_result/4`.
- Go pipeline uses `pending_action/5` and `permitted_action/5` directly.

Risk:

- Observability predicates (`ooda_phase`, `action_ready_for_routing`) don’t reflect reality.
- Autopoiesis learns rules against phantom predicates.

Fix options (choose one; do not mix):

1) **Make Go match policy** (preferred long-term)
   - ConstitutionGate asserts `permission_check_result(ActionID, /permit|/deny, Reason, Timestamp)`
   - Router asserts `ready_for_routing(ActionID)` before executing and retracts it after.
   - Policy derives `action_permitted/1`, `action_blocked/2`, etc. from `permission_check_result/4`.

2) **Make policy match Go** (faster, but reduces "logic owns orchestration")
   - Add IDB rules that derive the policy predicates from `pending_action/5`, `permitted_action/5`, and `routing_result/4`.
   - Example bridging rules:
     - `pending_permission_check(ActionID) :- pending_action(ActionID, _, _, _, _).`
     - `permission_check_result(ActionID, /permit, "", _) :- permitted_action(ActionID, _, _, _, _).`
     - `permission_check_result(ActionID, /deny, Reason, _) :- routing_result(ActionID, /failure, Reason, _).` *(only if denials always emit routing_result)*

Acceptance criteria:

- `current_ooda_phase/1` aligns with runtime states.
- A single action envelope can be traced across shards using `ActionID`.

### 4.2 Intent lifecycle truth: perception vs executive

Problem:

- `processed_intent/1` is currently used as “processed by perception”, but policy text sometimes treats it as “processed by executive”.

Fix:

- Introduce a new fact `executive_processed_intent(IntentID)` (or `intent_consumed_by_executive/1`).
- Perception retracts it when setting `/current_intent`.
- Executive asserts it once it emits the first `pending_action` for that intent.

Acceptance:

- `pending_intent(/current_intent)` is true only between Perception and Executive.

### 4.3 Reduce kernel churn (performance correctness)

Problem:

- Many call sites do `Assert(...)` repeatedly, forcing full re-evaluation on every fact.

Fix:

- Prefer batch ingestion:
  - `LoadFacts([]Fact)` for many facts
  - Or a new kernel API: `AssertWithoutEval` + `Evaluate()` exposed via interface (if needed)

Acceptance:

- Large world scans and campaigns do not degrade exponentially with time.

### 4.4 Action vocabulary drift prevention (policy ⇄ router ⇄ virtual store)

Problem:

- Action strings appear in multiple places:
  - Policy (`next_action(/xyz)`)
  - Router table (`ActionPattern: "xyz"`)
  - VirtualStore action types (`ActionType(...)`)

Fix:

- Build a `cmd/tools/action_linter`:
  - Parses policy for `next_action(/...)`
  - Parses router’s DefaultRoutes
  - Parses VirtualStore action type registry
  - Reports: missing routes, missing executors, unused routes, alias drift

Acceptance:

- CI fails if a policy action has no route/executor.

### 4.5 Make tool-routing context side-effect free

Problem:

- Tool relevance derivation currently mutates kernel state (`current_intent`, `user_intent`, `current_time`).

Fix (hard but correct):

- Use a *separate* ephemeral kernel instance for scoring, OR
- Add a “context frame” abstraction:
  - assert context facts
  - query
  - retract exactly those context facts

Acceptance:

- Tool relevance queries cannot trigger unrelated policy rules.

---

## 5) GCD + streaming (hardening plan)

Streaming is a correctness boundary: it can bypass validation if we accept partial outputs.

### 5.1 Streaming gate for Piggyback JSON

Implemented direction (already present in `internal/perception/transducer.go`):

- Abort streams that emit `surface_response` before `control_packet`
- Abort streams that never start JSON within a bounded prefix
- Validate `control_packet.mangle_updates` *as soon as the control_packet object closes*

Next hardening steps:

- Add unit tests for:
  - `indexOfJSONKeyOutsideStrings`
  - `extractJSONObjectValueAfterKey`
  - early abort conditions (surface-before-control, no JSON start)
- Add metrics:
  - stream abort counts
  - fallback-to-nonstream counts
  - validation error frequency by model/provider

Acceptance:

- A malformed streaming output never mutates kernel facts.

### 5.2 Scheduler + tracing must preserve context

Implemented direction (already present in working tree):

- `internal/core/api_scheduler.go`: scheduler wrapper forwards shard tracing context and supports streaming while holding API slots.
- `internal/perception/tracing_client.go`: tracing wrapper supports streaming and works through wrappers by interface (not concrete type assertions).

Next steps:

- Add integration test: scheduled + traced streaming call doesn’t lose shard attribution.

---

## 6) Implementation roadmap (phased)

This is the step-by-step “do all fixes” path. Keep phases small; land with tests.

### Phase 0 — Make policy see the current intent (DONE)

- [x] PerceptionFirewall emits `/current_intent`
- [x] Executive prefers `/current_intent`
- [x] Campaign queries scoped to `/current_intent`
- [x] Tool-routing uses `/tool_routing_context` + `/routing`

### Phase 1 - Make action envelopes traceable and policy-observable (DONE)

- [x] Decide coordination strategy: "Go matches policy" vs "policy bridges Go"
- [x] Ensure every action has:
  - `pending_action(ActionID, ...)`
  - `permission_check_result(ActionID, /permit|/deny, Reason, Timestamp)`
  - `routing_result(ActionID, /success|/failure, Details, Timestamp)`
- [x] Add end-to-end test asserting these facts exist for a simple read action.

### Phase 2 - Drift linter (tooling)

- [x] Implement `cmd/tools/action_linter`
- [ ] Run it in CI (or pre-commit) and fail on:
  - policy action has no router route
  - router route has no virtual store executor
  - executor exists but policy never emits it (dead action)

### Phase 3 - Kernel performance + fact lifecycle

- [ ] Reduce rebuild frequency in high-volume ingestion paths
- [x] Define retention policy for:
  - `routing_result` (timestamped + pruned)
  - `permission_check_result` (timestamped + pruned)
  - `execution_result` (timestamped + pruned)
- [ ] Add "compaction" action that prunes old action logs into cold storage.

### Phase 4 - Autopoiesis safety (learned rules)

- [x] Learned rules must be stratified and bounded:
  - created-fact limits (already supported via `engine.WithCreatedFactLimit`)
  - deny-list predicates that learned rules cannot define (implemented; blocks `permitted/1`, `safe_action/1`, and other protected heads)
- [ ] Add a "learned rule quarantine" mode for first N runs.

---

## 7) Verification checklist (what to run)

### 7.1 Tests

- `go test ./...`

### 7.2 Build (sqlite-vec headers required)

- PowerShell:
  - `Remove-Item -Force nerd.exe -ErrorAction SilentlyContinue`
  - `$env:CGO_CFLAGS='-IC:/CodeProjects/codeNERD/sqlite_headers'`
  - `go build -o nerd.exe ./cmd/nerd`

### 7.3 Runtime sanity script

- Start `nerd chat`, enter a simple `/read` intent, confirm:
  - `user_intent(/current_intent, ...)` exists
  - `next_action/1` derives (executive sees it)
  - `pending_action/5` → `permitted_action/5` → `routing_result/4` chain completes

---

## 8) Notes / principles (to keep the system clean)

- Prefer **one canonical fact** over “same concept, 3 predicates”.
- Avoid using `user_intent/5` for non-user internal contexts; make a dedicated predicate or a dedicated ID + category.
- Any new “LLM system” must be:
  - JIT-driven (prompt atoms)
  - GCD-validated if it emits control packets
  - observable via facts

---

## 9) Appendix A — Action vocabulary map (policy ⇄ router ⇄ virtual store)

The fastest way to kill drift is to maintain a single authoritative map.
This appendix is both a **checklist** and a **spec**.

### 9.1 Action strings (policy side)

Policy emits actions via `next_action(ActionType)` where `ActionType` is usually a name constant like `/read_file`.

Action naming rules:

- Use **name constants** (`/foo`) for action types in Mangle.
- Router normalizes by stripping leading `/` before matching.
- VirtualStore parses by stripping leading `/` when mapping to `ActionType`.

### 9.2 Router patterns (routing side)

Router matches `ActionType` against a route table (exact/prefix/contains).
The plan is to make routing deterministic and fully covered (no silent fallbacks).

### 9.3 VirtualStore executors (execution side)

VirtualStore accepts a `next_action` fact envelope for execution:

- `next_action(ActionType, Target, Payload?)`
- `ActionType` is normalized by removing the leading `/`.

### 9.4 “Must never drift” mapping table (starter set)

This table is not exhaustive yet; expand it until the drift linter can be strict.

| Policy action (`next_action`) | Router pattern | Router tool | VirtualStore ActionType |
|---|---|---|---|
| `/read_file` | `read_file` | `fs_read` | `read_file` |
| `/write_file` | `write_file` | `fs_write` | `write_file` |
| `/edit_file` | `edit_file` | `fs_edit` | `edit_file` |
| `/delete_file` | `delete_file` | `fs_delete` | `delete_file` |
| `/fs_read` | `fs_read` | `fs_read` | `fs_read` *(alias → read_file)* |
| `/fs_write` | `fs_write` | `fs_write` | `fs_write` *(alias → write_file)* |
| `/search_code` | `search_code` | `code_search` | `search_code` |
| `/search_files` | `search_files` | `code_search` | `search_files` *(alias → search_code)* |
| `/git_operation` | `git_operation` | `git_tool` | `git_operation` |
| `/show_diff` | `show_diff` | `git_tool` | `show_diff` |
| `/run_tests` | `run_tests` | `test_runner` | `run_tests` |
| `/build_project` | `build_project` | `build_tool` | `build_project` |
| `/browse` | `browse` | `browser_tool` | `browse` |
| `/research` | `research` | `research_tool` | `research` |
| `/delegate_coder` | `delegate_coder` | `shard_manager` | `delegate_coder` *(handled as delegation)* |
| `/delegate_reviewer` | `delegate_reviewer` | `shard_manager` | `delegate_reviewer` |
| `/delegate_researcher` | `delegate_researcher` | `shard_manager` | `delegate_researcher` |
| `/ask_user` | `ask_user` | `user_prompt` | `ask_user` |

Drift linter requirements:

- Every policy action must appear in router routes.
- Every router route must map to either a VirtualStore executor or an explicit “kernel_internal” handler.
- Every VirtualStore action must be either:
  - reachable from policy, or
  - intentionally “internal-only” (document why).

---

## 10) Appendix B — Fact wiring map (producer/consumer truth table)

This is the “wiring diagram” in factual form.

### 10.1 Intent facts

| Fact | Producer | Consumer(s) | Notes |
|---|---|---|---|
| `user_intent/5` | Perception (or chat fallback) | policy, executive, JIT selectors | canonical ID: `/current_intent` |
| `processed_intent/1` | Perception | policy (intent gating), observability | clarify semantics (perception vs executive) |
| `focus_resolution/4` | Perception | policy | drives clarification loop |
| `ambiguity_flag/3` | Perception | policy + UI | used to trigger ask_user |
| `clarification_needed/1` | policy | UI + Perception loop | should be derived, not asserted by LLM |

### 10.2 Action pipeline facts

| Fact | Producer | Consumer(s) | Notes |
|---|---|---|---|
| `next_action/1` | policy | executive | must remain derived (IDB) |
| `pending_action/5` | executive | constitution | ActionID correlates everything |
| `permitted_action/5` | constitution | router | primary routing stream today |
| `action_permitted/1` | constitution (today) | policy (ooda) | should be derived from permission_check_result |
| `routing_result/4` | router | UI + policy | ActionID should match pending_action ActionID |
| `routing_error/3` | router | UI + autopoiesis | should trigger autopoiesis route fixes |
| `execution_result/5` | virtual store | UI + policy | keep payload sizes bounded |

### 10.3 Tool relevance facts (ShardManager context)

| Fact | Producer | Consumer(s) | Notes |
|---|---|---|---|
| `current_shard_type/1` | shard manager | policy tool relevance | ephemeral context |
| `current_intent/1` | shard manager | policy tool relevance | uses `/tool_routing_context` |
| `user_intent/5` (routing context) | shard manager | policy tool relevance | category `/routing` (not `/mutation`) |
| `current_time/1` | shard manager | policy tool relevance | must be retracted/replaced, not accumulated |

---

## 11) Appendix C — Wiring gap catalogue (expanded)

This section is intentionally exhaustive and will grow as new gaps are discovered.
Each item includes: severity, symptom, root cause, fix, and test.

### 11.1 Intent scoping contamination

- **Severity:** Critical
- **Symptom:** policy derives nothing; executive idles; “no next_action” despite valid user input
- **Root cause:** Perception emitted non-canonical intent IDs while policy required `/current_intent`
- **Fix:** stable `/current_intent` + retract stale facts (see Section 3.1)
- **Test:** create an intent via PerceptionFirewall and assert `next_action` derives from policy

### 11.2 Campaign false triggers from synthetic intents

- **Severity:** High
- **Symptom:** campaign interrupt prompts appear unexpectedly during tool routing
- **Root cause:** tool-routing context asserted `/mutation` category for synthetic `user_intent`
- **Fix:** category `/routing` + dedicated ID `/tool_routing_context`
- **Test:** when `current_campaign(_)` holds, tool relevance queries must not derive `next_action(/ask_campaign_interrupt)`

### 11.3 Action envelope mismatch

- **Severity:** High
- **Symptom:** policy observability facts (`action_permitted/1`) never match reality
- **Root cause:** constitution asserted `action_permitted(ActionType)` instead of `action_permitted(ActionID)`
- **Fix:** assert/retract `action_permitted(ActionID)` consistently (see Section 3.4)
- **Test:** `pending_action` → `permitted_action` must also emit `action_permitted(ActionID)`

### 11.4 Unbounded “system log facts” growth

- **Severity:** Medium → High (long sessions)
- **Symptom:** kernel evaluation slows down over time, memory grows, autopoiesis triggers weirdly
- **Root cause:** action results, errors, and timestamps are accumulated without compaction
- **Fix:** define retention + compaction workflow; persist summaries to cold storage
- **Test:** synthetic loop of N actions keeps kernel fact count under threshold after compaction

### 11.5 Kernel rebuild storms

- **Severity:** Medium
- **Symptom:** large scans/campaigns take exponentially longer
- **Root cause:** repeated `Assert` calls trigger full fixpoint each time
- **Fix:** batch assertion APIs or restructure high-volume writers to use `LoadFacts`
- **Test:** benchmark: world scan to N files should scale roughly linearly

### 11.6 Streaming control-plane bypass

- **Severity:** Critical (safety boundary)
- **Symptom:** malformed streaming output mutates kernel facts or crashes parsing
- **Root cause:** accepting partial Piggyback JSON without early validation
- **Fix:** streaming gate + early GCD validation (Section 5)
- **Test:** fuzz streaming chunks; ensure invalid streams never yield mangle_updates

---

## 12) Appendix D — Observability + metrics plan

Metrics to add (minimum viable):

- `kernel_eval_duration_ms` (p50/p95)
- `kernel_fact_count_total` (EDB + derived)
- `ooda_phase_current`
- `intent_parse_confidence`
- `gcd_repairs_attempted`, `gcd_repairs_failed`
- `stream_aborts_total`, `stream_fallback_total`
- `routing_errors_total` by action type
- `action_success_rate` by action type and tool

Tracing:

- Ensure every LLM call has:
  - shard context (shard_id, shard_type, session_id, task_context)
  - prompt lengths
  - duration
  - success/failure

---

## 13) Appendix E — Prompt atom + JIT governance

Non-negotiables:

- All new LLM systems must use JIT atoms under `internal/prompt/atoms/**`.
- Control packet structure must be schema-validated and GCD-checked.

Suggested atom organization additions:

- `internal/prompt/atoms/system/ooda.yaml`
- `internal/prompt/atoms/mangle/policy_contracts.yaml` (schemas + invariants)
- `internal/prompt/atoms/mangle/action_vocab.yaml` (the drift table)

---

## 14) Appendix F — Git hygiene (when you’re ready to push)

Recommended conventional commit split for this work:

- `fix(perception): canonical /current_intent`
- `fix(policy): scope campaign queries to /current_intent`
- `fix(router): retract action_permitted by ActionID`
- `feat(tracing): add scheduled streaming + context forwarding`
- `docs(plans): add mangle integration hardening plan`
