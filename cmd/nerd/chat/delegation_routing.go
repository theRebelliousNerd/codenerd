package chat

import (
	"context"
	"strings"

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
	// RouteLegacy means the kernel was unavailable or produced no decision;
	// callers fall back to the legacy Go booleans (shouldDelegate,
	// detectMultiStepTask) so a kernel hiccup never bricks routing.
	RouteLegacy RouteKind = iota
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
		return "legacy"
	}
}

// RouteDecision is the arbitration outcome for one turn.
type RouteDecision struct {
	Kind  RouteKind
	Shard string // bare shard name for RouteDelegate ("coder"), "" otherwise
}

// decideRoute asserts this turn's routing EDB (delegation candidate,
// multi-step signals, perception signals) and asks the kernel for the single
// route_decision. All lane logic lives in policy/routing_arbitration.mg; this
// helper only ferries facts in and the decision out.
//
// Precedence when several lanes derive: respond_directly > multi_step >
// delegate > clarify (respond_directly is mutually exclusive by construction;
// multi_step/delegate can co-derive and decomposition wins, matching the
// legacy waterfall order).
//
// Fail-safe: a nil kernel, assert/query error, or empty derivation returns
// RouteLegacy and the caller uses the legacy Go gates.
func (m *Model) decideRoute(input string, intent perception.Intent, shardType string) RouteDecision {
	legacy := RouteDecision{Kind: RouteLegacy}
	if m.kernel == nil {
		return legacy
	}

	shardAtomStr := "/none"
	if shardType != "" {
		if strings.HasPrefix(shardType, "/") {
			shardAtomStr = shardType
		} else {
			shardAtomStr = "/" + shardType
		}
	}
	confInt := int64(intent.Confidence * 100)

	// Refresh the per-turn EDB. Retract-before-assert so stale facts from the
	// previous turn can never influence this decision.
	_ = m.kernel.Retract("delegation_candidate")
	_ = m.kernel.Retract("multi_step_signal")
	_ = m.kernel.Retract("intent_signal")

	if err := m.kernel.Assert(core.Fact{
		Predicate: "delegation_candidate",
		Args:      []any{"/current_intent", types.MangleAtom(shardAtomStr), confInt},
	}); err != nil {
		logging.Routing("[decideRoute] assert delegation_candidate failed, using legacy gates: %v", err)
		return legacy
	}
	for _, sig := range multiStepSignals(input, intent) {
		if err := m.kernel.Assert(core.Fact{
			Predicate: "multi_step_signal",
			Args:      []any{types.MangleAtom(sig)},
		}); err != nil {
			logging.Routing("[decideRoute] assert multi_step_signal failed, using legacy gates: %v", err)
			return legacy
		}
	}
	if intent.IsQuestion {
		if err := m.kernel.Assert(core.Fact{
			Predicate: "intent_signal",
			Args:      []any{types.MangleAtom("/is_question")},
		}); err != nil {
			logging.Routing("[decideRoute] assert intent_signal failed, using legacy gates: %v", err)
			return legacy
		}
	}

	facts, err := m.kernel.Query("route_decision")
	if err != nil {
		logging.Routing("[decideRoute] query route_decision failed, using legacy gates: %v", err)
		return legacy
	}
	if len(facts) == 0 {
		// No lane derived (e.g. /query non-question with no shard mapping).
		// That is a legitimate "no opinion": fall through to the legacy gates,
		// which for this shape end at articulation anyway.
		logging.Routing("[decideRoute] no route_decision derived (verb=%s question=%v shard=%s conf=%d) — legacy gates",
			intent.Verb, intent.IsQuestion, shardAtomStr, confInt)
		return legacy
	}

	derived := make(map[string]string, len(facts)) // route -> shard
	for _, f := range facts {
		if len(f.Args) != 2 {
			continue
		}
		route := types.ExtractString(f.Args[0])
		shard := strings.TrimPrefix(types.ExtractString(f.Args[1]), "/")
		if shard == "none" {
			shard = ""
		}
		if _, seen := derived[route]; !seen {
			derived[route] = shard
		}
	}

	var decision RouteDecision
	switch {
	case hasRoute(derived, "/respond_directly"):
		decision = RouteDecision{Kind: RouteRespondDirectly}
	case hasRoute(derived, "/multi_step"):
		decision = RouteDecision{Kind: RouteMultiStep}
	case hasRoute(derived, "/delegate"):
		decision = RouteDecision{Kind: RouteDelegate, Shard: derived["/delegate"]}
	case hasRoute(derived, "/clarify"):
		decision = RouteDecision{Kind: RouteClarify}
	default:
		return legacy
	}

	logging.Routing("[decideRoute] kernel decision: %s shard=%q (verb=%s question=%v candidates=%v)",
		decision.Kind, decision.Shard, intent.Verb, intent.IsQuestion, derived)
	return decision
}

func hasRoute(derived map[string]string, route string) bool {
	_, ok := derived[route]
	return ok
}

// shouldVerifyDelegation scopes the quality-verification retry loop to
// mutations. Verification re-runs the shard up to 3 times with an extra LLM
// verification call per attempt — worth it when code was written, pure
// overhead (and a major latency amplifier) for read-only query work like
// reviews and analyses.
func shouldVerifyDelegation(intent perception.Intent) bool {
	return intent.Category == "/mutation"
}

// shouldDelegate decides whether the current intent should be delegated to a
// shard. The verb->shard LOOKUP (shardType) is computed in Go by the caller
// (GetShardTypeForVerb reads the perception taxonomy corpus, which is siloed
// from the executive kernel). The DELEGATION DECISION — the confidence gate —
// is migrated to Mangle (Step 4): Go asserts delegation_candidate with the
// shard and confidence, then queries should_delegate.
//
// Fail-safe: if the kernel is nil, the assert/query errors, or the kernel
// returns no should_delegate fact, fall back to the legacy Go boolean
// (shardType != "" && confidence >= 0.5). This guarantees a kernel hiccup can
// never silently disable all delegation — it degrades to the prior behavior.
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
	// Whole-repo explain/teach → researcher even without explicit suggestion
	if intent.Confidence >= 0.7 && (intent.Verb == "/explain" || intent.Verb == "/explore" || intent.Verb == "/search") {
		t := strings.ToLower(intent.Target)
		if strings.Contains(t, "codebase") || strings.Contains(t, "project") ||
			strings.Contains(t, "architecture") || strings.Contains(t, "repository") ||
			strings.Contains(t, "entire") || strings.Contains(t, "whole") {
			return "researcher"
		}
	}
	return ""
}

func (m *Model) shouldDelegate(shardType string, confidence float64) bool {
	legacy := shardType != "" && confidence >= 0.5

	if m.kernel == nil {
		return legacy
	}

	// ShardType atom; /none signals "no shard mapped" so the Mangle rule can
	// reject it without depending on string emptiness.
	shardAtomStr := "/none"
	if shardType != "" {
		if strings.HasPrefix(shardType, "/") {
			shardAtomStr = shardType
		} else {
			shardAtomStr = "/" + shardType
		}
	}

	// Scale the 0.0-1.0 confidence float to a 0-100 integer (matches the
	// action_verified convention; the Mangle gate compares Conf >= 50).
	confInt := int64(confidence * 100)

	candidate := core.Fact{
		Predicate: "delegation_candidate",
		Args:      []any{"/current_intent", types.MangleAtom(shardAtomStr), confInt},
	}
	// Retract any stale candidate from a prior turn before asserting this one,
	// so a leftover high-confidence fact cannot leak into this decision.
	_ = m.kernel.RetractFact(core.Fact{Predicate: "delegation_candidate", Args: []any{"/current_intent"}})
	if err := m.kernel.Assert(candidate); err != nil {
		logging.Routing("[shouldDelegate] assert delegation_candidate failed, using legacy gate: %v", err)
		return legacy
	}

	facts, err := m.kernel.Query("should_delegate")
	if err != nil {
		logging.Routing("[shouldDelegate] query should_delegate failed, using legacy gate: %v", err)
		return legacy
	}
	if len(facts) == 0 {
		// No derivation: either no shard mapped or below threshold. The Mangle
		// rule and the legacy boolean agree on this, so returning the kernel's
		// "no" is correct; but if the kernel somehow lost the candidate fact we
		// just asserted, fall back rather than wrongly suppressing delegation.
		return legacy
	}
	// should_delegate(ShardType) derived -> delegate.
	return true
}

// shardTypeToTaskRequest maps a shard/persona name OR an intent verb into a
// TaskRequest. The executor requires IntentVerb to start with "/", so persona
// names get mapped to their canonical intent (and recorded as Persona for
// downstream routing).
func shardTypeToTaskRequest(shardType, task string) session.TaskRequest {
	st := strings.TrimSpace(shardType)
	if strings.HasPrefix(st, "/") {
		// Already an intent verb.
		return session.TaskRequest{IntentVerb: st, Task: task}
	}
	intent := personaToIntent(st)
	return session.TaskRequest{IntentVerb: intent, Persona: st, Task: task}
}

// personaToIntent maps a persona / agent name to its canonical intent verb.
// Unknown personas fall back to /consult/<name> so the executor can dispatch
// to a consultation flow rather than rejecting the request.
func personaToIntent(persona string) string {
	switch strings.ToLower(persona) {
	case "coder":
		return "/fix"
	case "tester":
		return "/test"
	case "reviewer":
		return "/review"
	case "researcher":
		return "/research"
	case "nemesis":
		return "/attack"
	case "librarian":
		return "/learn"
	case "planner":
		return "/plan"
	case "legislator":
		return "/legislate"
	case "constitution":
		return "/audit"
	case "":
		return "/general"
	default:
		// Custom specialist — route through a consultation intent so the
		// executor and config factory can pick it up by name.
		return "/consult/" + persona
	}
}

func (m *Model) withShardModelContext(ctx context.Context, shardType string) context.Context {
	if m == nil || m.Config == nil {
		return ctx
	}
	profile := m.Config.GetShardProfile(strings.TrimSpace(shardType))
	if strings.TrimSpace(profile.Model) == "" {
		return ctx
	}
	return context.WithValue(ctx, types.CtxKeyModelName, strings.TrimSpace(profile.Model))
}
