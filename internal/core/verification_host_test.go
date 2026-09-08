package core

import (
	"codenerd/internal/tactile"
	"context"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestVerificationUsesHostToolchain(t *testing.T) {
	binary, args := verificationShell("go env GOOS")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	result, err := tactile.NewDirectExecutor().Execute(ctx, tactile.Command{Binary: binary, Arguments: args, WorkingDirectory: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Success || result.ExitCode != 0 || strings.TrimSpace(result.Stdout) != runtime.GOOS {
		t.Fatalf("verification crossed host boundary: host=%s result=%+v", runtime.GOOS, result)
	}
}
func TestVerificationPreservesFailedExit(t *testing.T) {
	binary, args := verificationShell("exit 7")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	result, err := tactile.NewDirectExecutor().Execute(ctx, tactile.Command{Binary: binary, Arguments: args, WorkingDirectory: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if result.ExitCode != 7 {
		t.Fatalf("shell hid failure: %+v", result)
	}
}
