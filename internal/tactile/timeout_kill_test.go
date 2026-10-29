package tactile

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// TestDirectExecutor_TimeoutKillsGrandchildren pins the process-group kill:
// commands run wrapped in a shell, so the real work is a grandchild of the
// spawned process. Killing only the shell orphans the grandchild, which
// keeps the output pipes open and blocks Wait() until it exits on its own —
// a 2s timeout on `sleep 10` used to return after the full 10s.
func TestDirectExecutor_TimeoutKillsGrandchildren(t *testing.T) {
	ex := NewDirectExecutor()
	start := time.Now()
	res, err := ex.Execute(context.Background(), Command{
		Binary:    "bash",
		Arguments: []string{"-c", `sh -c "sleep 10"`},
		Limits:    &ResourceLimits{TimeoutMs: 2000},
	})
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !res.Killed {
		t.Errorf("expected Killed=true after timeout, got %+v", res)
	}
	if res.KillReason == "" {
		t.Error("expected KillReason to name the timeout")
	}
	// The sleep lasts 10s: returning sooner proves the timeout killed the
	// whole tree rather than the command completing on its own.
	if elapsed >= 9*time.Second {
		t.Errorf("timeout did not kill the process tree: took %v", elapsed)
	}
}

// TestDirectExecutor_CancelKillsGrandchildren pins the same group-kill path
// for context cancellation rather than the limits timeout.
func TestDirectExecutor_CancelKillsGrandchildren(t *testing.T) {
	ex := NewDirectExecutor()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan *ExecutionResult, 1)
	go func() {
		res, err := ex.Execute(ctx, Command{
			Binary:    "bash",
			Arguments: []string{"-c", `sh -c "sleep 10"`},
		})
		if err != nil {
			t.Errorf("Execute returned error: %v", err)
			done <- nil
			return
		}
		done <- res
	}()
	time.Sleep(500 * time.Millisecond)
	start := time.Now()
	cancel()
	select {
	case res := <-done:
		if time.Since(start) >= 9*time.Second {
			t.Errorf("cancel did not kill the process tree promptly")
		}
		if res == nil {
			t.Fatal("Execute failed")
		}
		if !res.Killed {
			t.Errorf("expected Killed=true after cancel, got %+v", res)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("Execute did not return after cancel")
	}
}

// TestDirectExecutor_TimeoutLeavesNoSurvivor pins that no descendant survives
// the timeout: the survivor would write a marker file after Execute returns.
// The inner exec is the MSYS shape taskkill /T cannot reach.
func TestDirectExecutor_TimeoutLeavesNoSurvivor(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available")
	}
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not available")
	}
	marker := filepath.Join(t.TempDir(), "alive")
	ex := NewDirectExecutor()
	start := time.Now()
	res, err := ex.Execute(context.Background(), Command{
		Binary:    "bash",
		Arguments: []string{"-c", `sh -c "exec sh -c 'sleep 4; echo alive > ` + filepath.ToSlash(marker) + `'"`},
		Limits:    &ResourceLimits{TimeoutMs: 1000},
	})
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !res.Killed {
		t.Errorf("expected Killed=true after timeout, got %+v", res)
	}
	if elapsed >= 3*time.Second {
		t.Errorf("timeout did not kill the process tree promptly: took %v", elapsed)
	}
	time.Sleep(5 * time.Second)
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Errorf("survivor wrote marker file after timeout (err=%v)", err)
	}
}
