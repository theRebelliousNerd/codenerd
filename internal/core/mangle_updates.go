package core

import (
	"fmt"
	"strings"
)

// MangleUpdatePolicy constrains which control-packet updates may be asserted.
type MangleUpdatePolicy struct {
	AllowedPredicates map[string]struct{}
	AllowedPrefixes   []string
	MaxUpdates        int
}

// MangleUpdateBlock records a rejected update and the reason.
type MangleUpdateBlock struct {
	Update string
	Reason string
}

// ModelObservationPolicy is the one allowlist for facts a model volunteers
// through control_packet.mangle_updates, shared by every surface that reads
// an envelope (the session executor and the chat turn). A model may report
// what it observed and what it did; it may not write the predicates the
// kernel derives decisions from, and it may not witness its own completion
// (see predicateAllowed).
//
// The prompt atom protocol/piggyback/mangle_updates teaches exactly this set
// with its declared arities. Add a predicate here and there together.
func ModelObservationPolicy() MangleUpdatePolicy {
	return MangleUpdatePolicy{
		AllowedPredicates: map[string]struct{}{
			"missing_tool_for":  {},
			"observation":       {},
			"task_status":       {},
			"task_completed":    {},
			"diagnostic":        {},
			"failing_test":      {},
			"test_state":        {},
			"review_finding":    {},
			"modified":          {},
			"modified_function": {},
			// checkpoint_verdict/4 is the campaign checkpoint's structured
			// reviewer verdict. internal/campaign/checkpoint.go queries the
			// kernel for it after the reviewer spawn and retracts it once
			// read, so it decides one checkpoint and nothing else. Without
			// this entry the verdict was blocked and every checkpoint
			// failed closed (audit campaign 5a2f4c8d, 2026-09-04).
			"checkpoint_verdict": {},
		},
		MaxUpdates: 100,
	}
}

// FilterMangleUpdates parses and validates control-packet mangle_updates.
// It rejects rules/decls/imports, enforces allowed predicates/prefixes, and
// validates predicate declarations/arity when possible.
func FilterMangleUpdates(kernel Kernel, updates []string, policy MangleUpdatePolicy) ([]Fact, []MangleUpdateBlock) {
	if len(updates) == 0 {
		return nil, nil
	}

	maxUpdates := policy.MaxUpdates
	if maxUpdates <= 0 {
		maxUpdates = len(updates)
	}

	factCap := min(len(updates), maxUpdates)
	facts := make([]Fact, 0, factCap)
	blocked := make([]MangleUpdateBlock, 0)

	for i, update := range updates {
		if i >= maxUpdates {
			blocked = append(blocked, MangleUpdateBlock{
				Update: update,
				Reason: "mangle_updates limit exceeded",
			})
			continue
		}

		trimmed := strings.TrimSpace(update)
		if trimmed == "" {
			continue
		}

		if strings.Contains(trimmed, ":-") {
			blocked = append(blocked, MangleUpdateBlock{
				Update: update,
				Reason: "rules are not allowed in mangle_updates",
			})
			continue
		}

		lower := strings.ToLower(trimmed)
		if strings.HasPrefix(lower, "decl ") || strings.HasPrefix(lower, "decl\t") {
			blocked = append(blocked, MangleUpdateBlock{
				Update: update,
				Reason: "Decl statements are not allowed in mangle_updates",
			})
			continue
		}
		if strings.HasPrefix(lower, "import ") || strings.HasPrefix(lower, "include ") {
			blocked = append(blocked, MangleUpdateBlock{
				Update: update,
				Reason: "imports are not allowed in mangle_updates",
			})
			continue
		}

		factText := strings.TrimSuffix(trimmed, ".")
		fact, err := ParseFactString(factText)
		if err != nil {
			blocked = append(blocked, MangleUpdateBlock{
				Update: update,
				Reason: fmt.Sprintf("parse error: %v", err),
			})
			continue
		}

		if !predicateAllowed(fact.Predicate, policy) {
			blocked = append(blocked, MangleUpdateBlock{
				Update: update,
				Reason: fmt.Sprintf("predicate %q not allowed in mangle_updates", fact.Predicate),
			})
			continue
		}

		if kernel != nil {
			if ok, reason := validatePredicateDeclaration(kernel, fact.Predicate, len(fact.Args)); !ok {
				blocked = append(blocked, MangleUpdateBlock{
					Update: update,
					Reason: reason,
				})
				continue
			}
		}

		facts = append(facts, fact)
	}

	return facts, blocked
}

func predicateAllowed(predicate string, policy MangleUpdatePolicy) bool {
	// These are host witnesses and conclusions, never model observations.
	// Even a permissive caller allowlist cannot delegate their authority.
	switch predicate {
	case "turn_acceptance", "turn_evidence", "turn_executed", "turn_done", "turn_cost":
		return false
	}
	if len(policy.AllowedPredicates) == 0 && len(policy.AllowedPrefixes) == 0 {
		return true
	}
	if _, ok := policy.AllowedPredicates[predicate]; ok {
		return true
	}
	for _, prefix := range policy.AllowedPrefixes {
		if strings.HasPrefix(predicate, prefix) {
			return true
		}
	}
	return false
}

func validatePredicateDeclaration(kernel Kernel, predicate string, arity int) (bool, string) {
	rk, ok := kernel.(*RealKernel)
	if !ok || rk == nil {
		return true, ""
	}

	rk.mu.RLock()
	programInfo := rk.programInfo
	schemaValidator := rk.schemaValidator
	rk.mu.RUnlock()

	if programInfo != nil && programInfo.Decls != nil {
		for predSym := range programInfo.Decls {
			if predSym.Symbol != predicate {
				continue
			}
			if predSym.Arity != arity {
				return false, fmt.Sprintf("arity mismatch: %s expects %d args (got %d)", predicate, predSym.Arity, arity)
			}
			return true, ""
		}
		return false, fmt.Sprintf("predicate %q not declared", predicate)
	}

	if schemaValidator != nil && !schemaValidator.IsDeclared(predicate) {
		return false, fmt.Sprintf("predicate %q not declared", predicate)
	}

	return true, ""
}
