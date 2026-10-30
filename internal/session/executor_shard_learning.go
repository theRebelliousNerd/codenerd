package session

import (
	"fmt"
	"strings"

	"codenerd/internal/logging"
	"codenerd/internal/perception"
	"codenerd/internal/types"
)

// What a persona's turns teach it. The JIT personas (coder, tester, reviewer,
// researcher) run as session-executor turns, not as system shards, and
// nothing on that path wrote their learning stores (.nerd/shards/
// <persona>_learnings.db): only the system shards called Save, so the
// personas held zero learnings while the JIT compiler recalls from every
// store for each compile (collectLearningAtomsFrom). A finished turn now
// records, into its own persona's store, what it taught:
//
//   - failure_pattern(task, reason): the kernel judged the turn /hollow or
//     /failed; the reason is the kernel's (missing evidence, owed gates) or
//     the turn's error.
//   - correction_pattern(what was rejected, how it was repaired): a write a
//     validator rejected that a later write replaced before the turn closed
//     (its effect closure is superseded, effect closure in tool_effects.go).
//   - success_pattern(task, how): the kernel verified a turn that had been
//     corrected on the way. A verified turn with nothing corrected teaches
//     nothing new and records nothing.
//
// A persona whose profile disables learning (enable_learning) records
// nothing; the gate is the same learningPolicy recordTurn honours.

// ShardLearningSaver is where a persona's learnings land: the boot's
// store.LearningStore.
type ShardLearningSaver interface {
	SaveBatch(shardType string, learnings []types.ShardLearning, sourceCampaign string) error
}

// SetShardLearnings installs the persona learning store. Nil disables it.
func (e *Executor) SetShardLearnings(s ShardLearningSaver) {
	if e == nil {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.shardLearnings = s
}

// HasShardLearnings reports whether a persona learning store is installed, so
// a boot test can ask whether turns teach their personas anything.
func (e *Executor) HasShardLearnings() bool {
	if e == nil {
		return false
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.shardLearnings != nil
}

// personaForVerb is the persona a verb runs as, in the spelling the shard
// profiles and the learning stores use ("coder"); "" for a verb no persona
// runs.
func personaForVerb(verb string) string {
	p := strings.TrimPrefix(perception.GetShardTypeForVerb(verb), "/")
	if p == "none" {
		return ""
	}
	return p
}

// learningAllowed is the persona's enable_learning, as installed by
// SetLearningPolicy; with no policy every persona learns.
func (e *Executor) learningAllowed(persona string) bool {
	e.mu.RLock()
	policy := e.learningPolicy
	e.mu.RUnlock()
	return policy == nil || policy(persona)
}

// recordShardLearnings saves what the finished turn taught its persona.
func (e *Executor) recordShardLearnings(verb, task string, outcome types.MangleAtom, result *ExecutionResult) {
	if e == nil || result == nil {
		return
	}
	e.mu.RLock()
	saver := e.shardLearnings
	e.mu.RUnlock()
	persona := personaForVerb(verb)
	if saver == nil || persona == "" || !e.learningAllowed(persona) {
		return
	}
	lessons := turnLessons(verb, task, outcome, result)
	if len(lessons) == 0 {
		return
	}
	if err := saver.SaveBatch(persona, lessons, ""); err != nil {
		logging.Get(logging.CategorySession).Warn("persona %s: saving %d learning(s): %v", persona, len(lessons), err)
	}
}

// turnLessons lists what a finished turn taught (see the file comment).
func turnLessons(verb, task string, outcome types.MangleAtom, result *ExecutionResult) []types.ShardLearning {
	what := strings.TrimSpace(verb + ": " + firstLine(task))
	var out []types.ShardLearning
	corrections := 0
	for _, r := range result.ToolReceipts {
		if r.Validation.State != types.ToolValidationRejected || r.Closure == nil || r.Closure.State != "superseded" {
			continue
		}
		corrections++
		rejected := fmt.Sprintf("%s wrote %s and validation rejected it", r.Tool, strings.Join(r.ChangedPaths(), ", "))
		if reason := strings.TrimSpace(r.Validation.Reason); reason != "" {
			rejected += ": " + reason
		}
		out = append(out, types.ShardLearning{FactPredicate: "correction_pattern", FactArgs: []any{rejected, "a later write replaced it before the turn closed"}, Confidence: 1})
	}
	switch outcome {
	case "/hollow", "/failed":
		out = append(out, types.ShardLearning{FactPredicate: "failure_pattern", FactArgs: []any{what, failureReason(outcome, result)}, Confidence: 1})
	case "/done":
		if corrections > 0 {
			out = append(out, types.ShardLearning{FactPredicate: "success_pattern", FactArgs: []any{what, fmt.Sprintf("verified after %d rejected write(s) were repaired", corrections)}, Confidence: 1})
		}
	}
	return out
}

// failureReason is the kernel's account of a failed turn: its missing
// evidence and owed gates, else the turn's error, else the verdict itself.
func failureReason(outcome types.MangleAtom, result *ExecutionResult) string {
	var parts []string
	if len(result.MissingEvidence) > 0 {
		parts = append(parts, "missing evidence "+strings.Join(result.MissingEvidence, ", "))
	}
	if len(result.OwedGates) > 0 {
		parts = append(parts, "owed gates "+strings.Join(result.OwedGates, ", "))
	}
	if len(parts) == 0 && result.Error != nil {
		parts = append(parts, firstLine(result.Error.Error()))
	}
	if len(parts) == 0 {
		parts = append(parts, "the kernel judged the turn "+string(outcome))
	}
	return strings.Join(parts, "; ")
}
