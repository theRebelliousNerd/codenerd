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

	factCap := len(updates)
	if factCap > maxUpdates {
		factCap = maxUpdates
	}
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
