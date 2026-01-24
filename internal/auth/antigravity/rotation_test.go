package antigravity

import (
	"testing"
	"time"
)

func TestHealthTracker(t *testing.T) {
	cfg := DefaultHealthScoreConfig()
	cfg.Initial = 50
	cfg.RecoveryRatePerHour = 10
	
	ht := NewHealthTracker(cfg)

	// Initial score
	if score := ht.GetScore(0); score != 50 {
		t.Errorf("Expected initial 50, got %d", score)
	}

	// Record Success
	ht.RecordSuccess(0)
	if score := ht.GetScore(0); score != 51 {
		t.Errorf("Expected 51 after success, got %d", score)
	}

	// Record Failure
	ht.RecordFailure(0)
	// 51 - 25 = 26
	if score := ht.GetScore(0); score != 26 {
		t.Errorf("Expected 26 after failure, got %d", score)
	}

	// Record Rate Limit
	ht.RecordRateLimit(0)
	// 26 - 15 = 11
	if score := ht.GetScore(0); score != 11 {
		t.Errorf("Expected 11 after rate limit, got %d", score)
	}
}

func TestTokenTracker(t *testing.T) {
	// 5 tokens max, 60 per minute (1 per second)
	tt := NewTokenTracker(5, 60.0, 5)

	// Consume all
	for i := 0; i < 5; i++ {
		if !tt.Consume(0) {
			t.Errorf("Failed to consume token %d", i)
		}
	}

	// Should be empty
	if tt.Consume(0) {
		t.Error("Should be exhausted")
	}

	// Refund one
	tt.Refund(0)
	if !tt.Consume(0) {
		t.Error("Should be able to consume after refund")
	}
}

func TestSelectHybridAccount(t *testing.T) {
	tt := NewTokenTracker(10, 10, 10)
	
	candidates := []AccountWithMetrics{
		{Index: 0, HealthScore: 10, LastUsed: time.Now()},                 // Poor health
		{Index: 1, HealthScore: 90, LastUsed: time.Now()},                 // Good health
		{Index: 2, HealthScore: 100, LastUsed: time.Now(), IsRateLimited: true}, // Rate limited
	}

	selected := SelectHybridAccount(candidates, tt)
	if selected != 1 {
		t.Errorf("Expected account 1 (healthy), got %d", selected)
	}
}
