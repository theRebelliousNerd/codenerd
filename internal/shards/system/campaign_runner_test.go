package system

import (
	"context"
	"errors"
	"testing"
)

// TestCampaignRunner_RestartBackoff_EscalatesOnRunFailure pins the
// exponential backoff for repeated Run failures. Previously tick() reset
// restartBackoffSec to 5 on every Run error, causing a hot retry loop
// (299 attempts in 25m against the same risk gate). After the fix the
// backoff doubles on each Run error, capped at 300s, matching the
// LoadCampaign failure path.
func TestCampaignRunner_RestartBackoff_EscalatesOnRunFailure(t *testing.T) {
	ctx := context.Background()
	shard := NewCampaignRunnerShard()
	if shard.restartBackoffSec != 5 {
		t.Fatalf("initial restartBackoffSec = %d, want 5", shard.restartBackoffSec)
	}

	// Use empty workspace so tick() exercises only the orchestrator-completion
	// branch and returns early without trying to start a new campaign.
	shard.workspace = ""

	failure := errors.New("mandatory northstar safety review missing")
	wantSequence := []int{10, 20, 40, 80, 160, 300, 300}
	for i, want := range wantSequence {
		done := make(chan error, 1)
		done <- failure
		shard.mu.Lock()
		shard.activeOrchDone = done
		shard.activeCampaignID = "campaign-test"
		// activeOrch intentionally left nil; tick clears it anyway.
		shard.mu.Unlock()

		shard.tick(ctx)

		shard.mu.RLock()
		got := shard.restartBackoffSec
		lastAttempt := shard.lastStartAttempt
		shard.mu.RUnlock()

		if got != want {
			t.Fatalf("iteration %d: restartBackoffSec = %d, want %d", i+1, got, want)
		}
		if lastAttempt.IsZero() {
			t.Fatalf("iteration %d: lastStartAttempt should be set after Run failure", i+1)
		}
		// Verify activeOrchDone was consumed.
		shard.mu.RLock()
		if shard.activeOrchDone != nil {
			t.Fatalf("iteration %d: activeOrchDone should be nil after consumption", i+1)
		}
		if shard.activeCampaignID != "" {
			t.Fatalf("iteration %d: activeCampaignID should be cleared", i+1)
		}
		shard.mu.RUnlock()
	}
}

func TestCampaignRunner_RestartBackoff_CapsAt300(t *testing.T) {
	ctx := context.Background()
	shard := NewCampaignRunnerShard()
	shard.workspace = ""

	cases := []struct {
		name string
		init int
		want int
	}{
		{"160 doubles to 300 cap", 160, 300},
		{"200 doubles but clamped to 300", 200, 300},
		{"300 stays at cap", 300, 300},
		{"299 doubles but clamped", 299, 300},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			shard.mu.Lock()
			shard.restartBackoffSec = tc.init
			done := make(chan error, 1)
			done <- errors.New("boom")
			shard.activeOrchDone = done
			shard.activeCampaignID = "cap-test"
			shard.mu.Unlock()

			shard.tick(ctx)

			shard.mu.RLock()
			got := shard.restartBackoffSec
			shard.mu.RUnlock()
			if got != tc.want {
				t.Fatalf("restartBackoffSec = %d, want %d (from init %d)", got, tc.want, tc.init)
			}
		})
	}
}

func TestCampaignRunner_RestartBackoff_ResetsOnSuccess(t *testing.T) {
	ctx := context.Background()
	shard := NewCampaignRunnerShard()
	shard.workspace = ""

	// Prime backoff to a non-default value.
	shard.mu.Lock()
	shard.restartBackoffSec = 80
	shard.mu.Unlock()

	// Successful Run (nil error) should reset to 5.
	done := make(chan error, 1)
	done <- nil
	shard.mu.Lock()
	shard.activeOrchDone = done
	shard.activeCampaignID = "success-campaign"
	shard.mu.Unlock()

	shard.tick(ctx)

	shard.mu.RLock()
	if shard.restartBackoffSec != 5 {
		t.Fatalf("after success: restartBackoffSec = %d, want 5", shard.restartBackoffSec)
	}
	if !shard.lastStartAttempt.IsZero() {
		t.Fatalf("after success: lastStartAttempt should be zero, got %v", shard.lastStartAttempt)
	}
	shard.mu.RUnlock()
}

func TestCampaignRunner_RestartBackoff_ResetsOnPause(t *testing.T) {
	ctx := context.Background()
	shard := NewCampaignRunnerShard()
	shard.workspace = ""

	shard.mu.Lock()
	shard.restartBackoffSec = 40
	shard.mu.Unlock()

	done := make(chan error, 1)
	done <- context.Canceled
	shard.mu.Lock()
	shard.activeOrchDone = done
	shard.activeCampaignID = "paused-campaign"
	shard.mu.Unlock()

	shard.tick(ctx)

	shard.mu.RLock()
	if shard.restartBackoffSec != 5 {
		t.Fatalf("after context.Canceled: restartBackoffSec = %d, want 5", shard.restartBackoffSec)
	}
	if !shard.lastStartAttempt.IsZero() {
		t.Fatalf("after context.Canceled: lastStartAttempt should be zero, got %v", shard.lastStartAttempt)
	}
	shard.mu.RUnlock()
}
