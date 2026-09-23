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

// taskVerb is the verb a task's turn runs: the kernel's task_delegation
// (campaign_rules.mg 8.3), from the task type's persona through the one
// persona table, except that a new document is created, not fixed. Until
// 2026-09-23 each handler named its own verb, and every file, refactor,
// shard-spawn and generic task ran as /fix (sweep finding F6).
func (o *Orchestrator) taskVerb(task *Task) (string, error) {
	if err := o.ensureTaskRows(task); err != nil {
		return "", err
	}
	return o.oneDerivedFor("task_delegation", task.ID)
}

// ensureTaskRows makes sure the kernel holds the task's own rows before a
// decision about it is asked: a row it does not hold is missing state, not a
// decision. The task in hand supplies them (Task.ToFacts).
func (o *Orchestrator) ensureTaskRows(task *Task) error {
	if o.kernel == nil {
		return fmt.Errorf("no kernel to derive the delegation of %s", task.ID)
	}
	rows, err := o.kernel.Query("campaign_task")
	if err != nil {
		return fmt.Errorf("query campaign_task: %w", err)
	}
	for _, f := range rows {
		if len(f.Args) > 0 && types.ExtractString(f.Args[0]) == task.ID {
			return nil
		}
	}
	if err := o.kernel.LoadFacts(task.ToFacts()); err != nil {
		return fmt.Errorf("load the rows of %s: %w", task.ID, err)
	}
	return nil
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
