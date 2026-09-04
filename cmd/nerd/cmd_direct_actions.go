// Package main implements the codeNERD CLI commands.
// This file contains direct action commands that mirror TUI verbs.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"sort"
	"strings"
	"sync"
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

// securityCmd runs security analysis
var securityCmd = &cobra.Command{
	Use:   "security <target>",
	Short: "Run security analysis on code",
	Long: `Spawns SecurityShard to analyze code for vulnerabilities.
Equivalent to typing "security <target>" in the TUI.

Example:
  nerd security internal/auth/
  nerd security handlers/user.go`,
	Args: cobra.MinimumNArgs(1),
	RunE: runDirectAction("security", "/security"),
}

// analyzeCmd runs general code analysis
var analyzeCmd = &cobra.Command{
	Use:   "analyze <target>",
	Short: "Run general analysis on code",
	Long: `Spawns ResearcherShard to analyze code structure and patterns.
Equivalent to typing "analyze <target>" in the TUI.

Example:
  nerd analyze internal/core/
  nerd analyze "the authentication flow"`,
	Args: cobra.MinimumNArgs(1),
	RunE: runDirectAction("researcher", "/analyze"),
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

// heartbeatInterval is the period between liveness messages while a shard
// is executing. Extracted as a constant so tests can inject a short interval
// without waiting on real 30-second ticks.
const heartbeatInterval = 30 * time.Second

// startHeartbeat emits a periodic liveness line to out while the shard is
// executing. The returned stop function closes the heartbeat goroutine and
// blocks until it has exited, guaranteeing no further writes after it returns
// so the result block cannot be interleaved.
//
// out should be the same stream the surrounding code uses (os.Stdout for the
// direct-action commands) so OS-level redirection keeps behaving. interval is
// the ticker period - production passes heartbeatInterval, tests inject a
// short interval to avoid real sleeps.
func startHeartbeat(out io.Writer, interval time.Duration) func() {
	done := make(chan struct{})
	finished := make(chan struct{})
	start := time.Now()
	ticker := time.NewTicker(interval)
	var stopOnce sync.Once
	go func() {
		defer close(finished)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				elapsed := time.Since(start).Round(time.Second)
				fmt.Fprintf(out, "   … still working (%s elapsed)\n", elapsed)
			}
		}
	}()
	return func() {
		stopOnce.Do(func() {
			close(done)
			<-finished
		})
	}
}

// runDirectAction creates a handler for direct action commands
func runDirectAction(shardType, verb string) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		target := strings.Join(args, " ")

		// Interactive mode: use multi-turn feedback loop
		if interactiveMode {
			return runInteractiveAction(shardType, verb, target)
		}

		// Initialize verbose tracer if --verbose flag is set
		PrintVerboseHeader()
		tracer := NewDebugTracer()
		defer tracer.Summary()

		tracer.TracePhase("INITIALIZATION")
		tracer.Trace("CONFIG", "timeout=%v, workspace=%s", timeout, workspace)
		tracer.Trace("CONFIG", "shard=%s, verb=%s", shardType, verb)

		// One-shot mode (original behavior)
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		tracer.TraceContext("created with timeout %v", timeout)

		// Handle graceful shutdown
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		go func() {
			<-sigCh
			fmt.Println("\n⏹️  Interrupted")
			tracer.Trace("SIGNAL", "received interrupt signal")
			cancel()
		}()

		fmt.Printf("🔧 Action: %s\n", verb)
		fmt.Printf("🎯 Target: %s\n", target)
		fmt.Printf("🤖 Shard:  %s\n", shardType)
		fmt.Println(strings.Repeat("─", 50))

		// Resolve API key
		key := resolveAPIKey(apiKey, workspace)
		tracer.Trace("CONFIG", "API key source: %s", func() string {
			if apiKey != "" {
				return "flag"
			}
			return "env"
		}())

		// Boot Cortex
		tracer.TracePhase("CORTEX BOOT")
		bootStart := time.Now()
		cortex, err := coresys.GetOrBootCortex(ctx, workspace, key, nil)
		if err != nil {
			tracer.TraceError("cortex boot failed: %v", err)
			return fmt.Errorf("failed to boot cortex: %w", err)
		}
		defer cortex.Close()
		tracer.Trace("CORTEX", "booted in %v", time.Since(bootStart).Round(time.Millisecond))

		// Add usage tracker
		if cortex.UsageTracker != nil {
			ctx = usage.NewContext(ctx, cortex.UsageTracker)
			tracer.Trace("CORTEX", "usage tracker attached")
		}

		// Spawn via unified SpawnTask. Pass the intent verb (e.g. /create),
		// not the persona shard name (coder). Mapping "coder"→/fix remapped
		// every create/refactor one-shot onto the wrong intent and made hollow
		// prose completions look successful.
		tracer.TracePhase("SHARD EXECUTION")
		tracer.TraceShard("spawning %s (intent %s) with task: %s", shardType, verb, target)
		fmt.Printf("⏳ Spawning %s shard...\n", shardType)
		stopHeartbeat := startHeartbeat(os.Stdout, heartbeatInterval)
		defer stopHeartbeat()

		// Snapshot the workspace root so undeclared writes become visible.
		//
		// Direct CLI verbs leave undeclared files in the repository root and
		// never mention it. Two instances measured live 2026-08-08: 'nerd test
		// internal/session/gate_names.go' left gate_cover.out in the root, and
		// 'nerd define-agent --name GofmtExpert --topic ...' created a whole
		// research/ directory containing a .mg file there. In both cases the
		// command exited 0 and said nothing about the file, so the user finds
		// out from git status later. The path is chosen by the model at write
		// time, not by a constant in the code, so this cannot be fixed by
		// correcting a hardcoded path - it needs a guard around execution.
		//
		// Campaigns already solve the same problem: internal/campaign/
		// orchestrator_tasks.go has recordRootBaseline and
		// sweepUndeclaredRootWrites, which snapshot the workspace root before
		// the run and afterwards handle anything new that no task declared.
		//
		// Direct verbs have no write set to check against, only a target
		// string, so we report only - moving a user's intended output would be
		// worse than leaving it. The campaign helper snapshotWorkspaceRoot
		// records only files (skips IsDir), so it would have missed the
		// research/ directory entirely; this version records directories as
		// well as files. Exclude .nerd since codeNERD writes there constantly.
		wsRoot := strings.TrimSpace(workspace)
		if wsRoot == "" {
			if cwd, err := os.Getwd(); err == nil {
				wsRoot = cwd
			}
		}
		rootBefore := snapshotDirectRoot(wsRoot)

		shardStart := time.Now()
		result, err := cortex.SpawnTaskWithTarget(ctx, verb, target, target)
		stopHeartbeat()
		rootAfter := snapshotDirectRoot(wsRoot)
		shardDuration := time.Since(shardStart)

		if err != nil {
			tracer.TraceError("shard failed after %v: %v", shardDuration.Round(time.Millisecond), err)
			// Print partial result if any (diagnostics) before non-zero exit.
			if strings.TrimSpace(result) != "" {
				fmt.Println(strings.Repeat("─", 50))
				fmt.Println("📋 Partial result (failed):")
				fmt.Println(result)
			}
			reportUndeclaredRootWrites(findNewRootEntries(rootBefore, rootAfter))
			return fmt.Errorf("shard execution failed: %w", err)
		}
		tracer.TraceShard("completed in %v, result length: %d chars", shardDuration.Round(time.Millisecond), len(result))

		// Defense in depth: an empty result is never a success, for ANY verb.
		//
		// This used to be scoped to isWriteOrientedDirectVerb, which let query
		// verbs exit 0 with nothing to show. Live: `nerd review <file>` ran 16
		// successful tool calls over 2m42s, hit the tool-iteration ceiling
		// before the model wrote its conclusion, and printed "📋 Result:"
		// followed by a blank line — exit code 0. The executor now forces a
		// final tool-free answer in that case (see forceFinalAnswer), so this
		// is the backstop for whatever produces the next empty string.
		// Shared with the interactive per-turn loop via checkDirectSpawnResult.
		newEntries, hollowErr := checkDirectSpawnResult(verb, result, rootBefore, rootAfter)
		if hollowErr != nil {
			reportUndeclaredRootWrites(newEntries)
			return hollowErr
		}

		fmt.Println(strings.Repeat("─", 50))
		fmt.Println("📋 Result:")
		fmt.Println(result)
		reportUndeclaredRootWrites(newEntries)

		tracer.TracePhase("COMPLETE")
		return nil
	}
}

// snapshotDirectRoot lists all entries (files and directories) directly in the
// workspace root, excluding the .nerd directory. Directories are included
// deliberately - the campaign helper snapshotWorkspaceRoot skips IsDir and would
// have missed the research/ directory observed live on 2026-08-08.
func snapshotDirectRoot(root string) map[string]bool {
	if strings.TrimSpace(root) == "" {
		return nil
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	seen := make(map[string]bool, len(entries))
	for _, e := range entries {
		if e.Name() == ".nerd" {
			continue
		}
		seen[e.Name()] = true
	}
	return seen
}

// findNewRootEntries returns the sorted list of entries present in after but
// absent from before. Either nil map yields nil (cannot compare, stay silent).
func findNewRootEntries(before, after map[string]bool) []string {
	if before == nil || after == nil {
		return nil
	}
	var added []string
	for name := range after {
		if !before[name] {
			added = append(added, name)
		}
	}
	if len(added) == 0 {
		return nil
	}
	sort.Strings(added)
	return added
}

// checkDirectSpawnResult applies the shared post-spawn guards for direct CLI
// verbs: the hollow-success guard (an empty result is never a success) and the
// undeclared-root-write sweep. It returns the sorted list of new root entries
// (nil when none) plus an error when the result is hollow. Both the one-shot
// path above and the interactive per-turn loop call this so the guards cannot
// drift apart again.
func checkDirectSpawnResult(verb, result string, rootBefore, rootAfter map[string]bool) ([]string, error) {
	newEntries := findNewRootEntries(rootBefore, rootAfter)
	if strings.TrimSpace(result) == "" {
		return newEntries, fmt.Errorf("hollow success blocked: %s completed with an empty result", verb)
	}
	return newEntries, nil
}

// reportUndeclaredRootWrites prints the shared undeclared-write warning for
// entries the sweep found. Extracted so one-shot and interactive turns render
// the verdict identically.
func reportUndeclaredRootWrites(newEntries []string) {
	if len(newEntries) > 0 {
		fmt.Printf("⚠️  Created in the repository root, undeclared: %s\n", strings.Join(newEntries, ", "))
	}
}

// isWriteOrientedDirectVerb used to live here, gating the hollow-success check
// to write verbs only. The check above now applies to every verb, which is a
// strictly broader condition, so the verb list had no remaining caller. Removed
// rather than left dormant — this is superseded logic, not a wiring gap.

// runPerceptionTest tests the perception transducer
func runPerceptionTest(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	input := strings.Join(args, " ")

	fmt.Printf("🎤 Input: %q\n", input)
	fmt.Println(strings.Repeat("─", 50))

	// Resolve API key
	key := resolveAPIKey(apiKey, workspace)

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
