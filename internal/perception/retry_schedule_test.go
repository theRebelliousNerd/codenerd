package perception

import (
	"testing"
	"time"

	"codenerd/internal/config"
)

// fastRetries runs the rest of a test under a 1ms retry backoff capped at
// 10ms. Under the production schedule (1s doubling to a 30s cap, and the cap
// itself for a 429 without Retry-After) each retry test slept through real
// seconds: TestExecuteOpenAIRequest_RetriesTransient alone waited 31s for one
// 500 and one 429. Those tests assert which attempts are made and what the
// caller sees, not the length of the wait. A test whose assertion is about
// the wait (a cancel that must land inside a backoff, a backoff that must be
// honored) sets the schedule it means with withRetryBackoff.
//
// The schedule is the process-wide config.LLMTimeouts, which every
// config.LoadUserConfig reinstalls, so it is set per test rather than once
// for the package.
func fastRetries(tb testing.TB) {
	tb.Helper()
	withRetryBackoff(tb, time.Millisecond, 10*time.Millisecond)
}

// withRetryBackoff runs the rest of a test under base/max retry delays and
// restores the previous schedule after it.
func withRetryBackoff(tb testing.TB, base, max time.Duration) {
	tb.Helper()
	prev := config.GetLLMTimeouts()
	next := prev
	next.RetryBackoffBase = base
	next.RetryBackoffMax = max
	config.SetLLMTimeouts(next)
	tb.Cleanup(func() { config.SetLLMTimeouts(prev) })
}
