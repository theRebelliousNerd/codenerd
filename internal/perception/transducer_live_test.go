package perception

import (
	"testing"
)

// TestPiggybackProtocol_Live was testing the ZAI client which is now mothballed.
// All perception now uses Gemini via GeminiThinkingTransducer.
// See gemini_structured_test.go for Gemini-based live tests.
func TestPiggybackProtocol_Live(t *testing.T) {
	t.Skip("ZAI client is mothballed - all perception now uses Gemini")
}
