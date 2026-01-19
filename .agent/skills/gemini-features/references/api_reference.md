# Gemini API Reference

## API Endpoint

```
Base URL: https://generativelanguage.googleapis.com/v1beta
Generate: POST /models/{model}:generateContent?key={API_KEY}
Stream:   POST /models/{model}:streamGenerateContent?alt=sse&key={API_KEY}
```

## Request Structure

```go
type GeminiRequest struct {
    Contents          []GeminiContent        `json:"contents"`
    SystemInstruction *GeminiContent         `json:"systemInstruction,omitempty"`
    GenerationConfig  GeminiGenerationConfig `json:"generationConfig,omitempty"`
    Tools             []GeminiTool           `json:"tools,omitempty"`
    ThoughtSignature  string                 `json:"thoughtSignature,omitempty"` // Multi-turn
}

type GeminiContent struct {
    Role  string       `json:"role,omitempty"` // "user", "model", "function"
    Parts []GeminiPart `json:"parts"`
}

type GeminiPart struct {
    Text             string                   `json:"text,omitempty"`
    FunctionCall     *GeminiFunctionCall      `json:"functionCall,omitempty"`
    FunctionResponse *GeminiFunctionResponse  `json:"functionResponse,omitempty"`
}

type GeminiFunctionCall struct {
    Name string                 `json:"name"`
    Args map[string]interface{} `json:"args"`
}

type GeminiFunctionResponse struct {
    Name     string                 `json:"name"`
    Response map[string]interface{} `json:"response"`
}
```

## Generation Config

```go
type GeminiGenerationConfig struct {
    Temperature      float64                `json:"temperature,omitempty"`      // Default 1.0 for Gemini 3
    MaxOutputTokens  int                    `json:"maxOutputTokens,omitempty"`  // Up to 65536
    ResponseMimeType string                 `json:"responseMimeType,omitempty"`
    ResponseSchema   map[string]interface{} `json:"responseJsonSchema,omitempty"`
    ThinkingConfig   *GeminiThinkingConfig  `json:"thinkingConfig,omitempty"`
}

type GeminiThinkingConfig struct {
    IncludeThoughts bool   `json:"includeThoughts,omitempty"` // Get thought summaries
    ThinkingLevel   string `json:"thinkingLevel,omitempty"`   // Gemini 3: minimal/low/medium/high
}
```

## Tool Definitions

```go
type GeminiTool struct {
    FunctionDeclarations []GeminiFunctionDeclaration `json:"functionDeclarations,omitempty"`
    GoogleSearch         *GeminiGoogleSearch         `json:"googleSearch,omitempty"`
    URLContext           *GeminiURLContext           `json:"urlContext,omitempty"`
}

type GeminiFunctionDeclaration struct {
    Name        string                 `json:"name"`
    Description string                 `json:"description"`
    Parameters  map[string]interface{} `json:"parameters,omitempty"` // JSON Schema
}

type GeminiGoogleSearch struct {} // Empty - presence enables it

type GeminiURLContext struct {
    URLs []string `json:"urls,omitempty"` // Max 20 URLs, 34MB each
}
```

## Response Structure

```go
type GeminiResponse struct {
    Candidates       []GeminiResponseCandidate `json:"candidates"`
    UsageMetadata    GeminiUsageMetadata       `json:"usageMetadata"`
    ThoughtSummary   string                    `json:"thoughtSummary,omitempty"`
    ThoughtSignature string                    `json:"thoughtSignature,omitempty"`
    Error            *GeminiError              `json:"error,omitempty"`
}

type GeminiResponseCandidate struct {
    Content struct {
        Parts []GeminiResponsePart `json:"parts"`
        Role  string               `json:"role"`
    } `json:"content"`
    FinishReason      string                   `json:"finishReason"`
    GroundingMetadata *GeminiGroundingMetadata `json:"groundingMetadata,omitempty"`
}

type GeminiResponsePart struct {
    Text         string              `json:"text,omitempty"`
    FunctionCall *GeminiFunctionCall `json:"functionCall,omitempty"`
}

type GeminiUsageMetadata struct {
    PromptTokenCount     int `json:"promptTokenCount"`
    CandidatesTokenCount int `json:"candidatesTokenCount"`
    TotalTokenCount      int `json:"totalTokenCount"`
    ThoughtsTokenCount   int `json:"thoughtsTokenCount,omitempty"` // Thinking tokens used
}
```

## Grounding Metadata

```go
type GeminiGroundingMetadata struct {
    GroundingChunks   []GeminiGroundingChunk   `json:"groundingChunks,omitempty"`
    GroundingSupports []GeminiGroundingSupport `json:"groundingSupports,omitempty"`
    WebSearchQueries  []string                 `json:"webSearchQueries,omitempty"`
    SearchEntryPoint  *GeminiSearchEntryPoint  `json:"searchEntryPoint,omitempty"`
}

type GeminiGroundingChunk struct {
    Web *struct {
        URI   string `json:"uri"`
        Title string `json:"title"`
    } `json:"web,omitempty"`
}

type GeminiGroundingSupport struct {
    Segment struct {
        StartIndex int    `json:"startIndex"`
        EndIndex   int    `json:"endIndex"`
        Text       string `json:"text"`
    } `json:"segment"`
    GroundingChunkIndices []int     `json:"groundingChunkIndices"`
    ConfidenceScores      []float64 `json:"confidenceScores"`
}
```

## Error Response

```go
type GeminiError struct {
    Code    int    `json:"code"`
    Message string `json:"message"`
    Status  string `json:"status"`
}
```

Common error codes:
- 400: Bad request (missing thought signature, invalid schema)
- 403: API key invalid or quota exceeded
- 429: Rate limit exceeded
- 500: Server error

## Model Specifications

| Model | Context | Output | Features |
|-------|---------|--------|----------|
| `gemini-3-pro-preview` | 1M | 64K | Full reasoning, all tools |
| `gemini-3-flash-preview` | 1M | 64K | Fast, minimal/low/medium/high thinking |
| `gemini-3-pro-image-preview` | 1M | 64K | Image generation, 4K |
