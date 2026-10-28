package broker

import "net/http"

// Test seams: conveniences only tests call. They live here, not in
// production files, so the dead-code budget measures production code.

// NewAnthropicCounter builds a counter against the given credentials. The
// fallback is used when the endpoint is unreachable, slow, or unauthorized, and
// its lower Confidence propagates to the caller rather than being hidden.
func NewAnthropicCounter(apiKey, baseURL string, httpClient *http.Client, fallback *EstimatingCounter) *AnthropicCounter {
	return NewAnthropicCounterWithWorkspace(apiKey, baseURL, "", httpClient, fallback)
}
