// Package main implements the codeNERD CLI commands.
// This file contains shard spawning and agent definition commands.
package main

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"codenerd/internal/config"
	coreshards "codenerd/internal/core/shards"
	coresys "codenerd/internal/system"
	"codenerd/internal/types"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

// =============================================================================
// SHARD SPAWNING COMMANDS - Agent definition and spawning (§7.0, §9.1)
// =============================================================================

// defineAgentCmd defines a new specialist shard (§9.1)
var defineAgentCmd = &cobra.Command{
	Use:   "define-agent",
	Short: "Define a new specialist shard agent",
	Long: `Creates a persistent specialist profile that can be spawned later.
The agent will undergo deep research to build its knowledge base.

Example:
  nerd define-agent --name RustExpert --topic "Tokio Async Runtime"`,
	RunE: defineAgent,
}

// spawnCmd spawns a shard agent (§7.0)
var spawnCmd = &cobra.Command{
	Use:   "spawn [shard-type] [task]",
	Short: "Spawn an ephemeral or persistent shard agent",
	Long: `Spawns a ShardAgent to handle a specific task in isolation.

Shard Types:
  - generalist: Ephemeral, starts blank (RAM only)
  - specialist: Persistent, loads knowledge shard from SQLite
  - coder: Specialized for code writing/TDD loop
  - researcher: Specialized for deep research
  - reviewer: Specialized for code review
  - tester: Specialized for test generation`,
	Args: cobra.MinimumNArgs(2),
	RunE: spawnShard,
}

// defineAgent creates a new specialist shard profile
func defineAgent(cmd *cobra.Command, args []string) error {
	name, _ := cmd.Flags().GetString("name")
	topic, _ := cmd.Flags().GetString("topic")

	// Validate name to prevent path traversal/injection
	if !regexp.MustCompile(`^[a-zA-Z0-9_-]+$`).MatchString(name) {
		return fmt.Errorf("invalid agent name: must be alphanumeric (dash/underscore allowed)")
	}

	logger.Info("Defining specialist agent",
		zap.String("name", name),
		zap.String("topic", topic))

	// Resolve API key
	key := apiKey
	if key == "" {
		key = os.Getenv("ZAI_API_KEY")
	}

	// Boot Cortex to get wired environment
	cortex, err := coresys.GetOrBootCortex(cmd.Context(), workspace, key, disableSystemShards)
	if err != nil {
		return fmt.Errorf("failed to boot cortex: %w", err)
	}
	defer cortex.Close()

	config := coreshards.DefaultSpecialistConfig(name, fmt.Sprintf("memory/shards/%s_knowledge.db", name))

	cortex.ShardManager.DefineProfile(name, config)

	// Trigger deep research phase (§9.2)
	// This spawns a researcher shard to build the knowledge base
	fmt.Printf("Initiating deep research on topic: %s...\n", topic)

	// Use 10 minute timeout for research
	ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Minute)
	defer cancel()

	researchTask := fmt.Sprintf("Research the topic '%s' and generate Mangle facts for the %s agent knowledge base.", topic, name)
	if _, err := cortex.SpawnTask(ctx, "researcher", researchTask); err != nil {
		logger.Warn("Deep research phase failed", zap.Error(err))
		fmt.Printf("Warning: Deep research failed (%v). Agent will start with empty knowledge base.\n", err)
	} else {
		fmt.Println("Deep research complete. Knowledge base populated.")
	}

	fmt.Printf("Agent '%s' defined with topic '%s'\n", name, topic)
	fmt.Println("Knowledge shard will be populated during first spawn.")
	return nil
}

// spawnShard spawns a shard agent
func spawnShard(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
	defer cancel()

	shardType := args[0]
	task := joinArgs(args[1:])

	logger.Info("Spawning shard",
		zap.String("type", shardType),
		zap.String("task", task))

	// Resolve API key
	key := apiKey
	if key == "" {
		key = os.Getenv("ZAI_API_KEY")
	}

	// Boot Cortex
	cortex, err := coresys.GetOrBootCortex(ctx, workspace, key, disableSystemShards)
	if err != nil {
		return fmt.Errorf("failed to boot cortex: %w", err)
	}
	defer cortex.Close()

	normalizedType := normalizeShardType(shardType)
	waitTimeout := spawnWaitTimeout(timeout)

	// Generate shard ID for fact recording
	shardID := fmt.Sprintf("%s-%d", shardType, time.Now().UnixNano())

	var result string
	var spawnErr error
	if cortex.ShardManager != nil {
		if cfg, ok := cortex.ShardManager.GetProfile(normalizedType); ok && cfg.Type == types.ShardTypeSystem {
			result, spawnErr = spawnSystemShardAndWait(ctx, cortex.ShardManager, normalizedType, task, waitTimeout)
		} else {
			result, spawnErr = cortex.SpawnTask(ctx, shardType, task)
		}
	} else {
		result, spawnErr = cortex.SpawnTask(ctx, shardType, task)
	}

	// Record execution facts regardless of success/failure
	facts := cortex.ShardManager.ResultToFacts(shardID, shardType, task, result, spawnErr)
	if len(facts) > 0 {
		if loadErr := cortex.Kernel.LoadFacts(facts); loadErr != nil {
			logger.Warn("Failed to load shard facts into kernel", zap.Error(loadErr))
		} else {
			logger.Debug("Recorded shard execution facts", zap.Int("count", len(facts)))
		}
	}

	if spawnErr != nil {
		return fmt.Errorf("spawn failed: %w", spawnErr)
	}

	fmt.Printf("Shard Result: %s\n", result)
	return nil
}

func normalizeShardType(input string) string {
	return strings.TrimLeft(strings.TrimSpace(input), "/")
}

func spawnWaitTimeout(cmdTimeout time.Duration) time.Duration {
	waitTimeout := config.GetLLMTimeouts().FollowUpTimeout
	if waitTimeout <= 0 {
		waitTimeout = 5 * time.Minute
	}
	if cmdTimeout > 0 && cmdTimeout < waitTimeout {
		return cmdTimeout
	}
	return waitTimeout
}

func spawnSystemShardAndWait(ctx context.Context, manager *coreshards.ShardManager, shardType, task string, waitTimeout time.Duration) (string, error) {
	if manager == nil {
		return "", fmt.Errorf("shard manager unavailable for system shard %s", shardType)
	}

	shardID, err := manager.SpawnAsyncWithContext(ctx, shardType, task, nil)
	if err != nil {
		return "", err
	}

	waitCtx, cancel := context.WithTimeout(ctx, waitTimeout)
	defer cancel()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-waitCtx.Done():
			return "", fmt.Errorf("system shard %s did not complete within %v (id=%s)", shardType, waitTimeout, shardID)
		case <-ticker.C:
			if res, ok := manager.GetResult(shardID); ok {
				if res.Error != nil {
					return "", res.Error
				}
				return res.Result, nil
			}
		}
	}
}
