# Plan: No Intent Match / No Next Action Ladder

This document is intentionally verbose. It records the current pipeline, the real
failure modes, wiring gaps, and a concrete plan to add a deterministic fallback
ladder and a safe learning-candidate path when the system cannot derive an intent
or action.

## Scope and Goals

Goal: add a logical, deterministic action ladder for these cases:

- No intent match (or low confidence / unknown verb / missing target).
- Intent exists but no next_action derived.

Constraints:

- Keep LLM-first intent classification, but add validation and guardrails.
- Do not auto-learn from noisy failures. Learning candidates must be explicit.
- Any new LLM-based behavior must use JIT prompt atoms (system/autopoiesis, etc).

## Current Pipeline (Observed)

### Perception and Intent Parsing

1) LLM-first parse with Piggyback + GCD:
   - `internal/perception/transducer.go` uses `ParseIntentWithGCD`.
   - If LLM fails, it falls back to `parseSimple` (LLM again), then to
     `heuristicParse` (regex + taxonomy).

2) Heuristic path:
   - `matchVerbFromCorpus` defaults to `/explain` if no verb match.
   - That means "no intent match" is masked, not explicit.

3) Perception Firewall:
   - `internal/shards/system/perception.go` emits `user_intent/5`.
   - Ambiguity (confidence < 0.70) emits `ambiguity_flag/3`.
   - Low confidence (confidence < 0.85) increments clarifications but does not
     emit a direct question.
   - It does not validate the verb against taxonomy or action mappings.

4) Chat-only clarification:
   - `cmd/nerd/chat/process.go` uses `shouldClarifyIntent` (confidence + missing
     target + heuristic rules).
   - This is separate from kernel policy and does not consume `ambiguity_flag`.

### Policy and Next Action Derivation

1) Action mapping:
   - `internal/core/defaults/policy/delegation.mg` maps verbs to actions with
     `action_mapping/2` and derives `next_action/1`.
   - If verb is unknown or unmapped, no `next_action` is derived.

2) Executive policy:
   - `internal/shards/system/executive.go` queries `next_action`.
   - If no actions and there is a `user_intent`, it records an unhandled case
     (`reason: no_action_derived`) for autopoiesis.
   - Autopoiesis tries to propose and possibly auto-apply a new Mangle rule
     after N cases.

3) OODA loop:
   - `internal/core/defaults/policy/system_ooda.mg` declares `ooda_stalled` and
     `escalation_needed` via `ooda_timeout`.
   - `ooda_timeout` is declared in `schemas_memory.mg` but is not asserted anywhere.

### Constitution Gate and Router

1) Constitution:
   - `internal/core/defaults/policy/constitution.mg` permits only `safe_action`.
   - If `permitted` cannot be derived, the constitution gate can record an
     unhandled case and optionally escalate.

2) Router:
   - `internal/shards/system/router.go` routes permitted actions.
   - If no route exists, it asserts `routing_error` (not `routing_result`).
   - Policy uses `routing_result` -> `routing_failed` -> `next_action` for
     escalation or pause/replan.

## Observed Gaps / Wiring Issues

1) No explicit "intent unknown" signal.
   - Unknown verbs from LLM or fallback lead to no `next_action`, not a specific
     "intent_unmapped" fact.
   - `matchVerbFromCorpus` default `/explain` masks true failures.

2) OODA stall escalation never triggers.
   - `ooda_timeout` is never asserted. `ooda_stalled` is dead code.

3) Clarification action lacks payload.
   - `next_action(/interrogative_mode)` is derived without a question or options.
   - `VirtualStore.handleInterrogative` expects `req.Target` to be the question.

4) Router missing-route escalation does not work.
   - Router asserts `routing_error`, but policy only reacts to `routing_result`.

5) Ambiguity signals are split across stacks.
   - Kernel uses `ambiguity_flag`, chat uses `intent.Confidence` and missing
     target. They do not feed each other.

6) Documentation drift:
   - `cmd/nerd/cmd_query.go` mentions `next_action(/ask_user)` for clarification,
     but policy emits `/interrogative_mode`.

## Desired Logical Action Ladder

### When Intent Is Unknown or Low Confidence

1) If LLM parsing fails and heuristic confidence is low:
   - Emit `intent_unknown/2` with reason (e.g., `llm_failed`, `heuristic_low`).
   - Trigger `next_action(/interrogative_mode, Question, Options)`.

2) If verb is not in taxonomy or action_mapping:
   - Emit `intent_unmapped/2` with reason `unknown_verb` or `no_action_mapping`.
   - Trigger clarification with suggested verbs (top N likely actions).

3) If target missing or ambiguous:
   - Emit `clarification_needed/1` (already done via focus_resolution).
   - Trigger `next_action(/interrogative_mode, Question, Options)`.

4) Only after explicit user correction:
   - Promote to `learned_exemplar` (taxonomy).
   - Do not auto-learn from a single parse failure.

### When No Next Action Is Derived

Classify the reason and pick a deterministic fallback:

1) Missing action mapping (verb not mapped):
   - Ask the user to confirm intent category and verb.
   - Record a learning candidate (not auto-applied).

2) Clarification needed (focus or ambiguity):
   - Enter interrogative mode with a concrete question.

3) Blocked by constitution:
   - Escalate to user with the reason.
   - Do not attempt learning.

4) Missing router/tool:
   - If action has no route, either propose a tool route (autopoiesis) or
     escalate with "no tool for action" + suggestions.

5) OODA stall timeout:
   - Escalate to user with a structured "no action derived after N seconds"
     explanation and a list of likely intents.

## Learning Candidate Strategy (Safe)

Use learning candidates as a staging buffer, not immediate rules:

1) Log candidates:
   - When `intent_unmapped` or `no_action_derived` happens repeatedly,
     create a `learning_candidate/4` fact (phrase, verb, target, reason).

2) Confirm with user:
   - Ask: "When you say X, should I do Y?".
   - Only after confirmation, write `learned_exemplar` to
     `learned_taxonomy.mg`.

3) Store in SQLite:
   - Reuse `TaxonomyStore.StoreLearnedExemplar`.
   - Store candidate state in a new table or existing learning store.

4) JIT prompts:
   - Any new LLM flow (clarifier, candidate summarizer) must use JIT atoms.

## Concrete Implementation Plan (Phased)

### Phase 0: Instrumentation and Facts (low risk)

- Add new facts to schemas:
  - `intent_unknown(Input, Reason).`
  - `intent_unmapped(Verb, Reason).`
  - `no_action_reason(IntentID, Reason).`
  - `learning_candidate(Phrase, Verb, Target, Reason).`
  - `ooda_timeout().` (already declared, wire it)

- Add event logging around:
  - LLM parse failures.
  - fallback parser path used.
  - unknown verbs or missing action mapping.

### Phase 1: Intent Validation + Unknown Verb Detection

Go changes:

- In `PerceptionFirewallShard.Perceive`:
  - Validate `intent.Verb` against taxonomy or `action_mapping`.
  - If unknown, emit `intent_unmapped` and mark intent as low confidence.
  - Optionally coerce to `/explain` but still emit "unknown" signal.

Mangle changes:

- Add rules to convert `intent_unmapped` into `next_action(/interrogative_mode, ...)`.

### Phase 2: Clarification Payloads

Mangle rules:

- Add `clarification_question/2` or `clarification_prompt/2` predicate.
- Derive questions from:
  - `ambiguity_flag`.
  - `clarification_needed`.
  - `intent_unmapped`.

Go changes:

- Update `ExecutivePolicyShard` (or a small helper) to attach:
  - `target = question`
  - `payload.options = []string{...}`
  when emitting `pending_action` for `/interrogative_mode`.

Note: This keeps the question generation deterministic and avoids a new LLM.
If an LLM-based clarifier is used, use JIT atoms and attach output as target/payload.

### Phase 3: OODA Timeout Wiring

Go changes (one of these):

Option A: ExecutivePolicy tick-based timer.
- Track time since last user_intent without a derived action.
- Assert `ooda_timeout` after N seconds.
- Retract on action derivation or intent processed.

Option B: SessionPlanner or PerceptionFirewall emits timeout facts.

Mangle changes:
- Keep existing `ooda_stalled` -> `escalation_needed` -> `next_action(/escalate_to_user)`.

### Phase 4: Router Missing-Route Fix

Router changes:

- When no route exists, assert `routing_result(ActionID, /failure, "no_handler", ...)`
  instead of (or in addition to) `routing_error`.

Mangle changes:

- Optionally add rules to respond to `routing_error` if we keep it.

### Phase 5: Learning Candidate Pipeline

Go changes:

- Build a small `learning_candidate` aggregator:
  - Track repeated `no_action_derived` and `intent_unmapped`.
  - Emit candidate facts only when repeated or user correction is explicit.

User flow:

- On confirmation, call `TaxonomyEngine.PersistLearnedFact` to store
  `learned_exemplar`.
- Do not auto-apply candidate to policy rules.

### Phase 6: UI / UX Alignment

- In chat UI, when `interrogative_mode` action occurs, show the question and
  options from `payload`.
- Use kernel facts (`ambiguity_flag`, `intent_unmapped`) to drive clarification
  even if the chat path is active.

### Phase 7: Tests and Validation

Add tests for:

- Intent validation: unknown verb -> `intent_unmapped` fact emitted.
- OODA timeout: `ooda_timeout` asserted after delay, `escalation_needed` derived.
- Clarification payload: `interrogative_mode` action has question/option payload.
- Router missing route: `routing_result` produced with failure and escalation.
- Learning candidate: candidate only after repeated failures or user confirmation.

## Immediate Wiring Gaps to Fix Early

1) `ooda_timeout` never asserted:
   - This makes `ooda_stalled` dead and no escalation on "no action derived".

2) Missing route does not trigger escalation:
   - `routing_error` is never consumed by policy.

3) `/interrogative_mode` has no question:
   - Clarifications can be empty in kernel path.

4) Chat vs kernel ambiguity mismatch:
   - Chat does not read `ambiguity_flag`, kernel does not use `shouldClarifyIntent`.

## Notes on LLM-First Classification

We keep LLM-first parsing for semantic correctness, but add a deterministic
validation layer:

- If verb not in taxonomy or action mapping, we do not trust the parse.
- If confidence low, we do not auto-derive actions.
- We still use LLM for clarification when asked, but via JIT prompt atoms.

## Open Questions

- Do we want unknown verbs to default to `/explain` or to a new `/clarify` verb?
- Should learning candidates be stored in `learned_taxonomy.mg` only after a
  two-step confirmation, or can they be auto-promoted after N repeats?
- Should `action_mapping` be moved to a queryable registry in Go for faster
  validation, or stay in Mangle only?

## Proposed Ordering (Summary)

1) Add intent validation + `intent_unmapped` facts.
2) Wire `ooda_timeout` and escalation.
3) Fix router missing-route -> `routing_result` failure.
4) Add clarification question payloads.
5) Introduce learning candidates + confirmation flow.
6) Align chat and kernel clarification behavior.
7) Add tests + documentation updates.

