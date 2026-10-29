package session

import (
	"context"
	"fmt"
	"strings"

	"codenerd/internal/logging"
	"codenerd/internal/perception"
	"codenerd/internal/types"
)

// A turn's restriction (perception/restriction.go) reaches the kernel here.
// Go measures -- the recognised kinds, and each tool's reviewed effect
// (tools.LookupEffect; /unknown when a tool declares none) -- and the kernel
// derives what the restriction forbids and owes (policy/constitution.mg, section 7D).
// Nothing in this file decides: it asserts facts and reads back the tools the
// kernel withholds and the verdict on a call. A turn with no enforceable
// restriction asserts nothing and pays nothing.

// assertTurnRestriction tells the kernel which restriction kinds bind the
// turn. It reports an error rather than carrying on unrestricted: a limit
// that could not be recorded is a guard that did not run.
func (e *Executor) assertTurnRestriction(turn types.MangleAtom, r perception.Restriction) error {
	if !r.Enforced() {
		return nil
	}
	if e.kernel == nil {
		return fmt.Errorf("turn restriction %v needs a kernel to enforce it", r.Kinds())
	}
	for _, kind := range r.Kinds() {
		if !e.assertTurnFact(types.Fact{Predicate: "turn_restriction", Args: []any{turn, types.MangleAtom("/" + string(kind))}}) {
			return fmt.Errorf("turn restriction %q could not be recorded in the kernel", kind)
		}
	}
	logging.Session("Turn %s restricted: %v (digest %s)", turn, r.Kinds(), r.Digest())
	return nil
}

// effectAtom is a tool's declared effect as a name constant; a tool with none
// is /unknown, which the policy bars like a write.
func (e *Executor) effectAtom(tool string) types.MangleAtom {
	effect, err := e.modularTools().LookupEffect(strings.TrimPrefix(tool, "/"))
	if err != nil || effect == "" {
		return "/unknown"
	}
	return types.MangleAtom("/" + string(effect))
}

// restrictionWithheldTools measures the candidate tools' effects for the turn
// and returns the ones the kernel withholds under its restriction. It only
// ever narrows. If the measurement cannot be asserted or the kernel cannot be
// asked, every candidate is withheld: no tools is not all tools.
func (e *Executor) restrictionWithheldTools(turn types.MangleAtom, r perception.Restriction, candidates []string) []string {
	if !r.Enforced() || len(candidates) == 0 {
		return nil
	}
	if e.kernel == nil {
		return candidates
	}
	for _, tool := range candidates {
		name := strings.TrimPrefix(tool, "/")
		if !validMangleActionAtom("/" + name) {
			// A name that cannot be an atom cannot be measured: withheld.
			continue
		}
		if !e.assertTurnFact(types.Fact{Predicate: "restriction_tool_effect",
			Args: []any{turn, types.MangleAtom("/" + name), e.effectAtom(name)}}) {
			return candidates
		}
	}
	facts, err := e.kernel.Query(fmt.Sprintf("restriction_withholds_tool(%s, Tool)", turn))
	if err != nil {
		logging.Get(logging.CategorySession).Warn("restriction_withholds_tool query failed: %v; the restricted turn keeps no tools", err)
		return candidates
	}
	var withheld []string
	for _, f := range facts {
		if len(f.Args) == 0 {
			continue
		}
		if tool := strings.TrimPrefix(types.ExtractString(f.Args[len(f.Args)-1]), "/"); tool != "" {
			withheld = append(withheld, tool)
		}
	}
	// A candidate whose name could not be measured is not left in the catalog.
	for _, tool := range candidates {
		if name := strings.TrimPrefix(tool, "/"); !validMangleActionAtom("/" + name) {
			withheld = append(withheld, tool)
		}
	}
	if len(withheld) > 0 {
		logging.Session("Turn catalog: the restriction withholds %v", withheld)
	}
	return withheld
}

// callRestrictionFacts are the facts that bind one admitted call to its turn's
// restriction: the owning turn and the tool's declared effect. The caller
// asserts them beside the pending_action and retracts them with it. Nil when
// the call's turn carries no restriction.
func (e *Executor) callRestrictionFacts(call ToolCall, actionID string) []types.Fact {
	if call.restrictedTurn == "" {
		return nil
	}
	return []types.Fact{
		{Predicate: "pending_action_turn", Args: []any{actionID, call.restrictedTurn}},
		{Predicate: "pending_action_effect", Args: []any{actionID, e.effectAtom(call.Name)}},
	}
}

// bindRestriction marks the call with the turn whose restriction binds it,
// read from the context the turn runs under. A restricted context without a
// turn identity is refused: there is no turn to key the restriction to.
func bindRestriction(ctx context.Context, call ToolCall) (ToolCall, error) {
	r := perception.RestrictionFromContext(ctx)
	if !r.Enforced() {
		return call, nil
	}
	turn, ok := catalogTurnFrom(ctx)
	if !ok {
		return call, fmt.Errorf("tool call refused: the turn is restricted (%v) but carries no turn identity to enforce it against", r.Kinds())
	}
	call.restrictedTurn = turn
	return call, nil
}

// restrictionDenial is the reason a call the restriction bars is refused,
// worded so the model does not retry it another way.
func (e *Executor) restrictionDenial(call ToolCall) string {
	return fmt.Sprintf("action not permitted: the user's request restricts this turn, and %s has %s effect; it is withheld for the whole request — do not retry it or work around it, report what you found instead",
		call.Name, strings.TrimPrefix(string(e.effectAtom(call.Name)), "/"))
}
