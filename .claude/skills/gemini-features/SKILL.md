---
name: gemini-features
description: |
  Master Google Gemini 3 API integration for codeNERD. This skill should be used when implementing Gemini-specific features including: thinking mode configuration (thinkingLevel vs thinkingBudget), thought signatures for multi-turn function calling, Google Search grounding, URL Context tool, document processing, and structured output. Covers Go implementation patterns, API request/response formats, and integration with codeNERD's perception layer. Use when writing or debugging Gemini client code, configuring thinking modes, or implementing grounding tools.
---

# Gemini Features Skill

Comprehensive guide for Google Gemini 3 API integration in codeNERD's Go codebase.

## Quick Reference

| Feature | Gemini 3 | Gemini 2.5 | codeNERD Location |
|---------|----------|------------|-------------------|
| Thinking Control | `thinkingLevel` (minimal/low/medium/high) | `thinkingBudget` (token count) | `client_gemini.go` |
| Context Window | 1M tokens | 1M tokens | Config |
| Output Limit | 64K tokens | 8K tokens | Config |
| Thought Signatures | Required for function calling | Optional | `CompleteWithToolResults` |
| Google Search | `{"google_search": {}}` | Same | `buildBuiltInTools()` |
| URL Context | Max 20 URLs, 34MB each | Same | `SetURLContextURLs()` |

## Models

```
gemini-3-pro-preview       - Complex reasoning, 1M context, 64K output
gemini-3-flash-preview     - Pro-level at Flash speed, 1M context, 64K output
gemini-3-pro-image-preview - Image generation, 4K resolution
```

## Core Implementation Patterns

### Thinking Mode Configuration

For Gemini 3, use `thinkingLevel` (lowercase values):

```go
// internal/perception/client_gemini.go
func (c *GeminiClient) buildThinkingConfig() *GeminiThinkingConfig {
    if !c.enableThinking {
        return nil
    }
    return &GeminiThinkingConfig{
        IncludeThoughts: true,
        ThinkingLevel:   c.thinkingLevel, // "minimal", "low", "medium", "high"
    }
}
```

**Level Semantics:**
- `minimal` - Minimal reasoning (Flash only)
- `low` - Light reasoning
- `medium` - Moderate reasoning (Flash only)
- `high` - Maximum reasoning depth (default for codeNERD)

For Gemini 2.5 compatibility, use `thinkingBudget` (token count 128-32768).

### Thought Signatures (CRITICAL for Multi-Turn)

Thought signatures are **mandatory** for Gemini 3 function calling. Missing signatures produce 400 errors.

**Key Rules:**
1. Signatures appear in **function call parts**, not just response level
2. For sequential calls, EACH function call needs its own signature
3. For parallel calls, only the FIRST includes a signature
4. Must return signatures in IDENTICAL positions when sending tool results

```go
// Response structure with thought signatures
type GeminiPart struct {
    Text             string                   `json:"text,omitempty"`
    FunctionCall     *GeminiFunctionCall      `json:"functionCall,omitempty"`
    FunctionResponse *GeminiFunctionResponse  `json:"functionResponse,omitempty"`
    // Note: thoughtSignature appears IN the function call, not separate
}

// Capturing thought signature from response
if geminiResp.ThoughtSignature != "" {
    c.lastThoughtSignature = geminiResp.ThoughtSignature
}

// Passing signature in multi-turn request
reqBody := GeminiRequest{
    Contents:         allContents,
    ThoughtSignature: c.lastThoughtSignature, // Pass back for continuity
}
```

### Google Search Grounding

Enable real-time web search to ground responses:

```go
// Add to tools array
tools = append(tools, GeminiTool{
    GoogleSearch: &GeminiGoogleSearch{}, // Empty struct enables it
})
```

**Response metadata:**
```go
type GeminiGroundingMetadata struct {
    GroundingChunks   []GeminiGroundingChunk   // Web sources
    WebSearchQueries  []string                  // Queries executed
    GroundingSupports []GeminiGroundingSupport  // Text segments with sources
}

// Extract sources for LLMToolResponse
for _, chunk := range gm.GroundingChunks {
    if chunk.Web != nil && chunk.Web.URI != "" {
        result.GroundingSources = append(result.GroundingSources, chunk.Web.URI)
    }
}
```

### URL Context Tool

Ground responses with specific URLs (documentation, repos, etc.):

```go
// Configure URLs (max 20, 34MB each)
client.SetURLContextURLs([]string{
    "https://example.com/api-docs",
    "https://github.com/org/repo/blob/main/README.md",
})

// Add to tools
if c.enableURLContext && len(c.urlContextURLs) > 0 {
    urls := c.urlContextURLs
    if len(urls) > 20 {
        urls = urls[:20] // API limit
    }
    tools = append(tools, GeminiTool{
        URLContext: &GeminiURLContext{URLs: urls},
    })
}
```

**Limitations:**
- No paywalled content
- No YouTube videos or Google Workspace files
- Content counts toward input tokens
- Two-step retrieval: cache first, then live fetch

## Decision: Deep Research vs Built-in Tools

**Deep Research Agent** (`/interactions` endpoint):
- Async, up to 60 minutes execution
- Cannot use codeNERD tools (custom function calling unsupported)
- Different API model (interactions vs generateContent)
- Best for: standalone research reports, not interactive use

**Recommendation for codeNERD:** Use Google Search + URL Context instead:
- Synchronous, fits into existing flow
- Works with codeNERD's tool ecosystem
- Grounding metadata for citations
- ResearcherShard can leverage both tools dynamically

## Wiring into codeNERD Shards

### ResearcherShard Integration

```go
// When Gemini is the provider, enable research tools
if isGeminiProvider(client) {
    geminiClient := client.(*GeminiClient)

    // For documentation research, set URLs
    geminiClient.SetURLContextURLs(docURLs)

    // Response will include grounding sources
    resp, _ := geminiClient.CompleteWithTools(ctx, system, user, tools)

    // Extract sources for citation
    if len(resp.GroundingSources) > 0 {
        // Add to session context for transparency
        sessionCtx.GroundingSources = resp.GroundingSources
    }
}
```

### Config Integration

User configuration in `.nerd/config.json`:

```json
{
  "provider": "gemini",
  "gemini_api_key": "...",
  "model": "gemini-3-flash-preview",
  "gemini": {
    "enable_thinking": true,
    "thinking_level": "high",
    "enable_google_search": true,
    "enable_url_context": true
  }
}
```

Loaded via `config.GeminiProviderConfig` and wired to client factory.

## API Request/Response Formats

See reference files for complete API structures:
- `references/api_reference.md` - Request/response types
- `references/thinking-mode.md` - Thinking configuration details
- `references/grounding-tools.md` - Google Search and URL Context
- `references/thought-signatures.md` - Multi-turn function calling
- `references/document-processing.md` - PDF and file handling

## Common Pitfalls

1. **Thinking Level Case** - Must be lowercase: `"high"` not `"High"`
2. **Thought Signature Missing** - 400 error in Gemini 3 function calling
3. **URL Context Limits** - Max 20 URLs, 34MB each
4. **Parallel Function Calls** - Only first call gets signature
5. **Temperature Default** - Gemini 3 defaults to 1.0; remove explicit temperature to avoid looping
6. **Built-in Tool Exclusivity** - Cannot combine built-in tools with function calling (yet)

## Testing

```bash
# Build with Gemini support
export CGO_CFLAGS="-I/mnt/c/CodeProjects/codeNERD/sqlite_headers"
go build -tags=sqlite_vec -o nerd.exe ./cmd/nerd

# Test Gemini client
go test ./internal/perception/... -run Gemini
```
