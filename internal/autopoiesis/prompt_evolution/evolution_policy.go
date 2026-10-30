package prompt_evolution

import (
	"context"
	"fmt"

	nerdconfig "codenerd/internal/config"
	"codenerd/internal/core/defaults"
	"codenerd/internal/mangle"
	"codenerd/internal/types"
)

// newEvolutionPolicy builds a bounded-lifetime component kernel from the same
// Decls and rules as the action kernel. Missing policy is an error; no Go
// branch substitutes for a failed derivation.
func newEvolutionPolicy(cfg EvolverConfig) (*mangle.Engine, error) {
	e, err := mangle.NewEngine(evolutionEngineConfig(), nil)
	if err != nil {
		return nil, err
	}
	if err := e.LoadSchemaString(defaults.ConfigParamsPolicy + defaults.EvolutionSchema + defaults.EvolutionPolicy); err != nil {
		return nil, err
	}
	params := nerdconfig.ParamFacts([]nerdconfig.Param{
		{Key: "/evolution_cohort_size", Value: int64(cfg.EvidenceCohortSize)},
		{Key: "/evolution_screen_calls", Value: int64(cfg.MaxCycleScreeningCalls)},
	})
	facts := make([]mangle.Fact, 0, len(params))
	for _, p := range params {
		facts = append(facts, mangle.Fact{Predicate: p.Predicate, Args: p.Args})
	}
	if err := e.AddFacts(facts); err != nil {
		return nil, err
	}
	// The policy declares the knobs it needs; a missing one refuses the
	// kernel rather than letting a rule over it derive nothing.
	missing, err := e.Query(context.Background(), "config_param_missing(/evolution, Key)")
	if err != nil {
		return nil, err
	}
	if len(missing.Bindings) > 0 {
		return nil, fmt.Errorf("prompt evolution policy is missing config: %v", missing.Bindings)
	}
	return e, nil
}

func (c *evidenceCohort) screenAllowed(ctx context.Context, used int) (bool, error) {
	if err := c.policy.AddFact("evolution_screen_used", c.id, used); err != nil {
		return false, err
	}
	r, err := c.policy.Query(ctx, fmt.Sprintf("evolution_screen_allowed(%q)", c.id))
	return err == nil && len(r.Bindings) > 0, err
}

func (c *evidenceCohort) generationAllowed(ctx context.Context, u *jevUse, status string, p float64) (bool, error) {
	facts := []mangle.Fact{{Predicate: "evolution_screen_observation", Args: []any{c.id, types.MangleAtom("/" + status)}}}
	if status == "answered" {
		facts = append(facts,
			mangle.Fact{Predicate: "evolution_screen_probability", Args: []any{c.id, p}},
			mangle.Fact{Predicate: "evolution_screen_threshold", Args: []any{c.id, u.yes}},
		)
	}
	if err := c.policy.AddFactsContext(ctx, facts); err != nil {
		return false, err
	}
	r, err := c.policy.Query(ctx, fmt.Sprintf("evolution_generation_allowed(%q)", c.id))
	return err == nil && len(r.Bindings) > 0, err
}

func evolutionEngineConfig() mangle.Config {
	cfg := mangle.DefaultConfig()
	// Derived membership and permission rows must be ready after assertions,
	// independent of the optional automatic-evaluation setting for other engines.
	cfg.AutoEval = true
	return cfg
}
