package perception

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// QuotaExhaustedError reports an exhausted rate-limit quota with a known reset.
type QuotaExhaustedError struct {
	ResetAt time.Time
	Source  string
	Message string
}

// Error returns a human-readable quota exhaustion message.
func (e *QuotaExhaustedError) Error() string {
	msg := strings.TrimSpace(e.Message)
	reset := e.ResetAt.UTC().Format(time.RFC3339)
	if e.Source == "" {
		return fmt.Sprintf("rate limit quota exhausted until %s: %s", reset, msg)
	}
	return fmt.Sprintf("rate limit quota exhausted until %s (%s): %s", reset, e.Source, msg)
}

// quotaHeaderValues reads the rate-limit pair from real response headers.
func quotaHeaderValues(header http.Header) (string, string) {
	if header == nil {
		return "", ""
	}
	return header.Get("X-RateLimit-Remaining"), header.Get("X-RateLimit-Reset")
}

// lookupBodyHeader finds a header value in the body metadata map case-insensitively.
func lookupBodyHeader(m map[string]string, key string) string {
	for k, v := range m {
		if strings.EqualFold(k, key) {
			return v
		}
	}
	return ""
}

// quotaBodyValues decodes the provider error body into its quota fields.
// A body that does not decode yields empty values so headers alone may suffice.
func quotaBodyValues(body []byte) (remaining, reset, message, source string) {
	if len(body) == 0 {
		return "", "", "", ""
	}
	var payload struct {
		Error struct {
			Message  string `json:"message"`
			Metadata struct {
				Headers     map[string]string `json:"headers"`
				LimitSource string            `json:"limit_source"`
			} `json:"metadata"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", "", "", ""
	}
	remaining = lookupBodyHeader(payload.Error.Metadata.Headers, "X-RateLimit-Remaining")
	reset = lookupBodyHeader(payload.Error.Metadata.Headers, "X-RateLimit-Reset")
	return remaining, reset, payload.Error.Message, payload.Error.Metadata.LimitSource
}

// parseQuotaRemaining reports whether the remaining value means exhausted (exactly 0).
func parseQuotaRemaining(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return false
	}
	return v == 0
}

// parseQuotaReset parses the reset value: >= 1e12 epoch millis, >= 1e9 epoch seconds.
func parseQuotaReset(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return time.Time{}, false
	}
	if v >= 1000000000000 {
		return time.UnixMilli(v), true
	}
	if v >= 1000000000 {
		return time.Unix(v, 0), true
	}
	return time.Time{}, false
}

// quotaExhaustion detects an exhausted quota: remaining is exactly 0 and reset is after now.
func quotaExhaustion(header http.Header, body []byte, now time.Time) (*QuotaExhaustedError, bool) {
	hRemaining, hReset := quotaHeaderValues(header)
	bRemaining, bReset, message, source := quotaBodyValues(body)
	remainingStr := hRemaining
	if remainingStr == "" {
		remainingStr = bRemaining
	}
	resetStr := hReset
	if resetStr == "" {
		resetStr = bReset
	}
	if !parseQuotaRemaining(remainingStr) {
		return nil, false
	}
	reset, ok := parseQuotaReset(resetStr)
	if !ok {
		return nil, false
	}
	if !reset.After(now) {
		return nil, false
	}
	return &QuotaExhaustedError{
		ResetAt: reset,
		Source:  source,
		Message: strings.TrimSpace(message),
	}, true
}
