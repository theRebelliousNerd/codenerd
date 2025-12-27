// Package main implements the codeNERD CLI commands.
// This file contains the runInstruction function implementing the OODA loop.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"codenerd/internal/articulation"
	"codenerd/internal/core"
	coresys "codenerd/internal/system"
	"codenerd/internal/usage"
	"codenerd/internal/world"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

// =============================================================================
// RUN INSTRUCTION - OODA Loop Implementation
// =============================================================================

// runCmd executes a single instruction
var runCmd = &cobra.Command{
	Use:   "run [instruction]",
	Short: "Execute a single instruction through the OODA loop",
	Long: `Processes a natural language instruction through the full Cortex pipeline:
  1. Perception: Transduce input to intent atoms
  2. Orient: Load facts, activate context via spreading activation
  3. Decide: Derive next_action via Mangle policy rules
  4. Act: Execute via VirtualStore, report via Articulation layer`,
	Args: cobra.MinimumNArgs(1),
	RunE: runInstruction,
}

// runInstruction executes a single instruction through the OODA loop
func runInstruction(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Handle graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		logger.Info("Received shutdown signal")
		cancel()
	}()

	userInput := joinArgs(args)
	logger.Info("Processing instruction", zap.String("input", userInput))

	// Resolve API key
	key := apiKey
	if key == "" {
		key = os.Getenv("ZAI_API_KEY")
	}

	// Boot Cortex (System Stabilization)
	cortex, err := coresys.GetOrBootCortex(ctx, workspace, key, disableSystemShards)
	if err != nil {
		return fmt.Errorf("failed to boot cortex: %w", err)
	}
	defer cortex.Close()

	// Add usage tracker to context if available
	if cortex.UsageTracker != nil {
		ctx = usage.NewContext(ctx, cortex.UsageTracker)
	}

	baseRouting, baseExec := systemResultBaselines(cortex.Kernel)

	emitter := articulation.NewEmitter()

	// 2. Perception Layer: Transduce Input -> Intent
	logger.Debug("Transducing user input to intent atoms")
	intent, err := cortex.Transducer.ParseIntent(ctx, userInput)
	if err != nil {
		return fmt.Errorf("perception error: %w", err)
	}
	logger.Info("Intent parsed",
		zap.String("verb", intent.Verb),
		zap.String("target", intent.Target))

	// /stats is deterministic and should not require running shards or policy.
	if intent.Verb == "/stats" {
		stats, err := computeStats(ctx, cortex.Workspace, intent.Target)
		if err != nil {
			stats = fmt.Sprintf("Stats error: %v", err)
		}
		emitter.Emit(articulation.PiggybackEnvelope{
			Surface: stats,
			Control: articulation.ControlPacket{
				IntentClassification: articulation.IntentClassification{
					Category:   intent.Category,
					Verb:       intent.Verb,
					Target:     intent.Target,
					Constraint: intent.Constraint,
					Confidence: intent.Confidence,
				},
				MangleUpdates: []string{fmt.Sprintf("observation(/stats, %q)", stats)},
			},
		})
		return nil
	}

	// 3. World Model: Incremental Scan Workspace (fast)
	logger.Debug("Scanning workspace incrementally", zap.String("path", cortex.Workspace))
	scanRes, err := cortex.Scanner.ScanWorkspaceIncremental(ctx, cortex.Workspace, cortex.LocalDB, world.IncrementalOptions{SkipWhenUnchanged: true})
	if err != nil {
		return fmt.Errorf("world model error: %w", err)
	}
	if scanRes != nil && !scanRes.Unchanged {
		if err := world.ApplyIncrementalResult(cortex.Kernel, scanRes); err != nil {
			return fmt.Errorf("world model apply error: %w", err)
		}
		logger.Debug("Workspace scan applied", zap.Int("facts", len(scanRes.NewFacts)))
	} else {
		logger.Debug("Workspace unchanged, using cached facts")
	}

	// 4. Load Facts into Hollow Kernel
	if err := cortex.Kernel.LoadFacts([]core.Fact{intent.ToFact()}); err != nil {
		return fmt.Errorf("kernel load error: %w", err)
	}

	// Update system facts (Time, etc.)
	if err := cortex.Kernel.UpdateSystemFacts(); err != nil {
		return fmt.Errorf("system facts update error: %w", err)
	}

	// 5. Query Executive Policy (Decide)
	logger.Debug("Querying executive policy")
	var output string

	// Check for delegation
	delegateFacts, _ := cortex.Kernel.Query("delegate_task")
	if len(delegateFacts) > 0 {
		// Execute via shard
		fact := delegateFacts[0]
		shardType := fmt.Sprintf("%v", fact.Args[0])
		task := fmt.Sprintf("%v", fact.Args[1])
		logger.Info("Delegating to shard", zap.String("type", shardType), zap.String("task", task))

		// Special handling for System Components
		if shardType == "/tool_generator" || shardType == "tool_generator" {
			// Autopoiesis: Tool Generation
			count, err := cortex.Orchestrator.ProcessKernelDelegations(ctx)
			if err != nil {
				output = fmt.Sprintf("Tool generation failed: %v", err)
			} else {
				output = fmt.Sprintf("Autopoiesis: Generated %d tools", count)
			}
		} else {
			// Standard Shard - use unified SpawnTask
			result, err := cortex.SpawnTask(ctx, shardType, task)
			if err != nil {
				output = fmt.Sprintf("Shard execution failed: %v", err)
			} else {
				output = fmt.Sprintf("Shard Result: %s", result)
			}
		}

	} else {
		// Query next_action
		actionFacts, _ := cortex.Kernel.Query("next_action")
		if len(actionFacts) > 0 {
			fact := actionFacts[0]
			logger.Info("Derived next_action (unary; executed by system shards if enabled)", zap.Any("action", fact))
			output = fmt.Sprintf("Next action: %v", fact.Args[0])
		} else {
			output = "No action derived from policy"
		}
	}

	routingNew, execNew := waitForSystemResults(ctx, cortex.Kernel, baseRouting, baseExec, 3*time.Second)
	if summary := formatSystemResults(routingNew, execNew); summary != "" {
		output = output + "\n\n" + summary
	}

	// 6. Articulation Layer: Report
	payload := articulation.PiggybackEnvelope{
		Surface: fmt.Sprintf("Processed: %s\nResult: %s", userInput, output),
		Control: articulation.ControlPacket{
			IntentClassification: articulation.IntentClassification{
				Category:   "/mutation", // Default for manual execution
				Verb:       "/execute",
				Target:     "system",
				Confidence: 1.0,
			},
			MangleUpdates: []string{"task_status(/complete)", fmt.Sprintf("observation(/result, %q)", output)},
		},
	}
	emitter.Emit(payload)

	return nil
}
