package types

import (
	"path/filepath"
	"strings"
)

// AgentKnowledgeDBPath is where an agent's knowledge store lives under a
// workspace's .nerd directory. Init writes it; the prompt loader, prompt sync,
// the agent registry, shard spawn, campaign specialist lookups and the agent
// wizard read it. One definition, because a reader that disagreed with the
// writer found no knowledge and said nothing.
func AgentKnowledgeDBPath(nerdDir, agent string) string {
	return filepath.Join(nerdDir, "shards", strings.ToLower(agent)+"_knowledge.db")
}
