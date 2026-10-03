package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"codenerd/internal/session"
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
	errWriter   io.Writer
	context     context.Context
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
		if !s.errReported && (s.context == nil || s.context.Err() == nil) {
			errWriter := s.errWriter
			if errWriter == nil {
				errWriter = os.Stderr
			}
			fmt.Fprintf(errWriter, "error: reading turns: %v\n", err)
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
	return runChatWith(cmd, args, chatDependencies{
		input: os.Stdin, output: os.Stdout, errOutput: os.Stderr,
		boot: bootChatRuntime, heartbeat: heartbeatInterval,
		notifySignals: func(notifications chan<- os.Signal) func() {
			signal.Notify(notifications, syscall.SIGINT, syscall.SIGTERM)
			return func() { signal.Stop(notifications) }
		},
	})
}

type chatRuntime struct {
	process func(context.Context, string) (*session.ExecutionResult, error)
	close   func() error
}

type chatDependencies struct {
	input         io.ReadCloser
	output        io.Writer
	errOutput     io.Writer
	boot          func(context.Context) (*chatRuntime, error)
	heartbeat     time.Duration
	notifySignals func(chan<- os.Signal) func()
}

func bootChatRuntime(processCtx context.Context) (*chatRuntime, error) {
	key := resolveAPIKey(apiKey, workspace)
	cortex, err := coresys.GetOrBootCortex(processCtx, workspace, key, disableSystemShards)
	if err != nil {
		return nil, err
	}
	runtime := &chatRuntime{close: cortex.Close}
	if cortex.VirtualStore != nil {
		cortex.VirtualStore.DisableBootGuard()
	}
	if cortex.SessionExecutor == nil {
		return runtime, fmt.Errorf("chat: session executor is not available (cortex boot did not provide one)")
	}
	// The session identity is minted once at boot (Cortex.SessionID) so every
	// headless turn persists under the same identity as campaign tasks and
	// sub-agents. Minting a second ID here used to fork the chat history away
	// from the boot identity.
	if sid := cortex.SessionID(); sid != "" {
		cortex.SessionExecutor.SetSessionID(sid)
	}
	runtime.process = func(turnCtx context.Context, turn string) (*session.ExecutionResult, error) {
		if cortex.UsageTracker != nil {
			turnCtx = usage.NewContext(turnCtx, cortex.UsageTracker)
		}
		return cortex.SessionExecutor.Process(turnCtx, turn)
	}
	return runtime, nil
}

func runChatWith(cmd *cobra.Command, args []string, dependencies chatDependencies) (outcome error) {
	processCtx, cancel := commandContext(cmd)
	defer cancel()
	if dependencies.notifySignals != nil {
		notifications := make(chan os.Signal, 1)
		stopSignals := dependencies.notifySignals(notifications)
		signalsDone := make(chan struct{})
		go func() {
			defer close(signalsDone)
			select {
			case <-notifications:
				cancel()
			case <-processCtx.Done():
			}
		}()
		defer func() {
			cancel()
			stopSignals()
			<-signalsDone
		}()
	}

	if err := processCtx.Err(); err != nil {
		return fmt.Errorf("chat: %w", err)
	}
	runtime, err := dependencies.boot(processCtx)
	if runtime != nil && runtime.close != nil {
		defer func() {
			cancel()
			if closeErr := runtime.close(); closeErr != nil {
				outcome = errors.Join(outcome, fmt.Errorf("chat cleanup: %w", closeErr))
			}
		}()
	}
	if err != nil {
		return fmt.Errorf("failed to boot cortex: %w", err)
	}
	if err := processCtx.Err(); err != nil {
		return fmt.Errorf("chat: %w", err)
	}
	if runtime == nil || runtime.process == nil {
		return fmt.Errorf("chat: session executor is not available (cortex boot did not provide one)")
	}

	src := newChatTurnSource(args, dependencies.input)
	src.errWriter = dependencies.errOutput
	src.context = processCtx
	if len(args) == 0 && dependencies.input != nil {
		readerDone := make(chan struct{})
		var readerCloseErr error
		stopReader := context.AfterFunc(processCtx, func() {
			defer close(readerDone)
			readerCloseErr = dependencies.input.Close()
		})
		defer func() {
			if !stopReader() {
				<-readerDone
				if readerCloseErr != nil && !errors.Is(readerCloseErr, os.ErrClosed) {
					outcome = errors.Join(outcome, fmt.Errorf("chat input cleanup: %w", readerCloseErr))
				}
			}
		}()
	}
	fmt.Fprintln(dependencies.output, "ready")
	turnNum := 0
	for {
		if err := processCtx.Err(); err != nil {
			return fmt.Errorf("chat: %w", err)
		}
		turn, ok := src.Next()
		if err := processCtx.Err(); err != nil {
			return fmt.Errorf("chat: %w", err)
		}
		if !ok {
			return nil
		}
		turnNum++
		fmt.Fprintf(dependencies.output, "── turn %d ──\n%s\n", turnNum, turn)
		turnCtx, turnCancel := context.WithCancel(processCtx)
		if err := turnCtx.Err(); err != nil {
			turnCancel()
			return fmt.Errorf("chat: %w", err)
		}
		stopHeartbeat := startHeartbeat(dependencies.output, dependencies.heartbeat)
		start := time.Now()
		result, procErr := runtime.process(turnCtx, turn)
		elapsed := time.Since(start)
		stopHeartbeat()
		turnCancel()
		if procErr != nil {
			fmt.Fprintf(dependencies.errOutput, "error: %v\n", procErr)
		} else if result == nil {
			fmt.Fprintf(dependencies.errOutput, "error: nil result for turn %d\n", turnNum)
		} else {
			renderChatTurn(dependencies.output, dependencies.errOutput, turnNum, result, elapsed)
		}
		if err := processCtx.Err(); err != nil {
			return fmt.Errorf("chat: %w", err)
		}
	}
}

// renderChatTurn renders one completed chat turn to w (stdout) and surfaces
// any soft execution error to errW (stderr). It is pure — no Cortex, no clock,
// no globals — so it is unit-testable without booting anything.
//
// A soft failure (result.Error set with a nil Process error) still prints the
// response text, which may hold partial diagnostics, and always prints the
// closing footer so a driver watching the stream sees both the error line and
// the counters. The footer carries err=<yes|no> for stdout-only drivers.
func renderChatTurn(w io.Writer, errW io.Writer, turnNum int, result *session.ExecutionResult, elapsed time.Duration) {
	_ = turnNum
	if result.Error != nil {
		fmt.Fprintf(errW, "error: %v\n", result.Error)
	}
	fmt.Fprintln(w, result.Response)
	errFlag := "no"
	if result.Error != nil {
		errFlag = "yes"
	}
	fmt.Fprintf(w, "[tools executed=%d ok=%d writes=%d err=%s elapsed=%s]\n",
		result.ToolCallsExecuted, result.SuccessfulToolCalls, result.SuccessfulWriteTools, errFlag, elapsed)
}
