package perception

// This file contains only imports and any shared utility functions for the LLM client system.
// The actual client implementations have been modularized into separate files:
//
// - client_types.go       - Type definitions, interfaces, and request/response structures
// - client_schema.go      - Piggyback envelope schema builder
// - client_zai.go         - Z.AI client implementation
// - client_anthropic.go   - Anthropic client implementation
// - client_openai.go      - OpenAI client implementation
// - client_gemini.go      - Google Gemini client implementation
// - client_xai.go         - xAI (Grok) client implementation
// - client_openrouter.go  - OpenRouter multi-provider client implementation
// - client_factory.go     - Provider detection and client factory functions
// - claude_cli_client.go  - Claude Code CLI subprocess client
// - codex_cli_client.go   - OpenAI Codex CLI subprocess client

// min returns the minimum of two integers (Go stdlib doesn't have generic min before 1.21)
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
