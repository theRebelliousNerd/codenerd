// Package main implements the codeNERD CLI commands.
// This file contains the runInstruction function implementing the OODA loop.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"codenerd/internal/articulation"
	"codenerd/internal/core"
	coresys "codenerd/internal/system"
	"codenerd/internal/types"
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

	// 5. Query Executive Policy (Decide) and actually execute.
	// Previously next_action(/delegate_coder) was printed and the command
	// exited 0 without ever spawning work — classic hollow success.
	logger.Debug("Querying executive policy")
	var output string
	var actionErr error
	actionExecuted := false

	// One-shot CLI is explicit user interaction — lift the boot guard so
	// VirtualStore RouteAction can run for non-delegate next_actions.
	if cortex.VirtualStore != nil {
		cortex.VirtualStore.DisableBootGuard()
	}

	// Check for explicit delegate_task facts first
	delegateFacts, _ := cortex.Kernel.Query("delegate_task")
	if len(delegateFacts) > 0 {
		fact := delegateFacts[0]
		shardType := types.ExtractString(fact.Args[0])
		task := types.ExtractString(fact.Args[1])
		if strings.TrimSpace(task) == "" {
			task = userInput
		}
		logger.Info("Delegating to shard", zap.String("type", shardType), zap.String("task", task))

		if shardType == "/tool_generator" || shardType == "tool_generator" {
			if cortex.Orchestrator == nil {
				actionErr = fmt.Errorf("tool_generator delegation requires autopoiesis orchestrator")
				output = actionErr.Error()
			} else {
				count, err := cortex.Orchestrator.ProcessKernelDelegations(ctx)
				if err != nil {
					actionErr = err
					output = fmt.Sprintf("Tool generation failed: %v", err)
				} else if count == 0 {
					// Fall back to a direct ouroboros generation from the task text
					need, detErr := cortex.Orchestrator.DetectToolNeed(ctx, task)
					if detErr != nil {
						actionErr = detErr
						output = fmt.Sprintf("Tool generation failed: %v", detErr)
					} else {
						if need == nil {
							need = buildCLIToolNeed(task)
						}
						loopRes := cortex.Orchestrator.ExecuteOuroborosLoop(ctx, need)
						if loopRes == nil || !loopRes.Success {
							errMsg := "unknown"
							if loopRes != nil && loopRes.Error != "" {
								errMsg = loopRes.Error
							}
							actionErr = fmt.Errorf("tool generation failed: %s", errMsg)
							output = actionErr.Error()
						} else {
							actionExecuted = true
							output = fmt.Sprintf("Autopoiesis: Generated tool %s", loopRes.ToolName)
						}
					}
				} else {
					actionExecuted = true
					output = fmt.Sprintf("Autopoiesis: Generated %d tools", count)
				}
			}
		} else {
			result, err := cortex.SpawnTask(ctx, shardType, task)
			if err != nil {
				actionErr = err
				output = fmt.Sprintf("Shard execution failed: %v", err)
				if strings.TrimSpace(result) != "" {
					output += "\n" + result
				}
			} else {
				actionExecuted = true
				output = fmt.Sprintf("Shard Result: %s", result)
			}
		}
	} else {
		// No delegate_task — try next_action and execute handoffs.
		actionFacts, _ := cortex.Kernel.Query("next_action")
		if len(actionFacts) == 0 {
			actionErr = fmt.Errorf("no action derived from policy")
			output = actionErr.Error()
		} else {
			action := types.ExtractString(actionFacts[0].Args[0])
			logger.Info("Derived next_action", zap.String("action", action))

			if shard := nextActionToShardType(action); shard != "" {
				// Execute the handoff that policy derived but did not surface
				// as delegate_task (e.g. /create → next_action(/delegate_coder)).
				logger.Info("Executing next_action handoff", zap.String("shard", shard))
				result, err := cortex.SpawnTask(ctx, shard, userInput)
				if err != nil {
					actionErr = err
					output = fmt.Sprintf("Handoff %s failed: %v", action, err)
					if strings.TrimSpace(result) != "" {
						output += "\n" + result
					}
				} else {
					actionExecuted = true
					output = fmt.Sprintf("Executed %s via %s:\n%s", action, shard, result)
				}
			} else if cortex.VirtualStore != nil {
				// Non-delegate actions: route through VirtualStore.
				fact := nextActionFact(action, userInput)
				// File the constitutional permission request first: the kernel
				// only derives permitted/3 from a matching pending_action/5
				// (constitution.mg default-deny), and the one-shot path has no
				// executive shard to file it. The kernel still decides —
				// safe_action and !dangerous_content gates apply unchanged.
				if err := assertPendingAction(cortex.Kernel, fact); err != nil {
					logger.Warn("Failed to file pending_action; RouteAction will be denied", zap.Error(err))
				}
				vsResult, vsErr := cortex.VirtualStore.RouteAction(ctx, fact)
				if vsErr != nil {
					actionErr = vsErr
					output = fmt.Sprintf("RouteAction(%s) failed: %v", action, vsErr)
				} else {
					actionExecuted = true
					output = fmt.Sprintf("Executed %s: %v", action, vsResult)
				}
			} else {
				actionErr = fmt.Errorf("derived next_action %s but no executor handled it (no delegate handoff or virtual store)", action)
				output = actionErr.Error()
			}
		}
	}

	routingNew, execNew := waitForSystemResults(ctx, cortex.Kernel, baseRouting, baseExec, 3*time.Second)
	if summary := formatSystemResults(routingNew, execNew); summary != "" {
		output = output + "\n\n" + summary
	}

	// 6. Articulation Layer: Report
	status := "/complete"
	if actionErr != nil || !actionExecuted {
		status = "/failed"
	}
	payload := articulation.PiggybackEnvelope{
		Surface: fmt.Sprintf("Processed: %s\nResult: %s", userInput, output),
		Control: articulation.ControlPacket{
			IntentClassification: articulation.IntentClassification{
				Category:   intent.Category,
				Verb:       intent.Verb,
				Target:     intent.Target,
				Constraint: intent.Constraint,
				Confidence: intent.Confidence,
			},
			MangleUpdates: []string{
				fmt.Sprintf("task_status(/manual_instruction, %s)", status),
				fmt.Sprintf("observation(/result, %q)", output),
			},
		},
	}
	emitter.Emit(payload)

	if actionErr != nil {
		return actionErr
	}
	if !actionExecuted {
		return fmt.Errorf("hollow success blocked: no side-effecting action executed for %q", userInput)
	}
	return nil
}

// nextActionFact builds the VirtualStore-routable fact for a policy-derived
// non-delegate next_action. VirtualStore.parseActionFact requires exactly
// (ActionID, Type, Target); the previous 2-arg {action, input} shape failed
// parsing for EVERY non-delegate verb (F-ROUTE-1: "invalid action fact:
// requires at least 3 arguments"), so /explain → /analyze_code never executed.
// Mirrors the tdd_loop.go action-fact pattern.
func nextActionFact(action, target string) core.Fact {
	return core.Fact{
		Predicate: "next_action",
		Args: []any{
			fmt.Sprintf("cli-%d", time.Now().UnixNano()),
			types.MangleAtom(action),
			target,
		},
	}
}

// assertPendingAction files the constitutional permission request for a
// CLI-routed next_action fact. constitution.mg derives permitted/3 only from
// safe_action + a matching pending_action/5 whose Target and Payload exactly
// equal what VirtualStore.CheckKernelPermitted recomputes from the parsed
// request — "{}" is the canonical JSON of the empty payload a bare 3-arg fact
// parses to, so this shape must stay in sync with nextActionFact.
func assertPendingAction(kernel core.Kernel, fact core.Fact) error {
	if kernel == nil {
		return fmt.Errorf("cannot file pending_action: no kernel")
	}
	if len(fact.Args) != 3 {
		return fmt.Errorf("cannot file pending_action: want 3-arg next_action fact, got %d args", len(fact.Args))
	}
	return kernel.Assert(core.Fact{
		Predicate: "pending_action",
		Args:      []any{fact.Args[0], fact.Args[1], fact.Args[2], "{}", time.Now().Unix()},
	})
}

// nextActionToShardType maps policy next_action atoms onto domain shard types
// that SpawnTask can run. Empty means "not a shard handoff".
func nextActionToShardType(action string) string {
	action = strings.TrimSpace(strings.TrimPrefix(action, "/"))
	switch strings.ToLower(action) {
	case "delegate_coder", "delegate_coder_shard":
		return "coder"
	case "delegate_tester":
		return "tester"
	case "delegate_reviewer":
		return "reviewer"
	case "delegate_researcher":
		return "researcher"
	case "delegate_tool_generator":
		return "tool_generator"
	default:
		return ""
	}
}
