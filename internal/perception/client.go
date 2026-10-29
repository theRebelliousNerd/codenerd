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
// - client_xai.go         - xAI (Grok) API-key client implementation
// - xaioauth/             - SuperGrok OAuth client (Hermes-style, modular package)
// - client_openrouter.go  - OpenRouter multi-provider client implementation
// - client_factory.go     - Provider detection and client factory functions
// - claude_cli_client.go  - Claude Code CLI subprocess client
// - codex_cli_client.go   - OpenAI Codex CLI subprocess client
// - client_meta_responses.go - Meta Muse Spark client implementation
// - client_ollama.go      - Ollama local inference client implementation
// - client_openai_compat*.go - OpenAI-compatible API + grounding support
// - client_gemini_*.go    - Gemini streaming, tools, and file support
// - client_zai_*.go       - Z.AI retry and streaming support
// - client_tool_helpers.go - Shared tool-call extraction helpers
// - broker_install.go     - Metering decorator installation
// - tracing_client.go     - Trace-capturing decorator
// - transport.go          - Shared pooled HTTP transport
// - usage_track.go        - Usage metering vocabulary
// - truncation.go         - Output ceilings and truncation errors
// - scanner_pool.go       - Pooled SSE line buffers
