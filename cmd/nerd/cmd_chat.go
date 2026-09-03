package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"codenerd/internal/config"
	coresys "codenerd/internal/system"
	"codenerd/internal/usage"

	"github.com/spf13/cobra"
)

// chatCmd runs a headless multi-turn chat through the main agent.
var chatCmd = &cobra.Command{
	Use:   "chat [turn...]",
	Short: "Run a headless multi-turn chat through the main agent",
	Long: `Runs a headless multi-turn chat through the main session executor.

Each positional argument is one turn, in order. When no arguments are
given, turns are read from stdin one per line. Blank lines are skipped;
a line that is exactly /quit or /exit ends the session.

Each turn goes through the same main-agent path as the TUI session
executor (perception, JIT prompt compilation, gated tool loop,
articulation, and turn persistence) without requiring a TTY.`,
	RunE: runChat,
}

// collectChatTurns resolves the ordered list of turns for a headless chat
// session. Positional args take precedence over stdin: when args is
// non-empty stdin is never read. Otherwise turns are read from r one per
// line, trimmed, with blank lines skipped, stopping at EOF or a line that
// is exactly /quit or /exit.
func collectChatTurns(args []string, r io.Reader) []string {
	if len(args) > 0 {
		turns := make([]string, len(args))
		copy(turns, args)
		return turns
	}
	turns := []string{}
	if r == nil {
		return turns
	}
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if line == "/quit" || line == "/exit" {
			break
		}
		turns = append(turns, line)
	}
	return turns
}

// runChat boots the Cortex once and feeds successive turns to
// cortex.SessionExecutor.Process, mirroring the TUI main-agent path.
func runChat(cmd *cobra.Command, args []string) error {
	turns := collectChatTurns(args, os.Stdin)
	if len(turns) == 0 {
		return nil
	}

	// A conversation has no natural total length, so the session itself is
	// not wrapped in the global --timeout. Each turn gets its own deadline
	// below. Cancellation comes from SIGINT/SIGTERM, same as runInstruction.
	processCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()
	defer signal.Stop(sigCh)

	key := resolveAPIKey(apiKey, workspace)

	cortex, err := coresys.GetOrBootCortex(processCtx, workspace, key, disableSystemShards)
	if err != nil {
		return fmt.Errorf("failed to boot cortex: %w", err)
	}
	defer cortex.Close()

	if cortex.UsageTracker != nil {
		processCtx = usage.NewContext(processCtx, cortex.UsageTracker)
	}

	if cortex.VirtualStore != nil {
		cortex.VirtualStore.DisableBootGuard()
	}

	if cortex.SessionExecutor == nil {
		return fmt.Errorf("chat: session executor is not available (cortex boot did not provide one)")
	}
	cortex.SessionExecutor.SetSessionID(fmt.Sprintf("session-%d", time.Now().UnixNano()))

	for i, turn := range turns {
		fmt.Printf("── turn %d ──\n%s\n", i+1, turn)
		turnCtx, turnCancel := context.WithTimeout(processCtx, config.GetLLMTimeouts().OODALoopTimeout)
		stopHeartbeat := startHeartbeat(os.Stdout, heartbeatInterval)
		start := time.Now()
		result, procErr := cortex.SessionExecutor.Process(turnCtx, turn)
		elapsed := time.Since(start)
		stopHeartbeat()
		turnCancel()
		if procErr != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", procErr)
			continue
		}
		if result == nil {
			fmt.Fprintf(os.Stderr, "error: nil result for turn %d\n", i+1)
			continue
		}
		fmt.Println(result.Response)
		fmt.Printf("[tools executed=%d ok=%d writes=%d elapsed=%s]\n",
			result.ToolCallsExecuted, result.SuccessfulToolCalls, result.SuccessfulWriteTools, elapsed)
	}
	return nil
}
