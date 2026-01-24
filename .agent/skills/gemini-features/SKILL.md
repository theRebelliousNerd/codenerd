---
name: gemini-features
description: |
  Master Google Gemini 3 API integration for codeNERD. This skill should be used when implementing Gemini-specific features including: thinking mode configuration (thinkingLevel), thought signatures for multi-turn function calling, Google Search grounding, URL Context tool, document processing, and structured output. Covers Go implementation patterns, API request/response formats, and integration with codeNERD's perception layer. Use when writing or debugging Gemini client code, configuring thinking modes, or implementing grounding tools.
---

# Gemini Features Skill

Comprehensive guide for Google Gemini 3 API integration in codeNERD's Go codebase.

## Quick Reference

| Feature | Gemini 3 | codeNERD Location |
|---------|----------|-------------------|
| Thinking Control | `thinkingLevel` (minimal/low/medium/high) | `client_gemini.go` |
| Context Window | 1M tokens | Config |
| Output Limit | 64K tokens | Config |
| Thought Signatures | Required for function calling | `CompleteWithToolResults` |
| Google Search | `{"googleSearch": {}}` | `buildBuiltInTools()` |
| URL Context | Max 20 URLs, 34MB each | `SetURLContextURLs()` |

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
7. **Structured Output + Grounding** - Gemini 3 DOES support combining structured output with Google Search/URL Context
8. **Thinking Mode + Structured Output** - When `includeThoughts: true`, response parts may have a `thought: true` boolean indicating thinking content. **MUST filter out thought parts** before parsing JSON:

   ```go
   for _, part := range resp.Candidates[0].Content.Parts {
       if part.Thought {
           // This is thinking content, not the actual response
           continue
       }
       result.WriteString(part.Text) // Actual JSON response
   }
   ```

## CRITICAL: Go Embedding Pitfall for Transducers

When implementing specialized transducers like `GeminiThinkingTransducer` that embed a base type, Go does **NOT** provide virtual method dispatch.

### The Problem

```go
// GeminiThinkingTransducer embeds *UnderstandingTransducer
type GeminiThinkingTransducer struct {
    *UnderstandingTransducer
}

// Override ParseIntentWithContext to handle thinking mode
func (t *GeminiThinkingTransducer) ParseIntentWithContext(...) (Intent, error) {
    // Custom Gemini thinking handling
}
```

If the CLI calls `ParseIntent` (not `ParseIntentWithContext`), Go uses the **embedded type's** `ParseIntent` method:

```go
// UnderstandingTransducer.ParseIntent (inherited)
func (t *UnderstandingTransducer) ParseIntent(ctx context.Context, input string) (Intent, error) {
    return t.ParseIntentWithContext(ctx, input, nil) // 't' is *UnderstandingTransducer!
}
```

**Result**: The override is completely bypassed. `UnderstandingTransducer.ParseIntentWithContext` is called instead of `GeminiThinkingTransducer.ParseIntentWithContext`.

### The Fix

The outer type MUST override ALL methods that delegate to overridden methods:

```go
// ParseIntent overrides the base implementation to ensure our ParseIntentWithContext is called.
// This is required because Go struct embedding doesn't provide virtual dispatch.
func (t *GeminiThinkingTransducer) ParseIntent(ctx context.Context, input string) (Intent, error) {
    return t.ParseIntentWithContext(ctx, input, nil)
}
```

### Interface Pass-Through Chain

Wrapper clients must expose all Gemini-specific interfaces:

| Interface | Methods | Purpose |
|-----------|---------|---------|
| `ThinkingProvider` | `IsThinkingEnabled()`, `GetThinkingLevel()`, `GetLastThoughtSummary()`, `GetLastThinkingTokens()` | Transducer selection |
| `GroundingProvider` | `GetLastGroundingSources()`, `IsGoogleSearchEnabled()`, `IsURLContextEnabled()` | Grounding tools |
| `ThoughtSignatureProvider` | `GetLastThoughtSignature()`, `SetLastThoughtSignature()` | Multi-turn function calling |
| `CacheProvider` | `CreateCache()`, `GetCache()`, `DeleteCache()`, `ListCaches()` | Context caching |
| `FileProvider` | `UploadFile()`, `GetFile()`, `DeleteFile()`, `ListFiles()` | Files API |
| `schemaCapableClient` | `SchemaCapable()`, `CompleteWithSchema()` | Structured output |

**Both** `TracingLLMClient` and `ScheduledLLMCall` must pass through each interface.

## Ecosystem Integration (Complete)

Grounding sources are fully wired into codeNERD's ecosystem:

### Integration Points

| Layer | File | How Grounding Is Used |
|-------|------|----------------------|
| **GeminiClient** | `client_gemini.go` | Captures sources in `lastGroundingSources` |
| **GroundingProvider** | `types/interfaces.go` | Interface for grounding-capable clients |
| **Articulation** | `chat/helpers.go` | Extracts sources via type assertion |
| **Main Response** | `chat/process.go` | Appends "**Sources:**" section |
| **Dream State** | `chat/process_dream.go` | Shard interpretation includes sources |
| **Logging** | `perception/client_gemini.go` | `grounding_sources=N` in logs |

### How It Works

```
User Query
    │
    ▼
Perception (ParseIntent)
    │
    ▼
Kernel Processing (Derive next_action)
    │
    ▼
articulateWithConversation()
    │ ◄─── GeminiClient.CompleteWithSystem()
    │      └─► Captures grounding sources
    │
    ▼
GroundingProvider type assertion
    │
    ▼
Sources appended to response
    │
    ▼
User sees: "Answer...\n\n**Sources:**\n- url1\n- url2"
```

### GroundingProvider Interface

```go
// types/interfaces.go
type GroundingProvider interface {
    GetLastGroundingSources() []string
    IsGoogleSearchEnabled() bool
    IsURLContextEnabled() bool
}

// Usage in any component
if gp, ok := client.(types.GroundingProvider); ok {
    sources := gp.GetLastGroundingSources()
    if len(sources) > 0 {
        response += "\n\n**Sources:**\n"
        for _, src := range sources {
            response += fmt.Sprintf("- %s\n", src)
        }
    }
}
```

### Runtime Methods

```go
// Enable/disable at runtime
client.SetEnableGoogleSearch(true)
client.SetEnableURLContext(true)
client.SetURLContextURLs([]string{"https://docs.example.com"})

// Check status
if client.IsGoogleSearchEnabled() { ... }

// Get last sources (after CompleteWithSystem)
sources := client.GetLastGroundingSources()
```

## Reasoning Traces for Self-Improvement (SPL Integration)

Gemini's Thinking Mode produces reasoning traces that feed into codeNERD's System Prompt Learning (SPL) system. This enables automatic improvement of prompt atoms based on understanding WHY tasks succeeded or failed.

### ThinkingProvider Interface

```go
// types/interfaces.go
type ThinkingProvider interface {
    GetLastThoughtSummary() string  // Model's reasoning process
    GetLastThinkingTokens() int     // Tokens used for reasoning
    IsThinkingEnabled() bool
    GetThinkingLevel() string       // "minimal", "low", "medium", "high"
}

// Usage: Check if client supports thinking metadata
if tp, ok := client.(types.ThinkingProvider); ok {
    summary := tp.GetLastThoughtSummary()
    tokens := tp.GetLastThinkingTokens()
}
```

### How Reasoning Traces Flow into SPL

```
LLM Call (Gemini with Thinking Mode)
    │
    ▼
CompleteWithSystem() / CompleteWithTools()
    │ ◄─── Captures ThoughtSummary, ThinkingTokens
    │
    ▼
recordShardExecution() [delegation.go]
    │ ◄─── Extracts via ThinkingProvider interface
    │
    ▼
ExecutionRecord populated with:
    - ThoughtSummary (model's reasoning)
    - ThinkingTokens (budget used)
    - GroundingSources (if grounding enabled)
    │
    ▼
FeedbackCollector.Record() [prompt_evolution]
    │
    ▼
TaskJudge.Evaluate() [LLM-as-Judge]
    │ ◄─── Uses ThoughtSummary in evaluation prompt
    │      (see judge.go:168-178)
    │
    ▼
JudgeVerdict with:
    - Category (LOGIC_ERROR, HALLUCINATION, etc.)
    - ImprovementRule ("When X, always Y")
    │
    ▼
Evolver generates new prompt atoms
```

### ExecutionRecord Fields

```go
// prompt_evolution/types.go
type ExecutionRecord struct {
    // ... task details ...

    // LLM Reasoning Metadata (Gemini 3+ Thinking Mode)
    ThoughtSummary   string   // Model's reasoning process summary
    ThinkingTokens   int      // Tokens used for reasoning
    GroundingSources []string // URLs used to ground the response
}
```

### Why This Matters

1. **Reasoning Quality Assessment**: The LLM-as-Judge can evaluate not just WHAT the model produced, but WHY it made certain decisions

2. **Error Root Cause**: When tasks fail, the ThoughtSummary reveals flawed reasoning patterns that can be corrected in future prompts

3. **Budget Monitoring**: ThinkingTokens helps track reasoning overhead and optimize thinking levels

4. **Grounding Transparency**: GroundingSources shows which external information influenced decisions

### Configuration

Enable thinking mode in `.nerd/config.json`:

```json
{
  "provider": "gemini",
  "model": "gemini-3-flash-preview",
  "gemini": {
    "enable_thinking": true,
    "thinking_level": "high"
  }
}
```

## Testing

### Build

```powershell
# Windows
$env:CGO_CFLAGS="-IC:/CodeProjects/codeNERD/sqlite_headers"
go build ./cmd/nerd

# Linux/WSL
export CGO_CFLAGS="-I/mnt/c/CodeProjects/codeNERD/sqlite_headers"
go build -tags=sqlite_vec -o nerd.exe ./cmd/nerd
```

### Unit Tests

```bash
# Test Gemini client
go test ./internal/perception/... -run Gemini
```

### Live Perception Tests

```powershell
# Clear logs first
rm .nerd/logs/*

# Test basic perception - should show Confidence > 0
go run ./cmd/nerd perception "hello world" -v

# Verify GeminiThinkingTransducer is selected
Select-String -Path ".nerd\logs\*_perception.log" -Pattern "GeminiThinkingTransducer"

# Expected output:
# 📊 Perception Results:
#    Category:   /query
#    Verb:       /greet
#    Confidence: 1.00  <-- NOT 0.00!

# Test code generation intent
go run ./cmd/nerd perception "write a function to sort a list" -v

# Expected:
#    Category:   /mutation
#    Verb:       /create
#    Shard: /coder
#    Confidence: >= 0.80
```

### Verify Grounding

```powershell
# Check grounding sources in logs
Select-String -Path ".nerd\logs\*_perception.log" -Pattern "grounding_sources"

# Check API logs for thinking tokens
Select-String -Path ".nerd\logs\*_api.log" -Pattern "thinking_tokens"
```

### Stress Testing

See the [stress-tester skill](..\..\stress-tester\SKILL.md) workflow:
`references/workflows/02-perception-articulation/gemini-transducer.md`
