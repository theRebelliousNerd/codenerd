// Package main implements the codeNERD CLI commands.
// This file contains direct action commands that mirror TUI verbs.
package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"codenerd/internal/perception"
	coresys "codenerd/internal/system"
	"codenerd/internal/usage"

	"github.com/spf13/cobra"
)

// =============================================================================
// DIRECT ACTION COMMANDS - Mirror TUI verbs for CLI testing
// =============================================================================

// reviewCmd runs code review directly
var reviewCmd = &cobra.Command{
	Use:   "review <target>",
	Short: "Run code review on a file or directory",
	Long: `Spawns ReviewerShard to analyze code for issues.
Equivalent to typing "review <target>" in the TUI.

Example:
  nerd review internal/core/kernel.go
  nerd review ./internal/shards/`,
	Args: cobra.MinimumNArgs(1),
	RunE: runDirectAction("reviewer", "/review"),
}

// fixCmd runs code fix directly
var fixCmd = &cobra.Command{
	Use:   "fix <target>",
	Short: "Fix bugs or issues in code",
	Long: `Spawns CoderShard to fix bugs in the specified target.
Equivalent to typing "fix <target>" in the TUI.

Example:
  nerd fix "the null pointer in auth.go"
  nerd fix internal/core/kernel.go`,
	Args: cobra.MinimumNArgs(1),
	RunE: runDirectAction("coder", "/fix"),
}

// testCmd runs tests directly
var testCmd = &cobra.Command{
	Use:   "test <target>",
	Short: "Run or generate tests",
	Long: `Spawns TesterShard to run or generate tests.
Equivalent to typing "test <target>" in the TUI.

Example:
  nerd test ./internal/core/...
  nerd test "add tests for kernel.go"`,
	Args: cobra.MinimumNArgs(1),
	RunE: runDirectAction("tester", "/test"),
}

// pushCmd runs git push directly
var pushCmd = &cobra.Command{
	Use:   "push [remote] [branch]",
	Short: "Push commits to remote repository",
	Long: `Executes git push to push commits to the remote repository.

Example:
  nerd push              # pushes to origin
  nerd push origin main  # pushes main to origin`,
	RunE: func(cmd *cobra.Command, args []string) error {
		gitArgs := []string{"push"}
		if len(args) > 0 {
			gitArgs = append(gitArgs, args...)
		}

		fmt.Printf("🚀 Executing: git %s\n", strings.Join(gitArgs, " "))
		fmt.Println(strings.Repeat("─", 50))

		gitCmd := exec.Command("git", gitArgs...)
		gitCmd.Dir = workspace
		gitCmd.Stdout = os.Stdout
		gitCmd.Stderr = os.Stderr
		return gitCmd.Run()
	},
}

// commitCmd runs git commit directly
var commitCmd = &cobra.Command{
	Use:   "commit <message>",
	Short: "Commit changes with a message",
	Long: `Executes git commit with the provided message.

Example:
  nerd commit "fix: resolve auth bug"`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		message := strings.Join(args, " ")

		fmt.Printf("📝 Executing: git commit -m %q\n", message)
		fmt.Println(strings.Repeat("─", 50))

		// First check status
		statusCmd := exec.Command("git", "status", "--porcelain")
		statusCmd.Dir = workspace
		status, _ := statusCmd.Output()

		if len(status) == 0 {
			fmt.Println("ℹ️  Nothing to commit, working tree clean")
			return nil
		}

		// Add all changes
		addCmd := exec.Command("git", "add", "-A")
		addCmd.Dir = workspace
		if err := addCmd.Run(); err != nil {
			return fmt.Errorf("git add failed: %w", err)
		}

		// Commit
		gitCmd := exec.Command("git", "commit", "-m", message)
		gitCmd.Dir = workspace
		gitCmd.Stdout = os.Stdout
		gitCmd.Stderr = os.Stderr
		return gitCmd.Run()
	},
}

// explainCmd explains code directly
var explainCmd = &cobra.Command{
	Use:   "explain <target>",
	Short: "Explain what code does",
	Long: `Analyzes and explains the specified code.
Equivalent to typing "explain <target>" in the TUI.

Example:
  nerd explain internal/core/kernel.go
  nerd explain "the OODA loop"`,
	Args: cobra.MinimumNArgs(1),
	RunE: runDirectAction("researcher", "/explain"),
}

// createCmd creates new code directly
var createCmd = &cobra.Command{
	Use:   "create <description>",
	Short: "Create new code or files",
	Long: `Spawns CoderShard to create new code.
Equivalent to typing "create <description>" in the TUI.

Example:
  nerd create "a retry wrapper for HTTP calls"
  nerd create internal/utils/retry.go`,
	Args: cobra.MinimumNArgs(1),
	RunE: runDirectAction("coder", "/create"),
}

// refactorCmd refactors code directly
var refactorCmd = &cobra.Command{
	Use:   "refactor <target>",
	Short: "Refactor existing code",
	Long: `Spawns CoderShard to refactor code.
Equivalent to typing "refactor <target>" in the TUI.

Example:
  nerd refactor internal/core/kernel.go
  nerd refactor "extract helper functions from process.go"`,
	Args: cobra.MinimumNArgs(1),
	RunE: runDirectAction("coder", "/refactor"),
}

// perceptionCmd tests perception/intent recognition
var perceptionCmd = &cobra.Command{
	Use:   "perception <input>",
	Short: "Test perception transducer (diagnostic)",
	Long: `Tests how the perception layer interprets user input.
Shows parsed intent, verb, target, and shard routing.

Example:
  nerd perception "review my code"
  nerd perception "push to github"`,
	Args: cobra.MinimumNArgs(1),
	RunE: runPerceptionTest,
}

// runDirectAction creates a handler for direct action commands
func runDirectAction(shardType, verb string) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		// Handle graceful shutdown
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		go func() {
			<-sigCh
			fmt.Println("\n⏹️  Interrupted")
			cancel()
		}()

		target := strings.Join(args, " ")
		task := fmt.Sprintf("%s %s", strings.TrimPrefix(verb, "/"), target)

		fmt.Printf("🔧 Action: %s\n", verb)
		fmt.Printf("🎯 Target: %s\n", target)
		fmt.Printf("🤖 Shard:  %s\n", shardType)
		fmt.Println(strings.Repeat("─", 50))

		// Resolve API key
		key := apiKey
		if key == "" {
			key = os.Getenv("ZAI_API_KEY")
		}

		// Boot Cortex
		cortex, err := coresys.GetOrBootCortex(ctx, workspace, key, nil)
		if err != nil {
			return fmt.Errorf("failed to boot cortex: %w", err)
		}
		defer cortex.Close()

		// Add usage tracker
		if cortex.UsageTracker != nil {
			ctx = usage.NewContext(ctx, cortex.UsageTracker)
		}

		// Spawn shard directly - use unified SpawnTask
		fmt.Printf("⏳ Spawning %s shard...\n", shardType)
		result, err := cortex.SpawnTask(ctx, shardType, task)
		if err != nil {
			return fmt.Errorf("shard execution failed: %w", err)
		}

		fmt.Println(strings.Repeat("─", 50))
		fmt.Println("📋 Result:")
		fmt.Println(result)

		return nil
	}
}

// runPerceptionTest tests the perception transducer
func runPerceptionTest(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	input := strings.Join(args, " ")

	fmt.Printf("🎤 Input: %q\n", input)
	fmt.Println(strings.Repeat("─", 50))

	// Resolve API key
	key := apiKey
	if key == "" {
		key = os.Getenv("ZAI_API_KEY")
	}

	// Boot Cortex (lightweight - just need transducer)
	cortex, err := coresys.GetOrBootCortex(ctx, workspace, key, nil)
	if err != nil {
		return fmt.Errorf("failed to boot cortex: %w", err)
	}
	defer cortex.Close()

	// Parse intent
	intent, err := cortex.Transducer.ParseIntent(ctx, input)
	if err != nil {
		return fmt.Errorf("perception error: %w", err)
	}

	// Get shard routing
	shardType := perception.GetShardTypeForVerb(intent.Verb)

	fmt.Printf("📊 Perception Results:\n")
	fmt.Printf("   Category:   %s\n", intent.Category)
	fmt.Printf("   Verb:       %s\n", intent.Verb)
	fmt.Printf("   Target:     %s\n", intent.Target)
	fmt.Printf("   Constraint: %s\n", intent.Constraint)
	fmt.Printf("   Confidence: %.2f\n", intent.Confidence)
	fmt.Println(strings.Repeat("─", 50))
	fmt.Printf("🔀 Routing:\n")
	if shardType == "" || shardType == "/none" {
		fmt.Printf("   Shard: (none - direct response)\n")
	} else {
		fmt.Printf("   Shard: %s\n", shardType)
	}
	fmt.Println(strings.Repeat("─", 50))
	fmt.Printf("💬 Response Preview:\n%s\n", truncateResponse(intent.Response, 500))

	return nil
}

// truncateResponse truncates long responses for display
func truncateResponse(s string, maxLen int) string {
	if len(s) > maxLen {
		return s[:maxLen] + "\n... (truncated)"
	}
	return s
}
