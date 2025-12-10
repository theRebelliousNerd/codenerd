# Dream State Load Stress Test

Stress test for speculative execution via dream state.

## Overview

Tests the Dream State with:

- 4+ consultants with 100+ perspectives
- Parallel speculation
- Result aggregation
- Memory consumption

**Expected Duration:** 25-40 minutes

## Quick Reference

### Test Commands

```bash
# Trigger dream state with complex question
./nerd.exe dream "what if we rewrote the entire codebase in Rust"

# Multiple dream queries
./nerd.exe dream "analyze 10 different architectural approaches"
./nerd.exe dream "predict consequences of removing the kernel"

# Check dream state
./nerd.exe query "dream_consultant"
./nerd.exe query "dream_perspective"
```

### Expected Behavior

- All consultants should respond
- Perspectives should be aggregated
- Memory should remain bounded
- Results should be coherent

---

## Severity Levels

### Conservative
- 2 consultants, 10 perspectives each

### Aggressive
- 4 consultants, 50 perspectives each

### Chaos
- 6 consultants, 100 perspectives each

### Hybrid
- Combine with active coding session

