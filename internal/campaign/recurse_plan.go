package campaign

import (
	"context"
	"fmt"
)

// RecurseConfig is the operator-facing shape of a recurse run, from the CLI
// flags or the /recurse arguments. Every field is settable from flags; the
// CLI coverage test enforces that.
type RecurseConfig struct {
	// MaxWaves bounds the run in passes over the DAG. Zero means unbounded,
	// which the CLIs only allow with yolo mode.
	MaxWaves int `json:"max_waves"`
	// Subsystems focuses the sweep; dependencies are pulled in automatically.
	Subsystems []string `json:"subsystems,omitzero"`
	// ContextBudget overrides each attempt campaign's context budget. Zero
	// keeps the default.
	ContextBudget int `json:"context_budget"`
}

// Normalize validates the configuration.
func (c RecurseConfig) Normalize() (RecurseConfig, error) {
	if c.MaxWaves < 0 {
		return c, fmt.Errorf("recurse: passes must be >= 0 (0 = unbounded with yolo)")
	}
	if c.ContextBudget < 0 {
		return c, fmt.Errorf("recurse: context budget must be >= 0")
	}
	return c, nil
}

// RecurseSweepOrder derives the workspace's DAG and returns it bottom to top,
// narrowed to subsystems (and what they depend on) when any are named. It is
// the order every pass sweeps and the order `nerd campaign recurse --plan`
// prints.
func RecurseSweepOrder(ctx context.Context, workspace string, subsystems []string) ([]SubsystemNode, error) {
	derived, err := deriveRecurseDAG(ctx, workspace)
	if err != nil {
		return nil, err
	}
	nodes, err := TopoOrder(derived)
	if err != nil {
		return nil, err
	}
	if len(subsystems) == 0 {
		return nodes, nil
	}
	if nodes, err = FilterDAG(nodes, subsystems); err != nil {
		return nil, err
	}
	return TopoOrder(nodes)
}
