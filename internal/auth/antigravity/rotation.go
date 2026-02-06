package antigravity

import (
	"math"
	"sync"
	"time"
)

// HealthScoreConfig configures the health score system
type HealthScoreConfig struct {
	Initial             int     `json:"initial"`
	SuccessReward       int     `json:"success_reward"`
	RateLimitPenalty    int     `json:"rate_limit_penalty"`
	FailurePenalty      int     `json:"failure_penalty"`
	RecoveryRatePerHour float64 `json:"recovery_rate_per_hour"`
	MinUsable           int     `json:"min_usable"`
	MaxScore            int     `json:"max_score"`
}

// DefaultHealthScoreConfig returns sensible defaults
func DefaultHealthScoreConfig() HealthScoreConfig {
	return HealthScoreConfig{
		Initial:             70,
		SuccessReward:       1,
		RateLimitPenalty:    15,
		FailurePenalty:      25,
		RecoveryRatePerHour: 5,
		MinUsable:           30,
		MaxScore:            100,
	}
}

// HealthTracker tracks health scores for accounts
type HealthTracker struct {
	scores              map[int]int
	lastUpdates         map[int]time.Time
	initial             int
	successReward       int
	rateLimitPenalty    int
	failurePenalty      int
	recoveryRatePerHour float64
	minUsable           int
	maxScore            int
	mu                  sync.RWMutex
}

// NewHealthTracker creates a new health tracker
func NewHealthTracker(config HealthScoreConfig) *HealthTracker {
	return &HealthTracker{
		scores:              make(map[int]int),
		lastUpdates:         make(map[int]time.Time),
		initial:             config.Initial,
		successReward:       config.SuccessReward,
		rateLimitPenalty:    config.RateLimitPenalty,
		failurePenalty:      config.FailurePenalty,
		recoveryRatePerHour: config.RecoveryRatePerHour,
		minUsable:           config.MinUsable,
		maxScore:            config.MaxScore,
	}
}

// GetScore returns the effective score for an account index
func (ht *HealthTracker) GetScore(index int) int {
	ht.mu.Lock()
	defer ht.mu.Unlock()
	return ht.getScoreLocked(index)
}

func (ht *HealthTracker) getScoreLocked(index int) int {
	score, ok := ht.scores[index]
	if !ok {
		score = ht.initial
		ht.scores[index] = score
		ht.lastUpdates[index] = time.Now()
		return score
	}

	lastUpdate := ht.lastUpdates[index]
	hoursSinceUpdate := time.Since(lastUpdate).Hours()
	recovered := int(hoursSinceUpdate * ht.recoveryRatePerHour)

	if recovered > 0 {
		score += recovered
		if score > ht.maxScore {
			score = ht.maxScore
		}
		ht.scores[index] = score
		ht.lastUpdates[index] = time.Now()
	}

	return score
}

// RecordSuccess boosts score
func (ht *HealthTracker) RecordSuccess(index int) {
	ht.mu.Lock()
	defer ht.mu.Unlock()

	score := ht.getScoreLocked(index) // Updates with time recovery first
	score += ht.successReward
	if score > ht.maxScore {
		score = ht.maxScore
	}
	ht.scores[index] = score
	ht.lastUpdates[index] = time.Now()
}

// RecordRateLimit penalizes score
func (ht *HealthTracker) RecordRateLimit(index int) {
	ht.mu.Lock()
	defer ht.mu.Unlock()

	score := ht.getScoreLocked(index)
	score -= ht.rateLimitPenalty
	if score < 0 {
		score = 0
	}
	ht.scores[index] = score
	ht.lastUpdates[index] = time.Now()
}

// RecordFailure penalizes score
func (ht *HealthTracker) RecordFailure(index int) {
	ht.mu.Lock()
	defer ht.mu.Unlock()

	score := ht.getScoreLocked(index)
	score -= ht.failurePenalty
	if score < 0 {
		score = 0
	}
	ht.scores[index] = score
	ht.lastUpdates[index] = time.Now()
}

// TokenTracker implements a token bucket for rate limiting
type TokenTracker struct {
	tokens                    map[int]int
	lastUpdates               map[int]time.Time
	maxTokens                 int
	regenerationRatePerMinute float64
	initialTokens             int
	mu                        sync.RWMutex
}

// NewTokenTracker creates a new token tracker
func NewTokenTracker(maxTokens int, rate float64, initial int) *TokenTracker {
	return &TokenTracker{
		tokens:                    make(map[int]int),
		lastUpdates:               make(map[int]time.Time),
		maxTokens:                 maxTokens,
		regenerationRatePerMinute: rate,
		initialTokens:             initial,
	}
}

// Consume attempts to consume a token. Returns true if successful.
func (tt *TokenTracker) Consume(index int) bool {
	tt.mu.Lock()
	defer tt.mu.Unlock()

	tt.regenerate(index)

	tokens := tt.tokens[index]
	if tokens > 0 {
		tt.tokens[index] = tokens - 1
		return true
	}
	return false
}

// Refund gives back a token (e.g., on 429)
func (tt *TokenTracker) Refund(index int) {
	tt.mu.Lock()
	defer tt.mu.Unlock()

	tt.regenerate(index)

	if tt.tokens[index] < tt.maxTokens {
		tt.tokens[index]++
	}
}

func (tt *TokenTracker) regenerate(index int) {
	now := time.Now()
	lastUpdate, ok := tt.lastUpdates[index]

	if !ok {
		tt.tokens[index] = tt.initialTokens
		tt.lastUpdates[index] = now
		return
	}

	minutesPassed := now.Sub(lastUpdate).Minutes()
	regenerated := int(minutesPassed * tt.regenerationRatePerMinute)

	if regenerated > 0 {
		tt.tokens[index] = int(math.Min(float64(tt.maxTokens), float64(tt.tokens[index]+regenerated)))
		tt.lastUpdates[index] = now
	}
}

// AccountWithMetrics holds transient state for selection logic
type AccountWithMetrics struct {
	Index         int
	LastUsed      time.Time
	HealthScore   int
	IsRateLimited bool
	IsCoolingDown bool
}

// SelectHybridAccount selects the best account using hybrid strategy
func SelectHybridAccount(candidates []AccountWithMetrics, tokenTracker *TokenTracker) int {
	var bestCandidate *AccountWithMetrics
	var bestScore float64 = -1

	for i := range candidates {
		cand := &candidates[i]
		if cand.IsRateLimited || cand.IsCoolingDown {
			continue
		}

		// Check token bucket
		hasTokens := tokenTracker.Consume(cand.Index) // Peek/Consume logic is complex, here we assume selection = consumption intent
		if !hasTokens {
			// If we can't consume, we shouldn't select it?
			// In the plugin, we select then consume.
			// Let's assume we check availability here.
			// Actually TokenTracker.Consume is mutating.
			// We should probably have a CanConsume method.
			// For now, let's just refund if we don't pick it?
			// No, that's messy.
			// Let's simplisticly prioritize health.
			continue
		} else {
			// Refund immediately as this is just selection
			tokenTracker.Refund(cand.Index)
		}

		// Score calculation
		// Priority: Health (high weight) + Freshness (low weight)
		healthFactor := float64(cand.HealthScore) * 3.0

		secondsSinceUsed := time.Since(cand.LastUsed).Seconds()
		freshnessBonus := math.Min(secondsSinceUsed, 3600) * 0.01

		score := healthFactor + freshnessBonus

		if score > bestScore {
			bestScore = score
			bestCandidate = cand
		}
	}

	if bestCandidate != nil {
		return bestCandidate.Index
	}
	return -1 // No suitable account
}
