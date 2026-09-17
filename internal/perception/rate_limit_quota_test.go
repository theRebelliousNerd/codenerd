package perception

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

func quotaTestNow() time.Time {
	return time.UnixMilli(1789600000000)
}

func quotaTestReset() time.Time {
	return time.UnixMilli(1789689600000)
}

func liveQuotaBody() []byte {
	return []byte(`{"error":{"message":"Rate limit exceeded: free-models-per-day-stealth. ","code":429,"metadata":{"headers":{"X-RateLimit-Limit":"1000","X-RateLimit-Remaining":"0","X-RateLimit-Reset":"1789689600000"},"limit_source":"openrouter_free_tier_daily","remedy_hint":"Wait for the daily reset (see X-RateLimit-Reset), or purchase credits to raise your free-model daily limit.","provider_name":null}},"user_id":"user_x"}`)
}

func quotaHeader(remaining, reset string) http.Header {
	h := http.Header{}
	h.Set("X-RateLimit-Remaining", remaining)
	h.Set("X-RateLimit-Reset", reset)
	return h
}

func checkExhausted(t *testing.T, got *QuotaExhaustedError, ok bool) {
	t.Helper()
	if !ok || got == nil {
		t.Fatalf("quotaExhaustion() = %v, %v; want exhausted", got, ok)
	}
	if !got.ResetAt.Equal(quotaTestReset()) {
		t.Fatalf("ResetAt = %v, want %v", got.ResetAt, quotaTestReset())
	}
}

func checkNotExhausted(t *testing.T, got *QuotaExhaustedError, ok bool) {
	t.Helper()
	if ok || got != nil {
		t.Fatalf("quotaExhaustion() = %v, %v; want not exhausted", got, ok)
	}
}

func TestQuotaExhaustionLiveBody(t *testing.T) {
	got, ok := quotaExhaustion(nil, liveQuotaBody(), quotaTestNow())
	checkExhausted(t, got, ok)
	if got.Source != "openrouter_free_tier_daily" {
		t.Fatalf("Source = %q, want %q", got.Source, "openrouter_free_tier_daily")
	}
}

func TestQuotaExhaustionRealHeaders(t *testing.T) {
	got, ok := quotaExhaustion(quotaHeader("0", "1789689600000"), []byte{}, quotaTestNow())
	checkExhausted(t, got, ok)
}

func TestQuotaExhaustionRemainingNonzero(t *testing.T) {
	got, ok := quotaExhaustion(quotaHeader("3", "1789689600000"), []byte{}, quotaTestNow())
	checkNotExhausted(t, got, ok)
}

func TestQuotaExhaustionResetPast(t *testing.T) {
	got, ok := quotaExhaustion(quotaHeader("0", "1789599999000"), []byte{}, quotaTestNow())
	checkNotExhausted(t, got, ok)
}

func TestQuotaExhaustionResetUnparseable(t *testing.T) {
	got, ok := quotaExhaustion(quotaHeader("0", "abc"), []byte{}, quotaTestNow())
	checkNotExhausted(t, got, ok)
}

func TestQuotaExhaustionEpochSeconds(t *testing.T) {
	got, ok := quotaExhaustion(quotaHeader("0", "1789689600"), []byte{}, quotaTestNow())
	checkExhausted(t, got, ok)
}

func TestQuotaExhaustionErrorText(t *testing.T) {
	got, ok := quotaExhaustion(nil, liveQuotaBody(), quotaTestNow())
	checkExhausted(t, got, ok)
	msg := got.Error()
	for _, want := range []string{"2026-09-18T00:00:00Z", "openrouter_free_tier_daily", "free-models-per-day-stealth"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("Error() = %q, want it to contain %q", msg, want)
		}
	}
}
