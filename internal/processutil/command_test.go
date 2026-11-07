package processutil

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestNonInteractiveProvidesFiniteEmptyStdin(t *testing.T) {
	cmd := NonInteractive(exec.Command("ignored"))
	if cmd.Stdin == nil {
		t.Fatal("NonInteractive left stdin nil")
	}
	got, err := io.ReadAll(cmd.Stdin)
	if err != nil {
		t.Fatalf("read stdin: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("stdin = %q, want immediate EOF", got)
	}
}

func TestNonInteractivePreservesExplicitStdin(t *testing.T) {
	want := strings.NewReader("payload")
	cmd := exec.Command("ignored")
	cmd.Stdin = want

	if got := NonInteractive(cmd); got != cmd {
		t.Fatal("NonInteractive returned a different command")
	}
	if cmd.Stdin != want {
		t.Fatal("NonInteractive replaced caller-provided stdin")
	}
}

func TestNonInteractiveAcceptsNil(t *testing.T) {
	if got := NonInteractive(nil); got != nil {
		t.Fatalf("NonInteractive(nil) = %v, want nil", got)
	}
}

func TestCombinedOutputRejectsPresetStderr(t *testing.T) {
	cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestHelperPrint$")
	cmd.Stderr = io.Discard
	_, err := CombinedOutput(cmd)
	if err == nil || !strings.Contains(err.Error(), "Stderr already set") {
		t.Fatalf("expected Stderr already set error, got %v", err)
	}
	if cmd.Process != nil {
		t.Fatal("process was started despite preset Stderr")
	}
}

func TestStartSetsWaitDelayAndNonInteractiveStdin(t *testing.T) {
	cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestStartHiHelper$")
	cmd.Env = append(os.Environ(), "CODENERD_START_HELPER=1")
	var buf bytes.Buffer
	cmd.Stdout = &buf
	release, err := Start(cmd)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer release()
	if cmd.WaitDelay != PipeWaitDelay {
		t.Fatalf("WaitDelay = %v, want %v", cmd.WaitDelay, PipeWaitDelay)
	}
	if cmd.Stdin == nil {
		t.Fatal("Start left stdin nil; want finite empty stdin")
	}
	if err := cmd.Wait(); err != nil {
		t.Fatalf("Wait failed: %v", err)
	}
	if !strings.Contains(buf.String(), "hi") {
		t.Fatalf("expected output to contain %q, got %q", "hi", buf.String())
	}
}

func TestStartInvalidCommandReturnsErrorWithCallableRelease(t *testing.T) {
	cmd := exec.CommandContext(t.Context(), filepath.Join(t.TempDir(), "no-such-binary"))
	release, err := Start(cmd)
	if err == nil {
		release()
		t.Fatal("Start with missing binary returned no error")
	}
	if release == nil {
		t.Fatal("Start returned nil release on error")
	}
	release()
	release()
	if cmd.Process != nil {
		t.Fatal("failed Start left a process behind")
	}
}

func TestRunInvalidCommandReturnsError(t *testing.T) {
	cmd := exec.CommandContext(t.Context(), filepath.Join(t.TempDir(), "no-such-binary"))
	if err := Run(cmd); err == nil {
		t.Fatal("Run with missing binary returned no error")
	}
}

func TestCombinedFailHelper(t *testing.T) {
	if os.Getenv("CODENERD_COMBINED_FAIL_HELPER") != "1" {
		return
	}
	fmt.Println("out-marker")
	fmt.Fprintln(os.Stderr, "err-marker")
	os.Exit(1)
}

func TestCombinedOutputReturnsOutputOnFailure(t *testing.T) {
	cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestCombinedFailHelper$")
	cmd.Env = append(os.Environ(), "CODENERD_COMBINED_FAIL_HELPER=1")
	out, err := CombinedOutput(cmd)
	if err == nil {
		t.Fatal("CombinedOutput with failing helper returned no error")
	}
	if !strings.Contains(string(out), "out-marker") || !strings.Contains(string(out), "err-marker") {
		t.Fatalf("combined output = %q, want both markers", out)
	}
}
