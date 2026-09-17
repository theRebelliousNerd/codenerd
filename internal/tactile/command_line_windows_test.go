//go:build windows

package tactile

import (
	"context"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestCommandLineDirectExecutor(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	e := NewDirectExecutorWithConfig(DefaultExecutorConfig())

	withLine := Command{
		Binary:      "cmd.exe",
		Arguments:   []string{"/D", "/S", "/C", `echo "a b"`},
		CommandLine: `cmd.exe /D /S /C "echo "a b""`,
	}
	result, err := e.Execute(ctx, withLine)
	if err != nil {
		t.Fatalf("Execute with CommandLine failed: %v", err)
	}
	if got := strings.TrimSpace(result.Stdout); got != `"a b"` {
		t.Fatalf("Execute with CommandLine: got stdout %q, want %q", got, `"a b"`)
	}

	withoutLine := Command{
		Binary:    "cmd.exe",
		Arguments: []string{"/D", "/S", "/C", `echo "a b"`},
	}
	result, err = e.Execute(ctx, withoutLine)
	if err != nil {
		t.Fatalf("Execute without CommandLine failed: %v", err)
	}
	if got := strings.TrimSpace(result.Stdout); got == `"a b"` {
		t.Fatalf("Execute without CommandLine: got stdout %q, expected corruption (not %q)", got, `"a b"`)
	}
}

func TestCommandLineLimitedExecutorWindows(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go not in PATH")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	e := NewLimitedExecutorWindows(DefaultExecutorConfig())

	withLine := Command{
		Binary:      "go",
		Arguments:   []string{"env", "GOARCH"},
		CommandLine: "go env GOOS",
		Limits:      &ResourceLimits{TimeoutMs: 30000},
	}
	result, err := e.Execute(ctx, withLine)
	if err != nil {
		t.Fatalf("LimitedExecutorWindows Execute with CommandLine failed: %v", err)
	}
	if got := strings.TrimSpace(result.Stdout); got != runtime.GOOS {
		t.Fatalf("LimitedExecutorWindows Execute with CommandLine: got stdout %q, want %q", got, runtime.GOOS)
	}

	withoutLine := Command{
		Binary:    "go",
		Arguments: []string{"env", "GOARCH"},
		Limits:    &ResourceLimits{TimeoutMs: 30000},
	}
	result, err = e.Execute(ctx, withoutLine)
	if err != nil {
		t.Fatalf("LimitedExecutorWindows Execute without CommandLine failed: %v", err)
	}
	if got := strings.TrimSpace(result.Stdout); got != runtime.GOARCH {
		t.Fatalf("LimitedExecutorWindows Execute without CommandLine: got stdout %q, want %q", got, runtime.GOARCH)
	}
}
