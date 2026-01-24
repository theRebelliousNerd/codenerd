// Package main implements CLI interactive mode for multi-turn shard interactions.
// This file provides the runInteractiveAction function that enables feedback loops.
package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	coresys "codenerd/internal/system"
	"codenerd/internal/usage"
)

// interactiveMode controls whether CLI commands run in interactive mode.
var interactiveMode bool

// InteractiveMetaCommand represents special commands in interactive mode.
type InteractiveMetaCommand string

const (
	MetaRefine  InteractiveMetaCommand = "refine"
	MetaRedo    InteractiveMetaCommand = "redo"
	MetaApprove InteractiveMetaCommand = "approve"
	MetaQuit    InteractiveMetaCommand = "quit"
	MetaHelp    InteractiveMetaCommand = "help"
)

// runInteractiveAction runs a shard action with interactive feedback loop.
// It keeps the Cortex alive across multiple turns, allowing the user to
// refine, redo, or approve results.
func runInteractiveAction(shardType, verb, initialTarget string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	// Handle graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\n⏹️  Interrupted")
		cancel()
	}()

	// Resolve API key
	key := apiKey
	if key == "" {
		key = os.Getenv("ZAI_API_KEY")
	}

	// Boot Cortex once for entire session
	fmt.Println("🔄 Booting Cortex for interactive session...")
	cortex, err := coresys.GetOrBootCortex(ctx, workspace, key, nil)
	if err != nil {
		return fmt.Errorf("failed to boot cortex: %w", err)
	}
	defer cortex.Close()

	// Add usage tracker
	if cortex.UsageTracker != nil {
		ctx = usage.NewContext(ctx, cortex.UsageTracker)
	}

	fmt.Println(strings.Repeat("─", 60))
	fmt.Println("🎮 Interactive Mode - Commands: refine, redo, approve, quit, help")
	fmt.Println(strings.Repeat("─", 60))

	// Initial task
	currentTask := fmt.Sprintf("%s %s", strings.TrimPrefix(verb, "/"), initialTarget)
	turnCount := 0
	lastResult := ""

	reader := bufio.NewReader(os.Stdin)

	for {
		turnCount++
		fmt.Printf("\n📋 Turn %d | Task: %s\n", turnCount, currentTask)
		fmt.Println(strings.Repeat("─", 60))

		// Spawn shard
		fmt.Printf("⏳ Spawning %s shard...\n", shardType)
		result, err := cortex.SpawnTask(ctx, shardType, currentTask)
		if err != nil {
			fmt.Printf("❌ Shard error: %v\n", err)
			fmt.Print("\n> ")
			input, _ := reader.ReadString('\n')
			input = strings.TrimSpace(input)
			if input == "quit" || input == "q" {
				break
			}
			continue
		}

		lastResult = result
		fmt.Println("\n📋 Result:")
		fmt.Println(strings.Repeat("─", 40))
		fmt.Println(result)
		fmt.Println(strings.Repeat("─", 40))

		// Prompt for next action
		fmt.Println("\n💡 Options: refine <feedback>, redo, approve, quit")
		fmt.Print("> ")

		input, err := reader.ReadString('\n')
		if err != nil {
			break
		}
		input = strings.TrimSpace(input)

		// Parse meta-command
		cmd, arg := parseMetaCommand(input)

		switch cmd {
		case MetaApprove:
			fmt.Println("\n✅ Approved! Final result saved.")
			fmt.Println(strings.Repeat("─", 60))
			fmt.Println(lastResult)
			return nil

		case MetaQuit:
			fmt.Println("\n👋 Exiting interactive mode.")
			return nil

		case MetaRedo:
			fmt.Println("🔄 Redoing with same task...")
			// currentTask stays the same

		case MetaRefine:
			if arg == "" {
				fmt.Println("⚠️  Usage: refine <your feedback>")
				turnCount-- // Don't count this as a turn
				continue
			}
			// Append refinement to task
			currentTask = fmt.Sprintf("%s\n\nRefinement: %s\n\nPrevious result context:\n%s",
				currentTask, arg, truncateForContext(lastResult, 2000))
			fmt.Printf("📝 Refined task with: %s\n", arg)

		case MetaHelp:
			printInteractiveHelp()
			turnCount-- // Don't count this as a turn
			continue

		default:
			// Treat as new task if it looks like one
			if strings.HasPrefix(input, "/") || len(input) > 20 {
				currentTask = fmt.Sprintf("%s %s", strings.TrimPrefix(verb, "/"), input)
				fmt.Printf("📝 New task: %s\n", input)
			} else if input == "" {
				fmt.Println("⚠️  Enter a command or type 'help' for options.")
				turnCount-- // Don't count this as a turn
				continue
			} else {
				// Treat short input as refinement
				currentTask = fmt.Sprintf("%s\n\nRefinement: %s", currentTask, input)
				fmt.Printf("📝 Refined with: %s\n", input)
			}
		}
	}

	return nil
}

// parseMetaCommand extracts the meta-command and optional argument.
func parseMetaCommand(input string) (InteractiveMetaCommand, string) {
	input = strings.TrimSpace(input)
	lower := strings.ToLower(input)

	if lower == "approve" || lower == "a" || lower == "ok" || lower == "done" {
		return MetaApprove, ""
	}
	if lower == "quit" || lower == "q" || lower == "exit" {
		return MetaQuit, ""
	}
	if lower == "redo" || lower == "r" || lower == "retry" {
		return MetaRedo, ""
	}
	if lower == "help" || lower == "h" || lower == "?" {
		return MetaHelp, ""
	}

	// Check for "refine <feedback>" pattern
	if strings.HasPrefix(lower, "refine:") {
		return MetaRefine, strings.TrimSpace(input[7:])
	}
	if strings.HasPrefix(lower, "refine ") {
		return MetaRefine, strings.TrimSpace(input[7:])
	}

	return "", input
}

// printInteractiveHelp prints help for interactive mode.
func printInteractiveHelp() {
	fmt.Println(`
📖 Interactive Mode Help
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Commands:
  refine <feedback>  Add feedback and re-run with context
  redo, r            Re-run the exact same task
  approve, a, ok     Accept the result and exit
  quit, q            Exit without saving
  help, h, ?         Show this help

Tips:
  • Just type a short phrase to refine the result
  • Type a new task description to start fresh
  • Previous results provide context for refinements

Example:
  > refine: make the function names more descriptive
  > also add error handling for nil input
  > approve
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━`)
}

// truncateForContext truncates text for use as context in prompts.
func truncateForContext(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "\n... [truncated]"
}
