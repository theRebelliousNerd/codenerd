# LLM Provider System Stress Test

Stress test for the multi-provider LLM client factory.

## Overview

Tests the LLM Provider System's handling of:

- Multi-provider client creation (8 providers)
- Auto-detection from environment and config
- Concurrent API calls across providers
- Rate limiting and timeout handling
- Provider fallback and failover
- CLI engine integration (Claude CLI, Codex CLI)

**Expected Duration:** 25-40 minutes total

### Key Files

- `internal/perception/client.go` - Client factory, provider detection
- `internal/perception/client_zai.go` - Z.AI implementation
- `internal/perception/client_anthropic.go` - Anthropic implementation
- `internal/perception/client_gemini.go` - Gemini implementation
- `internal/perception/client_openai.go` - OpenAI implementation
- `internal/perception/client_xai.go` - xAI implementation
- `internal/perception/client_openrouter.go` - OpenRouter implementation

### Supported Providers

| Provider | Default Model | Config Key |
|----------|---------------|------------|
| Z.AI | `glm-4.7` | `zai_api_key` |
| Anthropic | `claude-sonnet-4` | `anthropic_api_key` |
| OpenAI | `gpt-5.1-codex-max` | `openai_api_key` |
| Gemini | `gemini-3-pro-preview` | `gemini_api_key` |
| xAI | `grok-3-beta` | `xai_api_key` |
| OpenRouter | (various) | `openrouter_api_key` |
| Claude CLI | (subprocess) | `engine: claude-cli` |
| Codex CLI | (subprocess) | `engine: codex-cli` |

---

## Conservative Test (8-12 min)

Test basic provider detection and single calls.

### Step 1: Verify Provider Detection (wait 2 min)

```bash
./nerd.exe status
```

Check which provider was detected:

```bash
Select-String -Path ".nerd/logs/*perception*.log" -Pattern "provider|detected|using"
```

### Step 2: Single LLM Call (wait 4 min)

Execute simple task:

```bash
./nerd.exe perception "hello, please respond briefly"
```

Verify call completed:

```bash
Select-String -Path ".nerd/logs/*api*.log" -Pattern "Complete|response|tokens"
```

### Step 3: Provider Metadata (wait 2 min)

Check provider configuration:

```bash
./nerd.exe config show
Select-String -Path ".nerd/logs/*perception*.log" -Pattern "model=|endpoint=|timeout="
```

### Step 4: Timeout Configuration (wait 2 min)

Verify timeouts are applied:

```bash
Select-String -Path ".nerd/logs/*perception*.log" -Pattern "timeout|HTTPClient|deadline"
```

### Success Criteria

- [ ] Provider auto-detected correctly
- [ ] API call succeeded
- [ ] Model metadata logged
- [ ] Timeouts configured

---

## Aggressive Test (10-15 min)

Push provider with concurrent calls and large contexts.

### Step 1: Clear Logs (wait 1 min)

```bash
Remove-Item .nerd/logs/* -ErrorAction SilentlyContinue
```

### Step 2: Concurrent API Calls (wait 6 min)

Spawn multiple shards making API calls:

```bash
Start-Job { ./nerd.exe spawn coder "task 1" }
Start-Job { ./nerd.exe spawn coder "task 2" }
Start-Job { ./nerd.exe spawn reviewer "review task" }
Start-Job { ./nerd.exe spawn researcher "research topic" }

Get-Job | Wait-Job -Timeout 360
Get-Job | Receive-Job -ErrorAction SilentlyContinue
Get-Job | Remove-Job
```

### Step 3: Verify API Scheduler Integration (wait 3 min)

Check that APIScheduler limited concurrent calls:

```bash
Select-String -Path ".nerd/logs/*shards*.log" -Pattern "APIScheduler|slot|waiting"
```

### Step 4: Large Context Request (wait 4 min)

Test with large input:

```bash
./nerd.exe spawn coder "analyze and explain every function in internal/core/kernel.go with detailed explanations"
```

Monitor token usage:

```bash
Select-String -Path ".nerd/logs/*api*.log" -Pattern "tokens|context|input_tokens|output_tokens"
```

### Step 5: Rate Limit Handling (wait 2 min)

Check for rate limit responses:

```bash
Select-String -Path ".nerd/logs/*api*.log" -Pattern "rate limit|429|retry|backoff"
```

### Success Criteria

- [ ] Concurrent calls completed
- [ ] API scheduler managed contention
- [ ] Large context handled
- [ ] Rate limits triggered backoff (if applicable)

---

## Chaos Test (12-18 min)

Stress test with failures and edge cases.

### Step 1: Clear State (wait 1 min)

```bash
./nerd.exe /new-session
Remove-Item .nerd/logs/* -ErrorAction SilentlyContinue
```

### Step 2: Timeout Testing (wait 5 min)

Trigger potentially slow operations:

```bash
./nerd.exe spawn coder "generate a complete 2000-line implementation of a database engine"
```

Monitor for timeout handling:

```bash
Select-String -Path ".nerd/logs/*api*.log" -Pattern "timeout|deadline exceeded|context cancel"
```

### Step 3: Cancellation During API Call (wait 4 min)

Start a call and cancel it:

```bash
$job = Start-Job { ./nerd.exe spawn coder "very long detailed task" }
Start-Sleep 10
Stop-Job $job
Remove-Job $job
```

Check cancellation handling:

```bash
Select-String -Path ".nerd/logs/*api*.log" -Pattern "cancelled|cancel|context done"
```

### Step 4: Invalid Input Handling (wait 3 min)

Test with edge case inputs:

```bash
./nerd.exe perception ""
./nerd.exe perception "$(python -c 'print(\"x\" * 100000)')"
```

Check error handling:

```bash
Select-String -Path ".nerd/logs/*perception*.log" -Pattern "error|invalid|empty"
```

### Step 5: Connection Recovery (wait 4 min)

Force reconnection scenarios:

```bash
# Make several quick calls
for ($i=1; $i -le 5; $i++) {
    ./nerd.exe perception "quick test $i"
    Start-Sleep -Seconds 2
}
```

### Success Criteria

- [ ] Timeouts handled gracefully
- [ ] Cancellation cleaned up resources
- [ ] Invalid inputs rejected safely
- [ ] System recovered from errors

---

## Hybrid Test (12-15 min)

Test provider integration with full system.

### Step 1: Clear State (wait 1 min)

```bash
./nerd.exe /new-session
Remove-Item .nerd/logs/* -ErrorAction SilentlyContinue
```

### Step 2: Campaign with Multiple LLM Calls (wait 8 min)

Run campaign generating many API calls:

```bash
./nerd.exe campaign start "create a REST API with authentication"
```

Monitor API call patterns:

```bash
# Count calls during campaign
Select-String -Path ".nerd/logs/*api*.log" -Pattern "Complete" | Measure-Object
```

### Step 3: Verify Timeout Tier Usage (wait 3 min)

Check different timeout tiers applied:

```bash
Select-String -Path ".nerd/logs/*api*.log" -Pattern "ShardExecutionTimeout|ArticulationTimeout|FollowUpTimeout"
```

### Step 4: Provider-Shard Affinity (wait 3 min)

Verify correct timeouts per shard type:

```bash
Select-String -Path ".nerd/logs/*shards*.log" -Pattern "timeout|duration"
```

### Success Criteria

- [ ] Campaign completed with multiple API calls
- [ ] Different timeout tiers applied correctly
- [ ] No provider connection issues
- [ ] Shard-specific timeouts respected

---

## Post-Test Analysis

```bash
cd .claude/skills/stress-tester/scripts
python analyze_stress_logs.py --verbose
```

### Provider-Specific Queries

```bash
# Count API calls
Select-String -Path ".nerd/logs/*api*.log" -Pattern "Complete" | Measure-Object

# Check for errors
Select-String -Path ".nerd/logs/*api*.log" -Pattern "error|fail" |
    Where-Object { $_.Line -notmatch "LOGIC_ERROR" }

# Token usage (if logged)
Select-String -Path ".nerd/logs/*api*.log" -Pattern "tokens=" |
    ForEach-Object { $_.Line }

# Average response time
Select-String -Path ".nerd/logs/*api*.log" -Pattern "duration=" |
    ForEach-Object { $_.Line -match "duration=(\d+)" | Out-Null; [int]$matches[1] } |
    Measure-Object -Average
```

### Success Metrics

| Metric | Conservative | Aggressive | Chaos | Hybrid |
|--------|--------------|------------|-------|--------|
| Panics | 0 | 0 | 0 | 0 |
| API errors | 0 | <5% | <10% | <5% |
| Timeout failures | 0 | 0 | <3 | 0 |
| Rate limit hits | 0 | ≤5 | Any | ≤5 |
| Avg response time | <30s | <120s | Any | <60s |

---

## Known Issues to Watch For

| Issue | Symptom | Root Cause | Fix |
|-------|---------|------------|-----|
| API key missing | Client creation fails | Env/config not set | Check configuration |
| Rate limit | 429 errors | Too many requests | Implement backoff |
| Timeout | Context deadline | Response too slow | Increase timeout |
| Connection refused | Network error | Server unreachable | Check connectivity |
| Token overflow | Input too large | Context limit exceeded | Truncate input |

---

## Provider-Specific Notes

### Z.AI (GLM-4.7)

- 200K context, 128K output tokens
- 150+ seconds for simple prompts
- 600ms rate limit delay recommended

### Gemini 3 Flash/Pro

- Fast inference
- Lower context window
- Use for quick operations

### CLI Engines

- Subprocess-based
- No direct API calls
- Uses Claude Code / Codex CLI installed locally

---

## Related Files

- [api-scheduler-stress.md](../01-kernel-core/api-scheduler-stress.md) - API scheduling
- [timeout-consolidation.md](timeout-consolidation.md) - Timeout configuration
