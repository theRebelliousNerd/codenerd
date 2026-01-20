# Transform Package - Antigravity Only

## Scope

This package provides request/response transformation utilities **exclusively for the Antigravity client** (`client_antigravity.go`).

### Supported Models (Antigravity Gateway)

| Model Family | Models | Thinking Format |
|--------------|--------|-----------------|
| **Gemini 3** | `gemini-3-flash`, `gemini-3-pro` | camelCase: `includeThoughts`, `thinkingLevel` ("low"/"medium"/"high") |
| **Claude** | `claude-sonnet-4-5-thinking`, `claude-opus-4-5-thinking` | snake_case: `include_thoughts`, `thinking_budget` (numeric) |

### NOT in Scope

| Model | Endpoint | Why Excluded |
|-------|----------|--------------|
| Gemini 2.5 (`gemini-2.5-pro`, `gemini-2.5-flash`) | Gemini CLI | Uses `thinkingBudget` (numeric), accessed via different endpoint |
| Direct Gemini API | N/A | Use `client_gemini.go` instead |
| OpenAI/GPT | N/A | Not supported by Antigravity |

## Package Structure

```
transform/
├── models.go       # Model detection, resolution, tier extraction
├── thinking.go     # Model-aware thinking configuration builders
├── sanitizer.go    # Cross-model signature sanitization
├── recovery.go     # Thinking recovery for corrupted tool loops
└── *_test.go       # Comprehensive tests
```

## Key Functions

### Model Detection (`models.go`)

```go
// Check if model is Antigravity-supported
IsAntigravityModel("gemini-3-flash")  // true
IsAntigravityModel("gemini-2.5-pro")  // false (Gemini CLI)

// Get model family for routing
GetModelFamily("claude-sonnet-4-5-thinking")  // ModelFamilyClaude
GetModelFamily("gemini-3-flash")              // ModelFamilyGemini

// Extract thinking tier from model name
ExtractThinkingTier("gemini-3-flash-high")  // ThinkingTierHigh
```

### Thinking Configuration (`thinking.go`)

```go
// Gemini 3: returns {"includeThoughts": true, "thinkingLevel": "high"}
BuildThinkingConfigForModel("gemini-3-flash", true, ThinkingTierHigh, 0)

// Claude: returns {"include_thoughts": true, "thinking_budget": 32768}
BuildThinkingConfigForModel("claude-sonnet-4-5-thinking", true, ThinkingTierHigh, 0)
```

### Cross-Model Sanitization (`sanitizer.go`)

Fixes "Invalid `signature` in `thinking` block" error when switching models mid-session:

```go
// Before sending to Claude, strip Gemini signatures
result := SanitizeCrossModelPayload(contents, "claude-sonnet-4-5-thinking")
// Removes: thoughtSignature, thinkingMetadata from Gemini messages

// Before sending to Gemini, strip Claude signatures  
result := SanitizeCrossModelPayload(contents, "gemini-3-flash")
// Removes: signature from Claude thinking blocks
```

### Thinking Recovery (`recovery.go`)

Recovers from corrupted thinking state (tool loops without thinking):

```go
// Analyze conversation for tool loops
state := AnalyzeConversationState(contents)

// Check if recovery needed
if NeedsThinkingRecovery(state) {
    // Close the corrupted turn and start fresh
    contents = CloseToolLoopForThinking(contents)
}
```

## Integration with AntigravityClient

The `CompleteMultiTurn` method in `client_antigravity.go` uses this package automatically:

```go
func (c *AntigravityClient) CompleteMultiTurn(...) {
    // 1. Cross-model sanitization on model switch
    if previousModel != "" && GetModelFamily(previousModel) != GetModelFamily(currentModel) {
        SanitizeCrossModelPayload(contents, currentModel)
    }
    
    // 2. Thinking recovery for tool loops
    if enableThinkingRecovery {
        state := AnalyzeConversationState(contents)
        if NeedsThinkingRecovery(state) {
            contents = CloseToolLoopForThinking(contents)
        }
    }
    
    // 3. Model-aware thinking config
    thinkingConfig := BuildThinkingConfigForModel(model, ...)
}
```

## Testing

```bash
go test ./internal/perception/transform/... -v
```

## Common Issues

### "Invalid signature in thinking block"

**Cause**: Model switched mid-conversation, old signatures are invalid for new model.

**Fix**: `SanitizeCrossModelPayload()` is called automatically in `CompleteMultiTurn`.

### Thinking not working after tool loop

**Cause**: Context compaction stripped thinking blocks, breaking thinking continuity.

**Fix**: `CloseToolLoopForThinking()` injects synthetic messages to start a fresh turn.

### Wrong thinking format

**Cause**: Using Gemini format for Claude or vice versa.

**Fix**: `BuildThinkingConfigForModel()` automatically selects correct format.
