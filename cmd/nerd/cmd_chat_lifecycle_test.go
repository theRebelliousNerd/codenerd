package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"codenerd/internal/session"

	"github.com/spf13/cobra"
)

func chatLifecycleCommand(test *testing.T, parent context.Context, limit time.Duration) *cobra.Command {
	test.Helper()
	savedTimeout := timeout
	timeout = limit
	test.Cleanup(func() { timeout = savedTimeout })
	command := &cobra.Command{}
	command.SetContext(parent)
	return command
}

func chatLifecycleDependencies(boot func(context.Context) (*chatRuntime, error)) chatDependencies {
	return chatDependencies{
		boot: boot, output: io.Discard, errOutput: io.Discard, heartbeat: time.Hour,
	}
}

func awaitChatLifecycle(test *testing.T, result <-chan error) error {
	test.Helper()
	select {
	case outcome := <-result:
		return outcome
	case <-time.After(5 * time.Second):
		test.Fatal("chat lifecycle did not finish within the regression watchdog")
		return nil
	}
}

func TestChatLifecycle_DeadlineIncludesBoot(test *testing.T) {
	command := chatLifecycleCommand(test, context.Background(), 100*time.Millisecond)
	var bootDeadline time.Time
	dependencies := chatLifecycleDependencies(func(commandCtx context.Context) (*chatRuntime, error) {
		var bounded bool
		bootDeadline, bounded = commandCtx.Deadline()
		if !bounded {
			return nil, errors.New("boot received no command deadline")
		}
		<-commandCtx.Done()
		return nil, commandCtx.Err()
	})
	result := make(chan error, 1)
	go func() { result <- runChatWith(command, []string{"never admitted"}, dependencies) }()
	if outcome := awaitChatLifecycle(test, result); !errors.Is(outcome, context.DeadlineExceeded) {
		test.Fatalf("boot result = %v, want deadline exceeded", outcome)
	}
	if bootDeadline.IsZero() {
		test.Fatal("boot did not receive the deadline")
	}
}

func TestChatLifecycle_OneDeadlineAcrossTurnsAndPreservesArtifacts(test *testing.T) {
	parent, cancel := context.WithCancel(context.Background())
	test.Cleanup(cancel)
	command := chatLifecycleCommand(test, parent, 200*time.Millisecond)
	artifactPath := filepath.Join(test.TempDir(), "completed.go")
	artifactContent := []byte("package fixture\n")
	var bootDeadline time.Time
	var admitted []string
	closed := false
	dependencies := chatLifecycleDependencies(func(commandCtx context.Context) (*chatRuntime, error) {
		bootDeadline, _ = commandCtx.Deadline()
		return &chatRuntime{
			process: func(turnCtx context.Context, turn string) (*session.ExecutionResult, error) {
				admitted = append(admitted, turn)
				turnDeadline, bounded := turnCtx.Deadline()
				if !bounded || !turnDeadline.Equal(bootDeadline) {
					return nil, fmt.Errorf("turn deadline %v differs from boot %v", turnDeadline, bootDeadline)
				}
				if turn == "write" {
					if err := os.WriteFile(artifactPath, artifactContent, 0600); err != nil {
						return nil, err
					}
					return &session.ExecutionResult{Response: "saved", SuccessfulWriteTools: 1}, nil
				}
				<-turnCtx.Done()
				return nil, turnCtx.Err()
			},
			close: func() error { closed = true; return nil },
		}, nil
	})
	var output bytes.Buffer
	dependencies.output = &output
	result := make(chan error, 1)
	go func() { result <- runChatWith(command, []string{"write", "blocked", "forbidden"}, dependencies) }()
	if outcome := awaitChatLifecycle(test, result); !errors.Is(outcome, context.DeadlineExceeded) {
		test.Fatalf("command result = %v, want deadline exceeded", outcome)
	}
	if strings.Join(admitted, ",") != "write,blocked" || !closed {
		test.Fatalf("admitted = %v, cleanup = %v", admitted, closed)
	}
	preserved, err := os.ReadFile(artifactPath)
	if err != nil || !bytes.Equal(preserved, artifactContent) {
		test.Fatalf("completed artifact = %q, error = %v", preserved, err)
	}
	if !strings.Contains(output.String(), "writes=1 err=no") {
		test.Fatalf("completed turn footer missing: %q", output.String())
	}
}

func TestChatLifecycle_NonPositiveTimeoutAddsNoDeadline(test *testing.T) {
	for _, limit := range []time.Duration{0, -time.Second} {
		test.Run(limit.String(), func(test *testing.T) {
			command := chatLifecycleCommand(test, context.Background(), limit)
			admitted := 0
			dependencies := chatLifecycleDependencies(func(commandCtx context.Context) (*chatRuntime, error) {
				if _, bounded := commandCtx.Deadline(); bounded {
					return nil, errors.New("boot unexpectedly has a deadline")
				}
				return &chatRuntime{process: func(turnCtx context.Context, turn string) (*session.ExecutionResult, error) {
					if _, bounded := turnCtx.Deadline(); bounded {
						return nil, errors.New("turn unexpectedly has a deadline")
					}
					admitted++
					return &session.ExecutionResult{Response: turn}, nil
				}}, nil
			})
			if outcome := runChatWith(command, []string{"one", "two"}, dependencies); outcome != nil || admitted != 2 {
				test.Fatalf("result = %v, admitted = %d", outcome, admitted)
			}
		})
	}
}

func TestChatLifecycle_ParentCancellationReachesBootAndActiveTurn(test *testing.T) {
	for _, phase := range []string{"boot", "turn"} {
		test.Run(phase, func(test *testing.T) {
			parent, cancel := context.WithCancel(context.Background())
			test.Cleanup(cancel)
			command := chatLifecycleCommand(test, parent, 0)
			entered := make(chan struct{})
			admitted := 0
			dependencies := chatLifecycleDependencies(func(commandCtx context.Context) (*chatRuntime, error) {
				if phase == "boot" {
					close(entered)
					<-commandCtx.Done()
					return nil, commandCtx.Err()
				}
				return &chatRuntime{process: func(turnCtx context.Context, turn string) (*session.ExecutionResult, error) {
					admitted++
					close(entered)
					<-turnCtx.Done()
					return nil, turnCtx.Err()
				}}, nil
			})
			result := make(chan error, 1)
			go func() { result <- runChatWith(command, []string{"blocked", "forbidden"}, dependencies) }()
			select {
			case <-entered:
			case <-time.After(5 * time.Second):
				test.Fatal("lifecycle collaborator was not entered")
			}
			cancel()
			if outcome := awaitChatLifecycle(test, result); !errors.Is(outcome, context.Canceled) {
				test.Fatalf("result = %v, want parent cancellation", outcome)
			}
			if admitted > 1 {
				test.Fatalf("admitted %d turns after cancellation", admitted)
			}
		})
	}
}

type chatLifecycleInput struct {
	io.ReadCloser
	readStarted  chan struct{}
	closed       chan struct{}
	readOnce     sync.Once
	closeStarted chan struct{}
	closeRelease <-chan struct{}
}

func (input *chatLifecycleInput) Read(buffer []byte) (int, error) {
	input.readOnce.Do(func() { close(input.readStarted) })
	return input.ReadCloser.Read(buffer)
}

func (input *chatLifecycleInput) Close() error {
	defer close(input.closed)
	closeErr := input.ReadCloser.Close()
	if input.closeStarted != nil {
		close(input.closeStarted)
	}
	if input.closeRelease != nil {
		<-input.closeRelease
	}
	return closeErr
}

func TestChatLifecycle_DoesNotAbandonInputCleanup(test *testing.T) {
	reader, writer := io.Pipe()
	test.Cleanup(func() { _ = reader.Close(); _ = writer.Close() })
	parent, cancel := context.WithCancel(context.Background())
	test.Cleanup(cancel)
	command := chatLifecycleCommand(test, parent, 0)
	closeRelease := make(chan struct{})
	var releaseOnce sync.Once
	releaseClose := func() { releaseOnce.Do(func() { close(closeRelease) }) }
	test.Cleanup(releaseClose)
	input := &chatLifecycleInput{
		ReadCloser: reader, readStarted: make(chan struct{}), closed: make(chan struct{}),
		closeStarted: make(chan struct{}), closeRelease: closeRelease,
	}
	dependencies := chatLifecycleDependencies(func(context.Context) (*chatRuntime, error) {
		return &chatRuntime{process: func(context.Context, string) (*session.ExecutionResult, error) {
			return &session.ExecutionResult{}, nil
		}}, nil
	})
	dependencies.input = input
	result := make(chan error, 1)
	go func() { result <- runChatWith(command, nil, dependencies) }()
	select {
	case <-input.readStarted:
	case <-time.After(5 * time.Second):
		test.Fatal("input read not started")
	}
	cancel()
	select {
	case <-input.closeStarted:
	case <-time.After(5 * time.Second):
		test.Fatal("input close not started")
	}
	select {
	case outcome := <-result:
		test.Fatalf("command returned before its input closer finished: %v", outcome)
	case <-time.After(50 * time.Millisecond):
	}
	releaseClose()
	if outcome := awaitChatLifecycle(test, result); !errors.Is(outcome, context.Canceled) {
		test.Fatalf("result = %v, want cancellation", outcome)
	}
}

func TestChatLifecycle_CancellationUnblocksIdleInputAndJoinsCloser(test *testing.T) {
	for _, pipeKind := range []string{"io", "os"} {
		test.Run(pipeKind, func(test *testing.T) {
			var reader io.ReadCloser
			var writer io.WriteCloser
			if pipeKind == "io" {
				reader, writer = io.Pipe()
			} else {
				var err error
				reader, writer, err = os.Pipe()
				if err != nil {
					test.Fatal(err)
				}
			}
			test.Cleanup(func() { _ = reader.Close(); _ = writer.Close() })
			input := &chatLifecycleInput{ReadCloser: reader, readStarted: make(chan struct{}), closed: make(chan struct{})}
			parent, cancel := context.WithCancel(context.Background())
			test.Cleanup(cancel)
			command := chatLifecycleCommand(test, parent, 0)
			admitted := 0
			dependencies := chatLifecycleDependencies(func(context.Context) (*chatRuntime, error) {
				return &chatRuntime{process: func(context.Context, string) (*session.ExecutionResult, error) {
					admitted++
					return &session.ExecutionResult{}, nil
				}}, nil
			})
			dependencies.input = input
			result := make(chan error, 1)
			go func() { result <- runChatWith(command, nil, dependencies) }()
			select {
			case <-input.readStarted:
			case <-time.After(5 * time.Second):
				test.Fatal("chat did not begin its idle read")
			}
			cancel()
			if outcome := awaitChatLifecycle(test, result); !errors.Is(outcome, context.Canceled) {
				test.Fatalf("idle result = %v, want cancellation", outcome)
			}
			select {
			case <-input.closed:
			default:
				test.Fatal("input close callback was not joined")
			}
			if admitted != 0 {
				test.Fatalf("idle cancellation admitted %d turns", admitted)
			}
		})
	}
}

func TestChatLifecycle_DeadlineUnblocksIdleInput(test *testing.T) {
	reader, writer := io.Pipe()
	test.Cleanup(func() { _ = reader.Close(); _ = writer.Close() })
	command := chatLifecycleCommand(test, context.Background(), 100*time.Millisecond)
	dependencies := chatLifecycleDependencies(func(context.Context) (*chatRuntime, error) {
		return &chatRuntime{process: func(context.Context, string) (*session.ExecutionResult, error) {
			return nil, errors.New("idle input unexpectedly admitted a turn")
		}}, nil
	})
	dependencies.input = reader
	result := make(chan error, 1)
	go func() { result <- runChatWith(command, nil, dependencies) }()
	if outcome := awaitChatLifecycle(test, result); !errors.Is(outcome, context.DeadlineExceeded) {
		test.Fatalf("idle deadline result = %v", outcome)
	}
}

func TestChatLifecycle_ProductionStdinIdleDeadline(test *testing.T) {
	if os.Getenv("CODENERD_CHAT_IDLE_STDIN_CHILD") == "1" {
		command := chatLifecycleCommand(test, context.Background(), 100*time.Millisecond)
		dependencies := chatLifecycleDependencies(func(context.Context) (*chatRuntime, error) {
			return &chatRuntime{process: func(context.Context, string) (*session.ExecutionResult, error) {
				return nil, errors.New("idle production stdin admitted a turn")
			}}, nil
		})
		dependencies.input = os.Stdin
		if outcome := runChatWith(command, nil, dependencies); !errors.Is(outcome, context.DeadlineExceeded) {
			test.Fatalf("production stdin result = %v, want deadline exceeded", outcome)
		}
		return
	}
	executable, err := os.Executable()
	if err != nil {
		test.Fatal(err)
	}
	reader, writer, err := os.Pipe()
	if err != nil {
		test.Fatal(err)
	}
	test.Cleanup(func() { _ = reader.Close(); _ = writer.Close() })
	watchdog, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	child := exec.CommandContext(watchdog, executable, "-test.run=^TestChatLifecycle_ProductionStdinIdleDeadline$")
	child.Env = append(os.Environ(), "CODENERD_CHAT_IDLE_STDIN_CHILD=1")
	child.Stdin = reader
	output, err := child.CombinedOutput()
	if err != nil {
		test.Fatalf("inherited idle stdin did not cancel and join: %v (watchdog: %v)\n%s", err, watchdog.Err(), output)
	}
}

func TestChatLifecycle_StdinDriverDoesNotNeedEOFBeforeFirstTurn(test *testing.T) {
	reader, writer := io.Pipe()
	test.Cleanup(func() { _ = reader.Close(); _ = writer.Close() })
	parent, cancel := context.WithCancel(context.Background())
	test.Cleanup(cancel)
	command := chatLifecycleCommand(test, parent, 0)
	firstProcessed := make(chan struct{})
	dependencies := chatLifecycleDependencies(func(context.Context) (*chatRuntime, error) {
		return &chatRuntime{process: func(turnCtx context.Context, turn string) (*session.ExecutionResult, error) {
			if turn != "first" {
				return nil, fmt.Errorf("unexpected turn %q", turn)
			}
			close(firstProcessed)
			return &session.ExecutionResult{Response: "done"}, nil
		}}, nil
	})
	dependencies.input = reader
	result := make(chan error, 1)
	go func() { result <- runChatWith(command, nil, dependencies) }()
	if _, err := io.WriteString(writer, "first\n"); err != nil {
		test.Fatal(err)
	}
	select {
	case <-firstProcessed:
	case <-time.After(5 * time.Second):
		test.Fatal("first turn required stdin EOF")
	}
	if _, err := io.WriteString(writer, "/quit\n"); err != nil {
		test.Fatal(err)
	}
	if outcome := awaitChatLifecycle(test, result); outcome != nil {
		test.Fatalf("quit result = %v", outcome)
	}
}

func TestChatLifecycle_SignalsCancelWorkAndUnregisterOnEveryExit(test *testing.T) {
	for _, shouldSignal := range []bool{false, true} {
		test.Run(fmt.Sprint(shouldSignal), func(test *testing.T) {
			parent, cancel := context.WithCancel(context.Background())
			test.Cleanup(cancel)
			command := chatLifecycleCommand(test, parent, 0)
			entered := make(chan struct{})
			registered := make(chan chan<- os.Signal, 1)
			stopped := make(chan struct{})
			dependencies := chatLifecycleDependencies(func(context.Context) (*chatRuntime, error) {
				return &chatRuntime{process: func(turnCtx context.Context, turn string) (*session.ExecutionResult, error) {
					close(entered)
					if shouldSignal {
						<-turnCtx.Done()
						return nil, turnCtx.Err()
					}
					return &session.ExecutionResult{Response: turn}, nil
				}}, nil
			})
			dependencies.notifySignals = func(notifications chan<- os.Signal) func() {
				registered <- notifications
				return func() { close(stopped) }
			}
			result := make(chan error, 1)
			go func() { result <- runChatWith(command, []string{"first"}, dependencies) }()
			notifications := <-registered
			select {
			case <-entered:
			case <-time.After(5 * time.Second):
				test.Fatal("turn not admitted")
			}
			if shouldSignal {
				notifications <- os.Interrupt
			}
			outcome := awaitChatLifecycle(test, result)
			if shouldSignal && !errors.Is(outcome, context.Canceled) || !shouldSignal && outcome != nil {
				test.Fatalf("signal=%v, result=%v", shouldSignal, outcome)
			}
			select {
			case <-stopped:
			default:
				test.Fatal("signal registration was not released")
			}
		})
	}
}

func TestChatLifecycle_OrdinaryErrorsRetainFooterAndContinuation(test *testing.T) {
	command := chatLifecycleCommand(test, context.Background(), 0)
	dependencies := chatLifecycleDependencies(func(context.Context) (*chatRuntime, error) {
		return &chatRuntime{process: func(turnCtx context.Context, turn string) (*session.ExecutionResult, error) {
			switch turn {
			case "hard":
				return nil, errors.New("hard failure")
			case "nil":
				return nil, nil
			case "soft":
				return &session.ExecutionResult{Response: "partial", Error: errors.New("soft failure")}, nil
			default:
				return &session.ExecutionResult{Response: "success"}, nil
			}
		}}, nil
	})
	var output, errOutput bytes.Buffer
	dependencies.output, dependencies.errOutput = &output, &errOutput
	if outcome := runChatWith(command, []string{"hard", "nil", "soft", "success"}, dependencies); outcome != nil {
		test.Fatalf("ordinary error result = %v", outcome)
	}
	for _, expected := range []string{"partial", "err=yes", "success", "err=no"} {
		if !strings.Contains(output.String(), expected) {
			test.Fatalf("stdout lacks %q: %q", expected, output.String())
		}
	}
	for _, expected := range []string{"hard failure", "nil result for turn 2", "soft failure"} {
		if !strings.Contains(errOutput.String(), expected) {
			test.Fatalf("stderr lacks %q: %q", expected, errOutput.String())
		}
	}
}

func TestChatLifecycle_CancellationAndCleanupErrorsRemainVisible(test *testing.T) {
	parent, cancel := context.WithCancel(context.Background())
	test.Cleanup(cancel)
	command := chatLifecycleCommand(test, parent, 0)
	cleanupFailure := errors.New("cleanup failed")
	dependencies := chatLifecycleDependencies(func(context.Context) (*chatRuntime, error) {
		return &chatRuntime{
			process: func(context.Context, string) (*session.ExecutionResult, error) {
				cancel()
				return &session.ExecutionResult{Response: "partial", Error: context.Canceled}, nil
			},
			close: func() error { return cleanupFailure },
		}, nil
	})
	var output bytes.Buffer
	dependencies.output = &output
	outcome := runChatWith(command, []string{"first", "forbidden"}, dependencies)
	if !errors.Is(outcome, context.Canceled) || !errors.Is(outcome, cleanupFailure) {
		test.Fatalf("result lost cancellation or cleanup failure: %v", outcome)
	}
	if !strings.Contains(output.String(), "partial") || !strings.Contains(output.String(), "err=yes") || strings.Contains(output.String(), "turn 2") {
		test.Fatalf("cancelled turn rendering/admission = %q", output.String())
	}
}

func TestChatLifecycle_InheritsEarlierParentDeadline(test *testing.T) {
	parent, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	parentDeadline, _ := parent.Deadline()
	command := chatLifecycleCommand(test, parent, time.Hour)
	matchedDeadline := false
	dependencies := chatLifecycleDependencies(func(commandCtx context.Context) (*chatRuntime, error) {
		bootDeadline, _ := commandCtx.Deadline()
		return &chatRuntime{process: func(turnCtx context.Context, turn string) (*session.ExecutionResult, error) {
			turnDeadline, _ := turnCtx.Deadline()
			matchedDeadline = bootDeadline.Equal(parentDeadline) && turnDeadline.Equal(parentDeadline)
			<-turnCtx.Done()
			return nil, turnCtx.Err()
		}}, nil
	})
	result := make(chan error, 1)
	go func() { result <- runChatWith(command, []string{"blocked"}, dependencies) }()
	if outcome := awaitChatLifecycle(test, result); !errors.Is(outcome, context.DeadlineExceeded) || !matchedDeadline {
		test.Fatalf("result = %v, inherited boot/turn deadline = %v", outcome, matchedDeadline)
	}
}

func TestChatLifecycle_PreCancelledCommandDoesNotBoot(test *testing.T) {
	parent, cancel := context.WithCancel(context.Background())
	cancel()
	command := chatLifecycleCommand(test, parent, 0)
	booted := false
	dependencies := chatLifecycleDependencies(func(context.Context) (*chatRuntime, error) {
		booted = true
		return nil, errors.New("boot should not be reached")
	})
	if outcome := runChatWith(command, []string{"forbidden"}, dependencies); !errors.Is(outcome, context.Canceled) || booted {
		test.Fatalf("result = %v, booted = %v", outcome, booted)
	}
}

func TestChatLifecycle_ArgsNeverReadOrCloseInput(test *testing.T) {
	parent, cancel := context.WithCancel(context.Background())
	defer cancel()
	command := chatLifecycleCommand(test, parent, 0)
	input := &chatLifecycleInput{
		ReadCloser:  io.NopCloser(strings.NewReader("ignored\n")),
		readStarted: make(chan struct{}), closed: make(chan struct{}),
	}
	dependencies := chatLifecycleDependencies(func(context.Context) (*chatRuntime, error) {
		return &chatRuntime{process: func(context.Context, string) (*session.ExecutionResult, error) {
			cancel()
			return &session.ExecutionResult{Response: "done"}, nil
		}}, nil
	})
	dependencies.input = input
	if outcome := runChatWith(command, []string{"first"}, dependencies); !errors.Is(outcome, context.Canceled) {
		test.Fatalf("result = %v, want cancellation", outcome)
	}
	select {
	case <-input.readStarted:
		test.Fatal("positional arguments caused an input read")
	case <-input.closed:
		test.Fatal("positional arguments caused input closure")
	default:
	}
}

func TestChatLifecycle_RequestTimeoutDoesNotCancelCommand(test *testing.T) {
	for _, softFailure := range []bool{false, true} {
		test.Run(fmt.Sprint(softFailure), func(subtest *testing.T) {
			command := chatLifecycleCommand(subtest, context.Background(), 0)
			admitted := 0
			dependencies := chatLifecycleDependencies(func(context.Context) (*chatRuntime, error) {
				return &chatRuntime{process: func(context.Context, string) (*session.ExecutionResult, error) {
					admitted++
					if admitted > 1 {
						return &session.ExecutionResult{Response: "recovered"}, nil
					}
					if softFailure {
						return &session.ExecutionResult{Response: "partial", Error: context.DeadlineExceeded}, nil
					}
					return nil, context.DeadlineExceeded
				}}, nil
			})
			if outcome := runChatWith(command, []string{"first", "recover"}, dependencies); outcome != nil || admitted != 2 {
				subtest.Fatalf("result = %v, admitted = %d", outcome, admitted)
			}
		})
	}
}
