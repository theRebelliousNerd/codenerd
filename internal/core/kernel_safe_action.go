package core

import (
	"fmt"
	"sort"
	"strings"

	"codenerd/internal/types"
)

// =============================================================================
// SAFE_ACTION PROJECTION — canonical policy-consumer helper
// =============================================================================
//
// Policy consumers (VirtualStore permission cache, validators, trace
// explanations, canary tests) all need the same view: the set of actions the
// constitution classifies as safe via safe_action/1 facts.
//
// Before this projection each consumer issued its own raw
// k.Query("safe_action") call and hand-rolled the same normalization loop
// (types.ExtractString on Args[0], skip zero-arg rows, store both "/foo" and
// "foo" spellings). That duplication drifted: one consumer trimmed spaces,
// another did not; one stored only the raw spelling and missed alias lookups.
//
// This file is the single canonical projection. New policy consumers must use
// ProjectSafeActions / ListSafeActions / LookupSafeAction instead of querying
// "safe_action" directly. VirtualStore.rebuildPermissionCache is the reference
// adopter.
//
// Design notes:
//   - The projection is read-only: it never asserts or retracts facts.
//   - The returned set contains BOTH spellings ("/read_file" and "read_file")
//     for every action so callers can look up whichever form they hold without
//     normalizing first. ListSafeActions returns only the canonical slash form.
//   - Canonical form is slash-prefixed ("/read_file"), matching constitution.mg
//     where every safe_action fact is declared with a leading slash.
//   - Fail-closed: a nil kernel, a query error, or an empty action string never
//     yields permission. Callers treat a returned error as "deny".
//   - These helpers are intentionally NOT part of the types.Kernel interface.
//     Adding a method there would force every adapter and mock kernel in the
//     tree (chat, campaign, system, typestest) to implement it. Free functions
//     over the existing Query method work for RealKernel, CortexKernel, and
//     every stub without breaking the interface.

// CanonicalizeSafeActionName normalizes an action name to its canonical
// slash-prefixed form (e.g., "read_file" -> "/read_file").
// Returns "" for blank input so callers can fail closed on empty names.
func CanonicalizeSafeActionName(action string) string {
	action = strings.TrimSpace(action)
	if action == "" {
		return ""
	}
	if strings.HasPrefix(action, "/") {
		return action
	}
	return "/" + action
}

// ProjectSafeActions returns the O(1) lookup set of safe_action/1 facts
// derived by the kernel. Each action appears under both spellings so direct
// map lookups succeed regardless of leading-slash convention.
//
// Zero-arg rows and blank action names are skipped: they carry no
// classification. A nil kernel or a query failure returns a non-nil error and
// the caller must deny.
func ProjectSafeActions(k types.Kernel) (map[string]bool, error) {
	if k == nil {
		return nil, fmt.Errorf("cannot project safe_action: nil kernel")
	}
	results, err := k.Query("safe_action")
	if err != nil {
		return nil, fmt.Errorf("query safe_action: %w", err)
	}
	cache := make(map[string]bool, len(results)*2)
	for _, f := range results {
		if len(f.Args) == 0 {
			continue
		}
		action := types.ExtractString(f.Args[0])
		if strings.TrimSpace(action) == "" {
			continue
		}
		cache[action] = true
		if after, ok := strings.CutPrefix(action, "/"); ok {
			cache[after] = true
		} else {
			cache["/"+action] = true
		}
	}
	return cache, nil
}

// ListSafeActions returns the sorted canonical (slash-prefixed) names of all
// safe_action/1 facts. Use it when a stable ordered list is needed for
// logging, inventory, or tests. Use ProjectSafeActions when O(1) lookups
// are needed.
func ListSafeActions(k types.Kernel) ([]string, error) {
	if k == nil {
		return nil, fmt.Errorf("cannot list safe_action: nil kernel")
	}
	results, err := k.Query("safe_action")
	if err != nil {
		return nil, fmt.Errorf("query safe_action: %w", err)
	}
	seen := make(map[string]struct{}, len(results))
	out := make([]string, 0, len(results))
	for _, f := range results {
		if len(f.Args) == 0 {
			continue
		}
		canonical := CanonicalizeSafeActionName(types.ExtractString(f.Args[0]))
		if canonical == "" {
			continue
		}
		if _, dup := seen[canonical]; dup {
			continue
		}
		seen[canonical] = struct{}{}
		out = append(out, canonical)
	}
	sort.Strings(out)
	return out, nil
}

// LookupSafeAction reports whether action is in the projected set.
// It is nil-safe (nil set denies) and slash-insensitive: a set built by
// ProjectSafeActions already holds both spellings, but this also checks the
// counterpart spelling so sets built elsewhere with only canonical names
// still match. Blank action names always deny.
func LookupSafeAction(set map[string]bool, action string) bool {
	if len(set) == 0 || strings.TrimSpace(action) == "" {
		return false
	}
	if set[action] {
		return true
	}
	if after, ok := strings.CutPrefix(action, "/"); ok {
		return set[after]
	}
	return set["/"+action]
}

// SafeActionSet projects this kernel's safe_action/1 facts into the canonical
// O(1) lookup set. It is a thin method wrapper over ProjectSafeActions so
// holders of a concrete *RealKernel need not import the free function.
func (k *RealKernel) SafeActionSet() (map[string]bool, error) {
	return ProjectSafeActions(k)
}

// SafeActionList returns this kernel's sorted canonical safe-action names.
func (k *RealKernel) SafeActionList() ([]string, error) {
	return ListSafeActions(k)
}

// IsSafeAction reports whether action is classified safe by this kernel.
// Query failures fail closed (return false) so policy gates deny on error.
func (k *RealKernel) IsSafeAction(action string) bool {
	set, err := ProjectSafeActions(k)
	if err != nil {
		return false
	}
	return LookupSafeAction(set, action)
}

// SafeActionSet projects this cortex's safe_action/1 facts. CortexKernel fans
// out unowned-predicate queries across shards, so the same free function
// covers the federated case without duplicating normalization logic.
func (c *CortexKernel) SafeActionSet() (map[string]bool, error) {
	return ProjectSafeActions(c)
}

// SafeActionList returns this cortex's sorted canonical safe-action names.
func (c *CortexKernel) SafeActionList() ([]string, error) {
	return ListSafeActions(c)
}

// IsSafeAction reports whether action is classified safe by this cortex.
// Query failures fail closed (return false).
func (c *CortexKernel) IsSafeAction(action string) bool {
	set, err := ProjectSafeActions(c)
	if err != nil {
		return false
	}
	return LookupSafeAction(set, action)
}
