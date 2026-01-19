# Thought Signatures Reference

## Overview

Thought signatures are **encrypted reasoning context** that must be passed back to Gemini 3 in multi-turn function calling conversations. They are **mandatory** for Gemini 3 - missing signatures result in 400 errors.

## Critical Rules

1. **Signatures appear in function call parts**, not just at the response level
2. **For sequential calls**: EACH function call needs its own signature
3. **For parallel calls**: Only the FIRST function call includes a signature
4. **Must return signatures in IDENTICAL positions** when sending tool results

## Response Structure

```go
// Response-level thought signature
type GeminiResponse struct {
    Candidates       []GeminiResponseCandidate `json:"candidates"`
    UsageMetadata    GeminiUsageMetadata       `json:"usageMetadata"`
    ThoughtSummary   string                    `json:"thoughtSummary,omitempty"`
    ThoughtSignature string                    `json:"thoughtSignature,omitempty"` // Response level
    Error            *GeminiError              `json:"error,omitempty"`
}

// Function call can also contain thought signature
type GeminiFunctionCall struct {
    Name             string                 `json:"name"`
    Args             map[string]interface{} `json:"args"`
    ThoughtSignature string                 `json:"thoughtSignature,omitempty"` // Per-call
}
```

## Sequential Function Calls

When Gemini returns multiple function calls sequentially (one after another):

```json
{
  "candidates": [{
    "content": {
      "parts": [
        {
          "functionCall": {
            "name": "read_file",
            "args": {"path": "main.go"},
            "thoughtSignature": "sig-abc123"
          }
        },
        {
          "functionCall": {
            "name": "read_file",
            "args": {"path": "go.mod"},
            "thoughtSignature": "sig-def456"
          }
        }
      ]
    }
  }],
  "thoughtSignature": "sig-response-level"
}
```

**Each function call has its own signature** that must be returned in the exact position.

## Parallel Function Calls

When Gemini returns parallel function calls (executed together):

```json
{
  "candidates": [{
    "content": {
      "parts": [
        {
          "functionCall": {
            "name": "search",
            "args": {"query": "errors"},
            "thoughtSignature": "sig-only-first"
          }
        },
        {
          "functionCall": {
            "name": "search",
            "args": {"query": "logging"}
          }
        }
      ]
    }
  }]
}
```

**Only the first call has a signature** - this signature applies to the entire parallel batch.

## Sending Tool Results Back

### Capturing Signatures

```go
type toolCallWithSignature struct {
    call      *GeminiFunctionCall
    signature string
    index     int
}

func (c *GeminiClient) extractToolCalls(resp *GeminiResponse) []toolCallWithSignature {
    var calls []toolCallWithSignature

    // Capture response-level signature
    responseSig := resp.ThoughtSignature

    for i, part := range resp.Candidates[0].Content.Parts {
        if part.FunctionCall != nil {
            sig := part.FunctionCall.ThoughtSignature
            if sig == "" {
                sig = responseSig // Fall back to response-level
            }
            calls = append(calls, toolCallWithSignature{
                call:      part.FunctionCall,
                signature: sig,
                index:     i,
            })
        }
    }
    return calls
}
```

### Returning Results with Signatures

```go
func (c *GeminiClient) buildToolResultContent(
    toolCalls []toolCallWithSignature,
    results []string,
) *GeminiContent {
    parts := make([]GeminiPart, len(toolCalls))

    for i, tc := range toolCalls {
        parts[i] = GeminiPart{
            FunctionResponse: &GeminiFunctionResponse{
                Name: tc.call.Name,
                Response: map[string]interface{}{
                    "result": results[i],
                },
            },
        }

        // Return signature in IDENTICAL position
        if tc.signature != "" {
            parts[i].ThoughtSignature = tc.signature
        }
    }

    return &GeminiContent{
        Role:  "function",
        Parts: parts,
    }
}
```

### Request with Signature Passthrough

```go
func (c *GeminiClient) CompleteWithToolResults(
    ctx context.Context,
    systemPrompt string,
    contents []GeminiContent,
    toolResults []string,
    tools []ToolDefinition,
) (*LLMToolResponse, error) {
    // Build tool result content preserving signatures
    resultContent := c.buildToolResultContent(c.lastToolCalls, toolResults)

    // Build request
    reqBody := GeminiRequest{
        Contents:         append(contents, *resultContent),
        ThoughtSignature: c.lastThoughtSignature, // Response-level passthrough
        GenerationConfig: c.buildGenerationConfig(),
        Tools:            c.buildTools(tools),
    }

    // ...
}
```

## Multi-Turn Conversation Flow

```
Turn 1: User message
        ↓
Turn 2: Gemini response with function calls + signatures
        ↓
Turn 3: Function results with signatures in SAME positions
        ↓
Turn 4: Gemini continues with more calls or final response
        ↓
        (repeat until no more function calls)
```

## Client Implementation

```go
type GeminiClient struct {
    // ... other fields ...

    // Thought signature tracking
    lastThoughtSignature string
    lastToolCalls        []toolCallWithSignature
}

func (c *GeminiClient) GetLastThoughtSignature() string {
    return c.lastThoughtSignature
}

// After each response, store signature for next turn
func (c *GeminiClient) processResponse(resp *GeminiResponse) {
    if resp.ThoughtSignature != "" {
        c.lastThoughtSignature = resp.ThoughtSignature
    }
    c.lastToolCalls = c.extractToolCalls(resp)
}
```

## Error Handling

### Missing Signature Error (400)

```json
{
  "error": {
    "code": 400,
    "message": "Missing required thought signature for function call continuation",
    "status": "INVALID_ARGUMENT"
  }
}
```

**Solution:** Ensure `thoughtSignature` is captured from response and passed back.

### Mismatched Position Error (400)

```json
{
  "error": {
    "code": 400,
    "message": "Thought signature position mismatch",
    "status": "INVALID_ARGUMENT"
  }
}
```

**Solution:** Return signatures in the exact same position as they were received.

## Best Practices

1. **Always capture signatures** - Store both response-level and per-call signatures
2. **Preserve ordering** - Return tool results in the same order as function calls
3. **Handle missing gracefully** - Fall back to response-level signature if per-call is missing
4. **Debug with logging** - Log signatures during development to trace issues
5. **Clear on new conversation** - Reset stored signatures when starting fresh conversation

## Testing

```go
func TestThoughtSignaturePassthrough(t *testing.T) {
    client := NewGeminiClient(config)

    // First call - should receive signature
    resp1, _ := client.CompleteWithTools(ctx, system, user, tools)
    sig := client.GetLastThoughtSignature()
    require.NotEmpty(t, sig, "should receive thought signature")

    // Second call with results - should pass signature back
    resp2, _ := client.CompleteWithToolResults(ctx, system, contents, results, tools)

    // Verify no 400 error
    require.Nil(t, resp2.Error, "should not have signature error")
}
```
