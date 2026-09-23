package chat

import (
	"context"
	"strings"

	"codenerd/internal/config"
	"codenerd/internal/core"
	"codenerd/internal/logging"
	"codenerd/internal/perception"
	"codenerd/internal/session"
	"codenerd/internal/types"
)

// =============================================================================
// ROUTING ARBITRATION — the single DECIDE point per turn
// =============================================================================

// RouteKind enumerates the routing lanes derivable by
// policy/routing_arbitration.mg.
type RouteKind int

const (
	// RouteNone means no lane derived, or the kernel could not be asked: the
	// turn is answered by articulation, and nothing is delegated or
	// decomposed. Until 2026-09-23 this was RouteLegacy, and the caller asked
	// Go copies of the delegation and multi-step gates instead, which answered
	// in place of the kernel's "no" (sweep finding F12).
	RouteNone RouteKind = iota
	// RouteRespondDirectly terminates the turn in prose: no clarifier shards,
	// no decomposition, no delegation, no autopoiesis analysis.
	RouteRespondDirectly
	// RouteClarify asks the user before acting (low-confidence mutation).
	RouteClarify
	// RouteMultiStep decomposes the request into sequential subtasks.
	RouteMultiStep
	// RouteDelegate hands the task to a single shard.
	RouteDelegate
)

// String renders the lane for logs and Glass Box events.
func (k RouteKind) String() string {
	switch k {
	case RouteRespondDirectly:
		return "respond_directly"
	case RouteClarify:
		return "clarify"
	case RouteMultiStep:
		return "multi_step"
	case RouteDelegate:
		return "delegate"
	default:
		return "none"
	}
}

// RouteDecision is the arbitration outcome for one turn.
type RouteDecision struct {
	Kind  RouteKind
	Shard string // bare shard name for RouteDelegate ("coder"), "" otherwise
}

// decideRoute asserts this turn's routing EDB (delegation candidate,
// multi-step signals, perception signals, the routing section's thresholds)
// and asks the kernel for the turn's route_decision. All lane logic,
// precedence included, lives in policy/routing_arbitration.mg, which derives
// one lane at most; this helper only ferries facts in and the decision out.
//
// No lane is a decision: RouteNone. A kernel that cannot be asked (nil, or an
// assert or query error) routes the same way, loudly: nothing is delegated or
// decomposed on a decision nobody derived.
func (m *Model) decideRoute(input string, intent perception.Intent, shardType string) RouteDecision {
	none := RouteDecision{Kind: RouteNone}
	if m.kernel == nil {
		logging.RoutingError("[decideRoute] no kernel to ask: nothing is delegated or decomposed this turn")
		return none
	}
	if err := config.EnsureParams(m.kernel, m.Config.GetRoutingConfig().Params()); err != nil {
		logging.RoutingError("[decideRoute] the routing thresholds were not asserted: %v", err)
		return none
	}

	shardAtomStr := "/none"
	if shardType != "" {
		if strings.HasPrefix(shardType, "/") {
			shardAtomStr = shardType
		} else {
			shardAtomStr = "/" + shardType
		}
	}
	// The classifier's confidence is a ratio; the policy compares it as a
	// percentage, so a value outside [0,1] would be a negative or >100
	// score that no rule threshold anticipates.
	confInt := int64(min(max(intent.Confidence, 0), 1) * 100)

	// Refresh the per-turn EDB. Retract-before-assert so stale facts from the
	// previous turn can never influence this decision.
	_ = m.kernel.Retract("delegation_candidate")
	_ = m.kernel.Retract("multi_step_signal")
	_ = m.kernel.Retract("intent_signal")

	if err := m.kernel.Assert(core.Fact{
		Predicate: "delegation_candidate",
		Args:      []any{"/current_intent", types.MangleAtom(shardAtomStr), confInt},
	}); err != nil {
		logging.RoutingError("[decideRoute] assert delegation_candidate failed: %v", err)
		return none
	}
	for _, sig := range multiStepSignals(input, intent) {
		if err := m.kernel.Assert(core.Fact{
			Predicate: "multi_step_signal",
			Args:      []any{types.MangleAtom(sig)},
		}); err != nil {
			logging.RoutingError("[decideRoute] assert multi_step_signal failed: %v", err)
			return none
		}
	}
	var signals []string
	if intent.IsQuestion {
		signals = append(signals, "/is_question")
	}
	if trimmed := strings.TrimSpace(input); trimmed != "" && strings.EqualFold(trimmed, strings.TrimSpace(m.lastClarifyInput)) {
		signals = append(signals, "/clarified_already")
	}
	for _, sig := range signals {
		if err := m.kernel.Assert(core.Fact{
			Predicate: "intent_signal",
			Args:      []any{types.MangleAtom(sig)},
		}); err != nil {
			logging.RoutingError("[decideRoute] assert intent_signal %s failed: %v", sig, err)
			return none
		}
	}

	facts, err := m.kernel.Query("route_decision")
	if err != nil {
		logging.RoutingError("[decideRoute] query route_decision failed: %v", err)
		return none
	}
	if len(facts) == 0 {
		// No lane derived (e.g. a /query that is not a question, or a shard
		// candidate below the threshold that is not a mutation): the turn is
		// answered by articulation.
		logging.Routing("[decideRoute] no route_decision derived (verb=%s question=%v shard=%s conf=%d): articulation answers",
			intent.Verb, intent.IsQuestion, shardAtomStr, confInt)
		return none
	}
	if len(facts) > 1 {
		// The policy derives one lane at most; two is a contradiction in it,
		// and acting on either would be Go picking.
		logging.RoutingError("[decideRoute] the policy derived %d lanes, not one: %v", len(facts), facts)
		return none
	}

	f := facts[0]
	if len(f.Args) != 2 {
		logging.RoutingError("[decideRoute] malformed route_decision %v", f.Args)
		return none
	}
	shard := strings.TrimPrefix(types.ExtractString(f.Args[1]), "/")
	if shard == "none" {
		shard = ""
	}
	var decision RouteDecision
	switch route := types.ExtractString(f.Args[0]); route {
	case "/respond_directly":
		decision = RouteDecision{Kind: RouteRespondDirectly}
	case "/multi_step":
		decision = RouteDecision{Kind: RouteMultiStep}
	case "/delegate":
		decision = RouteDecision{Kind: RouteDelegate, Shard: shard}
	case "/clarify":
		decision = RouteDecision{Kind: RouteClarify}
	default:
		logging.RoutingError("[decideRoute] the policy derived an unknown lane %s", route)
		return none
	}

	logging.Routing("[decideRoute] kernel decision: %s shard=%q (verb=%s question=%v)",
		decision.Kind, decision.Shard, intent.Verb, intent.IsQuestion)
	return decision
}

// shouldVerifyDelegation scopes the quality-verification retry loop to
// mutations. Verification re-runs the shard up to 3 times with an extra LLM
// verification call per attempt — worth it when code was written, pure
// overhead (and a major latency amplifier) for read-only query work like
// reviews and analyses.
func shouldVerifyDelegation(intent perception.Intent) bool {
	return intent.Category == "/mutation"
}

// resolveShardTypeForIntent picks a concrete shard for delegation.
// Priority:
//  1. Verb corpus mapping (GetShardTypeForVerb)
//  2. LLM-suggested primary_shard from perception Ambiguity (shard=researcher)
//  3. Heuristic: high-confidence whole-codebase /explain → researcher
//
// Live test: "teach me about the codebase" had verb=/explain (ShardType=/none)
// and ambiguity shard=researcher but we never delegated — user waited forever
// on articulation behind /init. Honor the LLM suggestion.
func resolveShardTypeForIntent(intent perception.Intent) string {
	if st := perception.GetShardTypeForVerb(intent.Verb); st != "" && st != "/none" {
		return strings.TrimPrefix(st, "/")
	}
	for _, a := range intent.Ambiguity {
		if strings.HasPrefix(a, "shard=") {
			s := strings.TrimSpace(strings.TrimPrefix(a, "shard="))
			if s != "" && s != "none" && s != "/none" {
				return strings.TrimPrefix(s, "/")
			}
		}
	}
	return ""
}

// shardTypeToTaskRequest hands a persona or intent verb to the task executor
// as it is: the executor's intentFor is the one place a persona becomes a
// verb (the kernel's persona_verb table). Chat kept its own table
// (personaToIntent) until 2026-09-23, and it disagreed with the executor's
// (sweep finding F6).
func shardTypeToTaskRequest(shardType, task string) session.TaskRequest {
	return session.TaskRequest{IntentVerb: strings.TrimSpace(shardType), Task: task}
}

func (m *Model) withShardModelContext(ctx context.Context, shardType string) context.Context {
	if m == nil || m.Config == nil {
		return ctx
	}
	profile := m.Config.GetShardProfile(strings.TrimSpace(shardType))
	// The profile's sampling rides on the context beside the model override;
	// every client's request builder reads it (types.TemperatureFor). Unset
	// fields attach nothing, so a profile that never chose a temperature
	// leaves the client's own default in force.
	return config.ShardProfileContext(ctx, profile)
}

// shardMaxRetries is a delegation's attempt cap for a shard type, from its
// profile (shard_profiles.<type>.max_retries); VerifyWithRetry refuses a
// non-positive one.
func (m *Model) shardMaxRetries(shardType string) int {
	if m == nil || m.Config == nil {
		return 0
	}
	return m.Config.GetShardProfile(strings.TrimSpace(shardType)).MaxRetries
}

// shardLearningEnabled reports whether a shard type's profile allows its runs
// to be recorded for prompt evolution. Without a config every run is recorded.
func (m *Model) shardLearningEnabled(shardType string) bool {
	if m == nil || m.Config == nil {
		return true
	}
	return m.Config.GetShardProfile(strings.TrimSpace(shardType)).EnableLearning
}
