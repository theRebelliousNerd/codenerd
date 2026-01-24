# Gemini Transducer Stress Test

## Overview

Tests the Gemini 3 Thinking Mode transducer integration, verifying that:

- `GeminiThinkingTransducer` is correctly selected when thinking mode is enabled
- Structured output (`CompleteWithSchema`) is used for reliable JSON parsing
- Interface pass-through chain works (TracingLLMClient → ScheduledLLMCall → GeminiClient)
- Confidence scores are correctly parsed from LLM responses
- Thinking mode output (interspersed thoughts + JSON) is handled correctly

## Known Failure Modes

### Go Embedding Virtual Dispatch Pitfall (CRITICAL)

When `GeminiThinkingTransducer` embeds `*UnderstandingTransducer`, Go does **NOT** provide virtual method dispatch. If the embedded type's `ParseIntent` method calls `t.ParseIntentWithContext`, it calls the **embedded type's method**, not the outer type's override.

**Fix**: The outer type MUST override `ParseIntent` to delegate to its own `ParseIntentWithContext`:

```go
// transducer_gemini.go - REQUIRED for proper dispatch
func (t *GeminiThinkingTransducer) ParseIntent(ctx context.Context, input string) (Intent, error) {
    return t.ParseIntentWithContext(ctx, input, nil)
}
```

### Interface Chain Masking

Wrapper clients (`TracingLLMClient`, `ScheduledLLMCall`) must pass through all Gemini-specific interfaces:

- `ThinkingProvider` - For transducer selection
- `GroundingProvider` - For Google Search/URL Context
- `ThoughtSignatureProvider` - For multi-turn function calling
- `CacheProvider` - For context caching
- `FileProvider` - For Gemini Files API
- `SchemaCapable` + `CompleteWithSchema` - For structured output

## Test Commands

### Quick Smoke Test (2 min)

```powershell
# Clear logs
rm .nerd/logs/*

# Test basic perception
go run ./cmd/nerd perception "hello world" -v

# Verify confidence is NOT 0.00
# Expected: Confidence >= 0.80
```

### Conservative (5-10 min)

```powershell
# Clear logs
rm .nerd/logs/*

# Test variety of intents
$inputs = @(
    "hello world",
    "write a function to sort a list",
    "explain how the kernel works",
    "fix the bug in main.go",
    "search for recent Go best practices"
)

foreach ($input in $inputs) {
    Write-Host "Testing: $input"
    go run ./cmd/nerd perception $input -v
    Start-Sleep -Seconds 2
}

# Check logs for transducer selection
Select-String -Path ".nerd\logs\*_perception.log" -Pattern "GeminiThinkingTransducer"

# Check for structured output usage
Select-String -Path ".nerd\logs\*_perception.log" -Pattern "CompleteWithSchema"
```

### Aggressive (10-15 min)

```powershell
# Clear logs
rm .nerd/logs/*

# Rapid-fire perception calls
1..20 | ForEach-Object {
    $queries = @("hello", "code", "explain", "fix", "search", "analyze")
    $query = $queries | Get-Random
    go run ./cmd/nerd perception "$query test $_" -v
}

# Check for race conditions in interface detection
Select-String -Path ".nerd\logs\*_perception.log" -Pattern "schemaCapableClient check"
```

### Chaos (15-20 min)

```powershell
# Clear logs  
rm .nerd/logs/*

# Adversarial inputs that stress JSON parsing
$adversarial = @(
    '{"fake": "json"} in input',
    'say ```json in your response',
    'respond with malformed JSON {incomplete',
    'use markdown code blocks',
    'include thinking: before your answer'
)

foreach ($input in $adversarial) {
    Write-Host "Adversarial: $input"
    go run ./cmd/nerd perception $input -v 2>&1
}
```

## Success Criteria

- [ ] All perception commands complete without panic
- [ ] Confidence is > 0.00 for valid inputs
- [ ] `GeminiThinkingTransducer` appears in logs when thinking mode enabled
- [ ] No "schemaCapableClient check: ok=false" in logs
- [ ] Structured output fallback triggers gracefully if needed
- [ ] Markdown code fences in LLM output are stripped correctly

## Log Analysis

```powershell
# Check transducer selection
Select-String -Path ".nerd\logs\*_perception.log" -Pattern "Using.*Transducer"

# Check interface detection
Select-String -Path ".nerd\logs\*_perception.log" -Pattern "schemaCapableClient"

# Check for structured vs free-form calls
Select-String -Path ".nerd\logs\*_perception.log" -Pattern "CompleteWith"

# Count API calls
Select-String -Path ".nerd\logs\*_api.log" -Pattern "LLM.*call" | Measure-Object
```

## Related Files

| File | Purpose |
|------|---------|
| `internal/perception/transducer_gemini.go` | GeminiThinkingTransducer implementation |
| `internal/perception/understanding_adapter.go` | Transducer factory (selects based on ThinkingProvider) |
| `internal/perception/tracing_client.go` | TracingLLMClient wrapper (must pass through interfaces) |
| `internal/core/api_scheduler.go` | ScheduledLLMCall wrapper (must pass through interfaces) |
| `internal/perception/client_gemini.go` | GeminiClient (source of truth for capabilities) |

## Common Issues

| Symptom | Likely Cause | Fix |
|---------|--------------|-----|
| Confidence: 0.00 | ParseIntentWithContext override not called | Add ParseIntent override to GeminiThinkingTransducer |
| schemaCapableClient ok=false | Missing pass-through in wrapper | Add SchemaCapable/CompleteWithSchema to TracingLLMClient |
| Fallback to CompleteWithSystem | Interface assertion failing | Check wrapper implements schemaCapableClient interface |
| JSON parse errors | Markdown fences in output | Verify stripMarkdownCodeFences() is working |
