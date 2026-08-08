// Package main implements the codeNERD CLI commands.
// This file contains shard spawning and agent definition commands.
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
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
  - tester: Specialized for test generation
  - image_generator: Gemini Nano Banana 2 image gen (gemini-3.1-flash-image; never Ollama)`,
	Args: cobra.MinimumNArgs(2),
	RunE: spawnShard,
}

// defineAgent creates a new specialist shard profile
func defineAgent(cmd *cobra.Command, args []string) error {
	name, _ := cmd.Flags().GetString("name")
	topic, _ := cmd.Flags().GetString("topic")

	// Validate name to prevent path traversal/injection.
	if err := coresys.ValidateAgentName(name); err != nil {
		return err
	}

	logger.Info("Defining specialist agent",
		zap.String("name", name),
		zap.String("topic", topic))

	ws := workspace
	if strings.TrimSpace(ws) == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("resolve workspace: %w", err)
		}
		ws = cwd
	}

	// Persist the definition BEFORE booting. This command used to only call
	// ShardManager.DefineProfile — an in-memory map that died with the process —
	// so a "defined" agent left nothing on disk and `nerd spawn <name>` in the
	// next process found no prompts, no knowledge DB, and no registry entry.
	// Writing prompts.yaml first also means the Cortex boot below discovers the
	// agent, syncs its atoms into .nerd/shards/<name>_knowledge.db, and registers
	// that DB with the JIT compiler in this same run.
	promptsPath, err := coresys.WriteAgentDefinition(ws, name, topic, topic)
	if err != nil {
		return fmt.Errorf("failed to write agent definition: %w", err)
	}
	fmt.Printf("Wrote agent definition: %s\n", promptsPath)

	// Resolve API key
	key := resolveAPIKey(apiKey, workspace)

	// Boot Cortex to get wired environment
	cortex, err := coresys.GetOrBootCortex(cmd.Context(), workspace, key, disableSystemShards)
	if err != nil {
		return fmt.Errorf("failed to boot cortex: %w", err)
	}
	defer cortex.Close()

	knowledgePath := filepath.Join(ws, ".nerd", "shards", fmt.Sprintf("%s_knowledge.db", strings.ToLower(name)))
	config := coreshards.DefaultSpecialistConfig(name, knowledgePath)
	config.Type = types.ShardTypeUser
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
	fmt.Printf("Edit %s to shape its identity, then run: nerd spawn %s \"<task>\"\n", promptsPath, name)
	return nil
}

// spawnShard spawns a shard agent
func spawnShard(cmd *cobra.Command, args []string) error {
	// Image gen gets a tighter outer budget so missing/slow Gemini cannot hold
	// the CLI for the full 25m default timeout (live matrix hang).
	cmdTimeout := timeout
	if config.IsImageShardType(normalizeShardType(args[0])) && (cmdTimeout <= 0 || cmdTimeout > 3*time.Minute) {
		cmdTimeout = 3 * time.Minute
	}
	ctx, cancel := context.WithTimeout(cmd.Context(), cmdTimeout)
	defer cancel()

	shardType := args[0]
	task := joinArgs(args[1:])

	logger.Info("Spawning shard",
		zap.String("type", shardType),
		zap.String("task", task))

	// Resolve API key
	key := resolveAPIKey(apiKey, workspace)

	// Boot Cortex
	cortex, err := coresys.GetOrBootCortex(ctx, workspace, key, disableSystemShards)
	if err != nil {
		return fmt.Errorf("failed to boot cortex: %w", err)
	}
	defer cortex.Close()

	normalizedType := normalizeShardType(shardType)
	waitTimeout := spawnWaitTimeout(cmdTimeout)

	// Fail fast for image shards when Nano Banana 2 client is not wired.
	// Without this, Spawn could park on BaseShardAgent/queue with no progress.
	if config.IsImageShardType(normalizedType) {
		if cortex.ShardManager == nil {
			return fmt.Errorf("image_generator requires ShardManager with Gemini Nano Banana 2 client")
		}
		// Probe by attempting spawn; Spawn itself validates image client.
	}

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
	if cortex.ShardManager != nil {
		facts := cortex.ShardManager.ResultToFacts(shardID, shardType, task, result, spawnErr)
		if len(facts) > 0 {
			if loadErr := cortex.Kernel.LoadFacts(facts); loadErr != nil {
				logger.Warn("Failed to load shard facts into kernel", zap.Error(loadErr))
			} else {
				logger.Debug("Recorded shard execution facts", zap.Int("count", len(facts)))
			}
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
