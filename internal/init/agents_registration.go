package init

import (
	"encoding/json"
	"fmt"
	"time"

	"codenerd/internal/atomicfile"
	coreshards "codenerd/internal/core/shards"
	"codenerd/internal/types"
)

// sendAgentProgress sends an agent-specific progress update.
func (i *Initializer) sendAgentProgress(name, agentType, status string, kbSize int) {
	if i.config.ProgressChan != nil {
		select {
		case i.config.ProgressChan <- InitProgress{
			Phase:   "agent_creation",
			Message: fmt.Sprintf("Agent %s: %s", name, status),
			AgentUpdate: &AgentCreationUpdate{
				AgentName: name,
				AgentType: agentType,
				Status:    status,
				KBSize:    kbSize,
			},
		}:
		default:
		}
	}
}

// registerAgentsWithShardManager tells a running session's shard manager
// about the agents init created, with the profile boot gives a disk-discovered
// agent (internal/system/factory.go), so the agents spawn the same way before
// and after the next boot.
func (i *Initializer) registerAgentsWithShardManager(agents []CreatedAgent) {
	if i.shardMgr == nil {
		return
	}
	for _, agent := range agents {
		cfg := coreshards.DefaultSpecialistConfig(agent.Name, agent.KnowledgePath)
		cfg.Type = types.ShardTypeUser
		cfg.Tools = agent.Tools
		cfg.ToolPreferences = agent.ToolPreferences
		i.shardMgr.DefineProfile(agent.Name, cfg)
	}
}

// saveAgentRegistry saves the agent registry to disk.
func (i *Initializer) saveAgentRegistry(path string, agents []CreatedAgent) error {
	registry := AgentRegistry{
		Version:   "1.5.0",
		CreatedAt: time.Now(),
		Agents:    agents,
	}

	data, err := json.MarshalIndent(registry, "", "  ")
	if err != nil {
		return err
	}
	return atomicfile.WriteFile(path, data, 0o644)
}
