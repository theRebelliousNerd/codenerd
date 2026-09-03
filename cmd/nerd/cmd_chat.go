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

// chatTurnSource yields headless chat turns lazily, one at a time.
// Positional args take precedence over stdin: when args is non-empty Next
// yields the args in order and never touches the reader. Otherwise turns are
// read from r one line at a time, trimmed, with blank lines skipped,
// stopping at EOF or a line that is exactly /quit or /exit.
type chatTurnSource struct {
	args        []string
	scanner     *bufio.Scanner
	done        bool
	errReported bool
}

// newChatTurnSource builds a chatTurnSource. When args is non-empty the
// reader is never touched; otherwise r is wrapped in a bufio.Scanner with a
// 1 MiB max line. A nil reader with no args yields nothing.
func newChatTurnSource(args []string, r io.Reader) *chatTurnSource {
	if len(args) > 0 {
		turns := make([]string, len(args))
		copy(turns, args)
		return &chatTurnSource{args: turns}
	}
	if r == nil {
		return &chatTurnSource{}
	}
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	return &chatTurnSource{scanner: scanner}
}

// Next returns the next turn, or ok=false at EOF, on a line that is exactly
// /quit or /exit, or on a read error. After Scan returns false the scanner
// error is checked: on a non-nil, non-EOF error, "error: reading turns: %v"
// is printed to stderr once before reporting ok=false.
func (s *chatTurnSource) Next() (turn string, ok bool) {
	if s == nil || s.done {
		return "", false
	}
	if len(s.args) > 0 {
		turn := s.args[0]
		s.args = s.args[1:]
		return turn, true
	}
	if s.scanner == nil {
		s.done = true
		return "", false
	}
	for s.scanner.Scan() {
		line := strings.TrimSpace(s.scanner.Text())
		if line == "" {
			continue
		}
		if line == "/quit" || line == "/exit" {
			s.done = true
			return "", false
		}
		return line, true
	}
	if err := s.scanner.Err(); err != nil && err != io.EOF {
		if !s.errReported {
			fmt.Fprintf(os.Stderr, "error: reading turns: %v\n", err)
			s.errReported = true
		}
	}
	s.done = true
	s.scanner = nil
	return "", false
}

// runChat boots the Cortex once and feeds successive turns to
// cortex.SessionExecutor.Process, mirroring the TUI main-agent path.
func runChat(cmd *cobra.Command, args []string) error {
	// A conversation has no natural total length, so the session itself is
	// not wrapped in the global --timeout. Each turn gets its own deadline
	// below. Cancellation comes from SIGINT/SIGTERM, same as runInstruction.
	// The Cortex is booted first, before any turn is pulled, so a driver
	// holding the stdin pipe can decide turn N+1 after seeing turn N's
	// result instead of having to close the pipe before any turn runs.
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

	fmt.Println("ready")
	src := newChatTurnSource(args, os.Stdin)
	turnNum := 0
	for turn, ok := src.Next(); ok; turn, ok = src.Next() {
		turnNum++
		fmt.Printf("── turn %d ──\n%s\n", turnNum, turn)
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
			fmt.Fprintf(os.Stderr, "error: nil result for turn %d\n", turnNum)
			continue
		}
		fmt.Println(result.Response)
		fmt.Printf("[tools executed=%d ok=%d writes=%d elapsed=%s]\n",
			result.ToolCallsExecuted, result.SuccessfulToolCalls, result.SuccessfulWriteTools, elapsed)
	}
	return nil
}
