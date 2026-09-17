package core

import (
	"codenerd/internal/tactile"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestVerificationUsesHostToolchain(t *testing.T) {
	binary, args, commandLine := verificationShell("go env GOOS")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	result, err := tactile.NewDirectExecutor().Execute(ctx, tactile.Command{Binary: binary, Arguments: args, CommandLine: commandLine, WorkingDirectory: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Success || result.ExitCode != 0 || strings.TrimSpace(result.Stdout) != runtime.GOOS {
		t.Fatalf("verification crossed host boundary: host=%s result=%+v", runtime.GOOS, result)
	}
}
func TestVerificationPreservesFailedExit(t *testing.T) {
	binary, args, commandLine := verificationShell("exit 7")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	result, err := tactile.NewDirectExecutor().Execute(ctx, tactile.Command{Binary: binary, Arguments: args, CommandLine: commandLine, WorkingDirectory: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if result.ExitCode != 7 {
		t.Fatalf("shell hid failure: %+v", result)
	}
}

func TestVerificationPassesQuotedArgumentsVerbatim(t *testing.T) {
	binary, args, commandLine := verificationShell(`go env "GOOS"`)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	result, err := tactile.NewDirectExecutor().Execute(ctx, tactile.Command{Binary: binary, Arguments: args, CommandLine: commandLine, WorkingDirectory: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("quoted verification command failed: %+v", result)
	}
	if got := strings.TrimSpace(result.Stdout); got != runtime.GOOS {
		t.Fatalf("quoted argument corrupted: got %q want %q (result=%+v)", got, runtime.GOOS, result)
	}
}

func TestVerificationRunsQuotedShellScript(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not available")
	}
	dir := t.TempDir()
	binary, args, commandLine := verificationShell(`sh -c "if [ ! -f marker ]; then touch marker; exit 3; else exit 0; fi"`)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	first, err := tactile.NewDirectExecutor().Execute(ctx, tactile.Command{Binary: binary, Arguments: args, CommandLine: commandLine, WorkingDirectory: dir})
	if err != nil {
		t.Fatal(err)
	}
	if first.ExitCode != 3 {
		t.Fatalf("first run: got exit %d want 3 (result=%+v)", first.ExitCode, first)
	}
	if _, err := os.Stat(filepath.Join(dir, "marker")); err != nil {
		t.Fatalf("first run did not create marker: %v (result=%+v)", err, first)
	}
	second, err := tactile.NewDirectExecutor().Execute(ctx, tactile.Command{Binary: binary, Arguments: args, CommandLine: commandLine, WorkingDirectory: dir})
	if err != nil {
		t.Fatal(err)
	}
	if second.ExitCode != 0 {
		t.Fatalf("second run: got exit %d want 0 (result=%+v)", second.ExitCode, second)
	}
}
