package system

import (
	"fmt"
	"maps"
	"strconv"
	"strings"
	"time"

	"codenerd/internal/logging"
	"codenerd/internal/types"
)

type userIntentSnapshot struct {
	ID         string
	Category   string
	Verb       string
	Target     string
	Constraint string
	Timestamp  int64
}

func (e *ExecutivePolicyShard) latestUserIntent() *userIntentSnapshot {
	if e.Kernel == nil {
		return nil
	}
	facts, err := e.Kernel.Query("user_intent")
	if err != nil {
		return nil
	}

	// Prefer the canonical, stable intent ID when present.
	for _, f := range facts {
		if len(f.Args) < 5 {
			continue
		}
		id := types.ExtractString(f.Args[0])
		if id != "/current_intent" {
			continue
		}
		return &userIntentSnapshot{
			ID:         id,
			Category:   types.ExtractString(f.Args[1]),
			Verb:       types.ExtractString(f.Args[2]),
			Target:     types.ExtractString(f.Args[3]),
			Constraint: types.ExtractString(f.Args[4]),
			Timestamp:  time.Now().UnixNano(),
		}
	}

	var best *userIntentSnapshot
	for _, f := range facts {
		if len(f.Args) < 5 {
			continue
		}
		id := types.ExtractString(f.Args[0])
		ts, ok := parseIntentTimestamp(id)
		if !ok {
			continue
		}
		if best == nil || ts > best.Timestamp {
			best = &userIntentSnapshot{
				ID:         id,
				Category:   types.ExtractString(f.Args[1]),
				Verb:       types.ExtractString(f.Args[2]),
				Target:     types.ExtractString(f.Args[3]),
				Constraint: types.ExtractString(f.Args[4]),
				Timestamp:  ts,
			}
		}
	}
	return best
}

func parseIntentTimestamp(intentID string) (int64, bool) {
	const prefix = "/intent_"
	if !strings.HasPrefix(intentID, prefix) {
		return 0, false
	}
	ts, err := strconv.ParseInt(strings.TrimPrefix(intentID, prefix), 10, 64)
	if err != nil {
		return 0, false
	}
	return ts, true
}

func (e *ExecutivePolicyShard) loadClarificationPayload(intentID string) (string, []any) {
	if e.Kernel == nil || intentID == "" {
		return "", nil
	}

	question := ""
	if facts, err := e.Kernel.Query("clarification_question"); err == nil {
		for _, f := range facts {
			if len(f.Args) < 2 {
				continue
			}
			if types.ExtractString(f.Args[0]) != intentID {
				continue
			}
			if q, ok := f.Args[1].(string); ok {
				question = q
			} else {
				question = types.ExtractString(f.Args[1])
			}
			break
		}
	}

	options := make([]any, 0)
	if facts, err := e.Kernel.Query("clarification_option"); err == nil {
		for _, f := range facts {
			if len(f.Args) < 3 {
				continue
			}
			if types.ExtractString(f.Args[0]) != intentID {
				continue
			}
			verb := types.ExtractString(f.Args[1])
			label := types.ExtractString(f.Args[2])
			if label != "" && label != "<nil>" {
				options = append(options, fmt.Sprintf("%s (%s)", label, verb))
			} else {
				options = append(options, verb)
			}
		}
	}

	if question == "" {
		if facts, err := e.Kernel.Query("awaiting_clarification"); err == nil && len(facts) > 0 {
			if len(facts[0].Args) > 0 {
				if q, ok := facts[0].Args[0].(string); ok {
					question = q
				} else {
					question = types.ExtractString(facts[0].Args[0])
				}
			}
		}
	}
	if strings.TrimSpace(question) == "" {
		question = "I need a bit more detail to proceed. What would you like me to do?"
	}

	return question, options
}

func (e *ExecutivePolicyShard) loadUserInputString() string {
	if e.Kernel == nil {
		return ""
	}
	facts, err := e.Kernel.Query("user_input_string")
	if err != nil || len(facts) == 0 {
		return ""
	}
	if len(facts[0].Args) == 0 {
		return ""
	}
	return strings.TrimSpace(types.ExtractString(facts[0].Args[0]))
}

func (e *ExecutivePolicyShard) recordNoActionCandidate(intent *userIntentSnapshot, reason string) {
	if intent == nil || reason == "" {
		return
	}
	e.mu.RLock()
	store := e.candidateStore
	threshold := e.config.LearningCandidateThreshold
	e.mu.RUnlock()
	if store == nil {
		return
	}

	phrase := e.loadUserInputString()
	if phrase == "" {
		phrase = strings.TrimSpace(intent.Constraint)
	}
	phrase = strings.TrimSpace(phrase)
	if phrase == "" || phrase == "none" || phrase == "_" {
		phrase = strings.TrimSpace(fmt.Sprintf("%s %s", intent.Verb, intent.Target))
		phrase = strings.TrimSpace(phrase)
	}
	if phrase == "" {
		return
	}

	verb := normalizeAtom(intent.Verb)
	target := strings.TrimSpace(intent.Target)
	if target == "" || target == "none" || target == "_" {
		target = ""
	}

	count, err := store.RecordLearningCandidate(phrase, verb, target, reason)
	if err != nil {
		logging.SystemShardsDebug("[ExecutivePolicy] Failed to record learning candidate: %v", err)
		return
	}

	if e.Kernel == nil || threshold <= 0 {
		return
	}
	if existing, err := e.Kernel.Query("learning_candidate_count"); err == nil {
		for _, f := range existing {
			if len(f.Args) < 2 {
				continue
			}
			if existingPhrase, ok := f.Args[0].(string); ok && existingPhrase == phrase {
				_ = e.Kernel.RetractFact(f)
			}
		}
	}
	_ = e.Kernel.Assert(types.Fact{
		Predicate: "learning_candidate_count",
		Args:      []any{phrase, count},
	})

	if count >= threshold {
		_ = e.Kernel.Assert(types.Fact{
			Predicate: "learning_candidate",
			Args: []any{
				phrase,
				types.MangleAtom(verb),
				target,
				types.MangleAtom(reason),
			},
		})
	}
}

func copyStringAnyMap(src map[string]any) map[string]any {
	if len(src) == 0 {
		return map[string]any{}
	}
	dst := make(map[string]any, len(src))
	maps.Copy(dst, src)
	return dst
}

func (e *ExecutivePolicyShard) hydrateActionFromIntent(actionType string, target string, payload map[string]any, intent *userIntentSnapshot) (string, map[string]any) {
	if intent == nil {
		return target, payload
	}
	if payload == nil {
		payload = map[string]any{}
	}

	actionAtom := normalizeAtom(actionType)
	intentVerb := normalizeAtom(intent.Verb)
	intentTarget := strings.TrimSpace(intent.Target)
	intentConstraint := strings.TrimSpace(intent.Constraint)

	switch actionAtom {
	case "/interrogative_mode":
		payload["intent_id"] = intent.ID
		question, options := e.loadClarificationPayload(intent.ID)
		if strings.TrimSpace(question) != "" {
			target = question
		}
		if len(options) > 0 {
			payload["options"] = options
		}
		return target, payload
	case "/delegate_reviewer", "/delegate_coder", "/delegate_researcher", "/delegate_tool_generator":
		// For delegation actions, ensure we always supply a usable task string.
		task, _ := payload["task"].(string)
		task = strings.TrimSpace(task)
		if task == "" {
			task = strings.TrimSpace(target)
		}
		if task == "" {
			task = intentTarget
		}
		verb := strings.TrimPrefix(intentVerb, "/")
		if verb == "" {
			verb = "task"
		}
		if intentConstraint != "" && intentConstraint != "none" && intentConstraint != "_" {
			task = fmt.Sprintf("%s %s\nConstraint: %s", verb, task, intentConstraint)
		} else {
			task = fmt.Sprintf("%s %s", verb, task)
		}
		payload["task"] = task
		payload["intent_id"] = intent.ID
		return task, payload
	default:
		// Only hydrate target for actions where intent target is a reliable binding.
		// Avoid contaminating internal/TDD actions (e.g., read_error_log) with unrelated intent targets.
		switch actionAtom {
		case "/read_file", "/write_file", "/edit_file", "/delete_file", "/fs_read", "/fs_write", "/search_files", "/search_code", "/analyze_code":
			payload["intent_id"] = intent.ID
			if intentConstraint != "" && intentConstraint != "none" && intentConstraint != "_" {
				payload["intent_constraint"] = intentConstraint
			}
			if strings.TrimSpace(target) == "" && intentTarget != "" && intentTarget != "none" && intentTarget != "_" {
				return intentTarget, payload
			}
			return target, payload
		default:
			return target, payload
		}
	}
}

// queryActiveStrategies queries for currently active strategies.
func (e *ExecutivePolicyShard) queryActiveStrategies() ([]Strategy, error) {
	results, err := e.Kernel.Query("active_strategy")
	if err != nil {
		return nil, err
	}

	strategies := make([]Strategy, 0, len(results))
	for _, fact := range results {
		if len(fact.Args) < 1 {
			continue
		}
		name, ok := fact.Args[0].(string)
		if !ok {
			continue
		}
		strategies = append(strategies, Strategy{
			Name:        name,
			ActivatedAt: time.Now(),
		})
	}

	return strategies, nil
}

// queryNextActions queries for derived next actions.
func (e *ExecutivePolicyShard) queryNextActions() ([]ActionDecision, error) {
	results, err := e.Kernel.Query("next_action")
	if err != nil {
		// Record as unhandled for autopoiesis
		e.Autopoiesis.RecordUnhandled(
			"next_action",
			map[string]string{"error": err.Error()},
			nil,
		)
		return nil, err
	}

	// Also check for specific strategy-driven actions
	strategyActions := []string{
		"tdd_next_action",
		"campaign_next_action",
		"repair_next_action",
	}

	for _, predicate := range strategyActions {
		additional, err := e.Kernel.Query(predicate)
		if err == nil {
			results = append(results, additional...)
		}
	}

	actions := make([]ActionDecision, 0, len(results))

	delegations, err := e.queryDelegateActions()
	if err != nil {
		logging.Get(logging.CategorySystemShards).Warn("[ExecutivePolicy] Delegate task query failed: %v", err)
	} else {
		actions = append(actions, delegations...)
	}
	for _, fact := range results {
		if len(fact.Args) < 1 {
			continue
		}
		actionName, ok := fact.Args[0].(string)
		if !ok {
			continue
		}

		decision := ActionDecision{
			Action:    actionName,
			DerivedAt: time.Now(),
			FromRule:  fact.Predicate,
			Payload:   make(map[string]any),
			RawFact:   fact,
		}

		// Extract target if present
		if len(fact.Args) > 1 {
			decision.Target, _ = fact.Args[1].(string)
		}

		// Extract payload from remaining args
		if len(fact.Args) > 2 {
			for i := 2; i < len(fact.Args); i++ {
				if argMap, ok := fact.Args[i].(map[string]any); ok {
					maps.Copy(decision.Payload, argMap)
					continue
				}
				key := fmt.Sprintf("arg%d", i-2)
				decision.Payload[key] = fact.Args[i]
			}
		}

		// Allow shards/policy to pre-seed action IDs via payload
		if rawID, ok := decision.Payload["action_id"]; ok {
			if idStr, ok := rawID.(string); ok && idStr != "" {
				decision.ID = idStr
			}
			delete(decision.Payload, "action_id")
		}
		if decision.ID == "" {
			decision.ID = fmt.Sprintf("action-%d", time.Now().UnixNano())
		}

		actions = append(actions, decision)
	}

	// If no actions derived BUT there is an active user intent, record for autopoiesis.
	// BUG FIX: Only record when there's a genuine gap (user intent exists but no action derived).
	// Recording on every empty tick causes autopoiesis spam at startup when no user has
	// interacted yet, leading to immediate budget exhaustion and wasted LLM calls.
	if len(actions) == 0 && len(results) == 0 {
		// Check if there's an active user intent that we failed to handle
		if intent := e.latestUserIntent(); intent != nil {
			if !e.IsBootGuardActive() {
				reason := "/no_action_derived"
				if unmapped, err := e.Kernel.Query("intent_unmapped"); err == nil && len(unmapped) > 0 {
					reason = "/unmapped_verb"
				}
				alreadyRecorded := false
				if existing, err := e.Kernel.Query("no_action_reason"); err == nil {
					for _, f := range existing {
						if len(f.Args) < 2 {
							continue
						}
						if types.ExtractString(f.Args[0]) == intent.ID {
							alreadyRecorded = true
							break
						}
					}
				}
				if !alreadyRecorded {
					_ = e.Kernel.Assert(types.Fact{
						Predicate: "no_action_reason",
						Args:      []any{intent.ID, types.MangleAtom(reason)},
					})
					if reason == "/no_action_derived" {
						e.recordNoActionCandidate(intent, reason)
					}
				}
			}
			e.Autopoiesis.RecordUnhandled(
				"next_action",
				map[string]string{"reason": "no_action_derived", "intent_id": intent.ID},
				nil,
			)
		}
		// Otherwise: no user intent and no action is the NORMAL idle state - don't record
	}

	return actions, nil
}

func (e *ExecutivePolicyShard) queryDelegateActions() ([]ActionDecision, error) {
	results, err := e.Kernel.Query("delegate_task")
	if err != nil {
		return nil, err
	}

	actions := make([]ActionDecision, 0, len(results))
	for _, fact := range results {
		if len(fact.Args) < 3 {
			continue
		}

		shardType := types.ExtractString(fact.Args[0])
		target := types.ExtractString(fact.Args[1])
		status := types.ExtractString(fact.Args[2])
		if status != "/pending" && status != "pending" {
			continue
		}

		actionName := delegatedShardToAction(shardType)
		if actionName == "" || actionName == "/delegate_tool_generator" {
			continue
		}

		payload := map[string]any{
			"delegate_shard": shardType,
		}

		actions = append(actions, ActionDecision{
			ID:        fmt.Sprintf("delegate-%d", time.Now().UnixNano()),
			Action:    actionName,
			Target:    target,
			Payload:   payload,
			RawFact:   fact,
			DerivedAt: time.Now(),
			FromRule:  "delegate_task",
		})
	}

	return actions, nil
}

func delegatedShardToAction(shardType string) string {
	switch strings.TrimSpace(shardType) {
	case "/reviewer", "reviewer":
		return "/delegate_reviewer"
	case "/coder", "coder":
		return "/delegate_coder"
	case "/tester", "tester":
		return "/delegate_tester"
	case "/researcher", "researcher":
		return "/delegate_researcher"
	case "/tool_generator", "tool_generator":
		return "/delegate_tool_generator"
	default:
		return ""
	}
}

// checkBarriers checks for blocking conditions.
func (e *ExecutivePolicyShard) checkBarriers() (bool, string) {
	barrierPredicates := []string{
		"block_commit",
		"block_action",
		"executive_blocked",
		"test_state_blocking",
	}

	for _, predicate := range barrierPredicates {
		results, err := e.Kernel.Query(predicate)
		if err == nil && len(results) > 0 {
			// Extract reason from first result
			reason := predicate
			if len(results[0].Args) > 0 {
				if r, ok := results[0].Args[0].(string); ok {
					reason = r
				}
			}
			return true, reason
		}
	}

	return false, ""
}

// strategiesEqual checks if current strategies match tracked strategies.
func (e *ExecutivePolicyShard) strategiesEqual(new []Strategy) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if len(new) != len(e.activeStrategies) {
		return false
	}

	newNames := make(map[string]bool)
	for _, s := range new {
		newNames[s.Name] = true
	}

	for _, s := range e.activeStrategies {
		if !newNames[s.Name] {
			return false
		}
	}

	return true
}
