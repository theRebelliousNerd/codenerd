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
	// No research runs here, so the domain atom carries no knowledge yet.
	promptsPath, err := coresys.WriteAgentDefinition(ws, name, topic, topic, "")
	if err != nil {
		return fmt.Errorf("failed to write agent definition: %w", err)
	}
	fmt.Printf("Wrote agent definition: %s\n", promptsPath)

	// Resolve API key
	key := resolveAPIKey(apiKey, workspace)

	// Boot and research share the command context. The only deadline is
	// --timeout; unset means the research runs until it finishes or the
	// command is cancelled. Each LLM call inside the researcher still has
	// its own request bound.
	ctx, cancel := commandContext(cmd)
	defer cancel()

	cortex, err := coresys.GetOrBootCortex(ctx, workspace, key, disableSystemShards)
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
	// Image generation's request bound lives on the image client. This
	// command adds a deadline only when the user set --timeout; there is
	// no image-shard clamp and no default wait.
	ctx, cancel := commandContext(cmd)
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
	var outcome types.MangleAtom
	var spawnErr error
	if cortex.ShardManager != nil {
		if cfg, ok := cortex.ShardManager.GetProfile(normalizedType); ok && cfg.Type == types.ShardTypeSystem {
			var res types.ShardResult
			res, spawnErr = spawnSystemShardAndWait(ctx, cortex.ShardManager, normalizedType, task)
			result, outcome = res.Result, res.Outcome
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

	// The heading states the verdict the producer recorded. A run nobody
	// verified is not reported under the same word as one that was: the
	// ShardManager path observes only "it did not error", and saying so is the
	// difference between a result and a claim.
	fmt.Printf("Shard Result%s: %s\n", shardVerdictSuffix(outcome), result)
	return nil
}

// shardVerdictSuffix renders a shard's recorded verdict for the result
// heading. An empty outcome means the producer recorded none, which is not a
// pass, so it reads the same as /unverified.
func shardVerdictSuffix(outcome types.MangleAtom) string {
	switch outcome {
	case types.MangleAtom("/done"):
		return " (verified)"
	case types.MangleAtom("/hollow"):
		return " (hollow: no work was performed)"
	case types.MangleAtom("/failed"):
		return " (failed)"
	default:
		return " (unverified)"
	}
}

func normalizeShardType(input string) string {
	return strings.TrimLeft(strings.TrimSpace(input), "/")
}

func spawnSystemShardAndWait(ctx context.Context, manager *coreshards.ShardManager, shardType, task string) (types.ShardResult, error) {
	if manager == nil {
		return types.ShardResult{}, fmt.Errorf("shard manager unavailable for system shard %s", shardType)
	}

	shardID, err := manager.SpawnAsyncWithContext(ctx, shardType, task, nil)
	if err != nil {
		return types.ShardResult{}, err
	}

	// The wait ends when the shard finishes, the command context is
	// cancelled, or --timeout fires. There is no separate wait clock.
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			if ctx.Err() == context.DeadlineExceeded {
				return types.ShardResult{}, fmt.Errorf("system shard %s did not complete before --timeout (id=%s)", shardType, shardID)
			}
			return types.ShardResult{}, fmt.Errorf("system shard %s cancelled (id=%s): %w", shardType, shardID, ctx.Err())
		case <-ticker.C:
			if res, ok := manager.GetResult(shardID); ok {
				if res.Error != nil {
					return res, res.Error
				}
				return res, nil
			}
		}
	}
}
