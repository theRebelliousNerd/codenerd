# Glass-Box Tool Visibility Stress Test

Stress test for real-time tool execution visibility in the TUI.

## Overview

Tests the Glass-Box Tool Visibility system's handling of:

- Real-time tool invocation display
- Parameter visibility streaming
- Result output streaming
- Error context with remediation
- Concurrent tool visibility
- UI thread synchronization

**Expected Duration:** 15-25 minutes total

### Key Files

- `cmd/nerd/chat/glass_box.go` - Glass-box rendering
- `cmd/nerd/chat/model_update.go` - TUI update handling
- `cmd/nerd/chat/session.go` - Session integration
- `internal/transparency/transparency.go` - Operation visibility

### Features

- **Tool Execution Display**: Shows which tools are being invoked
- **Parameter Visibility**: Displays tool parameters in real-time
- **Result Streaming**: Shows tool output as it arrives
- **Error Context**: Provides detailed error information with remediation

---

## Conservative Test (5-8 min)

Test basic glass-box rendering.

### Step 1: Start Interactive Session (wait 2 min)

```bash
./nerd.exe chat
```

In the chat, enter a simple task:

```
write a hello world function in Go
```

**Observe:** Tool invocations should appear in glass-box format.

### Step 2: Verify Tool Display (wait 2 min)

Watch for tool execution display:

- Tool name shown
- Parameters visible
- Execution indicator active
- Result displayed when complete

Check logs:

```bash
Select-String -Path ".nerd/logs/*chat*.log" -Pattern "glass_box|tool_display|rendering"
```

### Step 3: Single Tool Execution (wait 2 min)

Execute task with single tool:

```
read the README.md file
```

Verify glass-box shows:

- File read tool invoked
- File path parameter
- Content result

### Step 4: Exit and Check Logs (wait 1 min)

Exit chat and check for rendering issues:

```bash
Select-String -Path ".nerd/logs/*chat*.log" -Pattern "error|panic" | Where-Object { $_.Line -notmatch "LOGIC_ERROR" }
```

### Success Criteria

- [ ] Tool invocations displayed in real-time
- [ ] Parameters visible during execution
- [ ] Results shown when tools complete
- [ ] No rendering errors

---

## Aggressive Test (6-10 min)

Push glass-box with complex operations.

### Step 1: Start Chat Session (wait 1 min)

```bash
./nerd.exe chat
```

### Step 2: Multi-Tool Operation (wait 4 min)

Execute task requiring multiple tools:

```
analyze the internal/core/kernel.go file for potential improvements
```

**Observe:**
- Multiple tool invocations displayed
- Tools shown in execution order
- Results accumulated

### Step 3: Rapid Tool Sequence (wait 3 min)

Execute task with rapid tool calls:

```
list files in internal/core, then read kernel.go, then show its functions
```

Watch for:

- Rapid glass-box updates
- No flickering or corruption
- All tools visible

### Step 4: Large Result Handling (wait 2 min)

Execute task with large output:

```
read all Go files in internal/core and summarize them
```

Verify large results handled:

- Scrolling works
- No truncation issues
- UI remains responsive

### Success Criteria

- [ ] Multi-tool operations displayed correctly
- [ ] Rapid sequences handled smoothly
- [ ] Large results scrollable
- [ ] UI remained responsive

---

## Chaos Test (8-12 min)

Stress test with edge cases.

### Step 1: Start Chat Session (wait 1 min)

```bash
./nerd.exe chat
```

### Step 2: Concurrent Tool Display (wait 4 min)

In chat, trigger concurrent operations:

```
spawn 3 parallel tasks: read README.md, analyze kernel.go, and list all Go files
```

**Observe:**

- Multiple glass-boxes may appear
- No display corruption
- All results visible

### Step 3: Error Display (wait 3 min)

Trigger tool error:

```
read a file that does not exist: nonexistent_file_xyz.go
```

Verify error display:

- Error shown in glass-box
- Remediation suggested
- No crash

### Step 4: Cancellation Display (wait 2 min)

Start long operation and cancel (Ctrl+C):

```
analyze the entire codebase in extreme detail
```

Press Ctrl+C during execution.

Verify:

- Glass-box shows cancellation
- No orphaned display elements
- Session recoverable

### Step 5: Streaming Output (wait 2 min)

Execute task with streaming:

```
explain the kernel architecture step by step
```

Watch for:

- Real-time streaming in glass-box
- Incremental updates
- Smooth rendering

### Success Criteria

- [ ] Concurrent display handled
- [ ] Errors displayed with remediation
- [ ] Cancellation cleaned up display
- [ ] Streaming rendered smoothly

---

## Hybrid Test (6-10 min)

Test glass-box integration with other systems.

### Step 1: Start Chat Session (wait 1 min)

```bash
./nerd.exe chat
```

### Step 2: Campaign with Glass-Box (wait 5 min)

Start campaign and watch glass-box:

```
/campaign start "add a greeting function to the codebase"
```

**Observe:**

- Campaign phases shown
- Tool invocations per phase
- Shard execution visible

### Step 3: Transparency Integration (wait 2 min)

Check transparency layer integration:

```bash
Select-String -Path ".nerd/logs/*transparency*.log" -Pattern "glass_box|tool_visible|observer"
```

### Step 4: Shard Observer (wait 2 min)

Verify shard observer integration:

```bash
Select-String -Path ".nerd/logs/*shards*.log" -Pattern "observer|phase|tracking"
```

### Success Criteria

- [ ] Campaign phases visible in glass-box
- [ ] Transparency layer integrated
- [ ] Shard observer connected
- [ ] Full execution trace visible

---

## Post-Test Analysis

```bash
cd .claude/skills/stress-tester/scripts
python analyze_stress_logs.py --verbose
```

### Glass-Box Specific Queries

```bash
# Count tool displays
Select-String -Path ".nerd/logs/*chat*.log" -Pattern "tool_display|glass_box" | Measure-Object

# Find rendering errors
Select-String -Path ".nerd/logs/*chat*.log" -Pattern "render error|display fail|UI crash"

# Check streaming events
Select-String -Path ".nerd/logs/*chat*.log" -Pattern "stream|chunk|incremental"
```

### Success Metrics

| Metric | Conservative | Aggressive | Chaos | Hybrid |
|--------|--------------|------------|-------|--------|
| Panics | 0 | 0 | 0 | 0 |
| Render errors | 0 | 0 | <3 | 0 |
| Display corruption | 0 | 0 | 0 | 0 |
| UI freezes | 0 | 0 | <1 | 0 |
| Orphaned elements | 0 | 0 | 0 | 0 |

---

## Known Issues to Watch For

| Issue | Symptom | Root Cause | Fix |
|-------|---------|------------|-----|
| Display flicker | Rapid updates cause flicker | No debouncing | Add debounce |
| Truncation | Long output cut off | Buffer limit | Increase buffer |
| Deadlock | UI freezes | Thread contention | Check mutexes |
| Race condition | Corrupted display | Concurrent updates | Synchronize updates |
| Memory leak | Growing memory | Retained glass-boxes | Clear on completion |

---

## UI Testing Tips

- **Terminal size**: Test with different terminal sizes
- **Color themes**: Verify visibility in different themes
- **Scrollback**: Check scrollback buffer handling
- **Focus**: Verify keyboard focus during tool execution

---

## Related Files

- [perception-to-campaign.md](../08-hybrid-integration/perception-to-campaign.md) - Full pipeline
- [shard-explosion.md](../03-shards-campaigns/shard-explosion.md) - Shard lifecycle
