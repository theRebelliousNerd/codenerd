package config

import (
	"errors"
	"fmt"

	"codenerd/internal/types"
)

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

// ParamKernel is what EnsureParams needs of a kernel.
type ParamKernel interface {
	Query(predicate string) ([]types.Fact, error)
	Assert(fact types.Fact) error
	RetractFact(fact types.Fact) error
}

// EnsureParams puts params into the kernel as config_param rows before a
// rule that reads them is asked. The kernel is shared and outlives a config,
// and a rule over an absent threshold derives nothing, so a row that is
// missing or holds another value is replaced; one that already holds this
// value is left alone, since config_param is replicated into every shard and
// re-asserting it re-evaluates all of them. Every key that could not be
// asserted is in the error.
func EnsureParams(k ParamKernel, params []Param) error {
	held := map[string]int64{}
	if rows, err := k.Query(ConfigParamPredicate); err == nil {
		for _, f := range rows {
			if len(f.Args) != 2 {
				continue
			}
			if v, ok := f.Args[1].(int64); ok {
				held[types.ExtractString(f.Args[0])] = v
			}
		}
	}
	var failed []error
	for _, p := range params {
		if v, ok := held[p.Key]; ok && v == p.Value {
			continue
		}
		key := types.MangleAtom(p.Key)
		_ = k.RetractFact(types.Fact{Predicate: ConfigParamPredicate, Args: []any{key}})
		if err := k.Assert(types.Fact{Predicate: ConfigParamPredicate, Args: []any{key, p.Value}}); err != nil {
			failed = append(failed, fmt.Errorf("config_param %s: %w", p.Key, err))
		}
	}
	return errors.Join(failed...)
}

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
