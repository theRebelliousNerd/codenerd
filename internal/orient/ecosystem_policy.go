package orient

import (
	"context"

	"codenerd/internal/config"
	"codenerd/internal/types"
)

type Outcome struct {
	OrientAgent            []types.Fact
	OrientAgentTopic       []types.Fact
	OrientAgentDescription []types.Fact
	OrientAgentPermission  []types.Fact
	OrientAgentPriority    []types.Fact
	OrientAgentKnowledge   []types.Fact
	OrientResearchTopic    []types.Fact
	AgentSourceWinner      []types.Fact
	AgentSourceLoser       []types.Fact
	AgentSourceDuplicate   []types.Fact
	ConfigParam            []types.Fact
	ConfigParamMissing     []types.Fact
}

// Run evaluates the same embedded program used by init over caller measurements.
func Run(ctx context.Context, facts []types.Fact) (*Outcome, error) {
	params := config.DefaultOrientConfig().Params()
	overridden := map[string]bool{}
	for _, fact := range facts {
		if fact.Predicate == "config_param" && len(fact.Args) == 2 {
			overridden[types.ExtractString(fact.Args[0])] = true
		}
	}
	var kept []config.Param
	for _, param := range params {
		if !overridden[types.ExtractString(param.Key)] {
			kept = append(kept, param)
		}
	}
	for _, fact := range facts {
		if fact.Predicate == "config_param" && len(fact.Args) == 2 {
			kept = append(kept, config.Param{Key: types.ExtractString(fact.Args[0]), Value: number(fact.Args[1])})
		}
	}
	engine, err := newEngine(kept)
	if err != nil {
		return nil, err
	}
	defer engine.Close()
	if err := engine.Assert(facts); err != nil {
		return nil, err
	}
	if err := engine.Evaluate(ctx); err != nil {
		return nil, err
	}
	out := &Outcome{}
	var qerr error
	take := func(pred string, dst *[]types.Fact) {
		if qerr != nil {
			return
		}
		rows, err := engine.Query(pred)
		if err != nil {
			qerr = err
			return
		}
		*dst = rows
	}
	take("orient_agent", &out.OrientAgent)
	take("orient_agent_topic", &out.OrientAgentTopic)
	take("orient_agent_description", &out.OrientAgentDescription)
	take("orient_agent_permission", &out.OrientAgentPermission)
	take("orient_agent_priority", &out.OrientAgentPriority)
	take("orient_agent_knowledge", &out.OrientAgentKnowledge)
	take("orient_research_topic", &out.OrientResearchTopic)
	take("agent_source_winner", &out.AgentSourceWinner)
	take("agent_source_loser", &out.AgentSourceLoser)
	take("agent_source_duplicate", &out.AgentSourceDuplicate)
	take("config_param", &out.ConfigParam)
	take("config_param_missing", &out.ConfigParamMissing)
	if qerr != nil {
		return nil, qerr
	}
	return out, nil
}
