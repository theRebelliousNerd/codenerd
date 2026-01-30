# Plan: No Intent Match / No Next Action Ladder

This document is intentionally verbose. It records the current pipeline, the real
failure modes, wiring gaps, and a concrete plan to add a deterministic fallback
ladder and a safe learning-candidate path when the system cannot derive an intent
or action.

**Status:** APPROVED FOR IMPLEMENTATION
**Version:** 2.0.0 (Bulletproof Edition)
**Last Updated:** 2025-01-01

---

## Table of Contents

1. [Scope and Goals](#scope-and-goals)
2. [Current Pipeline (Observed)](#current-pipeline-observed)
3. [Observed Gaps / Wiring Issues](#observed-gaps--wiring-issues)
4. [Desired Logical Action Ladder](#desired-logical-action-ladder)
5. [Learning Candidate Strategy (Safe)](#learning-candidate-strategy-safe)
6. [Concrete Implementation Plan (Phased)](#concrete-implementation-plan-phased)
7. [Schema Declarations (CRITICAL)](#schema-declarations-critical)
8. [Mangle Policy Rules](#mangle-policy-rules)
9. [Go Implementation Details](#go-implementation-details)
10. [Integration Wiring Checklist](#integration-wiring-checklist)
11. [Test Cases](#test-cases)
12. [Open Questions (Resolved)](#open-questions-resolved)

---

## Scope and Goals

Goal: add a logical, deterministic action ladder for these cases:

- No intent match (or low confidence / unknown verb / missing target).
- Intent exists but no next_action derived.

Constraints:

- Keep LLM-first intent classification, but add validation and guardrails.
- Do not auto-learn from noisy failures. Learning candidates must be explicit.
- Any new LLM-based behavior must use JIT prompt atoms (system/autopoiesis, etc).
- All new predicates MUST have schema declarations before use.
- Boot guard awareness - no spurious triggers during session rehydration.

---

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

---

## Observed Gaps / Wiring Issues

### GAP 1: No explicit "intent unknown" signal
- Unknown verbs from LLM or fallback lead to no `next_action`, not a specific
  "intent_unmapped" fact.
- `matchVerbFromCorpus` default `/explain` masks true failures.
- **Impact:** System silently fails instead of asking for clarification.

### GAP 2: OODA stall escalation never triggers
- `ooda_timeout` is declared but never asserted by any Go code.
- `ooda_stalled` rule depends on `ooda_timeout()` which is always false.
- **Impact:** Dead code - escalation_needed never fires.

### GAP 3: Clarification action lacks payload
- `next_action(/interrogative_mode)` is derived as arity-1 without question or options.
- `VirtualStore.handleInterrogative` expects `req.Target` to be the question
  and `req.Payload["options"]` for choices.
- **Impact:** Clarification mode activates but shows empty/generic question.

### GAP 4: Router missing-route escalation does not work
- Router asserts `routing_error`, but policy only reacts to `routing_result`.
- No rule converts `routing_error` -> escalation.
- **Impact:** Missing routes silently fail.

### GAP 5: Ambiguity signals are split across stacks
- Kernel uses `ambiguity_flag/3`, chat uses `intent.Confidence` and missing target.
- They do not feed each other.
- Chat's `shouldClarifyIntent` doesn't read kernel facts.
- Kernel doesn't consume chat's clarification logic.
- **Impact:** Duplicate/inconsistent clarification triggers.

### GAP 6: Documentation drift
- `cmd/nerd/cmd_query.go` mentions `next_action(/ask_user)` for clarification,
  but policy emits `/interrogative_mode`.
- **Impact:** Confusion; inconsistent action naming.

### GAP 7: Boot guard not considered for OODA timeout
- `executive.go:105-106` has `bootGuardActive: true` to prevent stale actions.
- If `ooda_timeout` is wired, it could fire during boot before user interaction.
- **Impact:** Potential spurious escalation at startup.

### GAP 8: TaxonomyStore.StoreLearnedExemplar exists but not wired
- `internal/perception/taxonomy_persistence.go` has `StoreLearnedExemplar`.
- Not connected to the learning candidate confirmation flow.
- **Impact:** Learned patterns not persisted correctly.

---

## Desired Logical Action Ladder

### When Intent Is Unknown or Low Confidence

1) If LLM parsing fails and heuristic confidence is low:
   - Emit `intent_unknown/2` with reason (e.g., `/llm_failed`, `/heuristic_low`).
   - Trigger `next_action(/interrogative_mode)` with question payload.

2) If verb is not in taxonomy or action_mapping:
   - Emit `intent_unmapped/2` with reason `/unknown_verb` or `/no_action_mapping`.
   - Trigger clarification with suggested verbs (top N likely actions).

3) If target missing or ambiguous:
   - Emit `clarification_needed/1` (already done via focus_resolution).
   - Trigger `next_action(/interrogative_mode)` with question payload.

4) Only after explicit user correction:
   - Promote to `learned_exemplar` (taxonomy).
   - Do not auto-learn from a single parse failure.

### When No Next Action Is Derived

Classify the reason and pick a deterministic fallback:

1) Missing action mapping (verb not mapped):
   - Emit `no_action_reason(/current_intent, /unmapped_verb)`.
   - Ask the user to confirm intent category and verb.
   - Record a learning candidate (not auto-applied).

2) Clarification needed (focus or ambiguity):
   - Enter interrogative mode with a concrete question.

3) Blocked by constitution:
   - Escalate to user with the reason.
   - Do not attempt learning.

4) Missing router/tool:
   - If action has no route, emit `no_action_reason(IntentID, /no_route)`.
   - Either propose a tool route (autopoiesis) or escalate with suggestions.

5) OODA stall timeout:
   - Escalate to user with a structured "no action derived after N seconds"
     explanation and a list of likely intents.

---

## Learning Candidate Strategy (Safe)

Use learning candidates as a staging buffer, not immediate rules:

1) Log candidates:
   - When `intent_unmapped` or `no_action_derived` happens repeatedly (3+ times),
     create a `learning_candidate/4` fact (phrase, verb, target, reason).
   - Track per-session in RAM, persist candidates to SQLite after session.

2) Confirm with user:
   - Ask: "When you say X, should I do Y?".
   - Only after confirmation, write `learned_exemplar` to taxonomy store.

3) Store in SQLite:
   - Use existing `TaxonomyStore.StoreLearnedExemplar` in `taxonomy_persistence.go`.
   - Store candidate state in `learning_candidates` table (new).

4) JIT prompts:
   - Any new LLM flow (clarifier, candidate summarizer) must use JIT atoms.
   - Location: `internal/prompt/atoms/system/clarification.yaml` (new).

5) Threshold configuration:
   - `learning_candidate_threshold`: 3 (repeated failures before candidate)
   - `learning_candidate_auto_promote`: false (require user confirmation)
   - Stored in `.nerd/config.json`.

---

## Concrete Implementation Plan (Phased)

### Phase 0: Schema Declarations (CRITICAL - Must Be First)

**Files to modify:**
- `internal/core/defaults/schemas_perception.mg`
- `internal/core/defaults/schemas_memory.mg`

**Add declarations:**

```mangle
# In schemas_perception.mg - Add after existing perception predicates

# Intent unknown - LLM parse failed or heuristic confidence too low
# Reason: /llm_failed, /heuristic_low, /no_verb_match
Decl intent_unknown(Input.Type<string>, Reason).

# Intent unmapped - verb not in taxonomy or action_mapping
# Reason: /unknown_verb, /no_action_mapping, /deprecated_verb
Decl intent_unmapped(Verb.Type<string>, Reason).

# Reason why no action was derived for an intent
# Reason: /unmapped_verb, /no_route, /blocked_by_constitution, /ooda_timeout
Decl no_action_reason(IntentID.Type<string>, Reason).

# Learning candidate - staged before promotion to learned_exemplar
# Accumulated after repeated failures, requires user confirmation
Decl learning_candidate(Phrase.Type<string>, Verb.Type<string>, Target.Type<string>, Reason).

# Clarification question with options (hydrates interrogative_mode)
Decl clarification_question(IntentID.Type<string>, Question.Type<string>).
Decl clarification_option(IntentID.Type<string>, OptionVerb.Type<string>, OptionLabel.Type<string>).

# Track learning candidate occurrences for threshold
Decl learning_candidate_count(Phrase.Type<string>, Count.Type<int>).
```json

**Validation:**
- Run `go test ./internal/core/...` to ensure schema loads
- Run `node tools/mangle-check.js` for syntax validation

---

### Phase 1: Intent Validation + Unknown Verb Detection

**Files to modify:**
- `internal/shards/system/perception.go`
- `internal/perception/taxonomy.go`

**Go changes in `perception.go`:**

```go
// Add to Perceive() after intent normalization, before emitting user_intent

// Validate verb against known action mappings
if !p.isVerbMapped(intent.Verb) {
    logging.SystemShards("[PerceptionFirewall] Unmapped verb detected: %s", intent.Verb)
    _ = p.Kernel.Assert(types.Fact{
        Predicate: "intent_unmapped",
        Args:      []interface{}{intent.Verb, types.MangleAtom("/unknown_verb")},
    })
    // Reduce confidence to trigger clarification path
    intent.Confidence = min(intent.Confidence, 0.4)
}
```

**Add helper method to perception.go:**

```go
// isVerbMapped checks if a verb has a known action mapping.
// Uses cached taxonomy data to avoid kernel query on every parse.
func (p *PerceptionFirewallShard) isVerbMapped(verb string) bool {
    // Check against DefaultTaxonomyData from perception package
    normalizedVerb := normalizeAtom(verb)
    for _, entry := range perception.DefaultTaxonomyData {
        if normalizeAtom(entry.Verb) == normalizedVerb {
            return true
        }
    }
    return false
}
```mangle

**Mangle rules in `clarification.mg`:**

```mangle
# Derive clarification when intent is unmapped
next_action(/interrogative_mode) :-
    intent_unmapped(Verb, /unknown_verb),
    !any_awaiting_clarification(/yes).

# Generate question for unmapped verb
clarification_question(/current_intent, Question) :-
    intent_unmapped(Verb, /unknown_verb),
    Question = fn:string_concat("I don't recognize the action '", Verb, "'. What would you like me to do?").

# Suggest common alternatives
clarification_option(/current_intent, /explain, "Explain or describe something").
clarification_option(/current_intent, /fix, "Fix a bug or issue").
clarification_option(/current_intent, /review, "Review code for issues").
clarification_option(/current_intent, /search, "Search the codebase").
clarification_option(/current_intent, /test, "Run or generate tests").
```

---

### Phase 2: Clarification Payloads

**Files to modify:**
- `internal/shards/system/executive.go`
- `internal/core/virtual_store_workflows.go`
- `internal/core/defaults/policy/clarification.mg`

**Go changes in `executive.go` - Enhance `hydrateActionFromIntent`:**

```go
// In hydrateActionFromIntent, add case for interrogative_mode
case "/interrogative_mode":
    // Query kernel for clarification question and options
    if e.Kernel != nil {
        questions, _ := e.Kernel.Query("clarification_question")
        for _, q := range questions {
            if len(q.Args) >= 2 {
                if id, ok := q.Args[0].(string); ok && id == "/current_intent" {
                    if question, ok := q.Args[1].(string); ok {
                        target = question
                        payload["question"] = question
                    }
                }
            }
        }

        options, _ := e.Kernel.Query("clarification_option")
        var optionList []string
        for _, opt := range options {
            if len(opt.Args) >= 3 {
                if id, ok := opt.Args[0].(string); ok && id == "/current_intent" {
                    if label, ok := opt.Args[2].(string); ok {
                        optionList = append(optionList, label)
                    }
                }
            }
        }
        if len(optionList) > 0 {
            payload["options"] = optionList
        }
    }
    payload["intent_id"] = intent.ID
    return target, payload
```go

**Note:** `handleInterrogative` in `virtual_store_workflows.go:162-186` already
correctly extracts `req.Target` as question and `req.Payload["options"]` - no changes needed there.

---

### Phase 3: OODA Timeout Wiring

**Files to modify:**
- `internal/shards/system/executive.go`
- `internal/core/defaults/policy/system_ooda.mg`

**Go changes in `executive.go`:**

```go
// Add fields to ExecutivePolicyShard struct
type ExecutivePolicyShard struct {
    // ... existing fields ...

    // OODA timeout tracking
    lastUserIntentTime time.Time
    oodaTimeoutEmitted bool
}

// Add to DisableBootGuard() - start OODA tracking after boot
func (e *ExecutivePolicyShard) DisableBootGuard() {
    e.mu.Lock()
    defer e.mu.Unlock()
    if e.bootGuardActive {
        e.bootGuardActive = false
        e.lastUserIntentTime = time.Now() // Start tracking from first interaction
        e.oodaTimeoutEmitted = false
        logging.SystemShards("[ExecutivePolicy] Boot guard disabled, OODA tracking started")
    }
}

// Add to evaluatePolicy() - check for OODA timeout
func (e *ExecutivePolicyShard) evaluatePolicy(ctx context.Context) error {
    // ... existing strategy query code ...

    // OODA timeout check - only after boot guard disabled
    e.mu.RLock()
    bootGuardActive := e.bootGuardActive
    lastIntentTime := e.lastUserIntentTime
    oodaEmitted := e.oodaTimeoutEmitted
    e.mu.RUnlock()

    if !bootGuardActive && !oodaEmitted {
        // Check if we have pending intent but no action for 30+ seconds
        if intent := e.latestUserIntent(); intent != nil {
            if time.Since(lastIntentTime) > 30*time.Second {
                // Check if there are any derived actions
                actions, _ := e.queryNextActions()
                if len(actions) == 0 {
                    logging.SystemShards("[ExecutivePolicy] OODA timeout: no action derived for 30s")
                    _ = e.Kernel.Assert(types.Fact{
                        Predicate: "ooda_timeout",
                        Args:      []interface{}{},
                    })
                    e.mu.Lock()
                    e.oodaTimeoutEmitted = true
                    e.mu.Unlock()
                }
            }
        }
    }

    // ... rest of existing code ...
}

// Reset OODA timeout when new intent arrives
// Call this from wherever user_intent is processed
func (e *ExecutivePolicyShard) ResetOODATimeout() {
    e.mu.Lock()
    defer e.mu.Unlock()
    e.lastUserIntentTime = time.Now()
    e.oodaTimeoutEmitted = false
    // Retract stale ooda_timeout if present
    if e.Kernel != nil {
        _ = e.Kernel.Retract("ooda_timeout")
    }
}
```

**Policy already exists in `system_ooda.mg:63-69`, just needs the fact:**

```mangle
# These rules will now fire once ooda_timeout() is asserted by Go
ooda_stalled("no_action_derived") :-
    pending_intent(_),
    ooda_timeout().

escalation_needed(/ooda_loop, "stalled", Reason) :-
    ooda_stalled(Reason).
```mangle

**Add escalation action derivation in `system_ooda.mg`:**

```mangle
# Derive escalate action when OODA stalls
next_action(/escalate_to_user) :-
    escalation_needed(/ooda_loop, "stalled", _).
```

---

### Phase 4: Router Missing-Route Fix

**Files to modify:**
- `internal/shards/system/router.go`
- `internal/core/defaults/policy/system.mg` (or new `routing_escalation.mg`)

**Go changes in `router.go` - Modify processPermittedActions:**

```go
// Replace current routing_error assertion (around line 450-454):
if !found {
    logging.Routing("No route found for action: %s (target=%s)", actionType, target)

    // Emit routing_result with failure status (policy can react to this)
    _ = r.Kernel.Assert(types.Fact{
        Predicate: "routing_result",
        Args: []interface{}{
            actionID,
            types.MangleAtom("/failure"),
            "no_handler",
            time.Now().Unix(),
        },
    })

    // Also emit no_action_reason for the intent
    _ = r.Kernel.Assert(types.Fact{
        Predicate: "no_action_reason",
        Args: []interface{}{actionID, types.MangleAtom("/no_route")},
    })

    if r.config.AllowUnmappedActions {
        r.Autopoiesis.RecordUnhandled(
            fmt.Sprintf("route(%s)", actionType),
            map[string]string{"action": actionType, "target": target},
            nil,
        )
    }
    continue
}
```mangle

**Mangle rules in new `routing_escalation.mg`:**

```mangle
# Escalate when routing fails
next_action(/escalate_to_user) :-
    routing_result(_, /failure, "no_handler", _).

# Or trigger interrogative mode to ask what tool to use
next_action(/interrogative_mode) :-
    no_action_reason(_, /no_route),
    !any_awaiting_clarification(/yes).

clarification_question(IntentID, "I don't have a tool to handle this action. Would you like me to try a different approach?") :-
    no_action_reason(IntentID, /no_route).
```

---

### Phase 5: Learning Candidate Pipeline

**Files to modify:**
- `internal/shards/system/perception.go` (or new `learning_aggregator.go`)
- `internal/perception/taxonomy_persistence.go`
- `internal/store/learning_candidates.go` (new file)

**New file: `internal/store/learning_candidates.go`:**

```go
package store

import (
    "database/sql"
    "time"
)

// LearningCandidate represents a potential learned pattern awaiting confirmation
type LearningCandidate struct {
    ID        int64
    Phrase    string
    Verb      string
    Target    string
    Reason    string
    Count     int
    CreatedAt time.Time
    UpdatedAt time.Time
    Confirmed bool
}

// LearningCandidateStore manages learning candidates in SQLite
type LearningCandidateStore struct {
    db *sql.DB
}

// NewLearningCandidateStore creates a store with the given database
func NewLearningCandidateStore(db *sql.DB) (*LearningCandidateStore, error) {
    store := &LearningCandidateStore{db: db}
    if err := store.ensureSchema(); err != nil {
        return nil, err
    }
    return store, nil
}

func (s *LearningCandidateStore) ensureSchema() error {
    _, err := s.db.Exec(`
        CREATE TABLE IF NOT EXISTS learning_candidates (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            phrase TEXT NOT NULL,
            verb TEXT NOT NULL,
            target TEXT DEFAULT '',
            reason TEXT NOT NULL,
            count INTEGER DEFAULT 1,
            created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
            updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
            confirmed BOOLEAN DEFAULT FALSE,
            UNIQUE(phrase, verb)
        )
    `)
    return err
}

// RecordCandidate increments count or inserts new candidate
func (s *LearningCandidateStore) RecordCandidate(phrase, verb, target, reason string) (int, error) {
    result, err := s.db.Exec(`
        INSERT INTO learning_candidates (phrase, verb, target, reason, count)
        VALUES (?, ?, ?, ?, 1)
        ON CONFLICT(phrase, verb) DO UPDATE SET
            count = count + 1,
            updated_at = CURRENT_TIMESTAMP
    `, phrase, verb, target, reason)
    if err != nil {
        return 0, err
    }

    // Return current count
    var count int
    _ = s.db.QueryRow(`SELECT count FROM learning_candidates WHERE phrase = ? AND verb = ?`, phrase, verb).Scan(&count)
    return count, nil
}

// GetCandidatesAboveThreshold returns candidates ready for confirmation
func (s *LearningCandidateStore) GetCandidatesAboveThreshold(threshold int) ([]LearningCandidate, error) {
    rows, err := s.db.Query(`
        SELECT id, phrase, verb, target, reason, count, created_at, updated_at, confirmed
        FROM learning_candidates
        WHERE count >= ? AND confirmed = FALSE
        ORDER BY count DESC, updated_at DESC
    `, threshold)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var candidates []LearningCandidate
    for rows.Next() {
        var c LearningCandidate
        if err := rows.Scan(&c.ID, &c.Phrase, &c.Verb, &c.Target, &c.Reason, &c.Count, &c.CreatedAt, &c.UpdatedAt, &c.Confirmed); err != nil {
            continue
        }
        candidates = append(candidates, c)
    }
    return candidates, nil
}

// ConfirmCandidate marks a candidate as confirmed and promotes to taxonomy
func (s *LearningCandidateStore) ConfirmCandidate(id int64) error {
    _, err := s.db.Exec(`UPDATE learning_candidates SET confirmed = TRUE WHERE id = ?`, id)
    return err
}

// RejectCandidate removes a candidate
func (s *LearningCandidateStore) RejectCandidate(id int64) error {
    _, err := s.db.Exec(`DELETE FROM learning_candidates WHERE id = ?`, id)
    return err
}
```go

**Wire into perception.go:**

```go
// Add to PerceptionFirewallShard struct
type PerceptionFirewallShard struct {
    // ... existing fields ...
    candidateStore *store.LearningCandidateStore
}

// Add method to record learning candidate
func (p *PerceptionFirewallShard) recordLearningCandidate(phrase, verb, target, reason string) {
    if p.candidateStore == nil {
        return
    }

    count, err := p.candidateStore.RecordCandidate(phrase, verb, target, reason)
    if err != nil {
        logging.SystemShardsDebug("[PerceptionFirewall] Failed to record learning candidate: %v", err)
        return
    }

    // Emit fact for kernel tracking
    _ = p.Kernel.Assert(types.Fact{
        Predicate: "learning_candidate_count",
        Args:      []interface{}{phrase, count},
    })

    const threshold = 3
    if count >= threshold {
        logging.SystemShards("[PerceptionFirewall] Learning candidate ready for confirmation: %q -> %s (count=%d)", phrase, verb, count)
        _ = p.Kernel.Assert(types.Fact{
            Predicate: "learning_candidate",
            Args:      []interface{}{phrase, verb, target, reason},
        })
    }
}
```

**Integration with TaxonomyStore (already exists):**

```go
// In taxonomy_persistence.go - StoreLearnedExemplar is already implemented
// Just need to call it when user confirms a candidate

// Add method to TaxonomyEngine in taxonomy.go
func (t *TaxonomyEngine) ConfirmLearningCandidate(phrase, verb, target, constraint string, confidence float64) error {
    if t.store == nil {
        return fmt.Errorf("no taxonomy store configured")
    }
    return t.store.StoreLearnedExemplar(phrase, verb, target, constraint, confidence)
}
```go

---

### Phase 6: UI / UX Alignment

**Files to modify:**
- `cmd/nerd/chat/process.go`
- `cmd/nerd/chat/process_dream.go`

**Consolidate clarification to kernel-driven:**

```go
// In process.go - modify clarification check to use kernel facts

func (m Model) shouldClarifyFromKernel() (bool, string, []string) {
    if m.kernel == nil {
        return false, "", nil
    }

    // Check for awaiting_clarification fact
    awaiting, _ := m.kernel.Query("awaiting_clarification")
    if len(awaiting) > 0 {
        question := ""
        if len(awaiting[0].Args) > 0 {
            question, _ = awaiting[0].Args[0].(string)
        }

        // Get options
        options, _ := m.kernel.Query("clarification_option")
        var optionLabels []string
        for _, opt := range options {
            if len(opt.Args) >= 3 {
                if label, ok := opt.Args[2].(string); ok {
                    optionLabels = append(optionLabels, label)
                }
            }
        }

        return true, question, optionLabels
    }

    // Also check ambiguity_flag
    ambiguity, _ := m.kernel.Query("ambiguity_flag")
    if len(ambiguity) > 0 {
        return true, "I'm not sure I understand. Could you clarify?", nil
    }

    return false, "", nil
}

// Update processInput to check kernel clarification first
func (m Model) processInput(ctx context.Context, input string) tea.Msg {
    // ... existing code ...

    // Check kernel-driven clarification FIRST
    if needsClarify, question, options := m.shouldClarifyFromKernel(); needsClarify {
        return clarificationMsg{
            Question: question,
            Options:  options,
        }
    }

    // Then fall back to chat-based shouldClarifyIntent for edge cases
    // ... rest of existing code ...
}
```

---

### Phase 7: Tests and Validation

**Test files to create/modify:**

1. `internal/shards/system/perception_validation_test.go` (new)
2. `internal/shards/system/executive_ooda_test.go` (new)
3. `internal/shards/system/router_escalation_test.go` (new)
4. `internal/store/learning_candidates_test.go` (new)

**Test cases:**

```go
// perception_validation_test.go
func TestIntentUnmapped_UnknownVerb(t *testing.T) {
    // Setup perception shard with kernel
    // Parse input with unknown verb like "/foobar something"
    // Assert intent_unmapped fact is emitted
    // Assert confidence is reduced
}

func TestIntentUnmapped_TriggersInterrogativeMode(t *testing.T) {
    // Setup perception + executive with kernel
    // Parse input with unknown verb
    // Run policy evaluation
    // Assert next_action(/interrogative_mode) is derived
    // Assert clarification_question fact exists
}

// executive_ooda_test.go
func TestOODATimeout_NotEmittedDuringBootGuard(t *testing.T) {
    // Create executive with boot guard active
    // Wait 35 seconds
    // Assert ooda_timeout NOT emitted
}

func TestOODATimeout_EmittedAfterBootGuardDisabled(t *testing.T) {
    // Create executive, disable boot guard
    // Assert user_intent without action mapping
    // Wait 35 seconds
    // Assert ooda_timeout IS emitted
    // Assert escalation_needed derived
}

func TestOODATimeout_ResetsOnNewIntent(t *testing.T) {
    // Create executive, disable boot guard
    // Wait 25 seconds
    // Submit new intent
    // Wait 10 more seconds
    // Assert ooda_timeout NOT emitted (timer reset)
}

// router_escalation_test.go
func TestRouter_MissingRouteEmitsRoutingResult(t *testing.T) {
    // Setup router with no route for "foobar_action"
    // Submit permitted_action for foobar_action
    // Assert routing_result(_, /failure, "no_handler", _) emitted
    // Assert no_action_reason(_, /no_route) emitted
}

// learning_candidates_test.go
func TestLearningCandidate_ThresholdBehavior(t *testing.T) {
    // Record same candidate 2 times
    // Assert learning_candidate fact NOT emitted
    // Record 3rd time
    // Assert learning_candidate fact IS emitted
}

func TestLearningCandidate_ConfirmationFlow(t *testing.T) {
    // Create candidate above threshold
    // Confirm candidate
    // Assert learned_exemplar in taxonomy store
}
```mangle

---

## Schema Declarations (CRITICAL)

All predicates MUST be declared before use. Add to appropriate schema files:

### `internal/core/defaults/schemas_perception.mg`

```mangle
# ============================================================================
# INTENT VALIDATION PREDICATES
# ============================================================================

# Intent unknown - LLM parse failed or heuristic confidence too low
# Reason atoms: /llm_failed, /heuristic_low, /no_verb_match, /parse_error
Decl intent_unknown(Input.Type<string>, Reason).

# Intent unmapped - verb recognized but not in action_mapping
# Reason atoms: /unknown_verb, /no_action_mapping, /deprecated_verb
Decl intent_unmapped(Verb.Type<string>, Reason).

# Reason why no action was derived for a specific intent
# Reason atoms: /unmapped_verb, /no_route, /blocked_by_constitution, /ooda_timeout, /clarification_pending
Decl no_action_reason(IntentID.Type<string>, Reason).

# ============================================================================
# LEARNING CANDIDATE PREDICATES
# ============================================================================

# Learning candidate - staged pattern awaiting user confirmation
# Accumulated after repeated failures, requires explicit user confirmation
Decl learning_candidate(Phrase.Type<string>, Verb.Type<string>, Target.Type<string>, Reason).

# Track learning candidate occurrence count for threshold logic
Decl learning_candidate_count(Phrase.Type<string>, Count.Type<int>).

# ============================================================================
# CLARIFICATION PREDICATES
# ============================================================================

# Clarification question generated for an intent
Decl clarification_question(IntentID.Type<string>, Question.Type<string>).

# Clarification options - verbs/actions user can choose from
Decl clarification_option(IntentID.Type<string>, OptionVerb.Type<string>, OptionLabel.Type<string>).
```

### `internal/core/defaults/schemas_memory.mg` (verify existing)

```mangle
# OODA timeout - already declared but verify it matches
# ooda_timeout() - True when OODA loop has stalled (30s+ without action)
# Computed by Go based on last_action_time vs current_time
Decl ooda_timeout().
```mangle

---

## Mangle Policy Rules

### `internal/core/defaults/policy/clarification.mg` (additions)

```mangle
# ============================================================================
# INTENT UNKNOWN / UNMAPPED CLARIFICATION
# ============================================================================

# Trigger interrogative mode for unknown intents
next_action(/interrogative_mode) :-
    intent_unknown(_, _),
    !any_awaiting_clarification(/yes).

next_action(/interrogative_mode) :-
    intent_unmapped(_, _),
    !any_awaiting_clarification(/yes).

# Generate clarification questions for unmapped verbs
clarification_question(/current_intent, Question) :-
    intent_unmapped(Verb, /unknown_verb),
    Question = fn:string_concat("I don't recognize the action '", Verb, "'. What would you like me to do?").

clarification_question(/current_intent, "Could you rephrase what you'd like me to do?") :-
    intent_unknown(_, /llm_failed).

clarification_question(/current_intent, "I'm not confident I understood correctly. Could you clarify?") :-
    intent_unknown(_, /heuristic_low).

# Default clarification options (always available)
clarification_option(/current_intent, /explain, "Explain or describe something") :- intent_unmapped(_, _).
clarification_option(/current_intent, /fix, "Fix a bug or issue") :- intent_unmapped(_, _).
clarification_option(/current_intent, /review, "Review code for issues") :- intent_unmapped(_, _).
clarification_option(/current_intent, /search, "Search the codebase") :- intent_unmapped(_, _).
clarification_option(/current_intent, /test, "Run or generate tests") :- intent_unmapped(_, _).
clarification_option(/current_intent, /create, "Create new code or files") :- intent_unmapped(_, _).

# ============================================================================
# NO ACTION REASON HANDLING
# ============================================================================

# Emit no_action_reason when intent exists but no next_action derived
# This rule fires AFTER policy evaluation if no next_action was derived
# Note: This is a diagnostic predicate, not an action trigger

# When no route exists, clarify
next_action(/interrogative_mode) :-
    no_action_reason(_, /no_route),
    !any_awaiting_clarification(/yes).

clarification_question(IntentID, "I don't have a tool to handle this action. Would you like me to try a different approach?") :-
    no_action_reason(IntentID, /no_route).
```

### `internal/core/defaults/policy/system_ooda.mg` (additions)

```mangle
# ============================================================================
# OODA TIMEOUT ESCALATION (activated once ooda_timeout() is asserted by Go)
# ============================================================================

# Escalate when OODA stalls
next_action(/escalate_to_user) :-
    escalation_needed(/ooda_loop, "stalled", _).

# Alternative: trigger interrogative mode instead of raw escalation
next_action(/interrogative_mode) :-
    ooda_stalled("no_action_derived"),
    !any_awaiting_clarification(/yes).

clarification_question(/current_intent, "I'm having trouble determining what action to take. Could you rephrase your request?") :-
    ooda_stalled("no_action_derived").
```mangle

### New file: `internal/core/defaults/policy/learning.mg`

```mangle
# ============================================================================
# LEARNING CANDIDATE POLICY
# ============================================================================

# When a learning candidate reaches threshold, suggest confirmation
# This could trigger a special confirmation flow in the UI

Decl learning_candidate_ready(Phrase, Verb).

learning_candidate_ready(Phrase, Verb) :-
    learning_candidate(Phrase, Verb, _, _).

# Future: Could trigger a special action to confirm learning
# next_action(/confirm_learning) :- learning_candidate_ready(_, _).
```

---

## Go Implementation Details

### File Modifications Summary

| File | Changes |
|------|---------|
| `internal/core/defaults/schemas_perception.mg` | Add 6 new Decl statements |
| `internal/core/defaults/policy/clarification.mg` | Add clarification rules for intent_unknown/unmapped |
| `internal/core/defaults/policy/system_ooda.mg` | Add escalation action derivation |
| `internal/core/defaults/policy/learning.mg` | New file for learning candidate rules |
| `internal/shards/system/perception.go` | Add verb validation, learning candidate recording |
| `internal/shards/system/executive.go` | Add OODA timeout tracking, clarification payload hydration |
| `internal/shards/system/router.go` | Emit routing_result on failure instead of routing_error |
| `internal/store/learning_candidates.go` | New file for candidate storage |
| `cmd/nerd/chat/process.go` | Kernel-driven clarification check |

### Dependency Order

1. **Schema declarations** (must be first - Mangle won't accept predicates without Decl)
2. **Policy rules** (depend on schema)
3. **Go changes to perception.go** (emit new facts)
4. **Go changes to executive.go** (OODA timeout + payload hydration)
5. **Go changes to router.go** (routing_result emission)
6. **New store file** (learning candidates)
7. **Chat UI changes** (kernel-driven clarification)
8. **Tests** (validate everything works)

---

## Integration Wiring Checklist

### Schema Wiring
- [ ] Add all Decl statements to schemas_perception.mg
- [ ] Verify ooda_timeout Decl in schemas_memory.mg
- [ ] Run `go build` to ensure schemas load
- [ ] Run `node tools/mangle-check.js` for syntax validation

### Policy Wiring
- [ ] Add clarification rules to clarification.mg
- [ ] Add escalation rules to system_ooda.mg
- [ ] Create learning.mg policy file
- [ ] Verify all predicates used in rules have Decl statements

### Go Wiring - Perception
- [ ] Add isVerbMapped helper to perception.go
- [ ] Call isVerbMapped in Perceive() after normalization
- [ ] Emit intent_unmapped fact when verb not mapped
- [ ] Add candidateStore field to PerceptionFirewallShard
- [ ] Add recordLearningCandidate method
- [ ] Wire candidateStore in session.go initialization

### Go Wiring - Executive
- [ ] Add lastUserIntentTime and oodaTimeoutEmitted fields
- [ ] Modify DisableBootGuard to start OODA tracking
- [ ] Add OODA timeout check in evaluatePolicy
- [ ] Add ResetOODATimeout method
- [ ] Call ResetOODATimeout when new user_intent arrives
- [ ] Add interrogative_mode case in hydrateActionFromIntent
- [ ] Query clarification_question and clarification_option facts
- [ ] Populate payload with question and options

### Go Wiring - Router
- [ ] Change routing_error to routing_result with /failure status
- [ ] Add no_action_reason fact emission on missing route

### Go Wiring - Store
- [ ] Create learning_candidates.go
- [ ] Implement LearningCandidateStore with SQLite
- [ ] Wire store creation in session initialization

### Go Wiring - Chat UI
- [ ] Add shouldClarifyFromKernel method
- [ ] Check kernel clarification before chat-based clarification
- [ ] Display question and options from kernel facts

### Testing
- [ ] Create perception_validation_test.go
- [ ] Create executive_ooda_test.go
- [ ] Create router_escalation_test.go
- [ ] Create learning_candidates_test.go
- [ ] Run full test suite: `go test ./...`

---

## Test Cases

### Unit Tests

1. **Intent Validation**
   - Unknown verb emits intent_unmapped
   - Confidence reduced for unmapped verbs
   - intent_unmapped triggers next_action(/interrogative_mode)
   - clarification_question and clarification_option facts generated

2. **OODA Timeout**
   - Boot guard active: no ooda_timeout emitted
   - Boot guard disabled + 30s wait: ooda_timeout emitted
   - New intent resets timeout
   - ooda_stalled derived from ooda_timeout
   - escalation_needed derived from ooda_stalled

3. **Router Escalation**
   - Missing route emits routing_result with /failure
   - no_action_reason emitted with /no_route
   - Policy derives interrogative_mode from no_route

4. **Learning Candidates**
   - Count increments on repeated failures
   - Threshold (3) triggers learning_candidate fact
   - Confirmation promotes to learned_exemplar
   - Rejection removes candidate

### Integration Tests

1. **End-to-End Unknown Verb**
   - User submits "/foobar do something"
   - System asks clarifying question with options
   - User selects option
   - System executes selected action

2. **End-to-End OODA Stall**
   - User submits ambiguous request
   - No action derived for 30s
   - System escalates with explanation
   - User rephrases
   - System processes correctly

3. **End-to-End Learning**
   - User submits non-standard phrasing 3 times
   - System offers to learn pattern
   - User confirms
   - Pattern works on next use

---

## Open Questions (Resolved)

### Q1: Default to `/explain` or new `/clarify` verb?
**Resolution:** Keep `/explain` as fallback for truly ambiguous queries. Add `/interrogative_mode` action (already exists) for explicit clarification needs. The `/clarify` verb is unnecessary.

### Q2: Auto-promote learning candidates after N repeats?
**Resolution:** No. Require explicit user confirmation. Set `learning_candidate_auto_promote: false` in config. Threshold of 3 triggers suggestion, user must confirm.

### Q3: Move action_mapping to Go registry?
**Resolution:** Keep in Mangle for policy consistency. Add Go-side cache (`isVerbMapped`) using `DefaultTaxonomyData` for fast validation without kernel query. Cache is populated at boot from the Go taxonomy data.

### Q4: Which is canonical - /ask_user or /interrogative_mode?
**Resolution:** `/interrogative_mode` is canonical (used in policy). Update documentation in cmd_query.go. `/ask_user` is deprecated/alias.

### Q5: How to pass options through next_action?
**Resolution:** Use separate `clarification_question/2` and `clarification_option/3` predicates. Executive hydrates these into the action payload. VirtualStore.handleInterrogative already expects payload["options"].

---

## Proposed Ordering (Summary)

1. Add schema declarations (Phase 0) - **CRITICAL FIRST**
2. Add policy rules (Phase 0.5)
3. Add intent validation + `intent_unmapped` facts (Phase 1)
4. Add clarification question payloads (Phase 2)
5. Wire `ooda_timeout` and escalation (Phase 3)
6. Fix router missing-route -> `routing_result` failure (Phase 4)
7. Introduce learning candidates + confirmation flow (Phase 5)
8. Align chat and kernel clarification behavior (Phase 6)
9. Add tests + documentation updates (Phase 7)

---

## Addendum: Critic Learned-Exemplar Extraction Hardening (Post-Plan Finding)

During live verification of the learning-candidate flow, a concrete wiring gap
emerged that is not captured in the original phases: the Critic LLM frequently
returns verbose text (JSON payloads, self-reflections, or multi-sentence analysis)
that surrounds the `learned_exemplar(...)` fact. The original extraction logic
was naive (substring to first `)`), which collapses when:

- The Critic includes trailing prose after the fact.
- Constraint strings contain commas (`,`) which break naive `strings.Split`.
- The Critic emits JSON with embedded commas and quotes, causing the extracted
  string to include many stray commas before any closing `)`.

**Impact:** The learning-candidate pipeline appears to work, but in practice the
Critic output can exceed the parser’s tolerance. The result is a failed learning
candidate staging step (no confirmation prompt), even when the LLM did output a
valid `learned_exemplar` fact. This blocks the entire “safe learning candidate”
path and hides a valid teaching opportunity from the user.

### Required Hardening Changes (New Tasks)

1) **Quote-aware extraction of `learned_exemplar`:**
   - Scan forward from `learned_exemplar(` and find the first closing `)` that
     is *not* inside a quoted string.
   - If a trailing `.` exists, include it; otherwise append one.
   - This keeps extraction resilient even when the LLM wraps the fact in JSON
     or surrounds it with narrative text.

2) **Comma-safe parsing of fact arguments:**
   - Replace naive `strings.Split` with a parser that splits on commas only
     when *not* inside quoted strings.
   - Allow constraints like `"ensure: wired to the TUI, log a note"` without
     breaking argument count.

3) **Preserve full learned fact during confirmation:**
   - When staging a learning candidate, store the raw fact in a kernel fact
     (`learning_candidate_fact`) so confirmation can persist *exactly what the
     Critic proposed*, including constraint string and confidence value.

4) **Test the hardened path in both modes:**
   - **Manual /learn path:** trigger the Critic, confirm yes, verify
     `.nerd/mangle/learned_taxonomy.mg` is updated only after confirmation.
   - **Auto-learn path:** trigger dissatisfaction, confirm yes, verify the
     same persistence rule.
   - Confirm `learning_candidates` records switch from `pending` → `confirmed`.

### Additional Hardening (Follow-up)

5) **Tighten Critic prompt output contract:**
   - Explicitly forbid JSON, commentary, or extra text.
   - Require a single-line `learned_exemplar(...)` or empty output.

6) **Add unit tests for extraction + parsing:**
   - `ExtractFactFromResponse` with JSON noise and parentheses inside quotes.
   - `ParseLearnedFact` with comma in constraint + escaped quotes.

### Validation Notes

- The hardening tasks must be considered part of the “learning candidate” phase
  and should be tracked in the TODO list as a new mini-phase.
- This work does **not** relax the safety model: candidates are still staged
  only after explicit confirmation, and no auto-promotion is introduced.

---

**Total Estimated Files to Modify:** 12
**Total New Files:** 3
**Risk Level:** Medium (touches perception, executive, router, but with clear boundaries)

---

*End of Plan*

> *[Archived & Reviewed by The Librarian on 2026-01-30]*
