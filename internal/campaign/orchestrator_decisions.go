package campaign

import (
	"fmt"
	"sort"

	"codenerd/internal/types"
)

// The orchestrator's decisions are the kernel's (policy/campaign_decisions.mg).
// These helpers ask for one and name the empty answer: a decision the kernel
// did not derive is an error, never a default the driver picks for itself.

// derivedFor returns the second argument of every row of predicate whose first
// argument is key, sorted and de-duplicated. A binary decision predicate is
// keyed by the thing decided about (a task, a phase) and carries the decision
// (/build, /retry, /complete ...) as its second argument.
func (o *Orchestrator) derivedFor(predicate, key string) ([]string, error) {
	if o.kernel == nil {
		return nil, fmt.Errorf("no kernel to derive %s for %s", predicate, key)
	}
	facts, err := o.kernel.Query(predicate)
	if err != nil {
		return nil, fmt.Errorf("query %s: %w", predicate, err)
	}
	seen := map[string]bool{}
	var out []string
	for _, f := range facts {
		if len(f.Args) < 2 || types.ExtractString(f.Args[0]) != key {
			continue
		}
		v := types.ExtractString(f.Args[1])
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	sort.Strings(out)
	return out, nil
}

// holdsFor reports whether the kernel derives the unary predicate for key (a
// phase, a campaign). A failed query is an error, never a default.
func (o *Orchestrator) holdsFor(predicate, key string) (bool, error) {
	if o.kernel == nil {
		return false, fmt.Errorf("no kernel to derive %s for %s", predicate, key)
	}
	facts, err := o.kernel.Query(predicate)
	if err != nil {
		return false, fmt.Errorf("query %s: %w", predicate, err)
	}
	for _, f := range facts {
		if len(f.Args) > 0 && types.ExtractString(f.Args[0]) == key {
			return true, nil
		}
	}
	return false, nil
}

// oneDerivedFor is derivedFor for a decision that must have exactly one answer.
func (o *Orchestrator) oneDerivedFor(predicate, key string) (string, error) {
	got, err := o.derivedFor(predicate, key)
	if err != nil {
		return "", err
	}
	switch len(got) {
	case 0:
		return "", fmt.Errorf("the kernel derived no %s for %s", predicate, key)
	case 1:
		return got[0], nil
	default:
		return "", fmt.Errorf("the kernel derived %d %s rows for %s (%v); the policy must decide one", len(got), predicate, key, got)
	}
}
