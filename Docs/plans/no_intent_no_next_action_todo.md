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

- [ ] Add verb validation in `internal/shards/system/perception.go`:
  - [ ] Build `isVerbMapped` helper (uses DefaultTaxonomyData + action mappings).
  - [ ] Emit `intent_unmapped(Verb, /unknown_verb)` when verb not mapped.
  - [ ] Reduce confidence for unmapped verbs to force clarification ladder.
  - [ ] Emit `intent_unknown` when parse fails or confidence is too low.
- [ ] Re-check `matchVerbFromCorpus` default `/explain` behavior; ensure it does not mask `intent_unknown`.
- [ ] Add structured reason atoms: `/llm_failed`, `/heuristic_low`, `/no_verb_match`, `/no_action_mapping` as needed.

## Phase 2: Clarification Payload Hydration

- [ ] Update `internal/shards/system/executive.go`:
  - [ ] Add hydration path for `/interrogative_mode` to include question + options.
  - [ ] Query `clarification_question/2` by intent id and set `req.Target`.
  - [ ] Query `clarification_option/3` and set `req.Payload["options"]` as list.
  - [ ] Ensure fallback question when none found.
- [ ] Validate `VirtualStore.handleInterrogative` behavior still matches payload format.

## Phase 3: OODA Timeout Wiring

- [ ] Add OODA stall tracking to `internal/shards/system/executive.go`:
  - [ ] Track last user intent timestamp.
  - [ ] Only assert `ooda_timeout()` after N seconds (e.g., 30s) with no action.
  - [ ] Respect boot guard: no timeout during session rehydration.
  - [ ] Reset timeout when new intent arrives or action derived.
- [ ] Ensure `ooda_timeout` is asserted to kernel facts so policy derives `ooda_stalled`.
- [ ] Confirm escalation rules actually trigger an action when OODA stalls.

## Phase 4: Router Missing-Route Escalation

- [ ] Update `internal/shards/system/router.go`:
  - [ ] On missing route, emit `routing_result(ActionID, /failure, "no_handler", Timestamp)`.
  - [ ] Optionally emit `no_action_reason(IntentID, /no_route)` if intent context is available.
  - [ ] Keep existing `routing_error` only if still needed; ensure policy path covers failure.
- [ ] Verify policy rules react to `routing_result` failure.

## Phase 5: Learning Candidate Storage (Safe)

- [ ] Add new store `internal/store/learning_candidates.go`:
  - [ ] Create SQLite table `learning_candidates` with status + metadata.
  - [ ] Implement `RecordCandidate`, `ListCandidates`, `ConfirmCandidate`, `RejectCandidate`.
  - [ ] Store reason, phrase, verb, target, counts, timestamps.
- [ ] Wire the candidate store into session init (likely in `cmd/nerd/chat/session.go` and/or shard registration).
- [ ] Connect `PerceptionFirewall` to the candidate store:
  - [ ] Increment count on repeated failures (`intent_unmapped` or `no_action_derived`).
  - [ ] Assert `learning_candidate` when threshold met.
  - [ ] Do not auto-promote; only record.
- [ ] Confirm `TaxonomyStore.StoreLearnedExemplar` is called only after explicit user confirmation.

## Phase 6: Kernel-Driven Clarification in Chat UI

- [ ] Update `cmd/nerd/chat/process.go`:
  - [ ] Add `shouldClarifyFromKernel` (reads `awaiting_clarification`, `clarification_question`, `clarification_option`).
  - [ ] Prefer kernel clarification over chat heuristic `shouldClarifyIntent`.
  - [ ] Display kernel-provided question and options to the user.
- [ ] Ensure ambiguity signals are not duplicated or conflicting between kernel and chat.

## Phase 7: Docs + Config Alignment

- [ ] Update `cmd/nerd/cmd_query.go` to reflect `/interrogative_mode` (deprecate `/ask_user`).
- [ ] Add config values in `.nerd/config.json`:
  - [ ] `learning_candidate_threshold: 3`
  - [ ] `learning_candidate_auto_promote: false`
- [ ] Verify any mention of `learned.mg` now points to `learned_taxonomy.mg`.

## JIT Prompt Atom Work (Required for New LLM Flows)

- [ ] Add prompt atoms for clarification flow (e.g., `internal/prompt/atoms/system/clarification.yaml`).
- [ ] Add prompt atoms for learning candidate confirmation flow (if new LLM prompt is introduced).
- [ ] Ensure atoms follow internal vs project-specific placement rules.
- [ ] Ensure compiler consumes these atoms (add references where appropriate).

## Tests

- [ ] Add `internal/shards/system/perception_validation_test.go`:
  - [ ] Unknown verb emits `intent_unmapped`.
  - [ ] Confidence reduced to force clarification.
  - [ ] `clarification_question` derived for unknown verbs.
- [ ] Add `internal/shards/system/executive_ooda_test.go`:
  - [ ] Boot guard prevents `ooda_timeout`.
  - [ ] After timeout, `ooda_timeout` asserted.
  - [ ] New intent resets timeout.
- [ ] Add `internal/shards/system/router_escalation_test.go`:
  - [ ] Missing route emits `routing_result` with `/failure`.
  - [ ] `no_action_reason(_, /no_route)` emits and triggers clarification.
- [ ] Add `internal/store/learning_candidates_test.go`:
  - [ ] Record/increment/retrieve candidates.
  - [ ] Confirm and reject paths work correctly.
- [ ] Run full test suite: `go test ./...`.

## Integration + Wiring Verification

- [ ] Confirm new policy file `learning.mg` is loaded by kernel.
- [ ] Confirm all new predicates are Decl'd and used safely (no unbound vars).
- [ ] Confirm routing failure escalation derives an actionable `next_action`.
- [ ] Confirm interrogative payloads arrive at `VirtualStore.handleInterrogative`.
- [ ] Confirm OODA stall behavior respects boot guard.
- [ ] Confirm learning candidates are persisted but not auto-applied.
- [ ] Confirm chat UI uses kernel clarification path before its own heuristics.
- [ ] Confirm docs and config changes are consistent and mention the correct taxonomy file.

## Git Hygiene

- [ ] Review `git status` for unrelated changes and avoid reverting them.
- [ ] Use conventional commit message.
- [ ] Push to GitHub after meaningful milestones.
