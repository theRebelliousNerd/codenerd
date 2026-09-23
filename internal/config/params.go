package config

import "codenerd/internal/types"

// Param is one numeric knob a Mangle rule reads, as the fact
// config_param(Key, Value): the generic bridge from config.json to the policy
// (internal/core/defaults/policy/config_params.mg).
//
// It is one predicate for every section rather than a fact per feature. The
// campaign had one of those, campaign_config/5, and its values came from Go
// defaults the config file could not reach; the policy's own thresholds
// (campaign_acceptance_limit(3), task_retry_exhausted's literal 3) had no path
// at all. A section that has knobs the policy needs gives them a Params method
// and asserts ParamFacts; its rules name the keys they need with
// config_param_required(Section, Key), and the component that asserts the
// section refuses to run while config_param_missing(Section, Key) derives.
//
// Values are integers: Mangle compares integers only (a float in a comparison
// aborts the fixpoint), so a switch is 0 or 1 and a ratio is a percent.
type Param struct {
	// Key is the name constant, with its leading slash:
	// "/<section>_<field>", e.g. "/campaign_max_task_attempts".
	Key   string
	Value int64
}

// ConfigParamPredicate is the fact every Param becomes.
const ConfigParamPredicate = "config_param"

// ParamFacts renders params as config_param facts. The key is asserted as a
// name constant on purpose: the rules join on a literal /name.
func ParamFacts(params []Param) []types.Fact {
	out := make([]types.Fact, 0, len(params))
	for _, p := range params {
		out = append(out, types.Fact{
			Predicate: ConfigParamPredicate,
			Args:      []any{types.MangleAtom(p.Key), p.Value},
		})
	}
	return out
}
