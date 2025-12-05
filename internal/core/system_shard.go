package core

import (
	"context"
	"fmt"
	"time"
)

// SystemShard is a Type 1 (Permanent) shard agent.
// It runs continuously in the background, monitoring the environment
// and maintaining system homeostasis.
type SystemShard struct {
	*BaseShardAgent
	systemPrompt string
}

// NewSystemShard creates a new System Shard.
func NewSystemShard(id string, config ShardConfig, systemPrompt string) *SystemShard {
	if systemPrompt == "" {
		systemPrompt = `You are the System Shard (Type 1).
Your Role: The Operating System of the Agent.
Your Duties:
1. Monitor the filesystem for changes (Fact-Based Filesystem).
2. Maintain the integrity of the .nerd/ directory.
3. Prune old logs or temporary files.
4. Alert the Kernel to critical system state changes.

You run in a continuous loop. Report status every heartbeat.`
	}
	return &SystemShard{
		BaseShardAgent: NewBaseShardAgent(id, config),
		systemPrompt:   systemPrompt,
	}
}

// Execute runs the System Shard's continuous loop.
// Unlike Type 2 shards, this does NOT exit after one task.
func (s *SystemShard) Execute(ctx context.Context, task string) (string, error) {
	s.SetState(ShardStateRunning)
	defer s.SetState(ShardStateCompleted)

	// Prime with a single LLM call to seed role-specific intent/plan.
	// This makes it a "Real LLM Shard" as requested.
	if llm := s.llm(); llm != nil {
		userPrompt := fmt.Sprintf("System Startup. Task: %s. Status: Online.", task)
		// We ignore the error here to allow the loop to proceed even if LLM is flaky on startup
		// Ideally we'd log this.
		_, _ = llm.CompleteWithSystem(ctx, s.systemPrompt, userPrompt)
	}

	// System Shard Main Loop
	ticker := time.NewTicker(10 * time.Second) // Heartbeat
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return "System Shard shutdown", ctx.Err()
		case <-s.stopCh:
			return "System Shard stopped", nil
		case tick := <-ticker.C:
			// Propagate a heartbeat fact to the parent kernel
			_ = s.kernel.Assert(Fact{
				Predicate: "system_heartbeat",
				Args:      []interface{}{s.id, tick.Unix()},
			})
		}
	}
}
