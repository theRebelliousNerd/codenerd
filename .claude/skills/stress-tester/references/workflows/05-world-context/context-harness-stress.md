# Context Harness Stress Test

Stress test for the infinite context system using the context test harness.

## Overview

Tests the context harness system's handling of:

- Long-running scenario simulations (50-100 turns)
- Spreading activation under high fact density
- Semantic compression with extreme ratios
- JIT prompt compilation with large atom corpora
- Checkpoint validation with stringent requirements
- Context feedback learning loops
- Piggyback protocol parsing under load

**Expected Duration:** 25-45 minutes total

### Key Files

- `internal/testing/context_harness/harness.go` - Main orchestrator
- `internal/testing/context_harness/simulator.go` - Session simulation
- `internal/testing/context_harness/scenarios.go` - Pre-built test scenarios
- `internal/testing/context_harness/metrics.go` - Metrics collection
- `internal/testing/context_harness/file_logger.go` - Session logging
- `internal/context/activation.go` - Spreading activation engine
- `internal/context/compressor.go` - Semantic compression

### Log Output

Context harness sessions output to `.nerd/context-tests/session-YYYYMMDD-HHMMSS/`:

| Log File | Purpose |
|----------|---------|
| `prompts.log` | Full LLM prompts with token counts |
| `jit-compilation.log` | JIT atom selection traces |
| `spreading-activation.log` | Activation score calculations |
| `compression.log` | Compression ratio per turn |
| `piggyback-protocol.log` | Control packet parsing |
| `context-feedback.log` | Predicate usefulness ratings |
| `summary.log` | Checkpoint results and metrics |

---

## Conservative Test (8-12 min)

Test basic context harness functionality with default scenarios.

### Step 1: Build and Prepare (wait 2 min)

```powershell
$env:CGO_CFLAGS="-IC:/CodeProjects/codeNERD/sqlite_headers"
go build -tags=sqlite_vec -o nerd.exe ./cmd/nerd
Remove-Item .nerd/context-tests/* -Recurse -Force -ErrorAction SilentlyContinue
```

### Step 2: Run Debugging Marathon Scenario (wait 8 min)

```powershell
./nerd.exe test-context --scenario debugging-marathon
```

Monitor progress:

```powershell
# Watch for checkpoint results
Get-Content .nerd/context-tests/*/summary.log -Wait -Tail 20
```

### Step 3: Verify Results (wait 2 min)

```powershell
# Check for failures
Select-String -Path ".nerd/context-tests/*/summary.log" -Pattern "FAILED|PASSED"

# Check checkpoint results
Select-String -Path ".nerd/context-tests/*/summary.log" -Pattern "Checkpoint.*failed"
```

### Success Criteria

- [ ] Harness completed without panics
- [ ] All log files generated (7 files)
- [ ] Encoding ratio < 1.0 (compression working)
- [ ] Retrieval recall > 50%
- [ ] No token budget violations

---

## Aggressive Test (10-15 min)

Push context system with multiple scenarios and high turn counts.

### Step 1: Clear State (wait 1 min)

```powershell
Remove-Item .nerd/context-tests/* -Recurse -Force -ErrorAction SilentlyContinue
Remove-Item .nerd/logs/* -ErrorAction SilentlyContinue
```

### Step 2: Run All Scenarios Sequentially (wait 12 min)

```powershell
./nerd.exe test-context --all --format json > context_results.json
```

Monitor system resources during execution:

```powershell
# In separate terminal
while ($true) { Get-Process nerd -ErrorAction SilentlyContinue | Select-Object CPU, WorkingSet64; Start-Sleep 5 }
```

### Step 3: Analyze Results (wait 2 min)

```powershell
# Parse and analyze all sessions
$sessions = Get-ChildItem .nerd/context-tests -Directory
foreach ($s in $sessions) {
    Write-Host "=== $($s.Name) ==="
    Select-String -Path "$($s.FullName)/summary.log" -Pattern "STATUS:|Encoding Ratio:|Precision:|Recall:"
}
```

### Step 4: Cross-Reference with System Logs (wait 2 min)

```powershell
python .claude/skills/log-analyzer/scripts/parse_context_harness.py `
    (Get-ChildItem .nerd/context-tests -Directory | Sort-Object LastWriteTime -Descending | Select-Object -First 1).FullName `
    --no-schema > /tmp/context.mg

python .claude/skills/log-analyzer/scripts/parse_log.py .nerd/logs/*.log --no-schema >> /tmp/context.mg
```

### Success Criteria

- [ ] All 4 scenarios completed
- [ ] Memory stayed under 2GB
- [ ] No goroutine leaks (check with pprof)
- [ ] Cross-reference logs correlate correctly
- [ ] JSON output valid and parseable

---

## Chaos Test (12-18 min)

Stress test with edge cases and failure modes.

### Step 1: Clear State (wait 1 min)

```powershell
Remove-Item .nerd/context-tests/* -Recurse -Force -ErrorAction SilentlyContinue
Remove-Item .nerd/logs/* -ErrorAction SilentlyContinue
```

### Step 2: Rapid Scenario Switching (wait 5 min)

Run scenarios in rapid succession to stress session management:

```powershell
$scenarios = @("debugging-marathon", "feature-implementation", "refactoring-campaign", "research-build")
foreach ($s in $scenarios) {
    Start-Job -ScriptBlock { param($scenario) ./nerd.exe test-context --scenario $scenario } -ArgumentList $s
    Start-Sleep -Seconds 2
}
Get-Job | Wait-Job -Timeout 300
Get-Job | Receive-Job
Get-Job | Remove-Job
```

### Step 3: High Fact Density Test (wait 5 min)

Inject many facts before running harness:

```powershell
# Pre-populate kernel with many facts
./nerd.exe eval "
    test_fact(1, 'high_density_test').
    test_fact(2, 'high_density_test').
    # ... repeat pattern
"

./nerd.exe test-context --scenario debugging-marathon
```

Monitor spreading activation:

```powershell
Select-String -Path ".nerd/context-tests/*/spreading-activation.log" -Pattern "score=" | Measure-Object
```

### Step 4: Extreme Compression Test (wait 4 min)

Force aggressive compression by reducing token budget:

```powershell
# Modify config for low token budget
# Then run harness and check compression ratios
Select-String -Path ".nerd/context-tests/*/compression.log" -Pattern "ratio=" | `
    ForEach-Object { if ($_ -match "ratio=([0-9.]+)") { [float]$matches[1] } } | `
    Measure-Object -Average -Minimum
```

### Step 5: Checkpoint Stress (wait 3 min)

Verify checkpoint validation under pressure:

```powershell
# Count failed checkpoints
$failures = Select-String -Path ".nerd/context-tests/*/summary.log" -Pattern "Checkpoint.*failed"
Write-Host "Failed checkpoints: $($failures.Count)"

# Analyze failure patterns
$failures | ForEach-Object { $_.Line }
```

### Success Criteria

- [ ] No panics during concurrent scenarios
- [ ] Session files not corrupted
- [ ] High fact density handled gracefully
- [ ] Compression ratios stayed reasonable (> 0.1)
- [ ] Checkpoint failures have clear reasons

---

## Hybrid Test (10-15 min)

Test context harness integration with other systems.

### Step 1: Clear State (wait 1 min)

```powershell
Remove-Item .nerd/context-tests/* -Recurse -Force -ErrorAction SilentlyContinue
Remove-Item .nerd/logs/* -ErrorAction SilentlyContinue
```

### Step 2: Harness with Real LLM Engine (wait 8 min)

Run harness in real mode (requires LLM API access):

```powershell
./nerd.exe test-context --scenario debugging-marathon --mode real
```

Monitor API calls:

```powershell
Select-String -Path ".nerd/logs/*api*.log" -Pattern "CompleteWithSystem|duration"
```

### Step 3: Cross-System Correlation (wait 4 min)

Verify harness events correlate with system logs:

```powershell
# Parse both log sources
$session = (Get-ChildItem .nerd/context-tests -Directory | Sort-Object LastWriteTime -Descending | Select-Object -First 1).FullName

python .claude/skills/log-analyzer/scripts/parse_context_harness.py $session --no-schema > /tmp/combined.mg
python .claude/skills/log-analyzer/scripts/parse_log.py .nerd/logs/*.log --no-schema >> /tmp/combined.mg

# Query for correlations
cd .claude/skills/log-analyzer/scripts/logquery
./logquery.exe /tmp/combined.mg -i

# In REPL:
# ?jit_compilation(T, A, _, _), log_entry(T2, /perception, _, _, _, _), fn:minus(T2, T) < 1000.
```

### Step 4: Verify JIT Integration (wait 2 min)

Check JIT compiler was properly exercised:

```powershell
Select-String -Path ".nerd/context-tests/*/jit-compilation.log" -Pattern "atoms selected" | Measure-Object
Select-String -Path ".nerd/logs/*jit*.log" -Pattern "compiled|selected" | Measure-Object
```

### Success Criteria

- [ ] Real LLM calls completed successfully
- [ ] Harness events correlate with system logs (within 1s)
- [ ] JIT compilation traces match between harness and system logs
- [ ] No orphaned sessions or leaked resources

---

## Post-Test Analysis

### Quick Analysis

```powershell
# Comprehensive log analysis
python .claude/skills/stress-tester/scripts/analyze_stress_logs.py

# Context-specific analysis
$latestSession = (Get-ChildItem .nerd/context-tests -Directory | Sort-Object LastWriteTime -Descending | Select-Object -First 1).FullName
python .claude/skills/log-analyzer/scripts/parse_context_harness.py $latestSession --format json | ConvertFrom-Json
```

### Context Harness Queries

```powershell
# Parse session for Mangle analysis
python .claude/skills/log-analyzer/scripts/parse_context_harness.py $latestSession --no-schema > /tmp/harness.mg
cd .claude/skills/log-analyzer/scripts/logquery

# Interactive analysis
./logquery.exe /tmp/harness.mg -i

# Key queries:
# ?failed_checkpoint(Turn, Desc, P, R)
# ?low_activation(T, FactId, Score)
# ?aggressive_compression(T, Ratio)
# ?jit_recompilation(Tokens, Count)
```

### Cross-Reference Analysis

```bash
# Combined analysis with system logs
python .claude/skills/log-analyzer/scripts/parse_context_harness.py $latestSession --cross-ref .nerd/logs/*.log -o combined.mg
./logquery.exe combined.mg --builtin context-issues
```

### Success Metrics

| Metric | Conservative | Aggressive | Chaos | Hybrid |
|--------|--------------|------------|-------|--------|
| Panics | 0 | 0 | 0 | 0 |
| Session failures | 0 | 0 | <20% | 0 |
| Avg recall | >50% | >40% | >30% | >50% |
| Avg precision | >10% | >5% | >3% | >10% |
| Token violations | 0 | 0 | <5 | 0 |
| Compression ratio | 0.2-0.5 | 0.1-0.5 | >0.1 | 0.2-0.5 |

---

## Known Issues to Watch For

| Issue | Symptom | Root Cause | Fix |
|-------|---------|------------|-----|
| Checkpoint always fails | Recall=0 at checkpoint | Facts not being activated | Check spreading activation weights |
| JIT hotspot | Same prompt compiled 10+ times | Kernel state not advancing | Check fact assertion after actions |
| Compression explosion | Ratio > 1.0 (enrichment) | Compressor adding metadata | Review compression logic |
| Session leak | Old sessions not cleaned | FileLogger not closing | Check Close() calls |
| Activation timeout | Spreading activation slow | Too many facts in graph | Implement fact pruning |

---

## Related Files

- [context-compression.md](context-compression.md) - Compression-specific stress tests
- [mangle-self-healing.md](../01-kernel-core/mangle-self-healing.md) - Mangle integration
- [jit-clean-loop.md](../09-new-systems/jit-clean-loop.md) - JIT execution loop tests
- [long-running-session.md](../07-full-system-chaos/long-running-session.md) - Extended session stability
