# TODO: No Intent Match / No Next Action Ladder

This is the execution checklist derived from `Docs/plans/no_intent_no_next_action_plan.md`.
It mirrors the full plan and breaks it into concrete, trackable tasks with
explicit wiring points, file paths, and validation steps. Keep this list in
sync with the plan document as changes land.

## Re-ingest + Alignment

- [x] Re-read the plan (v2.0.0 Bulletproof Edition) in `Docs/plans/no_intent_no_next_action_plan.md`.
- [x] Re-confirm plan intent: deterministic fallback ladder for unknown intent or no `next_action`, with safe learning candidates only.
- [x] Re-confirm constraints: LLM-first intent classification, no auto-learn from noisy failures, JIT prompt atoms for new LLM flows, boot guard awareness.
- [x] Re-confirm canonical action name: `/interrogative_mode` (deprecate `/ask_user` in docs).
- [x] Re-confirm learning candidate policy: threshold 3, explicit user confirmation only, no auto-promotion.

## Phase 0: Schema Declarations (CRITICAL FIRST)

- [x] Add new Decl predicates for unknown intent, unmapped verbs, learning candidates, clarification payloads, and no-action reasons.
  - [x] Decide correct schema file location (use `internal/core/defaults/schemas_intent.mg`).
  - [x] Add `Decl intent_unknown(Input, Reason).`
  - [x] Add `Decl intent_unmapped(Verb, Reason).`
  - [x] Add `Decl no_action_reason(IntentID, Reason).`
  - [x] Add `Decl learning_candidate(Phrase, Verb, Target, Reason).`
  - [x] Add `Decl clarification_question(IntentID, Question).`
  - [x] Add `Decl clarification_option(IntentID, OptionVerb, OptionLabel).`
  - [x] Add `Decl learning_candidate_count(Phrase, Count).`
  - [x] Add `Decl learning_candidate_ready(Phrase, Verb).`
- [x] Verify `Decl ooda_timeout().` exists in `internal/core/defaults/schemas_memory.mg` and matches usage.
- [ ] Validate schemas compile: `go test ./internal/core/...` (blocked: `internal/store/prompt_reembed.go` undefined `embedding`).
- [ ] Validate Mangle syntax using tooling if available (`tools/mangle-check.js`).

## Phase 0.5: Policy Rules

- [x] Update `internal/core/defaults/policy/clarification.mg`:
  - [x] Add `next_action(/interrogative_mode)` when `intent_unknown` and no awaiting clarification.
  - [x] Add `next_action(/interrogative_mode)` when `intent_unmapped` and no awaiting clarification.
  - [x] Add `clarification_question` rules for `intent_unknown` reasons (`/llm_failed`, `/heuristic_low`, `/no_verb_match`).
  - [x] Add `clarification_question` rule for `intent_unmapped` with unknown verb (use string concat function).
  - [x] Add `clarification_question` rule for `intent_unmapped` with `/no_action_mapping`.
  - [x] Add default `clarification_option` set for unmapped verbs (`/explain`, `/fix`, `/review`, `/search`, `/test`, `/create`).
  - [x] Add `next_action(/interrogative_mode)` and question when `no_action_reason(_, /no_route)` occurs.
  - [x] Add `next_action(/interrogative_mode)` and question when `no_action_reason(_, /no_action_derived)` occurs.
- [x] Update `internal/core/defaults/policy/system_ooda.mg`:
  - [x] Derive `next_action(/escalate_to_user)` or `next_action(/interrogative_mode)` when `ooda_stalled` is present.
  - [x] Provide `clarification_question` for `ooda_stalled("no_action_derived")`.
- [x] Create new policy file `internal/core/defaults/policy/learning.mg`:
  - [x] Add `learning_candidate_ready` derivation for future confirmation flows.
  - [x] Keep rule set minimal and safe (no auto-apply).
- [x] Ensure all new predicates used in policy have Decl statements.
- [x] Confirm policy directory auto-loads `learning.mg` (no `kernel_init.go` change needed).

## Phase 1: Intent Validation + Unknown Verb Detection

- [x] Add verb validation in `internal/shards/system/perception.go`:
  - [x] Build `classifyVerbMapping` helper (uses VerbCorpus + action mappings).
  - [x] Emit `intent_unmapped(Verb, /unknown_verb)` when verb not mapped.
  - [x] Reduce confidence for unmapped verbs to force clarification ladder.
  - [x] Emit `intent_unknown` when parse fails or confidence is too low.
- [x] Re-check `matchVerbFromCorpus` default `/explain` behavior; ensure it does not mask `intent_unknown` (now emits `/no_verb_match` + low confidence).
- [x] Add structured reason atoms: `/llm_failed`, `/heuristic_low`, `/no_verb_match`, `/no_action_mapping`.

## Phase 2: Clarification Payload Hydration

- [x] Update `internal/shards/system/executive.go`:
  - [x] Add hydration path for `/interrogative_mode` to include question + options.
  - [x] Query `clarification_question/2` by intent id and set `req.Target`.
  - [x] Query `clarification_option/3` and set `req.Payload["options"]` as list.
  - [x] Ensure fallback question when none found.
- [x] Validate `VirtualStore.handleInterrogative` behavior still matches payload format.

## Phase 3: OODA Timeout Wiring

- [x] Add OODA stall tracking to `internal/shards/system/executive.go`:
  - [x] Track last user intent timestamp.
  - [x] Only assert `ooda_timeout()` after N seconds (e.g., 30s) with no action.
  - [x] Respect boot guard: no timeout during session rehydration.
  - [x] Reset timeout when new intent arrives or action derived.
- [x] Ensure `ooda_timeout` is asserted to kernel facts so policy derives `ooda_stalled`.
- [x] Confirm escalation rules actually trigger an action when OODA stalls.

## Phase 4: Router Missing-Route Escalation

- [x] Update `internal/shards/system/router.go`:
  - [x] On missing route, emit `routing_result(ActionID, /failure, "no_handler", Timestamp)`.
  - [x] Emit `no_action_reason(IntentID, /no_route)` when route is missing.
  - [x] Keep existing `routing_error` only if still needed; ensure policy path covers failure.
- [x] Verify policy rules react to `routing_result` failure.

## Phase 5: Learning Candidate Storage (Safe)

- [x] Add new store `internal/store/learning_candidates.go`:
  - [x] Create SQLite table `learning_candidates` with status + metadata.
  - [x] Implement `RecordLearningCandidate`, `ListLearningCandidates`, `ConfirmLearningCandidate`, `RejectLearningCandidate`.
  - [x] Store reason, phrase, verb, target, counts, timestamps.
- [x] Wire the candidate store into session init (in `cmd/nerd/chat/session.go`).
- [x] Connect `PerceptionFirewall` to the candidate store:
  - [x] Increment count on repeated failures (`intent_unmapped`).
  - [x] Assert `learning_candidate` when threshold met.
  - [x] Do not auto-promote; only record.
- [x] Extend learning candidate capture for `no_action_derived` (use `user_input_string` as phrase source).
- [x] Confirm `TaxonomyStore.StoreLearnedExemplar` is called only after explicit user confirmation.

## Phase 6: Kernel-Driven Clarification in Chat UI

- [x] Update `cmd/nerd/chat/process.go`:
  - [x] Add `shouldClarifyFromKernel` (reads `awaiting_clarification`, `clarification_question`, `clarification_option`).
  - [x] Prefer kernel clarification over chat heuristic `shouldClarifyIntent`.
  - [x] Display kernel-provided question and options to the user.
- [x] Ensure ambiguity signals are not duplicated or conflicting between kernel and chat.

## Phase 7: Docs + Config Alignment

- [x] Update `cmd/nerd/cmd_query.go` to reflect `/interrogative_mode` (deprecate `/ask_user`).
- [x] Add config values in `.nerd/config.json`:
  - [x] `learning_candidate_threshold: 3`
  - [x] `learning_candidate_auto_promote: false`
- [x] Audit docs to distinguish kernel `learned.mg` vs taxonomy `learned_taxonomy.mg` (avoid blanket renames).

## Addendum: Critic Learned-Exemplar Hardening

- [x] Harden `ExtractFactFromResponse` to capture the first `learned_exemplar(...)` outside quotes.
- [x] Split learned_exemplar args on commas *outside* quotes to support constraint strings with commas.
- [x] Preserve raw `learned_exemplar` during confirmation so constraints/confidence survive intact.
- [x] Validate manual `/learn`: no `.nerd/mangle/learned_taxonomy.mg` until confirmation.
- [x] Validate auto-learn: dissatisfaction triggers candidate; confirmation appends to `.nerd/mangle/learned_taxonomy.mg`.
- [x] Tighten Critic prompt output contract (single-line learned_exemplar or empty).
- [x] Add unit tests for extraction + parsing (JSON noise, commas, escaped quotes).

## JIT Prompt Atom Work (Required for New LLM Flows)

- [x] Add prompt atoms for clarification flow (e.g., `internal/prompt/atoms/system/clarification.yaml`).
- [x] Add prompt atoms for learning candidate confirmation flow (if new LLM prompt is introduced).
- [x] Ensure atoms follow internal vs project-specific placement rules.
- [x] Ensure compiler can load these atoms (selection wiring still TBD if new LLM flow is added).

## Tests

- [x] Add `internal/shards/system/perception_validation_test.go`:
  - [x] Unknown verb emits `intent_unmapped`.
  - [x] Confidence reduced to force clarification.
  - [x] `clarification_question` derived for unknown verbs.
- [x] Add `internal/shards/system/executive_ooda_test.go`:
  - [x] Boot guard prevents `ooda_timeout`.
  - [x] After timeout, `ooda_timeout` asserted.
  - [x] New intent resets timeout.
- [x] Add `internal/shards/system/router_escalation_test.go`:
  - [x] Missing route emits `routing_result` with `/failure`.
  - [x] `no_action_reason(_, /no_route)` emits and triggers clarification.
- [x] Add `internal/store/learning_candidates_test.go`:
  - [x] Record/increment/retrieve candidates.
  - [x] Confirm and reject paths work correctly.
- [ ] Run full test suite: `go test ./...` (blocked: `internal/store/prompt_reembed.go` undefined `embedding`).

## Integration + Wiring Verification

- [x] Confirm new policy file `learning.mg` is loaded by kernel.
- [x] Confirm all new predicates are Decl'd and used safely (no unbound vars).
- [x] Confirm routing failure escalation derives an actionable `next_action`.
- [x] Confirm interrogative payloads arrive at `VirtualStore.handleInterrogative`.
- [x] Confirm OODA stall behavior respects boot guard.
- [x] Confirm learning candidates are persisted but not auto-applied.
- [x] Confirm chat UI uses kernel clarification path before its own heuristics.
- [x] Confirm docs and config changes are consistent and distinguish `learned.mg` vs `learned_taxonomy.mg`.

## Git Hygiene

- [x] Review `git status` for unrelated changes and avoid reverting them.
- [x] Use conventional commit message.
- [x] Push to GitHub after meaningful milestones.

> *[Archived & Reviewed by The Librarian on 2026-01-24]*
