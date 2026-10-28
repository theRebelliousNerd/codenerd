package orient

import (
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
