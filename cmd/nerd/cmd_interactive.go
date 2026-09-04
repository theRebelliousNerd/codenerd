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
	// A multi-turn session has no natural total length: the process context
	// lives until SIGINT/SIGTERM, and each shard turn gets its own timeout
	// from the global --timeout flag (package-level timeout). A single review
	// turn can take 10 minutes, so a hidden 30m total ceiling breaks every
	// later SpawnTask with "context deadline exceeded".
	processCtx, processCancel := context.WithCancel(context.Background())
	defer processCancel()

	// Handle graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\n⏹️  Interrupted")
		processCancel()
	}()

	// Resolve API key
	key := resolveAPIKey(apiKey, workspace)

	// Boot Cortex once for entire session
	fmt.Println("🔄 Booting Cortex for interactive session...")
	cortex, err := coresys.GetOrBootCortex(processCtx, workspace, key, nil)
	if err != nil {
		return fmt.Errorf("failed to boot cortex: %w", err)
	}
	defer cortex.Close()

	// Captured once; applied to each per-turn context below so usage tracking
	// survives the move from one long-lived ctx to per-turn timeouts.
	usageTracker := cortex.UsageTracker

	fmt.Println(strings.Repeat("─", 60))
	fmt.Println("🎮 Interactive Mode - Commands: refine, redo, approve, quit, help")
	fmt.Println(strings.Repeat("─", 60))

	// Initial task
	currentTask := fmt.Sprintf("%s %s", strings.TrimPrefix(verb, "/"), initialTarget)
	turnCount := 0
	lastResult := ""

	reader := bufio.NewReader(os.Stdin)

	wsRoot := strings.TrimSpace(workspace)
	if wsRoot == "" {
		if cwd, err := os.Getwd(); err == nil {
			wsRoot = cwd
		}
	}

	for {
		turnCount++
		fmt.Printf("\n📋 Turn %d | Task: %s\n", turnCount, currentTask)
		fmt.Println(strings.Repeat("─", 60))

		// Each turn gets its own timeout from --timeout. Explicit cancel:
		// defer in a loop would leak until the session ends.
		turnCtx, turnCancel := context.WithTimeout(processCtx, timeout)
		if usageTracker != nil {
			turnCtx = usage.NewContext(turnCtx, usageTracker)
		}
		// Spawn shard via the same call as the one-shot path: the intent verb
		// (e.g. /create), not the persona shard name (coder), with the
		// original target and the evolving task.
		fmt.Printf("⏳ Spawning %s shard...\n", shardType)
		rootBefore := snapshotDirectRoot(wsRoot)
		result, err := cortex.SpawnTaskWithTarget(turnCtx, verb, currentTask, initialTarget)
		turnCancel()
		rootAfter := snapshotDirectRoot(wsRoot)
		if err != nil {
			fmt.Printf("❌ Shard error: %v\n", err)
			reportUndeclaredRootWrites(findNewRootEntries(rootBefore, rootAfter))
			fmt.Print("\n> ")
			_, _, shouldExit := nextInteractiveCommand(reader, true)
			if shouldExit {
				break
			}
			continue
		}

		// Same guards as the one-shot path: hollow-success plus the
		// undeclared-root-write sweep, via the shared helper.
		newEntries, hollowErr := checkDirectSpawnResult(verb, result, rootBefore, rootAfter)
		if hollowErr != nil {
			fmt.Printf("❌ %v\n", hollowErr)
			reportUndeclaredRootWrites(newEntries)
			fmt.Print("\n> ")
			_, _, shouldExit := nextInteractiveCommand(reader, true)
			if shouldExit {
				break
			}
			continue
		}

		lastResult = result
		fmt.Println("\n📋 Result:")
		fmt.Println(strings.Repeat("─", 40))
		fmt.Println(result)
		reportUndeclaredRootWrites(newEntries)
		fmt.Println(strings.Repeat("─", 40))

		// Prompt for next action
		fmt.Println("\n💡 Options: refine <feedback>, redo, approve, quit")
		fmt.Print("> ")

		cmd, arg, shouldExit := nextInteractiveCommand(reader, false)
		if shouldExit && cmd != MetaQuit {
			break
		}

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
			// nextInteractiveCommand returns ("", rawInput) for non-meta
			// input, so arg carries the trimmed raw line here.
			input := arg
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

// nextInteractiveCommand reads one stdin line and decides whether the
// interactive loop should exit. It returns the parsed meta-command, its arg
// (feedback for refine; trimmed raw input for the default case), and
// shouldExit. A read error (including io.EOF) always means exit — matching the
// success branch's `if err != nil { break }` — so exhausted stdin cannot spin
// the shard-error branch forever. Quit variants (quit/q/exit) are routed
// through parseMetaCommand so they are honored consistently on both the error
// and success paths. lastTurnErrored identifies the call site for tests; exit
// semantics are identical on both paths.
func nextInteractiveCommand(reader *bufio.Reader, lastTurnErrored bool) (InteractiveMetaCommand, string, bool) {
	_ = lastTurnErrored
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", strings.TrimSpace(line), true
	}
	cmd, arg := parseMetaCommand(line)
	if cmd == MetaQuit {
		return cmd, arg, true
	}
	return cmd, arg, false
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
