//go:build unix

package main

import (
	"context"
	"errors"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/spf13/cobra"
)

// Ctrl+C on a campaign command cancels its context (the orchestrator pauses
// and persists) instead of killing the process. Not parallel: the signal goes
// to the whole test process.
func TestCampaignRunContextTurnsSIGINTIntoAPause(t *testing.T) {
	ctx, cancel := campaignRunContext(&cobra.Command{})
	defer cancel()
	if err := syscall.Kill(os.Getpid(), syscall.SIGINT); err != nil {
		t.Fatal(err)
	}
	select {
	case <-ctx.Done():
	case <-time.After(10 * time.Second):
		t.Fatal("SIGINT did not cancel the campaign context")
	}
	if !errors.Is(ctx.Err(), context.Canceled) {
		t.Fatalf("ctx.Err() = %v, want canceled", ctx.Err())
	}
}
