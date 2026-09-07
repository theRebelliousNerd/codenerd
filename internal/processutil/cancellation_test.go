package processutil

import (
	"context"
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestCancellableCommandTerminatesOnDeadline(t *testing.T) {
	if os.Getenv("CODENERD_CANCEL_HELPER") == "1" {
		time.Sleep(30 * time.Second)
		return
	}
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	cmd := Cancellable(exec.CommandContext(ctx, os.Args[0], "-test.run=^TestCancellableCommandTerminatesOnDeadline$"))
	cmd.Env = append(os.Environ(), "CODENERD_CANCEL_HELPER=1")
	started := time.Now()
	if _, err := cmd.CombinedOutput(); err == nil {
		t.Fatal("sleeping child escaped cancellation")
	}
	if ctx.Err() == nil {
		t.Fatal("child failed before exercising cancellation")
	}
	if time.Since(started) > 12*time.Second {
		t.Fatal("cancellation exceeded bounded cleanup")
	}
}
