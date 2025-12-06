# Autopoiesis Implementation for System Shards

## Overview

Added comprehensive learning/pattern tracking (autopoiesis) infrastructure to system shards in `internal/shards/system/`. System shards can now learn from successes, failures, and user corrections, persisting these learnings across sessions using the LearningStore.

## Changes Made

### 1. BaseSystemShard (`base.go`)

#### Added Fields:
- `learningStore *store.LearningStore` - Connection to persistent learning storage
- `patternSuccess map[string]int` - Tracks successful patterns
- `patternFailure map[string]int` - Tracks failed patterns
- `corrections map[string]int` - Tracks user corrections
- `learningEnabled bool` - Toggle for learning system

#### Added Methods:
- `SetLearningStore(ls *store.LearningStore)` - Sets learning store and loads existing patterns
- `loadLearnedPatterns()` - Loads patterns from LearningStore on initialization
- `trackSuccess(pattern string)` - Records successful pattern (persists after 3 occurrences)
- `trackFailure(pattern, reason string)` - Records failed pattern (persists after 2 occurrences)
- `trackCorrection(original, corrected string)` - Records user correction (persists after 2 occurrences)
- `persistLearning() error` - Forces immediate persistence of all learning state

### 2. PerceptionFirewall (`perception.go`)

#### Learning Integration:
- **Success Tracking**: Tracks high-confidence intent parses (`verb:category` patterns)
- **Failure Tracking**: Tracks ambiguous/low-confidence parses
- **Correction Tracking**: Records when users correct parsed intents

#### New Methods:
- `RecordCorrection(originalIntent, correctedIntent Intent)` - Public API for recording user corrections
  - Tracks full intent corrections
  - Tracks specific verb corrections
  - Tracks category corrections
- `GetLearnedPatterns() map[string][]string` - Returns learned patterns for inspection
- `buildSystemPromptWithLearning() string` - Injects learned patterns into LLM system prompt

#### Behavioral Changes:
- High-confidence parses (≥ threshold) → track success
- Ambiguous parses (< ambiguity threshold) → track failure
- LLM prompts now include learned corrections and patterns to avoid

### 3. ExecutivePolicy (`executive.go`)

#### Learning Integration:
- **Action Success**: Tracks which rule→action derivations succeed
- **Action Failure**: Tracks blocked actions and why
- **Strategy Learning**: Tracks strategy-level success/failure patterns

#### New Methods:
- `RecordActionOutcome(action, fromRule string, succeeded bool, errorMsg string)` - Public API for recording action execution outcomes
  - Tracks rule-level outcomes
  - Tracks strategy-level outcomes
- `GetLearnedPatterns() map[string][]string` - Returns learned patterns for inspection

#### Behavioral Changes:
- Successful action derivation → track success (`fromRule:action`)
- Blocked actions → track failure with reason
- Autopoiesis prompts include successful/failed patterns for context

### 4. Tests (`learning_test.go`)

Comprehensive test coverage for:
- Base learning infrastructure (tracking, persistence, loading)
- Perception learning (intent tracking, corrections)
- Executive learning (outcome tracking, strategy patterns)
- LearningStore integration (cross-session persistence)

All tests pass successfully.

## Usage Examples

### Perception Firewall Learning

```go
// Initialize with learning
ls, _ := store.NewLearningStore(".nerd/shards")
perception := NewPerceptionFirewallShard()
perception.SetLearningStore(ls)

// Learning happens automatically during operation
// High-confidence parses are tracked as successes
// Low-confidence parses are tracked as failures

// External systems can record corrections
original := Intent{Verb: "search", Category: "query", Target: "main.go"}
corrected := Intent{Verb: "explain", Category: "query", Target: "main.go"}
perception.RecordCorrection(original, corrected)

// Get learned patterns
patterns := perception.GetLearnedPatterns()
// patterns["successful"] → successful parse patterns
// patterns["failed"] → ambiguous patterns to avoid
// patterns["corrections"] → user corrections
```

### Executive Policy Learning

```go
// Initialize with learning
ls, _ := store.NewLearningStore(".nerd/shards")
executive := NewExecutivePolicyShard()
executive.SetLearningStore(ls)

// Learning happens automatically during policy evaluation
// Successful action derivations are tracked
// Blocked actions are tracked

// External systems can record action outcomes
executive.RecordActionOutcome("write_file", "tdd_next_action", true, "")
executive.RecordActionOutcome("deploy", "campaign_next_action", false, "permission_denied")

// Get learned patterns
patterns := executive.GetLearnedPatterns()
// patterns["successful"] → successful action patterns
// patterns["failed"] → failed action patterns with reasons
```

## Persistence

Learning data is persisted to SQLite databases in `.nerd/shards/`:
- `perception_firewall_learnings.db` - Perception patterns
- `executive_policy_learnings.db` - Executive patterns
- Other system shards get their own databases

Patterns are automatically loaded when a shard is initialized with a LearningStore.

## Thresholds

- **Success persistence**: 3+ occurrences
- **Failure persistence**: 2+ occurrences
- **Correction persistence**: 2+ occurrences
- **Confidence decay**: Patterns not reinforced for 7+ days decay (handled by LearningStore)

## Benefits

1. **Self-improvement**: Shards learn from mistakes and successes
2. **User adaptation**: System adapts to user's specific patterns and preferences
3. **Cross-session memory**: Learnings persist across restarts
4. **Reduced ambiguity**: Perception learns to avoid ambiguous patterns
5. **Better action selection**: Executive learns which actions work well in which contexts
6. **LLM prompt enhancement**: Learned patterns are injected into LLM prompts for better guidance

## Architecture Alignment

This implementation follows the codeNERD neuro-symbolic architecture:
- **Logic-First**: Learning patterns are structured facts, not opaque weights
- **Transparent**: All learned patterns are inspectable and editable
- **Deterministic**: Pattern tracking uses explicit thresholds, not probabilistic updates
- **Persistent**: Knowledge survives across sessions via SQLite
- **Piggyback Protocol Ready**: Facts can be emitted for kernel processing
