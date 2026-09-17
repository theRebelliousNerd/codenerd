package processutil

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRun_DeadlineKillsTheWholeTree(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available")
	}
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not available")
	}
	dir := t.TempDir()
	marker := filepath.Join(dir, "alive")
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	// The inner `exec` is what matters — MSYS exec leaves a process taskkill /T does not reach,
	// so this is the case the job object exists for.
	cmd := exec.CommandContext(ctx, "bash", "-c", `sh -c "exec sh -c 'sleep 4; echo alive > `+filepath.ToSlash(marker)+`'"`)
	started := time.Now()
	err := Run(cmd)
	if err == nil {
		t.Fatal("Run returned no error although the deadline passed")
	}
	if ctx.Err() == nil {
		t.Fatal("command failed before exercising cancellation")
	}
	if time.Since(started) > 3*time.Second {
		t.Fatal("cancellation exceeded bounded cleanup")
	}
	time.Sleep(5 * time.Second)
	if _, statErr := os.Stat(marker); !os.IsNotExist(statErr) {
		t.Fatal("grandchild survived cancellation")
	}
}

func TestHelperPrint(t *testing.T) {
	if os.Getenv("CODENERD_HELLO_HELPER") != "1" {
		return
	}
	fmt.Println("hello")
}

func TestRun_NormalExitLeavesNoError(t *testing.T) {
	cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestHelperPrint$")
	cmd.Env = append(os.Environ(), "CODENERD_HELLO_HELPER=1")
	out, err := CombinedOutput(cmd)
	if err != nil {
		t.Fatalf("CombinedOutput failed: %v", err)
	}
	if !strings.Contains(string(out), "hello") {
		t.Fatalf("expected output to contain %q, got %q", "hello", string(out))
	}
}

func TestRun_SelfReexecChildTerminatesOnDeadline(t *testing.T) {
	if os.Getenv("CODENERD_CANCEL_HELPER") == "1" {
		time.Sleep(30 * time.Second)
		return
	}
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestRun_SelfReexecChildTerminatesOnDeadline$")
	cmd.Env = append(os.Environ(), "CODENERD_CANCEL_HELPER=1")
	started := time.Now()
	if _, err := CombinedOutput(cmd); err == nil {
		t.Fatal("sleeping child escaped cancellation")
	}
	if ctx.Err() == nil {
		t.Fatal("child failed before exercising cancellation")
	}
	if time.Since(started) > 12*time.Second {
		t.Fatal("cancellation exceeded bounded cleanup")
	}
}

func TestCombinedOutput_RejectsPresetStdout(t *testing.T) {
	cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestHelperPrint$")
	cmd.Stdout = io.Discard
	_, err := CombinedOutput(cmd)
	if err == nil || !strings.Contains(err.Error(), "Stdout already set") {
		t.Fatalf("expected Stdout already set error, got %v", err)
	}
	if cmd.Process != nil {
		t.Fatal("process was started despite preset Stdout")
	}
}
