# MCP JIT Tool Compiler Stress Test

Stress test for the Model Context Protocol integration and JIT Tool Compiler.

## Overview

Tests the MCP system's handling of:

- Tool discovery and registration from MCP servers
- JIT tool selection using Mangle logic + vector scoring
- Three-tier rendering (Full/Condensed/Minimal)
- Skeleton/Flesh bifurcation for context-aware tool sets
- Concurrent tool analysis and metadata extraction

**Expected Duration:** 20-35 minutes total

### Key Files

- `internal/mcp/compiler.go` - JIT Tool Compiler
- `internal/mcp/store.go` - SQLite tool storage with embeddings
- `internal/mcp/analyzer.go` - LLM-based metadata extraction
- `internal/mcp/client.go` - MCP server connections
- `internal/mcp/renderer.go` - Tool set rendering

---

## Conservative Test (5-10 min)

Test basic MCP tool registration and selection.

### Step 1: Verify MCP Store (wait 2 min)

```bash
./nerd.exe status
```

Check for MCP store initialization in logs:

```bash
Select-String -Path ".nerd/logs/*mcp*.log" -Pattern "MCPToolStore|initialized"
```

### Step 2: List Available Tools (wait 2 min)

```bash
./nerd.exe mcp tools
```

**Verify:** Tool list is retrieved without errors.

### Step 3: Simple Tool Selection (wait 3 min)

Trigger a shard that uses MCP tools:

```bash
./nerd.exe spawn coder "read the README file"
```

Monitor JIT tool selection:

```bash
Select-String -Path ".nerd/logs/*mcp*.log" -Pattern "JIT selected|skeleton|flesh"
```

### Step 4: Verify Tool Rendering (wait 2 min)

Check that tools were rendered at appropriate tiers:

```bash
Select-String -Path ".nerd/logs/*mcp*.log" -Pattern "Full render|Condensed|Minimal"
```

### Success Criteria

- [ ] MCP store initialized without errors
- [ ] Tools discovered from configured servers
- [ ] Skeleton tools always included
- [ ] Flesh tools selected based on context
- [ ] No rendering failures

---

## Aggressive Test (8-12 min)

Push tool selection system with complex contexts.

### Step 1: Clear State (wait 1 min)

```bash
./nerd.exe /new-session
Remove-Item .nerd/logs/* -ErrorAction SilentlyContinue
```

### Step 2: High-Context Tool Selection (wait 5 min)

Trigger complex task requiring many tools:

```bash
./nerd.exe spawn coder "refactor the entire internal/mcp package to use dependency injection"
```

Monitor tool selection scoring:

```bash
Select-String -Path ".nerd/logs/*mcp*.log" -Pattern "score=|hybrid_score|relevance"
```

### Step 3: Concurrent Tool Selections (wait 4 min)

Spawn multiple shards competing for tool selection:

```bash
Start-Job { ./nerd.exe spawn coder "write unit tests" }
Start-Job { ./nerd.exe spawn reviewer "review security" }
Start-Job { ./nerd.exe spawn researcher "research Go patterns" }

Get-Job | Wait-Job -Timeout 240
Get-Job | Receive-Job -ErrorAction SilentlyContinue
Get-Job | Remove-Job
```

### Step 4: Verify No Tool Conflicts (wait 2 min)

```bash
Select-String -Path ".nerd/logs/*mcp*.log" -Pattern "conflict|lock|contention"
```

### Success Criteria

- [ ] Complex context produced relevant tool selection
- [ ] Concurrent selections completed without deadlock
- [ ] Hybrid scoring (Logic × 0.7 + Vector × 0.3) calculated
- [ ] No tool store contention errors

---

## Chaos Test (10-15 min)

Stress test with edge cases and failure modes.

### Step 1: Clear State (wait 1 min)

```bash
./nerd.exe /new-session
Remove-Item .nerd/logs/* -ErrorAction SilentlyContinue
```

### Step 2: Tool Selection with Empty Context (wait 3 min)

Test skeleton-only selection:

```bash
./nerd.exe spawn coder ""
```

Verify skeleton tools were selected:

```bash
Select-String -Path ".nerd/logs/*mcp*.log" -Pattern "skeleton only|default tools"
```

### Step 3: Token Budget Exhaustion (wait 5 min)

Request massive tool context:

```bash
./nerd.exe spawn coder "use every available tool to analyze the entire codebase comprehensively"
```

Monitor token budget management:

```bash
Select-String -Path ".nerd/logs/*mcp*.log" -Pattern "token budget|truncat|overflow"
```

### Step 4: Malformed Tool Metadata (wait 4 min)

If MCP server returns malformed data, system should handle gracefully:

```bash
# Check error handling in logs
Select-String -Path ".nerd/logs/*mcp*.log" -Pattern "parse error|invalid tool|malformed"
```

### Step 5: Rapid Tool Reselection (wait 3 min)

Force rapid tool recompilation:

```bash
for ($i=1; $i -le 10; $i++) {
    ./nerd.exe spawn coder "task $i"
    Start-Sleep -Milliseconds 500
}
```

### Success Criteria

- [ ] Empty context uses skeleton tools only
- [ ] Token budget respected (no overflow)
- [ ] Malformed metadata handled gracefully
- [ ] Rapid reselection didn't cause crashes
- [ ] System remained stable

---

## Hybrid Test (10-15 min)

Test MCP integration with other systems.

### Step 1: Clear State (wait 1 min)

```bash
./nerd.exe /new-session
Remove-Item .nerd/logs/* -ErrorAction SilentlyContinue
```

### Step 2: Campaign with Tool Evolution (wait 10 min)

Start campaign that may use different tools per phase:

```bash
./nerd.exe campaign start "build a CLI tool with file operations and HTTP requests"
```

Monitor tool selection across phases:

```bash
# Check tool selection per phase
Select-String -Path ".nerd/logs/*mcp*.log" -Pattern "phase|tool_selected"
```

### Step 3: Verify Mangle Integration (wait 3 min)

Check that Mangle facts were asserted:

```bash
./nerd.exe query "mcp_tool_selected"
./nerd.exe query "mcp_tool_capability"
```

### Step 4: Tool Selection with JIT Prompt (wait 3 min)

Verify tool context injection into prompts:

```bash
Select-String -Path ".nerd/logs/*jit*.log" -Pattern "MCP tools|tool context"
```

### Success Criteria

- [ ] Different tools selected for different phases
- [ ] Mangle facts recorded for tool selection
- [ ] Tool context properly injected into prompts
- [ ] Campaign completed with tool assistance

---

## Post-Test Analysis

```bash
cd .claude/skills/stress-tester/scripts
python analyze_stress_logs.py --verbose
```

### MCP-Specific Queries

```bash
# Count tool selections
Select-String -Path ".nerd/logs/*mcp*.log" -Pattern "selected tool" | Measure-Object

# Check rendering distribution
Select-String -Path ".nerd/logs/*mcp*.log" -Pattern "Full render" | Measure-Object
Select-String -Path ".nerd/logs/*mcp*.log" -Pattern "Condensed" | Measure-Object
Select-String -Path ".nerd/logs/*mcp*.log" -Pattern "Minimal" | Measure-Object

# Find errors
Select-String -Path ".nerd/logs/*mcp*.log" -Pattern "error|panic|fail"
```

### Success Metrics

| Metric | Conservative | Aggressive | Chaos | Hybrid |
|--------|--------------|------------|-------|--------|
| Panics | 0 | 0 | 0 | 0 |
| Selection failures | 0 | 0 | <5% | 0 |
| Token overflows | 0 | 0 | 0 | 0 |
| Avg selection time | <1s | <3s | <5s | <3s |

---

## Known Issues to Watch For

| Issue | Symptom | Root Cause | Fix |
|-------|---------|------------|-----|
| Tool not found | "unknown tool" error | Server disconnected | Check MCP connections |
| Score NaN | Hybrid score calculation fails | Missing embedding | Regenerate embeddings |
| Rendering timeout | Tool list truncated | Too many tools | Reduce MaxTools config |
| Mangle fact missing | Query returns empty | Fact assertion failed | Check kernel integration |

---

## Related Files

- [perception-to-campaign.md](../08-hybrid-integration/perception-to-campaign.md) - Full pipeline
- [api-scheduler-stress.md](../01-kernel-core/api-scheduler-stress.md) - API scheduling
