package processutil

import (
	"io"
	"os/exec"
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
