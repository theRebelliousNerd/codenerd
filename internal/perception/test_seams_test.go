package perception

// Test seams: conveniences only tests call. They live here, not in
// production files, so the dead-code budget measures production code.

// NewZAIClient creates a new ZAI client with default config.
func NewZAIClient(apiKey string) *ZAIClient {
	config := DefaultZAIConfig(apiKey)
	return NewZAIClientWithConfig(config)
}
